package vm

import (
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// hmEntry é um par chave/valor armazenado num tHashMap.
type hmEntry struct {
	key advplrt.Value
	val advplrt.Value
}

// hashMapState é o estado Go da classe tHashMap (TDN: "Manipulação de
// matriz HashMap"). A TDN documenta a API em estilo funcional — HMNew()
// devolve o objeto oHash e as demais funções (HMSet, HMGet, HMAdd, ...) o
// recebem sempre como primeiro argumento posicional (ex.: `HMSet(oHash,
// cKey, xVal)`), nunca como chamada de método (`oHash:HMSet(...)`). Por
// isso estas natives são despachadas como funções comuns, e não via
// callNativeMethod/classes intrínsecas.
//
// Internamente mantemos uma lista de entradas (para preservar ordem de
// inserção, usada por HMList) mais um índice por chave canônica (para
// busca O(1) amortizado em HMGet/HMGetN/HMSet/HMSetN).
type hashMapState struct {
	entries []hmEntry
	index   map[string]int // chave canônica -> posição em entries
}

func newHashMapState() *hashMapState {
	return &hashMapState{index: map[string]int{}}
}

// hmCanonicalKey converte um advplrt.Value na sua forma canônica de chave
// de mapa Go, distinguindo tipos (ex.: número 3 != string "3").
func hmCanonicalKey(v advplrt.Value) string {
	if v == nil || v == advplrt.Nil {
		return "N:"
	}
	switch t := v.(type) {
	case *advplrt.StringValue:
		return "S:" + t.Val
	case *advplrt.NumberValue:
		return "#:" + strconv.FormatFloat(t.Val, 'f', -1, 64)
	case *advplrt.BoolValue:
		return "B:" + strconv.FormatBool(t.Val)
	default:
		return "O:" + advplrt.ToString(v)
	}
}

// hmGetHash extrai o *hashMapState de um oHash (primeiro argumento das
// natives desta categoria). Retorna nil se o valor não for um tHashMap
// válido criado por HMNew/AToHM.
func hmGetHash(v advplrt.Value) *hashMapState {
	obj, ok := v.(*advplrt.ObjectValue)
	if !ok {
		return nil
	}
	state, ok := obj.Native.(*hashMapState)
	if !ok {
		return nil
	}
	return state
}

// hmApplyTrim replica os valores documentados de nTrim: 0 não altera,
// 1 elimina espaços à esquerda, 2 à direita, 3 ambos. Usa apenas o
// espaço ASCII 32 no cutset (não strings.TrimSpace) para não remover
// outros caracteres de controle silenciosamente.
func hmApplyTrim(s string, nTrim int) string {
	switch nTrim {
	case 1:
		return strings.TrimLeft(s, " ")
	case 2:
		return strings.TrimRight(s, " ")
	case 3:
		return strings.Trim(s, " ")
	default:
		return s
	}
}

// hmColSpec é um par (coluna 1-based, tipo de trim) usado para montar
// chaves compostas em AToHM/HMAdd/HMKey.
type hmColSpec struct {
	col  int
	trim int
}

// hmParseColSpecs lê os pares variádicos [nColuna_1, nTrim_1, nColuna_N,
// nTrim_N, ...] a partir de startIdx. Se nenhuma coluna for informada,
// usa por padrão a coluna 1 sem trim, conforme documentado em todas as
// funções desta categoria que aceitam colunas compostas.
func hmParseColSpecs(args []advplrt.Value, startIdx int) []hmColSpec {
	var specs []hmColSpec
	for i := startIdx; i < len(args); i += 2 {
		n, ok := args[i].(*advplrt.NumberValue)
		if !ok {
			continue
		}
		trim := 0
		if i+1 < len(args) {
			if t, ok := args[i+1].(*advplrt.NumberValue); ok {
				trim = int(t.Val)
			}
		}
		specs = append(specs, hmColSpec{col: int(n.Val), trim: trim})
	}
	if len(specs) == 0 {
		specs = []hmColSpec{{col: 1, trim: 0}}
	}
	return specs
}

// hmBuildKey monta a chave (simples ou composta) de uma linha (row)
// combinando as colunas indicadas em specs, aplicando trim (apenas em
// valores caractere) e concatenando a representação em string de cada
// coluna — mesma lógica usada por AToHM, HMAdd e HMKey.
func hmBuildKey(row []advplrt.Value, specs []hmColSpec) string {
	var sb strings.Builder
	for _, spec := range specs {
		idx := spec.col - 1
		if idx < 0 || idx >= len(row) {
			continue
		}
		val := row[idx]
		if s, ok := val.(*advplrt.StringValue); ok {
			sb.WriteString(hmApplyTrim(s.Val, spec.trim))
		} else {
			sb.WriteString(advplrt.ToString(val))
		}
	}
	return sb.String()
}

// hmSet insere ou atualiza (upsert) uma entrada, preservando a ordem de
// inserção original em updates (não move a entrada para o fim).
func (h *hashMapState) set(key advplrt.Value, val advplrt.Value) {
	ck := hmCanonicalKey(key)
	if pos, exists := h.index[ck]; exists {
		h.entries[pos].val = val
		return
	}
	h.index[ck] = len(h.entries)
	h.entries = append(h.entries, hmEntry{key: key, val: val})
}

// hmGet busca uma entrada por chave. Retorna o valor e se foi encontrada.
func (h *hashMapState) get(key advplrt.Value) (advplrt.Value, bool) {
	ck := hmCanonicalKey(key)
	pos, exists := h.index[ck]
	if !exists {
		return nil, false
	}
	return h.entries[pos].val, true
}

// registerManipulacaodematrizHashMapNatives registra as funções de
// manipulação de matriz HashMap (tHashMap): AToHM, HMAdd, HMClean, HMGet,
// HMGetN, HMKey, HMList, HMNew, HMSet, HMSetN.
//
// HMDel não é implementada: confirmado em docs/tdn-gap-stubs.md como
// stub sem spec real na fonte TDN espelhada — pulada por instrução
// explícita da task, não por omissão.
//
// LIMITAÇÃO CONHECIDA (docs/tdn-known-limitations.md, "Parâmetros por
// referência (@var) em natives"): HMGet, HMGetN e HMList documentam
// parâmetros de saída por referência (@aVal, @aVal, @aElem). Este VM não
// tem mecanismo para mutar uma variável escalar do chamador passada sem
// `@` explícito na chamada de uma native. Quando o argumento de saída é,
// na prática, um *advplrt.ArrayValue (arrays JÁ são tipo referência neste
// VM e no AdvPL real, sem precisar de `@`), a mutação in-place funciona e
// É aplicada aqui — é o que HMList sempre usa (aElem é array, conforme
// exemplo da TDN) e o que HMGet/HMGetN aplicam best-effort quando o
// terceiro argumento é um array. Quando o chamador segue o exemplo
// literal da TDN (`Local oVal := nil; HMGet(oHash,chave,oVal)`), oVal
// permanece Nil após a chamada — o valor de retorno lRet (achou/não
// achou) é a forma correta e suportada de checar o resultado.
func (v *VM) registerManipulacaodematrizHashMapNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// HMNew() -> oHash
	natives["HMNEW"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := advplrt.NewObject("THASHMAP", nil)
		obj.Native = newHashMapState()
		return obj, nil
	}

	// HMSet( < oHash >, < yKey >, < xVal > ) -> lRet
	natives["HMSET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		state.set(getArg(args, 1), getArg(args, 2))
		return advplrt.True, nil
	}

	// HMSetN( < oHash >, < nKey >, < nVal > ) -> lRet
	natives["HMSETN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		state.set(getArg(args, 1), getArg(args, 2))
		return advplrt.True, nil
	}

	// HMGet( < oHash >, < yKey >, < @aVal > ) -> lRet
	natives["HMGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		val, found := state.get(getArg(args, 1))
		if !found {
			return advplrt.False, nil
		}
		// Best-effort out-param: só funciona quando o chamador passou um
		// array (ver comentário da função de registro).
		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok {
			arr.Elements = []advplrt.Value{val}
		}
		return advplrt.True, nil
	}

	// HMGetN( < oHash >, < nKey >, < @aVal > ) -> lRet
	natives["HMGETN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		val, found := state.get(getArg(args, 1))
		if !found {
			return advplrt.False, nil
		}
		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok {
			arr.Elements = []advplrt.Value{val}
		}
		return advplrt.True, nil
	}

	// HMKey( < aArray >, [ nColuna_1 ], [ n_Trim_1 ], [ nColuna_N ], [ n_Trim_N ] ) -> cKey
	natives["HMKEY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewString(""), nil
		}
		specs := hmParseColSpecs(args, 1)
		return advplrt.NewString(hmBuildKey(arr.Elements, specs)), nil
	}

	// HMClean( < oHash > ) -> lRet
	natives["HMCLEAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		state.entries = nil
		state.index = map[string]int{}
		return advplrt.True, nil
	}

	// HMList( < oHash >, < @aElem > ) -> lRet
	natives["HMLIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		if arr, ok := getArg(args, 1).(*advplrt.ArrayValue); ok {
			elems := make([]advplrt.Value, len(state.entries))
			for i, e := range state.entries {
				elems[i] = e.val
			}
			arr.Elements = elems
		}
		return advplrt.True, nil
	}

	// HMAdd( < oHash >, < aVal >, [ nColuna_1 ], [ nTrim_1 ], [ nColuna_N ], [ nTrim_N ] ) -> lRet
	natives["HMADD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		state := hmGetHash(getArg(args, 0))
		if state == nil {
			return advplrt.False, nil
		}
		row, ok := getArg(args, 1).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.False, nil
		}
		specs := hmParseColSpecs(args, 2)
		key := hmBuildKey(row.Elements, specs)
		state.set(advplrt.NewString(key), row)
		return advplrt.True, nil
	}

	// AToHM( < aMatriz >, [ nColuna_1 ], [ nTrim_1 ], [ nColuna_N ], [ nTrim_N ] ) -> oHash
	natives["ATOHM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := advplrt.NewObject("THASHMAP", nil)
		state := newHashMapState()
		obj.Native = state

		matriz, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return obj, nil
		}
		specs := hmParseColSpecs(args, 1)
		for _, rowVal := range matriz.Elements {
			row, ok := rowVal.(*advplrt.ArrayValue)
			if !ok {
				continue
			}
			key := hmBuildKey(row.Elements, specs)
			state.set(advplrt.NewString(key), row)
		}
		return obj, nil
	}
}
