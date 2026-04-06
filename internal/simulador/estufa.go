package simulador

import (
	"encoding/json"
	"io"
	"math"
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

// Estado local dos atuadores da estufa.
// Atualizado via GET /api/estado a cada 200ms.
// Necessario porque em Docker cada sensor roda em container separado
// e nao compartilha memoria com o servidor.
// Importante: sensores enviam apenas dados crus.
// A interpretacao e feita no servidor.
// O sensor nao registra eventos.

var (
	estufaEstadoMu   sync.RWMutex
	estufaBomba      = map[string]bool{}
	estufaVentilador = map[string]bool{}
	estufaLed        = map[string]bool{}
)

// Simula a estufa e envia dados via UDP a cada 1ms.
// O sensor apenas simula e envia os valores.
// O servidor decide os acionamentos.
func IniciarSensorEstufa(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)

	// Consulta estado dos atuadores a cada 200ms via HTTP
	// (estado e gerenciado pelo servidor, nao pelo sensor)
	go pollEstadoEstufa(httpBase, nodeID)

	enderecoServidor, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Endereco UDP invalido: %v", nodeID, sensorID, err)
	}
	conn, err := net.DialUDP("udp", nil, enderecoServidor)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Erro ao criar socket UDP: %v", nodeID, sensorID, err)
	}
	defer conn.Close()

	logger.Sensor.Printf("[%s/%s] Enviando dados UDP -> %s (1ms)", nodeID, sensorID, serverIP)

	var valorAtual float64
	switch tipo {
	case "umidade":
		valorAtual = 60.0 + rand.Float64()*15.0
	case "temperatura":
		valorAtual = 22.0 + rand.Float64()*4.0
	case "luminosidade":
		valorAtual = 400.0 + rand.Float64()*200.0
	}

	for {
		estufaEstadoMu.RLock()
		bombaLigada := estufaBomba[nodeID]
		ventiladorLigado := estufaVentilador[nodeID]
		ledLigado := estufaLed[nodeID]
		estufaEstadoMu.RUnlock()

		// Simulacao fisica do ambiente.
		// Eventos aleatorios afetam o valor.
		// Anomalias sao detectadas no servidor.
		switch tipo {

		case "umidade":
			if bombaLigada {
				valorAtual += 0.005 + rand.Float64()*0.004
			} else {
				valorAtual -= 0.0015 + rand.Float64()*0.0010
			}
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			if ventiladorLigado {
				valorAtual -= 0.0012 + rand.Float64()*0.0006
			} else {
				valorAtual += 0.0004 + rand.Float64()*0.0004
			}
			valorAtual += (rand.Float64() - 0.5) * 0.0002
			if rand.Float64() < 0.000010 {
				valorAtual += 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 10, 55)

		case "luminosidade":
			if ledLigado {
				// LED ligado: luz artificial estavel.
				alvo := 800.0
				valorAtual += (alvo - valorAtual) * 0.05
				valorAtual += (rand.Float64() - 0.5) * 8.0
			} else {
				// Ciclo dia/noite com periodo de 120s.
				// Usa seno baseado no tempo real para que todos os sensores
				// fiquem sincronizados no mesmo "horario do dia".
				// Pico (meio-dia): ~750 Lux  |  Vale (meia-noite): ~0 Lux
				// Limiar de ativacao LED: 300 Lux → LED liga por ~36s em cada ciclo.
				periodoMs := int64(120000)
				tMs := time.Now().UnixNano() / 1e6
				fase := float64(tMs%periodoMs) / float64(periodoMs) // 0.0 a 1.0
				luzSolar := 750.0 * math.Sin(math.Pi*fase)
				// Variacao suave de nuvens.
				nuvens := 20.0 * math.Sin(float64(tMs)/3000.0)
				alvo := clamp(luzSolar+nuvens, 0, 800)
				// Transicao suave para o valor alvo.
				valorAtual += (alvo - valorAtual) * 0.008
				valorAtual = clamp(valorAtual, 0, 800)
			}
		}

		// Envia dado cru via UDP.
		dadosJSON, err := json.Marshal(models.MensagemSensor{
			NodeID:        nodeID,
			SensorID:      sensorID,
			Tipo:          tipo,
			Valor:         valorAtual,
			Unidade:       unidade,
			Timestamp:     time.Now(),
			StatusLeitura: "normal",
		})
		if err == nil {
			conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			conn.Write(dadosJSON)
		}

		time.Sleep(1 * time.Millisecond)
	}
}

// Consulta o estado dos atuadores a cada 200ms.
func pollEstadoEstufa(httpBase, nodeID string) {
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		func() {
			resp, err := client.Get(httpBase + "/api/estado")
			if err != nil {
				return
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
			estufaBomba[nodeID] = toBool(node["bomba_ligada"])
			estufaVentilador[nodeID] = toBool(node["ventilador_ligado"])
			estufaLed[nodeID] = toBool(node["led_ligado"])
			estufaEstadoMu.Unlock()
		}()
		time.Sleep(200 * time.Millisecond)
	}
}

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

func deriveHTTPBase(serverIP string) string {
	host := serverIP
	if idx := strings.LastIndex(serverIP, ":"); idx >= 0 {
		host = serverIP[:idx]
	}
	return "http://" + host + ":8082"
}
