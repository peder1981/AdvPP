package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Tests das 25 funções de Manipulação de Variáveis Globais (HashMap):
// VarBeginT, VarClean, VarCleanA, VarCleanX, VarDel, VarDelA, VarDelX,
// VarEndT, VarGet, VarGet_A, VarGetA, VarGetAA, VarGetAD, VarGetD, VarGetX,
// VarGetXA, VarGetXD, VarIsUID, VarSet, VarSetA, VarSetAD, VarSetD,
// VarSetUID, VarSetX, VarSetXD.

func newVarTestVM(t *testing.T) *VM {
	t.Helper()
	return NewVM(&compiler.Bytecode{}, false)
}

// valNumber/valStr/valBool/valArr são helpers para montar advplrt.Value.

func TestVarSetUIDAndVarIsUID(t *testing.T) {
	v := newVarTestVM(t)

	// VarIsUID antes da criação -> .F.
	if r, _ := v.natives["VARISUID"].Fn([]advplrt.Value{advplrt.NewString("sessao1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarIsUID antes da criação deveria ser .F.")
	}

	// VarSetUID cria -> .T.
	if r, _ := v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("sessao1")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetUID de sessão nova deveria ser .T.")
	}

	// VarIsUID após a criação -> .T.
	if r, _ := v.natives["VARISUID"].Fn([]advplrt.Value{advplrt.NewString("sessao1")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarIsUID após a criação deveria ser .T.")
	}

	// VarSetUID de sessão já existente sem lTemUID (padrão .F.) -> .F.
	if r, _ := v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("sessao1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetUID de sessão existente sem lTemUID deveria ser .F.")
	}

	// VarSetUID de sessão existente com lTemUID=.T. -> .T.
	if r, _ := v.natives["VARSETUID"].Fn([]advplrt.Value{
		advplrt.NewString("sessao1"), advplrt.NewBool(true),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetUID de sessão existente com lTemUID=.T. deveria ser .T.")
	}

	// Chaves são case-sensitive: outra caixa -> sessão diferente.
	if r, _ := v.natives["VARISUID"].Fn([]advplrt.Value{advplrt.NewString("Sessao1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarIsUID de UID com caixa diferente deveria ser .F.")
	}
}

func TestVarCleanRemovesSession(t *testing.T) {
	v := newVarTestVM(t)

	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("sessao2")})
	v.natives["VARSETX"].Fn([]advplrt.Value{
		advplrt.NewString("sessao2"), advplrt.NewString("ch1"), advplrt.NewNumber(42),
	})

	if r, _ := v.natives["VARCLEAN"].Fn([]advplrt.Value{advplrt.NewString("sessao2")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarClean de sessão existente deveria ser .T.")
	}

	// Após VarClean a sessão não existe mais.
	if r, _ := v.natives["VARISUID"].Fn([]advplrt.Value{advplrt.NewString("sessao2")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarIsUID após VarClean deveria ser .F.")
	}

	// VarClean de sessão inexistente -> .F.
	if r, _ := v.natives["VARCLEAN"].Fn([]advplrt.Value{advplrt.NewString("nao_existe")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarClean de sessão inexistente deveria ser .F.")
	}
}

func TestVarSetXVarGetXVarGetXA(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})

	// VarSetX armazena na Tabela X.
	if r, _ := v.natives["VARSETX"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("ch1"), advplrt.NewNumber(42),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetX deveria ser .T.")
	}

	// VarGetX de chave existente -> .T.
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("ch1"), advplrt.NewNumber(0),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX de chave existente deveria ser .T.")
	}

	// VarGetX de chave inexistente -> .F.
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("nao_tem"), advplrt.NewNumber(0),
	}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX de chave inexistente deveria ser .F.")
	}

	// VarGetX de sessão inexistente -> .F.
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{
		advplrt.NewString("outra"), advplrt.NewString("ch1"), advplrt.NewNumber(0),
	}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX de sessão inexistente deveria ser .F.")
	}

	// VarGetXA retorna a lista {chave, valor} da Tabela X.
	aList := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETXA"].Fn([]advplrt.Value{advplrt.NewString("s"), aList}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetXA deveria ser .T.")
	}
	if len(aList.Elements) != 1 {
		t.Fatalf("VarGetXA deveria retornar 1 par, veio %d", len(aList.Elements))
	}
	pair, ok := aList.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(pair.Elements) != 2 {
		t.Fatalf("par de VarGetXA deveria ser array de 2 elementos, veio %T", aList.Elements[0])
	}
	if advplrt.ToString(pair.Elements[0]) != "ch1" {
		t.Errorf("chave da lista = %q, want ch1", advplrt.ToString(pair.Elements[0]))
	}
	if n := advplrt.ToFloat(pair.Elements[1]); n != 42 {
		t.Errorf("valor da lista = %v, want 42", n)
	}

	// VarGetXA de sessão inexistente -> .F. e lista intacta.
	aList2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETXA"].Fn([]advplrt.Value{advplrt.NewString("nao"), aList2}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetXA de sessão inexistente deveria ser .F.")
	}
}

func TestVarSetVarGetAndTables(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})

	// VarSet grava nas duas tabelas.
	aVal := advplrt.NewArray([]advplrt.Value{advplrt.NewString("a"), advplrt.NewNumber(1)})
	if r, _ := v.natives["VARSET"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewString("valorX"), aVal,
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSet deveria ser .T.")
	}

	// VarGetX recupera da Tabela X.
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewString(""),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX de chave gravada por VarSet deveria ser .T.")
	}

	// VarGetA recupera a Tabela A (arrays são referência -> writeback in-place).
	aOut := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), aOut}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA de chave gravada por VarSet deveria ser .T.")
	}
	if len(aOut.Elements) != 2 || advplrt.ToString(aOut.Elements[0]) != "a" {
		t.Errorf("VarGetA deveria devolver {a, 1}, veio %s", aOut.String())
	}
}

func TestVarSetAVarGetA(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})

	aVal := advplrt.NewArray([]advplrt.Value{
		advplrt.NewBool(true), advplrt.NewNumber(-1), advplrt.NewString("ricardo"),
	})
	if r, _ := v.natives["VARSETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chA"), aVal}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetA deveria ser .T.")
	}

	// VarGetA de chave existente -> .T. e array populado in-place.
	aOut := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chA"), aOut}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA de chave existente deveria ser .T.")
	}
	if len(aOut.Elements) != 3 {
		t.Fatalf("VarGetA deveria devolver 3 elementos, veio %d", len(aOut.Elements))
	}
	if !aOut.Elements[0].(*advplrt.BoolValue).Val {
		t.Errorf("aOut[0] deveria ser .T.")
	}
	if advplrt.ToFloat(aOut.Elements[1]) != -1 {
		t.Errorf("aOut[1] deveria ser -1")
	}
	if advplrt.ToString(aOut.Elements[2]) != "ricardo" {
		t.Errorf("aOut[2] deveria ser ricardo")
	}

	// VarGetA de chave inexistente -> .F.
	aOut2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("nao"), aOut2}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA de chave inexistente deveria ser .F.")
	}
}

func TestVarCleanXVarCleanA(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})
	v.natives["VARSETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewNumber(1)})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chA"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(9)}),
	})

	// VarCleanX remove só a Tabela X.
	if r, _ := v.natives["VARCLEANX"].Fn([]advplrt.Value{advplrt.NewString("s")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarCleanX deveria ser .T.")
	}
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewNumber(0)}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX após VarCleanX deveria ser .F.")
	}
	aOut := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chA"), aOut}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA após VarCleanX deveria continuar .T. (Tabela A intacta)")
	}

	// VarCleanA remove só a Tabela A.
	if r, _ := v.natives["VARCLEANA"].Fn([]advplrt.Value{advplrt.NewString("s")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarCleanA deveria ser .T.")
	}
	aOut2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chA"), aOut2}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA após VarCleanA deveria ser .F.")
	}

	// Sessão continua existindo.
	if r, _ := v.natives["VARISUID"].Fn([]advplrt.Value{advplrt.NewString("s")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarIsUID após VarCleanX/A deveria ser .T.")
	}
}

func TestVarDelVarDelXVarDelA(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})
	v.natives["VARSETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewNumber(1)})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chA"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(2)}),
	})
	v.natives["VARSETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas"), advplrt.NewString("x")})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("ambas"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(3)}),
	})

	// VarDelX remove só da Tabela X.
	if r, _ := v.natives["VARDELX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarDelX deveria ser .T.")
	}
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas"), advplrt.NewNumber(0)}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX após VarDelX deveria ser .F.")
	}
	aOut := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas"), aOut}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA após VarDelX deveria continuar .T.")
	}

	// VarDelA remove só da Tabela A.
	if r, _ := v.natives["VARDELA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarDelA deveria ser .T.")
	}
	aOut2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETA"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ambas"), aOut2}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetA após VarDelA deveria ser .F.")
	}

	// VarDel remove das duas tabelas de chX/chA.
	if r, _ := v.natives["VARDEL"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarDel de chX deveria ser .T.")
	}
	if r, _ := v.natives["VARGETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewNumber(0)}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetX após VarDel deveria ser .F.")
	}

	// VarDel de chave inexistente -> .F.?
	if r, _ := v.natives["VARDEL"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("inexistente")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarDel de chave inexistente deveria ser .F.")
	}
}

func TestVarBeginTAndVarEndT(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})

	// VarBeginT inicia transação na chave.
	if r, _ := v.natives["VARBEGINT"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ch1")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarBeginT deveria ser .T.")
	}

	// Segunda VarBeginT na mesma chave (já transacionada) -> .F.
	if r, _ := v.natives["VARBEGINT"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ch1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarBeginT duplicada na mesma chave deveria ser .F.")
	}

	// Durante a transação, VarDel/VarClean devem falhar (chave bloqueada).
	if r, _ := v.natives["VARDEL"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ch1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarDel durante transação deveria ser .F.")
	}

	// VarEndT finaliza a transação.
	if r, _ := v.natives["VARENDT"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ch1")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarEndT deveria ser .T.")
	}

	// VarEndT sem transação em curso -> .F.
	if r, _ := v.natives["VARENDT"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("ch1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarEndT sem transação deveria ser .F.")
	}

	// VarBeginT em sessão inexistente -> .F.
	if r, _ := v.natives["VARBEGINT"].Fn([]advplrt.Value{advplrt.NewString("nao"), advplrt.NewString("ch1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarBeginT em sessão inexistente deveria ser .F.")
	}

	// VarEndT em sessão inexistente -> .F.
	if r, _ := v.natives["VARENDT"].Fn([]advplrt.Value{advplrt.NewString("nao"), advplrt.NewString("ch1")}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarEndT em sessão inexistente deveria ser .F.")
	}
}

func TestVarGetDirtyVariants(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})

	// Variantes Dirty: VarSetXD, VarSetAD, VarSetD.
	if r, _ := v.natives["VARSETXD"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewString("dirty")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetXD deveria ser .T.")
	}
	if r, _ := v.natives["VARSETAD"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chA"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(7)}),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetAD deveria ser .T.")
	}
	if r, _ := v.natives["VARSETD"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("chD"), advplrt.NewString("dx"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(8)}),
	}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarSetD deveria ser .T.")
	}

	// VarGetXD, VarGetAD, VarGetD recuperam.
	if r, _ := v.natives["VARGETXD"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewString("")}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetXD deveria ser .T.")
	}
	aOut := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETAD"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chA"), aOut}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetAD deveria ser .T.")
	}
	if len(aOut.Elements) != 1 || advplrt.ToFloat(aOut.Elements[0]) != 7 {
		t.Errorf("VarGetAD deveria devolver {7}, veio %s", aOut.String())
	}
	aOut2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETD"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chD"), advplrt.NewString(""), aOut2}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetD deveria ser .T.")
	}
	if len(aOut2.Elements) != 1 || advplrt.ToFloat(aOut2.Elements[0]) != 8 {
		t.Errorf("VarGetD deveria devolver aTabelaA {8}, veio %s", aOut2.String())
	}

	// VarGet (transacionado) também lê chaves gravadas via dirty.
	aOut3 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGET"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("chX"), advplrt.NewString(""), aOut3}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGet de chave existente deveria ser .T.")
	}
	// VarGet de chave inexistente -> .F.
	if r, _ := v.natives["VARGET"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("nao"), advplrt.NewString(""), advplrt.NewArray([]advplrt.Value{})}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGet de chave inexistente deveria ser .F.")
	}
}

func TestVarGet_A(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})
	v.natives["VARSETX"].Fn([]advplrt.Value{advplrt.NewString("s"), advplrt.NewString("kx"), advplrt.NewNumber(11)})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("ka"), advplrt.NewArray([]advplrt.Value{advplrt.NewString("arr")}),
	})

	aX := advplrt.NewArray([]advplrt.Value{})
	aA := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGET_A"].Fn([]advplrt.Value{advplrt.NewString("s"), aX, aA}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGet_A deveria ser .T.")
	}
	if len(aX.Elements) != 1 {
		t.Errorf("VarGet_A Tabela X deveria ter 1 par, veio %d", len(aX.Elements))
	}
	if len(aA.Elements) != 1 {
		t.Errorf("VarGet_A Tabela A deveria ter 1 par, veio %d", len(aA.Elements))
	}
	pairX, _ := aX.Elements[0].(*advplrt.ArrayValue)
	if pairX == nil || advplrt.ToString(pairX.Elements[0]) != "kx" || advplrt.ToFloat(pairX.Elements[1]) != 11 {
		t.Errorf("par X inválido: %v", aX.String())
	}

	// VarGet_A de sessão inexistente -> .F.
	aX2 := advplrt.NewArray([]advplrt.Value{})
	aA2 := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGET_A"].Fn([]advplrt.Value{advplrt.NewString("nao"), aX2, aA2}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGet_A de sessão inexistente deveria ser .F.")
	}
}

func TestVarGetAA(t *testing.T) {
	v := newVarTestVM(t)
	v.natives["VARSETUID"].Fn([]advplrt.Value{advplrt.NewString("s")})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("ka"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(5)}),
	})
	v.natives["VARSETA"].Fn([]advplrt.Value{
		advplrt.NewString("s"), advplrt.NewString("kb"), advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(6)}),
	})

	aList := advplrt.NewArray([]advplrt.Value{})
	if r, _ := v.natives["VARGETAA"].Fn([]advplrt.Value{advplrt.NewString("s"), aList}); !r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetAA deveria ser .T.")
	}
	if len(aList.Elements) != 2 {
		t.Fatalf("VarGetAA deveria retornar 2 pares, veio %d", len(aList.Elements))
	}
	if r, _ := v.natives["VARGETAA"].Fn([]advplrt.Value{advplrt.NewString("nao"), advplrt.NewArray([]advplrt.Value{})}); r.(*advplrt.BoolValue).Val {
		t.Errorf("VarGetAA de sessão inexistente deveria ser .F.")
	}
}
