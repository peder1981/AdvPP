# PO-UI no padrão TOTVS — Onda 1 (REST) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fontes REST reais do Protheus 12.1.2510 respondem via HTTP no AdvPP igual ao
Protheus. O escopo é `WSRESTFUL` clássico, `@Get`+`oRest`, `FWAdapterBaseV2`,
`FwApiManager` (leitura) e `TamSx3`, servidos pelo `advplc serve` na mesma origem.

**Architecture:**
- O compilador ganha:
  - `_Super:` (chamada à superclasse, inclusive nativa);
  - compilação de vários fontes num só bytecode (como um RPO), com `Static Function`
    restrita ao arquivo;
  - metadados de `WSMETHOD` (verbo, `PATH`) guardados em `ClassInfo.Annotations`.
- O `pkg/rest` ganha um handler de baixo nível (`Request`/`Response`), rotas `:param`,
  registro sem pânico e montagem dentro do mux do `pkg/webui`.
- Na VM: o dispatch do `WSRESTFUL` clássico, o objeto `oRest`, `TamSx3` e as classes
  nativas `FWAdapterBaseV2`/`FwApiManager`. O tradutor de filtro OData fica num arquivo
  Go puro, testável sozinho.

**Tech Stack:** Go 1.27 sem CGO, `net/http` (ServeMux 1.22+), SQLite local já
existente (com SX3).

**Spec:** `docs/superpowers/specs/2026-11-22-poui-compat-totvs-design.md` (seções 7.1
e 8, linha "Onda 1").

## Global Constraints

- **Compatibilidade total:** programas existentes, incluindo `WSRestServer():New/
  AddRoute/Serve`, `@Get` recebendo um objeto JSON de parâmetros, `FWMBrowse`,
  MSDIALOG, `Msg*` e `PoApp`, continuam idênticos. `go test ./...` verde em toda task.
- **Executáveis para Linux, Windows e macOS:** em toda task,
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$(mktemp -d)/x" ./cmd/advplc`
  e `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o "$(mktemp -d)/x" ./cmd/advplc`
  compilam. Nunca sobrescrever `./advplc`.
- **Bytecode determinístico:** o job `determinism` da CI segue verde. Metadados novos
  só em estruturas já existentes (`ClassInfo.Annotations`). Nada muda no
  `serializer.go`.
- **Sem dependências novas e sem CGO.**
- **SQL seguro:** só propriedades mapeadas (`AddMapFields`/`SetApiMap`) chegam ao SQL,
  e os valores vão sempre como parâmetro `?`. Campo não mapeado em
  `filter`/`order`/`fields` devolve 400.
- **Configuração nova** em `shared.Config` (`pkg/tools/shared/config.go`):
  `RestPrefix string \`json:"rest_prefix,omitempty"\``, padrão `/rest`.
- **Corpus TOTVS só para validar, localmente.** Nenhum fonte do corpus entra no repo.
  Testes que o usam leem `ADVPP_CORPUS_2510` e fazem `t.Skip` quando ela está ausente.
- **Fontes de teste** `.prw`/`.tlpp` em CP-1252. Textos, comentários e commits em
  português, sem linhas de atribuição a ferramenta ou modelo.
- **`master` protegido:** trabalhar no branch `feat/poui-onda1-rest`, com PR no fim.

## Review Focus

1. **Duas rotas conflitantes** (ex.: `GET /api/x/{a}` e `GET /api/{b}/y`, de fontes
   diferentes): o `ServeMux` entra em pânico no registro. Esperado: erro claro citando
   as duas rotas e os fontes; o servidor sobe com as demais. Teste:
   `TestAddRouteConflitoNaoEntraEmPanico` (Task 4).
2. **Erro de runtime dentro do método REST** (exceção AdvPL, método inexistente):
   resposta 500 `{"code":500,"message":...}`; a VM filha é liberada e a requisição
   seguinte funciona. Teste: `TestWSRestErroRuntimeDevolve500EContinua` (Task 5).
3. **`filter` hostil** (`nome eq 'x'' or 1=1 --'`, `;`, campo não mapeado, parêntese
   aberto): 400 sem executar SQL. Teste: `TestFiltroHostil` (Task 7).
4. **WSDATA numérica com valor inválido ou ausente na query** (`?pageSize=abc`): a
   propriedade fica `Nil`, o `Default` do método vale e não há pânico. Teste:
   `TestWSDataNumericaInvalidaViraNil` (Task 5).
5. **Requisições concorrentes:** o estado de resposta (corpo, status, headers) de uma
   requisição não vaza para a outra. Teste: `TestWSRestConcorrenteIsolado` (Task 5,
   `go test -race`).

---

## File Structure

| Arquivo | Responsabilidade |
|---|---|
| `pkg/parser/parser.go` (modificar) | `parseWSClient`/`parseWSMethodImpl`: guardar verbo e `PATH`; método sem nome passa a se chamar pelo verbo |
| `pkg/compiler/codegen.go` (modificar) | `_Super:`; anotações `WSRESTFUL`/`WSMETHOD` em `ClassInfo`; `CompileProject` |
| `pkg/compiler/opcodes.go` (modificar) | opcode `OP_CALL_SUPER` |
| `pkg/vm/vm.go` (modificar) | `OP_CALL_SUPER`; herança de classe nativa (`rootNativeClass`) |
| `pkg/rest/rest.go` (modificar) | `Request`/`Response`, `Route.Raw`, `:param`, `AddRoute` com erro, `Handler()` |
| `pkg/vm/wsrestful.go` (criar) | dispatch do `WSRESTFUL` clássico + `SetRestFault`/`HTTPSetStatus` |
| `pkg/vm/orest.go` (criar) | objeto `oRest` nas rotas anotadas |
| `pkg/vm/odata_filter.go` (criar) | tradução de `filter`/`order`/`fields` para SQL parametrizado (Go puro) |
| `pkg/vm/adapterv2.go` (criar) | `FWAdapterBaseV2` + `TamSx3` |
| `pkg/vm/apimanager.go` (criar) | `FwApiManager` (leitura) |
| `cmd/advplc/main.go`, `cmd/advplc/serve_rest.go` (criar) | `serve <diretório>` e REST montado em `rest_prefix` |
| `tests/rest_totvs/*.tlpp`/`*.prw` (criar) | fixtures próprios (não TOTVS) |
| `tests/rest_totvs/corpus_test.go` (criar) | validação no corpus (só local) |

---

### Task 1: `_Super:` e herança de classe nativa

**Files:**
- Modify: `pkg/compiler/opcodes.go` (novo `OP_CALL_SUPER` no fim do enum)
- Modify: `pkg/compiler/codegen.go` (método-chamada cujo receptor é o identificador
  `_Super`)
- Modify: `pkg/vm/vm.go` (`OP_CALL_SUPER`; `newInstance` e `callNativeMethod` para
  classe de usuário com raiz nativa)
- Test: `pkg/vm/super_test.go`

**Interfaces:**
- Produces:
  - `OP_CALL_SUPER`, com `Str = "<ClasseQueDefineOMetodo>:<Metodo>"` e
    `Arg2 = nº de args`. O receptor é o `self` do frame atual.
  - `func (v *VM) rootNativeClass(className string) string`: sobe a cadeia `Parent`
    até achar uma classe nativa conhecida (`"FWADAPTERBASEV2"`, `"FWAPIMANAGER"`...)
    ou devolve `""`.
  - `func registerNativeBase(name string, factory func() *advplrt.ObjectValue)`: o
    mapa que as Tasks 7 e 8 usam para declarar classes nativas herdáveis.
- Regra: uma instância de classe de usuário com raiz nativa recebe `obj.Native` da
  fábrica da raiz. Método não achado no bytecode cai em `callNativeMethod` usando o
  nome da raiz.

- [X] **Step 1: Escrever os testes que falham**

```go
func TestSuperChamaMetodoDoPai(t *testing.T) {
	// Base:New grava ::cX := "base"; Filho:New faz _Super:New() e depois ::cY := "filho"
	out := saidaDe(t, srcSuperDoisNiveis)
	if out != "base|filho\n" { t.Fatalf("got %q", out) }
}
func TestSuperComArgs(t *testing.T) { /* _Super:New("a", 2) chega como cA=="a", nB==2 */ }
func TestSuperNativoRegistrado(t *testing.T) {
	// registerNativeBase("TESTBASE", fábrica cujo Native responde ECO(x) -> x)
	// Class Filho FROM TestBase; Filho():New():Eco("oi") imprime "oi"
}
```

- [X] **Step 2:** `go test ./pkg/vm -run 'TestSuper' -v`. Esperado: FAIL ("cannot call
  method New on non-object").
- [X] **Step 3:** Implementar o opcode, o codegen e a VM conforme Interfaces. O
  codegen conhece a classe que está sendo compilada (`MethodImpl.ClassName`). Quem
  resolve o pai é a VM em tempo de execução, porque o pai pode estar em outro fonte
  (Task 2) ou ser nativo.
- [X] **Step 4:** Rodar `go test ./pkg/vm -run 'TestSuper' -v` e `go test ./...`.
  Esperado: PASS, sem regressão no `bytecode_determinism_test`.
- [X] **Step 5: Commit** — `feat(vm): _Super: e herança de classe nativa`

---

### Task 2: `CompileProject` e `advplc serve <diretório>`

**Files:**
- Modify: `pkg/compiler/codegen.go`
- Modify: `cmd/advplc/main.go` (`serve` aceita diretório)
- Test: `pkg/compiler/project_test.go`, `cmd/advplc/serve_dir_test.go`

**Interfaces:**
- Produces:
  - `func CompileProject(progs []*ast.Program) (*Bytecode, error)`: um único
    `Compiler` compila os programas em sequência, no mesmo pool e no mesmo `Code`, sem
    relocação.
  - `Static Function` fica registrada como `"STATIC:<ARQUIVO>:<NOME>"` (arquivo em
    maiúsculas, sem diretório). As chamadas a ela dentro do mesmo arquivo são emitidas
    com o nome qualificado.
  - `User Function` e classes são globais. Nome duplicado entre arquivos dá erro
    citando os dois arquivos.
  - O corpo principal de cada programa é ignorado, exceto os inicializadores dos
    `Static` de arquivo, emitidos no início do "main" (`MainOffset` aponta para o
    bloco de inicialização, que termina em `OP_HALT`; sem `Static`, aponta direto
    para o `HALT`). Desvio documentado na implementação: o plano original previa
    `MainOffset` → `HALT` direto, mas inicializar `Static` de arquivo exige código
    executável antes do halt.
  - `func compilaDiretorio(dir string, opts *Options) (*compiler.Bytecode, error)` em
    `cmd/advplc`: preprocessa e parseia cada `.prw`/`.prx`/`.tlpp` (não recursivo) com
    `dir` no include path. Ordem dos arquivos: alfabética, para manter o determinismo.

- [X] **Step 1: Escrever os testes que falham**

```go
func TestProjetoStaticPorArquivo(t *testing.T) {
	// a.prw e b.prw definem Static Function JsToAdvpl() retornando "A" e "B";
	// U_A() chama JsToAdvpl() e U_B() idem -> "A" e "B"
}
func TestProjetoHerancaEntreArquivos(t *testing.T) {
	// repo.tlpp: Class Repo FROM Base; base.tlpp: Class Base -> Repo():New() funciona
}
func TestProjetoUserFunctionDuplicadaErro(t *testing.T) {
	// U_X em a.prw e b.prw -> erro contendo "a.prw" e "b.prw"
}
```

- [X] **Step 2:** `go test ./pkg/compiler -run TestProjeto -v`. Esperado: FAIL (função
  indefinida).
- [X] **Step 3:** Implementar `CompileProject` e `compilaDiretorio`. No `serve`, se o
  argumento for diretório, usar `compilaDiretorio`; se for arquivo, nada muda. O
  `--watch` sobre diretório observa o mtime de todos os arquivos.
- [X] **Step 4:** `go test ./pkg/compiler ./cmd/advplc -v` e `go test ./...`. Esperado:
  PASS.
- [X] **Step 5: Commit** — `feat(compiler): CompileProject (vários fontes, Static por
  arquivo) e advplc serve <diretório>`

---

### Task 3: metadados de `WSRESTFUL` no parser e no bytecode

**Files:**
- Modify: `pkg/parser/parser.go` (`parseWSClient` ~l.940, `parseWSMethodImpl` ~l.1084)
- Modify: `pkg/compiler/codegen.go` (registro de classes, ~l.135)
- Test: `pkg/parser/wsrestful_test.go`

**Interfaces:**
- Produces (em `ClassInfo.Annotations` da classe do serviço):
  - `{Name:"WSRESTFUL", Value:""}`;
  - para cada `WSMETHOD`: `{Name:"WSMETHOD", Value:"<VERBO>|<NomeDoMetodo>|<PATH>"}`.
    O `PATH` é o literal string avaliado (vazio se ausente).
  - `WSDATA` segue em `Properties` (nome → tipo).
- Método REST sem nome passa a se chamar **pelo verbo** (`"GET"`, `"POST"`...), tanto na
  declaração quanto na implementação. Hoje a implementação vira `"REST"`, o que faz
  dois métodos sem nome numa mesma classe colidirem.
- O `PATH` que não for um literal string (expressão) é guardado vazio, com aviso do
  parser (`parser.Aviso`).

- [X] **Step 1: Escrever os testes que falham**

```go
func TestWSRestfulGuardaVerboEPath(t *testing.T) {
	// WSRESTFUL Clientes DESCRIPTION "x"
	//   WSDATA page AS INTEGER OPTIONAL
	//   WSMETHOD GET DESCRIPTION "lista" PATH "/api/v1/clientes" PRODUCES APPLICATION_JSON
	//   WSMETHOD GET umCliente DESCRIPTION "um" PATH "/api/v1/clientes/{id}"
	// END WSRESTFUL
	// WSMETHOD GET WSRECEIVE page WSSERVICE Clientes ... Return .T.
	// WSMETHOD GET umCliente WSSERVICE Clientes ... Return .T.
	// espera Annotations: WSRESTFUL, "GET|GET|/api/v1/clientes", "GET|umCliente|/api/v1/clientes/{id}"
	// e Methods contendo as chaves "GET" e "umCliente"
}
func TestWSRestfulDoisVerbosSemNomeNaoColidem(t *testing.T) { /* GET e POST sem nome -> métodos "GET" e "POST" */ }
```

- [X] **Step 2:** `go test ./pkg/parser -run TestWSRestful -v`. Esperado: FAIL.
- [X] **Step 3:** Implementar. O AST leva as anotações em `ClassDecl.Annotations`
  (`MethodDecl` não precisa mudar).
- [X] **Step 4:** `go test ./...` e `go test ./cmd/advplc -run Determinism`. Esperado:
  PASS.
- [X] **Step 5: Commit** — `feat(parser): WSRESTFUL guarda verbo e PATH de cada
  WSMETHOD`

---

### Task 4: `pkg/rest`, com handler de baixo nível, `:param` e registro sem pânico

**Files:**
- Modify: `pkg/rest/rest.go`
- Test: `pkg/rest/rest_raw_test.go`

**Interfaces:**
- Produces:

```go
type Request struct {
	Method     string
	Path       string
	PathParams map[string]string // nomes como declarados
	Query      url.Values        // ordem original preservada em QueryOrder
	QueryOrder []string
	Header     http.Header
	Body       []byte
}
type Response struct {
	Status      int // 0 = 200
	ContentType string // "" = application/json
	Header      map[string]string
	Body        []byte
}
// Route ganha: Raw func(*Request) *Response — se não-nil, tem precedência sobre Handler.
func (s *Server) AddRouteE(r Route) error   // valida conflito; AddRoute segue sem erro (compat)
func (s *Server) Handler() http.Handler     // mux com limitBody; usado pelo webui
func NormalizaPath(p string) string         // "/a/:id/b" -> "/a/{id}/b"; garante "/" inicial
```

- `AddRouteE` detecta o conflito montando um `ServeMux` de prova com `recover()`. O erro
  cita as duas rotas.
- O `Handler()` é montado uma vez por `Server`. As rotas registradas depois de montado
  causam a remontagem na próxima requisição (mutex).

- [X] **Step 1: Escrever os testes que falham**

```go
func TestRawRecebeTudo(t *testing.T) // GET /a/{id}?x=1&y=2 com header X-K e corpo -> Request com PathParams["id"], QueryOrder ["x","y"], Header, Body
func TestRawStatusEHeaders(t *testing.T) // Response{Status:201, Header:{"X-T":"1"}, Body:`{}`} -> 201, X-T, Content-Type application/json
func TestNormalizaPathDoisPontos(t *testing.T) // "/api/x/:InternalID" -> "/api/x/{InternalID}"
func TestAddRouteConflitoNaoEntraEmPanico(t *testing.T) // "/api/x/{a}" e "/api/{b}/y" no mesmo verbo -> erro contendo ambos; Handler() segue servindo a primeira
func TestRotasAntigasIntactas(t *testing.T) // Route.Handler (map params) responde igual a hoje
```

- [X] **Step 2:** `go test ./pkg/rest -v`. Esperado: FAIL.
- [X] **Step 3:** Implementar conforme Interfaces.
- [X] **Step 4:** `go test ./pkg/rest ./pkg/vm -v`. Esperado: PASS.
- [X] **Step 5: Commit** — `feat(rest): handler de baixo nível, rotas :param e
  conflito sem pânico`

---

### Task 5: dispatch do `WSRESTFUL` clássico

**Files:**
- Create: `pkg/vm/wsrestful.go`
- Modify: `pkg/vm/vm.go` (`callNativeMethod`: objeto com `Native *wsRestReq` atende os
  métodos abaixo quando o bytecode não os define)
- Modify: `pkg/vm/natives.go` (`SETRESTFAULT`, `HTTPSETSTATUS`, `GETCONTENT` globais)
- Modify: `pkg/vm/rest_native.go` (`WSRestServer:New` também registra os `WSRESTFUL`)
- Test: `pkg/vm/wsrestful_test.go`, `tests/rest_totvs/clientes_classico.prw`

**Interfaces:**
- Consumes: Task 3 (anotações), Task 4 (`Route.Raw`, `AddRouteE`, `NormalizaPath`).
- Produces:
  - `func (v *VM) RegistrarWSRestful(s *rest.Server, prefixo string) []error`: uma rota
    por `WSMETHOD`. Path = `prefixo + PATH`. Sem `PATH`, o path é
    `prefixo + "/" + lower(serviço)`, e a rota também casa `.../{resto...}`, com os
    segmentos indo para `aURLParms`.
  - Por requisição: `newChildVM()`; instância nova da classe; `obj.Native =
    &wsRestReq{...}`; chamada a `"<Classe>::<Metodo>"`.
  - Binding: cada `WSDATA` recebe o valor da query string (nome case-insensitive). Tipo
    `INTEGER`/`NUMERIC`/`FLOAT` vira número, `BOOLEAN` vira lógico e o resto vira
    caractere. Valor inválido ou ausente fica `Nil`.
  - Propriedades e métodos do objeto do serviço:
    - `aURLParms` (array de strings) e `aQueryString` (array `{chave, valor}` na
      ordem da URL);
    - `SetResponse(c)` (acumula no corpo), `SetContentType(c)`, `SetHeader(k,v)`,
      `GetHeader(k)`, `SetStatus(n)` e `GetContent()` (corpo cru, string).
  - Funções globais, que agem sobre a requisição corrente da VM:
    - `SetRestFault(nCode, cMsg)` grava o status `nCode` (fora de 100–599 vira 400) e o
      corpo `{"code":nCode,"message":cMsg}`, que substitui o `SetResponse`;
    - `HTTPSetStatus(n)`;
    - `GetContent()`.
  - Retorno `.F.` sem `SetRestFault` dá 500 `{"code":500,"message":"método
    <Classe>:<Metodo> retornou .F."}`.
  - Erro de runtime dá 500 `{"code":500,"message":<erro>}`.
  - A resposta é UTF-8 (as strings da VM já são UTF-8 depois do preprocessador).

- [X] **Step 1: Escrever os testes que falham** (servidor via
  `httptest.NewServer(s.Handler())`, fonte em `tests/rest_totvs/clientes_classico.prw`)

```go
func TestWSRestClassicoGetLista(t *testing.T)            // GET /rest/api/v1/clientes?page=2 -> 200, corpo do SetResponse, Self:page == 2 ecoado
func TestWSRestClassicoPathParam(t *testing.T)           // GET /rest/api/v1/clientes/000123 -> corpo contém "000123" (via ::aURLParms ou path param)
func TestWSRestClassicoPostGetContent(t *testing.T)      // POST corpo {"nome":"x"} -> método lê ::GetContent() e responde 201 via SetStatus
func TestWSRestClassicoSetRestFault(t *testing.T)        // SetRestFault(404,"não achei") + Return .F. -> 404 {"code":404,"message":"não achei"}
func TestWSRestErroRuntimeDevolve500EContinua(t *testing.T) // 1ª chamada força erro -> 500; 2ª chamada normal -> 200
func TestWSDataNumericaInvalidaViraNil(t *testing.T)     // ?page=abc -> Default page := 1 vale -> corpo ecoa 1
func TestWSRestConcorrenteIsolado(t *testing.T)          // 50 goroutines com ids distintos -> cada corpo contém só o próprio id (rodar com -race)
func TestWSRestServerAntigoIntacto(t *testing.T)         // tests/rest_server_test.prw segue respondendo igual
```

- [X] **Step 2:** `go test ./pkg/vm -run 'TestWSRest|TestWSData' -race -v`. Esperado:
  FAIL.
- [X] **Step 3:** Implementar conforme Interfaces.
- [X] **Step 4:** Repetir o comando do Step 2 e rodar `go test ./...`. Esperado: PASS.
- [X] **Step 5: Commit** — `feat(vm): WSRESTFUL clássico de ponta a ponta`

---

### Task 6: `oRest` nas rotas anotadas (`@Get` etc.)

**Files:**
- Create: `pkg/vm/orest.go`
- Modify: `pkg/vm/rest_native.go` (`autoRegisterAnnotatedRoutes` usa `Route.Raw`;
  aceita `endpoint=` na anotação; usa `NormalizaPath`)
- Test: `pkg/vm/orest_test.go`, `tests/rest_totvs/orest_api.tlpp`

**Interfaces:**
- Consumes: Task 4.
- Produces:
  - Durante a chamada, a variável `Private oRest` fica visível à função anotada. Métodos
    (case-insensitive):
    - leitura: `getQueryRequest()` (JsonObject chave→string), `getPathParamsRequest()`
      (JsonObject), `getBodyRequest()` (string), `getHeaderRequest()` (JsonObject);
    - resposta: `setResponse(x)` (string ou JsonObject; JsonObject é serializado),
      `setStatusCode(n)`, `setKeyHeaderResponse(k,v)`,
      `updateKeyHeaderResponse(k,v)`, `setFault(x)` (status 400 se nenhum outro foi
      definido, mais o corpo x).
  - Compatibilidade: a função continua recebendo o objeto JSON de parâmetros como
    argumento. Se `oRest:setResponse` não foi chamado, o valor de retorno vira o corpo,
    como hoje.
  - `@Get("/a/:id")`, `@Get(endpoint="/a/:id", description="...")` e
    `@GET('/a')` são equivalentes.
- Uma variável local chamada `oRest` (o corpus usa esse nome também para cliente
  `FWRest`) sombreia a `Private`, sem conflito.

- [X] **Step 1: Escrever os testes que falham**

```go
func TestORestQueryEPath(t *testing.T)      // @Get("/api/t/v1/item/:code") ?x=1 -> corpo {"code":"A1","x":"1"} montado com getPathParamsRequest/getQueryRequest
func TestORestStatusEHeader(t *testing.T)   // setStatusCode(201) + setKeyHeaderResponse("X-Total","7") -> 201 e header
func TestORestJsonObjectSerializado(t *testing.T) // setResponse(jObj) -> Content-Type application/json, corpo JSON
func TestAnotacaoEndpointNomeado(t *testing.T)    // @Get(endpoint="/api/t/v1/n") registra /rest/api/t/v1/n
func TestAnotadoSemORestIntacto(t *testing.T)     // tests/tlpp_oop_rest_test.tlpp segue igual
```

- [X] **Step 2:** `go test ./pkg/vm -run 'TestORest|TestAnotac|TestAnotado' -v`.
  Esperado: FAIL.
- [X] **Step 3:** Implementar conforme Interfaces.
- [X] **Step 4:** `go test ./...`. Esperado: PASS.
- [X] **Step 5: Commit** — `feat(vm): objeto oRest e @Get(endpoint=..., :param)`

---

### Task 7: filtro OData, `TamSx3` e `FWAdapterBaseV2`

**Files:**
- Create: `pkg/vm/odata_filter.go`, `pkg/vm/adapterv2.go`
- Modify: `pkg/vm/natives.go` (`TAMSX3`)
- Test: `pkg/vm/odata_filter_test.go`, `pkg/vm/adapterv2_test.go`,
  `tests/rest_totvs/armazens_repo.tlpp`, `tests/rest_totvs/armazens_api.tlpp`,
  `tests/rest_totvs/armazens_fixture.sql`

**Interfaces:**
- Consumes: Task 1 (`registerNativeBase("FWADAPTERBASEV2", ...)`), Tasks 5 e 6 (o
  adapter é chamado de dentro de um `WSMETHOD` ou de um `@Get`).
- Produces:

```go
// odata_filter.go — Go puro, sem VM
type MapaCampo struct{ Prop, Coluna, Tipo string; Filtra, Ordena bool }
func TraduzFiltro(filtro string, mapa map[string]MapaCampo) (sql string, args []any, err error)
// gramática: expr := termo { ("and"|"or") termo }; termo := "(" expr ")" | "not" termo | prop op valor | funcao
//  op: eq ne gt ge lt le; valor: 'texto' (aspas dobradas = aspa) | número | true/false | null
//  funcao: contains(prop,'x') | startswith(prop,'x') | endswith(prop,'x')  -> LIKE ? com escape
func TraduzOrdem(ordem string, mapa map[string]MapaCampo) (string, error)    // "-nome,codigo" -> "COL_NOME DESC, COL_COD ASC"
func TraduzCampos(campos string, mapa map[string]MapaCampo) ([]string, error) // "a,b" -> colunas; vazio = todas
```

  - Na `FWAdapterBaseV2`, todos estes métodos são obrigatórios:
    - `New(cVerbo, lList)` e `AddMapFields(cProp, cCampo, lFiltra, lOrdena,
      aStruct)`. O `aStruct` é `{campo, tipo, tamanho, decimais}`, e o tipo dele
      define a conversão no JSON. A semântica exata dos argumentos 3 e 4 deve ser
      confirmada no corpus (comentários, uso com `.F.`) antes do Step 3, e o que for
      confirmado vai num comentário no código.
    - `SetPage`, `SetPageSize` (padrões 1 e 10), `SetQuery` (com `#QueryFields#` e
      `#QueryWhere#`), `SetWhere`, `SetOrder`, `SetOrderQuery`,
      `SetUrlFilter(aFiltro)` e `SetFields`.
    - `GetFixQuery`, `Execute`, `FillGetResponse`, `GetJsonResponse`,
      `GetErrorMessage` e `HasErrorMessage`.
  - O formato de `aFiltro` (`{{"FILTER", cFiltro}}`, como no corpus) e os nomes de
    query lidos sem filtro explícito (`page`, `pageSize`, `filter`, `order`, `fields`)
    vêm da requisição corrente (Tasks 5 e 6).
  - Saída de `GetJsonResponse`: `{"items":[...], "hasNext":bool}`.
  - Paginação: busca `pageSize+1` linhas para calcular o `hasNext`, com `LIMIT`/`OFFSET`
    parametrizados.
  - `TamSx3(cCampo)` devolve `{nTamanho, nDecimais, cTipo}` lido do SX3 local. Campo
    inexistente devolve `{0, 0, ""}`.
- Erro (filtro inválido, campo não mapeado): `Execute()` devolve `.F.` e
  `GetErrorMessage()` traz o motivo. O fixture REST responde 400 com essa mensagem.

- [X] **Step 1: Escrever os testes que falham**

```go
func TestFiltroSimples(t *testing.T)   // "filial eq '01' and codigo ge '000010'" -> "(NNR_FILIAL = ? AND NNR_CODIGO >= ?)", args ["01","000010"]
func TestFiltroFuncoes(t *testing.T)   // "contains(descri,'50%')" -> "NNR_DESCRI LIKE ? ESCAPE '\\'", args ["%50\\%%"]
func TestFiltroHostil(t *testing.T)    // casos: "descri eq 'x'' or 1=1 --'" (vira um só valor literal), "descri eq 'x'; drop", "senha eq '1'" (não mapeado), "(descri eq 'x'" -> err != nil exceto o 1º, cujo único arg é "x' or 1=1 --"
func TestOrdemECampos(t *testing.T)    // "-descri" -> "NNR_DESCRI DESC"; campos "codigo,xyz" -> erro (xyz não mapeado)
func TestTamSx3(t *testing.T)          // SX3 de fixture NNR_CODIGO C 2 0 -> {2,0,"C"}; "XXX_NADA" -> {0,0,""}
func TestAdapterListaPaginada(t *testing.T) // fixture com 25 armazéns; GET /rest/api/est/v1/armazens?page=3&pageSize=10 -> 5 items, hasNext=false; page=1 -> 10 items, hasNext=true
func TestAdapterFiltroEOrdemPelaURL(t *testing.T) // ?filter=descri eq 'Central'&order=-codigo -> só os de "Central", ordem decrescente
func TestAdapterCampoNaoMapeado400(t *testing.T)   // ?order=senha -> 400, message cita "senha"
```

- [X] **Step 2:** `go test ./pkg/vm -run 'TestFiltro|TestOrdem|TestTamSx3|TestAdapter' -v`.
  Esperado: FAIL.
- [X] **Step 3:** Implementar o filtro, depois o `TamSx3`, depois o adapter. Os
  fixtures `armazens_*.tlpp` são escritos do zero, imitando a forma do corpus
  (`Class ... FROM FWAdapterBaseV2`, `_Super:New`, `#QueryFields#`), sem copiar código
  TOTVS.
- [X] **Step 4:** `go test ./...`. Esperado: PASS.
- [X] **Step 5: Commit** — `feat(vm): FWAdapterBaseV2, TamSx3 e filtro OData
  parametrizado`

---

### Task 8: `FwApiManager` (leitura)

**Files:**
- Create: `pkg/vm/apimanager.go`
- Test: `pkg/vm/apimanager_test.go`, `tests/rest_totvs/apimanager_api.prw`

**Interfaces:**
- Consumes: Task 1 (`registerNativeBase("FWAPIMANAGER", ...)`), Task 7
  (`TraduzFiltro`/`TraduzOrdem`/`TraduzCampos`).
- Produces, por ordem de uso no corpus:
  - `New(cServico, cVersao)` (2 args);
  - `SetApiAlias(cAlias)`, `SetApiMap(aMapa)`, `SetApiRelation(...)`,
    `SetApiFilter(...)`, `SetApiQString(aQueryString)`, `SetQuery`;
  - `Activate()`, `GetJsonSerialize()`, `ToObjectJson()`, `ToArray()`, `GetJsonObject()`,
    `SetJsonError`/`GetJsonError`/`oJsonError`, `IsActive()` e `Destroy()`.
  - O formato de `aMapa` e de `SetApiRelation` sai do corpus no Step 3. O de
    `SetApiRelation` cobre a relação mestre-detalhe por campos de junção.
  - `ToExecAuto()` devolve erro de runtime "FwApiManager:ToExecAuto não suportado no
    AdvPP (requer ExecAuto/MVC)". É uma lacuna conhecida, registrada no
    `COMPONENT_STATUS.md` (53 usos no corpus).

- [X] **Step 1: Escrever os testes que falham**

```go
func TestApiManagerListaComFiltro(t *testing.T) // SetApiAlias("NNR")+SetApiMap+SetApiQString({{"filter","codigo eq '01'"}})+Activate -> GetJsonSerialize com items[0].codigo == "01" e hasNext
func TestApiManagerErroJson(t *testing.T)       // SetJsonError(400,"x") -> GetJsonError() devolve {"code":400,"message":"x"}
func TestApiManagerToExecAutoErroClaro(t *testing.T) // erro contém "não suportado"
```

- [X] **Step 2:** `go test ./pkg/vm -run TestApiManager -v`. Esperado: FAIL.
- [X] **Step 3:** Levantar no corpus a forma de `aMapa`/`SetApiRelation` (ler 3
  fontes que usam `SetApiMap`) e implementar.
- [X] **Step 4:** `go test ./...`. Esperado: PASS.
- [X] **Step 5: Commit** — `feat(vm): FwApiManager (leitura)`

---

### Task 9: REST montado no `advplc serve`

**Files:**
- Create: `cmd/advplc/serve_rest.go`
- Modify: `cmd/advplc/main.go` (`serveFile` e o caminho de diretório da Task 2)
- Modify: `pkg/webui/server.go` (`Server.MountREST(prefixo string, h http.Handler)`)
- Modify: `pkg/tools/shared/config.go` (`RestPrefix`)
- Test: `cmd/advplc/serve_rest_test.go`

**Interfaces:**
- Consumes: Tasks 4 a 6.
- Produces:
  - `func (srv *webui.Server) MountREST(prefixo string, h http.Handler)`: rotas sob o
    prefixo passam pela mesma checagem de host/origem (`allowedHost`) das demais.
  - No `serve`, uma VM "de registro" (`vm.NewVM(bc, false)` + `attachDatabase`)
    registra `@Get`/`WSRESTFUL` num `rest.Server` montado em `RestPrefix` (padrão
    `/rest`). Cada requisição usa `newChildVM` dessa VM.
  - Os erros de registro (conflitos da Task 4) são impressos no console do terminal no
    início, um por linha.
  - Com `--watch`, a recompilação troca o `rest.Server` atomicamente.
  - `serve` sem nenhuma rota REST não monta nada, então o comportamento atual se
    mantém.

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestServeMontaRestNoPrefixo(t *testing.T)     // serve de tests/rest_totvs/ -> GET /rest/api/est/v1/armazens -> 200 com "items"
func TestServeRestPrefixoConfigurado(t *testing.T) // RestPrefix "/api-local" -> rota responde em /api-local/..., 404 em /rest/...
func TestServeRestRespeitaHost(t *testing.T)       // Host: evil.example -> 403 (mesma regra do /ui)
func TestServeSemRestInalterado(t *testing.T)      // serve de tests/webui_test.prw -> GET /rest/x -> 404 e SPA normal em /
```

- [ ] **Step 2:** `go test ./cmd/advplc -run TestServe -v`. Esperado: FAIL.
- [ ] **Step 3:** Implementar conforme Interfaces.
- [ ] **Step 4:** `go test ./...`, mais um teste manual:
  `./advplc serve tests/rest_totvs --port 8099` e
  `curl -s localhost:8099/rest/api/est/v1/armazens | head -c 200`. Esperado: JSON com
  `items` e `hasNext`.
- [ ] **Step 5: Commit** — `feat(serve): REST TOTVS na mesma origem do advplc serve`

---

### Task 10: validação no corpus 12.1.2510 (local)

**Files:**
- Create: `tests/rest_totvs/corpus_test.go`, `tests/rest_totvs/corpus_fixtures/*.sql`
  (só esquema e dados fictícios das tabelas usadas; nenhum fonte TOTVS)

**Interfaces:**
- Consumes: tudo acima.
- O teste lê `ADVPP_CORPUS_2510` e faz `t.Skip("ADVPP_CORPUS_2510 não definida")` sem
  ela.

- [ ] **Step 1: Escrever os testes**

```go
func TestCorpusAdaptersCheck(t *testing.T)   // advplc check (via compilaComAvisos) nos 214 arquivos com FWAdapterBaseV2 -> 0 erros (lista dos que falham no t.Errorf)
func TestCorpusWSRestfulAmostra(t *testing.T) // amostra fixa de 500 arquivos com WSRESTFUL (semente 2510, ordenação alfabética) -> 0 erros de check
func TestCorpusCincoFontesHTTP(t *testing.T) // 5 fontes reais escolhidos no Step 2 (módulos diferentes; ≥1 oRest, ≥1 WSRESTFUL clássico, ≥1 FWAdapterBaseV2) carregados com CompileProject + fixture SQL -> GET documentado responde 200 com o formato esperado
```

- [ ] **Step 2:** Se `ADVPP_CORPUS_2510` estiver ausente ou inválida, marcar esta task
  como adiada, registrar o motivo no `COMPONENT_STATUS.md` e seguir para a Task 11
  com os testes de corpus em `SKIP`. Não implementar correções de corpus sem acesso
  real ao corpus.
- [ ] **Step 3:** Com o corpus disponível, escolher os 5 fontes (registrar os caminhos
  relativos ao corpus no teste) e escrever os fixtures SQL.
- [ ] **Step 4:** Rodar `ADVPP_CORPUS_2510="<caminho>" go test ./tests/rest_totvs -run
  TestCorpus -v -timeout 30m`. Cada falha é um bug real: corrigir na task de origem
  (com teste de regressão próprio, sem fonte TOTVS) até ficar verde. Se uma falha
  exigir função Protheus grande demais para esta onda, parar e escalar ao operador
  com a lista de pendências.
- [ ] **Step 5:** Rodar `go test ./...` sem a variável. Esperado: PASS, com os testes
  de corpus em `SKIP` quando a variável estiver ausente.
- [ ] **Step 6: Commit** — `test: validação REST TOTVS no corpus 12.1.2510 (local)`

---

### Task 11: documentação, revisão e encerramento da onda

**Files:**
- Modify: `docs/COMPONENT_STATUS.md` (seção "Recursos REST 2.0": tirar a limitação do
  `WSRESTFUL` clássico; listar `oRest`, `FWAdapterBaseV2`, `FwApiManager`, `TamSx3`,
  `_Super:`, `serve <diretório>`; lacunas: `ToExecAuto`, outros bancos fora do
  critério, validação de corpus pendente quando aplicável)
- Modify: `CHANGELOG.md` (entrada da onda 1)

- [ ] **Step 1:** Atualizar a documentação com o resultado da Task 10 quando o corpus
  estiver disponível. Se a Task 10 estiver adiada, registrar explicitamente que a
  validação no corpus permanece pendente e listar os testes cobertos sem corpus.
- [ ] **Step 2:** Confirmar a CI verde nos 3 SOs (Linux, Windows, macOS) e no job
  `determinism`, depois de autorização para push.
- [ ] **Step 3:** Commit e PR somente com pedido explícito do operador — `docs: onda
  1 PO-UI (REST TOTVS)`. Abrir o PR `feat/poui-onda1-rest` → `master`. A release
  segue a skill `advpp-release`, quando o operador pedir.
