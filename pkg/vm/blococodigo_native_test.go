package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
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
	v := NewVM(&compiler.Bytecode{}, false)

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
	}

	for _, c := range cases {
		_, err := v.natives["DBEVAL"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("DBEVal(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
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
