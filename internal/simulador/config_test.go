package simulador

import (
	"os"
	"testing"
	"time"
)

func TestEnvDurationMSBounds(t *testing.T) {
	const key = "TEST_SENSOR_INTERVAL_MS"
	defer os.Unsetenv(key)

	os.Setenv(key, "0")
	if got := envDurationMS(key, 10, 1, 100); got != 1*time.Millisecond {
		t.Fatalf("limite minimo falhou: got=%s", got)
	}

	os.Setenv(key, "9999")
	if got := envDurationMS(key, 10, 1, 100); got != 100*time.Millisecond {
		t.Fatalf("limite maximo falhou: got=%s", got)
	}

	os.Setenv(key, "abc")
	if got := envDurationMS(key, 10, 1, 100); got != 10*time.Millisecond {
		t.Fatalf("fallback default falhou: got=%s", got)
	}
}

func TestTipoAtuadorID(t *testing.T) {
	cases := map[string]string{
		"bomba_estufa_1":     "bomba",
		"painel_led_01":      "led",
		"exaustor_teto_a":    "exaustor",
		"aquecedor_abc":      "aquecedor",
		"motor_comedouro_01": "motor",
		"valvula_agua_01":    "valvula",
	}

	for in, want := range cases {
		if got := tipoAtuadorID(in); got != want {
			t.Fatalf("tipoAtuadorID(%q)=%q, want=%q", in, got, want)
		}
	}
}
