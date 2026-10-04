package dap

import (
	"context"
	"testing"
	"time"
)

func sessionTestConfig() SessionConfig {
	return SessionConfig{
		DAPath: "fake-da", LogFile: "/tmp/fake-da.log",
		Server: "172.17.0.3", Port: 1234, Build: "7.00.210324P",
		Env: "P12", EnvType: "totvs_server_protheus", Token: "tok",
		Program: "U_TESTWAIT", Workspace: "/ws", SrcPath: "/ws/U_TESTWAIT.prw",
		SmartclientBin: "/bin/true", Display: ":99",
		ClickX: 10, ClickY: 20,
		PollEvery: 10 * time.Millisecond, StopTimeout: 2 * time.Second,
	}
}

// newTestSession monta um Session sobre RPC roteirizado.
func newTestSession(t *testing.T, cfg SessionConfig,
	handler func(req rpcMsg) []rpcMsg, clicks *int) *Session {
	t.Helper()
	rpc := newStreamRPC(t, rpcDAP, handler)
	s := &Session{cfg: cfg, rpc: rpc}
	s.clickFn = func() { *clicks++ }
	return s
}

func okHandler(capture *rpcMsg) func(req rpcMsg) []rpcMsg {
	return func(req rpcMsg) []rpcMsg {
		cmd, _ := req["command"].(string)
		if capture != nil && cmd == "launch" {
			*capture = req
		}
		return []rpcMsg{dapResp(req, nil)}
	}
}

func TestSessionLaunchParametros(t *testing.T) {
	var launchReq rpcMsg
	clicks := 0
	s := newTestSession(t, sessionTestConfig(), okHandler(&launchReq), &clicks)
	if _, err := newSessionWithRPC(sessionTestConfig(), s.rpc, s.clickFn); err != nil {
		t.Fatal(err)
	}
	if err := s.Launch(context.Background()); err != nil {
		t.Fatal(err)
	}
	args := launchReq["arguments"].(rpcMsg)
	wants := map[string]any{
		"program": "U_TESTWAIT", "token": "tok", "server": "172.17.0.3",
		"port": float64(1234), "build": "7.00.210324P", "environment": "P12",
		"environmentType": "totvs_server_protheus", "smartclientBin": "/bin/true",
		"cwb": "/tmp", "secure": float64(0),
	}
	for k, w := range wants {
		if args[k] != w {
			t.Errorf("launch.%s=%v, esperado %v", k, args[k], w)
		}
	}
	wf := args["workspaceFolders"].([]any)
	if len(wf) != 1 || wf[0] != "/ws" {
		t.Errorf("workspaceFolders=%v", wf)
	}
}

func TestSessionWaitStoppedRetornaFrame(t *testing.T) {
	clicks := 0
	handler := func(req rpcMsg) []rpcMsg {
		switch req["command"] {
		case "configurationDone":
			return []rpcMsg{
				{"seq": 9, "type": "event", "event": "stopped"},
				dapResp(req, nil),
			}
		case "threads":
			return []rpcMsg{dapResp(req, map[string]any{
				"threads": []any{map[string]any{"id": float64(1), "name": "main"}},
			})}
		case "stackTrace":
			return []rpcMsg{dapResp(req, map[string]any{
				"stackFrames": []any{map[string]any{
					"id": float64(7), "name": "TESTWAIT", "line": float64(5),
				}},
			})}
		default:
			return []rpcMsg{dapResp(req, nil)}
		}
	}
	s := newTestSession(t, sessionTestConfig(), handler, &clicks)
	if _, err := newSessionWithRPC(sessionTestConfig(), s.rpc, s.clickFn); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigurationDone(); err != nil {
		t.Fatal(err)
	}
	fid, err := s.WaitStopped(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fid != 7 {
		t.Fatalf("fid=%d, esperado 7", fid)
	}
	if clicks == 0 {
		t.Fatal("esperava pelo menos um clique (dismiss de diálogo)")
	}
}

func TestSessionEvaluateSucessoEErro(t *testing.T) {
	clicks := 0
	handler := func(req rpcMsg) []rpcMsg {
		args, _ := req["arguments"].(rpcMsg)
		expr := args["expression"]
		if expr == "quebrada" {
			return []rpcMsg{{
				"seq": 2, "type": "response",
				"request_seq": req["seq"], "success": false,
				"command": "evaluate", "message": "erro de runtime",
			}}
		}
		return []rpcMsg{dapResp(req, map[string]any{"result": "\"cG9rZQ==\""})}
	}
	s := newTestSession(t, sessionTestConfig(), handler, &clicks)
	if _, err := newSessionWithRPC(sessionTestConfig(), s.rpc, s.clickFn); err != nil {
		t.Fatal(err)
	}
	got, err := s.Evaluate("Encode64(GetApoRes('x'))", 0, time.Second)
	if err != nil || got != "\"cG9rZQ==\"" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := s.Evaluate("quebrada", 0, time.Second); err == nil {
		t.Fatal("esperava erro de evaluate recusado")
	}
}
