# Security Vulnerabilities — Priority List (AdvPP v2.0.3)

> **✅ RESOLVIDO.** Todos os 8 achados abaixo foram corrigidos na mesma
> janela de auditoria (2026-07-29) — ver
> [`CODE_REVIEW_SIGN_OFF.md`](CODE_REVIEW_SIGN_OFF.md) para o sign-off e
> evidência de cada fix. Este arquivo é o registro histórico do scan
> inicial (pré-fix); os campos "Status: Open/Pending" abaixo refletem o
> estado NO MOMENTO DO SCAN, não o estado atual do código. Confirmado
> nesta auditoria (v3.0.2): `pkg/vm/browse.go` usa `identRe.MatchString`
> + queries parametrizadas em `browseSave`/`browseDelete`, e
> `pkg/vm/natives.go` executa comandos via `exec.Command(parts[0],
> parts[1:]...)` sem shell (sem RCE via `/bin/sh -c`).

**Date:** 2026-07-29  
**Total Findings:** 8  
**Scan Date:** 2026-07-29 (Task 2: OWASP/CWE Manual Scan)

---

## Critical (CVSS 9.0+) — Fix Immediately

### 1. SQL Injection in FWMBrowse (CWE-89) — CVSS 9.8

| Property | Value |
|----------|-------|
| **File** | `pkg/vm/browse.go` |
| **Functions** | `browseItems()`, `browseSave()`, `browseDelete()` |
| **Lines** | 214–296 |
| **Severity** | Critical — Data loss, information disclosure |
| **Attack Vector** | Network (SQL injection via browse alias) |
| **Status** | Open |

**Issue:** SQL table names (alias) and column names concatenated into query without re-validation immediately before execution. Validation check exists in `browseColumns()` but not re-applied at query construction.

**Fix:** Re-validate alias/column names with `identRe.MatchString()` immediately before each SQL operation, or use parameterized table/column schema (if supported).

**Task:** Task 3 (Security Fix — SQL Injection & Command Injection)

---

### 2. Command Injection in WaitRun (CWE-78) — CVSS 9.8

| Property | Value |
|----------|-------|
| **File** | `pkg/vm/natives.go` |
| **Function** | `WAITRUN()` native |
| **Lines** | 1761–1777 |
| **Severity** | Critical — Remote Code Execution (RCE) |
| **Attack Vector** | Network (user input to WaitRun) |
| **Status** | Open |

**Issue:** Shell interpreter used (`sh -c`, `cmd /c`) allows arbitrary command chaining and shell operators (`|`, `;`, `&&`, `||`).

**Fix:** 
- **Option A:** Remove shell; parse command string and use `exec.Command(parts[0], parts[1:]...)` without shell.
- **Option B:** Document limitation; require users to avoid shell operators.

**Task:** Task 3 (Security Fix — SQL Injection & Command Injection)

---

## High (CVSS 7.0–8.9) — Fix Before Stable Release

### 3. Null Pointer Dereference (CWE-476) — CVSS 7.5

| Property | Value |
|----------|-------|
| **File** | `pkg/runtime/values.go` (multiple methods) |
| **Locations** | `NilValue.Equals()`, `ObjectValue.SetProp()`, array/object operations |
| **Severity** | High — DoS via SIGSEGV crash |
| **Attack Vector** | Network (malformed input causing nil ops) |
| **Status** | Open |

**Issue:** Value methods and object/array operations assume non-nil receivers. Nil interface (typed-nil pointer) not properly guarded.

**Fix:** Add nil checks to all receiver methods and collection operations.

**Task:** Task 4 (Security Fix — Null Safety & Graceful Error Handling)

---

### 4. Uncontrolled Recursion — DoS (CWE-674) — CVSS 7.5

| Property | Value |
|----------|-------|
| **File** | `pkg/parser/parser.go`, `pkg/vm/vm.go` |
| **Locations** | Parser (MaxRecursionDepth=1000), VM (Eval, codeblock calls) |
| **Severity** | High — Stack exhaustion, DoS |
| **Attack Vector** | Network (deeply nested expressions/codeblocks) |
| **Status** | Partially Mitigated |

**Issue:** 
- ✅ Parser enforces MaxRecursionDepth = 1000
- ✅ VM tracks call frames (MaxCallFrames = 5000)
- ❌ Codeblock-to-codeblock recursion not validated against MaxCallFrames
- ❌ No depth check in Eval() native

**Fix:** Add recursion depth validation in Eval() and all codeblock execution paths.

**Task:** Task 4–5 (Resource Limits & Recursion Handling)

---

### 5. Insecure Deserialization (CWE-502) — CVSS 7.5

| Property | Value |
|----------|-------|
| **Files** | `pkg/vm/browse.go`, `pkg/vm/dialog.go`, `pkg/vm/mcp_native.go`, `pkg/rest/rest.go`, `pkg/ui/msdialog.go` |
| **Severity** | High — Stack exhaustion, memory exhaustion DoS |
| **Attack Vector** | Network (deeply nested or huge JSON payloads) |
| **Status** | Open |

**Issue:** JSON deserialization without validation of nesting depth or object/array size. Deeply nested JSON (10k+ levels) causes stack overflow. Huge arrays cause OOM.

**Fix:** Validate JSON structure before unmarshal:
- Max nesting depth: 100 levels
- Max object keys: 10k
- Max array elements: 1M

**Task:** Task 5 (Security Fix — Resource Limits)

---

### 6. Uncontrolled Resource Consumption — Array/Object Size (CWE-400) — CVSS 7.0

| Property | Value |
|----------|-------|
| **File** | `pkg/runtime/values.go` |
| **Structs** | `ArrayValue`, `ObjectValue` |
| **Severity** | High — Memory exhaustion, OOM |
| **Attack Vector** | Network (create huge arrays/objects) |
| **Status** | Open |

**Issue:** No limits on array elements or object properties. Attacker can allocate 2M-element arrays or 100k-property objects, exhausting memory.

**Fix:** Enforce limits:
- MaxArrayElements: 1,000,000
- MaxObjectProperties: 10,000

**Task:** Task 5 (Security Fix — Resource Limits)

---

## Medium (CVSS 4.0–6.9) — Fix in Next Release

### 7. Integer Overflow (CWE-190) — CVSS 5.3

| Property | Value |
|----------|-------|
| **File** | `pkg/runtime/values.go` |
| **Type** | `NumberValue.Val` (float64) |
| **Severity** | Medium — Silent arithmetic errors |
| **Attack Vector** | Network (large number calculations) |
| **Status** | Open |

**Issue:** float64 arithmetic overflows silently to Infinity/NaN. No error indication.

**Example:**
```
1.7976931348623157e+308 + 1.7976931348623157e+308 = Infinity
```

**Fix:** Check for Infinity after arithmetic operations; return ErrorValue.

**Prioritize:** Lower (affects rare edge cases)

**Task:** Task 5–6 (Arithmetic validation)

---

## Low (CVSS 0.1–3.9) — Fix in Future Hardening Pass

### 8. Missing Rate Limiting (CWE-770) — CVSS 3.7

| Property | Value |
|----------|-------|
| **Component** | HTTP REST server (advplc serve) |
| **Severity** | Low — DoS via request flooding |
| **Attack Vector** | Network (unlimited HTTP requests) |
| **Status** | Open |

**Issue:** No per-IP rate limiting, no request body size limit, no connection limit.

**Fix:** Implement rate limiter middleware (100 req/sec per IP).

**Prioritize:** Lower (can be added post-launch)

**Task:** Task 6 (HTTP hardening)

---

## Remediation Timeline

| Task | Vulnerabilities | ETA | Status |
|------|---|---|---|
| **Task 1** | Setup fuzzing infrastructure | ✅ | Complete |
| **Task 2** | Manual OWASP/CWE scan (this doc) | ✅ | Complete |
| **Task 3** | Fix SQLi & RCE (critical) | Next | Pending |
| **Task 4** | Fix null safety (high) | Next | Pending |
| **Task 5** | Add resource limits (high/medium) | Next | Pending |
| **Task 6** | Verify crypto & HTTP (medium/low) | Next | Pending |
| **Task 7** | Fuzzing validation (1M+ iterations) | Next | Pending |

---

## Test Cases (To Be Implemented in Tasks 3-6)

### SQLi Test Case
```advpl
User Function TestSQLInjectionDbSeek()
    Local cInjection := "1' OR '1'='1"
    DbSelectArea("SA1")
    DbSeek("A1_COD", cInjection)
    // Should NOT execute arbitrary SQL
Return .T.
```

### RCE Test Case
```advpl
User Function TestCommandInjectionWaitRun()
    Local nRet := WaitRun("echo test; rm -rf /tmp/advpp-test")
    // Should NOT execute second command
Return nRet == 0
```

### Null Safety Test Case
```advpl
User Function TestNilComparison()
    Local oNil := Nil
    // Should NOT crash
    If oNil == Nil
        Return .T.
    EndIf
Return .F.
```

### Resource Limit Test Case
```advpl
User Function TestArraySizeLimit()
    Local aArr := {}
    Local i := 0
    For i := 1 To 2_000_000
        If !aAdd(aArr, i)
            Return .T.  // Limit enforced
        EndIf
    Next
Return .F.
```

---

## Compliance Notes

**OWASP Top 10 (2021):** Covers A03:2021 (Injection), A06:2021 (Vulnerable Components)

**CWE Critical List:**
- CWE-89 (SQL Injection)
- CWE-78 (OS Command Injection)
- CWE-476 (Null Pointer Dereference)
- CWE-190 (Integer Overflow)
- CWE-674 (Uncontrolled Recursion)
- CWE-502 (Insecure Deserialization)
- CWE-400 (Uncontrolled Resource Consumption)
- CWE-770 (Missing Rate Limiting)

**Security Baseline:** All findings documented and prioritized for remediation.

---

**Document Owner:** Security Audit (Task 2)  
**Last Updated:** 2026-07-29  
**Status:** OPEN (Awaiting fixes in Tasks 3–6)
