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

// QR decompõe A [m,n] (m>=n) em Q [m,m] ortogonal e R [m,n] triangular superior,
// por refletores de Householder. Devolve {Q, R} (float64).
func (a *Tensor) QR() (q *Tensor, r *Tensor, err error) {
	if len(a.Shape) != 2 {
		return nil, nil, fmt.Errorf("QR: requer 2D, veio %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	if m < n {
		return nil, nil, fmt.Errorf("QR: requer m>=n, veio [%d,%d]", m, n)
	}
	f := a.AsDType(Float64)
	// R começa como cópia de A; Q como identidade [m,m].
	R := make([][]float64, m)
	Q := make([][]float64, m)
	for i := 0; i < m; i++ {
		R[i] = make([]float64, n)
		copy(R[i], f.Data64[i*n:(i+1)*n])
		Q[i] = make([]float64, m)
		Q[i][i] = 1
	}
	for k := 0; k < n && k < m-1; k++ {
		// vetor de Householder da coluna k (linhas k..m-1)
		var norm float64
		for i := k; i < m; i++ {
			norm += R[i][k] * R[i][k]
		}
		norm = math.Sqrt(norm)
		if norm < luEps {
			continue
		}
		if R[k][k] > 0 {
			norm = -norm
		}
		v := make([]float64, m)
		v[k] = R[k][k] - norm
		for i := k + 1; i < m; i++ {
			v[i] = R[i][k]
		}
		var vnorm2 float64
		for i := k; i < m; i++ {
			vnorm2 += v[i] * v[i]
		}
		if vnorm2 < luEps {
			continue
		}
		// R := (I - 2 v vᵀ / vᵀv) R
		for j := 0; j < n; j++ {
			var dot float64
			for i := k; i < m; i++ {
				dot += v[i] * R[i][j]
			}
			f2 := 2 * dot / vnorm2
			for i := k; i < m; i++ {
				R[i][j] -= f2 * v[i]
			}
		}
		// Q := Q (I - 2 v vᵀ / vᵀv)
		for i := 0; i < m; i++ {
			var dot float64
			for j := k; j < m; j++ {
				dot += Q[i][j] * v[j]
			}
			f2 := 2 * dot / vnorm2
			for j := k; j < m; j++ {
				Q[i][j] -= f2 * v[j]
			}
		}
	}
	qt := NewDType([]int{m, m}, Float64)
	for i := 0; i < m; i++ {
		copy(qt.Data64[i*m:(i+1)*m], Q[i])
	}
	rt := NewDType([]int{m, n}, Float64)
	for i := 0; i < m; i++ {
		copy(rt.Data64[i*n:(i+1)*n], R[i])
	}
	return qt, rt, nil
}

const (
	jacobiMaxSweeps = 100
	jacobiEps       = 1e-14
	symTol          = 1e-9 // tolerância p/ verificar simetria
)

// EigSym calcula autovalores e autovetores de uma matriz SIMÉTRICA [n,n] por
// rotações de Jacobi cíclicas. Devolve {valores [n], vetores [n,n]} (float64), com
// os autovalores em ordem decrescente e as COLUNAS de vetores como autovetores.
func (a *Tensor) EigSym() (values *Tensor, vectors *Tensor, err error) {
	m, n, err := a.mat2D()
	if err != nil {
		return nil, nil, err
	}
	// exige simetria
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if math.Abs(m[i][j]-m[j][i]) > symTol {
				return nil, nil, fmt.Errorf("EigSym: matriz não é simétrica em (%d,%d)", i, j)
			}
		}
	}
	// V = identidade (acumula autovetores)
	V := make([][]float64, n)
	for i := 0; i < n; i++ {
		V[i] = make([]float64, n)
		V[i][i] = 1
	}
	for sweep := 0; sweep < jacobiMaxSweeps; sweep++ {
		// soma dos off-diagonais ao quadrado
		var off float64
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				off += m[i][j] * m[i][j]
			}
		}
		if off < jacobiEps {
			break
		}
		for p := 0; p < n; p++ {
			for q := p + 1; q < n; q++ {
				if math.Abs(m[p][q]) < jacobiEps {
					continue
				}
				// ângulo de rotação que zera m[p][q]
				theta := (m[q][q] - m[p][p]) / (2 * m[p][q])
				tsign := 1.0
				if theta < 0 {
					tsign = -1.0
				}
				tval := tsign / (math.Abs(theta) + math.Sqrt(theta*theta+1))
				c := 1 / math.Sqrt(tval*tval+1)
				s := tval * c
				// aplica rotação em M: linhas/colunas p,q
				for i := 0; i < n; i++ {
					mip := m[i][p]
					miq := m[i][q]
					m[i][p] = c*mip - s*miq
					m[i][q] = s*mip + c*miq
				}
				for i := 0; i < n; i++ {
					mpi := m[p][i]
					mqi := m[q][i]
					m[p][i] = c*mpi - s*mqi
					m[q][i] = s*mpi + c*mqi
				}
				// acumula em V
				for i := 0; i < n; i++ {
					vip := V[i][p]
					viq := V[i][q]
					V[i][p] = c*vip - s*viq
					V[i][q] = s*vip + c*viq
				}
			}
		}
	}
	// autovalores na diagonal; ordena desc levando os autovetores junto
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if m[idx[j]][idx[j]] > m[idx[i]][idx[i]] {
				idx[i], idx[j] = idx[j], idx[i]
			}
		}
	}
	values = NewDType([]int{n}, Float64)
	vectors = NewDType([]int{n, n}, Float64)
	for k, id := range idx {
		values.Data64[k] = m[id][id]
		for i := 0; i < n; i++ {
			vectors.Data64[i*n+k] = V[i][id]
		}
	}
	return values, vectors, nil
}
