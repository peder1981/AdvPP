# README.md Audit Report — 2026-07-29

**Status:** ⚠️ CRITICAL ISSUES FOUND  
**Score:** 52/100 (README oversells by ~40%)  
**Recommendation:** Fix critical issues before next release (v2.0.5)

---

## Executive Summary

The README.md contains **extensive claims about features and documentation** that **do NOT match what's actually shipped**. Of 12 major referenced files/directories:

- ✅ **5 exist and are correct** (52%)
- ❌ **7 are missing or outdated** (58%)

### Critical Problems (7)

1. **VS Code Extension Version** (Line 853)
   - Claim: v2.0.3
   - Reality: v2.0.4 published
   - Impact: CRITICAL

2. **Missing File: TDN_FUNCTIONS.md** (Line 373)
   - Referenced but doesn't exist
   - Impact: CRITICAL — broken link

3. **Missing Directory: tests/llm/** (Lines 609–616)
   - 5 AdvPL examples promised
   - Entire "AI in AdvPL" section unsupported
   - Impact: CRITICAL

4. **Missing File: tests/llm/corpus.txt** (Line 636)
   - Referenced corpus missing
   - Impact: CRITICAL — examples won't work

5. **Missing File: tests/http_native_test.prw** (Line 373)
   - Referenced AdvPL test missing (only Go test exists)
   - Impact: CRITICAL

6. **Contradictory Claims: MVC Support** (Lines 225, 322, 792–800)
   - Line 322: "100% de Compatibilidade — funcionam perfeitamente"
   - Line 225: "eventos não conectados às ações"
   - Line 792–800: Confirms events not connected
   - Impact: CRITICAL — self-contradictory

7. **Outdated Autodiff Documentation** (Lines 768–778)
   - Says "apenas SGD", "Variable não implementa SoftmaxCE ainda"
   - But line 592 shows Adam, SoftmaxCE exist
   - Impact: CRITICAL — contradicts own content

### Medium Problems (5)

8. **Wrong Path Reference** (Lines 823, 831)
   - References `docs/GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md`
   - File is at `./GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md` (root)

9. **Missing File: tests/classifier_demo.prw** (Line 595)
   - Example referenced but missing

10. **Contradictory Autodiff Claims** (Lines 594–600 vs 768)
    - Modules claimed (Linear, Embedding) then denied

11. **Misleading Native Function Count** (Line 822)
    - "243 funções" but many are stubs/no-ops

12. **Questionable Implementations** (LLM/Tensor/Autograd)
    - No evidence of full implementation
    - Claimed pkg/ directories don't exist

---

## What's Actually Delivered ✅

- Lexer, parser, compiler (88 opcodes)
- Basic runtime (ConOut, Str, Val, etc.)
- Console I/O (ConIn)
- File I/O (MemoRead, MemoWrite, FOpen, etc.)
- System calls (WaitRun)
- SQLite database
- Classes, inheritance
- Code blocks, closures
- Multi-threading (StartJob)
- Database operations
- Array functions
- IDE interface (partial — missing event handling)

---

## What's NOT Delivered ❌

- tests/llm/ examples (entire section)
- Dom Casmurro corpus
- TDN_FUNCTIONS.md guide
- tests/http_native_test.prw example
- tests/classifier_demo.prw example
- Complete MVC event handling
- Full LLM/Tensor/Autograd (unclear if implemented)

---

## Action Items

### CRITICAL (Before v2.0.5)
- [ ] Update VS Code version to v2.0.4 (line 853)
- [ ] Remove or create tests/llm/* examples
- [ ] Remove or provide corpus.txt
- [ ] Remove or create TDN_FUNCTIONS.md
- [ ] Remove or create tests/http_native_test.prw
- [ ] Fix MVC claim (remove "100% funciona perfeitamente")
- [ ] Update autodiff docs to reflect Adam/SoftmaxCE

### MEDIUM (Before v2.1)
- [ ] Fix path to GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md
- [ ] Remove or create tests/classifier_demo.prw
- [ ] Add caveat about native function stubs
- [ ] Verify/document LLM/Tensor/Autograd actually exist

---

## Recommendation

**README is ASPIRATIONAL (wishlist), not DESCRIPTIVE (reality).**

Either:
1. **Implement & test all missing features**, OR
2. **Rewrite README to match what's shipped**

Current state: README oversells by ~40%.

---

Generated: 2026-07-29  
Auditor: Claude Code  
Scope: README.md lines 1–856
