package main

import (
	"bytes"
	"testing"
)

func TestWSMontarELerFrameTexto(t *testing.T) {
	payload := []byte(`{"tipo":"ping"}`)
	frame := wsMontar(wsOpText, payload)

	gotPayload, gotOp, err := wsLerFrame(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("erro ao ler frame ws: %v", err)
	}
	if gotOp != wsOpText {
		t.Fatalf("opcode inesperado: got=%d want=%d", gotOp, wsOpText)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("payload diferente: got=%q want=%q", gotPayload, payload)
	}
}

func TestWSLerFrameMascaraCliente(t *testing.T) {
	payload := []byte("hello")
	mask := [4]byte{0x11, 0x22, 0x33, 0x44}

	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}

	// Frame cliente -> servidor com máscara:
	// FIN+TEXT, MASK+LEN, MASK(4 bytes), PAYLOAD_MASKED
	frame := []byte{0x81, byte(0x80 | len(payload))}
	frame = append(frame, mask[:]...)
	frame = append(frame, masked...)

	gotPayload, gotOp, err := wsLerFrame(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("erro ao ler frame mascarado: %v", err)
	}
	if gotOp != wsOpText {
		t.Fatalf("opcode inesperado: got=%d want=%d", gotOp, wsOpText)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("payload desmascarado inválido: got=%q want=%q", gotPayload, payload)
	}
}
