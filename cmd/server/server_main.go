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
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/network"
	"FarmNode/internal/state"
	"FarmNode/internal/storage"
)

// ── WebSocket nativo RFC 6455 ─────────────────────────────────────────────────

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

// ── Hub WebSocket ─────────────────────────────────────────────────────────────

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
	logger.Integrador.Printf("[WS] Browser conectado: %s (total=%d)", c.conn.RemoteAddr(), h.total())
}

func (h *Hub) remover(c *clienteWS) {
	h.mu.Lock()
	delete(h.clientes, c)
	h.mu.Unlock()
	c.conn.Close()
	logger.Integrador.Printf("[WS] Browser desconectado (total=%d)", h.total())
}

func (h *Hub) total() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clientes)
}

func (h *Hub) broadcast(tipo string, dados interface{}) {
	payload, err := json.Marshal(dados)
	if err != nil {
		return
	}
	msg, err := json.Marshal(map[string]interface{}{"tipo": tipo, "dados": json.RawMessage(payload)})
	if err != nil {
		return
	}
	h.mu.RLock()
	for c := range h.clientes {
		c := c
		go wsEnviarTexto(c.conn, &c.writeMu, msg) //nolint
	}
	h.mu.RUnlock()
}

// ── Constantes ────────────────────────────────────────────────────────────────

const (
	DeviceInactivityTimeout = 5 * time.Minute
	SensorMonitorInterval   = 10 * time.Second
	AtuadorCheckInterval    = 10 * time.Second
	RuleEvalMinInterval     = 50 * time.Millisecond
	LogMinVariacao          = 0.5
	LogMinIntervalo         = 2 * time.Second
	LogMaxPorMinuto         = 300
)

var (
	numWorkers         = envInt("UDP_WORKERS", defaultUDPWorkers(), 8, 4096)
	udpQueueSize       = envInt("UDP_QUEUE_SIZE", defaultUDPQueueSize(), 1024, 1_000_000)
	udpReadBufferBytes = envInt("UDP_READ_BUFFER_BYTES", 16*1024*1024, 256*1024, 256*1024*1024)
	terminalVelocidade = os.Getenv("TERMINAL_VELOCIDADE") == "1"
)

func defaultUDPWorkers() int {
	n := runtime.NumCPU() * 8
	if n < 64 {
		return 64
	}
	if n > 512 {
		return 512
	}
	return n
}

func defaultUDPQueueSize() int {
	q := defaultUDPWorkers() * 2048
	if q < 8192 {
		return 8192
	}
	if q > 262144 {
		return 262144
	}
	return q
}

func envInt(key string, def, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ── Registro de sensores em runtime ──────────────────────────────────────────

type sensorRuntime struct {
	NodeID     string
	SensorID   string
	Tipo       string
	Alias      string
	Unidade    string
	LastValue  float64
	LastSeen   time.Time
	LastSource string
}

var (
	sensoresMu sync.RWMutex
	sensores   = make(map[string]*sensorRuntime) // key: node|sensor_id
	aliasCount = make(map[string]int)            // key: node|tipo -> contador
)

func sensorKey(nodeID, sensorID, origem string) string {
	if sensorID != "" {
		return nodeID + "|" + sensorID
	}
	return nodeID + "|" + origem
}

func registrarSensorRuntime(sensor models.MensagemSensor, origem *net.UDPAddr) *sensorRuntime {
	now := time.Now()
	src := ""
	if origem != nil {
		src = origem.String()
	}
	key := sensorKey(sensor.NodeID, sensor.SensorID, src)

	sensoresMu.Lock()
	defer sensoresMu.Unlock()

	if existente, ok := sensores[key]; ok {
		existente.LastSeen = now
		existente.LastValue = sensor.Valor
		existente.Unidade = sensor.Unidade
		existente.LastSource = src
		return existente
	}

	idxKey := sensor.NodeID + "|" + sensor.Tipo
	aliasCount[idxKey]++
	alias := fmt.Sprintf("%s_%d", sensor.Tipo, aliasCount[idxKey])

	rt := &sensorRuntime{
		NodeID:     sensor.NodeID,
		SensorID:   sensor.SensorID,
		Tipo:       sensor.Tipo,
		Alias:      alias,
		Unidade:    sensor.Unidade,
		LastValue:  sensor.Valor,
		LastSeen:   now,
		LastSource: src,
	}
	sensores[key] = rt
	logger.Sensor.Printf("[DISCOVERY] Sensor: node=%s tipo=%s id=%s alias=%s",
		sensor.NodeID, sensor.Tipo, sensor.SensorID, alias)
	return rt
}

// ── Defaults de configuração por nó ──────────────────────────────────────────

func initNodeDefaults(nodeID string) {
	if nodeEhEstufa(nodeID) {
		setDefault(state.AlvoUmidadeMinima, nodeID, 15.0)
		setDefault(state.AlvoUmidadeMaxima, nodeID, 55.0)
		setDefault(state.AlvoTempMaxima, nodeID, 35.0)
		setDefault(state.AlvoLuzMinima, nodeID, 600.0)
		setDefault(state.LimiteCriticoUmidade, nodeID, 5.0)
		setDefault(state.LimiteCriticoTempEstufa, nodeID, 45.0)
		setDefault(state.LimiteCriticoLuminosidade, nodeID, 100.0)
	}
	if nodeEhGalinheiro(nodeID) {
		setDefault(state.AlvoAmoniaMaxima, nodeID, 20.0)
		setDefault(state.AlvoTempMinima, nodeID, 24.0)
		setDefault(state.AlvoRacaoMinima, nodeID, 10.0)
		setDefault(state.AlvoRacaoMaxima, nodeID, 90.0)
		setDefault(state.AlvoAguaMinima, nodeID, 15.0)
		setDefault(state.AlvoAguaMaxima, nodeID, 80.0)
		setDefault(state.LimiteCriticoAmonia, nodeID, 35.0)
		setDefault(state.LimiteCriticoRacao, nodeID, 5.0)
		setDefault(state.LimiteCriticoAgua, nodeID, 5.0)
		setDefault(state.LimiteCriticoTempGalinheiro, nodeID, 15.0)
	}
}

func setDefault(m map[string]float64, key string, val float64) {
	if _, ok := m[key]; !ok {
		m[key] = val
	}
}

func nodeEhEstufa(nodeID string) bool {
	n := strings.ToLower(strings.TrimSpace(nodeID))
	return strings.HasPrefix(n, "estufa")
}

func nodeEhGalinheiro(nodeID string) bool {
	n := strings.ToLower(strings.TrimSpace(nodeID))
	return strings.HasPrefix(n, "galinheiro")
}

// ── Monitores de inatividade ──────────────────────────────────────────────────

func monitorarSensores() {
	ticker := time.NewTicker(SensorMonitorInterval)
	defer ticker.Stop()
	for range ticker.C {
		agora := time.Now()
		var removidos []*sensorRuntime
		ativosPorNoTipo := make(map[string]int)

		sensoresMu.Lock()
		for key, s := range sensores {
			if agora.Sub(s.LastSeen) > DeviceInactivityTimeout {
				removidos = append(removidos, s)
				delete(sensores, key)
			}
		}
		for _, s := range sensores {
			ativosPorNoTipo[s.NodeID+"|"+s.Tipo]++
		}
		sensoresMu.Unlock()

		if len(removidos) == 0 {
			continue
		}
		state.Mutex.Lock()
		for _, s := range removidos {
			if node, ok := state.ValoresSensores[s.NodeID]; ok {
				delete(node, s.Alias)
				if ativosPorNoTipo[s.NodeID+"|"+s.Tipo] == 0 {
					delete(node, s.Tipo)
				}
				if len(node) == 0 {
					delete(state.ValoresSensores, s.NodeID)
				}
			}
			msg := fmt.Sprintf("Sensor %s/%s (%s) removido por inatividade > %s",
				s.NodeID, s.Tipo, s.SensorID, DeviceInactivityTimeout)
			logger.Integrador.Printf("[MONITOR] %s", msg)
			gerarAlerta(s.NodeID, s.Tipo, s.LastValue, msg, "aviso")
		}
		state.Mutex.Unlock()
	}
}

func monitorarAtuadores() {
	ticker := time.NewTicker(AtuadorCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		removidos := network.PruneAtuadoresInativos(DeviceInactivityTimeout)
		for _, a := range removidos {
			// Remove do estado dinâmico também
			state.Mutex.Lock()
			if m, ok := state.EstadoAtuadores[a.NodeID]; ok {
				delete(m, a.AtuadorID)
			}
			state.Mutex.Unlock()
			msg := fmt.Sprintf("Atuador %s/%s removido por inatividade > %s",
				a.NodeID, a.AtuadorID, DeviceInactivityTimeout)
			logger.Integrador.Printf("[MONITOR] %s", msg)
			gerarAlerta(a.NodeID, a.AtuadorID, 0, msg, "aviso")
		}
	}
}

// ── Alertas ───────────────────────────────────────────────────────────────────

var (
	ultimoAlerta   = make(map[string]time.Time)
	ultimoAlertaMu sync.Mutex
)

func gerarAlerta(nodeID, tipo string, valor float64, mensagem, nivel string) {
	ultimoAlertaMu.Lock()
	key := nodeID + "|" + tipo + "|" + nivel
	ultima := ultimoAlerta[key]
	ultimoAlertaMu.Unlock()
	if time.Since(ultima) < 20*time.Second {
		return
	}
	ultimoAlertaMu.Lock()
	ultimoAlerta[key] = time.Now()
	ultimoAlertaMu.Unlock()

	logger.Integrador.Printf("[ALERTA/%s] %s/%s: %s (=%.2f)", nivel, nodeID, tipo, mensagem, valor)
	storage.LogAlerta(nodeID, tipo, valor, mensagem, nivel)
	alertas, _ := storage.GetAlertas(true)
	hub.broadcast("alerta", alertas)
}

func acionarAtuador(nodeID, atuadorID, comando, motivo string) bool {
	if !network.AtuadorConectado(nodeID, atuadorID) {
		msg := fmt.Sprintf("Atuador %s/%s indisponivel: '%s' nao enviado", nodeID, atuadorID, comando)
		logger.Integrador.Printf("[AUTO] %s", msg)
		gerarAlerta(nodeID, atuadorID, 0, msg, "critico")
		return false
	}
	if ok := network.EnviarComandoTCP(nodeID, atuadorID, comando, motivo); !ok {
		msg := fmt.Sprintf("Atuador %s/%s desconectou durante envio: '%s' falhou", nodeID, atuadorID, comando)
		logger.Integrador.Printf("[AUTO] %s", msg)
		gerarAlerta(nodeID, atuadorID, 0, msg, "critico")
		return false
	}
	return true
}

// ── Worker pool UDP ───────────────────────────────────────────────────────────

type pacoteUDP struct {
	dados  []byte
	origem *net.UDPAddr
}

var udpQueue chan pacoteUDP
var (
	totalDatagramas     uint64
	ultimoDatagramaNS   int64
	totalUDPInvalidos   uint64
	ultimoInvalidoDatNS int64
)

var (
	regrasThrottleMu sync.Mutex
	ultimoRegraRun   = make(map[string]time.Time)
)

func iniciarWorkers() {
	for i := 0; i < numWorkers; i++ {
		go func() {
			for pkt := range udpQueue {
				handleSensorUDP(pkt.dados, pkt.origem)
			}
		}()
	}
}

func deveProcessarRegras(nodeID, tipo string, agora time.Time) bool {
	key := nodeID + "|" + tipo
	regrasThrottleMu.Lock()
	defer regrasThrottleMu.Unlock()
	ult := ultimoRegraRun[key]
	if !ult.IsZero() && agora.Sub(ult) < RuleEvalMinInterval {
		return false
	}
	ultimoRegraRun[key] = agora
	return true
}

// ── Filtro de log ─────────────────────────────────────────────────────────────

var (
	logFilterMu      sync.Mutex
	logUltimoValor   = make(map[string]float64)
	logUltimoInstant = make(map[string]time.Time)
	logJanelaMinuto  = make(map[string]time.Time)
	logContadorMin   = make(map[string]int)
)

func deveLogar(nodeID, tipo string, valor float64) bool {
	chave := nodeID + "|" + tipo
	logFilterMu.Lock()
	defer logFilterMu.Unlock()

	ultimo, temValor := logUltimoValor[chave]
	ultimoT := logUltimoInstant[chave]

	variou := !temValor || absF(valor-ultimo) >= LogMinVariacao
	tempoOk := time.Since(ultimoT) >= LogMinIntervalo

	agora := time.Now()
	janela := logJanelaMinuto[chave]
	if janela.IsZero() || agora.Sub(janela) >= time.Minute {
		logJanelaMinuto[chave] = agora
		logContadorMin[chave] = 0
	}

	if variou || tempoOk {
		if !variou && logContadorMin[chave] >= LogMaxPorMinuto {
			return false
		}
		logUltimoValor[chave] = valor
		logUltimoInstant[chave] = agora
		logContadorMin[chave]++
		return true
	}
	return false
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ── Processamento UDP ─────────────────────────────────────────────────────────

func handleSensorUDP(dados []byte, origem *net.UDPAddr) {
	var sensor models.MensagemSensor
	if err := json.Unmarshal(dados, &sensor); err != nil {
		t := atomic.AddUint64(&totalUDPInvalidos, 1)
		atomic.StoreInt64(&ultimoInvalidoDatNS, time.Now().UnixNano())
		if t%1000 == 0 {
			logger.Sensor.Printf("[AVISO] %d datagramas UDP inválidos (JSON)", t)
		}
		return
	}
	recebidoEm := time.Now()
	atomic.AddUint64(&totalDatagramas, 1)
	atomic.StoreInt64(&ultimoDatagramaNS, recebidoEm.UnixNano())

	rt := registrarSensorRuntime(sensor, origem)

	state.Mutex.Lock()
	if _, ok := state.ValoresSensores[sensor.NodeID]; !ok {
		state.ValoresSensores[sensor.NodeID] = make(map[string]float64)
	}
	initNodeDefaults(sensor.NodeID)
	state.ValoresSensores[sensor.NodeID][sensor.Tipo] = sensor.Valor
	state.ValoresSensores[sensor.NodeID][rt.Alias] = sensor.Valor
	state.Mutex.Unlock()

	if terminalVelocidade {
		logger.Sensor.Printf("[VELO] %s node=%s tipo=%s id=%s alias=%s val=%.3f",
			recebidoEm.Format(time.RFC3339Nano), sensor.NodeID, sensor.Tipo,
			sensor.SensorID, rt.Alias, sensor.Valor)
	}

	if deveLogar(sensor.NodeID, sensor.Tipo, sensor.Valor) {
		storage.LogSensor(sensor)
	}

	if deveProcessarRegras(sensor.NodeID, sensor.Tipo, recebidoEm) {
		processarRegrasAutomaticas(sensor)
	}
}

// ── Regras automáticas — totalmente dinâmicas ─────────────────────────────────
//
// Em vez de IDs fixos, o servidor busca atuadores pelo PREFIXO do tipo.
// Ex: para acionar a bomba do nó "Estufa_A", busca o primeiro atuador_id
// cujo prefixo seja "bomba" registrado naquele nó.
// Se não houver nenhum conectado, gera alerta.

func processarRegrasAutomaticas(sensor models.MensagemSensor) {
	type ap struct {
		nodeID, tipo, msg, nivel string
		val                      float64
	}
	var alertas []ap

	state.Mutex.Lock()

	if nodeEhEstufa(sensor.NodeID) {
		switch sensor.Tipo {

		case "umidade":
			min := state.AlvoUmidadeMinima[sensor.NodeID]
			max := state.AlvoUmidadeMaxima[sensor.NodeID]
			crit := state.LimiteCriticoUmidade[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "bomba", sensor.SensorID)
			if atuID == "" {
				break // nenhum atuador de bomba conectado
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor < min && !on {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.2f%% < %.1f%% -> LIGAR %s", sensor.NodeID, sensor.Valor, min, atuID)
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "umidade_baixa") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "umidade_baixa")
					alertas = append(alertas, ap{sensor.NodeID, "umidade",
						fmt.Sprintf("Umidade baixa: %.2f%% — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor > max && on {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.2f%% > %.1f%% -> DESLIGAR %s", sensor.NodeID, sensor.Valor, max, atuID)
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "umidade_ideal") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "umidade_ideal")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "umidade",
					fmt.Sprintf("Umidade CRITICA: %.2f%%!", sensor.Valor), "critico", sensor.Valor})
			}

		case "temperatura":
			max := state.AlvoTempMaxima[sensor.NodeID]
			crit := state.LimiteCriticoTempEstufa[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "ventilador", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor > max && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "temp_alta") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "temp_alta")
					alertas = append(alertas, ap{sensor.NodeID, "temperatura",
						fmt.Sprintf("Temperatura alta: %.2fC — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor < max-5.0 && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "temp_normal") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "temp_normal")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor > crit {
				alertas = append(alertas, ap{sensor.NodeID, "temperatura",
					fmt.Sprintf("Temperatura CRITICA: %.2fC!", sensor.Valor), "critico", sensor.Valor})
			}

		case "luminosidade":
			min := state.AlvoLuzMinima[sensor.NodeID]
			crit := state.LimiteCriticoLuminosidade[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "led", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor < min && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "luz_baixa") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "luz_baixa")
					alertas = append(alertas, ap{sensor.NodeID, "luminosidade",
						fmt.Sprintf("Luz baixa: %.2f Lux — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor > min*2 && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "luz_ok") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "luz_ok")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "luminosidade",
					fmt.Sprintf("Luminosidade CRITICA: %.2f Lux!", sensor.Valor), "critico", sensor.Valor})
			}
		}
	}

	if nodeEhGalinheiro(sensor.NodeID) {
		switch sensor.Tipo {

		case "amonia":
			max := state.AlvoAmoniaMaxima[sensor.NodeID]
			crit := state.LimiteCriticoAmonia[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "exaustor", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor >= max && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "amonia_elevada") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "amonia_elevada")
					alertas = append(alertas, ap{sensor.NodeID, "amonia",
						fmt.Sprintf("Amonia elevada: %.2f ppm — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor < max-10.0 && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "amonia_normal") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "amonia_normal")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor >= crit {
				alertas = append(alertas, ap{sensor.NodeID, "amonia",
					fmt.Sprintf("Amonia CRITICA: %.2f ppm!", sensor.Valor), "critico", sensor.Valor})
			}

		case "temperatura":
			min := state.AlvoTempMinima[sensor.NodeID]
			crit := state.LimiteCriticoTempGalinheiro[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "aquecedor", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor < min && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "temp_baixa") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "temp_baixa")
					alertas = append(alertas, ap{sensor.NodeID, "temperatura",
						fmt.Sprintf("Temp baixa: %.2fC — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor > min+5.0 && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "temp_normal") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "temp_normal")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "temperatura",
					fmt.Sprintf("Temperatura CRITICA: %.2fC!", sensor.Valor), "critico", sensor.Valor})
			}

		case "racao":
			min := state.AlvoRacaoMinima[sensor.NodeID]
			max := state.AlvoRacaoMaxima[sensor.NodeID]
			crit := state.LimiteCriticoRacao[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "motor", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor < min && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "racao_baixa") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "racao_baixa")
					alertas = append(alertas, ap{sensor.NodeID, "racao",
						fmt.Sprintf("Racao baixa: %.2f%% — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor >= max && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "racao_cheia") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "racao_cheia")
				} else {
					state.Mutex.Lock()
				}
			}
			if sensor.Valor < crit {
				alertas = append(alertas, ap{sensor.NodeID, "racao",
					fmt.Sprintf("Racao CRITICA: %.2f%%!", sensor.Valor), "critico", sensor.Valor})
			}

		case "agua":
			min := state.AlvoAguaMinima[sensor.NodeID]
			max := state.AlvoAguaMaxima[sensor.NodeID]
			crit := state.LimiteCriticoAgua[sensor.NodeID]
			atuID := state.FindAtuadorPorTipoParaChave(sensor.NodeID, "valvula", sensor.SensorID)
			if atuID == "" {
				break
			}
			on := state.GetAtuador(sensor.NodeID, atuID)
			if sensor.Valor < min && !on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "LIGAR", "agua_baixa") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, true)
					storage.LogAtuador(sensor.NodeID, atuID, "LIGAR", "agua_baixa")
					alertas = append(alertas, ap{sensor.NodeID, "agua",
						fmt.Sprintf("Agua baixa: %.2f%% — %s acionado", sensor.Valor, atuID), "aviso", sensor.Valor})
				} else {
					state.Mutex.Lock()
				}
			} else if sensor.Valor >= max && on {
				state.Mutex.Unlock()
				if acionarAtuador(sensor.NodeID, atuID, "DESLIGAR", "agua_cheia") {
					state.Mutex.Lock()
					state.SetAtuador(sensor.NodeID, atuID, false)
					storage.LogAtuador(sensor.NodeID, atuID, "DESLIGAR", "agua_cheia")
				} else {
					state.Mutex.Lock()
				}
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

// ── Construir estado para dashboard/API ───────────────────────────────────────

func construirEstado() map[string]interface{} {
	// Captura estado dos atuadores enquanto mutex está locked
	state.Mutex.Lock()
	out := make(map[string]interface{})
	for nodeID, sens := range state.ValoresSensores {
		nodeData := make(map[string]interface{})
		for tipo, val := range sens {
			nodeData[tipo] = val
		}
		for atuID, ligado := range state.AtuadoresDoNo(nodeID) {
			nodeData["atu_"+atuID] = ligado
		}
		out[nodeID] = nodeData
	}
	// Captura snapshot do estado dos atuadores para usar fora do mutex
	estadoSnap := make(map[string]map[string]bool)
	for nodeID, m := range state.EstadoAtuadores {
		snap := make(map[string]bool, len(m))
		for k, v := range m {
			snap[k] = v
		}
		estadoSnap[nodeID] = snap
	}
	state.Mutex.Unlock()

	// Lista de sensores por nó
	sensoresMu.RLock()
	for _, s := range sensores {
		nodeRaw, ok := out[s.NodeID]
		if !ok {
			nodeRaw = map[string]interface{}{}
			out[s.NodeID] = nodeRaw
		}
		node := nodeRaw.(map[string]interface{})
		list, _ := node["_sensores"].([]map[string]interface{})
		list = append(list, map[string]interface{}{
			"sensor_id": s.SensorID,
			"tipo":      s.Tipo,
			"alias":     s.Alias,
			"valor":     s.LastValue,
			"unidade":   s.Unidade,
			"last_seen": s.LastSeen.Format(time.RFC3339Nano),
		})
		node["_sensores"] = list
	}
	sensoresMu.RUnlock()

	// Garante que todos os nós já em out tenham _sensores e _atuadores
	allAtuadores := network.AtuadoresConectadosInfo()

	// Índice por nodeID para acesso rápido
	atuPorNo := make(map[string][]network.AtuadorConectadoInfo)
	for _, a := range allAtuadores {
		atuPorNo[a.NodeID] = append(atuPorNo[a.NodeID], a)
	}

	// Monta _atuadores para nós já conhecidos
	for nodeID := range out {
		node := out[nodeID].(map[string]interface{})
		if _, ok := node["_sensores"]; !ok {
			node["_sensores"] = []map[string]interface{}{}
		}
		list := make([]map[string]interface{}, 0)
		for _, a := range atuPorNo[nodeID] {
			ligado := false
			if m, ok := estadoSnap[a.NodeID]; ok {
				ligado = m[a.AtuadorID]
			}
			list = append(list, map[string]interface{}{
				"node_id":    a.NodeID,
				"atuador_id": a.AtuadorID,
				"tipo":       a.Tipo,
				"conectado":  a.Conectado,
				"ligado":     ligado,
				"last_seen":  a.LastSeen.Format(time.RFC3339Nano),
			})
		}
		node["_atuadores"] = list
	}

	// Nós que só têm atuadores (sem sensores)
	for nodeID, atus := range atuPorNo {
		if _, ok := out[nodeID]; ok {
			continue
		}
		list := make([]map[string]interface{}, 0, len(atus))
		for _, a := range atus {
			ligado := false
			if m, ok := estadoSnap[a.NodeID]; ok {
				ligado = m[a.AtuadorID]
			}
			list = append(list, map[string]interface{}{
				"node_id":    a.NodeID,
				"atuador_id": a.AtuadorID,
				"tipo":       a.Tipo,
				"conectado":  a.Conectado,
				"ligado":     ligado,
				"last_seen":  a.LastSeen.Format(time.RFC3339Nano),
			})
		}
		out[nodeID] = map[string]interface{}{
			"_sensores":  []map[string]interface{}{},
			"_atuadores": list,
		}
	}

	return out
}

// ── Rotas HTTP / WebSocket ────────────────────────────────────────────────────

func startDashboard() {
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, getDashboardHTML())
	})
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/estado", handleAPIEstado)
	http.HandleFunc("/api/sensor/", handleAPISensorData)
	http.HandleFunc("/api/atuador/history", handleAPIAtuadorHistory)
	http.HandleFunc("/api/alertas", handleAPIAlertas)
	http.HandleFunc("/api/config", handleAPIConfig)
	http.HandleFunc("/api/atuadores/conectados", handleAPIAtuadoresConectados)
	http.HandleFunc("/api/velocidade", handleAPIVelocidade)

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
				p, _ := json.Marshal(estado)
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
		if err != nil {
			break
		}
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
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Tipo {

	case "comando":
		if msg.NodeID == "" || msg.AtuadorID == "" || msg.Comando == "" {
			hub.broadcast("comando_resultado", map[string]interface{}{
				"ok":         false,
				"node_id":    msg.NodeID,
				"atuador_id": msg.AtuadorID,
				"comando":    msg.Comando,
				"erro":       "payload_invalido",
			})
			return
		}
		// Dinâmico: aciona qualquer atuador_id sem switch fixo
		if acionarAtuador(msg.NodeID, msg.AtuadorID, msg.Comando, "manual_dashboard") {
			estado := msg.Comando == "LIGAR"
			state.Mutex.Lock()
			state.SetAtuador(msg.NodeID, msg.AtuadorID, estado)
			state.Mutex.Unlock()
			storage.LogAtuador(msg.NodeID, msg.AtuadorID, msg.Comando, "manual_dashboard")
			logger.Integrador.Printf("[WS] Comando dashboard: %s/%s -> %s", msg.NodeID, msg.AtuadorID, msg.Comando)
			hub.broadcast("comando_resultado", map[string]interface{}{
				"ok":         true,
				"node_id":    msg.NodeID,
				"atuador_id": msg.AtuadorID,
				"comando":    msg.Comando,
			})
		} else {
			hub.broadcast("comando_resultado", map[string]interface{}{
				"ok":         false,
				"node_id":    msg.NodeID,
				"atuador_id": msg.AtuadorID,
				"comando":    msg.Comando,
				"erro":       "atuador_indisponivel_ou_falha_envio",
			})
		}

	case "ack_alerta":
		if msg.ID == "" {
			return
		}
		storage.AckAlerta(msg.ID)
		alertas, _ := storage.GetAlertas(false)
		hub.broadcast("alerta", alertas)

	case "config":
		if msg.NodeID == "" || msg.Dados == nil {
			return
		}
		var campos map[string]float64
		if err := json.Unmarshal(msg.Dados, &campos); err != nil {
			return
		}
		aplicarConfig(msg.NodeID, campos)
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

func aplicarConfig(nodeID string, campos map[string]float64) {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	initNodeDefaults(nodeID)
	for k, v := range campos {
		switch k {
		case "umidade_min":
			state.AlvoUmidadeMinima[nodeID] = v
		case "umidade_max":
			state.AlvoUmidadeMaxima[nodeID] = v
		case "temp_max":
			state.AlvoTempMaxima[nodeID] = v
		case "luz_min":
			state.AlvoLuzMinima[nodeID] = v
		case "critico_umidade":
			state.LimiteCriticoUmidade[nodeID] = v
		case "critico_temp":
			if nodeEhEstufa(nodeID) {
				state.LimiteCriticoTempEstufa[nodeID] = v
			} else {
				state.LimiteCriticoTempGalinheiro[nodeID] = v
			}
		case "critico_luz":
			state.LimiteCriticoLuminosidade[nodeID] = v
		case "racao_min":
			state.AlvoRacaoMinima[nodeID] = v
		case "racao_max":
			state.AlvoRacaoMaxima[nodeID] = v
		case "amonia_max":
			state.AlvoAmoniaMaxima[nodeID] = v
		case "agua_min":
			state.AlvoAguaMinima[nodeID] = v
		case "agua_max":
			state.AlvoAguaMaxima[nodeID] = v
		case "temp_min":
			state.AlvoTempMinima[nodeID] = v
		case "critico_racao":
			state.LimiteCriticoRacao[nodeID] = v
		case "critico_amonia":
			state.LimiteCriticoAmonia[nodeID] = v
		case "critico_agua":
			state.LimiteCriticoAgua[nodeID] = v
		}
	}
	logger.Integrador.Printf("[CONFIG] Atualizado: %s %v", nodeID, campos)
}

func configPorNode(nodeID string) map[string]interface{} {
	cfg := map[string]interface{}{}
	if nodeEhEstufa(nodeID) {
		cfg["umidade_min"] = state.AlvoUmidadeMinima[nodeID]
		cfg["umidade_max"] = state.AlvoUmidadeMaxima[nodeID]
		cfg["temp_max"] = state.AlvoTempMaxima[nodeID]
		cfg["luz_min"] = state.AlvoLuzMinima[nodeID]
		cfg["critico_umidade"] = state.LimiteCriticoUmidade[nodeID]
		cfg["critico_temp"] = state.LimiteCriticoTempEstufa[nodeID]
		cfg["critico_luz"] = state.LimiteCriticoLuminosidade[nodeID]
	}
	if nodeEhGalinheiro(nodeID) {
		cfg["racao_min"] = state.AlvoRacaoMinima[nodeID]
		cfg["racao_max"] = state.AlvoRacaoMaxima[nodeID]
		cfg["amonia_max"] = state.AlvoAmoniaMaxima[nodeID]
		cfg["agua_min"] = state.AlvoAguaMinima[nodeID]
		cfg["agua_max"] = state.AlvoAguaMaxima[nodeID]
		cfg["temp_min"] = state.AlvoTempMinima[nodeID]
		cfg["critico_racao"] = state.LimiteCriticoRacao[nodeID]
		cfg["critico_amonia"] = state.LimiteCriticoAmonia[nodeID]
		cfg["critico_agua"] = state.LimiteCriticoAgua[nodeID]
		cfg["critico_temp"] = state.LimiteCriticoTempGalinheiro[nodeID]
	}
	return cfg
}

// ── Handlers HTTP ─────────────────────────────────────────────────────────────

func handleAPIEstado(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(construirEstado())
}

func handleAPISensorData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tipo := strings.TrimPrefix(r.URL.Path, "/api/sensor/")
	if tipo == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	horas := 1
	fmt.Sscanf(r.URL.Query().Get("horas"), "%d", &horas)
	dados, err := storage.GetSensorDataByType(tipo, horas)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(dados)
}

func handleAPIAtuadorHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	horas := 24
	fmt.Sscanf(r.URL.Query().Get("horas"), "%d", &horas)
	dados, err := storage.GetAllAtuadorHistory(horas)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(dados)
}

func handleAPIAlertas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ativos := r.URL.Query().Get("ativos") == "true"
	alertas, _ := storage.GetAlertas(ativos)
	if alertas == nil {
		alertas = []storage.AlertaLog{}
	}
	json.NewEncoder(w).Encode(alertas)
}

func handleAPIAtuadoresConectados(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(network.AtuadoresConectadosInfo())
}

func handleAPIVelocidade(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ultimo := atomic.LoadInt64(&ultimoDatagramaNS)
	resp := map[string]interface{}{
		"total_datagramas":     atomic.LoadUint64(&totalDatagramas),
		"total_invalidos_json": atomic.LoadUint64(&totalUDPInvalidos),
		"ultimo_datagrama":     "",
		"ultimo_invalido_json": "",
		"trace_terminal":       terminalVelocidade,
	}
	if ultimo > 0 {
		resp["ultimo_datagrama"] = time.Unix(0, ultimo).Format(time.RFC3339Nano)
	}
	ultimoInv := atomic.LoadInt64(&ultimoInvalidoDatNS)
	if ultimoInv > 0 {
		resp["ultimo_invalido_json"] = time.Unix(0, ultimoInv).Format(time.RFC3339Nano)
	}
	json.NewEncoder(w).Encode(resp)
}

func handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodPost {
		var req struct {
			NodeID string             `json:"node_id"`
			Dados  map[string]float64 `json:"dados"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NodeID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		aplicarConfig(req.NodeID, req.Dados)
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET: retorna configuração de todos os nós conhecidos
	nodes := map[string]struct{}{}
	state.Mutex.Lock()
	for nodeID := range state.ValoresSensores {
		nodes[nodeID] = struct{}{}
	}
	for nodeID := range state.EstadoAtuadores {
		nodes[nodeID] = struct{}{}
	}
	state.Mutex.Unlock()

	sensoresMu.RLock()
	for _, s := range sensores {
		nodes[s.NodeID] = struct{}{}
	}
	sensoresMu.RUnlock()

	for _, a := range network.AtuadoresConectadosInfo() {
		nodes[a.NodeID] = struct{}{}
	}

	out := map[string]interface{}{}
	state.Mutex.Lock()
	for nodeID := range nodes {
		initNodeDefaults(nodeID)
		out[nodeID] = configPorNode(nodeID)
	}
	state.Mutex.Unlock()

	json.NewEncoder(w).Encode(out)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	if err := storage.InitDB("farmnode_logs.db"); err != nil {
		log.Fatalf("Erro ao inicializar storage: %v", err)
	}
	defer storage.CloseDB()

	udpQueue = make(chan pacoteUDP, udpQueueSize)
	iniciarWorkers()

	go network.EscutarAtuadoresTCP("0.0.0.0:6000")
	go monitorarSensores()
	go monitorarAtuadores()
	go startDashboard()

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:8080")
	if err != nil {
		log.Fatalf("UDP addr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("UDP listen: %v", err)
	}
	defer conn.Close()
	if err := conn.SetReadBuffer(udpReadBufferBytes); err != nil {
		logger.Sensor.Printf("[UDP] Falha ao ajustar buffer de leitura (%d bytes): %v", udpReadBufferBytes, err)
	}

	logger.Integrador.Println("FarmNode v3 iniciado!")
	logger.Integrador.Printf("  UDP  :8080  sensores  (workers=%d fila=%d rcvbuf=%dB)", numWorkers, udpQueueSize, udpReadBufferBytes)
	logger.Integrador.Println("  TCP  :6000  atuadores (conexão única, reconexão automática)")
	logger.Integrador.Println("  HTTP :8082  dashboard + WebSocket")
	logger.Integrador.Printf("  TERMINAL_VELOCIDADE=%v", terminalVelocidade)

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
			handleSensorUDP(pacote, remoteAddr)
			if dropped%1000 == 0 {
				logger.Sensor.Printf("[AVISO] fila UDP cheia %d vezes; fallback de processamento em linha ativado", dropped)
			}
		}
	}
}
