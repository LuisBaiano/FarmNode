package state

import (
	"hash/fnv"
	"sort"
	"strings"
	"sync"
)

var (
	Mutex sync.Mutex

	// Valores atuais dos sensores: node_id -> tipo -> valor
	ValoresSensores = map[string]map[string]float64{}

	// Estado dos atuadores: node_id -> atuador_id -> ligado/desligado
	// Totalmente dinâmico — qualquer atuador_id é aceito
	EstadoAtuadores = map[string]map[string]bool{}

	// Limites de acionamento automático por node_id
	// Estufa
	AlvoUmidadeMinima = map[string]float64{}
	AlvoUmidadeMaxima = map[string]float64{}
	AlvoTempMaxima    = map[string]float64{}
	AlvoLuzMinima     = map[string]float64{}

	// Galinheiro
	AlvoAmoniaMaxima = map[string]float64{}
	AlvoTempMinima   = map[string]float64{}
	AlvoRacaoMinima  = map[string]float64{}
	AlvoRacaoMaxima  = map[string]float64{}
	AlvoAguaMinima   = map[string]float64{}
	AlvoAguaMaxima   = map[string]float64{}

	// Limites críticos para alertas visuais
	LimiteCriticoUmidade        = map[string]float64{}
	LimiteCriticoTempEstufa     = map[string]float64{}
	LimiteCriticoLuminosidade   = map[string]float64{}
	LimiteCriticoAmonia         = map[string]float64{}
	LimiteCriticoRacao          = map[string]float64{}
	LimiteCriticoAgua           = map[string]float64{}
	LimiteCriticoTempGalinheiro = map[string]float64{}

	rrMu              sync.Mutex
	atuadorRoundRobin = map[string]int{}
)

// SetAtuador define o estado de um atuador dinamicamente.
func SetAtuador(nodeID, atuadorID string, ligado bool) {
	if EstadoAtuadores[nodeID] == nil {
		EstadoAtuadores[nodeID] = make(map[string]bool)
	}
	EstadoAtuadores[nodeID][atuadorID] = ligado
}

// GetAtuador retorna o estado de um atuador. false se não existir.
func GetAtuador(nodeID, atuadorID string) bool {
	if m, ok := EstadoAtuadores[nodeID]; ok {
		return m[atuadorID]
	}
	return false
}

// FindAtuadorPorTipo mantém compatibilidade: quando não há chave de sensor,
// usa round-robin por nó+tipo para distribuir comandos entre atuadores.
func FindAtuadorPorTipo(nodeID, prefixo string) string {
	return FindAtuadorPorTipoParaChave(nodeID, prefixo, "")
}

// FindAtuadorPorTipoParaChave seleciona atuador por tipo usando uma chave estável
// (ex.: sensor_id) para obter mapeamento 1:1 consistente.
// Se chave vier vazia, usa round-robin por nó+tipo.
func FindAtuadorPorTipoParaChave(nodeID, prefixo, chave string) string {
	ids := matchingAtuadores(nodeID, prefixo)
	if len(ids) == 0 {
		return ""
	}

	if chave != "" {
		idx := hashKey(chave) % len(ids)
		return ids[idx]
	}

	rrKey := nodeID + "|" + prefixo
	rrMu.Lock()
	idx := atuadorRoundRobin[rrKey] % len(ids)
	atuadorRoundRobin[rrKey] = (idx + 1) % len(ids)
	rrMu.Unlock()
	return ids[idx]
}

func matchingAtuadores(nodeID, prefixo string) []string {
	m, ok := EstadoAtuadores[nodeID]
	if !ok || len(m) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		if strings.HasPrefix(id, prefixo) || (prefixo == "led" && strings.Contains(id, "led")) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func hashKey(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}

// AtuadoresDoNo retorna todos os atuador_ids conhecidos para um nó.
func AtuadoresDoNo(nodeID string) map[string]bool {
	if m, ok := EstadoAtuadores[nodeID]; ok {
		return m
	}
	return map[string]bool{}
}
