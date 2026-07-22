# AdvPP LM Loss + Adam (S3b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`).

**Goal:** Adicionar ao autograd a loss de classificação (softmax + cross-entropy), o otimizador Adam, o backward de embedding (IndexRows) e das ativações Tanh/Sigmoid/Gelu, permitindo treinar um classificador float.

**Architecture:** Estende `pkg/autograd` (S3a) com ops novas + `Adam`, ligadas à VM no padrão de `Variable`/`SGD`. Reusa kernels do `pkg/tensor` (Softmax, IndexRows, Tanh, Sigmoid, Gelu) no forward; backwards em Go.

**Tech Stack:** Go puro, `pkg/tensor` (S2), `pkg/autograd` (S3a), a VM AdvPP, fixtures AdvPL.

## Global Constraints

- Go puro, sem CGO. float32. NÃO modificar `pkg/tensor`.
- Ops funcionais (devolvem `Variable` nova); Adam muta params in-place.
- Índices no AdvPL 1-based; internamente 0-based (SoftmaxCE alvo e IndexRows idx).
- Erros na VM → `advplrt.NewError` via `verr` (helper do S3a em `pkg/vm/autograd_native.go`).
- Nova classe builtin `Adam`: registrar em `builtinClasses` (codegen), `OP_NEW_INSTANCE` (vm.go), `callNativeMethod` (vm.go).
- Rebuild: `go build -o advplc ./cmd/advplc`. Testes: `go test ./pkg/autograd`; fixtures `./advplc run`.
- Regressão verde: `go test ./...`; S3a/S2 sem regressão (`train_demo`, `autograd_test`, `mlp_demo`, `tensor_test`).

---

## Task 1: ativações diferenciáveis (Tanh, Sigmoid, Gelu)

**Files:** Modify `pkg/autograd/ops.go`, `pkg/autograd/autograd_test.go`.

**Interfaces:** Produces `(*Variable).Tanh() *Variable`; `Sigmoid() *Variable`; `Gelu() *Variable`. Consumes `pkg/tensor` (`Tanh`/`Sigmoid`/`Gelu`), `pkg/autograd` (`addGrad`, `Mul`), gradCheck (Task 2 do S3a, já em autograd_test.go).

- [ ] **Step 1: Write the failing test** — adicionar a `pkg/autograd/autograd_test.go`:

```go
func TestGradActivations(t *testing.T) {
	gradCheck(t, "tanh", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Tanh().Sum()
	})
	gradCheck(t, "sigmoid", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Sigmoid().Sum()
	})
	gradCheck(t, "gelu", mustT([]float32{-1, 0.5, 2, -0.3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Gelu().Sum()
	})
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./pkg/autograd` → FAIL (`Tanh` etc. indefinidos).

- [ ] **Step 3: Write the implementation** — adicionar a `pkg/autograd/ops.go`:

```go
// Tanh. dA = dY ⊙ (1 - tanh(A)²).
func (a *Variable) Tanh() *Variable {
	y := a.Value.Tanh()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		d := tensor.New(y.Shape)
		for i, yv := range y.Data {
			d.Data[i] = 1 - yv*yv
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}

// Sigmoid. dA = dY ⊙ σ(1-σ).
func (a *Variable) Sigmoid() *Variable {
	y := a.Value.Sigmoid()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		d := tensor.New(y.Shape)
		for i, yv := range y.Data {
			d.Data[i] = yv * (1 - yv)
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}

// Gelu (aproximação tanh). dA = dY ⊙ gelu'(A).
func (a *Variable) Gelu() *Variable {
	y := a.Value.Gelu()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		const c = 0.7978845608
		d := tensor.New(a.Value.Shape)
		for i, xv := range a.Value.Data {
			x := float64(xv)
			u := c * (x + 0.044715*x*x*x)
			tv := math.Tanh(u)
			dg := 0.5*(1+tv) + 0.5*x*(1-tv*tv)*c*(1+3*0.044715*x*x)
			d.Data[i] = float32(dg)
		}
		dg, _ := out.Grad.Mul(d)
		addGrad(a, dg)
	}
	return out
}
```
(`math` já é importado em `ops.go`.)

- [ ] **Step 4: Run test to verify it passes** — `go test ./pkg/autograd` → PASS.

- [ ] **Step 5: Commit**
```bash
git add pkg/autograd/ops.go pkg/autograd/autograd_test.go
git commit -m "autograd: Tanh, Sigmoid, Gelu differentiable ops"
```

---

## Task 2: `IndexRows` (embedding) + `SoftmaxCE` diferenciáveis

**Files:** Modify `pkg/autograd/ops.go`, `pkg/autograd/autograd_test.go`.

**Interfaces:** Produces `(*Variable).IndexRows(idx []int) (*Variable, error)`; `(*Variable).SoftmaxCE(targets []int) (*Variable, error)`. `idx`/`targets` são 0-based. Consumes `pkg/tensor` (`IndexRows`, `Softmax`), `math`.

- [ ] **Step 1: Write the failing test** — adicionar a `pkg/autograd/autograd_test.go`:

```go
func TestGradIndexRows(t *testing.T) {
	// tabela [3,2]; colhe linhas [2,0,2]; grad w.r.t. a tabela (scatter-add)
	gradCheck(t, "indexrows", mustT([]float32{1, 2, 3, 4, 5, 6}, []int{3, 2}), func(x *Variable) *Variable {
		y, err := x.IndexRows([]int{2, 0, 2})
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
}

func TestGradSoftmaxCE(t *testing.T) {
	// logits [2,3]; alvos [0,2]; grad w.r.t. logits
	gradCheck(t, "softmaxce", mustT([]float32{2, 1, 0.1, 0.3, 0.2, 3}, []int{2, 3}), func(x *Variable) *Variable {
		l, err := x.SoftmaxCE([]int{0, 2})
		if err != nil {
			panic(err)
		}
		return l
	})
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./pkg/autograd` → FAIL.

- [ ] **Step 3: Write the implementation** — adicionar a `pkg/autograd/ops.go`:

```go
// IndexRows: colhe linhas de A[R,C] nos índices idx (0-based) -> [K,C].
// backward (scatter-add): dA[idx[k],:] += dY[k,:].
func (a *Variable) IndexRows(idx []int) (*Variable, error) {
	y, err := a.Value.IndexRows(idx)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		c := a.Value.Shape[1]
		dA := tensor.New(a.Value.Shape)
		for k, r := range idx {
			for j := 0; j < c; j++ {
				dA.Data[r*c+j] += out.Grad.Data[k*c+j]
			}
		}
		addGrad(a, dA)
	}
	return out, nil
}

// SoftmaxCE: A = logits [N,C]; targets = N índices de classe (0-based).
// loss = média_i(-log softmax(A)[i, t_i]) (estável); dA = (softmax − onehot)/N.
func (a *Variable) SoftmaxCE(targets []int) (*Variable, error) {
	if len(a.Value.Shape) != 2 {
		return nil, fmt.Errorf("SoftmaxCE: logits devem ser 2D [N,C]")
	}
	n, c := a.Value.Shape[0], a.Value.Shape[1]
	if len(targets) != n {
		return nil, fmt.Errorf("SoftmaxCE: %d alvos para %d linhas", len(targets), n)
	}
	sm, err := a.Value.Softmax(1)
	if err != nil {
		return nil, err
	}
	var loss float32
	for i := 0; i < n; i++ {
		ti := targets[i]
		if ti < 0 || ti >= c {
			return nil, fmt.Errorf("SoftmaxCE: alvo %d fora de faixa (0..%d)", ti, c-1)
		}
		loss += -float32(math.Log(float64(sm.Data[i*c+ti]) + 1e-12))
	}
	loss /= float32(n)
	y, _ := tensor.FromData([]float32{loss}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0] / float32(n)
		dA := tensor.New(a.Value.Shape)
		for i := 0; i < n; i++ {
			for j := 0; j < c; j++ {
				val := sm.Data[i*c+j]
				if j == targets[i] {
					val -= 1
				}
				dA.Data[i*c+j] = g * val
			}
		}
		addGrad(a, dA)
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes** — `go test ./pkg/autograd` → PASS.

- [ ] **Step 5: Commit**
```bash
git add pkg/autograd/ops.go pkg/autograd/autograd_test.go
git commit -m "autograd: IndexRows (embedding) and SoftmaxCE differentiable ops"
```

---

## Task 3: otimizador Adam

**Files:** Create `pkg/autograd/adam.go`; Modify `pkg/autograd/autograd_test.go`.

**Interfaces:** Produces `type Adam struct{...}`; `NewAdam(params []*Variable, lr float32) *Adam`; `(*Adam).Step()`; `(*Adam).ZeroGrad()`. Consumes `Variable`, `pkg/tensor` (`New`), ops `Mul`/`Sum` (para o teste).

- [ ] **Step 1: Write the failing test** — adicionar a `pkg/autograd/autograd_test.go`:

```go
func TestAdamReducesLoss(t *testing.T) {
	p := NewLeaf(mustT([]float32{3, -4}, []int{2}))
	lossOf := func() float32 {
		sq, _ := p.Value.Mul(p.Value)
		return sq.SumAll()
	}
	before := lossOf()
	opt := NewAdam([]*Variable{p}, 0.1)
	for i := 0; i < 10; i++ {
		opt.ZeroGrad()
		l, _ := p.Mul(p)
		l.Sum().Backward()
		opt.Step()
	}
	after := lossOf()
	if !(after < before) {
		t.Fatalf("Adam nao reduziu a loss: antes=%v depois=%v", before, after)
	}
	if opt.t != 10 {
		t.Fatalf("t esperado 10, veio %d", opt.t)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./pkg/autograd` → FAIL (`NewAdam` indefinido).

- [ ] **Step 3: Write the implementation** — criar `pkg/autograd/adam.go`:

```go
package autograd

import (
	"math"

	"github.com/advpl/compiler/pkg/tensor"
)

// Adam (Kingma & Ba 2014) com correção de viés. m/v por parâmetro.
type Adam struct {
	params          []*Variable
	lr, b1, b2, eps float32
	t               int
	m, v            []*tensor.Tensor
}

func NewAdam(params []*Variable, lr float32) *Adam {
	m := make([]*tensor.Tensor, len(params))
	v := make([]*tensor.Tensor, len(params))
	for i, p := range params {
		m[i] = tensor.New(p.Value.Shape)
		v[i] = tensor.New(p.Value.Shape)
	}
	return &Adam{params: params, lr: lr, b1: 0.9, b2: 0.999, eps: 1e-8, m: m, v: v}
}

func (o *Adam) Step() {
	o.t++
	bc1 := 1 - float32(math.Pow(float64(o.b1), float64(o.t)))
	bc2 := 1 - float32(math.Pow(float64(o.b2), float64(o.t)))
	for i, p := range o.params {
		if p.Grad == nil {
			continue
		}
		g := p.Grad.Data
		md := o.m[i].Data
		vd := o.v[i].Data
		pd := p.Value.Data
		for j := range pd {
			md[j] = o.b1*md[j] + (1-o.b1)*g[j]
			vd[j] = o.b2*vd[j] + (1-o.b2)*g[j]*g[j]
			mhat := md[j] / bc1
			vhat := vd[j] / bc2
			pd[j] -= o.lr * mhat / (float32(math.Sqrt(float64(vhat))) + o.eps)
		}
	}
}

func (o *Adam) ZeroGrad() {
	for _, p := range o.params {
		p.Grad = nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes** — `go test ./pkg/autograd` → PASS.

- [ ] **Step 5: Commit**
```bash
git add pkg/autograd/adam.go pkg/autograd/autograd_test.go
git commit -m "autograd: Adam optimizer with bias correction"
```

---

## Task 4: ligação na VM (métodos novos de `Variable` + classe `Adam`) + fixture

**Files:** Modify `pkg/vm/autograd_native.go`, `pkg/compiler/codegen.go`, `pkg/vm/vm.go`; Test `tests/lmloss_test.prw`.

**Interfaces:** Consumes `pkg/autograd` (Tasks 1-3); helpers `wrapVariable`/`argVariable`/`verr`/`shapeFromArg`/`getArg` (S3a/S2). Produces métodos `Tanh`/`Sigmoid`/`Gelu`/`IndexRows`/`SoftmaxCE` em `Variable`; classe `Adam` (`New`/`Step`/`ZeroGrad`).

- [ ] **Step 1: Write the failing test** — criar `tests/lmloss_test.prw`:

```advpl
User Function LmLossTst()
    Local oLog := Variable():FromArray({2,1,0.1, 0.3,0.2,3}, {2,3})   // logits [2,3]
    Local oL := oLog:SoftmaxCE({1, 3})   // alvos classe 1 e 3 (1-based)
    Local oEmb := Variable():FromArray({1,2, 3,4, 5,6}, {3,2})        // tabela [3,2]
    Local oPick := oEmb:IndexRows({3, 1})                             // linhas 3 e 1
    Local oH := Variable():FromArray({-1,0.5, 2,-0.3}, {2,2})
    Local nFail := 0

    oL:Backward()
    If oLog:Grad():Size() != 6
        ConOut("FALHA grad softmaxce"); nFail++
    EndIf
    If oPick:Value():Shape()[1] != 2 .Or. oPick:Value():Shape()[2] != 2
        ConOut("FALHA forma IndexRows"); nFail++
    EndIf
    If oL:Value():ToArray()[1] < 0
        ConOut("FALHA loss negativa"); nFail++
    EndIf
    // Adam roda sem erro
    Begin Sequence
        Local oOpt := Adam():New({oEmb}, 0.01)
        oEmb:IndexRows({1,2,3}):Sum():Backward()
        oOpt:Step()
    Recover
        ConOut("FALHA: Adam lancou erro"); nFail++
    End Sequence

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
```

- [ ] **Step 2: Run test to verify it fails** — `go build -o advplc ./cmd/advplc && ./advplc run tests/lmloss_test.prw` → FAIL (`SoftmaxCE`/`Adam` não reconhecidos).

- [ ] **Step 3: Registrar `Adam` (3 pontos)** — em `pkg/compiler/codegen.go`, `builtinClasses`, adicionar:
```go
	"ADAM":          true,
```
Em `pkg/vm/vm.go`, `OP_NEW_INSTANCE` (junto de `case "SGD":`):
```go
		case "ADAM":
			v.push(newAdamObject())
			return nil
```
Em `pkg/vm/vm.go`, `callNativeMethod` (junto de `case "SGD":`):
```go
	case "Adam":
		return v.callAdamMethod(obj, upperMethod, args)
```

- [ ] **Step 4: Adicionar os métodos novos e a classe Adam** — em `pkg/vm/autograd_native.go`:

(a) No `switch method` de `callVariableMethod`, adicionar antes do `default`:
```go
	case "TANH":
		v.push(wrapVariable(self.Tanh()))
	case "SIGMOID":
		v.push(wrapVariable(self.Sigmoid()))
	case "GELU":
		v.push(wrapVariable(self.Gelu()))
	case "INDEXROWS":
		idx := shapeFromArg(getArg(args, 0))
		for i := range idx {
			idx[i]--
		}
		r, err := self.IndexRows(idx)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "SOFTMAXCE":
		tg := shapeFromArg(getArg(args, 0))
		for i := range tg {
			tg[i]--
		}
		r, err := self.SoftmaxCE(tg)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
```

(b) No fim do arquivo, adicionar a classe Adam:
```go
type adamState struct{ opt *autograd.Adam }

func newAdamObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Adam", nil)
	obj.Native = &adamState{}
	return obj
}

func (v *VM) callAdamMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*adamState)
	switch method {
	case "NEW":
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewError("Adam:New requer um array de Variables")
		}
		params := make([]*autograd.Variable, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			o, ok := e.(*advplrt.ObjectValue)
			if !ok {
				return advplrt.NewError("Adam:New: elemento não é Variable")
			}
			vv, ok := o.Native.(*autograd.Variable)
			if !ok {
				return advplrt.NewError("Adam:New: elemento não é Variable")
			}
			params = append(params, vv)
		}
		st.opt = autograd.NewAdam(params, float32(advplrt.ToFloat(getArg(args, 1))))
		v.push(obj)
	case "STEP":
		if st.opt != nil {
			st.opt.Step()
		}
		v.push(obj)
	case "ZEROGRAD":
		if st.opt != nil {
			st.opt.ZeroGrad()
		}
		v.push(obj)
	default:
		return advplrt.NewError("Adam: método desconhecido " + method)
	}
	return nil
}
```

- [ ] **Step 5: Rebuild e rodar o fixture** — `go build -o advplc ./cmd/advplc && ./advplc run tests/lmloss_test.prw` → `OK: 3/3 verificacoes passaram.`

- [ ] **Step 6: Regressão** — `go test ./... 2>&1 | grep -vE '^ok|no test files'` (vazio); `./advplc run tests/train_demo.prw 2>&1 | tail -1` (OK).

- [ ] **Step 7: Commit**
```bash
git add pkg/autograd/ pkg/vm/autograd_native.go pkg/vm/vm.go pkg/compiler/codegen.go tests/lmloss_test.prw
git commit -m "vm: SoftmaxCE/IndexRows/activations methods + Adam class"
```

---

## Task 5: aceite (classificador com CE+Adam) + docs

**Files:** Create `tests/classifier_demo.prw`; Modify `README.md`, `CHANGELOG.md`.

**Interfaces:** Consumes `Variable` (SoftmaxCE/Tanh/MatMul/Add/Argmax via Value) e `Adam` (Task 4).

- [ ] **Step 1: Escrever a demo** — criar `tests/classifier_demo.prw`:

```advpl
// Classificador: 4 pontos 2D, 2 classes linearmente separaveis. MLP 2-8-2 com Tanh,
// loss SoftmaxCE, otimizador Adam. Verifica loss caindo e acuracia 100%.
User Function ClassifierDemo()
    Local oX := Variable():FromArray({0,0, 0,1, 1,0, 1,1}, {4,2})
    Local aAlvo := {1, 2, 2, 1}   // classe 1-based (XOR-como-classes: 0/1)
    Local oW1 := Variable():FromArray({0.3,-0.2,0.1,0.4,-0.3,0.2,0.5,-0.1, -0.4,0.3,0.2,-0.5,0.1,0.4,-0.2,0.3}, {2,8})
    Local ob1 := Variable():FromArray({0,0,0,0,0,0,0,0}, {8})
    Local oW2 := Variable():FromArray({0.2,-0.3, 0.4,0.1, -0.2,0.5, 0.3,-0.4, 0.1,0.2, -0.5,0.3, 0.4,-0.1, 0.2,0.3}, {8,2})
    Local ob2 := Variable():FromArray({0,0}, {2})
    Local oOpt := Adam():New({oW1, ob1, oW2, ob2}, 0.05)
    Local nEpoca := 0, oH, oLog, oLoss
    Local nInicial := 0, nAtual := 0

    For nEpoca := 1 To 500
        oH   := oX:MatMul(oW1):Add(ob1):Tanh()
        oLog := oH:MatMul(oW2):Add(ob2)        // logits [4,2]
        oLoss := oLog:SoftmaxCE(aAlvo)
        nAtual := oLoss:Value():ToArray()[1]
        If nEpoca == 1
            nInicial := nAtual
        EndIf
        If nEpoca == 1 .Or. Mod(nEpoca,100) == 0
            ConOut("epoca " + Str(nEpoca,4) + " loss " + Str(nAtual,9,5))
        EndIf
        oOpt:ZeroGrad()
        oLoss:Backward()
        oOpt:Step()
    Next nEpoca

    // acuracia: argmax por linha dos logits vs alvo
    Local aPred := oLog:Value():Argmax(2):ToArray()   // Argmax por eixo 2 -> [4], 1-based
    Local nOk := 0, i := 0
    For i := 1 To 4
        If aPred[i] == aAlvo[i]
            nOk++
        EndIf
    Next i
    ConOut("loss " + Str(nInicial,9,5) + " -> " + Str(nAtual,9,5) + " | acuracia " + Str(nOk,1) + "/4")
    If nAtual < nInicial * 0.5 .And. nOk == 4
        ConOut("OK: classificador treinou (softmax-CE + Adam funcionam).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente ou acuracia < 4/4")
    EndIf
Return
```

- [ ] **Step 2: Rodar a demo** — `./advplc run tests/classifier_demo.prw` → loss cai e:
```
OK: classificador treinou (softmax-CE + Adam funcionam).
```
Se falhar (loss não cai ou acurácia < 4/4), PARE e reporte BLOCKED com a trajetória — indica bug de backward/otimizador, não algo a mascarar.

- [ ] **Step 3: README** — em `README.md`, na seção "## Autodiff e treino", adicionar ao final:
```markdown

Loss de classificação e otimizador robusto: `oLoss := oLogits:SoftmaxCE(aAlvo)`
(softmax + cross-entropy, alvo por índices de classe); `Adam():New(aParams, nLR)`
(`Step`/`ZeroGrad`). Ativações diferenciáveis `Tanh`/`Sigmoid`/`Gelu` e `IndexRows`
(embedding, com backward scatter-add). Ver `tests/classifier_demo.prw`.
```

- [ ] **Step 4: CHANGELOG** — em `CHANGELOG.md`, na seção `## [Não lançado]`, adicionar ao final:
```markdown

### Loss de LM + Adam (Sub-projeto 3b)

- Autograd ganhou a loss de classificação **`SoftmaxCE`** (softmax + cross-entropy
  fundida, estável; gradiente `softmax − onehot`), o otimizador **Adam** (com
  correção de viés), o backward de **embedding** (`IndexRows` diferenciável,
  scatter-add) e das ativações **Tanh/Sigmoid/Gelu**. Classes AdvPL: métodos novos
  em `Variable` + classe `Adam`. Aceite: `tests/classifier_demo.prw` treina um
  classificador (loss cai, acurácia 100%). Módulos e trainer ficam para o S3c.
```

- [ ] **Step 5: Commit**
```bash
git add tests/classifier_demo.prw README.md CHANGELOG.md
git commit -m "autograd: classifier training demo (softmax-CE + Adam) + docs"
```

---

## Verificação final
- `go test ./...` verde; `tests/lmloss_test.prw` e `tests/classifier_demo.prw` OK.
- S3a/S2 sem regressão. Os 5 critérios de aceite da spec satisfeitos.
