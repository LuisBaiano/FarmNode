package state

import "sync"

var (
	// Mutex garante acesso seguro concorrente a todos os mapas
	Mutex sync.Mutex

	// Valores em tempo real dos sensores (atualizados a cada datagrama UDP)
	ValoresSensores = map[string]map[string]float64{
		"Estufa_A":     make(map[string]float64),
		"Estufa_B":     make(map[string]float64),
		"Galinheiro_A": make(map[string]float64),
		"Galinheiro_B": make(map[string]float64),
	}

	// ── Estados dos atuadores (true = Ligado) ─────────────────────────────────
	BombaIrrigacao = make(map[string]bool)
	Ventilador     = make(map[string]bool)
	LuzArtifical   = make(map[string]bool)
	Exaustor       = make(map[string]bool)
	Aquecedor      = make(map[string]bool)
	MotorComedouro = make(map[string]bool)
	ValvulaAgua    = make(map[string]bool)

	// ── Limites de ativação automática (configuráveis pelo dashboard) ──────────

	// Estufa — Bomba de irrigação
	// Liga quando umidade < Min, desliga quando > Max
	AlvoUmidadeMinima = map[string]float64{"Estufa_A": 15.0, "Estufa_B": 15.0}
	AlvoUmidadeMaxima = map[string]float64{"Estufa_A": 55.0, "Estufa_B": 55.0}

	// Estufa — Ventilador
	// Liga quando temp > Max, desliga quando < (Max - 5°C)
	AlvoTempMaxima = map[string]float64{"Estufa_A": 35.0, "Estufa_B": 35.0}

	// Estufa — Painel LED (ativação manual/automática por luminosidade)
	AlvoLuzMinima = map[string]float64{"Estufa_A": 300.0, "Estufa_B": 300.0}

	// Galinheiro — Exaustor (amônia)
	// Liga quando amônia > Max, desliga quando < (Max - 10 ppm)
	AlvoAmoniaMaxima = map[string]float64{"Galinheiro_A": 20.0, "Galinheiro_B": 20.0}

	// Galinheiro — Aquecedor (temperatura)
	// Liga quando temp < Min, desliga quando > (Min + 5°C)
	AlvoTempMinima = map[string]float64{"Galinheiro_A": 24.0, "Galinheiro_B": 24.0}

	// Galinheiro — Motor Comedouro (ração)
	// Liga quando ração < Min, desliga quando > Max
	AlvoRacaoMinima = map[string]float64{"Galinheiro_A": 10.0, "Galinheiro_B": 10.0}
	AlvoRacaoMaxima = map[string]float64{"Galinheiro_A": 90.0, "Galinheiro_B": 90.0}

	// Galinheiro — Válvula de Água
	// Liga quando água < Min, desliga quando > Max
	AlvoAguaMinima = map[string]float64{"Galinheiro_A": 15.0, "Galinheiro_B": 15.0}
	AlvoAguaMaxima = map[string]float64{"Galinheiro_A": 80.0, "Galinheiro_B": 80.0}

	// ── Limites críticos — geram alertas visuais no dashboard ─────────────────
	// São piores que os limites de ativação automática.
	// Ex: bomba deveria ter ligado com 15% mas algo ocorreu e caiu para 5%.
	// A lógica automática continua tentando agir, mas o alerta notifica o operador.

	LimiteCriticoUmidade   = map[string]float64{"Estufa_A": 5.0, "Estufa_B": 5.0}
	LimiteCriticoTempEstufa = map[string]float64{"Estufa_A": 45.0, "Estufa_B": 45.0}
	LimiteCriticoAmonia    = map[string]float64{"Galinheiro_A": 35.0, "Galinheiro_B": 35.0}
	LimiteCriticoRacao     = map[string]float64{"Galinheiro_A": 5.0, "Galinheiro_B": 5.0}
	LimiteCriticoAgua      = map[string]float64{"Galinheiro_A": 5.0, "Galinheiro_B": 5.0}
)
