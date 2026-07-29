# Stability Roadmap — Tasks 9–17 Implementation Plan

**Date:** 2026-07-29  
**Cycle:** Stability Cycle (Part 2 of Integral Audit)  
**Scope:** Transform edge case analysis into concrete fixes and validation  
**Goal:** 100% edge case coverage, 300-file corpus passes, zero crashes

---

## Phase Overview

```
Task 8 (This Task): Crash Mining & Analysis ✅
        ↓
Task 9:  Null Safety Audit (comprehensive)
Task 10: Bounds Checking (arrays, strings, numeric)
Task 11: Concurrency Fixes (locks, goroutines, shared state)
Task 12: Resource Exhaustion Tests (memory, stack, handles)
        ↓ (Validation phase)
Task 13: Corpus Validation (300-file OKF test)
Task 14: Edge Case Matrix (50 opcodes × 5 cases)
Task 15: Timeout & Deadlock Detection
Task 16: Error Path Testing (every error code)
Task 17: Stability Validation & Report
```

---

## Task 9: Null Safety Audit (Comprehensive)

**Duration:** 6–8 hours  
**Objective:** Extend v1.22.1 null guard fix to ALL dereferences  
**Output:** 20+ commits (one per null-prone area)

### 9.1: Audit Phase
- [ ] Grep for all `.Method()`, `.Field` access in `pkg/vm`, `pkg/runtime`
- [ ] Identify receivers that could be nil (from `Equals`, `Compare`, `String`, `Type`)
- [ ] Check if guard exists: `if receiver == nil { return ... }`
- [ ] Document 3+ null-prone paths

**Example findings:**
```go
// Current: may crash if val is nil
if val.Equals(nil) { ... }

// Fixed: guard first
if val == nil { return error or default }
if val.Equals(nil) { ... }
```

### 9.2: Fix Phase
- [ ] Add nil guards to `Value` interface methods (all 10+ implementations)
- [ ] Add nil guards to method calls on objects
- [ ] Add nil guards to field access chains
- [ ] Test each fix with unit test

**Areas to cover:**
1. `Equals` method on all Value types
2. `String` method on all Value types
3. `IsTruthy` method on all Value types
4. `Type` method on all Value types
5. Object property access (`.Field`, `[key]`)
6. Array element access (via receiver)
7. Method invocation (`:Method()`)
8. Assignment targets (lvalues)

### 9.3: Validation
- [ ] Run existing test suite (no regressions)
- [ ] Run new nil safety tests (`security_null_safety_test.prw` from Task 4)
- [ ] Corpus check: 300-file OKF passes
- [ ] Commit: `stability(9): extend nil guards to all Value dereferences`

**Test Coverage Target:** 20+ nil scenarios

---

## Task 10: Bounds Checking (Arrays, Strings, Numeric)

**Duration:** 8–10 hours  
**Objective:** Add validation at boundaries; prevent silent wrong answers  
**Output:** 30+ commits

### 10.1: Array Bounds

**Current State:**
```go
// Unchecked
result := array[idx]  // idx < 0 or idx >= len(array)?
```

**Fix:**
```go
if idx < 0 || idx >= len(array) {
    return &ErrorValue{Description: "array index out of bounds"}
}
result := array[idx]
```

**Operations to cover:**
- `aGet(array, idx)` — negative indices?
- `aSet(array, idx, value)` — extend or error?
- `aScan` — out of bounds in search?
- `aSort` — safe with huge arrays?
- Direct array access `array[idx]` in expressions

**Tests:**
- [ ] `TestArrayNegativeIndex`: `aGet(a, -1)` → error
- [ ] `TestArrayOOB`: `aGet(a, 1000)` → nil (safe)
- [ ] `TestArrayHuge`: 2M-element array → error after 1M
- [ ] `TestArrayModifyDuringIteration`: `aEval` modifies array
- [ ] Commit: `stability(10a): add array bounds validation`

### 10.2: String Bounds

**Current State:**
```go
// Unchecked
substr := SubStr(s, 1, 100)  // s only 10 chars?
```

**Fix:**
```go
start, length := validateStringBounds(s, start, length)
substr := s[start : start+length]
```

**Operations to cover:**
- `SubStr(s, n1, n2)` — negative start/length?
- `At(s, substr)` — empty substring?
- `Upper`, `Lower` on 10MB string
- String concatenation overflow

**Tests:**
- [ ] `TestStringNegativeStart`: `SubStr(s, -1, 5)` → error
- [ ] `TestStringEmptySearch`: `At(s, "")` → behavior
- [ ] `TestStringHugeConcat`: concat to 10MB+ → error
- [ ] Commit: `stability(10b): add string bounds validation`

### 10.3: Numeric Bounds

**Current State:**
```go
// Silent overflow to Inf
result := 1.8e308 + 1.8e308  // Becomes Inf, no error
```

**Fix:**
```go
if math.IsInf(result, 0) {
    return &ErrorValue{Description: "numeric overflow"}
}
```

**Operations to cover:**
- `+`, `-`, `*` overflow detection
- `/` by zero (already caught)
- `%` by zero (already caught)
- `**` (power) with huge exponents
- Logical operators on error values

**Tests:**
- [ ] `TestNumericOverflow`: `1.8e308 + 1.8e308` → error
- [ ] `TestNumericPower`: `2 ** 10000` → error or cap
- [ ] `TestNumericLogicalError`: error && .T. → propagate
- [ ] Commit: `stability(10c): add numeric overflow detection`

### 10.4: Input Validation (Parser/Lexer)

**Current State (after v1.8.7 fix):**
- NBSP corrupted identifiers (FIXED)
- Newline collapsing statements (FIXED in multiple commits)

**Remaining checks:**
- Line number consistency (same-line checks exist, audit coverage)
- Character encoding edge cases (NBSP, tab vs space)
- Comment state tracking (fixed in v1.8.5)

**Tests:**
- [ ] `TestEncodingNBSP`: NBSP handled as whitespace
- [ ] `TestParserSameLine`: `(` must be same line as function name
- [ ] `TestCommentState`: `#ifdef` inside `/* ... */` ignored
- [ ] Commit: `stability(10d): validate parser input consistency`

---

## Task 11: Concurrency Fixes (Locks, Goroutines, Shared State)

**Duration:** 6–8 hours  
**Objective:** Make shared resources thread-safe; prevent race conditions  
**Output:** 10+ commits

### 11.1: Database Locking

**Current State:**
- `RecLock` is a no-op (documented)
- `MsUnlock` is a no-op (documented)
- SQLite has `busy_timeout` but no table-level locking

**Fix (Priority: Low, documented as limitation):**
- Document that `RecLock`/`MsUnlock` are no-ops
- Suggest: use transactions + `BEGIN IMMEDIATE` for critical sections
- Tests: validate no silent data corruption (SQLite handles via WAL mode)

**Tests:**
- [ ] `TestRecLockNoOp`: Verify documented no-op behavior
- [ ] `TestConcurrentUpdate`: Two VMs update same table → last-write-wins
- [ ] Commit: `stability(11a): document locking limitations`

### 11.2: Goroutine Limits & Cleanup

**Current State (v2.0.3):**
- `StartJob` limited to 1000 concurrent
- No goroutine leak detection

**Fix:**
- Add goroutine cleanup validation in tests
- Monitor `runtime.NumGoroutine()` before/after job completion
- Add timeout for job completion

**Tests:**
- [ ] `TestStartJobLimit`: 1001 jobs → error after 1000
- [ ] `TestJobCompletion`: Goroutine count stable after jobs done
- [ ] `TestJobTimeout`: Long-running job with timeout
- [ ] Commit: `stability(11b): validate goroutine cleanup`

### 11.3: File Handle Resource Limits

**Current State:**
- No limit on open file handles
- FCreate/FOpen could exhaust system limit

**Fix (Priority: Medium):**
- Add `MaxOpenFiles = 100` constant (conservative, tunable)
- Track open handles; return error when limit reached
- Add cleanup test (ensure Close actually frees handle)

**Tests:**
- [ ] `TestFileHandleLimit`: Open 101 files → error on 101st
- [ ] `TestFileHandleCleanup`: Close handle → count decrements
- [ ] Commit: `stability(11c): add file handle limits`

### 11.4: Async Callback Safety

**Current State (v1.10.2 fix):**
- `MsgYesNo` uses channel-based sync (safe)
- UI callbacks properly serialize

**Validation:**
- Verify no race conditions in `Dialog.ShowAndRun()` + channel pattern
- Run tests under `-race` flag

**Tests:**
- [ ] `TestUICallbackSafety`: Dialog callback races caught
- [ ] `TestMsgYesNoCorrect`: Choice actually returned
- [ ] Commit: `stability(11d): validate UI callback thread safety`

### 11.5: Shared Bytecode Cache

**Current State:**
- Bytecode cache exists; may be shared across VMs
- No explicit locking

**Validation:**
- Verify cache is read-only after compilation
- Or add `sync.RWMutex` if updates are possible

**Tests:**
- [ ] `TestBytecodeRaceCondition`: Multiple VMs load cache simultaneously
- [ ] Commit: `stability(11e): audit bytecode cache concurrency`

---

## Task 12: Resource Exhaustion Tests (Memory, Stack, Handles)

**Duration:** 4–6 hours  
**Objective:** Verify limits are enforced; no silent OOM or hang  
**Output:** 8+ commits

### 12.1: Memory Limits

**Current State (v2.0.3):**
- Array size: 1M (enforced)
- String length: 10MB (enforced)
- Object properties: 10k (enforced)
- JSON nesting: 100 (enforced)

**Tests:**
- [ ] `TestArraySizeLimit`: 1M+1 elements → error
- [ ] `TestStringLengthLimit`: 10MB+1 bytes → error
- [ ] `TestObjectPropertyLimit`: 10k+1 properties → error
- [ ] `TestJSONNestingLimit`: 101 levels deep → error
- [ ] Commit: `stability(12a): validate memory limits enforced`

### 12.2: Stack Limits

**Current State (v2.0.3):**
- Recursion depth: 1000 (enforced)
- Call frames: 5000 (enforced)
- Stack size: 10000 (enforced)

**Tests:**
- [ ] `TestRecursionDepth`: 1001 levels → error at 1001
- [ ] `TestCallFrames`: 5001 nested calls → error
- [ ] Commit: `stability(12b): validate stack limits enforced`

### 12.3: Goroutine Limits

**Current State (v2.0.3):**
- StartJob: 1000 concurrent (enforced)

**Tests:**
- [ ] `TestStartJobLimit`: 1001 jobs → error on 1001st
- [ ] `TestJobPoolExhaustion`: Rapid job submissions don't leak
- [ ] Commit: `stability(12c): validate job limits enforced`

### 12.4: Timeout Validation

**New (not yet enforced):**
- LLM generate: timeout 5 minutes
- File I/O: timeout 30 seconds
- HTTP request: timeout 30 seconds
- DB query: timeout 30 seconds

**Tests:**
- [ ] `TestLLMTimeout`: Generate > 5 min → error
- [ ] `TestFileIOTimeout`: Hang on file I/O → error
- [ ] `TestHTTPTimeout`: Slow HTTP → timeout
- [ ] `TestDBTimeout`: Slow query → timeout
- [ ] Commit: `stability(12d): implement timeout validation`

---

## Task 13: Corpus Validation (300-file OKF Test)

**Duration:** 4–6 hours (depends on fix throughput from Tasks 9–12)  
**Objective:** Run 300-file OKF corpus; verify zero crashes  
**Output:** Corpus test results + 5+ commits (fixes for new crashes)

### 13.1: Corpus Check
```bash
advplc check tests/okf-corpus/*.prw
# Expected: 300/300 PASS (same as v2.0.3 baseline)
```

### 13.2: Identify New Crashes
- [ ] Any failures after Tasks 9–12 fixes
- [ ] Categorize by type (nil, bounds, recursion, etc.)
- [ ] Isolate minimal repro
- [ ] Fix and re-test

### 13.3: Document Pass Rate
- [ ] Before Task 9: 300/300 (baseline)
- [ ] After Task 9: 300/300 (nil safety shouldn't break anything)
- [ ] After Task 10: 300/300 (bounds should be non-breaking)
- [ ] After Task 11–12: 300/300 (resource limits enforced gracefully)

**Tests:**
- [ ] Corpus passes: 300/300 ✅
- [ ] No new crashes introduced
- [ ] All error paths graceful
- [ ] Commit: `stability(13): corpus validation complete (300/300)`

---

## Task 14: Edge Case Matrix (50 Opcodes × 5 Cases)

**Duration:** 8–12 hours  
**Objective:** Implement matrix from EDGE_CASE_MATRIX.md  
**Output:** 100+ test cases (unit tests)

### 14.1: Test Organization
```
tests/
  stability/
    edge_cases_test.prw      (AdvPL fixture file)
    op_*.prw                 (individual opcode tests)
cmd/advplc/
  edge_cases_matrix_test.go  (test harness in Go)
```

### 14.2: Prioritize Tests (P0 → P2)

**P0 (Must Have — 22 tests, ~4 hours)**
- Nil comparisons (8 types) — 8 tests
- Numeric bounds (overflow, div-by-zero) — 4 tests
- Array bounds (negative, OOB, huge) — 5 tests
- String bounds (negative, long) — 5 tests

**P1 (Should Have — 20 tests, ~6 hours)**
- Circular references (arrays, objects) — 6 tests
- Recursion depth (direct, mutual) — 6 tests
- Async safety (callback timing) — 4 tests
- Goroutine limits (start, cleanup) — 4 tests

**P2 (Nice to Have — 60+ tests, ~8 hours)**
- All remaining edge cases from EDGE_CASE_MATRIX.md
- Opcodes: arithmetic (10), array (10), string (10), object (10), recursion (6), concurrency (6), misc (8)

### 14.3: Test Template
```advpl
User Function TestNilComparison_Number()
    Local x  // Uninitialized
    Local lOk := .F.
    
    Begin Sequence
        If x == Nil
            lOk := .T.  // Should reach here, not crash
        EndIf
    Recover Using oErr
        ConOut("ERROR: " + oErr:Description)
    End Sequence
    
    Return lOk
```

### 14.4: Scoring
- [ ] P0 tests: 22/22 pass ✅ (baseline)
- [ ] P1 tests: 20/20 pass ✅ (after Tasks 9–12)
- [ ] P2 tests: 60+/60+ pass (aspirational)
- [ ] Commit: `stability(14): comprehensive edge case matrix (100+ tests)`

---

## Task 15: Timeout & Deadlock Detection

**Duration:** 4–6 hours  
**Objective:** Detect and prevent hangs; enforce timeouts  
**Output:** 6+ commits (one per timeout category)

### 15.1: Timeout Implementation

**Categories:**
- [ ] LLM generate: 5 minutes (already enforced via context.Context)
- [ ] File I/O: 30 seconds (new; implement with context.WithTimeout)
- [ ] HTTP request: 30 seconds (verify existing implementation)
- [ ] DB query: 30 seconds (verify SQLite timeout)
- [ ] Goroutine wait: 30 seconds (new; for StartJob)

**Tests:**
- [ ] `TestLLMGenTimeout`: Mock slow model → timeout
- [ ] `TestFileIOTimeout`: Hanging file operation → timeout
- [ ] `TestHTTPTimeout`: Slow server → timeout
- [ ] `TestDBTimeout`: Slow query → timeout
- [ ] `TestJobWaitTimeout`: Job doesn't complete → timeout
- [ ] Commit: `stability(15a): implement and validate timeouts`

### 15.2: Deadlock Detection

**Scenarios:**
- [ ] Circular lock dependencies (unlikely, but check)
- [ ] Goroutine blocked forever on channel (detect via timeout)
- [ ] Recursive try/catch with no escape (check depth limit)

**Tests:**
- [ ] `TestDeadlockDetection`: Monitor for goroutine hangs
- [ ] `TestChannelTimeout`: Blocked channel → timeout
- [ ] Commit: `stability(15b): deadlock detection and recovery`

---

## Task 16: Error Path Testing (Every Error Code)

**Duration:** 6–8 hours  
**Objective:** Exercise all error returns; verify graceful handling  
**Output:** 10+ commits

### 16.1: Error Classification

**Error Types:**
- Array bounds: 3 errors
- String bounds: 3 errors
- Numeric: 4 errors (overflow, div-by-zero, etc.)
- Object: 3 errors
- Concurrency: 5 errors (job limit, goroutine leak, etc.)
- I/O: 5 errors (file open, read, write)
- DB: 3 errors (connection, query, transaction)
- Parse/compile: 10+ errors

**Total:** 40+ distinct error codes

### 16.2: Test Each Error

**Template:**
```advpl
User Function TestErrorArrayOOB()
    Local a := {1, 2, 3}
    Local oErr := Nil
    
    Begin Sequence
        aGet(a, 999)  // Out of bounds
    Recover Using oErr
        // Verify error message
        Return "array" $ Lower(oErr:Description)
    End Sequence
    
    Return .F.  // Should not reach
```

### 16.3: Error Propagation

**Verify:**
- [ ] Errors captured by Try/Catch
- [ ] Errors don't crash VM
- [ ] Errors have descriptive message
- [ ] Errors include context (line number, function name)

**Tests:**
- [ ] 40+ error scenarios
- [ ] Each has Try/Catch block
- [ ] Commit: `stability(16): comprehensive error path testing (40+ errors)`

---

## Task 17: Stability Validation & Report

**Duration:** 4–6 hours  
**Objective:** Consolidate findings; produce final report  
**Output:** Final report + 2 commits

### 17.1: Cross-Cycle Validation

**Checklist:**
- [ ] Security fixes (Task 3–7) don't break stability
- [ ] Stability fixes (Task 9–16) don't break security
- [ ] No regressions in documentation (covered by Cycle 3)
- [ ] All 3 cycles align on requirements

**Validation:**
```bash
make test              # All unit tests pass
advplc check corpus/   # Corpus passes (300/300)
go test -race ./...    # No race conditions
```

### 17.2: Coverage Metrics

**Report:**
```
STABILITY CYCLE VALIDATION REPORT
===================================

Edge Case Coverage:
  - Basic types: 48/48 ✅ (Nil, Zero, Negative, Huge, Empty, Circular)
  - Arithmetic: 10/10 ✅
  - Arrays: 10/10 ✅
  - Strings: 10/10 ✅
  - Objects: 10/10 ✅
  - Recursion: 6/6 ✅
  - Concurrency: 6/6 ✅
  TOTAL: 100/100 ✅ (100% coverage)

Crash History:
  - v1.22.1 SIGSEGV: ✅ FIXED (v1.22.1)
  - v2.0.3 UI detection: ✅ FIXED (v2.0.3)
  - 11 known crashes: ✅ ALL FIXED
  - Regression risk: ✅ ZERO

Resource Limits:
  - Array size (1M): ✅ Enforced
  - String length (10MB): ✅ Enforced
  - Object properties (10k): ✅ Enforced
  - JSON nesting (100): ✅ Enforced
  - Recursion depth (1000): ✅ Enforced
  - Call frames (5000): ✅ Enforced
  - Concurrent jobs (1000): ✅ Enforced
  - File handles (100): ✅ Enforced
  - Timeouts (5m LLM, 30s I/O): ✅ Enforced

Test Results:
  - Corpus (300 files): 300/300 PASS ✅
  - Edge case matrix (100+ tests): PASS ✅
  - Error path tests (40+ errors): PASS ✅
  - Null safety tests (20+ scenarios): PASS ✅

Commits:
  - Task 9 (Null Safety): 20 commits
  - Task 10 (Bounds): 30 commits
  - Task 11 (Concurrency): 10 commits
  - Task 12 (Resources): 8 commits
  - Task 13 (Corpus): 5 commits
  - Task 14 (Matrix): 1 commit (100+ tests)
  - Task 15 (Timeouts): 2 commits
  - Task 16 (Errors): 1 commit (40+ tests)
  - Task 17 (Report): This commit
  TOTAL: 77 commits organized by topic
```

### 17.3: Lessons Learned

**Key Findings:**
1. Uninitialized variables are critical (v1.22.1 SIGSEGV)
   → Always initialize Value slots to Nil sentinel
2. Newline stripping creates drift bugs (v1.8.7 NBSP)
   → Preserve line numbers throughout parsing
3. Async callbacks need synchronization (v1.10.2 MsgYesNo)
   → Use channels, not bare goroutines for UI
4. Character encoding has surprises (NBSP corruption)
   → Test with real editor-inserted characters
5. Parser metadata (source location, include boundaries) is critical
   → Preserve during preprocessing

### 17.4: Recommendations for Ongoing Maintenance

**Quarterly:**
- [ ] Fuzz-test for 1 hour (lexer, parser, VM)
- [ ] Run corpus check (300 files)
- [ ] Run edge case matrix (100+ tests)

**Per Release:**
- [ ] Document any new crash fixed
- [ ] Update CRASH_HISTORY_ANALYSIS.md
- [ ] Add test case for each crash
- [ ] Cross-validate with Security & Documentation cycles

**Backlog:**
- Tail recursion optimization (not critical, blocked by architecture change)
- Inline Value representation (blocked by large refactor)
- Full-system memory limit (monitoring via cgroups, not critical)

### 17.5: Final Commit

```bash
git add docs/stability/
git commit -m "stability(17): comprehensive stability validation and report

Stability Cycle Complete:
  ✅ 11 known crashes analyzed and fixed
  ✅ 100+ edge cases catalogued and tested
  ✅ 300-file OKF corpus validates (0 crashes)
  ✅ Resource limits enforced (array, string, json, jobs, recursion)
  ✅ Null safety extended to all dereferences
  ✅ Bounds checking added (arrays, strings, numeric)
  ✅ Concurrency safety verified (goroutines, async callbacks)
  ✅ Error paths comprehensive (40+ error codes tested)
  ✅ Zero regressions (all tests pass on all platforms)

Coverage:
  - Edge cases: 100/100 ✅
  - Crash history: 11/11 FIXED ✅
  - Test matrix: 100+ scenarios ✅
  - Corpus: 300/300 PASS ✅

Ready for Documentation Cycle (Tasks 18–25)."
```

---

## Timeline Summary

| Task | Duration | Status | Dependencies |
|------|----------|--------|--------------|
| Task 8 | 4h | ✅ COMPLETE | (This task) |
| Task 9 | 6–8h | Planned | Task 8 ✅ |
| Task 10 | 8–10h | Planned | Task 8–9 |
| Task 11 | 6–8h | Planned | Task 8–10 |
| Task 12 | 4–6h | Planned | Task 8–11 |
| Task 13 | 4–6h | Planned | Task 9–12 |
| Task 14 | 8–12h | Planned | Task 9–12 |
| Task 15 | 4–6h | Planned | Task 9–12 |
| Task 16 | 6–8h | Planned | Task 9–15 |
| Task 17 | 4–6h | Planned | Task 9–16 |
| **TOTAL** | **54–80h** | Planned | Integration |

**Critical Path:** Task 9 → Task 10 → Task 13 (corpus validation gate)  
**Parallel Tracks:** Task 11–12, Task 14–15 can run in parallel with main path

---

## Success Criteria (Final)

✅ = MUST PASS

- [ ] **Zero crashes** on 300-file corpus (OKF real code)
- [ ] **100% edge case coverage** (all 100+ scenarios in EDGE_CASE_MATRIX.md)
- [ ] **All resource limits enforced** (array, string, json, recursion, jobs, timeouts)
- [ ] **Null safety comprehensive** (no nil dereferences possible)
- [ ] **Bounds checking complete** (arrays, strings, numeric operations)
- [ ] **Concurrency safe** (race-tested with `-race` flag)
- [ ] **Error paths graceful** (40+ error codes tested, never crashes)
- [ ] **No regressions** (all existing tests pass; no new failures)
- [ ] **Documentation current** (README reflects stability status; COMPONENT_STATUS updated)

**Definition of Success:** All 8 boxes checked = **STABILITY CYCLE COMPLETE** ✅

---

## Integration with Other Cycles

**Security Cycle (Tasks 1–7):** ✅ COMPLETE  
→ Stability cycle builds on security fixes (null guards from Task 4, resource limits from Task 5)

**Documentation Cycle (Tasks 18–25):** PLANNED  
→ Stability findings feed Documentation: update README (stability status), COMPONENT_STATUS (tested edge cases), FEATURE_MATRIX (unsupported features that would break on edge cases)

**Final Integration (Tasks 26–27):** PLANNED  
→ Cross-cycle validation: confirm security fixes don't break stability, stability fixes don't break security, documentation matches reality
