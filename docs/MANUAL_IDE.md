# Manual do Usuário - AdvPP IDE

## Introdução

O AdvPP IDE é um ambiente de desenvolvimento integrado completo para programação AdvPL/TLPP, projetado para proporcionar uma experiência moderna e eficiente para desenvolvedores TOTVS Protheus.

## Requisitos do Sistema

- **Sistema Operacional**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+)
- **Memória RAM**: Mínimo 4GB, recomendado 8GB
- **Espaço em Disco**: 500MB para instalação
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
go build -o advpp-ide ./cmd/advpp-ide

# Instalar
sudo cp advpp-ide /usr/local/bin/
```

## Primeiros Passos

### Inicialização

Para iniciar o AdvPP IDE:

```bash
advpp-ide
```

Ou através do menu de aplicações do seu sistema.

### Interface Principal

A interface do AdvPP IDE é composta por:

- **Barra de Menu**: Acesso a todas as funcionalidades
- **Barra de Ferramentas**: Acesso rápido a comandos frequentes
- **Explorador de Arquivos**: Navegação por arquivos do projeto
- **Editor de Código**: Área principal de edição
- **Terminal**: Terminal integrado para comandos
- **Barra de Status**: Informações sobre o estado atual

## Funcionalidades

### Editor de Código

#### Recursos do Editor

- **Syntax Highlighting**: Colorização de sintaxe AdvPL/TLPP
- **Autocompletar**: Sugestões automáticas de código
- **Formatação**: Formatação automática de código
- **Folding**: Recolhimento de blocos de código
- **Multi-cursor**: Edição simultânea em múltiplas posições
- **Snippets**: Fragmentos de código reutilizáveis

#### Atalhos de Teclado

| Comando | Atalho |
|---------|--------|
| Novo Arquivo | Ctrl+N |
| Abrir Arquivo | Ctrl+O |
| Salvar Arquivo | Ctrl+S |
| Salvar Como | Ctrl+Shift+S |
| Fechar Arquivo | Ctrl+W |
| Desfazer | Ctrl+Z |
| Refazer | Ctrl+Y |
| Copiar | Ctrl+C |
| Colar | Ctrl+V |
| Recortar | Ctrl+X |
| Localizar | Ctrl+F |
| Substituir | Ctrl+H |
| Ir para Linha | Ctrl+G |
| Compilar | F5 |
| Executar | F9 |

### Gerenciamento de Projetos

#### Criar Novo Projeto

1. Menu **Arquivo** → **Novo Projeto**
2. Selecione o tipo de projeto (AdvPL ou TLPP)
3. Defina o nome e localização
4. Configure as propriedades do projeto

#### Abrir Projeto Existente

1. Menu **Arquivo** → **Abrir Projeto**
2. Navegue até o diretório do projeto
3. Selecione o arquivo `.advpp-project`

#### Estrutura de Projeto

```
meu-projeto/
├── src/              # Arquivos fonte
│   ├── main.prw
│   └── functions.prw
├── include/          # Arquivos de cabeçalho
│   └── myheader.ch
├── resources/        # Recursos do projeto
│   └── images/
└── .advpp-project   # Configuração do projeto
```

### Compilação

#### Compilar Arquivo Individual

1. Abra o arquivo no editor
2. Pressione **F5** ou menu **Compilar** → **Compilar Arquivo**
3. O resultado aparece no terminal

#### Compilar Projeto Completo

1. Menu **Compilar** → **Compilar Projeto**
2. Todos os arquivos são compilados
3. Erros e warnings são listados

#### Opções de Compilação

- **Target**: AdvPL ou TLPP
- **Output**: Bytecode ou executável
- **Optimization**: Nível de otimização
- **Debug**: Incluir símbolos de debug

### Depuração

#### Iniciar Depuração

1. Menu **Depurar** → **Iniciar Depuração**
2. Defina breakpoints clicando na margem do editor
3. Use os controles de depuração para navegar

#### Controles de Depuração

- **Step Over (F10)**: Executa linha atual
- **Step Into (F11)**: Entra em função
- **Step Out (Shift+F11)**: Sai de função
- **Continue (F5)**: Continua execução
- **Stop (Shift+F5)**: Para depuração

#### Inspeção de Variáveis

- **Watch**: Adiciona variáveis para monitoramento
- **Locals**: Mostra variáveis locais
- **Call Stack**: Mostra pilha de chamadas

### Integração com Dicionário

#### Autocompletar Baseado no Dicionário

O IDE se integra com o dicionário de dados para fornecer autocompletar inteligente:

1. Configure o caminho do dicionário em **Ferramentas** → **Configurações**
2. O IDE carrega automaticamente tabelas e campos
3. Ao digitar, sugestões contextuais aparecem

#### Navegação para Definições

- **Ctrl+Click**: Vai para definição de símbolo
- **F12**: Vai para definição
- **Shift+F12**: Vai para referências

### Gerenciamento de Versões

#### Integração Git

O IDE possui integração nativa com Git:

- **Commit**: Menu **Git** → **Commit**
- **Push**: Menu **Git** → **Push**
- **Pull**: Menu **Git** → **Pull**
- **Branch**: Menu **Git** → **Gerenciar Branches**
- **Merge**: Menu **Git** → **Merge**

#### Visualização de Diff

- **Diff View**: Mostra diferenças entre versões
- **Blame**: Mostra autor de cada linha
- **History**: Mostra histórico de alterações

## Configurações

### Configurações do Editor

Acesse **Ferramentas** → **Configurações** → **Editor**:

- **Fonte**: Tipo e tamanho da fonte
- **Tema**: Tema de cores (claro/escuro)
- **Indentação**: Tamanho e tipo (tabs/espaços)
- **Line Numbers**: Mostrar números de linha
- **Word Wrap**: Quebra de linha automática

### Configurações de Compilação

Acesse **Ferramentas** → **Configurações** → **Compilação**:

- **Compilador**: Caminho do compilador
- **Flags**: Flags de compilação adicionais
- **Output Directory**: Diretório de saída
- **Include Path**: Caminhos de include

### Configurações do Dicionário

Acesse **Ferramentas** → **Configurações** → **Dicionário**:

- **Database Path**: Caminho do banco de dados
- **Auto Load**: Carregar automaticamente
- **Cache Size**: Tamanho do cache

## Ferramentas Externas

### AdvCfg (Configurador de Tabelas)

Acesse através de **Ferramentas** → **AdvCfg**:

- Gerenciamento de tabelas do dicionário
- Definição de estrutura de campos
- Configuração de índices
- Definição de triggers

### AdvEditor (Editor de Banco de Dados)

Acesse através de **Ferramentas** → **AdvEditor**:

- Visualização de dados de tabelas
- Edição de registros
- Execução de queries SQL
- Importação/exportação de dados

## Dicas e Truques

### Produtividade

- **Multi-cursor**: Alt+Click para criar múltiplos cursores
- **Quick Open**: Ctrl+P para abrir arquivos rapidamente
- **Command Palette**: Ctrl+Shift+P para comandos
- **Split View**: Ctrl+\ para dividir a tela

### Snippets Úteis

```advpl
// Função padrão
function nomeFuncao()
    // código
return

// User Function
user function nomeUF()
    // código
return
```

### Boas Práticas

- Use nomes descritivos para variáveis e funções
- Comente código complexo
- Mantenha funções pequenas e focadas
- Use constantes em vez de valores mágicos
- Valide entradas de usuário

## Solução de Problemas

### Erros Comuns

**Erro de compilação:**
- Verifique sintaxe do código
- Confirme que todos os includes estão acessíveis
- Valide tipos de dados

**Erro de execução:**
- Verifique se o bytecode foi gerado corretamente
- Confirme dependências do sistema
- Valide contexto de execução

**Problemas de performance:**
- Limpe o cache do projeto
- Desabilite plugins não utilizados
- Aumente a memória alocada

### Log de Erros

O log de erros está disponível em:

- **Linux**: `~/.advpp/logs/`
- **Conteúdo**: Erros, warnings, informações de debug

### Suporte

Para obter suporte:

- **GitHub Issues**: https://github.com/peder1981/AdvPP/issues
- **Documentação**: https://github.com/peder1981/AdvPP/wiki
- **Comunidade**: Fórum da comunidade AdvPP

## Atalhos Completos

### Menu Arquivo

| Comando | Atalho |
|---------|--------|
| Novo Arquivo | Ctrl+N |
| Abrir Arquivo | Ctrl+O |
| Salvar Arquivo | Ctrl+S |
| Salvar Todos | Ctrl+Shift+S |
| Fechar Arquivo | Ctrl+W |
| Fechar Todos | Ctrl+Shift+W |
| Sair | Ctrl+Q |

### Menu Editar

| Comando | Atalho |
|---------|--------|
| Desfazer | Ctrl+Z |
| Refazer | Ctrl+Y |
| Recortar | Ctrl+X |
| Copiar | Ctrl+C |
| Colar | Ctrl+V |
| Localizar | Ctrl+F |
| Substituir | Ctrl+H |
| Ir para Linha | Ctrl+G |

### Menu Compilar

| Comando | Atalho |
|---------|--------|
| Compilar Arquivo | F5 |
| Compilar Projeto | Ctrl+Shift+B |
| Executar | F9 |
| Depurar | Ctrl+D |

### Menu Depurar

| Comando | Atalho |
|---------|--------|
| Iniciar Depuração | F5 |
| Parar Depuração | Shift+F5 |
| Step Over | F10 |
| Step Into | F11 |
| Step Out | Shift+F11 |
| Toggle Breakpoint | F9 |

## Recursos Avançados

### Macros

Grave e reproduza sequências de ações:

1. Menu **Ferramentas** → **Macros** → **Gravar Macro**
2. Execute as ações desejadas
3. Menu **Ferramentas** → **Macros** → **Parar Gravação**
4. Salve a macro para uso futuro

### Plugins

O IDE suporta plugins para extensão de funcionalidades:

- Instale plugins através de **Ferramentas** → **Plugins**
- Configure plugins em **Ferramentas** → **Configurações** → **Plugins**
- Desenvolva plugins usando a API pública

### Customização

Personalize o IDE:

- **Temas**: Crie ou importe temas personalizados
- **Keybindings**: Configure atalhos personalizados
- **Snippets**: Crie snippets personalizados
- **Templates**: Crie templates de projeto

## Conclusão

O AdvPP IDE é uma ferramenta poderosa para desenvolvimento AdvPL/TLPP, oferecendo recursos modernos e integração completa com o ecossistema TOTVS Protheus. Com este manual, você deve ser capaz de utilizar todas as funcionalidades principais e aumentar sua produtividade no desenvolvimento.

Para mais informações, visite a documentação oficial em https://github.com/peder1981/AdvPP.
