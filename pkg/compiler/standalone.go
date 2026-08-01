package compiler

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
// the resulting binary to outputFile. title becomes the app window's
// title (falls back to "AdvPP" if empty). buildLog receives the `go
// build` subprocess's combined stdout/stderr (pass os.Stdout for CLI use).
//
// gui marks the program as a desktop app: the stub always opens the Fyne
// window instead of falling back to the terminal UI, and on Windows the
// binary is linked into the GUI subsystem. Without it a Windows build is a
// console executable, so double-clicking it allocates a console, stdin
// looks like a TTY, and the stub picks the terminal UI -- which has no
// MSDIALOG support at all. See BuildStandaloneGUI.
func BuildStandalone(bc *Bytecode, outputFile, title string, buildLog io.Writer) error {
	return BuildStandaloneGUI(bc, outputFile, title, false, buildLog)
}

// BuildStandaloneGUI is BuildStandalone with the desktop-app switch.
func BuildStandaloneGUI(bc *Bytecode, outputFile, title string, gui bool, buildLog io.Writer) error {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return err
	}
	if title == "" {
		title = "AdvPP"
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
	// repo doesn't try to compile the template as part of pkg/compiler),
	// and substitute the window-title placeholder — a plain string
	// replace is enough since title only ever comes from a sanitized
	// filename (see cmd/advplc's buildStandalone), never arbitrary input.
	var cleanStub []string
	for _, line := range strings.Split(stubTemplate, "\n") {
		if strings.Contains(line, "+build ignore") || strings.Contains(line, "//go:build ignore") {
			continue
		}
		line = strings.ReplaceAll(line, "__ADVPP_APP_TITLE__", title)
		line = strings.ReplaceAll(line, "__ADVPP_BUILT_AS_GUI__", strconv.FormatBool(gui))
		cleanStub = append(cleanStub, line)
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
	cmd := exec.Command("go", goBuildArgs(tempOutput, gui, targetGOOS())...)
	cmd.Dir = tempDir
	cmd.Stdout = buildLog
	cmd.Stderr = buildLog
	// The temp go.mod only declares standaloneModule itself; its transitive
	// deps (Fyne, sqlite, etc.) aren't listed as indirect requires yet, and
	// there's no go.sum. -mod=mod lets `go build` complete the module graph
	// and write go.sum itself instead of failing and asking for a separate
	// `go mod tidy` first — everything it needs is normally already in the
	// local module cache (this compiler already depends on the same
	// packages), so this shouldn't need network access in practice.
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	if err := moveFile(filepath.Join(tempDir, tempOutput), outputFile); err != nil {
		return fmt.Errorf("cannot move executable: %v", err)
	}
	return nil
}

// goBuildArgs assembles the `go build` argv. -H=windowsgui puts the binary
// in the Windows GUI subsystem: no console window behind the app, and no
// console handles either -- which is what makes the stub's
// IsTerminal(os.Stdin) check answer false and route to Fyne. It means
// nothing on other platforms, so it only goes in where it applies.
func goBuildArgs(output string, gui bool, goos string) []string {
	args := []string{"build", "-o", output}
	if gui && goos == "windows" {
		args = append(args, "-ldflags", "-H=windowsgui")
	}
	return append(args, ".")
}

// targetGOOS is the platform `go build` will emit for: the GOOS env var
// when the caller cross-compiles, otherwise this process's own platform.
func targetGOOS() string {
	if goos := os.Getenv("GOOS"); goos != "" {
		return goos
	}
	return runtime.GOOS
}

// moveFile moves src to dst, falling back to copy+remove when they're on
// different volumes — os.Rename fails with "cannot move the file to a
// different disk drive" on Windows in that case (e.g. the OS temp
// directory and the caller's working directory are commonly on different
// drives on GitHub Actions' windows-latest runners), and returns EXDEV on
// Unix for the equivalent cross-filesystem case.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}

	info, err := in.Stat()
	if err != nil {
		in.Close()
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		in.Close()
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	// src's handle must be closed before removing it — Windows (unlike
	// POSIX) refuses to delete a file that still has an open handle, so
	// this can't be a deferred Close() running after the return.
	in.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Remove(src)
}
