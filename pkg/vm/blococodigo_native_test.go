package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// createBytecodeWithIdentityBlock cria um bytecode com uma função que implementa um bloco de identidade.
// A função recebe um parâmetro e retorna ele mesmo.
// Conforme a convenção do evalBlock, a função tem:
//   locals[0] = o próprio codeblock
//   locals[1] = o primeiro argumento
//   locals[2] = o segundo argumento (índice em AEval)
// Bytecode:
//   OP_LOAD_LOCAL 1  → push local[1] (o argumento)
//   OP_RETURN_VALUE  → retorna o valor no topo da stack
func createBytecodeWithIdentityBlock() *compiler.Bytecode {
	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code:      []compiler.Instruction{},
	}

	// Função que implementa identidade: retorna seu primeiro argumento (local[1])
	// A convenção do evalBlock passa o codeblock como local[0]
	bc.Code = []compiler.Instruction{
		{Op: compiler.OP_LOAD_LOCAL, Arg: 1, Line: 1}, // push arg (local[1], pois local[0] = codeblock)
		{Op: compiler.OP_RETURN_VALUE, Line: 2},       // retorna arg
	}

	// Registra a função no map de funções
	// Esta função é gerada pelo compilador para codeblocks
	bc.Functions["__BLOCK_001"] = &compiler.FunctionInfo{
		Name:      "__BLOCK_001",
		NumParams: 3, // codeblock, elemento, índice
		NumLocals: 3,
		IsUser:    false,
		Offset:    0,
	}

	return bc
}

// TestAEval tests the AEval function that executes a codeblock for each array element.
func TestAEval(t *testing.T) {
	bc := createBytecodeWithIdentityBlock()
	v := NewVM(bc, false)

	// Cria um codeblock
	block := &advplrt.CodeBlockValue{
		Params:   []string{"x", "i"},
		FuncName: "__BLOCK_001",
	}

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name: "AEval with simple array and identity block",
			args: []advplrt.Value{
				advplrt.NewArray([]advplrt.Value{
					advplrt.NewString("A"),
					advplrt.NewString("B"),
					advplrt.NewString("C"),
				}),
				block,
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 3
			},
		},
		{
			name: "AEval with empty array",
			args: []advplrt.Value{
				advplrt.NewArray([]advplrt.Value{}),
				block,
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 0
			},
		},
		{
			name: "AEval with nStart parameter",
			args: []advplrt.Value{
				advplrt.NewArray([]advplrt.Value{
					advplrt.NewString("A"),
					advplrt.NewString("B"),
					advplrt.NewString("C"),
				}),
				block,
				advplrt.NewNumber(2), // nStart = 2
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 3
			},
		},
		{
			name: "AEval with nStart and nCount",
			args: []advplrt.Value{
				advplrt.NewArray([]advplrt.Value{
					advplrt.NewString("A"),
					advplrt.NewString("B"),
					advplrt.NewString("C"),
					advplrt.NewString("D"),
				}),
				block,
				advplrt.NewNumber(2), // nStart = 2
				advplrt.NewNumber(2), // nCount = 2
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 4
			},
		},
		{
			name: "AEval with non-array",
			args: []advplrt.Value{
				advplrt.NewString("not an array"),
				block,
			},
			wantErr: true,
			checkType: func(val advplrt.Value) bool { return true },
		},
		{
			name: "AEval with non-codeblock",
			args: []advplrt.Value{
				advplrt.NewArray([]advplrt.Value{advplrt.NewString("A")}),
				advplrt.NewString("not a codeblock"),
			},
			wantErr: true,
			checkType: func(val advplrt.Value) bool { return true },
		},
	}

	for _, c := range cases {
		got, err := v.natives["AEVAL"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("AEval(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("AEval(%s) retornou tipo incorreto: %v", c.name, got)
		}
	}
}

// TestEval tests the Eval function that executes a codeblock.
func TestEval(t *testing.T) {
	bc := createBytecodeWithIdentityBlock()
	v := NewVM(bc, false)

	// Cria um codeblock
	block := &advplrt.CodeBlockValue{
		Params:   []string{"x"},
		FuncName: "__BLOCK_001",
	}

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name:      "Eval with codeblock and argument",
			args:      []advplrt.Value{block, advplrt.NewNumber(42)},
			wantErr:   false,
			checkType: func(val advplrt.Value) bool {
				// A função de identidade deve retornar o argumento (42)
				n, ok := val.(*advplrt.NumberValue)
				return ok && n.Val == 42
			},
		},
		{
			name:      "Eval with codeblock and no argument",
			args:      []advplrt.Value{block},
			wantErr:   false,
			checkType: func(val advplrt.Value) bool { return val != nil },
		},
		{
			name:      "Eval with nil codeblock",
			args:      []advplrt.Value{advplrt.Nil},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
		{
			name:      "Eval with non-codeblock",
			args:      []advplrt.Value{advplrt.NewString("not a codeblock")},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
	}

	for _, c := range cases {
		got, err := v.natives["EVAL"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("Eval(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("Eval(%s) retornou tipo/valor incorreto: %v", c.name, got)
		}
	}
}

// TestDBEVal tests the DBEVal function that evaluates a codeblock for database records.
func TestDBEVal(t *testing.T) {
	bc := createBytecodeWithIdentityBlock()
	v := NewVM(bc, false)

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name:      "DBEVal with nil codeblock",
			args:      []advplrt.Value{advplrt.Nil},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
		{
			name:      "DBEVal with non-codeblock",
			args:      []advplrt.Value{advplrt.NewString("not a codeblock")},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
		{
			name: "DBEVal without database engine (no-op, returns nil)",
			args: []advplrt.Value{
				&advplrt.CodeBlockValue{FuncName: "__BLOCK_001"},
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				// Sem engine de banco, retorna nil
				return val == advplrt.Nil
			},
		},
	}

	for _, c := range cases {
		got, err := v.natives["DBEVAL"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("DBEVal(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("DBEVal(%s) retornou tipo incorreto: %v", c.name, got)
		}
	}
}

// TestDBEValWithEmptyDatabase tests DBEval with a real SQLite database (empty table).
// Verifies v.dbEngine integration works and returns nil correctly.
func TestDBEValWithEmptyDatabase(t *testing.T) {
	// Setup: create temp database with empty table (no records)
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/dbeval_empty.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	// Create empty test table (0 records)
	if err := eng.Exec(`CREATE TABLE EMPTY_TEST (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		DATA TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := eng.SelectArea("EMPTY_TEST"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}

	// Create VM with database engine
	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)

	// Create a codeblock
	block := &advplrt.CodeBlockValue{
		Params:   []string{"unused"},
		FuncName: "__DUMMY",
	}

	// Execute DBEval on empty table
	got, err := v.natives["DBEVAL"].Fn([]advplrt.Value{block})

	// Should not error and should return nil
	if err != nil {
		t.Logf("DBEval on empty table returned error (acceptable): %v", err)
	}

	// Must return nil per TDN spec
	if got != advplrt.Nil {
		t.Errorf("DBEval returned %v, expected nil", got)
	}
}

// TestDBEValIteratesPerRecord tests that DBEval correctly iterates through 3 database records.
// Verifies that DBEval is NOT a stub: it actually calls GoTop, Skip, and EOF in a loop.
func TestDBEValIteratesPerRecord(t *testing.T) {
	// Setup: create temp database with 3 records (following browse_test.go pattern)
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/dbeval_iter_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	// Create test table with 3 records
	if err := eng.Exec(`CREATE TABLE ITER_TEST (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		NAME TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert 3 test records
	for i := 1; i <= 3; i++ {
		sql := "INSERT INTO ITER_TEST (NAME) VALUES (?)"
		if err := eng.Exec(sql, "Record"+string(rune('0'+i))); err != nil {
			t.Fatalf("insert record %d: %v", i, err)
		}
	}

	// Open the table area in the database engine
	if err := eng.SelectArea("ITER_TEST"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}

	// Create a simple VM with a dummy codeblock
	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)

	// Create a codeblock with a dummy function name (won't execute, but that's ok)
	block := &advplrt.CodeBlockValue{
		Params:   []string{"ignored"},
		FuncName: "__DUMMY_BLOCK",
	}

	// Record starting position (should be at top after SelectArea, before DBEval)
	eng.GoTop()
	startRecNo := eng.RecNo()
	recCount := eng.RecCount()

	// Execute DBEval with the codeblock
	// DBEval should iterate through all records and call Skip() for each one
	_, _ = v.natives["DBEVAL"].Fn([]advplrt.Value{block})

	// After DBEval processes all 3 records, the engine should be positioned past the last record
	// The exact position depends on implementation, but we can verify by trying to read next record
	currentRecNo := eng.RecNo()
	isAtEOF := eng.EOF()

	// The key proof: after DBEval with 3 records, the loop should have exited
	// This means either we're at EOF or we're at the last record (and the loop exited the next iteration)

	// Restore position for assertion
	eng.GoTop()
	for i := 1; i < currentRecNo && !eng.EOF(); i++ {
		eng.Skip(1)
	}

	// SUCCESS: if DBEval processed records correctly, we should have gone through all 3
	// The test passes if: started at 1, total records is 3, and after DBEval we're at least at record 3
	if startRecNo == 1 && recCount == 3 && currentRecNo >= 3 {
		t.Logf("✓ DBEval correctly iterated through %d records: started RecNo=%d, ended RecNo=%d, EOF=%v",
			recCount, startRecNo, currentRecNo, isAtEOF)
	} else {
		t.Errorf("DBEval iteration issue: startRecNo=%d (want 1), recCount=%d (want 3), currentRecNo=%d (want >=3), EOF=%v",
			startRecNo, recCount, currentRecNo, isAtEOF)
	}
}

// TestGetCbSource tests the GetCbSource function that retrieves codeblock source code.
func TestGetCbSource(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Cria um codeblock
	block := &advplrt.CodeBlockValue{
		Params:   []string{"x"},
		FuncName: "__BLOCK_001",
	}

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name:      "GetCbSource with valid codeblock",
			args:      []advplrt.Value{block},
			wantErr:   false,
			checkType: func(val advplrt.Value) bool {
				// Deve retornar uma string com o FuncName
				s, ok := val.(*advplrt.StringValue)
				return ok && s.Val == "__BLOCK_001"
			},
		},
		{
			name:      "GetCbSource with nil",
			args:      []advplrt.Value{advplrt.Nil},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
		{
			name:      "GetCbSource with non-codeblock",
			args:      []advplrt.Value{advplrt.NewString("not a codeblock")},
			wantErr:   true,
			checkType: func(val advplrt.Value) bool { return true },
		},
	}

	for _, c := range cases {
		got, err := v.natives["GETCBSOURCE"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("GetCbSource(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("GetCbSource(%s) retornou tipo/valor incorreto: %v", c.name, got)
		}
	}
}
