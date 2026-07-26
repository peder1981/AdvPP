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

func main() {
	elog := func(msg string) {
		fmt.Fprintf(os.Stderr, "[ADVPP] %s\n", msg)
	}

	defer func() {
		if r := recover(); r != nil {
			elog(fmt.Sprintf("PANIC: %v", r))
		}
	}()

	// STEP 1: Load bytecode
	var bc compiler.Bytecode
	if err := json.Unmarshal(bytecodeData, &bc); err != nil {
		elog("FATAL: cannot unmarshal bytecode: " + err.Error())
		os.Exit(1)
	}

	// STEP 2: Detect if program needs UI.
	// Console-only programs (no MSDIALOG/FWMBrowse/etc calls) should run
	// headless and exit immediately without opening a Fyne window.
	// The test fixture standalone_console_test.prw is 100% console.
	hasUI := false
	for _, fn := range bc.Functions {
		for _, an := range fn.Annotations {
			// Annotations like @Get/@Post indicate web UI routes
			if an.Name == "Get" || an.Name == "Post" || an.Name == "Put" || an.Name == "Patch" || an.Name == "Delete" {
				hasUI = true
			}
		}
	}
	// Also check if bytecode references MVC classes or framework UI classes
	if !hasUI {
		for className := range bc.Classes {
			if className == "FWFORMVIEW" || className == "FWFORMMODEL" || className == "FWFORMBROWSE" ||
				className == "FWGRIDPROCESS" || className == "LLM" || className == "MCPSERVER" ||
				className == "WSRESTSERVER" || className == "TMAILMESSAGE" || className == "TENSOR" ||
				className == "VARIABLE" || className == "SGD" || className == "ADAM" ||
				className == "LINEAR" || className == "EMBEDDING" || className == "JSONOBJECT" {
				hasUI = true
				break
			}
		}
	}

	if !hasUI || os.Getenv("ADVPP_HEADLESS_STANDALONE") != "" {
		// Run headless — no GUI needed, just execute and exit
		v := vm.NewVM(&bc, false)
		v.SetDBFactory(func() vm.DBEngine {
			dbPath := shared.ResolveDatabasePath("")
			engine, err := db.NewSQLiteEngine(dbPath)
			if err != nil {
				return nil
			}
			return engine
		})
		_, err := v.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// STEP 3: GUI mode — create Fyne window and run with UI support
	a := app.New()
	a.Settings().SetTheme(ui.NewTheme())
	w := a.NewWindow("__ADVPP_APP_TITLE__")
	w.Resize(fyne.NewSize(720, 480))
	w.CenterOnScreen()

	header := widget.NewLabelWithStyle("__ADVPP_APP_TITLE__", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header.Alignment = fyne.TextAlignLeading

	console := ui.NewOutputConsole()
	w.SetContent(container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, console.GetWidget()))

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

	done := make(chan int, 1)
	go func() {
		code := 1
		_, err := v.Run()
		if err != nil {
			console.Append("Runtime error: " + err.Error())
		} else {
			code = 0
		}
		w.Close() // Signals ShowAndRun to return
		done <- code
	}()

	w.ShowAndRun()
	exitCode := <-done
	os.Exit(exitCode)
}
