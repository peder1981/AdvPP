# AdvPP Language Robustness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fechar quatro limitações de semântica da linguagem AdvPP — `If`/`IIF` sem curto-circuito, falta de iteração de chaves de hash, closures só de nível único, e `Private` sem escopo dinâmico.

**Architecture:** Mudanças no compilador (`pkg/compiler`) e na VM (`pkg/vm`), com apoio no runtime (`pkg/runtime`). Cada feature é isolada: uma forma especial no compilador (If/IIF), um native (GetNames), um modelo de upvalue tipado estilo Lua (closures aninhadas) e um ambiente dinâmico com pilha de restauração (Private). Verificação por fixtures `.prw` executados com `./advplc run`.

**Tech Stack:** Go (puro, `CGO_ENABLED=0`), bytecode VM própria, fixtures em AdvPL (`.prw`).

## Global Constraints

- Go puro, sem CGO. Rebuild após qualquer mudança Go: `go build -o advplc ./cmd/advplc`.
- Novos opcodes: adicionar ao **final** do enum `Opcode` em `pkg/compiler/opcodes.go` (preserva o `iota`) e registrar o nome no mapa `opcodeNames`.
- Fixtures são `.prw` na pasta `tests/`; comportamento verificado com `./advplc run tests/<nome>.prw` e inspeção da saída `ConOut`.
- Fontes `.prw` deste projeto são UTF-8 (o AdvPP aceita UTF-8, ao contrário do Protheus real que exige CP1252).
- Regressão obrigatória verde ao fim de cada task: `go test ./...` e os fixtures existentes + exemplos `pt_llm.prw`/`pt_chat.prw`/`pt_nn.prw` continuam compilando (`./advplc check`) e rodando.
- Ordem de resolução de nomes após a Task 4: Local/param (slot estático) → Upvalue (closure) → Dinâmico (Private/não-declarado dentro de função) → global (só escopo de arquivo).
- Não commitar em branch fora do fluxo do repositório: o projeto trabalha direto em `master` (convenção existente).

---

## File Structure

- `pkg/runtime/values.go` — `ObjectValue` ganha `Keys []string` + método `SetProp` (ordem de inserção das chaves).
- `pkg/vm/natives.go` — native `GetNames`.
- `pkg/vm/vm.go` — usa `SetProp`; captura de upvalue tipada; opcodes dinâmicos; `dynEnv` + restauração no return.
- `pkg/compiler/opcodes.go` — `UpvalDesc` + constantes; `FunctionInfo.Upvals`; opcodes `OP_LOAD_DYN`/`OP_STORE_DYN`/`OP_DECL_DYN`.
- `pkg/compiler/codegen.go` — forma especial If/IIF; upvalue recursivo/tipado; resolução dinâmica; compilação de Private/Public.
- `tests/getnames_test.prw`, `tests/ifshort_test.prw`, `tests/nestclosure_test.prw`, `tests/dynprivate_test.prw` — fixtures.

---

## Task 0: Commit da base (closures de nível único)

A árvore de trabalho já contém a implementação de **closures de nível único**
(feita antes deste plano, testada e documentada, mas ainda sem commit). A Task 3
constrói sobre ela (renomeia `FunctionInfo.UpvalSlots` → `Upvals`). Commitar essa
base primeiro deixa o histórico limpo e o plano executável do estado atual.

**Files:**
- Commit (já modificados na árvore): `pkg/runtime/values.go`, `pkg/compiler/opcodes.go`, `pkg/compiler/codegen.go`, `pkg/vm/vm.go`, `README.md`, `CHANGELOG.md`, `tests/closures_test.prw`.

- [ ] **Step 1: Verificar que a base compila e o fixture passa**

Run: `go build -o advplc ./cmd/advplc && ./advplc run tests/closures_test.prw`
Expected:
```
AEval acumula em Local externo (esperado 60): 60
Eval le capturas (esperado 47): 47
counter que escapou (esperado 3): 3
```

- [ ] **Step 2: Commit da base**

```bash
git add pkg/runtime/values.go pkg/compiler/opcodes.go pkg/compiler/codegen.go pkg/vm/vm.go README.md CHANGELOG.md tests/closures_test.prw
git commit -m "Add single-level closures (upvalues by reference)"
```

Nota: `docs/superpowers/plans/` (este plano) fica de fora deste commit; commite-o
separadamente ou junto da Task 5, conforme preferir.

---

## Task 1: Iteração de chaves de hash (`GetNames`)

**Files:**
- Modify: `pkg/runtime/values.go` (struct `ObjectValue` ~187; adicionar método `SetProp`)
- Modify: `pkg/vm/vm.go` (casos `OP_ARRAY_SET`, `OP_SET_PROP`, `OP_NEW_OBJECT`)
- Modify: `pkg/vm/natives.go` (novo native no mapa `natives`)
- Test: `tests/getnames_test.prw`

**Interfaces:**
- Produces: `(*advplrt.ObjectValue).SetProp(key string, val Value)` — grava preservando ordem de inserção em `Keys []string`; `GetNames(oJson)` native → array de strings com as chaves na ordem de inserção.

- [ ] **Step 1: Write the failing test**

Criar `tests/getnames_test.prw`:

```advpl
User Function GetNamesTst()
    Local j := JsonObject():New()
    Local aK := {}
    Local i := 0
    Local cCat := ""
    j["um"]   := 1
    j["dois"] := 2
    j["tres"] := 3
    aK := GetNames(j)
    ConOut("count (esperado 3): " + Str(Len(aK)))
    For i := 1 To Len(aK)
        cCat += AllTrim(aK[i]) + ","
    Next i
    ConOut("ordem (esperado um,dois,tres,): " + cCat)
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `./advplc run tests/getnames_test.prw`
Expected: FAIL — erro `unknown function: GETNAMES`.

- [ ] **Step 3: Adicionar `Keys` e `SetProp` ao `ObjectValue`**

Em `pkg/runtime/values.go`, alterar a struct (adicionar campo `Keys`) e adicionar o método logo após ela:

```go
type ObjectValue struct {
	ClassName string
	Props     map[string]Value
	Keys      []string // ordem de inserção das chaves (para GetNames)
	Class     *ClassDef
	Native    interface{} // estado Go de classes de framework nativas (ex.: FWGridProcess)
}

// SetProp grava uma propriedade preservando a ordem de inserção das chaves.
func (o *ObjectValue) SetProp(key string, val Value) {
	if _, exists := o.Props[key]; !exists {
		o.Keys = append(o.Keys, key)
	}
	o.Props[key] = val
}
```

- [ ] **Step 4: Usar `SetProp` nos três pontos que criam chave de hash**

Em `pkg/vm/vm.go`, no caso `OP_ARRAY_SET` (ramo do `*advplrt.ObjectValue`), trocar:

```go
			// Chave de bracket em JsonObject/hash: case-sensitive (semantica JSON).
			if s, ok := idx.(*advplrt.StringValue); ok {
				o.Props[s.Val] = val
			}
```

por:

```go
			// Chave de bracket em JsonObject/hash: case-sensitive (semantica JSON).
			if s, ok := idx.(*advplrt.StringValue); ok {
				o.SetProp(s.Val, val)
			}
```

No caso `OP_NEW_OBJECT`, trocar `obj.Props[strings.ToUpper(s.Val)] = val` por `obj.SetProp(strings.ToUpper(s.Val), val)`.

No caso `OP_SET_PROP` (ramo `*advplrt.ObjectValue`), trocar `o.Props[strings.ToUpper(propName)] = val` por `o.SetProp(strings.ToUpper(propName), val)`.

- [ ] **Step 5: Adicionar o native `GetNames`**

Em `pkg/vm/natives.go`, dentro do mapa `natives` (ex.: logo após a entrada `"WSADVVALUE"`), adicionar:

```go
		// GetNames(oJson): array com as chaves do hash, na ordem de inserção.
		"GETNAMES": func(args []advplrt.Value) (advplrt.Value, error) {
			if o, ok := getArg(args, 0).(*advplrt.ObjectValue); ok {
				elems := make([]advplrt.Value, len(o.Keys))
				for i, k := range o.Keys {
					elems[i] = advplrt.NewString(k)
				}
				return advplrt.NewArray(elems), nil
			}
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
```

- [ ] **Step 6: Rebuild**

Run: `go build -o advplc ./cmd/advplc`
Expected: sem erros.

- [ ] **Step 7: Run test to verify it passes**

Run: `./advplc run tests/getnames_test.prw`
Expected:
```
count (esperado 3): 3
ordem (esperado um,dois,tres,): um,dois,tres,
```

- [ ] **Step 8: Regressão**

Run: `go test ./pkg/... 2>&1 | grep -v '^ok\|no test files'`
Expected: sem saída (nenhuma falha).
Run: `./advplc check tests/*.prw pt_llm.prw pt_chat.prw pt_nn.prw`
Expected: `... 0 failed`.

- [ ] **Step 9: Commit**

```bash
git add pkg/runtime/values.go pkg/vm/vm.go pkg/vm/natives.go tests/getnames_test.prw
git commit -m "Add GetNames native + insertion-ordered hash keys"
```

---

## Task 2: `If()`/`IIF()` com curto-circuito

**Files:**
- Modify: `pkg/compiler/codegen.go` (início de `compileCallExpr`, ~957)
- Test: `tests/ifshort_test.prw`

**Interfaces:**
- Consumes: nada de tasks anteriores.
- Produces: `If(cond, a, b)` / `IIF(cond, a, b)` com 3 argumentos avaliam só o ramo escolhido.

- [ ] **Step 1: Write the failing test**

Criar `tests/ifshort_test.prw`:

```advpl
User Function IfShortTst()
    Local aPos := {}
    Local aNeg := {}
    Local aNums := {3, -2, 5, -8}
    Local i := 0
    For i := 1 To Len(aNums)
        If(aNums[i] > 0, aAdd(aPos, aNums[i]), aAdd(aNeg, aNums[i]))
    Next i
    ConOut("pos (esperado 2): " + Str(Len(aPos)))
    ConOut("neg (esperado 2): " + Str(Len(aNeg)))
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `./advplc run tests/ifshort_test.prw`
Expected: FAIL — os dois ramos rodam a cada iteração:
```
pos (esperado 2): 4
neg (esperado 2): 4
```

- [ ] **Step 3: Adicionar a forma especial no compilador**

Em `pkg/compiler/codegen.go`, no início de `func (c *Compiler) compileCallExpr(e *ast.CallExpr) error {`, **antes** da checagem `if _, isClass := c.bc.Classes[e.Name]; isClass {`, inserir:

```go
	// If()/IIF() com 3 argumentos: forma especial de curto-circuito — avalia só
	// o ramo escolhido (o native avaliaria os dois, pois args são calculados antes).
	if up := strings.ToUpper(e.Name); (up == "IF" || up == "IIF") && len(e.Args) == 3 {
		if err := c.compileExpr(e.Args[0]); err != nil {
			return err
		}
		jFalse := c.emitJump(OP_JUMP_IF_FALSE, e.Loc.Line)
		if err := c.compileExpr(e.Args[1]); err != nil {
			return err
		}
		jEnd := c.emitJump(OP_JUMP, e.Loc.Line)
		c.patchJump(jFalse, len(c.bc.Code))
		if err := c.compileExpr(e.Args[2]); err != nil {
			return err
		}
		c.patchJump(jEnd, len(c.bc.Code))
		return nil
	}
```

(`strings` já é importado em `codegen.go`; `OP_JUMP_IF_FALSE` remove a condição da pilha ao saltar.)

- [ ] **Step 4: Rebuild**

Run: `go build -o advplc ./cmd/advplc`
Expected: sem erros.

- [ ] **Step 5: Run test to verify it passes**

Run: `./advplc run tests/ifshort_test.prw`
Expected:
```
pos (esperado 2): 2
neg (esperado 2): 2
```

- [ ] **Step 6: Regressão (inclui os exemplos, que usam `If()` com args puros)**

Run: `go test ./pkg/... 2>&1 | grep -v '^ok\|no test files'`
Expected: sem saída.
Run: `./advplc run pt_nn.prw 2>&1 | tail -1`
Expected: `OK: 3/3 verificacoes passaram.`

- [ ] **Step 7: Commit**

```bash
git add pkg/compiler/codegen.go tests/ifshort_test.prw
git commit -m "If/IIF short-circuit: eval only the chosen branch"
```

---

## Task 3: Closures aninhadas (upvalues tipados, estilo Lua)

**Files:**
- Modify: `pkg/compiler/opcodes.go` (`FunctionInfo.UpvalSlots []int` → `Upvals []UpvalDesc`; novo tipo `UpvalDesc` + constantes)
- Modify: `pkg/compiler/codegen.go` (`funcContext.upvalSlots` → `upvalDescs`; `resolveUpvalue` recursivo + `addUpval`; `compileCodeBlock`)
- Modify: `pkg/vm/vm.go` (caso `OP_NEW_CODEBLOCK`)
- Test: `tests/nestclosure_test.prw`

**Interfaces:**
- Consumes: mecanismo de upvalue de nível único já existente (`CodeBlockValue.Upvalues []*Value`, `OP_LOAD_UPVAL`/`OP_STORE_UPVAL`, `funcContext.parent`).
- Produces: `compiler.UpvalDesc{ Kind uint8; Index int }`, `compiler.UpvalLocal = 0`, `compiler.UpvalParent = 1`, `FunctionInfo.Upvals []UpvalDesc`. Captura em profundidade N níveis.

- [ ] **Step 1: Write the failing test**

Criar `tests/nestclosure_test.prw`:

```advpl
User Function NestClosTst()
    Local nSoma := 0
    Local aX := {10, 20}
    Local aY := {1, 2}
    // bloco interno captura nSoma (Local da função, 2 níveis acima) e x (param do bloco externo)
    AEval(aX, {|x| AEval(aY, {|y| nSoma := nSoma + x + y }) })
    // (10+1)+(10+2)+(20+1)+(20+2) = 66
    ConOut("soma aninhada (esperado 66): " + Str(nSoma))
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `./advplc run tests/nestclosure_test.prw`
Expected: FAIL — o bloco interno não alcança `nSoma` (2 níveis acima); soma fica errada (`0`).

- [ ] **Step 3: Trocar `UpvalSlots []int` por `Upvals []UpvalDesc` em `opcodes.go`**

Em `pkg/compiler/opcodes.go`, na struct `FunctionInfo`, trocar a linha `UpvalSlots  []int` por:

```go
	Upvals      []UpvalDesc // closures: origem de cada upvalue capturado por este codeblock
```

E adicionar, perto do topo do arquivo (após a definição de `Instruction` ou antes de `FunctionInfo`):

```go
// UpvalDesc descreve a origem de um upvalue capturado por um codeblock (closure).
type UpvalDesc struct {
	Kind  uint8 // UpvalLocal (slot do frame envolvente) ou UpvalParent (upvalue do bloco pai)
	Index int
}

const (
	UpvalLocal  uint8 = 0 // captura frame.Locals[Index]
	UpvalParent uint8 = 1 // captura parentBlock.Upvalues[Index]
)
```

- [ ] **Step 4: Ajustar `funcContext` e a resolução recursiva em `codegen.go`**

Em `pkg/compiler/codegen.go`, na struct `funcContext`, trocar `upvalSlots []int` por:

```go
	upvalDescs []UpvalDesc    // índice de upvalue → origem (LOCAL/UPVAL)
```

Substituir a função `resolveUpvalue` existente por esta versão recursiva + helper:

```go
// resolveUpvalue: se `name` é capturável do escopo envolvente (Local do pai ou
// upvalue do pai, recursivamente), aloca/reusa um upvalue e retorna seu índice.
func (c *Compiler) resolveUpvalue(name string) (int, bool) {
	return c.resolveUpvalueIn(c.currentFunc, name)
}

func (c *Compiler) resolveUpvalueIn(fc *funcContext, name string) (int, bool) {
	if fc == nil || fc.parent == nil {
		return 0, false
	}
	if idx, ok := fc.upvals[name]; ok {
		return idx, true
	}
	if slot, ok := fc.parent.locals[name]; ok && slot&0x8000 == 0 {
		return c.addUpval(fc, name, UpvalDesc{Kind: UpvalLocal, Index: slot})
	}
	if pidx, ok := c.resolveUpvalueIn(fc.parent, name); ok {
		return c.addUpval(fc, name, UpvalDesc{Kind: UpvalParent, Index: pidx})
	}
	return 0, false
}

func (c *Compiler) addUpval(fc *funcContext, name string, d UpvalDesc) (int, bool) {
	idx := len(fc.upvalDescs)
	if fc.upvals == nil {
		fc.upvals = make(map[string]int)
	}
	fc.upvals[name] = idx
	fc.upvalDescs = append(fc.upvalDescs, d)
	return idx, true
}
```

- [ ] **Step 5: Gravar `Upvals` no `FunctionInfo` em `compileCodeBlock`**

Em `pkg/compiler/codegen.go`, em `compileCodeBlock`, trocar a linha `info.UpvalSlots = c.currentFunc.upvalSlots` por:

```go
	info.Upvals = c.currentFunc.upvalDescs // origem dos upvalues (closures aninhadas)
```

- [ ] **Step 6: Captura tipada em `OP_NEW_CODEBLOCK` na VM**

Em `pkg/vm/vm.go`, no caso `OP_NEW_CODEBLOCK`, substituir o bloco de captura (o `if info, ok := v.bc.Functions[funcName]; ok && len(info.UpvalSlots) > 0 ...`) por:

```go
		// Closure: captura por referência os slots do frame atual (LOCAL) ou os
		// upvalues do bloco pai (UPVAL, encadeando níveis).
		if info, ok := v.bc.Functions[funcName]; ok && len(info.Upvals) > 0 && v.current != nil {
			cb.Upvalues = make([]*advplrt.Value, len(info.Upvals))
			parentCb, _ := v.current.Locals[0].(*advplrt.CodeBlockValue)
			for i, d := range info.Upvals {
				if d.Kind == compiler.UpvalParent {
					if parentCb != nil && d.Index >= 0 && d.Index < len(parentCb.Upvalues) {
						cb.Upvalues[i] = parentCb.Upvalues[d.Index]
						continue
					}
				} else {
					if d.Index >= 0 && d.Index < len(v.current.Locals) {
						cb.Upvalues[i] = &v.current.Locals[d.Index]
						continue
					}
				}
				var box advplrt.Value = advplrt.Nil
				cb.Upvalues[i] = &box
			}
		}
```

- [ ] **Step 7: Rebuild**

Run: `go build -o advplc ./cmd/advplc`
Expected: sem erros.

- [ ] **Step 8: Run test to verify it passes**

Run: `./advplc run tests/nestclosure_test.prw`
Expected:
```
soma aninhada (esperado 66): 66
```

- [ ] **Step 9: Regressão (closures de nível único devem continuar OK)**

Run: `go test ./pkg/... 2>&1 | grep -v '^ok\|no test files'`
Expected: sem saída.
Run: `./advplc run tests/closures_test.prw`
Expected:
```
AEval acumula em Local externo (esperado 60): 60
Eval le capturas (esperado 47): 47
counter que escapou (esperado 3): 3
```

- [ ] **Step 10: Commit**

```bash
git add pkg/compiler/opcodes.go pkg/compiler/codegen.go pkg/vm/vm.go tests/nestclosure_test.prw
git commit -m "Nested closures: typed upvalues (LOCAL/UPVAL) with recursive capture"
```

---

## Task 4: `Private` com escopo dinâmico

**Files:**
- Modify: `pkg/compiler/opcodes.go` (opcodes `OP_LOAD_DYN`/`OP_STORE_DYN`/`OP_DECL_DYN` + nomes)
- Modify: `pkg/compiler/codegen.go` (`compileVarDecl` para Private/Public; fallback dinâmico no load/store de `*ast.Ident`)
- Modify: `pkg/vm/vm.go` (`VM.dynEnv`; `CallFrame.dynRestore` + tipo `dynBinding`; execução dos opcodes; restauração em `doReturn`; init em `NewVM`)
- Test: `tests/dynprivate_test.prw`

**Interfaces:**
- Consumes: `OP_JUMP` etc. já existentes; nada das tasks anteriores em termos de tipos.
- Produces: `Private`/`Public` viram variáveis dinâmicas; nome não declarado dentro de função resolve dinamicamente.

- [ ] **Step 1: Write the failing test**

Criar `tests/dynprivate_test.prw`:

```advpl
User Function DynPrivTst()
    Private cCtx := "A"
    ModB()
    ConOut("apos ModB (esperado Z): " + cCtx)
Return

Static Function ModB()
    ConOut("B ve (esperado A): " + cCtx)
    cCtx := "Z"
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `./advplc run tests/dynprivate_test.prw`
Expected: FAIL — `Private` não propaga; `ModB` não enxerga `cCtx` de `DynPrivTst` e sua escrita não volta:
```
B ve (esperado A):
apos ModB (esperado Z): A
```

- [ ] **Step 3: Adicionar os opcodes dinâmicos**

Em `pkg/compiler/opcodes.go`, no fim do enum `Opcode` (antes do `)` que fecha o bloco `const`), adicionar:

```go
	// Variáveis dinâmicas (Private/Public e nomes não declarados dentro de função).
	// Str = nome. DECL cria/sombra o binding (com restauração no return da função).
	OP_LOAD_DYN
	OP_STORE_DYN
	OP_DECL_DYN
```

No mapa `opcodeNames`, adicionar:

```go
	OP_LOAD_DYN:  "LOAD_DYN",
	OP_STORE_DYN: "STORE_DYN",
	OP_DECL_DYN:  "DECL_DYN",
```

- [ ] **Step 4: VM — ambiente dinâmico, restauração e execução**

Em `pkg/vm/vm.go`:

(a) Na struct `VM`, adicionar o campo:

```go
	dynEnv map[string]advplrt.Value // variáveis dinâmicas (Private/Public), escopo por pilha de chamadas
```

(b) Na struct `CallFrame`, adicionar:

```go
	dynRestore []dynBinding // bindings dinâmicos a restaurar quando este frame retornar
```

E o tipo (perto de `CallFrame`):

```go
type dynBinding struct {
	name string
	had  bool
	prev advplrt.Value
}
```

(c) Em `NewVM`, no literal do `VM{...}`, adicionar:

```go
		dynEnv: make(map[string]advplrt.Value),
```

(d) No `switch instr.Op` do `execute`, adicionar os casos:

```go
	case compiler.OP_LOAD_DYN:
		if val, ok := v.dynEnv[instr.Str]; ok {
			v.push(val)
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_STORE_DYN:
		v.dynEnv[instr.Str] = v.pop()
	case compiler.OP_DECL_DYN:
		prev, had := v.dynEnv[instr.Str]
		if v.current != nil {
			v.current.dynRestore = append(v.current.dynRestore, dynBinding{name: instr.Str, had: had, prev: prev})
		}
		v.dynEnv[instr.Str] = advplrt.Nil
```

(e) Em `doReturn`, no início da função (antes de `oldFrame := v.current` ou logo após), restaurar os bindings do frame que está saindo:

```go
func (v *VM) doReturn(val advplrt.Value) error {
	oldFrame := v.current
	// restaura os bindings dinâmicos (Private/Public) criados neste frame
	for i := len(oldFrame.dynRestore) - 1; i >= 0; i-- {
		b := oldFrame.dynRestore[i]
		if b.had {
			v.dynEnv[b.name] = b.prev
		} else {
			delete(v.dynEnv, b.name)
		}
	}
	// ... resto da função doReturn inalterado ...
```

- [ ] **Step 5: Compilador — `Private`/`Public` viram dinâmicos**

Em `pkg/compiler/codegen.go`, substituir a função `compileVarDecl` por:

```go
func (c *Compiler) compileVarDecl(s *ast.VarDecl) error {
	scope := strings.ToLower(s.Scope)
	if scope == "private" || scope == "public" {
		c.emit(OP_DECL_DYN, 0, 0, s.Name, s.Loc.Line)
		if s.Value != nil {
			if err := c.compileExpr(s.Value); err != nil {
				return err
			}
			c.emit(OP_STORE_DYN, 0, 0, s.Name, s.Loc.Line)
		}
		return nil
	}
	if s.Value != nil {
		if err := c.compileExpr(s.Value); err != nil {
			return err
		}
		idx := c.addLocal(s.Name)
		if idx&0x8000 != 0 {
			c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, s.Name, s.Loc.Line)
		} else {
			c.emit(OP_STORE_LOCAL, idx, 0, s.Name, s.Loc.Line)
		}
	} else if s.Type != "" {
		emitDefaultValue(c, s.Type, s.Name, s.Loc.Line)
	}
	return nil
}
```

- [ ] **Step 6: Compilador — fallback dinâmico no load/store de identificador**

Em `pkg/compiler/codegen.go`, no `compileStoreTarget`, caso `*ast.Ident`, trocar o `else` final (o que chama `addLocal`) por um que só usa global no escopo de arquivo e dinâmico dentro de função:

```go
	case *ast.Ident:
		if idx, ok := c.resolveLocal(target.Name); ok {
			c.emit(OP_STORE_LOCAL, idx, 0, target.Name, line)
		} else if uidx, ok := c.resolveUpvalue(target.Name); ok {
			c.emit(OP_STORE_UPVAL, uidx, 0, target.Name, line)
		} else if c.currentFunc == nil {
			idx := c.addLocal(target.Name)
			if idx&0x8000 != 0 {
				c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, target.Name, line)
			} else {
				c.emit(OP_STORE_LOCAL, idx, 0, target.Name, line)
			}
		} else {
			c.emit(OP_STORE_DYN, 0, 0, target.Name, line)
		}
```

No `compileExpr`, caso `*ast.Ident`, fazer o análogo para leitura:

```go
	case *ast.Ident:
		if idx, ok := c.resolveLocal(e.Name); ok {
			c.emit(OP_LOAD_LOCAL, idx, 0, e.Name, e.Loc.Line)
		} else if uidx, ok := c.resolveUpvalue(e.Name); ok {
			c.emit(OP_LOAD_UPVAL, uidx, 0, e.Name, e.Loc.Line)
		} else if c.currentFunc == nil {
			idx := c.addLocal(e.Name)
			if idx&0x8000 != 0 {
				c.emit(OP_LOAD_GLOBAL, idx&0x7FFF, 0, e.Name, e.Loc.Line)
			} else {
				c.emit(OP_LOAD_LOCAL, idx, 0, e.Name, e.Loc.Line)
			}
		} else {
			c.emit(OP_LOAD_DYN, 0, 0, e.Name, e.Loc.Line)
		}
```

- [ ] **Step 7: Rebuild**

Run: `go build -o advplc ./cmd/advplc`
Expected: sem erros.

- [ ] **Step 8: Run test to verify it passes**

Run: `./advplc run tests/dynprivate_test.prw`
Expected:
```
B ve (esperado A): A
apos ModB (esperado Z): Z
```

- [ ] **Step 9: Regressão completa (esta task muda a resolução de nomes — validar tudo)**

Run: `go test ./... 2>&1 | grep -v '^ok\|no test files'`
Expected: sem saída.
Run: `make test 2>&1 | tail -1`
Expected: `fixtures: NN pass, 0 fail` (NN ≥ 38, contando os fixtures novos).
Run: `./advplc run pt_nn.prw 2>&1 | tail -1`
Expected: `OK: 3/3 verificacoes passaram.`
Run: `printf 'oi\nsair\n' | ./advplc run pt_chat.prw 2>&1 | grep -c 'bot'`
Expected: ≥ 1 (o REPL responde).

Se algum fixture existente quebrar por depender de variável não declarada tratada como Local implícito, o conserto correto é declarar o `Local` que faltava nesse fixture (a mudança de comportamento é intencional e documentada na spec).

- [ ] **Step 10: Commit**

```bash
git add pkg/compiler/opcodes.go pkg/compiler/codegen.go pkg/vm/vm.go tests/dynprivate_test.prw
git commit -m "Private/Public dynamic scoping: dynamic env + name resolution"
```

---

## Task 5: Documentação

**Files:**
- Modify: `README.md` (seção de recursos de linguagem / funções)
- Modify: `CHANGELOG.md` (nova seção)

**Interfaces:**
- Consumes: as quatro features das tasks 1–4.
- Produces: docs atualizadas.

- [ ] **Step 1: Atualizar o README**

Em `README.md`, na seção "Funções de array de ordem superior (com bloco de código)", **remover** qualquer nota de limitação de closures aninhadas (se houver) e, na tabela de funções, garantir a linha de `GetNames`:

```markdown
| `GetNames(oJson)` | Array com as chaves de um JsonObject, na ordem de inserção |
```

Na seção "Recursos AdvPL", adicionar as linhas:

```markdown
- `If()`/`IIF()` com 3 argumentos fazem curto-circuito (avaliam só o ramo escolhido)
- `Private`/`Public` com escopo dinâmico (visíveis às funções chamadas)
- Closures aninhadas: codeblocks capturam Locais N níveis acima por referência
```

- [ ] **Step 2: Atualizar o CHANGELOG**

Em `CHANGELOG.md`, logo após a linha `Todas as mudanças notáveis deste projeto são documentadas aqui.`, adicionar:

```markdown
## [Não lançado]

### Robustez da linguagem (Sub-projeto 1)

- **`If()`/`IIF()` curto-circuito** — com 3 argumentos, avaliam só o ramo escolhido
  (forma especial no compilador); antes o native avaliava os dois ramos.
- **`GetNames(oJson)`** — itera as chaves de um `JsonObject` na ordem de inserção
  (o `ObjectValue` passou a manter a ordem via `SetProp`).
- **Closures aninhadas** — upvalues tipados (LOCAL/UPVAL) com resolução recursiva;
  um codeblock dentro de codeblock captura Locais N níveis acima por referência.
- **`Private`/`Public` com escopo dinâmico** — variáveis dinâmicas visíveis às
  funções chamadas até o declarante retornar (ambiente dinâmico na VM +
  `OP_LOAD_DYN`/`OP_STORE_DYN`/`OP_DECL_DYN`). Nome não declarado dentro de função
  passa a resolver dinamicamente (semântica AdvPL correta).
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: language robustness (If/IIF, GetNames, nested closures, dynamic Private)"
```

---

## Notas de verificação final

Ao término das 5 tasks:
- `go test ./...` verde.
- `make test` verde com 4 fixtures novos (`getnames_test`, `ifshort_test`, `nestclosure_test`, `dynprivate_test`).
- `pt_llm.prw`, `pt_chat.prw`, `pt_nn.prw` rodam com os mesmos resultados de antes.
- Os 4 critérios de aceite da spec satisfeitos (um por fixture).

Publicação de release (tag/CI) é decisão do usuário — não faz parte deste plano.
