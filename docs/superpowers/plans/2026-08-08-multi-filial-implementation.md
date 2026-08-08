# Multi-Filial (RpcSetEnv/FWxFilial) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the AdvPP VM session-level "filial ativa" state (`RpcSetEnv`/`FWxFilial`, Protheus-style multi-empresa/filial) and teach `FWMBrowse` to auto-filter/auto-stamp by a `FILIAL` column, exactly like it already does for `D_E_L_E_T_`.

**Architecture:** One new VM field (`filialAtiva string`) plus two new natives (`RPCSETENV` setter, `FWXFILIAL` getter with per-table truncation via a `X2_FILIAL_COMPART` config table read through the existing `SQLEngine` interface). `pkg/vm/browse.go` gains a `hasFilial` flag parallel to the existing `hasDelete` one, threaded through `browseColumns` → `browseItems`/`browseSave`.

**Tech Stack:** Go 1.24, existing `pkg/vm` package, `database/sql` via `pkg/db.SQLiteEngine` (already implements the `SQLEngine` interface used here).

## Global Constraints

- Parameterized queries only (`?` placeholders) for any value that isn't a fixed SQL keyword — mirrors the existing CWE-89 comments already in `browse.go`. Never string-concatenate a filial value into SQL text.
- Native names are registered UPPERCASE in the `natives` map in `pkg/vm/natives.go:registerNatives` (existing convention — every entry is `"FOO":` regardless of the mixed-case spelling AdvPL source uses to call it).
- Default to the **most restrictive** behavior on any ambiguity (missing config, unknown table) — nível 6 (totalmente exclusiva), never nível 0 (compartilhada).
- This plan does not touch `TCLink`/`TCUnlink` — out of scope (see spec, "Fora de Escopo").
- Follow `gofmt` — run `gofmt -l pkg/vm/` before every commit in this plan; it must print nothing.

---

### Task 1: Estado de sessão — campo `filialAtiva` + `RPCSETENV`

**Files:**
- Modify: `pkg/vm/vm.go:107` (VM struct field block, right after `httpHeaders`/timeout fields)
- Modify: `pkg/vm/natives.go` (new native, placed after the `TCSQLQUERY` entry, ~line 1293)
- Test: `pkg/vm/filial_test.go` (new)

**Interfaces:**
- Produces: `VM.filialAtiva string` (unexported field, read by Task 2's `resolveFilial`); native `RPCSETENV(cFilial)` callable from AdvPL as `RpcSetEnv(cFilial)`.

- [ ] **Step 1: Add the VM field**

In `pkg/vm/vm.go`, inside the `VM` struct (find the block ending in `jobIDSeq int64 // ...`), add:

```go
	filialAtiva    string                    // filial ativa da sessão (RpcSetEnv/FWxFilial), 6 chars GG+UU+FF; "" = nenhuma definida ainda
```

- [ ] **Step 2: Write the failing test**

Create `pkg/vm/filial_test.go`:

```go
package vm

import (
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestRpcSetEnvGravaFilialAtiva(t *testing.T) {
	v := NewVM(nil, false)
	_, err := v.natives["RPCSETENV"].Fn([]advplrt.Value{advplrt.NewString("010101")})
	if err != nil {
		t.Fatalf("RpcSetEnv retornou erro: %v", err)
	}
	if v.filialAtiva != "010101" {
		t.Errorf("filialAtiva = %q, quer %q", v.filialAtiva, "010101")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./pkg/vm/ -run TestRpcSetEnvGravaFilialAtiva -v`
Expected: FAIL — `v.natives["RPCSETENV"]` is nil (native doesn't exist yet), so the `.Fn` call panics on a nil pointer. If `NewVM(nil, false)` itself panics on a nil bytecode argument, check `pkg/vm/vm.go`'s `NewVM` for a nil-bytecode guard; if none exists, pass `&compiler.Bytecode{}` instead (empty, valid) — import `"github.com/advpl/compiler/pkg/compiler"` in that case.

- [ ] **Step 4: Add the `RPCSETENV` native**

In `pkg/vm/natives.go`, right after the `"TCSQLQUERY": func(...) {...},` block (the one ending `return advplrt.NewArray(elems), nil\n\t\t},` around line 1293), insert:

```go
		// --- Multi-filial (RpcSetEnv/FWxFilial) ---
		// RpcSetEnv grava a filial ativa da sessão (convenção Protheus,
		// 6 chars GG+UU+FF). Chamada uma vez no login/troca de contexto de
		// quem usa o recurso — programas que nunca chamam isto continuam
		// com filialAtiva == "", e FWMBrowse/FWxFilial se comportam
		// exatamente como hoje (ver Task 3).
		"RPCSETENV": func(args []advplrt.Value) (advplrt.Value, error) {
			v.filialAtiva = advplrt.ToString(getArg(args, 0))
			return advplrt.True, nil
		},
```

- [ ] **Step 5: Confirm it passes**

Run: `go test ./pkg/vm/ -run TestRpcSetEnvGravaFilialAtiva -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
cd /home/peder/Projetos/AdvPP
gofmt -l pkg/vm/
git add pkg/vm/vm.go pkg/vm/natives.go pkg/vm/filial_test.go
git commit -m "feat(vm): estado de sessao para filial ativa (RpcSetEnv)"
```

---

### Task 2: `FWxFilial` com nível de compartilhamento

**Files:**
- Create: `pkg/vm/filial.go`
- Modify: `pkg/vm/natives.go` (add `FWXFILIAL` native, right after `RPCSETENV`)
- Test: `pkg/vm/filial_test.go` (extend)

**Interfaces:**
- Consumes: `v.filialAtiva` (Task 1), `SQLEngine.QueryRows` (existing interface, `pkg/vm/browse.go:24`)
- Produces: `func (v *VM) resolveFilial(eng SQLEngine, alias string) string` — used directly by `browse.go` in Task 3, and by the `FWXFILIAL` native itself. `func truncarFilial(filial string, nivel int) string` — pure helper, unit-tested standalone.

- [ ] **Step 1: Write the failing test for the pure truncation function**

Append to `pkg/vm/filial_test.go`:

```go
func TestTruncarFilial(t *testing.T) {
	cases := []struct {
		filial string
		nivel  int
		want   string
	}{
		{"010101", 6, "010101"},
		{"010101", 4, "0101  "},
		{"010101", 2, "01    "},
		{"010101", 0, "      "},
		{"", 6, "      "},        // sessão sem filial definida ainda
		{"01", 6, "01    "},      // filial curta (nao deveria acontecer, mas nao pode panicar)
	}
	for _, c := range cases {
		got := truncarFilial(c.filial, c.nivel)
		if got != c.want {
			t.Errorf("truncarFilial(%q, %d) = %q, quer %q", c.filial, c.nivel, got, c.want)
		}
	}
}

func TestResolveFilialSemConfigUsaNivel6(t *testing.T) {
	v := NewVM(nil, false)
	v.filialAtiva = "010101"
	// nil SQLEngine (nenhum banco conectado) -- deve cair no default 6,
	// nao panicar.
	got := v.resolveFilial(nil, "UNI")
	if got != "010101" {
		t.Errorf("resolveFilial sem engine = %q, quer %q (default nivel 6)", got, "010101")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/vm/ -run 'TestTruncarFilial|TestResolveFilialSemConfigUsaNivel6' -v`
Expected: FAIL — `truncarFilial`/`resolveFilial` undefined.

- [ ] **Step 3: Implement `pkg/vm/filial.go`**

```go
package vm

import (
	"strconv"
	"strings"
)

// resolveFilial computa a filial "efetiva" pra uma tabela: a filial ativa
// da sessão (RpcSetEnv), truncada/espacada conforme o NIVEL configurado em
// X2_FILIAL_COMPART pra essa tabela. Sem engine conectado, sem a tabela de
// config, ou sem linha pro alias -- default NIVEL=6 (mais restritivo,
// falha segura: nunca vaza dado achando que uma tabela e compartilhada
// quando ninguem configurou nada).
func (v *VM) resolveFilial(eng SQLEngine, alias string) string {
	nivel := 6
	if eng != nil {
		rows, err := eng.QueryRows("SELECT NIVEL FROM X2_FILIAL_COMPART WHERE TABELA = ?", strings.ToUpper(alias))
		if err == nil && len(rows) > 0 {
			if n, convErr := strconv.Atoi(strings.TrimSpace(rows[0]["NIVEL"])); convErr == nil {
				nivel = n
			}
		}
	}
	return truncarFilial(v.filialAtiva, nivel)
}

// truncarFilial mantem os primeiros nivel caracteres da filial e completa
// o resto com espaco ate 6 -- mesma regra usada pro valor GRAVADO em cada
// linha (ver browse.go, Task 3), por isso uma comparacao de igualdade
// simples (WHERE FILIAL = ?) funciona pra qualquer nivel sem CASE/lógica
// condicional na query.
func truncarFilial(filial string, nivel int) string {
	if nivel < 0 {
		nivel = 0
	}
	if nivel > 6 {
		nivel = 6
	}
	base := filial
	for len(base) < 6 {
		base += " "
	}
	base = base[:6]
	return base[:nivel] + strings.Repeat(" ", 6-nivel)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./pkg/vm/ -run 'TestTruncarFilial|TestResolveFilialSemConfigUsaNivel6' -v`
Expected: PASS

- [ ] **Step 5: Add the `FWXFILIAL` native**

In `pkg/vm/natives.go`, right after `"RPCSETENV"` (Task 1, Step 4), add:

```go
		// FWxFilial(cAlias): devolve a filial ativa truncada conforme o
		// nivel de compartilhamento de cAlias (ver resolveFilial,
		// pkg/vm/filial.go). O parametro existe por fidelidade a
		// assinatura real do Protheus -- hoje o nivel e' lido de
		// X2_FILIAL_COMPART, uma tabela por instalacao, nao por alias
		// dentro do mesmo banco.
		"FWXFILIAL": func(args []advplrt.Value) (advplrt.Value, error) {
			alias := advplrt.ToString(getArg(args, 0))
			sqlEng, _ := v.dbEngine.(SQLEngine)
			return advplrt.NewString(v.resolveFilial(sqlEng, alias)), nil
		},
```

- [ ] **Step 6: Write an integration-style test using a real SQLite engine**

Append to `pkg/vm/filial_test.go`:

```go
func TestFWxFilialComNivelConfigurado(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/filial_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec("CREATE TABLE X2_FILIAL_COMPART (TABELA TEXT PRIMARY KEY, NIVEL INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create X2_FILIAL_COMPART: %v", err)
	}
	if err := eng.Exec("INSERT INTO X2_FILIAL_COMPART (TABELA, NIVEL) VALUES ('PLANO_CONTAS', 4)"); err != nil {
		t.Fatalf("insert config: %v", err)
	}

	v := NewVM(nil, false)
	v.SetDBEngine(eng)
	v.filialAtiva = "010101"

	got, err := v.natives["FWXFILIAL"].Fn([]advplrt.Value{advplrt.NewString("PLANO_CONTAS")})
	if err != nil {
		t.Fatalf("FWxFilial erro: %v", err)
	}
	if got.(*advplrt.StringValue).Val != "0101  " {
		t.Errorf("FWxFilial('PLANO_CONTAS') = %q, quer %q (nivel 4)", got.(*advplrt.StringValue).Val, "0101  ")
	}

	// Tabela sem config: cai no default 6.
	got2, _ := v.natives["FWXFILIAL"].Fn([]advplrt.Value{advplrt.NewString("UNI")})
	if got2.(*advplrt.StringValue).Val != "010101" {
		t.Errorf("FWxFilial('UNI') sem config = %q, quer %q (default nivel 6)", got2.(*advplrt.StringValue).Val, "010101")
	}
}
```

Add `"github.com/advpl/compiler/pkg/db"` to the imports at the top of `filial_test.go`.

If `advplrt.StringValue` isn't the exact exported type/field name for a string `Value`, check `pkg/runtime/values.go` for the real type (search `NewString` — its return type's underlying struct) and adjust the assertion accordingly; the important assertion is the returned string content, not the exact Go type spelling.

- [ ] **Step 7: Run full filial test file**

Run: `go test ./pkg/vm/ -run TestFWxFilial -v` then `go test ./pkg/vm/ -v -run Filial`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
cd /home/peder/Projetos/AdvPP
gofmt -l pkg/vm/
git add pkg/vm/filial.go pkg/vm/natives.go pkg/vm/filial_test.go
git commit -m "feat(vm): FWxFilial com nivel de compartilhamento (X2_FILIAL_COMPART)"
```

---

### Task 3: `browse.go` — auto-filtro completo por `FILIAL`

**Files:**
- Modify: `pkg/vm/browse.go` (`browseColumns` ~line 154, `browseItems` ~line 211, `browseSave` ~line 258, `runBrowse` ~line 98)
- Test: `pkg/vm/browse_test.go` (new)

**Interfaces:**
- Consumes: `v.resolveFilial(eng, alias)` from Task 2.
- Produces: `browseColumns` now returns `(cols []browseColumn, hasDelete, hasFilial bool, err error)` — 4 values, not 3. `browseItems`/`browseSave` gain `hasFilial bool, cFilial string` parameters. Any other caller of these three functions elsewhere in the codebase must be updated to match (grep confirms `runBrowse` in this same file is the only caller — see Step 1).

- [ ] **Step 1: Confirm there is only one call site to update**

Run: `grep -rn "browseColumns(\|browseItems(\|browseSave(\|browseDelete(" pkg/vm/*.go`
Expected: All four names appear only in `browse.go` itself (the three function definitions plus their one call each from `runBrowse`). If any other file calls them, note it — the signature changes below must be applied there too.

- [ ] **Step 2: Write the failing test**

Create `pkg/vm/browse_test.go`:

```go
package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/db"
)

func TestBrowseFiltraPorFilial(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('999', '010102')"); err != nil {
		t.Fatalf("seed B: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "UNI")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if !hasFilial {
		t.Fatal("hasFilial deveria ser true (tabela tem coluna FILIAL)")
	}

	items, err := browseItems(eng, "UNI", cols, hasDelete, hasFilial, "010101")
	if err != nil {
		t.Fatalf("browseItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("browseItems devolveu %d itens, quer 1 (só a filial 010101)", len(items))
	}
	if items[0]["UNI_CODIGO"] != "101" {
		t.Errorf("UNI_CODIGO = %v, quer '101'", items[0]["UNI_CODIGO"])
	}

	// Incluir sob filial 010102 -- deve estampar FILIAL sozinho.
	err = browseSave(eng, "UNI", cols, hasDelete, hasFilial, "010102",
		browseAction{Action: "save", Recno: 0, Data: map[string]string{"UNI_CODIGO": "202"}})
	if err != nil {
		t.Fatalf("browseSave (incluir): %v", err)
	}
	rows, _ := eng.QueryRows("SELECT FILIAL FROM UNI WHERE UNI_CODIGO = '202'")
	if len(rows) != 1 || rows[0]["FILIAL"] != "010102" {
		t.Errorf("linha incluida com FILIAL = %v, quer '010102'", rows)
	}

	// Sob a filial 010102, browseItems agora deve ver 2 linhas (999 + 202 novo).
	items2, _ := browseItems(eng, "UNI", cols, hasDelete, hasFilial, "010102")
	if len(items2) != 2 {
		t.Errorf("browseItems sob 010102 devolveu %d itens, quer 2", len(items2))
	}
}

func TestBrowseSemColunaFilialComportamentoInalterado(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test2.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE SEM_FILIAL (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		NOME TEXT
	)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := eng.Exec("INSERT INTO SEM_FILIAL (NOME) VALUES ('x')"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "SEM_FILIAL")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if hasFilial {
		t.Fatal("hasFilial deveria ser false (tabela sem coluna FILIAL)")
	}
	items, err := browseItems(eng, "SEM_FILIAL", cols, hasDelete, hasFilial, "")
	if err != nil {
		t.Fatalf("browseItems: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("tabela sem FILIAL: browseItems devolveu %d itens, quer 1 (sem filtro)", len(items))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go build ./... && go test ./pkg/vm/ -run TestBrowse -v`
Expected: FAIL to compile — `browseColumns` still returns 3 values (`hasFilial` doesn't exist yet), `browseItems`/`browseSave` don't accept `hasFilial`/`cFilial` yet. A compile error is the expected failure here.

- [ ] **Step 4: Change `browseColumns` to detect `FILIAL`**

In `pkg/vm/browse.go`, inside `browseColumns` (~line 154), the loop that sets `hasDelete`:

```go
	physSet := map[string]bool{}
	hasDelete := false
	for _, p := range phys {
		name := strings.ToUpper(p["NAME"])
		physSet[name] = true
		if name == "D_E_L_E_T_" {
			hasDelete = true
		}
	}
```

becomes:

```go
	physSet := map[string]bool{}
	hasDelete := false
	hasFilial := false
	for _, p := range phys {
		name := strings.ToUpper(p["NAME"])
		physSet[name] = true
		if name == "D_E_L_E_T_" {
			hasDelete = true
		}
		if name == "FILIAL" {
			hasFilial = true
		}
	}
```

And the function's final `return cols, hasDelete, nil` becomes `return cols, hasDelete, hasFilial, nil`. Update the function signature line itself:

```go
func (v *VM) browseColumns(eng SQLEngine, alias string) ([]browseColumn, bool, bool, error) {
```

(both `bool` returns: `hasDelete`, then `hasFilial`, in that order — matches the order they're declared above.)

- [ ] **Step 5: Change `browseItems` to filter by `FILIAL`**

Replace the whole function:

```go
func browseItems(eng SQLEngine, alias string, cols []browseColumn, hasDelete, hasFilial bool, cFilial string) ([]map[string]any, error) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Property
	}
	query := fmt.Sprintf("SELECT rowid AS browse_recno_, %s FROM %s", strings.Join(names, ", "), alias)
	var conds []string
	var qargs []any
	if hasDelete {
		conds = append(conds, "D_E_L_E_T_ <> '*'")
	}
	if hasFilial {
		conds = append(conds, "FILIAL = ?")
		qargs = append(qargs, cFilial)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := eng.QueryRows(query, qargs...)
	if err != nil {
		return nil, fmt.Errorf("FWMBrowse: %w", err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		item := map[string]any{}
		recno, _ := strconv.ParseInt(r["BROWSE_RECNO_"], 10, 64)
		item["recno"] = recno
		for _, c := range cols {
			val := r[c.Property]
			if c.Type == "N" {
				n, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
				item[c.Property] = n
			} else {
				item[c.Property] = val
			}
		}
		items = append(items, item)
	}
	return items, nil
}
```

(Kept the existing `"rowid" is aliased explicitly...` comment above the `SELECT` line — don't drop it, it documents a real prior bug.)

- [ ] **Step 6: Change `browseSave` to stamp `FILIAL` on Incluir**

In `browseSave`, the `if act.Recno == 0 { ... }` block:

```go
	if act.Recno == 0 {
		if hasDelete {
			names = append(names, "D_E_L_E_T_")
			vals = append(vals, " ")
		}
		q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", alias,
			strings.Join(names, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", "))
		return eng.Exec(q, vals...)
	}
```

becomes:

```go
	if act.Recno == 0 {
		if hasDelete {
			names = append(names, "D_E_L_E_T_")
			vals = append(vals, " ")
		}
		if hasFilial {
			names = append(names, "FILIAL")
			vals = append(vals, cFilial)
		}
		q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", alias,
			strings.Join(names, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", "))
		return eng.Exec(q, vals...)
	}
```

And the function signature:

```go
func browseSave(eng SQLEngine, alias string, cols []browseColumn, hasDelete, hasFilial bool, cFilial string, act browseAction) error {
```

The `UPDATE ... WHERE rowid = ?` branch (Alterar) stays untouched — per spec, no additional constraint there; the row was only reachable if it already passed the `FILIAL` filter in `browseItems`.

- [ ] **Step 7: Wire it all together in `runBrowse`**

```go
	cols, hasDelete, hasFilial, err := v.browseColumns(sqlEng, b.alias)
	if err != nil {
		return err
	}

	cFilial := ""
	if hasFilial {
		cFilial = v.resolveFilial(sqlEng, b.alias)
	}

	for {
		items, err := browseItems(sqlEng, b.alias, cols, hasDelete, hasFilial, cFilial)
		...
			case "save":
				if err := browseSave(sqlEng, b.alias, cols, hasDelete, hasFilial, cFilial, act); err != nil {
```

(Only the four call-site lines change — `cols, hasDelete, err := v.browseColumns(...)`, the `items, err := browseItems(...)` call, and the `browseSave(...)` call, plus the two new lines computing `cFilial` right after `browseColumns`. Everything else in `runBrowse` — the JSON marshaling, the `switch act.Action`, `browseDelete` call — stays exactly as today.)

- [ ] **Step 8: Run the test to verify it now passes**

Run: `go build ./... && go test ./pkg/vm/ -run TestBrowse -v`
Expected: PASS, both `TestBrowseFiltraPorFilial` and `TestBrowseSemColunaFilialComportamentoInalterado`.

- [ ] **Step 9: Run the full package test suite to catch anything else broken**

Run: `go build ./... && go test ./... 2>&1 | tail -40`
Expected: no compile errors anywhere in the repo (confirms Step 1's "only one call site" was correct), all pre-existing tests still pass.

- [ ] **Step 10: Commit**

```bash
cd /home/peder/Projetos/AdvPP
gofmt -l pkg/vm/
git add pkg/vm/browse.go pkg/vm/browse_test.go
git commit -m "feat(vm): FWMBrowse filtra e estampa FILIAL automaticamente"
```

---

### Task 4: Teste de integração ponta-a-ponta + release

**Files:**
- Create: `tests/multi_filial_test.prw` (fixture — check `tests/` dir at repo root for the existing naming convention before creating)
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: Tasks 1-3 (all natives + browse.go behavior) via real compiled `advplc`.

- [ ] **Step 1: Check the existing `tests/` fixture convention**

Run: `ls tests/*.prw | head -5` and read one short existing fixture (e.g. the smallest file) to match its header/structure — this repo already has AdvPL test fixtures run through `advplc run`/`advplc check` in CI (see `.github/workflows/`), not Go-level `_test.go` for end-to-end AdvPL behavior.

- [ ] **Step 2: Write the fixture**

Create `tests/multi_filial_test.prw`:

```advpl
#include "totvs.ch"

User Function MultiFilialTest()
    TCSqlExec("CREATE TABLE IF NOT EXISTS UNI (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, D_E_L_E_T_ TEXT DEFAULT ' ', UNI_CODIGO TEXT, FILIAL TEXT)")
    TCSqlExec("DELETE FROM UNI")

    RpcSetEnv("010101")
    ConOut("filial 010101: " + FWxFilial("UNI"))

    RpcSetEnv("010102")
    ConOut("filial 010102: " + FWxFilial("UNI"))

    // Sem X2_FILIAL_COMPART pra UNI: default nivel 6, devolve a propria
    // filial ativa inteira.
    If FWxFilial("UNI") == "010102"
        ConOut("PASS: FWxFilial sem config usa nivel 6")
    Else
        ConOut("FALHA: FWxFilial sem config = " + FWxFilial("UNI"))
    EndIf
Return
```

- [ ] **Step 3: Run it**

Run: `go run ./cmd/advplc run tests/multi_filial_test.prw`
Expected output includes `filial 010101: 010101`, `filial 010102: 010102`, `PASS: FWxFilial sem config usa nivel 6`.

- [ ] **Step 4: Add to CHANGELOG.md**

At the top of `CHANGELOG.md`, above the current top entry, add:

```markdown
## [Unreleased]

### Adicionado

- **`RpcSetEnv(cFilial)`/`FWxFilial(cAlias)`** — estado de sessão multi-filial
  (convenção Protheus: 6 caracteres, GG+UU+FF). `FWxFilial` devolve a filial
  ativa truncada conforme o nível de compartilhamento configurado por tabela
  em `X2_FILIAL_COMPART` (default: nível 6, exclusiva — falha segura sem
  configuração).
- **`FWMBrowse` filtra e estampa `FILIAL` automaticamente** quando a tabela
  física tem essa coluna — mesmo mecanismo que já existe para `D_E_L_E_T_`.
  Motivado pelo multi-condomínio do GesCon.
```

- [ ] **Step 5: Commit**

```bash
cd /home/peder/Projetos/AdvPP
git add tests/multi_filial_test.prw CHANGELOG.md
git commit -m "test: fixture de integracao para RpcSetEnv/FWxFilial + CHANGELOG"
```

- [ ] **Step 6: Cut the release**

Not part of this plan's automated steps — follow the repo's existing release process (`make release VERSION=x.y.z`, per `CHANGELOG.md`'s own "Build, empacotamento e release" section) once every task above is merged. The **GesCon plan depends on this version existing** — record the version number chosen here so it can be pinned in GesCon's `ADVPP_VERSION` file.
