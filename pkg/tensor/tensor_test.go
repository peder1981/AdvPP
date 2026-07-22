package tensor

import "testing"

func TestNewAndSize(t *testing.T) {
	x := New([]int{2, 3})
	if x.Size() != 6 {
		t.Fatalf("Size = %d, quer 6", x.Size())
	}
	for _, v := range x.Data {
		if v != 0 {
			t.Fatalf("New deve ser zeros, achei %v", v)
		}
	}
}

func TestFromDataAndOffset(t *testing.T) {
	x, err := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	off, _ := x.Offset([]int{1, 2}) // 0-based (linha 1, col 2) -> 1*3+2 = 5
	if off != 5 {
		t.Fatalf("Offset = %d, quer 5", off)
	}
	got, _ := x.At([]int{1, 2})
	if got != 6 {
		t.Fatalf("At = %v, quer 6", got)
	}
}

func TestFromDataSizeMismatch(t *testing.T) {
	if _, err := FromData([]float32{1, 2, 3}, []int{2, 2}); err == nil {
		t.Fatal("esperava erro de tamanho")
	}
}

func TestElementwiseSameShape(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{10, 20, 30, 40}, []int{2, 2})
	got, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{11, 22, 33, 44}
	for i := range want {
		if got.Data[i] != want[i] {
			t.Fatalf("Add[%d]=%v quer %v", i, got.Data[i], want[i])
		}
	}
}

func TestBroadcastRowAndCol(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	row, _ := FromData([]float32{10, 20, 30}, []int{3})
	gr, err := a.Add(row) // por linha
	if err != nil {
		t.Fatal(err)
	}
	if gr.Data[0] != 11 || gr.Data[5] != 36 {
		t.Fatalf("broadcast linha errado: %v", gr.Data)
	}
	col, _ := FromData([]float32{100, 200}, []int{2, 1})
	gc, err := a.Add(col) // por coluna
	if err != nil {
		t.Fatal(err)
	}
	if gc.Data[0] != 101 || gc.Data[5] != 206 {
		t.Fatalf("broadcast coluna errado: %v", gc.Data)
	}
}

func TestScalarOps(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3}, []int{3})
	if g := a.MulScalar(2); g.Data[2] != 6 {
		t.Fatalf("MulScalar errado: %v", g.Data)
	}
	sc, _ := FromData([]float32{5}, []int{1})
	g, _ := a.Add(sc) // b escalar (Size 1)
	if g.Data[0] != 6 {
		t.Fatalf("add escalar errado: %v", g.Data)
	}
}

func TestBroadcastIncompatible(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{1, 2, 3}, []int{3})
	if _, err := a.Add(b); err == nil {
		t.Fatal("esperava erro de broadcast incompatível")
	}
}

func TestMatMul(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{5, 6, 7, 8}, []int{2, 2})
	got, err := a.MatMul(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{19, 22, 43, 50}
	for i := range want {
		if got.Data[i] != want[i] {
			t.Fatalf("MatMul[%d]=%v quer %v", i, got.Data[i], want[i])
		}
	}
}

func TestMatVec(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	v, _ := FromData([]float32{1, 0, 1}, []int{3})
	got, err := a.MatMul(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Shape) != 1 || got.Data[0] != 4 || got.Data[1] != 10 {
		t.Fatalf("MatVec errado: shape=%v data=%v", got.Shape, got.Data)
	}
}

func TestTransposeReshape(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	tr, err := a.Transpose()
	if err != nil {
		t.Fatal(err)
	}
	if tr.Shape[0] != 3 || tr.Shape[1] != 2 || tr.Data[0] != 1 || tr.Data[1] != 4 {
		t.Fatalf("Transpose errado: shape=%v data=%v", tr.Shape, tr.Data)
	}
	rs, err := a.Reshape([]int{3, 2})
	if err != nil {
		t.Fatal(err)
	}
	if rs.Shape[0] != 3 || rs.Data[5] != 6 {
		t.Fatalf("Reshape errado: %v", rs.Data)
	}
	if _, err := a.Reshape([]int{4, 2}); err == nil {
		t.Fatal("esperava erro de reshape incompatível")
	}
}
