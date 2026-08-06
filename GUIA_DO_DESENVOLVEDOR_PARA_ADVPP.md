# Guia do Desenvolvedor — AdvPP Compiler

**Compilador AdvPL/TLPP em Go** · `github.com/advpl/compiler` · **105 arquivos .go** · ~32.866 linhas

---

## Índice

1. [Visão Geral da Arquitetura](#1-visao-geral-da-arquitetura)
2. [CLI — advplc](#2-cli--advplc)
3. [Pipeline de Compilação](#3-pipeline-de-compila%C3%A7%C3%A3o)
    - 3.1 [Conversão de Codificação (CP-1252 → UTF-8)](#31-convers%C3%A3o-de-codifica%C3%A7%C3%A3o-cp-1252--utf-8)
    - 3.2 [Preprocessor](#32-preprocessor)
    - 3.3 [Lexer](#33-lexer)
    - 3.4 [Parser](#34-parser)
    - 3.5 [Code Generation / Bytecode](#35-code-generation--bytecode)
4. [Máquina Virtual (VM)](#4-m%C3%A1quina-virtual-vm)
    - 4.1 [Tipos de Valor (`pkg/runtime/values.go`)](#41-tipos-de-valor-pkgruntimevaluesgo)
    - 4.2 [Ambiente de Variáveis (`pkg/runtime/environment.go`)](#42-ambiente-de-vari%C3%A1veis-pkgruntimeenvironmentgo)
    - 4.3 [VM Core (`pkg/vm/vm.go`)](#43-vm-core-pkgvmvmgo)
    - 4.4 [Funções Nativas Registradas (~250+)](#44-fun%C3%A7%C3%B5es-nativas-registradas-250)
        - 4.4.1 [TUI / Terminal (`pkg/vm/ui_render.go`)](#441-tui--terminal-pkgvmui_rendergo)
    - 4.5 [Sistema de Diálogo (MsDialog)](#45-sistema-de-di%C3%A1logo-msdialog)
    - 4.6 [Browse (FWMBrowse Auto-CRUD)](#46-browse-fwmbrowse-auto-crud)
    - 4.7 [Grid Process (FWGridProcess)](#47-grid-process-fwgridprocess)
    - 4.8 [Macro Substitution (&)](#48-macro-substitution)
    - 4.9 [Debugger](#49-debugger)
    - 4.10 [Tensor (Operações com Tensores)](#410-tensor-opera%C3%A7%C3%B5es-com-tensores)
    - 4.11 [Autograd (Diferenciação Automática)](#411-autograd-diferencia%C3%A7%C3%A3o-autom%C3%A1tica)
    - 4.12 [LLM (Inferência GGUF)](#412-llm-infer%C3%AAncia-gguf)
    - 4.13 [TMailMessage (E-mail SMTP)](#413-tmailmessage-e-mail-smtp)
    - 4.14 [MCPServer (Model Context Protocol)](#414-mcpserver-model-context-protocol)
    - 4.15 [WSRestServer (REST API)](#415-wsrestserver-rest-api)
5. [Framework MVC](#5-framework-mvc)
6. [UI Desktop (Fyne)](#6-ui-desktop-fyne)
7. [UI Web (PO-UI)](#7-ui-web-po-ui)
8. [Banco de Dados SQLite](#8-banco-de-dados-sqlite)
9. [Protocolos (DAP, MCP, REST)](#9-protocolos-dap-mcp-rest)
10. [Build Standalone](#10-build-standalone)
11. [Tabela de Opcodes do Bytecode](#11-tabela-de-opcodes-do-bytecode)
12. [Resumo de Coverage](#12-resumo-de-coverage)

---

## 1. Visão Geral da Arquitetura

```
┌─────────────────────────────────────────────────────────┐
│                    advplc CLI                            │
│  run | compile | exec | check | serve | debug | build   │
└──────────────────────┬──────────────────────────────────┘
                       │
          ┌────────────▼────────────┐
          │   Preprocessor (.cpp)   │  #include, #define, #ifdef, BEGINSQL, #xcommand
          └────────────┬────────────┘
                       │
          ┌────────────▼────────────┐
          │       Lexer (Tokenizer) │  Source → []Token (CP-1252 auto-detect)
          └────────────┬────────────┘
                       │
          ┌────────────▼────────────┐
          │      Parser (AST)       │  []Token → ast.Program (~200 node types)
          └────────────┬────────────┘
                       │
          ┌────────────▼────────────┐
          │   Code Generator        │  AST → Bytecode (.abf JSON)
          │   (Opcode 0..61)        │  Stack-based VM instructions
          └────────────┬────────────┘
                       │
          ┌────────────▼────────────┐
          │     VM Execution Engine │  Run() / RunFunction() / StartJob()
          │   + 250+ Native Fns     │  Strings, Math, Date, Array, File, DB, UI, TUI
          └────────────┬────────────┘
                       │
         ┌─────────────┼──────────────────┐
         │             │                  │
    ┌────▼────┐   ┌────▼────┐       ┌────▼────┐
    │ SQLite  │   │  DAP    │       │  Web UI │
    │  DB     │   │ Debug   │       │ SSE/WS  │
    └─────────┘   └─────────┘       └─────────┘
```

### Estrutura de Pacotes

| Pacote | Arquivos | Responsabilidade |
|--------|----------|------------------|
| `cmd/advplc` | main.go (+17 testes) | CLI principal: compile, run, serve, debug, build, check |
| `cmd/advpp-ide` | main.go | IDE desktop completo com editor Fyne |
| `cmd/adveditor` | main.go | Editor de código com banco SQLite |
| `pkg/compiler` | 6 arquivos | Bytecode compiler, opcodes, standalone builder |
| `pkg/lexer` | 2 arquivos | Tokenizer do idioma AdvPL/TLPP |
| `pkg/parser` | 2 arquivos | Parser recursivo descendente, expressions |
| `pkg/ast` | 1 arquivo | ~200 tipos de nós AST |
| `pkg/preprocessor` | 2 arquivos | #include, #define, #ifdef, #xcommand, SQL blocks |
| `pkg/runtime` | 3 arquivos | Tipos de valor dinâmico, ambiente de escopo |
| `pkg/vm` | 15 arquivos | VM execution engine + todas as nativas |
| `pkg/mvc` | 4 arquivos | FWFormModel, FWFormView, FWFormBrowse |
| `pkg/ui` | 11 arquivos | Widgets Fyne: editor, console, file tree, renderer MVC |
| `pkg/webui` | 1 arquivo | Servidor HTTP com SSE para modo web PO-UI |
| `pkg/db` | 1 arquivo | Motor SQLite com workarea Protheus |
| `pkg/dap` | 3 arquivos | Debug Adapter Protocol |
| `pkg/mcp` | 2 arquivos | Model Context Protocol JSON-RPC |
| `pkg/rest` | 1 arquivo | Servidor REST HTTP |
| `pkg/tensor` | 4 arquivos | Tensor N-dimensional, linear algebra |
| `pkg/autograd` | 5 arquivos | Reverse-mode autodiff |
| `pkg/nn` | 2 arquivos | Camadas neurais (Linear, Embedding) |
| `pkg/llm` | 11 arquivos | Inferência GGUF I2_S (BitNet), tokenizers |
| `pkg/tools/shared` | 5 arquivos | Configuração compartilhada, database, treeview |

---

## 2. CLI — advplc

### Comandos

| Comando | Descrição | Exemplo |
|---------|-----------|---------|
| `run` | Compila e executa um fonte AdvPL | `advplc run programa.prw` |
| `compile` | Compila para bytecode (.abf JSON) | `advplc compile programa.prw -o out.abf` |
| `exec` | Executa bytecode já compilado | `advplc exec out.abf` |
| `check` | Verifica sintaxe sem executar (multi-arquivo) | `advplc check a.prw b.prw c.prw` |
| `serve` | Modo web — UI no browser via PO-UI/Angular | `advplc serve programa.prw --port 9000` |
| `debug` | DAP server para IDE debugging | `advplc debug` (pipe stdin/stdout do editor) |
| `build` | Build executável standalone | `advplc build programa.prw -o Programa` |
| `ast` | Imprime a estrutura AST | `advplc ast programa.prw` |
| `bytecode` | Imprime bytecode instructions | `advplc bytecode programa.prw` |
| `version` / `-v` | Versão do compilador | `advplc -v` |

### Opções Globais

| Opção | Descrição |
|-------|-----------|
| `--include <path>`, `-I <path>` | Adiciona caminho para busca de includes (repetível) |
| `--define <nome=val>`, `-D` | Define símbolo no preprocessor |
| `--ui` | Habilita interface gráfica (Fyne desktop) |
| `--headless` | Desabilita interface (padrão) |
| `--db <backend>` | Backend de banco: `sqlite` (padrão) ou `odbc` |
| `--db-path <caminho>` | Path do arquivo SQLite |
| `--port <n>` | Porta do servidor web (modo serve) |
| `--watch`, `-w` | Hot reload: recompila ao detectar mudança no fonte |
| `--debug-port <n>` | Abre listener TCP DAP para attach debugging |
| `-o <arquivo>` | Arquivo de saída (para `compile` e `build`) |
| `--gui` | `build`: fixa o binário como app desktop — janela Fyne sempre, sem tentar console; no Windows linka no subsistema GUI (`-H=windowsgui`, ldflags, ver `goBuildArgs` em `pkg/compiler/standalone.go`), sem flash de console. Obrigatório para programas com `MSDIALOG`. Sem essa flag, o binário standalone decide sozinho em runtime (console se TTY + nada de UI detectado no bytecode; Fyne caso contrário) — ver `pkg/compiler/stub_template.go`. `ADVPP_FORCE_GUI=1` é o equivalente em runtime, mais fraco (não muda o subsistema Windows). |

### Pipeline Interno de `loadAndCompile()`

1. `os.ReadFile(sourceFile)` — lê bytes brutos
2. `convertToUTF8()` — detecta CP-1252 e converte para UTF-8 (tabela lookup 256 bytes)
3. `preprocessor.Process()` — processa #include, #define, #ifdef, BEGINSQL, #xcommand
4. `lexer.Tokenize()` — gera `[]Token` da fonte pré-processada
5. `parser.Parse()` — gera `*ast.Program`
6. `compiler.Compile()` — gera `*compiler.Bytecode`

---

## 3. Pipeline de Compilação

### 3.1 Conversão de Codificação (CP-1252 → UTF-8)

**Função:** `convertToUTF8(source []byte) (string, error)` em `cmd/advplc/main.go`

Detecta se o source é UTF-8 válido. Se não, assume CP-1252 e converte usando uma tabela lookup de 256 entradas construída em `init()`. Blocos ASCII contíguos são copiados de uma vez (`buf.Write(source[start:i])`), maximizando performance.

**Cobertura CP-1252:** Mapeia todos os 256 valores de byte incluindo caracteres especiais em 0x80–0x9F (€, ‚, ..., †, ‡, ˆ, ‰, Š, ‹, Œ, Ž, '' "", •, –, —, ˜, ™, etc.)

### 3.2 Preprocessor (`pkg/preprocessor/`)

**Tipo Principal:** `Preprocessor`

| Método | Descrição |
|--------|-----------|
| `NewPreprocessor(includePaths []string)` | Cria preprocessor com caminhos de busca para #include |
| `Process(source, fileName string) (string, error)` | Aplica todos os preprocessamentos, retorna texto transformado |
| `GetDefines() map[string]string` | Retorna defines ativos (#define) |
| `RootBoundaryLine int` | Linha limite — includes aparecem antes desta linha |

#### Diretivas Suportadas

| Diretiva | Comportamento |
|----------|--------------|
| `#include <file>` | Carrega recursivamente (depth max 30); inclui antes do RootBoundaryLine |
| `#define nome valor` | Armazena no mapa defines; suporta continuação com `;` |
| `#undefine` / `#undef nome` | Remove define |
| `#ifdef nome` / `#ifndef nome` | Condição de compilação condicional (aninhável) |
| `#else` / `#endif` | Controle de ramos condicionais |
| `#xcommand ... => ...` / `#command` | Macro DSL: pattern → result template (recompilação automática de código) |
| `#xtranslate ... => ...` / `#translate` | Similar a #command para tradução de tokens |
| `BEGINSQL [ALIAS nome]` | Coleta SQL até ENDSQL; converte em string-building AdvPL |
| Continuação de linha (`;` no final) | Junta próxima linha física |

#### Command Rules Engine (#xcommand)

Compile padrões DSL em regras internas (`commandRule`) que são aplicadas a cada linha de código. Pipeline:

1. `parseCommandDef(def)` — separa `pattern => result`
2. `compilePattern(s)` — cria tokens: literais, marcadores `<name>`, opcionais `[...]`, restritos `<name:LIT1,LIT2>`
3. `matchPattern(pattern, tokens)` — backtracking recursive matcher
4. `expandResult(result, match)` — substitui marcadores por texto capturado: `<name>` = raw text, `<{name}>` = codeblock `{|| text}`, `<.name.>` = boolean `.T./.F.`

### 3.3 Lexer (`pkg/lexer/`)

**Tipo Principal:** `Lexer`

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `Tokenize(source, fileName string) ([]Token, error)` | fonte + nome arquivo | tokens | Cria lexer e tokeniza em uma chamada |
| `FilterTokens(tokens []Token) ([]Token, error)` | tokens brutos | tokens limpos | Remove TOKEN_NEWLINE, LINECOMMENT, BLOCKCOMMENT |

#### Token Types (~43 tipos)

```
Literais:         TOKEN_NUMBER, TOKEN_STRING, TOKEN_IDENT
Lógicos:          TOKEN_TRUE (.T./.Y.), TOKEN_FALSE (.F./.N.), TOKEN_NIL (.NULL./.NIL.)
Aritméticos:      TOKEN_PLUS(+), TOKEN_MINUS(-), TOKEN_STAR(*), TOKEN_SLASH(/), TOKEN_PERCENT(%)
Comparação:       TOKEN_EQ(==), TOKEN_NEQ(!=/</>#), TOKEN_LT(<), TOKEN_GT(>), TOKEN_LTE(<=), TOKEN_GTE(>=)
Lógicos prefix:   TOKEN_DOT_AND(.AND.), TOKEN_DOT_OR(.OR.), TOKEN_DOT_NOT(.NOT.)
Incremento:       TOKEN_INCREMENT(++), TOKEN_DECREMENT(--)
Potência:         TOKEN_CARET(^), TOKEN_DOUBLESTAR(**)
Ponctuação:       LPAREN, RPAREN, LBRACKET, RBRACKET, LBRACE, RBRACE, SEMICOLON, COMMA, DOT(.), COLON(:), DOUBLECOLON(::), ARROW(->)
Avanço:           AT(@), AMPERSAND(&), PIPE(|), TILDE(~), DOLLAR($)
Pré-processador:  TOKEN_PREPROC_INCLUDE, DEFINE, UNDEFINE, IFDEF, IFNDEF, ELSE, ENDIF, XCOMMAND, XTRANSLATE, COMMAND, TRANSLATE, DIRECTIVE
Comentários:      TOKEN_LINECOMMENT(//,&&), TOKEN_BLOCKCOMMENT(/* */)
```

#### Keywords (80+ palavras reservadas)

`FUNCTION`, `STATIC`, `USER`, `PROCEDURE`, `RETURN`, `MAIN`, `IF`, `ELSEIF`, `ELSE`, `ENDIF`, `FOR`, `TO`, `STEP`, `NEXT`, `WHILE`, `ENDDO`, `END`, `DO`, `CASE`, `ENDCASE`, `OTHERWISE`, `EXIT`, `LOOP`, `BREAK`, `CONTINUE`, `BEGIN`, `SEQUENCE`, `RECOVER`, `USING`, `TRY`, `CATCH`, `FINALLY`, `ENDTRY`, `THROW`, `LOCAL`, `PRIVATE`, `PUBLIC`, `GLOBAL`, `PARAMETERS`, `CLASS`, `ENDCLASS`, `DATA`, `METHOD`, `CONSTRUCTOR`, `FROM`, `AS`, `OPERATOR`, `INTERFACE`, `ENDINTERFACE`, `IMPLEMENTS`, `VARIANT`, `VARIADIC`, `JSON`, `INTEGER`, `DOUBLE`, `DECIMAL`, `CHARACTER`, `NUMERIC`, `LOGICAL`, `DATE`, `ARRAY`, `OBJECT`, `MEMO`, `FIXED`, `UNDEFINED`, `AND`, `OR`, `NOT`, `IN`, `IS`, `OF`, `ON`, `THREAD`, `JOB`, `EXPORT`, `ENUM`, `ENDENUM`, `REST`, `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `NAMESPACE`, `SAY`, `SIZE`, `PIXEL`, `COORD`, `TITLE`, `ACTION`, `WHEN`, `VALID`, `UPDATE`, `READ`, `ADD`, `ACTIVATE`, `WINDOW`, `DIALOG`, `BUTTON`, `SBUTTON`, `MENU`, `BROWSE`, `REPORT`, `SECTION`, `PANEL`, `BLOCK`, `INCLUDE`, `TRANSLATE`, `COMMAND`, `UNDEFINE`, `IFDEF`, `IFNDEF`

#### Comportamentos Especiais do Lexer

- **NBSP (0xC2 0xA0):** Detecta UTF-8 encode de non-breaking space
- **Bracket strings:** `[texto]` funciona como literal string ou array index dependendo do contexto
- **Dot literals:** `.T.`, `.F.`, `.Y.`, `.N.`, `.NULL.`, `.AND.`, `.OR.`, `.NOT.` são tokens unitários
- **Continuação de linha:** `;` no final da linha une com próxima linha física

### 3.4 Parser (`pkg/parser/`)

**Tipo Principal:** `Parser`

| Método | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `NewParser(tokens, fileName, defines)` | tokens lexados | `*Parser` | Cria parser com tokens |
| `Parse() (*ast.Program, error)` | — | AST root | Parseia stream de tokens em programa completo |

#### Estratégia de Parsing

- **Statements:** Pattern keyword-dispatch — peeks primeiro token, matching keyword, dispatch handler específico
- **Expressions:** Pratt top-down operator precedence chain

#### Precência de Operadores (lowest → highest)

```
parseExpression
  → parseOr (.Or./OR)
  → parseAnd (.And./AND)
  → parseNot (.Not./!)
  → parseComparison (==, =, !=, <=>, <=, >=, $)
  → parseAddition (+, -)
  → parseMultiplication (*, /, %)
  → parsePower (^, **) ← right associative
  → parseUnary (-, +, ++, --)
  → parsePostfix (:method(), [idx], ->field, (), ++, -- suffix)
  → parsePrimary (literals, identifiers, macros, JSON)
```

#### DSL Commands Desugaring

Todos os commands legacy AdvPL (DEFINE/REDEFINE, @SCREEN SAY/GET, CLIPPER DB commands) são parsados e convertidos em `CallExpr` — preservam a estrutura syntax sem requerer runtime engine dedicado. São "degenerated" em chamadas de função nomeadas como `DESLUGGED_DEFINE_MS_DIALOG`, etc.

### 3.5 Code Generation / Bytecode (`pkg/compiler/`)

**Tipo Principal:** `Compiler`

| Método | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `New()` | — | `*Compiler` | Cria novo compiler com bytecode vazio |
| `Compile(program *ast.Program)` | AST tree | `*Bytecode, error` | **Entry point** — compila todo AST para bytecode |

#### Otimizações do Codegen

- **Constant pool deduplication:** números e strings repetidos compartilham referência
- **Inline IF()/IIF():** special case — short-circuit evaluation (só avalia ramo escolhido)
- **EVAL(codeblock):** usa `OP_EVAL_CODEBLOCK` ao invés de `OP_CALL_NATIVE`
- **Closure upvalue capture:** resolve variáveis de frames parentais para closures
- **Global bit flag:** locals globais são marcados com bit 0x8000 no slot index
- **Reserved function names:** builtins (ERRORCLASS, JSONOBJECT, FWFORMVIEW, etc.) vão para `OP_NEW_INSTANCE`
- **Named parameters:** cada argumento tem marker `OP_NAMED_ARG` antes do valor real

#### Classes Built-in Geradas Automaticamente

`ERRORCLASS`, `JSONOBJECT`, `JSONARRAY`, `FWFORMVIEW`, `FWFORMMODEL`, `FWFORMBROWSE`, `FWGRIDPROCESS`, `FWMBROWSE`, `LLM`, `MCPSERVER`, `WSRESTSERVER`, `TMAILMESSAGE`, `TENSOR`, `VARIABLE`, `SGD`, `ADAM`, `LINEAR`, `EMBEDDING`

#### Serialização de Bytecode

| Função | Descrição |
|--------|-----------|
| `SaveBytecode(bc, filename string)` | Marshal → JSON file |
| `LoadBytecode(filename string)` | Unmarshal JSON → `*Bytecode` |
| `BuildStandaloneGUI(bc, outputFile, title, gui, logWriter)` | Embed bytecode em stub Go compilável → executável native (`gui bool` = flag `--gui`, ver seção 10) |

---

## 4. Máquina Virtual (VM)

### 4.1 Tipos de Valor (`pkg/runtime/values.go`)

Interface base: `Value`

| Método | Retorno | Descrição |
|--------|---------|-----------|
| `Type()` | `string` | Character de tipo: "N"/"C"/"L"/"D"/"A"/"B"/"O"/"U"/"E" |
| `String()` | `string` | Representação textual |
| `IsTruthy()` | `bool` | Truthiness: nil/empty = false |
| `Equals(other Value)` | `bool` | Igualdade semântica |
| `IsNil()` | `bool` | É NIL? |

#### Tipos Implementados

| Tipo | Struct | Field | Description |
|------|--------|-------|-------------|
| Undefined/Nil | `NilValue` | (singleton) | AdvPL `NIL` — `Type()` = "U" |
| Number | `NumberValue` | `Val float64` | Caching de inteiros -256..4096 |
| String | `StringValue` | `Val string` | Strings UTF-8 |
| Logical | `BoolValue` | `Val bool` | Singleton True/False |
| Date | `DateValue` | `Val time.Time` | Dates como `time.Time` Go |
| Array | `ArrayValue` | `Elements []Value` | Arrays multidimensionais |
| Code Block | `CodeBlockValue` | `Params, Body, Env, FuncName, Upvalues` | Closures com upvalues |
| Object | `ObjectValue` | `ClassName, Props map[string]Value, Keys, Class, Native` | JsonObject e objetos framework |
| Function | `FunctionValue` | `Name, Fn func([]Value)(Value, error)` | Native functions da VM |
| Error | `ErrorValue` | `Description, Severity, Stack, ClassName, GenCode` | Catchable via Try/Catch |

#### Funções Fábrica

| Função | Parâmetros | Retorno |
|--------|-----------|---------|
| `Nil()` | — | `*NilValue` |
| `True()` / `False()` | — | `*BoolValue` |
| `NewNumber(v float64)` | valor numérico | `*NumberValue` |
| `NewString(s string)` | texto | `*StringValue` |
| `NewBool(b bool)` | booleano | `*BoolValue` |
| `NewDate(t time.Time)` | data/hora | `*DateValue` |
| `NewArray(elems []Value)` | elementos | `*ArrayValue` |
| `NewError(desc string)` | descrição | `*ErrorValue` |
| `ToFloat(v Value)` | qualquer valor | `float64` |
| `ToString(v Value)` | qualquer valor | `string` |
| `ToBool(v Value)` | qualquer valor | `bool` |
| `ValType(v Value)` | qualquer valor | `string` ("N", "C", "L", "A", "B", "O", "U", "D") |

### 4.2 Ambiente de Variáveis (`pkg/runtime/environment.go`)

Implementa variáveis Private/Public com scoped chaining.

| Método | Descrição |
|--------|-----------|
| `NewEnvironment(parent)` | Cria scope vinculado a parent |
| `Define(name, value)` | Define variável no scope atual (uppercase normalizado) |
| `Get(name)` | Lookup na cadeia (scope atual → pais → avô...) |
| `Set(name, value)` | Set no scope atual ou ancestral |
| `Has(name)` | Verifica existência |

### 4.3 VM Core (`pkg/vm/vm.go`)

#### Tipos Principais

**`DBEngine` interface** — abstração de workarea Protheus

| Método | Retorno | Descrição |
|--------|---------|-----------|
| `SelectArea(alias string)` | error | Seleciona área de trabalho (tabela) |
| `Seek(key string)` | (bool, error) | Busca chave na ordem atual |
| `Skip(count int)` | error | Move N registros |
| `GoTop()` | error | Posiciona no primeiro |
| `GoBottom()` | error | Posiciona no último |
| `EOF()` | bool | No fim do arquivo? |
| `BOF()` | bool | Antes do início? |
| `FieldGet(field string)` | (Value, error) | Lê campo |
| `FieldPut(field string, val Value)` | error | Escreve campo |
| `RecLock()` | error | Bloqueia registro |
| `MsUnlock()` | error | Desbloqueia e salva |
| `Append()` | error | Insere novo registro |
| `RecCount()` | int | Total de registros |
| `RecNo()` | int | Número do registro atual (1-based) |
| `FieldPos(field string)` | int | Posição do campo (1-based) |

**`UIProvider` interface** — abstração de interação com usuário

| Método | Retorno | Descrição |
|--------|---------|-----------|
| `MsgInfo(msg, title string)` | — | Caixa de informação |
| `MsgStop(msg, title string)` | — | Caixa de erro |
| `MsgAlert(msg, title string)` | — | Caixa de alerta |
| `MsgYesNo(msg, title string)` | bool | Confirmação sim/não |
| `Menu(items []string, title string)` | int | Menu seleção (1-based, 0=cancelado) |
| `InputText(prompt, def string, bIsPassword bool)` | string | Input de texto (maskable) |
| `Browse(spec []byte)` | `[]byte` | Renderiza FWMBrowse (web/desktop) |
| `Dialog(spec []byte)` | `[]byte` | Renderiza MSDIALOG (web/desktop) |

**`SignalKind`** enum: `SigNone`, `SigReturn`, `SigExit`, `SigLoop`, `SigBreak`

#### Métodos da VM

| Método | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `NewVM(bc, uiEnabled)` | bytecode, flag UI | `*VM` | Factory: registra classes + nativas, cria stack vazia |
| `SetDBEngine(engine)` | `DBEngine` | — | Define engine de banco |
| `SetDBFactory(factory)` | `func() DBEngine` | — | Factory per-job (StartJob abre conexão própria) |
| `SetUIProvider(provider)` | `UIProvider` | — | Define provider de interação |
| `SetOutputWriter(w)` | `io.Writer` | — | Redireciona output (Console/SSE) |
| `Run()` | — | `(Value, error)` | **Executa bytecode principal**, espera background jobs |
| `RunFunction(name, args)` | nome, argumentos | `(Value, error)` | Executa função específica (case-insensitive) |
| `StartJob(funcName, wait, args...)` | nome, wait, params | `error` | Spawn VM isolado em goroutine; se wait=true, bloqueia até fim |
| `execute(instr)` | Instruction | `error` | Dispatch de opcode único (~70 case branches) |
| `newInstance(className, args)` | nome classe, args | `error` | Cria objeto instanciado por classe |
| `callBlockSync(cb, args)` | codeblock, args | `Value` | Avalia codeblock síncrono (ASort/AEval/AScan) |
| `registerMVCModel/view/browse(idx, obj)` | — | — | Registra MVC objetos para lookup por Native |

#### Opcodes Implementados no VM (mapa ↔ bytecode)

| Opcode | Instrução VM | Opcodes (bytecode) |
|--------|-------------|---------------------|
| 0 | OP_NIL | Push nil |
| 1 | OP_TRUE | Push true |
| 2 | OP_FALSE | Push false |
| 3 | OP_NUMBER | Push number constant |
| 4 | OP_STRING | Push string constant |
| 5 | OP_DATE | Push date constant |
| 6-7 | LOAD/STORE_LOCAL | Stack ↔ local slot |
| 8-9 | LOAD/STORE_GLOBAL | Stack ↔ global |
| 10-11 | LOAD/STORE_SELF | Object self reference |
| 12-15 | NEW_ARRAY, GET, SET, LEN | Array operations |
| 16-18 | NEW_OBJECT, GET_PROP, SET_PROP | Object property access |
| 19 | CALL_METHOD | obj:Method() |
| 20 | NEW_INSTANCE | ClassName():New() |
| 21-22 | CALL_FUNC, CALL_NATIVE | Function calls |
| 23-24 | RETURN, RETURN_VALUE | Return statements |
| 25 | POP | Discard top of stack |
| 26-28 | JUMP, JUMP_IF_FALSE, JUMP_IF_TRUE | Control flow |
| 29-32 | ADD, SUB, MUL, DIV, MOD, POW, NEG | Arithmetic |
| 31 | EQ, NEQ, LT, GT | Comparison |
| 32 | LTE, GTE, AND, OR, NOT | Logic/comparison |
| 33 | DOLLAR, CONCAT | `$` (contains), `++` |
| 34-35 | NEW_CODEBLOCK, EVAL_CODEBLOCK | Code block creation/evaluation |
| 36-37 | TRY_BEGIN, TRY_END, THROW, CATCH | Exception handling |
| 38-42 | DB_SELECT, DB_SEEK, DB_SKIP, DB_GOTOP, DB_GOBOTTOM, EOF, BOF, FIELD_GET, FIELD_PUT, REC_LOCK, MS_UNLOCK | Database workarea |
| 43-52 | OP_MVC_* | MVC scaffolding (registration only) |
| 53 | MACRO | Macro substitution `&var` |
| 54 | HALT | Program termination |
| 55 | DUP, SWAP | Stack manipulation |
| 56-57 | JUMP_IF_FALSE_OR_POP, POP_AND_JUMP | Short-circuit logic |
| 58 | NAMED_ARG | Named argument marker |
| 59 | FORLOOP_CMP | Step-aware for-loop condition |
| 60-61 | LOAD/STORE_UPVAL, LOAD/STORE_DYN, DECL_DYN | Closures & dynamic vars |

### 4.4 Funções Nativas Registradas (~250+)

Todas registradas em `pkg/vm/natives.go` (e `pkg/vm/ui_render.go`, ver 4.4.1) com
`v.natives[UPPERCASE_NAME] = fn`.

#### Saída / Diálogos

Toda saída que pode conter sequências ANSI (cores, cursor, alt-screen) passa por
`stdoutW` (`pkg/vm/natives.go`), um `io.Writer` — `os.Stdout` puro em
Linux/macOS, e `go-colorable` no Windows (traduz os escapes ANSI em chamadas da
Win32 Console API, já que `cmd.exe`/PowerShell sem
`ENABLE_VIRTUAL_TERMINAL_PROCESSING` os imprimiria literalmente). Garante
paridade visual entre as 3 plataformas para as mesmas sequências de bytes.

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `CONOUT(...)` | variádico | Nil | Imprime no stderr + buffer de output |
| `CONOUTW(...)` | variádico | Nil | Same via fmt.Println (sem prefixo stderr) |
| `CONOUTRAW(cText)` | string | Nil | Escreve sem newline nem separador — só o 1º arg. Streaming (deltas de LLM token a token) na mesma linha do terminal |
| `MSGINFO(msg, [title])` | string, optional title | Nil | Dialog info |
| `MSGSTOP(msg, [title])` | string, optional title | Nil | Dialog stop/error |
| `MSGALERT(msg, [title])` | string, optional title | Nil | Dialog alert |
| `MSGYESNO(msg, [title])` | string, optional title | Bool | Confirmação Yes/No |
| `ALERT(msg)` | string | Nil | Dialog alert |
| `FWGETTEXT(prompt, [default], [bIsPassword])` | string | string | Input text dialog |
| `FWMENUSELECT(items[], [title])` | array | Number | Menu selection (1-based) |
| `CONIN([prompt])` | optional string | string \| Nil | Lê uma linha do stdin. **Nil no EOF real** (Ctrl+D, pipe esgotado) — distingue de `""` (usuário só apertou Enter), pra um REPL poder sair do loop em vez de reimprimir o prompt pra sempre; checar com `IsNil()` |

Quando o stdin é um terminal interativo de verdade (`term.IsTerminal`),
`CONIN` entra em raw mode e roda um mini line-editor (`pkg/vm/lineeditor.go`)
em vez do antigo `bufio.ReadString('\n')` puro: setas ↑/↓ navegam as
últimas 20 linhas digitadas (ignora repetição consecutiva, como
`HISTCONTROL=ignoredups` do bash), ←/→/Home/End/Ctrl+A/Ctrl+E movem o
cursor, Backspace/Delete editam, Ctrl+U/Ctrl+K apagam até o início/fim da
linha, Ctrl+L limpa a tela e Ctrl+C cancela a linha atual (equivale a
Enter em branco). Em stdin não-interativo (pipe/redirect, como testes
`printf ... | ./binario`) o comportamento continua idêntico ao antigo —
sem raw mode. Sem mudança de API: mesma assinatura, mesmo retorno.

#### 4.4.1 TUI / Terminal (`pkg/vm/ui_render.go`)

Primitivas visuais de baixo nível (lipgloss + glamour) para TUIs de terminal
escritas em AdvPL/TLPP e compiladas com `advplc build` — estilo
opencode/Claude Code (caixas com borda, markdown renderizado, tela
alternativa). O programa AdvPL compõe a interface com essas primitivas, em vez
de usar widgets prontos (`MSDIALOG`/`FWGETTEXT`/`FWMENUSELECT`, que continuam
existindo para diálogos tradicionais).

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `UIBOX(cTitle, cBody, cColor, [nWidth])` | string×3, optional number | string | Caixa com borda arredondada (lipgloss), título em negrito na 1ª linha. `cColor` = código ANSI 256 (`"39"`=ciano, `"212"`=rosa, `"240"`=cinza). `nWidth` omitido/0 = largura automática. Só renderiza a string — quem chama decide se imprime |
| `UISTREAMBOX(cTitle, cBodySoFar, cColor, [nWidth])` | string×3, optional number | Nil | Igual `UiBox`, mas **auto-redesenha**: apaga com ANSI a caixa anterior (altura rastreada em `v.lastBoxLines`) antes de imprimir a nova. Chamar a cada delta de um LLM com o texto acumulado até agora (não só o delta) — efeito de "cartão crescendo ao vivo", sem raw-mode de teclado nem redesenho de tela inteira |
| `UISTREAMRESET()` | — | Nil | Zera o rastreador de altura do `UiStreamBox` — chamar ao fechar um turno de streaming, antes do próximo elemento de tela |
| `UIMARKDOWN(cMarkdown, [nWidth])` | string, optional number | string | Renderiza markdown (negrito, itálico, listas, blocos de código, títulos) para ANSI via glamour, estilo "dark" fixo (não usa auto-detecção de tema via OSC 11 — pode travar em terminais/multiplexers que não respondem a essa query, mesma cautela já documentada para lipgloss em `pkg/ui/terminal.go`). `nWidth` padrão 80. Em erro de parse devolve `cMarkdown` sem alteração, nunca falha |
| `UIALTSCREENENTER()` | — | Nil | Entra na tela alternativa do terminal (mesmo buffer que vim/less/htop usam) — a saída normal do shell fica preservada. Instala handler de Ctrl+C (`os.Interrupt`, portátil) que restaura a tela antes de encerrar |
| `UIALTSCREENEXIT()` | — | Nil | Sai da tela alternativa, restaura o conteúdo normal — chamar na saída normal do programa |
| `UITERMWIDTH([nDefault])` | optional number | number | Largura do terminal em colunas. `nDefault` (padrão 80) é usado quando stdout não é um tty real (pipe/redirecionamento), onde `term.GetSize` falha |

#### Manipulação de Strings

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `ALLTRIM(str)` | string | string | Remove whitespace ambos lados |
| `LTRIM(str)` | string | string | Remove left whitespace |
| `RTRIM(str)` | string | string | Remove right whitespace |
| `TRIM(str)` | string | string | Alias para RTRIM |
| `STR(num, [width], decimals)` | numeric | string | Número formatado como string |
| `STRTRAN(str, search, replace)` | 3 strings | string | Substituição global |
| `STRZERO(num, size)` | numeric, int | string | Zero-padded string |
| `SUBSTR(str, start, [len])` | string | string | Substring (1-based) |
| `STUFF(str, start, count, repl)` | 4 strings | string | Substitui substring |
| `LEN(str|array)` | string or array | Number | Comprimento |
| `AT(search, str)` | 2 strings | Number | Posição 1-based (0=não encontrado) |
| `RAT(search, str)` | 2 strings | Number | Última posição |
| `ATC(search, str)` | 2 strings | Number | Case insensitive AT |
| `RATC(search, str)` | 2 strings | Number | Case insensitive RAT |
| `UPPER(str)` | string | string | Maiúsculas |
| `LOWER(str)` | string | string | Minúsculas |
| `PADC(str, size, [pad])` | 3 strings | string | Central-pad |
| `PADL(str, size, [pad])` | 3 strings | string | Left-pad |
| `PADR(str, size, [pad])` | 3 strings | string | Right-pad |
| `CHR(code)` | number | string | Caractere pelo código |
| `ASC(str)` | string | number | Código do caractere |
| `VAL(str)` | string | number | String → número |
| `CVALTOCHAR(val)` | any | string | Valor para string |
| `EMPTY(val)` | any | bool | .T. se vazio/nulo |
| `SPACE(n)` | number | string | n espaços |
| `REPLICATE(str, n)` | 2 args | string | Repete n vezes |
| `REPLICA(str, n)` | 2 args | string | Alias para REPLICATE |
| `STRTOKARR(str, delim)` | 2 strings | array | Split string → array |
| `LEFT(str, count)` | 2 args | string | n char da esquerda |
| `RIGHT(str, count)` | 2 args | string | n char da direita |
| `CAPSLOCK(str)` | string | string | Capitalize first letter |
| `PROPER(str)` | string | string | Title Case |
| `GETWORDNUM(str, num, [delim])` | string | string | Palavra n-ésima |
| `WORDS(str, [delim])` | string | number | Contagem de palavras |
| `FILENOEXT(path)` | string | string | Nome sem extensão |
| `FILEEXT(path)` | string | string | Extensão |
| `FILENAME(path)` | string | string | Nome do arquivo |
| `FILEPATH(path)` | string | string | Caminho do arquivo |
| `FILEDIR(path)` | string | string | Diretório |

#### Numéricas / Matemática

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `ABS(n)` | number | number | Valor absoluto |
| `INT(n)` | number | number | Trunca para inteiro |
| `ROUND(n, decimals)` | 2 args | number | Arredonda |
| `NOROUND(n, decimals)` | 2 args | number | Trunca decimais |
| `CEILING/FLOOR(n)` | number | number | Teto/chão |
| `MOD(a, b)` | 2 numbers | number | Resto da divisão |
| `MAX(...)` | variádico | number | Máximo de args |
| `MIN(...)` | variádico | number | Mínimo de args |
| `SQRT(n)` | number | number | Raiz quadrada |
| `EXP(n)` | number | number | e^x |
| `LOG(n)` | number | number | Log natural |
| `RANDOM([max])` | optional number | number | Random 1..max |
| `SIGN(n)` | number | number | -1/0/1 |
| `POWER(base, exp)` | 2 numbers | number | x^y |
| `PI()` | — | number | Constante π |
| `SIN/COS/TAN(rads)` | number | number | Trigonométricas |
| `ASIN/ACOS/ATAN(rads)` | number | number | Trig inversas |
| `DEG2RAD(deg)` | number | number | Graus → radianos |
| `RAD2DEG(rad)` | number | number | Radianos → graus |

#### Data / Tempo

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `DATE()` | — | Date | Data atual |
| `DAY(date)` | date | number | Dia |
| `MONTH(date)` | date | number | Mês |
| `YEAR(date)` | date | number | Ano |
| `CMONTH(date)` | date | string | Nome do mês |
| `CDOW(date)` | date | string | Nome do dia da semana |
| `DOW(date)` | date | number | 1=Domingo...7=Sábado |
| `TIME()` | — | string | "HH:MM:SS" |
| `SECONDS()` | — | number | Timestamp Unix |
| `CTOD(str)` | "DD/MM/YYYY" | Date | String → date |
| `DTOS(date)` | date | string | Date → "YYYYMMDD" |
| `DTOC(date)` | date | string | Date → "DD/MM/YYYY" |
| `STOD(str)` | "YYYYMMDD" | date | String → date |
| `ELAPTIME(t1, t2)` | 2 times | number | Diferença em segundos |
| `CTOT(str)` | "HH:MM:SS" | date | Time → date |
| `TTOC(d/t)` | date/time | string | → "HH:MM:SS" |
| `FW8601TODATE(str)` | RFC3339 | date | ISO → Protheus date |
| `FWDATETO8601(d)` | date | string | Protheus → RFC3339 |
| `SETDATE(fmt)` | string | — | Formato data (stub) |
| `SETCENT(n)` | number | — | Configurar século (stub) |
| `ARRAY(dims...)` | variadic | array | Array multidimensional de Nils |

#### Transforms

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `TRANSFORM(val, mask)` | any, string | string | Aplica máscara de formatação |

#### Checks de Caractere

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `ISDIGIT(str)` | string | bool | Só dígitos? |
| `ISALPHA(str)` | string | bool | Só letras? |
| `ISLOWER(str)` | string | bool | Minúsculas? |
| `ISUPPER(str)` | string | bool | Maiúsculas? |

#### Arrays

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `AADD(array, val)` | 2 args | value | Append (return val) |
| `ASIZE(array, size)` | 2 args | Nil | Redimensiona (padra com NIL) |
| `ASCAN(array, search[, start, count])` | variadic | number | Busca (1-based, 0=não achou) |
| `ADEL(array, idx)` | 2 args | Nil | Delete por índice |
| `AINS(array, idx)` | 2 args | Nil | Insert NIL shift-right |
| `ALEN(array)` | 1 arg | number | Length |
| `ACLONE(array)` | 1 arg | array | Shallow copy |
| `AFILL(array, val)` | 2 args | Nil | Preenche array |
| `ASORT(array [,start,count,block])` | variadic | array | Ordenação (com comparator opcional) |
| `AEVAL(array, block[,start,count])` | variadic | array | Avalia bloco para cada elemento |

#### Lógica / Tipagem

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `IIF(cond, true_val, false_val)` | 3 args | value | Condicional ternário (short-circuit) |
| `IF(cond, true_val, false_val)` | 3 args | value | Alias para IIF |
| `VALTYPE(val)` | any | string | Tipo: "N"/"C"/"L"/"A"/"B"/"O"/"U"/"D" |
| `TYPE(expr)` | identifier | string | Tipo de identificador |
| `ISNIL(val)` | any | bool | É NIL? |

#### Tratamento de Erros

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `USEREXCEPTION(msg)` | string | — | Lança exception |
| `THROW(msg)` | string | — | Alias para USEREXCEPTION |

#### Sistema / Misc

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `FINDFUNCTION(name)` | string | bool | Função existe? |
| `STARTJOB(funcName, env, wait, params...)` | variadic | bool | Background job |
| `SLEEP(ms)` | number | Nil | Sleep em milissegundos |
| `PROCNAME()` | — | string | Nome do proc (stub: "") |
| `PROCLINE()` | — | number | Linha do proc (stub: 0) |
| `GETMV(key, default)` | 2 args | value | Get MV parameter (stub: returns 2nd arg) |
| `GETNEWPAR(key)` | string | string | Nova parâmetro (stub) |
| `GETENV(name, [default])` | string | string | Environment variable |
| `FWHASH(text)` | string | string | SHA-256 hex digest |
| `FILE(path)` | string | bool | Arquivo existe? |
| `MAKEDIR(path)` | string | Nil | Criar diretório (stub) |
| `CURDIR()` | — | string | "./" |
| `GETSRVNAME()` | — | string | "localhost" |
| `FWHTTPENCODE(str)` | string | string | URL percent encoding |
| `FWURIDECODE(str)` | string | string | URI decode (pass-through) |
| `FWHTTPGET(cURL, [cCert, cPass])` | string, opcionais | number | Requisição GET, retorna o status HTTP |
| `FWHTTPPOST(cURL, cBody, cContentType, [cCert, cPass])` | string×3, opcionais | number | Requisição POST |
| `FWHTTPPUT(cURL, cBody, cContentType, [cCert, cPass])` | string×3, opcionais | number | Requisição PUT |
| `FWHTTPPATCH(cURL, cBody, cContentType, [cCert, cPass])` | string×3, opcionais | number | Requisição PATCH |
| `FWHTTPDELETE(cURL, [cCert, cPass])` | string, opcionais | number | Requisição DELETE |
| `FWHTTPBODY()` | — | string | Corpo da resposta da última requisição |
| `FWHTTPSTATUS()` | — | number | Status HTTP da última requisição |
| `FWHTTPERROR()` | — | string | Mensagem de erro da última requisição (se houver) |
| `FWHTTPTIMEOUT(nSeconds)` | number | Nil | Define o timeout (segundos) aplicado a toda requisição `FWHTTPGET/POST/PUT/PATCH/DELETE` seguinte, até ser trocado de novo. `nSeconds <= 0` restaura o default de 30s |
| `FWHTTPHEADER(cName, cValue)` | string, string | Nil | Registra um header customizado (ex.: `Authorization: Bearer ...`) aplicado a toda requisição seguinte, até `FWHTTPCLEARHEADERS()` |
| `FWHTTPCLEARHEADERS()` | — | Nil | Remove todos os headers customizados definidos via `FWHTTPHEADER` |
| `FWLOADSM0()` | — | bool | .T. stub |
| `FWJOINFILIAL(field, alias)` | 2 strings | string | Concat filial |
| `FWRESTAREA/FWGETAREA/FWAPPSTACK/FWCALLAPP` | — | Nil/string | Stubs |
| `FWLIBVERSION()` | — | string | "1.0.0" |
| `FWLISTBRANCHES()` | — | array | Empty array |
| `FWCLEARHLP()` | — | Nil | Clear help (stub) |
| `FWMSGRUN(cCmd, lWait)` | variadic | string | MsgRun (stub) |
| `FWMONITORMSG(msg, level)` | variadic | Nil | Monitor msg (stub) |
| `MPISSMART()` | — | bool | .F. stub |
| `MPUSERHASACCESS()` | — | bool | .T. stub |
| `MPCRIANUMS()` | — | string | "000001" stub |
| `MPDOCPATH()` | — | string | "./" |
| `MPDOCVIEW/MPBINVIEW/MPEXPCHK/MSDOCUMENT/MSMULTDIR` | — | Nil/array | Viewer stubs |
| `CHANGEQUERY(query)` | string | Nil | FW exec SQL (stub) |
| `CHKADVPLSYNTAX()` | — | bool | Syntax check (stub) |
| `FILLGETDADOS()` | — | Nil | DB fill (stub) |
| `FWEXECLOCALIZ()` | — | Nil | Localization exec (stub) |
| `FWEXISTLOCALIZ()` | — | bool | Localization exists (stub) |
| `FWQTTOCHR(qt)` | number | string | Quote-to-char |
| `FWREBUILDINDEX()` | — | bool | Rebuild index (stub) |
| `FWRULESDB()` | — | bool | Rules DB (stub) |
| `FWGRPPRIVDB()` | — | bool | Group priv DB (stub) |
| `FWPDCANUSE(username)` | string | bool | Auth can use (stub) |
| `FWPDLOGUSER(username)` | string | bool | Auth log user (stub) |
| `FWPUTSX5(key, val)` | 2 args | — | Put SX5 (stub) |
| `FWX2CHAVE(field)` | string | bool | SX2 unique key (stub) |
| `FWX2UNICO(field)` | string | bool | SX2 unique (stub) |
| `FWX3TITULO(field)` | string | string | SX3 title (stub) |
| `FWUSREMP(username)` | string | string | User emp (stub) |
| `FWVLDVINC(cVar)` | string | string | Validate variant (stub) |
| `PESQBRW()` | — | Nil | Browse search (stub) |
| `MARKBROW()` | — | Nil | Mark browse (stub) |
| `MAKESQLEXPR(alias, fields)` | variadic | string | SQL expression (stub) |
| `MAYIUSECODE()` | — | bool | Use code (stub) |
| `RESTINTER(savePath)` | string | Nil | REST inter (stub) |
| `SAVEINTER(fileName)` | string | Nil | Save inter (stub) |
| `PUTSX1HELP(field, help)` | 2 strings | — | SX1 help (stub) |
| `OLE_CREATELINK(progId)` | string | object | OLE create (stub) |
| `PROCESSA()` | — | Nil | Process A (stub) |
| `MENUDEF()` | — | Nil | Menu def (stub) |
| `I18N(key)` | string | string | i18n translation (stub) |
| `WSADVVALUE(value)` | any | value | WS adv value (stub) |
| `GETNAMES(obj)` | JsonObject | array | Lista propriedades do objeto |
| `HELP()` | — | Nil | Help (stub) |
| `FREEOBJ/FWFREEOBJ(obj)` | object | Nil | Free object (stub/no-op) |
| `FWALIASINDIC()` | — | bool | .F. stub |
| `FWMODEACCESS()` | — | number | 1 stub |
| `FWHASACCMODE()` | — | bool | .T. stub |
| `USRRETNAME()` | — | string | Username stub |
| `MSRETPATH()` | — | string | "./" stub |
| `WAITRUN(cmd)` | string | number | Executa shell command, retorna exit code |
| `PROCRUN(cPath, aArgs, bOnStdoutLine, [bOnStderrLine])` | string, array, codeblock×2 | number | Executa `cPath` com `aArgs` (sem shell, stdin fechado). Para cada linha de stdout chama `bOnStdoutLine(cLinha)` **sincronamente** — o AdvPL pode desenhar direto na tela, ideal pra TUIs consumindo NDJSON de um processo filho em streaming (ex.: CLI de um LLM). `bOnStderrLine` opcional (stderr descartado se omitido). Bloqueia até o processo terminar; retorna o exit code, ou -1 se não iniciou |

#### Funções Database (Workarea)

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `DBSELECTAREA(alias)` | string | Nil | Seleciona work area |
| `DBSEEK(key)` | any | Bool | Seek por chave |
| `DBSKIP([count])` | optional number | Nil | Skip N registros |
| `DBGOTOP()` | — | Nil | Vai ao primeiro |
| `DBGOBOTTOM()` | — | Nil | Vai ao último |
| `EOF()` | — | Bool | No fim? |
| `BOF()` | — | Bool | No início? |
| `RECLOCK()` | — | Bool | Lock record |
| `MSUNLOCK()` | — | Nil | Unlock |
| `RECCOUNT()` | — | Number | Total registros |
| `RECNO()` | — | Number | Nº registro atual |
| `DBCLOSEAREA()` | — | Nil | Close area (stub) |
| `DBSETORDER(nOrd)` | number | Nil | Set order (stub) |
| `DBSETFILTER(exp, exp2)` | variadic | Nil | Set filter (stub) |
| `DBCLEARFILTER()` | — | Nil | Clear filter (stub) |
| `DBAPPEND()` | — | Nil | Append record |
| `DBDELETE()` | — | Nil | Mark delete |
| `DBCOMMIT()` | — | Nil | Commit transaction |
| `SELECT()` | — | Number | Número da área |
| `ALIAS()` | — | String | Alias atual |
| `GETAREA([alias])` | optional | String | Salva área corrente |
| `RESTAREA(cAlias)` | string | — | Restaura área |
| `RETSQLNAME(alias)` | string | string | Nome físico tabela |
| `USED()` | — | Bool | .F. stub |
| `FIELDGET(field)` | — | Nil | Read field (stub) |
| `FIELDPUT(field, val)` | — | Nil | Write field (stub) |
| `FIELDPOS(field)` | string | Number | Position of field |
| `FIELDNAME()` | — | String | (stub) |
| `XFILIAL(alias)` | optional string | string | Filial filter |
| `TCSQLEXEC(query)` | string | Bool | Raw SQL execute |
| `TCSQLQUERY(query)` | string | array | Raw SQL query → JsonObject[] |

#### MVC Functions

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `FWFORMMODEL(name)` | string | Object | Cria FWFormModel |
| `FWFORMVIEW(name, model)` | 2 strings | Object | Cria FWFormView |
| `FWFORMBROWSE(name, model)` | 2 strings | Object | Cria FWFormBrowse |
| `FWFORMSTRUCT()` | — | Object | Form struct (stub) |
| `FWMBROWSE()` | — | Object | FWMBrowse object |
| `VIEWDEF()` | — | Nil | Scaffolding stub |
| `AXCADASTRO()` | — | Nil | Cadastro stub |

#### JSON

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `JSONOBJECT()` | — | Object | Cria JsonObject |

##### Métodos JsonObject

Acesso a chave é por colchete (`oJ["chave"]`, `oJ["a"]["b"]`), não por dois-pontos —
`oJ:chave` não resolve propriedades dinâmicas (só campos declarados de classe).

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `oJ:FromJson(cJson)` | string | Bool | Parser real (`encoding/json` da stdlib) — antes era stub que sempre devolvia Nil sem parsear nada. Objetos aninhados viram `JsonObject`, arrays viram `Array` 1-based, `null`→Nil. `.F.` se `cJson` não parsear como objeto (JSON inválido, ou top-level array/escalar) — nunca falha/lança erro |
| `oJ:ToString()` / `oJ:ToJson()` | — | string | Serializa de volta para JSON |
| `GetNames(oJ)` | JsonObject | array | Lista as propriedades, na ordem de inserção |

#### Math/Stat Extended

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `CEIL(n)` | number | number | Ceiling |
| `COSH/SINH/TANH(x)` | number | number | Hyperbolic |
| `FACT(n)` | number | number | Fatorial |
| `GCD(a, b)` | 2 numbers | number | MDC |
| `LCM(a, b)` | 2 numbers | number | MMC |
| `MEAN(array)` | array | number | Média aritmética |
| `VARIANCE(array)` | array | number | Variância amostral |
| `STDDEV(array)` | array | number | Desvio padrão |
| `MEDIAN(array)` | array | number | Mediana |
| `LINREG(xArr, yArr)` | 2 arrays | [intercept, slope] | Regressão linear |
| `INTERP(xArr, yArr, x)` | 3 args | number | Interpolação linear |
| `ATAN2(y, x)` | 2 numbers | number | Arctan 4 quadrantes |
| `LOG10(n)` | number | number | Log10 |
| `POW(base, exp)` | 2 numbers | number | Power |
| `MATVECTERN(matrix, ternaryVec)` | 2 args | array | Matrix-vector mul (sparse BLAS) |
| `FIT(block, epochs)` | variadic | value | Treinamento loop |

### 4.5 Sistema de Diálogo (MsDialog)

Arquivo: `pkg/vm/dialog.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `MSDIALOG(x1,y1,x2,y2,title)` | 5 args | Object | Cria objeto MsDialog |
| `AT_SAY(row,col,text,...)` | variadic | Nil | Registra SAY control |
| `AT_GET(row,col,var,val,picture,...)` | variadic | Nil | Registra GET control |
| `AT_BUTTON(row,col,text,action,...)` | variadic | Nil | Registra BUTTON control |
| `AT_BOX()` | — | Nil | Decorativo (no-op) |
| `ACTIVATE_MSDIALOG(dialogObj, [initBlock])` | variadic | Nil | Executa ciclo do diálogo |

#### Métodos no Objeto MsDialog

`NEW()`, `ACTIVATE()`, `END()`, `CLOSE()`, `DEACTIVATE()`, `SETTITLE()`, `SETPOS()`, `SETSIZE()`, `ADDSAY()`, `ADDGET()`, `ADDSTRUCTGET()`, `ADDSTRUCT()`, `ADDSBUTTON()`, `ADDIMAGE()`, `ADDWINDOW()`

### 4.6 Browse (FWMBrowse Auto-CRUD)

Arquivo: `pkg/vm/browse.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `FWMBROWSE()` | — | Object | Cria FWMBrowse |

#### Métodos FWMBrowse

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NEW()` | — | — | Constructor |
| `SETALIAS(alias)` | string | — | Sets table alias |
| `SETTITLE(title)` | string | — | Sets dialog title |
| `ACTIVATE()` | — | — | Full CRUD cycle |
| `DEACTIVATE()` | — | — | Destroys dialog |
| `DESTROY()` | — | — | Cleanup |

#### Fluxo `runBrowse()`

1. Detecta engine SQL + alias
2. Queries PRAGMA table_info + SX3 dictionary → columns metadata
3. SELECT rowid AS browse_recno_ WHERE D_E_L_E_T_=' ' → row items
4. Aguarda ação do usuário: save (INSERT/UPDATE), delete (soft-delete D_E_L_E_T_='*'), close
5. Retorna ao passo 3 para refresh

### 4.7 Grid Process (FWGridProcess)

Arquivo: `pkg/vm/grid.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `FWGRIDPROCESS()` | — | Object | Creates grid process |

#### Métodos FWGridProcess

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NEW(cFunName, cTitle, cDesc, bProcess, cPerg, cGrid, lSaveLog)` | variadic | — | Constructor |
| `SETTHREADGRID(nThreads)` | number | — | Thread pool size |
| `SETMAXTHREADGRID(max)` | number | — | Max concurrent |
| `ACTIVATE()` / `EXECUTE()` | — | — | Executa bProcess síncrono |
| `CALLEXCUTE(cb)` | codeblock | — | Dispatch async worker |
| `STOPEXECUTE()` | — | — | Stops all workers |
| `ISFINISHED()` | — | bool | Are all done? |
| `SETABORT(lAbort)` | bool | — | Set abort flag |
| `SETAFTEREXECUTE(cb)` | codeblock | — | Post-exec callback |
| `SETMETERS(nMeters)` | number | — | Number progress meters |
| `SETMAXMETER(idx, max)` | 2 args | — | Meter max value |
| `SETINCMETER(idx, inc)` | 2 args | — | Increment meter |
| `SAVELOG()` | — | Nil | Save log |
| `GETLASTLOG()` | — | string | Last log message |
| `SETNOPARAM()` | — | — | Disable parameters |
| `DEACTIVATE()` | — | — | Destroy |

### 4.8 Macro Substitution (&)

Arquivo: `pkg/vm/macro.go`

| Função | Descrição |
|--------|-----------|
| `evalMacroString(src string)` | Pipeline completo: lex → parse → compile → executa string como expressão AdvPL em frame isolado. Variáveis dynEnv (Private/Public) são compartilhadas com caller. Retornar valor da expressão após restore do frame. |

Suporte: `&ident`, `&(expression)`, `K2&cSuf` (macro-computed variable names).

### 4.9 Debugger

Arquivo: `pkg/vm/debug.go` + `pkg/dap/`

**Tipos debugger (`pkg/vm/`):**

| Tipo | Campos | Descrição |
|------|--------|-----------|
| `StepMode` | `StepNone, StepOver, StepIn, StepOut` | Modos de step |
| `Debugger` | `breakpoints, pauseRequested, resumeChan, OnStop` | Debugger instance |
| `StackFrameInfo` | `Name string, Line int` | Info de frame na stack |
| `VarInfo` | `Name, Type, Value` | Variável local info |

| Método | Descrição |
|--------|-----------|
| `NewDebugger()` | Cria debugger |
| `v.AttachDebugger(d)` | Liga debugger à VM |
| `d.SetBreakpoints(lines)` | Breakpoints por linha |
| `d.SetStopOnEntry(v)` | Pausa na entrada de função |
| `d.checkBreak(v, instr)` | Per instruction — decide se pausar |
| `d.pause(reason, line)` | Para execução, chama OnStop |
| `d.Continue()/Next()/StepIn()/StepOut()` | Resume execution |
| `d.RequestPause()` | Request pause imediato |
| `v.DebugStackFrames()` | `[]StackFrameInfo` — call stack |
| `v.DebugLocals(frameIndex)` | `[]VarInfo` — variáveis locais de frame |

**DAP Server (`pkg/dap/`):**

| Função | Descrição |
|--------|-----------|
| `NewConn(r, w)` | stdio connection with Content-Length framing |
| `NewServer(conn, compile, attach)` | Launch mode — compila e executa fonte |
| `NewAttachServer(conn, sourcePath)` | Attach mode — depura sessions browser |
| `OfferSession(v)` | Oferece VM para debugging (attach mode) |
| `Run()` | Event loop: ReadMessage → handle → repeat |

**Comandos DAP implementados:** `initialize`, `launch`, `attach`, `setBreakpoints`, `configurationDone`, `threads`, `stackTrace`, `scopes`, `variables`, `continue`, `next`, `stepIn`, `stepOut`, `pause`, `disconnect`, `terminate`

### 4.10 Tensor (Operações com Tensores)

Arquivo: `pkg/tensor/tensor.go` + `ops.go` + `ops64.go` + `linalg.go`

**Tipos:**

| Tipo | Campos | Descrição |
|------|--------|-----------|
| `DType` | — | `Float32` (default), `Float64` |
| `Tensor` | `Data []float32`, `Shape []int`, `Data64 []float64`, `DType DType` | Dense row-major tensor |

#### Constructors

| Função | Parâmetros | Retorno |
|--------|-----------|---------|
| `New(shape []int)` | dims | zero-filled float32 |
| `FromData(data []float32, shape []int)` | dados + shape | tensor com dados |
| `Rand(shape []int, scale float32)` | dims, range | random [-scale, scale] |
| `NewDType(shape, dt)` | dims + dtype | tensor typed zero |
| `FromData64(data, shape)` | f64 data + shape | float64 tensor |
| `Prod(shape)` | dims | produto de todos (= total elements) |

#### Métodos Tensor

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `Size()` | — | int | Total elementos |
| `Get(i int)` | index | float64 | Elemento flat |
| `Set(i int, v float64)` | index, val | — | Escreve elemento |
| `At(idx []int)` | multi-dim | (float32, error) | Elemento por dimensões |
| `SetAt(idx []int, val)` | indices, val | error | Escreve por dimensões |
| `Offset(idx []int)` | indices | (int, error) | Flat offset |
| `AsDType(dt)` | target dtype | *Tensor | Copy com dtype |
| `Add/Sub/Mul/Div(b)` | other tensor | (*Tensor, error) | Element-wise binary |
| `AddScalar/MulScalar(s)` | scalar | *Tensor | Element-wise scalar |
| `Exp/Log/Sqrt/Tanh/Relu/Sigmoid/Gelu` | — | *Tensor | Activation element-wise |
| `Softmax(axis)` | 0 or 1 | (*Tensor, error) | Softmax (stable) |
| `MatMul(b)` | other | (*Tensor, error) | Matrix multiplication |
| `Transpose()` | — | (*Tensor, error) | 2D transpose |
| `Reshape(shape)` | new shape | (*Tensor, error) | New shape same data |
| `IndexRows(idx)` | row indices | (*Tensor, error) | 2D row subset |

#### Reductions

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `SumAll/MeanAll/MaxAll` | — | float32 | Global reduction |
| `ArgmaxAll` | — | int | Index do máximo |
| `SumAxis/MeanAxis/MaxAxis/ArgmaxAxis` | axis 0/1 | (*Tensor, error) | Reduction by axis |

#### Linear Algebra (`linalg.go`)

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `Dot(b)` | vector | (float64, error) | Dot product |
| `Norm()` | — | float64 | Frobenius norm (L2) |
| `Det()` | — | (float64, error) | Determinant (2D square) |
| `Inv()` | — | (*Tensor, error) | Inverse via LU |
| `Solve(b)` | rhs [n] | (*Tensor, error) | Ax=b solution |
| `QR()` | — | (Q, R, error) | Householder QR |
| `EigSym()` | — | (vals, vecs, error) | Symmetric eigenvalues/eigenvectors |
| `Eig()` | — | (real, imag, error) | Full eigenvalues (complex) |

### 4.11 Autograd (Diferenciação Automática)

Arquivo: `pkg/autograd/variable.go` + `sgd.go` + `adam.go` + `ops.go`

**Tipo Principal: `Variable`**

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `Value` | `*tensor.Tensor` | Valor atual |
| `Grad` | `*tensor.Tensor` | Gradiente acumulado |
| `parents` | `[]*Variable` | Tape: quem gerou este |
| `backward` | `func()` | Closure de gradiente |

#### Constructors

| Função | Parâmetros | Retorno |
|--------|-----------|---------|
| `NewLeaf(t *tensor.Tensor)` | tensor | Leaf variable (sem parents) |

#### Métodos de Autograd (gradient tape)

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `MatMul(b)` | *Variable | *Variable | Forward + backward: dA=dY·Bᵀ, dB=Aᵀ·dY |
| `Add(b)` | *Variable | *Variable | Sum com broadcast gradient |
| `Mul(b)` | *Variable | *Variable | Hadamard: dA=dY⊙B, dB=dY⊙A |
| `Relu()` | — | *Variable | relu(x): dA=dY⊙(x>0) |
| `Sum()` | — | *Variable | Scalar: broadcast ones |
| `Mean()` | — | *Variable | Scalar: dY/N broadcast |
| `Tanh()` | — | *Variable | dA=dY⊙(1-tanh²) |
| `Sigmoid()` | — | *Variable | dA=dY⊙σ(1-σ) |
| `Gelu()` | — | *Variable | GELU tanh approximation |
| `IndexRows(idx)` | []int | *Variable | Scatter-add gradient back |
| `Reshape(shape)` | []int | *Variable | Gradient reshape to original |
| `SoftmaxCE(targets)` | []int | *Variable | Cross-entropy loss |
| `MSE(target)` | *Variable | *Variable | Mean squared error |
| `Backward()` | — | error | Topological reverse pass (scalar loss required) |

#### Optimizers

**SGD:**
| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewSGD(params, lr)` | []*Variable, float32 | *SGD | Standard SGD |
| `Step()` | — | — | p := p - lr·grad (in-place) |
| `ZeroGrad()` | — | — | Reset all gradients |

**Adam:**
| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewAdam(params, lr)` | []*Variable, float32 | *Adam | Adam (b1=0.9, b2=0.999, eps=1e-8) |
| `Step()` | — | — | Bias-corrected update |
| `ZeroGrad()` | — | — | Reset gradients + moment buffers |

### 4.12 LLM (Inferência GGUF)

Arquivo: `pkg/llm/model.go` + `tokenizer.go` + `gguf.go` + `i2s.go` + `sampling.go` + `simd_amd64.go`

#### GGUF File Operations

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `Open(path)` | filepath | `(*File, error)` | Abre GGUF (lazy tensor loads) |
| `Close()` | — | error | Fecha arquivo |
| `Tensor(name)` | name | `(*Tensor, bool)` | Lookup tensor descriptor |
| `TensorData(name)` | name | ([]byte, error) | Full tensor data |
| `TensorRange(name, off, len)` | name, offset, length | ([]byte, error) | Partial read |
| `Uint32/Float32/String/StringArray/Int32Array(key)` | KV key | value + found | Metadata KV queries |

#### Model Loading

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `LoadModel(path)` | GGUF path | `(*Model, error)` | Loads arch=llama only; ~file-size RAM |
| `Close()` | — | error | Closes underlying GGUF file |

#### Context / Inference

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `NewContext(m)` | *Model | `*Context` | Empty context (KV cache) |
| `Forward(token)` | int32 ID | ([]float32, error) | Single-token forward pass. Updates KV cache. |

#### Tokenizer

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `NewTokenizer(g)` | *File | `(*Tokenizer, error)` | Parses GPT-2 BPE vocab from GGUF |
| `BOS()` | — | int32 | Beginning of sequence ID |
| `EOS()` | — | int32 | End of sequence ID |
| `Encode(text)` | string | []int32 | Text → token IDs (BPE) |
| `Decode(ids)` | []int32 | string | Token IDs → text (BPE) |

#### Sampling

| Função | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `Greedy(logits)` | []float32 | int32 | Argmax (best token) |
| `Sample(logits, cfg, rng)` | logits, config, rand | int32 | Nucleus sampling; temp≤0 → greedy |

#### Kernel Functions (internal)

| Função | Descrição |
|--------|-----------|
| `MatMulI2S(w, x)` | Quantized ternary matmul (I2_S weights) |
| `MatMulF16(weightF16, nRows, nIn, x)` | F16 matmul (SIMD AVX2 accelerated on amd64) |
| `RMSNorm(x, weight, eps)` | RMS normalization |
| `RoPE(x, headDim, pos, ropeDims, freqBase)` | Rotary position embedding (in-place) |
| `SwiGLU(gate, up)` | SiLU(gate) * up — LLaMA FFN activation |
| `Softmax(x)` | Normalizes logits in-place |
| `EmbedRow(g, name, row, dim)` | Lazy F16 row decode from GGUF |
| `quantizeI8S(x)` | Absmax quantization (±127) |
| `Float16ToFloat32(h)` | Half-float → float32 |
| `DecodeF16Row(raw, n)` | F16 bytes → float32 |
| `parallelRows(nRows, fn)` | Goroutines-based parallel processing |

#### SIMD Acceleration (amd64)

| Variável/Função | Descrição |
|----------------|-----------|
| `hasAVX2` | Runtime CPUID detection |
| `hasF16CFMA` | F16+CFMA detection |
| `dotI2SBlocksAVX2()` | AVX2 ternary matmul kernel (Plan9 assembly) |
| `dotF16BlocksAVX2()` | VCVTPH2PS+FMA F16 dot (assembly) |
| `cpuid()` | CPUID instruction (assembly) |

### 4.13 TMailMessage (E-mail SMTP)

Arquivo: `pkg/vm/mail_native.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `TMAILMESSAGE()` | — | Object | Create TMailMessage class |
| `NEW()` | — | self | Constructor |
| `SETSERVER(server, [port])` | string, optional int | Nil | SMTP server + port (default 587) |
| `SETAUTH(user, pass)` | 2 strings | Nil | Plain auth credentials |
| `SETFROM(addr)` | string | Nil | Sender address |
| `ADDTO(addr)` | string | Nil | Add recipient (alias: SETTO) |
| `SETCC(addr)` | string | Nil | Add CC |
| `SETBCCTO(addr)` | string | Nil | Add BCC |
| `SET SUBJECT(subj)` | string | Nil | Subject line |
| `SETBODY(body)` | string | Nil | Email body |
| `SETATTACHMENT(path)` | string | Nil | Attachment file |
| `SEND()` | — | Bool | Send via net/smtp stdlib |

### 4.14 MCPServer (Model Context Protocol)

Arquivo: `pkg/vm/mcp_native.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `MCPSERVER()` | — | Object | Create MCP server class |
| `NEW(name, [version])` | 2 strings | self | Constructor |
| `ADDTOOL(toolName, description, schemaJSON, funcName)` | 4 args | Nil | Register AdvPL function as MCP tool |
| `SERVE()` | — | Nil | Start stdio JSON-RPC server |

### 4.15 WSRestServer (REST API)

Arquivo: `pkg/vm/rest_native.go`

| Função Nativa | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `WSRESTSERVER()` | — | Object | Create REST server class |
| `NEW(name, [version])` | 2 strings | self | Constructor (auto-registers annotated routes) |
| `ADDROUTE(method, path, funcName)` | 3 strings | Nil | Manual route registration |
| `GET()` | — | — | Auto-detected @Get routes |
| `POST()` | — | — | Auto-detected @Post routes |
| `PUT()` | — | — | Auto-detected @Put routes |
| `PATCH()` | — | — | Auto-detected @Patch routes |
| `DELETE()` | — | — | Auto-detected @Delete routes |
| `SERVE([addr/port])` | optional | Nil | HTTP listen (default :8080) |
| `SHUTDOWN()` | — | Nil | Graceful shutdown (5s timeout) |

Annotation support: `@Get("/path")`, `@Post("/path")`, etc. scan bytecode at startup.

---

## 5. Framework MVC

Arquivos: `pkg/mvc/model.go` + `view.go` + `browse.go` + `types.go`

### FWFormModel (`model.go`)

```go
type FWFormModel struct {
    Name       string
    Fields     []*FieldDef
    Validations map[string][]ValidationRule
    PrimaryKey []string
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewFWFormModel(name)` | string | *FWFormModel | Constructor |
| `AddField(field)` | *FieldDef | — | Adiciona campo |
| `AddValidation(fieldName, rule)` | 2 strings | — | Validation rule |
| `Validate(data)` | map | error | Validates data against rules |
| `GetField(name)` | string | *FieldDef | Campo por nome |
| `SetPrimaryKey(keys...)` | ...string | — | PK definition |
| `AddRequiredValidation()` | — | — | Required rule |
| `AddLengthValidation(min, max)` | 2 ints | — | Length rule |
| `AddRangeValidation(min, max)` | 2 values | — | Range rule |
| `AddCustomValidation(handler)` | func | — | Custom validator |
| `GetValidations()` | — | []ValidationRule | All rules |

### FWFormView (`view.go`)

```go
type FWFormView struct {
    Name      string
    Model     *FWFormModel
    Title     string
    Width     int
    Height    int
    Components []*Component
    Dialogs   []*Dialog
    MenuBar   *MenuBar
    ToolBar   *ToolBar
    StatusBar *StatusBar
    Events    map[string]EventHandler
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewFWFormView(name, model)` | 2 args | *FWFormView | Constructor |
| `AddComponent(comp)` | *Component | — | Add UI component |
| `AddDialog(dialog)` | *Dialog | — | Add dialog |
| `SetTitle()` / `SetSize()` | — | — | Title/size |
| `AddEvent(handler)` | *EventHandler | — | Event handler |
| `GetComponent(name)` | string | *Component | Get component |
| `SetValue()` / `GetValue()` | fieldName, val | — | Field get/set |
| `Validate()` | — | error | View validation |
| `TriggerEvent(eventName, context)` | 2 args | error | Fire event |
| `AddOnChange/OnClick/OnFocus/OnBlur()` | — | — | Shortcut event adds |

#### Component Types

`TButton`, `TGet`, `TComboBox`, `TCheckBox`, `TLabel`, `TPanel`, `TGroupBox`, `TTabs`, `TSplitter`, `TTreeView`, `TListView`, `TImage`, `TWindow`, `TSButton`

#### Dialog

```go
type Dialog struct {
    Name, Title string
    Width, Height int
    When, Valid interface{}
    Components []*Component
    DefaultButton string
}
```

### FWFormBrowse (`browse.go`)

```go
type FWFormBrowse struct {
    Name, Alias, Order, Filter, Title string
    Model *FWFormModel
    Columns []*BrowseColumn
    Fields []*BrowseField
    Events map[string]EventHandler
    Permissions struct { AllowAdd, AllowEdit, AllowDelete bool }
    Size, ReadOnly bool
    LineNumbers, MarkColumn bool
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewFWFormBrowse(name, model)` | 2 args | *FWFormBrowse | Constructor |
| `AddColumn(col)` | *BrowseColumn | — | Add column |
| `AddField(field)` | *BrowseField | — | Add field |
| `SetAlias/SetOrder/SetFilter/SetTitle/SetSize` | string | — | Configuration |
| `SetPermissions(add, edit, delete)` | 3 bools | — | CRUD permissions |
| `AddEvent(handler)` | *EventHandler | — | Event handler |
| `Validate()` | — | error | Validation |
| `TriggerEvent(eventName, context)` | 2 args | error | Fire event |
| `AddOnLineChange/OnDbClick/OnHeaderClick()` | — | — | Shortcut events |

---

## 6. UI Desktop (Fyne)

Arquivos: `pkg/ui/` (11 arquivos)

### FyneUIProvider (`provider.go`)

```go
type FyneUIProvider struct {
    Window     fyne.Window
    Console    *OutputConsole
    FileTree   *FileTree
    Editor     *CodeEditor
    TreeView   *fyne.Container
    BrowseUI   vm.BrowseUI
    DialogUI   vm.DialogUI
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewFyneUIProvider(window)` | fyne.Window | *FyneUIProvider | Constructor |
| `MsgInfo/MsgStop/MsgAlert/MsgYesNo` | variadic | — | Blocking dialogs |
| `Menu(items, title)` | []string, string | int | Menu selection |
| `InputText(prompt, def, password)` | 3 args | string | Text input dialog |
| `Browse(specJSON)` | []byte | []byte | FWMBrowse rendering |
| `Dialog(specJSON)` | []byte | []byte | MSDIALOG rendering |

### Renderer (`renderer.go`)

```go
type Renderer struct {
    Window fyne.Window
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewRenderer(window)` | fyne.Window | *Renderer | Constructor |
| `RenderFormView(view)` | *FWFormView | fyne.CanvasObject | Full form → Fyne container |
| `ShowFormView(view)` | *FWFormView | — | Create window + show |
| `RenderComponent(comp)` | *Component | fyne.CanvasObject | Single component → widget |
| `RenderButton(comp)` | *Component | *fyne.Button | Button widget |
| `RenderGet(comp)` | *Component | *entry.Entry | Text input widget |
| `RenderComboBox(comp)` | *Component | *widget.Box | ComboBox widget |
| `RenderCheckBox(comp)` | *Component | *widget.Check | Checkbox widget |
| `RenderLabel(comp)` | *Component | *widget.Label | Label widget |
| `RenderMenuBar(menuBar)` | *MenuBar | *fyne.MainMenu | Menu bar |
| `RenderMenu(item)` | *MenuItem | *fyne.Menu | Menu item/submenu |
| `RenderToolBar(toolBar)` | *ToolBar | fyne.CanvasObject | Toolbar |
| `RenderStatusBar(statusBar)` | *StatusBar | fyne.CanvasObject | Status bar |

### OutputConsole (`console.go`)

```go
type OutputConsole struct {
    Label  *widget.Label
    Scroll *container.Scroll
    Output []string
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewOutputConsole()` | — | *OutputConsole | Constructor |
| `GetWidget()` | — | fyne.CanvasObject | Fyne widget |
| `Append(text)` | string | — | Add line |
| `Clear()` | — | — | Clear all |

### ConsoleWriter (`consolewriter.go`)

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewConsoleWriter(console)` | *OutputConsole | *ConsoleWriter | Adapter constructor |
| `Write(p []byte)` | []byte | (int, error) | io.Writer impl → Console append |

### FileTree (`filetree.go`)

```go
type FileTree struct {
    List      *widget.List
    Files     []string
    Current   string
    CurrentDir string
    OnSelect  func(string)
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewFileTree()` | — | *FileTree | Constructor |
| `GetWidget()` | — | fyne.CanvasObject | Fyne list widget |
| `SetCurrent(path)` | string | — | Set current dir |
| `GetCurrent()` | — | string | Current path |
| `SetFiles(files)` | []string | — | Refresh listing |
| `SetOnSelect(callback)` | func(string) | — | Selection callback |
| `Refresh()` | — | — | Reload current dir |
| `GetSelectedFile()` | — | string | Selected path |

### CodeEditor (`editor.go`)

```go
type CodeEditor struct {
    Entry    *highlightEntry
    Preview  *widget.RichText
    Overlay  *tapOverlay
    Stack    *fyne.Container
    Filename string
    Modified bool
}
```

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewCodeEditor()` | — | *CodeEditor | Constructor |
| `GetContent()` | — | string | Source text |
| `SetContent(text)` | string | — | Replace source |
| `GetWidget()` | — | fyne.CanvasObject | Fyne widget |
| `SetFilename(name)` | string | — | Filename |
| `GetFilename()` | — | string | Current filename |
| `IsModified()` | — | bool | Dirty flag |
| `SetModified(modified)` | bool | — | Set dirty flag |

### Theme (`theme.go`)

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewTheme()` | — | fyne.Theme | Custom AdvPP teal theme (#0f6e68) |

---

## 7. UI Web (PO-UI)

Arquivo: `pkg/webui/server.go`

### Server

```go
type Server struct {
    SourceName string
    Run        RunFunc
    Sessions   map[string]*session
}
```

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `New(sourceName, run)` | string, RunFunc | *Server | Constructor |
| `Serve(addr)` | string | error | HTTP server: static files + /events (SSE) + /reply (POST) |
| `Broadcast(kind, text)` | 2 strings | — | SSE event to all sessions |

### Session (per-browser)

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `ask(kind, msg, title)` | 3 strings | string | Blocks until reply from browser |
| `askData(eventType, data)` | string, json.RawMessage | string | Structured data dialog |
| `reply(id, result)` | 2 strings | — | Unblocks waiting ask |

### Provider (implements vm.UIProvider)

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `MsgInfo/MsgStop/MsgAlert/MsgYesNo` | variadic | — | Dialogs sent via SSE |
| `Menu(items, title)` | []string, string | int | Menu via SSE |
| `InputText(prompt, def, password)` | 3 args | string | Input via SSE |
| `Browse(specJSON)` | []byte | []byte | FWMBrowse spec → action |
| `Dialog(specJSON)` | []byte | []byte | MSDIALOG spec → action |

### OutWriter

| Método | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `Write(b []byte)` | []byte | (int, error) | io.Writer → SSE "output" events |

---

## 8. Banco de Dados SQLite

Arquivo: `pkg/db/sqlite.go`

**Motor de bancos de dados SQLite com emulation de workarea Protheus.**

| Função/Método | Args | Retorno | Descrição |
|---------------|------|---------|-----------|
| `NewSQLiteEngine(dbPath)` | string | (*SQLiteEngine, error) | Abre SQLite via shared helper |
| `SelectArea(alias)` | string | error | PRAGMA table_info + carrega TODOS records em memória |
| `Seek(key)` | string | (bool, error) | Linear search no primeiro campo |
| `Skip(count)` | int | error | Move ponteiro N registros |
| `GoTop()` | — | error | Posiciona no primeiro |
| `GoBottom()` | — | error | Posiciona no último |
| `EOF()` | — | bool | Fim do arquivo? |
| `BOF()` | — | bool | Antes do início? |
| `FieldGet(field)` | string | (Value, error) | Leitura de campo (AdvPL value) |
| `FieldPut(field, val)` | string, Value | error | Escrita de campo (only in-memory) |
| `RecLock()` | — | error | Placeholder (no-op) |
| `MsUnlock()` | — | error | COMMIT: UPDATE row by R_C_N_O_ |
| `Append()` | — | error | INSERT blank row with type defaults + R_C_N_O_ |
| `FieldPos(field)` | string | int | Posição do campo (1-based) |
| `RecCount()` | — | int | Total registros |
| `RecNo()` | — | int | Registro atual (1-based) |
| `QueryRows(query, args...)` | string, variadic | ([]map[string]string, error) | Arbitrary SQL query |
| `Exec(query, args...)` | string, variadic | error | Execute SQL command |
| `Close()` | — | error | Close DB connection |

### Helpers internos

| Função | Descrição |
|--------|-----------|
| `convertDBValue(interface{}) Value` | SQL row → AdvPL value |
| `valueToSQL(Value) any` | AdvPL value → SQL parameter |

---

## 9. Protocolos (DAP, MCP, REST)

### DAP — Debug Adapter Protocol (`pkg/dap/`)

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewConn(r, w)` | io.Reader, io.Writer | *Conn | Stdio connection with Content-Length framing |
| `ReadMessage()` | — | (*Envelope, error) | Block until complete DAP message |
| `SendResponse(reqSeq, cmd, success, msg, body)` | variadic | error | DAP response |
| `SendEvent(event, body)` | string, any | error | DAP event emission |
| `NewServer(conn, compile, attach)` | 3 args | *Server | Launch mode DAP |
| `NewAttachServer(conn, sourcePath)` | 2 args | *Server | Attach mode DAP |
| `OfferSession(v)` | *vm.VM | (bool, func(error)) | Offer VM for debugging |
| `Run()` | — | error | Event loop |

### MCP — Model Context Protocol (`pkg/mcp/`)

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewServer(name, version)` | 2 strings | *Server | Constructor |
| `AddTool(t)` | Tool | — | Register/replace tool |
| `Serve(r, w)` | io.Reader, io.Writer | error | JSON-RPC stdio loop |

**Tool:**
| Campo | Tipo | Descrição |
|-------|------|-----------|
| `Name` | string | Tool identifier |
| `Description` | string | Human-readable |
| `InputSchema` | map[string]any | JSON Schema |
| `Handler` | func(args) (string, error) | Implementation |

**Métodos JSON-RPC:** `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`

### REST (`pkg/rest/`)

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `NewServer(name, version)` | 2 strings | *Server | Constructor |
| `AddRoute(r)` | Route | — | Register route |
| `Routes()` | — | []Route | Introspection (sorted) |
| `Serve(addr)` | string | error | Blocking HTTP server |
| `Shutdown(ctx)` | context.Context | error | Graceful shutdown |

**Routing:** `{param}` placeholders, query string + JSON body merge, GET `/_routes` introspection.

---

## 10. Build Standalone

Arquivo: `pkg/compiler/standalone.go`

| Função | Args | Retorno | Descrição |
|--------|------|---------|-----------|
| `BuildStandaloneGUI(bc, outputFile, title, gui, buildLog)` | *Bytecode, 2 strings, bool, io.Writer | error | Embed bytecode in Go stub, compile → native executable. `gui` vem da flag `--gui` (ver Opções Globais acima) — baked in no stub via `__ADVPP_BUILT_AS_GUI__` e usado tanto para decidir o caminho de execução quanto para adicionar `-H=windowsgui` ao `go build` no Windows (`goBuildArgs`). |

### Stub Template Embutido

O template Go (`pkg/compiler/stub_template.go`) é embutido no build via string
replace de placeholders (`__ADVPP_APP_TITLE__`, `__ADVPP_BUILT_AS_GUI__`) e
compilado de verdade (`go build`) contra um `go.mod` gerado que faz
`replace` do módulo AdvPP apontando pro checkout (`ADVPP_SRC` ou raiz do
repo). Em runtime, o binário resultante escolhe um de dois caminhos:

1. Unmarshal bytecode.json embutido → `*Bytecode`
2. Varre o bytecode em busca de UI natives/classes/anotações REST (`hasUI`)
3. Decide o caminho:
   - **Console** se `!hasUI`, ou stdin for TTY (e o binário não foi
     `--gui`/`ADVPP_FORCE_GUI`), ou `ADVPP_HEADLESS_STANDALONE` setado —
     anexa `TerminalUIProvider` se for TTY de fato, roda `v.Run()`
     diretamente e sai (sem Fyne, sem `app.New()`)
   - **Fyne** nos demais casos: cria app Fyne com tema custom AdvPP, window
     com título especificado, console output widget, `VM(bc, traceMode=true)`
     com console writer + `FyneUIProvider`, DB factory (SQLite via
     `shared.ResolveStandaloneDatabasePath`), `v.Run()` em goroutine,
     `ShowAndRun()` bloqueia main; ao completar, `a.Quit()` e exit code

O caminho console nunca instancia `app.New()`/Fyne — importante para apps
batch/CI que não podem depender de um display disponível.

---

## 11. Tabela de Opcodes do Bytecode

| # | Opcode | Arg1 | Arg2 | Str | Descrição |
|---|--------|------|------|-----|-----------|
| 0 | OP_NIL | constIdx | — | — | Push nil constant |
| 1 | OP_TRUE | — | — | — | Push true |
| 2 | OP_FALSE | — | — | — | Push false |
| 3 | OP_NUMBER | constIdx | — | — | Push number constant |
| 4 | OP_STRING | constIdx | — | — | Push string constant |
| 5 | OP_DATE | constIdx | — | — | Push date constant |
| 6 | OP_LOAD_LOCAL | slotIdx | — | — | Load local var onto stack |
| 7 | OP_STORE_LOCAL | slotIdx | — | — | Store stack top into local |
| 8 | OP_LOAD_GLOBAL | nameIdx | — | constStr | Load global/dynamic var |
| 9 | OP_STORE_GLOBAL | nameIdx | — | constStr | Store global var |
| 10 | OP_LOAD_SELF | — | — | — | Push self reference (object methods) |
| 11 | OP_STORE_SELF | propIdx | — | propName | Store to self property |
| 12 | OP_NEW_ARRAY | size | — | — | Create array |
| 13 | OP_ARRAY_GET | — | — | — | array[idx] → stack |
| 14 | OP_ARRAY_SET | — | — | — | array[idx] := stack_top |
| 15 | OP_ARRAY_LEN | — | — | — | #array → stack |
| 16 | OP_NEW_OBJECT | pairCount | — | — | Create JSON-like object |
| 17 | OP_GET_PROP | propIdx | — | propName | object.prop → stack |
| 18 | OP_SET_PROP | propIdx | — | propName | stack → object.prop |
| 19 | OP_CALL_METHOD | argCount | — | methodName | obj:method(args) |
| 20 | OP_NEW_INSTANCE | classIdx | — | className | ClassName():New(args) |
| 21 | OP_CALL_FUNC | funcIdx | argCount | funcName | User function call |
| 22 | OP_CALL_NATIVE | nameIdx | argCount | funcName | Native function call |
| 23 | OP_RETURN | — | — | — | Return void |
| 24 | OP_RETURN_VALUE | — | — | — | Return stack top |
| 25 | OP_POP | — | — | — | Pop stack top |
| 26 | OP_JUMP | target | — | — | Unconditional jump |
| 27 | OP_JUMP_IF_FALSE | target | — | — | Conditional jump |
| 28 | OP_JUMP_IF_TRUE | target | — | — | Jump if true |
| 29 | OP_ADD | — | — | — | Addition / concat |
| 30 | OP_SUB | — | — | — | Subtraction |
| 31 | OP_MUL | — | — | — | Multiplication |
| 32 | OP_DIV | — | — | — | Division |
| 33 | OP_MOD | — | — | — | Modulo |
| 34 | OP_POW | — | — | — | Power |
| 35 | OP_NEG | — | — | — | Negation |
| 36 | OP_EQ | — | — | — | Equality |
| 37 | OP_NEQ | — | — | — | Not equal |
| 38 | OP_LT | — | — | — | Less than |
| 39 | OP_GT | — | — | — | Greater than |
| 40 | OP_LTE | — | — | — | Less/equal |
| 41 | OP_GTE | — | — | — | Greater/equal |
| 42 | OP_AND | — | — | — | Logical AND |
| 43 | OP_OR | — | — | — | Logical OR |
| 44 | OP_NOT | — | — | — | Logical NOT |
| 45 | OP_DOLLAR | — | — | — | $ operator (contains) |
| 46 | OP_CONCAT | — | — | — | ++ string concat |
| 47 | OP_NEW_CODEBLOCK | offset | — | — | Create code block closure |
| 48 | OP_EVAL_CODEBLOCK | — | — | — | Evaluate code block |
| 49 | OP_TRY_BEGIN | — | — | — | Begin try block |
| 50 | OP_TRY_END | — | — | — | End try block |
| 51 | OP_THROW | — | — | — | Throw exception |
| 52 | OP_CATCH | catchSlot | — | — | Catch variable slot |
| 53 | OP_DB_SELECT | aliasIdx | — | aliasName | Select work area |
| 54 | OP_DB_SEEK | — | — | — | Seek in current area |
| 55 | OP_DB_SKIP | count | — | — | Skip n records |
| 56 | OP_DB_GOTOP | — | — | — | Go to first |
| 57 | OP_DB_GOBOTTOM | — | — | — | Go to last |
| 58 | OP_EOF | — | — | — | Is EOF? |
| 59 | OP_BOF | — | — | — | Is BOF? |
| 60 | OP_FIELD_GET | fieldIdx | — | fieldName | Get field value |
| 61 | OP_FIELD_PUT | fieldIdx | — | fieldName | Set field value |
| 62 | OP_REC_LOCK | — | — | — | Lock record |
| 63 | OP_MS_UNLOCK | — | — | — | Unlock record |
| 64 | OP_MVC_NEW_MODEL | modelIdx | — | modelName | Create MVC model |
| 65 | OP_MVC_NEW_VIEW | viewIdx | — | viewName | Create MVC view |
| 66 | OP_MVC_NEW_BROWSE | browseIdx | — | browseName | Create MVC browse |
| 67 | OP_MVC_ADD_FIELD | fieldIdx | — | fieldName | Add field to model |
| 68 | OP_MVC_ADD_COMPONENT | compIdx | — | compName | Add component to view |
| 69 | OP_MVC_ADD_COLUMN | colIdx | — | colName | Add column to browse |
| 70 | OP_MVC_SET_PROPERTY | propIdx | — | propName | Set MVC property |
| 71 | OP_MVC_GET_PROPERTY | propIdx | — | propName | Get MVC property |
| 72 | OP_MVC_VALIDATE | — | — | — | Validate model |
| 73 | OP_MVC_SHOW | — | — | — | Show MVC screen |
| 74 | OP_MACRO | — | — | — | Macro substitution |
| 75 | OP_HALT | — | — | — | Terminate program |
| 76 | OP_DUP | — | — | — | Duplicate stack top |
| 77 | OP_SWAP | — | — | — | Swap top two stack items |
| 78 | OP_JUMP_IF_FALSE_OR_POP | target | — | — | Short-circuit pop |
| 79 | OP_POP_AND_JUMP | target | — | — | Conditional pop-jump |
| 80 | OP_NAMED_ARG | argNameIdx | — | paramName | Named arg marker |
| 81 | OP_FORLOOP_CMP | — | — | — | For-loop condition (step-aware) |
| 82 | OP_LOAD_UPVAL | upvalIdx | — | — | Load upvalue |
| 83 | OP_STORE_UPVAL | upvalIdx | — | — | Store upvalue |
| 84 | OP_LOAD_DYN | dynIdx | — | — | Load dynamic var |
| 85 | OP_STORE_DYN | dynIdx | — | — | Store dynamic var |
| 86 | OP_DECL_DYN | dynIdx | — | name | Declare dynamic (Private/Public) |

---

## 12. Resumo de Coverage

### O que REALMENTE Funciona (testado e validado)

| Componente | Status | Evidência |
|------------|--------|-----------|
| **Lexer** | ✅ Full | 80+ keywords, CP-1252, bracket strings, dot literals, begin content |
| **Parser** | ✅ Full | 200+ AST nodes, Pratt precedence, DSL desugaring, named params |
| **Codegen** | ✅ Full | 70+ opcodes, closures, named args, IIF inline, constant dedup |
| **VM Core** | ✅ Full | Stack machine, try/catch, dynamic scoping, operator overloading |
| **Native Fns (strings)** | ✅ Full | 35+ functions tested |
| **Native Fns (math)** | ✅ Full | 25+ functions tested |
| **Native Fns (date/time)** | ✅ Full | 14+ functions tested |
| **Native Fns (arrays)** | ✅ Full | 12+ functions tested including ASort/AEval/AScan with blocks |
| **Native Fns (system)** | ✅ Full | Hash, file, env, sleep, findfunction, startjob |
| **Native Fns (dialog)** | ✅ Full | MsgInfo/Stop/Alert/YesNo, Menu, InputText |
| **Native Fns (file IO)** | ✅ Full | FCreate, FOpen, FReadStr, FWrite, FSeek, FClose, FError, Memoread/Memowrite |
| **Native Fns (DB workarea)** | ✅ Full | SelectArea, Seek, Skip, GoTop, RecLock, MsUnlock, Append |
| **Native Fns (raw SQL)** | ✅ Full | TCsqlExec, TCsqlQuery via SQLEngine interface |
| **Native Fns (JSON)** | ✅ Full | JsonObject, GetNames, FromJson (parser real, `encoding/json`) |
| **Native Fns (TUI/terminal)** | ✅ Full | UiBox, UiStreamBox/Reset, UiMarkdown (glamour), UiAltScreenEnter/Exit, UiTermWidth, ConOutRaw, ProcRun |
| **Native Fns (MVC)** | ✅ Full | FwFormModel, FwFormView, FwFormBrowse, FwMBrowse |
| **Native Fns (FWGridProcess)** | ✅ Full | Pool com threads, meters, afterExecute |
| **Native Fns (geometry)** | ✅ Full | Vec add/sub/dot/cross/norm/normalize/dist/angle/scale/rotate |
| **Native Fns (math stat)** | ✅ Full | Ceil, sinh/cosh/tanh, gcd/lcm/fact, mean/var/stddev/median, linreg, interp |
| **Native Fns (LLM)** | ✅ Full | GGUF I2_S load, tokenize, generate, close |
| **Native Fns (TMailMessage)** | ✅ Full | Setup SMTP + send |
| **Native Fns (MCPServer)** | ✅ Full | AddTool + Serve sobre stdio |
| **Native Fns (WSRestServer)** | ✅ Full | Routes + Serve com annotation scanning |
| **SQLite Engine** | ✅ Full | Workarea emulation, field get/put, append, unlock (msunlock commit) |
| **MsDialog renderer** | ✅ Full (web + desktop) | SAY/GET/BUTTON/BOX layout grid heuristic |
| **FWMBrowse renderer** | ✅ Full (web + desktop) | Auto-CRUD from SX3 + PRAGMA |
| **Web UI (SSE)** | ✅ Full | Per-session VM, async UI blocking, hot reload |
| **Desktop UI (Fyne)** | ✅ Full | Editor, console, file tree, MVC renderer, dialogs |
| **Tensor ops** | ✅ Full | 2D math, matmul, transpose, reshape, reductions, linalg |
| **Autograd** | ✅ Full | Tape-based reverse diff, SGD, Adam |
| **Neural Network layers** | ✅ Full | Linear, Embedding with Forward/Params |
| **DAP Debugger** | ✅ Full | Breakpoints, step, variables, stack frames |
| **Macro substitution** | ✅ Full | &var and &(expr) pipeline |
| **StartJob** | ✅ Full | Isolated VM per job, sync/async |
| **CP-1252 conversion** | ✅ Full | Full 256-byte table lookup |
| **Preprocessor** | ✅ Full | #include, #define, #ifdef, #xcommand, BEGINSQL |
| **Bytecode serialization** | ✅ Full | Save/Load .abf JSON |
| **Standalone build** | ✅ Full | Embed bytecode → native executable; console ou Fyne GUI (auto-detect, ou fixado via `--gui`) |
| **Check (multi-file parallel)** | ✅ Full | Worker pool per CPU |
| **Watch / Hot Reload** | ✅ Full | mtime polling 500ms |

### Stubs / Não Implementados (retornem valores placeholder)

| Função | Retorno Stub | Observação |
|--------|-------------|------------|
| `GETMV(key, default)` | `default` | Parâmetros MV não configurados |
| `MAKEDIR(path)` | Nil | Não implementado |
| `PROCNAME()` | `""` | Não implementado |
| `PROCLINE()` | `0` | Não implementado |
| `SETDATE(fmt)` | Nil | Apenas referencia |
| `SETCENT(n)` | Nil | Apenas referencia |
| `EMPNAME(cEmp)` | `"USER"` | Hardcoded stub |
| `FWALIASINDIC()` | `.F.` | Hardcoded stub |
| `FWMODEACCESS()` | `1` | Hardcoded stub |
| `FWHASACCMODE()` | `.T.` | Hardcoded stub |
| `FWLIBVERSION()` | `"1.0.0"` | Hardcoded version |
| Todas funções `MP*` | Nil/Bool/String stub | Módulos MultiPrint não carregados |
| `FWRESTAREA/FWGETAREA/etc.` | Nil/String/Array | Framework apps stubs |
| `ChangeQuery/TcSqlExec/TcSqlQuery` | Nil/Bool | Via SQLEngine interface em browse.go, não no workarea DBEngine |
| Variadas funções FW* de framework Protheus | Valores placeholder | Funções de UI complexa (FWTreeView, FWTabs, etc.) |
| `FWTreeview` | Nil (stub) | Não mapeado na VM (classe não registrada) |

### Observações Importantes

1. **CP-1252 obrigatório:** Todos os fontes `.prw`/`.prg`/`.tlpp` DEVEM estar em Windows-1252. O conversor automático está incluído no pipeline, mas depende da tabela `cp1252ToUTF8`.

2. **D_E_L_E_T_ = ' ':** Todas as queries de leitura filtram soft-delete implicitamente (via `DBCLOSEAREA` e browse engine).

3. **Filial:** `xFilial('XXX')` retorna `""` (stub) — projetos reais devem configurar o valor no código.

4. **Thread safety:** VMs de diferentes jobs são totalmente isoladas (nenhum estado compartilhado). O mesmo banco SQLite é aberto por múltiplas conexões (WAL mode).

5. **Operator overloading:** Objetos podem definir métodos `OPERATOR_ADD`, `OPERATOR_SUB`, etc. que são invocados automaticamente.

6. **Code blocks closures:** Suporte completo a upvalues — code blocks acessam variáveis do escopo pai via `OP_LOAD_UPVAL`/`OP_STORE_UPVAL`.

7. **Named arguments:** Chamadas suportam `func(paramNome := valor)` — a VM reordena via `reorderNamedArgs()`.

8. **IIF/IF inline:** Compiladas com short-circuit — só avalia o ramo escolhido (não é uma função biasa).
