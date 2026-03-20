package state

import "sync"

var (
	// Mutex garante que apenas uma thread acesse os mapas por vez
	Mutex sync.Mutex

	// --- ESTADOS DOS ATUADORES (true = Ligado, false = Desligado) ---
	// Estufas (3 Atuadores)
	BombaIrrigacao = make(map[string]bool)
	Ventilador     = make(map[string]bool)
	LuzArtifical   = make(map[string]bool)

	// Galinheiros (4 Atuadores)
	Exaustor       = make(map[string]bool)
	Aquecedor      = make(map[string]bool)
	MotorComedouro = make(map[string]bool)
	ValvulaAgua    = make(map[string]bool) // Controle de Água

	// --- VALORES ALVO (Limites configuráveis pelo Menu do Cliente) ---
	// Estufas
	AlvoUmidadeMinima = map[string]float64{"Estufa_A": 15.0, "Estufa_B": 15.0}
	AlvoTempMaxima    = map[string]float64{"Estufa_A": 35.0, "Estufa_B": 35.0}
	AlvoLuzMinima     = map[string]float64{"Estufa_A": 300.0, "Estufa_B": 300.0}

	// Galinheiros
	AlvoAmoniaMaxima = map[string]float64{"Galinheiro_A": 20.0, "Galinheiro_B": 20.0}
	AlvoTempMinima   = map[string]float64{"Galinheiro_A": 24.0, "Galinheiro_B": 24.0}
	AlvoRacaoMinima  = map[string]float64{"Galinheiro_A": 10.0, "Galinheiro_B": 10.0}
	AlvoAguaMinima   = map[string]float64{"Galinheiro_A": 15.0, "Galinheiro_B": 15.0} // Controle de Água
)
