# Sub-projeto 6a — Tensor float64 (dtype selecionável) no kernel AdvPP

**Data:** 2026-07-22
**Status:** Aprovado (design; decisões do usuário: dtype selecionável, objetivo ML+científico geral)
**Contexto maior:** primeiro dos quatro ciclos do kernel matemático (S6a Tensor float64 →
S6b álgebra linear → S6c geometria espacial → S6d aritmética/estatística). É a **base**:
álgebra e geometria exatas exigem dupla precisão; o ML rápido segue em float32.

## Motivação

O kernel de Tensor é float32 (escolha de velocidade para ML). Cálculos de álgebra linear
(inversa, determinante, solução de sistemas) e geometria acumulam erro em float32. Este
ciclo adiciona **precisão dupla selecionável por tensor** — mantendo o default float32
(ML intocado, rápido) e habilitando float64 sob demanda para as camadas de cálculo.

## Decisão de escopo (explícita)

- **dtype selecionável por tensor** (não trocar tudo para f64): `float32` default, `float64`
  sob demanda. O stack de ML (autograd/nn) permanece float32.
- **Propagação de float64 pelo autograd/nn é ADIADA (não-objetivo aqui).** Álgebra linear
  e geometria (S6b/S6c) são **não-diferenciáveis** — são cálculo, não treino — logo não
  usam o autograd. Um Variable/treino em f64 pode vir depois, se houver necessidade real.

## Objetivos (escopo)

- `pkg/tensor`: campo **`DType`** (`Float32`/`Float64`) no `Tensor`, com armazenamento f64
  (`Data64 []float64`) quando f64; acessores neutros de dtype (`Get(i) float64`,
  `Set(i, float64)`, `Size()`); construtores com dtype.
- **Ops fundamentais em f64** (base para S6b/S6c): elementwise `Add/Sub/Mul/Div` (com o
  mesmo broadcast do f32), `MatMul`, `Transpose`, `Reshape`, mais `Dot` (produto interno)
  e `Norm` (norma L2). Dispatch no topo de cada método por `DType`: o caminho f32 existente
  fica **byte-idêntico** (risco zero para o ML); o caminho f64 é adicionado.
- **VM**: `Tensor():New(aShape [, cDType])` (`"float32"` default, `"float64"`);
  `oT:DType()` → string; as ops existentes passam a respeitar o dtype; conversão
  `oT:ToFloat64()`/`oT:ToFloat32()`.
- Testes Go (precisão f64 vs f32 num cálculo sensível) + fixture AdvPL.

## Não-objetivos (YAGNI / ciclos seguintes)

- Autograd/nn em f64 (adiado, ver acima). Inversa/det/solve/eig/SVD (S6b). Geometria (S6c).
- Novas natives aritméticas/estatística (S6d). Tipos decimal/precisão arbitrária.
- Reescrever as ops f32 existentes (ficam intocadas; só ganham o dispatch no topo).

## Arquitetura

### `pkg/tensor/tensor.go`

```go
type DType int
const (Float32 DType = iota; Float64)

type Tensor struct {
    Shape []int
    Data   []float32 // usado quando DType==Float32
    Data64 []float64 // usado quando DType==Float64
    DType  DType
}
```
- `New(shape)`, `FromData(data []float32, shape)`, `Rand(shape, scale)` — **inalterados**
  (float32; assinaturas idênticas → sem ripple no ML).
- Novos: `NewDType(shape []int, dt DType)`, `FromData64(data []float64, shape []int)`,
  `RandDType(shape, scale, dt)`.
- `Size()` retorna o len do slice ativo. `Get(i int) float64` / `Set(i int, v float64)`
  leem/escrevem o slice ativo convertendo para/de float64 (dtype-neutro). `At/SetAt`
  (por índice multi-dim) passam a usar Get/Set.
- Helpers: `SameDType(a, b) DType` (promove para f64 se qualquer um for f64),
  `AsDType(dt) *Tensor` (converte/copia).

### `pkg/tensor/ops.go` (dispatch por dtype)

Cada op-alvo começa com um dispatch:
```go
func (a *Tensor) MatMul(b *Tensor) (*Tensor, error) {
    if a.DType == Float64 || b.DType == Float64 {
        return a.matMul64(b) // caminho f64 novo
    }
    // ... corpo f32 EXISTENTE, intocado ...
}
```
Ops com caminho f64 novo: `Add/Sub/Mul/Div` (elementwise+broadcast), `MatMul`,
`Transpose`, `Reshape` (independe de dtype — só move dados), `Dot`, `Norm`. Os
kernels f64 espelham a lógica f32 sobre `Data64`.

### VM (`pkg/vm/tensor_native.go`)

- `Tensor():New(aShape [, cDType])`: `cDType` opcional (`"float64"`/`"float32"`).
- `oT:DType()` → `"float32"`/`"float64"`. `oT:ToFloat64()` / `oT:ToFloat32()` → novo Tensor.
- `oT:FromArray(aData, aShape [, cDType])` idem. Métodos existentes inalterados na assinatura.
- `wrapTensor`/`floatsFromArg` continuam f32; a leitura de dados de f64 via `ToArray()`
  converte para número AdvPL (float64) naturalmente.

## Testes e critérios de aceite

1. **`go test ./pkg/tensor`**: (a) criar f64, `Get/Set`, `Size`, `DType` corretos; (b)
   `Add/Mul/MatMul/Transpose/Dot/Norm` em f64 dão o resultado certo; (c) **precisão**: um
   cálculo sensível (ex.: somar 1e8 vezes 1e-8, ou `Dot` de vetores com magnitudes
   dispares) tem erro menor em f64 que em f32; (d) MatMul com um operando f64 promove a f64.
2. **Regressão f32**: os testes existentes de `pkg/tensor`, `pkg/autograd`, `pkg/nn`
   passam inalterados (caminho f32 byte-idêntico). `go test ./...` verde.
3. **`tests/tensor_f64_test.prw`**: `Tensor():New({2,2},"float64")`, aritmética + matmul,
   `:DType()` == "float64", `ToFloat32/64` convertem; erro de forma → `ErrorValue`.
4. Modelos de `tests/llm/` sem regressão (`advplc check`).

## Ordem de implementação

1. `DType` + `Data64` + acessores (`Get/Set/Size/At/SetAt`) + construtores + `go test`.
2. Ops f64 com dispatch (`Add/Sub/Mul/Div/MatMul/Transpose/Reshape/Dot/Norm`) + grad/precisão test.
3. Ligação VM (`New` com dtype, `DType`, `ToFloat64/32`) + fixture.
4. Docs (README seção Tensor + CHANGELOG).
