package simulador

import (
	"encoding/json"
	"net"
	"os"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

// IniciarSensorGalinheiro simula o ambiente físico do galinheiro e envia dados ao servidor via UDP.
// UDP é usado pois sensores apenas enviam dados — não há necessidade de conexão persistente.
// O endereço do servidor vem de SERVER_IP (padrão: localhost:8080).
func IniciarSensorGalinheiro(nodeID, sensorID, tipo, _ /*ipOrigem*/, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	// Resolve endereço UDP do servidor
	enderecoServidor, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Endereço UDP inválido (%s): %v", nodeID, sensorID, serverIP, err)
	}

	// Cria socket UDP local (porta 0 = SO escolhe automaticamente)
	conn, err := net.DialUDP("udp", nil, enderecoServidor)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Erro ao criar socket UDP: %v", nodeID, sensorID, err)
	}
	defer conn.Close()

	logger.Sensor.Printf("[%s/%s] Enviando dados UDP → %s", nodeID, sensorID, serverIP)

	var valorAtual float64
	switch tipo {
	case "amonia":
		valorAtual = 5.0
	case "temperatura":
		valorAtual = 28.0
	case "racao":
		valorAtual = 80.0
	case "agua":
		valorAtual = 80.0
	}

	for {
		// 1. Lê estados dos atuadores com segurança
		state.Mutex.Lock()
		exaustorLigado  := state.Exaustor[nodeID]
		aquecedorLigado := state.Aquecedor[nodeID]
		motorLigado     := state.MotorComedouro[nodeID]
		valvulaLigada   := state.ValvulaAgua[nodeID]
		state.Mutex.Unlock()

		// 2. Simula física do ambiente
		switch tipo {
		case "amonia":
			if exaustorLigado {
				valorAtual -= 2.0
			} else {
				valorAtual += 0.5
			}
			if valorAtual > 100 { valorAtual = 100 }
			if valorAtual < 0   { valorAtual = 0   }

		case "temperatura":
			if aquecedorLigado {
				valorAtual += 1.0
			} else {
				valorAtual -= 0.2
			}
			if valorAtual > 45 { valorAtual = 45 }
			if valorAtual < 10 { valorAtual = 10 }

		case "racao":
			if motorLigado {
				valorAtual += 10.0
			} else {
				valorAtual -= 1.0
			}
			if valorAtual > 100 { valorAtual = 100 }
			if valorAtual < 0   { valorAtual = 0   }

		case "agua":
			if valvulaLigada {
				valorAtual += 15.0
			} else {
				valorAtual -= 1.5
			}
			if valorAtual > 100 { valorAtual = 100 }
			if valorAtual < 0   { valorAtual = 0   }
		}

		// 3. Serializa e envia pacote UDP
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
