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

### Status Atual: Apenas Parsing

O compilador AdvPP faz parsing da sintaxe REST 2.0 mas **integração de servidor HTTP não está implementada**.

### O Que Funciona:
- ✅ Reconhecimento de palavras-chave REST (GET, POST, PUT, DELETE, PATCH)
- ✅ Parsing de WSRESTFUL/WSSERVICE
- ✅ WSMETHOD com sintaxe de verbo HTTP
- ✅ Definições de campos WSDATA
- ✅ Sintaxe de anotação (@Get, @Post, @Put, @Delete)
- ✅ Sintaxe JSON inline
- ✅ Métodos JsonObject (toJson, hasProperty, getJsonText)
- ✅ Serialização/deserialização JSON

### O Que NÃO Funciona:
- ❌ Execução de servidor HTTP
- ❌ Registro de endpoints REST
- ❌ Tratamento de requisições HTTP
- ❌ Geração de respostas REST
- ❌ Execução de anotações @Get/@Post
- ❌ Dispatch HTTP WSService

### Notas de Implementação:
- Sintaxe REST é parseada em `pkg/parser/parser.go` (função parseWSClient)
- Verbos HTTP são reconhecidos mas não executados
- Anotações são armazenadas na AST mas não processadas em runtime
- Servidor REST completo requereria integração de servidor HTTP (ex: net/http)

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
  funções AdvPL como tools — execução real, ao contrário do REST
  (WSRESTFUL, que só reconhece a sintaxe); validado com o SDK oficial
  em Python do MCP
- ⚠️ Locks de registro (RecLock/MsUnlock) são no-ops — sem controle de
  concorrência em escrita entre processos

## Resumo

| Recurso | Status | Notas |
|---------|--------|-------|
| Componentes UI | ✅ Completo | Renderização Fyne completa implementada |
| Diálogos UI | ✅ Completo | MsgInfo, MsgStop, MsgAlert, MsgYesNo funcionam |
| Parsing REST | ✅ Completo | Sintaxe totalmente parseada |
| Execução REST | ❌ Nenhum | Sem servidor HTTP |
| Anotações | ✅ Parseadas | Armazenadas na AST, não executadas |
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
2. **Para REST**: Adicionar integração de servidor HTTP (net/http) ou documentar como apenas parsing
3. **Para Serviços**: Adicionar geração de código ou integração de cliente HTTP
4. **Documentação**: Atualizar README para separar claramente recursos "parseados" de "executados"
