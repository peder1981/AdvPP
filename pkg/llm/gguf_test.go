package llm

import (
	"os"
	"testing"
)

const falcon3Path = "/media/peder/DATA/BitNet/models/Falcon3-3B-Instruct-1.58bit/ggml-model-i2_s.gguf"

func TestOpenFalcon3(t *testing.T) {
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}

	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer g.Close()

	if arch, _ := g.String("general.architecture"); arch != "llama" {
		t.Errorf("architecture = %q, want llama", arch)
	}
	if n, _ := g.Uint32("llama.block_count"); n != 22 {
		t.Errorf("block_count = %d, want 22", n)
	}
	if n, _ := g.Uint32("llama.attention.head_count"); n != 12 {
		t.Errorf("head_count = %d, want 12", n)
	}
	if n, _ := g.Uint32("llama.attention.head_count_kv"); n != 4 {
		t.Errorf("head_count_kv = %d, want 4", n)
	}
	if n, _ := g.Uint32("llama.embedding_length"); n != 3072 {
		t.Errorf("embedding_length = %d, want 3072", n)
	}
	if n, _ := g.Uint32("llama.vocab_size"); n != 131072 {
		t.Errorf("vocab_size = %d, want 131072", n)
	}
	if len(g.Tensors) != 201 {
		t.Errorf("n_tensors = %d, want 201", len(g.Tensors))
	}

	typeCounts := map[GGMLType]int{}
	for _, tn := range g.Tensors {
		typeCounts[tn.Type]++
	}
	if got := typeCounts[GGMLTypeI2S]; got != 154 {
		t.Errorf("I2_S tensors = %d, want 154", got)
	}
	if got := typeCounts[GGMLTypeF32]; got != 45 {
		t.Errorf("F32 tensors = %d, want 45", got)
	}
	if got := typeCounts[GGMLTypeF16]; got != 2 {
		t.Errorf("F16 tensors = %d, want 2", got)
	}

	tokens, ok := g.StringArray("tokenizer.ggml.tokens")
	if !ok || len(tokens) != 131072 {
		t.Errorf("tokenizer.ggml.tokens: ok=%v len=%d, want 131072", ok, len(tokens))
	}

	// lê um tensor pequeno de verdade (norma final) e confere o tamanho
	tensor, ok := g.Tensor("output_norm.weight")
	if !ok {
		t.Fatal("output_norm.weight não encontrado")
	}
	data, err := g.TensorData("output_norm.weight")
	if err != nil {
		t.Fatalf("TensorData: %v", err)
	}
	if uint64(len(data)) != tensor.Size {
		t.Errorf("len(data) = %d, want %d", len(data), tensor.Size)
	}
	if tensor.Type != GGMLTypeF32 || tensor.NElements() != 3072 {
		t.Errorf("output_norm.weight: type=%v nelem=%d, want F32/3072", tensor.Type, tensor.NElements())
	}
}
