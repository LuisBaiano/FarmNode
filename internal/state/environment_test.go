package state

import (
	"sync"
	"testing"
)

func resetStateForTest() {
	Mutex = sync.Mutex{}
	rrMu = sync.Mutex{}
	ValoresSensores = map[string]map[string]float64{}
	EstadoAtuadores = map[string]map[string]bool{}
	atuadorRoundRobin = map[string]int{}
}

func TestFindAtuadorPorTipoParaChave_StableMapping(t *testing.T) {
	resetStateForTest()
	SetAtuador("Estufa_A", "bomba_01", false)
	SetAtuador("Estufa_A", "bomba_02", false)
	SetAtuador("Estufa_A", "bomba_03", false)

	a := FindAtuadorPorTipoParaChave("Estufa_A", "bomba", "sensor_x")
	b := FindAtuadorPorTipoParaChave("Estufa_A", "bomba", "sensor_x")
	if a == "" || b == "" {
		t.Fatalf("atuador vazio para chave estável")
	}
	if a != b {
		t.Fatalf("mapeamento não estável para mesma chave: %s != %s", a, b)
	}
}

func TestFindAtuadorPorTipo_RoundRobin(t *testing.T) {
	resetStateForTest()
	SetAtuador("Estufa_A", "ventilador_01", false)
	SetAtuador("Estufa_A", "ventilador_02", false)

	a := FindAtuadorPorTipo("Estufa_A", "ventilador")
	b := FindAtuadorPorTipo("Estufa_A", "ventilador")
	if a == "" || b == "" {
		t.Fatalf("atuador vazio")
	}
	if a == b {
		t.Fatalf("round-robin não alternou: %s == %s", a, b)
	}
}

func TestFindAtuadorPorTipo_LedFallback(t *testing.T) {
	resetStateForTest()
	SetAtuador("Estufa_A", "painel_led_01", false)
	if got := FindAtuadorPorTipo("Estufa_A", "led"); got == "" {
		t.Fatalf("esperava encontrar atuador led por contains")
	}
}
