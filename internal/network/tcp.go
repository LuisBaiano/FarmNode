package network

import (
	"encoding/json"
	"net"
	"os"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

func atuadorAddr() string {
	host := os.Getenv("ATUADOR_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("ATUADOR_PORT")
	if port == "" {
		port = "9000"
	}
	return net.JoinHostPort(host, port)
}

func EnviarComandoTCP(estufaID, acao, motivo string) {
	conn, err := net.Dial("tcp", atuadorAddr())
	if err != nil {
		logger.Atuador.Printf("Erro ao conectar no atuador da %s: %v", estufaID, err)
		return
	}
	defer conn.Close()

	comando := models.ComandoAtuador{
		EstufaID:          estufaID,
		AtuadorID:         "bomba_irrigacao_01",
		Comando:           acao,
		MotivoAcionamento: motivo,
		TimestampOrdem:    time.Now(),
	}

	json.NewEncoder(conn).Encode(comando)
	logger.Atuador.Printf("Ordem '%s' enviada para %s", acao, estufaID)
}

// EscutarComandosTCP é usado pelo CLIENTE (Estufas) para receber ordens
func EscutarComandosTCP(porta string) {
	listener, err := net.Listen("tcp", ":"+porta)
	if err != nil {
		logger.Atuador.Fatalf("Erro ao iniciar escuta TCP: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Atuador.Printf("Erro accept TCP: %v", err)
			continue
		}

		var comando models.ComandoAtuador
		if err := json.NewDecoder(conn).Decode(&comando); err != nil {
			logger.Atuador.Printf("Erro decode comando TCP: %v", err)
			conn.Close()
			continue
		}

		// Altera o estado da bomba de forma segura entre as threads
		state.Mutex.Lock()
		if comando.Comando == "LIGAR" {
			state.Ligadas[comando.EstufaID] = true
			logger.Atuador.Printf("Comando LIGAR aceito para %s", comando.EstufaID)
		} else if comando.Comando == "DESLIGAR" {
			state.Ligadas[comando.EstufaID] = false
			logger.Atuador.Printf("Comando DESLIGAR aceito para %s", comando.EstufaID)
		}
		state.Mutex.Unlock()

		conn.Close()
	}
}
