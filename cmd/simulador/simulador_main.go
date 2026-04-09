package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

func main() {
	sensorType := flag.String("sensor", "", "Tipo de sensor: umidade|temperatura|luminosidade|amonia|racao|agua")
	sensorID := flag.String("sensor-id", "", "ID único do sensor (gerado automaticamente se omitido)")
	nodeID := flag.String("node", "", "ID do nó (ex: Estufa_A, Galinheiro_B, MeuNo)")
	atuadorID := flag.String("atuador", "", "ID do atuador (ex: bomba_estufa_a_1)")
	flag.Parse()

	switch {

	// ── Sensor UDP ─────────────────────────────────────────────────────────────
	// Envia datagramas UDP para SERVER_IP:8080 a cada 1ms.
	// O servidor faz a descoberta e classificação automática.
	case *sensorType != "" && *nodeID != "":
		idSensor := *sensorID
		if idSensor == "" {
			idSensor = fmt.Sprintf("sensor_%s_auto", *sensorType)
		}
		serverIP := os.Getenv("SERVER_IP")
		if serverIP == "" {
			serverIP = "localhost:8080"
		}
		fmt.Printf("[SENSOR] '%s' (id=%s) no='%s' → UDP %s (1ms)\n", *sensorType, idSensor, *nodeID, serverIP)

		unidade := getUnidade(*sensorType)

		// O simulador é escolhido pelo prefixo do node_id.
		// Qualquer nó que comece com "Galinheiro" usa física de galinheiro,
		// qualquer outro usa física de estufa como padrão.
		if isGalinheiro(*nodeID) {
			simulador.IniciarSensorGalinheiro(*nodeID, idSensor, *sensorType, serverIP, unidade)
		} else {
			simulador.IniciarSensorEstufa(*nodeID, idSensor, *sensorType, serverIP, unidade)
		}

	// ── Atuador TCP ────────────────────────────────────────────────────────────
	// Conecta no servidor pela porta 6000 e fica aguardando comandos.
	// Reconecta automaticamente em caso de queda (backoff de 1s a 10s).
	case *atuadorID != "":
		if *nodeID == "" {
			fmt.Fprintf(os.Stderr, "Erro: -node é obrigatório para atuadores\n")
			fmt.Fprintf(os.Stderr, "Uso: ./simulador_exec -atuador <id> -node <node_id>\n")
			os.Exit(1)
		}

		serverAddr := os.Getenv("SERVER_ADDR")
		if serverAddr == "" {
			serverAddr = "localhost:6000"
		}

		fmt.Printf("[ATUADOR] '%s' no='%s' → TCP %s (reconexão automática)\n", *atuadorID, *nodeID, serverAddr)
		network.ConectarAtuadorTCP(serverAddr, *nodeID, *atuadorID)
		select {}

	// ── Uso incorreto ──────────────────────────────────────────────────────────
	default:
		fmt.Println("FarmNode Simulador — uso:")
		fmt.Println("  Sensor  : ./simulador_exec -sensor <tipo> -node <node_id> [-sensor-id <id>]")
		fmt.Println("  Atuador : ./simulador_exec -atuador <id> -node <node_id>")
		fmt.Println("")
		fmt.Println("Tipos de sensor: umidade | temperatura | luminosidade | amonia | racao | agua")
		fmt.Println("Variável SERVER_IP  (sensor)  — padrão: localhost:8080")
		fmt.Println("Variável SERVER_ADDR (atuador) — padrão: localhost:6000")
		os.Exit(1)
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

func isGalinheiro(nodeID string) bool {
	return strings.HasPrefix(strings.ToLower(nodeID), "galinheiro")
}
