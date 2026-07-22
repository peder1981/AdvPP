# Sub-projeto 3c — Módulos (Linear/Embedding) e trainer

**Data:** 2026-07-22
**Status:** Aprovado (design; decisões do autor por delegação explícita do usuário)
**Contexto maior:** terceiro e último ciclo do "Full autodiff/treino"
(S3a motor+SGD ✓ → S3b softmax-CE+Adam+embedding ✓ → **S3c** módulos+trainer).
Constrói sobre `pkg/autograd` (S3a/S3b) e `pkg/tensor` (S2).

## Motivação

O S3a/S3b já treinam, mas o código AdvPL gerencia pesos/bias e o laço à mão. Este
ciclo encapsula camadas em **módulos** (`Linear`, `Embedding`) que carregam seus
próprios parâmetros e um `Forward`, e um helper **`Fit`** para o laço de treino —
para **definir e treinar um modelo em poucas linhas** de AdvPL.

## Objetivos (escopo)

- Pacote `pkg/nn`: **`Linear`** (W,b) e **`Embedding`** (tabela) — cada um com
  `Forward` (constrói o grafo via autograd) e `Params()` (para o otimizador).
- Classes AdvPL `Linear` e `Embedding`; native **`Fit(bPasso, nEpocas)`** que roda
  o laço avaliando um codeblock por época (usa closures do S1).
- Testes: `go test` dos módulos (formas, grafo/grad); aceite treinando um MLP
  definido com módulos, conciso.

## Não-objetivos (YAGNI)

- `Sequential`/composição automática, dropout, batchnorm, salvar/carregar pesos.
- Outros módulos (Conv, Attention). Outros otimizadores.

## Arquitetura

`pkg/nn` (Go puro, testável) contém os módulos, que compõem ops do `pkg/autograd`
(S3a/S3b intocados). A VM expõe `Linear`/`Embedding` como classes (estado Go em
`ObjectValue.Native`) e `Fit` como native. `Fit` reusa `callBlockSync` (S1) para
avaliar o codeblock de passo.

### Tipos Go (`pkg/nn`)

```go
type Linear struct { W, B *autograd.Variable }
type Embedding struct { Table *autograd.Variable }
```
- `NewLinear(nIn, nOut int, scale float32) *Linear` — `W = Rand([nIn,nOut], scale)`,
  `B = zeros([nOut])`, ambos folhas (`autograd.NewLeaf`).
- `(*Linear).Forward(x *autograd.Variable) (*autograd.Variable, error)` =
  `x.MatMul(W).Add(B)`.
- `(*Linear).Params() []*autograd.Variable` = `{W, B}`.
- `NewEmbedding(nVocab, nDim int, scale float32) *Embedding` — `Table = Rand(...)`.
- `(*Embedding).Forward(idx []int) (*autograd.Variable, error)` = `Table.IndexRows(idx)`.
- `(*Embedding).Params()` = `{Table}`.

## API AdvPL + ligação na VM

- `Linear():New(nIn, nOut [, nScale])` (scale default 0.1). `oLin:Forward(oX)` →
  `Variable`. `oLin:Params()` → array de objetos `Variable` (`{W, b}`).
- `Embedding():New(nVocab, nDim [, nScale])`. `oEmb:Forward(aIdx)` → `Variable`
  (aIdx 1-based). `oEmb:Params()` → `{tabela}`.
- Native `Fit(bPasso, nEpocas)`: avalia o codeblock `bPasso` `nEpocas` vezes e
  devolve o **valor** da última avaliação (a loss final). `bPasso` deve fazer
  forward + `oOpt:ZeroGrad()` + `oLoss:Backward()` + `oOpt:Step()` e retornar a
  loss (número). Captura o modelo/dados/otimizador por closure.
- Erros → `advplrt.NewError` (catchável).

## Estrutura de pacotes / arquivos

- Criar `pkg/nn/module.go` (`Linear`, `Embedding`).
- Criar `pkg/nn/module_test.go` (`go test`).
- Modificar `pkg/vm/autograd_native.go` (classes `Linear`/`Embedding`; native `Fit`).
- Modificar `pkg/compiler/codegen.go` (`"LINEAR"`, `"EMBEDDING"` em `builtinClasses`).
- Modificar `pkg/vm/vm.go` (`OP_NEW_INSTANCE` + `callNativeMethod` para as classes).
- Modificar `pkg/vm/natives.go` (native `Fit`).
- Criar `tests/nn_demo.prw` (aceite).

## Testes e critérios de aceite

**`go test ./pkg/nn`:**
- `Linear`: `Params()` = {W [nIn,nOut], b [nOut]}; `Forward(x)` dá `[N,nOut]`; um
  `Backward` sobre `Forward(x).Sum()` preenche grads de W e b (formas certas).
- `Embedding`: `Forward(idx)` dá `[len(idx), nDim]`; backward preenche grad da tabela.

**`tests/nn_demo.prw` (aceite):** define um MLP com **dois `Linear`** + `Tanh`,
coleta os params dos módulos, e treina com `Adam` via `Fit(bPasso, nEpocas)` num
problema de classificação (SoftmaxCE). Verifica que a loss cai e a **acurácia final
é 100%** — provando que módulos + trainer permitem definir e treinar um modelo de
forma concisa.

### Critérios de aceite

1. `go test ./pkg/nn` passa (formas + grafo/grad).
2. `tests/nn_demo.prw` treina: loss cai, acurácia 100%.
3. `Fit` avalia o codeblock N vezes e devolve a loss final.
4. Erros (tipos errados) são `ErrorValue` capturável.
5. Regressão: `go test ./...` verde; S3a/S3b/S2 sem regressão.

## Ordem de implementação sugerida

1. `pkg/nn`: `Linear` + `Embedding` + `go test`.
2. Ligação na VM: classes `Linear`/`Embedding` + native `Fit` + rebuild.
3. `tests/nn_demo.prw` (aceite) + docs.
