package nn

import (
	"testing"

	"github.com/advpl/compiler/pkg/autograd"
	"github.com/advpl/compiler/pkg/tensor"
)

func leaf(data []float32, shape []int) *autograd.Variable {
	t, err := tensor.FromData(data, shape)
	if err != nil {
		panic(err)
	}
	return autograd.NewLeaf(t)
}

func TestLinearForwardAndParams(t *testing.T) {
	l := NewLinear(3, 2, 0.1)
	ps := l.Params()
	if len(ps) != 2 {
		t.Fatalf("Params len = %d, quer 2", len(ps))
	}
	x := leaf([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}) // [2,3]
	y, err := l.Forward(x)
	if err != nil {
		t.Fatal(err)
	}
	if !tensor.ShapeEq(y.Value.Shape, []int{2, 2}) {
		t.Fatalf("Forward shape = %v, quer [2 2]", y.Value.Shape)
	}
	if err := y.Sum().Backward(); err != nil {
		t.Fatal(err)
	}
	if l.W.Grad == nil || l.B.Grad == nil {
		t.Fatal("Backward deve preencher grads de W e b")
	}
	if !tensor.ShapeEq(l.W.Grad.Shape, []int{3, 2}) {
		t.Fatalf("grad de W = %v, quer [3 2]", l.W.Grad.Shape)
	}
}

func TestEmbeddingForward(t *testing.T) {
	e := NewEmbedding(4, 3, 0.1)
	if len(e.Params()) != 1 {
		t.Fatal("Embedding deve ter 1 param (a tabela)")
	}
	y, err := e.Forward([]int{2, 0, 2}) // 0-based
	if err != nil {
		t.Fatal(err)
	}
	if !tensor.ShapeEq(y.Value.Shape, []int{3, 3}) {
		t.Fatalf("Embedding Forward shape = %v, quer [3 3]", y.Value.Shape)
	}
	if err := y.Sum().Backward(); err != nil {
		t.Fatal(err)
	}
	if e.Table.Grad == nil {
		t.Fatal("Backward deve preencher grad da tabela")
	}
}
