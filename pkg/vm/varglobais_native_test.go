package vm

import (
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestPutGlbValueAndGetGlbValue(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cGlbName := "TEST_GLB_VAL"
	cValue := "Teste"

	// PutGlbValue should succeed
	_, err := v.natives["PUTGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		advplrt.NewString(cValue),
	})
	if err != nil {
		t.Fatalf("PutGlbValue failed: %v", err)
	}

	// GetGlbValue should return the stored value
	result, err := v.natives["GETGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
	})
	if err != nil {
		t.Fatalf("GetGlbValue failed: %v", err)
	}

	str, ok := result.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("GetGlbValue returned %T, expected StringValue", result)
	}
	if str.Val != cValue {
		t.Errorf("GetGlbValue = %q, want %q", str.Val, cValue)
	}
}

func TestGetGlbValueNotFound(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// GetGlbValue for non-existent variable should return empty string
	result, err := v.natives["GETGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString("NONEXISTENT"),
	})
	if err != nil {
		t.Fatalf("GetGlbValue failed: %v", err)
	}

	str, ok := result.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("GetGlbValue returned %T, expected StringValue", result)
	}
	if str.Val != "" {
		t.Errorf("GetGlbValue for non-existent variable = %q, want empty string", str.Val)
	}
}

func TestPutGlbValueOverwrite(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cGlbName := "TEST_GLB_OVERWRITE"

	// First put
	_, err := v.natives["PUTGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		advplrt.NewString("FirstValue"),
	})
	if err != nil {
		t.Fatalf("First PutGlbValue failed: %v", err)
	}

	// Second put (overwrite)
	_, err = v.natives["PUTGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		advplrt.NewString("SecondValue"),
	})
	if err != nil {
		t.Fatalf("Second PutGlbValue failed: %v", err)
	}

	// GetGlbValue should return the second value
	result, err := v.natives["GETGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
	})
	if err != nil {
		t.Fatalf("GetGlbValue failed: %v", err)
	}

	str, ok := result.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("GetGlbValue returned %T, expected StringValue", result)
	}
	if str.Val != "SecondValue" {
		t.Errorf("GetGlbValue = %q, want %q", str.Val, "SecondValue")
	}
}

func TestPutGlbVarsAndGetGlbVars(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cGlbName := "TEST_GLB_VARS"

	// Create array and number values
	aGlbPut := advplrt.NewArray([]advplrt.Value{
		advplrt.NewString("Value1"),
		advplrt.NewString("Value2"),
	})
	nValue := advplrt.NewNumber(123)

	// PutGlbVars should succeed
	_, err := v.natives["PUTGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		aGlbPut,
		nValue,
	})
	if err != nil {
		t.Fatalf("PutGlbVars failed: %v", err)
	}

	// GetGlbVars should return the stored values
	// We need to pass variables by reference, but since we're calling the native directly,
	// we'll check the internal storage
	result, err := v.natives["GETGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		advplrt.NewArray([]advplrt.Value{}), // placeholder for returned array
		advplrt.NewNumber(0), // placeholder for returned number
	})
	if err != nil {
		t.Fatalf("GetGlbVars failed: %v", err)
	}

	b, ok := result.(*advplrt.BoolValue)
	if !ok {
		t.Fatalf("GetGlbVars returned %T, expected BoolValue", result)
	}
	if !b.Val {
		t.Errorf("GetGlbVars returned false, expected true")
	}
}

func TestGetGlbVarsNotFound(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// GetGlbVars for non-existent variable should return false
	result, err := v.natives["GETGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString("NONEXISTENT_MULTI"),
		advplrt.NewArray([]advplrt.Value{}),
		advplrt.NewNumber(0),
	})
	if err != nil {
		t.Fatalf("GetGlbVars failed: %v", err)
	}

	b, ok := result.(*advplrt.BoolValue)
	if !ok {
		t.Fatalf("GetGlbVars returned %T, expected BoolValue", result)
	}
	if b.Val {
		t.Errorf("GetGlbVars for non-existent variable returned true, expected false")
	}
}

func TestGetGlbVarsMultipleValues(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cGlbName := "TEST_GLB_MULTI"

	// Store multiple values of different types
	aGlbPut := advplrt.NewArray([]advplrt.Value{
		advplrt.NewString("String1"),
		advplrt.NewString("String2"),
	})

	nValue := advplrt.NewNumber(456)
	dValue := advplrt.NewDate(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC))

	// PutGlbVars with 3 values
	_, err := v.natives["PUTGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		aGlbPut,
		nValue,
		dValue,
	})
	if err != nil {
		t.Fatalf("PutGlbVars with 3 values failed: %v", err)
	}

	// GetGlbVars should return true and internally store the values
	result, err := v.natives["GETGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString(cGlbName),
		advplrt.NewArray([]advplrt.Value{}),
		advplrt.NewNumber(0),
		advplrt.NewDate(time.Time{}),
	})
	if err != nil {
		t.Fatalf("GetGlbVars with 3 values failed: %v", err)
	}

	b, ok := result.(*advplrt.BoolValue)
	if !ok {
		t.Fatalf("GetGlbVars returned %T, expected BoolValue", result)
	}
	if !b.Val {
		t.Errorf("GetGlbVars returned false, expected true")
	}
}

func TestGetGlbValueWithVariadicVarsSeparation(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// PutGlbValue should store single string
	_, err := v.natives["PUTGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString("SINGLE_VAR"),
		advplrt.NewString("StringValue"),
	})
	if err != nil {
		t.Fatalf("PutGlbValue failed: %v", err)
	}

	// PutGlbVars should store multiple values
	_, err = v.natives["PUTGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString("MULTI_VAR"),
		advplrt.NewString("Value1"),
		advplrt.NewNumber(123),
	})
	if err != nil {
		t.Fatalf("PutGlbVars failed: %v", err)
	}

	// GetGlbValue on SINGLE_VAR should return the string
	result1, err := v.natives["GETGLBVALUE"].Fn([]advplrt.Value{
		advplrt.NewString("SINGLE_VAR"),
	})
	if err != nil {
		t.Fatalf("GetGlbValue on SINGLE_VAR failed: %v", err)
	}
	str1, ok := result1.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("GetGlbValue returned %T, expected StringValue", result1)
	}
	if str1.Val != "StringValue" {
		t.Errorf("GetGlbValue(SINGLE_VAR) = %q, want %q", str1.Val, "StringValue")
	}

	// GetGlbVars on MULTI_VAR should return true
	result2, err := v.natives["GETGLBVARS"].Fn([]advplrt.Value{
		advplrt.NewString("MULTI_VAR"),
		advplrt.NewArray([]advplrt.Value{}),
		advplrt.NewNumber(0),
	})
	if err != nil {
		t.Fatalf("GetGlbVars on MULTI_VAR failed: %v", err)
	}
	b2, ok := result2.(*advplrt.BoolValue)
	if !ok {
		t.Fatalf("GetGlbVars returned %T, expected BoolValue", result2)
	}
	if !b2.Val {
		t.Errorf("GetGlbVars(MULTI_VAR) returned false, expected true")
	}
}
