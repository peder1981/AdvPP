# Relatório de Status dos Componentes

## Componentes PO-UI orientados a eventos — fase 1 (atualizado 2026-11-20)

### Status Atual: Funcional e testado ponta a ponta (HTTP real + captura no Chromium)

- ✅ **`PoApp`/`PoPage`/`PoButton`/`PoInput`/`PoTable`/`PoTag`**: classes nativas
  (`pkg/vm/poui*.go`), árvore de componentes por VM.
- ✅ **Laço de eventos `PoApp:Activate()`**: handlers em codeblocks na VM da sessão,
  patches em lote, erro em handler não derruba o laço, evento sem destino é ignorado.
- ✅ **Timers `PoApp:SetTimer/KillTimer`**: param ao fechar a sessão (sem vazar goroutine).
- ✅ **Transporte**: evento SSE `ui` + `POST /ui` (`pkg/webui/ui.go`), origem e limite iguais a `/reply`.
- ✅ **Frontend**: `web/src/app/ui-host.ts` (renderer recursivo genérico).
- ✅ **Fase 2**: `PoMenu` + várias páginas (`Navigate`), `PoContainer`/`PoWidget`/
  `PoDivider`/`PoTabs`/`PoTab`, `PoTree`/`PoList`/`PoInfo`/`PoProgress`/`PoGauge`/
  `PoChart`, campos tipados, `SetCols`, `Notify`/`Confirm`/`Loading` e `Spawn`
  (sem shell, sem perda de linhas, morto ao fechar). Teste de navegador:
  `tests/poui/ao-vivo.sh`.
- ✅ **Fase 3**: `advplc serve --apps <dir> [--home <nome>]` (vários programas,
  VM por janela, compilação sob demanda) e `PoApp:Open`.
- ✅ **Fase 4**: `AdvCodeEditor` (CodeMirror 6, marcações via `check --format json`)
  e `AdvTerminal` (xterm.js + pty; Windows: erro claro nesta versão).
- ✅ **Onda 3a.1 — seleção**: `PoCombo`/`PoMultiselect`/`PoRadioGroup`/
  `PoCheckboxGroup`/`PoLookup` (`pkg/vm/poui3a_select.go`). `aOpcoes` no formato
  `{{cValor, cLabel}}`; URL de filtro prefixada com `RestPrefix`; o PO-UI chama o
  REST direto do navegador (`p-filter-service` string + `p-field-label`/
  `p-field-value`, contrato `{items, hasNext}` da onda 1). Métodos: `SetOptions`,
  `SetFilterService`, `SetFilterParams` (pares → objeto plano), `SetColumns`
  (número nos grupos; `{cProp, cRotulo[, cTipo]}` no lookup). Eventos: `OnChange`
  (valor; array em multiselect/checkboxgroup) e `OnSelected(oLinha)` no lookup.
- ✅ **Onda 3a.2 — data/hora**: `PoDatepicker`/`PoDatepickerRange`/
  `PoDatetimepicker`/`PoTimepicker` (`pkg/vm/poui3a_datetime.go`). Fronteira:
  `DateValue` sai como `AAAA-MM-DD` (vazio = `""`); evento ISO volta para
  `DateValue`; range usa `{start,end}` ISO; datetime preserva ISO 8601; hora
  `HH:MM[:SS]`. Métodos: `SetValue`/`GetValue`/`SetMinDate`/`SetMaxDate`/
  `SetFormat`/`SetMinTime`/`SetMaxTime`/`SetFormatDate`/`SetFormatTime`.
  Eventos: `OnChange`.
- ✅ **Onda 3a.3 — texto especializado**: `PoDecimal`/`PoEmail`/`PoUrl`/
  `PoRichText`/`PoSearch` (`pkg/vm/poui3a_text.go`). `SetValue`/`GetValue`/
  `SetDecimals`/`SetMin`/`SetMax`/`SetHeight`/`SetItems`/`SetFilterKeys`;
  eventos `OnChange`/`OnFilter`.
- ✅ **Onda 3a.4 — upload**: `PoUpload` + `POST /poui-upload?sid=&id=`
  (`pkg/webui/poui_upload.go`, helper em `pkg/tools/shared/poui_upload.go`).
  Multipart campo `files`; destino resolvido com `filepath.Abs`, restrito ao
  diretório de trabalho do serve ou a `PouiUploadDirs` do config; nomes
  sanitizados com `filepath.Base`; extensões proibidas → 415; limite default
  10 MB, teto configurável 100 MB via `SetMaxSizeMB` (limite de body próprio,
  não o geral de 8 MB). Retorna `aArquivos={{cNome,cCaminhoGravado,nBytes}}`.
- ✅ **Onda 3a.5 — páginas estáticas**: `PoPageList`/`PoPageEdit`/
  `PoPageDetail` como páginas do `PoApp` (Navigate incluso). Eventos:
  filtro/busca avançada (list), save/save-new/cancel (edit),
  edit/remove/back (detail). Render via `po-page-list/edit/detail` PO-UI.
- ✅ **Onda 3a.6 — templates dinâmicos**: `PoPageDynamicTable`/
  `PoPageDynamicEdit`/`PoPageDynamicDetail`/`PoPageDynamicSearch`
  (`pkg/vm/poui3a_dyn.go`, hooks em `poui3a_dyn_hook.go`). `New(oApp,
  cTitulo[, cUrl])` — url relativa ganha `RestPrefix`; CRUD direto no
  navegador contra o REST da onda 1 (`GET {items,hasNext}`, POST/PUT/
  DELETE `<url>/<id>`). `aFields={{cProp,cLabel,cTipo,lKey,lFilter,lVisible}}`
  → `{property,label,type,key,filter,visible}` do `po-dynamic-form`/
  `po-table` (frontend `dyn*` em `ui-host.ts`). `OnBeforeSave`/
  `OnBeforeRemove`: round-trip síncrono browser→AdvPL via op
  `beforeResult` {reqId,ok} com timeout de 30 s = recusa. Fixture:
  `tests/poui/3a_crud.prw` (compila; serve verificado com REST real).
- ✅ **Onda 3b.1 — agrupamento**: `PoAccordion`/`PoAccordionItem`
  (`SetShowManager`, `Expand`/`Collapse`, `OnExpand`/`OnCollapse`),
  `PoModal` (`Open`/`Close`/`SetPrimaryAction`/`SetSecondaryAction`/
  `SetSize`/`OnClose`; ações = eventos `click:*`), `PoStepper`/`PoStep`
  (`Next`/`Previous`/`GetStep`/`SetOrientation`/`OnChangeStep`;
  `OnCanActivate` via round-trip `beforeResult`), `PoSlide` (`aSlides`
  → `{image,alt,link}`, `SetInterval`/`SetHeight`). Modal renderizado
  sob `@if open` com diretiva `appModalOpen`.
- ✅ **Onda 3b.2 — ações**: `PoButtonGroup` (`AddButton(cLabel,cIcone,
  bAcao)`, `SetSelected`, toggle `none|single|multiple`), `PoDropdown`,
  `PoPopup`/`PoPopover` (alvo = componente, resolvido via `data-poid`),
  `PoContextMenu` (`OnSelect`/`OnChangeExpanded`), `PoToolbar`
  (`SetProfile`, `SetNotifications`). Ações = eventos `act:N` pelo
  dispatch genérico de handlers.
- ✅ **Onda 3b.3 — navegação**: `PoLink` (`SetOpenNewTab`, `OnClick`),
  `PoBreadcrumb` (`SetItems` com `{{cLabel,cLinkOuPagina}}` — página vira
  `{page:id}` e o evento `navigate` resolve o objeto para
  `OnNavigate(oComp, cLabel)`), `PoContextTabs` (`AddTab`/`CloseTab`;
  render via filhos `po-tab`), e o chrome do app: `PoNavbar`
  (`SetLogo`/`AddItem`/`AddIconAction`), `PoHeader` (`SetUser`/
  `AddAction`) e `PoMenuPanel` (`AddItem`/`SetLogo`) — singletons
  serializados como nós `nav`/`head`/`panel` do `mountJSON`, renderizados
  acima do conteúdo no `app.ts`; ações = `act:N` pelo dispatch genérico.
- ✅ **Onda 3b.4 — exibição**: `PoAvatar`, `PoBadge` (`SetValue`/
  `SetColor`/`SetStatus`), `PoImage` (`SetSrc`/`SetHeight`),
  `PoSkeleton` (`SetVariant`), `PoHelper` (`SetType`), `PoCalendar`
  (`SetMode`/`SetValue`/`GetValue`, `OnChange(d)`), `PoDynamicView`
  (`SetValue`/`SetFields`/`SetLoad`), `PoDisclaimerGroup` (`SetItems`/
  `SetTitle`, `OnChange`/`OnRemoveAll`), `PoFilterChip` (`SetSelected`,
  `OnChange`). `PoToaster` = `PoNotificationService`: `PoApp:Notify`
  estendido `(cMsg, cTipo, nDuracao, cOrientacao, cAcaoLabel, bAcao)` —
  ação volta ao AdvPL via evento `notifyact`. `GET /poui-asset?sid=&p=`
  serve src relativa de imagem com a mesma checagem de caminho do upload
  (`DirUploadPermitido` + rejeição de traversal + limite 10 MB).
- ✅ **Onda 3b.5 — páginas especiais**: `PoPageLogin` (`SetLiterals`/
  `SetRecovery`, `OnLogin(cUsuario,cSenha)→lOk` síncrono via
  `beforeResult`), `PoPageChangePassword` (`SetRequirements`,
  `OnSubmit(cAtual,cNova)→lOk`), `PoPageBlockedUser` (`SetContactEmail`),
  `PoPageSlide` (`Open`/`Close`, `OnClose`, aceita filhos) e
  `PoPageJobScheduler` (`SetParameters`, `cUrlServico` com RestPrefix).
  Senhas trafegam só no payload do evento — nunca em props, console,
  log ou `Get()` (`TestLoginSenhaNaoFicaNaProp`).
- ✅ **Aceite 3b**: `tests/poui/3b_vitrine.prw` — app com header,
  navbar, menu panel, menu, breadcrumb, context tabs, stepper, modal e
  componentes de exibição (compila e passa no `advplc check`).
- ✅ **Onda 3c — serviços e diretivas**: `PoApp:SetTheme` (`light`/`dark`
  ou JSON de tokens → `PoThemeService`), `PoApp:SetLanguage`
  (`pt`/`en`/`es`/`ru` → `PoLanguageService`), `PoApp:OnMediaChange`
  (breakpoints do grid PO-UI: sm≤1024, md≤1366, lg≤1465, xl; estado
  inicial + mudanças), `PoApp:OnNetworkChange` (`navigator.onLine` +
  eventos online/offline), `PoApp:StorageSet/Get/Remove` (localStorage
  namespaced `advpp:poui:<sid>:<chave>`; `StorageGet` síncrono via
  round-trip com timeout de 5 s → `Nil`, eventos alheios bufferizados em
  `a.deferred`), `PoApp:StartGuide`/`OnGuideEnd` (`PoUserGuideService`,
  passos `{oComponente,cTitulo,cTexto}` → `data-poid`), `SetTooltip` em
  todo componente (`p-tooltip` no wrapper, posição validada) e
  `SetDraggable`/`OnDrop` (HTML5 DnD; `OnDrop(oArrastado,nPos)` reordena
  `c.children` no servidor — evento `drop {drag,pos}`). Eventos de app
  usam o id reservado `poAppNodeID` (fora da sequência nextID).
  Fixture: `tests/poui/3c_servicos.prw`.
- ✅ **Onda 3c.4 — avaliação decidida** (operador aprovou
  `docs/superpowers/specs/2026-11-23-poui-3c-avaliacao.md`, 2026-11-23):
  **PoSync** adiado/fora de escopo (esforço alto, valor baixo hoje);
  **PoSearchAi** fora de escopo (exige decisão de produto/infra de IA);
  **po-code-editor** não substitui o `AdvCodeEditor` — se um app exigir,
  expor um `PoCodeEditor` fino sob demanda.

## gRPC embarcado, Smart Link real e auditoria de 27 docs TDN (atualizado 2026-08-23)

### Status Atual: Funcional e testado ponta a ponta (cliente e servidor reais)

Auditoria de conformidade contra 27 documentos TDN de referência
(diretivas de preprocessador `#command`/`#xcommand`/`#translate`/
`#xtranslate`, criptografia, classes "Não Visual"/"TLPP - Classes
úteis", família RPO), seguida da implementação de todos os gaps reais
encontrados e de duas rodadas de acompanhamento pedidas depois:
"implemente o framework gRPC real (https://grpc.io/)" no lugar de um
stub, e — ao descobrir via código-fonte real do Protheus 12.1.2510
(TechFin `backoffice.techfin.util.smartlink.tlpp`, RH Insights
`rh.sigagpe.insights.sendendcalc.tlpp`) que apps reais usam
`FwTotvsLinkClient` (HTTP+fila) em vez de `tGrpc` diretamente —
implementar essa classe também.

### O Que Funciona:
- ✅ **`Argon2id`** (função) e **`tPBKDF2`** (classe OOP completa,
  SHA1–SHA3-512): via `golang.org/x/crypto/argon2`/`pbkdf2`
- ✅ **`tHashMap` classe OOP** (`New/Set/Get/Del/List/Clean/nStatus`):
  interopera com a API funcional histórica `HMNew`/`HMSet`/...
- ✅ **`tJsonParser`** (`New/Json_Hash/Json_Parser/json_ok`): parsing
  JSON real, popula um `tHashMap` real quando pré-criado pelo chamador
- ✅ **`tUnicode`** (`Normalize` NFC/NFD/NFKC/NFKD,
  `ConvertEncoding`): via `golang.org/x/text/unicode/norm`
- ✅ **`TFtpClient`**: cliente FTP real (RFC 959, PASV) — testado
  upload/download com round-trip de conteúdo verificado contra um
  servidor FTP real
- ✅ **`Resource2File`/`GetPatchFile`/`SRCheckSourceSignature`**:
  completam a família "Manipulação de RPO"
- ✅ **2 bugs reais corrigidos no motor `#command`**: wild match marker
  `<*x*>`, extended-expression match marker `<(x)>` (nome do marcador
  nunca era casado), dumb stringify `#<x>` e smart stringify `<(x)>`
  (nunca reconhecidos) — a suíte de 500 testes que levou o motor a
  "100%" não exercitava essas 4 formas
- ✅ **`tGrpc`** (cliente gRPC real): `google.golang.org/grpc` de
  verdade — conexão HTTP/2 real, descoberta de serviço/método via
  **gRPC Server Reflection** em runtime (sem `.proto` local nenhum) e
  invocação dinâmica via `dynamicpb`
- ✅ **`GRPCServer`** (servidor gRPC real, simétrico ao
  `WSRestServer`): `AddMethod`/`Serve` expõem `User Function` como RPCs
  unárias com Server Reflection habilitada — testado com um cliente Go
  genuíno que descobre e invoca via reflection, zero stub gerado em
  tempo de compilação. Achado real durante o teste: o handler
  inicialmente usava `jsonMapToAdvplObject` (maiusculiza chaves, correto
  para `MCPServer`) em vez de `jsonToAdvplValue` (preserva case, correto
  para JSON externo genérico com acesso por colchete) — corrigido
- ✅ **`FwTotvsLinkClient`** (cliente Smart Link real, `net/http`):
  OAuth2 `client_credentials` real; endpoint/credenciais/tenant via
  `ADVPP_SMARTLINK_BASEURL`/`TOKENURL`/`CLIENTID`/`CLIENTSECRET`/
  `TENANTID` — sem configurar, falha honesta de rede, nunca sucesso
  simulado

### Notas de Implementação:
- `pkg/vm/argon2_native.go`, `pkg/vm/pbkdf2_native.go`,
  `pkg/vm/tunicode_native.go`, `pkg/vm/ftpclient_native.go`,
  `pkg/vm/tgrpc_native.go`, `pkg/vm/grpcserver_native.go`,
  `pkg/vm/totvslinkclient_native.go`, `pkg/vm/jsonparser_native.go`
  (novos); `pkg/vm/matrizhashmap_native.go`, `pkg/vm/rpo_native.go`,
  `pkg/preprocessor/commands.go` (estendidos)
- Novas dependências: `google.golang.org/grpc` v1.71.0,
  `google.golang.org/protobuf` v1.36.4 (compatíveis com go1.24 —
  versões mais novas exigem go1.25); `golang.org/x/crypto` e
  `golang.org/x/text` já eram dependências existentes
- `go build`/`go vet` cross-compile Linux/Windows/macOS limpos;
  `go test ./...` 20/20 pacotes verdes, sem regressão
- Detalhe técnico completo (assinaturas, limitações reais que
  permanecem, notas de confiança 🟡 INFERIDO onde a TDN não publica
  subpágina de detalhe) em `docs/tdn-known-limitations.md`

## Primitivas de TUI de baixo nível (atualizado 2026-08-02)

### Status Atual: Funcional e coberto por teste de integração

Além do `TerminalUIProvider` (diálogos prontos: `MSDIALOG`/`FWGetText`/
`FWMenuSelect`), o AdvPP agora expõe primitivas visuais de baixo nível para
um programa AdvPL compor a própria TUI — estilo opencode/Claude Code (caixas
com borda, markdown renderizado no terminal, tela alternativa, streaming de
LLM token a token). Ver `pkg/vm/ui_render.go` e seção 4.4.1 do
`GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md`.

### O Que Funciona:
- ✅ **`UiBox`/`UiStreamBox`/`UiStreamReset`** (lipgloss): caixa com borda
  arredondada e título; a variante `Stream` apaga e redesenha por cima a
  cada chamada (altura rastreada em `VM.lastBoxLines`), dando o efeito de
  "cartão crescendo ao vivo" sem raw-mode de teclado
- ✅ **`UiMarkdown`** (glamour): negrito/itálico/listas/código/títulos para
  ANSI, estilo "dark" fixo — mesma cautela contra auto-detecção de tema via
  OSC 11 (pode travar em terminal/multiplexer que não responde) já aplicada
  ao lipgloss em `pkg/ui/terminal.go`
- ✅ **`UiAltScreenEnter`/`UiAltScreenExit`**: tela alternativa (buffer do
  vim/less/htop) com handler de Ctrl+C que restaura antes de encerrar —
  sem isso um Ctrl+C durante o app travaria o terminal do usuário na tela
  alternativa
- ✅ **`UiTermWidth`**: largura real do terminal, cai num default
  configurável quando stdout não é TTY
- ✅ **`ConOutRaw`**: escreve sem newline — texto em streaming (deltas de
  LLM) na mesma linha
- ✅ **`ConIn` distingue EOF real de linha vazia**: agora devolve Nil (não
  `""`) quando stdin fecha de verdade (Ctrl+D, pipe esgotado) — um REPL
  consegue sair do loop em vez de reimprimir o prompt pra sempre
- ✅ **`ProcRun`**: executa um processo filho (sem shell) com callback
  síncrono por linha de stdout/stderr — consumo de NDJSON em streaming
  (ex.: CLI de um LLM) direto de dentro do AdvPL
- ✅ **`JsonObject:FromJson` é um parser real** (era stub que sempre
  devolvia Nil sem parsear nada): objetos aninhados viram `JsonObject`,
  arrays viram `Array` 1-based, `null`→Nil; `.F.` (não erro) em JSON
  inválido
- ✅ **Paridade Windows**: toda saída que pode conter ANSI passa por
  `stdoutW` (`go-colorable` no Windows, `os.Stdout` puro no
  Linux/macOS) — sem isso `cmd.exe`/PowerShell sem
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING` imprimiriam os escapes literalmente

### Notas de Implementação:
- `pkg/vm/ui_render.go`: as 7 natives de TUI (`registerUiRenderNatives`)
- `pkg/vm/natives.go`: `stdoutW`, `CONOUTRAW`, `PROCRUN`, `CONIN` (EOF→Nil)
- `pkg/vm/vm.go`: `VM.lastBoxLines`, `jsonToAdvplValue`,
  `JsonObject:FromJson` real
- Nova dependência: `github.com/charmbracelet/glamour` (+ `go-colorable`)
- Teste: `cmd/advplc/tui_natives_test.go` + fixture
  `tests/tui_natives_test.prw` — exercita as 7 natives de TUI, `ConOutRaw`,
  `ProcRun` (contra o próprio binário buildado, portátil entre SOs),
  `FromJson` (válido/inválido/aninhado) e o EOF de `ConIn`

## Executável Standalone — Console/TUI (atualizado 2026-07-29)

### Status Atual: Funcional e coberto por teste de integração

Até esta data, `advplc build` compilava e o binário resultante "rodava"
sem erro mesmo quando a interação real (login, menus, `FWMBrowse`) nunca
acontecia de fato — nenhum teste automatizado exercitava esse caminho, só
o smoke test 100%-console (`TestBuildStandaloneSmoke`, zero prompts). Os
dois problemas abaixo existiam desde a introdução do `build` e só foram
descobertos ao tentar usar um app de verdade (e-Gov) interativamente:

### O Que Funciona:
- ✅ **Console interativo real** (`pkg/ui/terminal.go`, `TerminalUIProvider`):
  `FWGetText`/`FWMenuSelect`/`Msg*` leem/escrevem no terminal de verdade
  quando stdin é um TTY — antes, sem nenhum `UIProvider` anexado no modo
  headless, esses natives nunca tocavam stdin e devolviam o default
  silenciosamente (login "passava direto" sem pedir nada)
- ✅ **TUI real via `huh`+`lipgloss`**: formulários com borda, seleção por
  seta, tema escuro fixo (sem depender de query OSC 11 ao terminal, que
  pode travar em ambientes que não respondem)
- ✅ **`FWMBrowse` funciona no console**, não só em `advplc serve` —
  listagem em tabela (largura adaptada ao terminal, com aviso de campos
  ocultos), Novo/Editar/Excluir com formulário multi-campo
- ✅ **Detecção console-vs-GUI corrigida**: a heurística antiga checava
  `bc.Classes`, que só registra classes *declaradas* (`class X from Y`) —
  nunca detectava `FWMBrowse():New()` (toda instanciação real de classe
  builtin). Trocada por varredura real do bytecode (`OP_CALL_NATIVE` para
  FWGetText/FWMenuSelect, `OP_NEW_INSTANCE` para FWMBrowse/MSDIALOG/etc)
- ✅ **`ADVPP_FORCE_GUI=1`**: força a janela Fyne mesmo rodando de um
  terminal — para um app cujo autor prefere GUI como padrão (ex.: e-Gov,
  GesCon), sem mudar o comportamento padrão (console-se-tem-TTY) de
  outros programas AdvPP
- ✅ Correção de bug correlato em `FWMBrowse`: o fallback de colunas sem
  entrada no SX3 expunha `R_E_C_N_O_`/`R_E_C_D_E_L_` (chave interna) como
  campo editável — um Novo/Editar sobrescrevia o próprio rowid e o
  INSERT/UPDATE seguinte falhava com "datatype mismatch"

### Notas de Implementação:
- `pkg/ui/terminal.go`: `TerminalUIProvider` (implementa `vm.UIProvider`
  e `vm.BrowseUI`)
- `pkg/compiler/stub_template.go`: decisão console-vs-GUI, `ADVPP_FORCE_GUI`
- `pkg/vm/browse.go`: exclusão de `R_E_C_N_O_`/`R_E_C_D_E_L_` do fallback
  sem SX3
- Teste de integração real (builda o binário, roda sob um pseudo-terminal
  de verdade, digita/navega/confirma exatamente como um humano):
  `cmd/advplc/build_standalone_interactive_test.go`
  (`TestBuildStandaloneInteractive`), fixture `tests/standalone_interactive_test.prw`
  — roda nos 3 SOs do CI (Linux/macOS via `creack/pty`, Windows via
  `github.com/UserExistsError/conpty`, ConPty real em Go puro sobre
  `golang.org/x/sys/windows`, sem cgo), abstraído em `ptysession_posix_test.go`
  / `ptysession_windows_test.go` — mesmo teste, mesmas asserções, cobertura
  real nas três plataformas que o release publica

**Adendo (2026-08-01):** `advplc build --gui` fixa a decisão console-vs-GUI
em tempo de compilação (a heurística acima descrita continua sendo o
comportamento padrão sem a flag). Diferença prática do `ADVPP_FORCE_GUI`
já documentado: `--gui` também linka o binário Windows no subsistema GUI
(`-H=windowsgui`, `pkg/compiler/standalone.go:goBuildArgs`), então nenhuma
janela de console aparece nem por um instante — `ADVPP_FORCE_GUI` sozinho
não muda o subsistema. Obrigatório para programas com `MSDIALOG`.

## Componentes UI

### Status Atual: Renderização Visual Implementada

O compilador AdvPP agora inclui renderização completa de widgets Fyne para todos os componentes UI.

### O Que Funciona:
- ✅ Estruturas de dados de componentes (TButton, TGet, TComboBox, TCheckBox, etc.)
- ✅ Estruturas de diálogo (Dialog, MenuBar, ToolBar, StatusBar)
- ✅ Propriedades de componentes (X, Y, Width, Height, Label, Value, etc.)
- ✅ Framework de tratamento de eventos (onChange, onClick, onGotFocus, onLostFocus)
- ✅ **Renderização de widgets Fyne para todos os componentes**
- ✅ **Renderização de TButton**
- ✅ **Renderização de TGet (entrada de texto)**
- ✅ **Renderização de TComboBox**
- ✅ **Renderização de TCheckBox**
- ✅ **Renderização de TLabel**
- ✅ **Renderização de MenuBar**
- ✅ **Renderização de ToolBar**
- ✅ **Renderização de StatusBar**
- ✅ **Renderização de view de formulário com conteúdo scrollable**
- ✅ Diálogos Fyne básicos (MsgInfo, MsgStop, MsgAlert, MsgYesNo)
- ✅ Componentes UI da IDE (CodeEditor, OutputConsole, FileTree)

### O Que NÃO Funciona:
- ❌ Execução de eventos de componentes (manipuladores definidos mas não conectados)
- ❌ Atualizações dinâmicas de componentes (sem two-way binding)

### Notas de Implementação:
- Componentes são definidos como structs Go em `pkg/mvc/view.go`
- Renderização Fyne implementada em `pkg/ui/renderer.go`
- Executável de teste visual: `./ui-test` (em `cmd/ui-test/`)
- Renderização completa de componentes agora funcional

## Recursos REST 2.0

### Status Atual: Funcional (anotações, DSL clássico WSRESTFUL e camada PO-UI)

O compilador AdvPP sobe um servidor HTTP REST **real** (`pkg/rest` + classe
nativa `WSRestServer`, mesmo padrão arquitetural do `MCPServer`) para o
estilo moderno TLPP de anotações (`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete`
sobre `User Function`) e para o DSL clássico `WSRESTFUL ... WSMETHOD ...
ENDWSRESTFUL`. Desde a onda 1 PO-UI, o `advplc serve` também monta essas
rotas sob `/rest` na mesma origem do modo web (`webui.Server.MountREST`),
e as rotas anotadas expõem o objeto `oRest` do TOTVS. Ver "Camada de
compatibilidade PO-UI" na seção de recursos executados.

### O Que Funciona:
- ✅ **Servidor HTTP real** (`net/http` puro, sem CGO/dependências): classe
  `WSRestServer` — `New()`, `AddRoute(verbo, path, funcao)`, `Serve(porta)`
- ✅ **Auto-discovery de rotas via anotação**: toda `User Function`
  anotada com `@Get("/path")`/`@Post(...)`/`@Put(...)`/`@Patch(...)`/
  `@Delete(...)` vira rota automaticamente ao criar o `WSRestServer`
- ✅ **Path params** (`/clientes/{id}`) via roteador nativo do Go 1.22+
  (`http.ServeMux`), populados no objeto de argumento da função AdvPL
- ✅ **Query string e corpo JSON** decodificados e mesclados no objeto
  de argumento (`oParam:CAMPO`) — path params têm precedência sobre o
  corpo JSON, que tem precedência sobre a query string (o path vem do
  roteamento e não pode ser falsificado pelo corpo)
- ✅ **Dispatch real para a função AdvPL** via VM isolada por requisição
  (mesmo mecanismo do `MCPServer`/`StartJob`: `v.RunFunction`), banco e
  bytecode compartilhados
- ✅ **Resposta JSON automática** do retorno da função (200), erro vira
  500, path inexistente vira 404, verbo não registrado num path existente
  vira 405
- ✅ Registro manual de rota via `AddRoute` (cobre casos onde a anotação
  não é usada)
- ✅ Sintaxe JSON inline, métodos JsonObject (toJson, hasProperty,
  getJsonText), serialização/deserialização JSON

### O Que NÃO Funciona (limitações conhecidas):
- ✅ **DSL clássico `WSRESTFUL`/`WSMETHOD` executando**: verbo+`PATH` de
  cada `WSMETHOD` (anotação `<VERBO>|<Nome>|<PATH>`) vira rota REST com
  dispatch por instância de classe em VM-filha isolada; `aURLParms`,
  `SetRestFault`, `HTTPSetStatus`, `GetContent` e `SetResponse`/
  `SetContentType`/`SetHeader`/`GetHeader`/`SetStatus` funcionam
  (`pkg/vm/wsrestful.go`). Registrado via `WSRestServer:New` e via
  `advplc serve` (`RegisterRestRoutes`)
- ❌ Geração de WSDL / cliente REST (`WSCLIENT` consumindo serviço
  externo) — fora de escopo, é consumo e não exposição de API

### Notas de Implementação:
- Servidor: `pkg/rest/rest.go` (stdlib `net/http`, roteador nativo do Go
  1.22+, sem dependências externas)
- Ponte VM: `pkg/vm/rest_native.go` (classe nativa `WSRestServer`,
  conversão JSON↔`advplrt.Value`, auto-discovery via
  `FunctionInfo.Annotations` do bytecode)
- Fixture de teste: `tests/rest_server_test.prw`; teste de integração
  Go (builda o binário, sobe o servidor, faz requisições HTTP reais):
  `cmd/advplc/rest_integration_test.go` (`TestRestServerFixture`)

## Construção de Serviços

### Status Atual: Parcial

### O Que Funciona:
- ✅ Parsing de sintaxe WSCLIENT/WSSTRUCT/WSRESTFUL
- ✅ Definições de protótipo WSMETHOD
- ✅ Definições de campos WSDATA
- ✅ Metadados de serviço (DESCRIPTION, NAMESPACE)
- ✅ Criação e manipulação de objetos JSON

### O Que NÃO Funciona:
- ❌ Geração de código WSDL
- ❌ Geração de código de cliente REST
- ❌ Invocação de serviço
- ✅ **Integração de cliente HTTP nativa**: `FWHttpGet`/`FWHttpPost`/`FWHttpPut`/`FWHttpPatch`/`FWHttpDelete` + `FWHttpBody`/`FWHttpStatus`/`FWHttpError` com suporte a certificado PKCS#12 (.pfx/.p12), timeout 30s, TLS com verificação. Testes de integração com servidor HTTP local. Adicionado v2.0.1.

## Banco de Dados e Multi-thread (atualizado 2026-07-08)

### Status Atual: Funcional

- ✅ **Banco SQLite compartilhado**: todas as ferramentas (advplc,
  adveditor, advpp-ide) resolvem o mesmo banco via
  `shared.ResolveDatabasePath` (flag → `ADVPP_DB` → config real em disco
  → `./advpp.db` local do diretório de trabalho, criado automaticamente)
- ✅ **Driver 100% Go** (modernc.org/sqlite, sem CGO) com WAL + busy_timeout
- ✅ **Natives de banco conectados ao VM**: DBSelectArea, DBSeek, DBSkip,
  RecCount, FieldGet/FieldPut etc. operam sobre o banco real
- ✅ **StartJob(cFunc, cEnv, lWait, params...)**: execução em VM isolado
  (goroutine), síncrona ou assíncrona, com conexão própria ao banco
- ✅ **FWGridProcess**: pool de threads com backpressure (SetThreadGrid,
  CallExecute, StopExecute, IsFinished, meters, SaveLog)
- ✅ **advplc check paralelo**: N arquivos com 1 worker por CPU
- ✅ **Renderer web (advplc serve)**: PO-UI embutido no binário; console,
  diálogos, FWMBrowse→po-table com dicionário SX3, po-dynamic-form,
  MSDIALOG legado por heurística de grade e hot reload (--watch)
- ✅ **Motor de inferência LLM (classe `LLM`)**: modelos GGUF I2_S
  (BitNet/Falcon3-1.58bit) via `pkg/llm`, 100% Go sem CGO, com kernel
  SIMD AVX2 em amd64 (fallback escalar em qualquer outra arquitetura),
  validado token a token contra o `llama.cpp` de referência. Desde
  v4.0.0, também decodifica pesos Q4_K/Q6_K e reconhece
  `general.architecture = "minicpm"` — validado executando o modelo
  real MiniCPM5-2B-Q4_K_M via `advplc run`, com saída batendo com o
  `llama.cpp` de referência
- ✅ **Conectividade real multi-provider (classe `DbConnection`, desde
  v4.0.0)**: `PostgreSQL`/`Oracle`/`SQL Server` via drivers 100% Go sem
  CGO (`pgx`, `go-ora`, `go-mssqldb`). `DBSetDriver("TOPCONN")` roteia
  `DBUseArea`/RDD pra conexão remota ativa (`RemoteSQLEngine`); SQL
  direto (`TCGenQry`/`TCSqlToArr`) já funciona assim que
  `DbConnection:Connect()` tem sucesso. `TCLINK`/SQLite local
  inalterados. Validado com round-trip real de CRUD contra um
  PostgreSQL real
- ✅ **Servidor MCP nativo (classe `MCPServer`)**: JSON-RPC 2.0 real
  sobre stdio via `pkg/mcp` (initialize/tools.list/tools.call), expõe
  funções AdvPL como tools — execução real; validado com o SDK oficial
  em Python do MCP
- ✅ **Servidor REST nativo (classe `WSRestServer`)**: HTTP real via
  `pkg/rest` (`net/http` puro), auto-discovery de rotas por anotação
  (`@Get`/`@Post`/`@Put`/`@Patch`/`@Delete`) ou registro manual via
  `AddRoute`, path params, dispatch para a função AdvPL via
  `v.RunFunction` — mesmo padrão do `MCPServer`; o DSL clássico
  `WSRESTFUL`/`WSMETHOD` executa via `pkg/vm/wsrestful.go` (ver
  "Recursos REST 2.0")
- ✅ **Camada de compatibilidade PO-UI (onda 1)**: `oRest` nas rotas
  anotadas, DSL clássico `WSRESTFUL`/`WSMETHOD` executando de verdade,
  filtro OData parametrizado, `TamSx3`, `FWAdapterBaseV2` e
  `FwApiManager` (somente leitura) — detalhes no plano
  `docs/superpowers/plans/2026-11-22-poui-onda1-rest.md`
- ✅ **Camada de compatibilidade PO-UI (onda 2)**: `FwCallApp` real com
  ponte `JsToAdvpl`/`AdvPLToJS` (`TWebChannel`), hosting de apps PO-UI
  externos (`advplc serve --poui-app <dir|.app> [fonte]`), endpoints de
  framework do `@totvs/protheus-lib-core` 21.1.4 (companies, branches,
  current-user, systemModules, session defaults, idiomas, marca,
  aliasAccess/functionAccess, genericList/genericQuery sobre SX3+SQLite;
  OAuth2/MFA respondem 501) — detalhes no plano
  `docs/superpowers/plans/2026-11-22-poui-plano-integral.md`. O app de
  teste `tests/poui/apps/fixture-app` (Angular + lib-core) valida o
  ciclo ponta a ponta via `tests/poui/ao-vivo.sh fixture-app`
- ✅ **`FwCallApp` — assinatura completa do corpus**: as 10 posições são
  aceitas; `oChannel` (posição 4) quando é `TWebChannel` é reutilizado
  como canal do `JsToAdvpl` (com `bJsToAdvpl`/`lConnected`/`nPort`
  preservados), `oEngine` (posição 2) quando é `TWebEngine` recebe a URL
  do app em `CURL`; `cSource` (posição 6) resolve a `JsToAdvpl` do fonte.
  `bJsToAdvpl` setado no canal tem precedência sobre a função. Demais
  posições são aceitas e ignoradas — o corpus 12.1.2510 só usa `cApp`
  na prática (a forma completa aparece comentada em VEIA372.prw).
- ✅ **`TWebChannel`/`TWebEngine` nativos** (dump binário do corpus
  12.1.2510): `New`/`Connect`/`Disconnect`/`AdvPLToJS`/`lConnected`/
  `nPort`/`bJsToAdvpl` no canal; `New(ownd,nrow,ncol,w,h,curl,nport)`/
  `Navigate`/`RunJavaScript`/`GoBack`/`GoForward`/`SetAsMain`/`SetHtml`/
  `ClearCache`/`Reload` no engine — o "navegador" do modo web é a aba
  PO-UI, então navegação é no-op com estado fiel (URL/lang em `Props`).
  `FwCallApp` fora do modo web (`advplc serve`) retorna o erro exato
  `FwCallApp: requer o modo web (advplc serve)`.
- ✅ **OAuth2 do framework**: `POST/GET /api/oauth2/v1/token` emite
  JWT `header.payload.` sem assinatura, com `userid`/`exp` (formato que
  `getTokenPayload()` do `@totvs/protheus-lib-core` decodifica). O REST
  não valida o token: a proteção é a checagem de origem/host.
- ⚠️ **MFA não suportado** (sessão local): `mfa/hasSecret` responde
  `{hasSecret:false}`, o que faz a lib-core pular o fluxo; `mfa/tok`
  responde `{tok:0}` e `mfa/TOTPURI` responde 501. Nenhum segredo é
  gerado nem gravado. (Uma versão anterior, não publicada, gerava TOTP e
  gravava `~/.advpp/mfa_secrets.json` sem que o REST exigisse o segundo
  fator; foi removida por decisão do operador em 2026-11-23.)
- ✅ **`FwApiManager:ToExecAuto` implementado**: fiel ao
  `FWAPIManager.prw` do corpus — `nTipo==1` anexa `{campo, valor, Nil}`
  por linha de `aDados` em `aRet`; demais tipos acumulam as triplas num
  `aItem` único anexado a `aRet` (transformador puro, não executa
  ExecAuto).
  `SetApiRelation` implementado no formato real do corpus (pares
  `{colFilho, colPai}` + aninhamento pai→filho no JSON).
- ✅ **Corpus TOTVS 12.1.2510 validado** (`ADVPP_CORPUS_2510`, raiz do
  corpus ou a pasta `fontes/`; medição de 2026-11-23): 0 falhas em 103
  fontes `FwCallApp`/`JsToAdvpl` e em 500 `WSRESTFUL` (amostra); 213 de
  214 `FWAdapterBaseV2` — o restante
  (`backoffice.pif.fiscal.insight.purchaseorderssuggested.data.tlpp`) é
  inválido no próprio corpus: o `/*{Protheus.doc}` da linha 899 não tem
  fechamento, o `/*/` da linha 957 o fecha (a mesma regra de 1.616
  comentários `/* ... /*/` do corpus) e o texto do Protheus.doc vira
  código. O teste exige que ele continue falhando. (A versão anterior
  deste item dizia "0 falhas em 110 e 225", números que não vieram de
  uma execução.) Parser/preprocessor/lexer estendidos
  para as formas reais (macros função-like, `[...]` repetidos, flags
  de GUI, cláusulas WS arbitrárias, nomes `1xxx`, `json` como
  identificador, `#define` quote-aware). `TestCorpusCincoFontesHTTP`
  executa 5 fontes reais de estilos distintos (WSRESTFUL clássico,
  oRest, controlador TLPP `@Get`, `FWAdapterBaseV2`+SQLite) — 5/5.
- ⚠️ **Locks de registro (RecLock/MsUnlock) são no-ops** — sem controle de
  concorrência em escrita entre processos

## Resumo

| Recurso | Status | Notas |
|---------|--------|-------|
| Componentes UI | ✅ Completo | Renderização Fyne completa implementada |
| Diálogos UI | ✅ Completo | MsgInfo, MsgStop, MsgAlert, MsgYesNo funcionam |
| Parsing REST | ✅ Completo | Sintaxe totalmente parseada |
| Execução REST (anotações @Get/@Post) | ✅ Funcional | Servidor HTTP real (`WSRestServer`), dispatch para a função AdvPL |
| Execução REST (DSL WSRESTFUL/WSMETHOD) | ✅ Funcional | Rotas por verbo+PATH, dispatch por classe em VM-filha |
| Anotações @Get/@Post/@Put/@Patch/@Delete | ✅ Executadas | Viram rota HTTP automaticamente |
| Suporte JSON | ✅ Completo | Sintaxe inline e JsonObject funcionam |
| Construção de Serviços | ⚠️ Parcial | Parseado, não gerado |

## Compatibilidade com IDE

### Status Atual: 100% Compatível

O compilador AdvPP e todos os componentes UI são totalmente compatíveis com a IDE AdvPP.

### Resultados de Testes
- ✅ Todos os arquivos de teste em `tests/*.prw` passam (`make test`)
- ✅ Componentes MVC funcionam no contexto da IDE
- ✅ Integração de provider UI funciona
- ✅ Saída do compilador com componentes UI funciona
- ✅ Execução da VM com renderização UI funciona
- ✅ Funções de diálogo (MsgInfo, MsgStop, MsgAlert, MsgYesNo) funcionam
- ✅ Suporte JSON funciona
- ✅ Funções nativas funcionam
- ✅ Estruturas de controle funcionam
- ✅ Arrays funcionam
- ✅ Funções de string funcionam

### Teste de Integração IDE
```bash
./advplc run tests/ide_integration_test.prw
```

Todos os testes passaram - 100% de compatibilidade com IDE verificada.

## Teste de Renderização UI

Para testar a renderização UI:
```bash
go build -o ui-test ./cmd/ui-test
./ui-test
```

Isso exibirá uma janela com:
- TLabel (título)
- TGet (entrada de texto)
- TComboBox (dropdown)
- TCheckBox (checkbox)
- TButton (botões)
- ToolBar (topo)
- StatusBar (fundo)

## Recomendações

1. **Para Componentes UI**: Implementar sistema de renderização de widgets Fyne ou documentar como apenas dados
2. **Para REST**: ~~DSL clássico `WSRESTFUL`/`WSMETHOD`~~ — implementado na onda 1 PO-UI (`pkg/vm/wsrestful.go`) e validado no corpus TOTVS 12.1.2510 (0 falhas em 500 fontes da amostra)
3. **Para Serviços**: Adicionar geração de código ou integração de cliente HTTP
4. **Documentação**: Atualizar README para separar claramente recursos "parseados" de "executados"
