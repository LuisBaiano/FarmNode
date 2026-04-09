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

// Estado local dos atuadores do galinheiro para simulação física.
// Detecta atuadores pelo prefixo: "exaustor", "aquecedor", "motor", "valvula".
var (
	galinheiroEstadoMu sync.RWMutex
	// node_id -> tipo_prefixo -> ligado
	galinheiroEstado = map[string]map[string]bool{}
)

func galinheiroLigado(nodeID, prefixo string) bool {
	galinheiroEstadoMu.RLock()
	defer galinheiroEstadoMu.RUnlock()
	if m, ok := galinheiroEstado[nodeID]; ok {
		return m[prefixo]
	}
	return false
}

func IniciarSensorGalinheiro(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)
	pollEvery := envDurationMS("ATUADOR_POLL_MS", 1000, 100, 10000)
	go pollEstadoGalinheiro(httpBase, nodeID, pollEvery)

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
	case "amonia":
		valorAtual = 3.0 + rand.Float64()*4.0
	case "temperatura":
		valorAtual = 26.0 + rand.Float64()*3.0
	case "racao":
		valorAtual = 65.0 + rand.Float64()*20.0
	case "agua":
		valorAtual = 60.0 + rand.Float64()*20.0
	}

	for {
		exaustorLigado := galinheiroLigado(nodeID, "exaustor")
		aquecedorLigado := galinheiroLigado(nodeID, "aquecedor")
		motorLigado := galinheiroLigado(nodeID, "motor")
		valvulaLigada := galinheiroLigado(nodeID, "valvula")

		switch tipo {
		case "amonia":
			if exaustorLigado {
				valorAtual -= 0.0015 + rand.Float64()*0.0010
			} else {
				valorAtual += 0.0006 + rand.Float64()*0.0004
			}
			if rand.Float64() < 0.000010 {
				valorAtual += 18.0 + rand.Float64()*12.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			if aquecedorLigado {
				valorAtual += 0.0004 + rand.Float64()*0.0004
			} else {
				valorAtual -= 0.0001 + rand.Float64()*0.0002
			}
			valorAtual += (rand.Float64() - 0.5) * 0.0001
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*6.0
			}
			valorAtual = clamp(valorAtual, 10, 45)

		case "racao":
			if motorLigado {
				valorAtual += 0.0060 + rand.Float64()*0.0040
			} else {
				valorAtual -= 0.0010 + rand.Float64()*0.0010
			}
			if rand.Float64() < 0.000010 {
				valorAtual -= 8.0 + rand.Float64()*7.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "agua":
			if valvulaLigada {
				valorAtual += 0.0080 + rand.Float64()*0.0040
			} else {
				valorAtual -= 0.0010 + rand.Float64()*0.0010
			}
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 0, 100)
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

// pollEstadoGalinheiro — mesmo padrão do estufa.go, sem IDs fixos.
func pollEstadoGalinheiro(httpBase, nodeID string, pollEvery time.Duration) {
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

			var estado map[string]json.RawMessage
			if err := json.Unmarshal(body, &estado); err != nil {
				return
			}
			nodeRaw, ok := estado[nodeID]
			if !ok {
				return
			}

			novo := map[string]bool{}

			// Lê campos "atu_<atuador_id>" que o servidor expõe
			var nodeData struct {
				Atuadores []struct {
					AtuadorID string `json:"atuador_id"`
					Ligado    bool   `json:"ligado"`
				} `json:"_atuadores"`
			}
			if err := json.Unmarshal(nodeRaw, &nodeData); err == nil {
				for _, a := range nodeData.Atuadores {
					tipo := tipoAtuadorID(a.AtuadorID)
					if tipo != "" {
						novo[tipo] = novo[tipo] || a.Ligado
					}
				}
			}

			var nodeMap map[string]json.RawMessage
			if err := json.Unmarshal(nodeRaw, &nodeMap); err == nil {
				for k, v := range nodeMap {
					if strings.HasPrefix(k, "atu_") {
						var ligado bool
						json.Unmarshal(v, &ligado)
						atuID := strings.TrimPrefix(k, "atu_")
						tipo := tipoAtuadorID(atuID)
						if tipo != "" {
							novo[tipo] = ligado
						}
					}
				}
			}

			galinheiroEstadoMu.Lock()
			galinheiroEstado[nodeID] = novo
			galinheiroEstadoMu.Unlock()
		}()
		time.Sleep(pollEvery)
	}
}
