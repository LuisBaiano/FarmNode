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

	// ── Limites de ativacao automatica (configuráveis pelo dashboard) ──────────

	// Estufa — Bomba de irrigacao
	// Liga quando umidade < Min, desliga quando > Max
	AlvoUmidadeMinima = map[string]float64{"Estufa_A": 15.0, "Estufa_B": 15.0}
	AlvoUmidadeMaxima = map[string]float64{"Estufa_A": 55.0, "Estufa_B": 55.0}

	// Estufa — Ventilador
	// Liga quando temperatura > Max, desliga quando < (Max - 5graus)
	AlvoTempMaxima = map[string]float64{"Estufa_A": 35.0, "Estufa_B": 35.0}

	// Estufa — Painel LED
	// Liga quando luminosidade < Min, desliga quando > (Min * 2)
	AlvoLuzMinima = map[string]float64{"Estufa_A": 300.0, "Estufa_B": 300.0}

	// Galinheiro — Exaustor (amonia)
	// Liga quando amonia >= Max, desliga quando < (Max - 10 ppm)
	AlvoAmoniaMaxima = map[string]float64{"Galinheiro_A": 20.0, "Galinheiro_B": 20.0}

	// Galinheiro — Aquecedor (temperatura)
	// Liga quando temperatura < Min, desliga quando > (Min + 5graus)
	AlvoTempMinima = map[string]float64{"Galinheiro_A": 24.0, "Galinheiro_B": 24.0}

	// Galinheiro — Motor Comedouro (racao)
	// Liga quando racao < Min, desliga quando >= Max
	AlvoRacaoMinima = map[string]float64{"Galinheiro_A": 10.0, "Galinheiro_B": 10.0}
	AlvoRacaoMaxima = map[string]float64{"Galinheiro_A": 90.0, "Galinheiro_B": 90.0}

	// Galinheiro — Valvula de Agua
	// Liga quando agua < Min, desliga quando >= Max
	AlvoAguaMinima = map[string]float64{"Galinheiro_A": 15.0, "Galinheiro_B": 15.0}
	AlvoAguaMaxima = map[string]float64{"Galinheiro_A": 80.0, "Galinheiro_B": 80.0}

	// ── Limites criticos — geram alertas visuais no dashboard ─────────────────
	// Representam situacoes piores que o limite de ativacao automatica.
	// A logica automatica continua agindo normalmente, mas o alerta notifica
	// o operador de que algo anormal ocorreu (falha de equipamento, evento fisico).

	// Estufa
	LimiteCriticoUmidade    = map[string]float64{"Estufa_A": 5.0,  "Estufa_B": 5.0}
	LimiteCriticoTempEstufa = map[string]float64{"Estufa_A": 45.0, "Estufa_B": 45.0}
	LimiteCriticoLuminosidade = map[string]float64{"Estufa_A": 100.0, "Estufa_B": 100.0}

	// Galinheiro
	LimiteCriticoAmonia        = map[string]float64{"Galinheiro_A": 35.0, "Galinheiro_B": 35.0}
	LimiteCriticoRacao         = map[string]float64{"Galinheiro_A": 5.0,  "Galinheiro_B": 5.0}
	LimiteCriticoAgua          = map[string]float64{"Galinheiro_A": 5.0,  "Galinheiro_B": 5.0}
	LimiteCriticoTempGalinheiro = map[string]float64{"Galinheiro_A": 15.0, "Galinheiro_B": 15.0}
)
