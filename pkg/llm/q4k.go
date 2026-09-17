package llm

import (
	"encoding/binary"
	"fmt"
)

// Q4KWeight é um tensor de pesos no formato Q4_K do llama.cpp/GGUF:
// super-blocos de 256 valores, cada um com duas escalas F16 (d, dmin) +
// 12 bytes de escalas/offsets de 6 bits por sub-bloco de 32 + 128 bytes de
// quants de 4 bits. Layout e fórmula replicados byte a byte de
// dequantize_row_q4_K/get_scale_min_k4 em ggml-quants.c (llama.cpp),
// verificados contra o código-fonte real (~/llama.cpp) nesta sessão — não
// é uma reconstrução de memória.
type Q4KWeight struct {
	Data  []byte
	NRows int
	NCols int // deve ser múltiplo de 256 (QK_K)
}

const (
	q4kSuperBlock = 256
	q4kBlockBytes = 144 // 2+2 (d,dmin F16) + 12 (scales) + 128 (qs)
)

// LoadQ4KWeight lê um tensor Q4_K do GGUF pronto para uso em MatMul.
func LoadQ4KWeight(g *File, name string) (*Q4KWeight, error) {
	t, ok := g.Tensor(name)
	if !ok {
		return nil, fmt.Errorf("llm: tensor %q não encontrado", name)
	}
	if t.Type != GGMLTypeQ4_K {
		return nil, fmt.Errorf("llm: tensor %q não é Q4_K (é %v)", name, t.Type)
	}
	if len(t.Shape) != 2 {
		return nil, fmt.Errorf("llm: tensor %q tem %d dimensões, esperado 2", name, len(t.Shape))
	}
	nCols := int(t.Shape[0])
	nRows := int(t.Shape[1])
	if nCols%q4kSuperBlock != 0 {
		return nil, fmt.Errorf("llm: tensor %q: nCols=%d não é múltiplo de %d", name, nCols, q4kSuperBlock)
	}
	raw, err := g.TensorData(name)
	if err != nil {
		return nil, err
	}
	blocksPerRow := nCols / q4kSuperBlock
	rowBytes := blocksPerRow * q4kBlockBytes
	if len(raw) < rowBytes*nRows {
		return nil, fmt.Errorf("llm: tensor %q menor que o esperado (%d bytes)", name, len(raw))
	}
	return &Q4KWeight{Data: raw, NRows: nRows, NCols: nCols}, nil
}

// getScaleMinK4 desempacota o par (escala,mínimo) de 6 bits do sub-bloco
// `j` (0..7) a partir dos 12 bytes de `scales` — réplica bit a bit de
// get_scale_min_k4 em ggml-quants.c.
func getScaleMinK4(j int, scales []byte) (sc, m uint8) {
	if j < 4 {
		return scales[j] & 63, scales[j+4] & 63
	}
	sc = (scales[j+4] & 0xF) | ((scales[j-4] >> 6) << 4)
	m = (scales[j+4] >> 4) | ((scales[j] >> 6) << 4)
	return sc, m
}

// dequantQ4KBlock decodifica um super-bloco de 144 bytes em 256 float32,
// gravando em out (deve ter len(out)>=256) — réplica de
// dequantize_row_q4_K para um único bloco.
func dequantQ4KBlock(block []byte, out []float32) {
	d := Float16ToFloat32(binary.LittleEndian.Uint16(block[0:2]))
	dmin := Float16ToFloat32(binary.LittleEndian.Uint16(block[2:4]))
	scales := block[4:16]
	qs := block[16:144]

	is := 0
	qOff := 0
	oOff := 0
	for j := 0; j < q4kSuperBlock; j += 64 {
		sc1, m1 := getScaleMinK4(is, scales)
		sc2, m2 := getScaleMinK4(is+1, scales)
		d1, mm1 := d*float32(sc1), dmin*float32(m1)
		d2, mm2 := d*float32(sc2), dmin*float32(m2)
		for l := 0; l < 32; l++ {
			out[oOff+l] = d1*float32(qs[qOff+l]&0xF) - mm1
		}
		for l := 0; l < 32; l++ {
			out[oOff+32+l] = d2*float32(qs[qOff+l]>>4) - mm2
		}
		qOff += 32
		oOff += 64
		is += 2
	}
}

// MatMul calcula out[r] = dot(dequant(w.row(r)), x). Dequantiza um
// super-bloco de 256 valores por vez num buffer reaproveitado (nunca
// materializa a linha inteira nem o tensor inteiro em F32) e acumula o
// produto escalar contra a faixa correspondente de x — mesmo espírito de
// custo de memória do MatMulI2S.
func (w *Q4KWeight) MatMul(x []float32) []float32 {
	if len(x) != w.NCols {
		panic(fmt.Sprintf("llm: Q4KWeight.MatMul: len(x)=%d, peso espera NCols=%d", len(x), w.NCols))
	}
	blocksPerRow := w.NCols / q4kSuperBlock
	rowBytes := blocksPerRow * q4kBlockBytes
	out := make([]float32, w.NRows)
	parallelRows(w.NRows, func(r0, r1 int) {
		var buf [q4kSuperBlock]float32
		for r := r0; r < r1; r++ {
			row := w.Data[r*rowBytes : (r+1)*rowBytes]
			var sum float32
			for b := 0; b < blocksPerRow; b++ {
				block := row[b*q4kBlockBytes : (b+1)*q4kBlockBytes]
				dequantQ4KBlock(block, buf[:])
				xs := x[b*q4kSuperBlock : (b+1)*q4kSuperBlock]
				for i := 0; i < q4kSuperBlock; i++ {
					sum += buf[i] * xs[i]
				}
			}
			out[r] = sum
		}
	})
	return out
}
