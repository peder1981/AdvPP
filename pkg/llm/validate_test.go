package llm

import (
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	llamaCLIPath = "/home/peder/Projetos/BitNet/build/bin/llama-cli"
)

// TestValidateAgainstLlamaCPP compara, token a token, a geração gulosa do
// motor Go com a saída do llama.cpp de referência (o fork BitNet do
// usuário, já validado matematicamente) para o mesmo prompt. É a checagem
// de correção mais forte que temos: greedy decoding é extremamente sensível
// a qualquer desvio numérico — um só argmax errado em qualquer passo muda o
// estado do KV cache e diverge a geração inteira daí em diante.
//
// Pulado se o binário de referência ou o modelo não estiverem disponíveis
// (ambiente fora da máquina de desenvolvimento do autor), ou com -short
// (é lento: ~40s/token no motor Go).
func TestValidateAgainstLlamaCPP(t *testing.T) {
	if testing.Short() {
		t.Skip("lento: gera múltiplos tokens no motor Go escalar")
	}
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	if _, err := os.Stat(llamaCLIPath); err != nil {
		t.Skipf("llama-cli de referência não disponível: %v", err)
	}

	const prompt = "The capital of France is"
	const nPredict = 6

	cmd := exec.Command(llamaCLIPath,
		"-m", falcon3Path,
		"-p", prompt,
		"-n", "6",
		"--temp", "0",
		"-t", "4",
		"--no-display-prompt",
		"--no-warmup",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("llama-cli: %v", err)
	}
	wantText := strings.TrimRight(string(out), "\n")

	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tok, err := NewTokenizer(g)
	if err != nil {
		g.Close()
		t.Fatalf("NewTokenizer: %v", err)
	}
	g.Close()

	m, err := LoadModel(falcon3Path)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()

	ctx := NewContext(m)
	rng := rand.New(rand.NewSource(1))

	var logits []float32
	for _, id := range tok.Encode(prompt) {
		if logits, err = ctx.Forward(id); err != nil {
			t.Fatalf("Forward (prompt): %v", err)
		}
	}

	var generated []int32
	for i := 0; i < nPredict; i++ {
		next := Sample(logits, SamplerConfig{Temperature: 0}, rng)
		generated = append(generated, next)
		if next == tok.EOS() {
			break
		}
		if logits, err = ctx.Forward(next); err != nil {
			t.Fatalf("Forward (geração): %v", err)
		}
	}
	gotText := tok.Decode(generated)

	if gotText != wantText {
		t.Errorf("divergência do motor Go vs llama.cpp:\n  Go:        %q\n  llama.cpp: %q", gotText, wantText)
	}
}
