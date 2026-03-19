package main

import (
	"fmt"
	"log"
	"net"

	"FarmNode/internal/network"
	"FarmNode/internal/simulador"
)

func main() {
	// 1. Inicia a escuta TCP em background (Thread 1)
	go network.EscutarComandosTCP("9000")

	// 2. Prepara a conexão UDP
	enderecoServidor, err := net.ResolveUDPAddr("udp4", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Erro DNS: %v", err)
	}
	conexaoUDP, _ := net.DialUDP("udp4", nil, enderecoServidor)
	defer conexaoUDP.Close()

	fmt.Println("Cliente das Estufas iniciado! (Enviando via UDP, Escutando via TCP)")

	// 3. Inicia as estufas em background (Threads 2 e 3)
	go simulador.Rodar("Estufa_A", conexaoUDP)
	go simulador.Rodar("Estufa_B", conexaoUDP)

	// Mantém o programa rodando
	select {}
}
