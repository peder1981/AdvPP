# Estrutura do Dicionário de Dados AdvPL

## Visão Geral

O AdvCfg utiliza um banco de dados SQLite para armazenar o dicionário de dados AdvPL, seguindo o padrão TOTVS Protheus. O dicionário é criado automaticamente na primeira execução e contém tabelas e dados pré-configurados.

## Localização

O dicionário é armazenado em:
```
./data/advpl_dictionary.db
```

## Tabelas do Dicionário

### SX2 - Tabelas

Armazena informações sobre as tabelas do sistema.

**Campos Principais:**
- `X2_CHAVE` - Chave primária (ex: "SA1")
- `X2_ALIAS` - Alias da tabela (ex: "SA1")
- `X2_NOME` - Nome da tabela (ex: "SA1")
- `X2_NOMEUSR` - Nome do usuário (ex: "Clientes")
- `X2_MODULO` - Módulo (ex: "SIGAFAT")
- `X2_TIPO` - Tipo (C=Caracter, N=Numérico, etc.)
- `X2_DESCRIC` - Descrição da tabela

**Tabelas Pré-Configuradas:**
- **SA1** - Clientes (SIGAFAT)
- **SA2** - Fornecedores (SIGACOM)
- **SE1** - Contas a Receber (SIGAFIN)
- **SE2** - Contas a Pagar (SIGAFIN)
- **SB1** - Produtos (SIGAEST)
- **SD1** - Vendas (SIGAFAT)
- **SF1** - Notas Fiscais (SIGAFAT)
- **SF2** - Itens da Nota Fiscal (SIGAFAT)
- **ZZ1** - Usuários (SIGACFG)
- **ZZ2** - Grupos de Acesso (SIGACFG)

### SX3 - Campos

Armazena informações sobre os campos das tabelas.

**Campos Principais:**
- `X3_ARQUIVO` - Nome da tabela (ex: "SA1")
- `X3_ORDEM` - Ordem do campo
- `X3_CAMPO` - Nome do campo (ex: "A1_COD")
- `X3_TIPO` - Tipo do campo (C, N, D, L, M)
- `X3_TAMANHO` - Tamanho do campo
- `X3_DECIMAL` - Decimais
- `X3_TITULO` - Título do campo
- `X3_DESCRIC` - Descrição do campo
- `X3_PICTURE` - Picture do campo
- `X3_VALID` - Validação do campo

**Campos Pré-Configurados para SA1:**
- `A1_COD` - Código do Cliente (C, 6)
- `A1_LOJA` - Loja do Cliente (C, 2)
- `A1_NOME` - Nome do Cliente (C, 40)
- `A1_NREDUZ` - Nome Reduzido (C, 15)
- `A1_TIPO` - Tipo de Cliente (C, 1)
- `A1_CGC` - CGC/CPF (C, 14)
- `A1_END` - Endereço (C, 40)
- `A1_BAIRRO` - Bairro (C, 30)
- `A1_MUN` - Município (C, 40)
- `A1_EST` - Estado (C, 2)
- `A1_CEP` - CEP (C, 8)
- `A1_TEL` - Telefone (C, 15)
- `A1_EMAIL` - E-mail (C, 60)
- `A1_MSBLQ` - Bloqueado (L, 1)
- `A1_VEND` - Vendedor (C, 6)

### SIX - Índices

Armazena informações sobre os índices das tabelas.

**Campos Principais:**
- `IX_ARQUIVO` - Nome da tabela (ex: "SA1")
- `IX_INDICE` - Número do índice
- `IX_ORDEM` - Ordem no índice
- `IX_CHAVE` - Expressão do índice (ex: "A1_FILIAL+A1_COD+A1_LOJA")
- `IX_DESCRIC` - Descrição do índice
- `IX_TIPO` - Tipo do índice
- `IX_FILTRO` - Filtro do índice

**Índices Pré-Configurados para SA1:**
1. `A1_FILIAL+A1_COD+A1_LOJA` - Chave Primária
2. `A1_NOME` - Nome do Cliente
3. `A1_CGC` - CGC/CPF do Cliente
4. `A1_VEND` - Vendedor do Cliente

### SX7 - Triggers

Armazena informações sobre triggers das tabelas.

**Campos Principais:**
- `X7_ARQUIVO` - Nome da tabela
- `X7_CAMPO` - Nome do campo
- `X7_SEQUENC` - Sequência
- `X7_EVENTO` - Evento (BeforeInsert, AfterInsert, etc.)
- `X7_ROTINA` - Rotina do trigger
- `X7_CONDIC` - Condição do trigger

### SX5 - Genéricas

Armazena tabelas genéricas (tabelas de domínio).

**Campos Principais:**
- `X5_TABELA` - Nome da tabela genérica (ex: "X3_TIPO")
- `X5_CHAVE` - Chave (ex: "C")
- `X5_DESCRIC` - Descrição (ex: "Caracter")
- `X5_TIPO` - Tipo do valor
- `X5_TAMANHO` - Tamanho
- `X5_DECIMAL` - Decimais

**Genéricas Pré-Configuradas:**
- **X3_TIPO** - Tipos de campos (C, N, D, L, M)
- **A1_TIPO** - Tipos de cliente (F-Física, J-Jurídica)

### SX6 - Parâmetros

Armazena parâmetros do sistema.

**Campos Principais:**
- `X6_VAR` - Nome do parâmetro
- `X6_TIPO` - Tipo do parâmetro
- `X6_DESCRIC` - Descrição do parâmetro
- `X6_TAMANHO` - Tamanho
- `X6_DECIMAL` - Decimais
- `X6_PRESEL` - Valor pré-selecionado

### SXB - Perguntas

Armazera perguntas de consultas (SX1).

**Campos Principais:**
- `XB_ALIAS` - Alias da pergunta
- `XB_TIPO` - Tipo da pergunta
- `XB_DESCRIC` - Descrição da pergunta
- `XB_TAMANHO` - Tamanho
- `XB_DECIMAL` - Decimais

## API do Dicionário

### Criar Dicionário

```go
dict, err := shared.NewDictionary("./data/advpl_dictionary.db")
if err != nil {
    // Tratar erro
}
defer dict.Close()
```

### Obter Tabelas

```go
tables, err := dict.GetTables()
for _, table := range tables {
    chave := table["X2_CHAVE"].(string)
    alias := table["X2_ALIAS"].(string)
    nome := table["X2_NOMEUSR"].(string)
}
```

### Obter Campos de uma Tabela

```go
fields, err := dict.GetFields("SA1")
for _, field := range fields {
    campo := field["X3_CAMPO"].(string)
    tipo := field["X3_TIPO"].(string)
    titulo := field["X3_TITULO"].(string)
}
```

### Obter Índices de uma Tabela

```go
indexes, err := dict.GetIndexes("SA1")
for _, idx := range indexes {
    chave := idx["IX_CHAVE"].(string)
    descric := idx["IX_DESCRIC"].(string)
}
```

### Obter Genéricas

```go
genericas, err := dict.GetGenericas("X3_TIPO")
for _, gen := range genericas {
    chave := gen["X5_CHAVE"].(string)
    descr := gen["X5_DESCRIC"].(string)
}
```

### Adicionar Tabela

```go
err := dict.AddTable("SB2", "SB2", "SB2", "Produtos", "SIGAEST", "C", "Cadastro de Produtos")
```

### Adicionar Campo

```go
err := dict.AddField("SB2", 1, "B2_COD", "C", 6, 0, "Código", "Código do Produto")
```

### Adicionar Índice

```go
err := dict.AddIndex("SB2", 1, 1, "B2_FILIAL+B2_COD", "Chave Primária")
```

## Regras de Validação

O dicionário implementa as seguintes regras de validação:

1. **SX2**
   - X2_CHAVE deve ser único
   - X2_ALIAS deve ser único
   - X2_TIPO deve ser C, N, D, L ou M

2. **SX3**
   - X3_ARQUIVO + X3_ORDEM deve ser único
   - X3_TIPO deve ser C, N, D, L ou M
   - X3_TAMANHO deve ser positivo
   - X3_DECIMAL não pode exceder X3_TAMANHO

3. **SIX**
   - IX_ARQUIVO + IX_INDICE + IX_ORDEM deve ser único
   - IX_INDICE deve ser positivo
   - IX_ORDEM deve ser positivo

4. **SX5**
   - X5_TABELA + X5_CHAVE deve ser único
   - X5_CHAVE não pode ser vazio

5. **SX6**
   - X6_VAR deve ser único
   - X6_VAR não pode ser vazio

6. **SXB**
   - XB_ALIAS deve ser único
   - XB_ALIAS não pode ser vazio

## Extensão do Dicionário

Para adicionar novas tabelas ao dicionário:

1. **Adicionar a tabela em SX2**
2. **Adicionar os campos em SX3**
3. **Adicionar os índices em SIX**
4. **Adicionar genéricas relevantes em SX5**
5. **Adicionar triggers em SX7** (se necessário)

## Integração com AdvCfg

O AdvCfg carrega automaticamente o dicionário ao iniciar:

```go
// Carrega dicionário
dict, err := shared.NewDictionary("./data/advpl_dictionary.db")
if err != nil {
    dialog.ShowError(err, w)
    return nil
}
ac.dictionary = dict

// Carrega dados na tree view
ac.loadDictionaryData()
```

## Backup e Restauração

### Backup

```bash
cp ./data/advpl_dictionary.db ./data/advpl_dictionary_backup.db
```

### Restauração

```bash
cp ./data/advpl_dictionary_backup.db ./data/advpl_dictionary.db
```

## Migrar Dicionário Protheus

Para migrar um dicionário do Protheus:

1. Exportar tabelas SX2, SX3, SIX, SX7, SX5, SX6, SXB do Protheus
2. Converter para formato SQLite
3. Importar no banco de dados do AdvCfg

## Performance

O dicionário SQLite oferece:
- Acesso rápido aos dados
- Transações ACID
- Índices automáticos
- Compactação automática
- Backup incremental

## Segurança

- O arquivo do dicionário deve ter permissões restritas
- Considerar criptografia para dados sensíveis
- Implementar controle de acesso no AdvCfg
- Logs de alterações do dicionário

## Limitações Atuais

1. Triggers (SX7) não implementados completamente
2. Parâmetros (SX6) não implementados completamente
3. Perguntas (SXB) não implementados completamente
4. Validação de integridade referencial não implementada
5. Geração de código a partir do dicionário não implementada

## Próximos Passos

1. Implementar validação completa do dicionário
2. Implementar gerador de código AdvPL
3. Implementar importação/exportação de dicionário
4. Implementar sincronização com Protheus
5. Implementar versionamento do dicionário
6. Implementar controle de alterações (audit trail)
