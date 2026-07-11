package compiler

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed stub_template.go
var stubTemplate string

// standaloneModule is the module path declared in this repo's go.mod — the
// generated standalone build's go.mod replaces it with a local checkout so
// `go build` doesn't need this module published anywhere.
const standaloneModule = "github.com/advpl/compiler"

// findModuleRoot locates a local checkout of this compiler's own module
// (the one containing stub_template.go's real go.mod), searching upward
// from both the current working directory and the running executable's
// directory. Building a standalone executable always needs this: the
// generated stub imports pkg/compiler and pkg/vm from this module, and
// standaloneModule isn't (yet) published anywhere `go build` could fetch
// it from directly.
func findModuleRoot() (string, error) {
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if env := os.Getenv("ADVPP_SRC"); env != "" {
		candidates = append([]string{env}, candidates...)
	}

	for _, start := range candidates {
		if root := walkUpForModule(start); root != "" {
			return root, nil
		}
	}
	return "", fmt.Errorf(
		"não encontrei um checkout local de %s (necessário para compilar um executável standalone) — "+
			"rode a partir de dentro do repositório AdvPP, ou defina a variável de ambiente ADVPP_SRC apontando para ele",
		standaloneModule,
	)
}

func walkUpForModule(start string) string {
	dir := start
	for {
		modFile := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modFile); err == nil {
			if strings.Contains(string(data), "module "+standaloneModule) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// BuildStandalone compiles already-produced bytecode into a native,
// standalone executable: it embeds the bytecode into a copy of
// stub_template.go, builds it in a temporary Go module that replaces
// standaloneModule with a local checkout (see findModuleRoot), and moves
// the resulting binary to outputFile. buildLog receives the `go build`
// subprocess's combined stdout/stderr (pass os.Stdout for CLI use).
func BuildStandalone(bc *Bytecode, outputFile string, buildLog io.Writer) error {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "advpp-build-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bytecodeFile := filepath.Join(tempDir, "bytecode.json")
	if err := SaveBytecode(bc, bytecodeFile); err != nil {
		return fmt.Errorf("cannot save bytecode: %v", err)
	}

	// Remove the +build ignore line (present so `go build ./...` in this
	// repo doesn't try to compile the template as part of pkg/compiler).
	var cleanStub []string
	for _, line := range strings.Split(stubTemplate, "\n") {
		if !strings.Contains(line, "+build ignore") && !strings.Contains(line, "//go:build ignore") {
			cleanStub = append(cleanStub, line)
		}
	}
	stubDst := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(stubDst, []byte(strings.Join(cleanStub, "\n")), 0644); err != nil {
		return fmt.Errorf("cannot write stub: %v", err)
	}

	goModContent := fmt.Sprintf(`module standalone

go 1.24

require %s v0.0.0

replace %s => %s
`, standaloneModule, standaloneModule, moduleRoot)
	goModFile := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModFile, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("cannot write go.mod: %v", err)
	}

	// Building with -o set to the caller's own basename (not a fixed name)
	// means the caller decides the extension (e.g. ".exe" on Windows) —
	// same convention advplc's CLI callers already use.
	tempOutput := filepath.Base(outputFile)
	cmd := exec.Command("go", "build", "-o", tempOutput, ".")
	cmd.Dir = tempDir
	cmd.Stdout = buildLog
	cmd.Stderr = buildLog
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	if err := os.Rename(filepath.Join(tempDir, tempOutput), outputFile); err != nil {
		return fmt.Errorf("cannot move executable: %v", err)
	}
	return nil
}
