package state

import "sync"

var (
	// Mutex garante que apenas uma thread acesse o mapa por vez
	Mutex sync.Mutex
	// Ligadas mapeia o ID da estufa para o estado atual da sua bomba (true/false)
	Ligadas = make(map[string]bool)
)
