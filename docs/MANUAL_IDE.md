# Manual do Usuário — AdvPP IDE

## Introdução

O AdvPP IDE (`advpp-ide`) é um editor gráfico leve (Fyne) para arquivos
AdvPL/TLPP, com highlight de sintaxe e comandos de compilar/rodar
integrados ao compilador `advplc`. Não é um IDE completo no sentido
VS Code/IntelliJ — para debugger real, autocompletar, "ir para definição"
e integração Git, use a [extensão VS Code](../tools/vscode-advpl/README.md)
(`advpl-tlpp-advpp`), que é a ferramenta que de fato implementa isso.

## Requisitos do Sistema

- **Sistema Operacional**: Linux, Windows ou macOS (Fyne é multiplataforma)
- **Memória RAM**: Mínimo 2GB
- **Processador**: x86_64 (amd64) ou ARM64

## Instalação

### Via Pacote Debian/Ubuntu (Linux)

```bash
curl -sL https://api.github.com/repos/peder1981/AdvPP/releases/latest \
  | grep browser_download_url | grep amd64.deb | cut -d'"' -f4 | xargs wget
sudo dpkg -i advpp_*_amd64.deb
sudo apt-get install -f
```

### Via Compilação

```bash
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP
go build -o advpp-ide ./cmd/advpp-ide
sudo cp advpp-ide /usr/local/bin/   # opcional
```

## Iniciando

```bash
advpp-ide
```

## Interface

A janela é dividida em: árvore de arquivos (esquerda), editor de código
(centro) e painel de saída/output (embaixo), com a barra de menu no topo.

## Menu File

| Item | O que faz |
|---|---|
| New | Limpa o editor para um `untitled.prw` novo |
| Open | Abre `.prw`/`.tlpp`/`.prg` via diálogo de arquivo |
| Save | Salva no arquivo atual (pede caminho se ainda não tiver um) |
| Save As | Salva com novo nome/caminho |
| Exit | Fecha a janela |

## Menu Edit

Os itens Cut/Copy/Paste/Find/Replace do menu **ainda não estão
implementados** (`// TODO` no código-fonte, `cmd/advpp-ide/main.go`) — são
placeholders sem ação. Recorte/copiar/colar continuam funcionando pelo
atalho de teclado padrão do sistema (Ctrl+X/C/V), porque o campo de edição
usa o widget nativo de texto do Fyne, que já suporta isso independente do
menu. Não há find/replace (nem por menu nem por atalho) hoje.

## Menu Build

| Item | O que faz |
|---|---|
| Compile | Roda o pipeline preprocess/lex/parse/compile sobre o arquivo aberto e mostra erros no painel de saída |
| Run | Compila e executa o arquivo (equivalente a `advplc run`) |
| Compile and Run | Os dois passos acima em sequência |
| Build standalone executable... | Gera um executável standalone (equivalente a `advplc build`) |

## Menu View

Toggle File Tree / Toggle Output estão no menu mas **ainda não
implementados** (mesma situação do Edit — placeholders `// TODO`).

## Menu Tools

**Open AdvEditor (database)**: inicia o `adveditor` (editor de banco/
dicionário, processo separado) — procura o binário ao lado do próprio
`advpp-ide` e cai para o `PATH` se não achar.

## Menu Help

**About**: mostra informações de versão.

## Syntax Highlighting

O editor colore a sintaxe AdvPL/TLPP (palavras-chave, strings,
comentários, etc.) via análise por expressão regular, atualizada quando o
campo perde o foco (não em tempo real a cada tecla).

## O que este IDE NÃO tem (evite documentação/expectativa contrária)

- Sem debugger integrado (breakpoints, step, watch, call stack) — isso
  existe de verdade no `advplc debug` + extensão VS Code (protocolo DAP
  real, `pkg/dap/`), não aqui.
- Sem autocompletar, "ir para definição" ou referências.
- Sem integração Git nativa.
- Sem sistema de projetos (`.advpp-project`), snippets, multi-cursor,
  macros, plugins ou temas customizáveis.
- Sem tela de configurações (fonte/tema/indentação etc.).

## Solução de Problemas

**Erro de compilação**: o painel de saída mostra a mensagem real do
`advplc` (linha/coluna do erro).

**AdvEditor não abre pelo menu Tools**: confirme que o binário `adveditor`
está ao lado do `advpp-ide` (mesmo diretório de instalação) ou no `PATH`.

### Suporte

- **GitHub Issues**: https://github.com/peder1981/AdvPP/issues
- **Repositório**: https://github.com/peder1981/AdvPP
