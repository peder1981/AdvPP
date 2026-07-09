package llm

import (
	"math/rand"
	"os"
	"testing"
)

func openTokenizer(t *testing.T) *Tokenizer {
	t.Helper()
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	tok, err := NewTokenizer(g)
	if err != nil {
		t.Fatalf("NewTokenizer: %v", err)
	}
	return tok
}

func TestTokenizerRoundTrip(t *testing.T) {
	tok := openTokenizer(t)

	cases := []string{
		"hello world",
		"Olá, mundo! Como vai você?",
		"The quick brown fox jumps over the lazy dog.",
		"função U_Teste() Return .T.",
		"",
		"   espaços   múltiplos   ",
	}
	for _, text := range cases {
		ids := tok.Encode(text)
		got := tok.Decode(ids)
		if got != text {
			t.Errorf("round-trip falhou: entrada=%q ids=%v saída=%q", text, ids, got)
		}
	}
}

func TestTokenizerBOSEOS(t *testing.T) {
	tok := openTokenizer(t)
	if tok.EOS() != 11 {
		t.Errorf("EOS() = %d, want 11", tok.EOS())
	}
	if tok.BOS() != 11 {
		t.Errorf("BOS() = %d, want 11", tok.BOS())
	}
}

func TestGreedy(t *testing.T) {
	logits := []float32{0.1, 5.0, -3.0, 2.0}
	if got := Greedy(logits); got != 1 {
		t.Errorf("Greedy = %d, want 1", got)
	}
}

func TestSampleGreedyEquivalence(t *testing.T) {
	logits := []float32{0.1, 5.0, -3.0, 2.0}
	rng := rand.New(rand.NewSource(1))
	if got := Sample(logits, SamplerConfig{Temperature: 0}, rng); got != 1 {
		t.Errorf("Sample(temp=0) = %d, want 1 (equivalente a Greedy)", got)
	}
}

func TestSampleDeterministicWithSeed(t *testing.T) {
	logits := []float32{1, 2, 3, 4, 5}
	cfg := SamplerConfig{Temperature: 0.8, TopK: 3}
	a := Sample(logits, cfg, rand.New(rand.NewSource(42)))
	b := Sample(logits, cfg, rand.New(rand.NewSource(42)))
	if a != b {
		t.Errorf("mesma seed produziu tokens diferentes: %d vs %d", a, b)
	}
	if a < 2 || a > 4 {
		t.Errorf("Sample com TopK=3 escolheu id=%d fora do top-3 esperado {2,3,4}", a)
	}
}
