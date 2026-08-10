package vm

import (
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestAllwaysFalse(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ALLWAYSFALSE"].Fn(nil)
	if err != nil {
		t.Fatalf("AllwaysFalse retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("AllwaysFalse() = %v, quer .F.", got)
	}
}

func TestAllwaysTrue(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ALLWAYSTRUE"].Fn(nil)
	if err != nil {
		t.Fatalf("AllwaysTrue retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("AllwaysTrue() = %v, quer .T.", got)
	}
}

func TestEmpty(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		arg  advplrt.Value
		want bool
	}{
		// String: vazia ou apenas espaços ASCII 32 (não tabs/newlines)
		{"string vazia", advplrt.NewString(""), true},
		{"string com espacos", advplrt.NewString("   "), true},
		{"string com espaco e conteudo", advplrt.NewString(" a "), false},
		{"string com conteudo", advplrt.NewString("abc"), false},
		{"string com tab (ASCII 9)", advplrt.NewString("\t"), false}, // TDN: NÃO é vazio (só ASCII 32)
		{"string com newline (ASCII 10)", advplrt.NewString("\n"), false}, // TDN: NÃO é vazio

		// Numérico: zero ou não-zero
		{"numero zero", advplrt.NewNumber(0), true},
		{"numero nao-zero", advplrt.NewNumber(5), false},
		{"numero negativo", advplrt.NewNumber(-3), false},

		// Lógico: falso é vazio, verdadeiro não é
		{"logico falso", advplrt.NewBool(false), true},
		{"logico verdadeiro", advplrt.NewBool(true), false},

		// Data: zero-value é vazia, qualquer outra não é
		{"data zero", advplrt.NewDate(time.Time{}), true},
		{"data nao-zero", advplrt.NewDate(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)), false},

		// Array: vazio ou com elementos
		{"array vazio", advplrt.NewArray([]advplrt.Value{}), true},
		{"array com um elemento", advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(1)}), false},

		// CodeBlock: sempre não-vazio (retorna false per TDN: "Sempre retorna falso ( .F. )")
		{"codeblock", &advplrt.CodeBlockValue{FuncName: "test"}, false},

		// Nil: sempre vazio
		{"nil", advplrt.Nil, true},
	}
	for _, c := range cases {
		got, err := v.natives["EMPTY"].Fn([]advplrt.Value{c.arg})
		if err != nil {
			t.Fatalf("Empty(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok || b.Val != c.want {
			t.Errorf("Empty(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}
