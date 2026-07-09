package llm

import (
	"math"
	"os"
	"testing"
)

// TestForwardRealModel roda um passo de inferência completo no Falcon3-3B-
// 1.58bit e confere que os logits são finitos e não degenerados. É lento
// (dezenas de segundos, forward pass escalar sem otimização) — pulado com
// `go test -short`.
func TestForwardRealModel(t *testing.T) {
	if testing.Short() {
		t.Skip("lento: forward pass completo em 22 camadas escalares")
	}
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}

	m, err := LoadModel(falcon3Path)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()

	if m.NLayer != 22 || m.NHead != 12 || m.NHeadKV != 4 || m.HeadDim != 256 {
		t.Fatalf("hparams = layer=%d head=%d headKV=%d headDim=%d, want 22/12/4/256",
			m.NLayer, m.NHead, m.NHeadKV, m.HeadDim)
	}

	ctx := NewContext(m)
	logits, err := ctx.Forward(11) // token bos/eos deste modelo
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if len(logits) != m.VocabSize {
		t.Fatalf("len(logits) = %d, want %d", len(logits), m.VocabSize)
	}

	maxV, minV := logits[0], logits[0]
	for _, v := range logits {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("logit inválido: %v", v)
		}
		if v > maxV {
			maxV = v
		}
		if v < minV {
			minV = v
		}
	}
	if maxV == minV {
		t.Fatal("todos os logits são idênticos — forward pass provavelmente quebrado")
	}

	// um segundo passo (pos=1) precisa continuar funcionando com o KV cache
	// já populado pela primeira chamada.
	if _, err := ctx.Forward(11); err != nil {
		t.Fatalf("Forward (segundo passo): %v", err)
	}
	if ctx.pos != 2 {
		t.Fatalf("ctx.pos = %d, want 2", ctx.pos)
	}
}
