package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

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
