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
// Atualizado via HTTP GET /api/estado a cada 200ms.
//
// IMPORTANTE: sensores enviam APENAS dados crus.
// Toda interpretacao e feita exclusivamente pelo SERVIDOR.

var (
	galinheiroEstadoMu  sync.RWMutex
	galinheiroExaustor  = map[string]bool{}
	galinheiroAquecedor = map[string]bool{}
	galinheiroMotor     = map[string]bool{}
	galinheiroValvula   = map[string]bool{}
)

// IniciarSensorGalinheiro simula o ambiente fisico do galinheiro e envia dados via UDP a cada 1ms.
// O sensor NAO interpreta os valores — apenas simula a fisica e envia dados crus.
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
		exaustorLigado  := galinheiroExaustor[nodeID]
		aquecedorLigado := galinheiroAquecedor[nodeID]
		motorLigado     := galinheiroMotor[nodeID]
		valvulaLigada   := galinheiroValvula[nodeID]
		galinheiroEstadoMu.RUnlock()

		// Fisica do ambiente — valores calibrados para demo de 10 minutos.
		// Eventos aleatorios afetam o valor mas NAO sao logados aqui.
		// O servidor detecta anomalias comparando o valor com os limiares.
		switch tipo {

		case "amonia":
			// Sem exaustor: +0.6 a +1.0 ppm/s → limiar (20ppm) em ~21s a partir de 3ppm
			// Com exaustor: -1.5 a -2.5 ppm/s  → recupera em ~8s
			if exaustorLigado {
				valorAtual -= 0.0015 + rand.Float64()*0.0010
			} else {
				valorAtual += 0.0006 + rand.Float64()*0.0004
			}
			// Evento catastrofico: pico maximo de amonia (ex: morte de aves, calor extremo).
			// Magnitude: +18 a +30 ppm — supera a purga do exaustor (-1.5-2.5 ppm/s)
			// e garante ultrapassar o limiar critico (35 ppm).
			if rand.Float64() < 0.000010 {
				valorAtual += 18.0 + rand.Float64()*12.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			// Sem aquecedor: -0.1 a -0.3°C/s → limiar (24°C) em ~25s a partir de 29°C
			// Com aquecedor: +0.4 a +0.8°C/s  → recupera em ~8s
			if aquecedorLigado {
				valorAtual += 0.0004 + rand.Float64()*0.0004
			} else {
				valorAtual -= 0.0001 + rand.Float64()*0.0002
			}
			valorAtual += (rand.Float64() - 0.5) * 0.0001
			// Evento catastrofico: queda brusca de temperatura (ex: porta aberta, falha eletrica).
			// Magnitude: -12 a -18°C — supera o aquecimento (+0.4-0.8°C/s)
			// e garante ultrapassar o limiar critico (15°C) a partir da temperatura normal (24-29°C).
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*6.0
			}
			valorAtual = clamp(valorAtual, 10, 45)

		case "racao":
			// Sem motor: -1.0 a -2.0%/s → limiar (10%) em ~37s a partir de 65%
			// Com motor: +6 a +10%/s    → recupera em ~10s
			if motorLigado {
				valorAtual += 0.0060 + rand.Float64()*0.0040
			} else {
				valorAtual -= 0.0010 + rand.Float64()*0.0010
			}
			// Evento catastrofico: falha grave no comedouro / consumo explosivo.
			// Magnitude: -8 a -15% — supera o abastecimento do motor (+6-10%/s)
			// e garante ultrapassar o limiar critico (5%) quando proximo do limiar de ativacao (10%).
			if rand.Float64() < 0.000010 {
				valorAtual -= 8.0 + rand.Float64()*7.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "agua":
			// Sem valvula: -1.0 a -2.0%/s → limiar (15%) em ~30s a partir de 60%
			// Com valvula: +8 a +12%/s    → recupera em ~6s
			if valvulaLigada {
				valorAtual += 0.0080 + rand.Float64()*0.0040
			} else {
				valorAtual -= 0.0010 + rand.Float64()*0.0010
			}
			// Evento catastrofico: rompimento do bebedouro / vazamento grave.
			// Magnitude: -12 a -20% — supera o enchimento da valvula (+8-12%/s)
			// e garante ultrapassar o limiar critico (5%).
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 0, 100)
		}

		// Envia dado cru via UDP — sem interpretacao, sem log de evento
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

// pollEstadoGalinheiro consulta o estado dos atuadores a cada 200ms.
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
			galinheiroExaustor[nodeID]  = toBool(node["exaustor_ligado"])
			galinheiroAquecedor[nodeID] = toBool(node["aquecedor_ligado"])
			galinheiroMotor[nodeID]     = toBool(node["motor_ligado"])
			galinheiroValvula[nodeID]   = toBool(node["valvula_ligada"])
			galinheiroEstadoMu.Unlock()
		}()
		time.Sleep(200 * time.Millisecond)
	}
}
