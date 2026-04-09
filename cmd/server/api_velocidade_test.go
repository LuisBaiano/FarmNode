package main

import (
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandleAPIVelocidadeCampos(t *testing.T) {
	atomic.StoreUint64(&totalDatagramas, 321)
	atomic.StoreUint64(&totalUDPInvalidos, 7)
	atomic.StoreInt64(&ultimoDatagramaNS, time.Now().UnixNano())
	atomic.StoreInt64(&ultimoInvalidoDatNS, time.Now().Add(-time.Second).UnixNano())

	req := httptest.NewRequest("GET", "/api/velocidade", nil)
	rr := httptest.NewRecorder()
	handleAPIVelocidade(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status inesperado: got=%d", rr.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json invalido: %v", err)
	}

	for _, k := range []string{"total_datagramas", "total_invalidos_json", "ultimo_datagrama", "ultimo_invalido_json", "trace_terminal"} {
		if _, ok := payload[k]; !ok {
			t.Fatalf("campo ausente em /api/velocidade: %s", k)
		}
	}
}
