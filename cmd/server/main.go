package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/network"
	"FarmNode/internal/state"
)

func main() {
	// Inicia o Painel de Controle (preparação para futura API/GUI)
	go menuInterativoServidor()

	enderecoUDP, err := net.ResolveUDPAddr("udp4", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("Erro ao resolver endereço UDP: %v", err)
	}

	conexaoUDP, err := net.ListenUDP("udp4", enderecoUDP)
	if err != nil {
		log.Fatalf("Erro ao iniciar escuta UDP: %v", err)
	}
	defer conexaoUDP.Close()

	logger.Integrador.Println("Cérebro FarmNode iniciado! Aguardando telemetria de todas as sub-redes...")

	buffer := make([]byte, 1024)

	for {
		n, _, err := conexaoUDP.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		var sensor models.MensagemSensor
		if err := json.Unmarshal(buffer[:n], &sensor); err != nil {
			continue
		}

		processarRegrasNegocio(sensor)
	}
}

// processarRegrasNegocio agora é DINÂMICO! Funciona para A, B, C, D...
func processarRegrasNegocio(sensor models.MensagemSensor) {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()

	// --- REGRAS GENÉRICAS PARA QUALQUER ESTUFA ---
	if strings.HasPrefix(sensor.NodeID, "Estufa") {
		switch sensor.Tipo {
		case "umidade":
			alvo := state.AlvoUmidadeMinima[sensor.NodeID]
			bombaLigada := state.BombaIrrigacao[sensor.NodeID]

			if sensor.Valor < alvo && !bombaLigada {
				logger.Integrador.Printf("[ALERTA] %s com Umidade %.1f%%. Acionando irrigação!", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "LIGAR", "umidade_critica")
				state.BombaIrrigacao[sensor.NodeID] = true

			} else if sensor.Valor > alvo+40.0 && bombaLigada {
				logger.Integrador.Printf("[INFO] %s Umidade %.1f%% recuperada. Desligando bomba.", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "bomba_irrigacao_01", "DESLIGAR", "umidade_ideal")
				state.BombaIrrigacao[sensor.NodeID] = false
			}
		}
	}

	// --- REGRAS GENÉRICAS PARA QUALQUER GALINHEIRO ---
	if strings.HasPrefix(sensor.NodeID, "Galinheiro") {
		switch sensor.Tipo {
		case "racao":
			alvo := state.AlvoRacaoMinima[sensor.NodeID]
			motorLigado := state.MotorComedouro[sensor.NodeID]

			if sensor.Valor < alvo && !motorLigado {
				logger.Integrador.Printf("[ALERTA] %s Ração em %.1f%%! Ligando motor.", sensor.NodeID, sensor.Valor)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "LIGAR", "racao_baixa")
				state.MotorComedouro[sensor.NodeID] = true

			} else if sensor.Valor > 90.0 && motorLigado {
				logger.Integrador.Printf("[INFO] %s Comedouros cheios. Desligando motor.", sensor.NodeID)
				network.EnviarComandoTCP(sensor.NodeID, "motor_comedouro_01", "DESLIGAR", "racao_cheia")
				state.MotorComedouro[sensor.NodeID] = false
			}
		}
	}
}

// menuInterativoServidor com lógica de sub-menus (ideal para mapear para rotas de API depois)
func menuInterativoServidor() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\n=== PAINEL DE CONTROLE CENTRAL (CÉREBRO) ===")
		fmt.Println("1. Ligar Bomba de Irrigação (Estufa)")
		fmt.Println("2. Alterar Alvo de Umidade (Estufa)")
		fmt.Println("3. Ligar Motor de Ração (Galinheiro)")
		fmt.Println("4. Alterar Alvo de Ração (Galinheiro)")
		fmt.Println("0. Sair")
		fmt.Print("Escolha uma ação: ")

		scanner.Scan()
		opcao := strings.TrimSpace(scanner.Text())

		if opcao == "0" {
			fmt.Println("Encerrando servidor...")
			os.Exit(0)
		}

		// Pergunta qual é o Nó alvo (A ou B)
		fmt.Print("Qual unidade? (Digite A ou B): ")
		scanner.Scan()
		letra := strings.ToUpper(strings.TrimSpace(scanner.Text()))
		if letra != "A" && letra != "B" {
			fmt.Println("Unidade inválida! Tente novamente.")
			continue
		}

		switch opcao {
		case "1":
			node := "Estufa_" + letra
			fmt.Printf(">>> Disparando comando TCP manual para %s...\n", node)
			network.EnviarComandoTCP(node, "bomba_irrigacao_01", "LIGAR", "comando_manual")
			state.Mutex.Lock()
			state.BombaIrrigacao[node] = true
			state.Mutex.Unlock()

		case "2":
			node := "Estufa_" + letra
			fmt.Printf("Digite o novo valor mínimo de umidade (%%) para %s: ", node)
			scanner.Scan()
			novoValor, err := strconv.ParseFloat(strings.TrimSpace(scanner.Text()), 64)
			if err == nil {
				state.Mutex.Lock()
				state.AlvoUmidadeMinima[node] = novoValor
				state.Mutex.Unlock()
				fmt.Printf(">>> Alvo de %s atualizado para %.1f%%\n", node, novoValor)
			}

		case "3":
			node := "Galinheiro_" + letra
			fmt.Printf(">>> Disparando comando TCP manual para %s...\n", node)
			network.EnviarComandoTCP(node, "motor_comedouro_01", "LIGAR", "comando_manual")
			state.Mutex.Lock()
			state.MotorComedouro[node] = true
			state.Mutex.Unlock()

		case "4":
			node := "Galinheiro_" + letra
			fmt.Printf("Digite o novo valor mínimo de ração (%%) para %s: ", node)
			scanner.Scan()
			novoValor, err := strconv.ParseFloat(strings.TrimSpace(scanner.Text()), 64)
			if err == nil {
				state.Mutex.Lock()
				state.AlvoRacaoMinima[node] = novoValor
				state.Mutex.Unlock()
				fmt.Printf(">>> Alvo de ração de %s atualizado para %.1f%%\n", node, novoValor)
			}

		default:
			fmt.Println("Opção inválida.")
		}
	}
}
