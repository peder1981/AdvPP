package vm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// oRestState é o estado de UMA requisição nas rotas anotadas (@Get/@Post...)
// quando a função usa o objeto oRest do TOTVS: a requisição de baixo nível,
// a resposta em construção e flags de o que já foi definido. Vive na
// VM-filha da requisição — requisições concorrentes não compartilham estado.
type oRestState struct {
	req       *rest.Request
	resp      *rest.Response
	bodySet   bool // setResponse/setFault já definiram o corpo
	statusSet bool // setStatusCode já definiu o status
}

// newORestObject cria o objeto oRest injetado como variável dinâmica
// (Private) na VM-filha da requisição. Métodos são despachados por
// callORestMethod via obj.Native — ver o hook em callNativeMethodOn.
func newORestObject(req *rest.Request) (*advplrt.ObjectValue, *oRestState) {
	st := &oRestState{
		req:  req,
		resp: &rest.Response{Header: map[string]string{}},
	}
	obj := advplrt.NewObject("oRest", nil)
	obj.Native = st
	return obj, st
}

// orestRawHandlerFor cria o Route.Raw de uma função anotada: monta os
// parâmetros no mesmo formato do dispatch legado (query < corpo JSON <
// path params, chaves em maiúsculas), injeta oRest no dynEnv da VM-filha
// e devolve a resposta — corpo definido por oRest:setResponse/setFault ou,
// na ausência deles, o valor de retorno da função (compatibilidade).
func (v *VM) orestRawHandlerFor(funcName string) func(*rest.Request) *rest.Response {
	return func(req *rest.Request) *rest.Response {
		params, errResp := orestParams(req)
		if errResp != nil {
			return errResp
		}
		job, done, err := v.newChildVM()
		if err != nil {
			return wsRestErrorResponse(500, err.Error())
		}
		defer done()

		oRestObj, st := newORestObject(req)
		// dynEnv é o ambiente de Private/Public: identificadores não
		// resolvidos na função caem em OP_LOAD_DYN. Um `Local oRest` ou
		// `Private oRest` no fonte sombreia este binding naturalmente
		// (OP_DECL_DYN registra restore; Local resolve antes do dyn).
		job.dynEnv["oRest"] = oRestObj
		job.dynEnv["OREST"] = oRestObj
		job.restReq = req
		defer func() { job.restReq = nil }()

		argObj := jsonMapToAdvplObject(params)
		result, err := job.RunFunction(funcName, []advplrt.Value{argObj})
		if err != nil {
			return wsRestErrorResponse(500, err.Error())
		}
		return st.buildResponse(result)
	}
}

// orestControllerHandlerFor cria o Route.Raw de um MÉTODO de classe
// anotado (@GET em `public method` dentro de `class ... endclass`) — o
// padrão de controlador REST do TOTVS. Por requisição: VM-filha isolada,
// oRest injetado, instância nova da classe (com construtor `new` quando
// existe) e invocação do método. A resposta segue o mesmo contrato de
// orestRawHandlerFor: setResponse/setFault ou valor de retorno.
func (v *VM) orestControllerHandlerFor(cls *compiler.ClassInfo, minfo *compiler.FunctionInfo) func(*rest.Request) *rest.Response {
	return func(req *rest.Request) *rest.Response {
		params, errResp := orestParams(req)
		if errResp != nil {
			return errResp
		}
		job, done, err := v.newChildVM()
		if err != nil {
			return wsRestErrorResponse(500, err.Error())
		}
		defer done()

		oRestObj, st := newORestObject(req)
		job.dynEnv["oRest"] = oRestObj
		job.dynEnv["OREST"] = oRestObj
		job.restReq = req
		defer func() { job.restReq = nil }()

		obj := advplrt.NewObject(cls.Name, job.classes[cls.Name])
		if ctor := findClassMethod(cls, "new"); ctor != nil {
			if err := job.invokeMethodFrame(obj, ctor.Name, ctor, nil, 0); err != nil {
				return wsRestErrorResponse(500, err.Error())
			}
			if _, err := job.runLoop(); err != nil {
				return wsRestErrorResponse(500, err.Error())
			}
		}

		// Método com parâmetros recebe o objeto de params (mesmo formato
		// da função anotada); método sem parâmetros é invocado direto.
		var args []advplrt.Value
		if minfo.NumParams > 1 {
			args = []advplrt.Value{jsonMapToAdvplObject(params)}
		}
		if err := job.invokeMethodFrame(obj, minfo.Name, minfo, args, len(args)); err != nil {
			return wsRestErrorResponse(500, err.Error())
		}
		result, err := job.runLoop()
		if err != nil {
			return wsRestErrorResponse(500, err.Error())
		}
		return st.buildResponse(result)
	}
}

// findClassMethod localiza um método da classe por nome, case-insensitive
// (o mapa Methods usa o nome como declarado — "new"/"New" variam).
func findClassMethod(cls *compiler.ClassInfo, name string) *compiler.FunctionInfo {
	upper := strings.ToUpper(name)
	for mname, minfo := range cls.Methods {
		if strings.ToUpper(mname) == upper {
			return minfo
		}
	}
	return nil
}

// orestParams replica o merge de parâmetros do dispatch legado de rotas
// anotadas (pkg/rest dispatch): query string primeiro, corpo JSON por cima,
// path params por último — todos com chaves em maiúsculas. Corpo JSON
// inválido devolve a mesma resposta 400 do caminho Handler.
func orestParams(req *rest.Request) (map[string]any, *rest.Response) {
	params := map[string]any{}
	for k, vals := range req.Query {
		if len(vals) > 0 {
			params[strings.ToUpper(k)] = vals[0]
		}
	}
	if len(strings.TrimSpace(string(req.Body))) > 0 {
		var bodyMap map[string]any
		if err := json.Unmarshal(req.Body, &bodyMap); err != nil {
			return nil, orestErrorResponse(400, "corpo JSON inválido: "+err.Error())
		}
		for k, val := range bodyMap {
			params[strings.ToUpper(k)] = val
		}
	}
	for k, val := range req.PathParams {
		params[strings.ToUpper(k)] = val
	}
	return params, nil
}

// orestErrorResponse monta o corpo {"error": msg} das falhas de validação
// do dispatch Raw (mesmo formato do writeJSON do pkg/rest).
func orestErrorResponse(status int, msg string) *rest.Response {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return &rest.Response{
		Status:      status,
		ContentType: "application/json",
		Body:        append(body, '\n'),
	}
}

// buildResponse monta a resposta final: se a função usou oRest:setResponse
// ou oRest:setFault, vale o que foi definido; senão o valor de retorno vira
// o corpo (JSON, ou RawResponse do sentinela __RAW_HTTP__) — o mesmo
// comportamento das rotas anotadas antes do oRest.
func (st *oRestState) buildResponse(result advplrt.Value) *rest.Response {
	if !st.bodySet {
		if raw, ok := advplRawHTTPResponse(result); ok {
			return &rest.Response{Status: raw.Status, ContentType: raw.ContentType, Body: raw.Body}
		}
		body, _ := json.Marshal(advplValueToJSON(result))
		return &rest.Response{
			Status:      200,
			ContentType: "application/json",
			Body:        append(body, '\n'),
		}
	}
	resp := st.resp
	if resp.Status == 0 {
		resp.Status = 200
	}
	if resp.ContentType == "" {
		resp.ContentType = "application/json"
	}
	return resp
}

// orestBodyBytes serializa o argumento de setResponse/setFault: objetos e
// arrays viram JSON; demais valores vão como texto puro (ToString).
func orestBodyBytes(val advplrt.Value) []byte {
	switch val.(type) {
	case *advplrt.ObjectValue, *advplrt.ArrayValue:
		if b, err := json.Marshal(advplValueToJSON(val)); err == nil {
			return b
		}
	}
	return []byte(advplrt.ToString(val))
}

// ---------------------------------------------------------------------------
// Métodos nativos do objeto oRest (obj.Native = *oRestState)
// ---------------------------------------------------------------------------

// callORestMethod despacha os métodos do oRest do TOTVS (nomes
// case-insensitive). Retorna erro para método desconhecido — vira 500 no
// dispatch, como os demais erros de runtime.
func (v *VM) callORestMethod(obj *advplrt.ObjectValue, st *oRestState, method string, args []advplrt.Value) error {
	switch strings.ToUpper(method) {
	case "GETQUERYREQUEST":
		m := make(map[string]any, len(st.req.Query))
		for k, vals := range st.req.Query {
			if len(vals) > 0 {
				m[strings.ToUpper(k)] = vals[0]
			}
		}
		v.push(jsonMapToAdvplObject(m))
	case "GETPATHPARAMSREQUEST":
		m := make(map[string]any, len(st.req.PathParams))
		for k, val := range st.req.PathParams {
			m[strings.ToUpper(k)] = val
		}
		v.push(jsonMapToAdvplObject(m))
	case "GETBODYREQUEST":
		v.push(advplrt.NewString(string(st.req.Body)))
	case "GETHEADERREQUEST":
		m := make(map[string]any, len(st.req.Header))
		for k, vals := range st.req.Header {
			if len(vals) > 0 {
				m[strings.ToUpper(k)] = vals[0]
			}
		}
		v.push(jsonMapToAdvplObject(m))
	case "SETRESPONSE":
		st.resp.Body = orestBodyBytes(getArg(args, 0))
		st.bodySet = true
		v.push(advplrt.Nil)
	case "SETSTATUSCODE":
		st.resp.Status = int(advplrt.ToFloat(getArg(args, 0)))
		st.statusSet = true
		v.push(advplrt.Nil)
	case "SETKEYHEADERRESPONSE", "UPDATEKEYHEADERRESPONSE":
		if st.resp.Header == nil {
			st.resp.Header = map[string]string{}
		}
		st.resp.Header[advplrt.ToString(getArg(args, 0))] = advplrt.ToString(getArg(args, 1))
		v.push(advplrt.Nil)
	case "SETFAULT":
		st.resp.Body = orestBodyBytes(getArg(args, 0))
		if !st.statusSet {
			st.resp.Status = 400
		}
		st.bodySet = true
		v.push(advplrt.Nil)
	default:
		return fmt.Errorf("oRest: método desconhecido %q", method)
	}
	return nil
}
