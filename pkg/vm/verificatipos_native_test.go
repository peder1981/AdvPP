package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
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

// TestTypePublicVariableWithAliasOpen tests TYPE() for PRIVATE/PUBLIC variables
// when a database alias is open (regression test for field-priority lookup).
// This ensures TYPE("myvar") returns the correct type even if an alias is open,
// and doesn't silently fail by checking a non-existent field.
func TestTypePublicVariableWithAliasOpen(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/type_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	// Create a test table with specific columns
	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}

	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)

	// Select the alias to open it
	if err := eng.SelectArea("UNI"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	v.currentAlias = "UNI"

	// Set a PRIVATE/PUBLIC variable with a name that is NOT a column in UNI
	// (existing columns are: R_E_C_N_O_, D_E_L_E_T_, UNI_CODIGO, FILIAL)
	v.dynEnv["myPublicVar"] = advplrt.NewString("test_value")

	// TYPE("myPublicVar") should return "C" (character), not "U" (undefined)
	// This would fail with the old implementation which returned "U" because
	// FieldGet always returns nil error even for non-existent fields.
	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("myPublicVar")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "C" {
		t.Errorf("TYPE('myPublicVar') com alias aberto = %q, quer %q (Caractere)\n"+
			"BUG: FieldPos/FieldGet não detectaram que 'myPublicVar' não é um campo,\n"+
			"então caiu na verificação de dynEnv e retornou o tipo correto.",
			result.(*advplrt.StringValue).Val, "C")
	}
}

// TestTypeFieldShadowsVariable tests that TYPE() returns the field's type
// when both a field and a variable have the same name (field takes priority).
func TestTypeFieldShadowsVariable(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/type_shadow_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	// Create a test table
	if err := eng.Exec(`CREATE TABLE TEST (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		TEST_FIELD TEXT
	)`); err != nil {
		t.Fatalf("create TEST: %v", err)
	}

	if err := eng.Exec("INSERT INTO TEST (TEST_FIELD) VALUES ('field_value')"); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)

	// Select the alias
	if err := eng.SelectArea("TEST"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	v.currentAlias = "TEST"

	// Set a PRIVATE/PUBLIC variable with the SAME name as a field
	// The field is TEST_FIELD (character), the variable is numeric
	v.dynEnv["TEST_FIELD"] = advplrt.NewNumber(999)

	// TYPE("TEST_FIELD") should return "C" (character from field),
	// NOT "N" (numeric from variable), because fields shadow variables
	result, err := v.natives["TYPE"].Fn([]advplrt.Value{advplrt.NewString("TEST_FIELD")})
	if err != nil {
		t.Fatalf("TYPE retornou erro: %v", err)
	}
	if result.(*advplrt.StringValue).Val != "C" {
		t.Errorf("TYPE('TEST_FIELD') com field e variable de mesmo nome = %q, quer %q (Caractere do field, não N do variable)\n"+
			"Field deve shadowing a variable.", result.(*advplrt.StringValue).Val, "C")
	}
}
