package llm

import (
	"os"
	"testing"
)

// BenchmarkLoadModel mede o custo de abrir e carregar o Falcon3-3B-1.58bit
// inteiro (header GGUF + todos os pesos). Serve de guarda de regressão para
// a fase de parsing do header (ver o bufio.Reader em gguf.go — antes fazia
// uma syscall pread por campo, dominando o tempo de load).
func BenchmarkLoadModel(b *testing.B) {
	if _, err := os.Stat(falcon3Path); err != nil {
		b.Skipf("modelo de teste não disponível: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := LoadModel(falcon3Path)
		if err != nil {
			b.Fatalf("LoadModel: %v", err)
		}
		m.Close()
	}
}

// BenchmarkForward mede um passo de forward pass completo (22 camadas +
// projeção de saída) já com o modelo carregado. Guarda de regressão para
// MatMulI2S/MatMulF16 — ver o histórico de otimizações no CHANGELOG (a
// correção Σq redundante e a tabela/kernel AVX2 de Float16 derrubaram o
// custo por passo em ~14x nesta máquina).
func BenchmarkForward(b *testing.B) {
	if _, err := os.Stat(falcon3Path); err != nil {
		b.Skipf("modelo de teste não disponível: %v", err)
	}
	m, err := LoadModel(falcon3Path)
	if err != nil {
		b.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()
	ctx := NewContext(m)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ctx.Forward(11); err != nil {
			b.Fatalf("Forward: %v", err)
		}
	}
}
