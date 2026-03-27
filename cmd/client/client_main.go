package main

import (
	"flag"
	"fmt"
	"os"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

func main() {
	sensorType := flag.String("sensor",  "", "Tipo de sensor: umidade|temperatura|luminosidade|amonia|racao|agua")
	nodeID      := flag.String("node",   "", "ID do nó: Estufa_A|Estufa_B|Galinheiro_A|Galinheiro_B")
	modoAtuador := flag.Bool("atuador",  false, "Inicia o listener de atuadores (porta única 9090)")
	flag.Parse()

	switch {

	// ── Modo sensor (envia via UDP) ────────────────────────────────────────────
	case *sensorType != "" && *nodeID != "":
		fmt.Printf("Iniciando sensor '%s' no nó '%s' (UDP)...\n", *sensorType, *nodeID)

		serverIP := os.Getenv("SERVER_IP")
		if serverIP == "" {
			serverIP = "localhost:8080"
		}

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
			fmt.Fprintf(os.Stderr, "Nó '%s' não reconhecido\n", *nodeID)
			os.Exit(1)
		}

	// ── Modo atuador (listener TCP único na porta 9090) ────────────────────────
	// Todos os atuadores da máquina compartilham esta porta.
	// O roteamento para o atuador correto é feito pelo AtuadorID no JSON.
	case *modoAtuador:
		fmt.Println("Iniciando listener de atuadores na porta 9090 (TCP, porta única)...")
		fmt.Println("Aguardando comandos do servidor...")
		network.EscutarComandosTCP("0.0.0.0:9090")
		select {} // bloqueia forever

	// ── Modo legado (tudo na mesma máquina, sem Docker) ───────────────────────
	default:
		fmt.Println("Modo legado: iniciando todos os sensores e atuadores localmente...")

		// Um único listener para todos os atuadores
		go network.EscutarComandosTCP("0.0.0.0:9090")

		// Sensores Estufa_A (UDP → localhost:8080)
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_umidade_01",      "umidade",      "localhost:8080", "%")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_temp_01",         "temperatura",  "localhost:8080", "C")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_luz_01",          "luminosidade", "localhost:8080", "Lux")

		// Sensores Galinheiro_A (UDP → localhost:8080)
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_amonia_01","amonia",      "localhost:8080", "ppm")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_temp_01",  "temperatura", "localhost:8080", "C")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_racao_01", "racao",       "localhost:8080", "%")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_agua_01",  "agua",        "localhost:8080", "%")

		select {}
	}
}

func getUnidade(tipo string) string {
	switch tipo {
	case "umidade", "racao", "agua":
		return "%"
	case "temperatura":
		return "C"
	case "luminosidade":
		return "Lux"
	case "amonia":
		return "ppm"
	default:
		return ""
	}
}
