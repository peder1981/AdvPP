package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/dap"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/ui"
	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

// version é injetada no build via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := runFile(sourceFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "compile":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		outputFile := "output.bytecode"
		if len(os.Args) >= 4 && os.Args[3] == "-o" && len(os.Args) >= 5 {
			outputFile = os.Args[4]
		}
		opts := parseOptions(os.Args[3:])
		if err := compileFile(sourceFile, outputFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Bytecode saved to: %s\n", outputFile)

	case "exec":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing bytecode file")
			os.Exit(1)
		}
		bytecodeFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := execBytecode(bytecodeFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		// -o era lido so quando caia exatamente em os.Args[3]: qualquer
		// flag antes dele (`build app.prw --gui -o App`) fazia o binario
		// sair calado em ./output. Procura em qualquer posicao.
		outputFile := "output"
		for i := 3; i+1 < len(os.Args); i++ {
			if os.Args[i] == "-o" {
				outputFile = os.Args[i+1]
				break
			}
		}
		opts := parseOptions(os.Args[3:])
		if err := buildStandalone(sourceFile, outputFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Standalone executable built: %s\n", outputFile)

	case "check":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		// Aceita múltiplos arquivos: verificação paralela (1 worker por CPU)
		rest := os.Args[2:]
		files := make([]string, 0, len(rest))
		i := 0
		for i < len(rest) && !strings.HasPrefix(rest[i], "-") {
			files = append(files, rest[i])
			i++
		}
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		opts := parseOptions(rest[i:])
		if opts.format == "json" {
			os.Exit(runCheckJSON(files, opts, os.Stdout))
		}
		if opts.format != "" {
			fmt.Fprintf(os.Stderr, "Error: unknown --format %q (use \"json\")\n", opts.format)
			os.Exit(1)
		}
		if len(files) == 1 {
			avisos, err := checkFile(files[0], opts)
			mostraAvisos(files[0], avisos)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("OK: syntax check passed")
		} else {
			os.Exit(checkFilesParallel(files, opts))
		}

	case "ast":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := printAST(sourceFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "bytecode":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := printBytecode(sourceFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "serve":
		if modoApps(os.Args[2:]) {
			if err := serveApps(parseOptions(os.Args[2:])); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing source file")
			os.Exit(1)
		}
		sourceFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := serveFile(sourceFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "debug":
		// Adaptador DAP (Debug Adapter Protocol) sobre stdio — sem arquivo
		// fixo na linha de comando: o fonte a compilar vem do campo
		// "program" do request "launch", enviado pelo editor. Flags de
		// compilação (--include, --define, --db, ...) valem como padrão
		// para qualquer "program" recebido.
		opts := parseOptions(os.Args[2:])
		if err := runDebugAdapter(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "peer":
		cmdPeer(os.Args[2:])

	case "rpo":
		if err := cmdRpo(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "inject":
		if err := cmdRpoInject(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\\n", err)
			os.Exit(1)
		}

	case "version", "--version", "-v":
		fmt.Printf("advplc %s\n", version)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

type Options struct {
	includes  []string
	defines   map[string]string
	uiEnabled bool
	dbPath    string
	dbBackend string
	port      string
	watch     bool
	gui       bool   // advplc build --gui: app desktop (janela sempre, subsistema GUI no Windows)
	debugPort string // advplc serve --debug-port: liga um listener DAP TCP (attach)
	format    string // check --format: "" (humano) ou "json" (um objeto por linha)
	apps      string // serve --apps <dir>: vários programas (um por arquivo)
	home      string // serve --home <nome>: app da página inicial
}

func parseOptions(args []string) *Options {
	opts := &Options{
		includes:  make([]string, 0),
		defines:   make(map[string]string),
		dbBackend: "sqlite",
	}
	for i := 0; i < len(args); i++ {
		if v, ok := strings.CutPrefix(args[i], "--format="); ok {
			opts.format = v
			continue
		}
		switch args[i] {
		case "--include", "-I":
			if i+1 < len(args) {
				opts.includes = append(opts.includes, args[i+1])
				i++
			}
		case "--define", "-D":
			if i+1 < len(args) {
				parts := strings.SplitN(args[i+1], "=", 2)
				if len(parts) == 2 {
					opts.defines[parts[0]] = parts[1]
				} else {
					opts.defines[parts[0]] = "1"
				}
				i++
			}
		case "--format":
			if i+1 < len(args) {
				opts.format = args[i+1]
				i++
			}
		case "--ui":
			opts.uiEnabled = true
		case "--headless":
			opts.uiEnabled = false
		case "--db":
			if i+1 < len(args) {
				opts.dbBackend = args[i+1]
				i++
			}
		case "--db-path":
			if i+1 < len(args) {
				opts.dbPath = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				opts.port = args[i+1]
				i++
			}
		case "--gui":
			opts.gui = true
		case "--watch", "-w":
			opts.watch = true
		case "--debug-port":
			if i+1 < len(args) {
				opts.debugPort = args[i+1]
				i++
			}
		case "--apps":
			if i+1 < len(args) {
				opts.apps = args[i+1]
				i++
			}
		case "--home":
			if i+1 < len(args) {
				opts.home = args[i+1]
				i++
			}
		}
	}
	return opts
}

func loadAndCompile(sourceFile string, opts *Options) (*compiler.Bytecode, error) {
	bc, _, err := compilaComAvisos(sourceFile, opts)
	return bc, err
}

// parseFonteComAvisos lê, preprocessa, lexa e parseia um fonte AdvPL/TLPP,
// devolvendo o AST, os avisos do parser+preprocessador e as dependências
// resolvidas (#include). includes já deve vir completo (o chamador decide
// os diretórios de #include).
func parseFonteComAvisos(sourceFile string, includes []string, opts *Options) (*ast.Program, []parser.Aviso, []string, error) {
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot read file %s: %v", sourceFile, err)
	}

	// Detect and convert CP1252 encoding to UTF-8 if needed
	sourceStr := preprocessor.SourceToUTF8(source)

	// Preprocess
	pp := preprocessor.NewPreprocessor(includes)
	for k, v := range opts.defines {
		pp.GetDefines()[k] = v
	}
	processed, err := pp.Process(sourceStr, sourceFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("preprocessor error: %v", err)
	}

	// Lex
	tokens, err := lexer.Tokenize(processed, sourceFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lexer error: %v", err)
	}

	// Parse
	p := parser.NewParser(tokens, sourceFile, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parser error: %v", err)
	}
	prog.RootBoundaryLine = pp.RootBoundaryLine

	avisos := p.Avisos()
	for _, a := range pp.Avisos() {
		msg := a.Mensagem
		if a.Arquivo != sourceFile {
			msg = a.Arquivo + ": " + msg
		}
		avisos = append(avisos, parser.Aviso{Line: a.Linha, Col: 1, Message: msg})
	}
	return prog, avisos, pp.Dependencias(), nil
}

// compilaComAvisos é o loadAndCompile que também devolve os avisos do parser
// (problemas que compilam, como no Protheus, mas que o check mostra).
func compilaComAvisos(sourceFile string, opts *Options) (*compiler.Bytecode, []parser.Aviso, error) {
	// Add default include paths (cópia: append não pode vazar no opts.includes)
	includes := append([]string{}, opts.includes...)
	includes = append(includes, filepath.Dir(sourceFile))

	prog, avisos, _, err := parseFonteComAvisos(sourceFile, includes, opts)
	if err != nil {
		return nil, nil, err
	}

	// Compile
	bc, err := compiler.Compile(prog)
	if err != nil {
		return nil, nil, fmt.Errorf("compiler error: %v", err)
	}
	return bc, avisos, nil
}

// extFonteProjeto: extensões que compilaDiretorio carrega num "RPO".
func extFonteProjeto(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".prw", ".prx", ".tlpp":
		return true
	}
	return false
}

// compilaDiretorio compila todos os fontes .prw/.prx/.tlpp da raiz de dir
// (não recursivo) num único bytecode via CompileProject — é o que
// `advplc serve <diretório>` carrega. Ordem alfabética dos arquivos mantém
// o bytecode determinístico.
func compilaDiretorio(dir string, opts *Options) (*compiler.Bytecode, error) {
	bc, _, err := compilaDiretorioDeps(dir, opts)
	return bc, err
}

// compilaDiretorioDeps é o compilaDiretorio que também devolve as
// dependências resolvidas (#include) de todos os fontes — o --watch usa
// para recompilar quando um .ch/include muda.
func compilaDiretorioDeps(dir string, opts *Options) (*compiler.Bytecode, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read directory %s: %v", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || !extFonteProjeto(e.Name()) {
			continue // só arquivos regulares .prw/.prx/.tlpp
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no .prw/.prx/.tlpp sources in %s", dir)
	}

	includes := append([]string{}, opts.includes...)
	includes = append(includes, dir)
	progs := make([]*ast.Program, 0, len(files))
	deps := make([]string, 0, len(files))
	for _, f := range files {
		prog, avisos, d, err := parseFonteComAvisos(f, includes, opts)
		mostraAvisos(f, avisos)
		if err != nil {
			return nil, nil, err
		}
		progs = append(progs, prog)
		deps = append(deps, d...)
	}
	bc, err := compiler.CompileProject(progs)
	if err != nil {
		return nil, nil, fmt.Errorf("compiler error: %v", err)
	}
	// Watch: além dos includes resolvidos, vigia os CANDIDATOS de
	// #include (inclusive os que ainda não existem) — criar um .ch
	// referenciado precisa mudar a assinatura e disparar recompilação.
	deps = append(deps, candidatosIncludes(anexaRaizes(files, deps), includes)...)
	return bc, deps, nil
}

// compilaFonteDeps é o loadAndCompile que também devolve as dependências
// (#include) do fonte — para o --watch de arquivo único.
func compilaFonteDeps(sourceFile string, opts *Options) (*compiler.Bytecode, []string, error) {
	includes := append([]string{}, opts.includes...)
	includes = append(includes, filepath.Dir(sourceFile))
	prog, _, deps, err := parseFonteComAvisos(sourceFile, includes, opts)
	if err != nil {
		return nil, nil, err
	}
	bc, err := compiler.Compile(prog)
	if err != nil {
		return nil, nil, fmt.Errorf("compiler error: %v", err)
	}
	deps = append(deps, candidatosIncludes(anexaRaizes([]string{sourceFile}, deps), includes)...)
	return bc, deps, nil
}

// anexaRaizes junta fontes e dependências já resolvidas numa lista de
// raízes para a varredura de candidatos de #include.
func anexaRaizes(fontes, deps []string) []string {
	out := make([]string, 0, len(fontes)+len(deps))
	out = append(out, fontes...)
	return append(out, deps...)
}

// reIncludeDiretiva casa `#include "x"` / `#include <x>` (qualquer caixa)
// no texto bruto — usado só pelo watch, para achar diretivas que o
// preprocessor não resolveu (include ainda inexistente não vira dep).
var reIncludeDiretiva = regexp.MustCompile(`(?im)^\s*#include\s+[<"]([^>"]+)[>"]`)

// candidatosIncludes varre fontes e includes já resolvidos (worklist)
// atrás de diretivas #include e devolve todos os caminhos candidatos,
// espelhando a busca do preprocessor: diretório do arquivo, caminhos de
// --include e subpastas ch/include/includes. Candidatos inexistentes
// entram na assinatura como "missing" — quando o arquivo for criado, a
// impressão digital muda e o watch recompila. Includes resolvidos são
// varridos recursivamente (um .ch pode incluir outro .ch).
func candidatosIncludes(raizes, includes []string) []string {
	vistos := make(map[string]bool)
	var out []string
	fila := append([]string(nil), raizes...)
	for len(fila) > 0 {
		f := fila[0]
		fila = fila[1:]
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		dirs := []string{filepath.Dir(f)}
		dirs = append(dirs, includes...)
		for _, m := range reIncludeDiretiva.FindAllSubmatch(data, -1) {
			nome := string(m[1])
			for _, dir := range dirs {
				for _, sub := range []string{"", "ch", "include", "includes"} {
					base := dir
					if sub != "" {
						base = filepath.Join(dir, sub)
					}
					cand := filepath.Join(base, nome)
					if vistos[cand] {
						continue
					}
					vistos[cand] = true
					out = append(out, cand)
					if info, err := os.Stat(cand); err == nil && info.Mode().IsRegular() {
						fila = append(fila, cand)
					}
				}
			}
		}
	}
	return out
}

// compilaAlvo escolhe o pipeline pelo argumento do serve: diretório vira
// projeto (vários fontes, como um RPO); arquivo continua fonte único.
func compilaAlvo(sourceFile string, opts *Options) (*compiler.Bytecode, error) {
	bc, _, err := compilaAlvoDeps(sourceFile, opts)
	return bc, err
}

// compilaAlvoDeps é o compilaAlvo que também devolve as dependências
// resolvidas (#include) do alvo — fonte único ou todas as fontes do
// diretório.
func compilaAlvoDeps(sourceFile string, opts *Options) (*compiler.Bytecode, []string, error) {
	if st, err := os.Stat(sourceFile); err == nil && st.IsDir() {
		return compilaDiretorioDeps(sourceFile, opts)
	}
	return compilaFonteDeps(sourceFile, opts)
}

func runFile(sourceFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}

	v := vm.NewVM(bc, opts.uiEnabled)
	// Attach a real terminal UIProvider only when stdin is an actual TTY —
	// piped/redirected stdin (test harnesses, CI) keeps the old
	// no-provider behavior, where FWGetText/FWMenuSelect never block and
	// just return their default (see natives.go). Without this check,
	// batch usage would hang forever on the first login prompt.
	if ui.IsTerminal(os.Stdin) {
		v.SetUIProvider(ui.NewTerminalUIProvider())
	}
	attachDatabase(v, opts)
	_, err = v.Run()
	return err
}

// attachDatabase conecta o VM ao banco SQLite compartilhado entre todas as
// ferramentas AdvPP (mesmo resolver usado por adveditor/advpp-ide).
// O banco é sempre anexado, mesmo que o arquivo ainda não exista — o driver
// SQLite cria um arquivo novo vazio no primeiro open. Isso permite que
// RetSqlName/DbSelectArea/etc. funcionem assim que o usuário criar tabelas
// nesse banco via adveditor, sem exigir configuração prévia:
// ResolveDatabasePath já resolve para um banco local (./advpp.db) do
// diretório de trabalho atual quando nada foi configurado globalmente.
func attachDatabase(v *vm.VM, opts *Options) {
	dbPath := shared.ResolveDatabasePath(opts.dbPath)
	// Fábrica: o VM principal e cada job do StartJob() abrem a própria
	// conexão sobre o mesmo arquivo (WAL permite leitura concorrente).
	v.SetDBFactory(func() vm.DBEngine {
		engine, err := db.NewSQLiteEngine(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot open database %s: %v\n", dbPath, err)
			return nil
		}
		return engine
	})
}

func checkFile(sourceFile string, opts *Options) ([]parser.Aviso, error) {
	_, avisos, err := compilaComAvisos(sourceFile, opts)
	return avisos, err
}

// mostraAvisos escreve os avisos no stderr, um por linha, com a posição.
func mostraAvisos(sourceFile string, avisos []parser.Aviso) {
	for _, a := range avisos {
		fmt.Fprintf(os.Stderr, "Aviso: %s:%d:%d: %s\n", sourceFile, a.Line, a.Col, a.Message)
	}
}

// serveFile executa o programa com a interface renderizada no browser
// (fase 1: console + diálogos). Cada aba/recarga do browser cria uma
// sessão com VM isolada — mesma semântica de work process do StartJob.
func serveFile(sourceFile string, opts *Options) error {
	// Diretório vira projeto (todos os fontes num bytecode só, como um
	// RPO); arquivo segue fonte único. compilaAlvoDeps decide pelo
	// os.Stat e devolve as dependências (#include) para o --watch.
	var deps atomic.Value // []string — deps do último compile bem-sucedido
	compile := func() (*compiler.Bytecode, error) {
		bc, d, err := compilaAlvoDeps(sourceFile, opts)
		if err == nil {
			deps.Store(d)
		}
		return bc, err
	}
	bc, err := compile()
	if err != nil {
		return err
	}
	// bytecode atual atrás de um ponteiro atômico: o --watch (fase 3) troca
	// pela versão recompilada e cada nova sessão executa a mais recente
	var current atomic.Pointer[compiler.Bytecode]
	current.Store(bc)

	// dbgSrv: sessão DAP "attach" conectada agora, se houver (--debug-port).
	// Um ponteiro atômico simples — só a conexão mais recente é considerada;
	// reconectar (ex.: reload do editor) substitui a anterior sem problema.
	var dbgSrv atomic.Pointer[dap.Server]

	// PoMenu:AddItem recusa ícone que o PO-UI embutido não tem.
	vm.DefinirIconesValidos(webui.Icones())
	srv := webui.New(filepath.Base(sourceFile),
		func(ui *webui.Provider, console *webui.OutWriter) error {
			v := vm.NewVM(current.Load(), true)
			v.SetUIProvider(ui)
			if st, err := os.Stat(sourceFile); err == nil {
				if st.IsDir() {
					v.SetSourceDir(sourceFile)
				} else {
					v.SetSourceDir(filepath.Dir(sourceFile))
				}
			}
			attachDatabase(v, opts)

			var release func(error)
			if ds := dbgSrv.Load(); ds != nil {
				if claimed, r := ds.OfferSession(v); claimed {
					release = r
					v.SetOutputWriter(io.MultiWriter(console, ds.OutputWriter()))
				}
			}
			if release == nil {
				v.SetOutputWriter(console)
			}

			// defer: roda também quando a sessão é abandonada (webui encerra a
			// goroutine com runtime.Goexit num diálogo pendente).
			defer v.CloseOwnedConnections()
			_, err := v.Run()
			if release != nil {
				release(err)
			}
			return err
		})

	// REST TOTVS (@Get/... e WSRESTFUL) montado sob shared.ResolveRestPrefix
	// ("/rest" por padrão). O ponteiro atômico é trocado pelo --watch.
	restSrv := montaRESTNoServe(bc, opts, srv)

	if opts.watch {
		go watchSource(sourceFile, compile, &deps, &current, srv, func(bc *compiler.Bytecode) {
			registraRest(bc, opts, restSrv)
		})
	}

	if opts.debugPort != "" {
		go serveDebugAttach(opts.debugPort, sourceFile, &dbgSrv)
	}

	port := shared.ResolveWebUIPort(opts.port)
	host := os.Getenv("ADVPL_SERVE_HOST")
	if host == "" {
		host = "localhost"
	}
	return srv.Serve(host + ":" + port)
}

// serveDebugAttach escuta conexões DAP (attach) numa porta TCP separada da
// porta HTTP do modo web — é isso que "advplc serve --debug-port N" liga.
// Cada conexão vira uma sessão attach independente (uma por vez de verdade
// importa: OfferSession só entrega sessões de browser pro dbgSrv atual).
func serveDebugAttach(port, sourceFile string, dbgSrv *atomic.Pointer[dap.Server]) {
	ln, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "debug-port: %v\n", err)
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		srv := dap.NewAttachServer(dap.NewConn(conn, conn), sourceFile)
		dbgSrv.Store(srv)
		go func() {
			defer conn.Close()
			srv.Run()
		}()
	}
}

// assinaturaFonte devolve uma impressão digital do alvo do watch: mtime do
// arquivo, ou a lista "nome:mtime" dos fontes do projeto (diretório), mais
// o mtime de cada dependência resolvida (#include/.ch — deps vem do último
// compile). Mudança de conteúdo, de nome, de conjunto de arquivos ou de um
// include dispara recompilação.
func assinaturaFonte(sourceFile string, deps []string) (string, error) {
	st, err := os.Stat(sourceFile)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if st.IsDir() {
		entries, err := os.ReadDir(sourceFile)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() || !extFonteProjeto(e.Name()) {
				continue
			}
			fmt.Fprintf(&b, "%s:%d;", e.Name(), info.ModTime().UnixNano())
		}
	} else {
		fmt.Fprintf(&b, "%s:%d", sourceFile, st.ModTime().UnixNano())
	}
	for _, d := range deps {
		if info, err := os.Stat(d); err == nil {
			fmt.Fprintf(&b, "|%s:%d", d, info.ModTime().UnixNano())
		} else {
			fmt.Fprintf(&b, "|%s:missing", d)
		}
	}
	return b.String(), nil
}

// watchSource observa o fonte (polling de mtime, sem dependências) — ou o
// conjunto de fontes e seus #include, quando o alvo é um diretório — e, a
// cada alteração, recompila e manda as sessões do browser recarregarem.
// Erro de compilação vai para o console do browser em vez de recarregar.
func watchSource(sourceFile string, compile func() (*compiler.Bytecode, error), deps *atomic.Value, current *atomic.Pointer[compiler.Bytecode], srv *webui.Server, onRecompiled ...func(*compiler.Bytecode)) {
	sig := func() string {
		d, _ := deps.Load().([]string)
		s, err := assinaturaFonte(sourceFile, d)
		if err != nil {
			return ""
		}
		return s
	}
	lastSig := sig()
	for {
		time.Sleep(500 * time.Millisecond)
		s := sig()
		if s == lastSig {
			continue
		}
		lastSig = s
		bc, err := compile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "watch: erro de compilação: %v\n", err)
			srv.Broadcast("error", fmt.Sprintf("erro de compilação: %v", err))
			continue
		}
		current.Store(bc)
		if len(onRecompiled) > 0 && onRecompiled[0] != nil {
			onRecompiled[0](bc)
		}
		fmt.Printf("watch: %s recompilado, recarregando sessões\n", filepath.Base(sourceFile))
		srv.Broadcast("reload", "fonte alterado")
	}
}

// runDebugAdapter serve uma sessão DAP (Debug Adapter Protocol) sobre
// stdin/stdout — é isso que "advplc debug" spawna quando o editor inicia
// uma sessão de debug (F5). O fonte a compilar chega via o request "launch"
// (campo "program"); opts fornece os defaults de compilação (--include,
// --define, --db, ...) vindos da linha de comando do próprio adaptador.
func runDebugAdapter(opts *Options) error {
	// CONOUT/CONOUTW (pkg/vm/natives.go) sempre espelham em os.Stdout, além
	// do writer opcional via SetOutputWriter — correto para run/serve, mas
	// aqui os.Stdout É o transporte DAP (frames Content-Length). Sem isso,
	// o primeiro ConOut() do programa debugado corrompe o stream JSON no
	// meio da sessão. Guarda o stdout real pro DAP e redireciona o global
	// os.Stdout (o que fmt.Println usa) pro /dev/null; a saída do programa
	// ainda chega ao usuário via evento "output" (outputWriter em server.go).
	realStdout := os.Stdout
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		os.Stdout = devNull
	}

	conn := dap.NewConn(os.Stdin, realStdout)
	srv := dap.NewServer(conn,
		func(sourceFile string) (*compiler.Bytecode, error) {
			return loadAndCompile(sourceFile, opts)
		},
		func(v *vm.VM) {
			attachDatabase(v, opts)
		},
	)
	return srv.Run()
}

// checkJSONResult é uma linha do `check --format json` (JSONL: um objeto
// por linha, ordem estável de entrada para consumo determinístico).
type checkJSONResult struct {
	File  string `json:"file"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// Posição do erro (para marcar no editor) e a mensagem sem ela. Sem
	// posição reconhecível no erro, line/col ficam de fora.
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	Message string `json:"message,omitempty"`
	// Avisos (compila, mas merece atenção), com posição.
	Warnings []parser.Aviso `json:"warnings,omitempty"`
}

// reFimPosicao: " at <arquivo>:<linha>:<coluna>" no fim dos erros do
// lexer/parser.
var reFimPosicao = regexp.MustCompile(`\s+at\s+\S.*:(\d+):(\d+)\s*$`)

// posicaoDoErro separa linha, coluna e mensagem de um erro de compilação.
func posicaoDoErro(e string) (int, int, string) {
	m := reFimPosicao.FindStringSubmatchIndex(e)
	if m == nil {
		return 0, 0, e
	}
	l, _ := strconv.Atoi(e[m[2]:m[3]])
	c, _ := strconv.Atoi(e[m[4]:m[5]])
	return l, c, strings.TrimSpace(e[:m[0]])
}

// runCheckJSON valida cada arquivo em sequência (ordem de saída == ordem de
// entrada) e escreve um objeto JSON por linha em w. Erros vão para o próprio
// JSON (stdout), nunca stderr, para o consumidor parsear sem adivinhar.
// Retorna 0 se todos passaram, 1 caso contrário (mesmo contrato do modo
// humano). Formato pensado para agentes (PiG/pig-advpp) e CI parseável.
func runCheckJSON(files []string, opts *Options, w *os.File) int {
	enc := json.NewEncoder(w)
	failed := false
	for _, f := range files {
		r := checkJSONResult{File: f, OK: true}
		avisos, err := checkFile(f, opts)
		r.Warnings = avisos
		if err != nil {
			r.OK = false
			r.Error = err.Error()
			r.Line, r.Col, r.Message = posicaoDoErro(r.Error)
			failed = true
		}
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}
	if failed {
		return 1
	}
	return 0
}

// checkFilesParallel verifica N arquivos com um pool de workers (1 por CPU).
// Cada arquivo é compilado de forma independente — o pipeline
// preprocessador→lexer→parser→codegen não compartilha estado entre arquivos.
func checkFilesParallel(files []string, opts *Options) int {
	type result struct {
		file   string
		avisos []parser.Aviso
		err    error
	}
	jobs := make(chan string)
	results := make(chan result)

	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				avisos, err := checkFile(f, opts)
				results <- result{f, avisos, err}
			}
		}()
	}
	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	pass, fail := 0, 0
	for r := range results {
		mostraAvisos(r.file, r.avisos)
		if r.err != nil {
			fail++
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", r.file, r.err)
		} else {
			pass++
			fmt.Printf("OK   %s\n", r.file)
		}
	}
	fmt.Printf("checked %d files: %d ok, %d failed (%d workers)\n",
		len(files), pass, fail, workers)
	if fail > 0 {
		return 1
	}
	return 0
}

func compileFile(sourceFile, outputFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}

	return compiler.SaveBytecode(bc, outputFile)
}

func execBytecode(bytecodeFile string, opts *Options) error {
	bc, err := compiler.LoadBytecode(bytecodeFile)
	if err != nil {
		return fmt.Errorf("cannot load bytecode: %v", err)
	}

	v := vm.NewVM(bc, opts.uiEnabled)
	// Attach a real terminal UIProvider only when stdin is an actual TTY —
	// piped/redirected stdin (test harnesses, CI) keeps the old
	// no-provider behavior, where FWGetText/FWMenuSelect never block and
	// just return their default (see natives.go). Without this check,
	// batch usage would hang forever on the first login prompt.
	if ui.IsTerminal(os.Stdin) {
		v.SetUIProvider(ui.NewTerminalUIProvider())
	}
	attachDatabase(v, opts)
	_, err = v.Run()
	return err
}

func buildStandalone(sourceFile, outputFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}
	title := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	if title != "" {
		title = strings.ToUpper(title[:1]) + title[1:]
	} else {
		title = "AdvPP"
	}
	return compiler.BuildStandaloneGUI(bc, outputFile, title, opts.gui, os.Stdout)
}

func printAST(sourceFile string, opts *Options) error {
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	includes := opts.includes
	includes = append(includes, filepath.Dir(sourceFile))

	pp := preprocessor.NewPreprocessor(includes)
	processed, err := pp.Process(preprocessor.SourceToUTF8(source), sourceFile)
	if err != nil {
		return err
	}

	tokens, err := lexer.Tokenize(processed, sourceFile)
	if err != nil {
		return err
	}

	p := parser.NewParser(tokens, sourceFile, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return err
	}

	fmt.Printf("Program: %s\n", sourceFile)
	fmt.Printf("  Functions: %d\n", len(prog.Functions))
	for _, fn := range prog.Functions {
		fmt.Printf("    %s (params: %d)\n", fn.Name, len(fn.Params))
	}
	fmt.Printf("  Classes: %d\n", len(prog.Classes))
	for _, cls := range prog.Classes {
		fmt.Printf("    Class %s (parent: %s, properties: %d, methods: %d)\n",
			cls.Name, cls.Parent, len(cls.Properties), len(cls.Methods))
	}
	fmt.Printf("  Methods: %d\n", len(prog.Methods))
	fmt.Printf("  Body statements: %d\n", len(prog.Body))

	return nil
}

func printBytecode(sourceFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}

	fmt.Printf("Constants: %d\n", len(bc.Constants))
	for i, c := range bc.Constants {
		switch c.Type {
		case "number":
			fmt.Printf("  [%d] number: %g\n", i, c.Num)
		case "string":
			fmt.Printf("  [%d] string: %q\n", i, c.Str)
		}
	}

	fmt.Printf("\nFunctions: %d\n", len(bc.Functions))
	for name, fn := range bc.Functions {
		fmt.Printf("  %s (params: %d, locals: %d, offset: %d)\n",
			name, fn.NumParams, fn.NumLocals, fn.Offset)
	}

	fmt.Printf("\nClasses: %d\n", len(bc.Classes))
	for name, cls := range bc.Classes {
		fmt.Printf("  %s (parent: %s, methods: %d)\n", name, cls.Parent, len(cls.Methods))
	}

	fmt.Printf("\nBytecode: %d instructions\n", len(bc.Code))
	for i, instr := range bc.Code {
		fmt.Printf("  %4d  %-20s arg=%d arg2=%d str=%q\n",
			i, instr.Op.String(), instr.Arg, instr.Arg2, instr.Str)
	}

	return nil
}

func printUsage() {
	fmt.Println(`advplc — AdvPL/TLPP Compiler

Usage:
  advplc <command> <source.prw> [options]

Commands:
  run       Compile and execute an AdvPL/TLPP source file
  compile   Compile source to bytecode file (use -o for output)
  exec      Execute a compiled bytecode file
  check     Validate syntax without executing (accepts multiple files)
  serve     Run the program with the UI rendered in the browser (web mode);
            accepts a source file or a directory — a directory loads every
            .prw/.prx/.tlpp in it as one project (like an RPO), with
            cross-file classes/functions and per-file Static Function
  build     Compile to a standalone desktop executable (Fyne) with embedded
            bytecode (use -o for output; run from within the repo or set
            ADVPP_SRC so the stub can find a checkout of this module)
  debug     Serve a Debug Adapter Protocol (DAP) session over stdio, for
            editor "Run and Debug" integration (breakpoints, step, locals)
  ast       Print the AST structure
  bytecode  Print the compiled bytecode

Options:
  --include <path>, -I <path>   Add include path (repeatable)
  --define <name=val>, -D       Define preprocessor symbol
  --ui                          Enable UI (Fyne)
  --headless                    Disable UI (default)
  --db <backend>                Database backend (only "sqlite" is
                                implemented; the flag is accepted but has
                                no effect on the actual backend used)
  --db-path <path>              Path to SQLite database file
                                (default: $ADVPP_DB, or the shared AdvPP
                                database configured in ~/.advpp — the same
                                database used by adveditor/advpp-ide)
  --format <fmt>                check: "json" emits one JSON object per
                                line instead of human text
  -o <file>                     Output file for compile command
  --port <n>                    Web mode port (default: webui_port in
                                ~/.advpp/advpp_config.json, or 8080)
  --apps <dir>                  serve: several programs, one per file —
                                <dir>/<name>.prw|.tlpp is served at
                                /app/<name> (no source argument)
  --home <name>                 serve --apps: app shown at "/"
  -w, --watch                   Web mode: recompile on source change and
                                reload browser sessions (hot reload)
  --gui                         build: mark the program as a desktop app.
                                The executable always opens the Fyne window
                                instead of falling back to the terminal UI,
                                and on Windows it is linked into the GUI
                                subsystem (no console window). Required for
                                any program that uses MSDIALOG.

Examples:
  advplc run hello.prw
  advplc run ui_demo.prw --ui
  advplc compile hello.prw -o hello.bytecode
  advplc exec hello.bytecode
  advplc check program.prw --include ./includes
  advplc check a.prw b.prw --format json
  advplc serve program.prw --port 9000 --watch
  advplc serve ./fontes --watch
  advplc serve --apps ./apps --home inicio
  advplc build program.prw -o myapp --db-path ./data.db
  advplc build app.prw -o MyApp --gui
  advplc bytecode program.prw`)
}
