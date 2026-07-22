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
