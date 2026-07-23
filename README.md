# AdvPP - Compilador AdvPL/TLPP em Go

Um compilador e interpretador totalmente funcional para as linguagens de programação AdvPL e TLPP, construído em Go.

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/peder1981/AdvPP/master/install.sh | sh
```

Detecta SO/arquitetura automaticamente (Linux amd64/arm64, macOS Apple Silicon) e instala o binário `advplc` mais recente em `~/.local/bin` (ou `/usr/local/bin` como root). Sem Go, sem dependências — o binário é estático.

Alternativas:
- **Debian/Ubuntu**: baixe o `.deb` em [Releases](https://github.com/peder1981/AdvPP/releases) e `sudo apt install ./advpp_*.deb`
- **Windows**: baixe o `.zip` em [Releases](https://github.com/peder1981/AdvPP/releases)
- **Extensão VS Code** (syntax highlighting, build/run/debug, debugger real, `advplc serve` attach): baixe o `.vsix` em [Releases](https://github.com/peder1981/AdvPP/releases) e instale com `code --install-extension advpl-tlpp-*.vsix` — já vem com o compilador embutido (linux-x64, linux-arm64, win32-x64, darwin-arm64), nada mais pra instalar. Fonte em [tools/vscode-advpl](tools/vscode-advpl/)

### Compilando do fonte

Requer Go 1.24+:

```bash
git clone https://github.com/peder1981/AdvPP && cd AdvPP
make build   # gera advplc, adveditor, advpp-ide na raiz do repo
```

## Recursos

- **Lexer**: Tokenizador completo para sintaxe AdvPL/TLPP incluindo palavras-chave, operadores, blocos de código e diretivas de pré-processador
- **Pré-processador**: Trata `#include`, `#define` (inclusive multi-linha), `#ifdef`/`#ifndef`/`#else`/`#endif`, e `#xCommand`/`#command`/`#xTranslate`/`#translate` com pattern-matching real (marcadores `<nome>`, cláusulas opcionais `[...]`, flags `[<nome:LITERAL>]`, resultado com `<{nome}>`/`<.nome.>`)
- **Parser**: Parser recursivo descendente completo gerando uma AST
- **Compilador**: Gera bytecode otimizado com 88 opcodes
- **Serialização de Bytecode**: Salva bytecode compilado em disco para execução posterior
- **Executáveis Standalone**: Constrói executáveis autossuficientes com bytecode embutido usando go:embed
- **Máquina Virtual**: VM completa com todos os opcodes implementados
- **Runtime**: Funções nativas (ConOut, MsgInfo, AllTrim, Str, Val, aAdd, aScan, Len, etc.)
- **I/O de disco, arquivo e sistema**: `MemoRead`/`MemoWrite`/`FErase`, API de handle para streaming (`FOpen`/`FCreate`/`FReadStr`/`FWrite`/`FSeek`/`FClose`/`FError`), console interativo `ConIn` e chamada de sistema `WaitRun` — ver seção [Funções de I/O, arquivo e sistema](#funções-de-io-arquivo-e-sistema)
- **BLAS ternária + IA em AdvPL puro**: kernel *multiply-free* `MatVecTern` (produto matriz-vetor ternário estilo BitNet) e três modelos escritos inteiramente em AdvPL — Markov (`pt_llm`), respondedor por recuperação (`pt_chat`) e híbrido Markov+rede neural ternária (`pt_nn`); ver [Exemplos de IA em AdvPL puro](#exemplos-de-ia-em-advpl-puro)
- **IDE Gráfica**: Ambiente de Desenvolvimento Gráfico usando Fyne com editor de código, navegador de arquivos e compilador integrado
- **Framework UI**: Aplicações gráficas usando Fyne (diálogos, formulários, grids, botões, menus)
- **Banco de Dados**: Operações de banco de dados baseadas em Workarea (DbSelectArea, DbSeek, DbSkip, RecLock, etc.)
- **Classes**: Sistema de classes completo com Data/Method/Constructor, herança via `from`
- **Blocos de Código**: Blocos de código executáveis `{|| ... }`
- **MVC**: Suporte FWFormModel, FWFormView, FWFormBrowse com validação de campos e tratamento de eventos
- **Multi-thread**: `StartJob()` (execução em VM isolado, semântica de work process) e `FWGridProcess` (pool de threads com `SetThreadGrid`, `CallExecute`, `StopExecute`, `IsFinished`, meters e log); `advplc check arq1 arq2 ...` verifica N arquivos em paralelo
- **Renderer web (PO-UI)**: `advplc serve programa.prw` executa o programa no servidor e renderiza a interface no browser com PO-UI (embutido no binário): console e diálogos em tempo real, `FWMBrowse`→`po-table` com dicionário SX3, formulários `po-dynamic-form`, MSDIALOG legado (`@ SAY/GET/BUTTON`) como modal por heurística de grade e hot reload com `--watch`
- **Motor de inferência LLM** (`pkg/llm` + classe `LLM`): carrega modelos GGUF quantizados em I2_S (BitNet/Falcon3-1.58bit) e gera texto direto do AdvPL/TLPP — 100% Go, sem CGO, com kernel SIMD AVX2 em amd64 e fallback escalar em qualquer outra arquitetura
- **Servidor MCP nativo** (`pkg/mcp` + classe `MCPServer`): expõe funções AdvPL/TLPP como "tools" de um servidor MCP real (JSON-RPC 2.0 sobre stdio) — funciona de verdade, validado com o SDK oficial do MCP
- **Servidor REST nativo** (`pkg/rest` + classe `WSRestServer`): sobe um servidor HTTP real (`net/http` puro) e expõe `User Function` anotadas com `@Get`/`@Post`/`@Put`/`@Patch`/`@Delete` como rotas, com path params (`/clientes/{id}`), corpo JSON e dispatch real para a função AdvPL — o DSL clássico `WSRESTFUL`/`WSMETHOD` continua só reconhecido na sintaxe (ver [Servidor REST](#servidor-rest-wsrestserver))
- **Núcleo de Tensor (float32)**: classe `Tensor` acelerada em Go (`pkg/tensor`) — `MatMul`, elementwise com broadcast, reduções, ativações, `Softmax`, `Argmax`, `IndexRows` — para construir e rodar modelos float com o AdvPL orquestrando; ver [Núcleo de Tensor](#núcleo-de-tensor)
- **Autodiff + treino (float32)**: motor de diferenciação reversa (`pkg/autograd`) com a classe `Variable` (tape + `Backward`), ops diferenciáveis (MatMul, Add, Mul, Relu, Sum, Mean, MSE) e otimizador `SGD` — treina modelos float com o AdvPL orquestrando; ver [Autodiff e treino](#autodiff-e-treino)

## Servidor MCP (`MCPServer`)

```advpl
User Function McpDemo()
    Local oMCP := MCPServer():New("meu-servidor", "1.0.0")
    oMCP:AddTool("soma", "Soma dois números", ;
        '{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}', ;
        "ToolSoma")
    oMCP:Serve() // bloqueia lendo/escrevendo em stdin/stdout
Return

User Function ToolSoma(oArgs)
Return cValToChar(oArgs:A + oArgs:B)
```

Roda com `advplc run meu_programa.prw` normalmente — não precisa de
comando novo. O `MCPServer` implementa o protocolo MCP (Model Context
Protocol) via JSON-RPC 2.0 sobre stdio — `initialize`, `tools/list`,
`tools/call` — em Go puro, sem CGO, sem dependências externas. Cada tool
chamada roda a função AdvPL correspondente numa VM isolada (mesmo
mecanismo do `StartJob`).

| Método | Descrição |
|--------|-----------|
| `New(cNome, cVersao)` | Cria o servidor |
| `AddTool(cNome, cDescricao, cSchemaJSON, cNomeFuncao)` | Registra uma tool; a função recebe um objeto com os argumentos (`oArgs:CAMPO`) e retorna o texto do resultado |
| `Serve()` | Sobe o loop stdio (bloqueia) |

Validado com o SDK oficial em Python do MCP (`cmd/advplc/mcp_integration_test.go`).

## Servidor REST (`WSRestServer`)

```advpl
@Get("/clientes/{id}")
User Function GetCliente(oParam)
    Local jRet := JsonObject():New()
    jRet["id"]   := oParam:ID       // path param populado automaticamente
    jRet["nome"] := "Cliente " + oParam:ID
Return jRet

@Post("/clientes")
User Function NovoCliente(oParam)
    Local jRet := JsonObject():New()
    jRet["criado"] := .T.
    jRet["nome"]    := oParam:NOME  // campo do corpo JSON da requisição
Return jRet

User Function RestDemo()
    Local oRest := WSRestServer():New("meu-servidor-rest", "1.0.0")
    // @Get/@Post/@Put/@Patch/@Delete acima já viram rota automaticamente;
    // AddRoute cobre o caso de registrar manualmente:
    oRest:AddRoute("GET", "/status", "GetStatus")
    oRest:Serve(8080) // bloqueia servindo HTTP na porta 8080
Return
```

Roda com `advplc run meu_programa.prw` normalmente. O `WSRestServer` sobe
um `net/http.Server` real (sem CGO, sem dependências externas) e, ao ser
criado, varre todas as `User Function` do programa procurando anotações
`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete("/path")` para registrar como
rotas automaticamente — path params (`{id}`) via roteador nativo do Go
1.22+, query string e corpo JSON mesclados num único objeto de argumento
(`oParam:CAMPO`, maiúsculo) passado para a função. Cada requisição roda a
função numa VM isolada (mesmo mecanismo do `MCPServer`/`StartJob`),
banco e bytecode compartilhados. O retorno da função vira o corpo JSON
da resposta (200); erro vira 500; path não registrado vira 404; verbo
não registrado num path existente vira 405.

| Método | Descrição |
|--------|-----------|
| `New(cNome, cVersao)` | Cria o servidor e auto-registra rotas de funções anotadas |
| `AddRoute(cVerbo, cPath, cNomeFuncao)` | Registra uma rota manualmente (`cPath` aceita `{param}`) |
| `Serve(nPorta)` | Sobe o servidor HTTP na porta indicada (bloqueia) |

**Limitação conhecida**: o DSL clássico `WSRESTFUL <nome> ... WSMETHOD
<verbo> PATH "..." ... ENDWSRESTFUL` é reconhecido pelo parser mas não
executado — o verbo e o `PATH` são descartados ao virar AST, e a
implementação do método é ligada a uma instância de classe (não a uma
função top-level, que é o que o dispatch HTTP sabe chamar). Para expor
esse serviço via HTTP hoje, reescreva no estilo anotações acima ou
registre a rota manualmente com `AddRoute`. Detalhes em
`COMPONENT_STATUS.md`.

Testado de ponta a ponta com requisições HTTP reais em
`cmd/advplc/rest_integration_test.go`.

## Motor de inferência LLM (`LLM`)

```advpl
User Function LlmDemo()
    Local oLLM := LLM():New("/caminho/Falcon3-3B-Instruct-1.58bit/ggml-model-i2_s.gguf")
    ConOut(oLLM:Generate("The capital of France is", 6, 0)) // prompt, nMaxTokens, nTemperatura (0=greedy)
    oLLM:Close()
Return
```

Motor de inferência para modelos **GGUF quantizados em I2_S** (pesos
ternários -1/0/+1, formato usado pelo BitNet e por conversões como o
Falcon3-3B-Instruct-1.58bit) — escrito inteiramente em Go
(`pkg/llm`), sem `llama.cpp`, sem CGO e sem dependências externas.
Compila e roda de forma idêntica em Linux, Windows e macOS
(amd64/arm64); em amd64 usa um kernel SIMD (AVX2) com detecção de CPU
em runtime, caindo automaticamente para um caminho escalar puro em
qualquer CPU/arquitetura sem esse suporte.

Métodos da classe `LLM`:

| Método | Descrição |
|--------|-----------|
| `New(cCaminhoGGUF)` | Carrega o modelo e o tokenizer |
| `Generate(cPrompt, nMaxTokens, nTemperatura)` | Gera texto (bloqueia até terminar; `nTemperatura<=0` = greedy) |
| `Tokenize(cTexto)` | Retorna um array de token ids |
| `Decode(aTokens)` | Converte token ids de volta em texto |
| `Close()` | Libera o modelo |

Validado **token a token** contra o `llama.cpp` de referência (ver
`pkg/llm/validate_test.go`). Limitações: só arquitetura GGUF `"llama"`
com pesos I2_S; sem streaming (ver CHANGELOG para a lista completa).

## Renderer web (`advplc serve`)

```bash
advplc serve tests/mvc_browse_test.prw          # http://localhost:8080
advplc serve programa.prw --port 9000 --watch   # porta própria + hot reload
```

O programa AdvPL/TLPP roda na VM do servidor (mesmo banco `ADVPP.db` de
todas as ferramentas) e o browser é o terminal de interface — mesmo
modelo do SmartClient HTML do Protheus. O frontend PO-UI/Angular vai
embutido no binário (`embed.FS`): nenhuma dependência extra em produção.

- `ConOut` → console em tempo real (SSE)
- `MsgYesNo`/`MsgInfo`/... → diálogos PO-UI que bloqueiam a VM até a resposta
- `FWMBrowse` sobre um alias → `po-table` com colunas/títulos do SX3 e
  CRUD completo (`po-dynamic-form` gerado do dicionário, soft-delete padrão)
- `DEFINE MSDIALOG` + `@ linha,coluna SAY/GET/BUTTON` → modal PO-UI; os
  valores digitados voltam para as variáveis do programa
- `--watch`: salvar o fonte recompila e recarrega o browser

Para alterar o frontend: `make web` (requer Node 20+; o resultado
compilado é versionado, então `go build` funciona sem Node).

## Framework MVC

O compilador AdvPP inclui um framework MVC (Model-View-Controller) completo para construir aplicações estruturadas:

### Componentes MVC

**FWFormModel** - Modelo de dados com definições de campos e validação:
```advpl
oModel := FWFormModel("CustomerModel")
```

**FWFormView** - View de formulário com componentes e tratamento de eventos:
```advpl
oView := FWFormView("CustomerView", oModel)
```

**FWFormBrowse** - Componente grid/browse para exibição de dados:
```advpl
oBrowse := FWFormBrowse("CustomerBrowse", oModel)
```

### Recursos
- Validação de campos (obrigatório, tamanho, intervalo, personalizado)
- Tratamento de eventos (onChange, onClick, onGotFocus, onLostFocus)
- **Renderização completa de widgets Fyne** (TButton, TGet, TComboBox, TCheckBox, TLabel)
- Estruturas de dados de componentes com renderização visual
- Suporte a diálogos (diálogos, menus, barras de ferramentas, barras de status)
- Eventos de browse (onLineChange, onDbClick, onHeaderClick)

**Nota**: Componentes UI agora renderizam visualmente usando Fyne. Manipuladores de eventos são definidos mas ainda não conectados às ações do usuário.

### Exemplo
```advpl
User Function MVCTest()
    Local oModel := FWFormModel("CustomerModel")
    Local oView := FWFormView("CustomerView", oModel)
    Local oBrowse := FWFormBrowse("CustomerBrowse", oModel)
    
    // Usar componentes MVC...
Return .T.
```

## Compilação

```bash
# Compilar todas as ferramentas (advplc, adveditor, advpp-ide)
make build

# Rodar os testes (build + verificação de todos os fixtures em tests/)
make test

# Cross-compilar o CLI para Linux/Windows/macOS (amd64 e arm64) em dist/
make cross

# Gerar pacotes versionados (.tar.gz/.zip) em dist/
make package VERSION=1.1.0
```

### Publicar uma nova versão no GitHub

```bash
make release VERSION=1.1.0
```

Isso cria e publica a tag `v1.1.0`. O GitHub Actions então compila
**nativamente** em Linux, Windows e macOS (incluindo as GUIs Fyne), gera os
pacotes (`.tar.gz`, `.zip`, `.deb`) e anexa tudo à Release automaticamente.

## Banco de dados compartilhado

Todas as ferramentas (advplc, adveditor, advpp-ide) enxergam o **mesmo**
banco SQLite, resolvido nesta ordem:

1. Flag explícita (`advplc run prog.prw --db-path /caminho/banco.db`)
2. Variável de ambiente `ADVPP_DB`
3. Banco configurado em `~/.advpp/advpp_config.json` (só se esse arquivo
   já existir — configurar isso é o que torna o banco "global")
4. Padrão: `./advpp.db` no diretório de trabalho atual — criado
   automaticamente (`RetSqlName`/`DbSelectArea`/etc. funcionam mesmo sem
   nenhuma tabela ainda; use o AdvEditor no mesmo diretório para criar
   tabelas, campos e índices nesse banco)

O driver SQLite é 100% Go (modernc.org/sqlite) — sem CGO, sem dependências
externas, idêntico em Linux, Windows e macOS.

## Uso

### Compilador de Linha de Comando

```bash
# Executar arquivo fonte AdvPL/TLPP (compila em memória e executa)
./advplc run program.prw

# Compilar fonte para arquivo de bytecode
./advplc compile program.prw -o program.bytecode

# Executar arquivo de bytecode compilado
./advplc exec program.bytecode

# Construir executável standalone (embute bytecode e runtime)
./advplc build program.prw -o program

# Verificar apenas sintaxe
./advplc check program.prw

# Imprimir estrutura AST
./advplc ast program.prw

# Imprimir bytecode
./advplc bytecode program.prw
```

### IDE Gráfica

```bash
# Iniciar ambiente de desenvolvimento gráfico
./advpp-ide
```

A IDE gráfica fornece:
- **Editor de Código**: Editor de texto multi-linha com suporte para arquivos .prw, .tlpp e .prg
- **Operações de Arquivo**: Funcionalidades Novo, Abrir, Salvar, Salvar Como
- **Explorador de Projeto**: Navegador de arquivos mostrando diretório atual com destaque de arquivos fonte
- **Integração de Build**: Comandos Compilar, Executar e Compilar & Executar
- **Console de Saída**: Mostra resultados de compilação e saída do programa
- **Suporte a Diálogos**: Funções MsgInfo, MsgStop, MsgAlert e MsgYesNo exibem diálogos Fyne
- **100% de Compatibilidade**: Todos os componentes MVC, renderização UI e recursos funcionam perfeitamente na IDE

## Suporte de Linguagem

### Recursos AdvPL
- User Function, Static Function, Function declarations
- Escopos de variável Local, Private, Public, Static
- If/ElseIf/Else/EndIf, For/Next (inclusive `Step` negativo/descendente), While/EndDo, Do Case/EndCase
- `Loop` (continue) e `Exit` (break) em loops, com aninhamento correto
- Tratamento de erro Begin Sequence/Recover/End Sequence
- Blocos de código `{|| expr }`
- Class/EndClass com Data, Method, Constructor
- Implementação de método fora do bloco de classe
- Acesso a campo de alias `SA1->A1_NOME`
- Auto-referência `::property`
- Todos os tipos de dados AdvPL: Character, Numeric, Logical, Date, Array, Code Block, Nil, Object
- `If()`/`IIF()` com 3 argumentos fazem curto-circuito (avaliam só o ramo escolhido)
- `Private`/`Public` com escopo dinâmico (visíveis às funções chamadas)
- Closures aninhadas: codeblocks capturam Locais N níveis acima por referência

### Recursos Adicionais TLPP
- Tipagem estática com palavra-chave `as`
- Tratamento de erro Try/Catch/EndTry
- Declarações de namespace
- Modificadores de acesso (Public, Private, Protected)
- Anotações REST (@Get, @Post, @Put, @Patch, @Delete) - executadas de verdade via `WSRestServer`, ver [Servidor REST](#servidor-rest-wsrestserver)
- Suporte JSON inline com métodos JsonObject
- Identificadores longos (com namespace)
- Tipos Integer, Double, Decimal, Variant, Variadic
- Parsing de sintaxe WSRESTFUL/WSSERVICE (DSL clássico — reconhecido, execução ainda não suportada; ver limitação em [Servidor REST](#servidor-rest-wsrestserver))

**Nota**: o DSL clássico `WSRESTFUL`/`WSMETHOD`/`ENDWSRESTFUL` é parseado mas não executado (o verbo/PATH são descartados no parser e o dispatch exigiria chamar método de instância). As anotações `@Get`/`@Post`/`@Put`/`@Patch`/`@Delete` sobre `User Function`, por outro lado, sobem um servidor HTTP real via `WSRestServer`.

## Funções de I/O, arquivo e sistema

O runtime expõe I/O de disco, uma API de handle de arquivo para streaming e uma
chamada de sistema — todas com semântica AdvPL nativa, em Go puro (sem CGO).

### I/O de disco (arquivo inteiro)

| Função | Descrição |
|--------|-----------|
| `MemoRead(cArq)` | Lê o arquivo inteiro e retorna como string (`""` se não existir) |
| `MemoWrite(cArq, cTexto)` | Grava a string no arquivo; retorna `.T.` em sucesso (alias: `MemoWrit`) |
| `FErase(cArq)` | Apaga o arquivo; `0` em sucesso, `-1` em erro |

### Console interativo

| Função | Descrição |
|--------|-----------|
| `ConIn([cPrompt])` | Lê uma linha do stdin (sem o `\n`); `""` no EOF. Contraparte de `ConOut` para programas de console interativos (REPL, chat) |

### Handle de arquivo (streaming — arquivos grandes)

| Função | Descrição |
|--------|-----------|
| `FCreate(cArq[, nAttr])` | Cria/trunca o arquivo; retorna handle (`>=1`) ou `-1` |
| `FOpen(cArq[, nMode])` | Abre existente; bit `0` de `nMode` = escrita (`0` = leitura). Handle ou `-1` |
| `FReadStr(nH, nBytes)` | Lê até `nBytes` e **retorna string** (`""` no fim do arquivo) |
| `FWrite(nH, cBuffer[, nBytes])` | Grava; retorna nº de bytes escritos |
| `FSeek(nH, nOffset[, nOrigin])` | `0`=início, `1`=atual, `2`=fim; retorna a nova posição |
| `FClose(nH)` | Fecha o handle; `.T.`/`.F.` |
| `FError()` | Código do último erro de I/O (`0` = sem erro) |

> A leitura usa `FReadStr` (retorna a string lida) em vez do `FRead` com buffer
> por referência — os natives da VM recebem valores, não lvalues, então byref em
> uma `Local` string não propagaria. `FReadStr` é a forma AdvPL genuína para isso.

```advpl
// Streaming de um arquivo grande em blocos de 4 KB
Local nH := FOpen("dados.txt", 0)
Local cBloco := FReadStr(nH, 4096)
While Len(cBloco) > 0
    // ... processa cBloco ...
    cBloco := FReadStr(nH, 4096)
End
FClose(nH)
```

### Chamada de sistema

| Função | Descrição |
|--------|-----------|
| `WaitRun(cCmd)` | Executa `cCmd` no shell do SO (cross-platform `sh -c` / `cmd /c`), herda stdio, espera e retorna o *exit code* (`0` = sucesso) |

Para **capturar** a saída de um comando, use o padrão AdvPL de redirecionar para
arquivo e ler — com a API de handle isso funciona para saída arbitrariamente
grande, em streaming:

```advpl
WaitRun("gerar_relatorio.sh > saida.txt")
Local nH := FOpen("saida.txt", 0)
Local cSaida := FReadStr(nH, 65536)  // ou em blocos, para arquivos enormes
FClose(nH)
```

### Álgebra linear ternária (BLAS)

| Função | Descrição |
|--------|-----------|
| `MatVecTern(aMat, aVecTern)` | Produto matriz-vetor *multiply-free* onde o vetor é **ternário** (`-1`/`0`/`+1`): `result[i] = Σ_j sign(vec[j])·mat[i][j]` — só soma/subtração, o kernel do BitNet. `aMat` é um array de M linhas (cada uma um array de N números); `aVecTern` tem N entradas |

Base para redes neurais ternárias em AdvPL: peso/ativação em `{-1,0,+1}`
eliminam a multiplicação, viabilizando treino e inferência sem BLAS de ponto
flutuante nem GPU (ver `tests/llm/pt_nn.prw`).

### Funções de array de ordem superior (com bloco de código)

Honram um bloco de código `{|...| ... }` de verdade (avaliado pela VM):

| Função | Descrição |
|--------|-----------|
| `ASort(aArr, [nIni], [nQtd], [bOrder])` | Ordena in-place; `bOrder(x,y)` retorna `.T.` se `x` vem antes de `y` (sem bloco: ascendente) |
| `AEval(aArr, bBloco, [nIni], [nQtd])` | Aplica `bBloco(elem, i)` a cada elemento |
| `AScan(aArr, uVal\|bBloco, [nIni], [nQtd])` | Posição do 1º elemento igual a `uVal` ou onde `bBloco(elem)` é `.T.`; `0` se não achar |
| `File(cArq)` | `.T.` se o arquivo existe (não-diretório) |
| `GetNames(oJson)` | Array com as chaves de um JsonObject, na ordem de inserção |

Os blocos são **closures de verdade**: capturam Locais do escopo envolvente por
referência — leitura e escrita — inclusive quando o bloco escapa da função que o
criou (estado persistente). Ex.: `AEval(a, {|x| nSoma := nSoma + x})` acumula no
`nSoma` externo; `{|| nN := nN + 1}` retornado por uma função vira um contador com
estado próprio. Captura em profundidade funciona completamente — bloco-dentro-de-bloco captura Locais N níveis acima por referência.

## Núcleo de Tensor

A classe `Tensor` (float32) guarda os dados como `[]float32` plano em Go — fora da
representação *boxed* de `Value` — e roda kernels de forward em Go puro. O AdvPL
orquestra; o Go faz a conta.

```advpl
Local oX  := Tensor():FromArray({1,2}, {1,2})
Local oW  := Tensor():Rand({2,3}, 0.1)
Local oH  := oX:MatMul(oW):Relu()          // [1,3]
Local oY  := oH:Softmax(2)                  // softmax por linha
Local nId := oY:Argmax()                    // classe prevista (1-based)
```

Construtores: `New(aForma)`, `FromArray(aDados, aForma)`, `Rand(aForma, nEscala)`.
Métodos: `Shape`, `Size`, `Get`/`Set`, `ToArray`; `Add`/`Sub`/`Mul`/`Div` (com
broadcast de escalar e linha/coluna), `AddScalar`/`MulScalar`; `MatMul`,
`Transpose`, `Reshape`; `Sum`/`Mean`/`Max`/`Argmax` (sem eixo → número; com eixo →
Tensor); `Exp`/`Log`/`Sqrt`/`Relu`/`Tanh`/`Sigmoid`/`Gelu`; `Softmax`; `IndexRows`
(lookup de embedding). Erros de forma são capturáveis por `Try/Catch`.

### Precisão selecionável (float32 / float64)

O dtype é escolhível **por tensor**: `float32` é o default (rápido, usado pelo ML) e
`float64` entra sob demanda para cálculo que exige exatidão (base do kernel de álgebra
linear/geometria). A precisão escalar do AdvPL já é float64; isto leva a dupla precisão
ao kernel de Tensor.

```advpl
Local oA := Tensor():New({2,2}, "float64")             // dtype float64
Local oB := Tensor():FromArray({1,2,3,4}, {2,2}, "float64")
? oB:DType()                                            // "float64"
Local oC := oA:ToFloat64()                              // converte f32 -> f64
? oB:Dot(oB)                                            // produto interno
? Tensor():FromArray({3,4},{2},"float64"):Norm()        // norma L2 = 5
```

Métodos de dtype: `DType()` (`"float32"`/`"float64"`), `ToFloat32()`/`ToFloat64()`,
`Dot(oOutro)` (produto interno) e `Norm()` (L2). As ops (`Add`/`MatMul`/… ) respeitam
o dtype e **promovem a float64** se qualquer operando for f64; o caminho float32
permanece idêntico (o ML não é afetado). Propagação de f64 pelo autodiff fica para um
ciclo futuro (álgebra/geometria não usam gradiente).

Este ciclo entrega o **forward** (inferência) + precisão dupla. Autodiff/treino veio em
ciclos seguintes.

### Álgebra linear (float64)

Sobre o Tensor float64, operações de álgebra linear em Go puro (não-diferenciáveis —
cálculo, não treino):

```advpl
Local oA := Tensor():FromArray({4,7,2,6}, {2,2}, "float64")
? oA:Det()                                  // determinante
Local oX := oA:Solve(oB)                     // resolve A·x = b (b vetor [n] ou [n,k])
Local oInv := oA:Inv()                        // inversa (A·Inv ≈ I)
Local aQR := oA:QR()                           // {Q, R} — Householder (Q·R ≈ A)
Local aEig := oS:EigSym()                       // {valores[n], vetores[n,n]} de matriz simétrica (Jacobi)
```

- **`Det()`** determinante via LU (pivô parcial); singular → 0.
- **`Solve(oB)`** resolve `A·x = b` por substituição direta/reversa sobre a LU.
- **`Inv()`** inversa resolvendo `A·X = I`; singular → erro capturável.
- **`QR()`** → `{Q, R}` por refletores de Householder (`Q` ortogonal, `R` triangular sup.).
- **`EigSym()`** → `{valores, vetores}` de matriz **simétrica** por rotações de Jacobi
  (autovalores decrescentes; colunas de `vetores` = autovetores). Não-simétrica → erro.
- **`SVD()`** → `{U, S, V}` (decomposição em valores singulares, Jacobi de um lado;
  `A ≈ U·diag(S)·Vᵀ`, `S` decrescente, suporta retangular m×n).
- **`Eig()`** → `{reais, imag}` — **todos** os autovalores de matriz **não-simétrica**
  (real), incluindo **pares complexos conjugados**, via redução a Hessenberg + QR de
  duplo shift (Francis/hqr). Para autovalor complexo, `imag` traz o par ±.

Erros (não-quadrada, singular, não-simétrica em `EigSym`, dims incompatíveis) são
`ErrorValue` capturáveis.

### Geometria espacial

Funções nativas sobre vetores/pontos como arrays (`{x,y}`/`{x,y,z}`), em float64:

```advpl
? VecCross({1,0,0}, {0,1,0})       // produto vetorial 3D -> {0,0,1}
? VecDot({1,2,3}, {4,5,6})          // produto escalar
? VecNorm({3,4})                     // magnitude -> 5
? VecDist({0,0}, {3,4})              // distância euclidiana -> 5
? VecAngle({1,0}, {0,1})            // ângulo (rad) -> π/2
Local aU := VecNormalize({3,4})     // vetor unitário
Local aR := RotateVec2({1,0}, nTheta)              // rotação 2D
Local aP := RotateVec3({1,0,0}, "z", nTheta)       // rotação 3D em torno de x/y/z
```

Também `VecAdd`, `VecSub`, `VecScale`. Erros (dims incompatíveis, cross fora de 3D) são capturáveis.

### Aritmética e estatística

Funções escalares adicionais: `Atan2(y,x)`, `Log10(x)`, `Pow(b,e)`, `Ceil(x)`,
`Sign(x)`, `Sinh/Cosh/Tanh(x)`, `Gcd(a,b)`, `Lcm(a,b)`, `Fact(n)`.

Estatística sobre arrays: `Mean(a)`, `Variance(a)`, `StdDev(a)` (amostrais), `Median(a)`,
`LinReg(aX, aY)` → `{a, b}` de `y = a + b·x` (mínimos quadrados), `Interp(aX, aY, x)`
(interpolação linear).

## Autodiff e treino

Sobre o núcleo de Tensor, a classe `Variable` grava um tape de operações e
`Backward()` propaga gradientes (reverse-mode autodiff). Com o otimizador `SGD`
dá pra TREINAR um modelo float — o AdvPL orquestra o laço; o Go faz forward e
backward.

```advpl
Local oW  := Variable():FromArray(aPesos, {nIn, nOut})
Local oB  := Variable():FromArray(aBias, {nOut})
Local oOpt := SGD():New({oW, oB}, 0.05)
// laço de treino:
Local oPred := oX:MatMul(oW):Add(oB):Relu()
Local oLoss := oPred:MSE(oY)
oOpt:ZeroGrad()
oLoss:Backward()          // preenche oW:Grad(), oB:Grad()
oOpt:Step()               // oW := oW - lr*grad
```

Ops diferenciáveis: `MatMul`, `Add` (com broadcast), `Mul`, `Relu`, `Sum`, `Mean`,
`MSE`. `oV:Value()`/`oV:Grad()` devolvem o `Tensor` de valor/gradiente. Este ciclo
entrega o motor + SGD; softmax/cross-entropy, Adam, embedding e módulos vêm nos
próximos ciclos. Corretude validada por verificação numérica de gradiente
(diferenças finitas) no `go test`.

Loss de classificação e otimizador robusto: `oLoss := oLogits:SoftmaxCE(aAlvo)`
(softmax + cross-entropy, alvo por índices de classe); `Adam():New(aParams, nLR)`
(`Step`/`ZeroGrad`). Ativações diferenciáveis `Tanh`/`Sigmoid`/`Gelu` e `IndexRows`
(embedding, com backward scatter-add). Ver `tests/classifier_demo.prw`.

Módulos e trainer: `Linear():New(nIn, nOut)` e `Embedding():New(nVocab, nDim)`
encapsulam parâmetros + `Forward`; `oMod:Params()` devolve os pesos para o
otimizador; `Fit(bPasso, nEpocas)` roda o laço de treino avaliando um codeblock por
época. Assim dá para definir e treinar um modelo em poucas linhas — ver
`tests/nn_demo.prw`.

## Exemplos de IA em AdvPL puro

Modelos escritos **inteiramente em AdvPL** (rodam com `advplc run <arq>`, cada um
com auto-teste), reunidos em **`tests/llm/`**. Diferente da classe `LLM` — que
carrega um GGUF pronto — aqui o modelo é construído na própria linguagem.

| Arquivo | O que é | Lê / Responde |
|---------|---------|---------------|
| `tests/llm/pt_llm.prw` | Cadeia de **Markov** de ordem variável em nível de byte (ordens 1–6, backoff) | Lê o prompt e **continua** o texto em PT-BR |
| `tests/llm/pt_chat.prw` | Respondedor por **recuperação**: normaliza (minúsculas + sem acento), tokeniza, descarta stopwords e pontua uma base de conhecimento por sobreposição de palavras | Lê a pergunta e **responde** com o item mais relevante (REPL via `ConIn`) |
| `tests/llm/pt_nn.prw` | **Híbrido Markov + rede neural ternária** (ELM) com **janela longa** (entrada e saída até 4096 tokens): contexto local posicional + bag long-context, perceptron médio, suavização interpolada e amostragem nucleus | Lê um seed de até 4096 tokens e gera um **documento multi-frase** de até 4096 tokens |
| `tests/llm/pt_neural.prw` | **LM neural char-level treinado por gradiente** (NPLM estilo Bengio): `Embedding → Reshape → Linear → Tanh → Linear → SoftmaxCE`, treinado com **Adam via `Fit`** sobre `corpus.txt` — o único que **aprende os pesos por backprop**, 100% sobre o stack de ML do AdvPP (S2+S3) | Lê um seed e gera texto PT-BR char-a-char por amostragem com temperatura |
| `tests/llm/dev_nn.prw` | **LM neural de código AdvPL, token-level** (dev-oriented): lexer AdvPL próprio → NPLM sobre tokens (vocab top-N + `<unk>`), treinado no código do repo + `algos_advpl.prw`. Gera/completa código AdvPL; tem **REPL de autocomplete** | Lê um prefixo AdvPL e continua gerando código token a token |
| `tests/llm/algos_advpl.prw` | **Biblioteca de 25 algoritmos** (lógica/leetcode/script) em AdvPL puro: ordenação, busca, recursão, strings, DP (troca de moedas, LCS), Kadane, two-sum, FizzBuzz… cada um testável | Auto-teste com asserts (`OK: todos passaram`); serve de corpus de código |

O `tests/llm/pt_nn.prw` é o "topo" do que se treina e roda **sem sair do AdvPL**. A
projeção ternária e a saída perceptron são multiply-free (via `MatVecTern`); o
aprendizado é medível (os erros do perceptron caem a cada passada); o Markov
interpolado dá o prior local enquanto a rede, com o **bag long-context** (janela
de até 4096 tokens, mantida incrementalmente em O(1) amortizado), condiciona a
geração. Algoritmos modernos: **perceptron médio** (Collins 2002), **suavização
interpolada** (Jelinek-Mercer), **amostragem nucleus** (top-p), **vocabulário
limitado** por frequência (top-N + `<unk>`) e **amostra de treino por stride** —
os dois últimos deixam o custo do treino limitado, independente do tamanho do
corpus. Gera **documentos multi-frase** de até 4096 tokens a partir de um seed de
até 4096 tokens.

Não é uma rede de ponto flutuante nem um transformer — atenção real sobre 4096
tokens exigiria float (inviável multiply-free em AdvPL); o bag é a aproximação de
custo limitado. É o limite honesto do que a linguagem permite treinar e executar
por conta própria. A qualidade e o contexto útil escalam com o corpus: forneça um
`corpus.txt` grande (carregado automaticamente via `MemoRead`).

O `corpus.txt` incluído é **_Dom Casmurro_ de Machado de Assis** (domínio
público, via [Project Gutenberg](https://www.gutenberg.org/ebooks/55752)),
~72 mil tokens — treina em ~30s e produz texto temático/machadiano. Remova o
`corpus.txt` para cair no corpus factual curado embutido (prosa mais limpa, porém
simples). Ressalva honesta: prosa literária complexa excede a capacidade de um
modelo n-grama+ELM — a saída fica temática mas não totalmente coerente.

### `pt_neural.prw` — o LM neural treinado por gradiente

Enquanto `pt_nn` usa uma rede ternária *sem* backprop, o `tests/llm/pt_neural.prw`
é o **capstone**: um LM neural char-level (byte-level, seguro para UTF-8/acentos)
**treinado de verdade por descida de gradiente**, montado 100% sobre o stack de ML
do AdvPP (Tensor S2 + autodiff/treino S3). Arquitetura NPLM (Bengio 2003):

```advpl
oEmb := Embedding():New(V, D)            // tabela de embeddings [V, D]
oL1  := Linear():New(k*D, H)
oL2  := Linear():New(H, V)
// forward de um lote de N exemplos (contexto de k chars -> próximo char):
oLog := oL2:Forward( oL1:Forward( oEmb:Forward(aX):Reshape({N, k*D}) ):Tanh() )
oLoss := oLog:SoftmaxCE(aAlvo)           // perda de próximo-char
// treino: Adam sobre Params() dos 3 módulos, via Fit(bPasso, nEpocas)
```

A única peça de motor que este ciclo adicionou é a op **`Variable:Reshape(aShape)`**
diferenciável — para concatenar os `k` embeddings de contexto num vetor por exemplo.
Geração char-a-char: dado um seed, faz forward com N=1, aplica temperatura + softmax
e amostra o próximo byte. No mini-corpus determinístico (auto-teste) a loss cai de
~2.77 para ~0.04 e o modelo reproduz o texto aprendido; no `corpus.txt` real
(_Dom Casmurro_, vocab 97) treina uma amostra em ~1min e a loss cai de ~4.58 para
~0.06, gerando morfologia PT-BR. Ressalva honesta: é um modelo pequeno num VM
interpretado — decora a amostra de treino e emerge estrutura do português, mas não é
fluente. É o "modelo neural completo em AdvPP", ponta a ponta (tokenizar → treinar →
gerar), provando que o stack float treina um LM de verdade.

### `dev_nn.prw` — LM de código AdvPL orientado a desenvolvimento

Mesmo motor do `pt_neural`, mas a unidade é o **token AdvPL** (não o byte): um **lexer
AdvPL escrito em AdvPL** quebra o fonte em keywords/identificadores/números/strings/
operadores, o vocabulário é os **top-N tokens por frequência + `<unk>`**, e o NPLM
prevê o próximo token. Treina no código do próprio repo (montado por
`tests/llm/build_corpus.sh` em `code_corpus.txt`) somado à biblioteca
`algos_advpl.prw` — 25 algoritmos clássicos (ordenação, busca, recursão, DP, Kadane,
two-sum, FizzBuzz…) que dão o **viés de lógica/leetcode**. Gera/completa código AdvPL
e traz um **REPL de autocomplete** (`ConIn`): digite um prefixo, recebe a continuação.

```
advpl> Local aLista := {}
       Local a3rdRow, 1,; oSize
       If SubStr( ?, i [ nLen Upper Len ...
advpl> For i := 1 To Len( ?)) ConOut( ?) Local ? := {} ...
```

No corpus real (46 mil tokens, vocab 301) a loss cai de ~5.70 para ~0.31 em ~90s, e a
geração reproduz idiomas AdvPL (`For..To Len()`, `ConOut()`, `Local := {}`,
`If/Else/Endif`, `dbSelectArea()`), com identificadores raros como `?` (`<unk>`).
**Teto honesto:** é um modelo pequeno num VM interpretado — aprende a *estrutura* de
tokens e os idiomas algorítmicos do corpus e gera código plausível enviesado a lógica,
mas **não raciocina nem resolve problemas novos de leetcode** (isso exige um LLM grande
pré-treinado). A "habilidade em lógica" vem do corpus curado + token-level, não de
capacidade de raciocínio. Escala com corpus/k/H maiores e mais camadas.
