# Discrepancy Roadmap — Tasks 19–24

**Purpose:** Map each of the 23 discrepancies found in Task 18 audit to fix tasks in the Documentation Cycle (Tasks 19–24).

---

## Task 19: API Documentation Phase 1 (Symbols 1–120)

**Scope:** Add doc comments to first 120 exported Go types/functions across all packages

**Discrepancies Fixed:**
- **#18 (API table missing stub labels — partial):** First 120 symbols get `// Stub: ...` prefix in doc comments
- **#20 (Opcode count mismatch — partial):** Begin opcode audit; identify which of 88 are actually used

**Commits Expected:**
- `documentation(19): add API docs for 120 symbols (1–120 range)`

**Validation:**
- Count: 120 symbols documented
- Each stub has `// Stub: no-op` or `// Stub: returns default` in doc comment
- Cross-check against natives.go register list

---

## Task 20: README Sync & Known Limitations

**Scope:** Fix README.md against reality; add dedicated "Known Limitations" section

**Discrepancies Fixed:**
- **#1 (WSRESTFUL status buried):** Create subsection "REST API: Annotations vs. Classic DSL" with clear workaround
- **#2 (RecLock/MsUnlock undocumented):** Add "Concurrent Access Limitations" section noting no-op status
- **#5 (Console/TUI not mentioned):** Add "CLI Interactive Mode" subsection for standalone builds
- **#6 (VS Code version outdated):** Update to v2.0.3
- **#7 (LLM limitations not disclosed):** Add "LLM Limitations" subsection (I2_S only, no F16/F32, no streaming)
- **#8 (Tensor precision not noted):** Mention "Float32 (default) or Float64 (for precision) available"
- **#9 (Autograd incomplete):** Add caveat "Supports MatMul, Add, Mul, Relu, Sum, Mean, MSE; Softmax/CrossEntropy deferred"
- **#10 (MVC events status ambiguous):** Change "Tratamento de eventos" to "Event Handlers (parsed, not yet wired to UI)"
- **#11 (HTTP limits not stated):** Add "HTTP Client Limits: 30s timeout, max 5 redirects, certificate verification required"
- **#12 (Standalone complexity under-noted):** Add note on console-vs-GUI auto-detection and `ADVPP_FORCE_GUI` env var
- **#14 (Native function count imprecise):** Update to "~240+" (actual: 243)
- **#19 (Opcode table incomplete):** Audit compiler.go, update opcodes section to cover 0–88 with full mapping
- **#21 (Tensor precision not in intro):** Mention in Section 4.10 intro: "Float32 and Float64 precision selectable"
- **#22 (LLM SIMD detail unexplained):** Add user note "Native performance on amd64 with AVX2; scalar fallback elsewhere"
- **#23 (MVC events cross-link missing):** Cross-reference README ↔ GUIA.md on MVC event handler status

**Commits Expected:**
- `documentation(20a): add Known Limitations section to README`
- `documentation(20b): update feature descriptions (WSRESTFUL, RecLock, Console, LLM, HTTP, Tensor, MVC, Autograd)`
- `documentation(20c): sync opcode table and version info`

**Validation:**
- README now has explicit "Known Limitations" section (not buried in feature desc)
- Each limitation has workaround or reference
- No feature claim contradicts COMPONENT_STATUS.md

---

## Task 21: API Documentation Phase 2 (Symbols 121–240)

**Scope:** Add doc comments to symbols 121–240

**Discrepancies Fixed:**
- **#18 (API table missing stub labels — partial):** Middle 120 symbols get `// Stub: ...` prefix
- **#4 (API docs 38% incomplete — partial):** Progress toward 468/468 symbols

**Commits Expected:**
- `documentation(21): add API docs for 120 symbols (121–240 range)`

**Validation:**
- Count: 120 new symbols documented
- Cumulative: 240/468 (51%) documented

---

## Task 22: API Documentation Phase 3 & 4 (Symbols 241–468)

**Scope:** Complete all 468 API symbols documentation

**Discrepancies Fixed:**
- **#18 (API table missing stub labels — complete):** All 468 symbols documented with stub markers
- **#20 (Opcode count mismatch — complete):** Opcode reference table finalized (0–88 complete)
- **#4 (API docs 38% incomplete — complete):** All 468 symbols now have doc comments

**Commits Expected:**
- `documentation(22a): add API docs for 120 symbols (241–360 range)`
- `documentation(22b): add API docs for 108 symbols (361–468 range, complete)`
- `documentation(22c): finalize opcode reference table (0–88 with full mapping)`

**Validation:**
- Count: 468/468 API symbols documented
- All opcodes (0–88) mapped to implementation
- Stub markers applied consistently

---

## Task 23: Feature Matrix & README Sync

**Scope:** Create FEATURE_MATRIX.md; verify all 50+ features have status

**Discrepancies Fixed:**
- **#3 (Feature matrix missing):** Create `docs/FEATURE_MATRIX.md` with all ~55 major features
- **#4 (API docs incomplete — context):** Feature matrix cross-references API symbols

**Feature Matrix Columns:**
| Feature | Status | Implemented | Parsed | Unsupported | Example | Limitations | Fix Status |
|---------|--------|-------------|--------|-------------|---------|-------------|-----------|
| WSRESTFUL DSL | Parsed | ❌ | ✅ | N/A | ... | Use anotações instead | 🔄 Known |
| RecLock/MsUnlock | Limited | ⚠️ | ✅ | No concurrency | ... | No-ops in v2.0.3 | 🔄 Known |
| ... | ... | ... | ... | ... | ... | ... | ... |

**Commits Expected:**
- `documentation(23a): create FEATURE_MATRIX.md with 55 major features`
- `documentation(23b): cross-reference features to API symbols and COMPONENT_STATUS`

**Validation:**
- Matrix includes all ~55 features from README
- Status column (Implemented/Parsed/Unsupported) matches README + COMPONENT_STATUS + code reality
- Every major feature has an example or reference

---

## Task 24: Examples Validation & Test Suite

**Scope:** Validate all README/GUIA examples are runnable; create `tests/readme_examples_test.prw`

**Discrepancies Fixed:**
- **#13 (Examples not runnable):** Every example in README/GUIA must pass `advplc run`
- **#4 (API docs incomplete — validation):** Example per symbol confirms working

**Test File:** `tests/readme_examples_test.prw`

**Structure:**
```advpl
// --- Servidor MCP ---
User Function TestMCPServerExample()
    // Example from README, lines X–Y
    Local oMCP := MCPServer():New("meu-servidor", "1.0.0")
    oMCP:AddTool("soma", "Soma dois números", '...', "ToolSoma")
    // ... (mock Serve, validate registration)
    ConOut("✅ MCP server example works")
Return .T.

// --- Servidor REST ---
User Function TestRESTServerExample()
    Local oRest := WSRestServer():New("meu-servidor-rest", "1.0.0")
    oRest:AddRoute("GET", "/status", "GetStatus")
    ConOut("✅ REST server example works")
Return .T.

// ... (continue for LLM, Tensor, Autograd, etc.)
```

**Commits Expected:**
- `documentation(24a): create tests/readme_examples_test.prw with all README examples`
- `documentation(24b): validate examples: 52 examples, 52 pass`

**Validation:**
- Run: `advplc run tests/readme_examples_test.prw`
- Expected: "✅ All X examples work"
- No failing examples
- If example fails: update README or document limitation

---

## Dependency Chain

```
Task 19 (API docs 1–120)
  ↓ (enables)
Task 21 (API docs 121–240)
  ↓ (enables)
Task 22 (API docs 241–468 + complete opcodes)
  ↓
Task 20 (README sync) ← can run in parallel after Task 19 starts
  ↓
Task 23 (Feature matrix) ← depends on Tasks 20 + 22
  ↓
Task 24 (Examples) ← depends on Tasks 20 + 23
```

**Execution Strategy:** Start Task 19 immediately. Run Tasks 20 and 21 in parallel. Task 22 can overlap with Task 20. Tasks 23 and 24 wait for 20/22 completion.

---

## Success Criteria

### Per-Task Checklist

#### Task 19 ✅
- [ ] 120 symbols documented (1–120)
- [ ] Stub markers applied
- [ ] Opcodes 0–88 audit started
- [ ] Commit passes CI

#### Task 20 ✅
- [ ] README has "Known Limitations" section
- [ ] WSRESTFUL, RecLock, Console, LLM, HTTP, Tensor, Autograd, MVC limitations explicit
- [ ] Opcode table updated (or marked for Task 22)
- [ ] Versions bumped (VS Code v2.0.3)
- [ ] No feature claim contradicts code reality
- [ ] Commit passes CI

#### Task 21 ✅
- [ ] 120 symbols documented (121–240)
- [ ] Cumulative: 240/468 (51%)
- [ ] Stub markers consistent with Task 19
- [ ] Commit passes CI

#### Task 22 ✅
- [ ] 108 symbols documented (361–468)
- [ ] Cumulative: 468/468 (100%) ✅
- [ ] Opcode table complete (0–88 mapping)
- [ ] Stub markers 100% applied
- [ ] 2 commits pass CI

#### Task 23 ✅
- [ ] FEATURE_MATRIX.md created (~55 rows)
- [ ] All features in matrix have status (Implemented/Parsed/Unsupported)
- [ ] Matrix cross-references API symbols (from Tasks 19–22)
- [ ] Matrix cross-references COMPONENT_STATUS.md
- [ ] Example column filled for 80%+ of features
- [ ] 2 commits pass CI

#### Task 24 ✅
- [ ] tests/readme_examples_test.prw created
- [ ] 52+ examples from README/GUIA run successfully
- [ ] Output: "✅ All 52 examples work"
- [ ] If any example fails: document as "Known Limitation" in README
- [ ] 2 commits pass CI

---

## Final Deliverables (Task 18 → Task 25)

After Tasks 19–24 complete:

1. **README.md (updated)** — feature claims match code; Known Limitations disclosed
2. **COMPONENT_STATUS.md (verified)** — status markers accurate (post-Task-11 updates)
3. **GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md (verified)** — API tables complete; opcodes mapped
4. **docs/FEATURE_MATRIX.md (new)** — 55+ features × status
5. **docs/documentation/DOCUMENTATION_AUDIT_RESULTS.md (this file)** — findings
6. **docs/documentation/DISCREPANCY_ROADMAP.md (this file)** — fix tracking
7. **tests/readme_examples_test.prw (new)** — 52+ examples validated

**Metrics:**
- 468/468 API symbols documented ✅
- 23/23 discrepancies fixed ✅
- 100% README–code sync ✅
- 0 failing examples ✅

---

## Roadmap Summary Table

| Task | Scope | Discrepancies | Commits | Validation |
|------|-------|---------------|---------|-----------|
| 19 | API 1–120 | #18, #20 partial | 1 | 120 syms + opcodes audit |
| 20 | README sync | #1–14, #19, #21–23 | 3 | No claim contradicts code |
| 21 | API 121–240 | #18, #4 partial | 1 | 240/468 cumulative |
| 22 | API 241–468 + opcodes | #18, #20, #4 complete | 2 | 468/468 + 0–88 mapping |
| 23 | Feature matrix | #3 complete | 2 | 55 features × status |
| 24 | Examples | #13 complete | 2 | 52+ examples pass |

**Grand Total:** 23 discrepancies × 6 tasks = all fixed by end of Task 24 (Documentation Cycle completion)

---

**Roadmap Created:** 2026-07-29 by Claude Code  
**Roadmap Status:** Ready for Tasks 19–24 execution  
**Next Step:** Task 25 (Documentation Report) will consolidate all findings
