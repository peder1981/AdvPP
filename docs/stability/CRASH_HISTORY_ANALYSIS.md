# Crash History & Root Cause Analysis — AdvPP v1.22.1 to v2.0.3

**Date:** 2026-07-29  
**Scope:** All documented crashes and critical failures from CHANGELOG  
**Purpose:** Identify patterns to guide Tasks 9–17 (null safety, bounds checking, concurrency)

---

## Executive Summary

**Crashes Found:** 11 critical + 30+ language gaps analyzed  
**Root Cause Categories:** Nil deref (3), parser drift (8), async race (1), encoding corruption (1)  
**High-Risk Areas:** Uninitialized variables, line number tracking, callback async, UTF-8 conversion

---

## Known Crashes (by Version)

### v2.0.3: Console Mode Detection Bug

**Date:** 2026-07-29  
**Trigger:** `advplc build` with `FWMBrowse` in standalone executable  
**Symptom:** Console UI never rendered; app ran headless without UI

```
advplc build app.prw -o App
// FWMBrowse():New() compiled but detection heuristic looked at 
// bc.Classes (declared classes), never instantiation (OP_NEW_INSTANCE)
// Result: detection thought no UI was needed; passed headless
```

**Root Cause:** Heuristic checked declared classes (`class X from Y`), not actual instantiation in bytecode (`OP_NEW_INSTANCE` opcodes). Detection ran before semantic analysis of which classes were actually constructed.

**Impact:** Standalone binaries silently skipped UI for apps using `FWMBrowse`, `MsgInfo`, `MSDIALOG`. User discovered via real app use (e-Gov project).

**Fix Applied:** v2.0.3 — Rewrote heuristic to scan bytecode for `OP_NEW_INSTANCE`, `OP_CALL_NATIVE` targeting UI functions. Validated in `build_standalone_interactive_test.go`.

**Status:** ✅ FIXED. Test: `TestBuildStandaloneInteractive` (3 platforms: Linux/macOS/Windows via ConPty).

**Lesson:** Compile-time heuristics must check runtime behavior, not just source-level declarations. Consider using bytecode analysis, not AST alone.

---

### v1.22.1: Null Pointer Dereference (SIGSEGV)

**Date:** 2026-07-24  
**Trigger:** Compare uninitialized variable to `Nil`

```advpl
Local x  // Uninitialized, no value assigned
If x == Nil  // CRASH: SIGSEGV in opComparison
```

**Root Cause:** Uninitialized `Local` slots contained Go's nil (zero-value of interface), not AdvPP's `Nil` value. When `x == Nil` ran, it called `.Equals(Nil)` on a nil pointer, crashing with SIGSEGV.

**Impact:** Any unbound variable compared to `Nil` crashed VM. Pattern is very common in AdvPL for optional parameters: `If oParam == Nil`.

**Fix Applied:** v1.22.1 — Frame allocation now pre-fills all `Locals` with `advplrt.Nil` via `newLocals()`. Every slot starts with a valid Value, never nil.

**Status:** ✅ FIXED. Validated in `security_null_safety_test.prw` (Task 4).

**Lesson:** Never let interface-typed variables be uninitialized. Allocate sentinel values at frame creation. Go's nil is not a valid Value in this VM.

---

### v1.22.1: Entry Point Selection Bug with `#include`

**Date:** 2026-07-24  
**Trigger:** Multi-file project with `#include` helper library

```advpl
// main.prw
#include "db.prw"

User Function MainApp()
    // Should run this
EndFunc

// db.prw
Static Function GcSqlLit()
    // Should NOT run, but it did
EndFunc
```

**Root Cause:** Entry point logic picked "first function declared after #include expansion". Since `#include` expands at file top, the helper function (`GcSqlLit`) appeared first in the expanded source, even though it was declared in the included file, not the root.

**Impact:** Wrong function executed as entry point. Multi-file projects could run initialization code instead of the actual app.

**Fix Applied:** v1.22.1 — Added `Preprocessor.RootBoundaryLine` to track where the root file's content begins (after all `#include` expansion). Entry point selection now prefers first function declared **in root file**. Falls back to first-of-all only when root has no functions.

**Status:** ✅ FIXED. Preserves backward compatibility: tests that intentionally declare entry point first still work.

**Lesson:** Preprocessor transformations (like `#include` expansion) must preserve source location metadata. Include boundaries are critical for semantic analysis.

---

### v1.22.0: Recover Without Variable Corrupted Local Slot 0

**Date:** 2026-07-24  
**Trigger:** `Begin Sequence ... Recover ... End Sequence` without `Using oErr`

```advpl
Begin Sequence
    SomeFailingCode()
Recover  // No "Using oErr" clause
    // Error silently swallowed
End Sequence

Local nFirst := 100  // This gets overwritten!
```

**Root Cause:** `TryCatch.CatchVarIdx` had no sentinel for "no variable". Go's zero-value (`0`) coincided with index 0 (the first local slot). `Recover` without `Using` wrote the error object to slot 0, silently corrupting the first declared local.

**Impact:** Silent data corruption. Any `Local` declared first in a function with bare `Recover` gets overwritten.

**Fix Applied:** v1.22.0 — Initialize `CatchVarIdx: -1` to represent "no catch variable". Check `!= -1` before writing the error object.

**Status:** ✅ FIXED. Validated in `OP_TRY_BEGIN`.

**Lesson:** Don't use zero-value to mean "sentinel". Use explicit sentinels (-1, nil, or a bool flag).

---

### v1.10.3: Windows Standalone Executable Hang

**Date:** 2026-07-11  
**Trigger:** `advplc build` on Windows, then run the `.exe`

**Symptom:** Program runs (console appears, MSDIALOG renders), but never closes; requires killing process.

**Root Cause:** `w.ShowAndRun()` (Fyne event loop) never returned after `v.Run()` completed. The VM's goroutine called `a.Quit()`, which closed the exit channel, but Fyne's event loop (in main goroutine) didn't notice.

**Impact:** Generated Windows executables unusable unless killed manually.

**Fix Applied:** v1.10.3 — Replaced graceful shutdown with `os.Exit(0)` after `v.Run()` completes. Executables are short-lived scripts, not long-running services.

**Status:** ✅ FIXED. Validated on GitHub Actions Windows runner.

**Lesson:** Avoid complex shutdown handshakes for short-lived processes. `os.Exit()` is often simpler and safer.

---

### v1.10.3: Windows File Move Across Drives

**Date:** 2026-07-11  
**Trigger:** `advplc build` on Windows with temp dir on different drive than output

```
temp dir: C:\Temp
output:   D:\bin\app.exe
os.Rename(C:\Temp\stub.exe, D:\bin\app.exe)  // ERROR: different drives
```

**Root Cause:** `os.Rename()` fails on Windows when source and destination are on different drives (can't move across volumes).

**Impact:** Build fails silently; no executable generated.

**Fix Applied:** v1.10.3 — Implement `moveFile()` helper with fallback: `Rename()` first, then `Copy() + Remove()` if rename fails.

**Status:** ✅ FIXED. Tested on GitHub Actions (temp on C:, checkout on D:).

**Lesson:** Cross-platform I/O needs OS-specific fallbacks. Test on all target platforms.

---

### v1.10.3: File Handle Not Closed Before Removal

**Date:** 2026-07-11  
**Trigger:** Fallback copy+remove on Windows with open file handle

```go
// OLD: file still open, handle not released
src, _ := os.Open(src)
io.Copy(dst, src)
// defer src.Close()  <- runs AFTER return
os.Remove(src)  // Windows: ERROR - file still open
```

**Root Cause:** `defer` cleanup runs after `return`, but `os.Remove()` happens before return. Windows refuses to delete open files (POSIX allows it).

**Impact:** Move operation leaves temporary files in temp directory.

**Fix Applied:** v1.10.3 — Explicitly `Close()` before `Remove()`, not via `defer`.

**Status:** ✅ FIXED.

**Lesson:** Don't rely on `defer` for cleanup order critical to correctness. Explicit close+error check is safer.

---

### v1.10.2: MsgYesNo Always Returned false

**Date:** 2026-07-11  
**Trigger:** Desktop Fyne backend: `MsgYesNo("Continue?")` in action handler

**Root Cause:** `dialog.ShowConfirm()` (Fyne) is asynchronous. Callback sets a variable, but code read the variable before callback ran. Result always `false`.

**Impact:** Confirmation dialogs never worked on desktop. Every "yes/no" choice was forced to "no".

**Fix Applied:** v1.10.2 — Moved VM execution to goroutine (off main Fyne thread) so dialog callbacks can process events. Use channel-based synchronization (same pattern as `MsgInfo`/`MsgAlert` already used).

**Status:** ✅ FIXED. Validated visually via Xvfb.

**Lesson:** GUI frameworks have async callbacks. Don't assume callback has run by the time function returns. Use channels or event loops for proper synchronization.

---

### v1.8.7: NBSP UTF-8 Byte Sequence Corrupts Identifier

**Date:** 2026-07-10  
**Trigger:** Source file with NBSP (non-breaking space) character

```
MSGET[NBSP]cBanco  // Editor inserted 0xA0
// CP-1252 → UTF-8: 0xA0 becomes 0xC2 0xA0
// Lexer heuristic: byte >= 0x80 is acentuated letter
// 0xC2 eaten into identifier: MSGET\xc2\xa0cBanco (corrupted)
```

**Root Cause:** Lexer assumed bytes >= 0x80 are multi-byte UTF-8 continuation of the CP-1252 conversion, but UTF-8 NBSP is 2 bytes: `0xC2 0xA0`. Lexer ate the `0xC2` into the identifier, then became confused by `0xA0`.

**Impact:** Identifiers after NBSP get corrupted, parser drifts 1–2 statements later.

**Fix Applied:** v1.8.7 — Recognize UTF-8 NBSP sequence (`0xC2 0xA0`) as whitespace in both `skipWhitespace()` and identifier scanner.

**Status:** ✅ FIXED. This was the root of a "drift bug" that appeared unsolvable for multiple iterations.

**Lesson:** Character encoding conversions (CP-1252 → UTF-8) introduce subtle two-byte sequences. Test with real editors that insert non-standard spaces.

---

### v1.8.5: #IFDEF Inside Block Comments Breaks Parser

**Date:** 2026-07-10  
**Trigger:** Commented-out preprocessor directive

```
/* This code is broken:
#IFDEF OLD_CODE
  ... code ...
#ENDIF
*/

REAL CODE HERE  // Parser is out of sync
```

**Root Cause:** Preprocessor consumed `#IFDEF`/`#ENDIF` inside `/* ... */` as real directives, deleting the `*/` from output and flipping the parser's comment state for the rest of the file.

**Impact:** Large portions of code become invisible to parser.

**Fix Applied:** v1.8.5 — Preprocessor now tracks block comment state. Skips directives when inside `/* ... */`.

**Status:** ✅ FIXED.

**Lesson:** State machines need to track all relevant context. Directives inside comments are edge cases that must be guarded.

---

### v1.6.0+: Parser Drift from Newline Handling

**Date:** Multiple versions  
**Trigger:** Newlines removed before parsing, causing collapsing of unrelated statements

```advpl
var := f()    // Statement 1, line 1
(alias)->fld := x  // Statement 2, line 2

// After newline removal:
var := f()(alias)->fld  // Looks like: f() called with (alias) as first argument
```

**Root Cause:** Newline stripping is done globally before parsing. Lookahead logic for `(` after identifier doesn't check that `(` is on the same line as identifier in the **original** source.

**Impact:** Many parsing failures that look random but are actually systematic. "Drift bugs" that appear in test #1, disappear in isolated test of lines 1–5, reappear in test of lines 1–100.

**Fix Applied:** v1.7.3, v1.8.0, etc. — Added checks that critical tokens (like `(` after identifier) are on the same source line. Consult `p.tokens[p.pos-1].Line == p.peek().Line` before consuming.

**Status:** ✅ FIXED (throughout parser). Pass-rate climbed from 70% → 100% as these checks were added systematically.

**Lesson:** Newline-removing preprocessors are dangerous. Every place that makes decisions based on token sequence needs to verify original line numbers.

---

## Pattern Analysis

| Crash Type | Count | Examples | Root Cause | Fix Task |
|-----------|-------|----------|-----------|----------|
| **Null dereference** | 3 | Uninitialized Local, uninitialized return, nil `.Equals()` | No sentinel for "uninitialized" | Task 9 (extend to all dereferences) |
| **Parser drift** | 8+ | Newline collapsing statements, comment state leaking | Removed context (newlines, comment flag) | Task 10 (bounds on parser input) |
| **Encoding corruption** | 1 | NBSP two-byte sequence | Character encoding edge case | Task 5 ✅ |
| **Async race** | 1 | MsgYesNo callback timing | Thread safety, goroutine coordination | Task 11 |
| **Entry point selection** | 1 | #include hiding root functions | Loss of metadata (source location) | Parser fix ✅ |
| **State machine (recover)** | 1 | No sentinel for "no catch var" | Zero-value semantics | Fix ✅ |
| **GUI resource** | 3 | Fyne event loop, file handles, process exit | Platform-specific shutdown | Fix ✅ |

---

## Severity Classification

### CRITICAL (VM Crash / Data Loss)

1. **v1.22.1 SIGSEGV**: Nil comparison → any uninitialized variable breaks  
   → **Impact:** Very high (common idiom in AdvPL)  
   → **Fix Status:** ✅ FIXED (v1.22.1)

2. **v1.8.7 NBSP corruption**: Encoding edge case → parser drift for rest of file  
   → **Impact:** High (non-obvious, hard to debug)  
   → **Fix Status:** ✅ FIXED (v1.8.7)

3. **v1.22.0 Recover bug**: Silent data corruption (first local overwritten)  
   → **Impact:** High (data loss, no error)  
   → **Fix Status:** ✅ FIXED (v1.22.0)

### HIGH (Feature Broken / Wrong Results)

4. **v2.0.3 UI detection**: Console mode not detected → app runs headless  
   → **Impact:** High (app appears to work but has no UI)  
   → **Fix Status:** ✅ FIXED (v2.0.3)

5. **v1.22.1 Entry point**: Wrong function executed as main  
   → **Impact:** High (app does wrong thing)  
   → **Fix Status:** ✅ FIXED (v1.22.1)

6. **v1.10.2 MsgYesNo**: Dialog choice always ignored  
   → **Impact:** High (user interaction broken)  
   → **Fix Status:** ✅ FIXED (v1.10.2)

### MEDIUM (Platform-Specific / Build-Time)

7. **v1.10.3 Windows file move**: Build fails across drives  
   → **Impact:** Medium (Linux/Mac unaffected; Windows users blocked)  
   → **Fix Status:** ✅ FIXED (v1.10.3)

8. **v1.10.3 Process hang**: Executable never closes (Windows only)  
   → **Impact:** Medium (UX bad, data safe)  
   → **Fix Status:** ✅ FIXED (v1.10.3)

### LOW (Parser Recovery / Rare)

9. **v1.8.5 #IFDEF in comments**: Directive processed inside comment  
   → **Impact:** Low (rare pattern; comments are typically short)  
   → **Fix Status:** ✅ FIXED (v1.8.5)

10. **Parser drift (various)**: Collapsing of statements → 8+ fixes  
    → **Impact:** Low (each isolated; fixed as discovered)  
    → **Fix Status:** ✅ FIXED (v1.7.3–v1.8.7)

---

## Roadmap: Which Task Addresses Each Crash

| Crash | Cause | Task | Action |
|-------|-------|------|--------|
| Nil comparison | Uninitialized Local | Task 9 | Comprehensive null safety audit |
| Encoding (NBSP) | UTF-8 edge case | Task 10 | Bounds/encoding validation |
| Parser drift | Removed newlines | Task 10 | Input validation (same-line checks) |
| Entry point | #include metadata loss | Task 10 | Preserve source location across transforms |
| Recover corruption | Zero-value sentinel | Task 9 | Null safety + validation |
| MsgYesNo async | Thread safety | Task 11 | Concurrency fixes |
| File handle | Close ordering | Task 12 | Resource cleanup validation |
| UI detection | Bytecode analysis | Task 9 | Full semantic analysis |
| Process hang | Goroutine coordination | Task 11 | Shutdown coordination |

---

## Edge Cases That Nearly Became Crashes

**Not yet fixed (low priority):**

1. **Arrays**: No bounds checking on `aGet`, `aSet` with huge/negative indices
   → Fixed in v1.9 (limits on array size), but element access still unchecked → Task 10

2. **Recursion**: Depth limit at 1000, but some paths don't check
   → Partially fixed in v2.0.3 (recursion limit enforced), needs completion → Task 9

3. **Concurrency**: `StartJob` with no goroutine limit
   → Added limit (1000) in v2.0.3 → Task 11 validates

4. **String length**: Limit at 10 MB, but some operations create temporary strings
   → Added limit in v2.0.3 → Task 10 validates

5. **JSON nesting**: No depth limit
   → Added limit (100) in v2.0.3 → Task 10 validates

---

## Summary

**All known crashes from v1.22.1 to v2.0.3 are documented and fixed.** The pattern is clear:

1. **Initialization bugs** (nil variables) → **Task 9: Null Safety Audit**
2. **Parser/lexer edge cases** (newlines, encoding) → **Task 10: Bounds Checking**
3. **Concurrency/resource bugs** (async callbacks, file handles) → **Task 11–12: Resource/Concurrency**
4. **No systematic testing of edge cases** → **Task 13–15: Edge Case Matrix + Corpus Validation**

Next: Create EDGE_CASE_MATRIX.md to identify **untested combinations** that could hide new crashes.
