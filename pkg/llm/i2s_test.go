package llm

import (
	"math"
	"os"
	"testing"
)

// buildTestRow monta os 32 bytes de uma linha de 128 valores ternários onde
// value[0]=-1, value[32]=0, value[64]=+1, value[96]=+1 e todo o resto é 0,
// replicando manualmente a codificação de 2 bits usada por ggml (byte gp
// guarda os códigos de value[gp], value[gp+32], value[gp+64], value[gp+96]
// nos bits 7:6, 5:4, 3:2, 1:0 — código 0=-1, 1=0, 2=+1).
func buildTestRow() []byte {
	row := make([]byte, 32)
	row[0] = (0 << 6) | (1 << 4) | (2 << 2) | (2 << 0) // -1, 0, +1, +1
	for i := 1; i < 32; i++ {
		row[i] = (1 << 6) | (1 << 4) | (1 << 2) | (1 << 0) // tudo zero
	}
	return row
}

func TestDotI2SRowKnownPattern(t *testing.T) {
	row := buildTestRow()
	// soma esperada dos ternários: -1 + 0 + 1 + 1 = 1
	q := make([]int8, 128)
	for i := range q {
		q[i] = 4
	}
	got := dotI2SRow(row, 0, 128, q, sumInt8(q))
	want := int32(1 * 4)
	if got != want {
		t.Errorf("dotI2SRow = %d, want %d", got, want)
	}
}

func TestMatMulI2SKnownPattern(t *testing.T) {
	w := &I2SWeight{
		Packed: buildTestRow(),
		Scale:  2.0,
		NRows:  1,
		NCols:  128,
	}
	x := make([]float32, 128)
	for i := range x {
		x[i] = 1.0
	}
	out := MatMulI2S(w, x)
	// dequantizado: value[0]=-2, value[64]=+2, value[96]=+2, resto 0 -> dot com todos 1 = 2.0
	want := float32(2.0)
	if diff := math.Abs(float64(out[0] - want)); diff > 1e-3 {
		t.Errorf("MatMulI2S = %v, want %v", out[0], want)
	}
}

func TestQuantizeI8SRoundTrip(t *testing.T) {
	x := []float32{-1, -0.5, 0, 0.5, 1}
	q, scale := quantizeI8S(x)
	if scale != 127 {
		t.Errorf("scale = %v, want 127", scale)
	}
	want := []int8{-127, -64, 0, 64, 127}
	for i := range q {
		if q[i] != want[i] {
			t.Errorf("q[%d] = %d, want %d", i, q[i], want[i])
		}
	}
}

// TestLoadI2SWeightRealModel confere um tensor I2_S real do Falcon3-1.58bit:
// carrega, roda MatMulI2S com um vetor de entrada real (embedding do token
// eos) e checa que a saída é finita e não trivialmente zero.
func TestLoadI2SWeightRealModel(t *testing.T) {
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer g.Close()

	w, err := LoadI2SWeight(g, "blk.0.attn_q.weight")
	if err != nil {
		t.Fatalf("LoadI2SWeight: %v", err)
	}
	if w.NRows != 3072 || w.NCols != 3072 {
		t.Fatalf("shape = %dx%d, want 3072x3072", w.NRows, w.NCols)
	}

	x, err := EmbedRow(g, "token_embd.weight", 11, 3072)
	if err != nil {
		t.Fatalf("EmbedRow: %v", err)
	}
	out := MatMulI2S(w, x)
	if len(out) != 3072 {
		t.Fatalf("len(out) = %d, want 3072", len(out))
	}
	var hasNonZero bool
	for _, v := range out {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("MatMulI2S produziu valor inválido: %v", v)
		}
		if v != 0 {
			hasNonZero = true
		}
	}
	if !hasNonZero {
		t.Fatal("MatMulI2S retornou tudo zero — suspeita de erro de decodificação")
	}
}
