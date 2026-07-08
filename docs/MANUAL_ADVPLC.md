# Manual do Usuário - AdvPlc (Compilador AdvPL/TLPP)

## Introdução

O AdvPlc é o compilador oficial do AdvPP para as linguagens AdvPL e TLPP. Ele processa código fonte e gera bytecode executável pela máquina virtual do AdvPP, suportando tanto a sintaxe clássica do AdvPL quanto as extensões modernas do TLPP.

## Requisitos do Sistema

- **Sistema Operacional**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+)
- **Memória RAM**: Mínimo 1GB, recomendado 2GB
- **Espaço em Disco**: 50MB para instalação
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
go build -o advplc ./cmd/advplc

# Instalar
sudo cp advplc /usr/local/bin/
```

## Visão Geral

### O que é o AdvPlc?

O AdvPlc é um compilador que:

- **Processa código fonte** AdvPL/TLPP
- **Gera bytecode** para a máquina virtual
- **Valida sintaxe** e semântica
- **Otimiza código** para melhor performance
- **Gera informações** de debug

### Fluxo de Compilação

```
Código Fonte (.prw/.tlpp)
    ↓
Análise Léxica (Tokenização)
    ↓
Análise Sintática (AST)
    ↓
Análise Semântica (Validação)
    ↓
Otimização
    ↓
Geração de Bytecode (.bytecode)
    ↓
Execução na VM
```

## Uso Básico

### Compilar Arquivo Individual

```bash
advplc arquivo.prw
```

Isso gera o arquivo `arquivo.bytecode`.

### Compilar com Saída Específica

```bash
advplc -o output.bytecode arquivo.prw
```

### Compilar Múltiplos Arquivos

```bash
advplc arquivo1.prw arquivo2.prw arquivo3.prw
```

### Compilar Diretório Inteiro

```bash
advplc -d ./src
```

Compila todos os arquivos `.prw` e `.tlpp` no diretório especificado.

## Opções de Linha de Comando

### Opções Principais

| Opção | Descrição | Exemplo |
|-------|-----------|---------|
| `-o, --output` | Define arquivo de saída | `-o saida.bytecode` |
| `-d, --dir` | Compila diretório inteiro | `-d ./src` |
| `-r, --recursive` | Compila recursivamente | `-r ./src` |
| `-v, --verbose` | Modo verboso | `-v` |
| `-q, --quiet` | Modo silencioso | `-q` |
| `-h, --help` | Mostra ajuda | `-h` |
| `--version` | Mostra versão | `--version` |

### Opções de Compilação

| Opção | Descrição | Exemplo |
|-------|-----------|---------|
| `-t, --target` | Define target (advpl/tlpp) | `-t tlpp` |
| `-O, --optimize` | Nível de otimização (0-3) | `-O2` |
| `-g, --debug` | Inclui símbolos de debug | `-g` |
| `--strict` | Modo estrito de validação | `--strict` |
| `--no-warnings` | Suprime warnings | `--no-warnings` |

### Opções de Include

| Opção | Descrição | Exemplo |
|-------|-----------|---------|
| `-I, --include` | Adiciona diretório de include | `-I ./include` |
| `-D, --define` | Define macro | `-D DEBUG` |
| `-U, --undefine` | Undefine macro | `-U DEBUG` |

## Sintaxe AdvPL

### Estrutura Básica

```advpl
// Comentário de linha
/* Comentário de bloco */

#include "totvs.ch"

function NomeFuncao(param1, param2)
    local nVar1 := 0
    local cVar2 := ""
    
    // Código da função
    nVar1 := param1 + param2
    
return nVar1
```

### User Functions

```advpl
user function NomeUF(param1)
    local nResult := 0
    
    // Código da user function
    nResult := param1 * 2
    
return nResult
```

### Variáveis

```advpl
// Variáveis locais
local nNumero := 0
local cTexto := "Hello"
local dData := Date()
local lLogico := .T.
local aArray := {}

// Variáveis privadas
private nPrivado := 0

// Variáveis públicas
public nPublico := 0
```

### Estruturas de Controle

```advpl
// If/Else
if nValor > 10
    Alert("Maior que 10")
else
    Alert("Menor ou igual a 10")
endif

// Do While
do while nContador < 10
    nContador++
enddo

// For
for nI := 1 to 10
    Alert(Str(nI))
next nI

// Case
do case
case nOpcao == 1
    Alert("Opção 1")
case nOpcao == 2
    Alert("Opção 2")
otherwise
    Alert("Outra opção")
endcase
```

### Funções de Manipulação de Dados

```advpl
// Database
dbSelectArea("SA1")
dbSeek("001")
dbSkip()
dbGoTop()
dbGoBottom()

// Queries
TCQuery("SELECT * FROM SA1 WHERE A1_FILIAL = '01'")
```

## Sintaxe TLPP

### Classes

```tlpp
class MinhaClasse
    private:
        nValor := 0
        cNome := ""
    
    public:
        method New(nValor, cNome) constructor
        method GetValor()
        method SetValor(nValor)
        method GetNome()
        method SetNome(cNome)
endclass

method New(nValor, cNome) class MinhaClasse
    ::nValor := nValor
    ::cNome := cNome
return self

method GetValor() class MinhaClasse
return ::nValor

method SetValor(nValor) class MinhaClasse
    ::nValor := nValor
return
```

### Interfaces

```tlpp
interface IMinhaInterface
    method Metodo1()
    method Metodo2()
endinterface

class MinhaClasse implements IMinhaInterface
    public:
        method Metodo1()
        method Metodo2()
endclass
```

### Operadores

```tlpp
class Numero
    private:
        nValor := 0
    
    public:
        method New(nValor) constructor
        method operator+(obj) // Sobrecarga de +
        method operator-(obj) // Sobrecarga de -
endclass

method operator+(obj) class Numero
    local nResult := Numero()
    nResult:SetValor(::nValor + obj:GetValor())
return nResult
```

### Try/Catch

```tlpp
try
    // Código que pode gerar erro
    nResult := 10 / 0
catch e
    // Tratamento de erro
    Alert("Erro: " + e:Message())
finally
    // Sempre executado
    Alert("Fim")
endtry
```

### JSON Inline

```tlpp
local jData := {
    "nome": "João",
    "idade": 30,
    "ativo": true
}

local cNome := jData["nome"]
local nIdade := jData["idade"]
```

### Namespaces

```tlpp
namespace MeuNamespace

class MinhaClasse
    // ...
endclass

endnamespace

// Uso
local obj := MeuNamespace.MinhaClasse()
```

### Tipagem

```tlpp
// Tipos explícitos
function Soma(n1: numeric, n2: numeric): numeric
return n1 + n2

// Inferência de tipos
local nValor := 10 // numeric
local cTexto := "Hello" // character
```

### Parâmetros Nomeados

```tlpp
function MinhaFuncao(p1, p2, p3)
    // ...
return

// Chamada com parâmetros nomeados
MinhaFuncao(p3=10, p1=20, p2=30)
```

## Erros Comuns de Compilação

### Erros de Sintaxe

**Erro: Esperado ';'**
```advpl
// Errado
local nVar := 0

// Correto
local nVar := 0;
```

**Erro: Parênteses não fechados**
```advpl
// Errado
if nValor > 10
    Alert("Maior"

// Correto
if nValor > 10
    Alert("Maior")
endif
```

### Erros de Tipo

**Erro: Tipo incompatível**
```advpl
// Errado
local nNumero := "texto" // string em vez de número

// Correto
local nNumero := 10
local cTexto := "texto"
```

### Erros de Escopo

**Erro: Variável não declarada**
```advpl
// Errado
nValor := 10 // variável não declarada

// Correto
local nValor := 10
```

### Erros de Include

**Erro: Arquivo não encontrado**
```advpl
// Errado
#include "arquivo_inexistente.ch"

// Correto
#include "totvs.ch"
```

## Otimizações

### Níveis de Otimização

**-O0**: Sem otimização (padrão)
- Compilação rápida
- Bytecode não otimizado
- Útil para debug

**-O1**: Otimização básica
- Remoção de código morto
- Simplificação de expressões
- Bom equilíbrio

**-O2**: Otimização agressiva
- Inlining de funções
- Loop unrolling
- Melhor performance

**-O3**: Otimização máxima
- Todas as otimizações de -O2
- Otimizações avançadas
- Maior tempo de compilação

### Exemplo de Otimização

```bash
# Sem otimização
advplc -O0 arquivo.prw

# Otimização básica
advplc -O1 arquivo.prw

# Otimização agressiva
advplc -O2 arquivo.prw

# Otimização máxima
advplc -O3 arquivo.prw
```

## Debug

### Compilar com Debug

```bash
advplc -g arquivo.prw
```

Isso inclui informações de debug no bytecode, permitindo:

- Breakpoints
- Inspeção de variáveis
- Call stack
- Step-by-step

### Executar com Debug

Use o AdvPP IDE para depurar o bytecode gerado.

## Integração com IDE

### Compilação via IDE

O AdvPP IDE integra o AdvPlc automaticamente:

1. Abra o arquivo no editor
2. Pressione **F5** para compilar
3. O resultado aparece no terminal
4. Erros são destacados no editor

### Configuração do Compilador

Configure o compilador no IDE:

1. Menu **Ferramentas** → **Configurações** → **Compilação**
2. Configure:
   - Caminho do compilador
   - Flags de compilação
   - Diretórios de include
   - Nível de otimização

## Projetos

### Arquivo de Projeto

Crie um arquivo `advpl-project.json`:

```json
{
  "name": "Meu Projeto",
  "version": "1.0.0",
  "target": "tlpp",
  "optimization": 2,
  "includes": [
    "./include"
  ],
  "defines": [
    "DEBUG"
  ],
  "sources": [
    "./src"
  ],
  "output": "./build"
}
```

### Compilar Projeto

```bash
advplc -p advpl-project.json
```

## Boas Práticas

### Nomenclatura

- **Funções**: Use `PascalCase` para funções
- **Variáveis**: Use `camelCase` para variáveis
- **Constantes**: Use `UPPER_CASE` para constantes
- **Classes**: Use `PascalCase` para classes

### Organização

```
projeto/
├── src/              # Arquivos fonte
│   ├── main.prw
│   └── functions.prw
├── include/          # Arquivos de cabeçalho
│   └── myheader.ch
├── build/            # Arquivos compilados
└── advpl-project.json
```

### Comentários

```advpl
// Comentário de linha - use para explicações curtas

/*
 * Comentário de bloco
 * Use para explicações detalhadas
 */

/**
 * Documentação de função
 * @param nParam1 Descrição do parâmetro
 * @return Descrição do retorno
 */
function MinhaFuncao(nParam1)
    // ...
return
```

### Validação

```advpl
// Valide entradas
if Empty(cNome)
    Alert("Nome obrigatório")
    return .F.
endif

// Valide tipos
if ValType(nValor) != "N"
    Alert("Valor deve ser numérico")
    return .F.
endif
```

## Solução de Problemas

### Erro: Arquivo não encontrado

**Causa**: Caminho do arquivo incorreto

**Solução**:
```bash
# Verifique se o arquivo existe
ls -la arquivo.prw

# Use caminho absoluto
advplc /caminho/completo/arquivo.prw
```

### Erro: Include não encontrado

**Causa**: Diretório de include não configurado

**Solução**:
```bash
# Adicione diretório de include
advplc -I ./include arquivo.prw
```

### Erro: Memória insuficiente

**Causa**: Arquivo muito grande

**Solução**:
```bash
# Compile arquivos individualmente
advplc arquivo1.prw
advplc arquivo2.prw
```

### Erro: Sintaxe inválida

**Causa**: Erro de sintaxe no código

**Solução**:
```bash
# Use modo verboso para detalhes
advplc -v arquivo.prw

# Verifique linha indicada no erro
```

## Performance

### Tempo de Compilação

- **Arquivo pequeno** (< 100 linhas): < 50ms
- **Arquivo médio** (100-1000 linhas): < 200ms
- **Arquivo grande** (> 1000 linhas): < 1s

### Tamanho do Bytecode

- **Arquivo pequeno**: ~1KB
- **Arquivo médio**: ~10KB
- **Arquivo grande**: ~100KB

### Otimizações de Performance

- Use `-O2` para melhor equilíbrio
- Compile apenas arquivos modificados
- Use include paths corretos
- Evite includes desnecessários

## Exemplos Completos

### Exemplo 1: Hello World

```advpl
#include "totvs.ch"

function Hello()
    Alert("Hello, World!")
return
```

### Exemplo 2: Função Matemática

```advpl
function Soma(n1, n2)
    local nResult := 0
    
    nResult := n1 + n2
    
return nResult
```

### Exemplo 3: Manipulação de Banco

```advpl
function ConsultaCliente(cCod)
    local cNome := ""
    
    dbSelectArea("SA1")
    dbSeek(cCod)
    
    if Found()
        cNome := SA1->A1_NOME
    endif
    
return cNome
```

### Exemplo 4: Classe TLPP

```tlpp
class Pessoa
    private:
        cNome := ""
        nIdade := 0
    
    public:
        method New(cNome, nIdade) constructor
        method GetNome()
        method SetNome(cNome)
        method GetIdade()
        method SetIdade(nIdade)
endclass

method New(cNome, nIdade) class Pessoa
    ::cNome := cNome
    ::nIdade := nIdade
return self

method GetNome() class Pessoa
return ::cNome

method SetNome(cNome) class Pessoa
    ::cNome := cNome
return
```

## Referência de Comandos

### Comandos Rápidos

```bash
# Compilar arquivo
advplc arquivo.prw

# Compilar com saída específica
advplc -o output.bytecode arquivo.prw

# Compilar diretório
advplc -d ./src

# Compilar com otimização
advplc -O2 arquivo.prw

# Compilar com debug
advplc -g arquivo.prw

# Compilar projeto
advplc -p advpl-project.json

# Mostrar versão
advplc --version

# Mostrar ajuda
advplc --help
```

## Conclusão

O AdvPlc é um compilador poderoso e flexível para AdvPL/TLPP, suportando tanto a sintaxe clássica quanto as extensões modernas. Com este manual, você deve ser capaz de compilar código fonte, otimizar performance e resolver problemas comuns de compilação.

Para mais informações, visite a documentação oficial em https://github.com/peder1981/AdvPP.
