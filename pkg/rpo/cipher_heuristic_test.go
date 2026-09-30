package rpo

import (
	"testing"
)

func TestCipherHeuristic_DesEdeEcb(t *testing.T) {
	h := NewCipherHeuristic()
	
	// DES-EDE-ECB: key=16 bytes, iv=0
	cipher, err := h.Identify(
		"2c92e5a7d1b59f4f", // 16 bytes
		"",
	)
	if err != nil {
		t.Fatalf("identificar des_ede: %v", err)
	}
	
	// Verificar se é um cipher ECB com key size 16
	if cipher.Mode != "ECB" {
		t.Errorf("modo esperado ECB, got %s", cipher.Mode)
	}
}

func TestCipherHeuristic_RC4(t *testing.T) {
	h := NewCipherHeuristic()
	
	// RC4: key=10 bytes (dentro range 5-40), iv=0
	cipher, err := h.Identify(
		"2c92e5a7d1b59f", // 10 bytes
		"",
	)
	if err != nil {
		t.Fatalf("identificar rc4: %v", err)
	}
	
	// RC4 deve ser identificado como stream cipher
	if cipher.Mode != "STREAM" {
		t.Errorf("modo esperado STREAM, got %s", cipher.Mode)
	}
}

func TestCaptureParser_IdentifyCiphers(t *testing.T) {
	parser := NewCaptureParser()
	
	err := parser.LoadFile("testdata/live_capture.json")
	if err != nil {
		t.Fatalf("carregar captura: %v", err)
	}
	
	err = parser.IdentifyCiphers()
	if err != nil {
		t.Fatalf("identificar ciphers: %v", err)
	}
	
	segments := parser.GetSegments()
	
	// Verificar se todos os segmentos foram identificados
	identified := 0
	for _, s := range segments {
		if s.Cipher != "" {
			identified++
		}
	}
	
	if identified != len(segments) {
		t.Errorf("identificados %d/%d segmentos", identified, len(segments))
	}
}
