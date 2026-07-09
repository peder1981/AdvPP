package llm

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sync"
)

// I2SWeight é um tensor de pesos ternários (-1, 0, +1) no formato I2_S do
// llama.cpp/BitNet: cada linha de nCols valores ocupa nCols/4 bytes (2 bits
// por valor, em blocos de 128 valores/32 bytes), com um único scale float32
// por tensor inteiro (não por linha/bloco).
type I2SWeight struct {
	Packed []byte
	Scale  float32
	NRows  int
	NCols  int
}

// LoadI2SWeight lê um tensor I2_S do GGUF pronto para uso em MatMulI2S.
func LoadI2SWeight(g *File, name string) (*I2SWeight, error) {
	t, ok := g.Tensor(name)
	if !ok {
		return nil, fmt.Errorf("llm: tensor %q não encontrado", name)
	}
	if t.Type != GGMLTypeI2S {
		return nil, fmt.Errorf("llm: tensor %q não é I2_S (é %v)", name, t.Type)
	}
	if len(t.Shape) != 2 {
		return nil, fmt.Errorf("llm: tensor %q tem %d dimensões, esperado 2", name, len(t.Shape))
	}

	raw, err := g.TensorData(name)
	if err != nil {
		return nil, err
	}
	nCols := int(t.Shape[0])
	nRows := int(t.Shape[1])
	packedLen := nCols * nRows / 4
	if len(raw) < packedLen+4 {
		return nil, fmt.Errorf("llm: tensor %q menor que o esperado (%d bytes)", name, len(raw))
	}
	scale := math.Float32frombits(binary.LittleEndian.Uint32(raw[packedLen:]))

	return &I2SWeight{Packed: raw[:packedLen], Scale: scale, NRows: nRows, NCols: nCols}, nil
}

// map2bitTable replica a tabela de decodificação de ggml (dequantize_row_i2_s):
// código de 2 bits -> peso ternário. O código 3 é inválido/não usado pelo
// quantizador e mapeia para 0, igual ao ggml de referência. Array em vez de
// switch: sem branch, é indexação direta no hot path do matmul.
var map2bitTable = [4]int32{-1, 0, 1, 0}

// dotI2SRow calcula a soma Σ ternário_i * q_i para a linha `row` de um
// tensor I2_S, replicando byte a byte a mesma varredura de
// dequantize_row_i2_s em ggml-quants.c (blocos de 128 valores / 32 bytes).
// ponytail: o caminho rápido assume blocos cheios (nCols múltiplo de 128,
// verdadeiro para todas as dimensões do Falcon3-1.58bit) e evita os 4
// testes de limite por elemento; um bloco final parcial ainda é tratado à
// parte para não quebrar em modelos com outras dimensões.
func dotI2SRow(packed []byte, row, nCols int, q []int8) int32 {
	rowBytes := nCols / 4
	rowData := packed[row*rowBytes : (row+1)*rowBytes]

	var sumi int32
	done := 0
	byteOff := 0
	for ; done+128 <= nCols; done, byteOff = done+128, byteOff+32 {
		for gp := 0; gp < 32; gp++ {
			b := rowData[byteOff+gp]
			sumi += map2bitTable[(b>>6)&0x3] * int32(q[done+gp])
			sumi += map2bitTable[(b>>4)&0x3] * int32(q[done+32+gp])
			sumi += map2bitTable[(b>>2)&0x3] * int32(q[done+64+gp])
			sumi += map2bitTable[(b>>0)&0x3] * int32(q[done+96+gp])
		}
	}

	if blkE := nCols - done; blkE > 0 {
		cols0 := clampBlock(blkE, 0)
		cols1 := clampBlock(blkE, 32)
		cols2 := clampBlock(blkE, 64)
		cols3 := clampBlock(blkE, 96)
		for gp := 0; gp < 32; gp++ {
			b := rowData[byteOff+gp]
			if gp < cols0 {
				sumi += map2bitTable[(b>>6)&0x3] * int32(q[done+gp])
			}
			if gp < cols1 {
				sumi += map2bitTable[(b>>4)&0x3] * int32(q[done+32+gp])
			}
			if gp < cols2 {
				sumi += map2bitTable[(b>>2)&0x3] * int32(q[done+64+gp])
			}
			if gp < cols3 {
				sumi += map2bitTable[(b>>0)&0x3] * int32(q[done+96+gp])
			}
		}
	}
	return sumi
}

// clampBlock retorna quantas colunas do sub-bloco em `base` (0/32/64/96)
// estão dentro do bloco parcial de tamanho blkE.
func clampBlock(blkE, base int) int {
	n := blkE - base
	if n < 0 {
		return 0
	}
	if n > 32 {
		return 32
	}
	return n
}

// quantizeI8S quantiza um vetor de ativação F32 para int8 por absmax,
// igual a quantize_row_i8_s em ggml-quants.c: scale = 127/max(|x|).
func quantizeI8S(x []float32) (q []int8, scale float32) {
	max := float32(1e-5)
	for _, v := range x {
		av := v
		if av < 0 {
			av = -av
		}
		if av > max {
			max = av
		}
	}
	scale = 127 / max
	q = make([]int8, len(x))
	for i, v := range x {
		qi := int32(math.Round(float64(v * scale)))
		if qi > 127 {
			qi = 127
		}
		if qi < -128 {
			qi = -128
		}
		q[i] = int8(qi)
	}
	return q, scale
}

// MatMulI2S calcula out[r] = dot(w.row(r), x) para todas as nRows linhas de
// um peso ternário, com x em F32. Paralelizado por faixa de linhas — é o
// gargalo do forward pass (chamado 7x por camada); cada linha é
// independente, então o ganho escala quase linear com o número de núcleos.
// ponytail: ainda escalar dentro de cada linha; assembly por arquitetura
// fica pra depois se isso não bastar.
func MatMulI2S(w *I2SWeight, x []float32) []float32 {
	if len(x) != w.NCols {
		panic(fmt.Sprintf("llm: MatMulI2S: len(x)=%d, peso espera NCols=%d", len(x), w.NCols))
	}
	q, actScale := quantizeI8S(x)
	out := make([]float32, w.NRows)
	parallelRows(w.NRows, func(r0, r1 int) {
		for r := r0; r < r1; r++ {
			sumi := dotI2SRow(w.Packed, r, w.NCols, q)
			out[r] = float32(sumi) / actScale * w.Scale
		}
	})
	return out
}

// parallelRows divide [0,nRows) em até runtime.NumCPU() faixas contíguas e
// roda fn(r0,r1) para cada uma em uma goroutine, aguardando todas.
func parallelRows(nRows int, fn func(r0, r1 int)) {
	nWorkers := runtime.GOMAXPROCS(0)
	if nWorkers > nRows {
		nWorkers = nRows
	}
	if nWorkers <= 1 {
		fn(0, nRows)
		return
	}
	chunk := (nRows + nWorkers - 1) / nWorkers
	var wg sync.WaitGroup
	for r0 := 0; r0 < nRows; r0 += chunk {
		r1 := r0 + chunk
		if r1 > nRows {
			r1 = nRows
		}
		wg.Add(1)
		go func(r0, r1 int) {
			defer wg.Done()
			fn(r0, r1)
		}(r0, r1)
	}
	wg.Wait()
}
