package main

import (
	"flag"
	"fmt"
	"os"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

// nodeDoAtuador mapeia cada atuador ao seu no
var nodeDoAtuador = map[string]string{
	"bomba_irrigacao_01": "Estufa_A",
	"ventilador_01":      "Estufa_A",
	"painel_led_01":      "Estufa_A",
	"exaustor_teto_01":   "Galinheiro_A",
	"aquecedor_01":       "Galinheiro_A",
	"motor_comedouro_01": "Galinheiro_A",
	"valvula_agua_01":    "Galinheiro_A",
}

func main() {
	sensorType := flag.String("sensor",  "", "Tipo de sensor: umidade|temperatura|luminosidade|amonia|racao|agua")
	nodeID      := flag.String("node",   "", "ID do no: Estufa_A|Galinheiro_A")
	atuadorID   := flag.String("atuador","", "ID do atuador: bomba_irrigacao_01|ventilador_01|...")
	flag.Parse()

	switch {

	// ── Modo sensor ────────────────────────────────────────────────────────────
	// Envia datagramas UDP para SERVER_IP:8080.
	// Responsabilidade: simular fisicamente o ambiente e enviar dados CRUS.
	// NAO interpreta valores, NAO loga eventos — apenas envia numeros.
	case *sensorType != "" && *nodeID != "":
		serverIP := os.Getenv("SERVER_IP")
		if serverIP == "" {
			serverIP = "localhost:8080"
		}
		fmt.Printf("Sensor '%s' no '%s' iniciado — UDP -> %s (1ms)\n", *sensorType, *nodeID, serverIP)

		switch *nodeID {
		case "Estufa_A", "Estufa_B":
			simulador.IniciarSensorEstufa(*nodeID,
				fmt.Sprintf("sensor_%s_01", *sensorType),
				*sensorType, serverIP, getUnidade(*sensorType))

		case "Galinheiro_A", "Galinheiro_B":
			simulador.IniciarSensorGalinheiro(*nodeID,
				fmt.Sprintf("sensor_%s_01", *sensorType),
				*sensorType, serverIP, getUnidade(*sensorType))

		default:
			fmt.Fprintf(os.Stderr, "No '%s' nao reconhecido\n", *nodeID)
			os.Exit(1)
		}

	// ── Modo atuador ───────────────────────────────────────────────────────────
	// CONECTA ao servidor na porta 6000 (nao escuta — e o atuador quem disca).
	// Porta unica para todos os atuadores: SERVER_ADDR:6000.
	// Reconecta automaticamente se a conexao cair.
	case *atuadorID != "":
		node, ok := nodeDoAtuador[*atuadorID]
		if !ok {
			fmt.Fprintf(os.Stderr, "Atuador '%s' nao reconhecido\n", *atuadorID)
			os.Exit(1)
		}

		// Endereco do servidor: SERVER_ADDR env var (padrao localhost:6000)
		serverAddr := os.Getenv("SERVER_ADDR")
		if serverAddr == "" {
			serverAddr = "localhost:6000"
		}

		fmt.Printf("Atuador '%s' (%s) conectando ao servidor %s...\n", *atuadorID, node, serverAddr)
		network.ConectarAtuadorTCP(serverAddr, node, *atuadorID)
		select {} // ConectarAtuadorTCP bloqueia em loop de reconexao

	// ── Modo legado (tudo na mesma maquina) ────────────────────────────────────
	default:
		fmt.Println("Modo legado: iniciando todos os sensores e atuadores localmente...")

		serverAddr := os.Getenv("SERVER_ADDR")
		if serverAddr == "" {
			serverAddr = "localhost:6000"
		}

		// Atuadores conectam ao servidor na porta 6000
		go network.ConectarAtuadorTCP(serverAddr, "Estufa_A",    "bomba_irrigacao_01")
		go network.ConectarAtuadorTCP(serverAddr, "Estufa_A",    "ventilador_01")
		go network.ConectarAtuadorTCP(serverAddr, "Estufa_A",    "painel_led_01")
		go network.ConectarAtuadorTCP(serverAddr, "Galinheiro_A","exaustor_teto_01")
		go network.ConectarAtuadorTCP(serverAddr, "Galinheiro_A","aquecedor_01")
		go network.ConectarAtuadorTCP(serverAddr, "Galinheiro_A","motor_comedouro_01")
		go network.ConectarAtuadorTCP(serverAddr, "Galinheiro_A","valvula_agua_01")

		// Sensores enviam UDP para localhost:8080
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_umidade_01",      "umidade",      "localhost:8080", "%")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_temp_01",         "temperatura",  "localhost:8080", "C")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_luz_01",          "luminosidade", "localhost:8080", "Lux")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_amonia_01","amonia",       "localhost:8080", "ppm")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_temp_01",  "temperatura",  "localhost:8080", "C")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_racao_01", "racao",        "localhost:8080", "%")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_agua_01",  "agua",         "localhost:8080", "%")

		select {}
	}
}

func getUnidade(tipo string) string {
	switch tipo {
	case "umidade", "racao", "agua": return "%"
	case "temperatura":              return "C"
	case "luminosidade":             return "Lux"
	case "amonia":                   return "ppm"
	default:                         return ""
	}
}
