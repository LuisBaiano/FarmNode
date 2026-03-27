package simulador

import (
	"encoding/json"
	"math/rand"
	"net"
	"os"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

// IniciarSensorEstufa simula o ambiente físico da estufa e envia dados ao servidor via UDP.
// UDP é usado pois sensores apenas enviam dados — não há necessidade de conexão persistente.
// O endereço do servidor vem de SERVER_IP (padrão: localhost:8080).
func IniciarSensorEstufa(nodeID, sensorID, tipo, _ /*ipOrigem*/, unidade string) {
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
	case "umidade":
		valorAtual = 40.0
	case "temperatura":
		valorAtual = 25.0
	case "luminosidade":
		valorAtual = 500.0
	}

	for {
		// 1. Lê estados dos atuadores com segurança
		state.Mutex.Lock()
		bombaLigada      := state.BombaIrrigacao[nodeID]
		ventiladorLigado := state.Ventilador[nodeID]
		ledLigado        := state.LuzArtifical[nodeID]
		state.Mutex.Unlock()

		// 2. Simula física do ambiente
		switch tipo {
		case "umidade":
			if bombaLigada {
				valorAtual += 5.0
			} else {
				valorAtual -= 0.5
			}
			if valorAtual > 100 { valorAtual = 100 }
			if valorAtual < 0   { valorAtual = 0   }

		case "temperatura":
			if ventiladorLigado {
				valorAtual -= 1.0
			} else {
				valorAtual += 0.2
			}
			if valorAtual > 50 { valorAtual = 50 }
			if valorAtual < 10 { valorAtual = 10 }

		case "luminosidade":
			if ledLigado {
				valorAtual = 800.0
			} else {
				valorAtual = 300.0 + rand.Float64()*200.0
			}
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

		// UDP: Write simples, sem conexão persistente
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(dadosJSON); err != nil {
			logger.Sensor.Printf("[%s/%s] Erro ao enviar UDP: %v", nodeID, sensorID, err)
		}

		time.Sleep(2 * time.Second)
	}
}
