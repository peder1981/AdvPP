// Stub template for standalone executable generation
// This file is embedded by the compiler when building standalone executables
// +build ignore

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/ui"
	"github.com/advpl/compiler/pkg/vm"
)

//go:embed bytecode.json
var bytecodeData []byte

// The window doubles as both the console (so ConOut output is visible at
// all on Windows, where a GUI-subsystem binary has no attached terminal —
// otherwise console-only programs would produce no visible output at all)
// and the dialog parent for MsgInfo/MSDIALOG/FWMBrowse, so those work the
// same way in a standalone build as they do in advpp-ide.
func main() {
	trace := os.Getenv("ADVPP_STUB_TRACE") != ""
	tlog := func(msg string) {
		if trace {
			fmt.Fprintln(os.Stderr, "ADVPP_STUB_TRACE: "+msg)
		}
	}

	tlog("start")
	var bc compiler.Bytecode
	if err := json.Unmarshal(bytecodeData, &bc); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bytecode: %v\n", err)
		os.Exit(1)
	}
	tlog("bytecode loaded")

	a := app.New()
	a.Settings().SetTheme(ui.NewTheme())
	tlog("app.New done")
	w := a.NewWindow("__ADVPP_APP_TITLE__")
	tlog("NewWindow done")
	w.Resize(fyne.NewSize(720, 480))
	w.CenterOnScreen()

	// Cabeçalho com o título do app: sem isso, a janela de base (por trás
	// de qualquer diálogo/menu) era um console preto vazio — sensação de
	// tela quebrada/desproporcional relatada em uso real, principalmente
	// com um diálogo pequeno flutuando no meio de muito espaço em branco.
	header := widget.NewLabelWithStyle("__ADVPP_APP_TITLE__", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header.Alignment = fyne.TextAlignLeading

	console := ui.NewOutputConsole()
	w.SetContent(container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, console.GetWidget()))
	tlog("content set")

	v := vm.NewVM(&bc, true)
	v.SetOutputWriter(ui.NewConsoleWriter(console))
	v.SetUIProvider(ui.NewFyneUIProvider(w))

	dbPath := shared.ResolveDatabasePath("")
	v.SetDBFactory(func() vm.DBEngine {
		engine, err := db.NewSQLiteEngine(dbPath)
		if err != nil {
			console.Append("Database warning: " + err.Error())
			return nil
		}
		return engine
	})

	// Fyne requires ShowAndRun() to be called from the main OS thread.
	// v.Run() must execute while the event loop is running so UI calls
	// (FWMenuSelect, FWGetText) can display dialogs. Synchronize:
	// 1. Start v.Run() in a goroutine
	// 2. Call ShowAndRun() on main thread to run the event loop
	// 3. v.Run() can now make UI calls that ShowAndRun()'s loop will handle
	// 4. When v.Run() completes, call a.Quit() to close the window
	// 5. ShowAndRun() returns, and we exit
	done := make(chan int)
	go func() {
		exitCode := 1
		tlog("v.Run starting in goroutine")
		if _, err := v.Run(); err != nil {
			tlog("v.Run returned error: " + err.Error())
			console.Append("Runtime error: " + err.Error())
		} else {
			tlog("v.Run completed successfully")
			exitCode = 0
		}
		// Signal the window to close, which will return ShowAndRun()
		tlog("calling a.Quit()")
		a.Quit()
		done <- exitCode
	}()

	// Run ShowAndRun on main thread (Fyne requirement).
	// While this blocks, the event loop processes UI calls from v.Run()
	// goroutine. When a.Quit() is called above, ShowAndRun() returns.
	tlog("calling ShowAndRun (will block until a.Quit)")
	w.ShowAndRun()
	tlog("ShowAndRun returned")

	// Wait for v.Run() goroutine to finish and get exit code
	exitCode := <-done
	tlog("exiting with code: " + fmt.Sprintf("%d", exitCode))
	os.Exit(exitCode)
}
