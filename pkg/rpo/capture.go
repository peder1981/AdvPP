package rpo

import (
	"encoding/json"
	"os"
)

// CaptureEvent é um evento bruto do formato JSON gerado tanto por
// tools/rpo-live-inspect/extract_rpo.py (gdb) quanto por
// tools/rpo-live-inspect/rpo_key_hook (LD_PRELOAD) — os dois produtores
// de captura ao vivo desta investigação usam o mesmo schema
// intencionalmente, para serem intercambiáveis aqui.
type CaptureEvent struct {
	N         int    `json:"n"`
	Type      string `json:"type"` // "setkey" | "evpinit" | "encrypt"
	Cipher    string `json:"cipher,omitempty"`
	Key       string `json:"key,omitempty"`
	IV        string `json:"iv,omitempty"`
	Plaintext string `json:"plaintext,omitempty"`
}

// CaptureSegment é um par (evpinit + encrypt) já correlacionado — a
// unidade útil pra decodificação: sabe o cipher/chave/iv reais E o
// plaintext capturado para aquela chamada específica.
type CaptureSegment struct {
	N         int
	Cipher    string
	Key       string
	IV        string
	Plaintext string
}

// LoadCaptureEvents lê um arquivo de captura (JSON: lista de
// CaptureEvent) gerado por qualquer um dos dois produtores.
func LoadCaptureEvents(path string) ([]CaptureEvent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []CaptureEvent
	if err := json.Unmarshal(raw, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// MergeCaptureSegments agrupa eventos "evpinit"+"encrypt" com o mesmo
// número de sequência `n` (como os dois produtores gravam, em pares,
// para cada chamada real de cifra) em CaptureSegment prontos pra usar
// com EncryptSegment/DecryptSegment.
func MergeCaptureSegments(events []CaptureEvent) []CaptureSegment {
	byN := map[int]*CaptureSegment{}
	var order []int
	for _, e := range events {
		s, ok := byN[e.N]
		if !ok {
			s = &CaptureSegment{N: e.N}
			byN[e.N] = s
			order = append(order, e.N)
		}
		switch e.Type {
		case "encrypt":
			s.Plaintext = e.Plaintext
		case "evpinit", "setkey":
			if e.Cipher != "" {
				s.Cipher = e.Cipher
			}
			if e.Key != "" {
				s.Key = e.Key
			}
			if e.IV != "" {
				s.IV = e.IV
			}
		}
	}
	segments := make([]CaptureSegment, 0, len(order))
	for _, n := range order {
		segments = append(segments, *byN[n])
	}
	return segments
}
