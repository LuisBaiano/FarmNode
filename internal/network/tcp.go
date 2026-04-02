package network

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

// ════════════════════════════════════════════════════════════════════════════
// Lado SERVIDOR — porta 6000
//
// Arquitetura: atuadores CONECTAM ao servidor (nao o contrario).
// Isso permite porta unica para todos os atuadores e elimina a necessidade
// de expor portas individuais nos containers de atuadores.
//
// Fluxo:
//  1. Servidor ouve em 0.0.0.0:6000
//  2. Atuador conecta e envia RegistroAtuador {"node_id":"...", "atuador_id":"..."}
//  3. Servidor guarda a conexao em atuadorConns[atuadorID]
//  4. Quando precisar acionar, server escreve ComandoAtuador na conexao existente
// ════════════════════════════════════════════════════════════════════════════

var (
	atuadorConns   = make(map[string]net.Conn) // atuadorID -> conexao persistente
	atuadorConnsMu sync.RWMutex
)

// EscutarAtuadoresTCP inicia o listener TCP na porta 6000.
// Todos os atuadores (de qualquer maquina) conectam nesta unica porta.
// Deve ser chamado uma vez em goroutine separada no servidor.
func EscutarAtuadoresTCP(ipPorta string) {
	listener, err := net.Listen("tcp", ipPorta)
	if err != nil {
		logger.Atuador.Fatalf("[TCP:6000] Erro ao iniciar listener: %v", err)
	}
	defer listener.Close()

	logger.Atuador.Printf("[TCP:6000] Aguardando conexoes de atuadores em %s", ipPorta)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Atuador.Printf("[TCP:6000] Erro ao aceitar conexao: %v", err)
			continue
		}
		go registrarAtuador(conn)
	}
}

// registrarAtuador le o registro inicial e mantem a conexao aberta para comandos.
func registrarAtuador(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var reg models.RegistroAtuador
	if err := json.NewDecoder(conn).Decode(&reg); err != nil {
		logger.Atuador.Printf("[TCP:6000] Registro invalido de %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{}) // remove deadline apos registro

	if reg.AtuadorID == "" || reg.NodeID == "" {
		logger.Atuador.Printf("[TCP:6000] Registro incompleto de %s", conn.RemoteAddr())
		conn.Close()
		return
	}

	// Guarda conexao no mapa
	atuadorConnsMu.Lock()
	// Fecha conexao anterior do mesmo atuador (reconexao)
	if old, ok := atuadorConns[reg.AtuadorID]; ok {
		old.Close()
	}
	atuadorConns[reg.AtuadorID] = conn
	atuadorConnsMu.Unlock()

	logger.Atuador.Printf("[TCP:6000] Atuador registrado: %s/%s (%s)",
		reg.NodeID, reg.AtuadorID, conn.RemoteAddr())

	// Mantem conexao viva — detecta desconexao pelo erro de leitura
	// Usa scanner para detectar desconexao (Read retorna erro quando o cliente fecha)
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Atuadores nao enviam dados periodicos; linhas vazias sao keepalive opcional
	}

	// Conexao encerrada (atuador desconectou)
	atuadorConnsMu.Lock()
	if atuadorConns[reg.AtuadorID] == conn {
		delete(atuadorConns, reg.AtuadorID)
	}
	atuadorConnsMu.Unlock()

	conn.Close()
	logger.Atuador.Printf("[TCP:6000] Atuador desconectado: %s/%s", reg.NodeID, reg.AtuadorID)
}

// EnviarComandoTCP envia um comando para o atuador usando a conexao persistente.
// Se o atuador nao estiver conectado, loga warning e retorna sem erro fatal.
func EnviarComandoTCP(nodeID, atuadorID, acao, motivo string) {
	atuadorConnsMu.RLock()
	conn, ok := atuadorConns[atuadorID]
	atuadorConnsMu.RUnlock()

	if !ok {
		logger.Atuador.Printf("[TCP:6000] Atuador '%s' nao conectado — comando '%s' ignorado", atuadorID, acao)
		return
	}

	comando := models.ComandoAtuador{
		NodeID:            nodeID,
		AtuadorID:         atuadorID,
		Comando:           acao,
		MotivoAcionamento: motivo,
		TimestampOrdem:    time.Now(),
	}

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(comando); err != nil {
		logger.Atuador.Printf("[TCP:6000] Erro ao enviar comando para %s: %v", atuadorID, err)
		// Remove conexao invalida
		atuadorConnsMu.Lock()
		if atuadorConns[atuadorID] == conn {
			delete(atuadorConns, atuadorID)
		}
		atuadorConnsMu.Unlock()
		conn.Close()
		return
	}

	logger.Atuador.Printf("[TCP:6000] '%s' enviado -> %s/%s", acao, nodeID, atuadorID)
}

// AtuadoresConectados retorna lista de atuadores atualmente conectados.
// Usado pelo /api/estado para informar quais atuadores estao online.
func AtuadoresConectados() []string {
	atuadorConnsMu.RLock()
	defer atuadorConnsMu.RUnlock()
	ids := make([]string, 0, len(atuadorConns))
	for id := range atuadorConns {
		ids = append(ids, id)
	}
	return ids
}

// ════════════════════════════════════════════════════════════════════════════
// Lado ATUADOR (cliente) — conecta ao servidor na porta 6000
//
// Cada container de atuador chama ConectarAtuadorTCP.
// Ele se registra e fica em loop lendo comandos do servidor.
// Se a conexao cair, tenta reconectar automaticamente a cada 3s.
// ════════════════════════════════════════════════════════════════════════════

// ConectarAtuadorTCP e chamado pelo processo cliente no modo atuador.
// Conecta ao servidor, registra o atuador e aguarda comandos indefinidamente.
func ConectarAtuadorTCP(serverAddr, nodeID, atuadorID string) {
	for {
		err := conectarEProcessar(serverAddr, nodeID, atuadorID)
		if err != nil {
			logger.Atuador.Printf("[ATUADOR] %s/%s: desconectado (%v), reconectando em 3s...",
				nodeID, atuadorID, err)
		}
		time.Sleep(3 * time.Second)
	}
}

func conectarEProcessar(serverAddr, nodeID, atuadorID string) error {
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Envia registro inicial
	reg := models.RegistroAtuador{NodeID: nodeID, AtuadorID: atuadorID}
	if err := json.NewEncoder(conn).Encode(reg); err != nil {
		return err
	}

	logger.Atuador.Printf("[ATUADOR] %s/%s registrado no servidor %s", nodeID, atuadorID, serverAddr)

	// Loop de leitura — aguarda comandos do servidor
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		linha := scanner.Bytes()
		if len(linha) == 0 {
			continue // keepalive vazio
		}

		var cmd models.ComandoAtuador
		if err := json.Unmarshal(linha, &cmd); err != nil {
			logger.Atuador.Printf("[ATUADOR] Comando invalido: %v", err)
			continue
		}

		processarComandoLocal(cmd, nodeID, atuadorID)
	}

	return scanner.Err()
}

// processarComandoLocal atualiza o estado local do atuador ao receber um comando.
func processarComandoLocal(cmd models.ComandoAtuador, nodeID, atuadorID string) {
	estado := cmd.Comando == "LIGAR"

	state.Mutex.Lock()
	switch atuadorID {
	case "bomba_irrigacao_01":
		state.BombaIrrigacao[nodeID] = estado
	case "ventilador_01":
		state.Ventilador[nodeID] = estado
	case "painel_led_01":
		state.LuzArtifical[nodeID] = estado
	case "exaustor_teto_01":
		state.Exaustor[nodeID] = estado
	case "aquecedor_01":
		state.Aquecedor[nodeID] = estado
	case "motor_comedouro_01":
		state.MotorComedouro[nodeID] = estado
	case "valvula_agua_01":
		state.ValvulaAgua[nodeID] = estado
	}
	state.Mutex.Unlock()

	logger.Atuador.Printf("[ATUADOR] %s/%s -> %s (%s)",
		nodeID, atuadorID, cmd.Comando, cmd.MotivoAcionamento)
}

// enderecoAtuador: mantido para compatibilidade com modo legado (sem Docker)
func enderecoAtuador(atuadorID string) string {
	if v := os.Getenv("ATUADOR_" + atuadorID); v != "" {
		return v
	}
	padroes := map[string]string{
		"bomba_irrigacao_01": "localhost:6001",
		"ventilador_01":      "localhost:6002",
		"painel_led_01":      "localhost:6003",
		"exaustor_teto_01":   "localhost:6004",
		"aquecedor_01":       "localhost:6005",
		"motor_comedouro_01": "localhost:6006",
		"valvula_agua_01":    "localhost:6007",
	}
	if p, ok := padroes[atuadorID]; ok {
		return p
	}
	return "localhost:6001"
}
