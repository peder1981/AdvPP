package dap

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// LSConfig configura a autenticação via advpls (language server).
type LSConfig struct {
	Path    string // binário advpls-linux
	Server  string
	Port    int
	Build   string
	Env     string
	User    string
	Pass    string
	ErrFile string // stderr do LS (log)
}

// TokenResult mantém o LS vivo para a sessão DAP.
type TokenResult struct {
	Token   string
	EnvType string
	rpc     *RPC
}

// Close encerra o processo do LS.
func (t *TokenResult) Close() error {
	if t != nil && t.rpc != nil {
		return t.rpc.Close()
	}
	return nil
}

// GetToken spawn o advpls e executa o fluxo de 3 requisições de token
// (dap_lib.py:94-116). A senha só viaja em authenticationInfo — nunca é
// logada ou embutida em erro (Lei 6).
func GetToken(cfg LSConfig) (*TokenResult, error) {
	ef, err := os.Create(cfg.ErrFile)
	if err != nil {
		return nil, fmt.Errorf("criando log do LS: %w", err)
	}
	defer ef.Close()
	cmd := exec.Command(cfg.Path, "language-server", "--log-all-to-stderr")
	cmd.Stderr = ef
	rpc, err := newRPCProcess(cmd, rpcLSP)
	if err != nil {
		return nil, err
	}
	res, err := getTokenFromRPC(rpc, cfg)
	if err != nil {
		_ = rpc.Close()
		return nil, err
	}
	return res, nil
}

// getTokenFromRPC executa o fluxo num RPC já conectado (separado para teste).
func getTokenFromRPC(rpc *RPC, cfg LSConfig) (*TokenResult, error) {
	if _, err := rpc.Request("initialize", rpcMsg{
		"processId": os.Getpid(), "rootUri": "file:///tmp",
		"capabilities": rpcMsg{}, "initializationOptions": rpcMsg{},
	}, 60*time.Second); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	rpc.Notify("initialized", rpcMsg{})

	r, err := rpc.Request("$totvsserver/connect", rpcMsg{
		"connectionInfo": rpcMsg{
			"connType": 3, "serverName": "local2310", "identification": "local2310",
			"serverType": 1, "server": cfg.Server, "port": cfg.Port,
			"build": cfg.Build, "bSecure": 0, "environment": cfg.Env,
			"autoReconnect": true,
		},
	}, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	token := digString(dig(r, "result"), "connectionToken")
	if token == "" {
		return nil, fmt.Errorf("connect sem connectionToken")
	}

	r, err = rpc.Request("$totvsserver/authentication", rpcMsg{
		"authenticationInfo": rpcMsg{
			"connectionToken": token, "environment": cfg.Env,
			"user": cfg.User, "password": cfg.Pass, "encoding": "CP1252",
		},
	}, 60*time.Second)
	if err != nil {
		// erro do próprio transporte; não ecoa params (Lei 6)
		return nil, fmt.Errorf("authentication: %w", err)
	}
	if errObj := dig(r, "error"); errObj != nil {
		return nil, fmt.Errorf("authentication recusada pelo servidor")
	}
	if tok := digString(dig(r, "result"), "connectionToken"); tok != "" {
		token = tok
	}

	r, err = rpc.Request("$totvsserver/serverInformations", rpcMsg{
		"serverInformationsInfo": rpcMsg{"connectionToken": token},
	}, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("serverInformations: %w", err)
	}
	etn := digNum(dig(r, "result", "serverInformations", "server"), "environmentDetectedType", 1)
	var envType string
	switch int(etn) {
	case 1:
		envType = "totvs_server_protheus"
	case 2:
		envType = "totvs_server_logix"
	default:
		envType = "totvs_server_totvstec"
	}
	return &TokenResult{Token: token, EnvType: envType, rpc: rpc}, nil
}

// dig navega um caminho de chaves aninhadas.
func dig(m rpcMsg, path ...string) rpcMsg {
	cur := m
	for _, k := range path {
		v, ok := cur[k]
		if !ok {
			return nil
		}
		cur, ok = v.(rpcMsg)
		if !ok {
			return nil
		}
	}
	return cur
}

func digString(m rpcMsg, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func digNum(m rpcMsg, key string, def float64) float64 {
	if m == nil {
		return def
	}
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}
