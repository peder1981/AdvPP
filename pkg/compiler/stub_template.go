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
	var bc compiler.Bytecode
	if err := json.Unmarshal(bytecodeData, &bc); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bytecode: %v\n", err)
		os.Exit(1)
	}

	a := app.New()
	w := a.NewWindow("AdvPP")
	w.Resize(fyne.NewSize(800, 500))

	console := ui.NewOutputConsole()
	w.SetContent(console.GetWidget())
	w.Show()

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

	exitCode := 0
	go func() {
		if _, err := v.Run(); err != nil {
			console.Append("Runtime error: " + err.Error())
			exitCode = 1
			return
		}
		a.Quit()
	}()

	w.ShowAndRun()
	os.Exit(exitCode)
}
