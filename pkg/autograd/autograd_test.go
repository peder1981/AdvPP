package autograd

import (
	"testing"

	"github.com/advpl/compiler/pkg/tensor"
)

func mustT(data []float32, shape []int) *tensor.Tensor {
	t, err := tensor.FromData(data, shape)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReduceGradTo(t *testing.T) {
	g := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	row := reduceGradTo(g, []int{3}) // soma eixo 0 -> [5,7,9]
	if row.Data[0] != 5 || row.Data[2] != 9 {
		t.Fatalf("row: %v", row.Data)
	}
	col := reduceGradTo(g, []int{2, 1}) // soma eixo 1 -> [6,15]
	if col.Data[0] != 6 || col.Data[1] != 15 {
		t.Fatalf("col: %v", col.Data)
	}
	sc := reduceGradTo(g, []int{1}) // soma tudo -> 21
	if sc.Data[0] != 21 {
		t.Fatalf("scalar: %v", sc.Data)
	}
}

func TestBackwardManualAndAccumulate(t *testing.T) {
	// x é folha; y = 2*x e z = 3*x (backward manual); l = y + z (soma escalar manual).
	// dl/dx = 2 + 3 = 5. Verifica ordem topológica + acúmulo de grad em nó reusado.
	x := NewLeaf(mustT([]float32{4}, []int{1}))
	y := &Variable{Value: mustT([]float32{8}, []int{1}), parents: []*Variable{x}}
	y.backward = func() { addGrad(x, y.Grad.MulScalar(2)) }
	z := &Variable{Value: mustT([]float32{12}, []int{1}), parents: []*Variable{x}}
	z.backward = func() { addGrad(x, z.Grad.MulScalar(3)) }
	l := &Variable{Value: mustT([]float32{20}, []int{1}), parents: []*Variable{y, z}}
	l.backward = func() {
		addGrad(y, l.Grad)
		addGrad(z, l.Grad)
	}
	l.Backward()
	if x.Grad == nil || x.Grad.Data[0] != 5 {
		t.Fatalf("x.Grad esperado 5, veio %v", x.Grad)
	}
}
