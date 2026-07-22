// Package nn fornece módulos de rede neural (camadas com parâmetros próprios e
// um Forward) sobre o autograd — a base para definir modelos de forma concisa.
package nn

import (
	"github.com/advpl/compiler/pkg/autograd"
	"github.com/advpl/compiler/pkg/tensor"
)

// Linear: camada densa y = x·W + b.
type Linear struct {
	W, B *autograd.Variable
}

func NewLinear(nIn, nOut int, scale float32) *Linear {
	return &Linear{
		W: autograd.NewLeaf(tensor.Rand([]int{nIn, nOut}, scale)),
		B: autograd.NewLeaf(tensor.New([]int{nOut})),
	}
}

func (l *Linear) Forward(x *autograd.Variable) (*autograd.Variable, error) {
	h, err := x.MatMul(l.W)
	if err != nil {
		return nil, err
	}
	return h.Add(l.B)
}

func (l *Linear) Params() []*autograd.Variable {
	return []*autograd.Variable{l.W, l.B}
}

// Embedding: tabela de embeddings; Forward colhe as linhas dos índices.
type Embedding struct {
	Table *autograd.Variable
}

func NewEmbedding(nVocab, nDim int, scale float32) *Embedding {
	return &Embedding{Table: autograd.NewLeaf(tensor.Rand([]int{nVocab, nDim}, scale))}
}

func (e *Embedding) Forward(idx []int) (*autograd.Variable, error) {
	return e.Table.IndexRows(idx)
}

func (e *Embedding) Params() []*autograd.Variable {
	return []*autograd.Variable{e.Table}
}
