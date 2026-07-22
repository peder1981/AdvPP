package tensor

import (
	"fmt"
	"math"
)

const luEps = 1e-12 // pivô abaixo disso => matriz singular

// mat2D extrai uma matriz quadrada [n,n] como [][]float64 (converte a f64).
// Devolve n e erro se não for 2D quadrada.
func (a *Tensor) mat2D() ([][]float64, int, error) {
	if len(a.Shape) != 2 || a.Shape[0] != a.Shape[1] {
		return nil, 0, fmt.Errorf("esperada matriz quadrada [n,n], veio %v", a.Shape)
	}
	n := a.Shape[0]
	f := a.AsDType(Float64)
	m := make([][]float64, n)
	for i := 0; i < n; i++ {
		m[i] = make([]float64, n)
		copy(m[i], f.Data64[i*n:(i+1)*n])
	}
	return m, n, nil
}

// fromMat2D empacota uma [][]float64 [n,n] num Tensor float64.
func fromMat2D(m [][]float64) *Tensor {
	n := len(m)
	out := NewDType([]int{n, n}, Float64)
	for i := 0; i < n; i++ {
		copy(out.Data64[i*n:(i+1)*n], m[i])
	}
	return out
}

// luDecompose faz a decomposição LU de m [n,n] in-place (Doolittle, pivô parcial).
// Devolve o vetor de permutação perm e o sinal das trocas. Erro se singular.
func luDecompose(m [][]float64, n int) (perm []int, sign float64, err error) {
	perm = make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	sign = 1
	for k := 0; k < n; k++ {
		// pivô parcial: maior |m[i][k]| em i>=k
		p := k
		max := math.Abs(m[k][k])
		for i := k + 1; i < n; i++ {
			if math.Abs(m[i][k]) > max {
				max = math.Abs(m[i][k])
				p = i
			}
		}
		if max < luEps {
			return nil, 0, fmt.Errorf("matriz singular (pivô ~0 na coluna %d)", k)
		}
		if p != k {
			m[k], m[p] = m[p], m[k]
			perm[k], perm[p] = perm[p], perm[k]
			sign = -sign
		}
		for i := k + 1; i < n; i++ {
			m[i][k] /= m[k][k]
			for j := k + 1; j < n; j++ {
				m[i][j] -= m[i][k] * m[k][j]
			}
		}
	}
	return perm, sign, nil
}

// Det: determinante via LU (produto da diagonal de U × sinal das trocas).
func (a *Tensor) Det() (float64, error) {
	m, n, err := a.mat2D()
	if err != nil {
		return 0, err
	}
	_, sign, err := luDecompose(m, n)
	if err != nil {
		return 0, nil // singular => determinante 0
	}
	det := sign
	for i := 0; i < n; i++ {
		det *= m[i][i]
	}
	return det, nil
}

// solveLU resolve L·U·x = P·b para uma coluna b (comprimento n), dado o LU e perm.
func solveLU(lu [][]float64, perm []int, b []float64, n int) []float64 {
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = b[perm[i]]
	}
	// substituição direta (L, diagonal unitária)
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			x[i] -= lu[i][j] * x[j]
		}
	}
	// substituição reversa (U)
	for i := n - 1; i >= 0; i-- {
		for j := i + 1; j < n; j++ {
			x[i] -= lu[i][j] * x[j]
		}
		x[i] /= lu[i][i]
	}
	return x
}

// Solve resolve A·x = b. b pode ser vetor [n] ou matriz [n,k]. Saída f64 com a
// mesma "forma" de b (vetor -> [n]; matriz -> [n,k]).
func (a *Tensor) Solve(b *Tensor) (*Tensor, error) {
	m, n, err := a.mat2D()
	if err != nil {
		return nil, err
	}
	perm, _, err := luDecompose(m, n)
	if err != nil {
		return nil, err
	}
	bf := b.AsDType(Float64)
	// vetor [n]
	if len(b.Shape) == 1 {
		if b.Shape[0] != n {
			return nil, fmt.Errorf("Solve: b [%d] incompatível com A [%d,%d]", b.Shape[0], n, n)
		}
		x := solveLU(m, perm, bf.Data64, n)
		out := NewDType([]int{n}, Float64)
		copy(out.Data64, x)
		return out, nil
	}
	// matriz [n,k]: resolve coluna a coluna
	if len(b.Shape) == 2 && b.Shape[0] == n {
		k := b.Shape[1]
		out := NewDType([]int{n, k}, Float64)
		col := make([]float64, n)
		for c := 0; c < k; c++ {
			for i := 0; i < n; i++ {
				col[i] = bf.Data64[i*k+c]
			}
			x := solveLU(m, perm, col, n)
			for i := 0; i < n; i++ {
				out.Data64[i*k+c] = x[i]
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("Solve: b deve ser [n] ou [n,k], veio %v", b.Shape)
}

// Inv: inversa resolvendo A·X = I.
func (a *Tensor) Inv() (*Tensor, error) {
	m, n, err := a.mat2D()
	if err != nil {
		return nil, err
	}
	perm, _, err := luDecompose(m, n)
	if err != nil {
		return nil, err
	}
	out := NewDType([]int{n, n}, Float64)
	col := make([]float64, n)
	for c := 0; c < n; c++ {
		for i := 0; i < n; i++ {
			if i == c {
				col[i] = 1
			} else {
				col[i] = 0
			}
		}
		x := solveLU(m, perm, col, n)
		for i := 0; i < n; i++ {
			out.Data64[i*n+c] = x[i]
		}
	}
	return out, nil
}
