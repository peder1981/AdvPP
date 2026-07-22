# Sub-projeto 6b — Álgebra linear (float64) no kernel AdvPP

**Data:** 2026-07-22
**Status:** Aprovado (design; segue o roadmap do kernel matemático, sobre o S6a)
**Contexto maior:** 2º dos quatro ciclos do kernel matemático (S6a Tensor f64 ✓ →
**S6b álgebra linear** → S6c geometria → S6d aritmética/estatística). O maior buraco:
o Tensor só tinha `MatMul`/`Transpose`.

## Motivação

Cálculo "funcional" de verdade exige resolver sistemas, inverter matrizes, calcular
determinantes e autovalores. Tudo isso em **float64** (S6a) para não acumular erro.
São operações **não-diferenciáveis** (cálculo, não treino) — não usam o autograd.

## Objetivos (escopo)

Métodos sobre `*tensor.Tensor` (matriz quadrada `[n,n]` salvo indicado), operando em
**float64** (promove/converte a f64 internamente; saída f64):

- **`LU()`** → `{L, U, P, sign}`: decomposição LU com pivotamento parcial (base dos demais).
- **`Det()`** `float64`: determinante via LU (produto da diagonal de U × sinal).
- **`Solve(b)`** → `x`: resolve `A·x = b` (b vetor `[n]` ou matriz `[n,k]`) por
  substituição direta/reversa sobre a LU.
- **`Inv()`** → `A⁻¹`: inversa resolvendo `A·X = I`.
- **`QR()`** → `{Q, R}`: decomposição QR por Householder (`A[m,n]`, m≥n; Q ortogonal, R triangular superior).
- **`EigSym()`** → `{valores[n], vetores[n,n]}`: autovalores/autovetores de matriz
  **simétrica** por rotações de Jacobi cíclicas (colunas de `vetores` = autovetores).

## Não-objetivos (follow-up documentado)

- **SVD** e **autovalores de matriz não-simétrica** (QR-iteration) — os maiores lifts
  numéricos; ficam para um ciclo posterior (S6b-ext) se houver necessidade.
- Matrizes esparsas, decomposição de Cholesky, pseudo-inversa, mínimos quadrados
  (derivam de QR/SVD; adicionáveis depois).
- Autograd sobre estas ops (não-diferenciáveis).

## Arquitetura

Novo arquivo `pkg/tensor/linalg.go`. Cada método valida forma (quadrada onde exigido),
converte a f64 (`AsDType(Float64)`) e devolve tensores f64. Algoritmos clássicos:

- **LU (Doolittle, pivô parcial):** decompõe `P·A = L·U`. Guarda numa matriz de
  trabalho + vetor de permutação `P` + sinal das trocas. Erro se singular (pivô ~0).
- **Det:** `sign · Π U[i][i]`.
- **Solve:** `L·y = P·b` (substituição direta), `U·x = y` (substituição reversa). Para
  b matriz `[n,k]`, resolve coluna a coluna.
- **Inv:** `Solve(I)`.
- **QR (Householder):** aplica refletores de Householder para zerar abaixo da diagonal;
  acumula `Q`. Valida `m≥n`.
- **EigSym (Jacobi):** exige `A` simétrica (tolerância). Rotações de Jacobi zeram o maior
  off-diagonal iterativamente até convergir; acumula os autovetores. Ordena por autovalor desc.

Tolerâncias e limites de iteração como constantes nomeadas. Erros → `error` (Go) /
`ErrorValue` (VM).

## API AdvPL / VM (`pkg/vm/tensor_native.go`)

- `oA:Det()` → número. `oA:Solve(oB)` → Tensor. `oA:Inv()` → Tensor.
- `oA:LU()` → array `{oL, oU, oP}` (P como matriz de permutação f64).
- `oA:QR()` → array `{oQ, oR}`. `oA:EigSym()` → array `{oValores, oVetores}`.
- Tudo devolve/opera em Tensor float64. Erros (não-quadrada, singular, não-simétrica,
  dims incompatíveis) são `ErrorValue` capturáveis por `Try/Catch`.

## Testes e critérios de aceite

1. **`go test ./pkg/tensor`** (novo `linalg_test.go`):
   - `Det`: casos conhecidos (2×2, 3×3; identidade→1; singular→0).
   - `Solve`: resíduo `‖A·x − b‖` ~0; contra solução conhecida.
   - `Inv`: `A·A⁻¹ ≈ I` (dentro de tolerância f64); singular → erro.
   - `QR`: `Q·R ≈ A` e `Qᵀ·Q ≈ I`.
   - `EigSym`: `A·v ≈ λ·v` para cada par; soma dos autovalores = traço; matriz
     não-simétrica → erro.
2. **`tests/linalg_test.prw`**: `Det`/`Solve`/`Inv`/`QR`/`EigSym` de matrizes pequenas;
   inversa vezes original ≈ identidade; erros capturáveis.
3. **Regressão**: `go test ./...` verde; f32/ML e modelos `tests/llm/` sem regressão.

## Ordem de implementação

1. LU (pivô parcial) + `Det` + `Solve` + `Inv` + `go test`.
2. `QR` (Householder) + `go test`.
3. `EigSym` (Jacobi) + `go test`.
4. Ligação VM (Det/Solve/Inv/LU/QR/EigSym) + `tests/linalg_test.prw`.
5. Docs (README + CHANGELOG).
