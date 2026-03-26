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

// IniciarSensorGalinheiro simula o ambiente físico do galinheiro
func IniciarSensorGalinheiro(nodeID, sensorID, tipo, ipOrigem, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "127.0.0.1:8080" // Padrão para localhost
	}
	conn, err := net.Dial("tcp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("Erro na conexão TCP %s: %v", ipOrigem, err)
	}
	defer conn.Close()

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
		state.Mutex.Lock()
		exaustorLigado := state.Exaustor[nodeID]
		aquecedorLigado := state.Aquecedor[nodeID]
		motorLigado := state.MotorComedouro[nodeID]
		valvulaLigada := state.ValvulaAgua[nodeID]
		state.Mutex.Unlock()

		switch tipo {
		case "amonia":
			if exaustorLigado {
				valorAtual -= 2.0 // Vento limpando o gás tóxico rápido
			} else {
				valorAtual += 0.5 // Acúmulo de fezes gerando gás
			}
		case "temperatura":
			if aquecedorLigado {
				valorAtual += 1.0 // Campânula aquecendo os pintinhos
			} else {
				valorAtual -= 0.2 // Esfriando naturalmente
			}
		case "racao":
			if motorLigado {
				valorAtual += 10.0 // Abastecendo rápido
			} else {
				valorAtual -= 1.0 // Aves comendo
			}
		case "agua":
			if valvulaLigada {
				valorAtual += 15.0 // Enchendo bebedouro rápido
			} else {
				valorAtual -= 1.5 // Aves bebendo
			}
		}

		// Limites lógicos de porcentagem e valores mínimos
		if valorAtual < 0 {
			valorAtual = 0
		}
		if valorAtual > 100 && (tipo == "racao" || tipo == "agua") {
			valorAtual = 100
		}

		dados := models.MensagemSensor{
			NodeID:        nodeID,
			SensorID:      sensorID,
			Tipo:          tipo,
			Valor:         valorAtual,
			Unidade:       unidade,
			Timestamp:     time.Now(),
			StatusLeitura: "normal",
		}

		dadosJSON, _ := json.Marshal(dados)
		conn.Write(append(dadosJSON, '\n'))

		time.Sleep(2 * time.Second)
	}
}
