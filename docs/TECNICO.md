# Documentação Técnica - AdvPP

## Visão Geral da Arquitetura

O AdvPP é uma suite completa de ferramentas para desenvolvimento AdvPL/TLPP, construída em Go e utilizando o framework Fyne para interface gráfica. A arquitetura é modular e extensível, permitindo fácil manutenção e adição de novas funcionalidades.

### Componentes Principais

```
AdvPP/
├── cmd/                    # Executáveis das ferramentas
│   ├── advpp-ide/         # IDE Principal
│   ├── advcfg/            # Configurador de Tabelas
│   ├── adveditor/         # Editor de Banco de Dados
│   └── advplc/            # Compilador
├── pkg/                    # Pacotes compartilhados
│   ├── compiler/          # Compilador AdvPL/TLPP
│   ├── mvc/               # Framework MVC
│   ├── tools/shared/      # Ferramentas compartilhadas
│   ├── ui/                # Componentes de UI
│   └── vm/                # Máquina Virtual
└── tests/                  # Testes e exemplos
```

## Compilador (pkg/compiler)

### Arquitetura do Compilador

O compilador AdvPL/TLPP processa código fonte e gera bytecode para execução na máquina virtual.

#### Fases de Compilação

1. **Análise Léxica**: Tokenização do código fonte
2. **Análise Sintática**: Construção da árvore de sintaxe abstrata (AST)
3. **Análise Semântica**: Verificação de tipos e escopo
4. **Geração de Código**: Produção de bytecode

#### Estrutura de Dados

```go
// Opcode representa uma instrução de bytecode
type Opcode struct {
    Code    byte
    Operand interface{}
}

// Contexto de compilação
type CompilerContext struct {
    Symbols    map[string]Symbol
    Constants  []interface{}
    Functions  []Function
}
```

### Suporte a TLPP

O compilador suporta extensões modernas da linguagem TLPP:

- **Classes**: Herança, interfaces, modificadores de acesso
- **Operadores**: Sobrecarga de operadores
- **JSON**: Sintaxe inline para objetos JSON
- **Try/Catch**: Tratamento de exceções
- **Tipagem**: Tipos estáticos e inferência de tipos
- **Namespaces**: Organização de código em namespaces

## Máquina Virtual (pkg/vm)

### Arquitetura da VM

A máquina virtual executa bytecode gerado pelo compilador, utilizando uma pilha para operandos e um contexto de execução.

#### Componentes

```go
// VM representa a máquina virtual
type VM struct {
    Stack      []interface{}
    Constants  []interface{}
    Globals    map[string]interface{}
    IP         int          // Instruction Pointer
    Functions  map[string]*Function
}

// Contexto de execução
type ExecutionContext struct {
    Locals    map[string]interface{}
    Arguments []interface{}
    Return    interface{}
}
```

#### Conjunto de Instruções

- **Pilha**: PUSH, POP, DUP, SWAP
- **Aritmética**: ADD, SUB, MUL, DIV, MOD
- **Comparação**: EQ, NE, LT, GT, LE, GE
- **Lógica**: AND, OR, NOT
- **Controle**: JMP, JZ, JNZ, CALL, RET
- **Memória**: LOAD, STORE, GETGLOBAL, SETGLOBAL
- **Objetos**: NEWOBJ, GETPROP, SETPROP, CALLMETH

### Funções Nativas

Funções nativas implementadas em Go para performance:

- **I/O**: Print, Input, File operations
- **String**: Len, Substr, Upper, Lower, Trim
- **Matemática**: Sin, Cos, Tan, Sqrt, Abs
- **Data**: Date, Time, Now
- **Banco de Dados**: Query, Exec, Begin, Commit, Rollback

## Framework MVC (pkg/mvc)

### Arquitetura MVC

O framework MVC implementa o padrão Model-View-Controller para construção de interfaces de usuário.

#### Componentes

```go
// ModelDef define a estrutura de dados
type ModelDef struct {
    Fields    []FieldDef
    Indexes   []IndexDef
    Triggers  []TriggerDef
}

// ViewDef define a apresentação
type ViewDef struct {
    Layout    LayoutType
    Widgets   []WidgetDef
    Events    []EventDef
}

// BrowseDef define a lista/grid
type BrowseDef struct {
    Columns   []ColumnDef
    Filters   []FilterDef
    Actions   []ActionDef
}
```

### Tipos de Layout

- **FormLayout**: Layout de formulário
- **TableLayout**: Layout de tabela
- **BoxLayout**: Layout de caixa horizontal/vertical
- **GridLayout**: Layout de grade

## Ferramentas Compartilhadas (pkg/tools/shared)

### Dicionário de Dados

O dicionário de dados centraliza metadados de tabelas do Protheus.

#### Estrutura de Tabelas

- **SX2**: Metadados de tabelas
- **SX3**: Estrutura de campos
- **SIX**: Definição de índices
- **SX7**: Triggers de banco
- **SX5**: Tabelas genéricas
- **SX6**: Parâmetros do sistema
- **SXB**: Perguntas do help

```go
// Dictionary representa o dicionário de dados
type Dictionary struct {
    db         *sql.DB
    loaded     bool
    tables     map[string]Table
    fields     map[string][]Field
    indexes    map[string][]Index
}
```

### Banco de Dados

Suporte a múltiplos drivers de banco de dados:

- **SQLite**: Banco de dados embutido (padrão)
- **DBF**: Arquivos DBF (compatibilidade Protheus)
- **TopConnect**: Conexão via TopConnect
- **Ctree**: Banco Ctree
- **BTrieve**: Banco BTrieve

```go
// Database representa uma conexão de banco
type Database struct {
    Driver    string
    Path      string
    Shared    bool
    ReadOnly  bool
    conn      *sql.DB
    structure map[string]TableStructure
}
```

### Configuração Compartilhada

Sistema de configuração compartilhado entre ferramentas:

```go
// Config representa a configuração
type Config struct {
    DefaultDatabase string `json:"default_database"`
    RecentFiles     []string `json:"recent_files"`
    EditorSettings  EditorConfig `json:"editor_settings"`
}
```

## Interface de Usuário (pkg/ui)

### Renderer

O renderer é responsável por desenhar componentes da interface:

```go
// Renderer desenha componentes da UI
type Renderer struct {
    Canvas    *canvas.Canvas
    Theme     Theme
    Fonts     map[string]resource.Resource
}

// Componentes suportados
type Component interface {
    Render(*Renderer)
    HandleEvent(Event)
}
```

### Componentes Disponíveis

- **TreeView**: Árvore hierárquica
- **DataGrid**: Grade de dados
- **FormEdit**: Formulário de edição
- **CodeEditor**: Editor de código
- **StatusBar**: Barra de status
- **Toolbar**: Barra de ferramentas

## Integração Entre Ferramentas

### Fluxo de Dados

```
AdvCfg → Dictionary → AdvEditor → IDE
   ↓         ↓            ↓         ↓
  SX2      SQLite      Tables   Autocomplete
  SX3      Data        Fields   Validation
  SIX      Cache       Indexes  Code Gen
```

### Comunicação

As ferramentas se comunicam através de:

1. **Banco de Dados Compartilhado**: `./data/advpl_dictionary.db`
2. **Arquivo de Configuração**: `~/.advpp/advpp_config.json`
3. **API Interna**: Chamadas diretas entre pacotes

## Performance

### Otimizações Implementadas

- **Cache do Dicionário**: Metadados em memória
- **Lazy Loading**: Carregamento sob demanda
- **Batch Operations**: Operações em lote
- **Connection Pooling**: Pool de conexões
- **Precompiled Statements**: Statements preparados

### Métricas

- **Tempo de Compilação**: < 100ms para arquivos médios
- **Tempo de Execução**: < 10ms para operações simples
- **Uso de Memória**: < 50MB para IDE
- **Startup Time**: < 2 segundos

## Segurança

### Implementações de Segurança

- **SQL Injection**: Uso de parâmetros em queries
- **Path Traversal**: Validação de caminhos
- **Input Validation**: Validação de entrada
- **Error Handling**: Tratamento de erros
- **Logging**: Log de operações

## Extensibilidade

### Plugins

O sistema suporta plugins através de:

1. **Funções Nativas**: Registro de funções Go
2. **Componentes UI**: Componentes customizados
3. **Drivers DB**: Drivers de banco customizados
4. **Compilador**: Extensões de linguagem

### API Pública

```go
// API para extensão
type Plugin interface {
    Name() string
    Version() string
    Init(*Context) error
    Shutdown() error
}
```

## Testes

### Estrutura de Testes

```
tests/
├── unit/              # Testes unitários
├── integration/       # Testes de integração
├── e2e/              # Testes end-to-end
└── benchmarks/       # Benchmarks
```

### Cobertura

- **Unit Tests**: > 80% de cobertura
- **Integration Tests**: Fluxos principais
- **E2E Tests**: Cenários de usuário

## Build e Deploy

### Build

```bash
# Build de todas as ferramentas
go build -o build/advpp-ide ./cmd/advpp-ide
go build -o build/advcfg ./cmd/advcfg
go build -o build/adveditor ./cmd/adveditor
go build -o build/advplc ./cmd/advplc
```

### Pacote Debian

```bash
# Criar pacote .deb
dpkg-deb --build debian advpp_1.0.0_amd64.deb
```

### Instalação

```bash
# Instalar pacote
sudo dpkg -i advpp_1.0.0_amd64.deb
```

## Troubleshooting

### Problemas Comuns

**Erro de compilação:**
- Verifique sintaxe do código
- Confirme imports corretos
- Valide tipos de dados

**Erro de execução:**
- Verifique bytecode válido
- Confirme dependências
- Valide contexto de execução

**Erro de banco de dados:**
- Verifique conexão
- Confirme permissões
- Valide estrutura

## Roadmap

### Próximas Versões

- **v1.1**: Debugger integrado
- **v1.2**: Suporte a REST API
- **v1.3**: Interface web
- **v2.0**: Refatoração completa

## Referências

- [Documentação Protheus](https://tdn.totvs.com)
- [Documentação TLPP](https://tdn.totvs.com/display/tec/TLPP)
- [Framework Fyne](https://fyne.io)
- [Go Language](https://golang.org)
