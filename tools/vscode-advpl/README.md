# AdvPL/TLPP for VS Code

Syntax highlighting for AdvPL and TLPP (TOTVS Protheus) source files
(`.prw`, `.prg`, `.prx`, `.tlpp`, `.ch`, `.th`, `.aph`), plus compiler task
integration for [AdvPP](https://github.com/peder1981/AdvPP).

Works in standard VS Code and in any VS Code-compatible fork (e.g.
NeuralInverse).

## Install

Download the `.vsix` from [Releases](https://github.com/peder1981/AdvPP/releases)
and install it:

```bash
code --install-extension advpl-tlpp-*.vsix
```

The package **bundles the `advplc` compiler** for linux-x64, linux-arm64,
win32-x64 and darwin-arm64 (`bin/<platform>-<arch>/`) — nothing else to
install. On an unsupported platform (e.g. Intel Mac), install `advplc`
separately (`curl -fsSL https://raw.githubusercontent.com/peder1981/AdvPP/master/install.sh | sh`)
and point `advpl.compilerPath` at it.

To rebuild the `.vsix` yourself (cross-compiles the compiler for all 4
platforms first): `./build-vsix.sh <version>`.

## What's included

- **Syntax highlighting**: keywords, control flow, types, strings
  (`"..."`, `'...'`, `[...]`), comments (`//`, `/* */`), preprocessor
  directives (`#include`, `#define`, ...), class/method/function
  declarations.
- **Language configuration**: comment toggling, bracket matching,
  auto-closing pairs, code folding on `FUNCTION`/`CLASS`/`IF`/`DO CASE`/etc.
- **Compiler commands with keybindings** — work in any workspace, no
  `.vscode/tasks.json` required (see below).

## Compiler commands & keybindings

The extension spawns `advplc` directly against the active file and streams
its output into an **"AdvPL" Output Channel**. It resolves the compiler in
this order: `advpl.compilerPath` setting → search upward from the open file
for an `advplc` binary (project-local build) → the compiler bundled with
this extension → `PATH`.

| Shortcut | Command | Runs |
|----------|---------|------|
| `Ctrl+F9` | AdvPL: Build standalone executable | `advplc build <file> -o <file-without-ext>` |
| `F9` | AdvPL: Run current file | `advplc run <file>` |
| `Ctrl+Shift+F9` | AdvPL: Compile current file (bytecode) | `advplc compile <file>` |
| `Ctrl+Alt+F9` | AdvPL: Serve current file (web / PO-UI) | `advplc serve <file> --debug-port <port>` |

Note: `F9` also collides with VS Code's built-in "Toggle Breakpoint" (`when: editorTextFocus`) — our binding took priority in testing, but if you ever see it toggle a breakpoint instead of running, that's the conflict surfacing; rebind one of them from **Preferences → Keyboard Shortcuts**.

`AdvPL: Check current file` (syntax check, `advplc check`) is available via
the Command Palette without a default keybinding. All shortcuts are
rebindable from **Preferences → Keyboard Shortcuts**.

The active file is saved automatically before each run.

## Compiler tasks (alternative)

If you open the [AdvPP](https://github.com/peder1981/AdvPP) repository
itself (which ships its own `.vscode/tasks.json`), the same operations are
also available via **Terminal → Run Task** — useful for chaining with other
tasks, but the commands above are faster for everyday use.

## Debugging (breakpoints / step / variables)

`F5` (Run and Debug) compiles and runs the active file with real breakpoints,
step over/in/out, a call stack, and local variable inspection — no
`launch.json` required, though one is generated if you customize it (Run and
Debug panel → `create a launch.json file`, type `advpl`).

- Click in the gutter to set a breakpoint, then `F5`.
- `F10` step over, `F11` step into, `Shift+F11` step out, `F5` continue
  (standard VS Code debug keybindings, unchanged).
- Locals show up in the Variables panel; `ConOut`/`ConOutW` output goes to
  the Debug Console.

Implementation: `advplc debug` runs as a standalone Debug Adapter Protocol
server over stdio (`pkg/dap` + hooks in `pkg/vm/debug.go`) — the extension
just spawns it and forwards VS Code's debug UI to it, so this also works in
any DAP-compatible editor, not just this extension.

### Debugging a web / PO-UI session (`advplc serve`)

`Ctrl+Alt+F9` (or the `AdvPL: Serve current file` command) always starts
`advplc serve` with `--debug-port` open (default `4711`, override with the
`advpl.debugPort` setting) — the server accepts a debugger *attach*
connection at any time, no restart needed.

To attach: run **AdvPL: Attach debugger to running serve session** (Command
Palette), or add an `attach` configuration to `launch.json` (Run and Debug
→ create a launch.json → pick "AdvPL: Attach to serve session"). Set your
breakpoints, then (re)load the page in the browser — each browser
tab/reload creates a fresh VM session server-side, and the debugger attaches
to the *next* one that starts after `configurationDone`.

This is architecturally different from launch mode: `advplc serve` is a
long-lived process that can have multiple concurrent browser sessions (one
VM each). Only **one session is debugged at a time** — if a second tab
connects while a debug session is already attached to another, it just runs
normally, no breakpoints. Reattach (rerun the attach command) to pick up a
different session. `ConOut`/`ConOutW` output shows in both the browser
console and the editor's Debug Console simultaneously while attached.

**Known limitations** (single active source file, single-session attach):
- No conditional/logpoint breakpoints, no expression evaluation in the
  Debug Console (watch/hover), no stepping into `#command`/`#xcommand`
  macro-expanded code or across `#include`d files.
- Stepping into a class method's operator overload (`+`, `==`, etc.) won't
  pause inside it — that path bypasses the debug hook (see comment in
  `pkg/vm/debug.go`).
- Attach mode only debugs one browser session at a time (see above) — no
  per-tab thread list in the Call Stack view.

## Building the extension

```bash
npx @vscode/vsce package
```

Produces `advpl-tlpp-<version>.vsix`, installable via **Extensions → Install
from VSIX...** in VS Code or NeuralInverse.
