package network

import (
	"encoding/json"
	"net"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

// Tabela de Roteamento dos Atuadores
// Cada atuador tem seu próprio IP e Porta exclusivos
var TabelaAtuadores = map[string]map[string]string{
	"Estufa_A": {
		"bomba_irrigacao_01": "127.0.1.10:6001",
		"ventilador_01":      "127.0.1.11:6002",
		"painel_led_01":      "127.0.1.12:6003",
	},
	"Galinheiro_A": {
		"exaustor_teto_01":   "127.0.2.10:6001",
		"aquecedor_01":       "127.0.2.11:6002",
		"motor_comedouro_01": "127.0.2.12:6003",
		"valvula_agua_01":    "127.0.2.13:6004",
	},
	"Estufa_B": {
		"bomba_irrigacao_01": "127.0.3.10:6001",
		"ventilador_01":      "127.0.3.11:6002",
		"painel_led_01":      "127.0.3.12:6003",
	},
	"Galinheiro_B": {
		"exaustor_teto_01":   "127.0.4.10:6001",
		"aquecedor_01":       "127.0.4.11:6002",
		"motor_comedouro_01": "127.0.4.12:6003",
		"valvula_agua_01":    "127.0.4.13:6004",
	},
}

// EnviarComandoTCP é usado pelo Servidor Central para discar para o IP exato do atuador
func EnviarComandoTCP(nodeID, atuadorID, acao, motivo string) {
	// Busca o IP/Porta específico do atuador na tabela
	enderecoAtuador, existe := TabelaAtuadores[nodeID][atuadorID]
	if !existe {
		logger.Atuador.Printf("Erro: Atuador %s não encontrado no node %s", atuadorID, nodeID)
		return
	}

	conn, err := net.Dial("tcp", enderecoAtuador)
	if err != nil {
		logger.Atuador.Printf("Erro ao conectar no atuador %s em %s: %v", atuadorID, enderecoAtuador, err)
		return
	}
	defer conn.Close()

	comando := models.ComandoAtuador{
		NodeID:            nodeID,
		AtuadorID:         atuadorID,
		Comando:           acao,
		MotivoAcionamento: motivo,
		TimestampOrdem:    time.Now(),
	}

	json.NewEncoder(conn).Encode(comando)
	logger.Atuador.Printf("Ordem '%s' enviada para %s (%s)", acao, atuadorID, enderecoAtuador)
}

// EscutarComandosTCP agora escuta em um IP:Porta ESPECÍFICO, não mais de forma global
func EscutarComandosTCP(ipPorta string, nodeID string, atuadorID string) {
	listener, err := net.Listen("tcp", ipPorta)
	if err != nil {
		logger.Atuador.Fatalf("Erro ao iniciar atuador %s em %s: %v", atuadorID, ipPorta, err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		var comando models.ComandoAtuador
		if err := json.NewDecoder(conn).Decode(&comando); err != nil {
			conn.Close()
			continue
		}

		// Altera o estado do sistema de forma segura baseado no ID do Atuador
		state.Mutex.Lock()
		estado := comando.Comando == "LIGAR"

		switch comando.AtuadorID {
		case "bomba_irrigacao_01":
			state.BombaIrrigacao[comando.NodeID] = estado
		case "ventilador_01":
			state.Ventilador[comando.NodeID] = estado
		case "painel_led_01":
			state.LuzArtifical[comando.NodeID] = estado
		case "exaustor_teto_01":
			state.Exaustor[comando.NodeID] = estado
		case "aquecedor_01":
			state.Aquecedor[comando.NodeID] = estado
		case "motor_comedouro_01":
			state.MotorComedouro[comando.NodeID] = estado
		case "valvula_agua_01":
			state.ValvulaAgua[comando.NodeID] = estado
		}

		logger.Atuador.Printf("[%s] %s -> %s", ipPorta, atuadorID, comando.Comando)
		state.Mutex.Unlock()

		conn.Close()
	}
}
