# Componentes PO-UI no AdvPP (base gráfica do AdvOS) — Design

Data: 2026-11-20 · Status: **aprovado pelo operador** (2026-11-20), com a condição da seção 5

## 1. Intenção

O AdvOS (sistema para o Chromebook Lenovo 100e Gen 3, em `~/Projetos/AdvOS`) deve
mostrar o máximo do que o compilador AdvPP consegue. É o computador do próprio autor do
AdvPP: um ambiente de **conectividade, multitarefa e desenvolvimento**, em que a
interface é escrita em AdvPL/TLPP e renderizada em PO-UI.

Hoje isso esbarra no renderer web (`advplc serve`). O frontend (`web/src/app/app.ts`,
373 linhas) só conhece 8 eventos e 8 componentes (`po-page-default`, `po-modal`,
`po-button`, `po-input`, `po-table`, `po-dynamic-form`, `po-tag`, `po-label`), e o
modelo é **bloqueante**: o programa para em cada `MsgYesNo`/`FWMenuSelect`. Por isso o
painel do AdvOS é uma sequência de menus.

Este projeto cria no AdvPP uma **camada de componentes PO-UI orientada a eventos**,
que serve a qualquer programa AdvPP e não só ao AdvOS. É o primeiro de três projetos:

1. **Este:** componentes + laço de eventos + vários programas por servidor.
2. AdvOS: área de trabalho (shell gráfico) sobre esta camada.
3. AdvOS: aplicativos (IDE AdvPL, terminal, arquivos, rede, monitor do sistema).

Os projetos 2 e 3 terão designs próprios. Aqui eles aparecem só como **primeiro cliente**:
os componentes da v1 são escolhidos pelo que eles vão precisar.

## 2. Objetivos

- **API em AdvPL/TLPP** para montar telas como objetos (`PoPage`, `PoMenu`,
  `PoTable`…), com eventos tratados por *codeblocks* (`:OnClick({|o| ...})`).
- **Não bloqueante:** depois de montar a tela, o programa entra num laço de eventos
  (`oApp:Activate()`). Clique, digitação, timer e linha de processo filho viram eventos
  tratados um de cada vez, na VM da janela.
- **Telas vivas:** alterar uma propriedade no AdvPL (`oBat:SetValue(87)`) atualiza o
  navegador na hora, sem recarregar.
- **Trabalho assíncrono:** timers (`oApp:SetTimer`) e processos filhos com saída em
  streaming (`oApp:Spawn`) não travam a tela (compilação, `nmcli`, `git`).
- **Vários programas por servidor:** um `advplc serve` atende a área de trabalho e
  cada aplicativo numa rota própria. Cada janela é uma sessão com VM própria
  (multitarefa real).
- **Editor de código e terminal** como componentes próprios do AdvPP (decisão do
  operador): `AdvCodeEditor` (CodeMirror 6, realce AdvPL/TLPP, marcação de erros) e
  `AdvTerminal` (xterm.js sobre um pseudo-terminal real).
- O frontend continua **genérico**: nenhum código Angular específico de aplicativo.

## 3. Não-objetivos

- Cobrir todo o catálogo PO-UI (~70 componentes). A v1 tem o conjunto da seção 6.2, e
  o resto entra sob demanda.
- Paridade no modo desktop Fyne (`advplc build`) ou no console. Os componentes novos
  são só do modo web. Fora dele, a classe instancia mas `Activate()` devolve erro claro.
- Reescrever `FWFormView`/`FWMBrowse`/MSDIALOG. Eles continuam como estão.
- Acesso remoto multiusuário. O alvo é `localhost`, e as regras de origem atuais seguem
  valendo.

## 4. Anti-objetivos (seria falha mesmo funcionando)

- Quebrar programas existentes: `MsgInfo`/`MsgYesNo`/`FWMenuSelect`/`FWGetText`/
  `MSDIALOG`/`FWMBrowse` em `advplc serve` continuam idênticos.
- Lógica de aplicativo em TypeScript.
- Abrir brecha de execução: `Spawn` e terminal executam sem shell implícito, só
  `localhost`, com a mesma checagem de origem de hoje.
- Quebrar o job `determinism` da CI (bytecode idêntico nos 4 SOs).
- Bloquear a VM numa chamada de rede/processo dentro de um handler: o laço de eventos
  não pode congelar a janela.

## 5. Restrições

- Go sem CGO. O frontend é Angular + PO-UI compilado por `make web` (Node 20) e
  versionado em `pkg/webui/dist` (embed).
- A VM é single-thread por sessão: handlers rodam no goroutine da VM. Goroutines
  auxiliares (timers, processos, pty) **só postam eventos**, nunca tocam a VM.
- Regras do repositório: testes cross-platform (`docs/testing-cross-platform.md`) e
  CHANGELOG.
- Dependências novas exigem aprovação do operador. As três da seção 10 foram aprovadas.
- **Condição do operador para aprovar:** o AdvPP segue **100% compatível** (todo programa
  e teste existente se comporta igual) e continua **gerando executáveis para macOS,
  Linux e Windows**. Critério de aceite de **toda fase**: `go test ./...` verde; a matriz
  da CI (Linux amd64/arm64, Windows, macOS) verde; o `cli-cross` gerando os 6 alvos; e o
  job `determinism` com bytecode idêntico nos 4 SOs.
- Código dependente de SO (pty) fica atrás de build tags: o Windows compila e gera o
  `.exe` normalmente; na v1 o `AdvTerminal` no Windows devolve erro claro ("terminal
  indisponível no Windows nesta versão"). ConPTY fica como evolução.

## 6. Arquitetura

### 6.1 Abordagens consideradas

| | Abordagem | Troca | Decisão |
|---|---|---|---|
| A | Componentes PO-UI dentro do AdvPP: classes AdvPL → árvore de componentes enviada ao navegador; eventos voltam como codeblocks | Maior esforço (VM, `pkg/webui`, frontend), mas tudo fica no compilador e serve a qualquer programa | **Escolhida pelo operador** |
| B | Backend AdvPL via REST (`WSRestServer`) + frontend Angular/PO-UI escrito à mão | Interface rica mais rápido, mas a interface fica em TypeScript, contra o propósito | descartada |
| C | Ampliar o MSDIALOG bloqueante (combo, checkbox, multiget) | Barato, mas sequencial: sem multitarefa nem tela viva | descartada |
| — | Janela nativa Fyne (sem Chromium) | Mais leve, mas o operador quer PO-UI | descartada |

Editor e terminal: **componentes próprios fora do PO-UI** (CodeMirror 6 / xterm.js),
escolha do operador. As alternativas (só `po-textarea`; só PO-UI agora e próprios
depois) foram descartadas por enfraquecer o foco em desenvolvimento.

### 6.2 API AdvPL/TLPP

Classes nativas em Go, registradas como as atuais (`FWMBROWSE`, `WSRESTSERVER`) em
`pkg/vm/vm.go`. Não precisam de `#include`.

```advpl
User Function Main()
	Local oApp  := PoApp():New("Rede")
	Local oPage := PoPage():New(oApp, "Wi-Fi")
	Local oTab  := PoTable():New(oPage)
	Local oBat  := PoGauge():New(oPage, "Bateria")

	oTab:SetColumns({{"ssid", "Rede"}, {"sinal", "Sinal", "number"}})
	oTab:OnSelect({|o, aLinha| Conectar(aLinha)})
	oPage:AddAction("Atualizar", {|| Varre(oTab)})
	oApp:SetTimer(5000, {|| oBat:SetValue(LeBateria())})
	oApp:Activate()        // laço de eventos; volta quando a janela fecha ou oApp:Close()
Return
```

Convenções (valem para todas as classes):

- `:New(oPai, ...)`: o pai é `PoApp`, uma página ou um contêiner. A ordem de criação é
  a ordem na tela.
- Propriedade: `:Set(cProp, xValor)` genérico, mais atalhos tipados (`SetValue`,
  `SetLabel`, `SetItems`, `SetVisible`, `SetDisabled`). Com a tela ativa, cada `Set`
  gera um patch.
- Evento: `:On<Evento>(bBloco)`. O bloco recebe o próprio componente e o valor do
  evento.
- `:GetValue()` devolve o último valor conhecido (o navegador envia `change`).

`PoApp`: `New(cTitulo)`, `Activate()`, `Close()`, `SetTimer(nMs, bBloco) -> nId`,
`KillTimer(nId)`, `Spawn(cProg, aArgs, bLinha, bFim) -> nPid` (sem shell; stdout e
stderr linha a linha como eventos), `Notify(cTexto, cTipo)` (toast PO-UI),
`Confirm(cTexto, bSim)` (diálogo não bloqueante), `Open(cRota)` (pede ao shell do AdvOS
outra janela), `Navigate(oPage)`.

Componentes da v1 (classe AdvPL → PO-UI):

| Grupo | Classes | PO-UI |
|---|---|---|
| Estrutura | `PoPage`, `PoMenu`, `PoTabs`/`PoTab`, `PoContainer`, `PoDivider`, `PoWidget` | `po-page-default`, `po-menu`, `po-tabs`, `po-container`, `po-divider`, `po-widget` |
| Dados | `PoTable`, `PoTree`, `PoList`, `PoInfo`, `PoTag`, `PoProgress`, `PoGauge`, `PoChart` | `po-table`, `po-tree-view`, `po-list-view`, `po-info`, `po-tag`, `po-progress`, `po-gauge`, `po-chart` |
| Entrada | `PoInput`, `PoPassword`, `PoNumber`, `PoSelect`, `PoCheckbox`, `PoSwitch`, `PoTextarea`, `PoButton` | `po-input`, `po-password`, `po-number`, `po-select`, `po-checkbox`, `po-switch`, `po-textarea`, `po-button` |
| Retorno | `PoApp:Notify`, `PoApp:Confirm`, `PoLoading` | `PoNotificationService`, `PoDialogService`, `po-loading-overlay` |
| Próprios | `AdvCodeEditor`, `AdvTerminal` | CodeMirror 6, xterm.js |

`AdvCodeEditor`: `SetText`/`GetText`, `SetLanguage("advpl"|"tlpp"|"json"|"sh")`,
`SetMarkers(aErros)` (linha, coluna, mensagem; o formato sai de
`advplc check --format json`, que já existe), `OnSave(bBloco)` (Ctrl+S),
`OnChange` com debounce.
`AdvTerminal`: `:New(oPai, cProg, aArgs)` abre um pseudo-terminal em Go. A entrada e o
redimensionamento vão direto ao pty, sem passar pela VM. `OnExit(bBloco)`.

### 6.3 Protocolo (sobre o SSE atual)

Evento SSE novo `ui`, sem mudar os tipos existentes:

- `{"type":"ui","op":"mount","tree":{id,type,props,children[]}}`: no `Activate()`.
- `{"op":"patch","id":17,"props":{...}}`: um `Set` com a tela ativa.
- `{"op":"insert"|"remove", ...}`: componente criado/destruído depois do mount.
- `{"op":"call","id":17,"method":"append","args":[...]}`: métodos sem estado no
  servidor (ex.: `PoChart:AddPoint`, `PoApp:Notify`).
- `{"op":"term","id":21,"data":"<base64>"}`: saída do pty.

Do navegador ao servidor: `POST /ui?s=<sid>` com `{"id":17,"event":"click","value":...}`,
e `POST /term?s=<sid>&id=21` com a entrada crua. Limite de corpo e checagem de origem
iguais ao `/reply`.

IDs são numéricos, sequenciais por sessão. Sem reconciliação de árvore no cliente: o
servidor manda operações explícitas.

### 6.4 Laço de eventos e concorrência

`Activate()` roda no goroutine da VM:

```
mount -> loop:
  ev := <-sessao.eventos (navegador, timers, processos, fim de pty)
  handler := tabela[ev.id][ev.event]
  callBlockSync(handler, componente, valor)  // mesma VM, sem corrida
  envia patches acumulados durante o handler (lote)
até Close() ou sessão abandonada
```

- Timers e `Spawn` são goroutines que **só postam** no canal da sessão.
- Handler demorado trava só a sua janela. As outras janelas são outras sessões/VMs.
  Fica documentado: trabalho longo vai em `Spawn` ou `StartJob`.
- Diálogos bloqueantes antigos (`MsgYesNo`…) chamados dentro de um handler continuam
  funcionando, porque usam o mesmo `ask` com o mesmo canal de resposta.

### 6.5 Vários programas por servidor

`advplc serve --apps <dir> [--home desktop.prw]` faz cada `<dir>/<nome>.prw|.tlpp`
virar a rota `/app/<nome>`. O modo atual `advplc serve programa.prw` continua igual. O
frontend lê a rota e pede `/events?s=..&app=<nome>`. Compilação sob demanda com cache
por mtime (reaproveita o `--watch`).

### 6.6 Frontend

`app.ts` ganha um `UiHostComponent` recursivo que mapeia `type` para o componente
PO-UI correspondente, com `@switch` sobre os tipos da tabela da seção 6.2. Os eventos
dos componentes viram `POST /ui`. Os componentes próprios ficam em
`web/src/app/adv-code-editor` e `web/src/app/adv-terminal`. O tema e as cores seguem o
PO-UI.

## 7. Testes

- **Go:** protocolo (mount/patch/ordem de IDs), laço de eventos (handler recebe valor;
  patch durante handler sai em lote; timer; `Spawn` com linhas e código de saída; sessão
  abandonada encerra o laço), terminal com pty (eco de um `cat`), checagem de origem nas
  rotas novas.
- **Sem navegador:** o padrão do AdvOS `tests/serve.sh`: dirigir `/events` + `/ui` por
  HTTP com um programa de exemplo e conferir os patches.
- **Navegador:** um teste por componente com Chromium headless sobre
  `examples/poui/*.prw`, com captura de tela para inspeção.
- **Compatibilidade:** os testes atuais do `serve` passam sem mudança.
- **Exemplos:** `examples/poui/` com um programa por grupo de componentes, mais um
  "monitor do sistema" usando timer, gauge, chart e `Spawn`.

## 8. Entregas em fases (cada uma utilizável sozinha)

1. Protocolo + laço de eventos + `PoApp`/`PoPage`/`PoButton`/`PoInput`/`PoTable`/
   `PoTag` + timers.
2. Os demais componentes PO-UI da v1 + `Spawn` + `Notify`/`Confirm`.
3. `advplc serve --apps` (vários programas).
4. `AdvCodeEditor` + `AdvTerminal`.

Depois da fase 4 começam os projetos 2 (área de trabalho) e 3 (aplicativos), com
designs próprios.

## 9. Decisões tomadas pelo autor do design (corrigíveis)

- Classes nativas em Go, e não biblioteca TLPP com `#include`: segue o padrão de
  `FWMBrowse`/`WSRestServer`, não exige include e as classes ficam disponíveis a
  qualquer programa.
- Operações explícitas (mount/patch) em vez de diff de árvore no cliente: menos código
  e depuração mais fácil.
- Prefixo `Po*` para PO-UI e `Adv*` para os componentes próprios, deixando visível o
  que é TOTVS e o que é do AdvPP.
- Modo web apenas, com erro claro fora dele.

## 10. Dependências novas (aprovadas pelo operador em 2026-11-20)

- `github.com/creack/pty` (Go, sem CGO): pseudo-terminal no Linux e no macOS.
- `codemirror` + `@codemirror/*` e `@xterm/xterm` (npm, para o frontend). Somam
  ~400 KB ao binário.

## 11. Deixado para quem implementa

Nomes internos, a gramática de realce AdvPL para o CodeMirror (partindo do lexer do
AdvPP), o debounce exato, o layout dos exemplos e a organização dos arquivos Go
(sugestão: `pkg/webui/ui.go`, `pkg/vm/poui_native.go`).
