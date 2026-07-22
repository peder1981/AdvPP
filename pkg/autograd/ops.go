package autograd

import (
	"fmt"
	"math"

	"github.com/advpl/compiler/pkg/tensor"
)

// MatMul: Y = A·B (2D x 2D). dA = dY·Bᵀ; dB = Aᵀ·dY.
func (a *Variable) MatMul(b *Variable) (*Variable, error) {
	if len(a.Value.Shape) != 2 || len(b.Value.Shape) != 2 {
		return nil, fmt.Errorf("Variable.MatMul: requer 2D x 2D")
	}
	y, err := a.Value.MatMul(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		bt, _ := b.Value.Transpose()
		da, _ := out.Grad.MatMul(bt)
		addGrad(a, da)
		at, _ := a.Value.Transpose()
		db, _ := at.MatMul(out.Grad)
		addGrad(b, db)
	}
	return out, nil
}

// Add com broadcast (do pkg/tensor). dA/dB = reduceGradTo(dY, forma).
func (a *Variable) Add(b *Variable) (*Variable, error) {
	y, err := a.Value.Add(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		addGrad(a, reduceGradTo(out.Grad, a.Value.Shape))
		addGrad(b, reduceGradTo(out.Grad, b.Value.Shape))
	}
	return out, nil
}

// Mul (Hadamard, mesma forma). dA = dY⊙B; dB = dY⊙A.
func (a *Variable) Mul(b *Variable) (*Variable, error) {
	y, err := a.Value.Mul(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		da, _ := out.Grad.Mul(b.Value)
		addGrad(a, da)
		db, _ := out.Grad.Mul(a.Value)
		addGrad(b, db)
	}
	return out, nil
}

// Relu. dA = dY ⊙ (A>0).
func (a *Variable) Relu() *Variable {
	y := a.Value.Relu()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		mask := tensor.New(a.Value.Shape)
		for i, v := range a.Value.Data {
			if v > 0 {
				mask.Data[i] = 1
			}
		}
		dg, _ := out.Grad.Mul(mask)
		addGrad(a, dg)
	}
	return out
}

// Sum (todos os elementos) -> escalar {1}. dA = broadcast(dY).
func (a *Variable) Sum() *Variable {
	y, _ := tensor.FromData([]float32{a.Value.SumAll()}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0]
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = g
		}
		addGrad(a, dg)
	}
	return out
}

// Mean -> escalar {1}. dA = broadcast(dY / N).
func (a *Variable) Mean() *Variable {
	n := float32(a.Value.Size())
	y, _ := tensor.FromData([]float32{a.Value.MeanAll()}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0] / n
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = g
		}
		addGrad(a, dg)
	}
	return out
}

// Tanh. dA = dY ⊙ (1 - tanh(A)²).
func (a *Variable) Tanh() *Variable {
	y := a.Value.Tanh()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		d := tensor.New(y.Shape)
		for i, yv := range y.Data {
			d.Data[i] = 1 - yv*yv
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}

// Sigmoid. dA = dY ⊙ σ(1-σ).
func (a *Variable) Sigmoid() *Variable {
	y := a.Value.Sigmoid()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		d := tensor.New(y.Shape)
		for i, yv := range y.Data {
			d.Data[i] = yv * (1 - yv)
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}

// Gelu (aproximação tanh). dA = dY ⊙ gelu'(A).
func (a *Variable) Gelu() *Variable {
	y := a.Value.Gelu()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		const c = 0.7978845608
		d := tensor.New(a.Value.Shape)
		for i, xv := range a.Value.Data {
			x := float64(xv)
			u := c * (x + 0.044715*x*x*x)
			tv := math.Tanh(u)
			dg := 0.5*(1+tv) + 0.5*x*(1-tv*tv)*c*(1+3*0.044715*x*x)
			d.Data[i] = float32(dg)
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}

// IndexRows: colhe linhas de A[R,C] nos índices idx (0-based) -> [K,C].
// backward (scatter-add): dA[idx[k],:] += dY[k,:].
func (a *Variable) IndexRows(idx []int) (*Variable, error) {
	y, err := a.Value.IndexRows(idx)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		c := a.Value.Shape[1]
		dA := tensor.New(a.Value.Shape)
		for k, r := range idx {
			for j := 0; j < c; j++ {
				dA.Data[r*c+j] += out.Grad.Data[k*c+j]
			}
		}
		addGrad(a, dA)
	}
	return out, nil
}

// Reshape muda a forma preservando os dados e a ordem. Backward reshapa o
// gradiente de volta à forma original (a correspondência é 1:1 elemento a elemento).
func (a *Variable) Reshape(shape []int) (*Variable, error) {
	y, err := a.Value.Reshape(shape)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		dg, err := out.Grad.Reshape(a.Value.Shape)
		if err != nil {
			return
		}
		addGrad(a, dg)
	}
	return out, nil
}

// SoftmaxCE: A = logits [N,C]; targets = N índices de classe (0-based).
// loss = média_i(-log softmax(A)[i, t_i]) (estável); dA = (softmax − onehot)/N.
func (a *Variable) SoftmaxCE(targets []int) (*Variable, error) {
	if len(a.Value.Shape) != 2 {
		return nil, fmt.Errorf("SoftmaxCE: logits devem ser 2D [N,C]")
	}
	n, c := a.Value.Shape[0], a.Value.Shape[1]
	if len(targets) != n {
		return nil, fmt.Errorf("SoftmaxCE: %d alvos para %d linhas", len(targets), n)
	}
	sm, err := a.Value.Softmax(1)
	if err != nil {
		return nil, err
	}
	var loss float32
	for i := 0; i < n; i++ {
		ti := targets[i]
		if ti < 0 || ti >= c {
			return nil, fmt.Errorf("SoftmaxCE: alvo %d fora de faixa (0..%d)", ti, c-1)
		}
		loss += -float32(math.Log(float64(sm.Data[i*c+ti]) + 1e-12))
	}
	loss /= float32(n)
	y, _ := tensor.FromData([]float32{loss}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0] / float32(n)
		dA := tensor.New(a.Value.Shape)
		for i := 0; i < n; i++ {
			for j := 0; j < c; j++ {
				val := sm.Data[i*c+j]
				if j == targets[i] {
					val -= 1
				}
				dA.Data[i*c+j] = g * val
			}
		}
		addGrad(a, dA)
	}
	return out, nil
}

// MSE(ŷ=a, alvo constante) -> escalar {1}. dŶ = (2/N)(Ŷ−alvo). O alvo não recebe grad.
func (a *Variable) MSE(target *Variable) (*Variable, error) {
	diff, err := a.Value.Sub(target.Value)
	if err != nil {
		return nil, err
	}
	n := float32(a.Value.Size())
	var s float32
	for _, d := range diff.Data {
		s += d * d
	}
	y, _ := tensor.FromData([]float32{s / n}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		scale := 2 * out.Grad.Data[0] / n
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = scale * diff.Data[i]
		}
		addGrad(a, dg)
	}
	return out, nil
}
