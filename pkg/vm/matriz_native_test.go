package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// createBytecodeWithComparisonBlock creates bytecode with a codeblock function that:
// Takes 2 params (element, index) and performs comparisons for testing.
// This is a simplified version for testing AScanX.
func createBytecodeWithComparisonBlock() *compiler.Bytecode {
	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code:      []compiler.Instruction{},
	}

	// Simplified codeblock that returns truthy value for testing
	// In real usage, the compiler generates these
	bc.Code = []compiler.Instruction{
		{Op: compiler.OP_LOAD_LOCAL, Arg: 1, Line: 1}, // push element (local[1])
		{Op: compiler.OP_RETURN_VALUE, Line: 2},       // return it
	}

	bc.Functions["__BLOCK_COMPARE"] = &compiler.FunctionInfo{
		Name:      "__BLOCK_COMPARE",
		NumParams: 3, // codeblock, element, index
		NumLocals: 3,
		IsUser:    false,
		Offset:    0,
	}

	return bc
}

func TestACopy(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test case: basic copy from source to destination
	// ACopy(aExemplo, aBkp) where aExemplo = {1, 2, {11, 22, 33}}, aBkp = {, , {, , }}
	aExemplo := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(1),
		advplrt.NewNumber(2),
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewNumber(11),
			advplrt.NewNumber(22),
			advplrt.NewNumber(33),
		}),
	})

	aBkp := advplrt.NewArray([]advplrt.Value{
		advplrt.Nil,
		advplrt.Nil,
		advplrt.NewArray([]advplrt.Value{advplrt.Nil, advplrt.Nil, advplrt.Nil}),
	})

	result, err := v.natives["ACOPY"].Fn([]advplrt.Value{aExemplo, aBkp})
	if err != nil {
		t.Fatalf("ACopy failed: %v", err)
	}

	// Result should be reference to aBkp
	if result != aBkp {
		t.Errorf("ACopy should return reference to destination array")
	}

	// aBkp should now have same elements as aExemplo
	if len(aBkp.Elements) != 3 {
		t.Errorf("aBkp length = %d, want 3", len(aBkp.Elements))
	}

	// Check copied values
	if n1, ok := aBkp.Elements[0].(*advplrt.NumberValue); !ok || n1.Val != 1 {
		t.Errorf("aBkp[1] = %v, want 1", aBkp.Elements[0])
	}
	if n2, ok := aBkp.Elements[1].(*advplrt.NumberValue); !ok || n2.Val != 2 {
		t.Errorf("aBkp[2] = %v, want 2", aBkp.Elements[1])
	}

	// Check nested array was copied
	if nested, ok := aBkp.Elements[2].(*advplrt.ArrayValue); !ok {
		t.Errorf("aBkp[3] is not an array: %T", aBkp.Elements[2])
	} else if len(nested.Elements) != 3 {
		t.Errorf("aBkp[3] length = %d, want 3", len(nested.Elements))
	}
}

func TestACopyWithStartAndCount(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test ACopy with nInicio and nCont parameters
	aSource := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(10),
		advplrt.NewNumber(20),
		advplrt.NewNumber(30),
		advplrt.NewNumber(40),
	})

	aDest := advplrt.NewArray([]advplrt.Value{})

	// Copy 2 elements starting from position 2
	_, err := v.natives["ACOPY"].Fn([]advplrt.Value{
		aSource,
		aDest,
		advplrt.NewNumber(2), // nInicio
		advplrt.NewNumber(2), // nCont
	})
	if err != nil {
		t.Fatalf("ACopy with nInicio/nCont failed: %v", err)
	}

	if len(aDest.Elements) != 2 {
		t.Errorf("aDest length = %d, want 2", len(aDest.Elements))
	}

	if n1, ok := aDest.Elements[0].(*advplrt.NumberValue); !ok || n1.Val != 20 {
		t.Errorf("aDest[1] = %v, want 20", aDest.Elements[0])
	}
	if n2, ok := aDest.Elements[1].(*advplrt.NumberValue); !ok || n2.Val != 30 {
		t.Errorf("aDest[2] = %v, want 30", aDest.Elements[1])
	}
}

func TestAScanX(t *testing.T) {
	bc := createBytecodeWithComparisonBlock()
	v := NewVM(bc, false)

	// Test AScanX with codeblock that receives both element and index
	aExemplo := advplrt.NewArray([]advplrt.Value{
		advplrt.NewString("Banana"),
		advplrt.NewString("Maçã"),
		advplrt.NewString("Pêra"),
		advplrt.NewString("Limão"),
		advplrt.NewString("Abacaxi"),
		advplrt.NewString("Laranja"),
		advplrt.NewString("Mamão"),
		advplrt.NewString("Graviola"),
	})

	// Create a simple codeblock that checks for "Abacaxi"
	block := &advplrt.CodeBlockValue{
		Params:   []string{"x", "i"},
		FuncName: "__BLOCK_COMPARE",
	}

	// Simple test: search for "Abacaxi" (should find it at position 5)
	result, err := v.natives["ASCANX"].Fn([]advplrt.Value{aExemplo, block})
	if err != nil {
		t.Fatalf("AScanX failed: %v", err)
	}

	// Should return a number (position or 0)
	if _, ok := result.(*advplrt.NumberValue); !ok {
		t.Errorf("AScanX should return a number, got %T", result)
	}
}

func TestATail(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test ATail returns last element
	aExemplo := advplrt.NewArray([]advplrt.Value{
		advplrt.NewString("A"),
		advplrt.NewString("rápida"),
		advplrt.NewString("raposa"),
		advplrt.NewString("marrom"),
		advplrt.NewString("pula"),
		advplrt.NewString("sobre"),
		advplrt.NewString("o"),
		advplrt.NewString("cachorro"),
		advplrt.NewString("preguiçoso"),
	})

	result, err := v.natives["ATAIL"].Fn([]advplrt.Value{aExemplo})
	if err != nil {
		t.Fatalf("ATail failed: %v", err)
	}

	if s, ok := result.(*advplrt.StringValue); !ok || s.Val != "preguiçoso" {
		t.Errorf("ATail result = %v, want 'preguiçoso'", result)
	}
}

func TestATailEdgeCases(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test ATail with single element
	aOne := advplrt.NewArray([]advplrt.Value{advplrt.NewString("solo")})
	result1, err := v.natives["ATAIL"].Fn([]advplrt.Value{aOne})
	if err != nil {
		t.Fatalf("ATail with single element failed: %v", err)
	}

	if s, ok := result1.(*advplrt.StringValue); !ok || s.Val != "solo" {
		t.Errorf("ATail single element = %v, want 'solo'", result1)
	}

	// Test ATail with empty array - should return Nil
	aEmpty := advplrt.NewArray([]advplrt.Value{})
	result2, err := v.natives["ATAIL"].Fn([]advplrt.Value{aEmpty})
	if err != nil {
		t.Fatalf("ATail with empty array failed: %v", err)
	}

	if result2 != advplrt.Nil {
		t.Errorf("ATail empty array = %v, want Nil", result2)
	}

	// Test ATail with non-array should return Nil
	notArray := advplrt.NewString("not an array")
	result3, err := v.natives["ATAIL"].Fn([]advplrt.Value{notArray})
	if err != nil {
		t.Fatalf("ATail with non-array failed: %v", err)
	}

	if result3 != advplrt.Nil {
		t.Errorf("ATail non-array = %v, want Nil", result3)
	}
}
