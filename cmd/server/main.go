package main

import (
	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"encoding/json"
	"log"
	"net"

	"FarmNode/internal/network"
)

func main() {
	estadoBombas := make(map[string]string)

	enderecoUDP, err := net.ResolveUDPAddr("udp4", ":8080")
	if err != nil {
		log.Fatalf("Erro ao resolver endereço UDP: %v", err)
	}

	conexaoUDP, err := net.ListenUDP("udp4", enderecoUDP)
	if err != nil {
		log.Fatalf("Erro ao iniciar escuta UDP: %v", err)
	}
	defer conexaoUDP.Close()

	logger.Integrador.Println("Servidor Cérebro iniciado! Escutando UDP na porta 8080...")

	buffer := make([]byte, 1024)

	for {
		n, _, err := conexaoUDP.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		var sensor models.MensagemSensor
		json.Unmarshal(buffer[:n], &sensor)

		logger.Integrador.Printf("[UDP] Recebido: %s -> %.1f%%", sensor.EstufaID, sensor.Valor)

		estadoAtual := estadoBombas[sensor.EstufaID]

		if sensor.Valor < 15.0 && estadoAtual != "LIGADA" {
			logger.Integrador.Printf("[ALERTA] %s atingiu %.1f%%. Acionando irrigação!", sensor.EstufaID, sensor.Valor)
			network.EnviarComandoTCP(sensor.EstufaID, "LIGAR", "umidade_critica_baixa")
			estadoBombas[sensor.EstufaID] = "LIGADA"
		}

		if sensor.Valor > 60.0 && estadoAtual == "LIGADA" {
			logger.Integrador.Printf("[INFO] %s atingiu %.1f%%. Desligando irrigação.", sensor.EstufaID, sensor.Valor)
			network.EnviarComandoTCP(sensor.EstufaID, "DESLIGAR", "umidade_ideal_atingida")
			estadoBombas[sensor.EstufaID] = "DESLIGADA"
		}
	}
}
