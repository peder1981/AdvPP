# Sub-projetos 6c/6d — Geometria espacial + Aritmética/Estatística

**Data:** 2026-07-22
**Status:** Aprovado (roadmap do kernel matemático; sobre S6a/S6b)
**Contexto:** 3º e 4º ciclos (S6a Tensor f64 ✓ → S6b álgebra linear ✓ → **S6c geometria**
→ **S6d aritmética/estatística**). Fecham o kernel matemático.

## S6c — Geometria espacial

Funções nativas (Go) operando sobre vetores/pontos como arrays AdvPL de números
(`{x,y}` ou `{x,y,z}`), em float64:

- `VecDot(aA, aB)` produto escalar (qualquer dim). `VecCross(aA, aB)` produto vetorial 3D.
- `VecNorm(aV)` magnitude. `VecNormalize(aV)` vetor unitário. `VecDist(aA, aB)` distância euclidiana.
- `VecAngle(aA, aB)` ângulo entre vetores (radianos, via acos do cosseno estável).
- `VecAdd/VecSub(aA,aB)`, `VecScale(aV, n)`.
- `RotateVec2(aV, nTheta)` rotação 2D; `RotateVec3(aV, cEixo, nTheta)` rotação 3D em torno de "x"/"y"/"z".

Erros (dims incompatíveis, cross fora de 3D) → `ErrorValue`.

## S6d — Aritmética faltante + estatística

Novas natives escalares: `ATAN2(y,x)`, `LOG10(x)`, `POW(b,e)`, `CEIL(x)`, `SIGN(x)`,
`SINH/COSH/TANH(x)`, `GCD(a,b)`, `LCM(a,b)`, `FACT(n)` (fatorial).

Estatística sobre array de números: `Mean(a)`, `StdDev(a)` (amostral), `Median(a)`,
`Variance(a)`, `Sum(a)` (já existe?), `LinReg(aX, aY)` → `{nA, nB}` (y=a+b·x, mínimos
quadrados). Interpolação `Interp(aX, aY, x)` (linear). Raiz por Newton omitida (YAGNI —
adicionável depois).

## Arquitetura

`pkg/vm/geometry_native.go` (geometria) e `pkg/vm/mathstat_native.go` (aritmética+stats),
cada um expondo `func geometryNatives()`/`func mathStatNatives()` que devolvem um
`map[string]func([]Value)(Value,error)`, mesclados em `registerNatives`. Helpers de
leitura de arrays de float64 e empacotamento de volta.

## Testes e critérios de aceite

- **`tests/geometry_test.prw`**: cross/dot/norm/dist/angle/normalize/rotações com valores
  conhecidos (ex.: cross({1,0,0},{0,1,0})={0,0,1}; angle de ortogonais = π/2;
  RotateVec2({1,0}, π/2) ≈ {0,1}). Erros capturáveis.
- **`tests/mathstat_test.prw`**: ATAN2/LOG10/POW/CEIL/SIGN/GCD/FACT; Mean/StdDev/Median/
  LinReg com casos conhecidos.
- **Regressão**: `go test ./...` verde; fixtures/modelos sem regressão.

## Ordem

1. S6c geometria (`geometry_native.go` + fixture).
2. S6d aritmética+estatística (`mathstat_native.go` + fixture).
3. Docs (README + CHANGELOG) e release do kernel matemático completo.
