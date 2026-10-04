package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

// newStreamRPC cria um RPC sobre pipes, com um handler que recebe cada
// requisição lida e devolve os frames a escrever de volta (nil = nada).
func newStreamRPC(t *testing.T, mode rpcMode, handler func(req map[string]any) []map[string]any) *RPC {
	t.Helper()
	respR, respW := io.Pipe() // RPC lê; teste escreve respostas
	reqR, reqW := io.Pipe()   // RPC escreve; teste lê requisições
	t.Cleanup(func() { respW.Close(); reqW.Close() })

	go func() {
		br := bufio.NewReader(reqR)
		for {
			headers := map[string]string{}
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				line = trimCRLF(line)
				if line == "" {
					break
				}
				if k, v, ok := cut(line, ":"); ok {
					headers[lower(k)] = trimSpace(v)
				}
			}
			n := atoi(headers["content-length"])
			if n <= 0 {
				return
			}
			body := make([]byte, n)
			if _, err := io.ReadFull(br, body); err != nil {
				return
			}
			var req map[string]any
			if err := json.Unmarshal(body, &req); err != nil {
				continue
			}
			for _, out := range handler(req) {
				data, _ := json.Marshal(out)
				if _, err := fmt.Fprintf(respW, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
					return
				}
				if _, err := respW.Write(data); err != nil {
					return
				}
			}
		}
	}()

	return newRPC(respR, reqW, mode, nil)
}

// helpers de string (evitar dependência de strings num teste de framing)
func trimCRLF(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
func cut(s, sep string) (a, b string, ok bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}
func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func dapResp(req map[string]any, body map[string]any) map[string]any {
	return map[string]any{
		"seq": 1, "type": "response",
		"request_seq": req["seq"], "success": true,
		"command": req["command"], "body": body,
	}
}

func lspResp(req map[string]any, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": result}
}

func TestRPC_DAPRequestResponse(t *testing.T) {
	rpc := newStreamRPC(t, rpcDAP, func(req map[string]any) []map[string]any {
		if req["command"] != "evaluate" {
			return nil
		}
		return []map[string]any{dapResp(req, map[string]any{"result": "\"aGk=\""})}
	})
	resp, err := rpc.Request("evaluate", map[string]any{"expression": "1+1"}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	body := resp["body"].(map[string]any)
	if body["result"] != "\"aGk=\"" {
		t.Fatalf("result=%v", body["result"])
	}
}

func TestRPC_DAPNotificacaoViaDrain(t *testing.T) {
	rpc := newStreamRPC(t, rpcDAP, func(req map[string]any) []map[string]any {
		if req["command"] == "configurationDone" {
			return []map[string]any{
				{"seq": 9, "type": "event", "event": "stopped"},
				dapResp(req, nil),
			}
		}
		return []map[string]any{dapResp(req, nil)}
	})
	if _, err := rpc.Request("configurationDone", nil, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	notifs := rpc.Drain(50*time.Millisecond, 10)
	found := false
	for _, n := range notifs {
		if n["event"] == "stopped" {
			found = true
		}
	}
	if !found {
		t.Fatalf("evento stopped não encontrado em %v", notifs)
	}
}

func TestRPC_LSPRequestResponse(t *testing.T) {
	rpc := newStreamRPC(t, rpcLSP, func(req map[string]any) []map[string]any {
		return []map[string]any{lspResp(req, map[string]any{"connectionToken": "tok1"})}
	})
	resp, err := rpc.Request("$totvsserver/connect", map[string]any{"a": 1}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp["result"].(map[string]any)["connectionToken"] != "tok1" {
		t.Fatalf("result=%v", resp["result"])
	}
}

func TestRPC_LSPAutoRespondeRequisicaoDoServidor(t *testing.T) {
	auto := make(chan map[string]any, 1)
	rpc := newStreamRPC(t, rpcLSP, func(req map[string]any) []map[string]any {
		if _, hasMethod := req["method"]; hasMethod {
			// responde o initialize e, no mesmo lote, manda uma requisição
			// do servidor (server→client) para o RPC auto-responder
			return []map[string]any{
				lspResp(req, nil),
				{"jsonrpc": "2.0", "id": 55, "method": "ping"},
			}
		}
		// nossa auto-resposta chegou de volta ao handler
		select {
		case auto <- req:
		default:
		}
		return nil
	})
	if _, err := rpc.Request("initialize", nil, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-auto:
		if m["id"] != float64(55) || m["result"] != nil {
			t.Fatalf("auto-resposta errada: %v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("auto-resposta não chegou ao handler")
	}
}

func TestRPC_Timeout(t *testing.T) {
	rpc := newStreamRPC(t, rpcDAP, func(req map[string]any) []map[string]any {
		return nil // nunca responde
	})
	if _, err := rpc.Request("slow", nil, 100*time.Millisecond); err == nil {
		t.Fatal("esperava timeout")
	}
}
