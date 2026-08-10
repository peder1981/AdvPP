package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestHMNew(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["HMNEW"].Fn(nil)
	if err != nil {
		t.Fatalf("HMNew retornou erro: %v", err)
	}
	obj, ok := got.(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("HMNew() = %T, quer *advplrt.ObjectValue", got)
	}
	if obj.ClassName != "THASHMAP" {
		t.Errorf("ClassName = %q, quer THASHMAP", obj.ClassName)
	}
	state, ok := obj.Native.(*hashMapState)
	if !ok {
		t.Fatalf("obj.Native = %T, quer *hashMapState", obj.Native)
	}
	if len(state.entries) != 0 {
		t.Errorf("HMNew() deveria criar hashmap vazio, tem %d entradas", len(state.entries))
	}
}

func TestHMSet(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)

	// Exemplo TDN: HMSet(oHash, "item7", 10) -> .T.
	got, err := v.natives["HMSET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item7"), advplrt.NewNumber(10)})
	if err != nil {
		t.Fatalf("HMSet retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Fatalf("HMSet(oHash,\"item7\",10) = %v, quer .T.", got)
	}

	state := oHash.(*advplrt.ObjectValue).Native.(*hashMapState)
	if len(state.entries) != 1 {
		t.Fatalf("esperava 1 entrada após HMSet, tem %d", len(state.entries))
	}
	if advplrt.ToString(state.entries[0].val) != "10" {
		t.Errorf("valor armazenado = %v, quer 10", state.entries[0].val)
	}

	// Edge case: atualizar chave existente não deve duplicar entrada
	_, err = v.natives["HMSET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item7"), advplrt.NewNumber(99)})
	if err != nil {
		t.Fatalf("HMSet (update) retornou erro: %v", err)
	}
	if len(state.entries) != 1 {
		t.Errorf("HMSet sobre chave existente deveria atualizar in-place, tem %d entradas", len(state.entries))
	}
	if advplrt.ToString(state.entries[0].val) != "99" {
		t.Errorf("valor atualizado = %v, quer 99", state.entries[0].val)
	}
}

func TestHMSetN(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)

	// Exemplo TDN: HMSetN(oHash,1,23) ... HMSetN(oHash,5,18)
	pairs := [][2]float64{{1, 23}, {2, 104}, {3, 41}, {4, 1}, {5, 18}}
	for _, p := range pairs {
		got, err := v.natives["HMSETN"].Fn([]advplrt.Value{oHash, advplrt.NewNumber(p[0]), advplrt.NewNumber(p[1])})
		if err != nil {
			t.Fatalf("HMSetN retornou erro: %v", err)
		}
		if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
			t.Fatalf("HMSetN(oHash,%v,%v) = %v, quer .T.", p[0], p[1], got)
		}
	}

	state := oHash.(*advplrt.ObjectValue).Native.(*hashMapState)
	if len(state.entries) != 5 {
		t.Fatalf("esperava 5 entradas, tem %d", len(state.entries))
	}

	// Edge case: oHash inválido
	got, err := v.natives["HMSETN"].Fn([]advplrt.Value{advplrt.Nil, advplrt.NewNumber(1), advplrt.NewNumber(2)})
	if err != nil {
		t.Fatalf("HMSetN(Nil,...) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMSetN com oHash inválido deveria retornar .F., veio %v", got)
	}
}

func TestHMGet(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)
	_, _ = v.natives["HMSET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item3"), advplrt.NewNumber(41)})

	// Exemplo TDN: HMGet(oHash,"item3",oVal) -> .T.
	got, err := v.natives["HMGET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item3"), advplrt.Nil})
	if err != nil {
		t.Fatalf("HMGet retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMGet(oHash,\"item3\",oVal) = %v, quer .T.", got)
	}

	// Edge case: chave inexistente -> .F.
	got, err = v.natives["HMGET"].Fn([]advplrt.Value{oHash, advplrt.NewString("naoexiste"), advplrt.Nil})
	if err != nil {
		t.Fatalf("HMGet retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMGet com chave inexistente deveria retornar .F., veio %v", got)
	}

	// Caso de uso real do out-param: quando o caller passa um array (tipo
	// documentado na tabela de parâmetros da TDN para aVal), o valor É
	// populado de volta, pois arrays são tipo referência neste VM.
	outArr := advplrt.NewArray([]advplrt.Value{advplrt.Nil})
	got, err = v.natives["HMGET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item3"), outArr})
	if err != nil {
		t.Fatalf("HMGet retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMGet com out-param array = %v, quer .T.", got)
	}
	if len(outArr.Elements) == 0 || advplrt.ToString(outArr.Elements[0]) != "41" {
		t.Errorf("outArr não foi populado com o valor achado: %v", outArr.Elements)
	}
}

func TestHMGetN(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)
	for _, p := range [][2]float64{{1, 23}, {2, 104}, {3, 41}, {4, 1}, {5, 18}} {
		_, _ = v.natives["HMSETN"].Fn([]advplrt.Value{oHash, advplrt.NewNumber(p[0]), advplrt.NewNumber(p[1])})
	}

	// Exemplo TDN: nval := 2; HMGetN(oHash,nval,varg) -> .T.
	got, err := v.natives["HMGETN"].Fn([]advplrt.Value{oHash, advplrt.NewNumber(2), advplrt.Nil})
	if err != nil {
		t.Fatalf("HMGetN retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMGetN(oHash,2,varg) = %v, quer .T.", got)
	}

	// Edge case: chave numérica inexistente -> .F.
	got, err = v.natives["HMGETN"].Fn([]advplrt.Value{oHash, advplrt.NewNumber(999), advplrt.Nil})
	if err != nil {
		t.Fatalf("HMGetN retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMGetN com chave inexistente deveria retornar .F., veio %v", got)
	}
}

func TestHMKey(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Exemplo TDN: cKey := HMKey({"item2",104},1,3,2,3)
	row := advplrt.NewArray([]advplrt.Value{advplrt.NewString("item2"), advplrt.NewNumber(104)})
	got, err := v.natives["HMKEY"].Fn([]advplrt.Value{row, advplrt.NewNumber(1), advplrt.NewNumber(3), advplrt.NewNumber(2), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("HMKey retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("HMKey() = %T, quer *advplrt.StringValue", got)
	}
	if s.Val != "item2104" {
		t.Errorf("HMKey() = %q, quer %q", s.Val, "item2104")
	}

	// Edge case: sem colunas especificadas -> usa coluna 1, sem trim
	row2 := advplrt.NewArray([]advplrt.Value{advplrt.NewString(" item2 ")})
	got, err = v.natives["HMKEY"].Fn([]advplrt.Value{row2})
	if err != nil {
		t.Fatalf("HMKey retornou erro: %v", err)
	}
	if s, ok := got.(*advplrt.StringValue); !ok || s.Val != " item2 " {
		t.Errorf("HMKey() default = %v, quer %q", got, " item2 ")
	}
}

func TestHMClean(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)
	_, _ = v.natives["HMSET"].Fn([]advplrt.Value{oHash, advplrt.NewString("item7"), advplrt.NewNumber(10)})

	got, err := v.natives["HMCLEAN"].Fn([]advplrt.Value{oHash})
	if err != nil {
		t.Fatalf("HMClean retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMClean(oHash) = %v, quer .T.", got)
	}
	state := oHash.(*advplrt.ObjectValue).Native.(*hashMapState)
	if len(state.entries) != 0 {
		t.Errorf("HMClean deveria esvaziar o hashmap, tem %d entradas", len(state.entries))
	}

	// Edge case: oHash inválido -> .F.
	got, err = v.natives["HMCLEAN"].Fn([]advplrt.Value{advplrt.Nil})
	if err != nil {
		t.Fatalf("HMClean(Nil) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMClean(Nil) deveria retornar .F., veio %v", got)
	}
}

func TestHMList(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	oHash, _ := v.natives["HMNEW"].Fn(nil)
	for _, p := range [][2]float64{{1, 23}, {2, 104}} {
		_, _ = v.natives["HMSETN"].Fn([]advplrt.Value{oHash, advplrt.NewNumber(p[0]), advplrt.NewNumber(p[1])})
	}

	listret := advplrt.NewArray([]advplrt.Value{})
	got, err := v.natives["HMLIST"].Fn([]advplrt.Value{oHash, listret})
	if err != nil {
		t.Fatalf("HMList retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMList(oHash,listret) = %v, quer .T.", got)
	}
	if len(listret.Elements) != 2 {
		t.Fatalf("listret deveria ter 2 elementos, tem %d", len(listret.Elements))
	}

	// Edge case: oHash inválido -> .F., array de saída intocado
	outArr := advplrt.NewArray([]advplrt.Value{})
	got, err = v.natives["HMLIST"].Fn([]advplrt.Value{advplrt.Nil, outArr})
	if err != nil {
		t.Fatalf("HMList(Nil,...) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMList com oHash inválido deveria retornar .F., veio %v", got)
	}
}

func TestHMAdd(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	lista := advplrt.NewArray([]advplrt.Value{
		advplrt.NewArray([]advplrt.Value{advplrt.NewString("item1"), advplrt.NewNumber(23)}),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString("item2"), advplrt.NewNumber(104)}),
	})
	oHash, err := v.natives["ATOHM"].Fn([]advplrt.Value{lista, advplrt.NewNumber(1), advplrt.NewNumber(3), advplrt.NewNumber(2), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("AToHM retornou erro: %v", err)
	}

	// Exemplo TDN: cKey := HMAdd(oHash,{"item10",5},1,3,2,3)
	newRow := advplrt.NewArray([]advplrt.Value{advplrt.NewString("item10"), advplrt.NewNumber(5)})
	got, err := v.natives["HMADD"].Fn([]advplrt.Value{oHash, newRow, advplrt.NewNumber(1), advplrt.NewNumber(3), advplrt.NewNumber(2), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("HMAdd retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMAdd(oHash,{\"item10\",5},1,3,2,3) = %v, quer .T.", got)
	}

	state := oHash.(*advplrt.ObjectValue).Native.(*hashMapState)
	if len(state.entries) != 3 {
		t.Fatalf("esperava 3 entradas após HMAdd, tem %d", len(state.entries))
	}

	// Edge case: oHash inválido -> .F.
	got, err = v.natives["HMADD"].Fn([]advplrt.Value{advplrt.Nil, newRow})
	if err != nil {
		t.Fatalf("HMAdd(Nil,...) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("HMAdd com oHash inválido deveria retornar .F., veio %v", got)
	}
}

func TestAToHM(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	lista := advplrt.NewArray([]advplrt.Value{
		advplrt.NewArray([]advplrt.Value{advplrt.NewString("item1"), advplrt.NewNumber(23)}),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(" item2 "), advplrt.NewNumber(104)}),
	})

	// Exemplo TDN: por default usa a primeira coluna sem remover espaços
	got, err := v.natives["ATOHM"].Fn([]advplrt.Value{lista})
	if err != nil {
		t.Fatalf("AToHM retornou erro: %v", err)
	}
	obj, ok := got.(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("AToHM() = %T, quer *advplrt.ObjectValue", got)
	}
	state := obj.Native.(*hashMapState)
	if len(state.entries) != 2 {
		t.Fatalf("esperava 2 entradas, tem %d", len(state.entries))
	}

	getRet, err := v.natives["HMGET"].Fn([]advplrt.Value{got, advplrt.NewString(" item2 "), advplrt.Nil})
	if err != nil {
		t.Fatalf("HMGet retornou erro: %v", err)
	}
	if b, ok := getRet.(*advplrt.BoolValue); !ok || !b.Val {
		t.Fatalf("HMGet(oHash1,\" item2 \",oVal) = %v, quer .T.", getRet)
	}

	// Edge case: aMatriz não é array -> hashmap vazio
	got, err = v.natives["ATOHM"].Fn([]advplrt.Value{advplrt.Nil})
	if err != nil {
		t.Fatalf("AToHM(Nil) retornou erro: %v", err)
	}
	obj2, ok := got.(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("AToHM(Nil) = %T, quer *advplrt.ObjectValue", got)
	}
	if len(obj2.Native.(*hashMapState).entries) != 0 {
		t.Errorf("AToHM(Nil) deveria gerar hashmap vazio")
	}
}
