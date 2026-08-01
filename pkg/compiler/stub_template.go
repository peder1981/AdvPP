// Stub template for standalone executable generation
// This file is embedded by the compiler when building standalone executables
// +build ignore

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	// Antes de tudo: se ha um Mesa3D instalado ao lado deste executavel, o
	// programa se relanca com o driver llvmpipe fixado. Sem isso o Mesa tenta
	// o d3d12 primeiro e, em VM, morre com 0x80070057 depois de ja ter
	// desenhado a janela. Nao retorna quando relanca.
	shared.ForcaMesaPorSoftware()

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

	// STEP 2: Detect whether the program has any interactive UI need at
	// all. The previous version of this check scanned bc.Classes for
	// FWFORMBROWSE/FWFORMVIEW/etc, but bc.Classes only holds
	// user-*declared* classes (`class X from Y`); a program that just
	// calls `FWMBrowse():New()` — every real FWMBrowse call site, e-Gov
	// included — instantiates a builtin class directly (OP_NEW_INSTANCE)
	// and never touches bc.Classes at all. So the heuristic silently
	// never fired for the single most common case, and FWMBrowse always
	// hit "requer o modo web (advplc serve)" in standalone builds no
	// matter what the bytecode actually did. This scans the real
	// instructions instead: OP_CALL_NATIVE for FWGetText/FWMenuSelect,
	// OP_NEW_INSTANCE for FWMBrowse/MSDIALOG-family builtin classes.
	hasUI := false
	uiNatives := map[string]bool{"FWGETTEXT": true, "FWMENUSELECT": true}
	uiClasses := map[string]bool{
		"FWMBROWSE": true, "FWFORMBROWSE": true, "FWFORMVIEW": true, "FWFORMMODEL": true,
		"FWGRIDPROCESS": true, "MSDIALOG": true,
	}
	for _, instr := range bc.Code {
		switch instr.Op {
		case compiler.OP_CALL_NATIVE:
			if uiNatives[strings.ToUpper(instr.Str)] {
				hasUI = true
			}
		case compiler.OP_NEW_INSTANCE:
			if uiClasses[strings.ToUpper(instr.Str)] {
				hasUI = true
			}
		}
		if hasUI {
			break
		}
	}
	// Annotations like @Get/@Post indicate web UI routes.
	if !hasUI {
	annotationScan:
		for _, fn := range bc.Functions {
			for _, an := range fn.Annotations {
				if an.Name == "Get" || an.Name == "Post" || an.Name == "Put" || an.Name == "Patch" || an.Name == "Delete" {
					hasUI = true
					break annotationScan
				}
			}
		}
	}

	// STEP 3: Route to console or GUI. A program with no UI need at all
	// (standalone_console_test.prw: pure ConOut, no prompts, no browse)
	// always runs headless, TTY or not — there's nothing to interact
	// with, so opening a window would be pure noise, and piping its
	// stdout (`./app | tee log`, CI) must still exit on its own instead
	// of blocking on a Fyne event loop nobody's driving. A program that
	// DOES need UI goes to the console when launched from a real
	// terminal (isTTY), or to Fyne otherwise (double-clicked from a file
	// manager, no console attached). ADVPP_HEADLESS_STANDALONE forces the
	// console path even without a TTY, for batch/CI runs that want the
	// old no-UIProvider fast-exit behavior (see natives.go) instead of a
	// window nobody's watching. ADVPP_FORCE_GUI does the opposite — skips
	// the console entirely even from a real terminal — for a program
	// whose author decided a desktop window is the better default (a
	// wrapper script sets this per-app; it's not this stub's call to make
	// for every AdvPP program launched from a terminal).
	// builtAsGUI is baked in by `advplc build --gui`: the author declared
	// this program a desktop app, so the terminal fallback never applies to
	// it and no wrapper script has to export ADVPP_FORCE_GUI per platform.
	const builtAsGUI = __ADVPP_BUILT_AS_GUI__
	isTTY := ui.IsTerminal(os.Stdin) && !builtAsGUI && os.Getenv("ADVPP_FORCE_GUI") == ""
	if !hasUI || isTTY || os.Getenv("ADVPP_HEADLESS_STANDALONE") != "" {
		// If stdin is a real terminal, attach a TerminalUIProvider so
		// FWGetText/FWMenuSelect/FWMBrowse/Msg* actually work (without any
		// UIProvider they never touch stdin at all — see natives.go — so a
		// console app would print its banner and exit silently the instant
		// it hit a login prompt). Piped/non-TTY stdin forced into this path
		// via the env var above keeps the old no-provider fast-exit
		// behavior, since there's no one there to answer a prompt.
		v := vm.NewVM(&bc, false)
		if isTTY {
			v.SetUIProvider(ui.NewTerminalUIProvider())
		}
		v.SetDBFactory(func() vm.DBEngine {
			dbPath := shared.ResolveStandaloneDatabasePath("__ADVPP_APP_TITLE__")
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

	// ResolveStandaloneDatabasePath, e nao ResolveDatabasePath: um app
	// distribuido e lancado de onde o usuario mandar, e "./advpp.db" dentro
	// de Program Files e um caminho onde o SQLite nao abre nada.
	dbPath := shared.ResolveStandaloneDatabasePath("__ADVPP_APP_TITLE__")
	v.SetDBFactory(func() vm.DBEngine {
		engine, err := db.NewSQLiteEngine(dbPath)
		if err != nil {
			console.Append("Aviso de banco (" + dbPath + "): " + err.Error())
			return nil
		}
		return engine
	})

	done := make(chan int, 1)
	go func() {
		_, err := v.Run()
		if err != nil {
			// A janela NAO fecha aqui, de proposito. Erro precoce -- banco
			// que nao abre, funcao que nao existe -- acontece em
			// milissegundos, e o w.Close() que ficava neste ponto corria
			// junto com o ShowAndRun: a janela fechava antes de pintar e o
			// programa sumia sem mensagem nenhuma. No subsistema GUI do
			// Windows nao ha stderr visivel, entao o usuario so via "o
			// aplicativo nao abre". Agora a mensagem fica na tela ate ele
			// fechar.
			console.Append("")
			console.Append("ERRO DE EXECUCAO: " + err.Error())
			console.Append("Banco em uso: " + dbPath)
			console.Append("A janela continua aberta para a leitura da mensagem.")
			done <- 1
			return
		}
		w.Close() // Signals ShowAndRun to return
		done <- 0
	}()

	w.ShowAndRun()
	exitCode := <-done
	os.Exit(exitCode)
}
