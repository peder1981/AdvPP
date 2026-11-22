# PO-UI no padrão TOTVS — Plano de execução integral (ondas 1, 2, 3a, 3b, 3c)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (escolha do operador em 2026-11-22). Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** `docs/superpowers/specs/2026-11-22-poui-compat-totvs-design.md` (aprovada em
2026-11-22).

**Plano detalhado da onda 1:** `docs/superpowers/plans/2026-11-22-poui-onda1-rest.md`. É
a fonte executável da onda 1; a seção 2 abaixo só resume seu escopo e seus gates. As
ondas 2 e 3 estão especificadas aqui por completo. Antes de cada onda começar, o
detalhamento dela é revisado contra o que as ondas anteriores entregaram de fato.
Nomes e assinaturas podem mudar só se a onda anterior tiver mudado a interface que
elas consomem, e qualquer mudança é registrada no próprio plano.

**Ordem de execução adotada:** uma onda por vez, sempre em branch próprio, com revisão
final antes de abrir a próxima. O plano integral é um mapa de programa; cada onda só
começa depois que a anterior tiver aceite técnico e checkpoint do operador.

---

## 0. Como este plano é executado

### 0.0 Gates obrigatórios

#### Gate 0 — preparação de cada onda

Antes de criar a branch ou despachar qualquer subagente:

1. atualizar `master` e confirmar que não há mudanças locais inesperadas;
2. criar o branch da onda a partir de `master` atualizado;
3. versionar os documentos de plano/spec necessários, sem misturá-los aos commits de
   implementação;
4. executar baseline `go test ./...`;
5. executar os builds Linux, Windows e macOS do `advplc`;
6. verificar `ADVPP_CORPUS_2510` quando a onda exigir validação de corpus;
7. revisar as tasks da onda contra o código realmente entregue pelas ondas anteriores;
8. registrar neste plano qualquer ajuste de interface, escopo ou dependência.

Se qualquer baseline falhar, a execução para até a causa ser isolada. Não se corrige
uma regressão preexistente dentro de uma task funcional sem autorização explícita.

#### Gate 1 — encerramento de cada onda

Uma onda só está pronta para revisão de merge quando:

- todas as tasks da onda estiverem concluídas ou explicitamente adiadas com motivo;
- `go test ./...` estiver verde;
- `go test -race` estiver verde nos pacotes afetados por concorrência;
- builds Linux, Windows e macOS estiverem verdes;
- o job `determinism` estiver verde localmente ou houver evidência equivalente antes
  do push;
- `make web`, quando a onda tocar o frontend, estiver executado e `pkg/webui/dist`
  versionado;
- documentação e changelog estiverem atualizados;
- revisão independente do branch inteiro estiver aprovada;
- o operador tiver autorizado push, PR ou release, quando aplicável.

### 0.1 Fluxo por onda

| Passo | O que acontece | Quem |
|---|---|---|
| 1 | Branch próprio a partir de `master` atualizado (`feat/poui-onda1-rest`, `feat/poui-onda2-fwcallapp`, `feat/poui-onda3a-campos`, `feat/poui-onda3b-layout`, `feat/poui-onda3c-servicos`) | orquestrador |
| 2 | Para cada task, em ordem: subagente implementador (com o texto da task + Global Constraints + Interfaces das tasks de que depende) | subagente novo por task |
| 3 | Revisão da task por subagente revisor novo (spec + diff + testes). Reprovou: volta ao implementador com os achados. Aprovou: próxima task | subagente novo por revisão |
| 4 | Fim da onda: revisão do branch inteiro (corretude, segurança, compatibilidade) | subagente novo |
| 5 | Push do branch e CI verde nos 3 SOs e no job `determinism`, somente após autorização | orquestrador |
| 6 | PR para `master` (sem linhas de atribuição), somente após autorização | orquestrador, **só com pedido do operador** |
| 7 | Release com a skill `advpp-release` (compilador + extensão VS Code + deploy) | orquestrador, **só com pedido do operador** |

**Paradas obrigatórias para o operador:**
- antes do primeiro push de cada onda;
- antes de cada PR e de cada release;
- sempre que uma task exigir dependência nova, decisão fora da spec, ou mexer em
  arquivo fora da lista da task.

### 0.2 Regras globais (valem para toda task de toda onda)

- **Compatibilidade total.** `MsgInfo`/`MsgYesNo`/`FWMenuSelect`/`FWGetText`/MSDIALOG/
  `FWMBrowse`/`PoApp` e os 24 `Po*` atuais, `WSRestServer` e `@Get` com objeto de
  parâmetros continuam idênticos. `go test ./...` verde ao fim de toda task.
- **Executáveis para Linux, Windows e macOS.** `GOOS=windows GOARCH=amd64
  CGO_ENABLED=0 go build -o "$(mktemp -d)/x" ./cmd/advplc` e o equivalente
  `darwin/arm64` compilam em toda task. Nunca sobrescrever `./advplc`/`./advplc.exe`.
- **Bytecode determinístico.** O job `determinism` fica verde, sem mudança no
  `pkg/compiler/serializer.go`.
- **Go sem CGO.** Nenhuma dependência nova de runtime. A única dependência nova
  aprovada é `@totvs/protheus-lib-core`, e só no app de teste da onda 2.
- **Configuração nova** só em `shared.Config` (`pkg/tools/shared/config.go`).
- **Lei 2: nada de marcação.** Nenhum identificador de origem, telemetria ou
  comentário-carimbo em código, nos apps servidos ou em commits. O único dado injetado
  nos apps é o shim da onda 2, descrito na seção 3.2.
- **Corpus TOTVS só para validar, localmente** (`ADVPP_CORPUS_2510`; sem ele,
  `t.Skip`). Nenhum fonte TOTVS entra no repositório.
- **Fontes de teste** `.prw`/`.tlpp` em CP-1252. Textos, comentários e commits em
  português.
- **Frontend genérico.** Nenhum código específico de um app em `web/`. `make web`
  regenera `pkg/webui/dist`, que é versionado.
- **Segurança.** Toda rota nova passa por `allowedHost`/checagem de origem, com limite
  de corpo de 8 MB. SQL só parametrizado e só sobre campos mapeados. Arquivo servido
  nunca sai do diretório ou zip do app.

### 0.3 Mapa de dependências

```
Onda 1 (REST) ──► Onda 2 (FwCallApp) ──► 3a (campos + templates dynamic)
                                   └────► 3b (layout/navegação) ──► 3c (serviços/diretivas)
```

A 3b depende só da fase 1 do `PoApp`, que já existe. Pode ser antecipada se o operador
quiser.

---

## 1. Inventário do que já existe (ponto de partida, 2026-11-22)

| Área | Estado |
|---|---|
| `pkg/rest` | servidor `net/http`, `Route{Method,Path,Handler(map)}`, `{param}`, `RawResponse` |
| `WSRestServer` (`pkg/vm/rest_native.go`) | `New/AddRoute/Serve/Shutdown`; `@Get` etc. → rota; cada requisição usa `newChildVM` |
| `WSRESTFUL` clássico | só parser; verbo e `PATH` descartados; sem dispatch |
| `_Super:` | não funciona ("cannot call method New on non-object") |
| Compilação de vários fontes | inexistente (um fonte = um bytecode) |
| `FwCallApp` | esboço que devolve `Nil` (`pkg/vm/natives.go:1445`) |
| `advplc serve` | SPA PO-UI embutida; SSE `/events`; `POST /reply`, `POST /ui`; `/app/{nome}` (`--apps`) |
| Classes `Po*` | 24 + `AdvCodeEditor`/`AdvTerminal`. Registro em `poClasses` (`pkg/vm/poui_native.go:12`); construtor `PoX():New(oPai, ...)`; genéricos `Set(cProp,x)`/`Get(cProp)`; frontend `@switch (node.type)` em `web/src/app/ui-host.ts` |
| SQLite local | auto-provisionado, com `SX3` (`X3_CAMPO, X3_TITULO, X3_TIPO, X3_TAMANHO, X3_DECIMAL, X3_ARQUIVO, X3_ORDEM`) |
| PO-UI no frontend | `@po-ui/ng-components` 21.23, Angular 21, zone.js obrigatório |

---

## 2. Onda 1 — REST no padrão TOTVS (resumo; detalhe no plano próprio)

**Branch:** `feat/poui-onda1-rest`.

| Task | Entrega | Testes-chave |
|---|---|---|
| 1 | `_Super:` (`OP_CALL_SUPER`), herança de classe nativa (`rootNativeClass`, `registerNativeBase`) | `TestSuperChamaMetodoDoPai`, `TestSuperNativoRegistrado` |
| 2 | `CompileProject` (Static por arquivo, `STATIC:<ARQ>:<NOME>`), `advplc serve <diretório>` | `TestProjetoStaticPorArquivo`, `TestProjetoHerancaEntreArquivos` |
| 3 | Parser: verbo/`PATH` de `WSMETHOD` em `ClassInfo.Annotations`; método sem nome = verbo | `TestWSRestfulGuardaVerboEPath` |
| 4 | `pkg/rest`: `Request`/`Response`, `Route.Raw`, `:param`, `AddRouteE` sem pânico, `Handler()` | `TestAddRouteConflitoNaoEntraEmPanico` |
| 5 | Dispatch do `WSRESTFUL` clássico (WSDATA, `aURLParms`, `SetResponse`, `SetRestFault`...) | `TestWSRestConcorrenteIsolado` (-race) |
| 6 | `oRest` em `@Get`; `@Get(endpoint=...)` | `TestORestQueryEPath` |
| 7 | Filtro OData parametrizado, `TamSx3`, `FWAdapterBaseV2` | `TestFiltroHostil`, `TestAdapterListaPaginada` |
| 8 | `FwApiManager` (leitura; `ToExecAuto` = erro claro de "não suportado") | `TestApiManagerListaComFiltro` |
| 9 | REST montado no `advplc serve` em `rest_prefix` (padrão `/rest`) | `TestServeRestRespeitaHost` |
| 10 | Validação no corpus, condicionada à disponibilidade de `ADVPP_CORPUS_2510`; sem corpus, fica pendente com `SKIP` | `TestCorpus*` (local) |
| 11 | `COMPONENT_STATUS.md`, `CHANGELOG.md`, revisão do branch; PR somente com pedido do operador | — |

**Aceite da onda 1:** Tasks 1–9 e 11 concluídas; Task 10 verde localmente quando o
corpus estiver disponível ou formalmente adiada quando não estiver; CI verde nos 3
SOs e no job `determinism`; revisão do branch aprovada. A ausência do corpus não
impede o merge técnico das Tasks 1–9 e 11, mas impede declarar a paridade completa da
onda 1 até a validação ser executada.

**Interfaces que a onda 2 consome:**
- `rest.Server.Handler()`, `rest.Route.Raw`, `rest.NormalizaPath`;
- `webui.Server.MountREST(prefixo, h)` e `shared.Config.RestPrefix`;
- `compiler.CompileProject` e o nome qualificado `STATIC:<ARQ>:<NOME>`;
- `vm.TraduzFiltro`/`TraduzOrdem`/`TraduzCampos` (para `genericList`/`genericQuery`).

---

## 3. Onda 2 — `FwCallApp` real, ponte JS ↔ AdvPL e apps externos

**Branch:** `feat/poui-onda2-fwcallapp`.

**Goal:** `FwCallApp("app")` num fonte rodando em `advplc serve` abre o app PO-UI numa
aba nova. O app conversa com o AdvPL pela `@totvs/protheus-lib-core` sem alteração. Um
app PO-UI de terceiros roda via `advplc serve --poui-app`.

### 3.1 Fatos levantados da `@totvs/protheus-lib-core` 21.1.4 (`fesm2022/totvs-protheus-lib-core.mjs`)

- **JS→AdvPL.** A lib chama `twebchannel.jsToAdvpl(type, content)` (fallback
  `dialog.jsToAdvpl`). `close` é o tipo de encerramento (`AdvplCloseApp`), com
  `content` `''` ou `'force'`.
- **Conexão.** `protheusConnected()` exige `twebchannel.gotConnection === true`.
  Enquanto for falso, `connectedJsToAdvpl` tenta de novo 99 vezes a cada 50 ms.
- **AdvPL→JS.** A lib registra
  `twebchannel.eventTarget.addEventListener(id, cb)`. O callback recebe um objeto com
  `{protheusResponse, subscriber}`. Em `buildObservable`, o `subscriber` fica em
  `twebchannel[id].subscriber`, e o `id` pode ser `"<type>-<idAleatório>"`, enviado
  como `content` no `jsToAdvpl`.
- **Contexto da sessão.** A lib lê do `sessionStorage`:
  - `ProCompany`, `ProBranch`, `ProAppConfig`, `ERPAPPCONFIG` (JSON);
  - `ProModule`, `appName`, `ProSystemModule`, `ProDatabase`, `ProIdiom`, `ProRole`,
    `ProUser`;
  - `RemoteType`, `SocketPort` (inteiros);
  - `ERPTOKEN`, que é lido e depois removido;
  - e do `localStorage`, `ProTheme` (JSON).
- **Configuração do app.** A lib lê `assets/data/appConfig.json`, mas não sobrescreve
  `ProAppConfig` se ele já existir no `sessionStorage`. A URL do REST é `api_baseUrl`
  ou `serverBackend + restEntryPoint`.
- **Endpoints que a lib chama:** ver a Task 2.5.

### 3.2 O shim (texto da divulgação exigida pela Lei 2; vai para o `COMPONENT_STATUS.md`)

`pkg/webui/poui_shim.js` (embutido via `embed`) é inserido como `<script>` inline logo
antes do primeiro `<script>` do `index.html` servido do app. O arquivo do app em disco
ou no zip nunca é alterado. Conteúdo funcional, completo:

1. Lê `sid` e `app` de `location.search` (`?sid=<id>&app=<nome>`).
2. Define `window.twebchannel`:
   - `gotConnection` é `false` até o `EventSource` abrir, e então vira `true`;
   - `eventTarget` é um `new EventTarget()`;
   - `jsToAdvpl(type, content)` faz `POST /poui-ui?sid=...` com corpo
     `{"app","type","content"}`.
3. Abre `EventSource('/events?s=<sid>&canal=poui')`. A cada evento `advpltojs`
   `{type, content}`, cria `const e = new Event(type); e.protheusResponse = content;
   e.subscriber = twebchannel[type]?.subscriber;` e dispara no `eventTarget`.
4. Preenche o `sessionStorage`. Os valores vêm de `GET /poui-ctx?sid=...` (síncrono no
   carregamento, via `XMLHttpRequest` síncrono para estar pronto antes do Angular):
   - `ProCompany`, `ProBranch`, `ProModule`, `appName`, `ProSystemModule`,
     `ProDatabase`, `ProIdiom`, `ProRole`, `ProUser`, `RemoteType`=`1`,
     `SocketPort`=`0`;
   - `ERPAPPCONFIG` e `ProAppConfig` = `{"name":<app>,"api_baseUrl":<origem>+<rest_prefix>,"serverBackend":<origem>,"restEntryPoint":<rest_prefix>}`;
   - `ERPTOKEN`.
5. `ERPTOKEN` é um valor aleatório por sessão do `FwCallApp`: 32 bytes de
   `crypto/rand` em hex, gerado no servidor. Ele existe só porque a lib exige a chave e
   a envia como `Authorization: Bearer`. O REST **não o valida**: a proteção é a
   checagem de origem e de host. O token não é gravado em disco, não sai da máquina e
   não identifica pessoa, modelo ou origem.

Nada mais é injetado. Os cabeçalhos HTTP das respostas do app são os mesmos das demais
rotas do `advplc serve`.

### Task 2.1: localizar e servir o app (pasta ou `.app`)

**Files:**
- Create: `pkg/webui/pouiapp.go`
- Test: `pkg/webui/pouiapp_test.go`

**Interfaces:**
- Produces:

```go
type PouiApp struct{ Nome string; fs fs.FS } // pasta (os.DirFS) ou zip em memória
func AbrePouiApp(caminho string, maxZip int64) (*PouiApp, error)
func LocalizaPouiApp(nome string, dirs []string) (string, error) // procura <dir>/<nome>.app e <dir>/<nome>/index.html, nesta ordem, em cada dir
func (a *PouiApp) ServeHTTP(w http.ResponseWriter, r *http.Request) // r.URL.Path relativo à raiz do app
```

- Regras:
  - o nome segue `NomeDeAppValido`;
  - o `.app` é um zip lido inteiro em memória, até `maxZip` (padrão 64 MB,
    `shared.Config.PouiAppMaxMB`), e nunca extraído para disco;
  - uma entrada do zip com `..`, caminho absoluto ou link simbólico é rejeitada ao
    abrir;
  - se o zip tiver um único diretório de topo que contém o `index.html`, esse
    diretório vira a raiz;
  - sem `index.html`, é erro;
  - `index.html` no caminho `/` e rotas de SPA desconhecidas (sem extensão) devolvem o
    `index.html`, com o shim injetado (Task 2.2);
  - `Content-Type` pela extensão (`mime.TypeByExtension`).

- [x] **Step 1: Escrever os testes que falham**

```go
func TestPouiAppPasta(t *testing.T)            // dir com index.html e assets/data/appConfig.json -> GET / e GET /assets/data/appConfig.json servem
func TestPouiAppZipRaizAninhada(t *testing.T)  // zip com meuapp/index.html -> GET / serve
func TestPouiAppZipSlipRejeitado(t *testing.T) // entrada "../x" -> AbrePouiApp erro
func TestPouiAppZipGrande(t *testing.T)        // zip > maxZip -> erro citando o limite
func TestPouiAppRotaSpa(t *testing.T)          // GET /clientes/123 -> index.html
func TestLocalizaPouiAppOrdem(t *testing.T)    // nome.app e nome/ no mesmo dir -> .app vence; nome inválido "a/b" -> erro
```

- [x] **Step 2:** `go test ./pkg/webui -run 'TestPouiApp|TestLocaliza' -v`. Esperado:
  FAIL.
- [x] **Step 3:** Implementar conforme Interfaces.
- [x] **Step 4:** `go test ./...`. Esperado: PASS.
- [x] **Step 5: Commit** — `feat(webui): servir app PO-UI de pasta ou .app em
  memória`

### Task 2.2: shim e contexto da sessão

**Files:**
- Create: `pkg/webui/poui_shim.js`, `pkg/webui/pouictx.go`
- Modify: `pkg/webui/pouiapp.go` (injeção no `index.html`)
- Test: `pkg/webui/pouictx_test.go`

**Interfaces:**
- Produces:

```go
type PouiCtx struct {
	App, Empresa, Filial, Modulo, Usuario, Idioma, RestPrefix, Origem string
	Token string // ERPTOKEN, crypto/rand 32 bytes hex
}
func NovoPouiCtx(app string, empresa, filial, modulo, usuario, idioma, restPrefix, origem string) (*PouiCtx, error)
func (c *PouiCtx) SessionStorage() map[string]string // chaves e formatos exatos da seção 3.2 item 4
func InjetaShim(index []byte) []byte                 // insere <script>…poui_shim.js…</script> antes do 1º <script; sem <script, antes de </body>
```

- `ProCompany` = `{"Code":<empresa>,"InternalId":<empresa>,"CorporateName":<empresa>}` e
  `ProBranch` = `{"Code":<filial>,"InternalId":<empresa+filial>,"Description":<filial>}`.
  Antes do Step 3, conferir no `.mjs` quais campos a lib lê desses objetos
  (`getCompany`/`getBranch`) e ajustar o formato para exatamente esses.

- [x] **Step 1: Escrever os testes que falham**

```go
func TestSessionStorageChaves(t *testing.T) // contém exatamente as chaves da seção 3.2.4; ERPAPPCONFIG.api_baseUrl == origem+restPrefix
func TestTokenAleatorioPorSessao(t *testing.T) // dois NovoPouiCtx -> tokens diferentes, len 64, só [0-9a-f]
func TestInjetaShimAntesDoPrimeiroScript(t *testing.T)
func TestInjetaShimSemScript(t *testing.T)  // vai antes de </body>
func TestShimSemIdentificadores(t *testing.T) // poui_shim.js não contém "advpp", "claude", URLs externas nem caracteres U+200B..U+200D/U+2060/U+FEFF
```

- [x] **Step 2:** `go test ./pkg/webui -run 'TestSession|TestToken|TestInjeta|TestShim'
  -v`. Esperado: FAIL.
- [x] **Step 3:** Implementar o Go e o `poui_shim.js` conforme a seção 3.2, em JS puro
  sem build e com no máximo 120 linhas.
- [x] **Step 4:** `go test ./...`. Esperado: PASS.
- [x] **Step 5: Commit** — `feat(webui): shim twebchannel e contexto de sessão do app
  PO-UI`

### Task 2.3: canal do app no servidor (`/poui/`, `/poui-ui`, `/poui-ctx`, SSE `advpltojs`)

**Files:**
- Modify: `pkg/webui/server.go`
- Create: `pkg/webui/poui_canal.go`
- Test: `pkg/webui/poui_canal_test.go`

**Interfaces:**
- Consumes: Tasks 2.1 e 2.2.
- Produces:
  - **Rotas novas**, todas com `allowedHost`, a mesma checagem de origem do `/ui` e
    limite de 8 MB:
    - `GET /poui/{app}/{resto...}`: o `PouiApp` da sessão (`?sid=` obrigatório na
      primeira carga; depois vem do cookie `poui_sid`, `HttpOnly`,
      `SameSite=Strict`, `Path=/poui/{app}/`);
    - `GET /poui-ctx?sid=`: devolve o JSON do `SessionStorage()`;
    - `POST /poui-ui?sid=`: corpo `{"app","type","content"}`, que vira o evento
      `jsToAdvpl` na sessão;
    - `/events?s=<sid>&canal=poui`: stream SSE da aba do app (eventos `advpltojs`).
  - Na `Provider` da sessão:

```go
func (p *Provider) AbrePouiApp(app *PouiApp, ctx *PouiCtx) (url string, err error) // registra app na sessão; envia evento "open-tab" {url} à SPA
func (p *Provider) PouiNext(timeout time.Duration) (tipo, conteudo string, fechou bool) // bloqueia até jsToAdvpl, close, ou aba desconectada por > timeout
func (p *Provider) PouiEnvia(tipo, conteudo string)                                // SSE advpltojs {type, content} à aba do app
func (p *Provider) FechaPouiApp()                                                  // desregistra; requisições seguintes a /poui/{app} -> 410
```

  - A SPA (`web/src/app/app.ts`) trata o evento `open-tab` com
    `window.open(url, '_blank')`. Se o pop-up for bloqueado, mostra um `po-notification`
    com o link clicável.
- `close` vindo do app, ou o `EventSource` da aba ficar fechado por mais de 10 s
  (`shared.Config.PouiDesconexaoSeg`, padrão 10), faz o `PouiNext` devolver
  `fechou=true`. O mesmo acontece no fim da sessão chamadora.

- [x] **Step 1: Escrever os testes que falham**

```go
func TestPouiCanalIdaEVolta(t *testing.T)        // POST /poui-ui {type:"preLoad"} -> PouiNext devolve "preLoad"; PouiEnvia("x","1") -> SSE canal=poui recebe advpltojs {"type":"x","content":"1"}
func TestPouiCloseEncerra(t *testing.T)          // POST type "close" -> PouiNext fechou=true
func TestPouiAbaFechadaTimeout(t *testing.T)     // SSE canal=poui conecta e desconecta; timeout 200ms -> fechou=true
func TestPouiSidInvalido404(t *testing.T)        // /poui-ctx?sid=nao-existe -> 404; /poui/app/ sem sid nem cookie -> 404
func TestPouiHostEstranho403(t *testing.T)       // Host: evil.example -> 403 em todas as rotas novas
func TestPouiDepoisDeFechar410(t *testing.T)
func TestPouiCorpoGrande413(t *testing.T)
```

- [x] **Step 2:** `go test ./pkg/webui -run TestPoui -race -v`. Esperado: FAIL.
- [x] **Step 3:** Implementar o canal e o `open-tab` na SPA. Rodar `make web` e
  versionar o `pkg/webui/dist`.
- [x] **Step 4:** `go test ./...`. Esperado: PASS.
- [x] **Step 5: Commit** — `feat(webui): canal do app PO-UI (aba nova, jsToAdvpl,
  advpltojs)`

### Task 2.4: `FwCallApp` e `oWebChannel` na VM

**Files:**
- Create: `pkg/vm/fwcallapp.go`
- Modify: `pkg/vm/natives.go` (remover o esboço `FWCALLAPP`)
- Modify: `pkg/vm/poui.go` (interface opcional nova)
- Test: `pkg/vm/fwcallapp_test.go`, `tests/poui/fwcallapp_test.prw`

**Interfaces:**
- Consumes: Task 2.3 (via interface, para a VM não importar `pkg/webui`).
- Produces:

```go
// pkg/vm/poui.go
type PouiAppUI interface {
	AbrePouiAppPorNome(nome string, dirs []string) (url string, err error)
	PouiNext(timeout time.Duration) (tipo, conteudo string, fechou bool)
	PouiEnvia(tipo, conteudo string)
	FechaPouiApp()
}
```

  - `FwCallApp(cApp, oOwner, oEngine, oChannel, cHost, cSource, ...)` devolve `Nil`,
    como no Protheus. Fluxo:
    1. Fora do modo web (o `UIProvider` não implementa `PouiAppUI`), dá erro
       `FwCallApp: requer o modo web (advplc serve)`.
    2. Diretórios de busca: o diretório do fonte chamador e depois
       `shared.Config.PouiAppsDirs`.
    3. A função de callback é a `Static Function JsToAdvpl` do arquivo `cSource` (sem
       extensão = o fonte chamador). O nome é resolvido como
       `STATIC:<ARQ>:JSTOADVPL`, com fallback para `U_JSTOADVPL`. Se não existir, os
       eventos são só descartados.
    4. O `oWebChannel` é um objeto nativo `TWebChannel` com
       `AdvPLToJS(cType, cContent)` → `PouiEnvia`. `cContent` não-string é serializado
       como JSON.
    5. Laço: `PouiNext`. Em `fechou`, sai. Nos outros casos chama `JsToAdvpl(oWebChannel,
       cType, cContent)` de forma síncrona na VM da sessão. Um erro de runtime no
       callback vai para o console da sessão e o laço segue.
    6. Ao sair: `FechaPouiApp`.
- Os parâmetros 2 a 5 e 7 em diante são aceitos e ignorados, e ficam documentados no
  `COMPONENT_STATUS.md`.

- [x] **Step 1: Escrever os testes que falham** (com um `PouiAppUI` falso no teste)

```go
func TestFwCallAppForaDoModoWeb(t *testing.T)      // erro exato "FwCallApp: requer o modo web (advplc serve)"
func TestFwCallAppChamaJsToAdvplDoFonte(t *testing.T) // fake entrega ("preLoad",""), depois fechou -> fonte imprime "preLoad" e FwCallApp retorna
func TestFwCallAppAdvplToJS(t *testing.T)          // JsToAdvpl faz oWebChannel:AdvPLToJS("r-1","ok") -> fake recebeu ("r-1","ok")
func TestFwCallAppCSourceOutroArquivo(t *testing.T) // projeto com a.prw (chama FwCallApp(..., "b")) e b.prw (Static JsToAdvpl) -> usa o de b
func TestFwCallAppErroNoCallbackNaoDerruba(t *testing.T)
func TestFwCallAppAppInexistente(t *testing.T)     // erro "FwCallApp: app <nome> não encontrado em <dirs>"
```

- [x] **Step 2:** `go test ./pkg/vm -run TestFwCallApp -v`. Esperado: FAIL.
- [x] **Step 3:** Implementar conforme Interfaces. No `cmd/advplc`, ligar a `Provider`
  do webui ao `PouiAppUI`.
- [x] **Step 4:** `go test ./...`. Esperado: PASS.
- [x] **Step 5: Commit** — `feat(vm): FwCallApp real com JsToAdvpl e AdvPLToJS`

### Task 2.5: endpoints de framework que a lib chama

**Files:**
- Create: `cmd/advplc/serve_framework.go`
- Test: `cmd/advplc/serve_framework_test.go`

**Interfaces:**
- Consumes: onda 1 (`rest.Route.Raw`, `MountREST`, `TraduzFiltro`...).
- Produces: rotas Go registradas no mesmo `rest.Server` da onda 1, sob `RestPrefix`.
  Fontes do usuário que declarem a mesma rota vencem (registro posterior). Respostas:

| Rota | Resposta |
|---|---|
| `GET /api/framework/environment/v1/companies` | `{"items":[{"Code":<emp>,"InternalId":<emp>,"CorporateName":<emp>}],"hasNext":false}` a partir do `SM0` local, se existir, ou da empresa corrente |
| `GET /api/framework/environment/v1/branches` | idem para filiais (`pkg/vm/filial.go`) |
| `GET /api/framework/current-user` e `GET /api/framework/v1/users/{id}` | usuário da sessão (`{"id","userName","displayName","emails":[]}`) |
| `GET /api/framework/v1/systemModules` | `{"items":[{"code":<modulo>,"description":<modulo>}],"hasNext":false}` |
| `GET /api/protheus/session/v1/defaults` | empresa, filial, módulo e idioma correntes |
| `GET /api/framework/v1/FwRestTranslate/{x}` | `{}` (sem tradução: textos originais) |
| `GET /api/language/v1/protheus-suported-languages` | `{"items":[{"code":"pt-br"},{"code":"en-us"},{"code":"es-es"}]}` |
| `GET /api/brand/v1/erp-brand` | `{}` |
| `GET /api/framework/v1/accessPrivilegesServices/aliasAccess/{x}` e `.../functionAccess/{x}` | acesso permitido (formato exato conferido no `.mjs` no Step 3) |
| `GET/POST /api/framework/v1/genericList` e `/genericQuery` | consulta ao SQLite com `TraduzFiltro`/`TraduzOrdem`/`TraduzCampos`; só tabelas com SX3 e só campos do SX3 |
| `/api/oauth2/v1/token`, `/api/framework/mfa/*` | 501 `{"code":501,"message":"não suportado no AdvPP (sessão local)"}` |

- O formato exato que a lib espera de cada resposta é conferido no `.mjs` (o
  `map`/`pipe` após cada `http.get`) no Step 3, e a tabela acima é corrigida no plano
  se divergir.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestFrameworkCompanies(t *testing.T)     // items[0].Code == empresa corrente
func TestFrameworkBranches(t *testing.T)
func TestFrameworkCurrentUser(t *testing.T)
func TestFrameworkGenericListSoSX3(t *testing.T) // tabela sem SX3 -> 400; campo fora do SX3 em filter -> 400
func TestFrameworkOauth501(t *testing.T)
func TestFrameworkFonteDoUsuarioVence(t *testing.T) // fonte com @Get("/api/framework/v1/systemModules") -> resposta do fonte
```

- [ ] **Step 2:** `go test ./cmd/advplc -run TestFramework -v`. Esperado: FAIL.
- [ ] **Step 3:** Conferir os formatos no `.mjs` e implementar.
- [ ] **Step 4:** `go test ./...`. Esperado: PASS.
- [ ] **Step 5: Commit** — `feat(serve): endpoints de framework para a
  protheus-lib-core`

### Task 2.6: `advplc serve --poui-app` (apps externos)

**Files:**
- Modify: `cmd/advplc/main.go`, `cmd/advplc/serve_rest.go`
- Test: `cmd/advplc/serve_pouiapp_test.go`

**Interfaces:**
- Produces: `advplc serve --poui-app <dir|arquivo.app> [fonte|diretório]`.
  - Sobe o REST do fonte ou diretório, se informado (onda 1), e os endpoints de
    framework (Task 2.5).
  - A raiz `/` redireciona para `/poui/<nome>/?sid=<sid>`, com uma sessão criada no
    servidor sem VM chamadora: os `jsToAdvpl` são descartados e o `close` só desregistra.
  - Todo o resto (shim, contexto, checagens) é idêntico ao do `FwCallApp`.

- [x] **Step 1: Escrever os testes que falham**

```go
func TestServePouiAppRedirecionaRaiz(t *testing.T) // GET / -> 302 para /poui/<nome>/?sid=
func TestServePouiAppComRest(t *testing.T)         // --poui-app fixture + tests/rest_totvs -> /rest/api/est/v1/armazens 200
func TestServePouiAppSemFonte(t *testing.T)        // só o app -> framework endpoints respondem; jsToAdvpl aceito (204) e descartado
```

- [x] **Step 2:** `go test ./cmd/advplc -run TestServePouiApp -v`. Esperado: FAIL.
- [x] **Step 3:** Implementar conforme Interfaces.
- [x] **Step 4:** `go test ./...`. Esperado: PASS.
- [x] **Step 5: Commit** — `feat(serve): --poui-app para apps PO-UI externos`

### Task 2.7: app de teste com `@totvs/protheus-lib-core`

**Files:**
- Create: `tests/poui/fixture-app-src/` (fonte do projeto Angular mínimo:
  `package.json`, `angular.json`, `src/`)
- Create: `tests/poui/apps/fixture-app/` (o `dist` compilado, com `index.html` na raiz;
  versionado, aprovado pelo operador)
- Create: `tests/poui/fixture-app.prw` (chama `FwCallApp("fixture-app")` e define a
  `Static Function JsToAdvpl`)
- Modify: `Makefile` (alvo `fixture-app`: `npm ci && npx ng build` em
  `tests/poui/fixture-app-src`, saída em `tests/poui/apps/fixture-app/`. O
  `FwCallApp("fixture-app")` acha o app porque `tests/poui/apps` está em
  `PouiAppsDirs` no teste, e `tests/poui/apps/fixture-app/index.html` existe)

**Interfaces:**
- O app (Angular 21, `@po-ui/ng-components` 21.23, mesma versão de `web/`;
  `@totvs/protheus-lib-core` 21.1.4 fixada sem `^`) tem:
  1. `ProtheusLibCoreModule` e `ProAppConfigService.loadAppConfig()` no bootstrap;
  2. página inicial `po-page-dynamic-table` com `p-service-api` =
     `<api_baseUrl>/api/est/v1/armazens`, que é o fixture da onda 1;
  3. botão "Pingar AdvPL" que usa
     `ProJsToAdvplService.buildObservable(..., {sendInfo:{type:'ping'}})` e mostra a
     resposta;
  4. botão "Fechar", que chama `ProAppConfigService.callAppClose(false)`.
- `fixture-app.prw`, em `JsToAdvpl`: para `ping`, chama
  `oWebChannel:AdvPLToJS(cContent, "pong")`.
- `assets/data/appConfig.json` traz `api_baseUrl` vazio (o shim fornece o valor).

- [x] **Step 1:** Criar o projeto, compilar com `make fixture-app`, e versionar
  `tests/poui/apps/fixture-app/`.
- [x] **Step 2:** Com `poui_apps_dirs` apontando para `tests/poui/apps`, rodar
  `./advplc serve tests/poui/fixture-app.prw --port 8098` e abrir
  no navegador (Chrome com a extensão claude-in-chrome). Verificar: a lista mostra os 25
  armazéns paginados; "Pingar" mostra "pong"; "Fechar" faz o `FwCallApp` retornar (o
  console da SPA mostra a linha seguinte do fonte).
- [x] **Step 3:** Estender `tests/poui/ao_vivo.mjs` com o cenário acima, para que
  `tests/poui/ao-vivo.sh fixture-app` o automatize.
- [x] **Step 4:** `tests/poui/ao-vivo.sh fixture-app`. Esperado: saída `OK` nos 3
  passos.
- [x] **Step 5: Commit** — `test: app de teste PO-UI com protheus-lib-core
  (FwCallApp de ponta a ponta)`

### Task 2.8: validação no corpus, documentação e encerramento

- [ ] **Step 1:** `TestCorpusFwCallAppCheck` (local, `ADVPP_CORPUS_2510`; `SKIP` sem
  corpus): os 91 arquivos com `FwCallApp` e os 58 com `JsToAdvpl` passam no `check`
  com 0 erros quando o corpus estiver disponível.
- [ ] **Step 2:** `COMPONENT_STATUS.md`:
  - `FwCallApp`, `TWebChannel`, `--poui-app` e os endpoints de framework;
  - os parâmetros ignorados do `FwCallApp`;
  - o texto da seção 3.2 (divulgação do shim);
  - lacunas: OAuth2/MFA e `TWebEngine` desktop.
- [ ] **Step 3:** `CHANGELOG.md`. Push, CI verde nos 3 SOs e no job `determinism`,
  e revisão do branch somente após autorização. PR e release somente com pedido
  explícito do operador.

**Review Focus da onda 2:**
1. Pop-up bloqueado pelo navegador: a SPA mostra o link e o `FwCallApp` segue
   esperando (`TestOpenTabFallback` no frontend, ou verificação ao vivo na Task 2.7).
2. Recarregar a aba do app (F5): reconecta pelo cookie `poui_sid` em menos de 10 s sem
   encerrar o `FwCallApp` (`TestPouiRecargaDentroDoPrazo`, Task 2.3).
3. Duas abas do mesmo app: a segunda recebe 409 `{"message":"app já aberto nesta
   sessão"}` (`TestPouiSegundaAba409`, Task 2.3).
4. O `.app` com `index.html` em caixa diferente (`Index.html`) é aceito
   (`TestPouiAppIndexCaixa`, Task 2.1).
5. O `AdvPLToJS` com conteúdo grande (1 MB) chega inteiro (`TestPouiEnviaGrande`,
   Task 2.3).

**Aceite da onda 2:** Task 2.7 ao vivo `OK`, corpus com `check` = 0 erros quando
`ADVPP_CORPUS_2510` estiver disponível (ou validação formalmente adiada), CI verde,
revisão aprovada e autorização explícita para push/PR.

---

## 4. Onda 3 — catálogo `Po*` completo

### 4.1 Receita comum (toda task da onda 3 segue)

Para cada componente:

1. **Registro** em `poClasses` (`pkg/vm/poui_native.go:12`), com
   `"PO<NOME>": {"Po<Nome>", "<tipo>"}`.
2. **Construtor** `Po<Nome>():New(oPai, ...)`: um `case` em `poNew` com os argumentos
   da tabela, que viram `props` iniciais. Pai inválido dá erro
   `Po<Nome>: pai inválido`.
3. **Métodos** específicos da tabela em `callPoCompMethod` (ou num helper
   `po<Nome>Method`, como o `poChartMethod`). Todo componente também aceita os
   genéricos já existentes:
   - `Set(cProp, x)` / `Get(cProp)`;
   - `SetVisible`, `SetDisabled`, `SetLabel`, `Id()`.
4. **Eventos:** handlers `On<Evento>(bBloco)` gravados em `handlers["<evento>"]`. O
   frontend envia `POST /ui {id, event, value}`. O valor chega ao bloco como o
   argumento documentado na tabela.
5. **Frontend:** um `@case ('<tipo>')` em `web/src/app/ui-host.ts` com o componente
   PO-UI e as props ligadas. Os eventos chamam o emissor existente. Se for preciso
   módulo PO-UI novo, entra em `app.config.ts`. Depois, `make web` e versionar o
   `pkg/webui/dist`.
6. **Testes Go:** em `pkg/vm/poui_<onda>_test.go`, `Test<Nome>Mount` (props iniciais no
   `mount`), `Test<Nome>Patch` (um setter gera `patch` com a prop certa) e
   `Test<Nome>Evento` (o evento do fake UI chega ao bloco com o argumento certo). Mais
   `Test<Nome>PaiInvalido`.
7. **Fixture ao vivo:** `tests/poui/<onda>_<nome>.prw`, com um cenário em
   `tests/poui/ao_vivo.mjs` (renderiza, interage uma vez e confere o efeito).
8. **Docs:** uma linha no `COMPONENT_STATUS.md` (componente, construtor, métodos,
   eventos).

**Tamanho das tasks:** um grupo coeso de componentes por task (as linhas agrupadas nas
tabelas). Cada task termina com `go test ./...` verde, os builds dos 3 SOs, `make web` e
o cenário ao vivo do grupo `OK`.

### 4.2 Onda 3a — campos e templates (branch `feat/poui-onda3a-campos`)

**Task 3a.1: seleção**

| Classe | Tag PO-UI | `New(oPai, ...)` | Métodos | Eventos (arg) |
|---|---|---|---|---|
| `PoCombo` | `po-combo` | `cLabel, aOpcoes, cValor` | `SetOptions(a)`, `SetFilterService(cUrl)`, `SetValue`, `GetValue` | `OnChange(cValor)` |
| `PoMultiselect` | `po-multiselect` | `cLabel, aOpcoes, aValores` | `SetOptions`, `SetValue(a)`, `GetValue()→a` | `OnChange(aValores)` |
| `PoRadioGroup` | `po-radio-group` | `cLabel, aOpcoes, cValor` | `SetOptions`, `SetColumns(n)`, `SetValue`, `GetValue` | `OnChange(cValor)` |
| `PoCheckboxGroup` | `po-checkbox-group` | `cLabel, aOpcoes, aValores` | `SetOptions`, `SetColumns(n)`, `SetValue(a)`, `GetValue()→a` | `OnChange(aValores)` |
| `PoLookup` | `po-lookup` | `cLabel, cUrlServico, cCampoValor, cCampoLabel` | `SetColumns(a)`, `SetValue`, `GetValue`, `SetFilterParams(o)` | `OnChange(cValor)`, `OnSelected(oLinha)` |

- `aOpcoes` = `{{cValor, cLabel}, ...}`, o mesmo formato de `poOpcoes`.
- Para `PoLookup` e `PoCombo:SetFilterService`, a URL relativa é prefixada com
  `RestPrefix` e deve responder o contrato de lista da onda 1. O PO-UI chama direto do
  navegador, e o AdvPP não intermedeia.

**Task 3a.2: data e hora**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoDatepicker` | `po-datepicker` | `cLabel, dValor` | `SetValue(d)`, `GetValue()→d`, `SetMinDate(d)`, `SetMaxDate(d)`, `SetFormat(c)` | `OnChange(d)` |
| `PoDatepickerRange` | `po-datepicker-range` | `cLabel, dIni, dFim` | `SetValue(dIni,dFim)`, `GetValue()→{dIni,dFim}` | `OnChange({dIni,dFim})` |
| `PoDatetimepicker` | `po-datetimepicker` | `cLabel, cIsoDataHora` | `SetValue(c)`, `GetValue()→c` (ISO 8601) | `OnChange(c)` |
| `PoTimepicker` | `po-timepicker` | `cLabel, cHora` | `SetValue("HH:MM[:SS]")`, `GetValue`, `SetFormat(c)` | `OnChange(c)` |

- `Date` AdvPL ↔ ISO `AAAA-MM-DD` na fronteira. Data vazia vira `""`, que vira
  `CToD("")`.

**Task 3a.3: texto especializado**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoDecimal` | `po-decimal` | `cLabel, nValor, nDecimais` | `SetValue(n)`, `GetValue()→n`, `SetDecimals(n)`, `SetMin(n)`, `SetMax(n)` | `OnChange(n)` |
| `PoEmail` | `po-email` | `cLabel, cValor` | `SetValue`, `GetValue` | `OnChange(c)` |
| `PoUrl` | `po-url` | `cLabel, cValor` | `SetValue`, `GetValue` | `OnChange(c)` |
| `PoRichText` | `po-rich-text` | `cLabel, cHtml` | `SetValue`, `GetValue`, `SetHeight(n)` | `OnChange(cHtml)` |
| `PoSearch` | `po-search` | `cPlaceholder` | `SetItems(a)`, `SetFilterKeys(a)` | `OnFilter(cTexto)` |

- O `PoRichText` devolve o HTML como o PO-UI entrega. O AdvPP não sanitiza: o HTML é
  dado do usuário, e quem o exibe em outro lugar sanitiza.

**Task 3a.4: upload**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoUpload` | `po-upload` | `cLabel, cDiretorioDestino` | `SetExtensions(a)`, `SetMaxSizeMB(n)`, `SetMultiple(l)` | `OnUpload(aArquivos)` |

- O `p-url` aponta para a rota nova `POST /poui-upload?sid=&id=`, que grava em
  `cDiretorioDestino`.
  - O destino é resolvido com `filepath.Abs` e precisa estar dentro do diretório de
    trabalho do `serve` ou de `shared.Config.PouiUploadDirs`.
  - O nome do arquivo é sanitizado com `filepath.Base` e sem `..`.
  - O tamanho é limitado por `SetMaxSizeMB` (padrão 10, teto 100). Esta rota tem limite
    de corpo próprio, igual ao `SetMaxSizeMB` do componente, em vez dos 8 MB das
    demais.
  - Extensões fora de `SetExtensions` dão 415.
- `aArquivos` = `{{cNome, cCaminhoGravado, nBytes}, ...}`.
- Testes extras: `TestUploadForaDoDiretorio403`, `TestUploadNomeComBarras`,
  `TestUploadExtensaoProibida415`.

**Task 3a.5: templates estáticos**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoPageList` | `po-page-list` | `oApp, cTitulo` | `AddAction(cLabel,bAcao)`, `SetFilter(cPlaceholder)`, `SetDisclaimers(a)` | `OnFilter(c)`, `OnAdvancedSearch()` |
| `PoPageEdit` | `po-page-edit` | `oApp, cTitulo` | — | `OnSave()`, `OnSaveNew()`, `OnCancel()` |
| `PoPageDetail` | `po-page-detail` | `oApp, cTitulo` | — | `OnEdit()`, `OnRemove()`, `OnBack()` |

- São páginas (pai = `PoApp`, como o `PoPage`): `PoApp:Navigate` aceita qualquer uma
  delas, e elas aceitam filhos como o `PoPage`.

**Task 3a.6: templates dinâmicos (ligados ao REST da onda 1)**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoPageDynamicTable` | `po-page-dynamic-table` | `oApp, cTitulo, cUrlServico` | `SetFields(a)`, `SetActions(o)`, `SetQuickSearchField(c)`, `SetKeepFilters(l)` | `OnCustomAction(cAcao, oLinha)` |
| `PoPageDynamicEdit` | `po-page-dynamic-edit` | `oApp, cTitulo, cUrlServico` | `SetFields(a)`, `SetDuplicates(a)` | `OnBeforeSave(oRegistro)→lOk` |
| `PoPageDynamicDetail` | `po-page-dynamic-detail` | `oApp, cTitulo, cUrlServico` | `SetFields(a)` | `OnBeforeRemove(oRegistro)→lOk` |
| `PoPageDynamicSearch` | `po-page-dynamic-search` | `oApp, cTitulo` | `SetFilters(a)`, `SetQuickFilter(c)` | `OnQuickSearch(c)`, `OnAdvancedSearch(oFiltros)`, `OnChangeDisclaimers(a)` |

- `aFields` = `{{cProp, cLabel, cTipo, lKey, lFilter, lVisible}, ...}`, com `cTipo` em
  `string|number|date|boolean|currency`. `PoFieldsDeSX3(cAlias)` é uma função nova que
  monta esse array a partir do SX3 local.
- O CRUD vai do navegador direto ao REST: o `GET` lista com o contrato da onda 1, o
  `POST`/`PUT`/`DELETE` vão em `<url>/<id>`.
- Os eventos `OnBefore*` são **síncronos**. O frontend usa o hook `p-before-*` do
  PO-UI, que espera uma `Observable`. O AdvPP faz o round-trip `POST /ui` → handler →
  resposta, que vira o patch `beforeResult` correlacionado por um `reqId`. O timeout é
  30 s e equivale a recusar.
- Testes extras: `TestDynamicTableListaDoAdapter` (ao vivo, com o fixture `armazens`
  da onda 1) e `TestBeforeSaveRecusa` (bloco devolve `.F.` e o registro não é gravado).

**Aceite da 3a:** todas as tasks `OK`, e um `tests/poui/3a_crud.prw` que monta um CRUD
completo de armazéns só com `PoPageDynamicTable`/`Edit`/`Detail` sobre o adapter da
onda 1, verificado ao vivo.

### 4.3 Onda 3b — layout e navegação (branch `feat/poui-onda3b-layout`)

**Task 3b.1: agrupamento**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoAccordion` | `po-accordion` | — | `SetShowManager(l)` | — |
| `PoAccordionItem` | `po-accordion-item` | `cTitulo` (pai = `PoAccordion`) | `Expand()`, `Collapse()` | `OnExpand()`, `OnCollapse()` |
| `PoModal` | `po-modal` | `cTitulo` (pai = `PoPage`/página) | `Open()`, `Close()`, `SetPrimaryAction(cLabel,b)`, `SetSecondaryAction(cLabel,b)`, `SetSize(c)` | `OnClose()` |
| `PoStepper` / `PoStep` | `po-stepper` / `po-step` | `PoStep`: `cLabel` (pai = `PoStepper`) | `PoStepper`: `Next()`, `Previous()`, `GetStep()`, `SetOrientation(c)` | `OnChangeStep(nPasso)`; `PoStep`: `OnCanActivate()→lOk` (síncrono, como na 3a.6) |
| `PoSlide` | `po-slide` | `aSlides` (`{{cImagem, cTitulo, cLink}}`) | `SetInterval(n)`, `SetHeight(n)` | — |

- O `PoModal` é filho lógico da página, mas é renderizado fora do fluxo do layout.

**Task 3b.2: ações**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoButtonGroup` | `po-button-group` | `cToggle` (`none|single|multiple`) | `AddButton(cLabel, cIcone, bAcao)`, `SetSelected(a)` | ações via `bAcao` |
| `PoDropdown` | `po-dropdown` | `cLabel` | `AddAction(cLabel, bAcao, cIcone)` | ações via `bAcao` |
| `PoPopup` | `po-popup` | `oAlvo` (componente) | `AddAction(cLabel, bAcao)`, `Open()`, `Close()` | ações |
| `PoPopover` | `po-popover` | `oAlvo, cTitulo` | `SetTrigger(c)`, `SetPosition(c)` (aceita filhos) | `OnClose()` |
| `PoContextMenu` | `po-context-menu` | `cTitulo` | `AddItem(cLabel, bAcao)`, `SetSelected(n)` | `OnSelect(nItem)` |
| `PoToolbar` | `po-toolbar` | `oApp, cTitulo` | `AddAction(cLabel, bAcao, cIcone)`, `SetProfile(cNome, cEmail, aAcoes)`, `SetNotifications(n)` | ações |

**Task 3b.3: navegação**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoBreadcrumb` | `po-breadcrumb` | `aItens` (`{{cLabel, cPagina}}`) | `SetItems(a)` | `OnNavigate(cPagina)`, que chama `PoApp:Navigate` |
| `PoNavbar` | `po-navbar` | `oApp` | `SetLogo(cUrl)`, `AddItem(cLabel, bAcao)`, `AddIconAction(cIcone, cLabel, bAcao)` | ações |
| `PoHeader` | `po-header` | `oApp, cMarca` | `SetUser(cNome, cEmail)`, `AddAction(cLabel, bAcao)` | ações |
| `PoMenuPanel` | `po-menu-panel` | `oApp` | `AddItem(cLabel, cIcone, bAcao)` | ações |
| `PoContextTabs` | `po-context-tabs` | — | `AddTab(cLabel, oPagina)`, `CloseTab(n)` | `OnClose(n)`, `OnActivate(n)` |
| `PoLink` | `po-link` | `cLabel, cUrl` | `SetOpenNewTab(l)` | `OnClick()` |

- Navbar, header e menu-panel são únicos por `PoApp`, como o `PoMenu` atual. Um
  segundo dá erro `PoApp: só um <Classe> por aplicação`.

**Task 3b.4: exibição**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoAvatar` | `po-avatar` | `cImagem, cTamanho` | `SetSrc(c)` | `OnClick()` |
| `PoBadge` | `po-badge` | `nValor, cCor` | `SetValue(n)`, `SetColor(c)`, `SetStatus(c)` | — |
| `PoImage` | `po-image` | `cSrc, cAlt` | `SetSrc`, `SetHeight(n)` | — |
| `PoSkeleton` | `po-skeleton` | `cTipo, cTamanho` | `SetVariant(c)` | — |
| `PoHelper` | `po-helper` | `cTitulo, cTexto` | `SetType(c)` | — |
| `PoCalendar` | `po-calendar` | `dValor` | `SetMode(c)`, `SetValue(d)`, `GetValue` | `OnChange(d)` |
| `PoDynamicView` | `po-dynamic-view` | `aCampos, oValor` | `SetValue(o)`, `SetFields(a)`, `SetLoad(cUrl)` | — |
| `PoDisclaimerGroup` | `po-disclaimer-group` | `cTitulo, aDisclaimers` | `SetItems(a)` | `OnChange(a)`, `OnRemoveAll()` |
| `PoFilterChip` | `po-filter-chip` | `cLabel` | `SetSelected(l)` | `OnChange(l)` |
| `PoToaster` | (serviço `PoNotificationService` com orientação) | — | — | — (ver `PoApp:Notify` estendido abaixo) |

- `PoApp:Notify(cMsg, cTipo, nDuracao, cOrientacao, cAcaoLabel, bAcao)`: estende o
  `Notify` atual. Os 2 primeiros argumentos mantêm a assinatura de hoje.
- Uma `src` de imagem relativa é resolvida contra o diretório do fonte e servida por
  `GET /poui-asset?sid=&p=`, com a mesma checagem de caminho do upload.

**Task 3b.5: páginas especiais**

| Classe | Tag | `New(oPai, ...)` | Métodos | Eventos |
|---|---|---|---|---|
| `PoPageLogin` | `po-page-login` | `oApp, cProduto` | `SetLiterals(o)`, `SetRecovery(l)` | `OnLogin(cUsuario, cSenha)→lOk` (síncrono) |
| `PoPageChangePassword` | `po-page-change-password` | `oApp` | `SetRequirements(a)` | `OnSubmit(cAtual, cNova)→lOk` |
| `PoPageBlockedUser` | `po-page-blocked-user` | `oApp, cMotivo` | `SetContactEmail(c)` | — |
| `PoPageSlide` | `po-page-slide` | `oApp, cTitulo` (aceita filhos) | `Open()`, `Close()` | `OnClose()` |
| `PoPageJobScheduler` | `po-page-job-scheduler` | `oApp, cTitulo, cUrlServico` | `SetParameters(a)` | — |

- A senha recebida em `OnLogin`/`OnSubmit` nunca vai para o console, para log ou para o
  `Get()` posterior: a prop do componente é limpa logo depois do evento. Teste:
  `TestLoginSenhaNaoFicaNaProp`.

**Aceite da 3b:** todas as tasks `OK`, e um `tests/poui/3b_vitrine.prw` com um app de
navegação completo (header, menu, breadcrumb, abas, modal e stepper) verificado ao vivo.

### 4.4 Onda 3c — serviços e diretivas (branch `feat/poui-onda3c-servicos`)

**Task 3c.1: tema, idioma e mídia**

| API AdvPL | Base PO-UI | Comportamento |
|---|---|---|
| `PoApp:SetTheme(cTema)` | `PoThemeService` | `"light"|"dark"` ou JSON de tokens (`PoThemeTokens`); patch global |
| `PoApp:SetLanguage(cIdioma)` | `PoI18nService` | `"pt"|"en"|"es"|"ru"`; muda os literais do PO-UI |
| `PoApp:OnMediaChange(bBloco)` | `PoMediaQueryService` | bloco recebe `"sm"|"md"|"lg"|"xl"` ao mudar a largura |

**Task 3c.2: armazenamento e rede**

| API AdvPL | Base PO-UI | Comportamento |
|---|---|---|
| `PoApp:StorageSet(cChave, xValor)` / `StorageGet(cChave)` / `StorageRemove(cChave)` | `PoStorageService` (IndexedDB/localStorage) | `StorageGet` é síncrono via round-trip (timeout 5 s → `Nil`); valores serializados em JSON |
| `PoApp:OnNetworkChange(bBloco)` | `PoNetworkService` | bloco recebe `.T.` (online) ou `.F.` (offline) |

**Task 3c.3: guia e diretivas**

| API AdvPL | Base PO-UI | Comportamento |
|---|---|---|
| `PoApp:StartGuide(aPassos)` | `PoUserGuideService` | `aPassos` = `{{oComponente, cTitulo, cTexto}}`; `OnGuideEnd(b)` |
| `oComp:SetTooltip(cTexto[, cPosicao])` | `p-tooltip` | todo componente; patch da prop `tooltip` |
| `oComp:SetDraggable(l)` e `oContainer:OnDrop(bBloco)` | `p-drag` / `p-drop-list` | bloco recebe `(oComponenteArrastado, nNovaPosicao)`; a ordem dos filhos do container é atualizada no servidor |

**Task 3c.4: avaliação de `PoSync`, `PoSearchAi` e `po-code-editor`**
- Escrever `docs/superpowers/specs/<data>-poui-3c-avaliacao.md` com, para cada item:
  uso real (corpus e apps TOTVS conhecidos), dependência de backend e custo.
- **Parar e levar ao operador.** Nada é implementado sem decisão dele.
- O `po-code-editor` é avaliado como alternativa ou complemento ao `AdvCodeEditor`
  (CodeMirror) já existente.

**Aceite da 3c:** tasks 3c.1 a 3c.3 `OK` ao vivo, e o documento da 3c.4 entregue ao
operador.

---

## 5. Riscos conhecidos e o que fazer

| Risco | Sinal | Ação |
|---|---|---|
| Fonte real do corpus depende de função Protheus não emulada (ex.: `FWExecStatement`, `MPSysOpenQuery`) | `TestCorpusCincoFontesHTTP` falha com "function not found" | implementar a função mínima com teste próprio (na onda 1, dentro da Task 10); se for grande, parar e escalar ao operador com a lista de pendências antes de ampliar o escopo |
| A lib TOTVS muda o contrato numa versão nova | o app de teste quebra após atualizar a lib | a versão fica fixada (21.1.4); atualização é task própria |
| `ServeMux` recusa padrões do corpus (ex.: `{a}` e literal no mesmo nível) | erros de registro na Task 9 | já tratado: erro listado e servidor segue; a lista entra no `COMPONENT_STATUS.md` |
| PO-UI 21 sem zone.js | componente novo fica invisível (opacity 0) | o zone.js já está nos polyfills; todo componente novo é verificado ao vivo |
| Eventos síncronos (`OnBefore*`, `OnLogin`, `StorageGet`) travarem a VM | tela congela | round-trip com timeout fixo e default seguro (recusar ou `Nil`) |

---

## 6. Critério de "pronto" de todo o projeto

1. As ondas 1, 2, 3a, 3b e 3c têm PR aceito em `master`, com CI verde nos 3 SOs e no
   job `determinism`.
2. Os testes de corpus (`ADVPP_CORPUS_2510`) passam localmente quando o corpus estiver
   disponível: 214 adapters, 500 `WSRESTFUL`, 91 `FwCallApp` e 58 `JsToAdvpl` com
   `check` = 0 erros, mais 5 fontes reais respondendo via HTTP. Se o corpus não
   estiver disponível, cada validação fica formalmente pendente e o projeto não pode
   ser declarado integralmente pronto.
3. O app de teste com `@totvs/protheus-lib-core` roda de ponta a ponta pelo
   `FwCallApp` e pelo `--poui-app`.
4. Todos os componentes do `llms.txt` estão cobertos por classe `Po*` ou registrados no
   `COMPONENT_STATUS.md` como fora de escopo, com o motivo: `po-grid` em
   desenvolvimento, `po-gauge` obsoleto, e os itens que a 3c.4 decidir.
5. O `COMPONENT_STATUS.md` e o `CHANGELOG.md` estão atualizados em cada onda, e as
   releases foram feitas quando o operador pediu.
