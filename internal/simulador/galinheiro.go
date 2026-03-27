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

// ── Estado local dos atuadores do galinheiro ─────────────────────────────────
// Atualizado periodicamente via HTTP GET /api/estado no servidor.

var (
	galinheiroEstadoMu   sync.RWMutex
	galinheiroExaustor   = map[string]bool{}
	galinheiroAquecedor  = map[string]bool{}
	galinheiroMotor      = map[string]bool{}
	galinheiroValvula    = map[string]bool{}
)

// IniciarSensorGalinheiro simula o ambiente físico do galinheiro e envia dados via UDP.
func IniciarSensorGalinheiro(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)

	// Goroutine que busca estado dos atuadores via HTTP a cada 2s
	go pollEstadoGalinheiro(httpBase, nodeID)

	// Socket UDP
	enderecoServidor, err := net.ResolveUDPAddr("udp", serverIP)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Endereço UDP inválido (%s): %v", nodeID, sensorID, serverIP, err)
	}
	conn, err := net.DialUDP("udp", nil, enderecoServidor)
	if err != nil {
		logger.Sensor.Fatalf("[%s/%s] Erro ao criar socket UDP: %v", nodeID, sensorID, err)
	}
	defer conn.Close()

	logger.Sensor.Printf("[%s/%s] Iniciado (UDP → %s | HTTP estado → %s)", nodeID, sensorID, serverIP, httpBase)

	// Valores iniciais realistas
	var valorAtual float64
	switch tipo {
	case "amonia":
		valorAtual = 3.0 + rand.Float64()*5.0   // começa entre 3-8 ppm (normal)
	case "temperatura":
		valorAtual = 26.0 + rand.Float64()*4.0  // começa entre 26-30°C
	case "racao":
		valorAtual = 70.0 + rand.Float64()*20.0 // começa entre 70-90%
	case "agua":
		valorAtual = 65.0 + rand.Float64()*20.0 // começa entre 65-85%
	}

	for {
		galinheiroEstadoMu.RLock()
		exaustorLigado  := galinheiroExaustor[nodeID]
		aquecedorLigado := galinheiroAquecedor[nodeID]
		motorLigado     := galinheiroMotor[nodeID]
		valvulaLigada   := galinheiroValvula[nodeID]
		galinheiroEstadoMu.RUnlock()

		// ── Física do ambiente ────────────────────────────────────────────────
		switch tipo {

		case "amonia":
			// Acúmulo natural de amônia pelas fezes
			acumulo := 0.3 + rand.Float64()*0.5 // entre +0.3 e +0.8 ppm por ciclo
			if exaustorLigado {
				// Exaustor ventilando: reduz entre -1.5 e -3.0 ppm por ciclo
				valorAtual -= 1.5 + rand.Float64()*1.5
			} else {
				valorAtual += acumulo
			}
			// Evento crítico: 1% de chance de pico súbito de amônia
			// (ex: lote de aves mais denso, calor extremo)
			if rand.Float64() < 0.01 {
				valorAtual += 8.0 + rand.Float64()*7.0
				logger.Sensor.Printf("[%s] ⚠ Evento: pico de amônia detectado!", nodeID)
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			if aquecedorLigado {
				valorAtual += 0.8 + rand.Float64()*0.4 // aquece entre +0.8 e +1.2
			} else {
				valorAtual -= 0.1 + rand.Float64()*0.3 // esfria naturalmente
			}
			// Variação natural ±0.1
			valorAtual += (rand.Float64() - 0.5) * 0.2
			valorAtual = clamp(valorAtual, 10, 45)

		case "racao":
			// Aves comem continuamente — consumo variável
			consumo := 0.4 + rand.Float64()*0.8 // entre -0.4 e -1.2 por ciclo
			if motorLigado {
				// Motor abastecendo: enche entre +6 e +12 por ciclo
				valorAtual += 6.0 + rand.Float64()*6.0
			} else {
				valorAtual -= consumo
			}
			// Evento crítico: 1.5% de chance de falha no motor (não abastece mesmo ligado)
			// Isso faz o nível cair abaixo do crítico sem que o motor consiga reagir
			if motorLigado && rand.Float64() < 0.015 {
				valorAtual -= 3.0 + rand.Float64()*4.0
				logger.Sensor.Printf("[%s] ⚠ Evento: falha no motor comedouro! Ração não reposta.", nodeID)
			}
			// Evento crítico: 0.8% de chance de consumo repentino alto
			// (ex: muitas aves com fome, briga no comedouro)
			if !motorLigado && rand.Float64() < 0.008 {
				valorAtual -= 4.0 + rand.Float64()*4.0
				logger.Sensor.Printf("[%s] ⚠ Evento: consumo acelerado de ração!", nodeID)
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "agua":
			// Consumo contínuo pelas aves
			consumo := 0.5 + rand.Float64()*1.0 // entre -0.5 e -1.5 por ciclo
			if valvulaLigada {
				// Válvula aberta: enche entre +8 e +16 por ciclo
				valorAtual += 8.0 + rand.Float64()*8.0
			} else {
				valorAtual -= consumo
			}
			// Evento crítico: 1% de chance de vazamento
			if rand.Float64() < 0.01 {
				valorAtual -= 5.0 + rand.Float64()*8.0
				logger.Sensor.Printf("[%s] ⚠ Evento: possível vazamento no bebedouro!", nodeID)
			}
			valorAtual = clamp(valorAtual, 0, 100)
		}

		// Envia datagrama UDP
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

// pollEstadoGalinheiro busca o estado dos atuadores no servidor a cada 2s via HTTP
func pollEstadoGalinheiro(httpBase, nodeID string) {
	client := &http.Client{Timeout: 3 * time.Second}
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
		time.Sleep(2 * time.Second)
	}
}
