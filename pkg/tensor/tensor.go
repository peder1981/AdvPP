// Package tensor fornece um tensor float32 denso (row-major) com kernels de
// forward em Go puro — a base numérica da classe AdvPL `Tensor`.
package tensor

import (
	"fmt"
	"math/rand"
)

type Tensor struct {
	Shape []int
	Data  []float32
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

// Size é o número total de elementos.
func (t *Tensor) Size() int { return len(t.Data) }

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

// At lê um elemento (idx 0-based).
func (t *Tensor) At(idx []int) (float32, error) {
	off, err := t.Offset(idx)
	if err != nil {
		return 0, err
	}
	return t.Data[off], nil
}

// SetAt grava um elemento (idx 0-based).
func (t *Tensor) SetAt(idx []int, val float32) error {
	off, err := t.Offset(idx)
	if err != nil {
		return err
	}
	t.Data[off] = val
	return nil
}
