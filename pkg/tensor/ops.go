package tensor

import (
	"fmt"
	"math"
)

// binOp aplica f elemento a elemento com broadcast limitado:
// mesma forma; b escalar (Size 1); ou a 2D [M,N] com b linha [N]/[1,N] ou coluna [M,1].
func binOp(a, b *Tensor, f func(x, y float32) float32) (*Tensor, error) {
	if ShapeEq(a.Shape, b.Shape) {
		out := New(a.Shape)
		for i := range a.Data {
			out.Data[i] = f(a.Data[i], b.Data[i])
		}
		return out, nil
	}
	if b.Size() == 1 {
		out := New(a.Shape)
		s := b.Data[0]
		for i := range a.Data {
			out.Data[i] = f(a.Data[i], s)
		}
		return out, nil
	}
	if len(a.Shape) == 2 {
		m, n := a.Shape[0], a.Shape[1]
		isRow := (len(b.Shape) == 1 && b.Shape[0] == n) ||
			(len(b.Shape) == 2 && b.Shape[0] == 1 && b.Shape[1] == n)
		if isRow {
			out := New(a.Shape)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data[i*n+j] = f(a.Data[i*n+j], b.Data[j])
				}
			}
			return out, nil
		}
		if len(b.Shape) == 2 && b.Shape[0] == m && b.Shape[1] == 1 {
			out := New(a.Shape)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data[i*n+j] = f(a.Data[i*n+j], b.Data[i])
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("shapes incompatíveis para broadcast: %v e %v", a.Shape, b.Shape)
}

func (a *Tensor) Add(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x + y })
}
func (a *Tensor) Sub(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x - y })
}
func (a *Tensor) Mul(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x * y })
}
func (a *Tensor) Div(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x / y })
}

func (a *Tensor) AddScalar(s float32) *Tensor {
	out := New(a.Shape)
	for i, v := range a.Data {
		out.Data[i] = v + s
	}
	return out
}
func (a *Tensor) MulScalar(s float32) *Tensor {
	out := New(a.Shape)
	for i, v := range a.Data {
		out.Data[i] = v * s
	}
	return out
}

var _ = math.Exp // math será usado nas ativações (Task 5)
