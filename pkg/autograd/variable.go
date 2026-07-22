// Package autograd implementa diferenciação reversa (reverse-mode autodiff) sobre
// o tensor float32 do pkg/tensor: cada Variable grava um tape de operações e
// Backward propaga gradientes de trás pra frente.
package autograd

import "github.com/advpl/compiler/pkg/tensor"

type Variable struct {
	Value    *tensor.Tensor
	Grad     *tensor.Tensor // acumulado; nil até receber gradiente
	parents  []*Variable
	backward func() // lê v.Grad e acumula nos Grad dos pais
}

// NewLeaf cria uma Variable folha (sem pais) a partir de um tensor de valor.
func NewLeaf(v *tensor.Tensor) *Variable {
	return &Variable{Value: v}
}

// addGrad acumula g no gradiente de v (soma se já houver; cópia na primeira vez).
func addGrad(v *Variable, g *tensor.Tensor) {
	if v.Grad == nil {
		c, _ := tensor.FromData(g.Data, g.Shape)
		v.Grad = c
		return
	}
	if sum, err := v.Grad.Add(g); err == nil {
		v.Grad = sum
	}
}

func onesLike(t *tensor.Tensor) *tensor.Tensor {
	out := tensor.New(t.Shape)
	for i := range out.Data {
		out.Data[i] = 1
	}
	return out
}

// reduceGradTo soma g sobre os eixos replicados no broadcast do Add, casando a
// forma alvo (os 4 casos do pkg/tensor: mesma / escalar / linha / coluna).
func reduceGradTo(g *tensor.Tensor, shape []int) *tensor.Tensor {
	if tensor.ShapeEq(g.Shape, shape) {
		return g
	}
	if tensor.Prod(shape) == 1 {
		out := tensor.New(shape)
		out.Data[0] = g.SumAll()
		return out
	}
	if len(g.Shape) == 2 {
		n := g.Shape[1]
		// linha [N] ou [1,N] -> soma no eixo 0
		if (len(shape) == 1 && shape[0] == n) || (len(shape) == 2 && shape[0] == 1 && shape[1] == n) {
			s, _ := g.SumAxis(0) // [N]
			if len(shape) == 2 {
				s, _ = s.Reshape(shape) // [N] -> [1,N]
			}
			return s
		}
		// coluna [M,1] -> soma no eixo 1
		if len(shape) == 2 && shape[0] == g.Shape[0] && shape[1] == 1 {
			s, _ := g.SumAxis(1) // [M]
			s, _ = s.Reshape(shape) // [M] -> [M,1]
			return s
		}
	}
	return g
}

// Backward propaga gradientes a partir desta Variable (deve ser escalar).
func (v *Variable) Backward() {
	var topo []*Variable
	visited := map[*Variable]bool{}
	var build func(n *Variable)
	build = func(n *Variable) {
		if visited[n] {
			return
		}
		visited[n] = true
		for _, p := range n.parents {
			build(p)
		}
		topo = append(topo, n)
	}
	build(v)
	v.Grad = onesLike(v.Value)
	for i := len(topo) - 1; i >= 0; i-- {
		if topo[i].backward != nil {
			topo[i].backward()
		}
	}
}
