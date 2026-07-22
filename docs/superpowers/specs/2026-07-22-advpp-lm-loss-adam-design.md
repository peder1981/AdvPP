# Sub-projeto 3b — Loss de LM (softmax-CE), Adam e backward de embedding

**Data:** 2026-07-22
**Status:** Aprovado (design; decisões tomadas pelo autor por delegação explícita do usuário)
**Contexto maior:** segundo dos três ciclos do "Full autodiff/treino"
(S3a motor+SGD ✓ → **S3b** softmax-CE+Adam+embedding → S3c módulos+trainer). Constrói
sobre `pkg/autograd` (S3a) e `pkg/tensor` (S2).

## Motivação

O S3a treina com MSE+SGD. Para treinar um **classificador/LM float de verdade**
faltam: a loss de classificação (**softmax + cross-entropy**), um otimizador
robusto (**Adam**), o gradiente de **embedding** (lookup de linhas) e o backward
das ativações restantes (Tanh/Sigmoid/Gelu). Este ciclo entrega essas peças.

## Objetivos (escopo)

- `Variable`: novas ops diferenciáveis **`Tanh`**, **`Sigmoid`**, **`Gelu`**;
  **`IndexRows(aIdx)`** (embedding, com backward scatter-add); e a loss fundida
  **`SoftmaxCE(aAlvo)`** (softmax + cross-entropy, estável).
- Otimizador **Adam** (`Step`/`ZeroGrad`), com `m`/`v` por parâmetro e correção de viés.
- Classes AdvPL: métodos novos em `Variable`; classe **`Adam`**.
- Testes: grad-check por diferenças finitas das ops novas; aceite treinando um
  **classificador** com softmax-CE + Adam (loss cai, acurácia alta).

## Não-objetivos (S3c)

- Módulos encapsulados (Linear/Embedding) e trainer.
- Outros otimizadores (RMSProp), regularização, batching/shuffling automático.

## Arquitetura

Adiciona a `pkg/autograd` (S3a intocado no que já existe) as ops novas e o Adam;
liga na VM no mesmo padrão de `Variable`/`SGD`. Reusa kernels do `pkg/tensor`
(`Softmax`, `IndexRows`, `Tanh`, `Sigmoid`, `Gelu`) no forward; backwards em Go.

## Ops novas — forward e backward exatos

| Op | Forward | Backward (dado `dY`) |
|---|---|---|
| `Tanh()` | `Y = tanh(A)` | `dA += dY ⊙ (1 − Y²)` |
| `Sigmoid()` | `Y = σ(A)` | `dA += dY ⊙ Y⊙(1 − Y)` |
| `Gelu()` | aproximação tanh (igual ao `pkg/tensor`) | `dA += dY ⊙ gelu'(A)` (derivada da aproximação tanh) |
| `IndexRows(aIdx)` | `Y = linhas de A nos índices aIdx` (`[K,C]`) | `dA += scatter-add`: para cada k, soma `dY[k,:]` na linha `aIdx[k]` de `dA` (zeros nas demais) |
| `SoftmaxCE(aAlvo)` | `A` = logits `[N,C]`; `aAlvo` = N índices de classe (1-based na API, 0-based interno); `loss = média_i(−log softmax(A)[i, alvo_i])` (estável via log-sum-exp) | `dA += (softmax(A) − onehot(alvo)) / N` |

`Gelu'` (aproximação tanh, `c=√(2/π)`): com `u = c(x + 0.044715 x³)`,
`t = tanh(u)`, `gelu'(x) = 0.5(1+t) + 0.5 x (1−t²) c (1 + 3·0.044715 x²)`.

`SoftmaxCE` é a raiz da loss (escalar `{1}`) — `Backward()` semeia grad 1. O
`aAlvo` é constante (não recebe grad).

## Otimizador Adam

```go
type Adam struct {
    params        []*Variable
    lr, b1, b2, eps float32
    t             int
    m, v          []*tensor.Tensor // estado por parâmetro
}
```
- Defaults: `b1=0.9`, `b2=0.999`, `eps=1e-8`. `NewAdam(params, lr)` usa os defaults.
- `Step()`: `t++`; para cada `p` com grad:
  `m := b1·m + (1−b1)·g`; `v := b2·v + (1−b2)·g²`;
  `mhat := m/(1−b1ᵗ)`; `vhat := v/(1−b2ᵗ)`;
  `p.Value -= lr·mhat/(√vhat + eps)` (in-place).
- `ZeroGrad()`: zera os grads.

## API AdvPL + ligação na VM

- `Variable` ganha os métodos: `Tanh()`, `Sigmoid()`, `Gelu()`, `IndexRows(aIdx)`
  (`aIdx` = array de índices 1-based), `SoftmaxCE(aAlvo)` (`aAlvo` = array de N
  índices de classe 1-based). Cada uma devolve `Variable`.
- Classe **`Adam`**: `Adam():New(aParams, nLR)`, `Step()`, `ZeroGrad()`.
- Erros → `advplrt.NewError` (catchável), via `verr` (do S3a).

## Estrutura de pacotes / arquivos

- Modificar `pkg/autograd/ops.go` (Tanh, Sigmoid, Gelu, IndexRows, SoftmaxCE).
- Criar `pkg/autograd/adam.go` (Adam).
- Modificar `pkg/autograd/autograd_test.go` (grad-check das ops novas + Adam).
- Modificar `pkg/vm/autograd_native.go` (novos métodos de `Variable` + classe `Adam`).
- Modificar `pkg/compiler/codegen.go` (`"ADAM"` em `builtinClasses`).
- Modificar `pkg/vm/vm.go` (`OP_NEW_INSTANCE` e `callNativeMethod` para `Adam`).
- Criar `tests/classifier_demo.prw` (aceite) e estender `tests/autograd_test.prw`.

## Testes e critérios de aceite

**`go test ./pkg/autograd`:**
- Grad-check por diferenças finitas: `Tanh`, `Sigmoid`, `Gelu`, `IndexRows`
  (grad w.r.t. a tabela) e `SoftmaxCE` (grad w.r.t. os logits, com alvos fixos).
- Adam: um `Step` reduz uma loss quadrática simples; estado `m`/`v`/`t` evolui.

**`tests/classifier_demo.prw` (aceite):** treina um classificador pequeno (ex.:
4-8 pontos 2D em 2 classes, MLP 2-H-2 com Tanh, loss `SoftmaxCE`, otimizador Adam)
por N épocas. Verifica que a **loss cai bem abaixo da inicial** e que a **acurácia
final** (via `Argmax` dos logits vs alvo) atinge **100%** no conjunto de treino —
prova softmax-CE + Adam + backward funcionando ponta a ponta.

### Critérios de aceite

1. Grad-check passa para todas as ops novas (`go test`).
2. Adam reduz a loss (`go test`).
3. `tests/classifier_demo.prw` treina: loss cai e acurácia final 100%.
4. Erros (alvo com índice fora de faixa, tipos errados) são `ErrorValue` capturável.
5. Regressão: `go test ./...` verde; S3a/S2 (`train_demo`, `autograd_test`,
   `mlp_demo`, `tensor_test`) sem regressão.

## Ordem de implementação sugerida

1. Ativações diferenciáveis (Tanh, Sigmoid, Gelu) + grad-check.
2. `IndexRows` diferenciável (scatter-add) + grad-check.
3. `SoftmaxCE` (fundida, estável) + grad-check.
4. `Adam` + `go test`.
5. Ligação na VM (métodos de `Variable` + classe `Adam`) + fixture.
6. `tests/classifier_demo.prw` (aceite) + docs.
