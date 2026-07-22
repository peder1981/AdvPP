# Sub-projeto 2 — Núcleo de Tensor (forward, float32)

**Data:** 2026-07-22
**Status:** Aprovado (design), pronto para plano de implementação
**Contexto maior:** segundo sub-projeto para tornar o AdvPP capaz de desenvolvimento
robusto de um modelo de linguagem. O Sub-projeto 1 (robustez da linguagem) foi
concluído e lançado (v1.15.0). Este ciclo entrega o núcleo numérico de **forward**
(inferência); autodiff/treino fica para um ciclo futuro.

## Motivação

A representação de valores da VM é *boxed* (`advplrt.Value` como interface; cada
número é um `*NumberValue` no heap) — a raiz do custo em cargas numéricas, como
documenta o próprio `pkg/runtime/values.go`. Um modelo float "de verdade" precisa
de matemática densa rápida. A solução: uma classe **`Tensor`** cujo estado vive em
Go como `[]float32` plano e contíguo, com kernels em Go puro, deixando o AdvPL só
orquestrar (criar tensores, chamar `MatMul`/`Softmax`/etc.). O mecanismo de classe
com estado Go nativo (`ObjectValue.Native`) já existe e é usado pelo `FWGridProcess`.

## Objetivos (escopo)

- Classe `Tensor` (dtype **float32**) com armazenamento Go plano, orquestrada do AdvPL.
- Kernels de forward: construção, elementwise (+ broadcast), matmul, transpose,
  reshape, reduções, ativações, softmax, argmax, embedding/gather.
- Ponte AdvPL↔Tensor (de/para array plano).
- Testes de kernel em `go test` + fixture AdvPL auto-verificável + demo de aceite
  (MLP float forward).

## Não-objetivos (ciclos futuros)

- Autodiff / backprop / otimizadores / treino automático.
- Camadas prontas (Linear/Embedding/Attention), tokenizer, loop de treino.
- Convolução; slicing/indexação geral N-D; broadcasting geral (só os casos abaixo).
- SIMD/AVX2 nos kernels (correção primeiro; otimização é ciclo à parte).

## Arquitetura

Classe nativa `Tensor`, no padrão já existente do `FWGridProcess`:
- Instância: `obj := advplrt.NewObject("Tensor", nil)`; `obj.Native = &tensor.Tensor{...}`.
- Registro da classe builtin + despacho de `:New(...)` no caminho de builtin-class
  (junto de `FWGRIDPROCESS` em `pkg/vm/vm.go`, `registerClasses` e o dispatch de
  `OP_NEW_INSTANCE`/builtin).
- Despacho de métodos: `callTensorMethod(obj, method, args)` (espelha
  `callGridProcessMethod` em `pkg/vm/grid.go`), lendo `obj.Native.(*tensor.Tensor)`.

Duas unidades, cada uma com responsabilidade única:
- **`pkg/tensor/tensor.go`** — o tipo `Tensor` e todos os kernels, em Go puro, sem
  dependência da VM (testável direto com `go test`).
- **`pkg/vm/tensor_native.go`** — a ligação com a VM (registro da classe, construtores,
  dispatch de métodos, ponte de/para `advplrt.Value`).

### Tipo Go (`pkg/tensor`)

```go
type Tensor struct {
    Shape []int     // dims, ex.: {2,3}
    Data  []float32 // row-major, len == produto(Shape)
}
```

Convenções: armazenamento **row-major**; índices expostos ao AdvPL são **1-based**
(internamente 0-based); `nAxis` é 1-based sobre as dims. Operações são **funcionais**
(cada uma devolve um `Tensor` novo), exceto `Set` (muta e devolve self).

## Superfície da API

### Construtores (estáticos na classe `Tensor`)

| Chamada | Semântica |
|---|---|
| `Tensor():New(aShape)` | Tensor de zeros com a forma dada (`aShape` = array de inteiros). |
| `Tensor():FromArray(aData, aShape)` | De um array AdvPL plano row-major (`aData`, números) + forma. Erro se `Len(aData) != produto(aShape)`. |
| `Tensor():Rand(aShape, nEscala)` | Uniforme em `[-nEscala, nEscala]` (init). `nEscala` default 1. |

### Métodos de instância (`oT`)

| Método | Retorno / semântica |
|---|---|
| `Shape()` | Array AdvPL de inteiros (dims). |
| `Size()` | Nº total de elementos. |
| `Get(aIdx)` | Escalar no índice multi-dim (`aIdx` 1-based). |
| `Set(aIdx, n)` | Grava o valor; devolve self. |
| `ToArray()` | Array AdvPL plano row-major (números float64). |
| `Add(oU)` / `Sub(oU)` / `Mul(oU)` / `Div(oU)` | Elementwise (Hadamard em `Mul`); Tensor novo. Broadcast: mesma forma; ou `oU` escalar (`Size()==1`); ou `[M,N]` com `[N]`/`[1,N]` (por linha) ou `[M,1]` (por coluna). |
| `AddScalar(n)` / `MulScalar(n)` | Elementwise com escalar; Tensor novo. |
| `MatMul(oU)` | `[M,K] x [K,N] -> [M,N]`; matvec `[M,K] x [K] -> [M]`. Erro se dims não casam. |
| `Transpose()` | Transposta 2D. Erro se não for 2D. |
| `Reshape(aShape)` | Mesma `Data`, nova forma (produto deve casar `Size()`). |
| `Sum([nAxis])` / `Mean([nAxis])` / `Max([nAxis])` | Sem eixo: reduz tudo → **número AdvPL** (escalar). Com `nAxis`: reduz ao longo do eixo, removendo-o → **Tensor**. |
| `Argmax([nAxis])` | Índice 1-based do máximo. Sem eixo: índice plano global → **número AdvPL**. Com eixo: **Tensor** de índices (o eixo removido). |
| `Exp()` / `Log()` / `Sqrt()` / `Relu()` / `Tanh()` / `Sigmoid()` / `Gelu()` | Elementwise; Tensor novo. |
| `Softmax([nAxis])` | Softmax estável (subtrai o max) ao longo do eixo; default: última dim. |
| `IndexRows(aIdx)` | Colhe linhas: `self [R,C]`, `aIdx` array de índices 1-based → `[Len(aIdx), C]` (lookup de embedding). |

### Broadcasting (só estes casos)

1. Mesma forma exata.
2. `oU` escalar (`Size()==1`) — aplica a todos.
3. `[M,N]` com vetor-linha `[N]` ou `[1,N]` — repete por linha.
4. `[M,N]` com vetor-coluna `[M,1]` — repete por coluna.

Qualquer outra combinação → erro. Broadcasting geral N-D é não-objetivo.

## Ponte AdvPL↔Tensor e tratamento de erros

- `FromArray`: lê cada elemento com `advplrt.ToFloat` e converte para `float32`.
- `ToArray`/`Get`: devolve `advplrt.NewNumber(float64(v))`.
- Erros de forma (matmul incompatível, reshape inválido, `FromArray` com tamanho
  errado, eixo fora de faixa) **lançam** um `ErrorValue` com mensagem clara (ex.:
  `"Tensor:MatMul: dims incompatíveis [2,3]x[2,4]"`), capturável por `Try/Catch`.

## Estrutura de pacotes / arquivos

- Criar `pkg/tensor/tensor.go` (tipo + construtores + helpers de índice/forma).
- Criar `pkg/tensor/ops.go` (kernels: elementwise/broadcast, matmul, transpose,
  reshape, reduções, ativações, softmax, argmax, index-rows). *(Split tensor.go/ops.go
  é sugestão; o plano pode consolidar se ficar pequeno.)*
- Criar `pkg/tensor/tensor_test.go` (go test dos kernels).
- Criar `pkg/vm/tensor_native.go` (registro da classe + `callTensorMethod` + ponte).
- Modificar `pkg/vm/vm.go` (registrar a classe builtin `Tensor` e rotear `:New`/dispatch,
  junto do padrão `FWGRIDPROCESS`).
- Criar `tests/tensor_test.prw` (fixture da API) e `tests/mlp_demo.prw` (aceite).

## Testes e critérios de aceite

**`go test ./pkg/tensor`** (kernels, valores conhecidos):
- `MatMul`: `[[1,2],[3,4]] x [[5,6],[7,8]] = [[19,22],[43,50]]`.
- `Softmax`: soma 1 por eixo; estável com valores grandes.
- Broadcast: `[M,N] + [N]` e `+ [M,1]` corretos; escalar.
- Reduções: `Sum`/`Mean`/`Max`/`Argmax` com e sem eixo.
- `Transpose`, `Reshape`, `IndexRows`, ativações elementwise.

**`tests/tensor_test.prw`** (auto-verificável, `OK: N/N`): exercita a API da classe
ponta a ponta (New/FromArray/MatMul/Add-broadcast/Softmax/Argmax/ToArray).

**Aceite — `tests/mlp_demo.prw`:** um MLP float pequeno em AdvPL com pesos fixos
conhecidos: `h := X:MatMul(W1):Add(b1):Relu()`; `y := h:MatMul(W2):Add(b2):Softmax()`;
`pred := y:Argmax()`. Resultado numérico conferido contra um cálculo de referência
(no próprio fixture ou no `go test`), provando o forward de um modelo float real com
o AdvPL orquestrando e o Go fazendo a conta.

### Critérios de aceite

1. Todos os kernels passam no `go test ./pkg/tensor`.
2. `tests/tensor_test.prw` → `OK: N/N`.
3. `tests/mlp_demo.prw` produz o resultado de referência esperado.
4. Erros de forma lançam `ErrorValue` capturável (um caso testado via `Try/Catch`).
5. Regressão: `go test ./...` verde; fixtures e exemplos existentes sem regressão.

## Ordem de implementação sugerida

1. `pkg/tensor`: tipo + construtores + `go test` de forma/índice/`FromData`/`ToArray`.
2. Kernels elementwise + broadcast + escalar (com `go test`).
3. `MatMul` + `Transpose` + `Reshape` (com `go test`).
4. Reduções + `Argmax` (com `go test`).
5. Ativações + `Softmax` + `IndexRows` (com `go test`).
6. `pkg/vm/tensor_native.go` + registro/dispatch na VM + `tests/tensor_test.prw`.
7. `tests/mlp_demo.prw` (aceite) + docs (README/CHANGELOG).
