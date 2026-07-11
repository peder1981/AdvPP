package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

// version é injetada no build via -ldflags "-X main.version=v1.2.3"
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
		outputFile := "output"
		if len(os.Args) >= 4 && os.Args[3] == "-o" && len(os.Args) >= 5 {
			outputFile = os.Args[4]
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
		if len(files) == 1 {
			if err := checkFile(files[0], opts); err != nil {
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
}

func parseOptions(args []string) *Options {
	opts := &Options{
		includes:  make([]string, 0),
		defines:   make(map[string]string),
		dbBackend: "sqlite",
	}
	for i := 0; i < len(args); i++ {
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
		case "--watch", "-w":
			opts.watch = true
		}
	}
	return opts
}

func loadAndCompile(sourceFile string, opts *Options) (*compiler.Bytecode, error) {
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %v", sourceFile, err)
	}

	// Detect and convert CP1252 encoding to UTF-8 if needed
	sourceStr, err := convertToUTF8(source)
	if err != nil {
		return nil, fmt.Errorf("encoding conversion error: %v", err)
	}

	// Add default include paths
	includes := opts.includes
	includes = append(includes, filepath.Dir(sourceFile))

	// Add defines from command line
	for k, v := range opts.defines {
		_ = k
		_ = v
	}

	// Preprocess
	pp := preprocessor.NewPreprocessor(includes)
	for k, v := range opts.defines {
		pp.GetDefines()[k] = v
	}
	processed, err := pp.Process(sourceStr, sourceFile)
	if err != nil {
		return nil, fmt.Errorf("preprocessor error: %v", err)
	}

	// Lex
	tokens, err := lexer.Tokenize(processed, sourceFile)
	if err != nil {
		return nil, fmt.Errorf("lexer error: %v", err)
	}

	// Parse
	p := parser.NewParser(tokens, sourceFile, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parser error: %v", err)
	}

	// Compile
	bc, err := compiler.Compile(prog)
	if err != nil {
		return nil, fmt.Errorf("compiler error: %v", err)
	}

	return bc, nil
}

// convertToUTF8 detects if the source is CP1252 encoded and converts it to UTF-8
func convertToUTF8(source []byte) (string, error) {
	// Check if the source is already valid UTF-8
	if utf8.Valid(source) {
		return string(source), nil
	}

	// If not valid UTF-8, assume CP1252 and convert (pure Go: portável e
	// determinístico em Linux/Windows/macOS — sem depender do iconv externo)
	converted, err := convertWithGoEncoding(source)
	if err == nil {
		return converted, nil
	}

	// If all conversions fail, return the original string
	// This allows the lexer to handle any encoding issues
	return string(source), nil
}

// cp1252ToUTF8 mapeia cada um dos 256 valores de byte possíveis para sua
// codificação UTF-8 (CP-1252 é um superset de ISO-8859-1 com caracteres
// extras em 0x80-0x9F) — construída uma vez em init() por
// buildCP1252ToUTF8Table. convertWithGoEncoding indexa a tabela em vez de
// resolver a mesma cadeia de comparações a cada byte: para um arquivo de
// alguns MB isso é a diferença entre milhões de branches+WriteRune (que
// internamente ainda decide quantos bytes UTF-8 emitir a cada chamada) e
// milhões de acessos de array + Write — foi ~23% do tempo de compilação
// de um arquivo grande em profile real.
var cp1252ToUTF8 = buildCP1252ToUTF8Table()

func buildCP1252ToUTF8Table() [256][]byte {
	var t [256][]byte
	for b := 0; b < 256; b++ {
		t[b] = cp1252ByteToUTF8(byte(b))
	}
	return t
}

// cp1252ByteToUTF8 é a conversão de referência byte a byte (usada só para
// construir cp1252ToUTF8, e em teste, para validar a tabela exaustivamente
// contra os 256 valores possíveis).
func cp1252ByteToUTF8(b byte) []byte {
	if b < 128 {
		return []byte{b}
	}
	if b >= 160 {
		// ISO-8859-1 / CP1252 range (maps directly to Unicode)
		return utf8.AppendRune(nil, rune(b))
	}
	// CP1252 specific characters in range 0x80-0x9F
	switch b {
	case 0x80:
		return utf8.AppendRune(nil, '€') // Euro sign
	case 0x82:
		return utf8.AppendRune(nil, '‚') // Single low-9 quotation mark
	case 0x83:
		return utf8.AppendRune(nil, 'ƒ') // Latin small letter f with hook
	case 0x84:
		return utf8.AppendRune(nil, '„') // Double low-9 quotation mark
	case 0x85:
		return utf8.AppendRune(nil, '…') // Horizontal ellipsis
	case 0x86:
		return utf8.AppendRune(nil, '†') // Dagger
	case 0x87:
		return utf8.AppendRune(nil, '‡') // Double dagger
	case 0x88:
		return utf8.AppendRune(nil, 'ˆ') // Modifier letter circumflex accent
	case 0x89:
		return utf8.AppendRune(nil, '‰') // Per mille sign
	case 0x8A:
		return utf8.AppendRune(nil, 'Š') // Latin capital letter S with caron
	case 0x8B:
		return utf8.AppendRune(nil, '‹') // Single left-pointing angle quotation mark
	case 0x8C:
		return utf8.AppendRune(nil, 'Œ') // Latin capital ligature OE
	case 0x8E:
		return utf8.AppendRune(nil, 'Ž') // Latin capital letter Z with caron
	case 0x91:
		return utf8.AppendRune(nil, '‘') // Left single quotation mark
	case 0x92:
		return utf8.AppendRune(nil, '’') // Right single quotation mark
	case 0x93:
		return utf8.AppendRune(nil, '“') // Left double quotation mark
	case 0x94:
		return utf8.AppendRune(nil, '”') // Right double quotation mark
	case 0x95:
		return utf8.AppendRune(nil, '•') // Bullet
	case 0x96:
		return utf8.AppendRune(nil, '–') // En dash
	case 0x97:
		return utf8.AppendRune(nil, '—') // Em dash
	case 0x98:
		return utf8.AppendRune(nil, '˜') // Small tilde
	case 0x99:
		return utf8.AppendRune(nil, '™') // Trade mark sign
	case 0x9A:
		return utf8.AppendRune(nil, 'š') // Latin small letter s with caron
	case 0x9B:
		return utf8.AppendRune(nil, '›') // Single right-pointing angle quotation mark
	case 0x9C:
		return utf8.AppendRune(nil, 'œ') // Latin small ligature oe
	case 0x9E:
		return utf8.AppendRune(nil, 'ž') // Latin small letter z with caron
	case 0x9F:
		return utf8.AppendRune(nil, 'Ÿ') // Latin capital letter Y with diaeresis
	default:
		// Unknown character, keep as is
		return []byte{b}
	}
}

// convertWithGoEncoding converte bytes CP-1252 para UTF-8 via cp1252ToUTF8.
// Copia em BLOCOS as sequências contíguas de bytes ASCII (b<128, a
// esmagadora maioria de qualquer fonte real — bytes >=128 normalmente só
// aparecem em acentos isolados dentro de comentários/strings) com um único
// buf.Write(source[start:i]) por trecho, em vez de uma chamada de método
// por byte: um arquivo de alguns MB tem só dezenas/centenas de bytes >=128
// mas milhões de bytes ASCII — uma chamada de WriteByte por byte (mesmo
// sendo o método mais barato do bytes.Buffer) ainda paga bounds-check e
// crescimento do buffer a cada chamada, e dominava o profile de um
// arquivo grande mesmo depois da tabela.
func convertWithGoEncoding(source []byte) (string, error) {
	var buf bytes.Buffer
	buf.Grow(len(source) + len(source)/4) // maioria ASCII 1:1; alguns bytes viram 2 bytes UTF-8
	start := 0
	for i, b := range source {
		if b >= 128 {
			if i > start {
				buf.Write(source[start:i])
			}
			buf.Write(cp1252ToUTF8[b])
			start = i + 1
		}
	}
	if start < len(source) {
		buf.Write(source[start:])
	}
	return buf.String(), nil
}

func runFile(sourceFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}

	v := vm.NewVM(bc, opts.uiEnabled)
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

func checkFile(sourceFile string, opts *Options) error {
	_, err := loadAndCompile(sourceFile, opts)
	return err
}

// serveFile executa o programa com a interface renderizada no browser
// (fase 1: console + diálogos). Cada aba/recarga do browser cria uma
// sessão com VM isolada — mesma semântica de work process do StartJob.
func serveFile(sourceFile string, opts *Options) error {
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}
	// bytecode atual atrás de um ponteiro atômico: o --watch (fase 3) troca
	// pela versão recompilada e cada nova sessão executa a mais recente
	var current atomic.Pointer[compiler.Bytecode]
	current.Store(bc)

	srv := webui.New(filepath.Base(sourceFile),
		func(ui *webui.Provider, console *webui.OutWriter) error {
			v := vm.NewVM(current.Load(), true)
			v.SetUIProvider(ui)
			v.SetOutputWriter(console)
			attachDatabase(v, opts)
			_, err := v.Run()
			return err
		})

	if opts.watch {
		go watchSource(sourceFile, opts, &current, srv)
	}

	port := shared.ResolveWebUIPort(opts.port)
	return srv.Serve("localhost:" + port)
}

// watchSource observa o fonte (polling de mtime, sem dependências) e, a cada
// alteração, recompila e manda as sessões do browser recarregarem. Erro de
// compilação vai para o console do browser em vez de recarregar.
func watchSource(sourceFile string, opts *Options, current *atomic.Pointer[compiler.Bytecode], srv *webui.Server) {
	lastMod := time.Time{}
	if info, err := os.Stat(sourceFile); err == nil {
		lastMod = info.ModTime()
	}
	for {
		time.Sleep(500 * time.Millisecond)
		info, err := os.Stat(sourceFile)
		if err != nil || !info.ModTime().After(lastMod) {
			continue
		}
		lastMod = info.ModTime()
		bc, err := loadAndCompile(sourceFile, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "watch: erro de compilação: %v\n", err)
			srv.Broadcast("error", fmt.Sprintf("erro de compilação: %v", err))
			continue
		}
		current.Store(bc)
		fmt.Printf("watch: %s recompilado, recarregando sessões\n", filepath.Base(sourceFile))
		srv.Broadcast("reload", "fonte alterado")
	}
}

// checkFilesParallel verifica N arquivos com um pool de workers (1 por CPU).
// Cada arquivo é compilado de forma independente — o pipeline
// preprocessador→lexer→parser→codegen não compartilha estado entre arquivos.
func checkFilesParallel(files []string, opts *Options) int {
	type result struct {
		file string
		err  error
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
				results <- result{f, checkFile(f, opts)}
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
	attachDatabase(v, opts)
	_, err = v.Run()
	return err
}

func buildStandalone(sourceFile, outputFile string, opts *Options) error {
	// Compile source to bytecode
	bc, err := loadAndCompile(sourceFile, opts)
	if err != nil {
		return err
	}

	// Create temporary directory for build
	tempDir, err := os.MkdirTemp("", "advpp-build-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save bytecode to temp file
	bytecodeFile := filepath.Join(tempDir, "bytecode.json")
	if err := compiler.SaveBytecode(bc, bytecodeFile); err != nil {
		return fmt.Errorf("cannot save bytecode: %v", err)
	}

	// Copy stub to temp directory
	// Use current working directory to find stub template
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %v", err)
	}
	stubPath := filepath.Join(cwd, "pkg/compiler/stub_template.go")

	stubSrc, err := os.ReadFile(stubPath)
	if err != nil {
		return fmt.Errorf("cannot read stub from %s: %v", stubPath, err)
	}
	// Remove the +build ignore line from the stub
	stubLines := strings.Split(string(stubSrc), "\n")
	var cleanStub []string
	for _, line := range stubLines {
		if !strings.Contains(line, "+build ignore") && !strings.Contains(line, "//go:build ignore") {
			cleanStub = append(cleanStub, line)
		}
	}
	stubDst := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(stubDst, []byte(strings.Join(cleanStub, "\n")), 0644); err != nil {
		return fmt.Errorf("cannot write stub: %v", err)
	}

	// Get project root directory
	absPath, err := filepath.Abs(sourceFile)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %v", err)
	}
	projectRoot := filepath.Dir(filepath.Dir(absPath))

	// Create go.mod for the standalone executable
	goModContent := fmt.Sprintf(`module standalone

go 1.24

require github.com/advpl/compiler v0.0.0

replace github.com/advpl/compiler => %s
`, projectRoot)
	goModFile := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModFile, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("cannot write go.mod: %v", err)
	}

	// Build the executable
	cmd := exec.Command("go", "build", "-o", outputFile, ".")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Building in directory: %s\n", tempDir)
	fmt.Printf("Output file: %s\n", outputFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	// Move the executable to the current directory
	finalPath := filepath.Join(".", outputFile)
	if err := os.Rename(filepath.Join(tempDir, outputFile), finalPath); err != nil {
		return fmt.Errorf("cannot move executable: %v", err)
	}

	return nil
}

func printAST(sourceFile string, opts *Options) error {
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}

	includes := opts.includes
	includes = append(includes, filepath.Dir(sourceFile))

	pp := preprocessor.NewPreprocessor(includes)
	processed, err := pp.Process(string(source), sourceFile)
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
  serve     Run the program with the UI rendered in the browser (web mode)
  ast       Print the AST structure
  bytecode  Print the compiled bytecode

Options:
  --include <path>, -I <path>   Add include path (repeatable)
  --define <name=val>, -D       Define preprocessor symbol
  --ui                          Enable UI (Fyne)
  --headless                    Disable UI (default)
  --db <backend>                Database backend: sqlite (default) or odbc
  --db-path <path>              Path to SQLite database file
                                (default: $ADVPP_DB, or the shared AdvPP
                                database configured in ~/.advpp — the same
                                database used by adveditor/advpp-ide)
  -o <file>                     Output file for compile command
  --port <n>                    Web mode port (default: webui_port in
                                ~/.advpp/advpp_config.json, or 8080)
  -w, --watch                   Web mode: recompile on source change and
                                reload browser sessions (hot reload)

Examples:
  advplc run hello.prw
  advplc run ui_demo.prw --ui
  advplc compile hello.prw -o hello.bytecode
  advplc exec hello.bytecode
  advplc check program.prw --include ./includes
  advplc serve program.prw --port 9000 --watch
  advplc bytecode program.prw`)
}
