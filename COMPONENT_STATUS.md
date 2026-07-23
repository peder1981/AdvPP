# Relatório de Status dos Componentes

## Componentes UI

### Status Atual: Renderização Visual Implementada

O compilador AdvPP agora inclui renderização completa de widgets Fyne para todos os componentes UI.

### O Que Funciona:
- ✅ Estruturas de dados de componentes (TButton, TGet, TComboBox, TCheckBox, etc.)
- ✅ Estruturas de diálogo (Dialog, MenuBar, ToolBar, StatusBar)
- ✅ Propriedades de componentes (X, Y, Width, Height, Label, Value, etc.)
- ✅ Framework de tratamento de eventos (onChange, onClick, onGotFocus, onLostFocus)
- ✅ **Renderização de widgets Fyne para todos os componentes**
- ✅ **Renderização de TButton**
- ✅ **Renderização de TGet (entrada de texto)**
- ✅ **Renderização de TComboBox**
- ✅ **Renderização de TCheckBox**
- ✅ **Renderização de TLabel**
- ✅ **Renderização de MenuBar**
- ✅ **Renderização de ToolBar**
- ✅ **Renderização de StatusBar**
- ✅ **Renderização de view de formulário com conteúdo scrollable**
- ✅ Diálogos Fyne básicos (MsgInfo, MsgStop, MsgAlert, MsgYesNo)
- ✅ Componentes UI da IDE (CodeEditor, OutputConsole, FileTree)

### O Que NÃO Funciona:
- ❌ Execução de eventos de componentes (manipuladores definidos mas não conectados)
- ❌ Atualizações dinâmicas de componentes (sem two-way binding)

### Notas de Implementação:
- Componentes são definidos como structs Go em `pkg/mvc/view.go`
- Renderização Fyne implementada em `pkg/ui/renderer.go`
- Executável de teste visual: `./ui-test` (em `cmd/ui-test/`)
- Renderização completa de componentes agora funcional

## Recursos REST 2.0

### Status Atual: Funcional (estilo anotações) / Apenas Parsing (DSL clássico WSRESTFUL)

O compilador AdvPP sobe um servidor HTTP REST **real** (`pkg/rest` + classe
nativa `WSRestServer`, mesmo padrão arquitetural do `MCPServer`) para o
estilo moderno TLPP de anotações (`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete`
sobre `User Function`). O DSL clássico `WSRESTFUL ... WSMETHOD ...
ENDWSRESTFUL` continua **apenas parseado** — ver limitação abaixo.

### O Que Funciona:
- ✅ **Servidor HTTP real** (`net/http` puro, sem CGO/dependências): classe
  `WSRestServer` — `New()`, `AddRoute(verbo, path, funcao)`, `Serve(porta)`
- ✅ **Auto-discovery de rotas via anotação**: toda `User Function`
  anotada com `@Get("/path")`/`@Post(...)`/`@Put(...)`/`@Patch(...)`/
  `@Delete(...)` vira rota automaticamente ao criar o `WSRestServer`
- ✅ **Path params** (`/clientes/{id}`) via roteador nativo do Go 1.22+
  (`http.ServeMux`), populados no objeto de argumento da função AdvPL
- ✅ **Query string e corpo JSON** decodificados e mesclados no objeto
  de argumento (`oParam:CAMPO`) — corpo tem precedência sobre path
  params, que tem precedência sobre query string
- ✅ **Dispatch real para a função AdvPL** via VM isolada por requisição
  (mesmo mecanismo do `MCPServer`/`StartJob`: `v.RunFunction`), banco e
  bytecode compartilhados
- ✅ **Resposta JSON automática** do retorno da função (200), erro vira
  500, path inexistente vira 404, verbo não registrado num path existente
  vira 405
- ✅ Registro manual de rota via `AddRoute` (cobre casos onde a anotação
  não é usada)
- ✅ Sintaxe JSON inline, métodos JsonObject (toJson, hasProperty,
  getJsonText), serialização/deserialização JSON

### O Que NÃO Funciona (limitações conhecidas):
- ❌ **DSL clássico `WSRESTFUL <nome> ... WSMETHOD <verbo> ... PATH "..."
  ... ENDWSRESTFUL`**: o parser (`parseWSClient` em
  `pkg/parser/parser.go`) reconhece a sintaxe e monta um `ast.ClassDecl`,
  mas descarta o verbo HTTP e a cláusula `PATH` ao fazer isso — só o
  nome do `WSMETHOD` sobrevive como protótipo. Além disso, a
  implementação real do método (`WSMETHOD ... WSSERVICE <classe>`) é um
  método de instância, e `v.RunFunction` (usado pelo dispatch HTTP) só
  chama funções top-level, não métodos de classe — precisaria criar a
  instância do serviço a cada requisição. Nenhuma das duas partes foi
  implementada: exigiria cirurgia de parser (capturar verbo+path como
  antes) e um caminho de dispatch novo (instanciar + chamar método) só
  para esse estilo, sem um caso de uso real nos corpora validados para
  justificar o esforço agora. Workaround: reescrever o serviço no estilo
  anotações (`@Get`/`@Post` sobre `User Function`), que já é 100%
  funcional, ou registrar a rota manualmente via `AddRoute`.
- ❌ Geração de WSDL / cliente REST (`WSCLIENT` consumindo serviço
  externo) — fora de escopo, é consumo e não exposição de API
- ❌ Controle fino de código HTTP de resposta pela função AdvPL (sempre
  200 em sucesso / 500 em erro — sem equivalente a `SetLegacySuccess`/
  `GetHTTPCode` do lado servidor)

### Notas de Implementação:
- Servidor: `pkg/rest/rest.go` (stdlib `net/http`, roteador nativo do Go
  1.22+, sem dependências externas)
- Ponte VM: `pkg/vm/rest_native.go` (classe nativa `WSRestServer`,
  conversão JSON↔`advplrt.Value`, auto-discovery via
  `FunctionInfo.Annotations` do bytecode)
- Fixture de teste: `tests/rest_server_test.prw`; teste de integração
  Go (builda o binário, sobe o servidor, faz requisições HTTP reais):
  `cmd/advplc/rest_integration_test.go` (`TestRestServerFixture`)

## Construção de Serviços

### Status Atual: Parcial

### O Que Funciona:
- ✅ Parsing de sintaxe WSCLIENT/WSSTRUCT/WSRESTFUL
- ✅ Definições de protótipo WSMETHOD
- ✅ Definições de campos WSDATA
- ✅ Metadados de serviço (DESCRIPTION, NAMESPACE)
- ✅ Criação e manipulação de objetos JSON

### O Que NÃO Funciona:
- ❌ Geração de código WSDL
- ❌ Geração de código de cliente REST
- ❌ Invocação de serviço
- ❌ Integração de cliente HTTP

## Banco de Dados e Multi-thread (atualizado 2026-07-08)

### Status Atual: Funcional

- ✅ **Banco SQLite compartilhado**: todas as ferramentas (advplc,
  adveditor, advpp-ide) resolvem o mesmo banco via
  `shared.ResolveDatabasePath` (flag → `ADVPP_DB` → config real em disco
  → `./advpp.db` local do diretório de trabalho, criado automaticamente)
- ✅ **Driver 100% Go** (modernc.org/sqlite, sem CGO) com WAL + busy_timeout
- ✅ **Natives de banco conectados ao VM**: DBSelectArea, DBSeek, DBSkip,
  RecCount, FieldGet/FieldPut etc. operam sobre o banco real
- ✅ **StartJob(cFunc, cEnv, lWait, params...)**: execução em VM isolado
  (goroutine), síncrona ou assíncrona, com conexão própria ao banco
- ✅ **FWGridProcess**: pool de threads com backpressure (SetThreadGrid,
  CallExecute, StopExecute, IsFinished, meters, SaveLog)
- ✅ **advplc check paralelo**: N arquivos com 1 worker por CPU
- ✅ **Renderer web (advplc serve)**: PO-UI embutido no binário; console,
  diálogos, FWMBrowse→po-table com dicionário SX3, po-dynamic-form,
  MSDIALOG legado por heurística de grade e hot reload (--watch)
- ✅ **Motor de inferência LLM (classe `LLM`)**: modelos GGUF I2_S
  (BitNet/Falcon3-1.58bit) via `pkg/llm`, 100% Go sem CGO, com kernel
  SIMD AVX2 em amd64 (fallback escalar em qualquer outra arquitetura),
  validado token a token contra o `llama.cpp` de referência
- ✅ **Servidor MCP nativo (classe `MCPServer`)**: JSON-RPC 2.0 real
  sobre stdio via `pkg/mcp` (initialize/tools.list/tools.call), expõe
  funções AdvPL como tools — execução real; validado com o SDK oficial
  em Python do MCP
- ✅ **Servidor REST nativo (classe `WSRestServer`)**: HTTP real via
  `pkg/rest` (`net/http` puro), auto-discovery de rotas por anotação
  (`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete`) ou registro manual via
  `AddRoute`, path params, dispatch para a função AdvPL via
  `v.RunFunction` — mesmo padrão do `MCPServer`; o DSL clássico
  `WSRESTFUL`/`WSMETHOD` continua só parseado (ver "Recursos REST 2.0")
- ⚠️ Locks de registro (RecLock/MsUnlock) são no-ops — sem controle de
  concorrência em escrita entre processos

## Resumo

| Recurso | Status | Notas |
|---------|--------|-------|
| Componentes UI | ✅ Completo | Renderização Fyne completa implementada |
| Diálogos UI | ✅ Completo | MsgInfo, MsgStop, MsgAlert, MsgYesNo funcionam |
| Parsing REST | ✅ Completo | Sintaxe totalmente parseada |
| Execução REST (anotações @Get/@Post) | ✅ Funcional | Servidor HTTP real (`WSRestServer`), dispatch para a função AdvPL |
| Execução REST (DSL WSRESTFUL/WSMETHOD) | ❌ Nenhum | Apenas parsing — ver "Recursos REST 2.0" |
| Anotações @Get/@Post/@Put/@Patch/@Delete | ✅ Executadas | Viram rota HTTP automaticamente |
| Suporte JSON | ✅ Completo | Sintaxe inline e JsonObject funcionam |
| Construção de Serviços | ⚠️ Parcial | Parseado, não gerado |

## Compatibilidade com IDE

### Status Atual: 100% Compatível

O compilador AdvPP e todos os componentes UI são totalmente compatíveis com a IDE AdvPP.

### Resultados de Testes
- ✅ Todos os 8 arquivos de teste existentes passam
- ✅ Componentes MVC funcionam no contexto da IDE
- ✅ Integração de provider UI funciona
- ✅ Saída do compilador com componentes UI funciona
- ✅ Execução da VM com renderização UI funciona
- ✅ Funções de diálogo (MsgInfo, MsgStop, MsgAlert, MsgYesNo) funcionam
- ✅ Suporte JSON funciona
- ✅ Funções nativas funcionam
- ✅ Estruturas de controle funcionam
- ✅ Arrays funcionam
- ✅ Funções de string funcionam

### Teste de Integração IDE
```bash
./advplc run tests/ide_integration_test.prw
```

Todos os testes passaram - 100% de compatibilidade com IDE verificada.

## Teste de Renderização UI

Para testar a renderização UI:
```bash
go build -o ui-test ./cmd/ui-test
./ui-test
```

Isso exibirá uma janela com:
- TLabel (título)
- TGet (entrada de texto)
- TComboBox (dropdown)
- TCheckBox (checkbox)
- TButton (botões)
- ToolBar (topo)
- StatusBar (fundo)

## Recomendações

1. **Para Componentes UI**: Implementar sistema de renderização de widgets Fyne ou documentar como apenas dados
2. **Para REST**: DSL clássico `WSRESTFUL`/`WSMETHOD` ainda precisa de captura de verbo+path no parser e dispatch via instância de classe (hoje `v.RunFunction` só chama funções top-level) — sem caso de uso real nos corpora para priorizar agora
3. **Para Serviços**: Adicionar geração de código ou integração de cliente HTTP
4. **Documentação**: Atualizar README para separar claramente recursos "parseados" de "executados"
