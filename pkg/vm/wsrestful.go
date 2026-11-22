package vm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// wsRestReq é o estado de UMA requisição WSRESTFUL clássica: o Request de
// baixo nível (pkg/rest), a resposta em construção e o fault opcional
// gravado por SetRestFault. Vive na VM-filha da requisição — nunca no pai
// — então requisições concorrentes não compartilham estado.
type wsRestReq struct {
	http  *rest.Request
	resp  *rest.Response
	fault *wsRestFault
}

type wsRestFault struct {
	code   int // código devolvido no corpo JSON (o nCode original)
	status int // status HTTP efetivo (nCode fora de 100–599 vira 400)
	msg    string
}

// ---------------------------------------------------------------------------
// Registro de rotas (WSRESTFUL ... WSMETHOD ... ENDWSRESTFUL)
// ---------------------------------------------------------------------------

// RegistrarWSRestful varre as classes WSRESTFUL do bytecode e registra uma
// rota REST por WSMETHOD: verbo+PATH da anotação ("<VERBO>|<Nome>|<PATH>").
// Sem PATH, a rota é "/<prefixo>/<serviço>" mais a variante curinga
// "/<prefixo>/<serviço>/{resto...}" que alimenta aURLParms.
// Devolve os erros de registro (método sem implementação, conflito de
// rotas) sem abortar os demais.
func (v *VM) RegistrarWSRestful(s *rest.Server, prefixo string) []error {
	var errs []error
	seen := make(map[*compiler.ClassInfo]struct{})
	for _, cls := range v.bc.Classes {
		if _, ok := seen[cls]; ok {
			continue // mesma classe sob chave simples e qualificada
		}
		seen[cls] = struct{}{}
		if !wsHasAnnotation(cls, "WSRESTFUL") {
			continue
		}
		service := wsServiceName(cls.Name)
		for _, ann := range cls.Annotations {
			if !strings.EqualFold(ann.Name, "WSMETHOD") {
				continue
			}
			verb, methodName, path, ok := wsParseMethodAnnotation(ann.Value)
			if !ok {
				errs = append(errs, fmt.Errorf("WSMETHOD malformado em %s: %q", cls.Name, ann.Value))
				continue
			}
			minfo := wsFindMethod(cls, methodName)
			if minfo == nil {
				errs = append(errs, fmt.Errorf("WSMETHOD %s::%s sem implementação", cls.Name, methodName))
				continue
			}
			for _, rp := range wsRoutesFor(prefixo, service, path) {
				norm := rest.NormalizaPath(rp)
				route := rest.Route{
					Method: verb,
					Path:   norm,
					Raw:    v.wsRestfulRawHandler(cls, minfo, norm),
				}
				if err := s.AddRouteE(route); err != nil {
					errs = append(errs, fmt.Errorf("%s %s (%s::%s): %w", verb, norm, cls.Name, methodName, err))
				}
			}
		}
	}
	return errs
}

func wsHasAnnotation(cls *compiler.ClassInfo, name string) bool {
	for _, ann := range cls.Annotations {
		if strings.EqualFold(ann.Name, name) {
			return true
		}
	}
	return false
}

// wsParseMethodAnnotation separa "<VERBO>|<Nome>|<PATH>". O nome pode ser
// o próprio verbo quando o WSMETHOD não tem nome próprio.
func wsParseMethodAnnotation(value string) (verb, methodName, path string, ok bool) {
	parts := strings.SplitN(value, "|", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	verb = strings.ToUpper(strings.TrimSpace(parts[0]))
	if verb == "" {
		return "", "", "", false
	}
	return verb, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), true
}

// wsFindMethod resolve o método WSMETHOD na classe, sem diferenciar caixa.
func wsFindMethod(cls *compiler.ClassInfo, name string) *compiler.FunctionInfo {
	if m, ok := cls.Methods[name]; ok {
		return m
	}
	upper := strings.ToUpper(name)
	for k, m := range cls.Methods {
		if strings.ToUpper(k) == upper {
			return m
		}
	}
	return nil
}

// wsServiceName devolve o nome simples da classe (último segmento após
// ponto) — usado no path default das rotas sem PATH.
func wsServiceName(className string) string {
	if i := strings.LastIndex(className, "."); i >= 0 {
		return className[i+1:]
	}
	return className
}

// wsRoutesFor monta os patterns de rota de um WSMETHOD. Com PATH usa só o
// PATH; sem PATH registra a rota exata do serviço e a curinga que joga os
// segmentos restantes em aURLParms.
func wsRoutesFor(prefixo, service, path string) []string {
	base := strings.TrimSuffix(prefixo, "/")
	if strings.TrimSpace(path) == "" {
		p := base + "/" + strings.ToLower(service)
		return []string{p, p + "/{resto...}"}
	}
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "/") {
		return []string{base + path}
	}
	return []string{base + "/" + path}
}

// ---------------------------------------------------------------------------
// Dispatch por requisição
// ---------------------------------------------------------------------------

func (v *VM) wsRestfulRawHandler(cls *compiler.ClassInfo, minfo *compiler.FunctionInfo, routePath string) func(*rest.Request) *rest.Response {
	return func(req *rest.Request) *rest.Response {
		return v.dispatchWSRestful(cls, minfo, routePath, req)
	}
}

// dispatchWSRestful executa UM WSMETHOD do WSRESTFUL clássico: cria uma VM
// filha isolada, instancia a classe de serviço com as propriedades WSDATA
// ligadas à query string, injeta aURLParms/aQueryString e despacha
// "<Classe>::<Metodo>". O estado da requisição fica em job.wsRestReq para
// os métodos nativos (SetResponse/SetRestFault/...) e para as funções
// globais (SetRestFault/HTTPSetStatus/GetContent).
func (v *VM) dispatchWSRestful(cls *compiler.ClassInfo, minfo *compiler.FunctionInfo, routePath string, req *rest.Request) *rest.Response {
	job, done, err := v.newChildVM()
	if err != nil {
		return wsRestErrorResponse(500, err.Error())
	}
	defer done()

	state := &wsRestReq{
		http: req,
		resp: &rest.Response{Header: map[string]string{}},
	}
	obj := job.newWSRestfulInstance(cls, state, req, routePath)

	job.wsRestReq = state
	defer func() { job.wsRestReq = nil }()

	result, err := job.invokeWSRestfulMethod(obj, minfo)
	if err != nil {
		return wsRestErrorResponse(500, err.Error())
	}
	if state.fault != nil {
		return state.faultResponse()
	}
	// `.F.` sem SetRestFault é falha do método — 500 com corpo JSON padrão.
	if b, ok := result.(*advplrt.BoolValue); ok && !b.Val {
		return wsRestErrorResponse(500, fmt.Sprintf("método %s retornou .F.", minfo.Name))
	}
	return state.resp
}

// invokeWSRestfulMethod monta o frame do método sobre o objeto de serviço
// (sem argumentos — self é o único parâmetro) e roda até o retorno.
func (v *VM) invokeWSRestfulMethod(obj *advplrt.ObjectValue, info *compiler.FunctionInfo) (advplrt.Value, error) {
	if err := v.invokeMethodFrame(obj, info.Name, info, nil, 0); err != nil {
		return advplrt.Nil, err
	}
	return v.runLoop()
}

// newWSRestfulInstance cria o objeto de serviço da requisição: uma
// instância nova por request (sem estado compartilhado entre chamadas),
// com cada WSDATA ligada à query string (case-insensitive, primeira
// ocorrência vence) e os atributos de protocolo aURLParms/aQueryString.
func (v *VM) newWSRestfulInstance(cls *compiler.ClassInfo, state *wsRestReq, req *rest.Request, routePath string) *advplrt.ObjectValue {
	obj := advplrt.NewObject(cls.Name, v.classes[cls.Name])
	queryVal, queryHas := wsQueryIndex(req)
	for name, typ := range cls.Properties {
		upper := strings.ToUpper(name)
		obj.SetProp(upper, wsDataValue(typ, queryVal[upper], queryHas[upper]))
	}
	obj.SetProp("AURLPARMS", wsURLParms(req, routePath))
	obj.SetProp("AQUERYSTRING", wsQueryString(req))
	obj.Native = state
	return obj
}

// wsQueryIndex indexa os pares da query string por chave em maiúsculas,
// preservando a PRIMEIRA ocorrência de cada chave (a ordem dos pares está
// em req.QueryPairs). Devolve valor e presença separados para distinguir
// "ausente" de "presente com valor vazio".
func wsQueryIndex(req *rest.Request) (map[string]string, map[string]bool) {
	vals := make(map[string]string, len(req.QueryPairs))
	has := make(map[string]bool, len(req.QueryPairs))
	for _, p := range req.QueryPairs {
		k := strings.ToUpper(p.Key)
		if !has[k] {
			has[k] = true
			vals[k] = p.Value
		}
	}
	return vals, has
}

// wsDataValue converte o valor textual da query para o tipo declarado no
// WSDATA. Numéricos inválidos e booleanos irreconhecíveis viram Nil; os
// demais tipos (CHARACTER, STRING, etc.) ficam como texto.
func wsDataValue(typeName, raw string, ok bool) advplrt.Value {
	if !ok {
		return advplrt.Nil
	}
	t := strings.ToUpper(strings.TrimSpace(typeName))
	switch {
	case strings.Contains(t, "INT"),
		strings.Contains(t, "NUM"),
		strings.Contains(t, "FLOAT"),
		strings.Contains(t, "DECIMAL"),
		strings.Contains(t, "DOUBLE"),
		strings.Contains(t, "REAL"):
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return advplrt.Nil
		}
		return advplrt.NewNumber(f)
	case strings.Contains(t, "BOOL"), strings.Contains(t, "LOGICAL"):
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true", "t", "1", "yes", "y", "sim", "s":
			return advplrt.True
		case "false", "f", "0", "no", "n", "nao":
			return advplrt.False
		default:
			return advplrt.Nil
		}
	default:
		return advplrt.NewString(raw)
	}
}

// wsPathParams extrai os parâmetros `{nome}` do pattern da rota, na ordem,
// marcando os curingas `{nome...}` que capturam o restante do path.
func wsPathParams(path string) []struct {
	name     string
	wildcard bool
} {
	var out []struct {
		name     string
		wildcard bool
	}
	for {
		i := strings.IndexByte(path, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(path[i:], '}')
		if j < 0 {
			break
		}
		raw := path[i+1 : i+j]
		out = append(out, struct {
			name     string
			wildcard bool
		}{
			name:     strings.TrimSuffix(raw, "..."),
			wildcard: strings.HasSuffix(raw, "..."),
		})
		path = path[i+j+1:]
	}
	return out
}

// wsURLParms monta aURLParms: array de strings com os path params na ordem
// do pattern. O curinga `{resto...}` vira um elemento por segmento.
func wsURLParms(req *rest.Request, routePath string) *advplrt.ArrayValue {
	elems := make([]advplrt.Value, 0, 4)
	for _, p := range wsPathParams(routePath) {
		val := req.PathParams[p.name]
		if p.wildcard {
			for _, seg := range strings.Split(val, "/") {
				if seg != "" {
					elems = append(elems, advplrt.NewString(seg))
				}
			}
			continue
		}
		if val != "" {
			elems = append(elems, advplrt.NewString(val))
		}
	}
	return advplrt.NewArray(elems)
}

// wsQueryString monta aQueryString: array de arrays {chave, valor} na
// ordem exata em que os pares aparecem na URL (chaves repetidas repetem).
func wsQueryString(req *rest.Request) *advplrt.ArrayValue {
	elems := make([]advplrt.Value, 0, len(req.QueryPairs))
	for _, p := range req.QueryPairs {
		elems = append(elems, advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(p.Key),
			advplrt.NewString(p.Value),
		}))
	}
	return advplrt.NewArray(elems)
}

// ---------------------------------------------------------------------------
// Métodos nativos do objeto de serviço (SetResponse, SetHeader, ...)
// ---------------------------------------------------------------------------

// callWSRestReqMethod despacha os métodos nativos do objeto WSRESTFUL
// (obj.Native = *wsRestReq). Métodos de mesmo nome em bytecode têm
// precedência — este dispatch só roda quando a classe não os define.
func (v *VM) callWSRestReqMethod(obj *advplrt.ObjectValue, req *wsRestReq, method string, args []advplrt.Value) error {
	switch strings.ToUpper(method) {
	case "SETRESPONSE":
		req.appendResponse(getArg(args, 0))
		v.push(advplrt.Nil)
	case "SETCONTENTTYPE":
		req.resp.ContentType = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
	case "SETHEADER":
		req.resp.Header[advplrt.ToString(getArg(args, 0))] = advplrt.ToString(getArg(args, 1))
		v.push(advplrt.Nil)
	case "GETHEADER":
		v.push(advplrt.NewString(req.http.Header.Get(advplrt.ToString(getArg(args, 0)))))
	case "SETSTATUS":
		req.resp.Status = int(advplrt.ToFloat(getArg(args, 0)))
		v.push(advplrt.Nil)
	case "GETCONTENT":
		v.push(advplrt.NewString(string(req.http.Body)))
	default:
		return fmt.Errorf("WSRESTFUL: método %s não suportado em %s", method, obj.ClassName)
	}
	return nil
}

// appendResponse acumula o corpo da resposta. Strings vão cruas; objetos e
// arrays viram JSON (mesma conversão das rotas anotadas).
func (r *wsRestReq) appendResponse(val advplrt.Value) {
	var s string
	switch val.(type) {
	case *advplrt.ObjectValue, *advplrt.ArrayValue:
		if b, err := json.Marshal(advplValueToJSON(val)); err == nil {
			s = string(b)
		} else {
			s = advplrt.ToString(val)
		}
	default:
		s = advplrt.ToString(val)
	}
	r.resp.Body = append(r.resp.Body, []byte(s)...)
}

// setFault grava o fault da requisição: o corpo JSON usa o nCode original
// e o status HTTP usa nCode quando válido (100–599), senão 400.
func (r *wsRestReq) setFault(code int, msg string) {
	status := code
	if status < 100 || status > 599 {
		status = 400
	}
	r.fault = &wsRestFault{code: code, status: status, msg: msg}
}

// faultResponse monta a resposta do fault, substituindo qualquer corpo
// acumulado por SetResponse.
func (r *wsRestReq) faultResponse() *rest.Response {
	body, _ := json.Marshal(map[string]any{"code": r.fault.code, "message": r.fault.msg})
	return &rest.Response{
		Status:      r.fault.status,
		ContentType: "application/json",
		Body:        body,
	}
}

// wsRestErrorResponse monta o corpo JSON de erro padrão das falhas de
// dispatch (erro de runtime, .F. sem fault).
func wsRestErrorResponse(status int, msg string) *rest.Response {
	body, _ := json.Marshal(map[string]any{"code": status, "message": msg})
	return &rest.Response{
		Status:      status,
		ContentType: "application/json",
		Body:        body,
	}
}

// ---------------------------------------------------------------------------
// Funções globais ligadas à requisição corrente
// ---------------------------------------------------------------------------

// registerWSRestfulNatives registra as funções globais do WSRESTFUL
// clássico. Todas operam sobre v.wsRestReq — o estado da requisição em
// curso na VM-filha — e falham fora de um dispatch REST.
func (v *VM) registerWSRestfulNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	natives["SETRESTFAULT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		req := v.wsRestReq
		if req == nil {
			return advplrt.Nil, fmt.Errorf("SetRestFault fora de requisição WSRESTFUL")
		}
		req.setFault(int(advplrt.ToFloat(getArg(args, 0))), advplrt.ToString(getArg(args, 1)))
		return advplrt.Nil, nil
	}
	natives["HTTPSETSTATUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		req := v.wsRestReq
		if req == nil {
			return advplrt.Nil, fmt.Errorf("HTTPSetStatus fora de requisição WSRESTFUL")
		}
		req.resp.Status = int(advplrt.ToFloat(getArg(args, 0)))
		return advplrt.Nil, nil
	}
	natives["GETCONTENT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		req := v.wsRestReq
		if req == nil {
			return advplrt.Nil, fmt.Errorf("GetContent fora de requisição WSRESTFUL")
		}
		return advplrt.NewString(string(req.http.Body)), nil
	}
}
