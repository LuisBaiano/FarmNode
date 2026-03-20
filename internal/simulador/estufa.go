package simulador

import (
	"encoding/json"
	"math/rand"
	"net"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

// IniciarSensorEstufa simula o ambiente físico da estufa
func IniciarSensorEstufa(nodeID, sensorID, tipo, ipOrigem, unidade string) {
	localAddr, _ := net.ResolveUDPAddr("udp4", ipOrigem)
	serverAddr, _ := net.ResolveUDPAddr("udp4", "127.0.0.1:8080")
	conn, err := net.DialUDP("udp4", localAddr, serverAddr)
	if err != nil {
		logger.Sensor.Fatalf("Erro na conexão UDP %s: %v", ipOrigem, err)
	}
	defer conn.Close()

	// Valores iniciais baseados no tipo do sensor
	var valorAtual float64
	switch tipo {
	case "umidade":
		valorAtual = 40.0
	case "temperatura":
		valorAtual = 25.0
	case "luminosidade":
		valorAtual = 500.0 // Medido em Lux
	}

	for {
		// 1. Lê os estados dos atuadores com segurança (Mutex)
		state.Mutex.Lock()
		bombaLigada := state.BombaIrrigacao[nodeID]
		ventiladorLigado := state.Ventilador[nodeID]
		ledLigado := state.LuzArtifical[nodeID]
		state.Mutex.Unlock()

		// 2. Aplica a física do ambiente (se o atuador ligar, o valor reage)
		switch tipo {
		case "umidade":
			if bombaLigada {
				valorAtual += 5.0 // Terra molhando rápido
			} else {
				valorAtual -= 0.5 // Secando devagar
			}
		case "temperatura":
			if ventiladorLigado {
				valorAtual -= 1.0 // Esfriando
			} else {
				valorAtual += 0.2 // Efeito estufa esquentando devagar
			}
		case "luminosidade":
			if ledLigado {
				valorAtual = 800.0 // Luz artificial no máximo
			} else {
				// Flutuação natural do sol/nuvens
				valorAtual = 300.0 + rand.Float64()*200.0
			}
		}

		// Garante que a umidade não passe de 100% nem fique negativa
		if valorAtual > 100 && tipo == "umidade" {
			valorAtual = 100
		}
		if valorAtual < 0 {
			valorAtual = 0
		}

		// 3. Monta e envia o pacote UDP
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
		conn.Write(dadosJSON)

		time.Sleep(2 * time.Second)
	}
}
