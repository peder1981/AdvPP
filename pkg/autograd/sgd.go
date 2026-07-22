package autograd

// SGD é o gradiente descendente estocástico simples: p := p - lr·grad(p).
type SGD struct {
	params []*Variable
	lr     float32
}

func NewSGD(params []*Variable, lr float32) *SGD {
	return &SGD{params: params, lr: lr}
}

// Step atualiza cada parâmetro in-place (mantém a identidade do tensor de valor).
func (o *SGD) Step() {
	for _, p := range o.params {
		if p.Grad == nil {
			continue
		}
		upd := p.Grad.MulScalar(o.lr)
		nv, err := p.Value.Sub(upd)
		if err == nil {
			copy(p.Value.Data, nv.Data)
		}
	}
}

// ZeroGrad zera os gradientes dos parâmetros antes do próximo backward.
func (o *SGD) ZeroGrad() {
	for _, p := range o.params {
		p.Grad = nil
	}
}
