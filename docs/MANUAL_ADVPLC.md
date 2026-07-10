# Manual do Usuário - AdvPlc (Compilador AdvPL/TLPP)

## Introdução

O AdvPlc é o compilador oficial do AdvPP para as linguagens AdvPL e TLPP. Ele processa código fonte e gera bytecode executável pela máquina virtual do AdvPP, suportando tanto a sintaxe clássica do AdvPL quanto as extensões modernas do TLPP.

## Requisitos do Sistema

- **Sistema Operacional**: Linux, Windows 64-bit ou macOS (binário 100% estático, sem dependências externas)
- **Arquiteturas**: amd64 e arm64
- **Espaço em Disco**: ~15MB

## Instalação

### Via Release do GitHub (recomendado)

Baixe o pacote da sua plataforma em https://github.com/peder1981/AdvPP/releases:

- `advpp-cli-<versão>-linux-amd64.tar.gz` / `linux-arm64`
- `advpp-cli-<versão>-windows-amd64.zip`
- `advpp-cli-<versão>-darwin-arm64.tar.gz` (macOS Apple Silicon)
- `advpp_<versão>_amd64.deb` (Debian/Ubuntu, inclui as ferramentas gráficas)

```bash
# Linux/macOS
tar xzf advpp-cli-*.tar.gz && sudo mv advplc /usr/local/bin/

# Debian/Ubuntu (suite completa)
sudo dpkg -i advpp_*_amd64.deb
```

### Via Compilação

```bash
# Clonar repositório
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP

# Compilar todas as ferramentas
make build

# Cross-compilar o CLI para todas as plataformas (dist/)
make cross
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

O advplc trabalha com subcomandos:

```bash
advplc <comando> <arquivo(s)> [opções]
```

### Executar um fonte diretamente

```bash
advplc run programa.prw
advplc run programa.prw --ui              # com diálogos gráficos (Fyne)
advplc run programa.prw --db-path meu.db  # conectado a um banco específico
```

### Compilar para bytecode

```bash
advplc compile programa.prw -o programa.bytecode
advplc exec programa.bytecode              # executa o bytecode
```

### Verificar sintaxe (um ou vários arquivos, em paralelo)

```bash
advplc check programa.prw

# Múltiplos arquivos: verificação paralela (1 worker por CPU).
# Os arquivos vêm ANTES das opções:
advplc check src/*.prw src/*.tlpp -I ./includes
```

Saída do modo múltiplo: `OK`/`FAIL` por arquivo e um resumo
(`checked N files: X ok, Y failed (W workers)`). Código de saída 1 se
qualquer arquivo falhar.

### Executável standalone

```bash
advplc build programa.prw -o programa
```

Embute bytecode e runtime num executável independente (requer Go instalado).

### Modo web (renderer no browser)

```bash
advplc serve programa.prw               # http://localhost:8080
advplc serve programa.prw --port 9000
advplc serve programa.prw --watch       # hot reload ao salvar o fonte
```

Executa o programa no servidor (mesma VM, mesmo banco `ADVPP.db`) e
renderiza a interface no browser com **PO-UI** (framework visual da
TOTVS), embutida no próprio binário — não precisa de Node, npm ou
SmartClient. Cada aba/recarga do browser cria uma sessão com VM isolada
e conexão própria ao banco.

O que é renderizado:

| Recurso AdvPL | No browser |
|---------------|------------|
| `ConOut(...)` | Console em tempo real |
| `MsgInfo`/`MsgStop`/`MsgAlert`/`MsgYesNo`/`Alert` | Diálogos PO-UI que bloqueiam a execução até a resposta |
| `FWMBrowse():New()` + `SetAlias("SA1")` + `SetDescription(...)` + `Activate()` | **`po-table`** com colunas e títulos do dicionário SX3; Incluir/Editar abrem **`po-dynamic-form`** gerado do dicionário; Excluir faz soft-delete (`D_E_L_E_T_='*'`) |
| `DEFINE MSDIALOG` + `@ linha,coluna SAY/GET/BUTTON` + `ACTIVATE MSDIALOG` | Modal PO-UI montado por heurística de grade; o que o usuário digita nos `GET`s **volta para as variáveis** do programa |

Com `--watch` (ou `-w`), o fonte é recompilado a cada alteração e todas
as sessões do browser recarregam automaticamente; erro de compilação
aparece no console do browser sem recarregar.

A porta padrão pode ser fixada em `~/.advpp/advpp_config.json`
(`"webui_port": "9000"`) — configuração compartilhada que futuramente
será editável pelo AdvCfg.

Limitações atuais do MSDIALOG web: codeblocks não capturam variáveis
locais (`ACTION {|| oDlg:End()}` não fecha o diálogo — qualquer clique
de botão fecha após executar o `ACTION`); `VALID` por campo ainda não
dispara validação no servidor.

### Inspeção

```bash
advplc ast programa.prw       # imprime a AST
advplc bytecode programa.prw  # imprime o bytecode
advplc version                # versão do compilador
```

## Opções de Linha de Comando

| Opção | Descrição | Exemplo |
|-------|-----------|---------|
| `-I, --include <dir>` | Adiciona diretório de include (repetível) | `-I ./includes` |
| `-D, --define <k=v>` | Define símbolo do preprocessador | `-D DEBUG=1` |
| `--ui` | Habilita interface gráfica (Fyne) | `--ui` |
| `--headless` | Desabilita UI (padrão) | `--headless` |
| `--db <backend>` | Backend de banco: sqlite (padrão) | `--db sqlite` |
| `--db-path <path>` | Caminho do banco SQLite | `--db-path dados.db` |
| `--port <n>` | Porta do modo web (`serve`) | `--port 9000` |
| `-w, --watch` | Hot reload no modo web (`serve`) | `--watch` |
| `-o <file>` | Arquivo de saída (compile/build) | `-o saida.bytecode` |

### Banco de dados compartilhado

Sem `--db-path`, o runtime resolve o banco na mesma ordem de todas as
ferramentas AdvPP: variável `ADVPP_DB` → config `~/.advpp/advpp_config.json`
→ padrão `~/.advpp/ADVPP.db`. Assim `DBSelectArea`/`DBSeek`/`RecCount` etc.
enxergam exatamente o mesmo banco que o AdvCfg e o AdvEditor.

### Encoding CP-1252

Fontes em CP-1252 (padrão Protheus) são detectados e convertidos
automaticamente para UTF-8 por conversor interno 100% Go — não há
dependência do `iconv` e o comportamento é idêntico em Linux, Windows e macOS.

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

## Performance

O compilador é otimizado para fontes reais do Protheus: um arquivo de
~574KB compila em ~0,1s e um lote de 300 fontes reais é verificado em
~1,2s com `advplc check` paralelo (1 worker por CPU).

Não há níveis de otimização configuráveis — o bytecode gerado é único.

## Multi-thread no runtime

### StartJob

```advpl
// StartJob(cFunc, cEnv, lWait, params...)
StartJob("U_Worker", "ENV", .F., "parametro")   // assíncrono
StartJob("U_Worker", "ENV", .T., "parametro")   // síncrono (bloqueia)
```

Cada job roda em uma VM isolada (memória própria, como um work process
do Protheus) com sua própria conexão ao banco SQLite compartilhado.
Jobs assíncronos pendentes são aguardados antes de o processo encerrar.

### FWGridProcess

Processamento em grid com pool de threads (ver documentação TDN):

```advpl
oGrid := FWGridProcess():New("ROTINA", "Titulo", "Descricao", Nil, "", "U_GridWk", .T.)
oGrid:SetThreadGrid(4)              // pool de 4 threads
For nX := 1 To 100
    If !oGrid:CallExecute(nX)       // despacha p/ o pool (backpressure)
        Exit                        // .F. = StopExecute() foi chamado
    EndIf
Next nX
oGrid:Activate()                    // espera as threads terminarem
If oGrid:IsFinished()
    ConOut("ok")
EndIf
```

Métodos suportados: `New`, `Activate`, `Execute`, `DeActivate`,
`SetThreadGrid`, `SetMaxThreadGrid`, `CallExecute`, `StopExecute`,
`IsFinished`, `SetAbort`, `SetAfterExecute`, `SetMeters`, `SetMaxMeter`,
`SetIncMeter`, `SetNoParam`, `SaveLog`, `GetLastLog`.
A interface gráfica de configuração do Protheus não é reproduzida
(runtime headless); a semântica de processamento é completa.

## Motor de inferência LLM (classe `LLM`)

O AdvPP embute um motor de inferência para modelos de linguagem
quantizados em **I2_S** (pesos ternários -1/0/+1, formato BitNet),
escrito inteiramente em Go — sem CGO, sem `llama.cpp`, sem
dependências de terceiros. Compila e roda de forma idêntica em Linux,
Windows e macOS (amd64/arm64).

### Exemplo

```advpl
User Function LlmDemo()
    Local oLLM
    Local cModelo := "/caminho/Falcon3-3B-Instruct-1.58bit/ggml-model-i2_s.gguf"

    oLLM := LLM():New(cModelo)
    ConOut(oLLM:Generate("The capital of France is", 6, 0))
    oLLM:Close()
Return
```

### Métodos

| Método | Parâmetros | Retorno | Descrição |
|--------|-----------|---------|-----------|
| `New` | `cCaminhoGGUF` | `Object` (self) | Carrega o modelo e o tokenizer. Pode levar alguns segundos (o arquivo GGUF tem tipicamente 1-2GB). |
| `Generate` | `cPrompt, nMaxTokens, nTemperatura` | `Character` | Roda o prompt pela rede e amostra até `nMaxTokens` novos tokens. `nTemperatura<=0` faz amostragem gulosa (greedy, determinística); a chamada **bloqueia** até terminar ou encontrar o token de fim de sequência. |
| `Tokenize` | `cTexto` | `Array` | Retorna os token ids (números) do texto, via BPE byte-level. |
| `Decode` | `aTokens` | `Character` | Converte um array de token ids de volta em texto. |
| `Close` | — | `Nil` | Libera o arquivo do modelo. |

### Arquitetura suportada

Só arquitetura GGUF `general.architecture = "llama"` com tensores de
peso em **I2_S** — é o caso do BitNet original convertido para essa
arquitetura e de conversões como o `Falcon3-3B-Instruct-1.58bit`. Não
suporta (ainda) a arquitetura customizada `bitnet-b1.58` (que tem
normas extras "SubLN" no grafo) nem outras quantizações (Q4_K, Q6_K,
etc.) — ver `pkg/llm/model.go`.

### Desempenho e SIMD

O kernel do produto escalar ternário usa **AVX2** em CPUs amd64 que o
suportam (detecção via CPUID em tempo de execução — sem AVX2, ou fora
de amd64, cai automaticamente para um caminho escalar puro em Go,
idêntico em resultado, só mais lento). Matmuls e atenção são
paralelizados por faixa de linhas/cabeças via goroutines
(`runtime.GOMAXPROCS`). Referência: ~5s/token no Falcon3-3B-1.58bit em
8 núcleos com AVX2.

### Validação

O motor foi validado **token a token** contra o `llama.cpp` de
referência (mesmo algoritmo do BitNet oficial) — ver
`pkg/llm/validate_test.go`. Rode `go test ./pkg/llm/...` (sem
`-short`) para reproduzir, desde que tenha o modelo e um binário
`llama-cli` de referência disponíveis localmente.

### Limitações conhecidas

- Sem streaming: `Generate` só devolve o texto completo ao final.
- Pré-tokenizador simplificado: não replica o split dígito-a-dígito
  específico do pré-tokenizador "falcon3" do llama.cpp (só afeta
  números com mais de um dígito).
- Uma sessão (`Context`) por objeto `LLM`; sem suporte a múltiplas
  sequências simultâneas na mesma chamada.

## Servidor MCP (classe `MCPServer`)

O AdvPP fala **MCP (Model Context Protocol)** nativamente — um servidor
que expõe funções AdvPL/TLPP como "tools" que qualquer cliente MCP
(Claude, outros agentes de IA) pode listar e chamar via JSON-RPC 2.0
sobre stdio. Diferente do suporte a REST (`WSRESTFUL`/`WSSERVICE`/
`@Get`/`@Post`), que hoje só **reconhece a sintaxe** e a descarta (sem
subir servidor HTTP nem despachar nada — ver seção de Sintaxe TLPP), o
`MCPServer` **executa de verdade**.

### Exemplo

```advpl
User Function McpDemo()
    Local oMCP := MCPServer():New("meu-servidor", "1.0.0")

    oMCP:AddTool("soma", "Soma dois números", ;
        '{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}', ;
        "ToolSoma")

    oMCP:Serve() // bloqueia lendo stdin / escrevendo stdout
Return

User Function ToolSoma(oArgs)
Return cValToChar(oArgs:A + oArgs:B)
```

Rode com `advplc run mcp_demo.prw` — não precisa de nenhum comando
novo de CLI. O processo vira um servidor MCP: qualquer cliente que
fale o protocolo (ex.: via `stdio_client` do SDK oficial) pode se
conectar via stdin/stdout do processo.

### Métodos

| Método | Parâmetros | Descrição |
|--------|-----------|-----------|
| `New` | `cNome, cVersao` | Cria o servidor (nome/versão aparecem no handshake `initialize`) |
| `AddTool` | `cNome, cDescricao, cSchemaJSON, cNomeFuncao` | Registra uma tool. `cSchemaJSON` é o JSON Schema dos parâmetros (pode ser `""` para aceitar qualquer objeto). `cNomeFuncao` é o nome de uma User Function que recebe **um objeto** com os argumentos da chamada (`oArgs:CAMPO`, em maiúsculas) e devolve o texto do resultado. |
| `Serve` | — | Sobe o loop de mensagens JSON-RPC sobre stdio. **Bloqueia** até o cliente fechar a conexão (EOF no stdin). Redireciona `ConOut`/console para stderr automaticamente, para não misturar saída de depuração com as mensagens do protocolo no stdout. |

### Isolamento de execução

Cada `tools/call` roda a função registrada em uma **VM isolada**
(mesmo mecanismo do `StartJob`) — necessário porque `Serve()` já está
em execução no meio da VM principal quando uma chamada chega; invocar
a função direto na mesma VM corromperia a pilha de chamadas em
andamento.

### Protocolo suportado

`initialize`, `notifications/initialized`, `tools/list`, `tools/call`,
`ping` — o essencial para expor tools. Não implementa `resources/*`,
`prompts/*` nem `sampling/*` (extensões do protocolo fora do escopo de
"tools", que é o caso de uso mais comum).

### Validação

Testado com o **SDK oficial em Python do MCP** (`stdio_client` +
`ClientSession`), não só com mensagens JSON-RPC feitas à mão — ver
`cmd/advplc/mcp_integration_test.go`.

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
