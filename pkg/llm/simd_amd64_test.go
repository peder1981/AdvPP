//go:build amd64

package llm

import (
	"math"
	"math/rand"
	"testing"
)

// rawCodeSumRef é uma referência Go pura e deliberadamente independente do
// kernel AVX2: soma dos códigos brutos de 2 bits (0..3) vezes a ativação
// int8, sem nenhum SIMD. Serve só para validar dotI2SBlocksAVX2 — qualquer
// erro de ordem de operando ou deslocamento no assembly deve divergir daqui
// com dados aleatórios.
func rawCodeSumRef(packed []byte, q []int8, nBlocks int) int32 {
	var sum int32
	for blk := 0; blk < nBlocks; blk++ {
		rowData := packed[blk*32 : (blk+1)*32]
		qBlk := q[blk*128 : (blk+1)*128]
		for gp := 0; gp < 32; gp++ {
			b := rowData[gp]
			sum += int32((b>>6)&0x3) * int32(qBlk[gp])
			sum += int32((b>>4)&0x3) * int32(qBlk[32+gp])
			sum += int32((b>>2)&0x3) * int32(qBlk[64+gp])
			sum += int32((b>>0)&0x3) * int32(qBlk[96+gp])
		}
	}
	return sum
}

func TestDotI2SBlocksAVX2VsScalar(t *testing.T) {
	if !hasAVX2 {
		t.Skip("CPU sem AVX2")
	}
	rng := rand.New(rand.NewSource(7))

	for _, nBlocks := range []int{0, 1, 2, 3, 10, 24, 72} {
		packed := make([]byte, nBlocks*32)
		q := make([]int8, nBlocks*128)
		rng.Read(packed)
		for i := range q {
			q[i] = int8(rng.Intn(256) - 128)
		}
		// códigos válidos são só 0,1,2 (3 é reservado/não usado pelo
		// quantizador) — restringe os bytes gerados a essa faixa nos 2
		// bits de cada posição para refletir dados reais.
		for i := range packed {
			var b byte
			for shift := 0; shift < 8; shift += 2 {
				code := byte(rng.Intn(3)) // 0,1,2
				b |= code << shift
			}
			packed[i] = b
		}

		want := rawCodeSumRef(packed, q, nBlocks)
		got := dotI2SBlocksAVX2(packed, q, nBlocks)
		if got != want {
			t.Fatalf("nBlocks=%d: dotI2SBlocksAVX2 = %d, want %d", nBlocks, got, want)
		}
	}
}

// TestDotI2SBlocksAVX2AllCodes cobre exaustivamente os 4 códigos possíveis
// (incluindo o código 3, inválido mas ainda assim precisa ter comportamento
// definido e consistente com a referência escalar).
func TestDotI2SBlocksAVX2AllCodes(t *testing.T) {
	if !hasAVX2 {
		t.Skip("CPU sem AVX2")
	}
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 50; trial++ {
		packed := make([]byte, 32)
		q := make([]int8, 128)
		rng.Read(packed)
		for i := range q {
			q[i] = int8(rng.Intn(256) - 128)
		}
		want := rawCodeSumRef(packed, q, 1)
		got := dotI2SBlocksAVX2(packed, q, 1)
		if got != want {
			t.Fatalf("trial %d: dotI2SBlocksAVX2 = %d, want %d (packed=%v)", trial, got, want, packed)
		}
	}
}

func TestDotI2SBlocksAVX2AllOnesQ(t *testing.T) {
	if !hasAVX2 {
		t.Skip("CPU sem AVX2")
	}
	rng := rand.New(rand.NewSource(3))
	packed := make([]byte, 32)
	rng.Read(packed)
	q := make([]int8, 128)
	for i := range q {
		q[i] = 1
	}
	got := dotI2SBlocksAVX2(packed, q, 1)
	want := rawCodeSumRef(packed, q, 1)
	t.Logf("got=%d want=%d packed=%v", got, want, packed)
	if got != want {
		t.Errorf("got=%d, want=%d", got, want)
	}
}

// TestDotI2SBlocksAVX2EachByteLane cobre cada posição de byte 0..31
// individualmente. Existe porque a primeira versão deste kernel tinha um
// VZEROUPPER posicionado antes da extração dos 128 bits altos do
// acumulador, zerando exatamente os dados de gp>=16 antes de lê-los — esse
// teste pega qualquer regressão na fronteira dos 128 bits baixos/altos.
// dotF16Ref é a referência pura-Go para dotF16BlocksAVX2, usando a mesma
// Float16ToFloat32 (tabela) que o caminho escalar de MatMulF16 usa — serve
// só para validar o kernel AVX2/F16C com dados aleatórios.
func dotF16Ref(rowF16 []byte, x []float32, nBlocks int) float32 {
	var sum float32
	for i := 0; i < nBlocks*8; i++ {
		h := uint16(rowF16[i*2]) | uint16(rowF16[i*2+1])<<8
		sum += Float16ToFloat32(h) * x[i]
	}
	return sum
}

func TestDotF16BlocksAVX2VsScalar(t *testing.T) {
	if !hasF16CFMA {
		t.Skip("CPU sem F16C/FMA")
	}
	rng := rand.New(rand.NewSource(13))
	for _, nBlocks := range []int{0, 1, 2, 3, 10, 24, 72, 384} {
		rowF16 := make([]byte, nBlocks*16)
		rng.Read(rowF16)
		x := make([]float32, nBlocks*8)
		for i := range x {
			x[i] = rng.Float32()*4 - 2
		}
		want := dotF16Ref(rowF16, x, nBlocks)
		got := dotF16BlocksAVX2(rowF16, x, nBlocks)
		// tolerância pequena: a soma horizontal do kernel agrupa em ordem
		// diferente da referência sequencial (ponto flutuante não é
		// associativo), diferença esperada é de arredondamento apenas.
		if diff := float32(math.Abs(float64(got - want))); diff > 1e-2*float32(math.Abs(float64(want))+1) {
			t.Fatalf("nBlocks=%d: dotF16BlocksAVX2 = %v, want %v (ref)", nBlocks, got, want)
		}
	}
}

// TestDotF16BlocksAVX2KnownPattern usa valores exatos em ponto flutuante
// (potências de 2, sem perda de precisão possível na conversão F16->F32
// nem na soma) para checar o resultado bit-a-bit, sem tolerância.
func TestDotF16BlocksAVX2KnownPattern(t *testing.T) {
	if !hasF16CFMA {
		t.Skip("CPU sem F16C/FMA")
	}
	// 8 valores F16 = 1.0 (bits 0x3C00), x = [1,2,3,4,5,6,7,8] -> soma = 36
	rowF16 := make([]byte, 16)
	for i := 0; i < 8; i++ {
		rowF16[i*2] = 0x00
		rowF16[i*2+1] = 0x3C
	}
	x := []float32{1, 2, 3, 4, 5, 6, 7, 8}
	got := dotF16BlocksAVX2(rowF16, x, 1)
	want := float32(36)
	if got != want {
		t.Errorf("dotF16BlocksAVX2 = %v, want %v", got, want)
	}
}

func TestDotI2SBlocksAVX2EachByteLane(t *testing.T) {
	if !hasAVX2 {
		t.Skip("CPU sem AVX2")
	}
	q := make([]int8, 128)
	for i := range q {
		q[i] = 1
	}
	for gp := 0; gp < 32; gp++ {
		packed := make([]byte, 32)
		packed[gp] = 0x55 // todos os 4 códigos = 1 nesta posição
		got := dotI2SBlocksAVX2(packed, q, 1)
		want := rawCodeSumRef(packed, q, 1)
		if got != want {
			t.Errorf("gp=%d: got=%d want=%d", gp, got, want)
		}
	}
}
