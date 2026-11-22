package vm

import (
	"fmt"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// adapterV2State é o estado Go da classe nativa FWAdapterBaseV2 (base dos
// adapters REST do corpus PO-UI): mapa de campos, query template com
// #QueryFields#/#QueryWhere#, paginação, filtro/ordem da URL e a resposta
// {"items":[...],"hasNext":bool} montada por Execute/FillGetResponse.
//
// Premissas documentadas (corpus indisponível — confirmar quando houver
// ADVPP_CORPUS_2510):
//   - lFiltra/lOrdena de AddMapFields: ausente/Nil vale .T. (permissivo);
//     .F. explícito desliga filtro/ordenação do campo.
//   - aStruct de AddMapFields: array de {campo, tipo, tamanho, decimais}
//     (array de arrays ou de objetos JsonObject); o tipo é indexado pelo
//     nome da coluna para a conversão do JSON.
//   - SetWhere recebe fragmento SQL cru do fonte (confiável); SetUrlFilter
//     recebe array de {"FILTER", cFiltro} (array de arrays ou objetos).
//   - page/pageSize/filter/order/fields da query string da requisição
//     corrente (v.restReq) preenchem os defaults; os setters explícitos
//     vencem.
type adapterV2State struct {
	verbo  string
	isList bool

	mapa  map[string]MapaCampo
	tipos map[string]sx3Tipo // coluna (maiúscula) -> tipo/tamanho/decimais

	page     int
	pageSize int
	pageSet  bool
	sizeSet  bool

	queryTmpl     string
	whereSQL      string
	orderExpr     string // SetOrder: "-campo,campo"
	orderQuerySQL string // SetOrderQuery: fragmento ORDER BY cru
	urlFilter     []string
	fieldsSel     string

	errMsg string
	hasErr bool
	resp   *advplrt.ObjectValue
}

type sx3Tipo struct {
	tipo   string
	tam    int
	dec    int
	temTyp bool
}

func init() {
	registerNativeBase("FWADAPTERBASEV2", func() *advplrt.ObjectValue {
		obj := advplrt.NewObject("FWAdapterBaseV2", nil)
		obj.Native = &adapterV2State{}
		return obj
	})
}

// adapterV2StateOf extrai o estado do adapter do objeto da instância:
// obj.Native é o *ObjectValue devolvido pela fábrica (que por sua vez
// guarda o *adapterV2State em .Native).
func adapterV2StateOf(obj *advplrt.ObjectValue) *adapterV2State {
	if st, ok := obj.Native.(*adapterV2State); ok {
		return st
	}
	if o, ok := obj.Native.(*advplrt.ObjectValue); ok {
		if st, ok := o.Native.(*adapterV2State); ok {
			return st
		}
	}
	return nil
}

// NativeCallMethod implementa nativeBaseMethodHandler — os métodos da
// FWAdapterBaseV2 que o bytecode da subclasse não define caem aqui via
// callNativeMethodOn/default. O valor devolvido é empurrado pela VM.
func (st *adapterV2State) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "NEW":
		st.verbo = advplrt.ToString(getArg(args, 0))
		st.isList = adapterTruthy(getArg(args, 1), true)
		if st.mapa == nil {
			st.mapa = map[string]MapaCampo{}
			st.tipos = map[string]sx3Tipo{}
		}
		return obj, nil

	case "ADDMAPFIELDS":
		if st.mapa == nil {
			st.mapa = map[string]MapaCampo{}
			st.tipos = map[string]sx3Tipo{}
		}
		adapterAddMapFields(st, args)
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
	case "SETORDERQUERY":
		st.orderQuerySQL = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil
	case "SETPAGE":
		st.page = int(advplrt.ToFloat(getArg(args, 0)))
		st.pageSet = true
		return advplrt.Nil, nil
	case "SETPAGESIZE":
		st.pageSize = int(advplrt.ToFloat(getArg(args, 0)))
		st.sizeSet = true
		return advplrt.Nil, nil
	case "SETURLFILTER":
		adapterSetURLFilter(st, getArg(args, 0))
		return advplrt.Nil, nil
	case "SETFIELDS":
		st.fieldsSel = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil

	case "GETFIXQUERY":
		sql, _, _, err := st.buildQuery(v)
		if err != nil {
			return advplrt.Nil, err
		}
		return advplrt.NewString(sql), nil

	case "EXECUTE":
		if err := st.execute(v); err != nil {
			return advplrt.False, nil
		}
		return advplrt.True, nil

	case "FILLGETRESPONSE", "GETJSONRESPONSE":
		if st.resp == nil {
			st.resp = adapterResponse(nil, false)
		}
		return st.resp, nil

	case "GETERRORMESSAGE":
		return advplrt.NewString(st.errMsg), nil
	case "HASERRORMESSAGE":
		return advplrt.NewBool(st.hasErr), nil

	default:
		return advplrt.Nil, fmt.Errorf("FWAdapterBaseV2: método desconhecido %q", method)
	}
}

// adapterTruthy resolve argumentos lógicos. def é o valor quando o
// argumento está ausente/Nil — usado para lFiltra/lOrdena (default .T.,
// ver premissa no tipo).
func adapterTruthy(val advplrt.Value, def bool) bool {
	if val == nil || advplrt.Deref(val) == advplrt.Nil {
		return def
	}
	return advplrt.ToBool(val)
}

// adapterAddMapFields implementa AddMapFields(cProp, cCampo, lFiltra,
// lOrdena, aStruct): registra a propriedade no mapa e indexa os tipos do
// aStruct por coluna para a conversão JSON do Execute.
func adapterAddMapFields(st *adapterV2State, args []advplrt.Value) {
	prop := advplrt.ToString(getArg(args, 0))
	coluna := strings.ToUpper(strings.TrimSpace(advplrt.ToString(getArg(args, 1))))
	filtra := adapterTruthy(getArg(args, 2), true)
	ordena := adapterTruthy(getArg(args, 3), true)

	typ := sx3Tipo{}
	if arr, ok := advplrt.Deref(getArg(args, 4)).(*advplrt.ArrayValue); ok {
		for _, el := range arr.Elements {
			if t, ok := adapterStructEntry(el); ok {
				st.tipos[t.coluna] = t.sx3
				if strings.EqualFold(t.coluna, coluna) {
					typ = t.sx3
					typ.temTyp = true
				}
			}
		}
	}
	st.mapa[prop] = MapaCampo{
		Prop:   prop,
		Coluna: coluna,
		Tipo:   typ.tipo,
		Filtra: filtra,
		Ordena: ordena,
	}
}

// adapterStructEntry lê uma entrada do aStruct: {campo, tipo, tamanho,
// decimais} como array de 4 posições ou objeto com chaves CAMPO/TIPO/
// TAMANHO/DECIMAIS (JsonObject).
func adapterStructEntry(val advplrt.Value) (struct {
	coluna string
	sx3    sx3Tipo
}, bool) {
	var out struct {
		coluna string
		sx3    sx3Tipo
	}
	switch e := advplrt.Deref(val).(type) {
	case *advplrt.ArrayValue:
		if len(e.Elements) < 4 {
			return out, false
		}
		out.coluna = strings.ToUpper(strings.TrimSpace(advplrt.ToString(e.Elements[0])))
		out.sx3 = sx3Tipo{
			tipo:   strings.ToUpper(strings.TrimSpace(advplrt.ToString(e.Elements[1]))),
			tam:    int(advplrt.ToFloat(e.Elements[2])),
			dec:    int(advplrt.ToFloat(e.Elements[3])),
			temTyp: true,
		}
	case *advplrt.ObjectValue:
		get := func(names ...string) advplrt.Value {
			for _, n := range names {
				if p, ok := e.Props[strings.ToUpper(n)]; ok {
					return p
				}
			}
			return advplrt.Nil
		}
		campo := advplrt.ToString(get("campo", "X3_CAMPO", "cCampo"))
		if campo == "" {
			return out, false
		}
		out.coluna = strings.ToUpper(strings.TrimSpace(campo))
		out.sx3 = sx3Tipo{
			tipo:   strings.ToUpper(strings.TrimSpace(advplrt.ToString(get("tipo", "X3_TIPO", "cTipo")))),
			tam:    int(advplrt.ToFloat(get("tamanho", "X3_TAMANHO", "nTamanho"))),
			dec:    int(advplrt.ToFloat(get("decimais", "X3_DECIMAL", "nDecimais"))),
			temTyp: true,
		}
	default:
		return out, false
	}
	if out.coluna == "" {
		return out, false
	}
	return out, true
}

// adapterSetURLFilter lê o aFiltro do SetUrlFilter: array de entradas
// {"FILTER", cFiltro} — cada entrada é um array de 2 posições
// ({"FILTER", valor}) ou um objeto com a chave FILTER.
func adapterSetURLFilter(st *adapterV2State, val advplrt.Value) {
	arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue)
	if !ok {
		return
	}
	for _, el := range arr.Elements {
		switch e := advplrt.Deref(el).(type) {
		case *advplrt.ArrayValue:
			for i := 0; i+1 < len(e.Elements); i++ {
				if strings.EqualFold(advplrt.ToString(e.Elements[i]), "FILTER") {
					st.urlFilter = append(st.urlFilter, advplrt.ToString(e.Elements[i+1]))
				}
			}
		case *advplrt.ObjectValue:
			for _, k := range e.Keys {
				if strings.EqualFold(k, "FILTER") {
					st.urlFilter = append(st.urlFilter, advplrt.ToString(e.Props[k]))
				}
			}
		}
	}
}

// adapterQueryValue lê um parâmetro da query string da requisição REST
// corrente (v.restReq). Fora de dispatch REST devolve "".
func adapterQueryValue(v *VM, key string) string {
	if v.restReq == nil {
		return ""
	}
	vals := v.restReq.Query[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// buildQuery monta o SQL final e os argumentos. Devolve também o
// pageSize efetivo (o Execute busca pageSize+1 linhas para o hasNext).
func (st *adapterV2State) buildQuery(v *VM) (string, []any, int, error) {
	if strings.TrimSpace(st.queryTmpl) == "" {
		return "", nil, 0, fmt.Errorf("FWAdapterBaseV2: SetQuery não foi chamado")
	}

	page := st.page
	if !st.pageSet {
		if n, err := strconv.Atoi(strings.TrimSpace(adapterQueryValue(v, "page"))); err == nil && n > 0 {
			page = n
		}
	}
	if page <= 0 {
		page = 1
	}
	pageSize := st.pageSize
	if !st.sizeSet {
		if n, err := strconv.Atoi(strings.TrimSpace(adapterQueryValue(v, "pageSize"))); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	fieldsExpr := st.fieldsSel
	if fieldsExpr == "" {
		fieldsExpr = adapterQueryValue(v, "fields")
	}
	cols, err := TraduzCampos(fieldsExpr, st.mapa)
	if err != nil {
		return "", nil, 0, err
	}
	fieldsSQL := strings.Join(cols, ", ")

	// WHERE: SetWhere cru + filtro OData da URL/SetUrlFilter, sempre
	// parametrizado. Cada expressão explícita vai entre parênteses para
	// não mudar a precedência do "and" que as une.
	var whereParts []string
	var args []any
	if w := strings.TrimSpace(st.whereSQL); w != "" {
		whereParts = append(whereParts, "("+w+")")
	}
	var filtros []string
	filtros = append(filtros, st.urlFilter...)
	if f := strings.TrimSpace(adapterQueryValue(v, "filter")); f != "" {
		filtros = append(filtros, f)
	}
	if len(filtros) > 0 {
		expr := make([]string, len(filtros))
		for i, f := range filtros {
			expr[i] = "(" + strings.TrimSpace(f) + ")"
		}
		fsql, fargs, err := TraduzFiltro(strings.Join(expr, " and "), st.mapa)
		if err != nil {
			return "", nil, 0, err
		}
		whereParts = append(whereParts, "("+fsql+")")
		args = append(args, fargs...)
	}
	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = "WHERE " + strings.Join(whereParts, " AND ")
	}

	orderSQL := ""
	if o := strings.TrimSpace(st.orderQuerySQL); o != "" {
		orderSQL = "ORDER BY " + o
	} else {
		ordExpr := st.orderExpr
		if ordExpr == "" {
			ordExpr = adapterQueryValue(v, "order")
		}
		if strings.TrimSpace(ordExpr) != "" {
			osql, err := TraduzOrdem(ordExpr, st.mapa)
			if err != nil {
				return "", nil, 0, err
			}
			if osql != "" {
				orderSQL = "ORDER BY " + osql
			}
		}
	}

	q := st.queryTmpl
	if strings.Contains(q, "#QueryFields#") {
		q = strings.ReplaceAll(q, "#QueryFields#", fieldsSQL)
	}
	if strings.Contains(q, "#QueryWhere#") {
		q = strings.ReplaceAll(q, "#QueryWhere#", whereSQL)
	} else if whereSQL != "" {
		q += " " + whereSQL
	}
	if orderSQL != "" {
		q += " " + orderSQL
	}
	if st.isList {
		// pageSize+1 linhas: a linha extra decide o hasNext sem COUNT.
		q += " LIMIT ? OFFSET ?"
		args = append(args, pageSize+1, (page-1)*pageSize)
	}
	return q, args, pageSize, nil
}

// execute roda a query e monta st.resp ({"items":[...],"hasNext":bool}).
// Falhas viram HasErrorMessage/GetErrorMessage — nunca panic.
func (st *adapterV2State) execute(v *VM) error {
	st.hasErr = false
	st.errMsg = ""

	sql, args, pageSize, err := st.buildQuery(v)
	if err != nil {
		st.hasErr = true
		st.errMsg = err.Error()
		return err
	}
	eng, ok := v.dbEngine.(SQLEngine)
	if !ok {
		st.hasErr = true
		st.errMsg = "FWAdapterBaseV2: engine de banco sem suporte a SQL"
		return fmt.Errorf("%s", st.errMsg)
	}
	rows, err := eng.QueryRows(sql, args...)
	if err != nil {
		st.hasErr = true
		st.errMsg = "FWAdapterBaseV2: " + err.Error()
		return err
	}
	hasNext := st.isList && len(rows) > pageSize
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
	st.resp = adapterResponse(items, hasNext)
	return nil
}

// adapterResponse monta o objeto {"items":[...], "hasNext":bool}.
func adapterResponse(items []advplrt.Value, hasNext bool) *advplrt.ObjectValue {
	obj := advplrt.NewObject("json", nil)
	obj.SetProp("items", advplrt.NewArray(items))
	obj.SetProp("hasNext", advplrt.NewBool(hasNext))
	return obj
}

// registerTamSx3Native registra TamSx3(cCampo): metadado do campo no
// dicionário SX3 local ({nTamanho, nDecimais, cTipo}). Campo inexistente
// ou engine sem SQL devolve {0, 0, ""} — nunca erro.
func (v *VM) registerTamSx3Native(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	natives["TAMSX3"] = func(args []advplrt.Value) (advplrt.Value, error) {
		out := map[string]any{"nTamanho": float64(0), "nDecimais": float64(0), "cTipo": ""}
		eng, ok := v.dbEngine.(SQLEngine)
		if ok {
			campo := strings.ToUpper(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
			if campo != "" {
				if rows, err := eng.QueryRows(
					"SELECT X3_TAMANHO, X3_DECIMAL, X3_TIPO FROM SX3 WHERE UPPER(X3_CAMPO) = ?",
					campo); err == nil && len(rows) > 0 {
					r := rows[0]
					tam, _ := strconv.Atoi(strings.TrimSpace(r["X3_TAMANHO"]))
					dec, _ := strconv.Atoi(strings.TrimSpace(r["X3_DECIMAL"]))
					out["nTamanho"] = float64(tam)
					out["nDecimais"] = float64(dec)
					out["cTipo"] = strings.TrimSpace(r["X3_TIPO"])
				}
			}
		}
		return jsonMapToAdvplObject(out), nil
	}
}

// adapterJSONValue converte o texto da linha QueryRows para o tipo do
// campo: N -> número, L -> lógico, demais -> string.
func adapterJSONValue(tipo, raw string) advplrt.Value {
	switch strings.ToUpper(strings.TrimSpace(tipo)) {
	case "N", "NUM", "FLOAT", "DECIMAL", "DOUBLE", "REAL", "I", "INT", "INTEGER":
		if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			return advplrt.NewNumber(f)
		}
		return advplrt.NewString(raw)
	case "L", "LOGICAL", "BOOL":
		switch strings.ToUpper(strings.TrimSpace(raw)) {
		case "T", "1", "TRUE", "S", "SIM":
			return advplrt.True
		default:
			return advplrt.False
		}
	default:
		return advplrt.NewString(raw)
	}
}

