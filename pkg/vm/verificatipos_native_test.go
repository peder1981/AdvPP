package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestValType tests the VALTYPE native function.
// VALTYPE takes a value directly and returns its type.
func TestValTypeCharacter(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["VALTYPE"].Fn([]advplrt.Value{advplrt.NewString("test")})
	if err != nil {
		t.Fatalf("VALTYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "C" {
		t.Errorf("VALTYPE(string) = %q, quer %q (Caractere)", result.(*advplrt.StringValue).Val, "C")
	}
}

func TestValTypeNumeric(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["VALTYPE"].Fn([]advplrt.Value{advplrt.NewNumber(123.45)})
	if err != nil {
		t.Fatalf("VALTYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "N" {
		t.Errorf("VALTYPE(number) = %q, quer %q (Numerico)", result.(*advplrt.StringValue).Val, "N")
	}
}

func TestValTypeLogical(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["VALTYPE"].Fn([]advplrt.Value{advplrt.True})
	if err != nil {
		t.Fatalf("VALTYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "L" {
		t.Errorf("VALTYPE(.T.) = %q, quer %q (Logico)", result.(*advplrt.StringValue).Val, "L")
	}
}

func TestValTypeArray(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	arr := advplrt.NewArray([]advplrt.Value{})
	result, err := v.natives["VALTYPE"].Fn([]advplrt.Value{arr})
	if err != nil {
		t.Fatalf("VALTYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "A" {
		t.Errorf("VALTYPE(array) = %q, quer %q (Array)", result.(*advplrt.StringValue).Val, "A")
	}
}

func TestValTypeNil(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	result, err := v.natives["VALTYPE"].Fn([]advplrt.Value{advplrt.Nil})
	if err != nil {
		t.Fatalf("VALTYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "U" {
		t.Errorf("VALTYPE(nil) = %q, quer %q (Nil/Undefined)", result.(*advplrt.StringValue).Val, "U")
	}
}

// TestType tests the TYPE native function.
// TYPE takes a string expression and looks up the variable/value.
func TestTypePublicVariable(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Set up a PUBLIC variable in dynEnv
	v.dynEnv["myvar"] = advplrt.NewString("test")

	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("myvar")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "C" {
		t.Errorf("TYPE('myvar') where myvar='test' = %q, quer %q (Caractere)", result.(*advplrt.StringValue).Val, "C")
	}
}

func TestTypePublicNumericVariable(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Set up a PUBLIC numeric variable
	v.dynEnv["nValue"] = advplrt.NewNumber(42.5)

	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("nValue")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "N" {
		t.Errorf("TYPE('nValue') where nValue=42.5 = %q, quer %q (Numerico)", result.(*advplrt.StringValue).Val, "N")
	}
}

func TestTypeUndefinedVariable(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Don't set any variable

	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("undefined_var")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "U" {
		t.Errorf("TYPE('undefined_var') = %q, quer %q (Undefined)", result.(*advplrt.StringValue).Val, "U")
	}
}

func TestTypeEmptyString(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Empty string parameter

	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "U" {
		t.Errorf("TYPE('') = %q, quer %q (Undefined)", result.(*advplrt.StringValue).Val, "U")
	}
}
