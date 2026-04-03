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

// Estado local dos atuadores da estufa.
// Atualizado via HTTP GET /api/estado a cada 200ms.
// Necessario porque em Docker cada sensor roda em container separado
// e nao compartilha memoria com o servidor.
//
// IMPORTANTE: sensores enviam APENAS dados crus (valor numerico).
// Toda interpretacao (alto, baixo, critico) e feita exclusivamente pelo SERVIDOR.
// Nenhum log de "evento" deve aparecer nos containers de sensores.

var (
	estufaEstadoMu   sync.RWMutex
	estufaBomba      = map[string]bool{}
	estufaVentilador = map[string]bool{}
	estufaLed        = map[string]bool{}
)

// IniciarSensorEstufa simula o ambiente fisico da estufa e envia dados via UDP a cada 1ms.
// O sensor NAO interpreta os valores — apenas simula a fisica e envia.
// O servidor recebe, interpreta e decide acionar ou nao os atuadores.
func IniciarSensorEstufa(nodeID, sensorID, tipo, serverAddr, unidade string) {
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = serverAddr
	}
	if serverIP == "" {
		serverIP = "localhost:8080"
	}

	httpBase := deriveHTTPBase(serverIP)

	// Consulta estado dos atuadores a cada 200ms via HTTP
	// (estado e gerenciado pelo servidor, nao pelo sensor)
	go pollEstadoEstufa(httpBase, nodeID)

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
	case "umidade":
		valorAtual = 60.0 + rand.Float64()*15.0
	case "temperatura":
		valorAtual = 22.0 + rand.Float64()*4.0
	case "luminosidade":
		valorAtual = 400.0 + rand.Float64()*200.0
	}

	for {
		estufaEstadoMu.RLock()
		bombaLigada      := estufaBomba[nodeID]
		ventiladorLigado := estufaVentilador[nodeID]
		ledLigado        := estufaLed[nodeID]
		estufaEstadoMu.RUnlock()

		// Fisica do ambiente — valores calibrados para demo de 10 minutos.
		// Eventos aleatorios afetam o valor mas NAO sao logados aqui.
		// O servidor detecta anomalias comparando o valor recebido com os limiares.
		switch tipo {

		case "umidade":
			// Sem bomba: -1.5 a -2.5%/s → limiar (15%) em ~22s a partir de 60%
			// Com bomba:  +5 a +9%/s    → recupera em ~6s
			if bombaLigada {
				valorAtual += 0.005 + rand.Float64()*0.004
			} else {
				valorAtual -= 0.0015 + rand.Float64()*0.0010
			}
			// Evento catastrofico: ruptura/drenagem severa.
			// Probabilidade: 0.00001/ms ≈ 0.01/s ≈ 9 eventos em 15min por sensor.
			// Magnitude: -12 a -20% num unico ciclo — supera a recuperacao da bomba
			// (+5-9%/s) e garante ultrapassar o limiar critico (5%).
			if rand.Float64() < 0.000010 {
				valorAtual -= 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 0, 100)

		case "temperatura":
			// Sem ventilador: +0.4 a +0.8°C/s → limiar (35°C) em ~22s a partir de 22°C
			// Com ventilador: -1.2 a -1.8°C/s  → recupera em ~7s
			if ventiladorLigado {
				valorAtual -= 0.0012 + rand.Float64()*0.0006
			} else {
				valorAtual += 0.0004 + rand.Float64()*0.0004
			}
			valorAtual += (rand.Float64() - 0.5) * 0.0002
			// Evento catastrofico: pico termico severo (ex: falha no isolamento).
			// Magnitude: +12 a +20°C — supera o resfriamento do ventilador
			// e garante ultrapassar o limiar critico (45°C).
			if rand.Float64() < 0.000010 {
				valorAtual += 12.0 + rand.Float64()*8.0
			}
			valorAtual = clamp(valorAtual, 10, 55)

		case "luminosidade":
			if ledLigado {
				valorAtual = 800.0 + (rand.Float64()-0.5)*60.0
			} else {
				valorAtual = 150.0 + rand.Float64()*450.0
			}
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

// pollEstadoEstufa consulta o estado dos atuadores no servidor a cada 200ms.
func pollEstadoEstufa(httpBase, nodeID string) {
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
			estufaEstadoMu.Lock()
			estufaBomba[nodeID]      = toBool(node["bomba_ligada"])
			estufaVentilador[nodeID] = toBool(node["ventilador_ligado"])
			estufaLed[nodeID]        = toBool(node["led_ligado"])
			estufaEstadoMu.Unlock()
		}()
		time.Sleep(200 * time.Millisecond)
	}
}

// ── Helpers compartilhados com galinheiro.go ──────────────────────────────────

func clamp(v, min, max float64) float64 {
	if v < min { return min }
	if v > max { return max }
	return v
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok { return b }
	return false
}

func deriveHTTPBase(serverIP string) string {
	host := serverIP
	if idx := strings.LastIndex(serverIP, ":"); idx >= 0 {
		host = serverIP[:idx]
	}
	return "http://" + host + ":8082"
}
