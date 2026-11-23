package vm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// apiManagerState é o estado Go da classe nativa FwApiManager — a camada de
// API de leitura usada pelos apps PO-UI e pelos WSMETHOD dos adapters do
// corpus 12.1.2510 (crm/FWAPIManager.prw é a referência).
//
// Formato REAL do aApiMap (evidência do corpus, ApiMap() de CRMS040 etc.):
//
//	aApiMap := {cAdapter, cApiName, cVersion, cNick, aStructs, cRootName}
//	aStructs[n] := {cAlias, cType, cNomeJson, cNick, aFields}
//	aFields[m]  := {cNomeApi, cCampoProtheus}   // campo pode ser "Exp:expr"
//	                                            // ou "COL1, COL2" (chave
//	                                            // interna composta)
//
// SetApiRelation(aChld, aFather, aConditions, cIndexKey):
//
//	aChld/aFather := {cAlias, cSchema, cNick}; aConditions :=
//	{{cCampoPai, cCampoFilho}, ...}; cIndexKey := ORDER BY do filho.
//
// Ciclo de vida real: Activate() só valida (lActive); SetJsonObject()
// executa a consulta e monta oJsonData = {hasNext, <cApiName>: [itens]}
// com structs filhos aninhados via SetApiRelation. Aqui a execução também
// é preguiçosa nos getters (GetJsonSerialize/ToObjectJson) — superset que
// mantém fontes que pulam SetJsonObject funcionando.
//
// Desvios documentados:
//   - SetApiMap também aceita o formato plano {prop, coluna} usado pela
//     fixture interna (não existe no corpus — mantido por compat).
//   - "filter" na querystring segue aceito como filtro OData (extensão;
//     no TOTVS qualquer chave desconhecida vira filtro de igualdade por
//     campo da API).
//   - Campos "Exp:..." são computados em workarea no Protheus — aqui são
//     omitidos do SELECT/projeção (não há avaliador de expressão).
//   - SetQuery (query custom por nick) executa a query raiz sem a
//     distribuição hierárquica do caminho lEspQuery — itens saem planos.
//   - ToArray/ToJsonArray/GetEstJson (caminho de escrita via JSON body)
//     retornam vazio — escrita depende de ExecAuto/MVC, inexistente.
type apiStructField struct {
	api   string // nome na API (chave do JSON)
	campo string // coluna(s) Protheus, "Exp:..." ou "COL1, COL2"
}

type apiStruct struct {
	alias, typ, name, nick string
	fields                 []apiStructField
}

// apiRelation espelha Self:aRelations do FWAPIManager real:
// {childAlias, fatherAlias, conditions, indexKey, childSchema,
//
//	fatherSchema, childNick, fatherNick}.
type apiRelation struct {
	childAlias, fatherAlias string
	pairs                   [][2]string // {campoPai, campoFilho}
	indexKey                string
	childNick, fatherNick   string
}

type apiManagerState struct {
	servico, versao string
	alias           string
	aliasInfo       string // aAlias[2] quando SetApiAlias recebe array
	aliasNick       string // aAlias[3]

	// Formato real do corpus.
	apiName   string // aApiMap[2] — chave raiz do JSON de resposta
	structs   []apiStruct
	relations []apiRelation

	// Formato plano legado {prop, coluna} (fixture interna).
	mapa      map[string]MapaCampo
	legacyMap bool

	// Parâmetros de consulta.
	fieldFilter map[string]bool           // api names pedidas em FIELDS
	filters     map[string][]string       // nick -> fragmentos "COL = 'v'"
	odataFilter string                    // extensão "filter" (OData)
	indexKey    string                    // ORDER do alias raiz
	expFilters  map[string][]apiExpFilter // nick -> {api, valor}
	customQuery map[string][2]string      // nick -> {query, group}
	queryTmpl   string                    // extensão SetQuery posicional antiga
	whereSQL    string                    // extensão SetWhere
	orderExpr   string                    // extensão SetOrder

	page     int
	pageSet  bool
	pageSize int
	sizeSet  bool

	lUseHasNext bool
	errMsg      string
	hasErr      bool
	active      bool
	executed    bool
	jsonData    *advplrt.ObjectValue
	items       *advplrt.ArrayValue
	jsonErr     *advplrt.ObjectValue
}

type apiExpFilter struct {
	api, valor string
}

func init() {
	registerNativeBase("FWAPIMANAGER", func() *advplrt.ObjectValue {
		obj := advplrt.NewObject("FwApiManager", nil)
		obj.Native = &apiManagerState{lUseHasNext: true}
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
		obj.Native = &apiManagerState{lUseHasNext: true}
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
		if st.filters == nil {
			st.filters = map[string][]string{}
		}
		return obj, nil

	case "SETAPIADAPTER":
		if s := strings.TrimSpace(advplrt.ToString(getArg(args, 0))); s != "" {
			st.servico = s
		}
		return advplrt.Nil, nil
	case "SETAPIVERSION":
		if s := strings.TrimSpace(advplrt.ToString(getArg(args, 0))); s != "" {
			st.versao = s
		}
		return advplrt.Nil, nil

	case "SETAPIALIAS":
		// Real: SetApiAlias({cAlias, cInfo, cNick}) — array de 3 posições.
		// Extensão: string simples (fixture legada).
		if arr, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ArrayValue); ok && len(arr.Elements) >= 1 {
			st.alias = strings.ToUpper(strings.TrimSpace(advplrt.ToString(arr.Elements[0])))
			if len(arr.Elements) >= 2 {
				st.aliasInfo = advplrt.ToString(arr.Elements[1])
			}
			if len(arr.Elements) >= 3 {
				st.aliasNick = advplrt.ToString(arr.Elements[2])
			}
		} else {
			st.alias = strings.ToUpper(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
		}
		return advplrt.Nil, nil

	case "SETAPIMAP":
		apiSetMap(st, v, getArg(args, 0))
		return advplrt.Nil, nil

	case "SETAPIRELATION":
		// Real: SetApiRelation(aChld, aFather, aConditions, cIndexKey)
		// onde aConditions = {{campoPai, campoFilho}, ...}.
		child := apiArr3(getArg(args, 0))
		father := apiArr3(getArg(args, 1))
		rel := apiRelation{
			childAlias:  strings.ToUpper(strings.TrimSpace(child[0])),
			fatherAlias: strings.ToUpper(strings.TrimSpace(father[0])),
			childNick:   child[2],
			fatherNick:  father[2],
			indexKey:    advplrt.ToString(getArg(args, 3)),
		}
		if conds, ok := advplrt.Deref(getArg(args, 2)).(*advplrt.ArrayValue); ok {
			for _, c := range conds.Elements {
				if pair, ok := advplrt.Deref(c).(*advplrt.ArrayValue); ok && len(pair.Elements) >= 2 {
					rel.pairs = append(rel.pairs, [2]string{
						advplrt.ToString(pair.Elements[0]),
						advplrt.ToString(pair.Elements[1]),
					})
				}
			}
		}
		st.relations = append(st.relations, rel)
		return advplrt.Nil, nil

	case "SETAPIQSTRING", "SETAPIQUERYSTRING":
		apiSetQstring(st, getArg(args, 0))
		return advplrt.Nil, nil

	case "SETAPIFIELDS":
		// Real: csv de nomes de campo da API -> aFieldFilter.
		st.fieldFilter = map[string]bool{}
		for _, f := range strings.Split(advplrt.ToString(getArg(args, 0)), ",") {
			if f = strings.ToUpper(strings.TrimSpace(f)); f != "" {
				st.fieldFilter[f] = true
			}
		}
		return advplrt.Nil, nil

	case "SETAPIFILTER":
		// Real: {cAlias, cNick, {cond, cond, ...}} — fragmentos SQL crus.
		if arr, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ArrayValue); ok && len(arr.Elements) >= 3 {
			nick := advplrt.ToString(arr.Elements[1])
			if conds, ok := advplrt.Deref(arr.Elements[2]).(*advplrt.ArrayValue); ok {
				for _, c := range conds.Elements {
					st.filters[nick] = append(st.filters[nick], advplrt.ToString(c))
				}
			}
		}
		return advplrt.Nil, nil

	case "SETEXPFILTER":
		// Filtros sobre campos computados ("Exp:") — aplicados pós-fetch
		// por comparação de valor, como o AdvPL faz no workarea.
		if arr, ok := advplrt.Deref(getArg(args, 0)).(*advplrt.ArrayValue); ok {
			for _, e := range arr.Elements {
				if tup, ok := advplrt.Deref(e).(*advplrt.ArrayValue); ok && len(tup.Elements) >= 3 {
					if vals, ok := advplrt.Deref(tup.Elements[2]).(*advplrt.ArrayValue); ok {
						for _, val := range vals.Elements {
							st.expFilters[advplrt.ToString(tup.Elements[1])] = append(
								st.expFilters[advplrt.ToString(tup.Elements[1])],
								apiExpFilter{api: advplrt.ToString(tup.Elements[0]), valor: advplrt.ToString(val)})
						}
					}
				}
			}
		}
		return advplrt.Nil, nil

	case "SETINDEXKEY":
		st.indexKey = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil

	case "SETQUERY":
		// Real: SetQuery(cNickName, cQuery, cGroup) — query custom por
		// struct. O caminho lEspQuery do Protheus distribui as linhas do
		// join nos níveis do JSON; aqui executamos a query do nick raiz
		// de forma plana (limitação documentada no cabeçalho).
		// Compat: chamada com 1 argumento era o template legado
		// "SELECT #QueryFields# FROM alias #QueryWhere#" da fixture.
		if len(args) < 2 || strings.TrimSpace(advplrt.ToString(getArg(args, 1))) == "" {
			st.queryTmpl = advplrt.ToString(getArg(args, 0))
			return advplrt.Nil, nil
		}
		nick := advplrt.ToString(getArg(args, 0))
		if st.customQuery == nil {
			st.customQuery = map[string][2]string{}
		}
		st.customQuery[nick] = [2]string{
			advplrt.ToString(getArg(args, 1)),
			advplrt.ToString(getArg(args, 2)),
		}
		return advplrt.Nil, nil

	// Extensões da fixture interna (não existem no FWAPIManager real).
	case "SETWHERE":
		st.whereSQL = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil
	case "SETORDER":
		st.orderExpr = advplrt.ToString(getArg(args, 0))
		return advplrt.Nil, nil

	case "SETPAGE":
		st.page = int(advplrt.ToFloat(getArg(args, 0)))
		st.pageSet = true
		return advplrt.Nil, nil
	case "SETPAGESIZE":
		st.pageSize = int(advplrt.ToFloat(getArg(args, 0)))
		st.sizeSet = true
		return advplrt.Nil, nil

	case "SETAPIHASNEXT":
		st.lUseHasNext = advplrt.ToBool(getArg(args, 0))
		return advplrt.Nil, nil

	case "ACTIVATE":
		// Real: só valida e liga lActive — a consulta acontece em
		// SetJsonObject. Mantemos a mesma separação.
		if err := st.activateValidate(); err != nil {
			return advplrt.False, nil
		}
		st.active = true
		return advplrt.True, nil

	case "ISACTIVE":
		return advplrt.NewBool(st.active && !st.hasErr), nil

	case "SETJSONOBJECT":
		if !st.active {
			return advplrt.False, nil
		}
		if err := st.execute(v); err != nil {
			return advplrt.False, nil
		}
		return advplrt.True, nil

	case "DESTROY":
		st.active = false
		st.executed = false
		st.jsonData = nil
		st.items = nil
		return advplrt.Nil, nil

	// Getters — executam preguiçosamente se Activate rodou mas
	// SetJsonObject não (superset documentado).
	case "GETJSONOBJECT", "TOOBJECTJSONDATA":
		st.ensureExecuted(v)
		if st.jsonData == nil {
			return advplrt.Nil, nil
		}
		return st.jsonData, nil

	case "GETJSONSERIALIZE":
		st.ensureExecuted(v)
		if st.jsonData == nil {
			return advplrt.NewString("null"), nil
		}
		return advplrt.NewString(apiSerialize(st.jsonData)), nil

	case "TOOBJECTJSON":
		// Real: serializa apenas o PRIMEIRO item do array da API.
		st.ensureExecuted(v)
		if st.items == nil || len(st.items.Elements) == 0 {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(apiSerializeObject(st.items.Elements[0])), nil

	case "TOARRAY":
		// Caminho de escrita (body JSON -> campos Protheus): não
		// suportado — depende de ExecAuto. Retorna array vazio.
		return advplrt.NewArray(nil), nil
	case "TOJSONARRAY", "GETJSONARRAY":
		return advplrt.NewArray(nil), nil
	case "GETESTJSON":
		return advplrt.NewString(""), nil

	case "SETJSONERROR":
		// Real: SetJsonError(cCode, cMessage, cHelp, aDetails) — código
		// vem como STRING ("400").
		code := advplrt.ToString(getArg(args, 0))
		msg := advplrt.ToString(getArg(args, 1))
		st.jsonErr = advplrt.NewObject("json", nil)
		st.jsonErr.SetProp("code", advplrt.NewString(code))
		st.jsonErr.SetProp("message", advplrt.NewString(msg))
		obj.SetProp("OJSONERROR", st.jsonErr)
		st.hasErr = true
		return advplrt.Nil, nil

	case "GETJSONERROR", "GETAPIERROR":
		if st.jsonErr == nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(apiSerialize(st.jsonErr)), nil

	// Getters de configuração (baratos, fiéis ao corpus).
	case "GETAPIALIAS":
		return advplrt.NewString(st.alias), nil
	case "GETAPIMAP":
		return advplrt.Nil, nil // estrutura interna — não exposta como valor
	case "GETAPIRELATION":
		return advplrt.NewArray(nil), nil
	case "GETAPIFIELDS":
		return advplrt.NewArray(nil), nil
	case "GETAPIADAPTER":
		return advplrt.NewString(st.servico), nil
	case "GETAPIVERSION":
		return advplrt.NewString(st.versao), nil
	case "GETAPIPAGE":
		return advplrt.NewNumber(float64(st.page)), nil
	case "GETAPIPGSIZE":
		return advplrt.NewNumber(float64(st.pageSize)), nil
	case "GETQUERY":
		return advplrt.NewArray(nil), nil
	case "GETAPIFILTER":
		return advplrt.NewArray(nil), nil
	case "GETEXPFILTER":
		return advplrt.NewArray(nil), nil
	case "GETORDERKEY":
		return advplrt.NewString(st.indexKey), nil
	case "GETPROTFIELD":
		return advplrt.NewString(""), nil
	case "GETPROTINFO":
		return advplrt.NewArray(nil), nil

	case "TOEXECAUTO":
		return advplrt.Nil, fmt.Errorf("FwApiManager:ToExecAuto não suportado no AdvPP (requer ExecAuto/MVC)")

	default:
		return advplrt.Nil, fmt.Errorf("FwApiManager: método desconhecido %q", method)
	}
}

// apiArr3 lê um array AdvPL de até 3 strings (aChildrenAlias/aFatherAlias).
func apiArr3(val advplrt.Value) [3]string {
	var out [3]string
	if arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue); ok {
		for i := 0; i < len(arr.Elements) && i < 3; i++ {
			out[i] = advplrt.ToString(arr.Elements[i])
		}
	}
	return out
}

// apiSetMap lê aApiMap nos DOIS formatos:
//
//	Real (corpus): {adapter, apiName, version, nick, structs[], root}
//	Legado:        {{prop, coluna}, ...} ou {{prop, coluna}, ...} flat
func apiSetMap(st *apiManagerState, v *VM, val advplrt.Value) {
	arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue)
	if !ok {
		return
	}
	// Detecção do formato real: elemento [4] (0-based) é array de structs,
	// cada struct um array de >=5 posições com [4] array de fields.
	if len(arr.Elements) >= 5 {
		if list, ok := advplrt.Deref(arr.Elements[4]).(*advplrt.ArrayValue); ok && len(list.Elements) > 0 {
			if first, ok := advplrt.Deref(list.Elements[0]).(*advplrt.ArrayValue); ok && len(first.Elements) >= 5 {
				if _, ok := advplrt.Deref(first.Elements[4]).(*advplrt.ArrayValue); ok {
					apiSetMapReal(st, arr, list)
					return
				}
			}
		}
	}
	apiSetMapLegado(st, v, arr)
}

// apiSetMapReal faz o parse do aApiMap no formato do corpus 12.1.2510.
func apiSetMapReal(st *apiManagerState, top, list *advplrt.ArrayValue) {
	st.apiName = strings.TrimSpace(advplrt.ToString(top.Elements[1]))
	if st.apiName == "" {
		st.apiName = "items"
	}
	if len(top.Elements) >= 3 {
		st.versao = advplrt.ToString(top.Elements[2])
	}
	st.structs = nil
	for _, el := range list.Elements {
		sarr, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
		if !ok || len(sarr.Elements) < 5 {
			continue
		}
		stt := apiStruct{
			alias: strings.ToUpper(strings.TrimSpace(advplrt.ToString(sarr.Elements[0]))),
			typ:   strings.ToUpper(strings.TrimSpace(advplrt.ToString(sarr.Elements[1]))),
			name:  advplrt.ToString(sarr.Elements[2]),
			nick:  advplrt.ToString(sarr.Elements[3]),
		}
		if fields, ok := advplrt.Deref(sarr.Elements[4]).(*advplrt.ArrayValue); ok {
			for _, fe := range fields.Elements {
				farr, ok := advplrt.Deref(fe).(*advplrt.ArrayValue)
				if !ok || len(farr.Elements) < 2 {
					continue
				}
				stt.fields = append(stt.fields, apiStructField{
					api:   advplrt.ToString(farr.Elements[0]),
					campo: strings.TrimSpace(advplrt.ToString(farr.Elements[1])),
				})
			}
		}
		st.structs = append(st.structs, stt)
	}
	// Se SetApiAlias não foi chamado, o alias raiz vem do primeiro struct.
	if st.alias == "" && len(st.structs) > 0 {
		st.alias = st.structs[0].alias
	}
	// Mapa api->campo do struct raiz para tradução de filtros/ordem.
	st.rebuildMapa()
}

// apiSetMapLegado mantém o formato plano {prop, coluna} da fixture.
func apiSetMapLegado(st *apiManagerState, v *VM, arr *advplrt.ArrayValue) {
	if st.mapa == nil {
		st.mapa = map[string]MapaCampo{}
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
	st.legacyMap = true
	// Converte para struct único para reusar o mesmo pipeline de saída.
	st.structs = []apiStruct{{
		alias: st.alias, typ: "FIELD", name: "items", nick: "items",
	}}
	for prop, mc := range st.mapa {
		st.structs[0].fields = append(st.structs[0].fields, apiStructField{api: prop, campo: mc.Coluna})
	}
	if st.apiName == "" {
		st.apiName = "items"
	}
	st.rebuildMapa()
}

// rebuildMapa reconstrói st.mapa (prop api -> MapaCampo) a partir dos
// structs — usado por TraduzFiltro/TraduzOrdem/TraduzCampos.
func (st *apiManagerState) rebuildMapa() {
	st.mapa = map[string]MapaCampo{}
	for _, s := range st.structs {
		for _, f := range s.fields {
			campo := f.campo
			if strings.HasPrefix(campo, "Exp:") || strings.Contains(campo, ",") {
				continue // computado/composto não é coluna filtrável simples
			}
			st.mapa[f.api] = MapaCampo{Prop: f.api, Coluna: strings.ToUpper(campo), Filtra: true, Ordena: true}
		}
	}
}

// apiSetQstring implementa SetApiQstring real: PAGE/PAGESIZE/ORDER/FIELDS
// são especiais; qualquer outra chave "[nick.]campoApi" vira filtro de
// igualdade "COL = 'valor'" no struct correspondente (nick resolvido por
// GetProtField — aqui simplificado: nick = penúltimo segmento, campo =
// último). "filter" segue como extensão OData.
func apiSetQstring(st *apiManagerState, val advplrt.Value) {
	arr, ok := advplrt.Deref(val).(*advplrt.ArrayValue)
	if !ok {
		return
	}
	if st.filters == nil {
		st.filters = map[string][]string{}
	}
	for _, el := range arr.Elements {
		tup, ok := advplrt.Deref(el).(*advplrt.ArrayValue)
		if !ok || len(tup.Elements) < 2 {
			continue
		}
		k := strings.TrimSpace(advplrt.ToString(tup.Elements[0]))
		v := advplrt.ToString(tup.Elements[1])
		switch strings.ToUpper(k) {
		case "PAGE":
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
				st.page, st.pageSet = n, true
			}
		case "PAGESIZE":
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
				st.pageSize, st.sizeSet = n, true
			}
		case "ORDER":
			st.indexKey = v
		case "FIELDS":
			st.fieldFilter = map[string]bool{}
			for _, f := range strings.Split(v, ",") {
				if f = strings.ToUpper(strings.TrimSpace(f)); f != "" {
					st.fieldFilter[f] = true
				}
			}
		case "FILTER":
			// Extensão OData (não existe no FWAPIManager real).
			st.odataFilter = v
		default:
			apiAddFieldFilter(st, k, v)
		}
	}
}

// apiAddFieldFilter resolve "[nick.]campoApi" para o struct/dono e adiciona
// o fragmento "COL = 'valor'" à lista de filtros do nick. Campo "Exp:" ou
// não mapeado cai em expFilters (comparação pós-fetch).
func apiAddFieldFilter(st *apiManagerState, key, valor string) {
	parts := strings.Split(key, ".")
	nick, api := "", parts[len(parts)-1]
	if len(parts) >= 2 {
		nick = parts[len(parts)-2]
	}
	// nick default: o do struct raiz.
	root := apiRootStruct(st)
	if nick == "" && root != nil {
		nick = root.nick
	}
	stt := apiFindStruct(st, nick)
	if stt == nil {
		stt = root
	}
	if stt == nil {
		return
	}
	for _, f := range stt.fields {
		if !strings.EqualFold(f.api, api) {
			continue
		}
		campo := f.campo
		if strings.HasPrefix(campo, "Exp:") {
			if st.expFilters == nil {
				st.expFilters = map[string][]apiExpFilter{}
			}
			st.expFilters[nick] = append(st.expFilters[nick], apiExpFilter{api: f.api, valor: valor})
			return
		}
		cond := apiCampoCond(stt, campo, valor)
		if cond != "" {
			st.filters[stt.nick] = append(st.filters[stt.nick], cond)
		}
		return
	}
	// Campo desconhecido -> expFilter (comparação por nome de prop).
	if st.expFilters == nil {
		st.expFilters = map[string][]apiExpFilter{}
	}
	st.expFilters[stt.nick] = append(st.expFilters[stt.nick], apiExpFilter{api: api, valor: valor})
}

// apiCampoCond monta "COL = 'valor'"; campo composto "A,B" vira
// "A+B = 'valor'" (concatenação SQL, como o StrTran do FWAPIManager real).
func apiCampoCond(stt *apiStruct, campo, valor string) string {
	if strings.Contains(campo, ",") {
		parts := strings.Split(campo, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(parts, " + ") + " = '" + strings.ReplaceAll(valor, "'", "''") + "'"
	}
	return strings.ToUpper(campo) + " = '" + strings.ReplaceAll(valor, "'", "''") + "'"
}

func apiRootStruct(st *apiManagerState) *apiStruct {
	if len(st.structs) == 0 {
		return nil
	}
	return &st.structs[0]
}

func apiFindStruct(st *apiManagerState, nick string) *apiStruct {
	nick = strings.TrimSpace(nick)
	for i := range st.structs {
		s := &st.structs[i]
		if strings.EqualFold(s.nick, nick) || strings.EqualFold(s.name, nick) || strings.EqualFold(s.alias, nick) {
			return s
		}
	}
	return nil
}

// activateValidate espelha Activate() real: carrega/valida o mapa e liga
// lActive — sem executar consulta.
func (st *apiManagerState) activateValidate() error {
	st.hasErr = false
	st.errMsg = ""
	if len(st.structs) == 0 {
		st.hasErr = true
		st.errMsg = "FwApiManager: APIMAP não informado"
		return fmt.Errorf("%s", st.errMsg)
	}
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
	st.active = true
	return nil
}

// ensureExecuted roda execute() uma vez se Activate validou mas
// SetJsonObject não foi chamado (superset preguiçoso documentado).
func (st *apiManagerState) ensureExecuted(v *VM) {
	if !st.active || st.executed || st.hasErr {
		return
	}
	_ = st.execute(v)
}

// apiTiposSX3 consulta o tipo SX3 de um conjunto de colunas.
func apiTiposSX3(v *VM, st *apiManagerState) map[string]string {
	out := map[string]string{}
	eng, ok := v.dbEngine.(SQLEngine)
	if !ok {
		return out
	}
	for _, s := range st.structs {
		for _, f := range s.fields {
			if strings.HasPrefix(f.campo, "Exp:") || strings.Contains(f.campo, ",") {
				continue
			}
			col := strings.ToUpper(strings.TrimSpace(f.campo))
			if _, done := out[col]; done {
				continue
			}
			rows, err := eng.QueryRows(
				"SELECT X3_TIPO FROM SX3 WHERE UPPER(X3_CAMPO) = ? AND UPPER(X3_ARQUIVO) = ?",
				col, s.alias)
			if err == nil && len(rows) > 0 {
				out[col] = strings.TrimSpace(rows[0]["X3_TIPO"])
			}
		}
	}
	return out
}

// execute roda a consulta do struct raiz e monta o JSON aninhado.
func (st *apiManagerState) execute(v *VM) error {
	st.executed = true
	st.hasErr = false
	st.errMsg = ""
	eng, ok := v.dbEngine.(SQLEngine)
	if !ok {
		st.hasErr = true
		st.errMsg = "FwApiManager: engine de banco sem suporte a SQL"
		return fmt.Errorf("%s", st.errMsg)
	}
	tipos := apiTiposSX3(v, st)
	tiposGet := func(col string) string { return tipos[strings.ToUpper(col)] }

	root := apiRootStruct(st)
	if root == nil {
		st.hasErr = true
		st.errMsg = "FwApiManager: APIMAP sem structs"
		return fmt.Errorf("%s", st.errMsg)
	}

	page := st.page
	if !st.pageSet {
		page = 1
	}
	if page <= 0 {
		page = 1
	}
	pageSize := st.pageSize
	if !st.sizeSet || pageSize <= 0 {
		pageSize = 20
	}

	items, hasNext, err := st.queryStruct(v, eng, tiposGet, root, 0, page, pageSize)
	if err != nil {
		st.hasErr = true
		st.errMsg = err.Error()
		return err
	}
	st.items = advplrt.NewArray(items)
	jsonData := advplrt.NewObject("json", nil)
	if st.lUseHasNext {
		jsonData.SetProp("hasNext", advplrt.NewBool(hasNext))
	}
	// cApiName preserva o case do aApiMap[2] ("items" minúsculo no
	// corpus) — SetJson real usa o nome cru como chave do JSON.
	name := st.apiName
	if name == "" {
		name = "items"
	}
	jsonData.SetProp(name, st.items)
	st.jsonData = jsonData
	return nil
}

// queryStruct executa a consulta de UM nível do mapa e retorna os itens.
// Filhos são resolvidos por relação (SetApiRelation) recursivamente.
// ponytail: N+1 por linha do pai — correto e simples; se doer, o caminho
// é um JOIN único por relação como o MntQuery custom do corpus.
func (st *apiManagerState) queryStruct(v *VM, eng SQLEngine, tiposGet func(string) string,
	stt *apiStruct, depth, page, pageSize int) ([]advplrt.Value, bool, error) {

	// Colunas do SELECT: campos declarados (colunas reais) + campos-pai
	// das relações que penduram neste struct.
	colSet := map[string]bool{}
	var cols []string
	addCol := func(c string) {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || colSet[c] {
			return
		}
		colSet[c] = true
		cols = append(cols, c)
	}
	for _, f := range stt.fields {
		if st.fieldFilter != nil && len(st.fieldFilter) > 0 && !st.fieldFilter[strings.ToUpper(f.api)] {
			continue
		}
		for _, c := range apiCampoCols(f.campo) {
			addCol(c)
		}
	}
	for _, rel := range st.relations {
		if !strings.EqualFold(rel.fatherNick, stt.nick) && rel.fatherNick != "" {
			continue
		}
		for _, pr := range rel.pairs {
			addCol(pr[0]) // campo do pai precisa estar no SELECT
		}
	}
	if len(cols) == 0 {
		return nil, false, fmt.Errorf("FwApiManager: struct %q sem colunas", stt.nick)
	}

	alias := stt.alias
	if alias == "" {
		alias = st.alias
	}
	if !identRe.MatchString(alias) {
		return nil, false, fmt.Errorf("FwApiManager: alias inválido %q", alias)
	}

	var where []string
	var args []any
	if w := strings.TrimSpace(st.whereSQL); w != "" && depth == 0 {
		where = append(where, "("+w+")")
	}
	// Filtros por nick: aplicam os do próprio struct e os sem nick.
	for nick, conds := range st.filters {
		if nick != "" && !strings.EqualFold(nick, stt.nick) && !strings.EqualFold(nick, stt.name) {
			continue
		}
		for _, c := range conds {
			where = append(where, "("+c+")")
		}
	}
	// Filtro OData (extensão) só no nível raiz.
	if depth == 0 && strings.TrimSpace(st.odataFilter) != "" {
		fsql, fargs, err := TraduzFiltro(st.odataFilter, st.mapa)
		if err != nil {
			return nil, false, err
		}
		where = append(where, "("+fsql+")")
		args = append(args, fargs...)
	}
	// ExpFilters viram comparação pós-fetch; nada no WHERE.

	var q string
	if depth == 0 && strings.TrimSpace(st.queryTmpl) != "" {
		// Template legado da fixture: "#QueryFields#"/"#QueryWhere#".
		whereSQL := ""
		if len(where) > 0 {
			whereSQL = "WHERE " + strings.Join(where, " AND ")
		}
		q = strings.ReplaceAll(st.queryTmpl, "#QueryFields#",
			alias+"."+strings.Join(cols, ", "+alias+"."))
		q = strings.ReplaceAll(q, "#QueryWhere#", whereSQL)
	} else {
		q = "SELECT " + alias + "." + strings.Join(cols, ", "+alias+".") +
			" FROM " + alias
		if len(where) > 0 {
			q += " WHERE " + strings.Join(where, " AND ")
		}
	}
	ord := ""
	if depth == 0 {
		ord = st.orderSQL(stt, st.indexKey)
		if ord == "" && st.orderExpr != "" {
			if osql, err := TraduzOrdem(st.orderExpr, st.mapa); err == nil {
				ord = osql
			}
		}
	} else if rel := st.relationForFather(stt.nick); rel != nil {
		ord = strings.TrimSpace(rel.indexKey)
	}
	if ord != "" {
		q += " ORDER BY " + ord
	}
	// Paginação só no nível raiz (como no Protheus: nPageSize conta
	// registros do alias pai).
	if depth == 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, pageSize+1, (page-1)*pageSize)
	}

	rows, err := eng.QueryRows(q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("FwApiManager: %s", err.Error())
	}
	hasNext := false
	if depth == 0 {
		hasNext = len(rows) > pageSize
		if hasNext {
			rows = rows[:pageSize]
		}
	}

	out := make([]advplrt.Value, 0, len(rows))
	for _, row := range rows {
		// Filtros exp (pós-fetch) — comparação do valor da prop com o
		// filtro, como o AdvPL compara o conteúdo do campo na workarea.
		if !st.expFilterOK(stt, row, tiposGet) {
			continue
		}
		out = append(out, st.buildStructItem(eng, tiposGet, stt, row, depth+1))
	}
	return out, hasNext, nil
}

// buildStructItem converte uma linha SQL no objeto JSON do struct,
// resolvendo recursivamente os structs filhos pendurados por relação.
func (st *apiManagerState) buildStructItem(eng SQLEngine, tiposGet func(string) string,
	stt *apiStruct, row map[string]string, depth int) *advplrt.ObjectValue {

	item := advplrt.NewObject("json", nil)
	for _, f := range stt.fields {
		if len(st.fieldFilter) > 0 && !st.fieldFilter[strings.ToUpper(f.api)] {
			continue
		}
		if val := apiFieldValue(row, f, tiposGet); val != nil {
			item.SetProp(f.api, val)
		}
	}
	if depth > 8 {
		// Teto de recursão para cadeias de relações patológicas.
		return item
	}
	for i := range st.relations {
		rel := &st.relations[i]
		if rel.fatherNick != "" && !strings.EqualFold(rel.fatherNick, stt.nick) {
			continue
		}
		child := apiFindStruct(st, rel.childNick)
		if child == nil {
			continue
		}
		childRows := st.queryChildren(eng, tiposGet, child, rel, row)
		childItems := make([]advplrt.Value, 0, len(childRows))
		for _, cr := range childRows {
			if !st.expFilterOK(child, cr, tiposGet) {
				continue
			}
			childItems = append(childItems, st.buildStructItem(eng, tiposGet, child, cr, depth+1))
		}
		attachChild(item, child, childItems)
	}
	return item
}

// attachChild pendura os itens do struct filho na chave JSON do struct
// (ASTRUCT_NAME = elements[2]); tipo "ITEM" vira array, demais viram
// objeto (primeiro registro — semântica do JsonObject do Protheus).
func attachChild(parent *advplrt.ObjectValue, child *apiStruct, items []advplrt.Value) {
	key := child.name
	if key == "" {
		key = child.nick
	}
	if strings.EqualFold(child.typ, "ITEM") {
		parent.SetProp(key, advplrt.NewArray(items))
		return
	}
	obj := advplrt.NewObject("json", nil)
	if len(items) > 0 {
		if src, ok := items[len(items)-1].(*advplrt.ObjectValue); ok {
			for _, k := range src.Keys {
				obj.SetProp(k, src.Props[k])
			}
		}
	}
	parent.SetProp(key, obj)
}

// queryChildren busca as linhas do struct filho que satisfazem os pares
// {campoPai, campoFilho} da relação contra a linha do pai.
func (st *apiManagerState) queryChildren(eng SQLEngine, tiposGet func(string) string,
	child *apiStruct, rel *apiRelation, fatherRow map[string]string) []map[string]string {

	alias := child.alias
	if alias == "" {
		alias = st.alias
	}
	if !identRe.MatchString(alias) {
		return nil
	}
	colSet := map[string]bool{}
	var cols []string
	addCol := func(c string) {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || colSet[c] {
			return
		}
		colSet[c] = true
		cols = append(cols, c)
	}
	for _, f := range child.fields {
		for _, c := range apiCampoCols(f.campo) {
			addCol(c)
		}
	}
	for _, pr := range rel.pairs {
		addCol(pr[1]) // campo do filho no WHERE
	}
	if len(cols) == 0 {
		return nil
	}

	var where []string
	var args []any
	for _, pr := range rel.pairs {
		fv, ok := fatherRow[strings.ToUpper(strings.TrimSpace(pr[0]))]
		if !ok {
			continue
		}
		fc := strings.ToUpper(strings.TrimSpace(pr[1]))
		where = append(where, alias+"."+fc+" = ?")
		args = append(args, fv)
	}
	// Filtros por nick do filho.
	for nick, conds := range st.filters {
		if nick != "" && !strings.EqualFold(nick, child.nick) && !strings.EqualFold(nick, child.name) {
			continue
		}
		for _, c := range conds {
			where = append(where, "("+c+")")
		}
	}
	q := "SELECT " + alias + "." + strings.Join(cols, ", "+alias+".") + " FROM " + alias
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	if ord := strings.TrimSpace(rel.indexKey); ord != "" {
		q += " ORDER BY " + ord
	}
	rows, err := eng.QueryRows(q, args...)
	if err != nil {
		return nil
	}
	return rows
}

// relationForFather devolve a primeira relação cujo pai é o nick dado
// (usada para ORDER BY do filho — simplificação: uma relação por pai).
func (st *apiManagerState) relationForFather(nick string) *apiRelation {
	for i := range st.relations {
		rel := &st.relations[i]
		if rel.fatherNick != "" && strings.EqualFold(rel.fatherNick, nick) {
			return rel
		}
	}
	return nil
}

// orderSQL traduz st.indexKey (nomes de API ou colunas) para ORDER BY SQL.
func (st *apiManagerState) orderSQL(stt *apiStruct, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	var parts []string
	for _, tok := range strings.Split(key, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		desc := false
		up := strings.ToUpper(tok)
		if strings.HasSuffix(up, " DESC") {
			desc = true
			tok = strings.TrimSpace(tok[:len(tok)-5])
		} else if strings.HasSuffix(up, " ASC") {
			tok = strings.TrimSpace(tok[:len(tok)-4])
		}
		col := ""
		for _, f := range stt.fields {
			if strings.EqualFold(f.api, tok) {
				cols := apiCampoCols(f.campo)
				if len(cols) == 1 {
					col = cols[0]
				}
				break
			}
		}
		if col == "" {
			// pode já ser nome de coluna
			if identRe.MatchString(strings.ToUpper(tok)) {
				col = strings.ToUpper(tok)
			}
		}
		if col == "" {
			continue
		}
		if desc {
			parts = append(parts, col+" DESC")
		} else {
			parts = append(parts, col)
		}
	}
	return strings.Join(parts, ", ")
}

// apiCampoCols devolve as colunas reais de um campo do struct —
// "" para Exp:, as partes para "A, B", a coluna para campo simples.
func apiCampoCols(campo string) []string {
	campo = strings.TrimSpace(campo)
	if campo == "" || strings.HasPrefix(campo, "Exp:") {
		return nil
	}
	if strings.Contains(campo, ",") {
		var out []string
		for _, p := range strings.Split(campo, ",") {
			if p = strings.ToUpper(strings.TrimSpace(p)); identRe.MatchString(p) {
				out = append(out, p)
			}
		}
		return out
	}
	c := strings.ToUpper(campo)
	if !identRe.MatchString(c) {
		return nil
	}
	return []string{c}
}

// apiFieldValue extrai o valor de um campo do struct a partir da linha.
// Campo simples -> coluna; composto -> junta com ","; Exp: -> nil.
func apiFieldValue(row map[string]string, f apiStructField, tiposGet func(string) string) advplrt.Value {
	campo := strings.TrimSpace(f.campo)
	if campo == "" || strings.HasPrefix(campo, "Exp:") {
		return nil
	}
	if strings.Contains(campo, ",") {
		var parts []string
		for _, p := range strings.Split(campo, ",") {
			p = strings.ToUpper(strings.TrimSpace(p))
			raw, ok := row[p]
			if !ok {
				parts = append(parts, "")
				continue
			}
			parts = append(parts, strings.TrimSpace(fmt.Sprint(raw)))
		}
		return advplrt.NewString(strings.Join(parts, ","))
	}
	col := strings.ToUpper(campo)
	raw, ok := row[col]
	if !ok {
		return nil
	}
	return adapterJSONValue(tiposGet(col), raw)
}

// expFilterOK aplica os filtros pós-fetch do struct (campos Exp:/API
// sem coluna) comparando o valor projetado com o esperado.
func (st *apiManagerState) expFilterOK(stt *apiStruct, row map[string]string, tiposGet func(string) string) bool {
	filters := st.expFilters[stt.nick]
	if len(filters) == 0 {
		return true
	}
	for _, ef := range filters {
		for _, f := range stt.fields {
			if !strings.EqualFold(f.api, ef.api) {
				continue
			}
			val := apiFieldValue(row, f, tiposGet)
			got := ""
			if val != nil {
				got = strings.TrimSpace(advplrt.ToString(val))
			}
			if got != strings.TrimSpace(ef.valor) {
				return false
			}
		}
	}
	return true
}

// apiResponse monta {"items":[...],"hasNext":bool} — contrato PO-UI.
func apiResponse(items []advplrt.Value, hasNext bool) *advplrt.ObjectValue {
	obj := advplrt.NewObject("json", nil)
	if items == nil {
		items = []advplrt.Value{}
	}
	obj.SetProp("items", advplrt.NewArray(items))
	obj.SetProp("hasNext", advplrt.NewBool(hasNext))
	return obj
}

// apiSerialize serializa o objeto de resposta para JSON.
func apiSerialize(obj *advplrt.ObjectValue) string {
	b, err := json.Marshal(advplValueToJSON(obj))
	if err != nil {
		return ""
	}
	return string(b)
}

func apiSerializeObject(val advplrt.Value) string {
	b, err := json.Marshal(advplValueToJSON(val))
	if err != nil {
		return ""
	}
	return string(b)
}
