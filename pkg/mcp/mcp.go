// Package mcp implementa um servidor MCP (Model Context Protocol) mínimo
// sobre stdio: JSON-RPC 2.0, uma mensagem por linha (sem framing estilo
// LSP). Stdlib puro, sem dependências externas — mesmo padrão do resto do
// AdvPP (CGO_ENABLED=0, idêntico nas 3 plataformas).
//
// Cobre o essencial pra expor "tools": initialize, notifications/initialized,
// tools/list, tools/call, ping. Não implementa resources/prompts/sampling
// (ver limitações no CHANGELOG) — são extensões futuras sobre o mesmo
// despachante caso um caso de uso concreto peça.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
)

const jsonrpcVersion = "2.0"

// rpcRequest é a forma de uma mensagem de entrada — pode ser request
// (com Id) ou notification (sem Id).
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool é uma ferramenta exposta pelo servidor MCP — tipicamente uma
// WSMETHOD de um bloco WSMCP no AdvPL/TLPP.
type Tool struct {
	Name        string
	Description string
	// InputSchema é o JSON Schema dos parâmetros (ex.: {"type":"object",
	// "properties":{...},"required":[...]}). Se nil, vira um schema vazio
	// (aceita qualquer objeto).
	InputSchema map[string]any
	// Handler recebe os argumentos já decodificados e devolve o texto de
	// resultado (content type "text") ou um erro (vira isError:true).
	Handler func(args map[string]any) (string, error)
}

// Server é um servidor MCP servindo um conjunto fixo de tools sobre stdio.
type Server struct {
	Name    string
	Version string

	mu    sync.Mutex
	tools map[string]Tool
}

// NewServer cria um servidor MCP vazio.
func NewServer(name, version string) *Server {
	return &Server{Name: name, Version: version, tools: map[string]Tool{}}
}

// AddTool registra (ou substitui) uma tool pelo nome.
func (s *Server) AddTool(t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Name] = t
}

// Serve lê mensagens JSON-RPC (uma por linha) de r e escreve as respostas
// em w, até r fechar ou ocorrer um erro de I/O. Bloqueia a chamador.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var writeMu sync.Mutex

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeMu.Lock()
			writeResponse(w, rpcResponse{JSONRPC: jsonrpcVersion, Error: &rpcError{Code: -32700, Message: "parse error: " + err.Error()}})
			writeMu.Unlock()
			continue
		}

		isNotification := len(req.ID) == 0
		result, rpcErr := s.dispatch(req.Method, req.Params)
		if isNotification {
			continue // notifications não têm resposta (ex.: notifications/initialized)
		}

		resp := rpcResponse{JSONRPC: jsonrpcVersion, ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		writeMu.Lock()
		err := writeResponse(w, resp)
		writeMu.Unlock()
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\r' || b[j-1] == '\n') {
		j--
	}
	return b[i:j]
}

func writeResponse(w io.Writer, resp rpcResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.handleInitialize(params)
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return s.handleToolsList()
	case "tools/call":
		return s.handleToolsCall(params)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

func (s *Server) handleInitialize(params json.RawMessage) (any, *rpcError) {
	var in struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &in) // params ausente/malformado: segue com defaults
	protocolVersion := in.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = "2024-11-05"
	}
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    s.Name,
			"version": s.Version,
		},
	}, nil
}

func (s *Server) handleToolsList() (any, *rpcError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	list := make([]map[string]any, 0, len(names))
	for _, name := range names {
		t := s.tools[name]
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		list = append(list, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	return map[string]any{"tools": list}, nil
}

func (s *Server) handleToolsCall(params json.RawMessage) (any, *rpcError) {
	var in struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	s.mu.Lock()
	tool, ok := s.tools[in.Name]
	s.mu.Unlock()
	if !ok {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("tool desconhecida: %q", in.Name)}
	}

	text, err := tool.Handler(in.Arguments)
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	}, nil
}
