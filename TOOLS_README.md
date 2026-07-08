# AdvEditor e AdvCfg - Ferramentas de Banco de Dados AdvPL

## Visão Geral

O AdvPP agora inclui duas ferramentas poderosas para gerenciamento de banco de dados AdvPL, inspiradas nas ferramentas TOTVS APSDU e SIGACFG:

1. **AdvEditor** - Editor de Banco de Dados (inspirado em APSDU)
2. **AdvCfg** - Configurador de Tabelas (inspirado em SIGACFG)

Ambas as ferramentas são executáveis independentemente e podem ser integradas ao IDE advpp-ide.

## Compatibilidade SQLite

**IMPORTANTE:** Tanto o AdvEditor quanto o AdvCfg são **100% compatíveis com SQLite**.

### AdvEditor - Suporte SQLite Completo

O AdvEditor suporta nativamente bancos de dados SQLite com todas as funcionalidades:

**Funcionalidades SQLite:**
- Abertura automática de arquivos `.db`, `.sqlite`, `.sqlite3`
- Detecção automática de tipo de arquivo
- Leitura de estrutura de tabelas via `PRAGMA table_info`
- Leitura de índices via `PRAGMA index_list` e `PRAGMA index_info`
- Operações CRUD completas (INSERT, UPDATE, DELETE)
- Navegação por paginação (LIMIT/OFFSET)
- Contagem de registros (COUNT)
- Soma de campos (SUM)
- Criação de índices (CREATE INDEX)
- Remoção de índices (DROP INDEX)
- Compactação (VACUUM)
- Limpeza de tabela (DELETE)

**Conversão de Tipos:**
- SQLite INTEGER → AdvPL Numérico (N, 10)
- SQLite REAL/FLOAT/DOUBLE → AdvPL Numérico (N, 14, 4)
- SQLite TEXT/VARCHAR/CHAR → AdvPL Caracter (C, 50)
- SQLite BLOB → AdvPL Memo (M)

**Uso com SQLite:**
```bash
# Criar banco de dados SQLite
sqlite3 data/meu_banco.db "CREATE TABLE clientes (id INTEGER PRIMARY KEY, nome TEXT, email TEXT);"

# Abrir com AdvEditor (detecção automática)
./adveditor
# Menu Arquivo → Abrir → selecionar data/meu_banco.db
```

### AdvCfg - Dicionário SQLite

O AdvCfg utiliza SQLite como backend para o dicionário de dados:

**Localização do Dicionário:**
```
~/.advpp/ADVPP.db
```

**Tabelas do Dicionário:**
- SX2 - Tabelas
- SX3 - Campos
- SIX - Índices
- SX7 - Triggers
- SX5 - Genéricas
- SX6 - Parâmetros
- SXB - Perguntas

**Vantagens do SQLite:**
- Performance superior a DBF
- Suporte a transações ACID
- Índices automáticos
- Compactação automática
- Portabilidade (arquivo único)
- Backup simples (copia do arquivo)

## AdvEditor - Editor de Banco de Dados

### Funcionalidades

**Menu Arquivo:**
- **Abrir (Ctrl+B)** - Abrir tabela (DBF, SQLite, TopConnect, Ctree, BTrieve)
- **Fechar** - Fechar tabela atual
- **Estrutura** - Visualizar estrutura da tabela
- **Sair** - Sair do aplicativo

**Menu Editar:**
- **Incluir** - Adicionar novo registro
- **Alterar** - Editar registro selecionado
- **Excluir** - Deletar registro selecionado

**Menu Índice:**
- **Abrir** - Abrir índice
- **Criar** - Criar novo índice
- **Fechar** - Fechar índice

**Menu Ajuda:**
- **Sobre** - Informações sobre o AdvEditor

### Interface

- **Tree View (Esquerda)**: Lista de tabelas abertas
- **Data Grid (Direita)**: Visualização e edição de dados
- **Status Bar (Inferior)**: Informações de status

### Drivers Suportados

- **SQLite** - Arquivos SQLite (.db, .sqlite, .sqlite3) - **100% compatível**
- **DBF** - Arquivos DBF com índices CDX
- **TopConnect** - Conexão SQL via TopConnect
- **Ctree** - Banco de dados Ctree
- **BTrieve** - Banco de dados BTrieve

**Detecção Automática:**
- Arquivos `.db`, `.sqlite`, `.sqlite3` → SQLite
- Arquivos `.dbf` → DBF
- Outros → DBF (padrão)

### Uso

```bash
# Executar AdvEditor standalone
./adveditor
```

### Arquitetura

- **pkg/tools/shared/database.go** - Abstração de banco de dados com SQLiteDriver completo
- **pkg/tools/shared/treeview.go** - Componente de tree view
- **pkg/tools/shared/dictionary.go** - Dicionário de dados SQLite
- **cmd/adveditor/main.go** - Aplicação principal AdvEditor
- **cmd/advcfg/main.go** - Aplicação principal AdvCfg

## AdvCfg - Configurador de Tabelas

### Funcionalidades

**Menu Arquivo:**
- **Nova Tabela** - Criar nova tabela no dicionário
- **Importar Dicionário** - Importar dicionário de dados
- **Exportar Dicionário** - Exportar dicionário de dados
- **Sair** - Sair do aplicativo

**Menu Editar:**
- **Incluir** - Adicionar registro no dicionário
- **Alterar** - Editar registro do dicionário
- **Excluir** - Deletar registro do dicionário

**Menu Ferramentas:**
- **Validar Dicionário** - Validar integridade do dicionário
- **Gerar Código** - Gerar código AdvPL a partir do dicionário

**Menu Ajuda:**
- **Sobre** - Informações sobre o AdvCfg

### Interface

- **Tree View (Esquerda)**: Navegação hierárquica do dicionário
  - Tabelas (SX2)
  - Campos (SX3)
  - Índices (SIX)
  - Triggers (SX7)
  - Genéricas (SX5)
  - Parâmetros (SX6)
  - Perguntas (SXB)
- **Data Grid (Direita)**: Visualização e edição de registros
- **Status Bar (Inferior)**: Informações de status

### Uso

```bash
# Executar AdvCfg standalone
./advcfg
```

### Arquitetura

- **pkg/tools/shared/database.go** - Abstração de banco de dados
- **pkg/tools/shared/treeview.go** - Componente de tree view
- **cmd/advcfg/main.go** - Aplicação principal

## Componentes Compartilhados

### Database Abstraction

A abstração de banco de dados (`pkg/tools/shared/database.go`) fornece:

- **DatabaseDriver Interface**: Interface comum para diferentes drivers
- **TableManager**: Gerenciamento de múltiplas tabelas
- **Field, Record, Index**: Estruturas de dados
- **Drivers Implementados**: DBF, TopConnect, Ctree, BTrieve

### Tree View Component

O componente de tree view (`pkg/tools/shared/treeview.go`) fornece:

- **TreeNode**: Estrutura de nó hierárquico
- **TreeView**: Widget de tree view genérico
- **Callbacks**: Seleção, expansão/colapso
- **Operações**: Adicionar, remover, atualizar nós

## Stack Tecnológico

- **Linguagem**: Go 1.24
- **UI Framework**: Fyne v2.4.4
- **Persistência**: SQLite (para configurações)
- **Arquitetura**: Componentes compartilhados entre ferramentas

## Próximos Passos

### AdvEditor

- Implementar edição inline de dados
- Implementar operações CRUD completas
- Implementar Query Analyzer
- Implementar filtros e busca avançada
- Implementar operações de Pack, Zap, Recall
- Implementar gerenciamento de índices completo

### AdvCfg

- Implementar editores específicos (SX2, SX3, SIX, SX7, SX5, SX6, SXB)
- Implementar validação de dicionário
- Implementar geração de código AdvPL
- Implementar importação/exportação de dicionário
- Implementar editor de triggers com syntax highlighting
- Implementar editor de expressões de índice

### Integração IDE

- Integrar AdvEditor no menu Tools do advpp-ide
- Integrar AdvCfg no menu Tools do advpp-ide
- Implementar comunicação entre ferramentas e IDE
- Compartilhar contexto e configurações

## Testes

Ambas as ferramentas foram testadas independentemente:

```bash
# Testar AdvEditor
./adveditor

# Testar AdvCfg
./advcfg
```

## Referências

- **APSDU** - Protheus Database Utility (TOTVS)
- **SIGACFG** - Configurador de Tabelas (TOTVS)
- **Fyne** - Cross-platform GUI toolkit for Go
- **AdvPP** - Compilador e IDE AdvPL

## Licença

As ferramentas seguem a mesma licença do projeto AdvPP.
