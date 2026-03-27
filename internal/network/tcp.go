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

const (
	// PortaAtuadores é a ÚNICA porta TCP usada por todos os atuadores.
	// O servidor se conecta nessa porta no host onde os atuadores estão rodando.
	// O roteamento para o atuador correto é feito pelo campo AtuadorID no JSON.
	PortaAtuadores = "9090"
)

// enderecoAtuadores retorna o host:porta onde os atuadores estão escutando.
// Configurável via variável de ambiente ATUADOR_HOST (ex: 192.168.1.103).
// Padrão: localhost (para rodar tudo na mesma máquina).
func enderecoAtuadores() string {
	host := os.Getenv("ATUADOR_HOST")
	if host == "" {
		host = "localhost"
	}
	return host + ":" + PortaAtuadores
}

// EnviarComandoTCP envia um comando para o host de atuadores na porta única 9090.
// O atuador correto é identificado pelo campo AtuadorID dentro do JSON.
func EnviarComandoTCP(nodeID, atuadorID, acao, motivo string) {
	endereco := enderecoAtuadores()

	conn, err := net.DialTimeout("tcp", endereco, 2*time.Second)
	if err != nil {
		logger.Atuador.Printf("[TCP] Não foi possível conectar em %s (atuadores): %v", endereco, err)
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

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(comando); err != nil {
		logger.Atuador.Printf("[TCP] Erro ao enviar comando %s/%s: %v", nodeID, atuadorID, err)
		return
	}

	logger.Atuador.Printf("[TCP] '%s' enviado → %s/%s (%s)", acao, nodeID, atuadorID, endereco)
}

// EscutarComandosTCP inicia o listener TCP único na porta 9090.
// Todos os atuadores da máquina compartilham esta porta.
// O roteamento para o atuador correto é feito pelo campo AtuadorID no JSON recebido.
// Deve ser chamado UMA VEZ no processo cliente, em goroutine separada.
func EscutarComandosTCP(ipPorta string) {
	listener, err := net.Listen("tcp", ipPorta)
	if err != nil {
		logger.Atuador.Fatalf("[TCP] Erro ao iniciar listener na porta %s: %v", ipPorta, err)
	}
	defer listener.Close()

	logger.Atuador.Printf("[TCP] Listener de atuadores escutando em %s (porta única)", ipPorta)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Atuador.Printf("[TCP] Erro ao aceitar conexão: %v", err)
			continue
		}
		go processarComando(conn)
	}
}

// processarComando decodifica o JSON recebido e roteia para o atuador correto
// com base no campo AtuadorID.
func processarComando(conn net.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var comando models.ComandoAtuador
	if err := json.NewDecoder(conn).Decode(&comando); err != nil {
		logger.Atuador.Printf("[TCP] Erro ao decodificar comando: %v", err)
		return
	}

	estado := comando.Comando == "LIGAR"

	state.Mutex.Lock()
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
	default:
		logger.Atuador.Printf("[TCP] AtuadorID desconhecido: %s", comando.AtuadorID)
	}
	state.Mutex.Unlock()

	logger.Atuador.Printf("[TCP:%s] %s/%s → %s (%s)",
		PortaAtuadores, comando.NodeID, comando.AtuadorID, comando.Comando, comando.MotivoAcionamento)
}
