package simulador

import (
	"encoding/json"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
)

// Estado local dos atuadores do galinheiro.
// Atualizado via GET /api/estado a cada 200ms.
// Importante: sensores enviam apenas dados crus.
// A interpretacao e feita no servidor.

var (
	galinheiroEstadoMu  sync.RWMutex
	galinheiroExaustor  = map[string]bool{}
	galinheiroAquecedor = map[string]bool{}
	galinheiroMotor     = map[string]bool{}
	galinheiroValvula   = map[string]bool{}
)

// Simula o galinheiro e envia dados via UDP a cada 1ms.
// O sensor apenas simula e envia os valores.
func IniciarSensorGalinheiro(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)

	go pollEstadoGalinheiro(httpBase, nodeID)

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
		galinheiroEstadoMu.RLock()
		exaustorLigado := galinheiroExaustor[nodeID]
		aquecedorLigado := galinheiroAquecedor[nodeID]
		motorLigado := galinheiroMotor[nodeID]
		valvulaLigada := galinheiroValvula[nodeID]
		galinheiroEstadoMu.RUnlock()

		// Simulacao fisica do ambiente.
		// Eventos aleatorios afetam o valor.
		// Anomalias sao detectadas no servidor.
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
func pollEstadoGalinheiro(httpBase, nodeID string) {
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
			galinheiroEstadoMu.Lock()
			galinheiroExaustor[nodeID] = toBool(node["exaustor_ligado"])
			galinheiroAquecedor[nodeID] = toBool(node["aquecedor_ligado"])
			galinheiroMotor[nodeID] = toBool(node["motor_ligado"])
			galinheiroValvula[nodeID] = toBool(node["valvula_ligada"])
			galinheiroEstadoMu.Unlock()
		}()
		time.Sleep(200 * time.Millisecond)
	}
}
