package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func runLines(t *testing.T, s *Server, lines ...string) []map[string]any {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	if err := s.Serve(in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("resposta inválida %q: %v", line, err)
		}
		responses = append(responses, m)
	}
	return responses
}

func TestInitializeEchoesProtocolVersion(t *testing.T) {
	s := NewServer("test-server", "0.1.0")
	resp := runLines(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	if len(resp) != 1 {
		t.Fatalf("esperava 1 resposta, veio %d", len(resp))
	}
	result, ok := resp[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("resposta sem result: %v", resp[0])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok || info["name"] != "test-server" {
		t.Errorf("serverInfo incorreto: %v", result["serverInfo"])
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	s := NewServer("test-server", "0.1.0")
	resp := runLines(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	)
	if len(resp) != 1 {
		t.Fatalf("notification não deveria gerar resposta; got %d respostas", len(resp))
	}
}

func TestToolsListAndCall(t *testing.T) {
	s := NewServer("test-server", "0.1.0")
	s.AddTool(Tool{
		Name:        "soma",
		Description: "Soma dois números",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
		Handler: func(args map[string]any) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return fmt.Sprintf("%g", a+b), nil
		},
	})

	resp := runLines(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"soma","arguments":{"a":2,"b":3}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"naoexiste","arguments":{}}}`,
	)
	if len(resp) != 3 {
		t.Fatalf("esperava 3 respostas, veio %d: %v", len(resp), resp)
	}

	listResult := resp[0]["result"].(map[string]any)
	tools := listResult["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "soma" {
		t.Errorf("tools/list incorreto: %v", tools)
	}

	callResult := resp[1]["result"].(map[string]any)
	if callResult["isError"] != false {
		t.Errorf("tools/call soma deveria ter isError=false: %v", callResult)
	}
	content := callResult["content"].([]any)[0].(map[string]any)
	if content["text"] != "5" {
		t.Errorf("tools/call soma = %v, want 5", content["text"])
	}

	if resp[2]["error"] == nil {
		t.Errorf("tools/call de tool inexistente deveria retornar erro JSON-RPC: %v", resp[2])
	}
}

func TestToolHandlerErrorBecomesIsError(t *testing.T) {
	s := NewServer("test-server", "0.1.0")
	s.AddTool(Tool{
		Name: "falha",
		Handler: func(args map[string]any) (string, error) {
			return "", errTest("deu ruim")
		},
	})
	resp := runLines(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"falha","arguments":{}}}`)
	result := resp[0]["result"].(map[string]any)
	if result["isError"] != true {
		t.Errorf("esperava isError=true, veio %v", result)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
