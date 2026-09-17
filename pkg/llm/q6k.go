package llm

import (
	"encoding/binary"
	"fmt"
)

// Q6KWeight é um tensor de pesos no formato Q6_K do llama.cpp/GGUF:
// super-blocos de 256 valores, cada um com 128 bytes de bits baixos (4
// bits), 64 bytes de bits altos (2 bits), 16 bytes de escala int8 por
// sub-bloco de 16 e uma escala F16 por super-bloco. Layout e fórmula
// replicados byte a byte de dequantize_row_q6_K em ggml-quants.c
// (llama.cpp), verificados contra o código-fonte real (~/llama.cpp) nesta
// sessão — não é uma reconstrução de memória. Usado hoje só pela projeção
// de saída (output.weight) de modelos Q4_K_M/Q6_K, que costuma sair em
// precisão maior que as camadas internas.
type Q6KWeight struct {
	Data  []byte
	NRows int
	NCols int // deve ser múltiplo de 256 (QK_K)
}

const q6kBlockBytes = 256/2 + 256/4 + 256/16 + 2 // ql + qh + scales + d(F16) = 210

// LoadQ6KWeight lê um tensor Q6_K do GGUF pronto para uso em MatMul.
func LoadQ6KWeight(g *File, name string) (*Q6KWeight, error) {
	t, ok := g.Tensor(name)
	if !ok {
		return nil, fmt.Errorf("llm: tensor %q não encontrado", name)
	}
	if t.Type != GGMLTypeQ6_K {
		return nil, fmt.Errorf("llm: tensor %q não é Q6_K (é %v)", name, t.Type)
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
	rowBytes := blocksPerRow * q6kBlockBytes
	if len(raw) < rowBytes*nRows {
		return nil, fmt.Errorf("llm: tensor %q menor que o esperado (%d bytes)", name, len(raw))
	}
	return &Q6KWeight{Data: raw, NRows: nRows, NCols: nCols}, nil
}

// dequantQ6KBlock decodifica um super-bloco de 210 bytes (ql[128]+qh[64]+
// scales[16]+d) em 256 float32 — réplica de dequantize_row_q6_K para um
// único bloco.
func dequantQ6KBlock(block []byte, out []float32) {
	ql := block[0:128]
	qh := block[128:192]
	sc := block[192:208]
	d := Float16ToFloat32(binary.LittleEndian.Uint16(block[208:210]))

	for n := 0; n < 256; n += 128 {
		qlN := ql[n/2 : n/2+64]
		qhN := qh[n/4 : n/4+32]
		scN := sc[n/16 : n/16+8]
		for l := 0; l < 32; l++ {
			is := l / 16
			q1 := int8((qlN[l]&0xF)|(((qhN[l]>>0)&3)<<4)) - 32
			q2 := int8((qlN[l+32]&0xF)|(((qhN[l]>>2)&3)<<4)) - 32
			q3 := int8((qlN[l]>>4)|(((qhN[l]>>4)&3)<<4)) - 32
			q4 := int8((qlN[l+32]>>4)|(((qhN[l]>>6)&3)<<4)) - 32
			// scN é int8_t no formato real (escala com sinal) — a
			// conversão via int8() antes de float32() é obrigatória,
			// senão um byte >127 seria lido como escala positiva grande
			// em vez de negativa.
			out[n+l+0] = d * float32(int8(scN[is+0])) * float32(q1)
			out[n+l+32] = d * float32(int8(scN[is+2])) * float32(q2)
			out[n+l+64] = d * float32(int8(scN[is+4])) * float32(q3)
			out[n+l+96] = d * float32(int8(scN[is+6])) * float32(q4)
		}
	}
}

// MatMul calcula out[r] = dot(dequant(w.row(r)), x), dequantizando um
// super-bloco por vez num buffer reaproveitado — mesmo espírito de custo
// de memória do MatMulI2S/Q4KWeight.MatMul.
func (w *Q6KWeight) MatMul(x []float32) []float32 {
	if len(x) != w.NCols {
		panic(fmt.Sprintf("llm: Q6KWeight.MatMul: len(x)=%d, peso espera NCols=%d", len(x), w.NCols))
	}
	blocksPerRow := w.NCols / q4kSuperBlock
	rowBytes := blocksPerRow * q6kBlockBytes
	out := make([]float32, w.NRows)
	parallelRows(w.NRows, func(r0, r1 int) {
		var buf [q4kSuperBlock]float32
		for r := r0; r < r1; r++ {
			row := w.Data[r*rowBytes : (r+1)*rowBytes]
			var sum float32
			for b := 0; b < blocksPerRow; b++ {
				block := row[b*q6kBlockBytes : (b+1)*q6kBlockBytes]
				dequantQ6KBlock(block, buf[:])
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
