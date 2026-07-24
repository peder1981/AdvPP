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

	go func() {
		tlog("v.Run starting")
		if _, err := v.Run(); err != nil {
			tlog("v.Run returned error: " + err.Error())
			console.Append("Runtime error: " + err.Error())
			// Leave the window open on error so the user can see it —
			// ShowAndRun below keeps blocking until they close it manually.
			return
		}
		// os.Exit terminates the process immediately and unconditionally,
		// on whatever goroutine calls it — used here instead of a.Quit()
		// because a.Quit() (close a done channel the glfw event loop's
		// select watches for) was observed to not reliably unblock
		// ShowAndRun() on Windows: the call itself returns fine, but the
		// event loop never notices and the process hangs forever needing
		// a manual kill. A short-lived standalone script doesn't need
		// graceful window teardown — it just needs to actually terminate.
		tlog("v.Run returned ok, exiting")
		os.Exit(0)
	}()

	tlog("calling ShowAndRun")
	w.ShowAndRun()
	tlog("ShowAndRun returned")
	os.Exit(1)
}
