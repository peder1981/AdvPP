# Álgebra linear (S6b) Implementation Plan

**Goal:** LU/Det/Solve/Inv + QR + EigSym (float64) sobre Tensor, em `pkg/tensor/linalg.go`.

**Global Constraints:** float64 (converte via AsDType); não-diferenciável; erros → `error`/`ErrorValue`. `go test ./...` verde; ML/f32 e `tests/llm/` sem regressão.

- **Task 1:** LU (pivô parcial) + `Det` + `Solve` + `Inv` + `linalg_test.go` (Det conhecidos, Solve resíduo~0, Inv·A≈I, singular→erro).
- **Task 2:** `QR` (Householder): Q·R≈A, QᵀQ≈I.
- **Task 3:** `EigSym` (Jacobi): A·v≈λ·v, soma autovalores=traço, não-simétrica→erro.
- **Task 4:** VM (`Det/Solve/Inv/LU/QR/EigSym`) + `tests/linalg_test.prw`.
- **Task 5:** Docs (README + CHANGELOG).
