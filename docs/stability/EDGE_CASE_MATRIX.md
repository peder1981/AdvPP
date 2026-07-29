# Edge Case Comprehensive Matrix — AdvPP v2.0.3

**Date:** 2026-07-29  
**Purpose:** Catalog all edge cases by data type and operation to guide test coverage (Tasks 13–15)  
**Target:** 240+ test scenarios (8 types × 5+ cases × 6+ operations)

---

## Executive Summary

**Data Types Covered:** 8 (Number, String, Array, Object, Date, CodeBlock, Class, Function)  
**Edge Cases Per Type:** 5–6 (Nil, Zero, Negative, Huge, Empty, Circular)  
**Operations Tested:** 40+ (arithmetic, array access, string operations, object access, recursion, concurrency)  
**Estimated Test Count:** 240+ scenarios

**Critical Gaps:**
- Array bounds checking (access with negative/huge indices)
- String operations on empty strings and very long strings
- Numeric overflow detection
- Circular reference handling
- Deep recursion limits

---

## Data Types × Edge Cases (8 × 6 = 48 Base Combinations)

### Type: Number (float64)

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil** | `x == Nil` (uninitialized) | No crash; return error or .F. | ✅ Fixed v1.22.1 | Low | `TestNumberNilComparison` |
| **Zero** | `n = 0` in all contexts | Safe; identity element for `+` | ✅ Safe | Low | `TestNumberZero` |
| **Negative** | `n = -1, -999` | Safe; normal arithmetic | ✅ Safe | Low | `TestNumberNegative` |
| **Huge** | `n = 1.8e308` (float64 max) | Silent overflow to Inf | ⚠️ Not detected | Medium | `TestNumberOverflow` |
| **Division by Zero** | `n / 0, n % 0` | Return error, not crash | ✅ Safe (caught) | Low | `TestDivByZero` |
| **Very Negative** | `n = -1.8e308` | Symmetric to huge | ⚠️ Not detected | Medium | `TestNumberNegativeOverflow` |

**Detected Gaps:**
- Overflow to `Inf` is silent (no error)
- No saturation (should cap at max rather than wrap to Inf)
- Tests: Task 10 (Task 13 validates)

---

### Type: String (UTF-8, CP-1252 conversion)

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil** | `c == Nil` (uninitialized) | No crash; return error | ✅ Fixed v1.22.1 | Low | `TestStringNilComparison` |
| **Empty** | `c = ""` | Safe; length = 0 | ✅ Safe | Low | `TestStringEmpty` |
| **Very Long** | `c = "x"*10_000_000` (10MB) | Reject with error | ✅ Limited to 10MB (v2.0.3) | Low | `TestStringHuge` |
| **Unicode (UTF-8)** | `c = "José"` | Safe; byte count ≠ char count | ✅ Safe | Low | `TestStringUnicode` |
| **NBSP (0xC2 0xA0)** | Editor-inserted non-breaking space | Tokenize as whitespace, not corruption | ✅ Fixed v1.8.7 | Low | `TestStringNBSP` |
| **Null Bytes** | `c = "abc\x00def"` | Safe; treat as data | ✅ Safe | Low | `TestStringNullBytes` |

**Detected Gaps:**
- String operations (SubStr, At, Upper, Lower) on boundary cases not fully tested
- UTF-8 multi-byte at string boundaries
- Tests: Task 10 (Task 13 validates)

---

### Type: Array

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil** | `a == Nil` (uninitialized) | No crash; return error | ✅ Fixed v1.22.1 | Low | `TestArrayNilComparison` |
| **Empty** | `a = {}` | Safe; length = 0 | ✅ Safe | Low | `TestArrayEmpty` |
| **Out-of-Bounds (positive)** | `aGet(a, 999)` in 10-elem array | Return nil, not crash | ✅ Safe | Low | `TestArrayOOB` |
| **Out-of-Bounds (negative)** | `aGet(a, -1)` | Return nil, not crash | ⚠️ Not tested | Medium | `TestArrayNegativeIndex` |
| **Huge Array** | `aAdd(a, x)` 1M+ times | Reject after 1M (v2.0.3 limit) | ✅ Limited | Low | `TestArrayHuge` |
| **Sparse Array** | Gaps in indices (1, 5, 10) | Handle gracefully | ✅ Safe | Low | `TestArraySparse` |
| **Circular Reference** | `a[1] := a` | Detect cycle; no infinite loop | ⚠️ Not detected | High | `TestArrayCircular` |

**Detected Gaps:**
- Negative indices not fully validated
- Circular references in arrays not detected
- Array operations on huge arrays (2M+) not tested
- Tests: Task 10–11 (Task 13 validates)

---

### Type: Object (JsonObject, custom classes)

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil** | `o == Nil` | No crash; return .T. | ✅ Safe | Low | `TestObjectNilComparison` |
| **No Property** | `o:NonExistent` | Return nil, not crash | ✅ Safe | Low | `TestObjectMissingProp` |
| **Huge Property Count** | 10k+ properties | Reject after limit (v2.0.3) | ✅ Limited to 10k | Low | `TestObjectHugeProps` |
| **Case Sensitivity** | `o:Prop` vs `o:prop` | Case-sensitive (v1.12 fix) | ✅ Fixed | Low | `TestObjectCaseSensitive` |
| **Circular Reference** | `o:Self := o` | Detect cycle; no infinite loop | ⚠️ Not detected | High | `TestObjectCircular` |
| **Method Call on Nil** | `nil:Method()` | Return error, not crash | ✅ Safe (v1.22 fix) | Low | `TestObjectNilMethod` |

**Detected Gaps:**
- Circular references in objects (especially serialization)
- Nested object depth limits
- Tests: Task 11 (Task 13 validates)

---

### Type: Date

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil/Zero Date** | `d = NIL` or `d = CtoD("")` | Safe; recognize as nil date | ✅ Safe | Low | `TestDateNil` |
| **Min/Max Year** | `CtoD("01/01/0001")`, `CtoD("12/31/9999")` | Within safe range | ✅ Safe | Low | `TestDateBoundary` |
| **Invalid Date** | `CtoD("99/99/9999")` | Return error, not crash | ✅ Safe | Low | `TestDateInvalid` |
| **Comparison** | `d1 < d2` | Lexicographic by YYYYMMDD | ✅ Safe | Low | `TestDateComparison` |
| **Arithmetic** | `d + 1` (add days) | Add days correctly | ✅ Safe | Low | `TestDateArithmetic` |
| **String Conversion** | `DtoC(d)` on edge dates | Handle year boundaries | ⚠️ Not tested | Low | `TestDateConversion` |

**Detected Gaps:**
- Leap year handling in date arithmetic
- Timezone handling (not applicable; dates are local)
- Tests: Task 10 (Task 13 validates)

---

### Type: CodeBlock (Closure)

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil** | `b == Nil` | No crash; return error | ✅ Safe | Low | `TestBlockNilComparison` |
| **Simple Block** | `{&#124; x + 1}` | Execute normally | ✅ Safe | Low | `TestBlockSimple` |
| **Nested Block** | `{&#124; {&#124; x + 1} }` | Handle nested closures | ✅ Safe | Low | `TestBlockNested` |
| **Capture External** | `{&#124; y + 1}` where y is Local | Capture by reference | ✅ Safe | Low | `TestBlockCapture` |
| **Deep Recursion** | `{&#124; SelfCall()}` 1000+ levels | Reject after 1000 depth | ✅ Limited (v2.0.3) | Low | `TestBlockRecursion` |
| **Circular Call** | `b1 := {&#124; b2()} ; b2 := {&#124; b1()}` | Detect infinite recursion | ✅ Limited by depth | Medium | `TestBlockCircularCall` |

**Detected Gaps:**
- Closure capture of external variables (upvalues) may have subtle bugs
- Mutual recursion between blocks may exceed depth limit
- Tests: Task 9–11 (Task 13 validates)

---

### Type: Class

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil Instance** | `o == Nil` | No crash; return .T. | ✅ Safe | Low | `TestClassNilInstance` |
| **Method Not Found** | `o:NonExistent()` | Return error, not crash | ✅ Safe | Low | `TestClassMissingMethod` |
| **Inheritance Chain** | `class C from B from A` 10+ levels | Handle deep inheritance | ⚠️ Not tested | Medium | `TestClassInheritanceDepth` |
| **Circular Inheritance** | `class A from B; class B from A` | Reject with error, not hang | ⚠️ Not validated | High | `TestClassCircularInheritance` |
| **Property Access** | `o:Prop := value` on nil object | Return error, not crash | ✅ Safe | Low | `TestClassNilProperty` |
| **Constructor Failure** | `New()` throws error | Capture error, don't crash VM | ✅ Safe (Try/Catch) | Low | `TestClassConstructorError` |

**Detected Gaps:**
- Deep inheritance chains not validated
- Circular class dependencies not detected
- Tests: Task 9–11 (Task 13 validates)

---

### Type: Function (User & Native)

| Case | Input | Expected Behavior | Current Status | Risk Level | Test |
|------|-------|-------------------|-----------------|------------|------|
| **Nil Reference** | `CallFunc(nil)` | Return error, not crash | ✅ Safe | Low | `TestFunctionNilCall` |
| **Too Many Arguments** | `Func(a, b)` called with 10 args | Ignore extras or reject | ✅ Safe (ignore) | Low | `TestFunctionTooManyArgs` |
| **Too Few Arguments** | `Func(a, b)` called with 0 args | Fill with Nil (v1.22 fix) | ✅ Fixed | Low | `TestFunctionTooFewArgs` |
| **Mutual Recursion** | `Func1() calls Func2() calls Func1()` | Reject after 1000 depth | ✅ Limited | Low | `TestFunctionMutualRecursion` |
| **Direct Recursion** | `Factorial(n) calls Factorial(n-1)` | Reject after 1000 depth | ✅ Limited | Low | `TestFunctionDirectRecursion` |
| **No Return Statement** | `Function Test() ... EndFunction` (no Return) | Return Nil silently | ✅ Safe | Low | `TestFunctionNoReturn` |

**Detected Gaps:**
- Parameter validation (type checking, range)
- Tail recursion optimization (not implemented, not expected)
- Tests: Task 9–11 (Task 13 validates)

---

## Operations × Edge Cases (40+ scenarios)

### Arithmetic Operations

| Operation | Input | Edge Case | Expected | Status | Task |
|-----------|-------|-----------|----------|--------|------|
| `+` | `n + 0` | Zero identity | `n` | ✅ | Baseline |
| `+` | `1.8e308 + 1.8e308` | Overflow | Error (not Inf) | ⚠️ | Task 10 |
| `-` | `0 - n` | Negation | `-n` | ✅ | Baseline |
| `*` | `n * 0` | Annihilator | `0` | ✅ | Baseline |
| `/` | `n / 0` | Division by zero | Error | ✅ | Baseline |
| `%` | `n % 0` | Modulo by zero | Error | ✅ | Baseline |
| `^` / `**` | `2 ** 1000` | Huge exponent | Error or cap | ⚠️ | Task 10 |
| `>`, `<` | `c1 > c2` (strings) | Lexicographic | Byte comparison | ✅ | v1.18 fix |
| `==` | `nil == nil` (Value interface) | Nil comparison | No crash | ✅ | v1.22 fix |
| `And`, `Or` | Truth table with Nil | Logical ops on error | Propagate error | ⚠️ | Task 10 |

**Test Count:** 10 scenarios  
**Gap:** Overflow detection, logical operators on error values

---

### Array Operations

| Operation | Input | Edge Case | Expected | Status | Task |
|-----------|-------|-----------|----------|--------|------|
| `aAdd(a, x)` | 1M+ times | Size limit | Error after 1M | ✅ | v2.0.3 |
| `aGet(a, i)` | `i < 0` | Negative index | Return nil (or error) | ⚠️ | Task 10 |
| `aGet(a, i)` | `i >= len(a)` | Out of bounds | Return nil | ✅ | Baseline |
| `aSet(a, i, x)` | `i < 0` | Negative index | Error or extend | ⚠️ | Task 10 |
| `aSet(a, i, x)` | `i >= len(a)` | Out of bounds | Extend array | ✅ | Baseline |
| `aScan(a, x)` | Empty array | Search in empty | Return 0 | ✅ | Baseline |
| `aSort(a, {&#124;x,y})` | Block throws error | Block error | Propagate error | ⚠️ | Task 10 |
| `Len(a)` | Huge array (2M) | Beyond limit | Return count or error | ⚠️ | Task 10 |
| `aClone(a)` | Circular ref `a[1] := a` | Deep copy cycle | Handle or error | ⚠️ | Task 11 |
| `aEval(a, {&#124;x})` | Block modifies array | During iteration | Undefined behavior | ⚠️ | Task 11 |

**Test Count:** 10 scenarios  
**Gaps:** Negative indices, block errors during operations, concurrent modification

---

### String Operations

| Operation | Input | Edge Case | Expected | Status | Task |
|-----------|-------|-----------|----------|--------|------|
| `SubStr(s, n1, n2)` | `n1 < 0` | Negative start | Return empty or error | ⚠️ | Task 10 |
| `SubStr(s, n1, n2)` | `n1 > len(s)` | Start beyond end | Return empty | ✅ | Baseline |
| `SubStr(s, n1, n2)` | `n2 > len(s)` | Length beyond end | Truncate to end | ✅ | Baseline |
| `At(s, substr)` | Empty substring `""` | Find empty | Return 1 or error | ⚠️ | Task 10 |
| `At(s, substr)` | Substring not found | Search miss | Return 0 | ✅ | Baseline |
| `Upper(s)`, `Lower(s)` | 10MB string | Huge string | Respect 10MB limit | ✅ | v2.0.3 |
| `Upper(s)`, `Lower(s)` | UTF-8 accents `"José"` | Unicode case | Handle correctly | ✅ | Baseline |
| `Len(s)` | Empty string | Length of empty | Return 0 | ✅ | Baseline |
| `Len(s)` | 10MB string | Huge length | Return byte count | ✅ | Baseline |
| `+` (concat) | `s1 + s2` exceeds 10MB | Concat overflow | Error | ✅ | v2.0.3 |

**Test Count:** 10 scenarios  
**Gaps:** Negative indices, empty substring search, boundary conditions

---

### Object/JSON Operations

| Operation | Input | Edge Case | Expected | Status | Task |
|-----------|-------|-----------|----------|--------|------|
| `oGet(o, key)` | Key doesn't exist | Missing key | Return nil | ✅ | Baseline |
| `oSet(o, key, value)` | 10k+ properties | Size limit | Error after 10k | ✅ | v2.0.3 |
| `oSet(o, key, value)` | Key is nil | Nil key | Error | ⚠️ | Task 10 |
| `oSet(o, key, value)` | Key is empty string | Empty key | Allow (or error) | ⚠️ | Task 10 |
| `oDel(o, key)` | Key doesn't exist | Delete missing | No-op or error | ⚠️ | Task 10 |
| `GetNames(o)` | 10k+ properties | Huge object | Return names in order | ✅ | Baseline |
| `GetNames(o)` | Nil object | Nil operand | Error | ✅ | v1.22 fix |
| `HasProperty(o, key)` | Case sensitivity | Key case | Case-sensitive lookup | ✅ | v1.18 fix |
| `toJson(o)` | Circular ref | Cycle in object | Infinite loop? | ⚠️ | Task 11 |
| `fromJson(s)` | Nesting > 100 levels | Deep JSON | Error | ✅ | v2.0.3 |

**Test Count:** 10 scenarios  
**Gaps:** Circular references, nil/empty keys, missing key semantics

---

### Recursion & Stack Operations

| Scenario | Depth | Expected | Status | Task |
|----------|-------|----------|--------|------|
| **Direct Recursion** | 1000+ | Error at 1001 | ✅ Limited | v2.0.3 |
| **Mutual Recursion** | A→B→A 1000+x | Error at 1001 | ✅ Limited | v2.0.3 |
| **Deep Block Nesting** | `{&#124; {&#124; {&#124;...}}}` 1000+ | Error at depth | ✅ Limited | v2.0.3 |
| **Tail Recursion** | `Fact(1M)` optimized | Should not exhaust stack | ⚠️ Not optimized | Task 9 |
| **Stack Variables** | 5k+ local variables | Error at limit | ✅ Limited | v2.0.3 |
| **Call Frames** | 5k+ nested calls | Error at limit | ✅ Limited | v2.0.3 |

**Test Count:** 6 scenarios  
**Gaps:** Tail call optimization, stack measurement, mutual recursion depth tracking

---

### Concurrency & Resource Operations

| Scenario | Load | Expected | Status | Task |
|----------|------|----------|--------|------|
| **StartJob** | 1000+ jobs | Error after 1000 | ✅ Limited (v2.0.3) | v2.0.3 |
| **Goroutine Leak** | Job doesn't complete | Cleanup on timeout | ⚠️ Not tested | Task 12 |
| **File Handles** | 100+ open files | Error after limit | ⚠️ Not limited | Task 12 |
| **DB Connections** | 100+ concurrent queries | Serialize or error | ✅ SQLite busy_timeout | v2.0.3 |
| **Memory per VM** | Array 100MB+ | Soft limit (monitor) | ⚠️ Not enforced | Task 11 |
| **Race Condition** | Shared state access | Thread-safe | ⚠️ Not audited | Task 11 |

**Test Count:** 6 scenarios  
**Gaps:** Goroutine leak detection, file handle limits, memory limits, thread safety

---

## Summary: Test Coverage by Category

| Category | Count | Status | Coverage % | Task |
|----------|-------|--------|------------|------|
| **Basic Data Types** | 48 | Partial | ~70% | Task 13 |
| **Arithmetic** | 10 | Partial | ~80% | Task 13 |
| **Arrays** | 10 | Partial | ~70% | Task 13 |
| **Strings** | 10 | Partial | ~70% | Task 13 |
| **Objects** | 10 | Partial | ~70% | Task 13 |
| **Recursion/Stack** | 6 | Good | ~85% | Task 13 |
| **Concurrency/Resources** | 6 | Partial | ~60% | Task 12–13 |
| **TOTAL** | **100+** | **~70%** | **~71%** | **Tasks 9–15** |

**Critical Gaps Identified:**
1. Circular reference detection (high risk, not tested)
2. Numeric overflow (medium risk, silent Inf)
3. Negative array indices (medium risk, undefined)
4. Concurrent modification (medium risk, undefined)
5. Goroutine leak detection (medium risk, resource leak)

---

## Recommended Test Organization

### Phase 1: Critical Path (Tasks 9–10)
- Nil safety (all types): 8 tests
- Numeric bounds: 4 tests
- Array/string bounds: 10 tests
- **Subtotal:** 22 tests (baseline stability)

### Phase 2: Comprehensive (Tasks 11–12)
- Circular references: 6 tests
- Concurrency/resources: 8 tests
- Recursion depth: 6 tests
- **Subtotal:** 20 tests (advanced safety)

### Phase 3: Corpus Validation (Tasks 13–15)
- All 100+ scenarios against OKF corpus
- 300-file corpus check for crashes
- Edge case matrix validation
- **Subtotal:** 300+ files + 100 unit tests

---

## Severity Priority for Test Implementation

| Priority | Tests | Target Task |
|----------|-------|-------------|
| **P0 (Must Have)** | Nil comparisons, bounds, overflow detection | Task 9–10 |
| **P1 (Should Have)** | Circular refs, async safety, goroutine limits | Task 11–12 |
| **P2 (Nice to Have)** | Edge case matrix full coverage, performance | Task 13–15 |

**Total Estimated Effort:** 240+ test scenarios = 60–80 hours of implementation + validation (Tasks 13–15).
