package vm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/advpl/compiler/pkg/rest"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// restState é o estado Go da classe nativa WSRestServer: o servidor HTTP
// real (pkg/rest) e a VM à qual as rotas despacham (via v.RunFunction, o
// mesmo mecanismo que MCPServer/StartJob usam para chamar uma User
// Function pelo nome numa VM isolada).
type restState struct {
	server *rest.Server
}

func newWSRestServerObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("WSRestServer", nil)
	obj.Native = &restState{}
	return obj
}

// restVerbAnnotations mapeia o nome da anotação AdvPL/TLPP (@Get, @Post...)
// para o verbo HTTP correspondente.
var restVerbAnnotations = map[string]string{
	"GET":    "GET",
	"POST":   "POST",
	"PUT":    "PUT",
	"PATCH":  "PATCH",
	"DELETE": "DELETE",
}

// callWSRestServerMethod implementa a classe nativa WSRestServer: expõe
// funções AdvPL/TLPP anotadas com @Get/@Post/@Put/@Patch/@Delete (ou
// registradas manualmente via AddRoute) como rotas de um servidor REST
// real, rodando sobre net/http — sem CGO, sem dependências externas
// (pkg/rest).
//
// Cobre o estilo moderno de REST 2.0 do TLPP (anotações sobre `User
// Function`), que carrega verbo+path completos até o bytecode
// (FunctionInfo.Annotations). O DSL clássico `WSRESTFUL ... WSMETHOD GET
// path("...") ... ENDWSRESTFUL` é reconhecido pelo parser (parseWSClient)
// mas hoje descarta o path e o verbo ao virar ast.ClassDecl — ver
// limitação documentada em COMPONENT_STATUS.md. Para esse estilo, use
// AddRoute() para registrar a rota manualmente.
func (v *VM) callWSRestServerMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*restState)
	if !ok {
		return fmt.Errorf("WSRestServer: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		name := advplrt.ToString(getArg(args, 0))
		version := "1.0.0"
		if len(args) > 1 {
			version = advplrt.ToString(args[1])
		}
		st.server = rest.NewServer(name, version)
		v.autoRegisterAnnotatedRoutes(st.server)
		v.push(obj)

	case "ADDROUTE":
		if st.server == nil {
			return fmt.Errorf("WSRestServer:AddRoute: chame New() primeiro")
		}
		httpMethod := strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		path := advplrt.ToString(getArg(args, 1))
		funcName := advplrt.ToString(getArg(args, 2))
		st.server.AddRoute(rest.Route{
			Method:  httpMethod,
			Path:    normalizeRestPath(path),
			Handler: v.restHandlerFor(funcName),
		})
		v.push(advplrt.Nil)

	case "SERVE":
		if st.server == nil {
			return fmt.Errorf("WSRestServer:Serve: chame New() primeiro")
		}
		addr := ":8080"
		if len(args) > 0 {
			switch a := args[0].(type) {
			case *advplrt.NumberValue:
				addr = fmt.Sprintf(":%d", int(a.Val))
			case *advplrt.StringValue:
				if _, err := strconv.Atoi(a.Val); err == nil {
					addr = ":" + a.Val
				} else {
					addr = a.Val
				}
			}
		}
		err := st.server.Serve(addr)
		if err != nil {
			return fmt.Errorf("WSRestServer:Serve: %w", err)
		}
		v.push(advplrt.Nil)

	case "SHUTDOWN":
		if st.server == nil {
			v.push(advplrt.Nil)
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := st.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("WSRestServer:Shutdown: %w", err)
		}
		v.push(advplrt.Nil)

	default:
		return fmt.Errorf("WSRestServer: método desconhecido %q", method)
	}
	return nil
}

// autoRegisterAnnotatedRoutes varre as funções do bytecode carregado
// procurando anotações @Get/@Post/@Put/@Patch/@Delete("/path") — o estilo
// REST 2.0 moderno do TLPP — e registra cada uma como rota. Anotações sem
// path explícito viram "/" + nome da função em minúsculas.
func (v *VM) autoRegisterAnnotatedRoutes(server *rest.Server) {
	for fname, info := range v.bc.Functions {
		for _, ann := range info.Annotations {
			httpMethod, ok := restVerbAnnotations[strings.ToUpper(ann.Name)]
			if !ok {
				continue
			}
			path := ann.Value
			if strings.TrimSpace(path) == "" {
				path = "/" + strings.ToLower(strings.TrimPrefix(strings.ToUpper(fname), "U_"))
			}
			server.AddRoute(rest.Route{
				Method:  httpMethod,
				Path:    normalizeRestPath(path),
				Handler: v.restHandlerFor(fname),
			})
		}
	}
}

// normalizeRestPath garante que o path começa com "/" — o roteador nativo
// do Go (net/http, 1.22+) exige isso nos patterns.
func normalizeRestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// restHandlerFor cria o Handler Go (pkg/rest) que despacha uma requisição
// HTTP para a função AdvPL funcName, rodando numa VM isolada — mesmo
// motivo do MCPServer: v.RunFunction diretamente NÃO é seguro aqui,
// reentraria no v.frames/v.current compartilhado da VM que está bloqueada
// dentro de Serve(), corrompendo a pilha de chamadas em andamento.
func (v *VM) restHandlerFor(funcName string) func(map[string]any) (any, error) {
	return func(params map[string]any) (any, error) {
		job := NewVM(v.bc, false)
		job.dbFactory = v.dbFactory
		if v.dbFactory != nil {
			job.dbEngine = v.dbFactory()
		}
		argObj := jsonMapToAdvplObject(params)
		result, err := job.RunFunction(funcName, []advplrt.Value{argObj})
		if err != nil {
			return nil, err
		}
		return advplValueToJSON(result), nil
	}
}

// advplValueToJSON converte um advplrt.Value (retorno de uma função
// AdvPL/TLPP) numa árvore de tipos Go nativos (map/slice/string/float64/
// bool/nil) pronta para encoding/json — contraparte de jsonValueToAdvpl
// (mcp_native.go), que faz o caminho inverso.
func advplValueToJSON(val advplrt.Value) any {
	switch x := val.(type) {
	case nil, *advplrt.NilValue:
		return nil
	case *advplrt.NumberValue:
		return x.Val
	case *advplrt.StringValue:
		return x.Val
	case *advplrt.BoolValue:
		return x.Val
	case *advplrt.DateValue:
		return x.Val.Format("2006-01-02")
	case *advplrt.ArrayValue:
		out := make([]any, len(x.Elements))
		for i, e := range x.Elements {
			out[i] = advplValueToJSON(e)
		}
		return out
	case *advplrt.ObjectValue:
		out := make(map[string]any, len(x.Props))
		for _, k := range x.Keys {
			out[k] = advplValueToJSON(x.Props[k])
		}
		return out
	default:
		return advplrt.ToString(val)
	}
}
