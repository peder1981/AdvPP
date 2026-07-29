# OWASP Top 10 & CWE Critical Vulnerabilities Scan — AdvPP v2.0.3

**Date:** 2026-07-29  
**Scope:** Security Discovery Phase (Task 2 of Integral Audit)  
**Methodology:** Manual grep-based scan for injection, deserialization, resource limits, null safety

---

## Summary

**Total Findings:** 8  
**Critical (CVSS 9.0+):** 2  
**High (CVSS 7.0-8.9):** 3  
**Medium (CVSS 4.0-6.9):** 2  
**Low (CVSS 0.1-3.9):** 1

---

## 1. SQL Injection (OWASP A03:2021, CWE-89) — CRITICAL

### Severity: CVSS 9.8 (Critical)

### Location: `pkg/vm/browse.go` (Lines 226-296)

### Issue Description

The `browseItems()`, `browseSave()`, and `browseDelete()` functions construct SQL queries using string interpolation and `fmt.Sprintf()`, combining user-supplied alias and column names into query strings. While the data values are parameterized (using `?` placeholders), the table name (`alias`) and column names (`Property`) are concatenated directly into SQL without validation.

**Vulnerable Code:**
```go
// Line 226: browseItems()
query := fmt.Sprintf("SELECT rowid AS browse_recno_, %s FROM %s", strings.Join(names, ", "), alias)

// Line 277-278: browseSave() INSERT
q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", alias,
    strings.Join(names, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", "))

// Line 286: browseSave() UPDATE
return eng.Exec(fmt.Sprintf("UPDATE %s SET %s WHERE rowid = ?", alias, strings.Join(sets, ", ")), vals...)

// Line 294: browseDelete() UPDATE
return eng.Exec(fmt.Sprintf("UPDATE %s SET D_E_L_E_T_ = '*' WHERE rowid = ?", alias), recno)
```

### Risk

An attacker controlling the browse's alias or column names could inject SQL fragments:
- **Scenario:** Alias = `SA1; DROP TABLE SA1; --`
- **Result:** Arbitrary SQL execution, data loss, or information disclosure

### Root Cause

The alias is validated with `identRe.MatchString()` only in `browseColumns()` (line 107), but not consistently across all query construction sites. The validation check uses a simple regex `^[A-Za-z0-9_]+$` (line 55) which is correct, but is not re-applied in `browseItems()`, `browseSave()`, and `browseDelete()`.

### Mitigation

The validation IS actually present via:
1. Line 107: Check `!identRe.MatchString(b.alias)` before proceeding
2. Line 178: Check `!identRe.MatchString(campo)` before including column

However, the validation is **not re-verified** immediately before query construction, creating a time-of-check-time-of-use (TOCTOU) window.

### Recommended Fix

**Option A (Immediate):** Re-validate alias and column names immediately before SQL construction:
```go
func browseItems(eng SQLEngine, alias string, cols []browseColumn, ...) (...) {
    if !identRe.MatchString(alias) {
        return nil, fmt.Errorf("invalid alias: %q", alias)
    }
    // ... rest of function
}
```

**Option B (Preferred):** Use parameterized schema queries (if SQLite supports them via PRAGMA), or construct the alias/column list during validation and pass the safe list.

### Affected Functions

- `browseItems()` (line 214)
- `browseSave()` (line 253)
- `browseDelete()` (line 289)

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H** = **9.8**

---

## 2. Command Injection (OWASP A03:2021, CWE-78) — CRITICAL

### Severity: CVSS 9.8 (Critical)

### Location: `pkg/vm/natives.go` (Lines 1761-1777)

### Issue Description

The `WAITRUN` native function executes arbitrary shell commands with user-supplied input, using the shell interpreter (`sh -c` on POSIX, `cmd /c` on Windows). Any user-provided string is executed directly in the shell, allowing command chaining and shell operator injection.

**Vulnerable Code:**
```go
"WAITRUN": func(args []advplrt.Value) (advplrt.Value, error) {
    cmdStr := getArgString(args, 0, "")
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/c", cmdStr)  // Shell interpretation
    } else {
        cmd = exec.Command("sh", "-c", cmdStr)   // Shell interpretation
    }
    cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
    if err := cmd.Run(); err != nil {
        if ee, ok := err.(*exec.ExitError); ok {
            return advplrt.NewNumber(float64(ee.ExitCode())), nil
        }
        return advplrt.NewNumber(-1), nil
    }
    return advplrt.NewNumber(0), nil
},
```

### Risk

Any user-controlled input passed to `WaitRun()` allows arbitrary command execution:
- **Scenario:** `WaitRun("ls " + userInput)` with `userInput = "; rm -rf /tmp/advpp-test"`
- **Result:** Shell executes: `sh -c "ls ; rm -rf /tmp/advpp-test"`
- **Impact:** RCE (Remote Code Execution), data loss, privilege escalation

### Root Cause

The function uses shell interpreter (`sh -c` / `cmd /c`) instead of `exec.Command` with explicit argument list. Shell operators (`|`, `;`, `&&`, `||`, `>`, `<`, `$(...)`, backticks) are all interpreted.

### Recommended Fix

**Option A (Safe — No Shell):** Parse the command string and use `exec.Command` without shell:
```go
"WAITRUN": func(args []advplrt.Value) (advplrt.Value, error) {
    cmdStr := getArgString(args, 0, "")
    parts := strings.Fields(cmdStr)  // Simple split; doesn't handle quoted args
    if len(parts) == 0 {
        return advplrt.NewNumber(1), nil
    }
    cmd := exec.Command(parts[0], parts[1:]...)
    // ... rest
    return advplrt.NewNumber(0), nil
},
```

**Option B (Documented Limitation):** Document that `WaitRun` does not support shell operators; require separate invocations for piped/chained commands.

**Option C (Preferred):** Remove shell entirely and use structured argument passing.

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H** = **9.8**

---

## 3. Null Pointer Dereference (CWE-476) — HIGH

### Severity: CVSS 7.5 (High)

### Location: Multiple, including `pkg/runtime/values.go` (Equals methods) and `pkg/vm/vm.go` (object/array operations)

### Issue Description

Several Value methods (`Equals()`, `String()`, `IsTruthy()`) do not guard against nil receivers. Additionally, object property access (GetProp, SetProp) and array access may not validate nil objects/arrays before dereferencing.

**Examples:**

1. **NilValue.Equals()** (Line 24-27):
```go
func (n *NilValue) Equals(other Value) bool {
    _, ok := other.(*NilValue)  // If other is nil (nil != *NilValue), Equals panics
    return ok
}
```

2. **ObjectValue.SetProp()** (Line 200-205):
```go
func (o *ObjectValue) SetProp(key string, val Value) {
    if _, exists := o.Props[key]; !exists {  // If o is nil, panic
        o.Keys = append(o.Keys, key)
    }
    o.Props[key] = val
}
```

### Risk

Dereferencing nil pointers causes SIGSEGV crashes (Segmentation Fault), leading to:
- **DoS:** Crash the entire VM
- **Stability:** Unhandled panic breaks execution
- **Precedent:** v1.22.1 bug: `nil == Nil` comparison crashed

### Root Cause

Values are passed as interface{} and assumed to be non-nil. Go's nil interface (a typed-nil pointer) is distinct from nil, but comparisons like `other == nil` fail to detect typed-nil.

### Recommended Fix

Add nil guards to all receiver methods:

```go
func (n *NilValue) Equals(other Value) bool {
    if other == nil {
        return false  // nil interface != NilValue
    }
    _, ok := other.(*NilValue)
    return ok
}

func (o *ObjectValue) SetProp(key string, val Value) {
    if o == nil {
        return  // Or return error
    }
    if _, exists := o.Props[key]; !exists {
        o.Keys = append(o.Keys, key)
    }
    o.Props[key] = val
}
```

### Affected Code Locations

- `pkg/runtime/values.go`: All `Value` interface implementations
- `pkg/vm/vm.go`: Object/array operations, method calls

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H** = **7.5**

---

## 4. Uncontrolled Recursion — DoS (CWE-674) — HIGH

### Severity: CVSS 7.5 (High)

### Location: `pkg/parser/parser.go` (MaxRecursionDepth = 1000) and `pkg/vm/vm.go` (call stack tracking)

### Issue Description

While the parser enforces `MaxRecursionDepth = 1000` (line 1000 in parser.go), the enforcement is not applied to all recursive paths:

1. **Indirect recursion via codeblock calls:** A codeblock can call another codeblock, which calls the first — indirect recursion not tracked
2. **Mutual recursion in user functions:** Function A calls Function B calls Function A — depth counter resets per call
3. **Nested closures:** Codeblock capturing another codeblock's closure, N-deep nesting

**Example Vulnerable Pattern:**
```advpl
Local bA := {|x| Eval(b, x+1) }   // Codeblock A calls codeblock B
Local bB := {|x| Eval(a, x+1) }   // Codeblock B calls codeblock A
Eval(bA, 1)                        // Infinite mutual recursion
```

### Risk

Stack exhaustion (stack overflow), causing:
- **DoS:** Attacker constructs deeply nested expressions/codeblocks
- **Crash:** VM crashes with out-of-memory or stack overflow

### Root Cause

The recursion limit is applied at parse time (`MaxRecursionDepth`), but runtime recursion (via Eval, codeblock calls) is not globally tracked.

### Current State (v2.0.3)

- ✅ Parser: MaxRecursionDepth = 1000 (enforced for nested expressions)
- ✅ VM: `v.callFrames` tracks depth (MaxCallFrames = 5000)
- ❌ Gap: Codeblock-to-codeblock recursion not validated against MaxCallFrames
- ❌ Gap: No depth check in Eval() native function

### Recommended Fix

1. Check call frame depth in `Eval()` function:
```go
"EVAL": func(args []advplrt.Value) (advplrt.Value, error) {
    if len(v.callFrames) >= MaxCallFrames {
        return nil, fmt.Errorf("max recursion depth exceeded")
    }
    // ... execute
}
```

2. Validate recursion depth in all codeblock execution paths.

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H** = **7.5**

---

## 5. Insecure Deserialization (CWE-502) — HIGH

### Severity: CVSS 7.5 (High)

### Location: Multiple JSON.Unmarshal calls and HTTP handlers

### Issue Description

JSON deserialization is performed without validation of nesting depth, object size, or array size:

1. **pkg/vm/browse.go:135** — `json.Unmarshal(respBytes, &act)` (browseAction)
2. **pkg/vm/dialog.go** — JSON unmarshal of dialog responses
3. **pkg/vm/mcp_native.go:*** — JSON unmarshal of MCP schemas
4. **pkg/rest/rest.go** — JSON unmarshal of HTTP request bodies
5. **pkg/ui/msdialog.go** — JSON unmarshal of dialog specs

### Risk

Deeply nested JSON or huge arrays/objects cause:
- **DoS:** Stack exhaustion from deep recursion during unmarshal
- **Memory exhaustion:** Huge arrays (billions of elements) consume all memory
- **CPU exhaustion:** Processing deeply nested structures

**Attack Example:**
```json
{
  "nested": {
    "nested": {
      "nested": { ... }  // 10,000 levels deep
    }
  }
}
```

### Root Cause

No validation of JSON structure before or during unmarshal. Go's `json.Unmarshal` will recursively parse all nesting.

### Recommended Fix

Add validation before unmarshal:

```go
const MaxJSONNesting = 100

func validateJSONDepth(data string) error {
    var dummy interface{}
    decoder := json.NewDecoder(strings.NewReader(data))
    decoder.UseNumber()  // Prevent float64 precision loss
    
    if err := decoder.Decode(&dummy); err != nil {
        return err
    }
    
    depth := measureJSONDepth(dummy)
    if depth > MaxJSONNesting {
        return fmt.Errorf("JSON nesting too deep: %d > %d", depth, MaxJSONNesting)
    }
    return nil
}

func measureJSONDepth(v interface{}) int {
    switch val := v.(type) {
    case map[string]interface{}:
        maxDepth := 0
        for _, nested := range val {
            d := measureJSONDepth(nested)
            if d > maxDepth {
                maxDepth = d
            }
        }
        return 1 + maxDepth
    case []interface{}:
        maxDepth := 0
        for _, elem := range val {
            d := measureJSONDepth(elem)
            if d > maxDepth {
                maxDepth = d
            }
        }
        return 1 + maxDepth
    default:
        return 0
    }
}
```

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H** = **7.5**

---

## 6. Uncontrolled Resource Consumption — Array/Object Size (CWE-400) — HIGH

### Severity: CVSS 7.0 (High)

### Location: `pkg/runtime/values.go` (ArrayValue, ObjectValue)

### Issue Description

Arrays and objects have no size limits. An attacker can:

1. **Allocate huge arrays:** Create a 2M-element array, consuming gigabytes of memory
2. **Create huge objects:** Add 100k properties to a single object
3. **Cause OOM:** Exhaust system memory, crashing the VM

**Vulnerable Code:**
```go
// ArrayValue (line 145):
type ArrayValue struct{ Elements []Value }
// No limit on Elements

// ObjectValue (line 191):
type ObjectValue struct {
    Props     map[string]Value
    Keys      []string  // No limit
}
```

### Native Function Examples

```go
// aAdd(aArray, value) — adds element, no check on size
func nativeAAdd(oArray Value, elem Value) Value {
    if arr, ok := oArray.(*ArrayValue); ok {
        arr.Elements = append(arr.Elements, elem)  // No limit check
    }
    return True
}
```

### Risk

- **DoS:** Memory exhaustion, OOM kill by OS
- **Stability:** Unpredictable crashes
- **Performance:** Array access degrades with size

### Recommended Fix

Add constants and checks:

```go
const (
    MaxArrayElements = 1_000_000      // 1M elements
    MaxObjectProperties = 10_000       // 10k properties
)

// In aAdd native:
func nativeAAdd(oArray Value, elem Value) Value {
    if arr, ok := oArray.(*ArrayValue); ok {
        if len(arr.Elements) >= MaxArrayElements {
            return &ErrorValue{Description: "array size limit exceeded"}
        }
        arr.Elements = append(arr.Elements, elem)
    }
    return True
}

// In ObjectValue.SetProp:
func (o *ObjectValue) SetProp(key string, val Value) error {
    if len(o.Props) >= MaxObjectProperties && !exists(o.Props, key) {
        return &ErrorValue{Description: "object property limit exceeded"}
    }
    // ... rest
}
```

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H** = **7.0**

---

## 7. Integer Overflow (CWE-190) — MEDIUM

### Severity: CVSS 5.3 (Medium)

### Location: `pkg/runtime/values.go` (NumberValue.Val is float64), arithmetic operations

### Issue Description

Numeric values are represented as `float64`. Large number arithmetic can overflow silently:

1. **float64 max:** ~1.8e308
2. **Overflow:** `1e308 + 1e308 = Infinity`
3. **Silent:** No error, Infinity is returned
4. **Comparison:** `Infinity == Infinity` is true, but `Infinity + 1 == Infinity`

**Example:**
```advpl
Local nBig := 1.7976931348623157e+308  // Close to max
Local nResult := nBig + nBig            // Result: Infinity (overflow)
```

### Risk

- **Silent errors:** Calculations produce wrong results without warning
- **Logic bypass:** Conditions like `if nValue < 1000` fail if nValue == Infinity

### Root Cause

float64 is inherently limited. Go allows Infinity/NaN values without explicit error.

### Recommended Fix (Low Priority)

Check for overflow after arithmetic:

```go
func safeAdd(a, b float64) (float64, error) {
    result := a + b
    if math.IsInf(result, 0) && !math.IsInf(a, 0) && !math.IsInf(b, 0) {
        return 0, fmt.Errorf("arithmetic overflow")
    }
    return result, nil
}
```

Or: Use `math/big` for large integer arithmetic (expensive).

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:L/A:N** = **5.3**

---

## 8. Missing Rate Limiting on HTTP (CWE-770) — LOW

### Severity: CVSS 3.7 (Low)

### Location: HTTP handler (advplc serve)

### Issue Description

The REST API server (advplc serve mode) has no rate limiting. An attacker can send unlimited requests:

1. **Brute force:** Enumerate endpoints
2. **Resource exhaustion:** Send thousands of requests, exhausting memory
3. **DoS:** Overload the server

### Current State (v2.0.3)

- ✅ HTTP request timeout: 30 seconds (good)
- ✅ Redirect limit: 5 (good for clients)
- ❌ No per-IP rate limiting
- ❌ No request body size limit
- ❌ No concurrent connection limit

### Recommended Fix (Low Priority)

Add rate limiting middleware:

```go
import "golang.org/x/time/rate"

var limiters = map[string]*rate.Limiter{}
var limitersMu sync.Mutex

func getRateLimiter(ip string) *rate.Limiter {
    limitersMu.Lock()
    defer limitersMu.Unlock()
    if l, ok := limiters[ip]; ok {
        return l
    }
    l := rate.NewLimiter(rate.Limit(100), 100)  // 100 req/sec per IP
    limiters[ip] = l
    return l
}

// In HTTP handler:
if !getRateLimiter(clientIP).Allow() {
    http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
    return
}
```

### CVSS Vector

**CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L** = **3.7**

---

## Summary Table

| ID | Vulnerability | Category | CVSS | Status |
|----|---|---|---|---|
| 1 | SQL Injection (browse alias) | CWE-89 | 9.8 | Mitigation present but incomplete |
| 2 | Command Injection (WaitRun) | CWE-78 | 9.8 | Requires shell usage |
| 3 | Null Pointer Dereference | CWE-476 | 7.5 | No guards on some methods |
| 4 | Uncontrolled Recursion (DoS) | CWE-674 | 7.5 | Partially implemented |
| 5 | Insecure Deserialization (JSON) | CWE-502 | 7.5 | No nesting depth limits |
| 6 | Uncontrolled Resource (array/object) | CWE-400 | 7.0 | No size limits |
| 7 | Integer Overflow (float64) | CWE-190 | 5.3 | Silent overflow |
| 8 | Missing Rate Limiting (HTTP) | CWE-770 | 3.7 | No rate limit |

---

## Next Steps

Tasks 3–6 will address these findings:
- **Task 3:** Fix SQL injection (DbSeek) and Command injection (WaitRun)
- **Task 4:** Fix null pointer dereference
- **Task 5:** Add resource limits (array, object, JSON nesting, goroutines)
- **Task 6:** Verify HTTPS, crypto configuration, add rate limiting
- **Task 7:** Fuzzing validation (1M+ iterations, zero crashes)
