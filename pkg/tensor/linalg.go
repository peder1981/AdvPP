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

const (
	svdMaxSweeps = 60
	svdEps       = 1e-14
)

// SVD decompõe A [m,n] em U [m,n] (colunas ortonormais), S [k] (valores singulares
// decrescentes, k=min(m,n)) e V [n,n] (ortogonal), tal que A ≈ U·diag(S)·Vᵀ.
// Usa Jacobi de um lado (estável). Devolve {U, S, V} (float64).
func (a *Tensor) SVD() (u *Tensor, s *Tensor, vt *Tensor, err error) {
	if len(a.Shape) != 2 {
		return nil, nil, nil, fmt.Errorf("SVD: requer 2D, veio %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	// Para m<n, calcula SVD de Aᵀ (=V Σ Uᵀ) e troca U<->V.
	if m < n {
		at, _ := a.Transpose()
		uu, ss, vv, e := at.SVD()
		return vv, ss, uu, e
	}
	f := a.AsDType(Float64)
	// U := cópia de A (colunas serão ortogonalizadas); V := I.
	U := make([][]float64, m)
	for i := 0; i < m; i++ {
		U[i] = make([]float64, n)
		copy(U[i], f.Data64[i*n:(i+1)*n])
	}
	V := make([][]float64, n)
	for i := 0; i < n; i++ {
		V[i] = make([]float64, n)
		V[i][i] = 1
	}
	for sweep := 0; sweep < svdMaxSweeps; sweep++ {
		off := 0.0
		for i := 0; i < n-1; i++ {
			for j := i + 1; j < n; j++ {
				var alpha, beta, gamma float64
				for r := 0; r < m; r++ {
					alpha += U[r][i] * U[r][i]
					beta += U[r][j] * U[r][j]
					gamma += U[r][i] * U[r][j]
				}
				off += math.Abs(gamma)
				if math.Abs(gamma) < svdEps*math.Sqrt(alpha*beta) || alpha == 0 || beta == 0 {
					continue
				}
				zeta := (beta - alpha) / (2 * gamma)
				tsign := 1.0
				if zeta < 0 {
					tsign = -1.0
				}
				tval := tsign / (math.Abs(zeta) + math.Sqrt(zeta*zeta+1))
				c := 1 / math.Sqrt(tval*tval+1)
				sn := c * tval
				for r := 0; r < m; r++ {
					ui, uj := U[r][i], U[r][j]
					U[r][i] = c*ui - sn*uj
					U[r][j] = sn*ui + c*uj
				}
				for r := 0; r < n; r++ {
					vi, vj := V[r][i], V[r][j]
					V[r][i] = c*vi - sn*vj
					V[r][j] = sn*vi + c*vj
				}
			}
		}
		if off < svdEps {
			break
		}
	}
	// valores singulares = normas das colunas de U; normaliza U.
	sig := make([]float64, n)
	for j := 0; j < n; j++ {
		var nrm float64
		for r := 0; r < m; r++ {
			nrm += U[r][j] * U[r][j]
		}
		sig[j] = math.Sqrt(nrm)
		if sig[j] > svdEps {
			for r := 0; r < m; r++ {
				U[r][j] /= sig[j]
			}
		}
	}
	// ordena por sigma desc (leva colunas de U e V junto)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if sig[idx[j]] > sig[idx[i]] {
				idx[i], idx[j] = idx[j], idx[i]
			}
		}
	}
	uOut := NewDType([]int{m, n}, Float64)
	sOut := NewDType([]int{n}, Float64)
	vOut := NewDType([]int{n, n}, Float64)
	for k, id := range idx {
		sOut.Data64[k] = sig[id]
		for r := 0; r < m; r++ {
			uOut.Data64[r*n+k] = U[r][id]
		}
		for r := 0; r < n; r++ {
			vOut.Data64[r*n+k] = V[r][id]
		}
	}
	return uOut, sOut, vOut, nil
}

const eigMaxIter = 100

// toHessenberg reduz a [n,n] (in-place) à forma de Hessenberg superior por
// similaridade de Householder (preserva os autovalores).
func toHessenberg(a [][]float64, n int) {
	for k := 0; k < n-2; k++ {
		var alpha float64
		for i := k + 1; i < n; i++ {
			alpha += a[i][k] * a[i][k]
		}
		alpha = math.Sqrt(alpha)
		if alpha == 0 {
			continue
		}
		if a[k+1][k] > 0 {
			alpha = -alpha
		}
		v := make([]float64, n)
		v[k+1] = a[k+1][k] - alpha
		for i := k + 2; i < n; i++ {
			v[i] = a[i][k]
		}
		var vv float64
		for i := k + 1; i < n; i++ {
			vv += v[i] * v[i]
		}
		if vv == 0 {
			continue
		}
		for j := 0; j < n; j++ { // esquerda: A := (I - 2vvᵀ/vv) A
			var s float64
			for i := k + 1; i < n; i++ {
				s += v[i] * a[i][j]
			}
			s = 2 * s / vv
			for i := k + 1; i < n; i++ {
				a[i][j] -= s * v[i]
			}
		}
		for i := 0; i < n; i++ { // direita: A := A (I - 2vvᵀ/vv)
			var s float64
			for j := k + 1; j < n; j++ {
				s += a[i][j] * v[j]
			}
			s = 2 * s / vv
			for j := k + 1; j < n; j++ {
				a[i][j] -= s * v[j]
			}
		}
	}
}

// hqr encontra os autovalores (reais e complexos conjugados) de uma matriz de
// Hessenberg superior real h [n,n] pelo algoritmo QR de duplo shift (Francis),
// à la EISPACK/Numerical Recipes. Devolve as partes real (wr) e imaginária (wi).
func hqr(h [][]float64, n int) (wr, wi []float64, ok bool) {
	wr = make([]float64, n)
	wi = make([]float64, n)
	var anorm float64
	for i := 0; i < n; i++ {
		lo := i - 1
		if lo < 0 {
			lo = 0
		}
		for j := lo; j < n; j++ {
			anorm += math.Abs(h[i][j])
		}
	}
	nn := n - 1
	t := 0.0
	for nn >= 0 {
		its := 0
		for {
			var l int
			for l = nn; l >= 1; l-- {
				s := math.Abs(h[l-1][l-1]) + math.Abs(h[l][l])
				if s == 0 {
					s = anorm
				}
				if math.Abs(h[l][l-1])+s == s {
					h[l][l-1] = 0
					break
				}
			}
			x := h[nn][nn]
			if l == nn { // 1 raiz real
				wr[nn] = x + t
				wi[nn] = 0
				nn--
				break
			}
			y := h[nn-1][nn-1]
			w := h[nn][nn-1] * h[nn-1][nn]
			if l == nn-1 { // bloco 2x2 -> 2 raízes
				p := 0.5 * (y - x)
				q := p*p + w
				z := math.Sqrt(math.Abs(q))
				x += t
				if q >= 0 { // par real
					if p >= 0 {
						z = p + z
					} else {
						z = p - z
					}
					wr[nn-1] = x + z
					wr[nn] = wr[nn-1]
					if z != 0 {
						wr[nn] = x - w/z
					}
					wi[nn-1] = 0
					wi[nn] = 0
				} else { // par complexo conjugado
					wr[nn-1] = x + p
					wr[nn] = x + p
					wi[nn-1] = -z
					wi[nn] = z
				}
				nn -= 2
				break
			}
			if its == eigMaxIter {
				return wr, wi, false
			}
			if its == 10 || its == 20 { // shift excepcional
				t += x
				for i := 0; i <= nn; i++ {
					h[i][i] -= x
				}
				s := math.Abs(h[nn][nn-1]) + math.Abs(h[nn-1][nn-2])
				y = 0.75 * s
				x = y
				w = -0.4375 * s * s
			}
			its++
			// dois subdiagonais consecutivos pequenos
			var mm int
			var p, q, r float64
			for mm = nn - 2; mm >= l; mm-- {
				z := h[mm][mm]
				rr := x - z
				ss := y - z
				p = (rr*ss-w)/h[mm+1][mm] + h[mm][mm+1]
				q = h[mm+1][mm+1] - z - rr - ss
				r = h[mm+2][mm+1]
				sc := math.Abs(p) + math.Abs(q) + math.Abs(r)
				p /= sc
				q /= sc
				r /= sc
				if mm == l {
					break
				}
				u := math.Abs(h[mm][mm-1]) * (math.Abs(q) + math.Abs(r))
				vv := math.Abs(p) * (math.Abs(h[mm-1][mm-1]) + math.Abs(z) + math.Abs(h[mm+1][mm+1]))
				if u+vv == vv {
					break
				}
			}
			for i := mm + 2; i <= nn; i++ {
				h[i][i-2] = 0
				if i != mm+2 {
					h[i][i-3] = 0
				}
			}
			// varredura QR de duplo shift (bulge chasing)
			for k := mm; k <= nn-1; k++ {
				if k != mm {
					p = h[k][k-1]
					q = h[k+1][k-1]
					r = 0
					if k != nn-1 {
						r = h[k+2][k-1]
					}
					x = math.Abs(p) + math.Abs(q) + math.Abs(r)
					if x != 0 {
						p /= x
						q /= x
						r /= x
					}
				}
				s := math.Sqrt(p*p + q*q + r*r)
				if p < 0 {
					s = -s
				}
				if s == 0 {
					continue
				}
				if k == mm {
					if l != mm {
						h[k][k-1] = -h[k][k-1]
					}
				} else {
					h[k][k-1] = -s * x
				}
				p += s
				x = p / s
				y = q / s
				z := r / s
				q /= p
				r /= p
				for j := k; j <= nn; j++ { // modificação de linha
					p = h[k][j] + q*h[k+1][j]
					if k != nn-1 {
						p += r * h[k+2][j]
						h[k+2][j] -= p * z
					}
					h[k+1][j] -= p * y
					h[k][j] -= p * x
				}
				mmin := nn
				if k+3 < nn {
					mmin = k + 3
				}
				for i := l; i <= mmin; i++ { // modificação de coluna
					p = x*h[i][k] + y*h[i][k+1]
					if k != nn-1 {
						p += z * h[i][k+2]
						h[i][k+2] -= p * r
					}
					h[i][k+1] -= p * q
					h[i][k] -= p
				}
			}
		}
	}
	return wr, wi, true
}

// Eig calcula TODOS os autovalores (reais e complexos) de uma matriz [n,n] real,
// não necessariamente simétrica, via redução a Hessenberg + QR de duplo shift.
// Devolve {reais [n], imag [n]} — para autovalor complexo, aparece o par conjugado.
func (a *Tensor) Eig() (real *Tensor, imag *Tensor, err error) {
	m, n, err := a.mat2D()
	if err != nil {
		return nil, nil, err
	}
	toHessenberg(m, n)
	wr, wi, ok := hqr(m, n)
	if !ok {
		return nil, nil, fmt.Errorf("Eig: não convergiu em %d iterações", eigMaxIter)
	}
	real = NewDType([]int{n}, Float64)
	imag = NewDType([]int{n}, Float64)
	copy(real.Data64, wr)
	copy(imag.Data64, wi)
	return real, imag, nil
}
