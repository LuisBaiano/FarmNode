package simulador

import (
	"encoding/json"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
)

// ── Estado local dos atuadores da estufa ─────────────────────────────────────
// Atualizado periodicamente via HTTP GET /api/estado no servidor.
// Necessário porque em Docker cada sensor roda em container separado
// e não compartilha memória com o processo dos atuadores.

var (
	estufaEstadoMu   sync.RWMutex
	estufaBomba      = map[string]bool{}
	estufaVentilador = map[string]bool{}
	estufaLed        = map[string]bool{}
)

// IniciarSensorEstufa simula o ambiente físico da estufa e envia dados via UDP.
// Parâmetro serverAddr: endereço UDP do servidor (ex: "192.168.101.7:8080").
// O endereço HTTP do dashboard é derivado automaticamente (porta 8082).
func IniciarSensorEstufa(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	// Deriva host HTTP a partir do SERVER_IP (troca porta por 8082)
	httpBase := deriveHTTPBase(serverIP)

	// Goroutine que busca estado dos atuadores via HTTP a cada 2s
	go pollEstadoEstufa(httpBase, nodeID)

	// Socket UDP para envio de dados
	enderecoServidor, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Endereço UDP inválido (%s): %v", nodeID, sensorID, serverIP, err)
	}
	conn, err := net.DialUDP("udp", nil, enderecoServidor)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Erro ao criar socket UDP: %v", nodeID, sensorID, err)
	}
	defer conn.Close()

	logger.Sensor.Printf("[%s/%s] Iniciado (UDP → %s | HTTP estado → %s)", nodeID, sensorID, serverIP, httpBase)

	// Valores iniciais realistas
	var valorAtual float64
	switch tipo {
	case "umidade":
		valorAtual = 60.0 + rand.Float64()*20.0 // começa entre 60-80%
	case "temperatura":
		valorAtual = 22.0 + rand.Float64()*4.0  // começa entre 22-26°C
	case "luminosidade":
		valorAtual = 400.0 + rand.Float64()*200.0
	}

	for {
		// Lê estado atual dos atuadores (atualizado pela goroutine HTTP)
		estufaEstadoMu.RLock()
		bombaLigada      := estufaBomba[nodeID]
		ventiladorLigado := estufaVentilador[nodeID]
		ledLigado        := estufaLed[nodeID]
		estufaEstadoMu.RUnlock()

		// ── Física do ambiente ────────────────────────────────────────────────
		switch tipo {

		case "umidade":
			// Evaporação natural com variação aleatória (simula clima)
			decaimento := 0.3 + rand.Float64()*0.8 // entre -0.3 e -1.1 por ciclo
			if bombaLigada {
				// Bomba funcionando: sobe entre +3 e +7 por ciclo
				valorAtual += 3.0 + rand.Float64()*4.0
			} else {
				valorAtual -= decaimento
			}
			// Evento crítico simulado: 1% de chance de vazar mais rápido
			// (ex: rachadura no substrato, dia muito quente)
			if rand.Float64() < 0.01 {
				valorAtual -= 3.0 + rand.Float64()*4.0
				logger.Sensor.Printf("[%s] ⚠ Evento: drenagem rápida detectada!", nodeID)
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			if ventiladorLigado {
				valorAtual -= 0.8 + rand.Float64()*0.4 // resfria entre -0.8 e -1.2
			} else {
				// Estufa esquenta naturalmente com o sol
				valorAtual += 0.1 + rand.Float64()*0.3
			}
			// Variação natural ±0.1 (rajadas, nuvens)
			valorAtual += (rand.Float64() - 0.5) * 0.2
			// Evento crítico: 0.5% de chance de pico térmico
			if rand.Float64() < 0.005 {
				valorAtual += 4.0 + rand.Float64()*3.0
				logger.Sensor.Printf("[%s] ⚠ Evento: pico térmico detectado!", nodeID)
			}
			valorAtual = clamp(valorAtual, 10, 55)

		case "luminosidade":
			if ledLigado {
				// LED ligado: valor estável em torno de 800 Lux ±50
				valorAtual = 800.0 + (rand.Float64()-0.5)*100.0
			} else {
				// Luz natural: flutua entre 200 e 700 Lux (simula nuvens)
				valorAtual = 200.0 + rand.Float64()*500.0
			}
		}

		// Envia datagrama UDP
		dados := models.MensagemSensor{
			NodeID:        nodeID,
			SensorID:      sensorID,
			Tipo:          tipo,
			Valor:         valorAtual,
			Unidade:       unidade,
			Timestamp:     time.Now(),
			StatusLeitura: "normal",
		}

		dadosJSON, err := json.Marshal(dados)
		if err != nil {
			logger.Sensor.Printf("[%s/%s] Erro ao serializar: %v", nodeID, sensorID, err)
			time.Sleep(2 * time.Second)
			continue
		}

		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(dadosJSON); err != nil {
			logger.Sensor.Printf("[%s/%s] Erro ao enviar UDP: %v", nodeID, sensorID, err)
		}

		time.Sleep(2 * time.Second)
	}
}

// pollEstadoEstufa busca o estado dos atuadores no servidor a cada 2s via HTTP
func pollEstadoEstufa(httpBase, nodeID string) {
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		func() {
			resp, err := client.Get(httpBase + "/api/estado")
			if err != nil {
				return // servidor ainda não disponível, tenta na próxima
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			var estado map[string]map[string]interface{}
			if err := json.Unmarshal(body, &estado); err != nil {
				return
			}

			node, ok := estado[nodeID]
			if !ok {
				return
			}

			estufaEstadoMu.Lock()
			estufaBomba[nodeID]      = toBool(node["bomba_ligada"])
			estufaVentilador[nodeID] = toBool(node["ventilador_ligado"])
			estufaLed[nodeID]        = toBool(node["led_ligado"])
			estufaEstadoMu.Unlock()
		}()
		time.Sleep(2 * time.Second)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// deriveHTTPBase extrai o host de "host:porta_udp" e monta "http://host:8082"
func deriveHTTPBase(serverIP string) string {
	host := serverIP
	if idx := strings.LastIndex(serverIP, ":"); idx >= 0 {
		host = serverIP[:idx]
	}
	return "http://" + host + ":8082"
}
