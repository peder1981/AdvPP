# Design de Ferramentas AdvEditor e AdvCfg

## Visão Geral

Baseado na análise do código TOTVS em `/home/peder/Downloads/811R4/`, vamos criar duas ferramentas modernas em Go com Fyne:

1. **AdvEditor** - Editor de Banco de Dados (inspirado em APSDU)
2. **AdvCfg** - Configurador de Tabelas (inspirado em SIGACFG)

## Arquitetura Compartilhada

### Estrutura de Diretórios

```
cmd/
  adveditor/          # AdvEditor standalone
  advcfg/             # AdvCfg standalone
pkg/
  tools/
    shared/           # Componentes compartilhados
      database.go     # Abstração de banco de dados
      treeview.go     # Tree view genérico
      grid.go         # Grid de dados genérico
      dialogs.go      # Diálogos comuns
    adveditor/        # Componentes específicos AdvEditor
      main.go         # Janela principal
      menu.go         # Menu e toolbar
      table_manager.go # Gerenciamento de tabelas
      data_editor.go  # Editor de dados
      structure.go    # Editor de estrutura
      index.go        # Gerenciamento de índices
    advcfg/           # Componentes específicos AdvCfg
      main.go         # Janela principal
      tree.go         # Tree de dicionário
      sx2_editor.go   # Editor de tabelas (SX2)
      sx3_editor.go   # Editor de campos (SX3)
      six_editor.go   # Editor de índices (SIX)
      sx7_editor.go   # Editor de triggers (SX7)
      sx5_editor.go   # Editor de genéricas (SX5)
      sx6_editor.go   # Editor de parâmetros (SX6)
      sxb_editor.go   # Editor de perguntas (SXB)
```

## AdvEditor - Editor de Banco de Dados

### Funcionalidades Principais

**Menu File:**
- Open (Ctrl+B) - Abrir tabela (DBF, TopConnect, Ctree, BTrieve)
- Close - Fechar tabela
- Structure - Ver estrutura da tabela
- New Structure - Criar nova estrutura
- Fields Position - Reordenar campos
- Status - Status da tabela
- Import (Ctrl+T) - Importar dados
- Exit - Sair

**Menu Util:**
- Copy To (Ctrl+Y) - Copiar dados
- Append From (Ctrl+A) - Anexar dados
- Pack (Ctrl+P) - Compactar tabela
- Zap (Ctrl+Z) - Limpar tabela
- Drop Table (Ctrl+D) - Excluir tabela
- Recall - Recuperar registros deletados
- Delete - Deletar registro
- Filter (Ctrl+F) - Filtrar dados
- Replace (Ctrl+R) - Substituir dados
- Count - Contar registros
- Sum - Somar campos
- Set Deleted - Mostrar deletados
- Set Softseek - Soft seek
- Customize - Personalizar
- Query Analyzer - Analisador de queries

**Menu Index:**
- Open (Ctrl+I) - Abrir índice
- Create - Criar índice
- Close - Fechar índice
- Erase All - Apagar todos
- Order (Ctrl+E) - Ordenar

**Menu Edit:**
- Include - Incluir registro
- Change - Alterar registro
- Exclude - Excluir registro

**Menu Find:**
- Locate (Ctrl+L) - Localizar
- Go To (Ctrl+G) - Ir para
- Seek (Ctrl+S) - Seek

### Componentes

**Database Abstraction:**
```go
type DatabaseDriver interface {
    Open(file string, readOnly, shared bool) error
    Close() error
    GetStructure() []Field
    GetData() []Record
    AddRecord(record Record) error
    UpdateRecord(recno int, record Record) error
    DeleteRecord(recno int) error
    Pack() error
    Zap() error
    GetIndexes() []Index
    CreateIndex(name string, expression string) error
}
```

**Table Manager:**
- Gerencia múltiplas tabelas abertas
- Mantém histórico de operações
- Suporta diferentes drivers (DBF, SQL, etc.)

**Data Editor:**
- Grid editável com suporte a tipos
- Navegação por página
- Filtros e busca
- Edição inline
- Validação de dados

**Structure Editor:**
- Visualização de campos
- Adicionar/remover campos
- Modificar tipos e tamanhos
- Reordenar campos

**Index Manager:**
- Visualização de índices
- Criar/editar/excluir índices
- Expressões de índice

## AdvCfg - Configurador de Tabelas

### Funcionalidades Principais

**Tree de Navegação:**
- Empresa
  - Dicionário de Dados (SX2)
    - Grupo de Campos (SXG)
    - Gatilhos (SX7)
    - Tabelas Genéricas (SX5)
    - Parâmetros (SX6)
    - Perguntas (SXB)
    - Consultas Padrão (SXB)

**Editores de Dicionário:**

**SX2 (Tabelas):**
- Cadastro de tabelas
- Propriedades: nome, descrição, módulo
- Estrutura de campos
- Índices

**SX3 (Campos):**
- Cadastro de campos
- Propriedades: nome, tipo, tamanho, decimal, validação
- Picture
- Help
- Validações

**SIX (Índices):**
- Cadastro de índices
- Propriedades: nome, ordem, expressão, filtro
- Chaves compostas

**SX7 (Triggers):**
- Cadastro de triggers
- Propriedades: tabela, evento, código
- Eventos: BeforeInsert, AfterInsert, BeforeUpdate, etc.

**SX5 (Genéricas):**
- Cadastro de tabelas genéricas
- Propriedades: código, descrição, tipo

**SX6 (Parâmetros):**
- Cadastro de parâmetros
- Propriedades: código, descrição, tipo, valor padrão
- Por módulo

**SXB (Perguntas):**
- Cadastro de perguntas
- Propriedades: código, pergunta, tipo, help
- Variáveis

### Componentes

**Dictionary Manager:**
- Gerencia SX2, SX3, SIX, SX7, SX5, SX6, SXB
- Validação de integridade
- Geração de código

**Tree Navigator:**
- Navegação hierárquica
- Filtros e busca
- Estado expandido/colapsado

**Field Editor:**
- Editor de campos com validação
- Suporte a todos os tipos AdvPL
- Picture editor

**Index Editor:**
- Editor de expressões de índice
- Validação de sintaxe
- Preview de chaves

**Trigger Editor:**
- Editor de código AdvPL
- Highlight de sintaxe
- Validação

## Integração com IDE

### Menu no advpp-ide

```go
// Adicionar menu Tools
toolsMenu := fyne.NewMenu("Tools", fyne.NewMenuItem("AdvEditor", func() {
    RunAdvEditor()
}))
toolsMenu.Items = append(toolsMenu.Items, fyne.NewMenuItem("AdvCfg", func() {
    RunAdvCfg()
}))
```

### Compartilhamento de Contexto

- Compiler compartilhado
- VM compartilhada
- Configurações compartilhadas
- Histórico compartilhado

## Implementação Faseada

### Fase 1: Estrutura Base
- Criar estrutura de diretórios
- Implementar abstração de banco de dados
- Implementar componentes compartilhados (treeview, grid, dialogs)

### Fase 2: AdvEditor Core
- Implementar janela principal
- Implementar menu e toolbar
- Implementar gerenciamento de tabelas
- Implementar editor de dados básico

### Fase 3: AdvEditor Avançado
- Implementar editor de estrutura
- Implementar gerenciador de índices
- Implementar operações (pack, zap, etc.)
- Implementar Query Analyzer

### Fase 4: AdvCfg Core
- Implementar janela principal
- Implementar tree de dicionário
- Implementar editor SX2 básico

### Fase 5: AdvCfg Avançado
- Implementar editores SX3, SIX, SX7
- Implementar editores SX5, SX6, SXB
- Implementar validação de dicionário
- Implementar geração de código

### Fase 6: Integração IDE
- Integrar AdvEditor no IDE
- Integrar AdvCfg no IDE
- Implementar comunicação entre ferramentas
- Testar integração completa

## Stack Tecnológico

- **Linguagem**: Go
- **UI Framework**: Fyne (fyne.io/fyne/v2)
- **Banco de Dados**: SQLite (local), drivers para DBF, SQL
- **Persistência**: JSON para configurações
- **Editor de Código**: Integração com CodeEditor existente

## Próximos Passos

1. Criar estrutura de diretórios
2. Implementar abstração de banco de dados
3. Implementar componentes compartilhados
4. Criar AdvEditor standalone
5. Criar AdvCfg standalone
6. Integrar com advpp-ide
7. Testar funcionalidades
8. Documentar uso
