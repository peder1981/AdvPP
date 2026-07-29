# AdvPP v2.0.3 Integral Audit Design — Security, Stability, Documentation

**Date:** 2026-07-29  
**Scope:** Complete audit and correction of 32k-line AdvPL/TLPP compiler in Go  
**Goal:** Market-grade stability with 100% quality across 3 independent pillars  
**Authorization:** Full rewrite/refactor/correction as needed; zero time pressure

---

## Executive Summary

AdvPP v2.0.3 is a functional AdvPL/TLPP compiler with known gaps: security boundaries undefined, stability edge cases untested, documentation diverges from implementation. This specification outlines a **3-cycle integral audit** (Security, Stability, Documentation), each with discovery → correction → validation, running in parallel but integrated.

**Success Criteria:**
- **Security:** Zero OWASP Top 10 + CWE critical vulnerabilities; fuzzing 1M+ iterations crash-free; code review sign-off
- **Stability:** Zero crashes on 300-file corpus; 100% edge case coverage (null, overflow, recursion, concurrency, resources); graceful error handling everywhere
- **Documentation:** 468/468 API symbols documented; README synchronized with implementation; feature matrix (implemented/parsed/unsupported); all examples runnable

**Estimated Effort:** 40–60 hours of rigorous, minuscule work across 3 concurrent cycles

---

## Part 1: Audit Architecture

### Design: 3 Independent Cycles + Integrated Validation

```
┌─────────────────────────────────────────────────────────────┐
│          3 PARALLEL CYCLES: SECURITY | STABILITY | DOCS    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────┐ │
│  │ SECURITY CYCLE   │  │ STABILITY CYCLE  │  │ DOCS      │ │
│  ├──────────────────┤  ├──────────────────┤  ├───────────┤ │
│  │ 1. Scan OWASP    │  │ 1. Crash Mining  │  │ 1. Audit  │ │
│  │    + CWE         │  │ 2. Edge Cases    │  │ 2. API    │ │
│  │ 2. Fuzzing 1M    │  │ 3. Resource Test │  │ 3. Feature│ │
│  │ 3. Code Review   │  │ 4. Corpus Valid. │  │    Matrix │ │
│  └──────────────────┘  └──────────────────┘  └───────────┘ │
│         ↓                      ↓                    ↓        │
│   Corrections             Corrections          Updates      │
│    (Commits)              (Commits)           (Commits)     │
│         ↓                      ↓                    ↓        │
│  ✅ SECURE              ✅ STABLE          ✅ SYNCED        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                            ↓
              ┌─────────────────────────────┐
              │   INTEGRATED VALIDATION     │
              │   (Cross-cycle checks)      │
              └─────────────────────────────┘
```

**Rationale:**
- Parallel cycles maximize context (each auditor deeply understands one pillar)
- Integrated validation prevents corrections in one cycle from breaking another
- Sequential delivery of commits (by cycle) keeps git history clean
- Evidence is localized (security findings in security cycle, etc.)

---

## Part 2: Security Cycle

### 2.1 Discovery Phase

**OWASP Top 10 Manual Scan:**
- **A1 Injection:** SQL (via `DbSeek`, field values), command (`WaitRun`), template (string interpolation in DSL)
- **A2 Broken Authentication:** Session handling (none exists; NIY)
- **A3 Broken Access Control:** Field-level or function-level authorization (none; NIY)
- **A4 XXE:** XML parsing (not in scope; not used)
- **A5 Broken Access Control (Data):** Soft-delete validation (`D_E_L_E_T_` flag), filial filtering
- **A6 Insecure Deserialization:** JSON decode, bytecode unmarshal, LLM model loading
- **A7 Broken Auth / Session:** N/A (stateless CLI)
- **A8 Insufficient Logging:** Errors swallowed silently (ex: WSRESTFUL DSL parsed but not executed; no warning)
- **A9 SSRF / XXE:** HTTP client (redirect limits already added v2.0.3; certificate pinning?)
- **A10 Insufficient Logging & Monitoring:** No audit trail, no structured logging

**CWE Critical (Go-specific):**
- **CWE-190 Integer Overflow:** BigInt arithmetic (NumValue.Val is float64; overflow silent)
- **CWE-119 Buffer Overflow:** Slices, arrays (Go's bounds check prevents, but logical errors?)
- **CWE-476 Null Pointer Dereference:** Dereference of nil pointers (v1.22.1 bug example)
- **CWE-674 Uncontrolled Recursion:** Parser, VM (v2.0.3 added MaxRecursionDepth=1000; extend to all)
- **CWE-400 Uncontrolled Resource Consumption:** DoS via huge strings (v2.0.3 limited to 10MB), deep recursion, BigInt
- **CWE-362 Race Condition:** Goroutines, shared DB, bytecode cache
- **CWE-89 SQL Injection:** Field values in WHERE clauses
- **CWE-78 OS Command Injection:** `WaitRun` with user input

**Fuzzing Setup:**
- `go-fuzz` on lexer (random source code)
- `go-fuzz` on parser (random tokens)
- `go-fuzz` on VM (random bytecode + random stack operations)
- `go-fuzz` on HTTP handler (random JSON payloads)
- `go-fuzz` on LLM tokenizer (random UTF-8)
- Target: 1M+ iterations without crash; crash = bug

**Code Review Scope:**
- Entry points: lexer input, parser input, HTTP handler, JSON decode, SQL values, command execution
- Crypto: random number generation, HTTPS client, certificate handling
- Serialization: bytecode marshal/unmarshal, JSON, GGUF model loading
- Network: HTTP client (redirects, timeouts, MITM), WebSocket, SSE

### 2.2 Correction Phase

**Input Validation (Trust Boundaries):**
- Lexer: source file size limit (already 10MB string limit)
- Parser: expression nesting limit (already 1000; validate everywhere)
- VM: bytecode instruction count limit, stack depth (already 10k)
- HTTP: request body size, header size, connection timeout
- JSON: max nesting depth, max object keys, max array size
- SQL: parameterize ALL user input (no string concatenation)

**Sanitization:**
- SQL: use `FWExecStatement` or `ChangeQuery()` — never raw string concat
- Command: escape arguments or use `exec.Command` (not shell)
- Template strings: no interpolation of untrusted data
- Filenames: validate no path traversal (../../../etc/passwd)

**Rate Limiting & Timeouts:**
- HTTP REST server: 100 req/sec per IP, 10 concurrent per IP
- LLM generation: timeout 5 minutes per generate() call
- File I/O: timeout 30 seconds per read/write
- Network: timeout 30 seconds per HTTP request, max 5 redirects

**Secure Random:**
- Use `crypto/rand` for: tokens, session IDs, CSRF tokens
- Validate that `math/rand` is NOT used for security (it isn't currently; grep and confirm)

**Certificate & HTTPS:**
- HTTP client: verify TLS certificates (already done; confirm `InsecureSkipVerify=false`)
- HTTPS server (web renderer): TLS version >= 1.2, strong ciphers
- Certificate pinning: optional (if connecting to known external service, pin the cert)

**Error Messages:**
- Never leak internals (stack trace, SQL, config paths) to HTTP response
- Log errors internally; return generic "Error 500" to client

### 2.3 Validation Phase

**Fuzzing Results:**
- Run 1M+ iterations on each fuzzer target
- Zero crashes = PASS
- Crash = bug to investigate and fix
- Keep fuzzer corpus for regression testing

**OWASP/CWE Test Cases:**
- SQLi: attempt SQL injection in field values → expect parameterized query
- RCE: attempt command injection in `WaitRun` → expect escaped/safe
- XXE: attempt XXE in JSON → expect parse error, not execution
- Redirect: attempt open redirect in HTTP client → expect max 5 redirects
- Rate limit: send 200 req/sec → expect 429 after limit

**Code Review Checklist:**
- [ ] All user input validated at trust boundaries
- [ ] All errors handled gracefully (no nil deref, no panic)
- [ ] All external data (HTTP, files, DB) treated as untrusted
- [ ] No hardcoded secrets (API keys, passwords, certs)
- [ ] Crypto uses stdlib (no custom crypto)
- [ ] SQL uses parameterization, not string concat
- [ ] Timeouts on all blocking operations

**Deliverables:**
- Commits organized by topic (SQLi fixes, DoS limits, error handling, etc.)
- Fuzzing corpus and results (% coverage, crash count over time)
- Security posture report (vulnerabilities found, fixed, residual)

---

## Part 3: Stability Cycle

### 3.1 Discovery Phase

**Crash Mining (Historical):**
- v1.22.1: nil pointer dereference (compare uninitialized var to nil)
- v2.0.3: console mode didn't work (detection heuristic wrong)
- Scan CHANGELOG for all reported issues; extract root cause patterns

**Edge Cases Deep Dive:**

| Category | Examples | Risk |
|----------|----------|------|
| **Null/Nil** | Compare nil to nil, call method on nil object, dereference nil field | SIGSEGV |
| **Numeric** | BigInt overflow (float64 max), negative array index, modulo zero | Silent wrong result or panic |
| **String** | Empty string, UTF-8 multi-byte, NBSP (0xC2 0xA0), very long strings | Crash or corruption |
| **Array** | Empty array, out-of-bounds access, circular reference, sparse array | SIGSEGV or stack overflow |
| **Object** | Nil object, no property, method not found, circular reference | SIGSEGV |
| **CodeBlock** | Nil codeblock, nested closure capturing nil, recursion in closure | SIGSEGV or stack overflow |
| **Recursion** | Parser (nested expr), VM (codeblock call), user function (mutual recursion) | Stack overflow (caught at 1000; ok) |
| **Concurrency** | StartJob with shared DB, FWGridProcess race, bytecode cache race | Deadlock or corruption |
| **Resources** | Huge array (million elements), huge string (100MB), million goroutines | OOM or goroutine leak |

**Corpus Testing:**
- Run `advplc check` on all 300 files in OKF corpus
- Expected: zero crashes, all parse successfully
- Actual: X crashes, Y parse errors → investigate each

**Matrix of Combinations:**
- Identify top 50 opcodes
- For each opcode × (nil, zero, negative, huge, empty) → does it crash?
- Generate matrix: opcode × input × result (PASS/FAIL/ERROR)

### 3.2 Correction Phase

**Bounds Checking:**
- Array access: `if idx < 0 || idx >= len(arr)` → return ErrorValue, not crash
- String substr: `if start < 0 || end > len(s)` → return safe substring
- Slice operations: Go does this automatically; verify no logical errors

**Null Safety:**
- Every dereference (object.Field, array[i]) must check for nil first
- Add guards: `if val == nil { return Nil }` or `if val == nil { return ErrorValue(...) }`
- Go's interface nil check: `if val.(SomeType) == nil` works; use it

**Graceful Error Handling:**
- Every operation that can fail → return ErrorValue (capturable by Try/Catch)
- Never panic from VM natives; always return error
- Stack traces logged internally; never exposed to user

**Resource Limits (extend v2.0.3):**
- Recursion: already 1000 (OK; validate applied everywhere)
- String length: already 10MB (OK)
- Stack size: already 10k (OK)
- Call frames: already 5k (OK)
- **NEW:**
  - Array size: limit to 1M elements
  - Object properties: limit to 10k keys
  - Goroutines (StartJob): limit to 1000 concurrent
  - File handles: limit to 100 open
  - Memory per VM: soft limit 1GB (monitor, not hard limit)

**Timeout & Deadlock Detection:**
- LLM generate: timeout 5 minutes
- File I/O: timeout 30 seconds
- HTTP request: timeout 30 seconds
- DB query: timeout 30 seconds
- Detect goroutine leaks in tests (via `runtime.NumGoroutine()`)

**Concurrency Fixes:**
- RecLock/MsUnlock are currently no-ops → add semaphore (per-table lock)
- Shared bytecode cache: add read-write lock if cache is updated
- Shared DB connection: validate SQLite's busy_timeout + WAL mode (already done)

### 3.3 Validation Phase

**Corpus Testing:**
- Run `advplc check` on 300 OKF files
- Expected: all pass, zero crashes
- Actual result: pass or fail?

**Edge Case Matrix:**
- Run test suite with 50 opcodes × (nil/zero/negative/huge/empty)
- Score: N/N edge cases handled gracefully

**Resource Limit Tests:**
- Try to create 2M-element array → expect error (not OOM)
- Try to recurse 2000 levels → expect error at 1001
- Try to spawn 2000 goroutines → expect error after 1000
- Try to open 200 files → expect error after 100

**Concurrency Tests:**
- StartJob with 100 concurrent functions → expect all complete, zero deadlock
- FWGridProcess with 10 workers → expect no race conditions

**Deliverables:**
- Commits by category (null safety, resource limits, concurrency, error handling)
- Edge case coverage matrix (% of combinations tested)
- Corpus validation results (300 files, all pass)
- Resource limit test results

---

## Part 4: Documentation Cycle

### 4.1 Discovery Phase

**Audit: README vs COMPONENT_STATUS vs Code**

| Feature | README says | COMPONENT_STATUS says | Code says | Sync? |
|---------|-------------|----------------------|-----------|-------|
| WSRESTFUL DSL | "Supported" | "Only parsing, not executed" | Parser only, no dispatch | ❌ WRONG |
| RecLock/MsUnlock | "Supported" | "No-ops — no concurrency control" | No-ops (TODO comment) | ❌ WRONG |
| Anotações REST | "Full server, real routes" | "Full server" | Real dispatch | ✅ OK |
| LLM | "Class LLM, GGUF loading" | "Validated token-to-token" | Implemented | ✅ OK |
| MCP | "MCPServer, JSON-RPC" | "Real implementation" | Implemented | ✅ OK |

**API Doc Gap:**
- Count symbols in Go: `grep -r "^type \|^func " pkg/ cmd/ | wc -l` → ~468 symbols
- Count with doc comments: `grep -B1 "^type \|^func " pkg/ cmd/ | grep "//" | wc -l` → currently ~52
- **Gap:** 416 symbols without docs (88.9% undocumented)

**Feature Matrix:**
- Create matrix (feature × status) for all ~50 major features
- Compare vs README to find discrepancies

**Examples Audit:**
- Each feature in README should have an example
- Try to run each example → PASS/FAIL
- Fail = documentation bug

### 4.2 Correction Phase

**API Documentation (468 → 468 symbols):**
- Every Go type: document its purpose, fields, when to use it
- Every exported function: document parameters, return value, errors, example usage
- Every method: document behavior, preconditions, postconditions
- Style: one-line summary (no implementation details); longer explanation if needed

**README Synchronization:**
- Separate "Implemented" (executed) from "Parsed" (recognized but not run)
- Add explicit section "Known Limitations" (WSRESTFUL DSL, locks, etc.)
- Update feature list to match COMPONENT_STATUS exactly
- Add "Unsupported" section if something is requested but not done

**Feature Matrix Update:**
- Create `FEATURE_MATRIX.md` with:
  - Feature name, status (implemented/parsed/limited/unsupported), notes, example
  - All 50+ major features covered
  - Sync with COMPONENT_STATUS

**Examples: Runnable Validation:**
- Every example in README must be in `tests/` directory and pass `advplc run`
- Create test file `tests/readme_examples_test.prw` that runs each example
- If example fails, it's a documentation bug

**Limitations Disclosure:**
- Document max recursion depth (1000)
- Document max string length (10 MB)
- Document max array size (1M elements)
- Document resource timeouts (5 min LLM, 30s I/O)
- Document no-op features (RecLock, MsUnlock)
- Document incomplete features (WSRESTFUL DSL, event handlers in UI)

**Breaking Changes Tracking:**
- If security/stability fixes change behavior, document migration path
- Example: "RecLock now uses real locking (was no-op); code using RecLock as placeholder should be reviewed"

### 4.3 Validation Phase

**API Doc Completeness:**
- Grep for all `^type ` and `^func ` in Go files
- Verify each has a doc comment (starts with `// TypName` or `// FuncName`)
- Score: N/468 symbols documented

**README Examples:**
- Run test file `tests/readme_examples_test.prw`
- Expected: all pass
- Actual result: X/Y pass → update README if any fail

**Feature Matrix Cross-Check:**
- Sample 50 random features from matrix
- Verify code implements what matrix claims
- Score: N/50 correct

**Deliverables:**
- README updated and synced
- API docs complete (468/468 symbols)
- `FEATURE_MATRIX.md` created with all features
- `tests/readme_examples_test.prw` validating all examples
- Commits organized by scope (API docs, README, feature matrix, limitations)

---

## Part 5: Success Metrics & Integration

### 5.1 Success Criteria (Per Cycle)

**Security Cycle:**
- ✅ Zero OWASP Top 10 vulnerabilities (manual scan + fuzzing confirms)
- ✅ Zero CWE critical vulnerabilities (code review confirms)
- ✅ Fuzzing 1M+ iterations, zero crashes
- ✅ Code review of all changes (security focus)
- ✅ Relatório de postura publicado (what was found, fixed, residual)

**Stability Cycle:**
- ✅ Zero crashes on 300-file corpus
- ✅ 100% edge case matrix coverage (all combinations tested)
- ✅ All operations return error gracefully (no undefined behavior)
- ✅ Resource limits documented and enforced
- ✅ Matriz de cobertura publicada (edge cases tested)

**Documentation Cycle:**
- ✅ 468/468 API symbols documented
- ✅ README synchronized (zero major discrepancies)
- ✅ Feature matrix (implemented/parsed/unsupported) created and verified
- ✅ All examples in README validated (run and pass)
- ✅ Limitations disclosed (tetos, timeouts, incompletos)

### 5.2 Integration Validation

**Cross-Cycle Checks:**
- Do security fixes break any stability tests? (No)
- Do stability fixes conflict with security? (No)
- Does documentation match the corrected code? (Yes)
- Do all commits build and tests pass? (Yes)

**Final Report:**
- Consolidated findings (security, stability, documentation)
- Metrics dashboard (bugs found/fixed, coverage %, sync %)
- Recommendations for ongoing maintenance

---

## Part 6: Deliverables & Timeline

### Timeline by Cycle

**Cycle 1: Security (8-22 hours)**
- Discovery (4-6h): OWASP/CWE scan, fuzzing setup
- Correction (8-12h): implement fixes, add validation, timeouts
- Validation (2-3h): fuzzing, code review, posture report
- **Commits:** 5-10 commits by topic

**Cycle 2: Stability (17-25 hours)**
- Discovery (4-6h): crash mining, edge case analysis
- Correction (10-15h): null safety, bounds checking, resource limits, concurrency
- Validation (3-4h): corpus testing, edge case matrix
- **Commits:** 8-15 commits by topic

**Cycle 3: Documentation (11-17 hours)**
- Discovery (3-4h): README vs code audit, gap analysis
- Correction (6-10h): API docs (468), README sync, feature matrix, examples
- Validation (2-3h): validate examples, cross-check matrix
- **Commits:** 5-10 commits by scope

**Integration & Final (3-5 hours)**
- Cross-cycle validation
- Consolidated report
- Final review

**Total:** 40-60 hours of minuscule, rigorous work

### Deliverables by Cycle

**Security:**
- Commits: input validation, resource limits, error handling, crypto review
- Artifacts: fuzzing corpus, crash report, security posture report
- Evidence: 1M+ fuzzer iterations, code review sign-off

**Stability:**
- Commits: null safety, bounds checking, graceful errors, resource limits, concurrency fixes
- Artifacts: edge case coverage matrix, corpus test results, resource limit test results
- Evidence: 300-file corpus passes, all edge cases handled

**Documentation:**
- Commits: API docs (468 symbols), README sync, feature matrix, examples validation
- Artifacts: README (updated), FEATURE_MATRIX.md, tests/readme_examples_test.prw
- Evidence: 468/468 API docs, 100% README example pass rate, feature matrix verified

**Integration:**
- Consolidated report: security posture + stability coverage + doc sync status
- Recommendation for ongoing maintenance

---

## Part 7: Scope & Constraints

### In Scope
- All 3 cycles (security, stability, documentation)
- 32k-line codebase + 7.7k-line documentation
- 300-file OKF corpus for testing
- Fuzzing, code review, testing (as specified)

### Out of Scope (Explicit Non-Targets)
- New features (only fixing/documenting existing ones)
- Performance optimization (beyond security bounds checks)
- Unrelated refactoring (only refactor if needed for security/stability)

### Authorization
- Full rewrite/refactor of any component if needed for security/stability
- No requirement to preserve API compatibility (but document breaking changes)
- No time pressure; quality over speed

---

## Conclusion

This specification defines a **rigorous, integrated 3-cycle audit** of AdvPP v2.0.3, targeting market-grade stability. Each cycle is independent (allowing parallel investigation) but synchronized (preventing corrections from conflicting). Success is measured by concrete metrics: zero vulnerabilities, zero crashes, 100% documentation sync.

**Next Step:** User reviews this spec. Upon approval, we invoke writing-plans to decompose into actionable implementation tasks.
