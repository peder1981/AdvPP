package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/vm"
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
		sourceFile := os.Args[2]
		opts := parseOptions(os.Args[3:])
		if err := checkFile(sourceFile, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK: syntax check passed")

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

// convertWithGoEncoding uses golang.org/x/text/encoding for conversion
func convertWithGoEncoding(source []byte) (string, error) {
	// Simple CP1252 to UTF-8 conversion
	// CP1252 is a superset of ISO-8859-1 with additional characters in range 0x80-0x9F
	var buf bytes.Buffer
	for _, b := range source {
		if b < 128 {
			buf.WriteByte(b)
		} else if b >= 160 && b <= 255 {
			// ISO-8859-1 / CP1252 range (maps directly to Unicode)
			buf.WriteRune(rune(b))
		} else {
			// CP1252 specific characters in range 0x80-0x9F
			switch b {
			case 0x80:
				buf.WriteRune('€') // Euro sign
			case 0x82:
				buf.WriteRune('‚') // Single low-9 quotation mark
			case 0x83:
				buf.WriteRune('ƒ') // Latin small letter f with hook
			case 0x84:
				buf.WriteRune('„') // Double low-9 quotation mark
			case 0x85:
				buf.WriteRune('…') // Horizontal ellipsis
			case 0x86:
				buf.WriteRune('†') // Dagger
			case 0x87:
				buf.WriteRune('‡') // Double dagger
			case 0x88:
				buf.WriteRune('ˆ') // Modifier letter circumflex accent
			case 0x89:
				buf.WriteRune('‰') // Per mille sign
			case 0x8A:
				buf.WriteRune('Š') // Latin capital letter S with caron
			case 0x8B:
				buf.WriteRune('‹') // Single left-pointing angle quotation mark
			case 0x8C:
				buf.WriteRune('Œ') // Latin capital ligature OE
			case 0x8E:
				buf.WriteRune('Ž') // Latin capital letter Z with caron
			case 0x91:
				buf.WriteRune('‘') // Left single quotation mark
			case 0x92:
				buf.WriteRune('’') // Right single quotation mark
			case 0x93:
				buf.WriteRune('“') // Left double quotation mark
			case 0x94:
				buf.WriteRune('”') // Right double quotation mark
			case 0x95:
				buf.WriteRune('•') // Bullet
			case 0x96:
				buf.WriteRune('–') // En dash
			case 0x97:
				buf.WriteRune('—') // Em dash
			case 0x98:
				buf.WriteRune('˜') // Small tilde
			case 0x99:
				buf.WriteRune('™') // Trade mark sign
			case 0x9A:
				buf.WriteRune('š') // Latin small letter s with caron
			case 0x9B:
				buf.WriteRune('›') // Single right-pointing angle quotation mark
			case 0x9C:
				buf.WriteRune('œ') // Latin small ligature oe
			case 0x9E:
				buf.WriteRune('ž') // Latin small letter z with caron
			case 0x9F:
				buf.WriteRune('Ÿ') // Latin capital letter Y with diaeresis
			default:
				// Unknown character, keep as is
				buf.WriteByte(b)
			}
		}
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
// ferramentas AdvPP (mesmo resolver usado por advcfg/adveditor/advpp-ide).
// Sem banco existente, o VM roda com os stubs — comportamento anterior.
func attachDatabase(v *vm.VM, opts *Options) {
	dbPath := shared.ResolveDatabasePath(opts.dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		if opts.dbPath != "" {
			fmt.Fprintf(os.Stderr, "Warning: database not found: %s\n", dbPath)
		}
		return
	}
	engine, err := db.NewSQLiteEngine(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot open database %s: %v\n", dbPath, err)
		return
	}
	v.SetDBEngine(engine)
}

func checkFile(sourceFile string, opts *Options) error {
	_, err := loadAndCompile(sourceFile, opts)
	return err
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
  check     Validate syntax without executing
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
                                database used by advcfg/adveditor/advpp-ide)
  -o <file>                     Output file for compile command

Examples:
  advplc run hello.prw
  advplc run ui_demo.prw --ui
  advplc compile hello.prw -o hello.bytecode
  advplc exec hello.bytecode
  advplc check program.prw --include ./includes
  advplc bytecode program.prw`)
}
