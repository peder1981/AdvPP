package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/advpl/compiler/pkg/rpo/dap"
	"github.com/advpl/compiler/pkg/rpo/wire"
)

// pullConfig são as opções de "advplc rpo pull".
type pullConfig struct {
	host        string
	port        int
	user        string
	envName     string
	build       string
	out         string
	manifest    string
	resPattern  string
	daPath      string
	lsPath      string
	workspace   string
	smartclient string
	program     string
	display     string
	clickX      int
	clickY      int
}

// pullEval avalia uma expressão no servidor (injetado nos testes).
type pullEval func(expr string, timeout time.Duration) (string, error)

type pullStatus string

const (
	pullOK   pullStatus = "ok"
	pullNil  pullStatus = "nil"
	pullSkip pullStatus = "skip"
	pullErr  pullStatus = "err"
)

// cmdRpoPull implementa "advplc rpo pull --out dir [--manifest f|
// --res padrao] ...": baixa recursos/APOs via Encode64(GetApoRes(...))
// numa sessão DAP viva (LS + debugAdapter + smartclient).
func cmdRpoPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	host := fs.String("host", "172.17.0.3", "ip do appserver")
	port := fs.Int("port", 1234, "porta do appserver")
	user := fs.String("user", "peder", "usuário Protheus")
	envName := fs.String("env", "P12", "ambiente Protheus")
	build := fs.String("build", "7.00.210324P", "build do appserver")
	out := fs.String("out", "", "diretório de saída (obrigatório)")
	manifest := fs.String("manifest", "", "arquivo com 1 nome por linha")
	resPattern := fs.String("res", "", "padrão GetResArray (ex.: *.PNG)")
	daPath := fs.String("da", "/tmp/opencode/tds-da/debugAdapter-linux",
		"binário do debug adapter (TDS)")
	lsPath := fs.String("ls", "/tmp/opencode/tds-ls/advpls-linux-226",
		"binário do language server (TDS)")
	workspace := fs.String("workspace", "",
		"dir com o .prw do programa de depuração")
	smartclient := fs.String("smartclient", "/tmp/opencode/advpls-test/sc_real.sh",
		"wrapper do smartclient")
	program := fs.String("program", "U_TESTWAIT", "programa de depuração")
	display := fs.String("display", ":99", "DISPLAY do X (cliques de dismiss)")
	click := fs.String("click", "691,549", "posição x,y do clique (vazio=nenhum)")

	// pull não tem argumentos posicionais: parse direto (apoSplitArgs
	// só conhece os flags do apo e jogaria os valores para "positional")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("uso: advplc rpo pull --out dir [--manifest f|--res padrao] " +
			"[--host h] [--port n] [--user u] [--env e] [--da bin] [--ls bin] " +
			"[--workspace dir] [--smartclient bin] [--program nome] [--display :n] [--click x,y]")
	}
	if *manifest == "" && *resPattern == "" {
		return fmt.Errorf("informe --manifest <arquivo> ou --res <padrao>")
	}
	if *workspace == "" {
		return fmt.Errorf("--workspace é obrigatório (dir com %s.prw)", *program)
	}

	clickX, clickY := 0, 0
	if *click != "" {
		parts := strings.SplitN(*click, ",", 2)
		if len(parts) == 2 {
			clickX, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
			clickY, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
	}

	cfg := pullConfig{
		host: *host, port: *port, user: *user, envName: *envName,
		build: *build, out: *out, manifest: *manifest, resPattern: *resPattern,
		daPath: *daPath, lsPath: *lsPath, workspace: *workspace,
		smartclient: *smartclient, program: *program, display: *display,
		clickX: clickX, clickY: clickY,
	}

	pass, err := pullPassword()
	if err != nil {
		return err
	}
	return pullRun(cfg, pass)
}

// pullPassword: ADVPP_RPO_PASS senão prompt sem eco (Lei 6 — nunca loga).
func pullPassword() (string, error) {
	if p := os.Getenv("ADVPP_RPO_PASS"); p != "" {
		return p, nil
	}
	fmt.Fprint(os.Stderr, "Senha do Protheus: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("lendo senha: %w", err)
	}
	return string(b), nil
}

// pullRun conecta (LS → token), sobe a sessão (DA), resolve a lista e
// baixa item a item com escrita atômica.
func pullRun(cfg pullConfig, pass string) error {
	if err := os.MkdirAll(cfg.out, 0755); err != nil {
		return fmt.Errorf("criando --out: %w", err)
	}
	t0 := time.Now()

	tok, err := dap.GetToken(dap.LSConfig{
		Path: cfg.lsPath, Server: cfg.host, Port: cfg.port,
		Build: cfg.build, Env: cfg.envName, User: cfg.user, Pass: pass,
		ErrFile: filepath.Join(cfg.out, ".pull-ls.err"),
	})
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}
	defer tok.Close()

	sess, err := dap.NewSession(dap.SessionConfig{
		DAPath: cfg.daPath,
		LogFile: filepath.Join(cfg.out, ".pull-da.log"),
		ErrFile: filepath.Join(cfg.out, ".pull-da.err"),
		Server: cfg.host, Port: cfg.port, Build: cfg.build,
		Env: cfg.envName, EnvType: tok.EnvType, Token: tok.Token,
		Program: cfg.program, Workspace: cfg.workspace,
		SrcPath: filepath.Join(cfg.workspace, cfg.program+".prw"),
		SmartclientBin: cfg.smartclient, Display: cfg.display,
		ClickX: cfg.clickX, ClickY: cfg.clickY,
	})
	if err != nil {
		return fmt.Errorf("sessão DAP: %w", err)
	}
	defer sess.Close()

	if err := sess.Launch(context.Background()); err != nil {
		return err
	}
	if err := sess.SetBreakpoints(); err != nil {
		return err
	}
	if err := sess.ConfigurationDone(); err != nil {
		return err
	}
	fid, err := sess.WaitStopped(context.Background())
	if err != nil {
		return fmt.Errorf("aguardando stop: %w", err)
	}
	fmt.Printf("Sessão DAP pronta (frameId=%d, envType=%s)\n", fid, tok.EnvType)

	eval := func(expr string, d time.Duration) (string, error) {
		return sess.Evaluate(expr, fid, d)
	}

	var names []string
	if cfg.manifest != "" {
		names, err = pullReadManifest(cfg.manifest)
	} else {
		names, err = pullResList(eval, cfg.resPattern)
	}
	if err != nil {
		return err
	}

	stats := map[pullStatus]int{}
	for i, name := range names {
		dst := filepath.Join(cfg.out, name)
		st, perr := pullOne(name, dst, eval)
		stats[st]++
		if st == pullErr && stats[pullErr] <= 20 {
			fmt.Printf("  ERR %s: %v\n", name, perr)
		}
		if (i+1)%200 == 0 {
			el := time.Since(t0).Seconds()
			rate := float64(i+1) / el
			eta := float64(len(names)-i-1) / rate
			fmt.Printf("PROGRESS %d/%d ok=%d nil=%d err=%d skip=%d rate=%.1f/s eta=%.0fmin\n",
				i+1, len(names), stats[pullOK], stats[pullNil],
				stats[pullErr], stats[pullSkip], rate, eta/60)
		}
	}
	fmt.Printf("PULL FIM: ok=%d nil=%d err=%d skip=%d tempo=%.0fs\n",
		stats[pullOK], stats[pullNil], stats[pullErr], stats[pullSkip],
		time.Since(t0).Seconds())
	return nil
}

// pullReadManifest lê 1 nome por linha (ignora vazios e comentários #).
func pullReadManifest(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrindo manifest: %w", err)
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("lendo manifest: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("manifest vazio: %s", path)
	}
	return names, nil
}

// pullResList monta a lista de resources via GetResArray + chunks AEval
// (mesmo método da onda 24 — dap_proxy24.py:311-330).
func pullResList(eval pullEval, pattern string) ([]string, error) {
	if _, err := eval("aAll := GetResArray('"+pattern+"')", 60*time.Second); err != nil {
		return nil, fmt.Errorf("GetResArray: %w", err)
	}
	v, err := eval("Len(aAll)", 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Len(aAll): %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(wire.UnquoteDAP(v)))
	if err != nil {
		return nil, fmt.Errorf("Len(aAll) não numérico: %q", v)
	}
	const chunk = 3000
	var names []string
	for start := 1; start <= n; start += chunk {
		end := start + chunk - 1
		if end > n {
			end = n
		}
		expr := fmt.Sprintf(
			"cOut := ''; AEval(aAll, {|x,n| IIf(n >= %d .and. n <= %d, "+
				"cOut := cOut + x + Chr(10), NIL)}); cOut", start, end)
		v, err := eval(expr, 180*time.Second)
		if err != nil {
			return nil, fmt.Errorf("chunk %d: %w", start, err)
		}
		for _, line := range strings.Split(wire.UnquoteDAP(v), "\n") {
			if line != "" {
				names = append(names, line)
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("GetResArray(%q) retornou vazio", pattern)
	}
	return names, nil
}

// pullOne baixa um item: avalia Encode64(GetApoRes('nome')), decodifica
// (aspas DAP → NIL/vazio/base64) e grava com escrita atômica. Pula
// arquivo já existente com size>0 (retomável — onda 39).
func pullOne(name, dst string, eval pullEval) (pullStatus, error) {
	if name == "" || strings.ContainsAny(name, "'\n\r") {
		return pullErr, fmt.Errorf("nome inválido: %q", name)
	}
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return pullSkip, nil
	}
	v, err := eval("Encode64(GetApoRes('"+name+"'))", 180*time.Second)
	if err != nil {
		return pullErr, err
	}
	data, status, err := wire.DecodeEvalValue(v)
	if err != nil {
		return pullErr, err
	}
	switch status {
	case wire.EvalOK:
		if dir := filepath.Dir(dst); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return pullErr, err
			}
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			return pullErr, err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return pullErr, err
		}
		return pullOK, nil
	case wire.EvalNil, wire.EvalEmpty:
		return pullNil, nil
	}
	return pullErr, fmt.Errorf("status indefinido (%s)", status)
}
