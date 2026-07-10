package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMCPServerFixture builda o advplc e roda tests/mcp_test.prw como
// servidor MCP real sobre stdio, exercitando initialize/tools.list/
// tools.call via mensagens JSON-RPC cruas — a mesma pilha que o SDK oficial
// do MCP (validado manualmente em Python) usaria, só sem a dependência
// externa do SDK para rodar em CI.
func TestMCPServerFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	binPath := filepath.Join(t.TempDir(), "advplc")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath, "run", "tests/mcp_test.prw")
	cmd.Dir = repoRoot
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer cmd.Process.Kill()

	reader := bufio.NewReader(stdout)
	readResponse := func() map[string]any {
		t.Helper()
		done := make(chan string, 1)
		go func() {
			line, _ := reader.ReadString('\n')
			done <- line
		}()
		select {
		case line := <-done:
			var m map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
				t.Fatalf("resposta inválida %q: %v", line, err)
			}
			return m
		case <-time.After(15 * time.Second):
			t.Fatal("timeout esperando resposta do servidor MCP")
			return nil
		}
	}

	send := func(msg string) {
		if _, err := stdin.Write([]byte(msg + "\n")); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	initResp := readResponse()
	result := initResp["result"].(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]any)
	if info["name"] != "advpp-demo" {
		t.Errorf("serverInfo.name = %v, want advpp-demo", info["name"])
	}

	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listResp := readResponse()
	tools := listResp["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools/list = %d tools, want 2: %v", len(tools), tools)
	}

	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"soma","arguments":{"a":10,"b":32}}}`)
	callResp := readResponse()
	callResult := callResp["result"].(map[string]any)
	content := callResult["content"].([]any)[0].(map[string]any)
	if content["text"] != "42" {
		t.Errorf("tools/call soma = %v, want 42", content["text"])
	}
	if callResult["isError"] != false {
		t.Errorf("isError = %v, want false", callResult["isError"])
	}

	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"saudacao","arguments":{"nome":"Claude"}}}`)
	callResp2 := readResponse()
	content2 := callResp2["result"].(map[string]any)["content"].([]any)[0].(map[string]any)
	if content2["text"] != "Ola, Claude!" {
		t.Errorf("tools/call saudacao = %v, want %q", content2["text"], "Ola, Claude!")
	}
}
