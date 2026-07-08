package main

import (
	"fmt"
	"os"
	"path/filepath"

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

	w := a.NewWindow("AdvPP IDE - AdvPL/TLPP Development Environment")
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

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			ide.showAboutDialog()
		}),
	)

	return fyne.NewMainMenu(fileMenu, editMenu, buildMenu, viewMenu, helpMenu)
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

	// Conecta ao banco SQLite compartilhado entre todas as ferramentas AdvPP
	dbPath := shared.ResolveDatabasePath("")
	if _, statErr := os.Stat(dbPath); statErr == nil {
		if engine, dbErr := db.NewSQLiteEngine(dbPath); dbErr == nil {
			v.SetDBEngine(engine)
			ide.output.Append("Database: " + dbPath)
		} else {
			ide.output.Append("Database warning: " + dbErr.Error())
		}
	}

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

func (ide *IDE) showAboutDialog() {
	content := widget.NewLabel("AdvPP IDE\n\nAdvPL/TLPP Compiler and Development Environment\n\nVersion 1.0.0\n\nA fully functional compiler and interpreter for the AdvPL and TLPP programming languages.")
	dialog.ShowInformation("About AdvPP IDE", content.Text, ide.window)
}
