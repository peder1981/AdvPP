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

func close32(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	m := float64(a)
	if m < 0 {
		m = -m
	}
	m2 := float64(b)
	if m2 < 0 {
		m2 = -m2
	}
	return float64(d) <= 1e-2+5e-2*(m+m2)/2
}

// gradCheck compara o grad analítico (Backward) de f em x com diferenças finitas.
// f constrói o grafo a partir de uma Variable e devolve a loss escalar {1}.
func gradCheck(t *testing.T, name string, x *tensor.Tensor, f func(*Variable) *Variable) {
	xv := NewLeaf(mustT(x.Data, x.Shape))
	f(xv).Backward()
	analytic := xv.Grad
	const eps = 1e-2
	for i := range x.Data {
		orig := x.Data[i]
		xp := mustT(x.Data, x.Shape)
		xp.Data[i] = orig + eps
		xm := mustT(x.Data, x.Shape)
		xm.Data[i] = orig - eps
		lp := f(NewLeaf(xp)).Value.Data[0]
		lm := f(NewLeaf(xm)).Value.Data[0]
		num := (lp - lm) / (2 * eps)
		if !close32(analytic.Data[i], num) {
			t.Fatalf("%s grad[%d]: analitico=%v numerico=%v", name, i, analytic.Data[i], num)
		}
	}
}

func TestGradMatMul(t *testing.T) {
	W := mustT([]float32{0.5, -0.3, 0.2, 0.1, 0.4, -0.6}, []int{3, 2}) // [3,2]
	// f(A) = sum(A[2,3] · W[3,2]); checa grad em A
	gradCheck(t, "matmul-A", mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}), func(a *Variable) *Variable {
		y, err := a.MatMul(NewLeaf(W))
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	A := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}) // [2,3]
	// f(B) = sum(A · B[3,2]); checa grad em B
	gradCheck(t, "matmul-B", mustT([]float32{0.5, -0.3, 0.2, 0.1, 0.4, -0.6}, []int{3, 2}), func(b *Variable) *Variable {
		y, err := NewLeaf(A).MatMul(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
}

func TestGradAddBroadcast(t *testing.T) {
	base := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	// bias linha [3]: f(b) = sum(base + b)
	gradCheck(t, "add-row", mustT([]float32{0.1, 0.2, 0.3}, []int{3}), func(b *Variable) *Variable {
		y, err := NewLeaf(base).Add(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	// bias coluna [2,1]
	gradCheck(t, "add-col", mustT([]float32{0.5, 0.7}, []int{2, 1}), func(b *Variable) *Variable {
		y, err := NewLeaf(base).Add(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
}

func TestGradMulReluSumMeanMSE(t *testing.T) {
	B := mustT([]float32{2, -1, 0.5, 3}, []int{2, 2})
	gradCheck(t, "mul", mustT([]float32{1, 2, 3, 4}, []int{2, 2}), func(x *Variable) *Variable {
		y, err := x.Mul(NewLeaf(B))
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	// Relu: valores longe de 0 (não-diferenciável só em 0)
	gradCheck(t, "relu", mustT([]float32{-2, 1, -0.5, 3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Relu().Sum()
	})
	gradCheck(t, "sum", mustT([]float32{1, 2, 3}, []int{3}), func(x *Variable) *Variable {
		return x.Sum()
	})
	gradCheck(t, "mean", mustT([]float32{1, 2, 3, 4}, []int{4}), func(x *Variable) *Variable {
		return x.Mean()
	})
	target := mustT([]float32{1, 0, 2, 1}, []int{2, 2})
	gradCheck(t, "mse", mustT([]float32{1.5, 0.2, 1.8, 0.9}, []int{2, 2}), func(x *Variable) *Variable {
		l, err := x.MSE(NewLeaf(target))
		if err != nil {
			panic(err)
		}
		return l
	})
}

func TestGradActivations(t *testing.T) {
	gradCheck(t, "tanh", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Tanh().Sum()
	})
	gradCheck(t, "sigmoid", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Sigmoid().Sum()
	})
	gradCheck(t, "gelu", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Gelu().Sum()
	})
}

func TestSGDStepReducesLoss(t *testing.T) {
	// loss = sum(p*p) = Σp²; grad = 2p; um passo com lr=0.1 encolhe p e reduz a loss.
	p := NewLeaf(mustT([]float32{3, -4}, []int{2}))
	lossBefore := func() float32 {
		sq, _ := p.Value.Mul(p.Value)
		return sq.SumAll()
	}
	before := lossBefore()

	opt := NewSGD([]*Variable{p}, 0.1)
	opt.ZeroGrad()
	l, _ := p.Mul(p)
	l.Sum().Backward()
	opt.Step()

	after := lossBefore()
	if !(after < before) {
		t.Fatalf("SGD nao reduziu a loss: antes=%v depois=%v", before, after)
	}
	// grad esperado 2p = [6,-8]; passo p -= 0.1*grad => [2.4,-3.2]
	if !close32(p.Value.Data[0], 2.4) || !close32(p.Value.Data[1], -3.2) {
		t.Fatalf("passo do SGD errado: %v", p.Value.Data)
	}
}

func TestZeroGrad(t *testing.T) {
	p := NewLeaf(mustT([]float32{1, 2}, []int{2}))
	p.Sum().Backward()
	if p.Grad == nil {
		t.Fatal("esperava grad apos backward")
	}
	NewSGD([]*Variable{p}, 0.1).ZeroGrad()
	if p.Grad != nil {
		t.Fatal("ZeroGrad deveria limpar o grad")
	}
}
