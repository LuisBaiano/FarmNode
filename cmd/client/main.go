package main

import (
	"fmt"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

func main() {
	fmt.Println("Iniciando Simulador de Hardware (Nós Distribuídos)...")
	fmt.Println("As placas estão ligadas e gerando dados via IPs locais (127.x.x.x)!")

	// =========================================================
	// 1. INICIANDO OS ATUADORES (Escutando ordens via TCP)
	// =========================================================

	// --- Estufa A (Sub-rede 1) ---
	go network.EscutarComandosTCP("127.0.1.10:6001", "Estufa_A", "bomba_irrigacao_01")
	go network.EscutarComandosTCP("127.0.1.11:6002", "Estufa_A", "ventilador_01")
	go network.EscutarComandosTCP("127.0.1.12:6003", "Estufa_A", "painel_led_01")

	// --- Galinheiro A (Sub-rede 2) ---
	go network.EscutarComandosTCP("127.0.2.10:6001", "Galinheiro_A", "exaustor_teto_01")
	go network.EscutarComandosTCP("127.0.2.11:6002", "Galinheiro_A", "aquecedor_01")
	go network.EscutarComandosTCP("127.0.2.12:6003", "Galinheiro_A", "motor_comedouro_01")
	go network.EscutarComandosTCP("127.0.2.13:6004", "Galinheiro_A", "valvula_agua_01")

	// --- Estufa B (Sub-rede 3) ---
	go network.EscutarComandosTCP("127.0.3.10:6001", "Estufa_B", "bomba_irrigacao_01")
	go network.EscutarComandosTCP("127.0.3.11:6002", "Estufa_B", "ventilador_01")
	go network.EscutarComandosTCP("127.0.3.12:6003", "Estufa_B", "painel_led_01")

	// --- Galinheiro B (Sub-rede 4) ---
	go network.EscutarComandosTCP("127.0.4.10:6001", "Galinheiro_B", "exaustor_teto_01")
	go network.EscutarComandosTCP("127.0.4.11:6002", "Galinheiro_B", "aquecedor_01")
	go network.EscutarComandosTCP("127.0.4.12:6003", "Galinheiro_B", "motor_comedouro_01")
	go network.EscutarComandosTCP("127.0.4.13:6004", "Galinheiro_B", "valvula_agua_01")

	// =========================================================
	// 2. INICIANDO OS SENSORES (Enviando telemetria via UDP)
	// =========================================================

	// --- Estufa A (Física Botânica) ---
	go simulador.IniciarSensorEstufa("Estufa_A", "sensor_umidade_01", "umidade", "127.0.1.10:5001", "%")
	go simulador.IniciarSensorEstufa("Estufa_A", "sensor_temp_01", "temperatura", "127.0.1.11:5002", "C")
	go simulador.IniciarSensorEstufa("Estufa_A", "sensor_luz_01", "luminosidade", "127.0.1.12:5003", "Lux")

	// --- Galinheiro A (Física Zootécnica) ---
	go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_amonia_01", "amonia", "127.0.2.10:5001", "ppm")
	go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_temp_01", "temperatura", "127.0.2.11:5002", "C")
	go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_racao_01", "racao", "127.0.2.12:5003", "%")
	go simulador.IniciarSensorGalinheiro("Galinheiro_A", "sensor_agua_01", "agua", "127.0.2.13:5004", "%")

	// --- Estufa B (Física Botânica) ---
	go simulador.IniciarSensorEstufa("Estufa_B", "sensor_umidade_01", "umidade", "127.0.3.10:5001", "%")
	go simulador.IniciarSensorEstufa("Estufa_B", "sensor_temp_01", "temperatura", "127.0.3.11:5002", "C")
	go simulador.IniciarSensorEstufa("Estufa_B", "sensor_luz_01", "luminosidade", "127.0.3.12:5003", "Lux")

	// --- Galinheiro B (Física Zootécnica) ---
	go simulador.IniciarSensorGalinheiro("Galinheiro_B", "sensor_amonia_01", "amonia", "127.0.4.10:5001", "ppm")
	go simulador.IniciarSensorGalinheiro("Galinheiro_B", "sensor_temp_01", "temperatura", "127.0.4.11:5002", "C")
	go simulador.IniciarSensorGalinheiro("Galinheiro_B", "sensor_racao_01", "racao", "127.0.4.12:5003", "%")
	go simulador.IniciarSensorGalinheiro("Galinheiro_B", "sensor_agua_01", "agua", "127.0.4.13:5004", "%")

	// O select vazio trava a Main Thread indefinidamente,
	// permitindo que as 28 goroutines acima rodem em paralelo para sempre.
	select {}
}
