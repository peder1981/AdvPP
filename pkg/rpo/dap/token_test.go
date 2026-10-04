package dap

import "testing"

func TestGetTokenFluxoCompleto(t *testing.T) {
	var methods []string
	var authParams rpcMsg
	handler := func(req rpcMsg) []rpcMsg {
		m, _ := req["method"].(string)
		methods = append(methods, m)
		switch m {
		case "initialize":
			return []rpcMsg{lspResp(req, rpcMsg{})}
		case "$totvsserver/connect":
			return []rpcMsg{lspResp(req, rpcMsg{"connectionToken": "tok-in"})}
		case "$totvsserver/authentication":
			authParams = req["params"].(rpcMsg)
			return []rpcMsg{lspResp(req, rpcMsg{"connectionToken": "tok-out"})}
		case "$totvsserver/serverInformations":
			return []rpcMsg{lspResp(req, rpcMsg{
				"serverInformations": rpcMsg{
					"server": rpcMsg{"environmentDetectedType": float64(1)},
				},
			})}
		}
		return nil
	}
	rpc := newStreamRPC(t, rpcLSP, handler)

	res, err := getTokenFromRPC(rpc, LSConfig{
		Server: "172.17.0.3", Port: 1234, Build: "7.00.210324P",
		Env: "P12", User: "u1", Pass: "dummy-pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	// o notify "initialized" também atravessa o harness do teste
	want := []string{"initialize", "initialized", "$totvsserver/connect",
		"$totvsserver/authentication", "$totvsserver/serverInformations"}
	if len(methods) != 5 {
		t.Fatalf("methods=%v", methods)
	}
	for i, w := range want {
		if methods[i] != w {
			t.Fatalf("methods[%d]=%s, esperado %s", i, methods[i], w)
		}
	}
	ai := authParams["authenticationInfo"].(rpcMsg)
	if ai["password"] != "dummy-pass" || ai["user"] != "u1" ||
		ai["connectionToken"] != "tok-in" || ai["encoding"] != "CP1252" {
		t.Fatalf("authenticationInfo errado: %v", ai)
	}
	if res.Token != "tok-out" || res.EnvType != "totvs_server_protheus" {
		t.Fatalf("res=%+v", res)
	}
}

func TestGetTokenEnvTypeFallback(t *testing.T) {
	handler := func(req rpcMsg) []rpcMsg {
		m, _ := req["method"].(string)
		switch m {
		case "initialize":
			return []rpcMsg{lspResp(req, rpcMsg{})}
		case "$totvsserver/connect", "$totvsserver/authentication":
			return []rpcMsg{lspResp(req, rpcMsg{"connectionToken": "t"})}
		default:
			return []rpcMsg{lspResp(req, rpcMsg{
				"serverInformations": rpcMsg{
					"server": rpcMsg{"environmentDetectedType": float64(9)},
				},
			})}
		}
	}
	res, err := getTokenFromRPC(newStreamRPC(t, rpcLSP, handler), LSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if res.EnvType != "totvs_server_totvstec" {
		t.Fatalf("EnvType=%s", res.EnvType)
	}
}
