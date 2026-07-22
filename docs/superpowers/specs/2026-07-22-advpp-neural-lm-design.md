# Sub-projeto 4 — LM neural char-level, treinado 100% em AdvPP

**Data:** 2026-07-22
**Status:** Aprovado (design; decisões pelo autor + 3 escolhas do usuário via AskUserQuestion)
**Contexto maior:** o capstone da missão original — montar e **treinar** um modelo de
linguagem neural inteiramente em AdvPL, sobre o stack de ML lançado na v1.16.0
(Tensor S2 + autodiff/treino S3a/b/c). Fecha "um LLM neural completo em AdvPP".

## Motivação

O stack S2+S3 treina redes (classificador → 100%), mas ninguém montou ainda um
**LM neural next-token** de verdade. Este ciclo entrega isso: um NPLM char-level
(estilo Bengio 2003) com **Embedding real**, treinado com Adam via `Fit` sobre um
corpus PT-BR, que **gera texto**. Falta ao motor apenas uma op — `Reshape`
diferenciável — para concatenar os embeddings do contexto.

## Decisões do usuário

1. **Arquitetura:** Embedding real + op `Reshape` nova (não o NPLM one-hot).
2. **Tokenização:** char-level (vocab ~80–100 chars do corpus).
3. **Escopo:** treino + geração + auto-teste determinístico.

## Objetivos (escopo)

- **Motor:** `Variable.Reshape(shape)` diferenciável em `pkg/autograd` (forward
  `tensor.Reshape`; backward reshapa o grad de volta à forma original) + método VM
  `Reshape(aShape)` + grad-check `go test` + fixture.
- **Modelo:** `tests/llm/pt_neural.prw` — tokenizador char-level, montador de
  exemplos (janela k → próximo char), forward Embedding→Reshape→Linear→Tanh→Linear,
  treino Adam+`Fit`+`SoftmaxCE`, geração por amostragem, auto-teste.
- **Dados:** mover `corpus.txt` → `tests/llm/corpus.txt`; ajustar `pt_nn.prw` para
  procurar `tests/llm/corpus.txt` (mantendo `corpus.txt` como fallback).

## Não-objetivos (YAGNI)

- Atenção/transformer, RNN/BPTT, batching/shuffle automático, checkpoint em disco.
- Tokenização subword/BPE, multi-camada profunda, regularização.
- Otimização de performance do VM. Treino no corpus real usa amostra + épocas modestas.

## Arquitetura

### Parte 1 — op `Reshape` diferenciável (`pkg/autograd/ops.go`)

```go
func (a *Variable) Reshape(shape []int) (*Variable, error)
```
- Forward: `y, err := a.Value.Reshape(shape)` (`tensor.Reshape`, ops.go:218; erro se
  o produto das dims ≠ Size). `out := &Variable{Value: y, parents: []*Variable{a}}`.
- Backward: `dg, _ := out.Grad.Reshape(a.Value.Shape); addGrad(a, dg)` (reshape
  preserva ordem/contagem → grad volta 1:1). Mesmo padrão dos ops existentes.

VM (`pkg/vm/autograd_native.go`, método de `Variable`): `case "RESHAPE"` — lê a forma
via `shapeFromArg(getArg(args,0))`, chama `self.Reshape(shp)`, erro → `verr`.

### Parte 2 — modelo `tests/llm/pt_neural.prw` (AdvPL puro)

**Tokenizador char-level:** varre o corpus, coleta chars distintos, ordena (`aSort`),
monta id↔char (1-based para casar com Embedding/SoftmaxCE). Vocab = V.

**Exemplos:** janela deslizante de tamanho `k` sobre a sequência de ids; para cada
posição, contexto = k ids anteriores, alvo = id seguinte. Gera `aX` (achatado,
comprimento N·k, ordem exemplo-maior) e `aAlvo` (N ids). Teto `N` amostrado por
stride para limitar custo no corpus real.

**Modelo (forward, dado aX/aAlvo):**
```
oEmb := Embedding():New(V, D)
oL1  := Linear():New(k*D, H)
oL2  := Linear():New(H, V)
// forward:
oE := oEmb:Forward(aX)          // [N*k, D]
oR := oE:Reshape({N, k*D})      // [N, k*D]   <-- op nova
oH := oL1:Forward(oR):Tanh()    // [N, H]
oLog := oL2:Forward(oH)         // [N, V]  (logits)
oLoss := oLog:SoftmaxCE(aAlvo)  // escalar
```

**Treino:** params = concat de `oEmb:Params()`, `oL1:Params()`, `oL2:Params()` (5
Variables); `oOpt := Adam():New(aParams, nLR)`; passo `{|| ZeroGrad; forward; loss;
Backward; Step; devolve loss }`; `Fit(bPasso, nEpocas)`. Loss cai.

**Geração:** dado um seed (string), pega os últimos k ids; forward com N=1 → logits
`[1,V]`; `aLogits := oLog:Value():ToArray()`; aplica temperatura (divide), softmax e
amostragem (cumulativa com `Random(1e6)/1e6`, com top-k simples) em AdvPL; anexa o
char; repete por `nGerar` passos. Contexto inicial preenchido com um char de padding
se o seed for menor que k.

**Auto-teste determinístico:** mini-corpus embutido pequeno e repetitivo (ex.: uma
frase curta repetida), k pequeno, poucas épocas; verifica `loss_final < loss_inicial
* 0.5` (aprendizado) e que `Gera(...)` devolve string não-vazia. Sem depender do
corpus externo (determinismo/velocidade).

**Corpus real:** `MemoRead("tests/llm/corpus.txt")` se `File(...)`; senão mini-corpus.

## Estrutura de arquivos

- Modificar `pkg/autograd/ops.go` (`Reshape`).
- Modificar `pkg/autograd/autograd_test.go` (grad-check de `Reshape`).
- Modificar `pkg/vm/autograd_native.go` (método `RESHAPE`).
- Criar `tests/reshape_test.prw` (fixture da op).
- Criar `tests/llm/pt_neural.prw` (o modelo + auto-teste).
- Mover `corpus.txt` → `tests/llm/corpus.txt`; ajustar `tests/llm/pt_nn.prw`.
- Docs: `README.md` (tabela de modelos + seção), `CHANGELOG.md`.

## Testes e critérios de aceite

1. **`go test ./pkg/autograd`**: grad-check por diferenças finitas de `Reshape`
   passa (grad w.r.t. entrada, formas diferentes).
2. **`tests/reshape_test.prw`**: `oV:Reshape(aShape)` muda a forma, `Backward`
   preenche grad na forma original; forma inválida → `ErrorValue` capturável.
3. **`tests/llm/pt_neural.prw` (auto-teste)**: treina no mini-corpus, `loss_final <
   loss_inicial*0.5`, e a geração devolve texto não-vazio. Imprime `OK: ...`.
4. **Corpus real**: rodar em `tests/llm/corpus.txt` treina (loss cai) e gera um
   trecho PT-BR mais coerente que aleatório (verificação visual, não assertada).
5. **Regressão**: `go test ./...` verde; S2/S3 e `pt_llm`/`pt_chat`/`pt_nn`
   compilam (`advplc check`) e rodam sem regressão.

## Ordem de implementação sugerida

1. `Reshape` diferenciável (autograd + grad-check) + método VM + fixture.
2. `pt_neural.prw`: tokenizador + exemplos + forward (com auto-teste de forma).
3. Treino + geração + auto-teste (mini-corpus).
4. Rodar no corpus real; mover corpus; ajustar `pt_nn`; docs.
