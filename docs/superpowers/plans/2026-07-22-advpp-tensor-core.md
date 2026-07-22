# AdvPP Tensor Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um núcleo de Tensor float32 acelerado em Go (`pkg/tensor`) exposto ao AdvPL como a classe `Tensor`, com kernels de forward (matmul, elementwise/broadcast, reduções, ativações, softmax, argmax, embedding) que permitem construir e rodar um modelo float com o AdvPL orquestrando.

**Architecture:** Kernels em Go puro sobre `[]float32` plano (`pkg/tensor`, testável com `go test`), ligados à VM por uma classe nativa `Tensor` cujo estado vive em `ObjectValue.Native` (mesmo mecanismo do `FWGridProcess`). O AdvPL cria tensores e chama métodos; o Go faz a conta.

**Tech Stack:** Go puro (sem CGO), a VM/compilador AdvPP existente, fixtures AdvPL (`.prw`).

## Global Constraints

- Go puro, sem CGO. dtype **float32**. Armazenamento **row-major**.
- Índices expostos ao AdvPL são **1-based**; internamente 0-based; `nAxis` é 1-based sobre as dims.
- Operações **funcionais** (cada uma devolve um Tensor novo), exceto `Set` (muta, devolve self).
- Erros de forma **lançam** `advplrt.NewError(msg)` (é `*ErrorValue`, capturável por `Try/Catch`); **nunca** `fmt.Errorf` na camada da VM (aborta o programa).
- Registrar classe builtin nova em TRÊS lugares: `builtinClasses` (`pkg/compiler/codegen.go`), switch de `OP_NEW_INSTANCE` (`pkg/vm/vm.go`), e `callNativeMethod` (`pkg/vm/vm.go`).
- Rebuild após mudança Go da VM/compilador: `go build -o advplc ./cmd/advplc`.
- Kernels testados com `go test ./pkg/tensor`; a classe testada com `./advplc run tests/*.prw`.
- Regressão obrigatória verde: `go test ./...`; fixtures e exemplos (`pt_nn.prw`) sem regressão.
- Reduções **sem eixo → número AdvPL**; **com eixo → Tensor**. Argmax idem (índices 1-based).

---

## File Structure

- `pkg/tensor/tensor.go` — tipo `Tensor`, construtores, helpers de forma/índice, ponte de dados.
- `pkg/tensor/ops.go` — kernels: elementwise/broadcast, matmul, transpose, reshape, reduções, ativações, softmax, argmax, index-rows.
- `pkg/tensor/tensor_test.go` — `go test` dos kernels.
- `pkg/vm/tensor_native.go` — classe `Tensor` na VM (construtores, `callTensorMethod`, ponte `advplrt.Value`↔`tensor.Tensor`).
- `pkg/compiler/codegen.go` — adiciona `"TENSOR"` a `builtinClasses`.
- `pkg/vm/vm.go` — `case "TENSOR"` em `OP_NEW_INSTANCE`; `case "Tensor"` em `callNativeMethod`.
- `tests/tensor_test.prw` — fixture da API (auto-verificável).
- `tests/mlp_demo.prw` — aceite (MLP float forward).

---

## Task 1: `pkg/tensor` — tipo, construtores, ponte de dados

**Files:**
- Create: `pkg/tensor/tensor.go`
- Test: `pkg/tensor/tensor_test.go`

**Interfaces:**
- Produces: `type Tensor struct { Shape []int; Data []float32 }`; `New(shape []int) *Tensor`; `FromData(data []float32, shape []int) (*Tensor, error)`; `Rand(shape []int, scale float32) *Tensor`; `(*Tensor).Size() int`; `(*Tensor).Offset(idx []int) (int, error)`; `(*Tensor).At(idx []int) (float32, error)`; `(*Tensor).SetAt(idx []int, val float32) error`; `Prod(shape []int) int`; `ShapeEq(a, b []int) bool`.

- [ ] **Step 1: Write the failing test**

Criar `pkg/tensor/tensor_test.go`:

```go
package tensor

import "testing"

func TestNewAndSize(t *testing.T) {
	x := New([]int{2, 3})
	if x.Size() != 6 {
		t.Fatalf("Size = %d, quer 6", x.Size())
	}
	for _, v := range x.Data {
		if v != 0 {
			t.Fatalf("New deve ser zeros, achei %v", v)
		}
	}
}

func TestFromDataAndOffset(t *testing.T) {
	x, err := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	off, _ := x.Offset([]int{1, 2}) // 0-based (linha 1, col 2) -> 1*3+2 = 5
	if off != 5 {
		t.Fatalf("Offset = %d, quer 5", off)
	}
	got, _ := x.At([]int{1, 2})
	if got != 6 {
		t.Fatalf("At = %v, quer 6", got)
	}
}

func TestFromDataSizeMismatch(t *testing.T) {
	if _, err := FromData([]float32{1, 2, 3}, []int{2, 2}); err == nil {
		t.Fatal("esperava erro de tamanho")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tensor`
Expected: FAIL (pacote não existe / símbolos indefinidos).

- [ ] **Step 3: Write the implementation**

Criar `pkg/tensor/tensor.go`:

```go
// Package tensor fornece um tensor float32 denso (row-major) com kernels de
// forward em Go puro — a base numérica da classe AdvPL `Tensor`.
package tensor

import (
	"fmt"
	"math/rand"
)

type Tensor struct {
	Shape []int
	Data  []float32
}

// Prod devolve o número de elementos de uma forma.
func Prod(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// ShapeEq diz se duas formas são idênticas.
func ShapeEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func copyInts(s []int) []int { return append([]int(nil), s...) }

// New cria um tensor de zeros com a forma dada.
func New(shape []int) *Tensor {
	return &Tensor{Shape: copyInts(shape), Data: make([]float32, Prod(shape))}
}

// FromData cria um tensor a partir de dados row-major + forma.
func FromData(data []float32, shape []int) (*Tensor, error) {
	if Prod(shape) != len(data) {
		return nil, fmt.Errorf("FromData: len(data)=%d != produto(shape=%v)=%d", len(data), shape, Prod(shape))
	}
	return &Tensor{Shape: copyInts(shape), Data: append([]float32(nil), data...)}, nil
}

// Rand cria um tensor uniforme em [-scale, scale].
func Rand(shape []int, scale float32) *Tensor {
	t := New(shape)
	for i := range t.Data {
		t.Data[i] = (rand.Float32()*2 - 1) * scale
	}
	return t
}

// Size é o número total de elementos.
func (t *Tensor) Size() int { return len(t.Data) }

// Offset converte um índice multi-dim (0-based) no offset row-major.
func (t *Tensor) Offset(idx []int) (int, error) {
	if len(idx) != len(t.Shape) {
		return 0, fmt.Errorf("Offset: idx com %d dims, tensor tem %d", len(idx), len(t.Shape))
	}
	off := 0
	for i, ix := range idx {
		if ix < 0 || ix >= t.Shape[i] {
			return 0, fmt.Errorf("Offset: índice %d fora de faixa na dim %d (0..%d)", ix, i, t.Shape[i]-1)
		}
		off = off*t.Shape[i] + ix
	}
	return off, nil
}

// At lê um elemento (idx 0-based).
func (t *Tensor) At(idx []int) (float32, error) {
	off, err := t.Offset(idx)
	if err != nil {
		return 0, err
	}
	return t.Data[off], nil
}

// SetAt grava um elemento (idx 0-based).
func (t *Tensor) SetAt(idx []int, val float32) error {
	off, err := t.Offset(idx)
	if err != nil {
		return err
	}
	t.Data[off] = val
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tensor`
Expected: PASS (3 testes).

- [ ] **Step 5: Commit**

```bash
git add pkg/tensor/tensor.go pkg/tensor/tensor_test.go
git commit -m "tensor: core type, constructors, indexing"
```

---

## Task 2: elementwise + broadcast + escalar

**Files:**
- Create: `pkg/tensor/ops.go`
- Modify: `pkg/tensor/tensor_test.go`

**Interfaces:**
- Consumes: `Tensor`, `New`, `FromData`, `ShapeEq`, `Prod` (Task 1).
- Produces: `(*Tensor).Add(b *Tensor) (*Tensor, error)`; `Sub`, `Mul`, `Div` (mesma assinatura); `(*Tensor).AddScalar(s float32) *Tensor`; `(*Tensor).MulScalar(s float32) *Tensor`.

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/tensor/tensor_test.go`:

```go
func TestElementwiseSameShape(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{10, 20, 30, 40}, []int{2, 2})
	got, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{11, 22, 33, 44}
	for i := range want {
		if got.Data[i] != want[i] {
			t.Fatalf("Add[%d]=%v quer %v", i, got.Data[i], want[i])
		}
	}
}

func TestBroadcastRowAndCol(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	row, _ := FromData([]float32{10, 20, 30}, []int{3})
	gr, err := a.Add(row) // por linha
	if err != nil {
		t.Fatal(err)
	}
	if gr.Data[0] != 11 || gr.Data[5] != 36 {
		t.Fatalf("broadcast linha errado: %v", gr.Data)
	}
	col, _ := FromData([]float32{100, 200}, []int{2, 1})
	gc, err := a.Add(col) // por coluna
	if err != nil {
		t.Fatal(err)
	}
	if gc.Data[0] != 101 || gc.Data[5] != 206 {
		t.Fatalf("broadcast coluna errado: %v", gc.Data)
	}
}

func TestScalarOps(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3}, []int{3})
	if g := a.MulScalar(2); g.Data[2] != 6 {
		t.Fatalf("MulScalar errado: %v", g.Data)
	}
	sc, _ := FromData([]float32{5}, []int{1})
	g, _ := a.Add(sc) // b escalar (Size 1)
	if g.Data[0] != 6 {
		t.Fatalf("add escalar errado: %v", g.Data)
	}
}

func TestBroadcastIncompatible(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{1, 2, 3}, []int{3})
	if _, err := a.Add(b); err == nil {
		t.Fatal("esperava erro de broadcast incompatível")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tensor`
Expected: FAIL (`Add`/`MulScalar` indefinidos).

- [ ] **Step 3: Write the implementation**

Criar `pkg/tensor/ops.go`:

```go
package tensor

import (
	"fmt"
	"math"
)

// binOp aplica f elemento a elemento com broadcast limitado:
// mesma forma; b escalar (Size 1); ou a 2D [M,N] com b linha [N]/[1,N] ou coluna [M,1].
func binOp(a, b *Tensor, f func(x, y float32) float32) (*Tensor, error) {
	if ShapeEq(a.Shape, b.Shape) {
		out := New(a.Shape)
		for i := range a.Data {
			out.Data[i] = f(a.Data[i], b.Data[i])
		}
		return out, nil
	}
	if b.Size() == 1 {
		out := New(a.Shape)
		s := b.Data[0]
		for i := range a.Data {
			out.Data[i] = f(a.Data[i], s)
		}
		return out, nil
	}
	if len(a.Shape) == 2 {
		m, n := a.Shape[0], a.Shape[1]
		isRow := (len(b.Shape) == 1 && b.Shape[0] == n) ||
			(len(b.Shape) == 2 && b.Shape[0] == 1 && b.Shape[1] == n)
		if isRow {
			out := New(a.Shape)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data[i*n+j] = f(a.Data[i*n+j], b.Data[j])
				}
			}
			return out, nil
		}
		if len(b.Shape) == 2 && b.Shape[0] == m && b.Shape[1] == 1 {
			out := New(a.Shape)
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					out.Data[i*n+j] = f(a.Data[i*n+j], b.Data[i])
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("shapes incompatíveis para broadcast: %v e %v", a.Shape, b.Shape)
}

func (a *Tensor) Add(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x + y })
}
func (a *Tensor) Sub(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x - y })
}
func (a *Tensor) Mul(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x * y })
}
func (a *Tensor) Div(b *Tensor) (*Tensor, error) {
	return binOp(a, b, func(x, y float32) float32 { return x / y })
}

func (a *Tensor) AddScalar(s float32) *Tensor {
	out := New(a.Shape)
	for i, v := range a.Data {
		out.Data[i] = v + s
	}
	return out
}
func (a *Tensor) MulScalar(s float32) *Tensor {
	out := New(a.Shape)
	for i, v := range a.Data {
		out.Data[i] = v * s
	}
	return out
}

var _ = math.Exp // math será usado nas ativações (Task 5)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tensor`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tensor/ops.go pkg/tensor/tensor_test.go
git commit -m "tensor: elementwise ops with limited broadcast + scalar"
```

---

## Task 3: MatMul + Transpose + Reshape

**Files:**
- Modify: `pkg/tensor/ops.go`
- Modify: `pkg/tensor/tensor_test.go`

**Interfaces:**
- Consumes: `Tensor`, `New`, `FromData`, `Prod` (Task 1).
- Produces: `(*Tensor).MatMul(b *Tensor) (*Tensor, error)`; `(*Tensor).Transpose() (*Tensor, error)`; `(*Tensor).Reshape(shape []int) (*Tensor, error)`.

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/tensor/tensor_test.go`:

```go
func TestMatMul(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4}, []int{2, 2})
	b, _ := FromData([]float32{5, 6, 7, 8}, []int{2, 2})
	got, err := a.MatMul(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []float32{19, 22, 43, 50}
	for i := range want {
		if got.Data[i] != want[i] {
			t.Fatalf("MatMul[%d]=%v quer %v", i, got.Data[i], want[i])
		}
	}
}

func TestMatVec(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	v, _ := FromData([]float32{1, 0, 1}, []int{3})
	got, err := a.MatMul(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Shape) != 1 || got.Data[0] != 4 || got.Data[1] != 10 {
		t.Fatalf("MatVec errado: shape=%v data=%v", got.Shape, got.Data)
	}
}

func TestTransposeReshape(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	tr, err := a.Transpose()
	if err != nil {
		t.Fatal(err)
	}
	if tr.Shape[0] != 3 || tr.Shape[1] != 2 || tr.Data[0] != 1 || tr.Data[1] != 4 {
		t.Fatalf("Transpose errado: shape=%v data=%v", tr.Shape, tr.Data)
	}
	rs, err := a.Reshape([]int{3, 2})
	if err != nil {
		t.Fatal(err)
	}
	if rs.Shape[0] != 3 || rs.Data[5] != 6 {
		t.Fatalf("Reshape errado: %v", rs.Data)
	}
	if _, err := a.Reshape([]int{4, 2}); err == nil {
		t.Fatal("esperava erro de reshape incompatível")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tensor`
Expected: FAIL (`MatMul` etc. indefinidos).

- [ ] **Step 3: Write the implementation**

Adicionar a `pkg/tensor/ops.go`:

```go
// MatMul: [M,K]x[K,N]->[M,N]; matvec [M,K]x[K]->[M]. Ordem i-k-j (cache).
func (a *Tensor) MatMul(b *Tensor) (*Tensor, error) {
	if len(a.Shape) == 2 && len(b.Shape) == 1 && a.Shape[1] == b.Shape[0] {
		m, k := a.Shape[0], a.Shape[1]
		out := New([]int{m})
		for i := 0; i < m; i++ {
			var s float32
			for p := 0; p < k; p++ {
				s += a.Data[i*k+p] * b.Data[p]
			}
			out.Data[i] = s
		}
		return out, nil
	}
	if len(a.Shape) == 2 && len(b.Shape) == 2 && a.Shape[1] == b.Shape[0] {
		m, k, n := a.Shape[0], a.Shape[1], b.Shape[1]
		out := New([]int{m, n})
		for i := 0; i < m; i++ {
			for p := 0; p < k; p++ {
				aip := a.Data[i*k+p]
				for j := 0; j < n; j++ {
					out.Data[i*n+j] += aip * b.Data[p*n+j]
				}
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("MatMul: dims incompatíveis %v x %v", a.Shape, b.Shape)
}

// Transpose: transposta 2D.
func (a *Tensor) Transpose() (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("Transpose: requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	out := New([]int{n, m})
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			out.Data[j*m+i] = a.Data[i*n+j]
		}
	}
	return out, nil
}

// Reshape: mesma Data, nova forma (produto deve casar).
func (a *Tensor) Reshape(shape []int) (*Tensor, error) {
	if Prod(shape) != a.Size() {
		return nil, fmt.Errorf("Reshape: forma %v incompatível com size %d", shape, a.Size())
	}
	return &Tensor{Shape: copyInts(shape), Data: append([]float32(nil), a.Data...)}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tensor`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tensor/ops.go pkg/tensor/tensor_test.go
git commit -m "tensor: matmul, matvec, transpose, reshape"
```

---

## Task 4: reduções + argmax

**Files:**
- Modify: `pkg/tensor/ops.go`
- Modify: `pkg/tensor/tensor_test.go`

**Interfaces:**
- Consumes: `Tensor`, `New` (Task 1).
- Produces: `(*Tensor).SumAll() float32`; `MeanAll() float32`; `MaxAll() float32`; `(*Tensor).ArgmaxAll() int` (0-based); `(*Tensor).SumAxis(axis int) (*Tensor, error)`; `MeanAxis`, `MaxAxis` (mesma assinatura); `(*Tensor).ArgmaxAxis(axis int) (*Tensor, error)` (índices 0-based como float32). `axis` é 0-based; reduções por eixo só para 2D.

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/tensor/tensor_test.go`:

```go
func TestReduceAll(t *testing.T) {
	a, _ := FromData([]float32{1, 5, 2, 4}, []int{2, 2})
	if a.SumAll() != 12 || a.MaxAll() != 5 || a.MeanAll() != 3 {
		t.Fatalf("reduce all errado: sum=%v max=%v mean=%v", a.SumAll(), a.MaxAll(), a.MeanAll())
	}
	if a.ArgmaxAll() != 1 { // 0-based: o 5 está no offset 1
		t.Fatalf("ArgmaxAll=%d quer 1", a.ArgmaxAll())
	}
}

func TestReduceAxis(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 4, 5, 6}, []int{2, 3})
	s0, _ := a.SumAxis(0) // sobre linhas -> [5,7,9]
	if !ShapeEq(s0.Shape, []int{3}) || s0.Data[0] != 5 || s0.Data[2] != 9 {
		t.Fatalf("SumAxis(0) errado: %v %v", s0.Shape, s0.Data)
	}
	s1, _ := a.SumAxis(1) // sobre colunas -> [6,15]
	if !ShapeEq(s1.Shape, []int{2}) || s1.Data[1] != 15 {
		t.Fatalf("SumAxis(1) errado: %v %v", s1.Shape, s1.Data)
	}
	am, _ := a.ArgmaxAxis(1) // por linha -> [2,2] (0-based)
	if am.Data[0] != 2 || am.Data[1] != 2 {
		t.Fatalf("ArgmaxAxis(1) errado: %v", am.Data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tensor`
Expected: FAIL (`SumAll` etc. indefinidos).

- [ ] **Step 3: Write the implementation**

Adicionar a `pkg/tensor/ops.go`:

```go
func (a *Tensor) SumAll() float32 {
	var s float32
	for _, v := range a.Data {
		s += v
	}
	return s
}
func (a *Tensor) MeanAll() float32 {
	if len(a.Data) == 0 {
		return 0
	}
	return a.SumAll() / float32(len(a.Data))
}
func (a *Tensor) MaxAll() float32 {
	m := a.Data[0]
	for _, v := range a.Data[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// ArgmaxAll devolve o offset (0-based) do máximo global.
func (a *Tensor) ArgmaxAll() int {
	bi := 0
	for i, v := range a.Data {
		if v > a.Data[bi] {
			bi = i
		}
	}
	_ = a.Data[0]
	return bi
}

// reduceAxis2D reduz um tensor 2D ao longo de axis (0 ou 1) com f (acumulador).
func (a *Tensor) reduceAxis2D(axis int, init float32, f func(acc, x float32) float32) (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("redução por eixo requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	switch axis {
	case 0:
		out := New([]int{n})
		for j := 0; j < n; j++ {
			acc := init
			for i := 0; i < m; i++ {
				acc = f(acc, a.Data[i*n+j])
			}
			out.Data[j] = acc
		}
		return out, nil
	case 1:
		out := New([]int{m})
		for i := 0; i < m; i++ {
			acc := init
			for j := 0; j < n; j++ {
				acc = f(acc, a.Data[i*n+j])
			}
			out.Data[i] = acc
		}
		return out, nil
	}
	return nil, fmt.Errorf("axis inválido: %d", axis)
}

func (a *Tensor) SumAxis(axis int) (*Tensor, error) {
	return a.reduceAxis2D(axis, 0, func(acc, x float32) float32 { return acc + x })
}
func (a *Tensor) MaxAxis(axis int) (*Tensor, error) {
	return a.reduceAxis2D(axis, float32(math.Inf(-1)), func(acc, x float32) float32 {
		if x > acc {
			return x
		}
		return acc
	})
}
func (a *Tensor) MeanAxis(axis int) (*Tensor, error) {
	s, err := a.SumAxis(axis)
	if err != nil {
		return nil, err
	}
	var cnt float32
	if axis == 0 {
		cnt = float32(a.Shape[0])
	} else {
		cnt = float32(a.Shape[1])
	}
	return s.MulScalar(1 / cnt), nil
}

// ArgmaxAxis: índices (0-based, como float32) do máximo por linha (axis 1) ou coluna (axis 0).
func (a *Tensor) ArgmaxAxis(axis int) (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("ArgmaxAxis requer 2D, tem %v", a.Shape)
	}
	m, n := a.Shape[0], a.Shape[1]
	switch axis {
	case 0:
		out := New([]int{n})
		for j := 0; j < n; j++ {
			bi := 0
			for i := 1; i < m; i++ {
				if a.Data[i*n+j] > a.Data[bi*n+j] {
					bi = i
				}
			}
			out.Data[j] = float32(bi)
		}
		return out, nil
	case 1:
		out := New([]int{m})
		for i := 0; i < m; i++ {
			bi := 0
			for j := 1; j < n; j++ {
				if a.Data[i*n+j] > a.Data[i*n+bi] {
					bi = j
				}
			}
			out.Data[i] = float32(bi)
		}
		return out, nil
	}
	return nil, fmt.Errorf("axis inválido: %d", axis)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tensor`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tensor/ops.go pkg/tensor/tensor_test.go
git commit -m "tensor: full + axis reductions and argmax"
```

---

## Task 5: ativações + softmax + index-rows

**Files:**
- Modify: `pkg/tensor/ops.go`
- Modify: `pkg/tensor/tensor_test.go`

**Interfaces:**
- Consumes: `Tensor`, `New` (Task 1).
- Produces: `(*Tensor).Exp/Log/Sqrt/Relu/Tanh/Sigmoid/Gelu() *Tensor`; `(*Tensor).Softmax(axis int) (*Tensor, error)` (axis 0-based; 1D usa axis 0, 2D usa axis 1 = por linha); `(*Tensor).IndexRows(idx []int) (*Tensor, error)` (idx 0-based).

- [ ] **Step 1: Write the failing test**

Adicionar a `pkg/tensor/tensor_test.go`:

```go
import "math"

func almost(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

func TestActivations(t *testing.T) {
	a, _ := FromData([]float32{-1, 0, 2}, []int{3})
	r := a.Relu()
	if r.Data[0] != 0 || r.Data[2] != 2 {
		t.Fatalf("Relu errado: %v", r.Data)
	}
	e := a.Exp()
	if !almost(e.Data[1], 1) {
		t.Fatalf("Exp(0)=%v quer 1", e.Data[1])
	}
}

func TestSoftmax(t *testing.T) {
	a, _ := FromData([]float32{1, 2, 3, 1, 2, 3}, []int{2, 3})
	s, err := a.Softmax(1) // por linha
	if err != nil {
		t.Fatal(err)
	}
	var row0 float32
	for j := 0; j < 3; j++ {
		row0 += s.Data[j]
	}
	if !almost(row0, 1) {
		t.Fatalf("softmax linha não soma 1: %v", row0)
	}
	if !(s.Data[2] > s.Data[0]) {
		t.Fatalf("softmax deve favorecer o maior logit")
	}
}

func TestSoftmaxStable(t *testing.T) {
	a, _ := FromData([]float32{1000, 1001, 1002}, []int{3})
	s, _ := a.Softmax(0)
	var sum float32
	for _, v := range s.Data {
		sum += v
		if math.IsNaN(float64(v)) {
			t.Fatal("softmax instável (NaN)")
		}
	}
	if !almost(sum, 1) {
		t.Fatalf("softmax 1D não soma 1: %v", sum)
	}
}

func TestIndexRows(t *testing.T) {
	a, _ := FromData([]float32{10, 11, 20, 21, 30, 31}, []int{3, 2})
	g, err := a.IndexRows([]int{2, 0}) // linhas 2 e 0 (0-based)
	if err != nil {
		t.Fatal(err)
	}
	if !ShapeEq(g.Shape, []int{2, 2}) || g.Data[0] != 30 || g.Data[3] != 11 {
		t.Fatalf("IndexRows errado: %v %v", g.Shape, g.Data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/tensor`
Expected: FAIL (`Relu`/`Softmax`/`IndexRows` indefinidos).

- [ ] **Step 3: Write the implementation**

Adicionar a `pkg/tensor/ops.go` (remova a linha `var _ = math.Exp` da Task 2, agora `math` é usado de verdade):

```go
func (a *Tensor) unary(f func(x float32) float32) *Tensor {
	out := New(a.Shape)
	for i, v := range a.Data {
		out.Data[i] = f(v)
	}
	return out
}

func f32(f func(float64) float64) func(float32) float32 {
	return func(x float32) float32 { return float32(f(float64(x))) }
}

func (a *Tensor) Exp() *Tensor  { return a.unary(f32(math.Exp)) }
func (a *Tensor) Log() *Tensor  { return a.unary(f32(math.Log)) }
func (a *Tensor) Sqrt() *Tensor { return a.unary(f32(math.Sqrt)) }
func (a *Tensor) Tanh() *Tensor { return a.unary(f32(math.Tanh)) }
func (a *Tensor) Relu() *Tensor {
	return a.unary(func(x float32) float32 {
		if x > 0 {
			return x
		}
		return 0
	})
}
func (a *Tensor) Sigmoid() *Tensor {
	return a.unary(func(x float32) float32 { return float32(1 / (1 + math.Exp(-float64(x)))) })
}
func (a *Tensor) Gelu() *Tensor {
	const c = 0.7978845608 // sqrt(2/pi)
	return a.unary(func(x float32) float32 {
		xf := float64(x)
		return float32(0.5 * xf * (1 + math.Tanh(c*(xf+0.044715*xf*xf*xf))))
	})
}

// Softmax estável ao longo de axis. 1D: axis 0 (todo o vetor). 2D: axis 1 (por linha).
func (a *Tensor) Softmax(axis int) (*Tensor, error) {
	if len(a.Shape) == 1 && axis == 0 {
		out := New(a.Shape)
		mx := a.MaxAll()
		var sum float32
		for i, v := range a.Data {
			e := float32(math.Exp(float64(v - mx)))
			out.Data[i] = e
			sum += e
		}
		for i := range out.Data {
			out.Data[i] /= sum
		}
		return out, nil
	}
	if len(a.Shape) == 2 && axis == 1 {
		m, n := a.Shape[0], a.Shape[1]
		out := New(a.Shape)
		for i := 0; i < m; i++ {
			mx := a.Data[i*n]
			for j := 1; j < n; j++ {
				if a.Data[i*n+j] > mx {
					mx = a.Data[i*n+j]
				}
			}
			var sum float32
			for j := 0; j < n; j++ {
				e := float32(math.Exp(float64(a.Data[i*n+j] - mx)))
				out.Data[i*n+j] = e
				sum += e
			}
			for j := 0; j < n; j++ {
				out.Data[i*n+j] /= sum
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("Softmax: combinação forma %v / axis %d não suportada", a.Shape, axis)
}

// IndexRows colhe linhas (idx 0-based) de um tensor 2D [R,C] -> [len(idx),C].
func (a *Tensor) IndexRows(idx []int) (*Tensor, error) {
	if len(a.Shape) != 2 {
		return nil, fmt.Errorf("IndexRows: requer 2D, tem %v", a.Shape)
	}
	r, c := a.Shape[0], a.Shape[1]
	out := New([]int{len(idx), c})
	for k, ix := range idx {
		if ix < 0 || ix >= r {
			return nil, fmt.Errorf("IndexRows: linha %d fora de faixa (0..%d)", ix, r-1)
		}
		copy(out.Data[k*c:(k+1)*c], a.Data[ix*c:(ix+1)*c])
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/tensor`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/tensor/ops.go pkg/tensor/tensor_test.go
git commit -m "tensor: activations, stable softmax, index-rows"
```

---

## Task 6: classe `Tensor` na VM + fixture da API

**Files:**
- Create: `pkg/vm/tensor_native.go`
- Modify: `pkg/compiler/codegen.go` (`builtinClasses` ~1077)
- Modify: `pkg/vm/vm.go` (switch `OP_NEW_INSTANCE` ~1386; `callNativeMethod` ~1240)
- Test: `tests/tensor_test.prw`

**Interfaces:**
- Consumes: todo o `pkg/tensor` (Tasks 1-5); padrão `newGridObject`/`callGridProcessMethod` (`pkg/vm/grid.go`); `getArg` (`pkg/vm/natives.go`).
- Produces: classe AdvPL `Tensor` com `New(aShape)`, `FromArray(aData,aShape)`, `Rand(aShape,nEscala)`, e métodos `Shape/Size/Get/Set/ToArray/Add/Sub/Mul/Div/AddScalar/MulScalar/MatMul/Transpose/Reshape/Sum/Mean/Max/Argmax/Exp/Log/Sqrt/Relu/Tanh/Sigmoid/Gelu/Softmax/IndexRows`.

- [ ] **Step 1: Write the failing test**

Criar `tests/tensor_test.prw`:

```advpl
User Function TensorTst()
    Local oA := Tensor():FromArray({1,2,3,4}, {2,2})
    Local oB := Tensor():FromArray({5,6,7,8}, {2,2})
    Local oC := oA:MatMul(oB)            // [[19,22],[43,50]]
    Local aC := oC:ToArray()
    Local oBias := Tensor():FromArray({10,20}, {2})   // broadcast por linha
    Local oS := oC:Add(oBias):Softmax(2) // softmax por linha (eixo 2)
    Local nPred := oC:Argmax()           // maior valor global -> offset 1-based
    Local nFail := 0

    If aC[1] != 19 .Or. aC[4] != 50
        ConOut("FALHA MatMul: " + Str(aC[1]) + "," + Str(aC[4])); nFail++
    EndIf
    If Abs(oS:ToArray()[1] + oS:ToArray()[2] - 1) > 0.001
        ConOut("FALHA Softmax linha nao soma 1"); nFail++
    EndIf
    If nPred != 4                        // 50 é o maior, offset row-major 1-based = 4
        ConOut("FALHA Argmax global: " + Str(nPred)); nFail++
    EndIf
    // erro de forma é capturável
    Begin Sequence
        oA:MatMul(Tensor():New({3,3}))
        ConOut("FALHA: matmul incompativel nao lancou"); nFail++
    Recover
    End Sequence

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build -o advplc ./cmd/advplc && ./advplc run tests/tensor_test.prw`
Expected: FAIL — `Tensor()` não é classe reconhecida (erro de compilação/execução).

- [ ] **Step 3: Registrar a classe builtin (3 pontos)**

Em `pkg/compiler/codegen.go`, no mapa `builtinClasses`, adicionar a linha:

```go
	"TENSOR":        true,
```

Em `pkg/vm/vm.go`, no `switch upperName` do `OP_NEW_INSTANCE` (junto de `case "FWGRIDPROCESS":`), adicionar:

```go
		case "TENSOR":
			v.push(newTensorObject())
			return nil
```

Em `pkg/vm/vm.go`, no `callNativeMethod`, no `switch obj.ClassName` (junto de `case "FWGridProcess":`), adicionar:

```go
	case "Tensor":
		return v.callTensorMethod(obj, upperMethod, args)
```

- [ ] **Step 4: Criar a ligação com a VM**

Criar `pkg/vm/tensor_native.go`:

```go
package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tensor"
)

func newTensorObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Tensor", nil)
	obj.Native = &tensor.Tensor{Shape: []int{0}, Data: []float32{}}
	return obj
}

func wrapTensor(t *tensor.Tensor) *advplrt.ObjectValue {
	obj := advplrt.NewObject("Tensor", nil)
	obj.Native = t
	return obj
}

// shapeFromArg lê um array AdvPL de inteiros como []int.
func shapeFromArg(val advplrt.Value) []int {
	a, ok := val.(*advplrt.ArrayValue)
	if !ok {
		return nil
	}
	out := make([]int, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = int(advplrt.ToFloat(e))
	}
	return out
}

// floatsFromArg lê um array AdvPL de números como []float32.
func floatsFromArg(val advplrt.Value) []float32 {
	a, ok := val.(*advplrt.ArrayValue)
	if !ok {
		return nil
	}
	out := make([]float32, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = float32(advplrt.ToFloat(e))
	}
	return out
}

func intsToAdvplArray(xs []int) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(xs))
	for i, x := range xs {
		el[i] = advplrt.NewNumber(float64(x))
	}
	return advplrt.NewArray(el)
}

func floatsToAdvplArray(xs []float32) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(xs))
	for i, x := range xs {
		el[i] = advplrt.NewNumber(float64(x))
	}
	return advplrt.NewArray(el)
}

// argTensor lê o *tensor.Tensor de um argumento que deve ser um objeto Tensor.
func argTensor(args []advplrt.Value, i int) (*tensor.Tensor, error) {
	o, ok := getArg(args, i).(*advplrt.ObjectValue)
	if !ok {
		return nil, advplrt.NewError("Tensor: argumento não é um objeto Tensor")
	}
	t, ok := o.Native.(*tensor.Tensor)
	if !ok {
		return nil, advplrt.NewError("Tensor: objeto sem estado interno de tensor")
	}
	return t, nil
}

// terr converte um erro de kernel num ErrorValue catchável.
func terr(err error) error { return advplrt.NewError("Tensor: " + err.Error()) }

// axisArg lê nAxis (1-based) e devolve 0-based, e se foi informado.
func axisArg(args []advplrt.Value, i int) (axis int, given bool) {
	if i >= len(args) {
		return 0, false
	}
	if _, ok := getArg(args, i).(*advplrt.NumberValue); !ok {
		return 0, false
	}
	return int(advplrt.ToFloat(getArg(args, i))) - 1, true
}

func (v *VM) callTensorMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	t, _ := obj.Native.(*tensor.Tensor)

	switch method {
	case "NEW":
		obj.Native = tensor.New(shapeFromArg(getArg(args, 0)))
		v.push(obj)
	case "FROMARRAY":
		nt, err := tensor.FromData(floatsFromArg(getArg(args, 0)), shapeFromArg(getArg(args, 1)))
		if err != nil {
			return terr(err)
		}
		obj.Native = nt
		v.push(obj)
	case "RAND":
		scale := float32(1)
		if _, ok := getArg(args, 1).(*advplrt.NumberValue); ok {
			scale = float32(advplrt.ToFloat(getArg(args, 1)))
		}
		obj.Native = tensor.Rand(shapeFromArg(getArg(args, 0)), scale)
		v.push(obj)

	case "SHAPE":
		v.push(intsToAdvplArray(t.Shape))
	case "SIZE":
		v.push(advplrt.NewNumber(float64(t.Size())))
	case "TOARRAY":
		v.push(floatsToAdvplArray(t.Data))
	case "GET":
		val, err := t.At(idxFromArg(getArg(args, 0)))
		if err != nil {
			return terr(err)
		}
		v.push(advplrt.NewNumber(float64(val)))
	case "SET":
		if err := t.SetAt(idxFromArg(getArg(args, 0)), float32(advplrt.ToFloat(getArg(args, 1)))); err != nil {
			return terr(err)
		}
		v.push(obj)

	case "ADD", "SUB", "MUL", "DIV":
		b, err := argTensor(args, 0)
		if err != nil {
			return err
		}
		var r *tensor.Tensor
		switch method {
		case "ADD":
			r, err = t.Add(b)
		case "SUB":
			r, err = t.Sub(b)
		case "MUL":
			r, err = t.Mul(b)
		case "DIV":
			r, err = t.Div(b)
		}
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "ADDSCALAR":
		v.push(wrapTensor(t.AddScalar(float32(advplrt.ToFloat(getArg(args, 0))))))
	case "MULSCALAR":
		v.push(wrapTensor(t.MulScalar(float32(advplrt.ToFloat(getArg(args, 0))))))

	case "MATMUL":
		b, err := argTensor(args, 0)
		if err != nil {
			return err
		}
		r, err := t.MatMul(b)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "TRANSPOSE":
		r, err := t.Transpose()
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "RESHAPE":
		r, err := t.Reshape(shapeFromArg(getArg(args, 0)))
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "SUM", "MEAN", "MAX", "ARGMAX":
		axis, given := axisArg(args, 0)
		if !given {
			switch method {
			case "SUM":
				v.push(advplrt.NewNumber(float64(t.SumAll())))
			case "MEAN":
				v.push(advplrt.NewNumber(float64(t.MeanAll())))
			case "MAX":
				v.push(advplrt.NewNumber(float64(t.MaxAll())))
			case "ARGMAX":
				v.push(advplrt.NewNumber(float64(t.ArgmaxAll() + 1))) // 1-based
			}
			return nil
		}
		var r *tensor.Tensor
		var err error
		switch method {
		case "SUM":
			r, err = t.SumAxis(axis)
		case "MEAN":
			r, err = t.MeanAxis(axis)
		case "MAX":
			r, err = t.MaxAxis(axis)
		case "ARGMAX":
			r, err = t.ArgmaxAxis(axis)
			if err == nil { // 1-based na saída
				r = r.AddScalar(1)
			}
		}
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "EXP":
		v.push(wrapTensor(t.Exp()))
	case "LOG":
		v.push(wrapTensor(t.Log()))
	case "SQRT":
		v.push(wrapTensor(t.Sqrt()))
	case "RELU":
		v.push(wrapTensor(t.Relu()))
	case "TANH":
		v.push(wrapTensor(t.Tanh()))
	case "SIGMOID":
		v.push(wrapTensor(t.Sigmoid()))
	case "GELU":
		v.push(wrapTensor(t.Gelu()))

	case "SOFTMAX":
		axis, given := axisArg(args, 0)
		if !given {
			axis = len(t.Shape) - 1 // última dim, 0-based
		}
		r, err := t.Softmax(axis)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "INDEXROWS":
		idx := shapeFromArg(getArg(args, 0)) // reusa leitura de ints
		for i := range idx {
			idx[i]-- // 1-based -> 0-based
		}
		r, err := t.IndexRows(idx)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	default:
		return advplrt.NewError("Tensor: método desconhecido " + method)
	}
	return nil
}

// idxFromArg lê um índice multi-dim AdvPL (1-based) como []int 0-based.
func idxFromArg(val advplrt.Value) []int {
	xs := shapeFromArg(val)
	for i := range xs {
		xs[i]--
	}
	return xs
}
```

- [ ] **Step 5: Rebuild e rodar o fixture**

Run: `go build -o advplc ./cmd/advplc && ./advplc run tests/tensor_test.prw`
Expected:
```
OK: 3/3 verificacoes passaram.
```

- [ ] **Step 6: Regressão**

Run: `go test ./... 2>&1 | grep -vE '^ok|no test files'`
Expected: sem saída.
Run: `./advplc run pt_nn.prw 2>&1 | tail -1`
Expected: `OK: 3/3 verificacoes passaram.`

- [ ] **Step 7: Commit**

```bash
git add pkg/tensor/ pkg/vm/tensor_native.go pkg/vm/vm.go pkg/compiler/codegen.go tests/tensor_test.prw
git commit -m "vm: Tensor class bound to pkg/tensor kernels"
```

---

## Task 7: aceite (MLP float) + docs

**Files:**
- Create: `tests/mlp_demo.prw`
- Modify: `README.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: a classe `Tensor` completa (Task 6).

- [ ] **Step 1: Escrever a demo de aceite (MLP forward)**

Criar `tests/mlp_demo.prw`:

```advpl
// MLP float pequeno: entrada X[1,2] -> Linear(2x2)+bias -> Relu -> Linear(2x2)+bias -> Softmax -> Argmax.
// Pesos fixos; resultado conferido contra cálculo manual.
User Function MlpDemo()
    Local oX  := Tensor():FromArray({1, 2}, {1, 2})
    Local oW1 := Tensor():FromArray({1, 0, 0, 1}, {2, 2})   // identidade
    Local ob1 := Tensor():FromArray({0, -3}, {2})           // bias
    Local oW2 := Tensor():FromArray({1, 0, 0, 1}, {2, 2})   // identidade
    Local ob2 := Tensor():FromArray({0, 0}, {2})
    Local oH, oY, nPred, aY, nFail := 0

    // h = relu(X·W1 + b1) = relu([1,2] + [0,-3]) = relu([1,-1]) = [1,0]
    oH := oX:MatMul(oW1):Add(ob1):Relu()
    // y = softmax(h·W2 + b2) = softmax([1,0])
    oY := oH:MatMul(oW2):Add(ob2):Softmax(2)
    aY := oY:ToArray()
    nPred := oY:Argmax()          // maior prob -> classe 1 (offset 1-based = 1)

    If Abs(oH:ToArray()[1] - 1) > 0.001 .Or. Abs(oH:ToArray()[2] - 0) > 0.001
        ConOut("FALHA camada oculta: " + Str(oH:ToArray()[1]) + "," + Str(oH:ToArray()[2])); nFail++
    EndIf
    // softmax([1,0]) = [e/(e+1), 1/(e+1)] ~ [0.731, 0.269]
    If Abs(aY[1] - 0.7311) > 0.001
        ConOut("FALHA softmax: " + Str(aY[1])); nFail++
    EndIf
    If nPred != 1
        ConOut("FALHA argmax: " + Str(nPred)); nFail++
    EndIf

    ConOut("MLP forward: h=[" + Str(oH:ToArray()[1],3,1) + "," + Str(oH:ToArray()[2],3,1) + "]" + ;
           " y=[" + Str(aY[1],5,3) + "," + Str(aY[2],5,3) + "] pred=" + Str(nPred,1))
    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
```

- [ ] **Step 2: Rodar a demo**

Run: `./advplc run tests/mlp_demo.prw`
Expected:
```
MLP forward: h=[1.0,0.0] y=[0.731,0.269] pred=1
OK: 3/3 verificacoes passaram.
```

- [ ] **Step 3: Atualizar o README**

Em `README.md`, na lista de "## Recursos", adicionar a linha:

```markdown
- **Núcleo de Tensor (float32)**: classe `Tensor` acelerada em Go (`pkg/tensor`) — `MatMul`, elementwise com broadcast, reduções, ativações, `Softmax`, `Argmax`, `IndexRows` — para construir e rodar modelos float com o AdvPL orquestrando; ver [Núcleo de Tensor](#núcleo-de-tensor)
```

E adicionar, antes da seção "## Exemplos de IA em AdvPL puro", uma seção nova:

```markdown
## Núcleo de Tensor

A classe `Tensor` (float32) guarda os dados como `[]float32` plano em Go — fora da
representação *boxed* de `Value` — e roda kernels de forward em Go puro. O AdvPL
orquestra; o Go faz a conta.

```advpl
Local oX  := Tensor():FromArray({1,2}, {1,2})
Local oW  := Tensor():Rand({2,3}, 0.1)
Local oH  := oX:MatMul(oW):Relu()          // [1,3]
Local oY  := oH:Softmax(2)                  // softmax por linha
Local nId := oY:Argmax()                    // classe prevista (1-based)
```

Construtores: `New(aForma)`, `FromArray(aDados, aForma)`, `Rand(aForma, nEscala)`.
Métodos: `Shape`, `Size`, `Get`/`Set`, `ToArray`; `Add`/`Sub`/`Mul`/`Div` (com
broadcast de escalar e linha/coluna), `AddScalar`/`MulScalar`; `MatMul`,
`Transpose`, `Reshape`; `Sum`/`Mean`/`Max`/`Argmax` (sem eixo → número; com eixo →
Tensor); `Exp`/`Log`/`Sqrt`/`Relu`/`Tanh`/`Sigmoid`/`Gelu`; `Softmax`; `IndexRows`
(lookup de embedding). Erros de forma são capturáveis por `Try/Catch`.

Este ciclo entrega o **forward** (inferência). Autodiff/treino é um ciclo futuro.
```

- [ ] **Step 4: Atualizar o CHANGELOG**

Em `CHANGELOG.md`, logo após `Todas as mudanças notáveis deste projeto são documentadas aqui.`, adicionar:

```markdown
## [Não lançado]

### Núcleo de Tensor (Sub-projeto 2, forward)

- Classe **`Tensor`** (float32) acelerada em Go (`pkg/tensor`): dados `[]float32`
  planos fora da representação *boxed* de `Value`, com kernels de forward em Go
  puro — `MatMul`/matvec, elementwise com broadcast limitado (escalar, linha,
  coluna), `Transpose`, `Reshape`, reduções (`Sum`/`Mean`/`Max`/`Argmax`, com e
  sem eixo), ativações (`Exp`/`Log`/`Sqrt`/`Relu`/`Tanh`/`Sigmoid`/`Gelu`),
  `Softmax` estável e `IndexRows` (embedding). O AdvPL orquestra; o Go faz a conta.
  Ligada à VM no padrão de classe nativa (`ObjectValue.Native`); erros de forma são
  `ErrorValue` capturáveis por `Try/Catch`. Aceite: `tests/mlp_demo.prw` roda o
  forward de um MLP float. Autodiff/treino fica para um ciclo futuro.
```

- [ ] **Step 5: Commit**

```bash
git add tests/mlp_demo.prw README.md CHANGELOG.md
git commit -m "tensor: MLP forward acceptance demo + docs"
```

---

## Notas de verificação final

Ao término das 7 tasks:
- `go test ./...` verde (inclui `pkg/tensor`).
- `./advplc run tests/tensor_test.prw` e `tests/mlp_demo.prw` → `OK: 3/3`.
- `pt_nn.prw`/`pt_chat.prw`/`pt_llm.prw` sem regressão.
- Os 5 critérios de aceite da spec satisfeitos.

Publicação de release (tag/CI) é decisão do usuário — fora deste plano.
