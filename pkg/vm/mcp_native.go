package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/advpl/compiler/pkg/mcp"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// mcpState é o estado Go da classe MCPServer: o servidor MCP (pkg/mcp) e a
// VM à qual as tools despacham (via v.RunFunction, a mesma engrenagem que
// StartJob usa para chamar uma User Function pelo nome).
type mcpState struct {
	server *mcp.Server
}

func newMCPServerObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("MCPServer", nil)
	obj.Native = &mcpState{}
	return obj
}

// callMCPServerMethod implementa a classe nativa MCPServer: expõe funções
// AdvPL/TLPP como "tools" de um servidor MCP (Model Context Protocol) real,
// rodando sobre stdio — sem CGO, sem dependências externas (pkg/mcp).
func (v *VM) callMCPServerMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*mcpState)
	if !ok {
		return fmt.Errorf("MCPServer: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		name := advplrt.ToString(getArg(args, 0))
		version := "1.0.0"
		if len(args) > 1 {
			version = advplrt.ToString(args[1])
		}
		st.server = mcp.NewServer(name, version)
		v.push(obj)

	case "ADDTOOL":
		if st.server == nil {
			return fmt.Errorf("MCPServer:AddTool: chame New() primeiro")
		}
		toolName := advplrt.ToString(getArg(args, 0))
		description := advplrt.ToString(getArg(args, 1))
		schemaJSON := advplrt.ToString(getArg(args, 2))
		funcName := advplrt.ToString(getArg(args, 3))

		var schema map[string]any
		if strings.TrimSpace(schemaJSON) != "" {
			if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
				return fmt.Errorf("MCPServer:AddTool: schema JSON inválido: %w", err)
			}
		}

		st.server.AddTool(mcp.Tool{
			Name:        toolName,
			Description: description,
			InputSchema: schema,
			Handler: func(toolArgs map[string]any) (string, error) {
				// Roda em uma VM isolada, própria da chamada — mesmo
				// mecanismo do StartJob. v.RunFunction diretamente NÃO é
				// seguro aqui: reentraria no v.frames/v.current compartilhado
				// da VM que está bloqueada dentro de Serve(), corrompendo a
				// pilha de chamadas em andamento.
				job := NewVM(v.bc, false)
				job.dbFactory = v.dbFactory
				if v.dbFactory != nil {
					job.dbEngine = v.dbFactory()
				}
				argObj := jsonMapToAdvplObject(toolArgs)
				result, err := job.RunFunction(funcName, []advplrt.Value{argObj})
				if err != nil {
					return "", err
				}
				return advplrt.ToString(result), nil
			},
		})
		v.push(advplrt.Nil)

	case "SERVE":
		if st.server == nil {
			return fmt.Errorf("MCPServer:Serve: chame New() primeiro")
		}
		// ConOut/etc continuam indo para o processo, mas não podem se
		// misturar com as mensagens JSON-RPC no stdout — redireciona o
		// console para stderr enquanto o servidor MCP estiver no ar.
		v.SetOutputWriter(os.Stderr)
		err := st.server.Serve(os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("MCPServer:Serve: %w", err)
		}
		v.push(advplrt.Nil)

	default:
		return fmt.Errorf("MCPServer: método desconhecido %q", method)
	}
	return nil
}

// jsonMapToAdvplObject converte um map[string]any (decodificado de JSON
// pelos argumentos de uma tool call) em um ObjectValue com Props em
// maiúsculas — mesma convenção de acesso a propriedade (obj:campo) usada
// pelo resto da VM. Dá pro handler AdvPL acessar oArgs:NOMEDOCAMPO.
func jsonMapToAdvplObject(m map[string]any) *advplrt.ObjectValue {
	obj := advplrt.NewObject("JsonObject", nil)
	for k, val := range m {
		obj.Props[strings.ToUpper(k)] = jsonValueToAdvpl(val)
	}
	return obj
}

func jsonValueToAdvpl(val any) advplrt.Value {
	switch x := val.(type) {
	case nil:
		return advplrt.Nil
	case bool:
		return advplrt.NewBool(x)
	case float64:
		return advplrt.NewNumber(x)
	case string:
		return advplrt.NewString(x)
	case []any:
		elems := make([]advplrt.Value, len(x))
		for i, e := range x {
			elems[i] = jsonValueToAdvpl(e)
		}
		return advplrt.NewArray(elems)
	case map[string]any:
		return jsonMapToAdvplObject(x)
	default:
		return advplrt.Nil
	}
}
