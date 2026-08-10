package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerVarglobaishashmapNatives registra as funções de Manipulação de
// Variáveis Globais (HashMap) — TDN: Manipulacao-de-variaveis-globais-HashMap.
//
// Modelo: sessões nomeadas (VarSetUID) com duas tabelas por chave — a
// "Tabela X" guarda valores primários (N/C/D/L) e a "Tabela A" guarda arrays.
// Chaves podem ter transação em curso (VarBeginT/VarEndT), que bloqueia as
// variantes síncronas (VarSet/VarSetX/VarSetA/VarGet/VarGetX/VarGetA/VarDel)
// enquanto as variantes "Dirty" (VarSetXD/VarSetAD/VarSetD/VarGetXD/...) não
// bloqueiam.
//
// LIMITAÇÃO (by-ref): como as variantes de leitura retornam o valor via
// parâmetro por referência (`@xValor`/`@aValor`), apenas o `@aValor` (array)
// é populado de volta — arrays são tipo referência neste VM (igual ao AdvPL
// real), então a mutação in-place funciona (mesmo mecanismo de HMGet/HMList).
// Valores escalares retornados por `@xValor` (N/C/D/L) não são escritos de
// volta na variável do chamador (ver docs/tdn-known-limitations.md); o valor
// de retorno lógico (.T./.F.) permanece correto e é a forma suportada de
// checar sucesso/falha. O conteúdo armazenado pode ser recuperado pelas
// listagens VarGetXA/VarGetAA/VarGet_A.
func (v *VM) registerVarglobaishashmapNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// VarBeginT( <cUID>, <cChave> ) -> lRet — inicia transação na chave,
	// bloqueando as variantes síncronas de leitura/gravação/deleção.
	natives["VARBEGINT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		sess.locked[cChave] = true
		return advplrt.True, nil
	}

	// VarEndT( <cUID>, <cChave> ) -> lRet — finaliza a transação na chave.
	natives["VARENDT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil || !sess.locked[cChave] {
			return advplrt.False, nil
		}
		delete(sess.locked, cChave)
		return advplrt.True, nil
	}

	// VarIsUID( <cUID> ) -> lRet — .T. se o UID está associado a uma sessão.
	natives["VARISUID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		if v.varSessions[cUID] != nil {
			return advplrt.True, nil
		}
		return advplrt.False, nil
	}

	// VarSetUID( <cUID> [, <lTemUID>] ) -> lRet — cria a sessão nomeada.
	// lTemUID=.F. (padrão): erro (.F.) se o UID já existir; .T. se aceitar
	// a sessão já existente.
	natives["VARSETUID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		lTemUID := advplrt.ToBool(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		if v.varSessions[cUID] != nil {
			if lTemUID {
				return advplrt.True, nil
			}
			return advplrt.False, nil
		}
		v.varSessions[cUID] = &varSession{
			x:      make(map[string]advplrt.Value),
			a:      make(map[string]advplrt.Value),
			locked: make(map[string]bool),
		}
		return advplrt.True, nil
	}

	// VarClean( <cUID> ) -> lRet — remove todos os dados de ambas as tabelas
	// e todas as transações da sessão (a sessão deixa de existir).
	natives["VARCLEAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		if v.varSessions[cUID] == nil {
			return advplrt.False, nil
		}
		delete(v.varSessions, cUID)
		return advplrt.True, nil
	}

	// VarCleanX( <cUID> ) -> lRet — remove todos os valores da "Tabela X".
	natives["VARCLEANX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		sess.x = make(map[string]advplrt.Value)
		return advplrt.True, nil
	}

	// VarCleanA( <cUID> ) -> lRet — remove todos os valores da "Tabela A".
	natives["VARCLEANA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		sess.a = make(map[string]advplrt.Value)
		return advplrt.True, nil
	}

	// VarDel( <cUID>, <cChave> ) -> lRet — remove a chave das duas tabelas
	// e da transação. Variante síncrona: falha (.F.) com transação em curso.
	natives["VARDEL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		if _, ok := sess.x[cChave]; !ok {
			if _, ok := sess.a[cChave]; !ok {
				return advplrt.False, nil
			}
		}
		delete(sess.x, cChave)
		delete(sess.a, cChave)
		return advplrt.True, nil
	}

	// VarDelX( <cUID>, <cChave> ) -> lRet — remove a chave da "Tabela X".
	natives["VARDELX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		if _, ok := sess.x[cChave]; !ok {
			return advplrt.False, nil
		}
		delete(sess.x, cChave)
		return advplrt.True, nil
	}

	// VarDelA( <cUID>, <cChave> ) -> lRet — remove a chave da "Tabela A".
	natives["VARDELA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		if _, ok := sess.a[cChave]; !ok {
			return advplrt.False, nil
		}
		delete(sess.a, cChave)
		return advplrt.True, nil
	}

	// VarSet( <cUID>, <cChave>, <xValor>, <aValor> ) -> lRet — insere ou
	// atualiza as duas tabelas (variante síncrona).
	natives["VARSET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		xValor := getArg(args, 2)
		aValor := getArg(args, 3)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		if arr, ok := aValor.(*advplrt.ArrayValue); ok {
			sess.a[cChave] = advplrt.NewArray(deepCopySlice(arr.Elements))
		}
		sess.x[cChave] = xValor
		return advplrt.True, nil
	}

	// VarSetX( <cUID>, <cChave>, <xValor> ) -> lRet — insere ou atualiza a
	// "Tabela X" (variante síncrona).
	natives["VARSETX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		xValor := getArg(args, 2)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		sess.x[cChave] = xValor
		return advplrt.True, nil
	}

	// VarSetA( <cUID>, <cChave>, <aValor> ) -> lRet — insere ou atualiza a
	// "Tabela A" (variante síncrona).
	natives["VARSETA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		aValor := getArg(args, 2)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		if arr, ok := aValor.(*advplrt.ArrayValue); ok {
			sess.a[cChave] = advplrt.NewArray(deepCopySlice(arr.Elements))
		}
		return advplrt.True, nil
	}

	// VarSetXD( <cUID>, <cChave>, <xValor> ) -> lRet — insere ou atualiza a
	// "Tabela X" sem bloqueio (Dirty).
	natives["VARSETXD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		xValor := getArg(args, 2)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		sess.x[cChave] = xValor
		return advplrt.True, nil
	}

	// VarSetAD( <cUID>, <cChave>, <aValor> ) -> lRet — insere ou atualiza a
	// "Tabela A" sem bloqueio (Dirty).
	natives["VARSETAD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		aValor := getArg(args, 2)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if arr, ok := aValor.(*advplrt.ArrayValue); ok {
			sess.a[cChave] = advplrt.NewArray(deepCopySlice(arr.Elements))
		}
		return advplrt.True, nil
	}

	// VarSetD( <cUID>, <cChave>, <xValor>, <aValor> ) -> lRet — insere ou
	// atualiza as duas tabelas sem bloqueio (Dirty).
	natives["VARSETD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))
		xValor := getArg(args, 2)
		aValor := getArg(args, 3)

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if arr, ok := aValor.(*advplrt.ArrayValue); ok {
			sess.a[cChave] = advplrt.NewArray(deepCopySlice(arr.Elements))
		}
		sess.x[cChave] = xValor
		return advplrt.True, nil
	}

	// VarGet( <cUID>, <cChave>, <@xValor>, <@aValor> ) -> lRet — recupera as
	// duas tabelas. .T. se a chave for encontrada em qualquer uma delas.
	natives["VARGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		found := false
		if xv, ok := sess.x[cChave]; ok {
			found = true
			writeVarOut(args, 2, xv)
		}
		if av, ok := sess.a[cChave]; ok {
			found = true
			writeVarOut(args, 3, av)
		}
		return advplrt.NewBool(found), nil
	}

	// VarGetX( <cUID>, <cChave>, <@xValor> ) -> lRet — recupera a "Tabela X".
	natives["VARGETX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		xv, ok := sess.x[cChave]
		if !ok {
			return advplrt.False, nil
		}
		writeVarOut(args, 2, xv)
		return advplrt.True, nil
	}

	// VarGetA( <cUID>, <cChave>, <@aValor> ) -> lRet — recupera a "Tabela A".
	natives["VARGETA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		if sess.locked[cChave] {
			return advplrt.False, nil
		}
		av, ok := sess.a[cChave]
		if !ok {
			return advplrt.False, nil
		}
		writeVarOut(args, 2, av)
		return advplrt.True, nil
	}

	// VarGetXD( <cUID>, <cChave>, <@xValor> ) -> lRet — recupera a "Tabela X"
	// sem bloqueio (Dirty).
	natives["VARGETXD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		xv, ok := sess.x[cChave]
		if !ok {
			return advplrt.False, nil
		}
		writeVarOut(args, 2, xv)
		return advplrt.True, nil
	}

	// VarGetAD( <cUID>, <cChave>, <@aValor> ) -> lRet — recupera a "Tabela A"
	// sem bloqueio (Dirty).
	natives["VARGETAD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		av, ok := sess.a[cChave]
		if !ok {
			return advplrt.False, nil
		}
		writeVarOut(args, 2, av)
		return advplrt.True, nil
	}

	// VarGetD( <cUID>, <cChave>, <@xValor>, <@aValor> ) -> lRet — recupera as
	// duas tabelas sem bloqueio (Dirty).
	natives["VARGETD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))
		cChave := advplrt.ToString(getArg(args, 1))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		found := false
		if xv, ok := sess.x[cChave]; ok {
			found = true
			writeVarOut(args, 2, xv)
		}
		if av, ok := sess.a[cChave]; ok {
			found = true
			writeVarOut(args, 3, av)
		}
		return advplrt.NewBool(found), nil
	}

	// VarGet_A( <cUID>, <@aListCV_X>, <@aListCV_A> ) -> lRet — lista de pares
	// {chave, valor} das duas tabelas.
	natives["VARGET_A"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		writeVarList(args, 1, sess.x)
		writeVarList(args, 2, sess.a)
		return advplrt.True, nil
	}

	// VarGetXA( <cUID>, <@aListCV> ) -> lRet — lista de pares {chave, valor}
	// da "Tabela X".
	natives["VARGETXA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		writeVarList(args, 1, sess.x)
		return advplrt.True, nil
	}

	// VarGetAA( <cUID>, <@aListCV> ) -> lRet — lista de pares {chave, valor}
	// da "Tabela A".
	natives["VARGETAA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cUID := advplrt.ToString(getArg(args, 0))

		v.varSessionsMu.Lock()
		defer v.varSessionsMu.Unlock()

		sess := v.varSessions[cUID]
		if sess == nil {
			return advplrt.False, nil
		}
		writeVarList(args, 1, sess.a)
		return advplrt.True, nil
	}
}

// writeVarOut escreve o valor recuperado no argumento de saída (índice idx),
// se ele for um array (tipo referência neste VM). Valores escalares passados
// por `@xValor` não podem ser escritos de volta (limitação documentada).
func writeVarOut(args []advplrt.Value, idx int, val advplrt.Value) {
	if idx >= len(args) {
		return
	}
	if out, ok := args[idx].(*advplrt.ArrayValue); ok {
		if av, ok := val.(*advplrt.ArrayValue); ok {
			out.Elements = deepCopySlice(av.Elements)
		} else {
			out.Elements = []advplrt.Value{val}
		}
	}
}

// writeVarList monta a lista de pares {chave, valor} de uma tabela no
// argumento de saída (array).
func writeVarList(args []advplrt.Value, idx int, table map[string]advplrt.Value) {
	if idx >= len(args) {
		return
	}
	out, ok := args[idx].(*advplrt.ArrayValue)
	if !ok {
		return
	}
	pairs := make([]advplrt.Value, 0, len(table))
	for k, val := range table {
		pair := []advplrt.Value{advplrt.NewString(k), val}
		pairs = append(pairs, advplrt.NewArray(pair))
	}
	out.Elements = pairs
}

// deepCopySlice produz uma cópia profunda de um slice de valores, de modo que
// arrays armazenados na "Tabela A" não compartilhem estado com o chamador
// (cada execução tem a sua cópia local, como a TDN documenta).
func deepCopySlice(src []advplrt.Value) []advplrt.Value {
	dst := make([]advplrt.Value, len(src))
	for i, v := range src {
		if av, ok := v.(*advplrt.ArrayValue); ok {
			dst[i] = advplrt.NewArray(deepCopySlice(av.Elements))
		} else {
			dst[i] = v
		}
	}
	return dst
}
