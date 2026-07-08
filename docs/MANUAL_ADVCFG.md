# Manual do Usuário - AdvCfg (Configurador de Tabelas)

## Introdução

O AdvCfg é o configurador de tabelas do AdvPP, inspirado no SIGACFG do TOTVS Protheus. Ele permite gerenciar o dicionário de dados, definir estruturas de tabelas, configurar índices, triggers e outros metadados essenciais para o desenvolvimento AdvPL/TLPP.

## Requisitos do Sistema

- **Sistema Operacional**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+)
- **Memória RAM**: Mínimo 2GB, recomendado 4GB
- **Espaço em Disco**: 100MB para instalação
- **Processador**: Arquitetura x86_64 (amd64)

## Instalação

### Via Pacote Debian/Ubuntu

```bash
# Baixar o pacote
wget https://github.com/peder1981/AdvPP/releases/download/v1.0.0/advpp_1.0.0_amd64.deb

# Instalar
sudo dpkg -i advpp_1.0.0_amd64.deb

# Resolver dependências se necessário
sudo apt-get install -f
```

### Via Compilação

```bash
# Clonar repositório
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP

# Compilar
go build -o advcfg ./cmd/advcfg

# Instalar
sudo cp advcfg /usr/local/bin/
```

## Primeiros Passos

### Inicialização

Para iniciar o AdvCfg:

```bash
advcfg
```

Ou através do menu de aplicações do seu sistema.

### Interface Principal

A interface do AdvCfg é composta por:

- **Barra de Menu**: Acesso a todas as funcionalidades
- **Tree View**: Navegação hierárquica do dicionário
- **Data Grid**: Visualização e edição de dados
- **Barra de Status**: Informações sobre o estado atual
- **Painel de Propriedades**: Detalhes do item selecionado

## Funcionalidades

### Dicionário de Dados

#### Estrutura do Dicionário

O dicionário de dados é organizado nas seguintes tabelas principais:

- **SX2**: Metadados de tabelas
- **SX3**: Estrutura de campos
- **SIX**: Definição de índices
- **SX7**: Triggers de banco
- **SX5**: Tabelas genéricas
- **SX6**: Parâmetros do sistema
- **SXB**: Perguntas do help

#### Banco de Dados Padrão

Por padrão, o AdvCfg utiliza o banco de dados `~/.advpp/ADVPP.db`. Este banco é criado automaticamente na primeira execução e populado com dados iniciais.

### Gerenciamento de Tabelas (SX2)

#### Criar Nova Tabela

1. Selecione **Tabelas (SX2)** na Tree View
2. Clique com o botão direito → **Nova Tabela**
3. Preencha os campos:
   - **X2_CHAVE**: Chave única da tabela
   - **X2_ALIAS**: Alias da tabela
   - **X2_NOME**: Nome completo da tabela
   - **X2_NOMEUSR**: Nome para exibição
   - **X2_MODULO**: Módulo do sistema
   - **X2_TIPO**: Tipo (C=Comum, V=Virtual, etc.)
   - **X2_DESCRIC**: Descrição da tabela
4. Clique em **Salvar**

#### Editar Tabela Existente

1. Selecione a tabela na Tree View
2. Os dados aparecem no Data Grid
3. Edite os campos diretamente no grid
4. Clique em **Salvar** para persistir as alterações

#### Excluir Tabela

1. Selecione a tabela na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Campos (SX3)

#### Adicionar Campo à Tabela

1. Selecione a tabela desejada em **Tabelas (SX2)**
2. Clique com o botão direito → **Adicionar Campo**
3. Preencha os campos:
   - **X3_ARQUIVO**: Nome da tabela
   - **X3_ORDEM**: Ordem do campo
   - **X3_CAMPO**: Nome do campo
   - **X3_TIPO**: Tipo de dado (C=Caracter, N=Numérico, D=Data, L=Lógico)
   - **X3_TAMANHO**: Tamanho do campo
   - **X3_DECIMAL**: Decimais (para numéricos)
   - **X3_TITULO**: Título para exibição
   - **X3_DESCRIC**: Descrição do campo
   - **X3_PICTURE**: Máscara de formatação
   - **X3_VALID**: Validação
   - **X3_USADO**: Campo em uso
   - **X3_RESERV**: Campo reservado
4. Clique em **Salvar**

#### Tipos de Campos Suportados

| Tipo | Descrição | Exemplo |
|------|-----------|---------|
| C | Caracter | "Nome do cliente" |
| N | Numérico | 1234.56 |
| D | Data | 01/01/2024 |
| L | Lógico | .T. ou .F. |
| M | Memo | Texto longo |
| B | Blob | Dados binários |

#### Editar Campo

1. Selecione **Campos (SX3)** na Tree View
2. Filtre pela tabela desejada
3. Edite os campos no Data Grid
4. Clique em **Salvar**

#### Excluir Campo

1. Selecione o campo na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Índices (SIX)

#### Criar Índice

1. Selecione a tabela desejada em **Tabelas (SX2)**
2. Clique com o botão direito → **Novo Índice**
3. Preencha os campos:
   - **INDICE**: Nome do índice
   - **ORDEM**: Ordem do índice
   - **CHAVE**: Campos que compõem o índice
   - **DESCRIC**: Descrição do índice
   - **TIPO**: Tipo de índice (U=Único, R=Regular)
   - **FILIAL**: Índice por filial
4. Clique em **Salvar**

#### Exemplo de Índice

```
INDICE: A1_FILIAL
ORDEM: 1
CHAVE: A1_FILIAL+A1_COD+A1_LOJA
DESCRIC: Índice por filial, código e loja
TIPO: U
FILIAL: .T.
```

#### Editar Índice

1. Selecione **Índices (SIX)** na Tree View
2. Filtre pela tabela desejada
3. Edite os campos no Data Grid
4. Clique em **Salvar**

#### Excluir Índice

1. Selecione o índice na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Triggers (SX7)

#### Criar Trigger

1. Selecione a tabela desejada em **Tabelas (SX2)**
2. Clique com o botão direito → **Novo Trigger**
3. Preencha os campos:
   - **X7_CAMPO**: Campo do trigger
   - **X7_SEQUENCIA**: Sequência do trigger
   - **X7_TRIGGER**: Código do trigger
   - **X7_CONDICAO**: Condição de execução
   - **X7_SENARIO**: Cenário (antes/depois)
4. Clique em **Salvar**

#### Tipos de Triggers

- **Before**: Executado antes da operação
- **After**: Executado após a operação
- **Instead**: Executado em vez da operação

#### Editar Trigger

1. Selecione **Triggers (SX7)** na Tree View
2. Filtre pela tabela desejada
3. Edite os campos no Data Grid
4. Clique em **Salvar**

#### Excluir Trigger

1. Selecione o trigger na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Genéricas (SX5)

#### Criar Genérica

1. Selecione **Genéricas (SX5)** na Tree View
2. Clique com o botão direito → **Nova Genérica**
3. Preencha os campos:
   - **X5_TABELA**: Tabela genérica
   - **X5_CHAVE**: Chave do registro
   - **X5_DESCRI**: Descrição
   - **X5_SPANISH**: Descrição em espanhol
   - **X5_ENGLISH**: Descrição em inglês
4. Clique em **Salvar**

#### Genéricas Comuns

- **01**: Sim/Não
- **02**: Ativo/Inativo
- **03**: Masculino/Feminino
- **04**: Jan/Fev/Mar... (meses)

#### Editar Genérica

1. Selecione a genérica na Tree View
2. Edite os campos no Data Grid
3. Clique em **Salvar**

#### Excluir Genérica

1. Selecione a genérica na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Parâmetros (SX6)

#### Criar Parâmetro

1. Selecione **Parâmetros (SX6)** na Tree View
2. Clique com o botão direito → **Novo Parâmetro**
3. Preencha os campos:
   - **X6_VAR**: Nome da variável (MV_XXX)
   - **X6_TIPO**: Tipo de dado
   - **X6_DESCRIC**: Descrição
   - **X6_CONTEUD**: Conteúdo padrão
   - **X6_DSCSPA**: Descrição em espanhol
   - **X6_DSCENG**: Descrição em inglês
4. Clique em **Salvar**

#### Parâmetros Comuns

- **MV_XFILIAL**: Filial atual
- **MV_XUSUARIO**: Usuário atual
- **MV_XDATA**: Data atual
- **MV_XHORA**: Hora atual

#### Editar Parâmetro

1. Selecione o parâmetro na Tree View
2. Edite os campos no Data Grid
3. Clique em **Salvar**

#### Excluir Parâmetro

1. Selecione o parâmetro na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

### Gerenciamento de Perguntas (SXB)

#### Criar Pergunta

1. Selecione **Perguntas (SXB)** na Tree View
2. Clique com o botão direito → **Nova Pergunta**
3. Preencha os campos:
   - **XB_ALIAS**: Alias da pergunta
   - **XB_DESCRI**: Descrição
   - **XB_TIPO**: Tipo de resposta
   - **XB_HELP**: Texto de ajuda
4. Clique em **Salvar**

#### Editar Pergunta

1. Selecione a pergunta na Tree View
2. Edite os campos no Data Grid
3. Clique em **Salvar**

#### Excluir Pergunta

1. Selecione a pergunta na Tree View
2. Clique com o botão direito → **Excluir**
3. Confirme a exclusão

## Operações com Dicionário

### Trocar Dicionário

1. Menu **Arquivo** → **Trocar Dicionário**
2. Navegue até o arquivo do dicionário desejado
3. Selecione o arquivo
4. O dicionário é recarregado automaticamente

### Exportar Dicionário

1. Menu **Arquivo** → **Exportar**
2. Selecione o formato (JSON, XML, CSV)
3. Escolha o local de salvamento
4. Clique em **Exportar**

### Importar Dicionário

1. Menu **Arquivo** → **Importar**
2. Selecione o arquivo a importar
3. Escolha as opções de importação
4. Clique em **Importar**

### Backup do Dicionário

1. Menu **Arquivo** → **Backup**
2. Escolha o local de salvamento
3. O backup é criado automaticamente

### Restaurar Backup

1. Menu **Arquivo** → **Restaurar**
2. Selecione o arquivo de backup
3. Confirme a restauração

## Configurações

### Configurações de Conexão

Acesse **Ferramentas** → **Configurações** → **Conexão**:

- **Database Path**: Caminho do banco de dados
- **Auto Connect**: Conectar automaticamente
- **Backup Frequency**: Frequência de backup

### Configurações de Interface

Acesse **Ferramentas** → **Configurações** → **Interface**:

- **Theme**: Tema da interface
- **Font Size**: Tamanho da fonte
- **Grid Lines**: Mostrar linhas do grid
- **Row Numbers**: Mostrar números de linha

### Configurações de Validação

Acesse **Ferramentas** → **Configurações** → **Validação**:

- **Auto Validate**: Validar automaticamente
- **Strict Mode**: Modo estrito de validação
- **Custom Rules**: Regras personalizadas

## Dicas e Truques

### Produtividade

- **Filtragem Rápida**: Use Ctrl+F para filtrar dados
- **Navegação**: Use as setas do teclado para navegar
- **Edição Rápida**: Dê duplo clique para editar
- **Atalhos**: Use atalhos para operações frequentes

### Boas Práticas

- **Nomenclatura**: Use nomes consistentes e descritivos
- **Documentação**: Documente tabelas e campos
- **Índices**: Crie índices apenas quando necessário
- **Triggers**: Use triggers com moderação
- **Backup**: Faça backup regularmente

### Validação

- **Integridade**: Valide integridade referencial
- **Tipos**: Verifique tipos de dados
- **Tamanhos**: Respeite tamanhos de campos
- **Índices**: Valide chaves duplicadas

## Solução de Problemas

### Erros Comuns

**Erro ao abrir dicionário:**
- Verifique se o arquivo existe
- Confirme permissões de acesso
- Valide formato do arquivo

**Erro ao salvar:**
- Verifique espaço em disco
- Confirme permissões de escrita
- Valide integridade dos dados

**Erro ao criar tabela:**
- Verifique se a chave já existe
- Confirme campos obrigatórios
- Valide tipos de dados

### Log de Erros

O log de erros está disponível em:

- **Linux**: `~/.advpp/logs/advcfg.log`
- **Conteúdo**: Erros, warnings, informações de debug

### Recuperação de Dados

Em caso de corrupção do dicionário:

1. Menu **Arquivo** → **Restaurar**
2. Selecione o backup mais recente
3. Confirme a restauração
4. Valide os dados restaurados

## Integração com Outras Ferramentas

### AdvEditor

O AdvCfg se integra com o AdvEditor:

- Dicionário compartilhado
- Estruturas sincronizadas
- Validação consistente

### AdvPP IDE

O AdvCfg se integra com o AdvPP IDE:

- Autocompletar baseado no dicionário
- Validação de código
- Geração de código

## Atalhos de Teclado

| Comando | Atalho |
|---------|--------|
| Novo Item | Ctrl+N |
| Salvar | Ctrl+S |
| Excluir | Delete |
| Localizar | Ctrl+F |
| Substituir | Ctrl+H |
| Recarregar | F5 |
| Backup | Ctrl+B |
| Importar | Ctrl+I |
| Exportar | Ctrl+E |

## Exemplos Práticos

### Exemplo 1: Criar Tabela de Clientes

1. Crie a tabela SA1 em SX2
2. Adicione campos em SX3:
   - A1_FILIAL (C, 2)
   - A1_COD (C, 6)
   - A1_LOJA (C, 2)
   - A1_NOME (C, 40)
   - A1_END (C, 40)
   - A1_BAIRRO (C, 20)
   - A1_MUN (C, 20)
   - A1_EST (C, 2)
   - A1_CEP (C, 8)
3. Crie índices em SIX:
   - A1_FILIAL (Único)
   - A1_COD (Regular)
4. Salve as alterações

### Exemplo 2: Configurar Trigger de Validação

1. Selecione a tabela SA1
2. Crie trigger em SX7:
   - Campo: A1_COD
   - Sequência: 1
   - Trigger: Validação de código
   - Condição: A1_COD != ""
3. Salve as alterações

## Conclusão

O AdvCfg é uma ferramenta essencial para gerenciamento do dicionário de dados AdvPL/TLPP. Com este manual, você deve ser capaz de criar e manter estruturas de tabelas, campos, índices e outros metadados necessários para o desenvolvimento.

Para mais informações, visite a documentação oficial em https://github.com/peder1981/AdvPP.
