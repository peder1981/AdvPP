# Integração AdvEditor, AdvCfg e advpp-ide

## Visão Geral

A integração entre as ferramentas AdvPP é fundamental para proporcionar uma experiência completa de desenvolvimento AdvPL/TLPP. As três ferramentas principais trabalham em conjunto:

1. **AdvEditor** - Editor de Banco de Dados (inspirado em APSDU)
2. **AdvCfg** - Configurador de Tabelas (inspirado em SIGACFG)
3. **advpp-ide** - IDE Principal para desenvolvimento

## Fluxo de Trabalho Integrado

### 1. Dicionário de Dados (AdvCfg)

**Inicialização:**
- AdvCfg abre automaticamente `./data/advpl_dictionary.db`
- Se o banco não existir, é criado automaticamente
- Se o banco existir mas estiver vazio, é populado com dados iniciais
- Tabelas do dicionário: SX2, SX3, SIX, SX7, SX5, SX6, SXB

**Funcionalidades:**
- Gerenciamento de tabelas do dicionário
- Definição de estrutura de tabelas
- Configuração de índices
- Definição de triggers
- Gerenciamento de genéricas
- Configuração de parâmetros
- Definição de perguntas

**Troca de Dicionário:**
- Menu Arquivo → Trocar Dicionário
- Permite selecionar outro arquivo de dicionário
- Recarrega automaticamente os dados

### 2. Editor de Banco de Dados (AdvEditor)

**Fluxo de Abertura:**
1. Menu Arquivo → Abrir (Ctrl+B)
2. **Seleção de Driver:**
   - SQLite (padrão)
   - DBF
   - TopConnect
   - Ctree
   - BTrieve
3. **Opções de Abertura:**
   - Compartilhado (padrão: true)
   - Somente leitura (padrão: false)
4. **Seleção de Arquivo:**
   - Filtrado automaticamente baseado no driver selecionado
   - SQLite: .db, .sqlite, .sqlite3
   - DBF: .dbf
   - Outros: conforme driver
5. **Seleção de Tabela (SQLite):**
   - Lista todas as tabelas do banco
   - Permite selecionar tabela específica
   - Carrega estrutura e dados

**Inicialização:**
- AdvEditor abre automaticamente `./data/advpl_dictionary.db` ao iniciar
- Lista todas as tabelas disponíveis no banco
- Permite navegar entre tabelas

**Troca de Banco de Dados:**
- Menu Arquivo → Trocar Banco de Dados
- Permite selecionar outro banco de dados
- Recarrega automaticamente as tabelas

### 3. IDE Principal (advpp-ide)

**Integração com AdvCfg:**
- Acesso ao dicionário de dados para autocompletar
- Validação de código baseada no dicionário
- Geração de código a partir do dicionário
- Navegação para definições no dicionário

**Integração com AdvEditor:**
- Acesso direto a tabelas do banco de dados
- Visualização de dados em tempo real
- Debug de queries SQL
- Teste de procedures e funções

## Banco de Dados Padrão

**Localização:** `./data/advpl_dictionary.db`

**Criação Automática:**
- Criado na primeira execução do AdvCfg
- Populado com dados iniciais se vazio
- Utilizado por ambas as ferramentas como padrão

**Tabelas do Dicionário:**
- **SX2** - Tabelas (metadados de tabelas)
- **SX3** - Campos (estrutura de campos)
- **SIX** - Índices (definição de índices)
- **SX7** - Triggers (gatilhos de banco)
- **SX5** - Genéricas (tabelas genéricas)
- **SX6** - Parâmetros (parâmetros do sistema)
- **SXB** - Perguntas (perguntas do help)

**Tabelas de Negócio Pré-Configuradas:**
- SA1 - Clientes
- SA2 - Fornecedores
- SE1 - Contas a Receber
- SE2 - Contas a Pagar
- SB1 - Produtos
- SD1 - Vendas
- SF1 - Notas Fiscais
- SF2 - Itens da Nota Fiscal
- ZZ1 - Usuários
- ZZ2 - Grupos de Acesso

## Comunicação Entre Ferramentas

### Compartilhamento de Banco de Dados

**Arquivo Único:**
- Todas as ferramentas usam o mesmo arquivo `./data/advpl_dictionary.db`
- SQLite permite acesso concorrente
- Lock automático para evitar conflitos

**Sincronização:**
- AdvCfg é a fonte de verdade para o dicionário
- AdvEditor lê do dicionário para estruturas
- advpp-ide usa o dicionário para validação

### Integração via API

**AdvCfg → AdvEditor:**
- AdvCfg expõe API para consulta do dicionário
- AdvEditor usa API para validar estruturas
- Sincronização automática de mudanças

**AdvCfg → advpp-ide:**
- API para autocompletar
- Validação de tipos de campos
- Geração de código baseada em templates

**AdvEditor → advpp-ide:**
- API para execução de queries
- Visualização de dados
- Debug de procedures

## Fluxo de Desenvolvimento Típico

### 1. Definição de Estrutura (AdvCfg)
```
1. Abrir AdvCfg
2. Criar/Editar tabela no dicionário
3. Definir campos
4. Configurar índices
5. Salvar no dicionário
```

### 2. Desenvolvimento de Código (advpp-ide)
```
1. Abrir advpp-ide
2. Criar novo arquivo AdvPL/TLPP
3. Usar autocompletar baseado no dicionário
4. Validar código com base no dicionário
5. Compilar
```

### 3. Teste de Dados (AdvEditor)
```
1. Abrir AdvEditor
2. Conectar ao banco de dados
3. Selecionar tabela
4. Visualizar dados
5. Testar queries
```

## Configuração

### Caminhos Padrão

```
./data/advpl_dictionary.db  - Dicionário de dados
./data/*.db                 - Bancos de dados SQLite
./data/*.dbf                - Arquivos DBF
```

### Variáveis de Ambiente

```bash
ADVP_DICTIONARY_PATH=./data/advpl_dictionary.db
ADVP_DATA_DIR=./data
ADVP_DEFAULT_DRIVER=SQLite
```

## Segurança

### Controle de Acesso

- AdvCfg requer autenticação (futuro)
- Permissões por usuário (futuro)
- Log de alterações (futuro)

### Backup Automático

- Backup automático do dicionário antes de alterações
- Backup de bancos de dados antes de operações destrutivas
- Histórico de versões (futuro)

## Performance

### Otimizações

- Cache do dicionário em memória
- Índices automáticos no SQLite
- Paginação de dados no AdvEditor
- Lazy loading de estruturas

### Concorrência

- SQLite permite múltiplos leitores
- Lock granular para escrita
- Timeout configurável

## Extensibilidade

### Plugins Futuros

- Integração com outros bancos (PostgreSQL, MySQL)
- Drivers personalizados
- Customização de UI
- Integração com ferramentas externas

### API Pública

```go
// API do Dicionário
type DictionaryAPI interface {
    GetTables() ([]Table, error)
    GetFields(table string) ([]Field, error)
    GetIndexes(table string) ([]Index, error)
    AddTable(table Table) error
    UpdateTable(table Table) error
    DeleteTable(table string) error
}

// API do Editor
type EditorAPI interface {
    OpenDatabase(driver, path string) error
    CloseDatabase() error
    ListTables() ([]string, error)
    Query(sql string) ([]Record, error)
}
```

## Troubleshooting

### Problemas Comuns

**Banco de dados não encontrado:**
- Verifique se `./data/advpl_dictionary.db` existe
- Verifique permissões de escrita no diretório

**Conflito de acesso:**
- SQLite permite múltiplos leitores
- Escrita requer lock exclusivo
- Aguarde liberação do lock

**Estrutura não sincronizada:**
- Recarregue o dicionário no AdvCfg
- Reinicie o AdvEditor
- Limpe o cache do advpp-ide

## Roadmap

### Curto Prazo
- ✅ Auto-abertura do banco padrão
- ✅ Seleção de driver antes do arquivo
- ✅ Seleção de tabela após conexão
- ⏳ Integração real com advpp-ide
- ⏳ Autocompletar baseado no dicionário

### Médio Prazo
- ⏳ Autenticação no AdvCfg
- ⏳ Log de alterações
- ⏳ Backup automático
- ⏳ Validação de código

### Longo Prazo
- ⏳ Suporte a outros bancos
- ⏳ Plugins personalizados
- ⏳ Integração com ferramentas externas
- ⏳ Interface web

## Conclusão

A integração entre AdvEditor, AdvCfg e advpp-ide é fundamental para proporcionar uma experiência completa de desenvolvimento AdvPL/TLPP. O banco de dados SQLite padrão serve como ponto central de integração, permitindo que todas as ferramentas compartilhem informações de forma consistente e eficiente.
