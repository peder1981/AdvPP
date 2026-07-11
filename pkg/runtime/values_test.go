package advplrt

import "testing"

// TestNewNumberValue confere que NewNumber sempre devolve o valor correto,
// dentro e fora da faixa cacheada (numberCacheMin..numberCacheMax) —
// inteiros e fracionários.
func TestNewNumberValue(t *testing.T) {
	cases := []float64{
		numberCacheMin, numberCacheMin + 1, -1, 0, 1, 7,
		numberCacheMax - 1, numberCacheMax,
		numberCacheMin - 1, numberCacheMax + 1, // fora da faixa cacheada
		1e9, -1e9, // bem fora da faixa
		0.5, 3.14, -2.5, // fracionários, mesmo dentro da faixa de inteiros
	}
	for _, v := range cases {
		n := NewNumber(v)
		if n.Val != v {
			t.Errorf("NewNumber(%v).Val = %v, want %v", v, n.Val, v)
		}
	}
}

// TestNewNumberCacheSharing confere que valores inteiros dentro da faixa
// cacheada compartilham o MESMO ponteiro entre chamadas (é o ponto da
// otimização — evitar alocação repetida), e que isso não quebra igualdade
// por valor via Equals.
func TestNewNumberCacheSharing(t *testing.T) {
	a := NewNumber(42)
	b := NewNumber(42)
	if a != b {
		t.Errorf("NewNumber(42) chamado duas vezes devolveu ponteiros diferentes: %p vs %p", a, b)
	}
	if !a.Equals(b) {
		t.Error("a.Equals(b) = false para o mesmo valor")
	}

	// fora da faixa cacheada, não precisa (nem deveria) compartilhar
	// ponteiro, mas o valor e Equals continuam corretos.
	c := NewNumber(1e9)
	d := NewNumber(1e9)
	if !c.Equals(d) {
		t.Error("c.Equals(d) = false para o mesmo valor fora da faixa cacheada")
	}
}

// TestNewNumberCacheImmutable confere que ler o valor cacheado repetidas
// vezes nunca muda — protege contra uma futura mudança que passe a mutar
// NumberValue.Val in-place em vez de sempre criar um novo (o que corromperia
// TODO outro lugar do VM segurando o mesmo ponteiro cacheado).
func TestNewNumberCacheImmutable(t *testing.T) {
	n1 := NewNumber(5)
	n2 := NewNumber(10)
	if n1.Val != 5 {
		t.Fatalf("n1.Val = %v, want 5", n1.Val)
	}
	if n2.Val != 10 {
		t.Fatalf("n2.Val = %v, want 10", n2.Val)
	}
	// pegar n1 de novo não pode ter sido afetado por criar n2
	n1Again := NewNumber(5)
	if n1Again.Val != 5 {
		t.Fatalf("n1Again.Val = %v, want 5 (cache corrompido por outra chamada)", n1Again.Val)
	}
}
