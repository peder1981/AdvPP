# AdvPP — Cobertura Íntegra do TDN AdvPL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar no compilador AdvPP (Go) toda função, classe de framework e comando com especificação real no espelho local do TDN AdvPL (`~/tdn-advpl-mirror/`), como uma nova versão grande do compilador.

**Architecture:** Um arquivo Go novo por categoria/família (`<categoria>_native.go` para Functions, `classes_<familia>.go` para Classes), registrado no hook central `registerNatives()` / catálogo de classes intrínsecas. Classes de framework são structs Go despachadas pelo mesmo mecanismo de método já usado para `Class()...EndClass` de usuário; classes visuais chamam o renderer PO-UI/MSDIALOG já existente.

**Tech Stack:** Go, `pkg/vm` (VM/natives), `pkg/runtime` (`advplrt.Value`), `pkg/compiler` (bytecode), Go `testing`.

**Spec:** `docs/superpowers/specs/2026-08-09-advpp-tdn-integral-design.md`

## Global Constraints

- Fonte de requisitos por função/classe: os arquivos `.md` já commitados em `~/tdn-advpl-mirror/`, um por função/classe, cada um com Sintaxe/Parâmetros/Retorno/Exemplo extraídos do TDN. Cada task aponta o arquivo/pasta exata — leia-o antes de implementar, ele é a fonte da verdade sobre assinatura e comportamento, não este plano.
- **Nunca implementar funções marcadas como stub no `_progress.log`/relatórios do crawl** (páginas TDN sem corpo real) sem antes checar `docs/tdn-gap-stubs.md` (criado na Task 1) — se o nome está lá, pule, não invente assinatura.
- TLPP está fora de escopo (não mexer).
- Deprecated functions/classes (listadas em `~/tdn-advpl-mirror/Deprecated/`) não entram em nenhuma task — não implementar.
- Toda native nova segue exatamente o padrão de assinatura já usado no repositório: `func(args []advplrt.Value) (advplrt.Value, error)`, registrada num `map[string]func(args []advplrt.Value) (advplrt.Value, error)` passado para uma função `func (v *VM) register<Categoria>Natives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error))` (ou `func register<Categoria>Natives(...)` sem receiver se a native não precisar de estado da VM — siga o padrão de `registerGeometryNatives`/`registerMathStatNatives`, que são funções soltas, vs. `v.registerHttpNatives`, que usa o VM).
- Toda native/classe nova ganha teste Go no mesmo pacote (`pkg/vm`), seguindo o padrão de `pkg/vm/filial_test.go`: instanciar `v := NewVM(&compiler.Bytecode{}, false)` e chamar `v.natives["NOME"].Fn([]advplrt.Value{...})` diretamente — sem precisar compilar um `.prw` real.
- Nomes de native no map são sempre MAIÚSCULOS (AdvPL é case-insensitive nas chamadas, mas o registro interno é uppercase — confira `ALLTRIM`, `FWHTTPGET`, etc.).
- Sem release intermediária: nenhum bump de versão nem publish durante a série de tasks. A última task da série (Comandos) fecha com a preparação da release grande, mas a execução da release em si (build+deploy+vsix+marketplace) só acontece depois de TODAS as tasks aprovadas — é um passo manual fora deste plano, disparado pelo usuário quando a série inteira estiver pronta.
- `go build ./...` e `go test ./...` devem passar limpos após cada task.

## Standard Task Procedure (referenciado por toda task de Functions abaixo)

Cada task de categoria de Functions segue este ciclo, uma vez por função da
lista da task (pule qualquer nome presente em `docs/tdn-gap-stubs.md`):

1. **Ler a fonte**: abrir `~/tdn-advpl-mirror/Functions/<Categoria>/<Nome>.md`
   — extrair Sintaxe, Parâmetros, Retorno e o Exemplo de código AdvPL.
2. **Escrever o teste primeiro** (`pkg/vm/<categoria>_native_test.go`), um
   `func Test<Nome>(t *testing.T)` cobrindo o caso do Exemplo do TDN +
   pelo menos 1 edge case óbvio (arg ausente, tipo errado quando a doc for
   explícita sobre isso). Usar exatamente o padrão de
   `pkg/vm/filial_test.go` (mostrado no exemplo completo de cada task).
3. **Rodar o teste e confirmar que falha** com "undefined" ou resultado nil
   (a native ainda não existe/não faz nada).
4. **Implementar a native** no arquivo da categoria
   (`pkg/vm/<categoria>_native.go`), seguindo a assinatura padrão. Um
   comentário de uma linha acima da native com a assinatura AdvPL exata
   (ex. `// ALLTRIM(cString) -> cString`) — sem docstring multi-parágrafo.
5. **Rodar o teste e confirmar que passa.**
6. **Commit** (`git add pkg/vm/<categoria>_native.go pkg/vm/<categoria>_native_test.go && git commit -m "feat(vm): implementa <Nome> (TDN: <Categoria>)"`).

Repita para cada função da lista. A task só fecha quando todas (menos as
já excluídas pelo ledger) estiverem implementadas, testadas e commitadas, e
`go build ./... && go test ./...` passar limpo.

---

### Task 1: Ledger de gaps (stubs sem spec no TDN)

**Files:**
- Create: `docs/tdn-gap-stubs.md`
- Create: `scripts/find-tdn-stubs.sh` (script auxiliar, reaproveitável se o mirror for atualizado no futuro)

**Interfaces:**
- Produces: `docs/tdn-gap-stubs.md` — lista markdown de `- NOME (categoria, url TDN)` para toda função/classe/comando cujo `.md` no mirror não tem conteúdo real (foi salvo com nota de "stub"/"página vazia"/"sem conteúdo" pelos crawlers, ou corpo com menos de ~15 linhas úteis). Todas as tasks seguintes leem este arquivo antes de implementar qualquer função da sua categoria.

- [ ] **Step 1: Escrever o script de varredura**

```bash
#!/usr/bin/env bash
# scripts/find-tdn-stubs.sh
# Varre ~/tdn-advpl-mirror/ e lista toda página que os crawlers marcaram
# como stub genuíno do TDN (sem Sintaxe/Parâmetros/Retorno reais).
set -euo pipefail
MIRROR="$HOME/tdn-advpl-mirror"
OUT="docs/tdn-gap-stubs.md"

echo "# TDN — Gaps sem especificação real" > "$OUT"
echo "" >> "$OUT"
echo "Gerado por \`scripts/find-tdn-stubs.sh\` a partir de \`~/tdn-advpl-mirror/\`." >> "$OUT"
echo "Páginas do TDN sem corpo real (stub) — não implementar sem spec." >> "$OUT"
echo "" >> "$OUT"

find "$MIRROR" -name "*.md" | while read -r f; do
  rel="${f#"$MIRROR"/}"
  if grep -qi "stub\|página vazia\|sem conteúdo\|genuine.*empty\|nota:.*vazi" "$f" 2>/dev/null; then
    lines=$(wc -l < "$f")
    if [ "$lines" -lt 20 ]; then
      name=$(basename "$f" .md)
      cat_path=$(dirname "$rel")
      url=$(grep -m1 '^https://tdn.totvs.com' "$f" || echo "?")
      echo "- $name ($cat_path) — $url" >> "$OUT"
    fi
  fi
done
```

- [ ] **Step 2: Rodar o script e verificar a saída**

Run: `chmod +x scripts/find-tdn-stubs.sh && ./scripts/find-tdn-stubs.sh`
Expected: `docs/tdn-gap-stubs.md` criado com pelo menos 100 linhas de itens
(o levantamento prévio nesta sessão contou ~162 stubs prováveis nas
Functions, mais os de Classes/Comandos).

- [ ] **Step 3: Revisão manual rápida**

Abrir `docs/tdn-gap-stubs.md` e conferir 10 entradas aleatórias contra o
`.md` de origem no mirror — confirmar que são mesmo stubs (corpo vazio ou
só metadado de Confluence), não falsos positivos por a página conter a
palavra "stub" dentro de um exemplo de código legítimo. Corrigir o script
e regerar se houver falsos positivos.

- [ ] **Step 4: Commit**

```bash
git add scripts/find-tdn-stubs.sh docs/tdn-gap-stubs.md
git commit -m "chore: gera ledger de gaps TDN (funções/classes sem spec real)"
```

---

### Task 2: Functions — Validação

**Files:**
- Create: `pkg/vm/validacao_native.go`
- Create: `pkg/vm/validacao_native_test.go`
- Modify: `pkg/vm/natives.go:1964-1970` (adicionar `v.registerValidacaoNatives(natives)` ao bloco de chamadas de registro, junto das demais `register*Natives`)

**Interfaces:**
- Consumes: `advplrt.Value`, `advplrt.NewBool` (pkg/runtime — mesmo padrão usado em outras natives que retornam lógico, ex. `RPCSETENV` em `pkg/vm/filial_test.go` que retorna `*advplrt.BoolValue`)
- Produces: natives `ALLWAYSFALSE`, `ALLWAYSTRUE`, `EMPTY` registradas em `v.natives`

Lista de funções desta categoria (fonte: `~/tdn-advpl-mirror/Functions/Validacao/`): **AllwaysFalse, AllwaysTrue, Empty**. Nenhuma marcada como stub nesta categoria (confirmar contra `docs/tdn-gap-stubs.md` da Task 1 mesmo assim, antes de implementar).

- [ ] **Step 1: Ler a fonte de cada função**

Abrir `~/tdn-advpl-mirror/Functions/Validacao/AllwaysFalse.md`,
`AllwaysTrue.md`, `Empty.md` — extrair Sintaxe/Parâmetros/Retorno/Exemplo
de cada.

- [ ] **Step 2: Escrever os testes (falham primeiro)**

```go
// pkg/vm/validacao_native_test.go
package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestAllwaysFalse(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ALLWAYSFALSE"].Fn(nil)
	if err != nil {
		t.Fatalf("AllwaysFalse retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("AllwaysFalse() = %v, quer .F.", got)
	}
}

func TestAllwaysTrue(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ALLWAYSTRUE"].Fn(nil)
	if err != nil {
		t.Fatalf("AllwaysTrue retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("AllwaysTrue() = %v, quer .T.", got)
	}
}

func TestEmpty(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		arg  advplrt.Value
		want bool
	}{
		{"string vazia", advplrt.NewString(""), true},
		{"string com espacos", advplrt.NewString("   "), true},
		{"string com conteudo", advplrt.NewString("abc"), false},
		{"numero zero", advplrt.NewNumber(0), true},
		{"numero nao-zero", advplrt.NewNumber(5), false},
		{"nil", advplrt.Nil, true},
	}
	for _, c := range cases {
		got, err := v.natives["EMPTY"].Fn([]advplrt.Value{c.arg})
		if err != nil {
			t.Fatalf("Empty(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok || b.Val != c.want {
			t.Errorf("Empty(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}
```

Run: `go test ./pkg/vm/ -run 'TestAllwaysFalse|TestAllwaysTrue|TestEmpty' -v`
Expected: FAIL — `panic: runtime error: invalid memory address or nil
pointer dereference` (native ainda não existe em `v.natives`).

- [ ] **Step 3: Implementar as natives**

```go
// pkg/vm/validacao_native.go
package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerValidacaoNatives registra funções de validação de valores:
// AllwaysFalse, AllwaysTrue, Empty.
func (v *VM) registerValidacaoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// AllwaysFalse() -> lFalse
	natives["ALLWAYSFALSE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}

	// AllwaysTrue() -> lTrue
	natives["ALLWAYSTRUE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(true), nil
	}

	// Empty(xVal) -> lEmpty — verdadeiro se xVal é o valor "vazio" do seu tipo
	// (string em branco, 0, .F., data em branco, array/objeto sem elementos, nil).
	natives["EMPTY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		val := getArg(args, 0)
		return advplrt.NewBool(isEmptyValue(val)), nil
	}
}

func isEmptyValue(val advplrt.Value) bool {
	if val == nil || val == advplrt.Nil {
		return true
	}
	switch t := val.(type) {
	case *advplrt.StringValue:
		return len(strings_TrimSpace(t.Val)) == 0
	case *advplrt.NumberValue:
		return t.Val == 0
	case *advplrt.BoolValue:
		return !t.Val
	case *advplrt.ArrayValue:
		return len(t.Elements) == 0
	default:
		return false
	}
}
```

**Nota de implementação:** confira o nome real do helper de trim já usado
no arquivo (`strings.TrimSpace` via import `"strings"`, não
`strings_TrimSpace` — esse nome é só ilustrativo neste plano; use o import
padrão já presente em `pkg/vm/natives.go` como referência exata). Confira
também se `advplrt.NewBool`/`advplrt.ArrayValue`/`.Elements` são os nomes
reais no pacote `pkg/runtime` (grep `type.*Value struct` em
`pkg/runtime/*.go` antes de escrever — os nomes acima são baseados no
padrão observado em `filial_test.go`/`natives.go`, confirme antes de
compilar).

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./pkg/vm/ -run 'TestAllwaysFalse|TestAllwaysTrue|TestEmpty' -v`
Expected: PASS (todos os 3 testes, incluindo os 6 casos de `TestEmpty`)

- [ ] **Step 5: Registrar no hook central**

Em `pkg/vm/natives.go`, no bloco de chamadas de registro (linhas 1964-1970
no estado atual do arquivo — confirme o número exato antes de editar, pode
ter mudado), adicionar:

```go
	v.registerValidacaoNatives(natives)
```

junto das demais linhas `v.register*Natives(natives)`.

- [ ] **Step 6: Build completo + suite completa**

Run: `go build ./... && go test ./...`
Expected: build limpo, toda a suite passando (não só os testes novos).

- [ ] **Step 7: Commit**

```bash
git add pkg/vm/validacao_native.go pkg/vm/validacao_native_test.go pkg/vm/natives.go
git commit -m "feat(vm): implementa categoria Validação (AllwaysFalse, AllwaysTrue, Empty)"
```

---

### Tasks 3-34: demais categorias de Functions

Cada task abaixo segue **exatamente o mesmo procedimento da Task 2**
(Standard Task Procedure, seção no topo deste plano) — ler a fonte no
mirror, teste primeiro, implementar, testar, registrar no hook central em
`pkg/vm/natives.go`, build+suite completa, commit. Só o nome da categoria,
o arquivo Go e a lista de funções mudam. Não repita a explicação do
procedimento em cada task — aplique o da Task 2/Standard Task Procedure.

Para cada task: **Files** = `pkg/vm/<slug>_native.go` (create) +
`pkg/vm/<slug>_native_test.go` (create) + `pkg/vm/natives.go` (modify, uma
linha `v.register<Categoria>Natives(natives)` no bloco de registro).
**Interfaces**: produces as natives listadas, registradas em `v.natives`.
Fonte de cada função: `~/tdn-advpl-mirror/Functions/<Pasta>/<Nome>.md`.
Antes de implementar qualquer nome, cruzar contra `docs/tdn-gap-stubs.md`
(Task 1) e pular os que estiverem lá.

| Task | Categoria TDN (pasta) | Arquivo Go | Funções (fonte: mirror, confirme stubs contra a Task 1) |
|---|---|---|---|
| 3 | Sincronismo-de-dados | `sincronismo_native.go` | GlbLock, GlbNmLock, GlbNmUnlock, GlbUnlock |
| 4 | Web-Services | `webservices_native.go` | WSClassNew, WSDescData, WSDL2Parser, WSDLParser — **atenção:** levantamento prévio indica que as 4 são stubs no TDN; se a Task 1 confirmar, esta task vira só a entrada no ledger, sem código novo — feche-a como "sem trabalho, confirmado via ledger" e siga para a próxima |
| 5 | Execucao-entre-processos | `execucaoprocessos_native.go` | IPCCount, IPCGo, IPCWaitEx |
| 6 | Integracao-Excel | `integracaoexcel_native.go` | MsGetArray, SIGA |
| 7 | Manipulacao-do-bloco-de-codigo | `blococodigo_native.go` | AEVal, DBEVal, Eval, GetCbSource (confira grafia exata do símbolo no `.md` — TDN usa "AEval"/"DBEval"/"GetCBSource" com capitalização diferente do nome de arquivo em alguns casos, use a grafia do H1 da página, não do nome de arquivo) |
| 8 | Tratamento-de-email | `email_native.go` | GetMailObj, SetMailObj (MailVersion é stub confirmado — pular) |
| 9 | Interface-SFTP | `sftp_native.go` | SFTPDirLs, SFTPDwld1, SFTPDwld2, SFTPUpld1, SFTPUpld2 |
| 10 | Manipulacao-de-memoria | `memoria_native.go` | __DeleteRmt, __ListRmt, __LoadRmt, __SaveRmt (__ClearRmt é stub confirmado — pular; note o prefixo `__` faz parte do nome real da native) |
| 11 | Controle-de-acesso | `controleacesso_native.go` | ADUserValid, ComputerName, GetAuthArgs, GetCredential, GetUserFromSID, LogUserName |
| 12 | Matematica | `matematica_native.go` | Ceiling, Exp, Log, Log10, Mod, Sqrt, mais o subpágina "Trigonométricas": ACos, ASin, ATan, Atn2, Cos, Sin, Tan (ler `~/tdn-advpl-mirror/Functions/Matematica/Trigonometricas.md` ou equivalente pra achar a lista exata) |
| 13 | Manipulacao-de-variaveis-globais | `varglobais_native.go` | ClearGlbValue, GetGlbValue, GetGlbVars, MemGlbSize, PutGlbValue, PutGlbVars, TimeGlbValue |
| 14 | Verificacao-dos-tipos-de-variaveis | `verificatipos_native.go` | ContType, Type, ValType, VarRef, VarSetGet, VarUnref (ClearVarSetGet é stub confirmado — pular) |
| 15 | SAML | `saml_native.go` | getSAMLID, getSAMLSvc, reloadSAML, saveIDPXML, setIDPConf, setSAMLID, setSAMLSvc, setSPCert — **atenção:** levantamento prévio indica que as 8 são stubs; confirme contra a Task 1, provável que esta task feche sem código novo |
| 16 | Manipulacao-de-classe | `manipclasse_native.go` | AttlsMemberOf (confirme grafia exata — TDN tem inconsistência de capitalização "AttlsMemberOf" vs "AttIsMemberOf", use a do H1), ClassDataArr, ClassMethArr, FindClass, GetClassName, MethIsMemberOf (DelClassIntf, FreeObj, GetParentTree são stubs confirmados — pular) |
| 17 | Manipulacao-de-RPO | `rpo_native.go` | ChkRpoChg, GetAPOInfo, GetApoRes, GetDependency, GetFuncArray, GetRpoLog, GetSrcArray, Resource2File, RetImgType |
| 18 | Manipulacao-de-variaveis-numericas | `varnumericas_native.go` | Abs, Int, Max, Min, NAnd, NOr, NXor, Random, Randomize, Round |
| 19 | Manipulacao-de-matriz-HashMap | `matrizhashmap_native.go` | AToHM, HMAdd, HMClean, HMDel, HMGet, HMGetN, HMKey, HMList, HMNew, HMSet, HMSetN |
| 20 | Manipulacao-do-arquivo-INI | `arquivoini_native.go` | DeleteKeyINI, DeleteSectionINI, GetINISessions, GetProfInt, GetProfString, GetPvProfileInt, GetPvProfString, GetSrvProfString, WritePProString, WriteProfString, WriteSrvProfString |
| 21 | Decimais-de-Ponto-Fixo | `decimaisponto_native.go` | DEC_ADD, DEC_DIV, DEC_MUL, DEC_NEW, DEC_POW, DEC_RESCALE, DEC_RESIZE, DEC_ROUND, DEC_SUB, DEC_TO_DBL, DEC_TOSTR (ler o índice `~/tdn-advpl-mirror/Functions/Decimais-de-Ponto-Fixo/Decimais-de-Ponto-Fixo.md` pra lista exata e completa — o crawl também salvou 2 subpáginas de explicação conceitual, sem função nova associada) |
| 22 | Manipulacao-de-matriz | `matriz_native.go` | Array, AAdd, AClone, ACopy, ADel, AFill, AIns, AScan, AScanX, ASize, ASort, ATail |
| 23 | Tratamento-de-XML | `xml_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Tratamento-de-XML/Tratamento-de-XML.md` pra lista exata (13 funções) |
| 24 | Manipulacao-de-Data-Hora | `datahora_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Manipulacao-de-Data-Hora/Manipulacao-de-Data-Hora.md` pra lista exata (17 funções) |
| 25 | Controle-de-processamento | `controleprocessamento_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Controle-de-processamento/Controle-de-processamento.md` pra lista exata (18 funções) |
| 26 | Controle-de-impressao | `controleimpressao_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Controle-de-impressao/Controle-de-impressao.md` pra lista exata (~19-20 funções, algumas stub confirmadas — cruzar com Task 1) |
| 27 | Conversao-entre-tipos-e-dados | `conversaotipos_native.go` | __HEXTODEC (stub, pular), Bin2D, Bin2F, Bin2I, Bin2L, Bin2Str, Bin2W, BmpToJpg, ColorToRGB, CToD, cValToChar, D2Bin, Dbl2Dt, Dt2Dbl, DToC, DToS, F2Bin, GetDtoDate, I2Bin, L2Bin, Str, StrZero, Val, W2Bin |
| 28 | Manipulacao-de-variaveis-globais-HashMap | `varglobaishashmap_native.go` | VarBeginT, VarClean, VarCleanA, VarCleanX, VarDel, VarDelA, VarDelX, VarEndT, VarGet, VarGetA, VarGet_A, VarGetAA, VarGetAD, VarGetD, VarGetX, VarGetXA, VarGetXD, VarIsUID, VarSet, VarSetA, VarSetAD, VarSetD, VarSetUID, VarSetX, VarSetXD |
| 29 | Interface-HTTP | `interfacehttp_native.go` | HTTPCGet, HTTPCPost, HTTPGet, HTTPGetStatus, HTTPPost, HTTPQuote, HTTPSGet, HTTPSPost, HTTPSQuote, SetNoProxyFor, SetProxy, mais os valores de "Valores de Content-Types" (ver `~/tdn-advpl-mirror/Functions/Interface-HTTP/Valores-de-Content-Types.md` — pode ser só uma tabela de constantes, não funções; avaliar se vira native ou constante de compilador). **Atenção:** ~20 outras páginas desta categoria são stub confirmadas (HttpCache, HttpCountSession, HTTPCTDisp, etc.) — cruzar com Task 1 e pular. Nota: `FWHTTPGET`/`FWHTTPPOST`/etc já existem (`httpclient_native.go`) — essas são funções `HTTP*`/`HTTPS*` **sem** o prefixo `FW`, uma API TOTVS mais antiga/paralela, não duplicata |
| 30 | Manipulacao-de-string | `string_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Manipulacao-de-string/Manipulacao-de-string.md` pra lista completa (51 funções) — cruzar cada uma contra `impl_natives` atual (`ALLTRIM`/`AT`/`LEFT`/`RIGHT`/`UPPER`/`LOWER` já existem em `natives.go`, não duplicar; as ~45 restantes — ANSIToOEM, Asc, BitOn, Chr, Compress, Decode64, DecodeUTF8, DecodeUTF16, Descend, Encode64, EncodeUTF8, EncodeUTF16, GetDToVal, GzStrComp, GzStrDecomp, IsAlpha, IsDigit, IsLower, IsUpper, Len, Look4Bit, LTrim, Match, MathC, MLCount, NotBit, OEMToANSI, Pad, PadC, PadL, PadR, RAt, Replicate, RTrim, Space, STRICONV, StrTokArr, StrTokArr2, StrTran, Stuff, StuffBit, SubStr, Transform (stub confirmado — pular), UnCompress, UnStuff — são as novas) |
| 31 | Manipulacao-de-arquivos-discos-IO | `arquivosdiscosio_native.go` | ler índice `~/tdn-advpl-mirror/Functions/Manipulacao-de-arquivos-discos-IO/Manipulacao-de-arquivos-discos-IO.md` pra lista completa (52 funções, ~12 stub confirmadas — cruzar com Task 1). Cuidado com `FT_FEOF` — o crawl encontrou um bug real de conteúdo no próprio TDN (título diz FT_FEOF, corpo documenta FT_FGoto) — implemente `FT_FEOF` e `FT_FGoto` como funções DIFERENTES mesmo assim (o nome da native é o que importa, não o conteúdo cruzado da doc — mas verifique com testes reais de AdvPL se sua semântica realmente diverge, e documente a discrepância num comentário se implementar `FT_FEOF` com base em análise própria em vez do texto capturado) |
| 32 | Ambiente | `ambiente_native.go` (subpasta `Funcoes-genericas`) | ler índice `~/tdn-advpl-mirror/Functions/Ambiente/Funcoes-genericas/Funcoes-genericas.md` pra lista completa (57 funções, ~15 stub confirmadas) |
| 33 | Banco-de-Dados | 3 arquivos: `bancodedados_ctree_native.go`, `bancodedados_dbaccess_native.go`, `bancodedados_generico_native.go` (3 subcategorias — 3 registros separados no hook, mas pode ser 1 task só cobrindo as 3, dado que juntas somam ~130 funções reais é grande demais pra deixar fora de fase própria — **decisão de execução:** se o implementador achar as 3 subcategorias juntas grandes demais pra 1 task só, dividir esta task em 3 sub-tasks 33a/33b/33c no momento da execução, seguindo o mesmo padrão) | ler os 3 índices em `~/tdn-advpl-mirror/Functions/Banco-de-Dados/{Funcoes-cTree,Funcoes-DBAccess,Funcoes-genericas}/*.md` |
| 34 | Componentes-de-interface-visual + Seguranca | `componentesui_native.go` + `seguranca_criptografia_native.go` + `seguranca_generica_native.go` | ler índices em `~/tdn-advpl-mirror/Functions/Componentes-de-interface-visual/{Funcoes-genericas,Remote}/*.md` e `~/tdn-advpl-mirror/Functions/Seguranca/{Criptografia,Generica}/*.md` — muitas stub confirmadas em Componentes, cruzar com Task 1 |

---

### Standard Class Procedure (referenciado pelas Tasks 35-38)

O mecanismo de classe de framework nativa **já existe e está comprovado**
no repositório (usado por `FWGridProcess`, `FWMBrowse`, `MsDialog`, `LLM`,
`JsonObject`, entre outras) — não precisa ser inventado. Localização
exata: `pkg/vm/vm.go`.

- `func (v *VM) newInstance(className string, _ []advplrt.Value) error`
  (por volta da linha 1700): tem um `switch upperName` que, para cada
  classe de framework conhecida, chama um construtor `newXxxObject()` que
  devolve um `*advplrt.ObjectValue` com `.Native` populado com um struct Go
  próprio da classe (ex. `newGridObject()` em `pkg/vm/grid.go`), e dá
  `v.push(obj)`.
- `func (v *VM) callNativeMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error`
  (por volta da linha 1500): tem um `switch obj.ClassName` que despacha
  pro método Go da classe (ex. `case "FWGridProcess": return
  v.callGridProcessMethod(obj, upperMethod, args)`).

Padrão exato a replicar por classe nova (baseado em `pkg/vm/grid.go`,
arquivo de referência completo — leia-o inteiro antes de começar):

```go
// pkg/vm/classes_<familia>.go
package vm

import advplrt "github.com/advpl/compiler/pkg/runtime"

// <nome>State é o estado Go da classe T<Nome> (TDN: <descrição curta>).
type tDialogState struct {
	// campos conforme as Propriedades documentadas no .md da classe
}

func newTDialogObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TDialog", nil)
	obj.Native = &tDialogState{}
	return obj
}

func (v *VM) callTDialogMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*tDialogState)
	if !ok {
		return fmt.Errorf("TDialog: objeto sem estado interno")
	}
	switch method {
	case "NEW":
		// ler ~/tdn-advpl-mirror/Classes/.../TDialog.md pra assinatura exata de New()
		v.push(obj)
	// um case por método público documentado no .md da classe
	default:
		return fmt.Errorf("unknown method %s on TDialog", strings.ToLower(method))
	}
	return nil
}
```

E então, em `pkg/vm/vm.go`, adicionar UMA entrada em cada um dos dois
switches acima por classe nova: `case "TDIALOG": v.push(newTDialogObject()); return nil`
em `newInstance`, e `case "TDialog": return v.callTDialogMethod(obj, upperMethod, args)`
em `callNativeMethod`.

Teste seguindo o mesmo padrão de `filial_test.go`: instanciar `v :=
NewVM(&compiler.Bytecode{}, false)`, chamar `v.newInstance("TDIALOG",
nil)`, `v.pop()` pra pegar o objeto, e então chamar métodos via
`v.callNativeMethod(obj, "METODO", args)` conferindo o retorno/estado.

---

### Task 35: Classes — Diálogo

**Files:**
- Create: `pkg/vm/classes_dialogo.go`
- Create: `pkg/vm/classes_dialogo_test.go`
- Modify: `pkg/vm/vm.go` (adicionar entradas em `newInstance` e `callNativeMethod`, seguindo o Standard Class Procedure acima)

**Interfaces:**
- Consumes: mecanismo de `newInstance`/`callNativeMethod` já existente em `pkg/vm/vm.go` (não recriar — só adicionar entradas nos switches existentes).
- Produces: as classes da família Diálogo (2 classes, conforme levantamento desta sessão — nomes exatos a confirmar em `~/tdn-advpl-mirror/Classes/Janelas/Dialogo/` ou pasta equivalente do mirror; uma delas é `TDialog`) registradas em `v.classes`/despacháveis via `callNativeMethod`.

- [ ] **Step 1: Confirmar os nomes exatos das 2 classes desta família**

Rodar: `find ~/tdn-advpl-mirror/Classes -ipath "*dialogo*" -o -ipath "*entrada*"` (a estrutura exata da pasta não foi capturada em detalhe nesta sessão — localizar via find/grep antes de prosseguir).

- [ ] **Step 2-N: Implementar cada classe desta família**

Seguindo o Standard Task Procedure (teste primeiro por método público
documentado, ler `~/tdn-advpl-mirror/Classes/.../<Classe>.md` como fonte) e
o Standard Class Procedure acima (padrão de `newXxxObject`/`callXxxMethod`
+ entradas nos dois switches de `vm.go`).

- [ ] **Step Final: Build completo + suite completa + commit**

Run: `go build ./... && go test ./...`

```bash
git add pkg/vm/classes_dialogo.go pkg/vm/classes_dialogo_test.go pkg/vm/vm.go
git commit -m "feat(vm): família de classes Diálogo (TDialog e afins)"
```

---

### Task 36: Classes — Mobile

**Files:**
- Create: `pkg/vm/classes_mobile.go`
- Create: `pkg/vm/classes_mobile_test.go`
- Modify: `pkg/vm/vm.go` (entradas em `newInstance`/`callNativeMethod`)

**Interfaces:**
- Consumes: mecanismo `newInstance`/`callNativeMethod` em `pkg/vm/vm.go` (Standard Class Procedure, definido antes da Task 35).
- Produces: as 6 classes da família Mobile registradas (nomes exatos: ler `~/tdn-advpl-mirror/Classes/` — pasta/índice da família Mobile, 6 classes conforme levantamento desta sessão).

Segue o Standard Task Procedure + Standard Class Procedure. Ler cada `.md`
de classe em `~/tdn-advpl-mirror/Classes/.../Mobile/` (ou caminho
equivalente encontrado), teste primeiro por método público documentado,
implementar (`newXxxObject`/`callXxxMethod` + entradas nos 2 switches de
`vm.go`), build+suite, commit.

---

### Task 37: Classes — Não Visual

**Files:**
- Create: `pkg/vm/classes_naovisual.go`
- Create: `pkg/vm/classes_naovisual_test.go`
- Modify: `pkg/vm/vm.go` (entradas em `newInstance`/`callNativeMethod`)

**Interfaces:**
- Consumes: mecanismo `newInstance`/`callNativeMethod` (Standard Class Procedure).
- Produces: as 22 classes da família Não Visual.

Segue o Standard Task Procedure + Standard Class Procedure. Fonte:
`~/tdn-advpl-mirror/Classes/.../Nao-Visual/` (confirmar caminho exato —
22 classes conforme levantamento desta sessão). Dado o volume (22
classes), considerar dividir em 2-3 sub-commits dentro da mesma task se o
diff ficar grande demais pra revisão de uma vez, mas fechar a task inteira
antes de passar pra próxima.

---

### Task 38: Classes — Visual

**Files:**
- Create: `pkg/vm/classes_visual.go` (ou múltiplos arquivos se 59 classes
  num arquivo só ficar grande demais — dividir por sub-grupo temático se
  necessário, ex. `classes_visual_controles.go` +
  `classes_visual_janelas.go`, decisão de execução no momento da task)
- Create: teste(s) correspondente(s)
- Modify: `pkg/vm/vm.go` (entradas em `newInstance`/`callNativeMethod`)

**Interfaces:**
- Consumes: mecanismo `newInstance`/`callNativeMethod` (Standard Class
  Procedure) + renderer PO-UI/MSDIALOG já existente. Localizar a interface
  exata de renderização antes de começar: `grep -rn "callMsDialogMethod\|MsDialog" pkg/vm/*.go`
  (já existe `case "MsDialog": return v.callMsDialogMethod(obj, upperMethod, args)`
  em `callNativeMethod` — ler `pkg/vm/*.go` onde `callMsDialogMethod` está
  definido como referência direta de "classe visual nativa que já
  desenha", em vez de `grid.go` que é não-visual).
- Produces: as 59 classes da família Visual (TWindow, TGet, TBrowse e as demais), com métodos de exibição roteados para o renderer existente.

Segue o Standard Task Procedure + Standard Class Procedure, com um passo
adicional antes do Step 1: ler a implementação de `callMsDialogMethod` (ou
equivalente) inteira, pois é o exemplo mais próximo já existente de uma
classe visual nativa — essa é a interface que cada classe visual nova vai
chamar por baixo dos panos em vez de reimplementar desenho do zero. Dado o
tamanho e risco desta task, ela é a única deste plano onde vale a pena
escrever, ANTES de implementar as 59 classes, uma classe piloto completa
(TWindow, a mais fundamental) com revisão própria antes de prosseguir pras
outras 58 — se o padrão do piloto não funcionar bem, é mais barato
descobrir isso numa classe do que em 59.

---

### Task 39: Comandos restantes

**Files:**
- Modify: arquivo(s) onde comandos já existentes estão registrados (localizar via `grep -rln "#command\|#xcommand\|registerCommand" pkg/compiler/*.go pkg/vm/*.go` — os 23 comandos documentados no TDN podem já estar parcialmente cobertos pela engine de `#command`/`#xcommand` v1.8.0; esta task cobre só o que faltar)
- Create: testes correspondentes

**Interfaces:**
- Consumes: engine de `#command`/`#xcommand` (v1.8.0, já implementada — ver `docs/superpowers/specs/` de sessões anteriores para o desenho dessa engine antes de mexer nela).
- Produces: os comandos do TDN (`~/tdn-advpl-mirror/Comandos/`, 23 entradas — lista completa em `~/tdn-advpl-mirror/Comandos/AdvPL-Comandos.md` e nos arquivos individuais) que ainda não têm definição de `#xcommand` equivalente no compilador.

- [ ] **Step 1: Mapear o que já existe**

Ler `~/tdn-advpl-mirror/Comandos/AdvPL-Comandos.md` e cada `.md` individual
(incluindo os salvos como `@ ... BITMAP`, `@ ... BROWSE`, `@ ... CHECKBOX`,
`@ ... FOLDER`, `@ ... GET`, `@ ... GET MULTILINE`, `@ ... GROUP`, `@ ...
MSPANEL`, `@ ... SAY`, `Comando TCQUERY`, `Comando USE`, e os demais
listados no índice). Para cada um, checar se já existe uma definição
`#xcommand` equivalente na engine atual — se sim, marcar como já coberto
e não reimplementar.

- [ ] **Step 2-N: Implementar o que faltar**

Para cada comando sem cobertura, seguir o padrão de definição de
`#xcommand` já usado pelos comandos existentes (ler um exemplo real no
compilador antes de escrever um novo), com teste que compila um `.prw` de
exemplo usando o comando e confere o bytecode/comportamento gerado.

- [ ] **Step Final: Build completo + suite completa + commit**

Run: `go build ./... && go test ./...`

```bash
git add <arquivos modificados>
git commit -m "feat(compiler): comandos TDN restantes"
```

---

## Fechamento da série (fora do escopo de execução automática deste plano)

Depois que a Task 39 fechar com `go build ./... && go test ./...` limpo em
toda a árvore, a série está completa. **Não iniciar release automaticamente
— pare aqui e reporte ao usuário.** A release grande (versão nova,
CHANGELOG consolidado, build+empacotamento+deploy laptop-peder/homelab+vsix
+Marketplace) é um passo manual disparado pelo usuário quando ele confirmar
que quer publicar, seguindo o mesmo processo já usado em releases
anteriores (v2.0.20, v2.0.22).
