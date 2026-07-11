# AdvEditor - Ferramenta de Banco de Dados AdvPL

## Visão Geral

O AdvPP inclui uma ferramenta gráfica única para gerenciamento de banco de
dados AdvPL, inspirada na ferramenta TOTVS APSDU: o **AdvEditor**. Ele
substitui as antigas duas ferramentas separadas (AdvEditor + AdvCfg,
descontinuada) — a mesma janela agora conecta ao banco SQLite local, cria/
edita/exclui tabelas e campos, e edita os dados linha a linha.

## Compatibilidade SQLite

**IMPORTANTE:** O AdvEditor é **100% compatível com SQLite**, o banco
padrão de todas as ferramentas AdvPP.

**Funcionalidades SQLite:**
- Abertura automática do banco local (`./advpp.db` no diretório de
  trabalho atual, criado automaticamente se ainda não existir — ver
  `pkg/tools/shared.ResolveDatabasePath`) ou de arquivos `.db`/`.sqlite`/
  `.sqlite3` escolhidos manualmente
- Criação e exclusão de tabelas (Tabela → Nova Tabela / Excluir Tabela)
- Adição e remoção de campos em tabelas existentes (ALTER TABLE)
- Leitura de estrutura via `PRAGMA table_info`, índices via
  `PRAGMA index_list`/`PRAGMA index_info`
- CRUD completo de registros (Incluir/Alterar/Excluir no menu Editar)
- **Exclusão lógica no estilo Protheus**: toda tabela criada pelo
  AdvEditor ganha as colunas de sistema `R_E_C_N_O_` (recno estável,
  auto-incrementado), `D_E_L_E_T_` (marcador `' '`/`'*'`) e
  `R_E_C_D_E_L_` (gêmeo booleano) — Excluir marca a linha como deletada
  em vez de removê-la de verdade; leituras (grid, Contar, Somar) escondem
  linhas deletadas por padrão
- Criação e exclusão de índices (menu Índice)
- Navegação por paginação (LIMIT/OFFSET)

**Conversão de Tipos:**
- SQLite INTEGER → AdvPL Numérico (N, 10)
- SQLite REAL/FLOAT/DOUBLE → AdvPL Numérico (N, 14, 4)
- SQLite TEXT/VARCHAR/CHAR → AdvPL Caracter (C, 50)
- SQLite BLOB → AdvPL Memo (M)

**Uso:**
```bash
# Abrir o banco local do diretório atual (criado automaticamente se preciso)
cd meu-projeto/
./adveditor

# Ou apontar para um banco específico
ADVPP_DB=/caminho/banco.db ./adveditor
```

## Funcionalidades do Menu

**Menu Arquivo:**
- **Abrir (Ctrl+B)** — abrir tabela (SQLite; DBF/TopConnect/Ctree/BTrieve
  têm suporte parcial, ver "Drivers Suportados" abaixo)
- **Trocar Banco de Dados** — apontar para outro arquivo
- **Fechar** — fechar tabela atual
- **Sair**

**Menu Tabela:**
- **Nova Tabela** — nome + lista de campos (nome/tipo/tamanho/decimal,
  adicionáveis dinamicamente) → `CREATE TABLE`
- **Excluir Tabela** — com confirmação
- **Estrutura** — visualiza os campos da tabela atual
- **Adicionar Campo** / **Remover Campo** — `ALTER TABLE ADD/DROP COLUMN`

**Menu Editar:**
- **Incluir** — formulário com um campo por coluna da tabela atual
- **Alterar** — edita o registro selecionado no grid
- **Excluir** — exclusão lógica do registro selecionado (ver acima)

**Menu Índice:**
- **Criar** — nome + campos (`CAMPO1+CAMPO2`, convenção Clipper)
- **Excluir** — escolhe entre os índices existentes da tabela

**Menu Ajuda:**
- **Sobre**

### Interface

- **Tree View (Esquerda)**: lista de tabelas do banco aberto
- **Data Grid (Direita)**: visualização de dados, clique numa linha para
  selecioná-la (usado por Alterar/Excluir)
- **Status Bar (Inferior)**: mensagens de status/erro

### Drivers Suportados

- **SQLite** — **100% compatível**, todas as funcionalidades acima
- **DBF**, **TopConnect**, **Ctree**, **BTrieve** — abertura e leitura de
  dados; criação/exclusão de tabela e campo ainda são SQLite-only (ver
  `pkg/tools/shared/database.go`, método `SQLiteDriver.CreateTable` etc.)

**Detecção Automática:**
- Arquivos `.db`, `.sqlite`, `.sqlite3` → SQLite
- Arquivos `.dbf` → DBF
- Outros → DBF (padrão)

### Uso

```bash
./adveditor
```

### Arquitetura

- **pkg/tools/shared/database.go** — abstração de banco (`DatabaseDriver`
  interface, `TableManager`, `SQLiteDriver` com DDL+CRUD completo)
- **pkg/tools/shared/treeview.go** — componente de tree view genérico
- **pkg/tools/shared/config.go** — resolução do banco compartilhado
  (`ResolveDatabasePath`)
- **cmd/adveditor/main.go** — aplicação principal

## Stack Tecnológico

- **Linguagem**: Go 1.24
- **UI Framework**: Fyne v2.4.4
- **Persistência**: SQLite (`modernc.org/sqlite`, puro Go, sem CGO)
- **Arquitetura**: componentes compartilhados com advplc/advpp-ide
  (`pkg/tools/shared`)

## Próximos Passos

- Edição inline de dados no grid (em vez de formulário modal)
- Query Analyzer / filtros e busca avançada
- Pack (purgar exclusões lógicas) e Zap (limpar tabela) expostos na UI —
  já implementados no driver (`SQLiteDriver.Pack`/`Zap`), faltam os menus
- DDL (CreateTable/AddColumn/etc.) para drivers além de SQLite
- Integrar AdvEditor no menu Tools do advpp-ide

## Referências

- **APSDU** — Protheus Database Utility (TOTVS), referência de UX para o
  fluxo de estrutura de tabela (campos como lista editável) e navegação
- **Fyne** — Cross-platform GUI toolkit for Go
- **AdvPP** — Compilador e IDE AdvPL

## Licença

Segue a mesma licença do projeto AdvPP.
