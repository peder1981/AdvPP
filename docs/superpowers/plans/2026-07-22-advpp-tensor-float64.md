# Tensor float64 (S6a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: subagent-driven-development ou executing-plans.

**Goal:** dtype selecionável (float32 default / float64) no kernel de Tensor, base do kernel matemático. ML segue em f32 intocado.

**Architecture:** `Tensor` ganha `DType` + `Data64`; ops-alvo fazem dispatch no topo (caminho f32 byte-idêntico, caminho f64 novo). VM expõe dtype. Autograd/nn f64 adiado.

## Global Constraints

- Caminho f32 **byte-idêntico** — `go test ./pkg/tensor ./pkg/autograd ./pkg/nn` e `tests/llm/*` sem regressão.
- Assinaturas de `New`/`FromData`/`Rand` inalteradas (sem ripple no ML).
- Erros de forma → `error` (Go) / `ErrorValue` (VM, via `terr`).

---

## Task 1: infra de dtype (`pkg/tensor/tensor.go`)

**Files:** Modify `pkg/tensor/tensor.go`, `pkg/tensor/tensor_test.go`.

- [ ] **Step 1** Teste: criar `NewDType({2,2},Float64)` → `DType==Float64`, `Size()==4`; `Set(0,1.5)`+`Get(0)==1.5`; `FromData64([]float64{1,2,3},{3})` ok e mismatch erra; `At/SetAt` funcionam em f64.
- [ ] **Step 2** Rodar → falha.
- [ ] **Step 3** Implementar: `type DType int; const (Float32 DType=iota; Float64)`; add `Data64 []float64` e `DType` ao struct; `NewDType`, `FromData64`, `RandDType`; `Size()` usa slice ativo; `Get(i)/Set(i,v)` (float64, slice ativo); `At/SetAt` via `Get/Set`; `SameDType(a,b)`, `AsDType(dt)`. `New/FromData/Rand` inalterados (default Float32, `DType` zero-value = Float32).
- [ ] **Step 4** `go test ./pkg/tensor` → PASS (novos + antigos).
- [ ] **Step 5** Commit `tensor: DType + Data64 + acessores neutros`.

## Task 2: ops f64 com dispatch (`pkg/tensor/ops.go`)

**Files:** Modify `pkg/tensor/ops.go`, `pkg/tensor/tensor_test.go`.

- [ ] **Step 1** Teste: em f64 — elementwise `Add/Mul`, `MatMul`, `Transpose`, `Reshape`, `Dot`, `Norm` corretos; **precisão**: soma de Kahan-style (1 + 1e8·1e-8) ou `Dot` com magnitudes díspares tem erro menor em f64 que f32; MatMul f64×f32 promove a f64.
- [ ] **Step 2** Rodar → falha (Dot/Norm/f64 paths).
- [ ] **Step 3** Implementar dispatch no topo das ops-alvo: `if DType==Float64 {...matXX64...}` + corpo f32 intocado abaixo. Kernels f64 sobre `Data64`. `Dot(b)`/`Norm()` novos (f32 e f64). `Reshape` só move dados (preserva dtype).
- [ ] **Step 4** `go test ./pkg/tensor ./pkg/autograd ./pkg/nn` → PASS.
- [ ] **Step 5** Commit `tensor: ops float64 (add/mul/matmul/transpose/dot/norm) por dispatch`.

## Task 3: ligação VM + fixture

**Files:** Modify `pkg/vm/tensor_native.go`; Create `tests/tensor_f64_test.prw`.

- [ ] **Step 1** Fixture: `Tensor():New({2,2},"float64")`, `:DType()=="float64"`, aritmética+MatMul, `:ToFloat32()`/`:ToFloat64()` convertem, forma inválida capturável.
- [ ] **Step 2** Rebuild+rodar → falha.
- [ ] **Step 3** VM: `New(aShape[,cDType])` (parse "float64"→Float64), `oT:DType()`, `oT:ToFloat32()/ToFloat64()`, `FromArray(...[,cDType])`. `callTensorMethod` novos cases.
- [ ] **Step 4** `go build` + `./advplc run tests/tensor_f64_test.prw` → OK.
- [ ] **Step 5** Regressão: `go test ./...`; `advplc check tests/*.prw tests/llm/*.prw` (real_protheus pré-existente ok ignorar).
- [ ] **Step 6** Commit `vm: dtype no Tensor (New/DType/ToFloatXX) + fixture`.

## Task 4: docs
- [ ] README seção Núcleo de Tensor: nota sobre dtype float64 selecionável. CHANGELOG `[Não lançado]` S6a. Commit.

## Verificação final
- `go test ./...` verde; f32 sem regressão; fixture OK; 4 critérios da spec.
