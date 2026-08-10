package vm

import (
	"testing"

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
		{"string vazia", advplrt.NewString(""), true},
		{"string com espacos", advplrt.NewString("   "), true},
		{"string com conteudo", advplrt.NewString("abc"), false},
		{"numero zero", advplrt.NewNumber(0), true},
		{"numero nao-zero", advplrt.NewNumber(5), false},
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
