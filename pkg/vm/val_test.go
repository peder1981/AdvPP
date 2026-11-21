package vm

import (
	"math"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Val lê o número do INÍCIO do texto, como no Protheus/Clipper: espaços à
// esquerda, sinal, dígitos e um ponto decimal; para no primeiro caractere
// inválido; sem número, 0. Não aceita expoente, NaN, Inf, hexadecimal.
func TestValLeNumeroDoInicio(t *testing.T) {
	val := NewVM(&compiler.Bytecode{}, false).natives["VAL"].Fn
	casos := []struct {
		in   string
		want float64
	}{
		{"123", 123}, {"  42", 42}, {"12.5", 12.5}, {"-3.25", -3.25}, {"+7", 7},
		{"2.88 3.11 2.92", 2.88},         // /proc/loadavg
		{"       36676076 kB", 36676076}, // /proc/meminfo
		{"12abc", 12}, {"1,5", 1}, {".5", 0.5}, {"-.5", -0.5}, {"5.", 5},
		{"1e5", 1}, {"NaN", 0}, {"Inf", 0}, {"0x10", 0}, {"abc", 0}, {"", 0}, {"-", 0}, {".", 0},
		{"1.2.3", 1.2}, {"\t9", 9},
	}
	for _, c := range casos {
		v, err := val([]advplrt.Value{advplrt.NewString(c.in)})
		if err != nil {
			t.Fatal(err)
		}
		got := advplrt.ToFloat(v)
		if got != c.want || math.IsNaN(got) {
			t.Errorf("Val(%q) = %v, esperado %v", c.in, got, c.want)
		}
	}
}
