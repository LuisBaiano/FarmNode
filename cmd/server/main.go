package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/network"
	"FarmNode/internal/state"
	"FarmNode/internal/storage"
)

func main() {
	// Inicializa banco de dados
	err := storage.InitDB("farmnode_logs.db")
	if err != nil {
		log.Fatalf("Erro ao inicializar banco de dados: %v", err)
	}
	defer storage.CloseDB()

	// Inicia Dashboard HTTP em thread separada
	go startDashboard()

	// Servidor TCP para receber dados de sensores
	listener, err := net.Listen("tcp", "0.0.0.0:8080")
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor TCP: %v", err)
	}
	defer listener.Close()

	logger.Integrador.Println("✓ FarmNode iniciado! Aguardando conexões TCP...")

	// Loop principal - aceita conexões de sensores
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Integrador.Printf("Erro ao aceitar conexão: %v", err)
			continue
		}
		go handleSensorConnection(conn)
	}
}

// handleSensorConnection processa conexões de sensores
func handleSensorConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		var sensor models.MensagemSensor
		if err := json.Unmarshal(scanner.Bytes(), &sensor); err != nil {
			logger.Sensor.Printf("Erro ao decodificar mensagem: %v", err)
			continue
		}

		// Registra log do sensor
		if err := storage.LogSensor(sensor); err != nil {
			logger.Sensor.Printf("Erro ao registrar log: %v", err)
		}

		// Processa regras de negócio automáticas
		processarRegrasAutomaticas(sensor)
	}
}

// processarRegrasAutomaticas aplica lógica automática baseada em sensores
func processarRegrasAutomaticas(sensor models.MensagemSensor) {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	// Regras para estufa
	if strings.HasPrefix(sensor.NodeID, "Estufa") {
		switch sensor.Tipo {
		case "umidade":
			alvo := state.AlvoUmidadeMinima[sensor.NodeID]
			ligado := state.BombaIrrigacao[sensor.NodeID]

			if sensor.Valor < alvo && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.1f%% < %.1f%% - ACIONANDO BOMBA", sensor.NodeID, sensor.Valor, alvo)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_critica")
				state.BombaIrrigacao[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_critica")
			} else if sensor.Valor > alvo+40.0 && ligado {
				logger.Integrador.Printf("[AUTO] %s: Umidade %.1f%% recuperada - DESLIGANDO BOMBA", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
				state.BombaIrrigacao[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
			}
		}
	}

	// Regras para galinheiro
	if strings.HasPrefix(sensor.NodeID, "Galinheiro") {
		switch sensor.Tipo {
		case "racao":
			alvo := state.AlvoRacaoMinima[sensor.NodeID]
			ligado := state.MotorComedouro[sensor.NodeID]

			if sensor.Valor < alvo && !ligado {
				logger.Integrador.Printf("[AUTO] %s: Ração %.1f%% < %.1f%% - ACIONANDO MOTOR", sensor.NodeID, sensor.Valor, alvo)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
				state.MotorComedouro[sensor.NodeID] = true
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
			} else if sensor.Valor > 90.0 && ligado {
				logger.Integrador.Printf("[AUTO] %s: Ração %.1f%% - DESLIGANDO MOTOR", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
				state.MotorComedouro[sensor.NodeID] = false
				storage.LogAtuador(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
			}
		}
	}
}

// startDashboard inicia servidor HTTP para dashboard e APIs
func startDashboard() {
	// Dashboard página HTML
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, getDashboardHTML())
	})

	// API: Estado atual dos atuadores
	http.HandleFunc("/api/data", handleAPIData)

	// API: Dados de sensor por tipo
	http.HandleFunc("/api/sensor/", handleAPISensorData)

	// API: Valor atual de sensor
	http.HandleFunc("/api/sensor/current/", handleAPISensorCurrent)

	// API: Histórico de atuadores
	http.HandleFunc("/api/atuador/history", handleAPIAtuadorHistory)

	// API: Controle de atuador
	http.HandleFunc("/api/atuador/control/", handleAPIAtuadorControl)

	log.Println("✓ Dashboard em http://localhost:8081/dashboard")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Erro no servidor HTTP: %v", err)
	}
}

// handleAPIData retorna estado atual dos atuadores
func handleAPIData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	data := map[string]interface{}{
		"bombaIrrigacao": state.BombaIrrigacao["Estufa_A"],
		"ventilador":     state.Ventilador["Estufa_A"],
		"luzArtificial":  state.LuzArtifical["Estufa_A"],
		"exaustor":       state.Exaustor["Galinheiro_A"],
		"aquecedor":      state.Aquecedor["Galinheiro_A"],
		"motorComedouro": state.MotorComedouro["Galinheiro_A"],
		"valvulaAgua":    state.ValvulaAgua["Galinheiro_A"],
	}

	json.NewEncoder(w).Encode(data)
}

// handleAPISensorData retorna dados históricos de sensor
func handleAPISensorData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipoSensor := strings.TrimPrefix(r.URL.Path, "/api/sensor/")
	if tipoSensor == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"erro": "tipo de sensor não especificado"})
		return
	}

	horas := 24
	if h := r.URL.Query().Get("horas"); h != "" {
		fmt.Sscanf(h, "%d", &horas)
	}

	dados, err := storage.GetSensorDataByType(tipoSensor, horas)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(dados)
}

// handleAPISensorCurrent retorna último valor de sensor
func handleAPISensorCurrent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipoSensor := strings.TrimPrefix(r.URL.Path, "/api/sensor/current/")
	if tipoSensor == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"erro": "tipo de sensor não especificado"})
		return
	}

	dados, err := storage.GetLatestSensorValue(tipoSensor)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(dados)
}

// handleAPIAtuadorHistory retorna histórico de atuadores
func handleAPIAtuadorHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	horas := 24
	if h := r.URL.Query().Get("horas"); h != "" {
		fmt.Sscanf(h, "%d", &horas)
	}

	dados, err := storage.GetAllAtuadorHistory(horas)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(dados)
}

// handleAPIAtuadorControl controla atuadores manualmente
func handleAPIAtuadorControl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		w.WriteHeader(405)
		json.NewEncoder(w).Encode(map[string]string{"erro": "método não permitido"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/atuador/control/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"erro": "parâmetros inválidos"})
		return
	}

	atuadorID := parts[0]
	acao := parts[1]

	if err := controlarAtuador(atuadorID, acao); err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
		return
	}

	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// controlarAtuador executa comando em atuador
func controlarAtuador(atuadorID, acao string) error {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	atuadores := map[string]struct {
		node  string
		nome  string
		state *map[string]bool
	}{
		"bombaIrrigacao": {"Estufa_A", "bomba_irrigacao_01", &state.BombaIrrigacao},
		"ventilador":     {"Estufa_A", "ventilador_01", &state.Ventilador},
		"luzArtificial":  {"Estufa_A", "painel_led_01", &state.LuzArtifical},
		"exaustor":       {"Galinheiro_A", "exaustor_teto_01", &state.Exaustor},
		"aquecedor":      {"Galinheiro_A", "aquecedor_01", &state.Aquecedor},
		"motorComedouro": {"Galinheiro_A", "motor_comedouro_01", &state.MotorComedouro},
		"valvulaAgua":    {"Galinheiro_A", "valvula_agua_01", &state.ValvulaAgua},
	}

	atu, ok := atuadores[atuadorID]
	if !ok {
		return fmt.Errorf("atuador não encontrado")
	}

	ligando := acao == "ligar"
	(*atu.state)[atu.node] = ligando

	comando := strings.ToUpper(acao)
	network.EnviarComandoTCP(atu.node, atu.nome, comando, "manual_dashboard")
	storage.LogAtuador(atu.node, atu.nome, comando, "manual_dashboard")

	return nil
}
