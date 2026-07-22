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

// MatMul: [M,K]x[K,N]->[M,N]; matvec [M,K]x[K]->[M]. Ordem i-k-j (cache).
func (a *Tensor) MatMul(b *Tensor) (*Tensor, error) {
	if len(a.Shape) == 2 && len(b.Shape) == 1 && a.Shape[1] == b.Shape[0] {
		m, k := a.Shape[0], a.Shape[1]
		out := New([]int{m})
		for i := 0; i < m; i++ {
			var s float32
			for p := 0; p < k; p++ {
				s += a.Data[i*k+p] * b.Data[p]
			}
			out.Data[i] = s
		}
		return out, nil
	}
	if len(a.Shape) == 2 && len(b.Shape) == 2 && a.Shape[1] == b.Shape[0] {
		m, k, n := a.Shape[0], a.Shape[1], b.Shape[1]
		out := New([]int{m, n})
		for i := 0; i < m; i++ {
			for p := 0; p < k; p++ {
				aip := a.Data[i*k+p]
				for j := 0; j < n; j++ {
					out.Data[i*n+j] += aip * b.Data[p*n+j]
				}
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("MatMul: dims incompatíveis %v x %v", a.Shape, b.Shape)
}

// Transpose: transposta 2D.
func (a *Tensor) Transpose() (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("Transpose: requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	out := New([]int{n, m})
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			out.Data[j*m+i] = a.Data[i*n+j]
		}
	}
	return out, nil
}

// Reshape: mesma Data, nova forma (produto deve casar).
func (a *Tensor) Reshape(shape []int) (*Tensor, error) {
	if Prod(shape) != a.Size() {
		return nil, fmt.Errorf("Reshape: forma %v incompatível com size %d", shape, a.Size())
	}
	return &Tensor{Shape: copyInts(shape), Data: append([]float32(nil), a.Data...)}, nil
}

func (a *Tensor) SumAll() float32 {
	var s float32
	for _, v := range a.Data {
		s += v
	}
	return s
}
func (a *Tensor) MeanAll() float32 {
	if len(a.Data) == 0 {
		return 0
	}
	return a.SumAll() / float32(len(a.Data))
}
func (a *Tensor) MaxAll() float32 {
	m := a.Data[0]
	for _, v := range a.Data[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// ArgmaxAll devolve o offset (0-based) do máximo global.
func (a *Tensor) ArgmaxAll() int {
	bi := 0
	for i, v := range a.Data {
		if v > a.Data[bi] {
			bi = i
		}
	}
	_ = a.Data[0]
	return bi
}

// reduceAxis2D reduz um tensor 2D ao longo de axis (0 ou 1) com f (acumulador).
func (a *Tensor) reduceAxis2D(axis int, init float32, f func(acc, x float32) float32) (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("redução por eixo requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	switch axis {
	case 0:
		out := New([]int{n})
		for j := 0; j < n; j++ {
			acc := init
			for i := 0; i < m; i++ {
				acc = f(acc, a.Data[i*n+j])
			}
			out.Data[j] = acc
		}
		return out, nil
	case 1:
		out := New([]int{m})
		for i := 0; i < m; i++ {
			acc := init
			for j := 0; j < n; j++ {
				acc = f(acc, a.Data[i*n+j])
			}
			out.Data[i] = acc
		}
		return out, nil
	}
	return nil, fmt.Errorf("axis inválido: %d", axis)
}

func (a *Tensor) SumAxis(axis int) (*Tensor, error) {
	return a.reduceAxis2D(axis, 0, func(acc, x float32) float32 { return acc + x })
}
func (a *Tensor) MaxAxis(axis int) (*Tensor, error) {
	return a.reduceAxis2D(axis, float32(math.Inf(-1)), func(acc, x float32) float32 {
		if x > acc {
			return x
		}
		return acc
	})
}
func (a *Tensor) MeanAxis(axis int) (*Tensor, error) {
	s, err := a.SumAxis(axis)
	if err != nil {
		return nil, err
	}
	var cnt float32
	if axis == 0 {
		cnt = float32(a.Shape[0])
	} else {
		cnt = float32(a.Shape[1])
	}
	return s.MulScalar(1 / cnt), nil
}

// ArgmaxAxis: índices (0-based, como float32) do máximo por linha (axis 1) ou coluna (axis 0).
func (a *Tensor) ArgmaxAxis(axis int) (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("ArgmaxAxis requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	switch axis {
	case 0:
		out := New([]int{n})
		for j := 0; j < n; j++ {
			bi := 0
			for i := 1; i < m; i++ {
				if a.Data[i*n+j] > a.Data[bi*n+j] {
					bi = i
				}
			}
			out.Data[j] = float32(bi)
		}
		return out, nil
	case 1:
		out := New([]int{m})
		for i := 0; i < m; i++ {
			bi := 0
			for j := 1; j < n; j++ {
				if a.Data[i*n+j] > a.Data[i*n+bi] {
					bi = j
				}
			}
			out.Data[i] = float32(bi)
		}
		return out, nil
	}
	return nil, fmt.Errorf("axis inválido: %d", axis)
}
