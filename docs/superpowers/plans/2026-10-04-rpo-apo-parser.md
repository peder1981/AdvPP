# Plano 1 — `advplc rpo apo` (desmontagem de blobs APO em Go)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Subcomando `advplc rpo apo` que desmonta blobs APO extraídos (3.465 custom) em identificadores, literais, snippets de código e call-graph candidato (oráculo opcional).

**Architecture:** Novo pacote-library `pkg/rpo/apo_blob.go` (parser puro, sem I/O de rede) + CLI `cmd/advplc/cmd_rpo_apo.go` (flag set + walk de diretório + relatório JSON/MD) registrada no switch de `cmd_rpo.go`. Confiança: presença em catálogo = fato; relação de chamada = sempre `INFERIDO`.

**Tech Stack:** Go 1.27 (módulo `github.com/advpl/compiler`), stdlib (`flag`, `encoding/json`, `regexp`, `os`, `path/filepath`).

## Global Constraints

- Executar tudo no worktree `/tmp/opencode/advpp-unstable-wt` (branch `unstable`, HEAD `e4f2149`); **nunca** no working tree principal do operador; **sem push**.
- Commits em PT-BR, formato `[FEAT|FIX|DOC|TEST] — descrição`, **sem** trailers de atribuição (Lei 2 / Anexo D).
- Fixtures binárias vêm de `/tmp/opencode/advpls-test/mass/apo/` (fonte real, onda 39).
- Baseline atual verde: `go test ./pkg/rpo/...` = ok (cached); `cmd/advplc` compila lento — usar `--timeout 300s` em `go test ./cmd/advplc/`.
- Nomenclatura húngara (`c`, `n`, `b`, `a`, `o`) em variáveis; comments/explicações em PT-BR.
- Nenhum conhecimento fabricado: nomes de campo/offset afirmados aqui foram verificados por hexdump nesta sessão (EXTXDEF=39 B, TECA740A=40 B, ABSLOGGER=292 B, TLPP=67 B).

---

### Task 1: Fixtures reais + `ParseApoBlob` (magic e kind)

**Files:**
- Create: `pkg/rpo/testdata/apo/EXTXDEF.PRW`, `TECA740A.PRW`, `ABSLOGGER.PRW`, `BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP` (copiados)
- Create: `pkg/rpo/apo_blob.go`
- Test: `pkg/rpo/apo_blob_test.go`

**Interfaces:**
- Consumes: nada (primeira task).
- Produces: `type ApoKind byte` com `ApoKindAdvPL='F'`, `ApoKindTLPP='T'`; `func ParseApoBlob(data []byte) (*ApoBlob, error)`; `type ApoBlob struct { Raw []byte; Kind ApoKind; Size int }` — tasks futuras acrescentam campos ao MESMO struct.

- [ ] **Step 1: Copiar fixtures reais**

```bash
mkdir -p pkg/rpo/testdata/apo
cp /tmp/opencode/advpls-test/mass/apo/EXTXDEF.PRW \
   /tmp/opencode/advpls-test/mass/apo/TECA740A.PRW \
   /tmp/opencode/advpls-test/mass/apo/ABSLOGGER.PRW \
   pkg/rpo/testdata/apo/
cp "/tmp/opencode/advpls-test/mass/apo/BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP" \
   pkg/rpo/testdata/apo/
ls -la pkg/rpo/testdata/apo/
```
Expected: 4 arquivos (39, 40, 292, 67 bytes).

- [ ] **Step 2: Escrever o teste falhando**

`pkg/rpo/apo_blob_test.go`:
```go
package rpo

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "apo", name))
	if err != nil {
		t.Fatalf("lendo fixture %s: %v", name, err)
	}
	return data
}

func TestParseApoBlob_MagicEKindAdvPL(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "EXTXDEF.PRW"))
	if err != nil {
		t.Fatalf("ParseApoBlob: %v", err)
	}
	if blob.Kind != ApoKindAdvPL {
		t.Errorf("Kind = %q, esperado %q", blob.Kind, ApoKindAdvPL)
	}
	if blob.Size != 39 {
		t.Errorf("Size = %d, esperado 39", blob.Size)
	}
}

func TestParseApoBlob_KindTLPP(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP"))
	if err != nil {
		t.Fatalf("ParseApoBlob: %v", err)
	}
	if blob.Kind != ApoKindTLPP {
		t.Errorf("Kind = %q, esperado %q", blob.Kind, ApoKindTLPP)
	}
}

func TestParseApoBlob_MagicInvalida(t *testing.T) {
	if _, err := ParseApoBlob([]byte{0x01, 0x02, 0x03}); err == nil {
		t.Error("esperava erro para magic inválida")
	}
}

func TestParseApoBlob_KindInvalido(t *testing.T) {
	data := []byte{0x75, 0x00, 0x00, 0x46, 0x46, 'X'}
	if _, err := ParseApoBlob(data); err == nil {
		t.Error("esperava erro para kind != F/T")
	}
}
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `go test ./pkg/rpo/ -run TestParseApoBlob -v`
Expected: FAIL (`undefined: ParseApoBlob`, `undefined: ApoKindAdvPL`)

- [ ] **Step 4: Implementar**

`pkg/rpo/apo_blob.go`:
```go
package rpo

import (
	"fmt"
)

// ApoHeaderPrefix é o prefixo observado em TODOS os 3.465 blobs APO
// extraídos (onda 39): 75 00 00 46 46. Verificado por hexdump em
// EXTXDEF.PRW, TECA740A.PRW, ABSLOGGER.PRW e amostra TLPP (2026-10-04).
var ApoHeaderPrefix = []byte{0x75, 0x00, 0x00, 0x46, 0x46}

// ApoKind é o byte de tipo na posição 5 do blob.
type ApoKind byte

const (
	ApoKindAdvPL ApoKind = 'F' // PRW/PRX/PRG/APH/APW
	ApoKindTLPP  ApoKind = 'T' // TLPP
)

// String retorna a forma legível do kind.
func (k ApoKind) String() string {
	switch k {
	case ApoKindAdvPL:
		return "AdvPL"
	case ApoKindTLPP:
		return "TLPP"
	}
	return fmt.Sprintf("desconhecido(%q)", byte(k))
}

// ApoBlob é a desmontagem de um blob APO extraído via GetApoRes.
type ApoBlob struct {
	Raw  []byte
	Kind ApoKind
	Size int
}

// ParseApoBlob valida o framing mínimo (magic 5 bytes + kind) do blob APO.
// O restante do layout (tabelas de records) ainda não é100% decifrado —
// ver docs/RPO-EXTRACTION-METHODOLOGY.md; as tabelas derivadas (strings,
// literais) são preenchidas por varredura nas tasks seguintes.
func ParseApoBlob(data []byte) (*ApoBlob, error) {
	if len(data) < len(ApoHeaderPrefix)+1 {
		return nil, fmt.Errorf("apo: blob muito pequeno (%d bytes)", len(data))
	}
	for i, b := range ApoHeaderPrefix {
		if data[i] != b {
			return nil, fmt.Errorf("apo: magic inválida no offset %d (0x%02X != 0x%02X)", i, data[i], b)
		}
	}
	kind := ApoKind(data[5])
	if kind != ApoKindAdvPL && kind != ApoKindTLPP {
		return nil, fmt.Errorf("apo: kind desconhecido 0x%02X (esperado 'F' ou 'T')", data[5])
	}
	return &ApoBlob{Raw: data, Kind: kind, Size: len(data)}, nil
}
```

- [ ] **Step 5: Rodar e confirmar pass**

Run: `go test ./pkg/rpo/ -run TestParseApoBlob -v`
Expected: PASS (4 testes)

- [ ] **Step 6: Commit**

```bash
git add pkg/rpo/apo_blob.go pkg/rpo/apo_blob_test.go pkg/rpo/testdata/apo/
git commit -m "FEAT — parser de framing APO (magic 75 00 00 46 46 + kind F/T) com fixtures reais"
```

---

### Task 2: Extração de strings e nome do arquivo

**Files:**
- Modify: `pkg/rpo/apo_blob.go` (campos novos no struct + 2 funções)
- Test: `pkg/rpo/apo_blob_test.go`

**Interfaces:**
- Consumes: `ParseApoBlob` da Task 1.
- Produces: `type ApoString struct { Offset int; Text string }`; campos `ApoBlob.Strings []ApoString`, `ApoBlob.FileName string`; `func (b *ApoBlob) ExtractStrings()` — Task 3 usa `b.Strings`.

- [ ] **Step 1: Teste falhando**

Acrescentar em `pkg/rpo/apo_blob_test.go`:
```go
func TestExtractStrings_ExtXdef(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "EXTXDEF.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	if len(blob.Strings) != 1 || blob.Strings[0].Text != "EXTXDEF.PRW" {
		t.Fatalf("Strings = %+v, esperado único [EXTXDEF.PRW]", blob.Strings)
	}
	if blob.FileName != "EXTXDEF.PRW" {
		t.Errorf("FileName = %q, esperado %q", blob.FileName, "EXTXDEF.PRW")
	}
}

func TestExtractStrings_AbsloggerTemTabelaDeNomes(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "ABSLOGGER.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	got := map[string]bool{}
	for _, s := range blob.Strings {
		got[s.Text] = true
	}
	for _, want := range []string{"ABSLOGGER", "CPROCNAME", "LFFACTIVE", "USELF", "ABSLOGGER.PRW"} {
		if !got[want] {
			t.Errorf("string %q não encontrada em %+v", want, blob.Strings)
		}
	}
	if blob.FileName != "ABSLOGGER.PRW" {
		t.Errorf("FileName = %q, esperado ABSLOGGER.PRW", blob.FileName)
	}
}

func TestExtractStrings_TLPPFilename(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	if blob.FileName != "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP" {
		t.Errorf("FileName = %q", blob.FileName)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./pkg/rpo/ -run TestExtractStrings -v`
Expected: FAIL (`blob.Strings undefined`)

- [ ] **Step 3: Implementar**

Acrescentar em `pkg/rpo/apo_blob.go` (imports: `regexp`, `strings`):
```go
// ApoString é uma string imprimível localizada no blob.
type ApoString struct {
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

// apoFileExtRe reconhece o nome-do-recurso como string com extensão conhecida.
var apoFileExtRe = regexp.MustCompile(`^[A-Za-z0-9_.]+\.(PRW|TLPP|PRX|APH|APW|PRG|CH|TRES|TRP)$`)

// ExtractStrings varre o blob por runs imprimíveis (ASCII 32..126, >= 4
// chars) e deriva FileName = primeira string com extensão de resource
// conhecida (heurística verificada: filename único em fixtures "vazias"
// e presente perto do EOF em ABSLOGGER.PRW).
func (b *ApoBlob) ExtractStrings() {
	b.Strings = nil
	b.FileName = ""
	current := make([]byte, 0, 64)
	start := 0
	flush := func(end int) {
		if len(current) >= 4 {
			text := string(current)
			b.Strings = append(b.Strings, ApoString{Offset: start, Text: text})
			if b.FileName == "" && apoFileExtRe.MatchString(text) {
				b.FileName = text
			}
		}
		current = current[:0]
	}
	for i := 0; i <= len(b.Raw); i++ {
		if i < len(b.Raw) && b.Raw[i] >= 32 && b.Raw[i] <= 126 {
			if len(current) == 0 {
				start = i
			}
			current = append(current, b.Raw[i])
			continue
		}
		flush(i)
	}
}
```
E alterar o struct da Task 1 para:
```go
type ApoBlob struct {
	Raw      []byte
	Kind     ApoKind
	Size     int
	Strings  []ApoString
	FileName string
}
```

- [ ] **Step 4: Rodar e confirmar pass**

Run: `go test ./pkg/rpo/ -run "TestParseApoBlob|TestExtractStrings" -v`
Expected: PASS (7 testes; Task 1 continua verde)

- [ ] **Step 5: Commit**

```bash
git add pkg/rpo/apo_blob.go pkg/rpo/apo_blob_test.go
git commit -m "FEAT — extração de strings APO e derivação de FileName por extensão"
```

---

### Task 3: Classificação (identificador/literal/snippet) + oráculo de call-graph

**Files:**
- Modify: `pkg/rpo/apo_blob.go`
- Test: `pkg/rpo/apo_blob_test.go`
- Create: `pkg/rpo/testdata/catalog_tiny.txt`

**Interfaces:**
- Consumes: `ApoBlob.Strings`, `FileName` (Task 2).
- Produces: `type CallCandidate struct { Name string; InCatalog bool; Confidence string }`; campos `ApoBlob.Identifiers/Literals/Snippets/CallCandidates []…`; `func (b *ApoBlob) Classify(catalog map[string]bool)`; `func LoadApoCatalog(path string) (map[string]bool, error)` — Task 4 (CLI) consome as duas.

- [ ] **Step 1: Fixture de catálogo minúsculo + teste falhando**

`pkg/rpo/testdata/catalog_tiny.txt`:
```
ABSLOGGER
GETAREA
RESTAREA
```

Acrescentar em `pkg/rpo/apo_blob_test.go`:
```go
func TestClassify_IdentifierLiteralSnippet(t *testing.T) {
	blob := &ApoBlob{
		Raw:  []byte{},
		Kind: ApoKindAdvPL,
		Strings: []ApoString{
			{Offset: 0, Text: "CPROCNAME"},
			{Offset: 10, Text: "AND D_E_L_E_T_ = ' '"},
			{Offset: 30, Text: "ABSLOGGER.PRW"},
		},
		FileName: "ABSLOGGER.PRW",
	}
	blob.Classify(nil)
	if len(blob.Identifiers) != 1 || blob.Identifiers[0] != "CPROCNAME" {
		t.Errorf("Identifiers = %+v", blob.Identifiers)
	}
	if len(blob.Snippets) != 1 || blob.Snippets[0] != "AND D_E_L_E_T_ = ' '" {
		t.Errorf("Snippets = %+v", blob.Snippets)
	}
	if len(blob.Literals) != 0 {
		t.Errorf("Literals = %+v, esperado vazio (filename e snippet excluídos)", blob.Literals)
	}
}

func TestClassify_CallCandidateInferido(t *testing.T) {
	catalog, err := LoadApoCatalog(filepath.Join("testdata", "catalog_tiny.txt"))
	if err != nil {
		t.Fatal(err)
	}
	blob := &ApoBlob{
		Kind: ApoKindAdvPL,
		Strings: []ApoString{
			{Offset: 0, Text: "ABSLOGGER"},
			{Offset: 10, Text: "NAOEXISTE"},
		},
	}
	blob.Classify(catalog)
	if len(blob.CallCandidates) != 1 {
		t.Fatalf("CallCandidates = %+v, esperado só o nome em catálogo", blob.CallCandidates)
	}
	c := blob.CallCandidates[0]
	if c.Name != "ABSLOGGER" || !c.InCatalog || c.Confidence != "INFERIDO" {
		t.Errorf("candidate = %+v, esperado {ABSLOGGER true INFERIDO}", c)
	}
}

func TestLoadApoCatalog(t *testing.T) {
	catalog, err := LoadApoCatalog(filepath.Join("testdata", "catalog_tiny.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !catalog["GETAREA"] || len(catalog) != 3 {
		t.Errorf("catalog = %+v", catalog)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./pkg/rpo/ -run "TestClassify|TestLoadApoCatalog" -v`
Expected: FAIL (`blob.Classify undefined`, `LoadApoCatalog undefined`)

- [ ] **Step 3: Implementar**

`pkg/rpo/apo_blob.go` (imports novos: `bufio`, `os`, `strings` já presentes/sumar):
```go
// CallCandidate é um candidato a chamada derivado por cruzamento com
// catálogo de nomes. InCatalog é FATO (consta no catálogo);
// Confidence é a relação de chamada — sempre INFERIDO (🟡), porque o
// blob APO não distingue uso de definição sem o layout completo.
type CallCandidate struct {
	Name       string `json:"name"`
	InCatalog  bool   `json:"in_catalog"`
	Confidence string `json:"confidence"`
}

// apoIdentRe: identificador AdvPL/TLPP clássico.
var apoIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{1,63}$`)

// apoSnippetRe: conteúdo característico de código AdvPL/TLPP dentro de
// string literal (padrões validados empiricalamente contra os 3.465 blobs:
// 788 PRW / 723 TLPP / 22 PRX / 1 PRG contêm match).
var apoSnippetRe = regexp.MustCompile(`(?i):=|->|%notdel%|%xfilial|%exp:` +
	`|beginsql|endsql|select .* from |where |d_e_l_e_t_` +
	`|function |return |if\(|endif|while |for |dbselectarea` +
	`|reclock|msunlock|fwlogmsg|fwexecstatement`)

// Classify separa Strings em Identifiers / Literals / Snippets e cruza
// identificadores com o catálogo (oráculo) para gerar CallCandidates.
// Prioridade: snippet > identificador > literal; FileName é sempre
// excluído (é o próprio resource, não conteúdo).
func (b *ApoBlob) Classify(catalog map[string]bool) {
	b.Identifiers = nil
	b.Literals = nil
	b.Snippets = nil
	b.CallCandidates = nil
	for _, s := range b.Strings {
		if b.FileName != "" && s.Text == b.FileName {
			continue
		}
		switch {
		case apoSnippetRe.MatchString(s.Text):
			b.Snippets = append(b.Snippets, s.Text)
		case apoIdentRe.MatchString(s.Text):
			b.Identifiers = append(b.Identifiers, s.Text)
			if catalog != nil && catalog[strings.ToUpper(s.Text)] {
				b.CallCandidates = append(b.CallCandidates, CallCandidate{
					Name: s.Text, InCatalog: true, Confidence: "INFERIDO",
				})
			}
		default:
			b.Literals = append(b.Literals, s.Text)
		}
	}
}

// LoadApoCatalog carrega um arquivo com um nome por linha (ex.:
// wire/catalog_sec2.txt, 113.431 nomes) em map uppercase→true.
func LoadApoCatalog(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	catalog := make(map[string]bool, 128*1024)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name != "" {
			catalog[strings.ToUpper(name)] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return catalog, nil
}
```
Acrescentar os 4 campos ao struct `ApoBlob`:
```go
	Identifiers    []string
	Literals       []string
	Snippets       []string
	CallCandidates []CallCandidate
```

- [ ] **Step 4: Rodar e confirmar pass**

Run: `go test ./pkg/rpo/ -v -count=1`
Expected: PASS (todos, incluindo regressões Tasks 1–2)

- [ ] **Step 5: Commit**

```bash
git add pkg/rpo/apo_blob.go pkg/rpo/apo_blob_test.go pkg/rpo/testdata/catalog_tiny.txt
git commit -m "FEAT — classificação APO (identificador/literal/snippet) e call-graph candidato por oráculo"
```

---

### Task 4: CLI `advplc rpo apo` com saída JSON

**Files:**
- Create: `cmd/advplc/cmd_rpo_apo.go`
- Modify: `cmd/advplc/cmd_rpo.go` (case no switch linhas ~21–59 + usage linhas ~66–87)
- Test: `cmd/advplc/cmd_rpo_apo_test.go`

**Interfaces:**
- Consumes: `rpo.ParseApoBlob`, `(*ApoBlob).ExtractStrings`, `(*ApoBlob).Classify`, `rpo.LoadApoCatalog` (Tasks 1–3).
- Produces: `func cmdRpoApo(args []string) error` — registrada em `cmdRpo` como `case "apo"`; formato JSON {`file`,`kind`,`size`,`identifiers`,`literals`,`snippets`,`call_candidates`}.

- [ ] **Step 1: Teste falhando (package main)**

`cmd/advplc/cmd_rpo_apo_test.go`:
```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdRpoApo_DirGeraJSON(t *testing.T) {
	// copia as fixtures reais para um dir temporário
	src := filepath.Join("..", "..", "pkg", "rpo", "testdata", "apo")
	dir := t.TempDir()
	for _, name := range []string{"EXTXDEF.PRW", "ABSLOGGER.PRW"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("lendo fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	err := cmdRpoApo([]string{
		"--catalog", filepath.Join("..", "..", "pkg", "rpo", "testdata", "catalog_tiny.txt"),
		"--out", out,
		dir,
	})
	if err != nil {
		t.Fatalf("cmdRpoApo: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "apo_report.json"))
	if err != nil {
		t.Fatalf("lendo relatório: %v", err)
	}
	var report []struct {
		FileName       string `json:"file"`
		Kind           string `json:"kind"`
		Identifiers    []string `json:"identifiers"`
		CallCandidates []struct {
			Name       string `json:"name"`
			Confidence string `json:"confidence"`
		} `json:"call_candidates"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(report) != 2 {
		t.Fatalf("report tem %d entradas, esperado 2", len(report))
	}
	byFile := map[string]int{}
	for i, e := range report {
		byFile[e.FileName] = i
		if e.Kind == "" {
			t.Errorf("entry %d sem kind", i)
		}
	}
	extIdx, ok := byFile["EXTXDEF.PRW"]
	if !ok {
		t.Fatalf("EXTXDEF.PRW ausente: %+v", report)
	}
	if report[extIdx].Kind != "AdvPL" {
		t.Errorf("EXTXDEF kind = %q", report[extIdx].Kind)
	}
	absIdx := byFile["ABSLOGGER.PRW"]
	for _, c := range report[absIdx].CallCandidates {
		if c.Name == "ABSLOGGER" && c.Confidence != "INFERIDO" {
			t.Errorf("confidence = %q, esperado INFERIDO", c.Confidence)
		}
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./cmd/advplc/ -run TestCmdRpoApo -v --timeout 300s`
Expected: FAIL (`undefined: cmdRpoApo`)

- [ ] **Step 3: Implementar o CLI**

`cmd/advplc/cmd_rpo_apo.go`:
```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/advpl/compiler/pkg/rpo"
)

// apoExtsDefine quais arquivos de um diretório são blobs APO.
var apoExts = map[string]bool{
	".PRW": true, ".TLPP": true, ".PRX": true,
	".APH": true, ".APW": true, ".PRG": true,
}

type apoJSON struct {
	FileName       string             `json:"file"`
	Kind           string             `json:"kind"`
	Size           int                `json:"size"`
	Identifiers    []string           `json:"identifiers"`
	Literals       []string           `json:"literals"`
	Snippets       []string           `json:"snippets"`
	CallCandidates []rpo.CallCandidate `json:"call_candidates"`
}

// cmdRpoApo implementa "advplc rpo apo <arquivo|dir> [--catalog f]
// [--out dir] [--format json|md]".
func cmdRpoApo(args []string) error {
	fs := flag.NewFlagSet("apo", flag.ContinueOnError)
	catalogPath := fs.String("catalog", "", "arquivo de catálogo de nomes (oráculo call-graph)")
	outDir := fs.String("out", "", "diretório de saída (padrão: stdout)")
	format := fs.String("format", "json", "formato: json|md")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("uso: advplc rpo apo [--catalog f] [--out dir] [--format json|md] <arquivo|dir>")
	}

	var catalog map[string]bool
	if *catalogPath != "" {
		var err error
		catalog, err = rpo.LoadApoCatalog(*catalogPath)
		if err != nil {
			return fmt.Errorf("carregando catálogo: %w", err)
		}
	}

	files, err := apoCollectFiles(fs.Arg(0))
	if err != nil {
		return err
	}

	reports := make([]apoJSON, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("lendo %s: %w", path, err)
		}
		blob, err := rpo.ParseApoBlob(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AVISO: %s: %v\n", filepath.Base(path), err)
			continue
		}
		blob.ExtractStrings()
		blob.Classify(catalog)
		reports = append(reports, apoJSON{
			FileName:       blob.FileName,
			Kind:           blob.Kind.String(),
			Size:           blob.Size,
			Identifiers:    blob.Identifiers,
			Literals:       blob.Literals,
			Snippets:       blob.Snippets,
			CallCandidates: blob.CallCandidates,
		})
	}

	var payload []byte
	var filename string
	switch *format {
	case "json":
		payload, err = json.MarshalIndent(reports, "", "  ")
		filename = "apo_report.json"
	case "md":
		payload = apoRenderMD(reports, *catalogPath)
		filename = "apo_report.md"
	default:
		return fmt.Errorf("formato desconhecido %q (json|md)", *format)
	}
	if err != nil {
		return err
	}

	if *outDir == "" {
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(*outDir, filename)
	if err := os.WriteFile(dest, payload, 0644); err != nil {
		return err
	}
	fmt.Printf("Relatório: %s (%d blobs)\n", dest, len(reports))
	return nil
}

// apoCollectFiles aceita arquivo único ou diretório (walk 1 nível).
func apoCollectFiles(arg string) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	entries, err := os.ReadDir(arg)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if apoExts[strings.ToUpper(filepath.Ext(e.Name()))] {
			files = append(files, filepath.Join(arg, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("nenhum blob APO em %s", arg)
	}
	return files, nil
}

// apoRenderMD gera o relatório markdown (tabela resumo + seções por arquivo).
func apoRenderMD(reports []apoJSON, catalogName string) []byte {
	var b strings.Builder
	b.WriteString("# Relatório de desmontagem APO\n\n")
	fmt.Fprintf(&b, "Catálogo: `%s` · Blobs: %d\n\n", catalogName, len(reports))
	b.WriteString("| Arquivo | Kind | Bytes | Identificadores | Literais | Snippets | Candidatos |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|\n")
	for _, r := range reports {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d |\n",
			r.FileName, r.Kind, r.Size,
			len(r.Identifiers), len(r.Literals), len(r.Snippets), len(r.CallCandidates))
	}
	return []byte(b.String())
}
```

- [ ] **Step 4: Registrar no switch e usage**

Em `cmd/advplc/cmd_rpo.go`, após `case "inject":` (blocos da Task 5 ainda não tocados):
```go
	case "apo":
		return cmdRpoApo(args[1:])
```
No `rpoUsageError()` (string de usage), acrescentar a linha:
```
  apo <arquivo|dir> [--catalog f] [--out dir] [--format json|md]
                                        desmonta blobs APO (identificadores, literais, snippets, call-graph)
```

- [ ] **Step 5: Rodar e confirmar pass**

Run: `go test ./cmd/advplc/ -run TestCmdRpoApo -v --timeout 300s`
Expected: PASS

- [ ] **Step 6: Smoke test no dataset real**

```bash
go run ./cmd/advplc rpo apo \
  /tmp/opencode/advpls-test/mass/apo \
  --catalog /tmp/opencode/advpls-test/wire/catalog_sec2.txt \
  --out /tmp/opencode/apo-report
head -20 /tmp/opencode/apo-report/apo_report.json
```
Expected: `Relatório: … (3465 blobs)` e JSON com entradas (pode ter AVISO para blobs com framing inesperado — acceptable, reportar contagem no final).

- [ ] **Step 7: Commit**

```bash
git add cmd/advplc/cmd_rpo_apo.go cmd/advplc/cmd_rpo_apo_test.go cmd/advplc/cmd_rpo.go
git commit -m "FEAT — subcomando advplc rpo apo (relatório JSON/MD de desmontagem APO)"
```

---

### Task 5: Fix do bug de injeção duplicada (`cmdRpo`)

**Files:**
- Modify: `cmd/advplc/cmd_rpo.go:21-64` (switch) 
- Test: `cmd/advplc/cmd_rpo_test.go`

**Interfaces:**
- Consumes: estrutura do `switch` existente.
- Produces: `var rpoInjectFn = cmdRpoInject` (indirection para teste); `case "inject": return rpoInjectFn(args[1:])` — sem bloco pós-switch.

- [ ] **Step 1: Teste falhando**

Acrescentar em `cmd/advplc/cmd_rpo_test.go`:
```go
// TestCmdRpo_InjectExecutaUmaVez regresse o bug de 2026-10-04: o case
// "inject" não retornava e o bloco após o switch reexecutava cmdRpoInject
// (2 chamadas por invocação).
func TestCmdRpo_InjectExecutaUmaVez(t *testing.T) {
	orig := rpoInjectFn
	defer func() { rpoInjectFn = orig }()
	calls := 0
	rpoInjectFn = func([]string) error {
		calls++
		return nil
	}
	if err := cmdRpo([]string{"inject", "x.rpo", "cap.json"}); err != nil {
		t.Fatalf("cmdRpo: %v", err)
	}
	if calls != 1 {
		t.Fatalf("cmdRpoInject chamado %d vezes, esperado 1", calls)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./cmd/advplc/ -run TestCmdRpo_InjectExecutaUmaVez -v --timeout 300s`
Expected: FAIL (`undefined: rpoInjectFn`) — e, após criar a var sem o fix, falharia com `chamado 2 vezes`.

- [ ] **Step 3: Implementar o fix**

Em `cmd/advplc/cmd_rpo.go`:
1. Acima de `func cmdRpo`, acrescentar:
```go
// rpoInjectFn permite testar a contagem de chamadas do case "inject".
var rpoInjectFn = cmdRpoInject
```
2. Trocar o case:
```go
	case "inject":
		return rpoInjectFn(args[1:])
```
3. **Apagar** o bloco órfão pós-switch (linhas atuais 60–63):
```go
		if err := cmdRpoInject(args[1:]); err != nil {
			return err
		}
		return nil
```

- [ ] **Step 4: Rodar e confirmar pass**

Run: `go test ./cmd/advplc/ -run TestCmdRpo_InjectExecutaUmaVez -v --timeout 300s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/advplc/cmd_rpo.go cmd/advplc/cmd_rpo_test.go
git commit -m "FIX — cmdRpo inject executava duas vezes (bloco órfão pós-switch removido)"
```

---

### Task 6: Verificação final + relatório MD no dataset real + gap analysis

**Files:**
- Modify: `cmd/advplc/cmd_rpo_apo_test.go` (teste MD)
- No código novo além do teste.

**Interfaces:**
- Consumes: tudo das Tasks 1–5.

- [ ] **Step 1: Teste do formato MD (falha → pass)**

```go
func TestCmdRpoApo_FormatoMD(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "pkg", "rpo", "testdata", "apo", "EXTXDEF.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "EXTXDEF.PRW"), data, 0644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := cmdRpoApo([]string{"--format", "md", "--out", out, dir}); err != nil {
		t.Fatalf("cmdRpoApo: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "apo_report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "| EXTXDEF.PRW | AdvPL | 39 |") {
		t.Errorf("tabela MD não contém linha esperada:\n%s", raw)
	}
}
```
Run: `go test ./cmd/advplc/ -run TestCmdRpoApo_FormatoMD -v --timeout 300s` → FAIL se `strings` não importado/`apoRenderMD` com bug → corrigir → PASS.
(Lembrar: `import "strings"` no arquivo de teste.)

- [ ] **Step 2: Build + suíte completa**

```bash
go build ./... && go vet ./pkg/rpo/ ./cmd/advplc/ && \
go test ./pkg/rpo/... -count=1 && \
go test ./cmd/advplc/ -count=1 --timeout 300s
```
Expected: build ok, vet limpo, todos PASS.

- [ ] **Step 3: Smoke MD nos 3.465 blobs + coleta de números da gap analysis**

```bash
go run ./cmd/advplc rpo apo /tmp/opencode/advpls-test/mass/apo \
  --catalog /tmp/opencode/advpls-test/wire/catalog_sec2.txt \
  --format md --out /tmp/opencode/apo-report
grep -c '| AdvPL |' /tmp/opencode/apo-report/apo_report.md
grep -c '| TLPP |' /tmp/opencode/apo-report/apo_report.md
```
Expected: contagens ≈ 2073+35+4+1+5 (AdvPL) e 1347 (TLPP); registrar números reais para o relatório ao operador (gap analysis: blobs com framing inválido = avisos do stderr).

- [ ] **Step 4: Commit**

```bash
git add cmd/advplc/cmd_rpo_apo_test.go
git commit -m "TEST — cobertura do formato MD no advplc rpo apo"
```

- [ ] **Step 5: Reportar gap analysis ao operador**

Apresentar: total processado vs. 3.465, avisos de framing, contagem de snippets/candidatos do relatório MD — e só então propor escrita do Plano 2 (`advplc rpo pull`).
