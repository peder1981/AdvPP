# AdvPP NN Modules (S3c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development ou executing-plans. Steps usam checkbox (`- [ ]`).

**Goal:** Módulos `Linear`/`Embedding` (encapsulam params + forward) em `pkg/nn` e um native `Fit` (laço de treino via codeblock), para definir e treinar um modelo em poucas linhas de AdvPL.

**Architecture:** `pkg/nn` (Go) compõe ops do `pkg/autograd` (S3a/S3b, intocados). VM expõe `Linear`/`Embedding` como classes e `Fit` como native (reusa `callBlockSync`/closures do S1).

**Tech Stack:** Go puro, `pkg/tensor`+`pkg/autograd`, a VM AdvPP, fixtures AdvPL.

## Global Constraints

- Go puro. float32. NÃO modificar `pkg/tensor`/`pkg/autograd` (só usar).
- Índices AdvPL 1-based → 0-based interno (Embedding).
- Erros na VM → `advplrt.NewError` via `verr` (S3a). Reuse `wrapVariable`/`argVariable`/`getArg`.
- Novas classes builtin `Linear`/`Embedding`: registrar em `builtinClasses` (codegen), `OP_NEW_INSTANCE` e `callNativeMethod` (vm.go). `Fit` é native (natives.go).
- Rebuild: `go build -o advplc ./cmd/advplc`. Testes: `go test ./pkg/nn`; fixtures `./advplc run`.
- Regressão verde: `go test ./...`; S3a/S3b/S2 sem regressão (`train_demo`, `classifier_demo`, `mlp_demo`).

---

## Task 1: `pkg/nn` — módulos `Linear` e `Embedding`

**Files:** Create `pkg/nn/module.go`, `pkg/nn/module_test.go`.

**Interfaces:** Produces `type Linear struct { W, B *autograd.Variable }`; `NewLinear(nIn, nOut int, scale float32) *Linear`; `(*Linear).Forward(x *autograd.Variable) (*autograd.Variable, error)`; `(*Linear).Params() []*autograd.Variable`. `type Embedding struct { Table *autograd.Variable }`; `NewEmbedding(nVocab, nDim int, scale float32) *Embedding`; `(*Embedding).Forward(idx []int) (*autograd.Variable, error)`; `(*Embedding).Params() []*autograd.Variable`. Consumes `pkg/autograd` (`NewLeaf`, `Variable.MatMul/Add/IndexRows/Sum/Backward`), `pkg/tensor` (`Rand`, `New`).

- [ ] **Step 1: Write the failing test** — criar `pkg/nn/module_test.go`:

```go
package nn

import (
	"testing"

	"github.com/advpl/compiler/pkg/autograd"
	"github.com/advpl/compiler/pkg/tensor"
)

func leaf(data []float32, shape []int) *autograd.Variable {
	t, err := tensor.FromData(data, shape)
	if err != nil {
		panic(err)
	}
	return autograd.NewLeaf(t)
}

func TestLinearForwardAndParams(t *testing.T) {
	l := NewLinear(3, 2, 0.1)
	ps := l.Params()
	if len(ps) != 2 {
		t.Fatalf("Params len = %d, quer 2", len(ps))
	}
	x := leaf([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}) // [2,3]
	y, err := l.Forward(x)
	if err != nil {
		t.Fatal(err)
	}
	if !tensor.ShapeEq(y.Value.Shape, []int{2, 2}) {
		t.Fatalf("Forward shape = %v, quer [2 2]", y.Value.Shape)
	}
	y.Sum().Backward()
	if l.W.Grad == nil || l.B.Grad == nil {
		t.Fatal("Backward deve preencher grads de W e b")
	}
	if !tensor.ShapeEq(l.W.Grad.Shape, []int{3, 2}) {
		t.Fatalf("grad de W = %v, quer [3 2]", l.W.Grad.Shape)
	}
}

func TestEmbeddingForward(t *testing.T) {
	e := NewEmbedding(4, 3, 0.1)
	if len(e.Params()) != 1 {
		t.Fatal("Embedding deve ter 1 param (a tabela)")
	}
	y, err := e.Forward([]int{2, 0, 2}) // 0-based
	if err != nil {
		t.Fatal(err)
	}
	if !tensor.ShapeEq(y.Value.Shape, []int{3, 3}) {
		t.Fatalf("Embedding Forward shape = %v, quer [3 3]", y.Value.Shape)
	}
	y.Sum().Backward()
	if e.Table.Grad == nil {
		t.Fatal("Backward deve preencher grad da tabela")
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./pkg/nn` → FAIL (pacote não existe).

- [ ] **Step 3: Write the implementation** — criar `pkg/nn/module.go`:

```go
// Package nn fornece módulos de rede neural (camadas com parâmetros próprios e
// um Forward) sobre o autograd — a base para definir modelos de forma concisa.
package nn

import (
	"github.com/advpl/compiler/pkg/autograd"
	"github.com/advpl/compiler/pkg/tensor"
)

// Linear: camada densa y = x·W + b.
type Linear struct {
	W, B *autograd.Variable
}

func NewLinear(nIn, nOut int, scale float32) *Linear {
	return &Linear{
		W: autograd.NewLeaf(tensor.Rand([]int{nIn, nOut}, scale)),
		B: autograd.NewLeaf(tensor.New([]int{nOut})),
	}
}

func (l *Linear) Forward(x *autograd.Variable) (*autograd.Variable, error) {
	h, err := x.MatMul(l.W)
	if err != nil {
		return nil, err
	}
	return h.Add(l.B)
}

func (l *Linear) Params() []*autograd.Variable {
	return []*autograd.Variable{l.W, l.B}
}

// Embedding: tabela de embeddings; Forward colhe as linhas dos índices.
type Embedding struct {
	Table *autograd.Variable
}

func NewEmbedding(nVocab, nDim int, scale float32) *Embedding {
	return &Embedding{Table: autograd.NewLeaf(tensor.Rand([]int{nVocab, nDim}, scale))}
}

func (e *Embedding) Forward(idx []int) (*autograd.Variable, error) {
	return e.Table.IndexRows(idx)
}

func (e *Embedding) Params() []*autograd.Variable {
	return []*autograd.Variable{e.Table}
}
```

- [ ] **Step 4: Run test to verify it passes** — `go test ./pkg/nn` → PASS.

- [ ] **Step 5: Commit**
```bash
git add pkg/nn/module.go pkg/nn/module_test.go
git commit -m "nn: Linear and Embedding modules over autograd"
```

---

## Task 2: classes `Linear`/`Embedding` na VM + native `Fit`

**Files:** Modify `pkg/vm/autograd_native.go`, `pkg/compiler/codegen.go`, `pkg/vm/vm.go`, `pkg/vm/natives.go`; Test `tests/nn_test.prw`.

**Interfaces:** Consumes `pkg/nn` (Task 1); helpers `wrapVariable`/`argVariable`/`verr`/`getArg` (S3a). Produces classes AdvPL `Linear` (`New`/`Forward`/`Params`) e `Embedding` (`New`/`Forward`/`Params`); native `Fit(bPasso, nEpocas)`.

- [ ] **Step 1: Write the failing test** — criar `tests/nn_test.prw`:

```advpl
User Function NnTst()
    Local oLin := Linear():New(2, 3)
    Local oX := Variable():FromArray({1,2, 3,4}, {2,2})   // [2,2]
    Local oY := oLin:Forward(oX)                          // [2,3]
    Local aP := oLin:Params()
    Local oEmb := Embedding():New(4, 2)
    Local oPick := oEmb:Forward({3, 1})                   // linhas 3 e 1 (1-based)
    Local nCont := 0
    Local nFinal := 0
    Local nFail := 0

    If oY:Value():Shape()[2] != 3
        ConOut("FALHA Linear Forward"); nFail++
    EndIf
    If Len(aP) != 2
        ConOut("FALHA Linear Params"); nFail++
    EndIf
    If oPick:Value():Shape()[1] != 2 .Or. oPick:Value():Shape()[2] != 2
        ConOut("FALHA Embedding Forward"); nFail++
    EndIf
    // Fit avalia o codeblock N vezes e devolve o ultimo valor
    nFinal := Fit({|| ContaEsoma(@nCont) }, 5)
    If nCont != 5 .Or. nFinal != 5
        ConOut("FALHA Fit: cont=" + Str(nCont,1) + " final=" + Str(nFinal,3))
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 4/4 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return

Static Function ContaEsoma(nCont)
    nCont := nCont + 1
Return nCont
```

(O `@nCont` passa por referência; combinado com o closure, `Fit` incrementa 5x e a última avaliação devolve 5.)

- [ ] **Step 2: Run test to verify it fails** — `go build -o advplc ./cmd/advplc && ./advplc run tests/nn_test.prw` → FAIL (`Linear` não reconhecida).

- [ ] **Step 3: Registrar as classes (3 pontos)** — em `pkg/compiler/codegen.go`, `builtinClasses`:
```go
	"LINEAR":        true,
	"EMBEDDING":     true,
```
Em `pkg/vm/vm.go`, `OP_NEW_INSTANCE` (junto de `case "ADAM":`):
```go
		case "LINEAR":
			v.push(newLinearObject())
			return nil
		case "EMBEDDING":
			v.push(newEmbeddingObject())
			return nil
```
Em `pkg/vm/vm.go`, `callNativeMethod` (junto de `case "Adam":`):
```go
	case "Linear":
		return v.callLinearMethod(obj, upperMethod, args)
	case "Embedding":
		return v.callEmbeddingMethod(obj, upperMethod, args)
```

- [ ] **Step 4: Adicionar as classes e o `Fit`** — em `pkg/vm/autograd_native.go` (fim do arquivo):

```go
import do topo já tem "github.com/advpl/compiler/pkg/nn"? Se não, adicione:
	"github.com/advpl/compiler/pkg/nn"
```

```go
type linearState struct{ m *nn.Linear }

func newLinearObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Linear", nil)
	obj.Native = &linearState{}
	return obj
}

func optScale(args []advplrt.Value, i int) float32 {
	if num, ok := getArg(args, i).(*advplrt.NumberValue); ok {
		return float32(num.Val)
	}
	return 0.1
}

func (v *VM) callLinearMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*linearState)
	switch method {
	case "NEW":
		nIn := int(advplrt.ToFloat(getArg(args, 0)))
		nOut := int(advplrt.ToFloat(getArg(args, 1)))
		if nIn <= 0 || nOut <= 0 {
			return advplrt.NewError("Linear:New: dimensões devem ser > 0")
		}
		st.m = nn.NewLinear(nIn, nOut, optScale(args, 2))
		v.push(obj)
	case "FORWARD":
		x, err := argVariable(args, 0)
		if err != nil {
			return err
		}
		r, err := st.m.Forward(x)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "PARAMS":
		v.push(paramsArray(st.m.Params()))
	default:
		return advplrt.NewError("Linear: método desconhecido " + method)
	}
	return nil
}

type embeddingState struct{ m *nn.Embedding }

func newEmbeddingObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Embedding", nil)
	obj.Native = &embeddingState{}
	return obj
}

func (v *VM) callEmbeddingMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*embeddingState)
	switch method {
	case "NEW":
		nVocab := int(advplrt.ToFloat(getArg(args, 0)))
		nDim := int(advplrt.ToFloat(getArg(args, 1)))
		if nVocab <= 0 || nDim <= 0 {
			return advplrt.NewError("Embedding:New: dimensões devem ser > 0")
		}
		st.m = nn.NewEmbedding(nVocab, nDim, optScale(args, 2))
		v.push(obj)
	case "FORWARD":
		idx := shapeFromArg(getArg(args, 0))
		for i := range idx {
			idx[i]--
		}
		r, err := st.m.Forward(idx)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "PARAMS":
		v.push(paramsArray(st.m.Params()))
	default:
		return advplrt.NewError("Embedding: método desconhecido " + method)
	}
	return nil
}

func paramsArray(ps []*autograd.Variable) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(ps))
	for i, p := range ps {
		el[i] = wrapVariable(p)
	}
	return advplrt.NewArray(el)
}
```

Em `pkg/vm/natives.go`, no mapa `natives` (ex.: após `"GETNAMES"`), adicionar o `Fit`:

```go
		// Fit(bPasso, nEpocas): avalia o codeblock bPasso nEpocas vezes e devolve
		// o valor da última avaliação (a loss final). Laço de treino conciso.
		"FIT": func(args []advplrt.Value) (advplrt.Value, error) {
			cb := getArg(args, 0)
			nEpocas := int(advplrt.ToFloat(getArg(args, 1)))
			var last advplrt.Value = advplrt.Nil
			for i := 0; i < nEpocas; i++ {
				r, err := v.callBlockSync(cb)
				if err != nil {
					return advplrt.Nil, err
				}
				last = r
			}
			return last, nil
		},
```

- [ ] **Step 5: Rebuild e rodar o fixture** — `go build -o advplc ./cmd/advplc && ./advplc run tests/nn_test.prw` → `OK: 4/4 verificacoes passaram.`

- [ ] **Step 6: Regressão** — `go test ./... 2>&1 | grep -vE '^ok|no test files'` (vazio); `./advplc run tests/classifier_demo.prw 2>&1 | tail -1` (OK).

- [ ] **Step 7: Commit**
```bash
git add pkg/nn/ pkg/vm/autograd_native.go pkg/vm/vm.go pkg/vm/natives.go pkg/compiler/codegen.go tests/nn_test.prw
git commit -m "vm: Linear/Embedding module classes + Fit training loop"
```

---

## Task 3: aceite (definir e treinar um MLP com módulos) + docs

**Files:** Create `tests/nn_demo.prw`; Modify `README.md`, `CHANGELOG.md`.

**Interfaces:** Consumes `Linear`, `Adam`, `Fit`, `Variable` (SoftmaxCE/Tanh/Argmax).

- [ ] **Step 1: Escrever a demo** — criar `tests/nn_demo.prw`:

```advpl
// Define um MLP 2-8-2 com dois módulos Linear + Tanh, e o treina com Adam via Fit,
// classificando 4 pontos (XOR-como-classes). Verifica loss caindo e acuracia 100%.
User Function NnDemo()
    Local oL1 := Linear():New(2, 8)
    Local oL2 := Linear():New(8, 2)
    Local oX  := Variable():FromArray({0,0, 0,1, 1,0, 1,1}, {4,2})
    Local aAlvo := {1, 2, 2, 1}
    Local aParams := Concat2(oL1:Params(), oL2:Params())
    Local oOpt := Adam():New(aParams, 0.05)
    Local nInicial := LossOnly(oL1, oL2, oX, aAlvo)
    Local nFinal := 0

    // Treina: cada passo faz forward + zerograd + backward + step, devolve a loss
    nFinal := Fit({|| Passo(oL1, oL2, oX, aAlvo, oOpt) }, 600)

    // Acuracia final
    Local oLog := oL2:Forward(oL1:Forward(oX):Tanh())
    Local aPred := oLog:Value():Argmax(2):ToArray()   // 1-based
    Local nOk := 0
    Local i := 0
    For i := 1 To 4
        If aPred[i] == aAlvo[i]
            nOk++
        EndIf
    Next i

    ConOut("loss " + Str(nInicial,9,5) + " -> " + Str(nFinal,9,5) + " | acuracia " + Str(nOk,1) + "/4")
    If nFinal < nInicial * 0.5 .And. nOk == 4
        ConOut("OK: modelo com modulos treinou (Linear + Adam + Fit).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente ou acuracia < 4/4")
    EndIf
Return

Static Function Passo(oL1, oL2, oX, aAlvo, oOpt)
    Local oLog := oL2:Forward(oL1:Forward(oX):Tanh())
    Local oLoss := oLog:SoftmaxCE(aAlvo)
    oOpt:ZeroGrad()
    oLoss:Backward()
    oOpt:Step()
Return oLoss:Value():ToArray()[1]

Static Function LossOnly(oL1, oL2, oX, aAlvo)
    Local oLog := oL2:Forward(oL1:Forward(oX):Tanh())
Return oLog:SoftmaxCE(aAlvo):Value():ToArray()[1]

Static Function Concat2(a, b)
    Local r := {}
    Local i := 0
    For i := 1 To Len(a)
        aAdd(r, a[i])
    Next i
    For i := 1 To Len(b)
        aAdd(r, b[i])
    Next i
Return r
```

- [ ] **Step 2: Rodar a demo** — `./advplc run tests/nn_demo.prw` → loss cai e:
```
OK: modelo com modulos treinou (Linear + Adam + Fit).
```
Se falhar (loss não cai OU acurácia < 4/4), PARE e reporte BLOCKED com a saída — indica bug nos módulos/Fit, não algo a mascarar. (Init aleatório: se um seed raro não convergir em 600 épocas, reporte a trajetória — não altere o modelo para mascarar.)

- [ ] **Step 3: README** — em `README.md`, na seção "## Autodiff e treino", adicionar ao final:
```markdown

Módulos e trainer: `Linear():New(nIn, nOut)` e `Embedding():New(nVocab, nDim)`
encapsulam parâmetros + `Forward`; `oMod:Params()` devolve os pesos para o
otimizador; `Fit(bPasso, nEpocas)` roda o laço de treino avaliando um codeblock por
época. Assim dá para definir e treinar um modelo em poucas linhas — ver
`tests/nn_demo.prw`.
```

- [ ] **Step 4: CHANGELOG** — em `CHANGELOG.md`, na seção `## [Não lançado]`, adicionar ao final:
```markdown

### Módulos e trainer (Sub-projeto 3c)

- Módulos de rede neural (`pkg/nn`): **`Linear`** (W,b) e **`Embedding`** (tabela),
  cada um encapsulando seus parâmetros + `Forward` e expondo `Params()` para o
  otimizador. Native **`Fit(bPasso, nEpocas)`** roda o laço de treino avaliando um
  codeblock por época (usando closures). Classes AdvPL `Linear`/`Embedding`. Aceite:
  `tests/nn_demo.prw` define e treina um MLP com módulos + Adam em poucas linhas
  (acurácia 100%). Fecha o "Full autodiff/treino" (S3a motor → S3b LM loss/Adam →
  S3c módulos).
```

- [ ] **Step 5: Commit**
```bash
git add tests/nn_demo.prw README.md CHANGELOG.md
git commit -m "nn: module-based MLP training demo + docs"
```

---

## Verificação final
- `go test ./...` verde (inclui `pkg/nn`); `tests/nn_test.prw` e `tests/nn_demo.prw` OK.
- S3a/S3b/S2 sem regressão. Os 5 critérios de aceite da spec satisfeitos.
