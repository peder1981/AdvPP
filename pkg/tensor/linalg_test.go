package tensor

import (
	"math"
	"sort"
	"testing"
)

func m64(data []float64, shape ...int) *Tensor {
	t, err := FromData64(data, shape)
	if err != nil {
		panic(err)
	}
	return t
}

func close64(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestDet(t *testing.T) {
	// 2x2: det([[1,2],[3,4]]) = -2
	if d, _ := m64([]float64{1, 2, 3, 4}, 2, 2).Det(); !close64(d, -2) {
		t.Fatalf("det 2x2 = %v, quer -2", d)
	}
	// 3x3 conhecido: det([[6,1,1],[4,-2,5],[2,8,7]]) = -306
	if d, _ := m64([]float64{6, 1, 1, 4, -2, 5, 2, 8, 7}, 3, 3).Det(); !close64(d, -306) {
		t.Fatalf("det 3x3 = %v, quer -306", d)
	}
	// identidade -> 1
	if d, _ := m64([]float64{1, 0, 0, 1}, 2, 2).Det(); !close64(d, 1) {
		t.Fatalf("det I = %v, quer 1", d)
	}
	// singular -> 0
	if d, _ := m64([]float64{1, 2, 2, 4}, 2, 2).Det(); !close64(d, 0) {
		t.Fatalf("det singular = %v, quer 0", d)
	}
}

func TestSolve(t *testing.T) {
	// A=[[2,1],[1,3]], b=[3,5] -> x=[0.8,1.4]
	a := m64([]float64{2, 1, 1, 3}, 2, 2)
	x, err := a.Solve(m64([]float64{3, 5}, 2))
	if err != nil {
		t.Fatal(err)
	}
	if !close64(x.Get(0), 0.8) || !close64(x.Get(1), 1.4) {
		t.Fatalf("Solve x = [%v,%v], quer [0.8,1.4]", x.Get(0), x.Get(1))
	}
	// resíduo A·x - b ~ 0
	ax, _ := a.MatMul(x)
	if !close64(ax.Get(0), 3) || !close64(ax.Get(1), 5) {
		t.Fatalf("resíduo não-nulo: A·x = [%v,%v]", ax.Get(0), ax.Get(1))
	}
}

func TestInv(t *testing.T) {
	a := m64([]float64{4, 7, 2, 6}, 2, 2) // inv = [[0.6,-0.7],[-0.2,0.4]]
	inv, err := a.Inv()
	if err != nil {
		t.Fatal(err)
	}
	// A·A⁻¹ ≈ I
	prod, _ := a.MatMul(inv)
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			want := 0.0
			if i == j {
				want = 1
			}
			if !close64(prod.Get(i*2+j), want) {
				t.Fatalf("A·A⁻¹[%d,%d] = %v, quer %v", i, j, prod.Get(i*2+j), want)
			}
		}
	}
	// singular -> erro
	if _, err := m64([]float64{1, 2, 2, 4}, 2, 2).Inv(); err == nil {
		t.Fatal("Inv de singular deveria falhar")
	}
	// não-quadrada -> erro
	if _, err := m64([]float64{1, 2, 3, 4, 5, 6}, 2, 3).Det(); err == nil {
		t.Fatal("Det de não-quadrada deveria falhar")
	}
}

func TestQR(t *testing.T) {
	a := m64([]float64{12, -51, 4, 6, 167, -68, -4, 24, -41}, 3, 3)
	q, r, err := a.QR()
	if err != nil {
		t.Fatal(err)
	}
	// Q·R ≈ A
	qr, _ := q.MatMul(r)
	for i := 0; i < 9; i++ {
		if math.Abs(qr.Get(i)-a.Get(i)) > 1e-6 {
			t.Fatalf("Q·R[%d] = %v, quer %v", i, qr.Get(i), a.Get(i))
		}
	}
	// Qᵀ·Q ≈ I
	qt, _ := q.Transpose()
	qtq, _ := qt.MatMul(q)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			want := 0.0
			if i == j {
				want = 1
			}
			if math.Abs(qtq.Get(i*3+j)-want) > 1e-9 {
				t.Fatalf("QᵀQ[%d,%d] = %v, quer %v", i, j, qtq.Get(i*3+j), want)
			}
		}
	}
}

func TestEigSym(t *testing.T) {
	// [[2,0],[0,3]] -> autovalores 3,2 (desc); autovetores = eixos
	vals, vecs, err := m64([]float64{2, 0, 0, 3}, 2, 2).EigSym()
	if err != nil {
		t.Fatal(err)
	}
	if !close64(vals.Get(0), 3) || !close64(vals.Get(1), 2) {
		t.Fatalf("autovalores = [%v,%v], quer [3,2]", vals.Get(0), vals.Get(1))
	}
	// matriz simétrica geral: A·v = λ·v para cada par; soma autovalores = traço
	a := m64([]float64{4, 1, 1, 3}, 2, 2) // traço 7
	vals, vecs, _ = a.EigSym()
	if !close64(vals.Get(0)+vals.Get(1), 7) {
		t.Fatalf("soma autovalores = %v, quer 7 (traço)", vals.Get(0)+vals.Get(1))
	}
	for k := 0; k < 2; k++ {
		vk := m64([]float64{vecs.Get(0*2 + k), vecs.Get(1*2 + k)}, 2)
		av, _ := a.MatMul(vk)                              // A·v
		lam := vals.Get(k)
		if math.Abs(av.Get(0)-lam*vk.Get(0)) > 1e-6 || math.Abs(av.Get(1)-lam*vk.Get(1)) > 1e-6 {
			t.Fatalf("A·v != λ·v para autovetor %d", k)
		}
	}
	// não-simétrica -> erro
	if _, _, err := m64([]float64{1, 2, 3, 4}, 2, 2).EigSym(); err == nil {
		t.Fatal("EigSym de não-simétrica deveria falhar")
	}
}

func TestSVD(t *testing.T) {
	// A = U S Vᵀ ; verifica reconstrução e ortogonalidade.
	a := m64([]float64{3, 0, 0, 0, -2, 0, 0, 0, 1}, 3, 3) // diagonal -> sing = 3,2,1
	u, s, v, err := a.SVD()
	if err != nil {
		t.Fatal(err)
	}
	// valores singulares (desc): 3,2,1
	if !close64(s.Get(0), 3) || !close64(s.Get(1), 2) || !close64(s.Get(2), 1) {
		t.Fatalf("sing = [%v,%v,%v], quer [3,2,1]", s.Get(0), s.Get(1), s.Get(2))
	}
	// reconstrução U·diag(S)·Vᵀ ≈ A
	n := 3
	sd := New([]int{n, n}).AsDType(Float64)
	for i := 0; i < n; i++ {
		sd.Set(i*n+i, s.Get(i))
	}
	vt, _ := v.Transpose()
	us, _ := u.MatMul(sd)
	rec, _ := us.MatMul(vt)
	for i := 0; i < 9; i++ {
		if math.Abs(rec.Get(i)-a.Get(i)) > 1e-6 {
			t.Fatalf("recon[%d]=%v quer %v", i, rec.Get(i), a.Get(i))
		}
	}

	// matriz retangular m>n
	b := m64([]float64{1, 2, 3, 4, 5, 6}, 3, 2)
	ub, sb, vb, err := b.SVD()
	if err != nil {
		t.Fatal(err)
	}
	sdb := NewDType([]int{2, 2}, Float64)
	sdb.Set(0, sb.Get(0))
	sdb.Set(3, sb.Get(1))
	vbt, _ := vb.Transpose()
	usb, _ := ub.MatMul(sdb)
	recb, _ := usb.MatMul(vbt)
	for i := 0; i < 6; i++ {
		if math.Abs(recb.Get(i)-b.Get(i)) > 1e-6 {
			t.Fatalf("recon retangular[%d]=%v quer %v", i, recb.Get(i), b.Get(i))
		}
	}
}

// ordena um par (re,im) por re desc para comparação estável.
func sortedRe(re *Tensor) []float64 {
	out := append([]float64(nil), re.Data64...)
	sort.Float64s(out)
	return out
}

func TestEigNonSym(t *testing.T) {
	// triangular superior -> autovalores na diagonal: 1,4,6
	re, im, err := m64([]float64{1, 2, 3, 0, 4, 5, 0, 0, 6}, 3, 3).Eig()
	if err != nil {
		t.Fatal(err)
	}
	got := sortedRe(re)
	if !close64(got[0], 1) || !close64(got[1], 4) || !close64(got[2], 6) {
		t.Fatalf("triangular eig = %v, quer [1,4,6]", got)
	}
	for i := 0; i < 3; i++ {
		if !close64(im.Get(i), 0) {
			t.Fatalf("triangular: imag[%d]=%v, quer 0", i, im.Get(i))
		}
	}

	// [[2,1],[1,2]] -> 3,1 (reais)
	re, im, _ = m64([]float64{2, 1, 1, 2}, 2, 2).Eig()
	got = sortedRe(re)
	if !close64(got[0], 1) || !close64(got[1], 3) {
		t.Fatalf("simétrica-2x2 eig = %v, quer [1,3]", got)
	}

	// [[0,-1],[1,0]] -> ±i (complexo puro)
	re, im, _ = m64([]float64{0, -1, 1, 0}, 2, 2).Eig()
	if !close64(re.Get(0), 0) || !close64(re.Get(1), 0) {
		t.Fatalf("rotação: partes reais = [%v,%v], quer [0,0]", re.Get(0), re.Get(1))
	}
	if !close64(math.Abs(im.Get(0)), 1) || !close64(math.Abs(im.Get(1)), 1) {
		t.Fatalf("rotação: |imag| = [%v,%v], quer [1,1]", im.Get(0), im.Get(1))
	}

	// 3x3 com par complexo: [[1,-1,0],[1,1,0],[0,0,3]] -> 3, 1±i
	re, im, _ = m64([]float64{1, -1, 0, 1, 1, 0, 0, 0, 3}, 3, 3).Eig()
	// soma das partes reais = traço (5); soma dos autovalores reais... valida traço
	var sumRe float64
	for i := 0; i < 3; i++ {
		sumRe += re.Get(i)
	}
	if !close64(sumRe, 5) {
		t.Fatalf("soma partes reais = %v, quer 5 (traço)", sumRe)
	}
	// deve haver um autovalor real 3 e um par com |imag|=1
	foundReal3, foundComplex := false, false
	for i := 0; i < 3; i++ {
		if close64(re.Get(i), 3) && close64(im.Get(i), 0) {
			foundReal3 = true
		}
		if close64(re.Get(i), 1) && close64(math.Abs(im.Get(i)), 1) {
			foundComplex = true
		}
	}
	if !foundReal3 || !foundComplex {
		t.Fatalf("esperava autovalor 3 e par 1±i; re=%v im=%v", re.Data64, im.Data64)
	}
}
