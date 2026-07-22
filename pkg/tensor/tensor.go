// Package tensor fornece um tensor float32 denso (row-major) com kernels de
// forward em Go puro — a base numérica da classe AdvPL `Tensor`.
package tensor

import (
	"fmt"
	"math/rand"
)

// DType é a precisão de armazenamento de um Tensor.
type DType int

const (
	Float32 DType = iota // default (zero-value): usa Data []float32
	Float64              // usa Data64 []float64
)

func (d DType) String() string {
	if d == Float64 {
		return "float64"
	}
	return "float32"
}

// Tensor é um tensor denso row-major. Por padrão float32 (Data); quando
// DType==Float64, os elementos ficam em Data64. O caminho float32 é o default e
// permanece intocado (o ML usa float32); o float64 é selecionável sob demanda.
type Tensor struct {
	Shape  []int
	Data   []float32 // usado quando DType==Float32
	Data64 []float64 // usado quando DType==Float64
	DType  DType
}

// Prod devolve o número de elementos de uma forma.
func Prod(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// ShapeEq diz se duas formas são idênticas.
func ShapeEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func copyInts(s []int) []int { return append([]int(nil), s...) }

// New cria um tensor de zeros com a forma dada.
func New(shape []int) *Tensor {
	return &Tensor{Shape: copyInts(shape), Data: make([]float32, Prod(shape))}
}

// FromData cria um tensor a partir de dados row-major + forma.
func FromData(data []float32, shape []int) (*Tensor, error) {
	if Prod(shape) != len(data) {
		return nil, fmt.Errorf("FromData: len(data)=%d != produto(shape=%v)=%d", len(data), shape, Prod(shape))
	}
	return &Tensor{Shape: copyInts(shape), Data: append([]float32(nil), data...)}, nil
}

// Rand cria um tensor uniforme em [-scale, scale].
func Rand(shape []int, scale float32) *Tensor {
	t := New(shape)
	for i := range t.Data {
		t.Data[i] = (rand.Float32()*2 - 1) * scale
	}
	return t
}

// NewDType cria um tensor de zeros com a forma e o dtype dados.
func NewDType(shape []int, dt DType) *Tensor {
	if dt == Float64 {
		return &Tensor{Shape: copyInts(shape), Data64: make([]float64, Prod(shape)), DType: Float64}
	}
	return New(shape)
}

// FromData64 cria um tensor float64 a partir de dados row-major + forma.
func FromData64(data []float64, shape []int) (*Tensor, error) {
	if Prod(shape) != len(data) {
		return nil, fmt.Errorf("FromData64: len(data)=%d != produto(shape=%v)=%d", len(data), shape, Prod(shape))
	}
	return &Tensor{Shape: copyInts(shape), Data64: append([]float64(nil), data...), DType: Float64}, nil
}

// RandDType cria um tensor uniforme em [-scale, scale] no dtype dado.
func RandDType(shape []int, scale float32, dt DType) *Tensor {
	if dt != Float64 {
		return Rand(shape, scale)
	}
	t := NewDType(shape, Float64)
	for i := range t.Data64 {
		t.Data64[i] = (rand.Float64()*2 - 1) * float64(scale)
	}
	return t
}

// Size é o número total de elementos (do slice ativo conforme o dtype).
func (t *Tensor) Size() int {
	if t.DType == Float64 {
		return len(t.Data64)
	}
	return len(t.Data)
}

// Get lê o elemento no offset i (0-based, row-major) como float64, seja qual for o dtype.
func (t *Tensor) Get(i int) float64 {
	if t.DType == Float64 {
		return t.Data64[i]
	}
	return float64(t.Data[i])
}

// Set grava v no offset i, convertendo para o dtype do tensor.
func (t *Tensor) Set(i int, v float64) {
	if t.DType == Float64 {
		t.Data64[i] = v
	} else {
		t.Data[i] = float32(v)
	}
}

// SameDType devolve Float64 se qualquer um dos dois for Float64 (promoção).
func SameDType(a, b *Tensor) DType {
	if a.DType == Float64 || b.DType == Float64 {
		return Float64
	}
	return Float32
}

// AsDType devolve uma cópia do tensor no dtype pedido (ou ele mesmo, se já for).
func (t *Tensor) AsDType(dt DType) *Tensor {
	if t.DType == dt {
		return t
	}
	out := NewDType(t.Shape, dt)
	for i := 0; i < t.Size(); i++ {
		out.Set(i, t.Get(i))
	}
	return out
}

// Offset converte um índice multi-dim (0-based) no offset row-major.
func (t *Tensor) Offset(idx []int) (int, error) {
	if len(idx) != len(t.Shape) {
		return 0, fmt.Errorf("Offset: idx com %d dims, tensor tem %d", len(idx), len(t.Shape))
	}
	off := 0
	for i, ix := range idx {
		if ix < 0 || ix >= t.Shape[i] {
			return 0, fmt.Errorf("Offset: índice %d fora de faixa na dim %d (0..%d)", ix, i, t.Shape[i]-1)
		}
		off = off*t.Shape[i] + ix
	}
	return off, nil
}

// At lê um elemento (idx 0-based) como float32 (compat.; use Get para f64 exato).
func (t *Tensor) At(idx []int) (float32, error) {
	off, err := t.Offset(idx)
	if err != nil {
		return 0, err
	}
	return float32(t.Get(off)), nil
}

// SetAt grava um elemento (idx 0-based).
func (t *Tensor) SetAt(idx []int, val float32) error {
	off, err := t.Offset(idx)
	if err != nil {
		return err
	}
	t.Set(off, float64(val))
	return nil
}
