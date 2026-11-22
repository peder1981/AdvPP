# Compatibilidade PO-UI no padrão TOTVS — design

Data: 2026-11-22. Substitui o não-objetivo "cobrir todo o catálogo PO-UI" da seção 3 de
`2026-11-20-poui-componentes-design.md` (o restante daquela spec segue valendo).

## 1. Intenção e porquê

O compilador e o interpretador chegaram a uma maturidade razoável, provada em três
projetos reais (GEBAN, GesCon, AdvOS). O próximo passo é deixar o AdvPP compatível com
o jeito como **os produtos TOTVS** usam PO-UI, para que fontes Protheus que dependem de
PO-UI rodem no AdvPP **sem alteração**.

Medição no corpus Protheus 12.1.2510 (~22 mil arquivos):

| Item | Arquivos | No AdvPP em 2026-11-22 |
|---|---|---|
| `WSRESTFUL` clássico / `@Get(` | 505 / 471 | clássico só é lido pelo parser, sem dispatch; `@Get` responde, mas sem `oRest` (`docs/COMPONENT_STATUS.md`, seção "Recursos REST 2.0") |
| objeto `oRest` | 40 dos 214 adapters | não existe |
| `hasNext` (contrato de lista PO-UI) | 562 | não existe |
| `FWAdapterBaseV2` | 214 | não existe |
| `FwCallApp` | 91 | esboço que não faz nada (`pkg/vm/natives.go:1445`) |
| `JsToAdvpl` (ponte JS ↔ AdvPL) | 58 | não existe |
| `FwApiManager` | 45 | não existe |
| classes `Po*` do AdvPP | 0 | 24 componentes |

Nos produtos TOTVS, a tela é um app Angular/PO-UI. O fonte AdvPL fornece o REST (quase
sempre com `FWAdapterBaseV2`) e abre o app com `FwCallApp`. As classes `Po*` do AdvPP
foram criadas por nós: são úteis, mas não trazem compatibilidade com TOTVS.

## 2. Objetivos

1. **Onda 1. REST no padrão TOTVS:**
   - o `WSRESTFUL` clássico (`WSMETHOD <verbo> ... PATH`) funcionando de ponta a ponta;
   - o objeto `oRest`;
   - `FWAdapterBaseV2` e `FwApiManager`;
   - o contrato de lista PO-UI (`items`, `hasNext`, `page`, `pageSize`, `filter`,
     `order`, `fields`).

   Um fonte real do corpus responde via HTTP igual ao Protheus (decisão do operador,
   2026-11-22).
2. **Onda 2. `FwCallApp` real:** abre um app PO-UI (`.app` ou pasta `dist/`) no
   navegador pelo `advplc serve`, com a ponte `JsToAdvpl` / `AdvPLToJS` e o contexto de
   sessão que a `@totvs/protheus-lib-core` espera. Um app PO-UI de terceiros, sem fonte
   AdvPL por trás, roda pelo mesmo mecanismo.
3. **Onda 3. Catálogo `Po*` completo,** dividido em subondas:
   - 3a: campos e templates;
   - 3b: layout e navegação;
   - 3c: serviços e diretivas.

Cada onda sai como release própria e é utilizável sozinha.

## 3. Não-objetivos

- Redistribuir apps `.app` TOTVS, ou fontes do corpus, no repositório ou no binário. O
  corpus serve só para validar, localmente. A única exceção, aprovada pelo operador em
  2026-11-22, é a `@totvs/protheus-lib-core` compilada dentro do `dist` do app de teste
  (seção 10).
- Paridade em modo desktop (Fyne) ou console. Tudo isto é só do modo web, como na spec
  de 20/11.
- Os componentes em desenvolvimento ou obsoletos no PO-UI: `po-grid` (em
  desenvolvimento) e `po-gauge` (obsoleto, sai na v22). O `PoGauge` atual continua, mas
  sem evolução.
- Autenticação real de usuários Protheus (MFA, OAuth2 contra um AppServer). O AdvPP
  emula a sessão local.
- Acesso remoto multiusuário além do que `ADVPP_WEBUI_ALLOWED_HOSTS` já permite.

## 4. Anti-objetivos (seria falha mesmo funcionando)

- Quebrar programas existentes. `FWMBrowse`, MSDIALOG, `Msg*`, `PoApp` e `WSRESTFUL`
  continuam idênticos, e todos os testes atuais passam.
- Injeção de SQL pelo `filter`/`order`/`fields` da URL: só campos mapeados em
  `AddMapFields` chegam ao SQL, e os valores vão sempre como parâmetro.
- Lógica de um app específico em TypeScript no frontend do AdvPP.
- Qualquer marcação, identificador de origem ou telemetria nos apps servidos (Lei 2).
  O que o shim da onda 2 injeta é estritamente funcional. Está descrito por completo na
  seção 7.4.
- Quebrar o job `determinism` da CI.

## 5. Restrições

- Go sem CGO. Testes cross-platform (`docs/testing-cross-platform.md`): Linux, Windows
  e macOS verdes na matriz da CI antes de declarar pronto.
- `master` protegido: tudo via PR. CHANGELOG em cada release.
- Configuração nova entra em `shared.Config` (`~/.advpp/advpp_config.json`).
- Dependência nova exige aprovação do operador (ver seção 10).
- Fontes de teste `.prw`/`.tlpp` em CP-1252.

## 6. Abordagens consideradas

- **A (escolhida pelo operador): hospedar no navegador via `advplc serve`.** O
  `FwCallApp` serve o app na mesma origem do REST e emula a ponte. Reaproveita o
  servidor web, o SSE e a checagem de origem que já existem. Não usa CGO nem
  dependência nova de runtime. Apps externos saem de graça.
- **B: janela desktop com webview.** Exige CGO, o que viola a restrição do projeto.
  Descartada.
- **C: só o REST, com o app rodando por fora (`ng serve` + proxy).** É mais barata, mas
  o `FwCallApp` continua sem fazer nada e não cumpre "rodar o fonte TOTVS sem mudar".
  Descartada.

## 7. Design

### 7.1 Onda 1: REST clássico, `oRest` e `FWAdapterBaseV2`

**REST clássico (o maior bloco da onda).** Hoje o `parseWSClient`
(`pkg/parser/parser.go`) descarta o verbo e o `PATH`, e o dispatch
(`v.RunFunction`) só chama funções top-level.

- O parser passa a guardar, em cada `WSMETHOD`, o verbo, o `PATH` (com segmentos
  `{param}`), o `WSSYNTAX` e o `DESCRIPTION`.
- No dispatch, cada requisição cria uma instância do serviço e chama o método.
- Ficam disponíveis no método:
  - `::aURLParms` / `Self:aQueryString`;
  - `::GetContent()`;
  - `::SetResponse()`, `::SetContentType()`, `::SetStatus()`;
  - `SetRestFault()`.

  Os nomes exatos, e quais são obrigatórios, vêm do levantamento no corpus pelo mesmo
  método usado para o adapter.
- **`oRest`:** o objeto global do TLPP REST (`oRest:getQueryRequest()`,
  `oRest:setResponse()`, `oRest:getBodyRequest()`, `oRest:setStatusCode()` e o que mais
  o corpus usar) passa a existir nos métodos `@Get` etc. O formato atual (um objeto
  JSON de parâmetros) segue funcionando.

**Que fontes viram rotas (decisão do autor).** No Protheus, todo fonte compilado no RPO
expõe suas rotas. No AdvPP:

- `advplc serve <diretório>` carrega todos os `.prw`/`.tlpp` do diretório numa VM, como
  um RPO.
- `advplc serve <fonte>` carrega só aquele fonte.
- Em ambos os casos, o REST sobe **na mesma porta e origem do `advplc serve`**, em
  `/rest/...`, com o prefixo configurável em `shared.Config.rest_prefix`.
- O `WSRestServer():Serve(porta)` atual continua igual.
- Herança entre classes de arquivos diferentes (`Class X FROM Y`) precisa funcionar
  nesse carregamento.

**Dados (decisão do autor).**
- Validação no SQLite local auto-provisionado. Os outros bancos do AdvPP ficam fora do
  critério de pronto.
- `TamSx3` passa a existir, lendo o SX3 do dicionário local.
- Para o teste, cada fonte validado ganha um script de fixture com as tabelas e os
  registros mínimos.

**`FWAdapterBaseV2`.**

Classe nativa em Go, no padrão de `FWMBrowse`/`WSRestServer`. Pode ser herdada por
classe AdvPL/TLPP (`Class X FROM FWAdapterBaseV2`, `_Super:New(cVerbo, lList)`).

Métodos, na ordem de frequência no corpus (todos obrigatórios):

| Método | Ocorrências | Método | Ocorrências |
|---|---|---|---|
| `AddMapFields` | 2052 | `SetPage` | 45 |
| `GetJsonResponse` | 74 | `GetErrorMessage` | 42 |
| `SetQuery` | 53 | `SetFields` | 25 |
| `SetOrder` | 50 | `GetFixQuery` | 25 |
| `SetWhere` | 47 | `SetUrlFilter` | 15 |
| `SetPageSize` | 47 | `HasErrorMessage` | 15 |
| `Execute` | 47 | `SetOrderQuery` | 14 |
| `FillGetResponse` | 47 | `New(cVerbo, lList)` | — |

O corpus mostra ainda `SetString`, `SetValue`, `SetProp` e `GetModel`. Pela forma de
uso, devem ser do JSON/modelo, e não do adapter. Quem implementa confirma.

Comportamento:
- `SetQuery` recebe SQL com os marcadores `#QueryFields#` e `#QueryWhere#`.
  - `#QueryFields#` vira a lista de colunas mapeadas, restrita a `fields` quando a URL
    pedir.
  - `#QueryWhere#` vira `SetWhere` + o filtro da URL (`SetUrlFilter`, sintaxe
    OData-like: `campo eq 'x'`, `and`/`or`, parênteses), traduzido apenas sobre
    propriedades mapeadas. Os valores vão sempre como parâmetro (`?`).
- A paginação usa `SetPage`/`SetPageSize`, com padrão page=1 e pageSize=10. A ordem vem
  de `SetOrder` ou do `order` da URL (prefixo `-` = descendente), de novo só sobre
  campos mapeados.
- `FillGetResponse` monta `{"items":[...], "hasNext":bool}`. As propriedades seguem o
  nome do mapa, e o tipo vem do 5º argumento de `AddMapFields`
  (`{campo, tipo, tamanho, decimais}`, de `TamSx3`).
- Erros vão para `GetErrorMessage`/`HasErrorMessage`, sem pânico.

**`FwApiManager`:** a API vem das 45 ocorrências no corpus, com o mesmo método de
levantamento.

**Contrato de lista também no `WSRESTFUL` atual:** os parâmetros `page`, `pageSize`,
`filter`, `order` e `fields` precisam estar acessíveis ao método
(`GetQueryRequest`/`aQueryString`), como no Protheus.

**Validação:**
1. `advplc check` em todos os 214 arquivos do corpus que usam o adapter: 0 erros.
2. Pelo menos 5 fontes reais do corpus (de módulos diferentes, um deles com `oRest` e
   um com `WSRESTFUL` clássico) respondendo via HTTP contra o SQLite com fixture. A
   resposta é comparada ao contrato documentado no TDN e no código do fonte.
3. Testes de injeção (`filter` com aspas, `;`, `--`, campo não mapeado). Campo não
   mapeado devolve 400.

### 7.2 Onda 2: `FwCallApp`

Assinatura vista no corpus:
`FwCallApp(cApp, oOwner, oEngine, oChannel, cHost, cSource, ...)`. O 6º parâmetro
(`cSource`) indica o fonte onde está a `Static Function JsToAdvpl(oWebChannel, cType,
cContent)`. Sem ele, vale o fonte que chamou.

Fluxo:
1. **Localizar o app.** Procurar `<cApp>.app` (zip) ou a pasta `<cApp>/` (com
   `index.html`):
   - primeiro, ao lado do fonte;
   - depois, nos diretórios de `shared.Config.poui_apps_dirs`.

   Nome validado pela regra de `NomeDeAppValido` (`pkg/webui/apps.go`). O zip é lido em
   memória, com limite de tamanho, e nunca extraído para disco. Conteúdo sem
   `index.html`, ou com caminho de entrada contendo `..`, é rejeitado.
2. **Servir** em `/poui/<cApp>/` (o `/app/` já é do `--apps`) na mesma origem do REST (`/api/...` e as rotas
   `WSRESTFUL`), com a checagem de origem atual. O `index.html` recebe a injeção do
   shim (7.4).
3. **Bloquear como no Protheus.** O app abre numa **aba nova** (`window.open` disparado
   pela SPA da sessão), e a SPA continua viva na aba original. A URL leva o `sid` da
   sessão chamadora, e o shim o usa no `POST /ui` e no `EventSource`. `FwCallApp` só
   retorna quando:
   - o app manda `close` (`ProJsToAdvplService.AdvplCloseApp`);
   - o `EventSource` da aba do app fica desconectado por mais de 10 s (aba fechada);
   - ou a sessão chamadora encerra.

   (Decisão do autor.)
   Enquanto espera, cada `jsToAdvpl(type, content)` vindo do app chama a `JsToAdvpl` do
   fonte, na VM da sessão (mesmo laço do `PoApp:Activate`).
4. **`oWebChannel:AdvPLToJS(cType, cContent)`** envia um evento SSE que o shim
   transforma em `twebchannel.eventTarget.dispatchEvent(new CustomEvent(cType,
   {detail: cContent}))`. O formato exato do evento é o que a lib espera; quem
   implementa confirma lendo a lib.

**Apps externos:** `advplc serve --poui-app <dir|arquivo.app> [fontes REST...]` serve o
app, sobe os fontes REST indicados e usa o mesmo shim, sem fonte chamador (os
`jsToAdvpl` são ignorados; o `close` encerra).

### 7.3 Endpoints de framework que a `protheus-lib-core` chama

Levantados da `@totvs/protheus-lib-core` 21.1.4. O AdvPP responde a partir do estado
local (`pkg/vm/filial.go`, usuário e idioma da sessão):

- `/api/framework/environment/v1/companies` e `/branches`
- `/api/framework/current-user` e `/api/framework/v1/users`
- `/api/framework/v1/systemModules`
- `/api/protheus/session/v1/defaults`
- `/api/framework/v1/FwRestTranslate/`
- `/api/language/v1/protheus-suported-languages`
- `/api/brand/v1/erp-brand`
- `/api/framework/v1/accessPrivilegesServices/{aliasAccess,functionAccess}/` (o AdvPP
  responde sempre "permitido")
- `/api/framework/v1/genericList` e `/genericQuery` (sobre o SQLite local, com a mesma
  regra de campos mapeados e parâmetros)

O conjunto mínimo é o que um app de teste exige na inicialização. Os demais entram sob
demanda, documentados no `COMPONENT_STATUS.md`. `/api/oauth2/v1/token` e `/mfa/*` ficam
fora (ver não-objetivos): respondem 501 com mensagem clara.

### 7.4 O shim injetado (divulgação completa, Lei 2)

Um `<script>` inline inserido antes do primeiro `<script>` do `index.html` servido. O
arquivo do app em disco ou no zip nunca é alterado. O shim:

1. Define `window.twebchannel = { jsToAdvpl(type, content), eventTarget: new
   EventTarget() }`. O `jsToAdvpl` faz `POST /ui` com `{app, type, content}`, e um
   `EventSource` alimenta o `eventTarget`.
2. Preenche o `sessionStorage` com as chaves que a lib lê:
   - `ProCompany`, `ProBranch`, `ProModule`, `appName`, `ProSystemModule`,
     `ProDatabase`;
   - `RemoteType`, `SocketPort`, `ProIdiom`, `ProRole`, `ProUser`;
   - `ERPAPPCONFIG`, com `api_baseUrl` = a origem local do `advplc serve`;
   - `ERPTOKEN`.
3. `ERPTOKEN` é um valor aleatório por sessão (`crypto/rand`, 32 bytes em hex), só
   porque a lib exige a chave preenchida e a envia como `Authorization: Bearer`.
   - O REST do AdvPP **não valida** esse token. A proteção continua sendo a checagem de
     origem e de host que já existe, então nenhum programa `WSRestServer` atual muda
     (decisão do autor).
   - Nunca sai da máquina, não é gravado em disco e não identifica pessoa, modelo nem
     origem.

Nenhum outro dado é injetado. O conteúdo literal do shim fica versionado num único
arquivo Go (`pkg/webui/poui_shim.js`, via embed), auditável.

### 7.5 Onda 3: catálogo `Po*`

Mesmo padrão das classes atuais (`pkg/vm/poui*.go` + mapeamento `type` → componente em
`web/src/app/ui-host.ts`). Nomes `Po<Componente>` como no `llms.txt`.

- **3a. Campos e templates:**
  - campos: `PoCombo`, `PoMultiselect`, `PoLookup` (sobre REST com o contrato da onda
    1), `PoRadioGroup`, `PoCheckboxGroup`, `PoDatepicker`, `PoDatepickerRange`,
    `PoDatetimepicker`, `PoTimepicker`, `PoDecimal`, `PoEmail`, `PoUrl`, `PoUpload`,
    `PoRichText`, `PoSearch`;
  - templates: `PoPageList`, `PoPageEdit`, `PoPageDetail`, `PoPageDynamicTable`,
    `PoPageDynamicEdit`, `PoPageDynamicDetail` e `PoPageDynamicSearch`. Os dynamic
    apontam para um endpoint REST (contrato da onda 1).
- **3b. Layout e navegação:** `PoAccordion`, `PoBreadcrumb`, `PoButtonGroup`,
  `PoDropdown`, `PoPopup`, `PoPopover`, `PoContextMenu`, `PoContextTabs`, `PoToolbar`,
  `PoNavbar`, `PoHeader`, `PoMenuPanel`, `PoStepper`, `PoSlide`, `PoCalendar`,
  `PoAvatar`, `PoBadge`, `PoImage`, `PoLink`, `PoSkeleton`, `PoHelper`, `PoToaster`,
  `PoDisclaimerGroup`, `PoFilterChip`, `PoDynamicView`, `PoModal`, `PoPageLogin`,
  `PoPageSlide`, `PoPageChangePassword`, `PoPageBlockedUser` e `PoPageJobScheduler`.
- **3c. Serviços e diretivas:**
  - `PoApp:SetTheme` (PoThemeService), i18n, storage (`localStorage` do navegador),
    `PoNotification` completo, media-query, user-guide e tooltip;
  - arrastar e soltar, como evento `drop` num container.
  - `PoSyncService`, `PoNetworkService` e o `po-search-ai` ficam para avaliação ao fim
    da 3c (offline-sync e IA dependem de backend próprio).

Cada componente leva teste Go (operações mount/patch) e uma fixture
`tests/poui/*.prw` com verificação ao vivo, como na fase 1.

## 8. Entregas

| Release | Conteúdo | Pronto quando |
|---|---|---|
| Onda 1 | `WSRESTFUL` clássico, `oRest`, `FWAdapterBaseV2`, `FwApiManager`, `TamSx3`, contrato de lista, REST no `advplc serve` | 214 adapters + amostra de 500 `WSRESTFUL` do corpus com `check` = 0 erros; os 5 fontes reais da seção 7.1 respondendo via HTTP; testes de injeção verdes; CI em 3 SOs verde |
| Onda 2 | `FwCallApp`, ponte, shim, endpoints de framework, `--poui-app` | app de teste com `protheus-lib-core` + `po-page-dynamic-table` lista dados de um adapter da onda 1, e `jsToAdvpl`/`AdvPLToJS`/`close` funcionam |
| 3a / 3b / 3c | catálogo `Po*` | cada componente com teste e fixture ao vivo |

## 9. Decisões do autor do design (corrigíveis)

- O `FWAdapterBaseV2` como classe nativa em Go, não em TLPP com include, pelo mesmo
  motivo da spec de 20/11.
- O shim é injetado na resposta HTTP, não gravado no app, para nunca alterar o artefato
  do usuário.
- Ordem 1 → 2 → 3: a onda 2 depende do contrato da 1, e a 3a (templates dynamic) também.
- `po-grid` e `po-gauge` ficam fora, pelo estado deles no próprio PO-UI.
- `advplc serve <diretório>` como "RPO", com REST na mesma porta, em `/rest/`.
- O app do `FwCallApp` abre em aba nova, ligada pelo `sid`. A rota é `/poui/<cApp>/`.
- `ERPTOKEN` não é validado: a checagem de origem atual é a proteção.
- Validação só no SQLite, com fixtures de dados por fonte.

## 10. Dependências

- Runtime: nenhuma nova.
- Teste da onda 2: um app fixture Angular com `@po-ui/ng-components` (já usado) e
  `@totvs/protheus-lib-core` (**aprovada pelo operador em 2026-11-22**, só no fixture
  de teste). Os fontes ficam em `tests/poui/fixture-app/`, e o `dist` compilado também é
  versionado (aprovado pelo operador). O pacote não declara licença no npm.

## 11. Para quem implementa

- Assinatura exata e semântica dos 3º e 4º argumentos de `AddMapFields` (provavelmente
  "filtrável" e "ordenável"): confirmar no TDN ou nos fontes do corpus.
- API do `FwApiManager`.
- Formato exato do evento AdvPL→JS que a lib escuta (linhas ~100-130 do
  `fesm2022/totvs-protheus-lib-core.mjs`).
- Mapeamento interno `type` → componente e nomes dos métodos de cada `Po*` novo,
  seguindo os `@Input`/`@Output` do PO-UI.
