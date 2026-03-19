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

// Rodar simula o comportamento físico de uma estufa
func Rodar(estufaID string, conexaoUDP *net.UDPConn) {
	umidadeAtual := 20.0 + rand.Float64()*10.0

	for {
		// Leitura segura do estado da bomba
		state.Mutex.Lock()
		bombaEstaLigada := state.Ligadas[estufaID]
		state.Mutex.Unlock()

		if bombaEstaLigada {
			umidadeAtual += rand.Float64() * 6.0
			logger.Sensor.Printf("%s Irrigando... Umidade: %.1f%%", estufaID, umidadeAtual)
		} else {
			umidadeAtual -= rand.Float64() * 1.5
		}

		dados := models.MensagemSensor{
			EstufaID:      estufaID,
			SensorID:      "sensor_umidade_01",
			Tipo:          "umidade",
			Valor:         umidadeAtual,
			Unidade:       "%",
			Timestamp:     time.Now(),
			StatusLeitura: "normal",
		}

		dadosJSON, _ := json.Marshal(dados)
		conexaoUDP.Write(dadosJSON)

		time.Sleep(2 * time.Second)
	}
}
