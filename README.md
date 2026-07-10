# AdvPP - Compilador AdvPL/TLPP em Go

Um compilador e interpretador totalmente funcional para as linguagens de programação AdvPL e TLPP, construído em Go.

## Recursos

- **Lexer**: Tokenizador completo para sintaxe AdvPL/TLPP incluindo palavras-chave, operadores, blocos de código e diretivas de pré-processador
- **Pré-processador**: Trata `#include`, `#define`, `#ifdef`/`#ifndef`/`#else`/`#endif`, `#xCommand`, `#xTranslate`
- **Parser**: Parser recursivo descendente completo gerando uma AST
- **Compilador**: Gera bytecode otimizado com 88 opcodes
- **Serialização de Bytecode**: Salva bytecode compilado em disco para execução posterior
- **Executáveis Standalone**: Constrói executáveis autossuficientes com bytecode embutido usando go:embed
- **Máquina Virtual**: VM completa com todos os opcodes implementados
- **Runtime**: Funções nativas (ConOut, MsgInfo, AllTrim, Str, Val, aAdd, aScan, Len, etc.)
- **IDE Gráfica**: Ambiente de Desenvolvimento Gráfico usando Fyne com editor de código, navegador de arquivos e compilador integrado
- **Framework UI**: Aplicações gráficas usando Fyne (diálogos, formulários, grids, botões, menus)
- **Banco de Dados**: Operações de banco de dados baseadas em Workarea (DbSelectArea, DbSeek, DbSkip, RecLock, etc.)
- **Classes**: Sistema de classes completo com Data/Method/Constructor, herança via `from`
- **Blocos de Código**: Blocos de código executáveis `{|| ... }`
- **MVC**: Suporte FWFormModel, FWFormView, FWFormBrowse com validação de campos e tratamento de eventos
- **Multi-thread**: `StartJob()` (execução em VM isolado, semântica de work process) e `FWGridProcess` (pool de threads com `SetThreadGrid`, `CallExecute`, `StopExecute`, `IsFinished`, meters e log); `advplc check arq1 arq2 ...` verifica N arquivos em paralelo
- **Renderer web (PO-UI)**: `advplc serve programa.prw` executa o programa no servidor e renderiza a interface no browser com PO-UI (embutido no binário): console e diálogos em tempo real, `FWMBrowse`→`po-table` com dicionário SX3, formulários `po-dynamic-form`, MSDIALOG legado (`@ SAY/GET/BUTTON`) como modal por heurística de grade e hot reload com `--watch`
- **Motor de inferência LLM** (`pkg/llm` + classe `LLM`): carrega modelos GGUF quantizados em I2_S (BitNet/Falcon3-1.58bit) e gera texto direto do AdvPL/TLPP — 100% Go, sem CGO, com kernel SIMD AVX2 em amd64 e fallback escalar em qualquer outra arquitetura
- **Servidor MCP nativo** (`pkg/mcp` + classe `MCPServer`): expõe funções AdvPL/TLPP como "tools" de um servidor MCP real (JSON-RPC 2.0 sobre stdio) — funciona de verdade (ao contrário do REST, que só reconhece a sintaxe), validado com o SDK oficial do MCP

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
comando novo. Diferente do suporte a REST (`WSRESTFUL`/`@Get`/`@Post`),
que hoje só reconhece a sintaxe e descarta (sem servidor HTTP nem
despacho real), o `MCPServer` **funciona de verdade**: implementa o
protocolo MCP (Model Context Protocol) via JSON-RPC 2.0 sobre stdio —
`initialize`, `tools/list`, `tools/call` — em Go puro, sem CGO, sem
dependências externas. Cada tool chamada roda a função AdvPL
correspondente numa VM isolada (mesmo mecanismo do `StartJob`).

| Método | Descrição |
|--------|-----------|
| `New(cNome, cVersao)` | Cria o servidor |
| `AddTool(cNome, cDescricao, cSchemaJSON, cNomeFuncao)` | Registra uma tool; a função recebe um objeto com os argumentos (`oArgs:CAMPO`) e retorna o texto do resultado |
| `Serve()` | Sobe o loop stdio (bloqueia) |

Validado com o SDK oficial em Python do MCP (`cmd/advplc/mcp_integration_test.go`).

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
# Compilar todas as ferramentas (advplc, advcfg, adveditor, advpp-ide)
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

Todas as ferramentas (advplc, advcfg, adveditor, advpp-ide) enxergam o
**mesmo** banco SQLite, resolvido nesta ordem:

1. Flag explícita (`advplc run prog.prw --db-path /caminho/banco.db`)
2. Variável de ambiente `ADVPP_DB`
3. Banco configurado em `~/.advpp/advpp_config.json`
4. Padrão: `~/.advpp/ADVPP.db` (criado automaticamente pelo advcfg)

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
- If/ElseIf/Else/EndIf, For/Next, While/EndDo, Do Case/EndCase
- Tratamento de erro Begin Sequence/Recover/End Sequence
- Blocos de código `{|| expr }`
- Class/EndClass com Data, Method, Constructor
- Implementação de método fora do bloco de classe
- Acesso a campo de alias `SA1->A1_NOME`
- Auto-referência `::property`
- Todos os tipos de dados AdvPL: Character, Numeric, Logical, Date, Array, Code Block, Nil, Object

### Recursos Adicionais TLPP
- Tipagem estática com palavra-chave `as`
- Tratamento de erro Try/Catch/EndTry
- Declarações de namespace
- Modificadores de acesso (Public, Private, Protected)
- Anotações REST (@Get, @Post, @Put, @Delete) - apenas parsing
- Suporte JSON inline com métodos JsonObject
- Identificadores longos (com namespace)
- Tipos Integer, Double, Decimal, Variant, Variadic
- Parsing de sintaxe WSRESTFUL/WSSERVICE

**Nota**: Anotações REST e sintaxe WSRESTFUL são parseadas mas não executadas. Integração de servidor HTTP necessária para execução de endpoints REST.
