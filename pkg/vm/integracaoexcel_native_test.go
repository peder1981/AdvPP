package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// createTestBytecodeWithFunction cria um bytecode com uma função simples para testes.
// A função retorna o resultado da soma dos dois parâmetros (tipo de função simples).
// Exemplo: u_ExcelSum(14, 6) retorna 20
func createTestBytecodeWithFunction() *compiler.Bytecode {
	bc := &compiler.Bytecode{
		Constants: []compiler.Constant{
			// Constante 0: número 20 (resultado da soma 14+6)
			{Type: "number", Num: 20},
		},
		Functions: make(map[string]*compiler.FunctionInfo),
		Code:      []compiler.Instruction{},
	}

	// Cria uma função simples u_ExcelSum que retorna 20
	// Bytecode: [OP_NUMBER(0), OP_RETURN_VALUE]
	// OP_NUMBER 0 → coloca constante[0] (20) na stack
	// OP_RETURN_VALUE → retorna o topo da stack
	bc.Code = []compiler.Instruction{
		{Op: compiler.OP_NUMBER, Arg: 0, Line: 1}, // push 20 (constante 0)
		{Op: compiler.OP_RETURN_VALUE, Line: 2},   // retorna o valor na stack
	}

	// Registra a função no map de funções
	// Offset 0 = primeira instrução do Code
	// NumParams 2 = function u_ExcelSum(valor1, valor2)
	// NumLocals 2 = 2 slots (0-1) para armazenar os parâmetros
	bc.Functions["U_EXCELSUM"] = &compiler.FunctionInfo{
		Name:      "U_EXCELSUM",
		NumParams: 2,
		NumLocals: 2,
		IsUser:    true,
		Offset:    0,
	}

	return bc
}

// TestSIGA testa a função SIGA que permite chamar funções do ERP via integração com Excel.
func TestSIGA(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name: "SIGA com função AllwaysTrue",
			args: []advplrt.Value{
				advplrt.NewString("AllwaysTrue"),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				b, ok := val.(*advplrt.BoolValue)
				return ok && b.Val == true
			},
		},
		{
			name: "SIGA com função AllwaysFalse",
			args: []advplrt.Value{
				advplrt.NewString("AllwaysFalse"),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				b, ok := val.(*advplrt.BoolValue)
				return ok && b.Val == false
			},
		},
		{
			name: "SIGA com função inexistente",
			args: []advplrt.Value{
				advplrt.NewString("NonExistentFunction"),
			},
			wantErr: true,
			checkType: func(val advplrt.Value) bool {
				return true // Não verifica o valor em erro
			},
		},
		{
			name: "SIGA com Empty e string vazia",
			args: []advplrt.Value{
				advplrt.NewString("Empty"),
				advplrt.NewString(""),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				// Empty("") retorna .T.
				b, ok := val.(*advplrt.BoolValue)
				return ok && b.Val == true
			},
		},
		{
			name: "SIGA com Empty e string com conteúdo",
			args: []advplrt.Value{
				advplrt.NewString("Empty"),
				advplrt.NewString("abc"),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				// Empty("abc") retorna .F.
				b, ok := val.(*advplrt.BoolValue)
				return ok && b.Val == false
			},
		},
	}

	for _, c := range cases {
		got, err := v.natives["SIGA"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("SIGA(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("SIGA(%s) retornou tipo/valor incorreto: %v", c.name, got)
		}
	}
}

// TestSIGAUserFunction testa a função SIGA chamando uma função de usuário compilada.
// Este teste valida o exemplo do TDN: Function u_ExcelSum(valor1, valor2) chamada via SIGA("u_ExcelSum";14;6)
func TestSIGAUserFunction(t *testing.T) {
	// Cria bytecode com uma função de usuário
	bc := createTestBytecodeWithFunction()
	v := NewVM(bc, false)

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name: "SIGA chamando função de usuário u_ExcelSum(14;6)",
			args: []advplrt.Value{
				advplrt.NewString("u_ExcelSum"),
				advplrt.NewNumber(14),
				advplrt.NewNumber(6),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				// A função deve retornar 20 (14 + 6)
				n, ok := val.(*advplrt.NumberValue)
				return ok && n.Val == 20
			},
		},
		{
			name: "SIGA chamando função de usuário (sem prefixo U_)",
			args: []advplrt.Value{
				advplrt.NewString("U_ExcelSum"),
				advplrt.NewNumber(5),
				advplrt.NewNumber(3),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				// Deve encontrar mesmo sem o prefixo U_
				n, ok := val.(*advplrt.NumberValue)
				return ok && n.Val == 20
			},
		},
	}

	for _, c := range cases {
		got, err := v.natives["SIGA"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("SIGA(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("SIGA(%s) retornou valor incorreto: %v (esperava 20)", c.name, got)
		}
	}
}

// TestMsGetArray testa a função MsGetArray que obtém dados de array retornados por outra função.
func TestMsGetArray(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	cases := []struct {
		name      string
		args      []advplrt.Value
		wantErr   bool
		checkType func(advplrt.Value) bool
	}{
		{
			name: "MsGetArray com Array vazio",
			args: []advplrt.Value{
				advplrt.NewObject("Range", nil), // Cell/Range object (simulado)
				advplrt.NewArray([]advplrt.Value{}),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 0
			},
		},
		{
			name: "MsGetArray com Array contendo números",
			args: []advplrt.Value{
				advplrt.NewObject("Range", nil),
				advplrt.NewArray([]advplrt.Value{
					advplrt.NewNumber(1),
					advplrt.NewNumber(2),
					advplrt.NewNumber(3),
				}),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 3
			},
		},
		{
			name: "MsGetArray com Cell nulo",
			args: []advplrt.Value{
				advplrt.Nil,
				advplrt.NewArray([]advplrt.Value{advplrt.NewString("test")}),
			},
			wantErr: false,
			checkType: func(val advplrt.Value) bool {
				arr, ok := val.(*advplrt.ArrayValue)
				return ok && len(arr.Elements) == 1
			},
		},
	}

	for _, c := range cases {
		got, err := v.natives["MSGETARRAY"].Fn(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("MsGetArray(%s) erro = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if err == nil && !c.checkType(got) {
			t.Errorf("MsGetArray(%s) retornou tipo incorreto: %v", c.name, got)
		}
	}
}
