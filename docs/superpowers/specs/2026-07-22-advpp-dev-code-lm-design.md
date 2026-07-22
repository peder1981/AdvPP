# Sub-projeto 5 — Modelo de código AdvPL (dev-oriented), token-level, em AdvPP

**Data:** 2026-07-22
**Status:** Aprovado (design; 3 escolhas do usuário + diretriz "lógica/leetcode/script")
**Contexto maior:** um LM neural **orientado a desenvolvimento** — treinado nos
fontes AdvPL do próprio AdvPP + uma biblioteca de algoritmos/leetcode em AdvPL —
que completa/gera código AdvPL. Reusa o stack neural (S2/S3 + Reshape do S4) e o
NPLM do `pt_neural`, agora **token-level** sobre lexemas AdvPL.

## Decisões do usuário

1. **Tokenização:** token-level de código (lexer AdvPL → tokens; vocab top-N + `<unk>`).
2. **Corpus:** fontes AdvPL/TLPP do repo — **estendido** com uma biblioteca de
   algoritmos/leetcode em AdvPL (para dar o viés de lógica pedido).
3. **Uso:** REPL interativo de autocomplete (+ auto-teste determinístico não-interativo).

## Expectativa honesta (teto)

Um NPLM pequeno treinado em ~300 KB num VM interpretado **não raciocina nem resolve
problemas novos** de leetcode. Ele aprende a distribuição de tokens AdvPL e os
idiomas algorítmicos do corpus, gerando código sintaticamente plausível e enviesado
a lógica. "Altas habilidades em lógica/leetcode" é entregue via **corpus curado**
(algoritmos corretos e idiomáticos) + token-level (gera lexemas válidos), não via
capacidade de raciocínio do modelo. Evolução: corpus maior, k/H maiores, 2ª camada.

## Objetivos (escopo)

- **Biblioteca de algoritmos AdvPL** (`tests/llm/algos_advpl.prw`): ~25 implementações
  clássicas de lógica/leetcode/script (ordenação, busca, recursão, strings, matemática,
  DP, estruturas), cada uma `Static Function`, com um `User Function` de auto-teste
  (asserts). Entregável real + material de corpus.
- **Modelo `tests/llm/dev_nn.prw`** (AdvPL puro, token-level):
  - **Lexer AdvPL** em AdvPL: quebra fonte em tokens (keywords, identificadores,
    números, strings, operadores multi/mono-char, pontuação).
  - **Vocab** top-N por frequência + `<unk>` (+ `<eos>` por arquivo).
  - **NPLM token-level:** `Embedding(V,D) → Reshape → Linear(k*D,H) → Tanh →
    Linear(H,V) → SoftmaxCE`, treino Adam via `Fit`.
  - **Geração/autocomplete:** prefixo → tokeniza → últimos k ids → forward → amostra
    próximo token → remonta texto (join com espaços) → repete até `<eos>`/N.
  - **REPL** `ConIn`: digita prefixo AdvPL, recebe completação. EOF/"sair" encerra.
  - **Auto-teste determinístico** (mini-corpus de código embutido): loss cai; geração
    de um prefixo fixo é não-vazia. Roda sem stdin (REPL sai no EOF).
- **Corpus assembly:** `tests/llm/code_corpus.txt` = concatenação dos fontes AdvPL do
  repo + `algos_advpl.prw`, gerado por um passo de build (script). `dev_nn` lê via
  `MemoRead` se existir; senão mini-corpus embutido.

## Não-objetivos (YAGNI)

- Preservar literais de string/números exatos (colapsam em `<unk>` se raros).
- Formatação/indentação perfeita na geração. Parser real (só lexer). Multi-camada.
- Fine-tuning por tarefa, embeddings pré-treinados, execução do código gerado.

## Arquitetura

Reusa 100% do stack de ML (nenhuma op de motor nova — o `Reshape` do S4 já basta). A
diferença vs `pt_neural` é a **unidade**: tokens AdvPL em vez de bytes. O pipeline
NPLM é idêntico; muda o tokenizador (lexer) e a des-tokenização (join).

### Lexer AdvPL (em AdvPL)

`Static Function Lex(cSrc)` → array de strings (tokens), na ordem:
- pula espaços/tabs/newlines (newline vira token `<nl>` opcional para o modelo
  aprender quebras — decisão: **sim**, emite `<nl>`);
- identificador/keyword: `[A-Za-z_][A-Za-z0-9_]*`;
- número: `[0-9]+(.[0-9]+)?`;
- string: `"..."`/`'...'` → token literal do conteúdo entre aspas colapsado no vocab
  por frequência (raros → `<unk>`);
- operadores 2-char: `:= == != <= >= -> ++ -- += -= ::` etc.; senão 1-char.
- comentários `//`/`/* */` → pulados (não entram no corpus de tokens).

### Vocab e ids

`BuildVocab(aTokens, nTopN)`: conta frequências, ordena desc, pega top-N, adiciona
`<unk>`. `Tok2Id`/`Id2Tok`. Tokens fora do top-N → `<unk>`. Ids 1-based.

### Modelo e treino

Idênticos ao `pt_neural` (Embedding→Reshape→Linear→Tanh→Linear→SoftmaxCE, Adam, Fit,
amostra por stride, `LossOnly`/`Passo`). Parâmetros default: k=4 tokens, D=32, H=128,
topN≈300, épocas≈120, nMax≈2500. Mini (auto-teste): k=3, D=16, H=32.

### Geração / des-tokenização

`Gera(prefixo, nTok, nTemp)`: tokeniza prefixo, janela de k ids (pad com `<unk>`),
forward N=1, `Sample` (softmax+temperatura, top-k opcional), mapeia id→token, junta:
`<nl>`→newline, pontuação cola sem espaço, resto com espaço. Para em `<eos>` ou nTok.

## Estrutura de arquivos

- Criar `tests/llm/algos_advpl.prw` (biblioteca de algoritmos + auto-teste).
- Criar `tests/llm/dev_nn.prw` (lexer + vocab + NPLM + REPL + auto-teste).
- Criar `tests/llm/build_corpus.sh` (concatena fontes → `code_corpus.txt`).
- `code_corpus.txt` gerado (gitignore — é derivado).
- Docs: `README.md` (tabela + seção do modelo de código), `CHANGELOG.md`.

## Testes e critérios de aceite

1. **`algos_advpl.prw`**: `advplc run` → todos os asserts passam (`OK: N/N`).
2. **Lexer**: um teste (dentro de `dev_nn` ou fixture) tokeniza `"Local nX := 0"` em
   `[Local][nX][:=][0]` (whitespace/coments corretos).
3. **`dev_nn.prw` (auto-teste, mini-corpus)**: treina, `loss_final < loss_inicial*0.5`,
   geração de um prefixo fixo é não-vazia. Roda sem stdin (REPL sai no EOF). `OK: ...`.
4. **Corpus real**: `build_corpus.sh` gera `code_corpus.txt`; `dev_nn` treina nele
   (loss cai) e gera código AdvPL plausível a partir de prefixos como `User Function`.
5. **REPL**: `printf 'Local nX\nsair\n' | advplc run dev_nn.prw` produz uma completação.
6. **Regressão**: `go test ./...` verde; os modelos de `tests/llm/` sem regressão.

## Ordem de implementação

1. `algos_advpl.prw` (biblioteca + auto-teste). 
2. `dev_nn.prw`: lexer + vocab (com teste de tokenização).
3. NPLM token-level: treino + auto-teste (mini-corpus).
4. Geração/des-tokenização + REPL.
5. `build_corpus.sh` + rodar no corpus real + docs.
