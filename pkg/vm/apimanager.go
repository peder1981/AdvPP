package vm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// apiManagerState é o estado Go da classe nativa FwApiManager — a camada de
// API de leitura usada pelos apps PO-UI (45 ocorrências no corpus). Monta a
// resposta {"items":[...],"hasNext":bool} sobre uma tabela-alias, com
// filtro OData, ordenação e paginação vindos de SetApiQString (ou de
// SetApiFilter/SetQuery explícitos).
//
// Premissas documentadas (corpus indisponível — confirmar quando houver
// ADVPP_CORPUS_2510):
//   - SetApiMap(aMapa): array de {prop, coluna} — array de arrays de 2
//     posições ({"codigo","NNR_CODIGO"}) ou objetos {PROP,CAMPO}; os campos
//     filtram/ordenam por padrão e o tipo vem do SX3 local (como TamSx3).
//   - SetApiQString(aQueryString): array de {chave, valor} (array de arrays
//     ou objetos); as chaves page/pageSize/filter/order/fields seguem os
//     mesmos nomes da URL do adapter.
//   - SetApiRelation: aceita e ignora por ora — a relação mestre-detalhe
//     por campos de junção depende do formato real do corpus.
//   - ToExecAuto é lacuna conhecida: ExecAuto/MVC não existe no AdvPP.
type apiManagerState struct {
	servico, versao string
	alias           string
	mapa            map[string]MapaCampo
	queryTmpl       string
	whereSQL        string
	orderExpr       string
	filterExpr      string
	params          map[string]string
	page            int
	pageSet         bool
	pageSize        int
	sizeSet         bool

	errMsg  string
	hasErr  bool
	resp    *advplrt.ObjectValue
	items   *advplrt.ArrayValue
	jsonErr *advplrt.ObjectValue
	active  bool
}

func init() {
	registerNativeBase("FWAPIMANAGER", func() *advplrt.ObjectValue {
		obj := advplrt.NewObject("FwApiManager", nil)
		obj.Native = &apiManagerState{}
		return obj
	})
}

// apiManagerStateOf extrai o estado — mesmo formato do adapterV2StateOf.
func apiManagerStateOf(obj *advplrt.ObjectValue) *apiManagerState {
	if st, ok := obj.Native.(*apiManagerState); ok {
		return st
	}
	if o, ok := obj.Native.(*advplrt.ObjectValue); ok {
		if st, ok := o.Native.(*apiManagerState); ok {
			return st
		}
	}
	return nil
}

// registerApiManagerNative registra a função FWAPIMANAGER(): o compilador
// emite OP_CALL para `FwApiManager()` (classe desconhecida no fonte), então
// a instanciação estilo TOTVS precisa de um nativo que devolve o objeto com
// o estado — mesma saída do newInstance para lookupNativeBase.
func (v *VM) registerApiManagerNative(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	natives["FWAPIMANAGER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := advplrt.NewObject("FwApiManager", nil)
		obj.Native = &apiManagerState{}
		return obj, nil
	}
}

// NativeCallMethod implementa nativeBaseMethodHandler para FwApiManager.
func (st *apiManagerState) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "NEW":
		st.servico = advplrt.ToString(getArg(args, 0))
		st.versao = advplrt.ToString(getArg(args, 1))
		if st.mapa == nil {
			st.mapa = map[string]MapaCampo{}
		}
		if st.params == nil {
			st.params = map[string]string{}
		}
		return obj, nil

	case "SETAPIALIAS":
		st.alias = strings.ToUpper(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
		return advplrt.Nil, nil

	case "SETAPIMAP":
		apiSetMap(st, v, getArg(args, 0))
		return advplrt.Nil, nil

	case "SETAPIFILTER":
		st.filterExpr = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil

	case "SETAPIQSTRING", "SETAPIQUERYSTRING":
		apiSetQString(st, getArg(args, 0))
		return advplrt.Nil, nil

	case "SETQUERY":
		st.queryTmpl = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil
	case "SETWHERE":
		st.whereSQL = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil
	case "SETORDER":
		st.orderExpr = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil

	// Relação mestre-detalhe: formato pendente do corpus — aceita sem
	// efeito para não quebrar fontes que a chamam.
	case "SETAPIRELATION":
		return advplrt.Nil, nil

	case "SETPAGE":
		st.page = int(advplrt.ToFloat(getArg(args, 0)))
		st.pageSet = true
		return advplrt.Nil, nil
	case "SETPAGESIZE":
		st.pageSize = int(advplrt.ToFloat(getArg(args, 0)))
		st.sizeSet = true
		return advplrt.Nil, nil

	case "ACTIVATE":
		if err := st.activate(v); err != nil {
			return advplrt.False, nil
		}
		st.active = true
		return advplrt.True, nil

	case "ISACTIVE":
		return advplrt.NewBool(st.active), nil

	case "DESTROY":
		st.active = false
		st.resp = nil
		st.items = nil
		return advplrt.Nil, nil

	case "GETJSONSERIALIZE":
		if st.resp == nil {
			st.resp = apiResponse(nil, false)
		}
		return advplrt.NewString(apiSerialize(st.resp)), nil

	case "TOOBJECTJSON", "GETJSONOBJECT":
		if st.resp == nil {
			st.resp = apiResponse(nil, false)
		}
		return st.resp, nil

	case "TOARRAY":
		if st.items == nil {
			st.items = advplrt.NewArray(nil)
		}
		return st.items, nil

	case "SETJSONERROR":
		code := int(advplrt.ToFloat(getArg(args, 0)))
		msg := advplrt.ToString(getArg(args, 1))
		st.jsonErr = advplrt.NewObject("json", nil)
		st.jsonErr.SetProp("code", advplrt.NewNumber(float64(code)))
		st.jsonErr.SetProp("message", advplrt.NewString(msg))
		// oJsonError é lido como propriedade do objeto.
		obj.SetProp("OJSONERROR", st.jsonErr)
		return advplrt.Nil, nil

	case "GETJSONERROR":
		if st.jsonErr == nil {
			return advplrt.Nil, nil
		}
		return st.jsonErr, nil

	case "TOEXECAUTO":
		return advplrt.Nil, fmt.Errorf("FwApiManager:ToExecAuto não suportado no AdvPP (requer ExecAuto/MVC)")

	default:
		return advplrt.Nil, fmt.Errorf("FwApiManager: método desconhecido %q", method)
	}
}

// apiSetMap lê aMapa: array de {prop, coluna} — array de 2 posições
// ({"codigo","NNR_CODIGO"}) ou objeto {PROP,CAMPO}/{PROPRIEDADE,COLUNA}.
// O tipo SX3 de cada coluna é resolvido no Activate (apiTiposSX3).
func apiSetMap(st *apiManagerState, v *VM, val advplrt.Value) {
	arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue)
	if !ok {
		return
	}
	for _, el := range arr.Elements {
		var prop, coluna string
		switch e := advplrt.Deref(el).(type) {
		case *advplrt.ArrayValue:
			if len(e.Elements) >= 2 {
				prop = advplrt.ToString(e.Elements[0])
				coluna = advplrt.ToString(e.Elements[1])
			}
		case *advplrt.ObjectValue:
			get := func(names ...string) string {
				for _, n := range names {
					if p, ok := e.Props[strings.ToUpper(n)]; ok {
						return advplrt.ToString(p)
					}
				}
				return ""
			}
			prop = get("prop", "propriedade", "cProp", "name")
			coluna = get("campo", "coluna", "cCampo", "column")
		}
		coluna = strings.ToUpper(strings.TrimSpace(coluna))
		if prop == "" || coluna == "" {
			continue
		}
		st.mapa[prop] = MapaCampo{Prop: prop, Coluna: coluna, Filtra: true, Ordena: true}
	}
}

// apiSetQString lê aQueryString: array de {chave, valor} — array de arrays
// de 2 posições ({"filter","x eq '1'"}) ou objetos {CHAVE,VALOR}/{KEY,VALUE}.
func apiSetQString(st *apiManagerState, val advplrt.Value) {
	arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue)
	if !ok {
		return
	}
	for _, el := range arr.Elements {
		var k, val2 string
		switch e := advplrt.Deref(el).(type) {
		case *advplrt.ArrayValue:
			if len(e.Elements) >= 2 {
				k = advplrt.ToString(e.Elements[0])
				val2 = advplrt.ToString(e.Elements[1])
			}
		case *advplrt.ObjectValue:
			get := func(names ...string) string {
				for _, n := range names {
					if p, ok := e.Props[strings.ToUpper(n)]; ok {
						return advplrt.ToString(p)
					}
				}
				return ""
			}
			k = get("chave", "key", "cKey", "name")
			val2 = get("valor", "value", "cValue", "val")
		}
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		st.params[k] = val2
	}
}

// apiParam devolve o parâmetro de query (SetApiQString) com default.
func (st *apiManagerState) apiParam(key, def string) string {
	if v, ok := st.params[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// apiTiposSX3 enriquece o mapa com o tipo de cada coluna lido do SX3 local
// (mesma consulta do TamSx3). Sem engine/linha, o tipo fica "" (string).
func apiTiposSX3(v *VM, st *apiManagerState) {
	eng, ok := v.dbEngine.(SQLEngine)
	if !ok {
		return
	}
	for prop, mc := range st.mapa {
		rows, err := eng.QueryRows(
			"SELECT X3_TIPO FROM SX3 WHERE UPPER(X3_CAMPO) = ? AND UPPER(X3_ARQUIVO) = ?",
			mc.Coluna, st.alias)
		if err == nil && len(rows) > 0 {
			mc.Tipo = strings.TrimSpace(rows[0]["X3_TIPO"])
			st.mapa[prop] = mc
		}
	}
}

// apiResponse monta {"items":[...],"hasNext":bool}.
func apiResponse(items []advplrt.Value, hasNext bool) *advplrt.ObjectValue {
	obj := advplrt.NewObject("json", nil)
	if items == nil {
		items = []advplrt.Value{}
	}
	obj.SetProp("items", advplrt.NewArray(items))
	obj.SetProp("hasNext", advplrt.NewBool(hasNext))
	return obj
}

// apiSerialize serializa o objeto de resposta para JSON (mesma conversão
// das rotas REST).
func apiSerialize(obj *advplrt.ObjectValue) string {
	b, err := json.Marshal(advplValueToJSON(obj))
	if err != nil {
		return ""
	}
	return string(b)
}

// activate executa a consulta de leitura e monta st.resp/st.items.
func (st *apiManagerState) activate(v *VM) error {
	st.hasErr = false
	st.errMsg = ""
	if st.alias == "" {
		st.hasErr = true
		st.errMsg = "FwApiManager: SetApiAlias não foi chamado"
		return fmt.Errorf("%s", st.errMsg)
	}
	if !identRe.MatchString(st.alias) {
		st.hasErr = true
		st.errMsg = "FwApiManager: alias inválido " + st.alias
		return fmt.Errorf("%s", st.errMsg)
	}
	if len(st.mapa) == 0 {
		st.hasErr = true
		st.errMsg = "FwApiManager: SetApiMap não foi chamado"
		return fmt.Errorf("%s", st.errMsg)
	}
	apiTiposSX3(v, st)

	page := st.page
	if !st.pageSet {
		if n, err := strconv.Atoi(strings.TrimSpace(st.apiParam("page", "1"))); err == nil && n > 0 {
			page = n
		}
	}
	if page <= 0 {
		page = 1
	}
	pageSize := st.pageSize
	if !st.sizeSet {
		if n, err := strconv.Atoi(strings.TrimSpace(st.apiParam("pageSize", "10"))); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	cols, err := TraduzCampos(st.apiParam("fields", ""), st.mapa)
	if err != nil {
		st.hasErr = true
		st.errMsg = err.Error()
		return err
	}
	fieldsSQL := strings.Join(cols, ", ")

	var whereParts []string
	var args []any
	if w := strings.TrimSpace(st.whereSQL); w != "" {
		whereParts = append(whereParts, "("+w+")")
	}
	filtros := []string{}
	if st.filterExpr != "" {
		filtros = append(filtros, "("+strings.TrimSpace(st.filterExpr)+")")
	}
	if f := strings.TrimSpace(st.apiParam("filter", "")); f != "" {
		filtros = append(filtros, "("+f+")")
	}
	if len(filtros) > 0 {
		fsql, fargs, err := TraduzFiltro(strings.Join(filtros, " and "), st.mapa)
		if err != nil {
			st.hasErr = true
			st.errMsg = err.Error()
			return err
		}
		whereParts = append(whereParts, "("+fsql+")")
		args = append(args, fargs...)
	}
	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = "WHERE " + strings.Join(whereParts, " AND ")
	}

	q := st.queryTmpl
	if strings.TrimSpace(q) == "" {
		q = "SELECT #QueryFields# FROM " + st.alias + " #QueryWhere#"
	}
	if strings.Contains(q, "#QueryFields#") {
		q = strings.ReplaceAll(q, "#QueryFields#", fieldsSQL)
	}
	if strings.Contains(q, "#QueryWhere#") {
		q = strings.ReplaceAll(q, "#QueryWhere#", whereSQL)
	} else if whereSQL != "" {
		q += " " + whereSQL
	}
	ordExpr := st.orderExpr
	if ordExpr == "" {
		ordExpr = st.apiParam("order", "")
	}
	if strings.TrimSpace(ordExpr) != "" {
		osql, err := TraduzOrdem(ordExpr, st.mapa)
		if err != nil {
			st.hasErr = true
			st.errMsg = err.Error()
			return err
		}
		if osql != "" {
			q += " ORDER BY " + osql
		}
	}
	q += " LIMIT ? OFFSET ?"
	args = append(args, pageSize+1, (page-1)*pageSize)

	eng, ok := v.dbEngine.(SQLEngine)
	if !ok {
		st.hasErr = true
		st.errMsg = "FwApiManager: engine de banco sem suporte a SQL"
		return fmt.Errorf("%s", st.errMsg)
	}
	rows, err := eng.QueryRows(q, args...)
	if err != nil {
		st.hasErr = true
		st.errMsg = "FwApiManager: " + err.Error()
		return err
	}
	hasNext := len(rows) > pageSize
	if hasNext {
		rows = rows[:pageSize]
	}
	items := make([]advplrt.Value, 0, len(rows))
	for _, row := range rows {
		obj := advplrt.NewObject("json", nil)
		for prop, mc := range st.mapa {
			raw, ok := row[strings.ToUpper(mc.Coluna)]
			if !ok {
				continue
			}
			obj.SetProp(prop, adapterJSONValue(mc.Tipo, raw))
		}
		items = append(items, obj)
	}
	st.items = advplrt.NewArray(items)
	st.resp = apiResponse(items, hasNext)
	return nil
}
