package vm

import (
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// relCmp deve comparar strings lexicograficamente (por byte) quando ambos os
// operandos são strings, e numericamente caso contrário. Antes, strings caíam
// em ToFloat (→0), fazendo " " >= "A" retornar verdadeiro (0>=0).
func TestRelCmpStrings(t *testing.T) {
	str := func(s string) advplrt.Value { return advplrt.NewString(s) }
	num := func(n float64) advplrt.Value { return advplrt.NewNumber(n) }

	cases := []struct {
		name string
		a, b advplrt.Value
		want int // sinal esperado
	}{
		{"espaco < A", str(" "), str("A"), -1},
		{"A > espaco", str("A"), str(" "), 1},
		{"iguais", str("abc"), str("abc"), 0},
		{"prefixo menor", str("ab"), str("abc"), -1},
		{"maiuscula < minuscula", str("A"), str("a"), -1}, // 'A'=65 < 'a'=97
		{"digito < letra", str("0"), str("A"), -1},        // '0'=48 < 'A'=65
		{"num 2 < 10", num(2), num(10), -1},
		{"num 10 > 2", num(10), num(2), 1},
		{"num iguais", num(5), num(5), 0},
	}
	for _, c := range cases {
		got := relCmp(c.a, c.b)
		if sign(got) != c.want {
			t.Errorf("%s: relCmp = %d (sinal %d), quer sinal %d", c.name, got, sign(got), c.want)
		}
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}
