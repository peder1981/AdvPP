package llm

import (
	"encoding/binary"
	"math"
)

// Float16ToFloat32 converte um half-float IEEE 754 (armazenado como uint16)
// para float32. Implementação portável (sem instruções de CPU específicas),
// necessária para permanecer 100% compatível entre linux/windows/macOS.
func Float16ToFloat32(h uint16) float32 {
	sign := uint32(h&0x8000) << 16
	rawExp := int32(h&0x7C00) >> 10
	mant := int32(h & 0x03FF)

	if rawExp == 0x1F {
		if mant == 0 {
			return math.Float32frombits(sign | 0x7F800000) // +/-Inf
		}
		return math.Float32frombits(sign | 0x7F800000 | (uint32(mant) << 13)) // NaN
	}

	exp := rawExp
	if rawExp == 0 {
		if mant == 0 {
			return math.Float32frombits(sign) // zero
		}
		// subnormal: normaliza deslocando a mantissa até o bit implícito
		exp = 1
		for mant&0x0400 == 0 {
			mant <<= 1
			exp--
		}
		mant &= 0x03FF
	}

	exp32 := uint32(exp - 15 + 127)
	bits := sign | (exp32 << 23) | (uint32(mant) << 13)
	return math.Float32frombits(bits)
}

// DecodeF16Row converte n valores F16 little-endian consecutivos em []float32.
func DecodeF16Row(raw []byte, n int) []float32 {
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = Float16ToFloat32(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return out
}

// EmbedRow lê e decodifica a linha `row` de uma tabela [nRows, rowDim]
// armazenada em F16 (token_embd.weight ou output.weight), sem carregar o
// tensor inteiro em memória.
func EmbedRow(g *File, tensorName string, row, rowDim int) ([]float32, error) {
	const f16Size = 2
	rowBytes := uint64(rowDim * f16Size)
	raw, err := g.TensorRange(tensorName, uint64(row)*rowBytes, rowBytes)
	if err != nil {
		return nil, err
	}
	return DecodeF16Row(raw, rowDim), nil
}

// MatMulF16 calcula logits[v] = dot(x, row_v) para cada uma das nRows linhas
// de um peso [nRows, nIn] armazenado em F16 (usado pela projeção de saída).
// Paralelizado por faixa de linhas — nRows=vocab_size (131072), o maior
// matmul do forward pass.
func MatMulF16(weightF16 []byte, nRows, nIn int, x []float32) []float32 {
	out := make([]float32, nRows)
	rowBytes := nIn * 2
	parallelRows(nRows, func(r0, r1 int) {
		for r := r0; r < r1; r++ {
			row := weightF16[r*rowBytes : (r+1)*rowBytes]
			var sum float32
			for i := 0; i < nIn; i++ {
				sum += Float16ToFloat32(binary.LittleEndian.Uint16(row[i*2:])) * x[i]
			}
			out[r] = sum
		}
	})
	return out
}

// RMSNorm aplica normalização RMS: y = x / rms(x) * weight.
func RMSNorm(x, weight []float32, eps float32) []float32 {
	var ss float32
	for _, v := range x {
		ss += v * v
	}
	scale := float32(1.0 / math.Sqrt(float64(ss)/float64(len(x))+float64(eps)))
	out := make([]float32, len(x))
	for i, v := range x {
		out[i] = v * scale * weight[i]
	}
	return out
}

// RoPE aplica rotary position embedding "NORM" (pares consecutivos
// x[2i],x[2i+1]) a um vetor de tamanho headDim, arquitetura LLM_ARCH_LLAMA
// no llama.cpp — é o RoPE usado pelo Falcon3-1.58bit (arch=llama no GGUF).
func RoPE(x []float32, headDim int, pos int, ropeDims int, freqBase float32) {
	for i := 0; i < ropeDims/2; i++ {
		freq := 1.0 / math.Pow(float64(freqBase), float64(2*i)/float64(ropeDims))
		theta := float64(pos) * freq
		cosT, sinT := float32(math.Cos(theta)), float32(math.Sin(theta))
		x0, x1 := x[2*i], x[2*i+1]
		x[2*i] = x0*cosT - x1*sinT
		x[2*i+1] = x0*sinT + x1*cosT
	}
}

// Softmax normaliza x em uma distribuição de probabilidade (in-place).
func Softmax(x []float32) {
	max := x[0]
	for _, v := range x[1:] {
		if v > max {
			max = v
		}
	}
	var sum float32
	for i, v := range x {
		e := float32(math.Exp(float64(v - max)))
		x[i] = e
		sum += e
	}
	for i := range x {
		x[i] /= sum
	}
}

// SiLU é a ativação sigmoid-linear-unit: x * sigmoid(x).
func SiLU(x float32) float32 {
	return x / (1 + float32(math.Exp(float64(-x))))
}

// SwiGLU calcula silu(gate) * up elemento a elemento (ativação da FFN estilo
// LLaMA), sobrescrevendo gate com o resultado.
func SwiGLU(gate, up []float32) {
	for i := range gate {
		gate[i] = SiLU(gate[i]) * up[i]
	}
}

// AddInPlace soma b em a (residual connections).
func AddInPlace(a, b []float32) {
	for i := range a {
		a[i] += b[i]
	}
}
