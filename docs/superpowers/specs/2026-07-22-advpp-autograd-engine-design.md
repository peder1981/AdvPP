# Sub-projeto 3a — Motor de autograd + treino básico (SGD)

**Data:** 2026-07-22
**Status:** Aprovado (design), pronto para plano de implementação
**Contexto maior:** primeiro de três ciclos que compõem o "Full autodiff/treino"
(S3a motor+SGD → S3b softmax-CE+Adam+embedding → S3c módulos+trainer). Constrói
sobre o núcleo de Tensor float32 do Sub-projeto 2 (`pkg/tensor`).

## Motivação

O `pkg/tensor` (S2) roda o **forward** de modelos float, mas treinar exige
**gradientes**. Este ciclo entrega um motor de diferenciação reversa (reverse-mode
autodiff, define-by-run) — uma classe `Variable` que grava um tape de operações e,
via `Backward()`, propaga gradientes de trás pra frente — mais um otimizador SGD.
Isso torna possível **treinar** um modelo float de verdade, com o AdvPL orquestrando
o laço de treino e o Go fazendo forward e backward.

## Objetivos (escopo)

- Pacote `pkg/autograd`: `Variable` (valor + grad + tape) e `Backward()`.
- Ops diferenciáveis: MatMul, Add (com broadcast), Mul, Relu, Sum, Mean, e a loss
  **MSE** (fundida).
- Otimizador **SGD** (`Step`/`ZeroGrad`).
- Classes AdvPL `Variable` e `SGD` na VM.
- Testes: verificação numérica de gradiente (diferenças finitas) em `go test`;
  fixture AdvPL; e demo de aceite que **treina um MLP** com a loss caindo.

## Não-objetivos (ciclos futuros)

- Softmax + cross-entropy, Adam, backward de ativações extra (Tanh/Sigmoid/Gelu) e
  de embedding (IndexRows) → **S3b**.
- Módulos encapsulados (Linear/Embedding) e trainer → **S3c**.
- `requires_grad` seletivo, broadcast em `Mul`, autodiff de ordem superior, GPU.

## Arquitetura

Pacote novo **`pkg/autograd`**, com o `pkg/tensor` (S2) **intocado** e reusado para
o forward e para a matemática do backward. Uma classe `Variable` **separada** de
`Tensor` (Tensor = dado/constante sem grad; Variable = nó diferenciável) — isola o
autograd e mantém os kernels forward puros.

### Tipo `Variable` e o tape

```go
type Variable struct {
    Value    *tensor.Tensor
    Grad     *tensor.Tensor // acumulado; nil até receber gradiente
    parents  []*Variable
    backward func()         // lê v.Grad e ACUMULA nos Grad dos pais
}
```

- **Folha** (leaf): `NewLeaf(val *tensor.Tensor) *Variable` — sem pais, `backward` nil.
- Cada op cria uma `Variable` nova com `parents` e um `backward` que distribui o
  gradiente. Acumulação de gradiente via helper `addGrad(v, g)`: se `v.Grad == nil`
  então `v.Grad = cópia(g)`, senão `v.Grad = v.Grad.Add(g)` (mesma forma).
- Toda Variable acumula grad (sem flag `requires_grad` neste ciclo); o otimizador
  atualiza só a lista de parâmetros que recebe.

### `Backward()`

Chamado numa Variable **escalar** (a loss, forma `{1}`). Passos:
1. Ordena topologicamente o grafo alcançável a partir da raiz (DFS pós-ordem →
   pais antes do nó; a lista fica `[entradas..., raiz]`).
2. Semeia `raiz.Grad = onesLike(raiz.Value)` (grad 1 na loss).
3. Percorre a lista em ordem **reversa** (raiz primeiro): para cada nó com
   `backward != nil`, chama `backward()`, que lê o `Grad` do nó e acumula nos pais.
   A ordem garante que, ao processar um nó, todos os grads a jusante já chegaram.

## Ops diferenciáveis — forward (via `pkg/tensor`) e backward exatos

| Op | Forward | Backward (dado `dY = Y.Grad`) |
|---|---|---|
| `MatMul(A,B)` | `Y = A·B` (2D) | `dA += dY·Bᵀ`; `dB += Aᵀ·dY` |
| `Add(A,B)` (broadcast) | `Y = A ⊕ B` | `dA += reduceGradTo(dY, A.shape)`; `dB += reduceGradTo(dY, B.shape)` |
| `Mul(A,B)` (mesma forma) | `Y = A ⊙ B` | `dA += dY ⊙ B`; `dB += dY ⊙ A` |
| `Relu(A)` | `Y = max(0,A)` | `dA += dY ⊙ (A > 0 ? 1 : 0)` |
| `Sum(A)` | `Y = Σ A` (escalar) | `dA += broadcast(dY)` (dY escalar → forma de A) |
| `Mean(A)` | `Y = média(A)` | `dA += broadcast(dY / N)` |
| `MSE(Ŷ, alvo)` | `loss = média((Ŷ−alvo)²)` | `dŶ += (2/N)(Ŷ−alvo)` (o `alvo` é constante, sem grad) |

`reduceGradTo(g, formaAlvo)` soma `g` sobre os eixos que foram replicados no
broadcast do `Add`, casando os 4 casos do S2: mesma forma → `g`; alvo escalar
(`Size 1`) → soma tudo; alvo linha `[N]`/`[1,N]` → soma no eixo 0; alvo coluna
`[M,1]` → soma no eixo 1. (Helper novo em `pkg/autograd`, usando `SumAxis`/`SumAll`
do `pkg/tensor`.)

`MatMul` backward usa `Transpose` + `MatMul` do `pkg/tensor`. `Relu` backward usa
uma máscara elementwise. Tudo reusa kernels existentes; nenhum kernel novo em
`pkg/tensor`.

## Otimizador SGD

```go
type SGD struct { params []*Variable; lr float32 }
```
- `Step()`: para cada `p`, se `p.Grad != nil`, `p.Value := p.Value − lr·p.Grad`
  (atualização **in-place** no tensor de valor).
- `ZeroGrad()`: zera `p.Grad` de cada parâmetro (`p.Grad = nil` ou tensor de zeros).

## API AdvPL + ligação na VM

Classes `Variable` e `SGD` registradas no padrão do `Tensor` (`builtinClasses` no
codegen; `case` em `OP_NEW_INSTANCE`; `case` em `callNativeMethod`), com estado Go
em `ObjectValue.Native`.

`Variable`:
- `Variable():New(oTensor)` — folha a partir de um objeto `Tensor` do S2.
- `Variable():FromArray(aData, aForma)` — folha a partir de array plano + forma.
- Ops (cada uma devolve uma `Variable` nova): `MatMul(oW)`, `Add(oB)`, `Mul(oU)`,
  `Relu()`, `Sum()`, `Mean()`, `MSE(oAlvo)`.
- `Backward()` — preenche os grads do grafo (devolve self).
- `Value()` → objeto `Tensor` (valor); `Grad()` → objeto `Tensor` (gradiente).

`SGD`:
- `SGD():New(aParams, nLR)` — `aParams` = array de objetos `Variable`.
- `Step()`, `ZeroGrad()`.

Argumentos que devem ser `Variable`/`Tensor` mas não são → erro `advplrt.NewError`
capturável (padrão do S2).

## Estrutura de pacotes / arquivos

- Criar `pkg/autograd/variable.go` (tipo `Variable`, `NewLeaf`, `addGrad`, `Backward`, helpers `onesLike`/`reduceGradTo`).
- Criar `pkg/autograd/ops.go` (as 7 ops diferenciáveis).
- Criar `pkg/autograd/sgd.go` (otimizador SGD).
- Criar `pkg/autograd/autograd_test.go` (grad-check por diferenças finitas + SGD).
- Criar `pkg/vm/autograd_native.go` (classes `Variable` e `SGD` na VM + ponte).
- Modificar `pkg/compiler/codegen.go` (`"VARIABLE"`, `"SGD"` em `builtinClasses`).
- Modificar `pkg/vm/vm.go` (`OP_NEW_INSTANCE` e `callNativeMethod` para as duas classes).
- Criar `tests/autograd_test.prw` (fixture da API) e `tests/train_demo.prw` (aceite).

## Testes e critérios de aceite

**`go test ./pkg/autograd`:**
- **Verificação numérica de gradiente** (diferenças finitas) para cada op: monta
  `f(x)` escalar, compara o grad analítico do `Backward` com
  `(f(x+ε) − f(x−ε)) / 2ε` (ε≈1e-3, tolerância ≈1e-2 em float32). Cobrir MatMul,
  Add-broadcast (bias linha e coluna), Mul, Relu, Sum, Mean, MSE.
- Teste do `SGD.Step` (um passo reduz a loss num caso linear fechado) e do
  acúmulo de gradiente (nó reusado soma contribuições).

**`tests/autograd_test.prw`** (auto-verificável `OK: N/N`): monta um grafo pequeno
(`y = Relu(X·W + b); l = MSE(y, alvo)`), `Backward`, e confere que `W:Grad()`/
`b:Grad()` têm as formas certas e valores não-nulos coerentes.

**Aceite — `tests/train_demo.prw`:** treina um **MLP tiny** para aprender uma função
não-linear (ex.: XOR, 4 exemplos, 1 camada oculta com Relu, loss MSE) por N épocas
com SGD. Imprime a loss a cada algumas épocas e **verifica que a loss final é
bem menor que a inicial** (ex.: cai abaixo de 10% da inicial) — prova que o
autodiff + SGD treinam de verdade, com o AdvPL orquestrando.

### Critérios de aceite

1. Grad-check por diferenças finitas passa para todas as 7 ops (`go test`).
2. `tests/autograd_test.prw` → `OK: N/N`.
3. `tests/train_demo.prw` treina o MLP e a loss final cai bem abaixo da inicial.
4. Erros (arg de tipo errado, `Backward` em não-escalar) são `ErrorValue` capturável.
5. Regressão: `go test ./...` verde; fixtures e exemplos do S2 (`mlp_demo.prw`,
   `tensor_test.prw`) sem regressão.

## Ordem de implementação sugerida

1. `pkg/autograd`: `Variable`, `NewLeaf`, `addGrad`, `onesLike`, `Backward` + o
   helper `reduceGradTo` (com `go test` de `reduceGradTo` e do tape).
2. Ops diferenciáveis (MatMul, Add, Mul, Relu, Sum, Mean, MSE), cada uma com
   grad-check por diferenças finitas.
3. `SGD` (`Step`/`ZeroGrad`) com `go test`.
4. `pkg/vm/autograd_native.go` + registro/dispatch na VM + `tests/autograd_test.prw`.
5. `tests/train_demo.prw` (aceite: treino do MLP) + docs (README/CHANGELOG).
