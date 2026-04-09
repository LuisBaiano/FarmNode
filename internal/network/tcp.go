package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"FarmNode/internal/logger"
	"FarmNode/internal/models"
	"FarmNode/internal/state"
)

type AtuadorConectadoInfo struct {
	NodeID    string    `json:"node_id"`
	AtuadorID string    `json:"atuador_id"`
	Tipo      string    `json:"tipo"`
	Conectado bool      `json:"conectado"`
	LastSeen  time.Time `json:"last_seen"`
}

type AtuadorEvento struct {
	Tipo      string    `json:"tipo"` // conectado | desconectado
	NodeID    string    `json:"node_id"`
	AtuadorID string    `json:"atuador_id"`
	Endereco  string    `json:"endereco"`
	Quando    time.Time `json:"quando"`
}

var (
	atuadorConns   = make(map[string]net.Conn)
	atuadorMeta    = make(map[string]AtuadorConectadoInfo)
	atuadorConnsMu sync.RWMutex
	atuadorEventos = make(chan AtuadorEvento, 2048)
)

func atuadorKey(nodeID, atuadorID string) string {
	return nodeID + "|" + atuadorID
}

func emitirEventoAtuador(tipo, nodeID, atuadorID string, addr net.Addr) {
	end := ""
	if addr != nil {
		end = addr.String()
	}
	ev := AtuadorEvento{
		Tipo:      tipo,
		NodeID:    nodeID,
		AtuadorID: atuadorID,
		Endereco:  end,
		Quando:    time.Now(),
	}
	select {
	case atuadorEventos <- ev:
	default:
		// evita bloquear fluxo de rede sob carga
	}
}

func EventosAtuador() <-chan AtuadorEvento {
	return atuadorEventos
}

// ── Servidor: escuta conexões de atuadores ────────────────────────────────────

func EscutarAtuadoresTCP(ipPorta string) {
	listener, err := net.Listen("tcp", ipPorta)
	if err != nil {
		logger.Atuador.Fatalf("[TCP:6000] Erro ao iniciar listener: %v", err)
	}
	defer listener.Close()
	logger.Atuador.Printf("[TCP:6000] Aguardando atuadores em %s", ipPorta)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Atuador.Printf("[TCP:6000] Erro ao aceitar: %v", err)
			continue
		}
		go registrarAtuador(conn)
	}
}

func registrarAtuador(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var reg models.RegistroAtuador
	if err := json.NewDecoder(conn).Decode(&reg); err != nil {
		logger.Atuador.Printf("[TCP:6000] Registro inválido de %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	if reg.AtuadorID == "" || reg.NodeID == "" {
		conn.Close()
		return
	}

	key := atuadorKey(reg.NodeID, reg.AtuadorID)

	atuadorConnsMu.Lock()
	if old, ok := atuadorConns[key]; ok && old != conn {
		old.Close()
	}
	atuadorConns[key] = conn
	atuadorMeta[key] = AtuadorConectadoInfo{
		NodeID:    reg.NodeID,
		AtuadorID: reg.AtuadorID,
		Tipo:      tipoDispositivo(reg.AtuadorID),
		Conectado: true,
		LastSeen:  time.Now(),
	}
	atuadorConnsMu.Unlock()

	// Registra o atuador no estado dinâmico do servidor
	state.Mutex.Lock()
	state.SetAtuador(reg.NodeID, reg.AtuadorID, false)
	state.Mutex.Unlock()

	logger.Atuador.Printf("[TCP:6000] Atuador registrado: %s/%s (%s)", reg.NodeID, reg.AtuadorID, conn.RemoteAddr())
	emitirEventoAtuador("conectado", reg.NodeID, reg.AtuadorID, conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		atuadorConnsMu.Lock()
		meta := atuadorMeta[key]
		meta.LastSeen = time.Now()
		meta.Conectado = true
		atuadorMeta[key] = meta
		atuadorConnsMu.Unlock()
	}

	atuadorConnsMu.Lock()
	if atual, ok := atuadorConns[key]; ok && atual == conn {
		delete(atuadorConns, key)
	}
	meta := atuadorMeta[key]
	meta.Conectado = false
	meta.LastSeen = time.Now()
	atuadorMeta[key] = meta
	atuadorConnsMu.Unlock()

	conn.Close()
	logger.Atuador.Printf("[TCP:6000] Atuador desconectado: %s/%s", reg.NodeID, reg.AtuadorID)
	emitirEventoAtuador("desconectado", reg.NodeID, reg.AtuadorID, conn.RemoteAddr())
}

func AtuadorConectado(nodeID, atuadorID string) bool {
	atuadorConnsMu.RLock()
	defer atuadorConnsMu.RUnlock()
	meta, ok := atuadorMeta[atuadorKey(nodeID, atuadorID)]
	return ok && meta.Conectado
}

func EnviarComandoTCP(nodeID, atuadorID, acao, motivo string) bool {
	key := atuadorKey(nodeID, atuadorID)

	atuadorConnsMu.RLock()
	conn, ok := atuadorConns[key]
	atuadorConnsMu.RUnlock()

	if !ok {
		logger.Atuador.Printf("[TCP:6000] Atuador '%s/%s' não conectado — '%s' ignorado", nodeID, atuadorID, acao)
		return false
	}

	cmd := models.ComandoAtuador{
		NodeID:            nodeID,
		AtuadorID:         atuadorID,
		Comando:           acao,
		MotivoAcionamento: motivo,
		TimestampOrdem:    time.Now(),
	}

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		logger.Atuador.Printf("[TCP:6000] Erro ao enviar para %s/%s: %v", nodeID, atuadorID, err)
		atuadorConnsMu.Lock()
		if atual, ok := atuadorConns[key]; ok && atual == conn {
			delete(atuadorConns, key)
		}
		meta := atuadorMeta[key]
		meta.Conectado = false
		meta.LastSeen = time.Now()
		atuadorMeta[key] = meta
		atuadorConnsMu.Unlock()
		conn.Close()
		return false
	}

	logger.Atuador.Printf("[TCP:6000] '%s' -> %s/%s", acao, nodeID, atuadorID)
	return true
}

func AtuadoresConectadosInfo() []AtuadorConectadoInfo {
	atuadorConnsMu.RLock()
	defer atuadorConnsMu.RUnlock()

	keys := make([]string, 0, len(atuadorMeta))
	for k := range atuadorMeta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]AtuadorConectadoInfo, 0, len(keys))
	for _, k := range keys {
		out = append(out, atuadorMeta[k])
	}
	return out
}

func AtuadoresPorNode(nodeID string) []AtuadorConectadoInfo {
	all := AtuadoresConectadosInfo()
	out := make([]AtuadorConectadoInfo, 0)
	for _, a := range all {
		if a.NodeID == nodeID {
			out = append(out, a)
		}
	}
	return out
}

func PruneAtuadoresInativos(timeout time.Duration) []AtuadorConectadoInfo {
	cutoff := time.Now().Add(-timeout)
	removidos := make([]AtuadorConectadoInfo, 0)

	atuadorConnsMu.Lock()
	defer atuadorConnsMu.Unlock()

	for k, meta := range atuadorMeta {
		if meta.Conectado || meta.LastSeen.After(cutoff) {
			continue
		}
		if conn, ok := atuadorConns[k]; ok {
			conn.Close()
			delete(atuadorConns, k)
		}
		removidos = append(removidos, meta)
		delete(atuadorMeta, k)
	}
	return removidos
}

// ── Cliente: conecta no servidor e processa comandos ─────────────────────────

// ConectarAtuadorTCP é chamado pelo container do atuador.
// Loop infinito com backoff — reconecta automaticamente.
func ConectarAtuadorTCP(serverAddr, nodeID, atuadorID string) {
	backoff := 1 * time.Second
	for {
		err := conectarEProcessar(serverAddr, nodeID, atuadorID)
		if err != nil {
			logger.Atuador.Printf("[ATUADOR] %s/%s: desconectado (%v), reconectando em %s...",
				nodeID, atuadorID, err, backoff)
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func conectarEProcessar(serverAddr, nodeID, atuadorID string) error {
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(15 * time.Second)
	}

	// Registro inicial
	reg := models.RegistroAtuador{NodeID: nodeID, AtuadorID: atuadorID}
	if err := json.NewEncoder(conn).Encode(reg); err != nil {
		return err
	}

	logger.Atuador.Printf("[ATUADOR] %s/%s registrado em %s", nodeID, atuadorID, serverAddr)

	// Keepalive: envia ping a cada 10s para manter a conexão TCP viva
	stopKA := make(chan struct{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_, _ = fmt.Fprintln(conn, "{}")
			case <-stopKA:
				return
			}
		}
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		linha := scanner.Bytes()
		if len(linha) == 0 || string(linha) == "{}" {
			continue
		}
		var cmd models.ComandoAtuador
		if err := json.Unmarshal(linha, &cmd); err != nil {
			logger.Atuador.Printf("[ATUADOR] Comando inválido: %v", err)
			continue
		}
		// Aplica o comando no estado local do processo atuador
		processarComandoLocal(cmd)
	}

	close(stopKA)
	return scanner.Err()
}

// processarComandoLocal — totalmente dinâmico.
// Não depende de IDs fixos: aplica em qualquer atuador_id.
func processarComandoLocal(cmd models.ComandoAtuador) {
	ligado := cmd.Comando == "LIGAR"

	state.Mutex.Lock()
	state.SetAtuador(cmd.NodeID, cmd.AtuadorID, ligado)
	state.Mutex.Unlock()

	logger.Atuador.Printf("[ATUADOR] %s/%s -> %s (%s)",
		cmd.NodeID, cmd.AtuadorID, cmd.Comando, cmd.MotivoAcionamento)
}

// tipoDispositivo extrai o prefixo do atuador_id como tipo.
// Ex: "bomba_estufa_a_1" -> "bomba"
func tipoDispositivo(id string) string {
	for i, c := range id {
		if c == '_' && i > 0 {
			return id[:i]
		}
	}
	return id
}
