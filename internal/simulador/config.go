package simulador

import (
	"os"
	"strconv"
	"strings"
	"time"
)

var tiposAtuadorConhecidos = []string{
	"bomba",
	"ventilador",
	"led",
	"exaustor",
	"aquecedor",
	"motor",
	"valvula",
}

func envDurationMS(key string, defMS, minMS, maxMS int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return time.Duration(defMS) * time.Millisecond
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(defMS) * time.Millisecond
	}
	if v < minMS {
		v = minMS
	}
	if v > maxMS {
		v = maxMS
	}
	return time.Duration(v) * time.Millisecond
}

func tipoAtuadorID(id string) string {
	low := strings.ToLower(id)
	for _, t := range tiposAtuadorConhecidos {
		if low == t || strings.HasPrefix(low, t+"_") || strings.Contains(low, "_"+t+"_") || strings.HasSuffix(low, "_"+t) {
			return t
		}
	}
	for _, t := range tiposAtuadorConhecidos {
		if strings.Contains(low, t) {
			return t
		}
	}
	for i, c := range id {
		if c == '_' && i > 0 {
			return strings.ToLower(id[:i])
		}
	}
	return strings.ToLower(id)
}
