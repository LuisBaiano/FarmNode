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

// Servidor TCP na porta 6000.
// Os atuadores iniciam a conexao com o servidor.
// Isso simplifica a rede com uma unica porta.

// Fluxo basico:
//  1. Servidor ouve em 0.0.0.0:6000
//  2. Atuador conecta e envia RegistroAtuador {"node_id":"...", "atuador_id":"..."}
//  3. Servidor guarda a conexao em atuadorConns[atuadorID]
//  4. O servidor envia comando na conexao registrada

var (
	atuadorConns   = make(map[string]net.Conn) // atuadorID -> conexao persistente
	atuadorConnsMu sync.RWMutex
)

// Inicia o listener TCP dos atuadores.

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

// Le o registro inicial e mantem a conexao do atuador.
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

	// Salva a conexao ativa.
	atuadorConnsMu.Lock()
	// Em reconexao, fecha a conexao antiga.
	if old, ok := atuadorConns[reg.AtuadorID]; ok {
		old.Close()
	}
	atuadorConns[reg.AtuadorID] = conn
	atuadorConnsMu.Unlock()

	logger.Atuador.Printf("[TCP:6000] Atuador registrado: %s/%s (%s)",
		reg.NodeID, reg.AtuadorID, conn.RemoteAddr())

	// Fica em leitura para detectar desconexao.

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Atuador pode enviar keepalive vazio.
	}

	// Remove atuador desconectado.
	atuadorConnsMu.Lock()
	if atuadorConns[reg.AtuadorID] == conn {
		delete(atuadorConns, reg.AtuadorID)
	}
	atuadorConnsMu.Unlock()

	conn.Close()
	logger.Atuador.Printf("[TCP:6000] Atuador desconectado: %s/%s", reg.NodeID, reg.AtuadorID)
}

// Informa se o atuador esta conectado.
func AtuadorConectado(atuadorID string) bool {
	atuadorConnsMu.RLock()
	defer atuadorConnsMu.RUnlock()
	_, ok := atuadorConns[atuadorID]
	return ok
}

// Envia comando na conexao do atuador e informa se deu certo.

func EnviarComandoTCP(nodeID, atuadorID, acao, motivo string) bool {
	atuadorConnsMu.RLock()
	conn, ok := atuadorConns[atuadorID]
	atuadorConnsMu.RUnlock()

	if !ok {
		logger.Atuador.Printf("[TCP:6000] Atuador '%s' nao conectado — comando '%s' ignorado", atuadorID, acao)
		return false
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
		// Remove conexao invalida do mapa.
		atuadorConnsMu.Lock()
		if atuadorConns[atuadorID] == conn {
			delete(atuadorConns, atuadorID)
		}
		atuadorConnsMu.Unlock()
		conn.Close()
		return false
	}

	logger.Atuador.Printf("[TCP:6000] '%s' enviado -> %s/%s", acao, nodeID, atuadorID)
	return true
}

// Retorna os IDs dos atuadores conectados.

func AtuadoresConectados() []string {
	atuadorConnsMu.RLock()
	defer atuadorConnsMu.RUnlock()
	ids := make([]string, 0, len(atuadorConns))
	for id := range atuadorConns {
		ids = append(ids, id)
	}
	return ids
}

// Lado atuador (cliente TCP).

// Em queda de conexao, tenta reconectar a cada 3s.

// Conecta, registra e aguarda comandos do servidor.

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

	// Envia registro inicial ao servidor.
	reg := models.RegistroAtuador{NodeID: nodeID, AtuadorID: atuadorID}
	if err := json.NewEncoder(conn).Encode(reg); err != nil {
		return err
	}

	logger.Atuador.Printf("[ATUADOR] %s/%s registrado no servidor %s", nodeID, atuadorID, serverAddr)

	// Loop de leitura dos comandos.
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

// Aplica localmente o comando recebido.
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

// Compatibilidade com modo legado (sem Docker).
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
