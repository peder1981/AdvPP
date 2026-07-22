# Sub-projeto 1 — Robustez da linguagem AdvPP

**Data:** 2026-07-22
**Status:** Aprovado (design), pronto para plano de implementação
**Contexto maior:** primeiro de dois sub-projetos para tornar o AdvPP capaz de
desenvolvimento robusto de um modelo de linguagem. O segundo (núcleo numérico ML:
Tensor/Matrix + kernels float) terá spec própria depois desta.

## Motivação

Ao construir modelos de linguagem em AdvPL nesta base (pt_llm, pt_chat, pt_nn),
recorreram limitações de semântica da linguagem que forçaram contornos. Este
sub-projeto fecha quatro delas, deixando o AdvPL sólido como linguagem antes de
investir no núcleo numérico.

## Objetivos (escopo)

1. `If()`/`IIF()` com **curto-circuito** (avalia só o ramo escolhido).
2. **Iteração de chaves** de `JsonObject` (`GetNames`).
3. **Closures aninhadas** (captura de variável livre 2+ níveis acima).
4. `Private` com **escopo dinâmico** real (visível às funções chamadas).

## Não-objetivos

- Performance do interpretador (otimização da VM) — fora de escopo.
- Núcleo numérico / float / autodiff — é o Sub-projeto 2.
- Captura de upvalue de nomes que sejam `Private`/`Public` (feature 4 cobre esses
  por resolução dinâmica, não por upvalue).

## Feature 1 — `If()`/`IIF()` curto-circuito

**Problema:** `If`/`IIF` são funções nativas; os argumentos são avaliados antes da
chamada, então **os dois ramos** executam. `If(x>0, aAdd(a,x), aAdd(b,x))` adiciona
a `a` **e** `b`.

**Solução:** forma especial no compilador. Em `compileCallExpr`, quando o nome
(upper) é `IF` ou `IIF` e há **exatamente 3 argumentos**, emitir controle de fluxo
em vez de `OP_CALL_NATIVE`:

```
compileExpr(cond)
JUMP_IF_FALSE L1
compileExpr(ramoVerdadeiro)
JUMP L2
L1: compileExpr(ramoFalso)
L2:
```

O valor do ramo escolhido fica no topo da pilha (a expressão `If(...)` avalia para
ele). Formas com número de argumentos diferente de 3 continuam indo para o native
atual (retrocompatível).

**Casos de borda:**
- `IF`/`IIF` como *statement* (`If ... EndIf`) não passa por `compileCallExpr` —
  não é afetado.
- Argumentos puros (sem efeito colateral) continuam com o mesmo resultado.

**Teste (fixture):** `If(x>0, aAdd(aPos,x), aAdd(aNeg,x))` sobre uma lista mista;
verificar que cada elemento entra em exatamente um dos arrays.

## Feature 2 — Iteração de chaves de hash (`GetNames`)

**Problema:** não há como percorrer as chaves de um `JsonObject`; isso força
manter listas paralelas (feito em pt_nn/pt_chat).

**Solução:** native `GetNames(oJson)` → array de strings com as chaves do
`ObjectValue.Props`. A ordem deve ser **de inserção** (previsível e útil). Como o
`Props` atual é um `map[string]Value` (Go, sem ordem), a implementação vai manter a
ordem de inserção — via uma slice de chaves paralela no `ObjectValue`, atualizada
quando uma chave nova é criada (em `OP_ARRAY_SET`, `OP_SET_PROP`, `OP_NEW_OBJECT` e
onde mais `Props` recebe chave nova). `GetNames` retorna essa slice.

**Casos de borda:** objeto vazio → array vazio; sobrescrever chave existente não
duplica na ordem.

**Teste (fixture):** montar hash com 3 chaves, `GetNames`, iterar e somar/concatenar;
verificar contagem e ordem de inserção.

## Feature 3 — Closures aninhadas (captura em profundidade)

**Estado atual:** upvalues de nível único — um bloco captura Locais do contexto
**imediatamente** envolvente (`resolveUpvalue` olha só `parent.locals`). Bloco
dentro de bloco alcançando um Local 2+ níveis acima cai para global.

**Solução:** modelo Lua de upvalues com **origem tipada**. Cada upvalue de um bloco
passa a ter:
- `Kind = LOCAL, Index = slot` → captura `&frame.Locals[slot]` do frame envolvente.
- `Kind = UPVAL, Index = i` → captura o **mesmo ponteiro** `parentCb.Upvalues[i]`
  (aponta para o slot original, N níveis acima).

`resolveUpvalue(name)` passa a **recorrer**: se `name` não é local do pai, tenta
resolvê-lo como upvalue do pai (recursivamente); se o pai o captura, o bloco atual
captura via `UPVAL(indice_do_pai)`. Todos os níveis apontam para o mesmo
armazenamento — escrita em qualquer nível é vista em todos.

**Mudanças de tipo:**
- `FunctionInfo.UpvalSlots []int` → `FunctionInfo.Upvals []UpvalDesc`, onde
  `UpvalDesc{ Kind uint8; Index int }` (Kind: 0=LOCAL, 1=UPVAL).
- `OP_NEW_CODEBLOCK` na VM: para cada `UpvalDesc`, captura de `frame.Locals[Index]`
  (LOCAL) ou de `parentCb.Upvalues[Index]` (UPVAL). O `parentCb` é o
  `CodeBlockValue` em execução no frame envolvente (`frame.Locals[0]`).

**Teste (fixture):** `{|x| AEval(a, {|y| soma := soma + x + y})}` com `soma` Local da
função e `x` param do bloco externo — o bloco interno captura os dois níveis.

## Feature 4 — `Private` com escopo dinâmico (completo)

**Semântica alvo (AdvPL real):** uma variável `Private` declarada na função A é
visível **por nome** a qualquer função que A chame (e às chamadas dessas), até A
retornar. Várias declarações formam uma pilha (shadowing dinâmico).

**Decisão tomada:** **escopo dinâmico completo** — nomes **não-declarados**
(não são Local/param/upvalue) passam a resolver dinamicamente (Private-like). É a
semântica correta do AdvPL (atribuir a um nome não declarado cria um `Private`,
dinamicamente escopado) e fecha a limitação de propagação de verdade.

Comportamento atual (a corrigir): dentro de uma função, uma referência a nome não
declarado **cria um Local implícito** naquele frame (o `addLocal` aloca um slot);
no escopo de arquivo, vira global. Com a feature, esses nomes deixam de virar
Local/global implícito e passam a ser **dinâmicos**. Declarações explícitas
(`Local`, param, variável de `For`) continuam em slots estáticos — a resolução
dinâmica só recai sobre referências realmente não declaradas.

**Implementação:**

*Runtime (VM):* um ambiente dinâmico `privateEnv` na VM — uma pilha de escopos. A
forma escolhida: um `map[string]Value` corrente + uma lista de "restaurações"
por frame. Ao declarar `Private X` (ou `Public X`), salva o binding anterior de X
(se houver) numa lista associada ao frame atual e grava X no map. No `Return` do
frame, restaura os bindings salvos (remove os criados, repõe os sobrescritos).
Assim o escopo dinâmico segue a pilha de chamadas corretamente.

*Compilador:* a resolução de um nome no `compileExpr`/`compileStoreTarget` passa a
ser, em ordem:
1. Local/param do bloco/função (slot estático) → `OP_LOAD_LOCAL`/`STORE_LOCAL`.
2. Upvalue (closure) → `OP_LOAD_UPVAL`/`STORE_UPVAL`.
3. **Caso contrário → dinâmico** → `OP_LOAD_DYN name` / `OP_STORE_DYN name`
   (antes: caía em slot global).

Declaração `Private X [:= expr]` / `Public X [:= expr]`: registra X como dinâmico
(o compilador não aloca slot) e, se houver inicializador, emite `OP_STORE_DYN X`.
Deixa de compilar como Local.

*Opcodes novos:* `OP_LOAD_DYN` (Str=nome) empilha o valor do env dinâmico (nil se
inexistente); `OP_STORE_DYN` (Str=nome) grava. `Private`/`Public` também emitem um
marcador para o frame saber quais nomes restaurar no return — detalhe de
implementação (ex.: `OP_DECL_DYN name`, que salva o binding anterior e registra a
restauração no frame).

**Mudança de comportamento (o risco):** código que hoje dependia de nome
não-declarado virar **Local implícito** (dentro de função) ou **global** (escopo
de arquivo) passa a ver **escopo dinâmico**. Código bem escrito declara todos os
Locais (convenção do projeto), então o risco é baixo; a suíte de regressão e os
exemplos `pt_*` validam.

**Performance:** Locais e upvalues continuam em slots estáticos (rápidos). O custo
de lookup por nome recai **só** sobre variáveis dinâmicas (Private/Public/não
declaradas), que não aparecem em loops quentes de código bem escrito.

**Teste (fixture):** função A declara `Private cCtx := "x"`, chama B; B lê e
escreve `cCtx`; de volta em A o valor reflete a escrita de B; após A retornar,
`cCtx` não vaza para o chamador de A.

## Transversal

**Opcodes novos:** `OP_LOAD_DYN`, `OP_STORE_DYN`, `OP_DECL_DYN` (feature 4).
Ajuste do modelo de upvalue (feature 3). `GetNames` é native (sem opcode).
Feature 1 reusa `OP_JUMP`/`OP_JUMP_IF_FALSE`.

**Arquivos afetados:**
- `pkg/compiler/opcodes.go` — novos opcodes, `UpvalDesc`, `FunctionInfo.Upvals`.
- `pkg/compiler/codegen.go` — If/IIF especial; upvalue recursivo/tipado;
  resolução dinâmica; compilação de Private/Public.
- `pkg/vm/vm.go` — execução dos opcodes novos; captura de upvalue tipada;
  `privateEnv` + restauração no return; ordem de chaves no `ObjectValue`.
- `pkg/vm/natives.go` — `GetNames`.
- `pkg/runtime/values.go` — `CodeBlockValue.Upvalues` (já existe); ordem de chaves
  no `ObjectValue` (slice de chaves).

**Estratégia de regressão:**
- `go test ./...` verde.
- Os 26 fixtures `tests/*.prw` + os exemplos `pt_llm`/`pt_chat`/`pt_nn` continuam
  compilando e rodando com os mesmos resultados.
- Um fixture novo por feature (`tests/*_test.prw`), cada um com asserções via
  `ConOut` de valores esperados.

## Critérios de aceite

1. `If(x>0, aAdd(a,x), aAdd(b,x))` afeta exatamente um array por elemento.
2. `GetNames(oJson)` devolve as chaves em ordem de inserção; iteração funciona.
3. Bloco aninhado captura e escreve variável 2+ níveis acima (mesmo armazenamento).
4. `Private` declarado em A é visível e mutável por B (chamada por A) e não vaza
   após A retornar.
5. Suíte Go + todos os fixtures + os 3 exemplos permanecem verdes.

## Ordem de implementação sugerida

1. `GetNames` (isolado, trivial, alto valor).
2. `If`/`IIF` curto-circuito (isolado, contido no compilador).
3. Closures aninhadas (evolui um mecanismo já existente).
4. `Private` escopo dinâmico (o mais invasivo; por último, com o resto estável).
