package tensor

import (
	"math"
	"testing"
)

func almost(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

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

func TestReduceAll(t *testing.T) {
	a, _ := FromData([]float32{1, 5, 2, 4}, []int{2, 2})
	if a.SumAll() != 12 || a.MaxAll() != 5 || a.MeanAll() != 3 {
		t.Fatalf("reduce all errado: sum=%v max=%v mean=%v", a.SumAll(), a.MaxAll(), a.MeanAll())
	}
	if a.ArgmaxAll() != 1 { // 0-based: o 5 está no offset 1
		t.Fatalf("ArgmaxAll=%d quer 1", a.ArgmaxAll())
	}
}

func TestReduceAxis(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	s0, _ := a.SumAxis(0) // sobre linhas -> [5,7,9]
	if !ShapeEq(s0.Shape, []int{3}) || s0.Data[0] != 5 || s0.Data[2] != 9 {
		t.Fatalf("SumAxis(0) errado: %v %v", s0.Shape, s0.Data)
	}
	s1, _ := a.SumAxis(1) // sobre colunas -> [6,15]
	if !ShapeEq(s1.Shape, []int{2}) || s1.Data[1] != 15 {
		t.Fatalf("SumAxis(1) errado: %v %v", s1.Shape, s1.Data)
	}
	am, _ := a.ArgmaxAxis(1) // por linha -> [2,2] (0-based)
	if am.Data[0] != 2 || am.Data[1] != 2 {
		t.Fatalf("ArgmaxAxis(1) errado: %v", am.Data)
	}
}

func TestActivations(t *testing.T) {
	a, _ := FromData([]float32{-1, 0, 2}, []int{3})
	r := a.Relu()
	if r.Data[0] != 0 || r.Data[2] != 2 {
		t.Fatalf("Relu errado: %v", r.Data)
	}
	e := a.Exp()
	if !almost(e.Data[1], 1) {
		t.Fatalf("Exp(0)=%v quer 1", e.Data[1])
	}
}

func TestSoftmax(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 1, 2, 3}, []int{2, 3})
	s, err := a.Softmax(1) // por linha
	if err != nil {
		t.Fatal(err)
	}
	var row0 float32
	for j := 0; j < 3; j++ {
		row0 += s.Data[j]
	}
	if !almost(row0, 1) {
		t.Fatalf("softmax linha não soma 1: %v", row0)
	}
	if !(s.Data[2] > s.Data[0]) {
		t.Fatalf("softmax deve favorecer o maior logit")
	}
}

func TestSoftmaxStable(t *testing.T) {
	a, _ := FromData([]float32{1000, 1001, 1002}, []int{3})
	s, _ := a.Softmax(0)
	var sum float32
	for _, v := range s.Data {
		sum += v
		if math.IsNaN(float64(v)) {
			t.Fatal("softmax instável (NaN)")
		}
	}
	if !almost(sum, 1) {
		t.Fatalf("softmax 1D não soma 1: %v", sum)
	}
}

func TestIndexRows(t *testing.T) {
	a, _ := FromData([]float32{10, 11, 20, 21, 30, 31}, []int{3, 2})
	g, err := a.IndexRows([]int{2, 0}) // linhas 2 e 0 (0-based)
	if err != nil {
		t.Fatal(err)
	}
	if !ShapeEq(g.Shape, []int{2, 2}) || g.Data[0] != 30 || g.Data[3] != 11 {
		t.Fatalf("IndexRows errado: %v %v", g.Shape, g.Data)
	}
}

func TestDTypeInfra(t *testing.T) {
	x := NewDType([]int{2, 2}, Float64)
	if x.DType != Float64 {
		t.Fatalf("DType = %v, quer Float64", x.DType)
	}
	if x.Size() != 4 {
		t.Fatalf("Size = %d, quer 4", x.Size())
	}
	x.Set(0, 1.5)
	if x.Get(0) != 1.5 {
		t.Fatalf("Get(0) = %v, quer 1.5", x.Get(0))
	}
	if err := x.SetAt([]int{1, 1}, 9); err != nil {
		t.Fatal(err)
	}
	if v, _ := x.At([]int{1, 1}); v != 9 {
		t.Fatalf("At(1,1) = %v, quer 9", v)
	}

	y, err := FromData64([]float64{1, 2, 3}, []int{3})
	if err != nil {
		t.Fatal(err)
	}
	if y.DType != Float64 || y.Get(2) != 3 {
		t.Fatalf("FromData64 errado: dtype=%v get2=%v", y.DType, y.Get(2))
	}
	if _, err := FromData64([]float64{1, 2}, []int{3}); err == nil {
		t.Fatal("FromData64 com mismatch deveria falhar")
	}

	// default é Float32
	z := New([]int{2})
	if z.DType != Float32 {
		t.Fatalf("New deve ser Float32 por default, veio %v", z.DType)
	}
	// AsDType converte f32->f64 preservando valores
	z.Data[0] = 2.5
	z64 := z.AsDType(Float64)
	if z64.DType != Float64 || z64.Get(0) != 2.5 {
		t.Fatalf("AsDType errado: dtype=%v get0=%v", z64.DType, z64.Get(0))
	}
}

func TestFloat64Ops(t *testing.T) {
	// Add/Mul em f64
	a, _ := FromData64([]float64{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData64([]float64{5, 6, 7, 8}, []int{2, 2})
	sum, _ := a.Add(b)
	if sum.DType != Float64 || sum.Get(3) != 12 {
		t.Fatalf("Add f64: dtype=%v get3=%v", sum.DType, sum.Get(3))
	}
	// MatMul f64
	mm, _ := a.MatMul(b) // [[1,2],[3,4]]·[[5,6],[7,8]] = [[19,22],[43,50]]
	if mm.DType != Float64 || mm.Get(0) != 19 || mm.Get(3) != 50 {
		t.Fatalf("MatMul f64 errado: %v %v", mm.Get(0), mm.Get(3))
	}
	// Transpose f64
	tp, _ := a.Transpose()
	if tp.Get(1) != 3 { // a=[[1,2],[3,4]] -> T=[[1,3],[2,4]], idx1 = 3
		t.Fatalf("Transpose f64 idx1 = %v, quer 3", tp.Get(1))
	}
	// Reshape preserva dtype
	rs, _ := a.Reshape([]int{4})
	if rs.DType != Float64 || rs.Get(2) != 3 {
		t.Fatalf("Reshape f64: dtype=%v get2=%v", rs.DType, rs.Get(2))
	}
	// promoção: f32 MatMul f64 -> f64
	af32, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	pr, _ := af32.MatMul(b)
	if pr.DType != Float64 {
		t.Fatalf("MatMul f32×f64 deveria promover a f64, veio %v", pr.DType)
	}
	// Dot e Norm
	d, _ := a.Dot(b) // 1*5+2*6+3*7+4*8 = 70
	if d != 70 {
		t.Fatalf("Dot = %v, quer 70", d)
	}
	c, _ := FromData64([]float64{3, 4}, []int{2})
	if c.Norm() != 5 {
		t.Fatalf("Norm = %v, quer 5", c.Norm())
	}
}

func TestFloat64Precision(t *testing.T) {
	// Somar 1 a um valor grande e subtrair: f64 preserva melhor que f32.
	// Constrói vetor [1e8, 1, -1e8] e soma via Dot com vetor de 1s.
	big := 1e8
	dataF64 := []float64{big, 1, -big}
	ones64, _ := FromData64([]float64{1, 1, 1}, []int{3})
	v64, _ := FromData64(dataF64, []int{3})
	got64, _ := v64.Dot(ones64) // esperado exatamente 1.0 em f64

	dataF32 := []float32{float32(big), 1, float32(-big)}
	ones32, _ := FromData([]float32{1, 1, 1}, []int{3})
	v32, _ := FromData(dataF32, []int{3})
	got32, _ := v32.Dot(ones32)

	errF64 := math.Abs(got64 - 1.0)
	errF32 := math.Abs(got32 - 1.0)
	if errF64 >= errF32 {
		t.Fatalf("f64 deveria ter erro menor: errF64=%v errF32=%v", errF64, errF32)
	}
	if errF64 > 1e-6 {
		t.Fatalf("f64 impreciso demais: erro=%v (got=%v)", errF64, got64)
	}
}
