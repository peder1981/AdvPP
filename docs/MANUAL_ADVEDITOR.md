# Manual do Usuário - AdvEditor (Editor de Banco de Dados)

## Introdução

O AdvEditor é um editor de banco de dados completo para o AdvPP, inspirado no APSDU do TOTVS Protheus. Ele permite visualizar, editar e manipular dados de bancos de dados SQLite, DBF e outros formatos suportados, com uma interface moderna e intuitiva.

## Requisitos do Sistema

- **Sistema Operacional**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+)
- **Memória RAM**: Mínimo 2GB, recomendado 4GB
- **Espaço em Disco**: 100MB para instalação
- **Processador**: Arquitetura x86_64 (amd64)

## Instalação

### Via Pacote Debian/Ubuntu

```bash
# Baixar o pacote
# Baixe o .deb mais recente em https://github.com/peder1981/AdvPP/releases
curl -sL https://api.github.com/repos/peder1981/AdvPP/releases/latest \
  | grep browser_download_url | grep amd64.deb | cut -d'"' -f4 | xargs wget

# Instalar
sudo dpkg -i advpp_*_amd64.deb

# Resolver dependências se necessário
sudo apt-get install -f
```

### Via Compilação

```bash
# Clonar repositório
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP

# Compilar
go build -o adveditor ./cmd/adveditor

# Instalar
sudo cp adveditor /usr/local/bin/
```

## Primeiros Passos

### Inicialização

Para iniciar o AdvEditor:

```bash
adveditor
```

Ou através do menu de aplicações do seu sistema.

### Interface Principal

A interface do AdvEditor é composta por:

- **Barra de Menu**: Acesso a todas as funcionalidades
- **Barra de Ferramentas**: Acesso rápido a comandos frequentes
- **Painel de Tabelas**: Lista de tabelas do banco
- **Grid de Dados**: Visualização e edição de registros
- **Painel de Filtros**: Filtros para consulta
- **Barra de Status**: Informações sobre o estado atual

## Funcionalidades

### Conexão com Banco de Dados

#### Abrir Banco de Dados

1. Menu **Arquivo** → **Abrir** (Ctrl+B)
2. **Seleção de Driver**:
   - **SQLite**: Banco de dados embutido (padrão)
   - **DBF**: Arquivos DBF
   - **TopConnect**: Conexão via TopConnect
   - **Ctree**: Banco Ctree
   - **BTrieve**: Banco BTrieve
3. **Opções de Abertura**:
   - **Compartilhado**: Permite acesso concorrente (padrão: true)
   - **Somente Leitura**: Modo apenas leitura (padrão: false)
4. **Seleção de Arquivo**:
   - Navegue até o arquivo desejado
   - O filtro é aplicado automaticamente baseado no driver
5. **Seleção de Tabela** (SQLite):
   - Lista todas as tabelas do banco
   - Selecione a tabela desejada
   - Clique em **OK**

#### Banco de Dados Padrão

O AdvEditor abre automaticamente o banco de dados padrão `~/.advpp/ADVPP.db` ao iniciar, se ele existir.

#### Fechar Banco de Dados

1. Menu **Arquivo** → **Fechar**
2. O banco de dados é fechado
3. As alterações pendentes são salvas automaticamente

### Visualização de Dados

#### Navegação por Tabelas

1. As tabelas disponíveis aparecem no painel esquerdo
2. Clique em uma tabela para carregar os dados
3. Os dados são exibidos no grid principal
4. Use a barra de rolagem para navegar pelos registros

#### Grid de Dados

O grid de dados permite:

- **Visualização**: Ver todos os registros da tabela
- **Edição**: Editar registros diretamente no grid
- **Ordenação**: Clique no cabeçalho para ordenar
- **Filtragem**: Use filtros para refinar resultados
- **Seleção**: Selecione múltiplos registros

#### Paginação

Para tabelas grandes, use a paginação:

- **Primeira Página**: Botão "|<"
- **Página Anterior**: Botão "<"
- **Próxima Página**: Botão ">"
- **Última Página**: Botão ">|"
- **Ir para**: Digite o número da página

### Edição de Dados

#### Adicionar Novo Registro

1. Clique no botão **Novo** na barra de ferramentas
2. Uma linha em branco aparece no grid
3. Preencha os campos desejados
4. Clique em **Salvar** para persistir

#### Editar Registro Existente

1. Clique na célula que deseja editar
2. Digite o novo valor
3. Pressione Enter ou clique fora da célula
4. As alterações são salvas automaticamente

#### Excluir Registro

1. Selecione o registro desejado
2. Clique no botão **Excluir** na barra de ferramentas
3. Confirme a exclusão

#### Desfazer Alterações

1. Clique no botão **Desfazer** na barra de ferramentas
2. As alterações não salvas são revertidas

### Consultas e Filtros

#### Filtro Simples

1. Use o painel de filtros
2. Selecione o campo
3. Digite o valor desejado
4. Clique em **Filtrar**

#### Filtro Avançado

1. Menu **Consulta** → **Filtro Avançado**
2. Construa sua consulta usando:
   - Operadores: =, !=, >, <, >=, <=, LIKE, IN
   - Conectores: AND, OR
   - Parênteses para agrupamento
3. Clique em **Executar**

#### Exemplo de Filtro Avançado

```
A1_FILIAL = '01' AND A1_COD LIKE 'A%' AND A1_NOME != ''
```

#### SQL Personalizado

1. Menu **Consulta** → **SQL Personalizado**
2. Digite sua query SQL
3. Clique em **Executar**
4. Os resultados aparecem no grid

#### Exemplo de SQL

```sql
SELECT A1_COD, A1_NOME, A1_MUN 
FROM SA1 
WHERE A1_FILIAL = '01' 
ORDER BY A1_NOME
```

### Operações em Lote

#### Atualização em Lote

1. Selecione os registros desejados
2. Menu **Operações** → **Atualizar em Lote**
3. Selecione o campo
4. Digite o novo valor
5. Clique em **Aplicar**

#### Exclusão em Lote

1. Selecione os registros desejados
2. Menu **Operações** → **Excluir em Lote**
3. Confirme a exclusão

#### Copiar/Colar em Lote

1. Selecione os registros desejados
2. Menu **Operações** → **Copiar**
3. Navegue para a tabela de destino
4. Menu **Operações** → **Colar**

### Importação/Exportação

#### Importar Dados

1. Menu **Arquivo** → **Importar**
2. Selecione o formato:
   - **CSV**: Valores separados por vírgula
   - **JSON**: Formato JSON
   - **XML**: Formato XML
   - **DBF**: Arquivo DBF
3. Selecione o arquivo
4. Configure as opções de importação
5. Clique em **Importar**

#### Exportar Dados

1. Selecione os registros desejados
2. Menu **Arquivo** → **Exportar**
3. Selecione o formato:
   - **CSV**: Valores separados por vírgula
   - **JSON**: Formato JSON
   - **XML**: Formato XML
   - **Excel**: Formato Excel
4. Escolha o local de salvamento
5. Clique em **Exportar**

#### Opções de Exportação

- **Campos**: Selecione quais campos exportar
- **Filtros**: Aplique filtros antes de exportar
- **Formatação**: Configure formatação de datas, números
- **Codificação**: Escolha a codificação (UTF-8, CP1252)

### Estrutura da Tabela

#### Visualizar Estrutura

1. Menu **Estrutura** → **Visualizar Estrutura**
2. A estrutura da tabela aparece em uma janela
3. Informações exibidas:
   - Nome do campo
   - Tipo de dado
   - Tamanho
   - Decimais
   - Chave primária
   - Permite nulo

#### Editar Estrutura

1. Menu **Estrutura** → **Editar Estrutura**
2. Adicione, modifique ou exclua campos
3. Clique em **Salvar** para aplicar alterações

#### Índices

1. Menu **Estrutura** → **Índices**
2. Visualize os índices da tabela
3. Crie, modifique ou exclua índices

### Transações

#### Iniciar Transação

1. Menu **Transação** → **Iniciar**
2. As alterações ficam pendentes
3. Você pode confirmar ou reverter

#### Confirmar Transação

1. Menu **Transação** → **Confirmar** (Commit)
2. As alterações são salvas permanentemente

#### Reverter Transação

1. Menu **Transação** → **Reverter** (Rollback)
2. As alterações são descartadas

### Relatórios

#### Gerar Relatório

1. Menu **Relatório** → **Gerar**
2. Configure o relatório:
   - Título
   - Campos a incluir
   - Filtros
   - Ordenação
   - Formatação
3. Clique em **Gerar**
4. O relatório é exibido

#### Exportar Relatório

1. Após gerar o relatório
2. Menu **Relatório** → **Exportar**
3. Selecione o formato (PDF, HTML, CSV)
4. Escolha o local de salvamento
5. Clique em **Exportar**

## Configurações

### Configurações de Conexão

Acesse **Ferramentas** → **Configurações** → **Conexão**:

- **Driver Padrão**: Driver padrão para conexão
- **Timeout**: Timeout de conexão
- **Pool Size**: Tamanho do pool de conexões
- **Auto Commit**: Commit automático

### Configurações de Interface

Acesse **Ferramentas** → **Configurações** → **Interface**:

- **Theme**: Tema da interface
- **Font Size**: Tamanho da fonte
- **Grid Lines**: Mostrar linhas do grid
- **Row Numbers**: Mostrar números de linha
- **Column Width**: Largura das colunas

### Configurações de Dados

Acesse **Ferramentas** → **Configurações** → **Dados**:

- **Page Size**: Tamanho da página
- **Max Rows**: Máximo de linhas
- **Auto Refresh**: Atualização automática
- **Cache Size**: Tamanho do cache

## Dicas e Truques

### Produtividade

- **Filtragem Rápida**: Use Ctrl+F para filtrar
- **Navegação**: Use as setas do teclado
- **Seleção Múltipla**: Use Ctrl+Click para selecionar
- **Atalhos**: Use atalhos para operações frequentes

### Boas Práticas

- **Backup**: Faça backup antes de alterações
- **Transações**: Use transações para operações complexas
- **Índices**: Crie índices para melhorar performance
- **Validação**: Valide dados antes de salvar
- **Documentação**: Documente estruturas complexas

### Performance

- **Limitar Resultados**: Use filtros para limitar resultados
- **Índices**: Use índices em consultas
- **Paginação**: Use paginação para tabelas grandes
- **Cache**: Ajuste o tamanho do cache

## Solução de Problemas

### Erros Comuns

**Erro ao abrir banco:**
- Verifique se o arquivo existe
- Confirme permissões de acesso
- Valide formato do arquivo

**Erro ao salvar:**
- Verifique espaço em disco
- Confirme permissões de escrita
- Valide integridade dos dados

**Erro de conexão:**
- Verifique se o driver está instalado
- Confirme configurações de conexão
- Valide credenciais (se aplicável)

### Log de Erros

O log de erros está disponível em:

- **Linux**: `~/.advpp/logs/adveditor.log`
- **Conteúdo**: Erros, warnings, informações de debug

### Recuperação de Dados

Em caso de perda de dados:

1. Menu **Arquivo** → **Restaurar**
2. Selecione o backup mais recente
3. Confirme a restauração
4. Valide os dados restaurados

## Integração com Outras Ferramentas

### AdvCfg

O AdvEditor se integra com o AdvCfg:

- Dicionário compartilhado
- Estruturas sincronizadas
- Validação consistente

### AdvPP IDE

O AdvEditor se integra com o AdvPP IDE:

- Queries diretas do IDE
- Debug de SQL
- Teste de procedures

## Atalhos de Teclado

| Comando | Atalho |
|---------|--------|
| Abrir Banco | Ctrl+B |
| Fechar Banco | Ctrl+W |
| Novo Registro | Ctrl+N |
| Salvar | Ctrl+S |
| Excluir | Delete |
| Filtro | Ctrl+F |
| SQL Personalizado | Ctrl+Q |
| Desfazer | Ctrl+Z |
| Refazer | Ctrl+Y |
| Copiar | Ctrl+C |
| Colar | Ctrl+V |
| Recarregar | F5 |
| Commit | Ctrl+K |
| Rollback | Ctrl+R |

## Exemplos Práticos

### Exemplo 1: Consultar Clientes

1. Abra o banco de dados
2. Selecione a tabela SA1
3. Aplique filtro: A1_FILIAL = '01'
4. Ordene por A1_NOME
5. Visualize os resultados

### Exemplo 2: Atualizar Endereço

1. Localize o cliente desejado
2. Edite o campo A1_END
3. Digite o novo endereço
4. Pressione Enter
5. A alteração é salva automaticamente

### Exemplo 3: Importar Clientes

1. Menu **Arquivo** → **Importar**
2. Selecione o arquivo CSV
3. Configure o mapeamento de campos
4. Clique em **Importar**
5. Valide os dados importados

## Drivers Suportados

### SQLite

- **Extensões**: .db, .sqlite, .sqlite3
- **Características**: Banco embutido, sem servidor
- **Vantagens**: Fácil uso, portabilidade
- **Limitações**: Escalabilidade limitada

### DBF

- **Extensões**: .dbf
- **Características**: Arquivo binário, compatibilidade Protheus
- **Vantagens**: Compatibilidade total
- **Limitações**: Tamanho máximo de arquivo

### TopConnect

- **Protocolo**: TCP/IP
- **Características**: Conexão remota
- **Vantagens**: Acesso a servidores Protheus
- **Limitações**: Requer servidor TopConnect

### Ctree

- **Protocolo**: TCP/IP
- **Características**: Banco de dados Ctree
- **Vantagens**: Performance alta
- **Limitações**: Requer servidor Ctree

### BTrieve

- **Protocolo**: TCP/IP
- **Características**: Banco de dados BTrieve
- **Vantagens**: Confiabilidade
- **Limitações**: Requer servidor BTrieve

## Segurança

### Permissões

- **Leitura**: Permite visualizar dados
- **Escrita**: Permite modificar dados
- **Exclusão**: Permite excluir dados
- **Estrutura**: Permite modificar estrutura

### Backup Automático

Configure backup automático:

1. Menu **Ferramentas** → **Configurações** → **Backup**
2. Configure:
   - Frequência
   - Local
   - Retenção
3. Ative o backup automático

### Auditoria

Ative auditoria para rastrear alterações:

1. Menu **Ferramentas** → **Configurações** → **Auditoria**
2. Configure:
   - Operações a registrar
   - Detalhes a capturar
   - Retenção de logs
3. Ative a auditoria

## Conclusão

O AdvEditor é uma ferramenta poderosa para manipulação de dados de banco de dados, oferecendo recursos modernos e compatibilidade com múltiplos formatos. Com este manual, você deve ser capaz de visualizar, editar e gerenciar dados de forma eficiente.

Para mais informações, visite a documentação oficial em https://github.com/peder1981/AdvPP.
