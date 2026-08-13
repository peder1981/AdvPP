# Feature Matrix — AdvPP v2.0.3

**Date:** 2026-07-29  
**Scope:** All 55+ major features of AdvPP compiler  
**Status Legend:** 🟢 Implemented | 🟡 Limited/Partial | 🔴 Parsed Only | ❓ Unsupported

---

## Core Language (Lexer, Parser, Compiler)

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Lexer: Full tokenization** | 🟢 | ✅ | — | — | `Local x := 1` | None | ✅ Complete |
| **Preprocessor: #include** | 🟢 | ✅ | — | — | `#include "header.ch"` | None | ✅ Complete |
| **Preprocessor: #define** | 🟢 | ✅ | — | — | `#define MAX 100` | Single-line only | ✅ Complete |
| **Preprocessor: #ifdef/#ifndef/#else** | 🟢 | ✅ | — | — | `#ifdef DEBUG` | None | ✅ Complete |
| **Preprocessor: #xCommand/#xTranslate** | 🟢 | ✅ | — | — | `#xcommand TEST => ConOut("test")` | Pattern markers, optional clauses | ✅ Complete |
| **Parser: Recursive descent** | 🟢 | ✅ | — | — | Full AST generation | Recursion limit 1000 | ✅ Complete |
| **Compiler: Bytecode generation** | 🟢 | ✅ | — | — | 88 opcodes | None | ✅ Complete |
| **Compiler: Serialization** | 🟢 | ✅ | — | — | `advplc compile prog.prw -o prog.bytecode` | None | ✅ Complete |

---

## Variables & Scope

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Local variables** | 🟢 | ✅ | — | — | `Local x := 1` | Hoisted to function start | ✅ Complete |
| **Private variables** | 🟢 | ✅ | — | — | `Private nGlobal := 0` | Dynamic scoping works | ✅ Complete |
| **Public variables** | 🟢 | ✅ | — | — | `Public aArray := {}` | Dynamic scoping works | ✅ Complete |
| **Static variables** | 🟢 | ✅ | — | — | `Static nCounter := 0` | File-local or function-local | ✅ Complete |
| **Closures: Nested capture** | 🟢 | ✅ | — | — | `{\\| nLocal + nOuter}` | By-reference capture | ✅ Complete |

---

## Control Flow

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **If/ElseIf/Else/EndIf** | 🟢 | ✅ | — | — | `If x > 0 ... ElseIf ...` | None | ✅ Complete |
| **For/Next (with Step)** | 🟢 | ✅ | — | — | `For i := 1 To 10 Step 2 ... Next` | Negative step supported | ✅ Complete |
| **While/EndDo** | 🟢 | ✅ | — | — | `While i > 0 ... EndDo` | None | ✅ Complete |
| **Do Case/EndCase** | 🟢 | ✅ | — | — | `Do Case ... Case x > 0 ...` | None | ✅ Complete |
| **Loop (continue)** | 🟢 | ✅ | — | — | `Loop` in for/while | None | ✅ Complete |
| **Exit (break)** | 🟢 | ✅ | — | — | `Exit` in for/while | None | ✅ Complete |
| **Begin Sequence/Recover/End** | 🟢 | ✅ | — | — | Legacy exception handling | No Try/Catch in AdvPL | ✅ Complete |
| **Try/Catch/EndTry (TLPP)** | 🟢 | ✅ | — | — | `Try ... Catch e ... EndTry` | TLPP only | ✅ Complete |

---

## Data Types

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Character/String** | 🟢 | ✅ | — | — | `cName := "Test"` | Max 10MB | ✅ Complete |
| **Numeric** | 🟢 | ✅ | — | — | `nValue := 123.45` | Float64 internally | ✅ Complete |
| **Logical** | 🟢 | ✅ | — | — | `lFlag := .T.` | None | ✅ Complete |
| **Date** | 🟢 | ✅ | — | — | `dDate := Date()` | YYYY-MM-DD format | ✅ Complete |
| **Array** | 🟢 | ✅ | — | — | `aArray := {1, 2, 3}` | Max 1M elements | ✅ Complete |
| **Object** | 🟢 | ✅ | — | — | `oObj := MyClass():New()` | Class instantiation | ✅ Complete |
| **Code Block** | 🟢 | ✅ | — | — | `bBlock := {\\| nX + 1}` | Closures with capture | ✅ Complete |
| **Nil** | 🟢 | ✅ | — | — | `uVar := Nil` | None | ✅ Complete |
| **JSON (TLPP)** | 🟢 | ✅ | — | — | `JsonObject():New()["key"] := 123` | Full serialization | ✅ Complete |
| **Integer (TLPP)** | 🟢 | ✅ | — | — | `Local nInt as Integer` | Optional type hint | ✅ Complete |
| **Double (TLPP)** | 🟢 | ✅ | — | — | `Local dVal as Double` | Optional type hint | ✅ Complete |
| **Decimal (TLPP)** | 🟢 | ✅ | — | — | `Local decVal as Decimal` | Optional type hint | ✅ Complete |

---

## Functions & OOP

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **User Function** | 🟢 | ✅ | — | — | `User Function Test() ... Return` | None | ✅ Complete |
| **Static Function** | 🟢 | ✅ | — | — | `Static Function Helper()` | File-local | ✅ Complete |
| **Function parameters** | 🟢 | ✅ | — | — | `Function Test(nParam, cName, ...)` | Variadic supported | ✅ Complete |
| **Return values** | 🟢 | ✅ | — | — | `Return nResult` | Any type | ✅ Complete |
| **Class declaration** | 🟢 | ✅ | — | — | `Class MyClass ... EndClass` | Single inheritance via `from` | ✅ Complete |
| **Inheritance (from)** | 🟢 | ✅ | — | — | `Class Child from Parent` | Single inheritance only | ✅ Complete |
| **Data properties** | 🟢 | ✅ | — | — | `Data nValue as Numeric` | Public/Private/Protected | ✅ Complete |
| **Methods** | 🟢 | ✅ | — | — | `Method Calculate()` | Public/Private/Protected | ✅ Complete |
| **Constructor** | 🟢 | ✅ | — | — | `Method New()` | Automatically called | ✅ Complete |
| **Static methods** | 🟢 | ✅ | — | — | `Static Method Helper()` | Class-level functions | ✅ Complete |
| **Method implementation** | 🟢 | ✅ | — | — | `Method New() as Object class MyClass` | Separate from declaration | ✅ Complete |
| **Self-reference (::)** | 🟢 | ✅ | — | — | `::nValue := 10` | Auto-referencing | ✅ Complete |

---

## Advanced Features

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Entry Points (User Function aliases)** | 🟢 | ✅ | — | — | `U_MT410INC` → named in manifest | Protheus integration | ✅ Complete |
| **WSRESTFUL DSL (classic)** | 🔴 | ❌ | ✅ | — | `WSRESTFUL Test ... ENDWSRESTFUL` | Parsed, not executed | ⚠️ Known Limitation |
| **REST annotations (@Get/@Post)** | 🟢 | ✅ | — | — | `@Get("/path") User Function Test()` | HTTP dispatch real | ✅ Complete |
| **MCP Server (Model Context Protocol)** | 🟢 | ✅ | — | — | `MCPServer():New("name", "1.0")` | JSON-RPC 2.0 over stdio | ✅ Complete |
| **Web Renderer (advplc serve)** | 🟢 | ✅ | — | — | `advplc serve prog.prw --port 8080` | PO-UI + hot reload | ✅ Complete |
| **Standalone builds** | 🟢 | ✅ | — | — | `advplc build prog.prw -o prog` | Console/GUI auto-detect | ✅ Complete |

---

## Database & Workspace

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **DbSelectArea** | 🟢 | ✅ | — | — | `DbSelectArea("SA1")` | Workarea selection | ✅ Complete |
| **DbSeek** | 🟢 | ✅ | — | — | `DbSeek("001")` | Sequential scan | ✅ Complete |
| **DbSkip** | 🟢 | ✅ | — | — | `DbSkip()` | Move to next record | ✅ Complete |
| **DbGoTop/DbGoBottom** | 🟢 | ✅ | — | — | `DbGoTop()` | Position in alias | ✅ Complete |
| **RecLock/MsUnlock** | 🟡 | ✅ | — | — | Per-table semaphores | Intra-VM only (not inter-process) | ⚠️ Limited |
| **SX3 (field dictionary)** | 🟢 | ✅ | — | — | Column metadata in FWMBrowse | Auto-discovered | ✅ Complete |
| **Soft-delete filtering** | 🟢 | ✅ | — | — | `D_E_L_E_T_ = ' '` | Built-in | ✅ Complete |
| **Filial filtering** | 🟢 | ✅ | — | — | `XX_FILIAL = xFilial("XXX")` | Built-in | ✅ Complete |

---

## UI & MVC

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **FWFormModel** | 🟢 | ✅ | — | — | Model with field definitions | Rendering real | ✅ Complete |
| **FWFormView** | 🟢 | ✅ | — | — | View with component layout | Rendering real | ✅ Complete |
| **FWFormBrowse** | 🟡 | ✅ | — | — | Grid component | Rendering real, event handlers parsed | ⚠️ Partial |
| **Event handlers (onChange, onClick)** | 🟡 | ✅ | — | — | Defined but not connected | Parsed in bytecode | ⚠️ Parsed Only |
| **MSDIALOG (@SAY/@GET/@BUTTON)** | 🟢 | ✅ | — | — | Legacy grid-based dialog | Rendering real | ✅ Complete |
| **Console I/O (ConOut, ConIn, FWGetText)** | 🟢 | ✅ | — | — | Real terminal in TTY | TTY detection | ✅ Complete |
| **Message dialogs (MsgInfo, MsgYesNo)** | 🟢 | ✅ | — | — | UI dialogs | Fyne + console modes | ✅ Complete |
| **Fyne GUI rendering** | 🟢 | ✅ | — | — | IDE and standalone GUI | Full widget set | ✅ Complete |

---

## Runtime & Natives

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Native functions (~250+)** | 🟢 | ✅ | — | — | ConOut, AllTrim, Str, Val, etc. | See GUIA for full list | ✅ Complete |
| **TUI rendering primitives** | 🟢 | ✅ | — | — | UiBox, UiStreamBox, UiMarkdown (glamour), UiAltScreenEnter/Exit, UiTermWidth | Terminal only, no raw-mode keyboard input | ✅ Complete |
| **ProcRun (subprocess streaming)** | 🟢 | ✅ | — | — | `ProcRun(path, args, {\|line\| ...})` | Sync per-line callback, no shell | ✅ Complete |
| **String functions** | 🟢 | ✅ | — | — | AllTrim, SubStr, Len, Upper, Lower | Native functions | ✅ Complete |
| **Array functions** | 🟢 | ✅ | — | — | aAdd, aDel, aScan, ASort, AEval | High-order with blocks | ✅ Complete |
| **File I/O (MemoRead/MemoWrite)** | 🟢 | ✅ | — | — | Whole-file operations | Max 10MB | ✅ Complete |
| **Streaming I/O (FOpen/FRead/FWrite)** | 🟢 | ✅ | — | — | File handle operations | 100 open handles max | ✅ Complete |
| **WaitRun (system calls)** | 🟢 | ✅ | — | — | `WaitRun("command")` | Cross-platform sh/cmd | ✅ Complete |
| **StartJob (multi-threading)** | 🟢 | ✅ | — | — | `StartJob("FuncName", ...)` | 1000 concurrent jobs max | ✅ Complete |
| **FWGridProcess (thread pool)** | 🟢 | ✅ | — | — | Grid-based parallel processing | Thread-safe callbacks | ✅ Complete |

---

## HTTP & Network

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **FWHttpGet/FWHttpPost** | 🟢 | ✅ | — | — | HTTP requests with certs | 30s timeout, max 5 redirects | ✅ Complete |
| **PKCS#12 certificates** | 🟢 | ✅ | — | — | `.pfx`/`.p12` support | TLS client auth | ✅ Complete |
| **SSE (Server-Sent Events)** | 🟢 | ✅ | — | — | ConOut via HTTP + SSE (unidirecional servidor→cliente, sem WebSocket) | Console streaming | ✅ Complete |

---

## Machine Learning & Tensors

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Tensor class (float32/float64)** | 🟢 | ✅ | — | — | Forward operations | No GPU | ✅ Complete |
| **Tensor: MatMul** | 🟢 | ✅ | — | — | Matrix multiplication | Differentiable | ✅ Complete |
| **Tensor: Elementwise ops** | 🟢 | ✅ | — | — | Add, Sub, Mul, Div (with broadcast) | Differentiable | ✅ Complete |
| **Tensor: Activations** | 🟢 | ✅ | — | — | Relu, Tanh, Sigmoid, Gelu | Differentiable | ✅ Complete |
| **Tensor: Reductions** | 🟢 | ✅ | — | — | Sum, Mean, Max, Argmax | Axis-aware | ✅ Complete |
| **Tensor: Softmax** | 🟢 | ✅ | — | — | Per-row/column softmax | Non-differentiable in autodiff | ✅ Complete |
| **Variable (autodiff tape)** | 🟡 | ✅ | — | — | Reverse-mode AD | Limited ops (no softmax/CE backward yet) | ⚠️ Limited |
| **SGD optimizer** | 🟢 | ✅ | — | — | `SGD():New(params, lr)` | Basic gradient descent | ✅ Complete |
| **Adam optimizer** | 🟢 | ✅ | — | — | `Adam():New(params, lr)` | Added v1.9.0+ | ✅ Complete |
| **Linear module** | 🟢 | ✅ | — | — | `Linear():New(in, out)` | With parameters | ✅ Complete |
| **Embedding module** | 🟢 | ✅ | — | — | `Embedding():New(vocab, dim)` | Lookup table | ✅ Complete |
| **LLM inference (GGUF I2_S)** | 🟡 | ✅ | — | — | `LLM():New("model.gguf")` | Only I2_S quantization | ⚠️ Limited |

---

## Algebra & Geometry

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **MatVecTern (ternary MatVec)** | 🟢 | ✅ | — | — | Bitwise-efficient Matrix-Vector | Multiply-free | ✅ Complete |
| **Vector ops (VecDot, VecCross, VecNorm)** | 🟢 | ✅ | — | — | Float64 geometric operations | Non-differentiable | ✅ Complete |
| **Linear algebra (Det, Inv, QR, SVD)** | 🟢 | ✅ | — | — | `Tensor:Det()`, `Tensor:QR()` | Float64 only | ✅ Complete |
| **Eigenvalue decomposition (EigSym, Eig)** | 🟢 | ✅ | — | — | Symmetric and general eigenvalues | Float64 only | ✅ Complete |

---

## AI Models (Pure AdvPL)

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Markov LM (pt_llm.prw)** | 🟢 | ✅ | — | — | Byte-level n-gram with backoff | Deterministic | ✅ Complete |
| **Retrieval QA (pt_chat.prw)** | 🟢 | ✅ | — | — | Bag-of-words semantic search | Knowledge-base matching | ✅ Complete |
| **Hybrid Markov + Neural (pt_nn.prw)** | 🟢 | ✅ | — | — | Ternary ELM + long context | ~4096 token window | ✅ Complete |
| **Neural LM trained by gradient (pt_neural.prw)** | 🟢 | ✅ | — | — | Char-level NPLM with Adam | Small model, overfitting risk | ✅ Complete |
| **Code completion LM (dev_nn.prw)** | 🟢 | ✅ | — | — | Token-level LM for AdvPL | Corpus-dependent | ✅ Complete |

---

## Security & Stability

| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Audit Status |
|---------|--------|-------------|--------|-------------|---------|------------|--------------|
| **Recursion depth limit** | 🟢 | ✅ | — | — | Max 1000 levels | Parser + VM enforced | ✅ Complete |
| **String size limit** | 🟢 | ✅ | — | — | Max 10 MB | Soft limit, error returned | ✅ Complete |
| **Array size limit** | 🟢 | ✅ | — | — | Max 1M elements | Soft limit, error returned | ✅ Complete |
| **Stack frame limit** | 🟢 | ✅ | — | — | Max 5000 call frames | VM enforced | ✅ Complete |
| **Goroutine limit (StartJob)** | 🟢 | ✅ | — | — | Max 1000 concurrent | CWE-400 DoS prevention | ✅ Complete |
| **Timeout enforcement** | 🟢 | ✅ | — | — | LLM 5min, I/O 30s, HTTP 30s | Context-based | ✅ Complete |
| **TLS certificate verification** | 🟢 | ✅ | — | — | HTTPS only with valid certs | No self-signed by default | ✅ Complete |

---

## Summary by Status

### 🟢 Fully Implemented (53 features)
Core language, OOP, control flow, data types, most DB/UI, all runtimes, ML/tensors, security

### 🟡 Limited/Partial (7 features)
- RecLock/MsUnlock (intra-VM only)
- FWFormBrowse event handlers (parsed, not connected)
- Variable autodiff (limited ops)
- LLM inference (I2_S only)

### 🔴 Parsed Only (1 feature)
- WSRESTFUL DSL (use anotações @Get/@Post instead)

### ❓ Unsupported (0 features)
All major features have at least parsing/limited implementation; no completely unsupported features.

---

## Audit Notes

- **Date Audited:** 2026-07-29 (Task 18)
- **Methodology:** Cross-checked README claims vs. COMPONENT_STATUS.md vs. code reality
- **Discrepancies Fixed:** All 23 from Task 18 audit
- **Next Review:** Task 26 (Integration validation)

See `docs/documentation/DOCUMENTATION_AUDIT_RESULTS.md` and `docs/documentation/DISCREPANCY_ROADMAP.md` for detailed findings.
