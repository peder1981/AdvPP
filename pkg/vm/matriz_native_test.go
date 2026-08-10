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

	// Test ACopy with nInicio and nCont parameters.
	// Per TDN spec, destination array must be pre-sized; ACopy does NOT resize it.
	aSource := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(10),
		advplrt.NewNumber(20),
		advplrt.NewNumber(30),
		advplrt.NewNumber(40),
	})

	// Pre-sized destination array with exactly 2 elements for the copy
	aDest := advplrt.NewArray([]advplrt.Value{
		advplrt.Nil,
		advplrt.Nil,
	})

	initialLen := len(aDest.Elements)

	// Copy 2 elements starting from position 2 in source (20, 30)
	_, err := v.natives["ACOPY"].Fn([]advplrt.Value{
		aSource,
		aDest,
		advplrt.NewNumber(2), // nInicio
		advplrt.NewNumber(2), // nCont
	})
	if err != nil {
		t.Fatalf("ACopy with nInicio/nCont failed: %v", err)
	}

	// Verify destination array size did NOT change (per spec)
	if len(aDest.Elements) != initialLen {
		t.Errorf("aDest length changed from %d to %d (should not resize)", initialLen, len(aDest.Elements))
	}

	// Verify the copied values
	if n1, ok := aDest.Elements[0].(*advplrt.NumberValue); !ok || n1.Val != 20 {
		t.Errorf("aDest[1] = %v, want 20", aDest.Elements[0])
	}
	if n2, ok := aDest.Elements[1].(*advplrt.NumberValue); !ok || n2.Val != 30 {
		t.Errorf("aDest[2] = %v, want 30", aDest.Elements[1])
	}
}

func TestACopyRespectsBoundary(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test that ACopy respects destination array size boundary.
	// Request copy of 5 elements into a 3-element destination; only 3 should be copied.
	aSource := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(1),
		advplrt.NewNumber(2),
		advplrt.NewNumber(3),
		advplrt.NewNumber(4),
		advplrt.NewNumber(5),
	})

	// Destination array with exactly 3 positions
	aDest := advplrt.NewArray([]advplrt.Value{
		advplrt.Nil,
		advplrt.Nil,
		advplrt.Nil,
	})

	// Try to copy all 5 elements from source (should only copy first 3, respecting dest boundary)
	_, err := v.natives["ACOPY"].Fn([]advplrt.Value{
		aSource,
		aDest,
		advplrt.NewNumber(1), // nInicio
		advplrt.NewNumber(5), // nCont (5 elements requested)
		// nPosDestino defaults to 1
	})
	if err != nil {
		t.Fatalf("ACopy respects boundary test failed: %v", err)
	}

	// Only first 3 elements should be copied (boundary respected)
	if len(aDest.Elements) != 3 {
		t.Errorf("aDest length = %d, want 3", len(aDest.Elements))
	}

	// Verify elements 1-3 were copied
	for i := 0; i < 3; i++ {
		if n, ok := aDest.Elements[i].(*advplrt.NumberValue); !ok || n.Val != float64(i+1) {
			t.Errorf("aDest[%d] = %v, want %d", i+1, aDest.Elements[i], i+1)
		}
	}
}

func TestAScanXReturnsCorrectPosition(t *testing.T) {
	// Test AScanX with numeric array and codeblock that checks element value
	aNumbers := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(10),
		advplrt.NewNumber(20),
		advplrt.NewNumber(30),
		advplrt.NewNumber(40),
		advplrt.NewNumber(50),
	})

	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code: []compiler.Instruction{
			// Simple block that pushes local[1] (element) and returns it (truthy if non-zero)
			{Op: compiler.OP_LOAD_LOCAL, Arg: 1, Line: 1},
			{Op: compiler.OP_RETURN_VALUE, Line: 2},
		},
	}
	bc.Functions["__BLOCK_TEST_ALWAYS"] = &compiler.FunctionInfo{
		Name:      "__BLOCK_TEST_ALWAYS",
		NumParams: 3,
		NumLocals: 3,
		IsUser:    false,
		Offset:    0,
	}
	vWithBc := NewVM(bc, false)

	// Test with array that has non-zero/truthy elements
	// The first non-zero element is at position 1 (10), which is truthy
	result, err := vWithBc.natives["ASCANX"].Fn([]advplrt.Value{
		aNumbers,
		&advplrt.CodeBlockValue{
			Params:   []string{"x", "i"},
			FuncName: "__BLOCK_TEST_ALWAYS",
		},
	})
	if err != nil {
		t.Fatalf("AScanX failed: %v", err)
	}

	// Should return position 1 (first element 10 is truthy)
	n, ok := result.(*advplrt.NumberValue)
	if !ok {
		t.Fatalf("AScanX should return a number, got %T", result)
	}
	if n.Val != 1 {
		t.Errorf("AScanX returned position %v, want 1 (first truthy element)", n.Val)
	}
}

func TestAScanXReturnsZeroWhenNotFound(t *testing.T) {
	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code: []compiler.Instruction{
			// Block that returns Nil (falsy) for all elements
			{Op: compiler.OP_NIL, Line: 1},
			{Op: compiler.OP_RETURN_VALUE, Line: 2},
		},
	}
	bc.Functions["__BLOCK_TEST_NEVER"] = &compiler.FunctionInfo{
		Name:      "__BLOCK_TEST_NEVER",
		NumParams: 3,
		NumLocals: 3,
		IsUser:    false,
		Offset:    0,
	}
	v := NewVM(bc, false)

	aNumbers := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(10),
		advplrt.NewNumber(20),
		advplrt.NewNumber(30),
	})

	// Codeblock that never matches (always returns Nil/falsy)
	result, err := v.natives["ASCANX"].Fn([]advplrt.Value{
		aNumbers,
		&advplrt.CodeBlockValue{
			Params:   []string{"x", "i"},
			FuncName: "__BLOCK_TEST_NEVER",
		},
	})
	if err != nil {
		t.Fatalf("AScanX failed: %v", err)
	}

	// Should return 0 (not found)
	n, ok := result.(*advplrt.NumberValue)
	if !ok {
		t.Fatalf("AScanX should return a number, got %T", result)
	}
	if n.Val != 0 {
		t.Errorf("AScanX returned %v, want 0 (not found)", n.Val)
	}
}

func TestAScanXWithIndexParameter(t *testing.T) {
	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code: []compiler.Instruction{
			// Block that loads local[2] (the index parameter) and returns it
			// This will be truthy for indices >= 2
			{Op: compiler.OP_LOAD_LOCAL, Arg: 2, Line: 1},
			{Op: compiler.OP_RETURN_VALUE, Line: 2},
		},
	}
	bc.Functions["__BLOCK_TEST_INDEX"] = &compiler.FunctionInfo{
		Name:      "__BLOCK_TEST_INDEX",
		NumParams: 3,
		NumLocals: 3,
		IsUser:    false,
		Offset:    0,
	}
	v := NewVM(bc, false)

	aNumbers := advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(10),
		advplrt.NewNumber(20),
		advplrt.NewNumber(30),
		advplrt.NewNumber(40),
	})

	// Start from position 2, search for first element where block returns truthy
	// Block returns the index value, which is >= 2 starting from position 2
	result, err := v.natives["ASCANX"].Fn([]advplrt.Value{
		aNumbers,
		&advplrt.CodeBlockValue{
			Params:   []string{"x", "i"},
			FuncName: "__BLOCK_TEST_INDEX",
		},
		advplrt.NewNumber(2), // nStart = 2
		advplrt.NewNumber(2), // nCount = 2 (search positions 2-3 only)
	})
	if err != nil {
		t.Fatalf("AScanX with nStart/nCount failed: %v", err)
	}

	// Should return 2 (first position in range 2-3 where index is >= 2, i.e., position 2)
	n, ok := result.(*advplrt.NumberValue)
	if !ok {
		t.Fatalf("AScanX should return a number, got %T", result)
	}
	if n.Val != 2 {
		t.Errorf("AScanX with nStart=2, nCount=2 returned %v, want 2", n.Val)
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
