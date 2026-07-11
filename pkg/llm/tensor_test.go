package llm

import (
	"math"
	"os"
	"testing"
)

func TestFloat16ToFloat32(t *testing.T) {
	cases := []struct {
		bits uint16
		want float32
	}{
		{0x0000, 0},
		{0x8000, float32(math.Copysign(0, -1))},
		{0x3C00, 1.0},  // 1.0
		{0xC000, -2.0}, // -2.0
		{0x3555, 0.333251953125},
		{0x7BFF, 65504},                 // maior valor normal representável
		{0x0001, 5.960464477539063e-08}, // menor subnormal (2^-24)
		{0x7C00, float32(math.Inf(1))},
		{0xFC00, float32(math.Inf(-1))},
	}
	for _, c := range cases {
		got := Float16ToFloat32(c.bits)
		if math.IsInf(float64(c.want), 0) {
			if !math.IsInf(float64(got), int(math.Copysign(1, float64(c.want)))) {
				t.Errorf("Float16ToFloat32(0x%04x) = %v, want Inf", c.bits, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("Float16ToFloat32(0x%04x) = %v, want %v", c.bits, got, c.want)
		}
	}
}

// TestFloat16TableExhaustive confere, para TODOS os 65536 padrões de bits
// possíveis, que a tabela pré-computada (float16Table, usada por
// Float16ToFloat32) é bit-a-bit idêntica à implementação de referência
// (float16ToFloat32Bits) — garantia total, não amostragem, já que o espaço
// de entrada é pequeno o bastante para cobrir por inteiro. NaN é comparado
// pelos bits crus (math.Float32bits), não por `==` (NaN != NaN em ponto
// flutuante) — payloads de NaN diferentes contariam como divergência real.
func TestFloat16TableExhaustive(t *testing.T) {
	for h := 0; h < 65536; h++ {
		want := float16ToFloat32Bits(uint16(h))
		got := Float16ToFloat32(uint16(h))
		if math.Float32bits(want) != math.Float32bits(got) {
			t.Fatalf("h=0x%04x: table=%v (bits %#x), reference=%v (bits %#x)",
				h, got, math.Float32bits(got), want, math.Float32bits(want))
		}
	}
}

func TestRMSNorm(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	w := []float32{1, 1, 1, 1}
	out := RMSNorm(x, w, 1e-6)
	// rms = sqrt((1+4+9+16)/4) = sqrt(7.5) = 2.7386...
	rms := math.Sqrt(7.5)
	for i, v := range x {
		want := float32(float64(v) / rms)
		if diff := math.Abs(float64(out[i] - want)); diff > 1e-5 {
			t.Errorf("RMSNorm[%d] = %v, want %v", i, out[i], want)
		}
	}
}

func TestSoftmaxSumsToOne(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	Softmax(x)
	var sum float32
	for _, v := range x {
		sum += v
	}
	if diff := math.Abs(float64(sum - 1)); diff > 1e-5 {
		t.Errorf("soma do softmax = %v, want 1", sum)
	}
}

func TestRoPEPreservesNorm(t *testing.T) {
	x := []float32{1, 0, 0, 1}
	before := x[0]*x[0] + x[1]*x[1] + x[2]*x[2] + x[3]*x[3]
	RoPE(x, 4, 5, 4, 10000)
	after := x[0]*x[0] + x[1]*x[1] + x[2]*x[2] + x[3]*x[3]
	if diff := math.Abs(float64(before - after)); diff > 1e-4 {
		t.Errorf("RoPE não preservou a norma: antes=%v depois=%v", before, after)
	}
}

func TestEmbedRowRealModel(t *testing.T) {
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer g.Close()

	row, err := EmbedRow(g, "token_embd.weight", 11, 3072) // token 11 = eos/bos deste modelo
	if err != nil {
		t.Fatalf("EmbedRow: %v", err)
	}
	if len(row) != 3072 {
		t.Fatalf("len(row) = %d, want 3072", len(row))
	}
	var hasNonZero bool
	for _, v := range row {
		if v != 0 {
			hasNonZero = true
		}
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("EmbedRow produziu valor inválido: %v", v)
		}
	}
	if !hasNonZero {
		t.Fatal("EmbedRow retornou tudo zero — suspeita de erro de decodificação")
	}
}
