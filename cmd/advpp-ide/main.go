package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/ui"
	"github.com/advpl/compiler/pkg/vm"
)

// version é injetada no build via -ldflags "-X main.version=v1.2.3" (make release).
var version = "dev"

type IDE struct {
	window   fyne.Window
	editor   *ui.CodeEditor
	output   *ui.OutputConsole
	fileTree *ui.FileTree
	app      fyne.App
}

func main() {
	a := app.New()
	a.SetIcon(nil)

	ide := &IDE{
		app: a,
	}

	w := a.NewWindow(fmt.Sprintf("AdvPP IDE %s - AdvPL/TLPP Development Environment", version))
	w.SetMainMenu(ide.makeMainMenu())
	w.Resize(fyne.NewSize(1200, 800))

	ide.window = w

	// Create IDE components
	ide.editor = ui.NewCodeEditor()
	ide.output = ui.NewOutputConsole()
	ide.fileTree = ui.NewFileTree()

	// Main layout: split left (file tree), center (editor), bottom (output)
	leftSplit := container.NewHSplit(ide.fileTree.GetWidget(), ide.editor.GetWidget())
	leftSplit.SetOffset(0.2)

	mainSplit := container.NewVSplit(leftSplit, ide.output.GetWidget())
	mainSplit.SetOffset(0.8)

	w.SetContent(mainSplit)
	w.ShowAndRun()
}

func (ide *IDE) makeMainMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New", func() {
			ide.newFile()
		}),
		fyne.NewMenuItem("Open", func() {
			ide.openFile()
		}),
		fyne.NewMenuItem("Save", func() {
			ide.saveFile()
		}),
		fyne.NewMenuItem("Save As", func() {
			ide.saveFileAs()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", func() {
			ide.window.Close()
		}),
	)

	editMenu := fyne.NewMenu("Edit",
		fyne.NewMenuItem("Cut", func() {
			// TODO: Implement cut
		}),
		fyne.NewMenuItem("Copy", func() {
			// TODO: Implement copy
		}),
		fyne.NewMenuItem("Paste", func() {
			// TODO: Implement paste
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Find", func() {
			// TODO: Implement find
		}),
		fyne.NewMenuItem("Replace", func() {
			// TODO: Implement replace
		}),
	)

	buildMenu := fyne.NewMenu("Build",
		fyne.NewMenuItem("Compile", func() {
			ide.compile()
		}),
		fyne.NewMenuItem("Run", func() {
			ide.run()
		}),
		fyne.NewMenuItem("Compile and Run", func() {
			ide.compileAndRun()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Build standalone executable...", func() {
			ide.buildStandalone()
		}),
	)

	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItem("Toggle File Tree", func() {
			// TODO: Implement toggle file tree
		}),
		fyne.NewMenuItem("Toggle Output", func() {
			// TODO: Implement toggle output
		}),
	)

	toolsMenu := fyne.NewMenu("Tools",
		fyne.NewMenuItem("Open AdvEditor (database)", func() {
			ide.openAdvEditor()
		}),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			ide.showAboutDialog()
		}),
	)

	return fyne.NewMainMenu(fileMenu, editMenu, buildMenu, viewMenu, toolsMenu, helpMenu)
}

// openAdvEditor launches the AdvEditor GUI as a separate process, looking
// first next to the running advpp-ide binary (same install/dist layout as
// the release packages) and falling back to PATH — same database-tool
// pairing already offered by advplc/adveditor via the shared local
// ./advpp.db convention (see attachDatabase).
func (ide *IDE) openAdvEditor() {
	path, err := adveditorPath()
	if err != nil {
		dialog.ShowError(err, ide.window)
		return
	}

	cmd := exec.Command(path)
	cmd.Dir, _ = os.Getwd()
	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("não foi possível iniciar o AdvEditor: %w", err), ide.window)
		return
	}
	ide.output.Append("AdvEditor iniciado: " + path)
}

func adveditorPath() (string, error) {
	name := "adveditor"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("binário %q não encontrado ao lado de advpp-ide nem no PATH", name)
}

func (ide *IDE) newFile() {
	ide.editor.SetContent("")
	ide.editor.SetFilename("untitled.prw")
	ide.editor.SetModified(false)
	ide.output.Append("New file created")
}

func (ide *IDE) openFile() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ide.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		data := make([]byte, 0)
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			data = append(data, buf[:n]...)
		}

		ide.editor.SetContent(string(data))
		ide.editor.SetFilename(reader.URI().Path())
		ide.editor.SetModified(false)
		ide.output.Append("Opened: " + reader.URI().Path())
	}, ide.window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".prw", ".tlpp", ".prg"}))
	if loc := ui.CurrentDirLocation(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

func (ide *IDE) saveFile() {
	if ide.editor.GetFilename() == "" || ide.editor.GetFilename() == "untitled.prw" {
		ide.saveFileAs()
		return
	}

	err := os.WriteFile(ide.editor.GetFilename(), []byte(ide.editor.GetContent()), 0644)
	if err != nil {
		dialog.ShowError(err, ide.window)
		return
	}

	ide.editor.SetModified(false)
	ide.output.Append("Saved: " + ide.editor.GetFilename())
}

func (ide *IDE) saveFileAs() {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ide.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		_, err = writer.Write([]byte(ide.editor.GetContent()))
		if err != nil {
			dialog.ShowError(err, ide.window)
			return
		}

		ide.editor.SetFilename(writer.URI().Path())
		ide.editor.SetModified(false)
		ide.output.Append("Saved: " + writer.URI().Path())
	}, ide.window)
	fd.SetFileName("untitled.prw")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".prw", ".tlpp", ".prg"}))
	if loc := ui.CurrentDirLocation(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// loadAndCompile runs the preprocess/lex/parse/compile pipeline over the
// editor's current content, shared by compile/run/buildStandalone so they
// stay in sync instead of drifting as three separate copies.
func (ide *IDE) loadAndCompile() (*compiler.Bytecode, string, error) {
	source := ide.editor.GetContent()
	filename := ide.editor.GetFilename()

	if filename == "" || filename == "untitled.prw" {
		return nil, filename, fmt.Errorf("please save the file first")
	}

	includes := []string{filepath.Dir(filename)}
	pp := preprocessor.NewPreprocessor(includes)
	processed, err := pp.Process(source, filename)
	if err != nil {
		return nil, filename, fmt.Errorf("preprocessor error: %w", err)
	}

	tokens, err := lexer.Tokenize(processed, filename)
	if err != nil {
		return nil, filename, fmt.Errorf("lexer error: %w", err)
	}

	p := parser.NewParser(tokens, filename, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return nil, filename, fmt.Errorf("parser error: %w", err)
	}

	bc, err := compiler.Compile(prog)
	if err != nil {
		return nil, filename, fmt.Errorf("compiler error: %w", err)
	}

	return bc, filename, nil
}

// bytecodeFilename derives the .bytecode output path for a source file,
// e.g. "foo.prw" -> "foo.bytecode" — same format advplc's own "compile"
// command writes (compiler.SaveBytecode), loadable via "advplc run" or via
// this IDE's own Run button without recompiling from source.
func bytecodeFilename(sourceFile string) string {
	ext := filepath.Ext(sourceFile)
	return strings.TrimSuffix(sourceFile, ext) + ".bytecode"
}

func (ide *IDE) compile() {
	bc, filename, err := ide.loadAndCompile()
	if err != nil {
		ide.output.Append("Error: " + err.Error())
		return
	}

	ide.output.Append("Compiling: " + filename)

	outputFile := bytecodeFilename(filename)
	if err := compiler.SaveBytecode(bc, outputFile); err != nil {
		ide.output.Append("Error saving bytecode: " + err.Error())
		return
	}

	ide.output.Append("Compilation successful: " + outputFile)
	ide.output.Append(fmt.Sprintf("Functions: %d, Classes: %d", len(bc.Functions), len(bc.Classes)))
}

func (ide *IDE) run() {
	bc, filename, err := ide.loadAndCompile()
	if err != nil {
		ide.output.Append("Error: " + err.Error())
		return
	}

	ide.output.Append("Running: " + filename)

	// Run VM
	v := vm.NewVM(bc, true)
	ide.attachDatabase(v)

	// Set UI provider for dialog functions
	uiProvider := ui.NewFyneUIProvider(ide.window)
	v.SetUIProvider(uiProvider)
	// Route ConOut/console writes into the IDE's own Output pane instead of
	// the process's real stdout, which a packaged GUI app has no visible
	// terminal for.
	v.SetOutputWriter(&consoleWriter{console: ide.output})

	// v.Run() must NOT execute on the Fyne main/event goroutine: MSDIALOG
	// (ui.FyneUIProvider.Dialog) blocks its calling goroutine until the
	// user clicks a button, and that click is itself only processed by
	// the main goroutine's event loop — calling v.Run() directly here
	// would deadlock the whole window the moment a program opens a
	// MSDIALOG. Running it on its own goroutine keeps the event loop free
	// to process the dialog's own button clicks that unblock it.
	go func() {
		result, err := v.Run()
		if err != nil {
			ide.output.Append("Runtime error: " + err.Error())
			return
		}

		ide.output.Append("Execution completed")
		if result != nil {
			ide.output.Append("Result: " + fmt.Sprintf("%v", result))
		}
	}()
}

func (ide *IDE) compileAndRun() {
	ide.compile()
	ide.run()
}

// buildStandalone compiles the current file into a native, standalone
// executable via compiler.BuildStandalone — same mechanism as `advplc
// build`. This only works when advpp-ide is running from within (or
// pointed at, via ADVPP_SRC) a full AdvPP source checkout with the Go
// toolchain installed, since the generated stub imports pkg/compiler and
// pkg/vm from this module, which isn't published anywhere `go build`
// could otherwise fetch it from — not something a plain downloaded
// release package alone can satisfy.
func (ide *IDE) buildStandalone() {
	bc, filename, err := ide.loadAndCompile()
	if err != nil {
		ide.output.Append("Error: " + err.Error())
		return
	}

	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ide.window)
			return
		}
		if writer == nil {
			return
		}
		outputFile := writer.URI().Path()
		writer.Close()
		os.Remove(outputFile) // BuildStandalone creates it itself via os.Rename

		ide.output.Append("Building standalone executable: " + outputFile)
		// go build can take a while (real subprocess compiling a Fyne
		// binary) — run off the UI goroutine so the window stays
		// responsive instead of appearing frozen for its duration.
		go func() {
			logWriter := &consoleWriter{console: ide.output}
			if err := compiler.BuildStandalone(bc, outputFile, logWriter); err != nil {
				ide.output.Append("Build error: " + err.Error())
				return
			}
			ide.output.Append("Standalone executable built: " + outputFile)
		}()
	}, ide.window)
	fd.SetFileName(name)
	if loc := ui.CurrentDirLocation(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// attachDatabase conecta o VM ao banco SQLite compartilhado entre todas as
// ferramentas AdvPP (mesmo padrão de advplc/adveditor): o banco é sempre
// anexado via fábrica, mesmo que o arquivo ainda não exista — o driver cria
// o arquivo no primeiro open. ResolveDatabasePath já resolve para um banco
// local (./advpp.db) do diretório de trabalho atual quando nada foi
// configurado globalmente, então tabelas criadas via "Tools > Open
// AdvEditor" ficam visíveis aqui sem configuração extra.
func (ide *IDE) attachDatabase(v *vm.VM) {
	dbPath := shared.ResolveDatabasePath("")
	v.SetDBFactory(func() vm.DBEngine {
		engine, err := db.NewSQLiteEngine(dbPath)
		if err != nil {
			ide.output.Append("Database warning: " + err.Error())
			return nil
		}
		return engine
	})
	ide.output.Append("Database: " + dbPath)
}

func (ide *IDE) showAboutDialog() {
	content := widget.NewLabel(fmt.Sprintf(
		"AdvPP IDE\n\nAdvPL/TLPP Compiler and Development Environment\n\nVersion %s\n\nA fully functional compiler and interpreter for the AdvPL and TLPP programming languages.\nDatabase, LLM inference engine, MCP server and #command engine included.",
		version,
	))
	dialog.ShowInformation("About AdvPP IDE", content.Text, ide.window)
}

// consoleWriter adapts ui.OutputConsole to io.Writer, splitting arbitrary
// writes (e.g. a `go build` subprocess's combined stdout/stderr) into
// line-based Append calls; a partial trailing line is buffered until the
// next Write completes it.
type consoleWriter struct {
	console *ui.OutputConsole
	partial string
}

func (w *consoleWriter) Write(p []byte) (int, error) {
	w.partial += string(p)
	lines := strings.Split(w.partial, "\n")
	for _, line := range lines[:len(lines)-1] {
		w.console.Append(line)
	}
	w.partial = lines[len(lines)-1]
	return len(p), nil
}
