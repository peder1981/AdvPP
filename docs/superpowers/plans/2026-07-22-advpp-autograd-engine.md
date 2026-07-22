# AdvPP Autograd Engine (S3a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um motor de diferenciação reversa (`pkg/autograd`) com `Variable`/tape/`Backward`, 7 ops diferenciáveis e um otimizador SGD, exposto ao AdvPL como as classes `Variable` e `SGD`, permitindo TREINAR um modelo float com a loss caindo.

**Architecture:** Reverse-mode define-by-run: cada op cria uma `Variable` que grava um closure de backward e seus pais; `Backward()` percorre o grafo em ordem topológica reversa acumulando gradientes. Reusa 100% os kernels do `pkg/tensor` (S2, intocado) para forward e backward. Ligado à VM no mesmo padrão da classe `Tensor`.

**Tech Stack:** Go puro (sem CGO), `pkg/tensor` (S2), a VM/compilador AdvPP, fixtures AdvPL.

## Global Constraints

- Go puro, sem CGO. dtype **float32**. NÃO modificar `pkg/tensor` (S2 congelado) — só usar.
- `Variable` acumula grad (sem `requires_grad`); o otimizador atualiza só os params que recebe.
- `Backward()` é chamado numa `Variable` **escalar** (loss, forma `{1}`); semeia grad 1.
- Erros na camada da VM → `advplrt.NewError` (catchável), via helper `terr` já existente em `pkg/vm/tensor_native.go`. Nunca `fmt.Errorf` na VM.
- Registrar classe builtin nova em TRÊS lugares: `builtinClasses` (`pkg/compiler/codegen.go`), switch `OP_NEW_INSTANCE` (`pkg/vm/vm.go`), `callNativeMethod` (`pkg/vm/vm.go`).
- Rebuild após mudança Go da VM/compilador: `go build -o advplc ./cmd/advplc`.
- Kernels/autograd testados com `go test ./pkg/autograd`; classes testadas com `./advplc run tests/*.prw`.
- Regressão obrigatória verde: `go test ./...`; fixtures e exemplos do S2 (`mlp_demo.prw`, `tensor_test.prw`) sem regressão.
- Ops funcionais (devolvem `Variable` nova), exceto `SGD.Step`/`ZeroGrad` (mutam params in-place).

---

## File Structure

- `pkg/autograd/variable.go` — `Variable`, `NewLeaf`, `addGrad`, `onesLike`, `reduceGradTo`, `Backward`.
- `pkg/autograd/ops.go` — as 7 ops: `MatMul`, `Add`, `Mul`, `Relu`, `Sum`, `Mean`, `MSE`.
- `pkg/autograd/sgd.go` — `SGD`, `NewSGD`, `Step`, `ZeroGrad`.
- `pkg/autograd/autograd_test.go` — grad-check por diferenças finitas + tape + SGD.
- `pkg/vm/autograd_native.go` — classes `Variable`/`SGD` na VM + ponte.
- `pkg/compiler/codegen.go` — `"VARIABLE"`, `"SGD"` em `builtinClasses`.
- `pkg/vm/vm.go` — `OP_NEW_INSTANCE` + `callNativeMethod` para as duas classes.
- `tests/autograd_test.prw` (fixture da API) e `tests/train_demo.prw` (aceite).

---

## Task 1: `pkg/autograd` — `Variable`, tape, `Backward`, `reduceGradTo`

**Files:**
- Create: `pkg/autograd/variable.go`
- Test: `pkg/autograd/autograd_test.go`

**Interfaces:**
- Consumes: `pkg/tensor` (`Tensor`, `New`, `FromData`, `ShapeEq`, `Prod`, `SumAll`, `SumAxis`, `Reshape`, `Add`, `MulScalar`).
- Produces: `type Variable struct { Value, Grad *tensor.Tensor; parents []*Variable; backward func() }`; `NewLeaf(v *tensor.Tensor) *Variable`; `addGrad(v *Variable, g *tensor.Tensor)`; `onesLike(t *tensor.Tensor) *tensor.Tensor`; `reduceGradTo(g *tensor.Tensor, shape []int) *tensor.Tensor`; `(*Variable).Backward()`.

- [ ] **Step 1: Write the failing test**

Criar `pkg/autograd/autograd_test.go`:

```go
package autograd

import (
	"testing"

	"github.com/advpl/compiler/pkg/tensor"
)

func mustT(data []float32, shape []int) *tensor.Tensor {
	t, err := tensor.FromData(data, shape)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReduceGradTo(t *testing.T) {
	g := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	row := reduceGradTo(g, []int{3}) // soma eixo 0 -> [5,7,9]
	if row.Data[0] != 5 || row.Data[2] != 9 {
		t.Fatalf("row: %v", row.Data)
	}
	col := reduceGradTo(g, []int{2, 1}) // soma eixo 1 -> [6,15]
	if col.Data[0] != 6 || col.Data[1] != 15 {
		t.Fatalf("col: %v", col.Data)
	}
	sc := reduceGradTo(g, []int{1}) // soma tudo -> 21
	if sc.Data[0] != 21 {
		t.Fatalf("scalar: %v", sc.Data)
	}
}

func TestBackwardManualAndAccumulate(t *testing.T) {
	// x é folha; y = 2*x e z = 3*x (backward manual); l = y + z (soma escalar manual).
	// dl/dx = 2 + 3 = 5. Verifica ordem topológica + acúmulo de grad em nó reusado.
	x := NewLeaf(mustT([]float32{4}, []int{1}))
	y := &Variable{Value: mustT([]float32{8}, []int{1}), parents: []*Variable{x}}
	y.backward = func() { addGrad(x, y.Grad.MulScalar(2)) }
	z := &Variable{Value: mustT([]float32{12}, []int{1}), parents: []*Variable{x}}
	z.backward = func() { addGrad(x, z.Grad.MulScalar(3)) }
	l := &Variable{Value: mustT([]float32{20}, []int{1}), parents: []*Variable{y, z}}
	l.backward = func() {
		addGrad(y, l.Grad)
		addGrad(z, l.Grad)
	}
	l.Backward()
	if x.Grad == nil || x.Grad.Data[0] != 5 {
		t.Fatalf("x.Grad esperado 5, veio %v", x.Grad)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/autograd`
Expected: FAIL (pacote/símbolos indefinidos).

- [ ] **Step 3: Write the implementation**

Criar `pkg/autograd/variable.go`:

```go
// Package autograd implementa diferenciação reversa (reverse-mode autodiff) sobre
// o tensor float32 do pkg/tensor: cada Variable grava um tape de operações e
// Backward propaga gradientes de trás pra frente.
package autograd

import "github.com/advpl/compiler/pkg/tensor"

type Variable struct {
	Value    *tensor.Tensor
	Grad     *tensor.Tensor // acumulado; nil até receber gradiente
	parents  []*Variable
	backward func() // lê v.Grad e acumula nos Grad dos pais
}

// NewLeaf cria uma Variable folha (sem pais) a partir de um tensor de valor.
func NewLeaf(v *tensor.Tensor) *Variable {
	return &Variable{Value: v}
}

// addGrad acumula g no gradiente de v (soma se já houver; cópia na primeira vez).
func addGrad(v *Variable, g *tensor.Tensor) {
	if v.Grad == nil {
		c, _ := tensor.FromData(g.Data, g.Shape)
		v.Grad = c
		return
	}
	if sum, err := v.Grad.Add(g); err == nil {
		v.Grad = sum
	}
}

func onesLike(t *tensor.Tensor) *tensor.Tensor {
	out := tensor.New(t.Shape)
	for i := range out.Data {
		out.Data[i] = 1
	}
	return out
}

// reduceGradTo soma g sobre os eixos replicados no broadcast do Add, casando a
// forma alvo (os 4 casos do pkg/tensor: mesma / escalar / linha / coluna).
func reduceGradTo(g *tensor.Tensor, shape []int) *tensor.Tensor {
	if tensor.ShapeEq(g.Shape, shape) {
		return g
	}
	if tensor.Prod(shape) == 1 {
		out := tensor.New(shape)
		out.Data[0] = g.SumAll()
		return out
	}
	if len(g.Shape) == 2 {
		n := g.Shape[1]
		// linha [N] ou [1,N] -> soma no eixo 0
		if (len(shape) == 1 && shape[0] == n) || (len(shape) == 2 && shape[0] == 1 && shape[1] == n) {
			s, _ := g.SumAxis(0) // [N]
			if len(shape) == 2 {
				s, _ = s.Reshape(shape) // [N] -> [1,N]
			}
			return s
		}
		// coluna [M,1] -> soma no eixo 1
		if len(shape) == 2 && shape[0] == g.Shape[0] && shape[1] == 1 {
			s, _ := g.SumAxis(1) // [M]
			s, _ = s.Reshape(shape) // [M] -> [M,1]
			return s
		}
	}
	return g
}

// Backward propaga gradientes a partir desta Variable (deve ser escalar).
func (v *Variable) Backward() {
	var topo []*Variable
	visited := map[*Variable]bool{}
	var build func(n *Variable)
	build = func(n *Variable) {
		if visited[n] {
			return
		}
		visited[n] = true
		for _, p := range n.parents {
			build(p)
		}
		topo = append(topo, n)
	}
	build(v)
	v.Grad = onesLike(v.Value)
	for i := len(topo) - 1; i >= 0; i-- {
		if topo[i].backward != nil {
			topo[i].backward()
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/autograd`
Expected: PASS (2 testes).

- [ ] **Step 5: Commit**

```bash
git add pkg/autograd/variable.go pkg/autograd/autograd_test.go
git commit -m "autograd: Variable, tape, Backward, reduceGradTo"
```

---

## Task 2: ops diferenciáveis + grad-check por diferenças finitas

**Files:**
- Create: `pkg/autograd/ops.go`
- Modify: `pkg/autograd/autograd_test.go`

**Interfaces:**
- Consumes: `Variable`, `NewLeaf`, `addGrad`, `reduceGradTo` (Task 1); `pkg/tensor` (`Transpose`, `MatMul`, `Add`, `Mul`, `Sub`, `Relu`, `SumAll`, `MeanAll`, `New`, `FromData`).
- Produces: `(*Variable).MatMul(b *Variable) (*Variable, error)`; `Add(b) (*Variable, error)`; `Mul(b) (*Variable, error)`; `Relu() *Variable`; `Sum() *Variable`; `Mean() *Variable`; `MSE(target *Variable) (*Variable, error)`.

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/autograd/autograd_test.go` (não precisa de novos imports — `close32` usa só conversões builtin):

```go
func close32(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	m := float64(a)
	if m < 0 {
		m = -m
	}
	m2 := float64(b)
	if m2 < 0 {
		m2 = -m2
	}
	return float64(d) <= 1e-2+5e-2*(m+m2)/2
}

// gradCheck compara o grad analítico (Backward) de f em x com diferenças finitas.
// f constrói o grafo a partir de uma Variable e devolve a loss escalar {1}.
func gradCheck(t *testing.T, name string, x *tensor.Tensor, f func(*Variable) *Variable) {
	xv := NewLeaf(mustT(x.Data, x.Shape))
	f(xv).Backward()
	analytic := xv.Grad
	const eps = 1e-2
	for i := range x.Data {
		orig := x.Data[i]
		xp := mustT(x.Data, x.Shape)
		xp.Data[i] = orig + eps
		xm := mustT(x.Data, x.Shape)
		xm.Data[i] = orig - eps
		lp := f(NewLeaf(xp)).Value.Data[0]
		lm := f(NewLeaf(xm)).Value.Data[0]
		num := (lp - lm) / (2 * eps)
		if !close32(analytic.Data[i], num) {
			t.Fatalf("%s grad[%d]: analitico=%v numerico=%v", name, i, analytic.Data[i], num)
		}
	}
}

func TestGradMatMul(t *testing.T) {
	W := mustT([]float32{0.5, -0.3, 0.2, 0.1, 0.4, -0.6}, []int{3, 2}) // [3,2]
	// f(A) = sum(A[2,3] · W[3,2]); checa grad em A
	gradCheck(t, "matmul-A", mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}), func(a *Variable) *Variable {
		y, err := a.MatMul(NewLeaf(W))
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	A := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3}) // [2,3]
	// f(B) = sum(A · B[3,2]); checa grad em B
	gradCheck(t, "matmul-B", mustT([]float32{0.5, -0.3, 0.2, 0.1, 0.4, -0.6}, []int{3, 2}), func(b *Variable) *Variable {
		y, err := NewLeaf(A).MatMul(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
}

func TestGradAddBroadcast(t *testing.T) {
	base := mustT([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	// bias linha [3]: f(b) = sum(base + b)
	gradCheck(t, "add-row", mustT([]float32{0.1, 0.2, 0.3}, []int{3}), func(b *Variable) *Variable {
		y, err := NewLeaf(base).Add(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	// bias coluna [2,1]
	gradCheck(t, "add-col", mustT([]float32{0.5, 0.7}, []int{2, 1}), func(b *Variable) *Variable {
		y, err := NewLeaf(base).Add(b)
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
}

func TestGradMulReluSumMeanMSE(t *testing.T) {
	B := mustT([]float32{2, -1, 0.5, 3}, []int{2, 2})
	gradCheck(t, "mul", mustT([]float32{1, 2, 3, 4}, []int{2, 2}), func(x *Variable) *Variable {
		y, err := x.Mul(NewLeaf(B))
		if err != nil {
			panic(err)
		}
		return y.Sum()
	})
	// Relu: valores longe de 0 (não-diferenciável só em 0)
	gradCheck(t, "relu", mustT([]float32{-2, 1, -0.5, 3}, []int{2, 2}), func(x *Variable) *Variable {
		return x.Relu().Sum()
	})
	gradCheck(t, "sum", mustT([]float32{1, 2, 3}, []int{3}), func(x *Variable) *Variable {
		return x.Sum()
	})
	gradCheck(t, "mean", mustT([]float32{1, 2, 3, 4}, []int{4}), func(x *Variable) *Variable {
		return x.Mean()
	})
	target := mustT([]float32{1, 0, 2, 1}, []int{2, 2})
	gradCheck(t, "mse", mustT([]float32{1.5, 0.2, 1.8, 0.9}, []int{2, 2}), func(x *Variable) *Variable {
		l, err := x.MSE(NewLeaf(target))
		if err != nil {
			panic(err)
		}
		return l
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/autograd`
Expected: FAIL (`MatMul`/`Add`/... indefinidos).

- [ ] **Step 3: Write the implementation**

Criar `pkg/autograd/ops.go`:

```go
package autograd

import (
	"fmt"

	"github.com/advpl/compiler/pkg/tensor"
)

// MatMul: Y = A·B (2D x 2D). dA = dY·Bᵀ; dB = Aᵀ·dY.
func (a *Variable) MatMul(b *Variable) (*Variable, error) {
	if len(a.Value.Shape) != 2 || len(b.Value.Shape) != 2 {
		return nil, fmt.Errorf("Variable.MatMul: requer 2D x 2D")
	}
	y, err := a.Value.MatMul(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		bt, _ := b.Value.Transpose()
		da, _ := out.Grad.MatMul(bt)
		addGrad(a, da)
		at, _ := a.Value.Transpose()
		db, _ := at.MatMul(out.Grad)
		addGrad(b, db)
	}
	return out, nil
}

// Add com broadcast (do pkg/tensor). dA/dB = reduceGradTo(dY, forma).
func (a *Variable) Add(b *Variable) (*Variable, error) {
	y, err := a.Value.Add(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		addGrad(a, reduceGradTo(out.Grad, a.Value.Shape))
		addGrad(b, reduceGradTo(out.Grad, b.Value.Shape))
	}
	return out, nil
}

// Mul (Hadamard, mesma forma). dA = dY⊙B; dB = dY⊙A.
func (a *Variable) Mul(b *Variable) (*Variable, error) {
	y, err := a.Value.Mul(b.Value)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a, b}}
	out.backward = func() {
		da, _ := out.Grad.Mul(b.Value)
		addGrad(a, da)
		db, _ := out.Grad.Mul(a.Value)
		addGrad(b, db)
	}
	return out, nil
}

// Relu. dA = dY ⊙ (A>0).
func (a *Variable) Relu() *Variable {
	y := a.Value.Relu()
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		mask := tensor.New(a.Value.Shape)
		for i, v := range a.Value.Data {
			if v > 0 {
				mask.Data[i] = 1
			}
		}
		dg, _ := out.Grad.Mul(mask)
		addGrad(a, dg)
	}
	return out
}

// Sum (todos os elementos) -> escalar {1}. dA = broadcast(dY).
func (a *Variable) Sum() *Variable {
	y, _ := tensor.FromData([]float32{a.Value.SumAll()}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0]
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = g
		}
		addGrad(a, dg)
	}
	return out
}

// Mean -> escalar {1}. dA = broadcast(dY / N).
func (a *Variable) Mean() *Variable {
	n := float32(a.Value.Size())
	y, _ := tensor.FromData([]float32{a.Value.MeanAll()}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		g := out.Grad.Data[0] / n
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = g
		}
		addGrad(a, dg)
	}
	return out
}

// MSE(ŷ=a, alvo constante) -> escalar {1}. dŶ = (2/N)(Ŷ−alvo). O alvo não recebe grad.
func (a *Variable) MSE(target *Variable) (*Variable, error) {
	diff, err := a.Value.Sub(target.Value)
	if err != nil {
		return nil, err
	}
	n := float32(a.Value.Size())
	var s float32
	for _, d := range diff.Data {
		s += d * d
	}
	y, _ := tensor.FromData([]float32{s / n}, []int{1})
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		scale := 2 * out.Grad.Data[0] / n
		dg := tensor.New(a.Value.Shape)
		for i := range dg.Data {
			dg.Data[i] = scale * diff.Data[i]
		}
		addGrad(a, dg)
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/autograd`
Expected: PASS (todos os grad-checks).

- [ ] **Step 5: Commit**

```bash
git add pkg/autograd/ops.go pkg/autograd/autograd_test.go
git commit -m "autograd: differentiable ops with finite-difference grad-check"
```

---

## Task 3: otimizador SGD

**Files:**
- Create: `pkg/autograd/sgd.go`
- Modify: `pkg/autograd/autograd_test.go`

**Interfaces:**
- Consumes: `Variable` (Task 1); ops `Mul`/`Sum` (Task 2); `pkg/tensor` (`MulScalar`, `Sub`).
- Produces: `type SGD struct{...}`; `NewSGD(params []*Variable, lr float32) *SGD`; `(*SGD).Step()`; `(*SGD).ZeroGrad()`.

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/autograd/autograd_test.go`:

```go
func TestSGDStepReducesLoss(t *testing.T) {
	// loss = sum(p*p) = Σp²; grad = 2p; um passo com lr=0.1 encolhe p e reduz a loss.
	p := NewLeaf(mustT([]float32{3, -4}, []int{2}))
	lossBefore := func() float32 {
		sq, _ := p.Value.Mul(p.Value)
		return sq.SumAll()
	}
	before := lossBefore()

	opt := NewSGD([]*Variable{p}, 0.1)
	opt.ZeroGrad()
	l, _ := p.Mul(p)
	l.Sum().Backward()
	opt.Step()

	after := lossBefore()
	if !(after < before) {
		t.Fatalf("SGD nao reduziu a loss: antes=%v depois=%v", before, after)
	}
	// grad esperado 2p = [6,-8]; passo p -= 0.1*grad => [2.4,-3.2]
	if !close32(p.Value.Data[0], 2.4) || !close32(p.Value.Data[1], -3.2) {
		t.Fatalf("passo do SGD errado: %v", p.Value.Data)
	}
}

func TestZeroGrad(t *testing.T) {
	p := NewLeaf(mustT([]float32{1, 2}, []int{2}))
	p.Sum().Backward()
	if p.Grad == nil {
		t.Fatal("esperava grad apos backward")
	}
	NewSGD([]*Variable{p}, 0.1).ZeroGrad()
	if p.Grad != nil {
		t.Fatal("ZeroGrad deveria limpar o grad")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/autograd`
Expected: FAIL (`NewSGD` indefinido).

- [ ] **Step 3: Write the implementation**

Criar `pkg/autograd/sgd.go`:

```go
package autograd

// SGD é o gradiente descendente estocástico simples: p := p - lr·grad(p).
type SGD struct {
	params []*Variable
	lr     float32
}

func NewSGD(params []*Variable, lr float32) *SGD {
	return &SGD{params: params, lr: lr}
}

// Step atualiza cada parâmetro in-place (mantém a identidade do tensor de valor).
func (o *SGD) Step() {
	for _, p := range o.params {
		if p.Grad == nil {
			continue
		}
		upd := p.Grad.MulScalar(o.lr)
		nv, err := p.Value.Sub(upd)
		if err == nil {
			copy(p.Value.Data, nv.Data)
		}
	}
}

// ZeroGrad zera os gradientes dos parâmetros antes do próximo backward.
func (o *SGD) ZeroGrad() {
	for _, p := range o.params {
		p.Grad = nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/autograd`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/autograd/sgd.go pkg/autograd/autograd_test.go
git commit -m "autograd: SGD optimizer (Step, ZeroGrad)"
```

---

## Task 4: classes `Variable` e `SGD` na VM + fixture

**Files:**
- Create: `pkg/vm/autograd_native.go`
- Modify: `pkg/compiler/codegen.go` (`builtinClasses`)
- Modify: `pkg/vm/vm.go` (switch `OP_NEW_INSTANCE`; `callNativeMethod`)
- Test: `tests/autograd_test.prw`

**Interfaces:**
- Consumes: `pkg/autograd` (Tasks 1-3); `pkg/tensor`; helpers `wrapTensor`/`floatsFromArg`/`shapeFromArg`/`terr`/`getArg` (já em `pkg/vm/tensor_native.go`/`natives.go`).
- Produces: classes AdvPL `Variable` (`New`/`FromArray`/`MatMul`/`Add`/`Mul`/`Relu`/`Sum`/`Mean`/`MSE`/`Backward`/`Value`/`Grad`) e `SGD` (`New`/`Step`/`ZeroGrad`).

- [ ] **Step 1: Write the failing test**

Criar `tests/autograd_test.prw`:

```advpl
User Function AutogradTst()
    Local oX  := Variable():FromArray({1,2,3,4,5,6}, {2,3})   // [2,3]
    Local oW  := Variable():FromArray({0.1,0.2,0.1,0.2,0.1,0.2}, {3,2})  // [3,2]
    Local ob  := Variable():FromArray({0.5,0.5}, {2})         // bias linha
    Local oAlvo := Variable():FromArray({1,0,0,1}, {2,2})
    Local oY, oL, aGW, nFail := 0

    oY := oX:MatMul(oW):Add(ob):Relu()     // [2,2]
    oL := oY:MSE(oAlvo)                     // escalar
    oL:Backward()

    aGW := oW:Grad():Shape()
    If aGW[1] != 3 .Or. aGW[2] != 2
        ConOut("FALHA forma do grad de W: " + Str(aGW[1]) + "," + Str(aGW[2])); nFail++
    EndIf
    If ob:Grad():Size() != 2
        ConOut("FALHA tamanho do grad de b"); nFail++
    EndIf
    // loss é escalar >= 0
    If oL:Value():ToArray()[1] < 0
        ConOut("FALHA loss negativa"); nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build -o advplc ./cmd/advplc && ./advplc run tests/autograd_test.prw`
Expected: FAIL — `Variable()` não reconhecida.

- [ ] **Step 3: Registrar as classes builtin (3 pontos)**

Em `pkg/compiler/codegen.go`, no mapa `builtinClasses`, adicionar:

```go
	"VARIABLE":      true,
	"SGD":           true,
```

Em `pkg/vm/vm.go`, no `switch upperName` do `OP_NEW_INSTANCE` (junto de `case "TENSOR":`):

```go
		case "VARIABLE":
			v.push(newVariableObject())
			return nil
		case "SGD":
			v.push(newSGDObject())
			return nil
```

Em `pkg/vm/vm.go`, no `callNativeMethod` (junto de `case "Tensor":`):

```go
	case "Variable":
		return v.callVariableMethod(obj, upperMethod, args)
	case "SGD":
		return v.callSGDMethod(obj, upperMethod, args)
```

- [ ] **Step 4: Criar a ligação com a VM**

Criar `pkg/vm/autograd_native.go`:

```go
package vm

import (
	"github.com/advpl/compiler/pkg/autograd"
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tensor"
)

func newVariableObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Variable", nil)
	obj.Native = autograd.NewLeaf(&tensor.Tensor{Shape: []int{0}, Data: []float32{}})
	return obj
}

func wrapVariable(v *autograd.Variable) *advplrt.ObjectValue {
	obj := advplrt.NewObject("Variable", nil)
	obj.Native = v
	return obj
}

// argVariable lê o *autograd.Variable de um argumento que deve ser um objeto Variable.
func argVariable(args []advplrt.Value, i int) (*autograd.Variable, error) {
	o, ok := getArg(args, i).(*advplrt.ObjectValue)
	if !ok {
		return nil, advplrt.NewError("Variable: argumento não é um objeto Variable")
	}
	vv, ok := o.Native.(*autograd.Variable)
	if !ok {
		return nil, advplrt.NewError("Variable: objeto sem estado interno de Variable")
	}
	return vv, nil
}

func (v *VM) callVariableMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	self, _ := obj.Native.(*autograd.Variable)

	switch method {
	case "NEW":
		// New(oTensor): folha a partir de um objeto Tensor do S2
		to, ok := getArg(args, 0).(*advplrt.ObjectValue)
		if !ok {
			return advplrt.NewError("Variable:New requer um objeto Tensor")
		}
		tt, ok := to.Native.(*tensor.Tensor)
		if !ok {
			return advplrt.NewError("Variable:New: objeto não é Tensor")
		}
		obj.Native = autograd.NewLeaf(tt)
		v.push(obj)
	case "FROMARRAY":
		shp := shapeFromArg(getArg(args, 1))
		if err := validShape(shp); err != nil {
			return err
		}
		tt, err := tensor.FromData(floatsFromArg(getArg(args, 0)), shp)
		if err != nil {
			return terr(err)
		}
		obj.Native = autograd.NewLeaf(tt)
		v.push(obj)

	case "MATMUL", "ADD", "MUL", "MSE":
		b, err := argVariable(args, 0)
		if err != nil {
			return err
		}
		var r *autograd.Variable
		switch method {
		case "MATMUL":
			r, err = self.MatMul(b)
		case "ADD":
			r, err = self.Add(b)
		case "MUL":
			r, err = self.Mul(b)
		case "MSE":
			r, err = self.MSE(b)
		}
		if err != nil {
			return terr(err)
		}
		v.push(wrapVariable(r))
	case "RELU":
		v.push(wrapVariable(self.Relu()))
	case "SUM":
		v.push(wrapVariable(self.Sum()))
	case "MEAN":
		v.push(wrapVariable(self.Mean()))

	case "BACKWARD":
		self.Backward()
		v.push(obj)
	case "VALUE":
		v.push(wrapTensor(self.Value))
	case "GRAD":
		if self.Grad == nil {
			v.push(wrapTensor(tensor.New(self.Value.Shape)))
		} else {
			v.push(wrapTensor(self.Grad))
		}

	default:
		return advplrt.NewError("Variable: método desconhecido " + method)
	}
	return nil
}

// --- SGD ---

type sgdState struct{ opt *autograd.SGD }

func newSGDObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("SGD", nil)
	obj.Native = &sgdState{}
	return obj
}

func (v *VM) callSGDMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*sgdState)

	switch method {
	case "NEW":
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewError("SGD:New requer um array de Variables")
		}
		params := make([]*autograd.Variable, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			o, ok := e.(*advplrt.ObjectValue)
			if !ok {
				return advplrt.NewError("SGD:New: elemento não é Variable")
			}
			vv, ok := o.Native.(*autograd.Variable)
			if !ok {
				return advplrt.NewError("SGD:New: elemento não é Variable")
			}
			params = append(params, vv)
		}
		lr := float32(advplrt.ToFloat(getArg(args, 1)))
		st.opt = autograd.NewSGD(params, lr)
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
		return advplrt.NewError("SGD: método desconhecido " + method)
	}
	return nil
}
```

- [ ] **Step 5: Rebuild e rodar o fixture**

Run: `go build -o advplc ./cmd/advplc && ./advplc run tests/autograd_test.prw`
Expected:
```
OK: 3/3 verificacoes passaram.
```

- [ ] **Step 6: Regressão**

Run: `go test ./... 2>&1 | grep -vE '^ok|no test files'`
Expected: sem saída.
Run: `./advplc run tests/mlp_demo.prw 2>&1 | tail -1`
Expected: `OK: 3/3 verificacoes passaram.`

- [ ] **Step 7: Commit**

```bash
git add pkg/autograd/ pkg/vm/autograd_native.go pkg/vm/vm.go pkg/compiler/codegen.go tests/autograd_test.prw
git commit -m "vm: Variable and SGD classes bound to pkg/autograd"
```

---

## Task 5: aceite (treino de MLP) + docs

**Files:**
- Create: `tests/train_demo.prw`
- Modify: `README.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: classes `Variable` e `SGD` (Task 4).

- [ ] **Step 1: Escrever a demo de treino (MLP com SGD)**

Criar `tests/train_demo.prw`:

```advpl
// Treina um MLP 2-4-1 (Relu) para ajustar y = x1 + 2*x2, com pesos fixos, SGD e MSE.
// A loss deve cair bem abaixo da inicial — prova autodiff + treino ponta a ponta.
User Function TrainDemo()
    Local oX  := Variable():FromArray({1,0, 0,1, 1,1, 2,1}, {4,2})     // 4 exemplos
    Local oY  := Variable():FromArray({1, 2, 3, 4}, {4,1})             // y = x1 + 2*x2
    // Pesos iniciais fixos e ASSIMÉTRICOS (quebram a simetria entre unidades ocultas
    // -> mais capacidade). Bias positivo mantém as ReLU ativas nos inputs positivos.
    Local oW1 := Variable():FromArray({0.10,0.15,0.20,0.05, 0.05,0.10,0.15,0.20}, {2,4})
    Local ob1 := Variable():FromArray({0.50,0.40,0.60,0.50}, {4})
    Local oW2 := Variable():FromArray({0.10,0.20,0.15,0.05}, {4,1})
    Local ob2 := Variable():FromArray({0}, {1})
    Local oOpt := SGD():New({oW1, ob1, oW2, ob2}, 0.05)
    Local nEpoca := 0
    Local oH, oPred, oLoss
    Local nInicial := 0, nAtual := 0

    For nEpoca := 1 To 1000
        // forward
        oH    := oX:MatMul(oW1):Add(ob1):Relu()
        oPred := oH:MatMul(oW2):Add(ob2)
        oLoss := oPred:MSE(oY)
        nAtual := oLoss:Value():ToArray()[1]
        If nEpoca == 1
            nInicial := nAtual
        EndIf
        If nEpoca == 1 .Or. Mod(nEpoca, 200) == 0
            ConOut("epoca " + Str(nEpoca,4) + " loss " + Str(nAtual,10,5))
        EndIf
        // backward + passo
        oOpt:ZeroGrad()
        oLoss:Backward()
        oOpt:Step()
    Next nEpoca

    ConOut("loss inicial " + Str(nInicial,10,5) + " -> final " + Str(nAtual,10,5))
    If nAtual < nInicial * 0.2
        ConOut("OK: treino reduziu a loss (autodiff + SGD funcionam).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente (final >= 20% da inicial)")
    EndIf
Return
```

- [ ] **Step 2: Rodar a demo**

Run: `./advplc run tests/train_demo.prw`
Expected: a loss cai a cada bloco de épocas e termina com:
```
OK: treino reduziu a loss (autodiff + SGD funcionam).
```
Se a loss NÃO cair abaixo de 20% da inicial, PARE e reporte (indica bug de backward, não algo a mascarar afrouxando o limite).

- [ ] **Step 3: Atualizar o README**

Em `README.md`, na lista de "## Recursos", adicionar a linha:

```markdown
- **Autodiff + treino (float32)**: motor de diferenciação reversa (`pkg/autograd`) com a classe `Variable` (tape + `Backward`), ops diferenciáveis (MatMul, Add, Mul, Relu, Sum, Mean, MSE) e otimizador `SGD` — treina modelos float com o AdvPL orquestrando; ver [Autodiff e treino](#autodiff-e-treino)
```

E adicionar, logo após a seção "## Núcleo de Tensor", uma seção nova:

```markdown
## Autodiff e treino

Sobre o núcleo de Tensor, a classe `Variable` grava um tape de operações e
`Backward()` propaga gradientes (reverse-mode autodiff). Com o otimizador `SGD`
dá pra TREINAR um modelo float — o AdvPL orquestra o laço; o Go faz forward e
backward.

```advpl
Local oW  := Variable():FromArray(aPesos, {nIn, nOut})
Local oB  := Variable():FromArray(aBias, {nOut})
Local oOpt := SGD():New({oW, oB}, 0.05)
// laço de treino:
Local oPred := oX:MatMul(oW):Add(oB):Relu()
Local oLoss := oPred:MSE(oY)
oOpt:ZeroGrad()
oLoss:Backward()          // preenche oW:Grad(), oB:Grad()
oOpt:Step()               // oW := oW - lr*grad
```

Ops diferenciáveis: `MatMul`, `Add` (com broadcast), `Mul`, `Relu`, `Sum`, `Mean`,
`MSE`. `oV:Value()`/`oV:Grad()` devolvem o `Tensor` de valor/gradiente. Este ciclo
entrega o motor + SGD; softmax/cross-entropy, Adam, embedding e módulos vêm nos
próximos ciclos. Corretude validada por verificação numérica de gradiente
(diferenças finitas) no `go test`.
```

- [ ] **Step 4: Atualizar o CHANGELOG**

Em `CHANGELOG.md`, na seção `## [Não lançado]` (criada no S2), adicionar ao final dela:

```markdown

### Autodiff + treino (Sub-projeto 3a)

- Motor de **diferenciação reversa** (`pkg/autograd`): classe `Variable` (valor +
  grad + tape de ops) e `Backward()` em ordem topológica reversa; ops
  diferenciáveis MatMul, Add (broadcast), Mul, Relu, Sum, Mean e a loss MSE, todas
  reusando os kernels do `pkg/tensor` (S2, intocado). Otimizador **SGD**
  (`Step`/`ZeroGrad`). Classes AdvPL `Variable` e `SGD`. Corretude por verificação
  numérica de gradiente (diferenças finitas) no `go test`; aceite `tests/train_demo.prw`
  treina um MLP e a loss cai bem abaixo da inicial. Softmax-CE, Adam, embedding e
  módulos ficam para os próximos ciclos.
```

- [ ] **Step 5: Commit**

```bash
git add tests/train_demo.prw README.md CHANGELOG.md
git commit -m "autograd: MLP training acceptance demo + docs"
```

---

## Notas de verificação final

Ao término das 5 tasks:
- `go test ./...` verde (inclui `pkg/autograd` com grad-check).
- `./advplc run tests/autograd_test.prw` → `OK: 3/3`.
- `./advplc run tests/train_demo.prw` → treina, loss cai < 20% da inicial.
- S2 sem regressão (`mlp_demo.prw`, `tensor_test.prw` OK).
- Os 5 critérios de aceite da spec satisfeitos.

Publicação de release (tag/CI) é decisão do usuário — fora deste plano.
