package main

import (
	"flag"
	"fmt"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

func main() {
	sensorType := flag.String("sensor", "", "Tipo de sensor (umidade, temperatura, luminosidade, amonia, racao, agua)")
	nodeID := flag.String("node", "", "ID do nó (Estufa_A, Galinheiro_A, etc.)")
	atuadorID := flag.String("atuador", "", "ID do atuador (bomba_irrigacao_01, ventilador_01, etc.)")
	flag.Parse()

	if *sensorType != "" && *nodeID != "" {
		// Modo sensor
		fmt.Printf("Iniciando sensor %s no nó %s...\n", *sensorType, *nodeID)
		switch *nodeID {
		case "Estufa_A", "Estufa_B":
			simulador.IniciarSensorEstufa(*nodeID, fmt.Sprintf("sensor_%s_01", *sensorType), *sensorType, "localhost:5001", getUnidade(*sensorType))
		case "Galinheiro_A", "Galinheiro_B":
			simulador.IniciarSensorGalinheiro(*nodeID, fmt.Sprintf("sensor_%s_01", *sensorType), *sensorType, "localhost:5001", getUnidade(*sensorType))
		default:
			fmt.Printf("Nó %s não reconhecido\n", *nodeID)
		}
	} else if *atuadorID != "" {
		// Modo atuador
		fmt.Printf("Iniciando atuador %s...\n", *atuadorID)
		var endereco string
		switch *atuadorID {
		case "bomba_irrigacao_01":
			endereco = "0.0.0.0:6001"
		case "ventilador_01":
			endereco = "0.0.0.0:6002"
		case "painel_led_01", "exaustor_teto_01":
			endereco = "0.0.0.0:6003"
		case "aquecedor_01":
			endereco = "0.0.0.0:6002"
		case "motor_comedouro_01":
			endereco = "0.0.0.0:6003"
		case "valvula_agua_01":
			endereco = "0.0.0.0:6004"
		default:
			fmt.Printf("Atuador %s não reconhecido\n", *atuadorID)
			return
		}
		// Determinar nodeID baseado no atuador
		var node string
		switch *atuadorID {
		case "bomba_irrigacao_01", "ventilador_01", "painel_led_01":
			node = "Estufa_A"
		case "exaustor_teto_01", "aquecedor_01", "motor_comedouro_01", "valvula_agua_01":
			node = "Galinheiro_A"
		}
		network.EscutarComandosTCP(endereco, node, *atuadorID)
		select {}
	} else {
		// Modo legado (iniciar tudo)
		fmt.Println("Iniciando Simulador de Hardware (Nós Distribuídos)...")
		fmt.Println("As placas estão ligadas e gerando dados via IPs locais (127.x.x.x)!")

		// Iniciar atuadores
		go network.EscutarComandosTCP("127.0.1.10:6001", "Estufa_A", "bomba_irrigacao_01")
		go network.EscutarComandosTCP("127.0.1.11:6002", "Estufa_A", "ventilador_01")
		go network.EscutarComandosTCP("127.0.1.12:6003", "Estufa_A", "painel_led_01")
		go network.EscutarComandosTCP("127.0.2.10:6001", "Galinheiro_A", "exaustor_teto_01")
		go network.EscutarComandosTCP("127.0.2.11:6002", "Galinheiro_A", "aquecedor_01")
		go network.EscutarComandosTCP("127.0.2.12:6003", "Galinheiro_A", "motor_comedouro_01")
		go network.EscutarComandosTCP("127.0.2.13:6004", "Galinheiro_A", "valvula_agua_01")

		// Iniciar sensores
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_umidade_01", "umidade", "127.0.1.10:5001", "%")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_temp_01", "temperatura", "127.0.1.11:5002", "C")
		go simulador.IniciarSensorEstufa("Estufa_A", "sensor_luz_01", "luminosidade", "127.0.1.12:5003", "Lux")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_amonia_01", "amonia", "127.0.2.10:5001", "ppm")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_temp_01", "temperatura", "127.0.2.11:5002", "C")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_racao_01", "racao", "127.0.2.12:5003", "%")
		go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_agua_01", "agua", "127.0.2.13:5004", "%")

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
