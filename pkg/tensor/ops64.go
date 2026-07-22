package tensor

import (
	"fmt"
	"math"
)

// binOp64 aplica f elemento a elemento em float64 (mesmo broadcast do binOp f32),
// promovendo ambos os operandos para float64. Saída sempre Float64.
func binOp64(a, b *Tensor, f func(x, y float64) float64) (*Tensor, error) {
	a = a.AsDType(Float64)
	b = b.AsDType(Float64)
	if ShapeEq(a.Shape, b.Shape) {
		out := NewDType(a.Shape, Float64)
		for i := range a.Data64 {
			out.Data64[i] = f(a.Data64[i], b.Data64[i])
		}
		return out, nil
	}
	if b.Size() == 1 {
		out := NewDType(a.Shape, Float64)
		s := b.Data64[0]
		for i := range a.Data64 {
			out.Data64[i] = f(a.Data64[i], s)
		}
		return out, nil
	}
	if len(a.Shape) == 2 {
		m, n := a.Shape[0], a.Shape[1]
		isRow := (len(b.Shape) == 1 && b.Shape[0] == n) ||
			(len(b.Shape) == 2 && b.Shape[0] == 1 && b.Shape[1] == n)
		if isRow {
			out := NewDType(a.Shape, Float64)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data64[i*n+j] = f(a.Data64[i*n+j], b.Data64[j])
				}
			}
			return out, nil
		}
		if len(b.Shape) == 2 && b.Shape[0] == m && b.Shape[1] == 1 {
			out := NewDType(a.Shape, Float64)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data64[i*n+j] = f(a.Data64[i*n+j], b.Data64[i])
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("shapes incompatíveis para broadcast: %v e %v", a.Shape, b.Shape)
}

// matMul64 é o caminho float64 do MatMul (matriz-vetor e matriz-matriz), promovendo
// os operandos para float64. Espelha o kernel f32.
func matMul64(a, b *Tensor) (*Tensor, error) {
	a = a.AsDType(Float64)
	b = b.AsDType(Float64)
	if len(a.Shape) == 2 && len(b.Shape) == 1 && a.Shape[1] == b.Shape[0] {
		m, k := a.Shape[0], a.Shape[1]
		out := NewDType([]int{m}, Float64)
		for i := 0; i < m; i++ {
			var s float64
			for p := 0; p < k; p++ {
				s += a.Data64[i*k+p] * b.Data64[p]
			}
			out.Data64[i] = s
		}
		return out, nil
	}
	if len(a.Shape) == 2 && len(b.Shape) == 2 && a.Shape[1] == b.Shape[0] {
		m, k, n := a.Shape[0], a.Shape[1], b.Shape[1]
		out := NewDType([]int{m, n}, Float64)
		for i := 0; i < m; i++ {
			for p := 0; p < k; p++ {
				aip := a.Data64[i*k+p]
				for j := 0; j < n; j++ {
					out.Data64[i*n+j] += aip * b.Data64[p*n+j]
				}
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("MatMul: dims incompatíveis %v x %v", a.Shape, b.Shape)
}

// Dot é o produto interno de dois vetores (mesma quantidade de elementos),
// calculado na maior precisão dos dois (promove a float64 se qualquer um for f64).
func (a *Tensor) Dot(b *Tensor) (float64, error) {
	if a.Size() != b.Size() {
		return 0, fmt.Errorf("Dot: tamanhos diferentes %d e %d", a.Size(), b.Size())
	}
	if a.DType == Float64 || b.DType == Float64 {
		var s float64
		for i := 0; i < a.Size(); i++ {
			s += a.Get(i) * b.Get(i)
		}
		return s, nil
	}
	var s float32
	for i := range a.Data {
		s += a.Data[i] * b.Data[i]
	}
	return float64(s), nil
}

// Norm é a norma L2 (euclidiana) de todos os elementos, na precisão do tensor.
func (a *Tensor) Norm() float64 {
	if a.DType == Float64 {
		var s float64
		for _, v := range a.Data64 {
			s += v * v
		}
		return math.Sqrt(s)
	}
	var s float32
	for _, v := range a.Data {
		s += v * v
	}
	return math.Sqrt(float64(s))
}
