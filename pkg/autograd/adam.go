package autograd

import (
	"math"

	"github.com/advpl/compiler/pkg/tensor"
)

// Adam (Kingma & Ba 2014) com correção de viés. m/v por parâmetro.
type Adam struct {
	params          []*Variable
	lr, b1, b2, eps float32
	t               int
	m, v            []*tensor.Tensor
}

func NewAdam(params []*Variable, lr float32) *Adam {
	m := make([]*tensor.Tensor, len(params))
	v := make([]*tensor.Tensor, len(params))
	for i, p := range params {
		m[i] = tensor.New(p.Value.Shape)
		v[i] = tensor.New(p.Value.Shape)
	}
	return &Adam{params: params, lr: lr, b1: 0.9, b2: 0.999, eps: 1e-8, m: m, v: v}
}

func (o *Adam) Step() {
	o.t++
	bc1 := 1 - float32(math.Pow(float64(o.b1), float64(o.t)))
	bc2 := 1 - float32(math.Pow(float64(o.b2), float64(o.t)))
	for i, p := range o.params {
		if p.Grad == nil {
			continue
		}
		g := p.Grad.Data
		md := o.m[i].Data
		vd := o.v[i].Data
		pd := p.Value.Data
		for j := range pd {
			md[j] = o.b1*md[j] + (1-o.b1)*g[j]
			vd[j] = o.b2*vd[j] + (1-o.b2)*g[j]*g[j]
			mhat := md[j] / bc1
			vhat := vd[j] / bc2
			pd[j] -= o.lr * mhat / (float32(math.Sqrt(float64(vhat))) + o.eps)
		}
	}
}

func (o *Adam) ZeroGrad() {
	for _, p := range o.params {
		p.Grad = nil
	}
}
