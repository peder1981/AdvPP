package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	fd.Show()
}

func (ide *IDE) compile() {
	source := ide.editor.GetContent()
	filename := ide.editor.GetFilename()

	if filename == "" || filename == "untitled.prw" {
		ide.output.Append("Error: Please save the file first")
		return
	}

	ide.output.Append("Compiling: " + filename)

	// Preprocess
	includes := []string{filepath.Dir(filename)}
	pp := preprocessor.NewPreprocessor(includes)
	processed, err := pp.Process(source, filename)
	if err != nil {
		ide.output.Append("Preprocessor error: " + err.Error())
		return
	}

	// Lex
	tokens, err := lexer.Tokenize(processed, filename)
	if err != nil {
		ide.output.Append("Lexer error: " + err.Error())
		return
	}

	// Parse
	p := parser.NewParser(tokens, filename, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		ide.output.Append("Parser error: " + err.Error())
		return
	}

	// Compile
	bc, err := compiler.Compile(prog)
	if err != nil {
		ide.output.Append("Compiler error: " + err.Error())
		return
	}

	ide.output.Append("Compilation successful")
	ide.output.Append(fmt.Sprintf("Functions: %d, Classes: %d", len(bc.Functions), len(bc.Classes)))
}

func (ide *IDE) run() {
	source := ide.editor.GetContent()
	filename := ide.editor.GetFilename()

	if filename == "" || filename == "untitled.prw" {
		ide.output.Append("Error: Please save the file first")
		return
	}

	ide.output.Append("Running: " + filename)

	// Compile first
	includes := []string{filepath.Dir(filename)}
	pp := preprocessor.NewPreprocessor(includes)
	processed, err := pp.Process(source, filename)
	if err != nil {
		ide.output.Append("Preprocessor error: " + err.Error())
		return
	}

	tokens, err := lexer.Tokenize(processed, filename)
	if err != nil {
		ide.output.Append("Lexer error: " + err.Error())
		return
	}

	p := parser.NewParser(tokens, filename, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		ide.output.Append("Parser error: " + err.Error())
		return
	}

	bc, err := compiler.Compile(prog)
	if err != nil {
		ide.output.Append("Compiler error: " + err.Error())
		return
	}

	// Run VM
	v := vm.NewVM(bc, true)
	ide.attachDatabase(v)

	// Set UI provider for dialog functions
	uiProvider := ui.NewFyneUIProvider(ide.window)
	v.SetUIProvider(uiProvider)

	result, err := v.Run()
	if err != nil {
		ide.output.Append("Runtime error: " + err.Error())
		return
	}

	ide.output.Append("Execution completed")
	if result != nil {
		ide.output.Append("Result: " + fmt.Sprintf("%v", result))
	}
}

func (ide *IDE) compileAndRun() {
	ide.compile()
	ide.run()
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
