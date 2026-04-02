package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/network"
	"FarmNode/internal/state"
	"FarmNode/internal/storage"
)

// ════════════════════════════════════════════════════════════════════════════
// WebSocket nativo — RFC 6455 (sem dependencias externas)
// ════════════════════════════════════════════════════════════════════════════

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
const (
	wsOpText  = 0x1
	wsOpClose = 0x8
	wsOpPing  = 0x9
	wsOpPong  = 0xA
)

func wsAcceptKey(k string) string {
	h := sha1.New()
	h.Write([]byte(k + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func wsUpgrade(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.ReadWriter, error) {
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		return nil, nil, fmt.Errorf("nao e WebSocket")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, nil, fmt.Errorf("chave ausente")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack indisponivel")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
		wsAcceptKey(key) + "\r\n\r\n"
	rw.WriteString(resp)
	rw.Flush()
	return conn, rw, nil
}

func wsLerFrame(r io.Reader) ([]byte, byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, 0, err
	}
	op := hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
	plen := int(hdr[1] & 0x7F)
	switch plen {
	case 126:
		ext := make([]byte, 2)
		io.ReadFull(r, ext)
		plen = int(ext[0])<<8 | int(ext[1])
	case 127:
		ext := make([]byte, 8)
		io.ReadFull(r, ext)
		plen = int(ext[4])<<24 | int(ext[5])<<16 | int(ext[6])<<8 | int(ext[7])
	}
	var mk [4]byte
	if masked {
		io.ReadFull(r, mk[:])
	}
	payload := make([]byte, plen)
	io.ReadFull(r, payload)
	if masked {
		for i := range payload {
			payload[i] ^= mk[i%4]
		}
	}
	return payload, op, nil
}

func wsMontar(op byte, payload []byte) []byte {
	l := len(payload)
	var h []byte
	h = append(h, 0x80|op)
	switch {
	case l <= 125:
		h = append(h, byte(l))
	case l <= 65535:
		h = append(h, 126, byte(l>>8), byte(l))
	default:
		h = append(h, 127, 0, 0, 0, 0, byte(l>>24), byte(l>>16), byte(l>>8), byte(l))
	}
	return append(h, payload...)
}

func wsEnviarTexto(conn net.Conn, mu *sync.Mutex, data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	_, err := conn.Write(wsMontar(wsOpText, data))
	return err
}

// ════════════════════════════════════════════════════════════════════════════
// Hub WebSocket — multiplos clientes (browsers) simultaneos
//
// O "cliente" do sistema e a PESSOA que visualiza o dashboard.
// Podem existir varios browsers conectados ao mesmo servidor simultaneamente.
// ════════════════════════════════════════════════════════════════════════════

type clienteWS struct {
	conn    net.Conn
	writeMu sync.Mutex
}

type Hub struct {
	mu       sync.RWMutex
	clientes map[*clienteWS]struct{}
}

var hub = &Hub{clientes: make(map[*clienteWS]struct{})}

func (h *Hub) registrar(c *clienteWS) {
	h.mu.Lock()
	h.clientes[c] = struct{}{}
	h.mu.Unlock()
	logger.Integrador.Printf("[WS] Cliente (browser) conectado: %s. Total: %d",
		c.conn.RemoteAddr(), h.total())
}

func (h *Hub) remover(c *clienteWS) {
	h.mu.Lock()
	delete(h.clientes, c)
	h.mu.Unlock()
	c.conn.Close()
	logger.Integrador.Printf("[WS] Cliente desconectado. Total: %d", h.total())
}

func (h *Hub) total() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clientes)
}

func (h *Hub) broadcast(tipo string, dados interface{}) {
	payload, err := json.Marshal(dados)
	if err != nil { return }
	msg, err := json.Marshal(map[string]interface{}{
		"tipo":  tipo,
		"dados": json.RawMessage(payload),
	})
	if err != nil { return }
	h.mu.RLock()
	for c := range h.clientes {
		c := c
		go wsEnviarTexto(c.conn, &c.writeMu, msg) //nolint
	}
	h.mu.RUnlock()
}

// ════════════════════════════════════════════════════════════════════════════
// Monitoramento de falha de sensores UDP
//
// UDP nao tem conexao — o servidor detecta falhas rastreando quando foi
// recebido o ultimo datagrama de cada sensor.
// Se um sensor nao enviar dados por mais de SensorTimeoutMs, gera alerta.
// ════════════════════════════════════════════════════════════════════════════

const SensorTimeoutMs = 5000 // 5 segundos sem dados = sensor perdido

var (
	ultimaLeituraMu sync.Mutex
	ultimaLeitura   = make(map[string]time.Time) // "nodeID|tipo" -> ultimo recebimento
)

func registrarAtividadeSensor(nodeID, tipo string) {
	ultimaLeituraMu.Lock()
	ultimaLeitura[nodeID+"|"+tipo] = time.Now()
	ultimaLeituraMu.Unlock()
}

// monitorarSensores verifica periodicamente se algum sensor parou de enviar.
// Roda em goroutine separada.
func monitorarSensores() {
	// Sensores esperados
	sensoresEsperados := []struct{ node, tipo string }{
		{"Estufa_A", "umidade"},
		{"Estufa_A", "temperatura"},
		{"Estufa_A", "luminosidade"},
		{"Galinheiro_A", "amonia"},
		{"Galinheiro_A", "temperatura"},
		{"Galinheiro_A", "racao"},
		{"Galinheiro_A", "agua"},
	}

	// Aguarda 10s antes de comecar a monitorar (tempo para sensores iniciarem)
	time.Sleep(10 * time.Second)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ultimaLeituraMu.Lock()
		agora := time.Now()
		for _, s := range sensoresEsperados {
			chave := s.node + "|" + s.tipo
			ultima, visto := ultimaLeitura[chave]
			if !visto {
				// Nunca recebeu dados deste sensor
				logger.Integrador.Printf("[MONITOR] Sensor %s/%s: nenhum dado recebido ainda", s.node, s.tipo)
				continue
			}
			atraso := agora.Sub(ultima).Milliseconds()
			if atraso > SensorTimeoutMs {
				msg := fmt.Sprintf("Sensor %s/%s sem dados ha %dms — possivel falha de conexao UDP!",
					s.node, s.tipo, atraso)
				logger.Integrador.Printf("[MONITOR] FALHA: %s", msg)
				storage.LogAlerta(s.node, s.tipo, 0, msg, "critico")
				alertas, _ := storage.GetAlertas(true)
				hub.broadcast("alerta", alertas)
			}
		}
		ultimaLeituraMu.Unlock()
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Throttle de alertas
// ════════════════════════════════════════════════════════════════════════════

var (
	ultimoAlerta   = make(map[string]time.Time)
	ultimoAlertaMu sync.Mutex
)

func gerarAlerta(nodeID, tipo string, valor float64, mensagem, nivel string) {
	ultimoAlertaMu.Lock()
	key   := nodeID + "|" + tipo + "|" + nivel
	ultima := ultimoAlerta[key]
	ultimoAlertaMu.Unlock()
	if time.Since(ultima) < 20*time.Second { return }
	ultimoAlertaMu.Lock()
	ultimoAlerta[key] = time.Now()
	ultimoAlertaMu.Unlock()

	logger.Integrador.Printf("[ALERTA/%s] %s/%s: %s (=%.2f)", nivel, nodeID, tipo, mensagem, valor)
	storage.LogAlerta(nodeID, tipo, valor, mensagem, nivel)
	alertas, _ := storage.GetAlertas(true)
	hub.broadcast("alerta", alertas)
}

// ════════════════════════════════════════════════════════════════════════════
// Worker Pool UDP — suporta ~7000 msg/s sem spawn ilimitado de goroutines
// ════════════════════════════════════════════════════════════════════════════

const (
	NumWorkers   = 64
	UDPQueueSize = 8192
)

type pacoteUDP struct {
	dados  []byte
	origem *net.UDPAddr
}

var udpQueue = make(chan pacoteUDP, UDPQueueSize)

func iniciarWorkers() {
	for i := 0; i < NumWorkers; i++ {
		go func() {
			for pkt := range udpQueue {
				handleSensorUDP(pkt.dados, pkt.origem)
			}
		}()
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Main
// ════════════════════════════════════════════════════════════════════════════

func main() {
	if err := storage.InitDB("farmnode_logs.db"); err != nil {
		log.Fatalf("Erro ao inicializar storage: %v", err)
	}
	defer storage.CloseDB()

	iniciarWorkers()

	// Listener TCP :6000 — atuadores conectam aqui (porta unica para todos)
	go network.EscutarAtuadoresTCP("0.0.0.0:6000")

	// Monitoramento de falha de sensores UDP
	go monitorarSensores()

	go startDashboard()

	// Listener UDP :8080 — sensores enviam datagramas aqui
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:8080")
	if err != nil { log.Fatalf("UDP addr: %v", err) }
	conn, err := net.ListenUDP("udp", addr)
	if err != nil { log.Fatalf("UDP listen: %v", err) }
	defer conn.Close()

	logger.Integrador.Println("FarmNode v3 iniciado!")
	logger.Integrador.Printf("  -> Sensores  : UDP  :8080 (1ms, worker pool=%d, fila=%d)", NumWorkers, UDPQueueSize)
	logger.Integrador.Println("  -> Atuadores : TCP  :6000 (porta unica — atuadores conectam ao servidor)")
	logger.Integrador.Println("  -> Dashboard : HTTP :8082/dashboard  (multiplos clientes WS simultaneos)")
	logger.Integrador.Println("  -> WebSocket : WS   :8082/ws  (RFC 6455 nativo)")

	buf := make([]byte, 4096)
	dropped := 0
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			logger.Sensor.Printf("Erro UDP: %v", err)
			continue
		}
		pacote := make([]byte, n)
		copy(pacote, buf[:n])
		select {
		case udpQueue <- pacoteUDP{dados: pacote, origem: remoteAddr}:
		default:
			dropped++
			if dropped%1000 == 0 {
				logger.Sensor.Printf("[AVISO] %d datagramas UDP descartados (fila cheia)", dropped)
			}
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Processamento UDP — executado pelos workers
// ════════════════════════════════════════════════════════════════════════════

// ── Filtro de log de sensores ─────────────────────────────────────────────────
//
// Com sensores enviando a 1ms, gravar cada leitura geraria ~7000 logs/s.
// Em 15 minutos seriam >6 milhões de registros — inviável.
//
// Critério de salvamento: salva SE qualquer uma das condições for verdadeira:
//  1. O valor variou >= LogMinVariacao em relação ao último valor salvo
//  2. Passaram >= LogMinIntervalo segundos desde o último salvamento
//
// Isso mantém o histórico legível no dashboard sem explodir o JSON.
// O envio UDP a 1ms continua intacto — só o salvamento em disco é filtrado.

const (
	LogMinVariacao  = 0.5              // salva se valor mudou >= 0.5 unidades
	LogMinIntervalo = 2 * time.Second  // salva ao menos a cada 2 segundos
)

var (
	logFilterMu      sync.Mutex
	logUltimoValor   = make(map[string]float64)   // "nodeID|tipo" -> ultimo valor salvo
	logUltimoInstant = make(map[string]time.Time)  // "nodeID|tipo" -> ultimo instante salvo
)

// deveLogar retorna true se esta leitura deve ser persistida em disco.
func deveLogar(nodeID, tipo string, valor float64) bool {
	chave := nodeID + "|" + tipo
	logFilterMu.Lock()
	defer logFilterMu.Unlock()

	ultimo, temValor := logUltimoValor[chave]
	ultimoT := logUltimoInstant[chave]

	variou    := !temValor || absF(valor-ultimo) >= LogMinVariacao
	tempoOk   := time.Since(ultimoT) >= LogMinIntervalo

	if variou || tempoOk {
		logUltimoValor[chave]   = valor
		logUltimoInstant[chave] = time.Now()
		return true
	}
	return false
}

func absF(v float64) float64 {
	if v < 0 { return -v }
	return v
}

// ─────────────────────────────────────────────────────────────────────────────

func handleSensorUDP(dados []byte, origem *net.UDPAddr) {
	var sensor models.MensagemSensor
	if err := json.Unmarshal(dados, &sensor); err != nil {
		logger.Sensor.Printf("Datagrama invalido de %s: %v", origem, err)
		return
	}

	// Registra atividade do sensor (para deteccao de falha)
	registrarAtividadeSensor(sensor.NodeID, sensor.Tipo)

	// Atualiza estado em tempo real (sempre — para dashboard e regras)
	state.Mutex.Lock()
	if _, ok := state.ValoresSensores[sensor.NodeID]; !ok {
		state.ValoresSensores[sensor.NodeID] = make(map[string]float64)
	}
	state.ValoresSensores[sensor.NodeID][sensor.Tipo] = sensor.Valor
	state.Mutex.Unlock()

	// Salva em disco somente se valor variou suficientemente OU tempo minimo passou
	// O dashboard continua recebendo atualizacoes a cada 1s via WebSocket (sem impacto)
	if deveLogar(sensor.NodeID, sensor.Tipo, sensor.Valor) {
		storage.LogSensor(sensor)
	}

	processarRegrasAutomaticas(sensor)
}

// ════════════════════════════════════════════════════════════════════════════
// Regras automaticas — TODA interpretacao ocorre aqui, no servidor
//
// O servidor recebe valores crus dos sensores e decide:
//  - Acionar atuadores (ligar/desligar)
//  - Gerar alertas de aviso ou critico
//  - Empurrar atualizacoes para os clientes via WebSocket
// ════════════════════════════════════════════════════════════════════════════

func processarRegrasAutomaticas(sensor models.MensagemSensor) {
	type ap struct{ nodeID, tipo, msg, nivel string; val float64 }
	var alertas []ap

	state.Mutex.Lock()

	if strings.HasPrefix(sensor.NodeID, "Estufa") {
		switch sensor.Tipo {
		case "umidade":
			min, max, crit := state.AlvoUmidadeMinima[sensor.NodeID], state.AlvoUmidadeMaxima[sensor.NodeID], state.LimiteCriticoUmidade[sensor.NodeID]
			on := state.BombaIrrigacao[sensor.NodeID]
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.2f%% < %.1f%% -> LIGAR BOMBA", sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_baixa")
				state.BombaIrrigacao[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_baixa")
			} else if sensor.Valor > max && on {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.2f%% > %.1f%% -> DESLIGAR BOMBA", sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
				state.BombaIrrigacao[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "umidade",
					fmt.Sprintf("Umidade CRITICA: %.2f%% — bomba nao reagiu!", sensor.Valor), "critico", sensor.Valor})
			}

		case "temperatura":
			max, crit := state.AlvoTempMaxima[sensor.NodeID], state.LimiteCriticoTempEstufa[sensor.NodeID]
			on := state.Ventilador[sensor.NodeID]
			if sensor.Valor > max && !on {
				logger.Integrador.Printf("[AUTO] %s: Temp %.2fC > %.1fC -> LIGAR VENTILADOR", sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "ventilador_01", "LIGAR", "temp_alta")
				state.Ventilador[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "ventilador_01", "LIGAR", "temp_alta")
			} else if sensor.Valor < max-5.0 && on {
				logger.Integrador.Printf("[AUTO] %s: Temp %.2fC normalizada -> DESLIGAR VENTILADOR", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "ventilador_01", "DESLIGAR", "temp_normal")
				state.Ventilador[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "ventilador_01", "DESLIGAR", "temp_normal")
			}
			if sensor.Valor > crit {
				alertas = append(alertas, ap{sensor.NodeID, "temperatura",
					fmt.Sprintf("Temperatura CRITICA: %.2fC!", sensor.Valor), "critico", sensor.Valor})
			}

		case "luminosidade":
			min, crit := state.AlvoLuzMinima[sensor.NodeID], state.LimiteCriticoLuminosidade[sensor.NodeID]
			on := state.LuzArtifical[sensor.NodeID]
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Luz %.2f Lux < %.1f -> LIGAR LED", sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "painel_led_01", "LIGAR", "luz_baixa")
				state.LuzArtifical[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "painel_led_01", "LIGAR", "luz_baixa")
			} else if sensor.Valor > min*2 && on {
				logger.Integrador.Printf("[AUTO] %s: Luz %.2f Lux normalizada -> DESLIGAR LED", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "painel_led_01", "DESLIGAR", "luz_ok")
				state.LuzArtifical[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "painel_led_01", "DESLIGAR", "luz_ok")
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "luminosidade",
					fmt.Sprintf("Luminosidade CRITICA: %.2f Lux!", sensor.Valor), "critico", sensor.Valor})
			}
		}
	}

	if strings.HasPrefix(sensor.NodeID, "Galinheiro") {
		switch sensor.Tipo {
		case "amonia":
			max, crit := state.AlvoAmoniaMaxima[sensor.NodeID], state.LimiteCriticoAmonia[sensor.NodeID]
			on := state.Exaustor[sensor.NodeID]
			if sensor.Valor >= max && !on {
				logger.Integrador.Printf("[AUTO] %s: Amonia %.2f ppm >= %.1f -> LIGAR EXAUSTOR", sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "exaustor_teto_01", "LIGAR", "amonia_elevada")
				state.Exaustor[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "exaustor_teto_01", "LIGAR", "amonia_elevada")
				alertas = append(alertas, ap{sensor.NodeID, "amonia",
					fmt.Sprintf("Amonia elevada: %.2f ppm — Exaustor acionado automaticamente", sensor.Valor), "aviso", sensor.Valor})
			} else if sensor.Valor < max-10.0 && on {
				logger.Integrador.Printf("[AUTO] %s: Amonia %.2f ppm normalizada -> DESLIGAR EXAUSTOR", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "exaustor_teto_01", "DESLIGAR", "amonia_normal")
				state.Exaustor[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "exaustor_teto_01", "DESLIGAR", "amonia_normal")
			}
			if sensor.Valor >= crit {
				alertas = append(alertas, ap{sensor.NodeID, "amonia",
					fmt.Sprintf("Amonia CRITICA: %.2f ppm!", sensor.Valor), "critico", sensor.Valor})
			}

		case "temperatura":
			min, crit := state.AlvoTempMinima[sensor.NodeID], state.LimiteCriticoTempGalinheiro[sensor.NodeID]
			on := state.Aquecedor[sensor.NodeID]
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Temp %.2fC < %.1fC -> LIGAR AQUECEDOR", sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "aquecedor_01", "LIGAR", "temp_baixa")
				state.Aquecedor[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "aquecedor_01", "LIGAR", "temp_baixa")
			} else if sensor.Valor > min+5.0 && on {
				logger.Integrador.Printf("[AUTO] %s: Temp %.2fC normalizada -> DESLIGAR AQUECEDOR", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "aquecedor_01", "DESLIGAR", "temp_normal")
				state.Aquecedor[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "aquecedor_01", "DESLIGAR", "temp_normal")
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "temperatura",
					fmt.Sprintf("Temperatura CRITICA: %.2fC!", sensor.Valor), "critico", sensor.Valor})
			}

		case "racao":
			min, max, crit := state.AlvoRacaoMinima[sensor.NodeID], state.AlvoRacaoMaxima[sensor.NodeID], state.LimiteCriticoRacao[sensor.NodeID]
			on := state.MotorComedouro[sensor.NodeID]
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Racao %.2f%% < %.1f%% -> LIGAR MOTOR", sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
				state.MotorComedouro[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
			} else if sensor.Valor >= max && on {
				logger.Integrador.Printf("[AUTO] %s: Racao %.2f%% >= %.1f%% -> DESLIGAR MOTOR", sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
				state.MotorComedouro[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "racao",
					fmt.Sprintf("Racao CRITICA: %.2f%%!", sensor.Valor), "critico", sensor.Valor})
			}

		case "agua":
			min, max, crit := state.AlvoAguaMinima[sensor.NodeID], state.AlvoAguaMaxima[sensor.NodeID], state.LimiteCriticoAgua[sensor.NodeID]
			on := state.ValvulaAgua[sensor.NodeID]
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Agua %.2f%% < %.1f%% -> LIGAR VALVULA", sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "valvula_agua_01", "LIGAR", "agua_baixa")
				state.ValvulaAgua[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "valvula_agua_01", "LIGAR", "agua_baixa")
			} else if sensor.Valor >= max && on {
				logger.Integrador.Printf("[AUTO] %s: Agua %.2f%% >= %.1f%% -> DESLIGAR VALVULA", sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "valvula_agua_01", "DESLIGAR", "agua_cheia")
				state.ValvulaAgua[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "valvula_agua_01", "DESLIGAR", "agua_cheia")
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "agua",
					fmt.Sprintf("Agua CRITICA: %.2f%%!", sensor.Valor), "critico", sensor.Valor})
			}
		}
	}

	state.Mutex.Unlock()

	for _, a := range alertas {
		gerarAlerta(a.nodeID, a.tipo, a.val, a.msg, a.nivel)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Dashboard HTTP + WebSocket
// ════════════════════════════════════════════════════════════════════════════

func startDashboard() {
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, getDashboardHTML())
	})
	http.HandleFunc("/ws",                  handleWebSocket)
	http.HandleFunc("/api/estado",          handleAPIEstado)   // usado pelos sensores
	http.HandleFunc("/api/sensor/",         handleAPISensorData)
	http.HandleFunc("/api/atuador/history", handleAPIAtuadorHistory)
	http.HandleFunc("/api/alertas",         handleAPIAlertas)
	http.HandleFunc("/api/config",          handleAPIConfigGET)

	logger.Integrador.Println("Dashboard: http://0.0.0.0:8082/dashboard")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatalf("HTTP: %v", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, rw, err := wsUpgrade(w, r)
	if err != nil {
		logger.Integrador.Printf("[WS] Upgrade falhou: %v", err)
		return
	}
	c := &clienteWS{conn: conn}
	hub.registrar(c)
	defer hub.remover(c)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				estado := construirEstado()
				p, _  := json.Marshal(estado)
				msg, _ := json.Marshal(map[string]interface{}{"tipo": "estado", "dados": json.RawMessage(p)})
				if err := wsEnviarTexto(conn, &c.writeMu, msg); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	reader := rw.Reader
	for {
		payload, op, err := wsLerFrame(reader)
		if err != nil { break }
		switch op {
		case wsOpText:
			processarMensagemCliente(payload)
		case wsOpClose:
			c.writeMu.Lock()
			conn.Write(wsMontar(wsOpClose, nil))
			c.writeMu.Unlock()
			return
		case wsOpPing:
			c.writeMu.Lock()
			conn.Write(wsMontar(wsOpPong, payload))
			c.writeMu.Unlock()
		}
	}
}

func processarMensagemCliente(raw []byte) {
	var msg struct {
		Tipo      string          `json:"tipo"`
		NodeID    string          `json:"node_id"`
		AtuadorID string          `json:"atuador_id"`
		Comando   string          `json:"comando"`
		ID        string          `json:"id"`
		Dados     json.RawMessage `json:"dados"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil { return }

	switch msg.Tipo {
	case "comando":
		if msg.NodeID == "" || msg.AtuadorID == "" || msg.Comando == "" { return }
		estado := msg.Comando == "LIGAR"
		state.Mutex.Lock()
		switch msg.AtuadorID {
		case "bomba_irrigacao_01":  state.BombaIrrigacao[msg.NodeID] = estado
		case "ventilador_01":       state.Ventilador[msg.NodeID] = estado
		case "painel_led_01":       state.LuzArtifical[msg.NodeID] = estado
		case "exaustor_teto_01":    state.Exaustor[msg.NodeID] = estado
		case "aquecedor_01":        state.Aquecedor[msg.NodeID] = estado
		case "motor_comedouro_01":  state.MotorComedouro[msg.NodeID] = estado
		case "valvula_agua_01":     state.ValvulaAgua[msg.NodeID] = estado
		default:
			state.Mutex.Unlock()
			return
		}
		state.Mutex.Unlock()
		network.EnviarComandoTCP(msg.NodeID, msg.AtuadorID, msg.Comando, "manual_dashboard")
		storage.LogAtuador(msg.NodeID, msg.AtuadorID, msg.Comando, "manual_dashboard")
		logger.Integrador.Printf("[WS] Comando dashboard: %s/%s -> %s", msg.NodeID, msg.AtuadorID, msg.Comando)

	case "ack_alerta":
		if msg.ID == "" { return }
		storage.AckAlerta(msg.ID)
		alertas, _ := storage.GetAlertas(false)
		hub.broadcast("alerta", alertas)

	case "config":
		if msg.NodeID == "" || msg.Dados == nil { return }
		var campos map[string]float64
		if err := json.Unmarshal(msg.Dados, &campos); err != nil { return }
		aplicarConfig(msg.NodeID, campos)
	}
}

func construirEstado() map[string]interface{} {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	sensEA := state.ValoresSensores["Estufa_A"]
	sensGA := state.ValoresSensores["Galinheiro_A"]
	return map[string]interface{}{
		"Estufa_A": map[string]interface{}{
			"umidade": sensEA["umidade"], "temperatura": sensEA["temperatura"],
			"luminosidade":      sensEA["luminosidade"],
			"bomba_ligada":      state.BombaIrrigacao["Estufa_A"],
			"ventilador_ligado": state.Ventilador["Estufa_A"],
			"led_ligado":        state.LuzArtifical["Estufa_A"],
		},
		"Galinheiro_A": map[string]interface{}{
			"amonia": sensGA["amonia"], "temperatura": sensGA["temperatura"],
			"racao": sensGA["racao"], "agua": sensGA["agua"],
			"exaustor_ligado":  state.Exaustor["Galinheiro_A"],
			"aquecedor_ligado": state.Aquecedor["Galinheiro_A"],
			"motor_ligado":     state.MotorComedouro["Galinheiro_A"],
			"valvula_ligada":   state.ValvulaAgua["Galinheiro_A"],
		},
	}
}

func aplicarConfig(nodeID string, campos map[string]float64) {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	for k, v := range campos {
		switch nodeID + "|" + k {
		case "Estufa_A|umidade_min":       state.AlvoUmidadeMinima[nodeID] = v
		case "Estufa_A|umidade_max":       state.AlvoUmidadeMaxima[nodeID] = v
		case "Estufa_A|temp_max":          state.AlvoTempMaxima[nodeID] = v
		case "Estufa_A|luz_min":           state.AlvoLuzMinima[nodeID] = v
		case "Estufa_A|critico_umidade":   state.LimiteCriticoUmidade[nodeID] = v
		case "Estufa_A|critico_temp":      state.LimiteCriticoTempEstufa[nodeID] = v
		case "Estufa_A|critico_luz":       state.LimiteCriticoLuminosidade[nodeID] = v
		case "Galinheiro_A|racao_min":     state.AlvoRacaoMinima[nodeID] = v
		case "Galinheiro_A|racao_max":     state.AlvoRacaoMaxima[nodeID] = v
		case "Galinheiro_A|amonia_max":    state.AlvoAmoniaMaxima[nodeID] = v
		case "Galinheiro_A|agua_min":      state.AlvoAguaMinima[nodeID] = v
		case "Galinheiro_A|agua_max":      state.AlvoAguaMaxima[nodeID] = v
		case "Galinheiro_A|temp_min":      state.AlvoTempMinima[nodeID] = v
		case "Galinheiro_A|critico_racao": state.LimiteCriticoRacao[nodeID] = v
		case "Galinheiro_A|critico_amonia":state.LimiteCriticoAmonia[nodeID] = v
		case "Galinheiro_A|critico_agua":  state.LimiteCriticoAgua[nodeID] = v
		case "Galinheiro_A|critico_temp":  state.LimiteCriticoTempGalinheiro[nodeID] = v
		}
	}
	logger.Integrador.Printf("[WS] Config atualizada: %s", nodeID)
}

// handleAPIEstado: mantido para que os sensores consultem estado dos atuadores
func handleAPIEstado(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(construirEstado())
}

func handleAPISensorData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tipo := strings.TrimPrefix(r.URL.Path, "/api/sensor/")
	if tipo == "" { w.WriteHeader(http.StatusBadRequest); return }
	horas := 1
	fmt.Sscanf(r.URL.Query().Get("horas"), "%d", &horas)
	dados, err := storage.GetSensorDataByType(tipo, horas)
	if err != nil { w.WriteHeader(http.StatusInternalServerError); return }
	json.NewEncoder(w).Encode(dados)
}

func handleAPIAtuadorHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	horas := 24
	fmt.Sscanf(r.URL.Query().Get("horas"), "%d", &horas)
	dados, err := storage.GetAllAtuadorHistory(horas)
	if err != nil { w.WriteHeader(http.StatusInternalServerError); return }
	json.NewEncoder(w).Encode(dados)
}

func handleAPIAlertas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ativos := r.URL.Query().Get("ativos") == "true"
	alertas, _ := storage.GetAlertas(ativos)
	if alertas == nil { alertas = []storage.AlertaLog{} }
	json.NewEncoder(w).Encode(alertas)
}

func handleAPIConfigGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
	w.Header().Set("Content-Type", "application/json")
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Estufa_A": map[string]interface{}{
			"umidade_min": state.AlvoUmidadeMinima["Estufa_A"],
			"umidade_max": state.AlvoUmidadeMaxima["Estufa_A"],
			"temp_max":    state.AlvoTempMaxima["Estufa_A"],
			"luz_min":     state.AlvoLuzMinima["Estufa_A"],
			"critico_umidade": state.LimiteCriticoUmidade["Estufa_A"],
			"critico_temp":    state.LimiteCriticoTempEstufa["Estufa_A"],
			"critico_luz":     state.LimiteCriticoLuminosidade["Estufa_A"],
		},
		"Galinheiro_A": map[string]interface{}{
			"racao_min":      state.AlvoRacaoMinima["Galinheiro_A"],
			"racao_max":      state.AlvoRacaoMaxima["Galinheiro_A"],
			"amonia_max":     state.AlvoAmoniaMaxima["Galinheiro_A"],
			"agua_min":       state.AlvoAguaMinima["Galinheiro_A"],
			"agua_max":       state.AlvoAguaMaxima["Galinheiro_A"],
			"temp_min":       state.AlvoTempMinima["Galinheiro_A"],
			"critico_racao":  state.LimiteCriticoRacao["Galinheiro_A"],
			"critico_amonia": state.LimiteCriticoAmonia["Galinheiro_A"],
			"critico_agua":   state.LimiteCriticoAgua["Galinheiro_A"],
			"critico_temp":   state.LimiteCriticoTempGalinheiro["Galinheiro_A"],
		},
	})
}
