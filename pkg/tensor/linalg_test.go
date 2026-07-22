package tensor

import (
	"math"
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
