package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/network"
	"FarmNode/internal/state"
	"FarmNode/internal/storage"
)

// ── Throttle de alertas ───────────────────────────────────────────────────────
// Evita inundar logs/dashboard com o mesmo alerta a cada 2 segundos.
// Um alerta da mesma condição só é regenerado após 30 segundos.

var (
	ultimoAlerta   = make(map[string]time.Time)
	ultimoAlertaMu sync.Mutex
)

func gerarAlerta(nodeID, tipo string, valor float64, mensagem, nivel string) {
	ultimoAlertaMu.Lock()
	key := nodeID + "|" + tipo + "|" + nivel
	ultima := ultimoAlerta[key]
	ultimoAlertaMu.Unlock()

	if time.Since(ultima) < 30*time.Second {
		return // throttle: máximo 1 alerta por condição a cada 30s
	}

	ultimoAlertaMu.Lock()
	ultimoAlerta[key] = time.Now()
	ultimoAlertaMu.Unlock()

	logger.Integrador.Printf("[ALERTA/%s] %s/%s: %s (valor=%.2f)", nivel, nodeID, tipo, mensagem, valor)
	storage.LogAlerta(nodeID, tipo, valor, mensagem, nivel)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	if err := storage.InitDB("farmnode_logs.db"); err != nil {
		log.Fatalf("Erro ao inicializar storage: %v", err)
	}
	defer storage.CloseDB()

	go startDashboard()

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:8080")
	if err != nil {
		log.Fatalf("Erro ao resolver endereço UDP: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor UDP na :8080: %v", err)
	}
	defer conn.Close()

	logger.Integrador.Println("✓ FarmNode v3 iniciado!")
	logger.Integrador.Println("  → Sensores : UDP  :8080")
	logger.Integrador.Println("  → Atuadores: TCP  :9090 (via ATUADOR_HOST)")
	logger.Integrador.Println("  → Dashboard: HTTP :8082/dashboard")

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			logger.Sensor.Printf("Erro ao receber datagrama UDP: %v", err)
			continue
		}
		pacote := make([]byte, n)
		copy(pacote, buf[:n])
		go handleSensorUDP(pacote, remoteAddr)
	}
}

// handleSensorUDP processa um datagrama recebido de sensor
func handleSensorUDP(dados []byte, origem *net.UDPAddr) {
	var sensor models.MensagemSensor
	if err := json.Unmarshal(dados, &sensor); err != nil {
		logger.Sensor.Printf("Erro ao decodificar datagrama de %s: %v", origem, err)
		return
	}

	state.Mutex.Lock()
	if _, ok := state.ValoresSensores[sensor.NodeID]; !ok {
		state.ValoresSensores[sensor.NodeID] = make(map[string]float64)
	}
	state.ValoresSensores[sensor.NodeID][sensor.Tipo] = sensor.Valor
	state.Mutex.Unlock()

	logger.Sensor.Printf("[UDP] [%s/%s] %s = %.2f %s",
		sensor.NodeID, sensor.SensorID, sensor.Tipo, sensor.Valor, sensor.Unidade)

	if err := storage.LogSensor(sensor); err != nil {
		logger.Sensor.Printf("Erro ao registrar log: %v", err)
	}

	processarRegrasAutomaticas(sensor)
}

// ── Regras automáticas ────────────────────────────────────────────────────────

// processarRegrasAutomaticas avalia cada leitura de sensor e:
//  1. Aciona/desaciona atuadores conforme os limiares configurados
//  2. Gera alertas de aviso ou crítico quando os limiares são ultrapassados
//
// Os alertas críticos NÃO alteram a lógica de ativação automática —
// eles apenas notificam o operador de uma situação anormal.
func processarRegrasAutomaticas(sensor models.MensagemSensor) {
	// Coleta pendências de alerta para disparar FORA do lock (evita deadlock)
	type alertaPendente struct {
		nodeID, tipo, mensagem, nivel string
		valor                         float64
	}
	var alertas []alertaPendente

	state.Mutex.Lock()

	// ── Estufa ────────────────────────────────────────────────────────────────
	if strings.HasPrefix(sensor.NodeID, "Estufa") {
		switch sensor.Tipo {

		case "umidade":
			min := state.AlvoUmidadeMinima[sensor.NodeID]
			max := state.AlvoUmidadeMaxima[sensor.NodeID]
			crit := state.LimiteCriticoUmidade[sensor.NodeID]
			ligado := state.BombaIrrigacao[sensor.NodeID]

			if sensor.Valor < min && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.1f%% < %.1f%% → LIGANDO BOMBA",
					sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_baixa")
				state.BombaIrrigacao[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_baixa")
			} else if sensor.Valor > max && ligado {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.1f%% > %.1f%% → DESLIGANDO BOMBA",
					sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
				state.BombaIrrigacao[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
			}

			if sensor.Valor < crit {
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "umidade",
					fmt.Sprintf("Umidade CRÍTICA: %.1f%% — bomba deveria ter agido em %.1f%%!", sensor.Valor, min),
					"critico", sensor.Valor,
				})
			}

		case "temperatura":
			max := state.AlvoTempMaxima[sensor.NodeID]
			crit := state.LimiteCriticoTempEstufa[sensor.NodeID]
			ligado := state.Ventilador[sensor.NodeID]

			if sensor.Valor > max && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Temp %.1f°C > %.1f°C → LIGANDO VENTILADOR",
					sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "ventilador_01", "LIGAR", "temp_alta")
				state.Ventilador[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "ventilador_01", "LIGAR", "temp_alta")
			} else if sensor.Valor < max-5.0 && ligado {
				logger.Integrador.Printf("[AUTO] %s: Temp %.1f°C normalizada → DESLIGANDO VENTILADOR",
					sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "ventilador_01", "DESLIGAR", "temp_normal")
				state.Ventilador[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "ventilador_01", "DESLIGAR", "temp_normal")
			}

			if sensor.Valor > crit {
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "temperatura",
					fmt.Sprintf("Temperatura CRÍTICA: %.1f°C — ventilador deveria ter agido em %.1f°C!", sensor.Valor, max),
					"critico", sensor.Valor,
				})
			}
		}
	}

	// ── Galinheiro ────────────────────────────────────────────────────────────
	if strings.HasPrefix(sensor.NodeID, "Galinheiro") {
		switch sensor.Tipo {

		case "amonia":
			max := state.AlvoAmoniaMaxima[sensor.NodeID]
			crit := state.LimiteCriticoAmonia[sensor.NodeID]
			ligado := state.Exaustor[sensor.NodeID]

			if sensor.Valor >= max && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Amônia %.1f ppm >= %.1f ppm → LIGANDO EXAUSTOR",
					sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "exaustor_teto_01", "LIGAR", "amonia_elevada")
				state.Exaustor[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "exaustor_teto_01", "LIGAR", "amonia_elevada")
				// Aviso ao acionar (informa que o sistema agiu)
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "amonia",
					fmt.Sprintf("Amônia elevada: %.1f ppm — Exaustor acionado automaticamente", sensor.Valor),
					"aviso", sensor.Valor,
				})
			} else if sensor.Valor < max-10.0 && ligado {
				logger.Integrador.Printf("[AUTO] %s: Amônia %.1f ppm normalizada → DESLIGANDO EXAUSTOR",
					sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "exaustor_teto_01", "DESLIGAR", "amonia_normal")
				state.Exaustor[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "exaustor_teto_01", "DESLIGAR", "amonia_normal")
			}

			if sensor.Valor >= crit {
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "amonia",
					fmt.Sprintf("Amônia CRÍTICA: %.1f ppm — exaustor ligado mas nível ainda alto!", sensor.Valor),
					"critico", sensor.Valor,
				})
			}

		case "temperatura":
			min := state.AlvoTempMinima[sensor.NodeID]
			ligado := state.Aquecedor[sensor.NodeID]

			if sensor.Valor < min && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Temp %.1f°C < %.1f°C → LIGANDO AQUECEDOR",
					sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "aquecedor_01", "LIGAR", "temp_baixa")
				state.Aquecedor[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "aquecedor_01", "LIGAR", "temp_baixa")
			} else if sensor.Valor > min+5.0 && ligado {
				logger.Integrador.Printf("[AUTO] %s: Temp %.1f°C normalizada → DESLIGANDO AQUECEDOR",
					sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "aquecedor_01", "DESLIGAR", "temp_normal")
				state.Aquecedor[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "aquecedor_01", "DESLIGAR", "temp_normal")
			}

		case "racao":
			min := state.AlvoRacaoMinima[sensor.NodeID]
			max := state.AlvoRacaoMaxima[sensor.NodeID]
			crit := state.LimiteCriticoRacao[sensor.NodeID]
			ligado := state.MotorComedouro[sensor.NodeID]

			if sensor.Valor < min && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Ração %.1f%% < %.1f%% → LIGANDO MOTOR",
					sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
				state.MotorComedouro[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
			} else if sensor.Valor >= max && ligado {
				logger.Integrador.Printf("[AUTO] %s: Ração %.1f%% >= %.1f%% → DESLIGANDO MOTOR",
					sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
				state.MotorComedouro[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
			}

			if sensor.Valor < crit {
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "racao",
					fmt.Sprintf("Ração CRÍTICA: %.1f%% — motor deveria ter agido em %.1f%%! Verificar equipamento.", sensor.Valor, min),
					"critico", sensor.Valor,
				})
			}

		case "agua":
			min := state.AlvoAguaMinima[sensor.NodeID]
			max := state.AlvoAguaMaxima[sensor.NodeID]
			crit := state.LimiteCriticoAgua[sensor.NodeID]
			ligado := state.ValvulaAgua[sensor.NodeID]

			if sensor.Valor < min && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Água %.1f%% < %.1f%% → LIGANDO VÁLVULA",
					sensor.NodeID, sensor.Valor, min)
				network.EnviarComandoTCP(sensor.NodeID, "valvula_agua_01", "LIGAR", "agua_baixa")
				state.ValvulaAgua[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "valvula_agua_01", "LIGAR", "agua_baixa")
			} else if sensor.Valor >= max && ligado {
				logger.Integrador.Printf("[AUTO] %s: Água %.1f%% >= %.1f%% → DESLIGANDO VÁLVULA",
					sensor.NodeID, sensor.Valor, max)
				network.EnviarComandoTCP(sensor.NodeID, "valvula_agua_01", "DESLIGAR", "agua_cheia")
				state.ValvulaAgua[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "valvula_agua_01", "DESLIGAR", "agua_cheia")
			}

			if sensor.Valor < crit {
				alertas = append(alertas, alertaPendente{
					sensor.NodeID, "agua",
					fmt.Sprintf("Água CRÍTICA: %.1f%% — válvula deveria ter agido em %.1f%%! Verificar equipamento.", sensor.Valor, min),
					"critico", sensor.Valor,
				})
			}
		}
	}

	state.Mutex.Unlock()

	// Dispara alertas fora do lock
	for _, a := range alertas {
		gerarAlerta(a.nodeID, a.tipo, a.valor, a.mensagem, a.nivel)
	}
}

// ── Dashboard HTTP ────────────────────────────────────────────────────────────

func startDashboard() {
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, getDashboardHTML())
	})

	http.HandleFunc("/api/estado", handleAPIEstado)
	http.HandleFunc("/api/comando", handleAPIComando)
	http.HandleFunc("/api/sensor/", handleAPISensorData)
	http.HandleFunc("/api/atuador/history", handleAPIAtuadorHistory)
	http.HandleFunc("/api/alertas", handleAPIAlertas)
	http.HandleFunc("/api/alertas/ack", handleAPIAlertaAck)
	http.HandleFunc("/api/config", handleAPIConfig)

	logger.Integrador.Println("✓ Dashboard disponível em http://0.0.0.0:8082/dashboard")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatalf("Erro no servidor HTTP: %v", err)
	}
}

func handleAPIEstado(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	sensEA := state.ValoresSensores["Estufa_A"]
	sensGA := state.ValoresSensores["Galinheiro_A"]

	json.NewEncoder(w).Encode(map[string]interface{}{
		"Estufa_A": map[string]interface{}{
			"umidade": sensEA["umidade"], "temperatura": sensEA["temperatura"],
			"luminosidade":      sensEA["luminosidade"],
			"bomba_ligada":      state.BombaIrrigacao["Estufa_A"],
			"ventilador_ligado": state.Ventilador["Estufa_A"],
			"led_ligado":        state.LuzArtifical["Estufa_A"],
		},
		"Galinheiro_A": map[string]interface{}{
			"amonia": sensGA["amonia"], "temperatura": sensGA["temperatura"],
			"racao": sensGA["racao"], "agua": sensGA["agua"],
			"exaustor_ligado":  state.Exaustor["Galinheiro_A"],
			"aquecedor_ligado": state.Aquecedor["Galinheiro_A"],
			"motor_ligado":     state.MotorComedouro["Galinheiro_A"],
			"valvula_ligada":   state.ValvulaAgua["Galinheiro_A"],
		},
	})
}

type ComandoDashboard struct {
	NodeID    string `json:"node_id"`
	AtuadorID string `json:"atuador_id"`
	Comando   string `json:"comando"`
}

func handleAPIComando(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"erro": "método não permitido"})
		return
	}

	var cmd ComandoDashboard
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"erro": "JSON inválido"})
		return
	}

	estado := cmd.Comando == "LIGAR"
	state.Mutex.Lock()
	switch cmd.AtuadorID {
	case "bomba_irrigacao_01":
		state.BombaIrrigacao[cmd.NodeID] = estado
	case "ventilador_01":
		state.Ventilador[cmd.NodeID] = estado
	case "painel_led_01":
		state.LuzArtifical[cmd.NodeID] = estado
	case "exaustor_teto_01":
		state.Exaustor[cmd.NodeID] = estado
	case "aquecedor_01":
		state.Aquecedor[cmd.NodeID] = estado
	case "motor_comedouro_01":
		state.MotorComedouro[cmd.NodeID] = estado
	case "valvula_agua_01":
		state.ValvulaAgua[cmd.NodeID] = estado
	default:
		state.Mutex.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"erro": "atuador desconhecido"})
		return
	}
	state.Mutex.Unlock()

	network.EnviarComandoTCP(cmd.NodeID, cmd.AtuadorID, cmd.Comando, "manual_dashboard")
	storage.LogAtuador(cmd.NodeID, cmd.AtuadorID, cmd.Comando, "manual_dashboard")
	logger.Integrador.Printf("[DASHBOARD] %s/%s → %s", cmd.NodeID, cmd.AtuadorID, cmd.Comando)

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIAlertas retorna todos os alertas ou apenas os ativos (?ativos=true)
func handleAPIAlertas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	apenasAtivos := r.URL.Query().Get("ativos") == "true"
	alertas, err := storage.GetAlertas(apenasAtivos)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}
	if alertas == nil {
		alertas = []storage.AlertaLog{}
	}
	json.NewEncoder(w).Encode(alertas)
}

// handleAPIAlertaAck reconhece (ack) um alerta pelo ID
func handleAPIAlertaAck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"erro": "id obrigatório"})
		return
	}

	if err := storage.AckAlerta(body.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIConfig — GET retorna thresholds, POST atualiza
func handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodGet {
		state.Mutex.Lock()
		defer state.Mutex.Unlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"Estufa_A": map[string]interface{}{
				"umidade_min":     state.AlvoUmidadeMinima["Estufa_A"],
				"umidade_max":     state.AlvoUmidadeMaxima["Estufa_A"],
				"temp_max":        state.AlvoTempMaxima["Estufa_A"],
				"critico_umidade": state.LimiteCriticoUmidade["Estufa_A"],
				"critico_temp":    state.LimiteCriticoTempEstufa["Estufa_A"],
			},
			"Galinheiro_A": map[string]interface{}{
				"racao_min":      state.AlvoRacaoMinima["Galinheiro_A"],
				"racao_max":      state.AlvoRacaoMaxima["Galinheiro_A"],
				"amonia_max":     state.AlvoAmoniaMaxima["Galinheiro_A"],
				"agua_min":       state.AlvoAguaMinima["Galinheiro_A"],
				"agua_max":       state.AlvoAguaMaxima["Galinheiro_A"],
				"temp_min":       state.AlvoTempMinima["Galinheiro_A"],
				"critico_racao":  state.LimiteCriticoRacao["Galinheiro_A"],
				"critico_amonia": state.LimiteCriticoAmonia["Galinheiro_A"],
				"critico_agua":   state.LimiteCriticoAgua["Galinheiro_A"],
			},
		})
		return
	}

	if r.Method == http.MethodPost {
		var payload map[string]map[string]float64
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"erro": "JSON inválido"})
			return
		}

		state.Mutex.Lock()
		for nodeID, campos := range payload {
			for chave, valor := range campos {
				switch nodeID + "|" + chave {
				case "Estufa_A|umidade_min":
					state.AlvoUmidadeMinima[nodeID] = valor
				case "Estufa_A|umidade_max":
					state.AlvoUmidadeMaxima[nodeID] = valor
				case "Estufa_A|temp_max":
					state.AlvoTempMaxima[nodeID] = valor
				case "Estufa_A|critico_umidade":
					state.LimiteCriticoUmidade[nodeID] = valor
				case "Estufa_A|critico_temp":
					state.LimiteCriticoTempEstufa[nodeID] = valor
				case "Galinheiro_A|racao_min":
					state.AlvoRacaoMinima[nodeID] = valor
				case "Galinheiro_A|racao_max":
					state.AlvoRacaoMaxima[nodeID] = valor
				case "Galinheiro_A|amonia_max":
					state.AlvoAmoniaMaxima[nodeID] = valor
				case "Galinheiro_A|agua_min":
					state.AlvoAguaMinima[nodeID] = valor
				case "Galinheiro_A|agua_max":
					state.AlvoAguaMaxima[nodeID] = valor
				case "Galinheiro_A|temp_min":
					state.AlvoTempMinima[nodeID] = valor
				case "Galinheiro_A|critico_racao":
					state.LimiteCriticoRacao[nodeID] = valor
				case "Galinheiro_A|critico_amonia":
					state.LimiteCriticoAmonia[nodeID] = valor
				case "Galinheiro_A|critico_agua":
					state.LimiteCriticoAgua[nodeID] = valor
				}
			}
		}
		state.Mutex.Unlock()

		logger.Integrador.Printf("[CONFIG] Thresholds atualizados pelo dashboard")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func handleAPISensorData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipoSensor := strings.TrimPrefix(r.URL.Path, "/api/sensor/")
	if tipoSensor == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"erro": "tipo de sensor não especificado"})
		return
	}

	horas := 1
	if h := r.URL.Query().Get("horas"); h != "" {
		fmt.Sscanf(h, "%d", &horas)
	}

	dados, err := storage.GetSensorDataByType(tipoSensor, horas)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(dados)
}

func handleAPIAtuadorHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	horas := 24
	if h := r.URL.Query().Get("horas"); h != "" {
		fmt.Sscanf(h, "%d", &horas)
	}

	dados, err := storage.GetAllAtuadorHistory(horas)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(dados)
}
