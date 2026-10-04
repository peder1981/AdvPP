package dap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// SessionConfig configura a sessão de depuração (debugAdapter + smartclient).
type SessionConfig struct {
	DAPath        string // debugAdapter-linux
	LogFile       string // --log-file do DA
	ErrFile       string // stderr do DA
	Server        string
	Port          int
	Build         string
	Env           string
	EnvType       string // totvs_server_protheus | ..._logix | ..._totvstec
	Token         string
	Program       string // ex.: "U_TESTWAIT"
	Workspace     string // dir com o .prw do Program
	SrcPath       string // Workspace + "/" + Program + ".prw"
	SmartclientBin string
	Display       string // ex.: ":99" (Xvfb)
	ClickX        int    // clique para dismiss de diálogo (0 = sem clique)
	ClickY        int
	PollEvery     time.Duration // default 3s
	StopTimeout   time.Duration // default 120s
}

// Session é uma sessão DAP viva (DA spawnado).
type Session struct {
	cfg     SessionConfig
	rpc     *RPC
	clickFn func()
}

// NewSession spawn o debugAdapter e envia initialize.
func NewSession(cfg SessionConfig) (*Session, error) {
	if cfg.PollEvery == 0 {
		cfg.PollEvery = 3 * time.Second
	}
	if cfg.StopTimeout == 0 {
		cfg.StopTimeout = 120 * time.Second
	}
	ef, err := os.Create(cfg.ErrFile)
	if err != nil {
		return nil, fmt.Errorf("criando log do DA: %w", err)
	}
	cmd := exec.Command(cfg.DAPath, "--log-all-to-stderr", "--log-file="+cfg.LogFile)
	cmd.Stderr = ef
	rpc, err := newRPCProcess(cmd, rpcDAP)
	ef.Close()
	if err != nil {
		return nil, err
	}
	s := &Session{cfg: cfg, rpc: rpc}
	s.clickFn = s.execClick
	if err := s.initialize(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// newSessionWithRPC monta sessão sobre RPC já criado (testes).
func newSessionWithRPC(cfg SessionConfig, rpc *RPC, clickFn func()) (*Session, error) {
	if cfg.PollEvery == 0 {
		cfg.PollEvery = 3 * time.Second
	}
	if cfg.StopTimeout == 0 {
		cfg.StopTimeout = 120 * time.Second
	}
	s := &Session{cfg: cfg, rpc: rpc}
	s.clickFn = clickFn
	if err := s.initialize(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) initialize() error {
	if _, err := s.rpc.Request("initialize", rpcMsg{
		"clientID": "advpp", "clientName": "AdvPP",
		"adapterID": "totvs_language_debug",
		"linesStartAt1": true, "columnsStartAt1": true,
		"pathFormat": "path", "supportsLoadedSourcesRequest": true,
	}, 60*time.Second); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	return nil
}

// Launch inicia o debug com o smartclient (dap_proxy39.py:117-122).
func (s *Session) Launch(ctx context.Context) error {
	resp, err := s.rpc.Request("launch", rpcMsg{
		"type": "totvs_language_debug", "name": "AdvPP", "request": "launch",
		"program": s.cfg.Program, "programArguments": []any{},
		"environment": s.cfg.Env, "environmentType": s.cfg.EnvType,
		"token": s.cfg.Token, "server": s.cfg.Server, "port": s.cfg.Port,
		"build": s.cfg.Build, "secure": 0,
		"workspaceFolders": []string{s.cfg.Workspace}, "cwb": "/tmp",
		"smartclientBin": s.cfg.SmartclientBin,
		"enableMultiThread": false, "isMultiSession": false,
	}, 90*time.Second)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	if ok, _ := resp["success"].(bool); !ok {
		return fmt.Errorf("launch recusado: %v", resp["message"])
	}
	s.rpc.Drain(2*time.Second, 10)
	return nil
}

// SetBreakpoints marca todas as linhas do fonte do programa (a DA ignora
// linhas não-executáveis — padrão das ondas 20-43).
func (s *Session) SetBreakpoints() error {
	src, err := os.ReadFile(s.cfg.SrcPath)
	if err != nil {
		return fmt.Errorf("lendo fonte para breakpoints: %w", err)
	}
	n := 0
	for _, b := range src {
		if b == '\n' {
			n++
		}
	}
	if n == 0 {
		n = 1
	}
	lines := make([]any, 0, n)
	bps := make([]any, 0, n)
	for i := 1; i <= n; i++ {
		lines = append(lines, i)
		bps = append(bps, rpcMsg{"line": i})
	}
	resp, err := s.rpc.Request("setBreakpoints", rpcMsg{
		"source": rpcMsg{
			"name": filepath.Base(s.cfg.SrcPath), "path": s.cfg.SrcPath,
		},
		"lines": lines, "breakpoints": bps,
	}, 30*time.Second)
	if err != nil {
		return fmt.Errorf("setBreakpoints: %w", err)
	}
	if ok, _ := resp["success"].(bool); !ok {
		return fmt.Errorf("setBreakpoints recusado: %v", resp["message"])
	}
	return nil
}

// ConfigurationDone libera o start do debug.
func (s *Session) ConfigurationDone() error {
	if _, err := s.rpc.Request("configurationDone", nil, 20*time.Second); err != nil {
		return fmt.Errorf("configurationDone: %w", err)
	}
	return nil
}

// WaitStopped aguarda o evento stopped (com cliques de dismiss a cada
// poll) e devolve o id do frame topo da pilha.
func (s *Session) WaitStopped(ctx context.Context) (int, error) {
	deadline := time.Now().Add(s.cfg.StopTimeout)
	stopped := false
	for !stopped && time.Now().Before(deadline) {
		for _, e := range s.rpc.Drain(s.cfg.PollEvery, 10) {
			if e["event"] == "stopped" {
				stopped = true
			}
		}
		s.clickFn()
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}
	if !stopped {
		return 0, fmt.Errorf("sessão não parou em %s", s.cfg.StopTimeout)
	}
	// threads → stackTrace (com retries, como as ondas)
	for attempt := 0; attempt < 4; attempt++ {
		fid, err := s.topFrame()
		if err == nil {
			return fid, nil
		}
		s.rpc.Drain(2*time.Second, 10)
		s.clickFn()
	}
	return 0, fmt.Errorf("sem frames após stopped")
}

func (s *Session) topFrame() (int, error) {
	r, err := s.rpc.Request("threads", nil, 15*time.Second)
	if err != nil {
		return 0, err
	}
	body, _ := r["body"].(rpcMsg)
	thr, _ := body["threads"].([]any)
	tid := 1
	if len(thr) > 0 {
		if t0, ok := thr[0].(rpcMsg); ok {
			if v, ok := t0["id"].(float64); ok {
				tid = int(v)
			}
		}
	}
	r, err = s.rpc.Request("stackTrace", rpcMsg{
		"threadId": tid, "startFrame": 0, "levels": 30,
	}, 20*time.Second)
	if err != nil {
		return 0, err
	}
	body, _ = r["body"].(rpcMsg)
	frames, _ := body["stackFrames"].([]any)
	if len(frames) == 0 {
		return 0, fmt.Errorf("pilha vazia")
	}
	f0, ok := frames[0].(rpcMsg)
	if !ok {
		return 0, fmt.Errorf("frame malformado")
	}
	v, ok := f0["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("frame sem id")
	}
	return int(v), nil
}

// Evaluate roda uma expressão AdvPL no contexto do frame (DAP evaluate →
// MS_DBGEVAL 0x8149 no servidor). Retorna o resultado textual cru
// (com aspas DAP, se string).
func (s *Session) Evaluate(expr string, frameID int, timeout time.Duration) (string, error) {
	resp, err := s.rpc.Request("evaluate", rpcMsg{
		"expression": expr, "frameId": frameID, "context": "repl",
	}, timeout)
	if err != nil {
		return "", fmt.Errorf("evaluate: %w", err)
	}
	if ok, _ := resp["success"].(bool); !ok {
		return "", fmt.Errorf("evaluate recusado: %v", resp["message"])
	}
	body, _ := resp["body"].(rpcMsg)
	res, _ := body["result"].(string)
	return res, nil
}

// Close encerra o DA.
func (s *Session) Close() error {
	if s.rpc != nil {
		return s.rpc.Close()
	}
	return nil
}

// execClick clica na posição do diálogo do smartclient (dismiss). Melhor
// esforço: sem xdotool/display, apenas não clica.
func (s *Session) execClick() {
	if s.cfg.ClickX == 0 && s.cfg.ClickY == 0 {
		return
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		return
	}
	cmd := exec.Command("xdotool", "mousemove",
		strconv.Itoa(s.cfg.ClickX), strconv.Itoa(s.cfg.ClickY), "click", "1")
	cmd.Env = append(os.Environ(), "DISPLAY="+s.cfg.Display)
	_ = cmd.Run()
}
