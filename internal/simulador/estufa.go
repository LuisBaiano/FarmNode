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

// Estado local dos atuadores da estufa para simulação física.
// Atualizado via GET /api/estado a cada 200ms.
// Detecta atuadores pelo prefixo do tipo: "bomba", "ventilador", "led".
var (
	estufaEstadoMu sync.RWMutex
	// node_id -> tipo_prefixo -> ligado
	estufaEstado = map[string]map[string]bool{}
)

func estufaLigado(nodeID, prefixo string) bool {
	estufaEstadoMu.RLock()
	defer estufaEstadoMu.RUnlock()
	if m, ok := estufaEstado[nodeID]; ok {
		return m[prefixo]
	}
	return false
}

func IniciarSensorEstufa(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)
	pollEvery := envDurationMS("ATUADOR_POLL_MS", 1000, 100, 10000)
	go pollEstadoEstufa(httpBase, nodeID, pollEvery)

	enderecoServidor, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Endereço UDP inválido: %v", nodeID, sensorID, err)
	}
	conn, err := net.DialUDP("udp", nil, enderecoServidor)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Erro ao criar socket UDP: %v", nodeID, sensorID, err)
	}
	defer conn.Close()

	sendEvery := envDurationMS("SENSOR_INTERVAL_MS", 1, 1, 1000)
	logger.Sensor.Printf("[%s/%s] Enviando UDP -> %s (%s)", nodeID, sensorID, serverIP, sendEvery)

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
		bombaLigada := estufaLigado(nodeID, "bomba")
		ventiladorLigado := estufaLigado(nodeID, "ventilador")
		ledLigado := estufaLigado(nodeID, "led")

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
				alvo := 800.0
				valorAtual += (alvo - valorAtual) * 0.05
				valorAtual += (rand.Float64() - 0.5) * 8.0
			} else {
				periodoMs := int64(120000)
				tMs := time.Now().UnixNano() / 1e6
				fase := float64(tMs%periodoMs) / float64(periodoMs)
				luzSolar := 750.0 * math.Sin(math.Pi*fase)
				nuvens := 20.0 * math.Sin(float64(tMs)/3000.0)
				alvo := clamp(luzSolar+nuvens, 0, 800)
				valorAtual += (alvo - valorAtual) * 0.008
				valorAtual = clamp(valorAtual, 0, 800)
			}
		}

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

		time.Sleep(sendEvery)
	}
}

// pollEstadoEstufa consulta /api/estado e extrai o estado dos atuadores
// pelo prefixo do tipo, sem depender de IDs fixos.
func pollEstadoEstufa(httpBase, nodeID string, pollEvery time.Duration) {
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

			// estado: node_id -> { _atuadores: [{atuador_id, conectado}, ...], ... }
			var estado map[string]json.RawMessage
			if err := json.Unmarshal(body, &estado); err != nil {
				return
			}
			nodeRaw, ok := estado[nodeID]
			if !ok {
				return
			}
			var nodeData struct {
				Atuadores []struct {
					AtuadorID string `json:"atuador_id"`
					Ligado    bool   `json:"ligado"`
				} `json:"_atuadores"`
			}
			if err := json.Unmarshal(nodeRaw, &nodeData); err != nil {
				return
			}

			// Detecta estado por prefixo do atuador_id
			novo := map[string]bool{}
			for _, a := range nodeData.Atuadores {
				tipo := tipoAtuadorID(a.AtuadorID)
				if tipo == "" {
					continue
				}
				novo[tipo] = novo[tipo] || a.Ligado
			}

			// Tenta ler campos "atu_*" do nodeData para estado ON/OFF
			var nodeMap map[string]json.RawMessage
			if err := json.Unmarshal(nodeRaw, &nodeMap); err == nil {
				for k, v := range nodeMap {
					if strings.HasPrefix(k, "atu_") {
						var ligado bool
						json.Unmarshal(v, &ligado)
						// extrai prefixo do atuador_id
						atuID := strings.TrimPrefix(k, "atu_")
						tipo := tipoAtuadorID(atuID)
						if tipo != "" {
							novo[tipo] = ligado
						}
					}
				}
			}

			estufaEstadoMu.Lock()
			estufaEstado[nodeID] = novo
			estufaEstadoMu.Unlock()
		}()
		time.Sleep(pollEvery)
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

// prefixoTipo extrai o primeiro segmento do atuador_id.
// Ex: "bomba_estufa_a_1" -> "bomba"
func prefixoTipo(id string) string {
	return tipoAtuadorID(id)
}

func deriveHTTPBase(serverIP string) string {
	host := serverIP
	if idx := strings.LastIndex(serverIP, ":"); idx >= 0 {
		host = serverIP[:idx]
	}
	return "http://" + host + ":8082"
}
