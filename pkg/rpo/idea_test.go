package rpo

import (
	"testing"
)

// TestIDEA_KnownBroken documenta o estado real: a implementação em
// idea.go NÃO passa em round-trip nem reproduz dado real capturado (ver
// aviso em idea.go). Mantido como teste (não removido, não ignorado
// silenciosamente) para que qualquer tentativa futura de terminar a
// depuração tenha um sinal verde/vermelho claro, em vez de descobrir o
// problema de novo do zero.
func TestIDEA_KnownBroken(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewIDEACipher(key)
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	ct := make([]byte, 8)
	c.Encrypt(ct, pt)
	back := make([]byte, 8)
	c.Decrypt(back, ct)

	roundTripOK := string(back) == string(pt)
	t.Logf("round-trip ok=%v pt=%x ct=%x back=%x", roundTripOK, pt, ct, back)
	if roundTripOK {
		t.Fatal("round-trip passou! Se você corrigiu o bug, atualize o aviso [!] em idea.go e adicione de volta a validação contra dado real (ver histórico deste arquivo).")
	}
}
