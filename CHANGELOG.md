# Changelog

Todas as mudanças relevantes no projeto AdvPP.

## [Unreleased]

### Added
- **Onda 1 da compatibilidade PO-UI/TOTVS (REST)**: `advplc serve` agora
  monta as rotas REST do projeto na mesma origem do modo web, sob `/rest`
  (configurável via `rest_prefix`/`ADVPL_REST_PREFIX`), com a mesma
  checagem de host/origem — `webui.Server.MountREST`.
- DSL clássico `WSRESTFUL`/`WSMETHOD ... ENDWSRESTFUL` executa de verdade:
  verbo+`PATH`/`SERVICE` de cada método viram rotas REST com dispatch por
  classe, `aURLParms`, `SetRestFault` e `HTTPSetStatus`.
- Objeto `oRest` nas rotas anotadas (`@Get`/`@Post`/...): leitura de
  query/path/body/header e resposta via `setResponse`, `setStatusCode`,
  `setKeyHeaderResponse`, `updateKeyHeaderResponse` e `setFault`.
- `FWAdapterBaseV2` (herdável): `AddMapFields`, `SetQuery` com
  `#QueryFields#`/`#QueryWhere#`, `SetPage`/`SetPageSize`, `Execute`,
  `FillGetResponse`, `GetJsonResponse` (`items`+`hasNext`) — filtro
  OData (`eq/ne/gt/ge/lt/le`, `contains`/`startswith`/`endswith`,
  `null`, `orderBy`) traduzido para SQL parametrizado.
- `FwApiManager` (somente leitura): `SetApiAlias`, `SetApiMap`,
  `SetApiQString`, `SetQuery`, `Activate`, `GetJsonSerialize`,
  `ToObjectJson`, `ToArray`, `GetJsonObject`, `SetJsonError`/
  `GetJsonError`/`oJsonError`, `IsActive`, `Destroy`.
- `TamSx3(cCampo)`: metadados do campo (`nTamanho`, `nDecimais`, `cTipo`)
  lidos do SX3 local.
- `advplc serve <diretório>`: compila todos os fontes do diretório num
  único bytecode (projeto/RPO) e serve; `--watch` recompila e troca as
  rotas REST atomicamente.
- **Onda 2 da compatibilidade PO-UI/TOTVS (FwCallApp + apps externos)**:
  `FwCallApp` abre apps PO-UI do `@totvs/protheus-lib-core` em aba do
  navegador com a ponte `JsToAdvpl`/`AdvPLToJS` (`TWebChannel`), e
  `advplc serve --poui-app <dir|arquivo.app> [fonte]` hospeda um app
  externo (pasta ou `.app` em memória) com redirect da raiz, contexto de
  sessão e endpoints de framework (companies, branches, current-user,
  systemModules, session defaults, genericList/genericQuery).
- App de teste Angular real (`tests/poui/apps/fixture-app`,
  `@totvs/protheus-lib-core` 21.1.4 fixada) validado de ponta a ponta por
  `tests/poui/ao-vivo.sh fixture-app`: tabela REST, ping/pong via
  JsToAdvpl e fechamento de aba.
- **Validação contra o corpus real Protheus 12.1.2510**
  (`ADVPP_CORPUS_2510`): parser, preprocessor e lexer estendidos até
  **0 falhas** em 110 fontes com `FwCallApp`/`JsToAdvpl`, 225 com
  `FWAdapterBaseV2` e 500 `WSRESTFUL` na amostra — macros `#define`
  função-like e multilinha, grupos opcionais `[...]` repetidos, flags
  de GUI sem vírgula (`20 MEMO`, `)HSCROLL`), cláusulas WS em ordem
  arbitrária (`WSRECEIVE`/`WSSEND`/`QUERYPARAM`/`PATHPARAM`/
  `HEADERPARAM`), nomes de método iniciando com dígito (`1customers`),
  `json` como identificador AdvPL, `#define` sem expansão dentro de
  strings e `\` solto tolerado.
- `FwApiManager` no formato real do corpus: `aApiMap` aninhado
  (structs em `aApiMap[5]`, fields `{apiName, campo}`), `SetApiRelation`
  com pares `{colFilho, colPai}` e aninhamento pai→filho no JSON de
  resposta; `SetApiQString` interpreta `page`/`pageSize`/`order`/`fields`.
- OAuth2/MFA do framework em modo sessão local: `POST/GET
  /api/oauth2/v1/token` emite JWT `header.payload.` (userid/exp) no
  formato que `getTokenPayload()` do lib-core decodifica; MFA responde
  "desconfigurado" (`hasSecret=false`) para a lib pular o fluxo.

### Known limitations
- `FwApiManager:ToExecAuto` retorna erro explícito (requer ExecAuto/MVC).
- `FwCallApp` aceita e ignora os parâmetros 2–5 e 7 em diante — o corpus
  12.1.2510 só usa `cApp` (posição 1) e `cSource` (posição 6); fora do
  modo web retorna erro. `TWebEngine`/desktop não existe.
- MFA real (TOTP/segredo) não é implementado — os endpoints respondem
  "desconfigurado", o que é o comportamento correto para sessão local.

## [4.8.7] - 2026-11-22

### Fixed
- **Certificado A1 em `.pfx` (PKCS#12) não era lido**: `PFXInfo`,
  `PFXCert2PEM`, `PFXCA2PEM`, `PFXKey2PEM` e `PFX2PEM` só aceitavam PEM/DER
  com extensão `.pfx`, e `FWHTTPGet/Post/...` com certificado de cliente usava
  um decodificador que recusa PFX com mais de um certificado — justamente o
  caso do A1 da ICP-Brasil, que traz a cadeia. Agora os dois caminhos leem
  PKCS#12 de verdade (cifra legada 3DES/RC2, padrão do Windows), com a cadeia;
  o certificado de cliente vem primeiro e o TLS manda a cadeia inteira. PFX
  com cifra AES (padrão do OpenSSL 3) dá erro explicando como reexportar
  (`openssl pkcs12 -export -legacy`). Achado na NFS-e do GesCon.
- `PFXKey2PEM` e `PFX2PEM` gravam a chave privada com permissão 0600 (era
  0644).

## [4.8.6] - 2026-11-21

### Fixed
- **Erro de SQL em `TCSqlExec`/`TCSqlQuery` encerrava o programa**: restrição
  violada (NOT NULL, índice único), `RAISE(ABORT)` de gatilho ou tabela
  inexistente saíam da função nativa como erro fatal, que nem `Try/Catch` nem
  `ErrorBlock` + `Begin Sequence` capturavam. Agora o erro volta para o
  programa: `TCSqlExec` devolve `.F.`, `TCSqlQuery` devolve `{}`, e a mensagem
  fica em `TCSqlError()` (limpa a cada chamada bem-sucedida). O retorno segue
  lógico, não o número negativo do Protheus, porque o código AdvPP existente
  testa `If !TCSqlExec(...)`. Achado no GesCon (as travas por gatilho da
  tabela de cobranças derrubavam a operação).
- **`BEGIN; ...; COMMIT;` que falhava no meio deixava a transação aberta**:
  a conexão voltava ao pool com a transação pendente, e as escritas seguintes
  que pegavam essa conexão caíam dentro dela (nunca confirmadas: sumiam no fim
  do processo). `Exec` dos motores SQLite e remoto agora roda numa conexão
  dedicada e emite `ROLLBACK` nela quando o comando falha.

## [4.8.5] - 2026-11-21

### Fixed
- **`NULL` do Postgres (e de todo banco remoto) chegava como o texto `"Nil"`
  no `TCSqlQuery`**: a 4.8.4 corrigiu `cValToChar(Nil)`, mas o motor remoto
  (Postgres/Oracle/MSSQL) convertia o `NULL` com a representação de depuração
  do `Nil` antes de entregar a linha, e o campo chegava como `"Nil"` (tipo C).
  `Empty()` e `== ""` falhavam e a interface mostrava "Nil" em colunas vazias.
  Agora vira `""`, como sempre foi no motor SQLite. Achado rodando o ERP GEBAN
  sobre Postgres.

## [4.8.4] - 2026-11-21

### Added
- **Passagem por referência (`@var`)**: `Inc(@n)` agora altera a variável do
  chamador, como no Protheus. Antes o `@` era descartado pelo parser e o flag
  `ByRef` nunca usado — toda chamada era por valor, em silêncio. Vale para local,
  `Private`/`Public`, `Static` de arquivo, variável capturada por codeblock,
  repasse em cadeia (`A(@x)` → `B(@x)`), métodos e codeblocks (`Eval`). Nativas
  recebem o valor; as que devolvem algo em parâmetro `@` gravam nele
  (`PrivSignRSA(..., @cErrStr)` agora devolve a mensagem do erro). Opcodes novos
  `REF_LOCAL/REF_UPVAL/REF_GLOBAL/REF_DYN` no fim da tabela (bytecode antigo segue
  válido).

### Fixed
- **`#include` de fonte inexistente quebrava programa existente** (regressão da
  4.8.3, achada na suíte do GesCon): virou erro de compilação, e um include morto
  que sempre foi ignorado passou a impedir a execução. Agora é aviso com
  arquivo:linha no `advplc check` (texto e `--format json`), como os terminadores
  órfãos da 4.8.1, e o programa compila.
- **`Nil` convertido para texto virava a palavra `"Nil"`**: `cValToChar(Nil)`,
  `AllTrim(Nil)`, `Upper(Nil)`, `cValToChar(oJson["ausente"])`, JSON `null`, SQL
  `NULL` — um campo ausente passava por preenchido com "Nil". Nil é o nulo do
  AdvPL: em texto vira `""`. O depurador continua mostrando `Nil`.

## [4.8.3] - 2026-11-21

Correções de compatibilidade com o Protheus encontradas rodando o ERP GEBAN.
Todas mudam comportamento — código que dependia do comportamento antigo
(errado) pode precisar de ajuste.

### Fixed
- **`.And.`/`.Or.` sem curto-circuito**: os dois lados eram sempre avaliados, então
  `x != Nil .And. x:Foo()` quebrava com x Nil e `lOk := lOk .And. Migra()` chamava
  `Migra()` mesmo com lOk falso. Agora o lado direito só roda quando decide o
  resultado; o resultado continua lógico.
- **Aritmética e comparação de datas**: `SToD("20280201") + 90` dava `90` (número),
  `data - data` dava 0 e toda comparação entre datas errava (`ToFloat(data)` era 0:
  `dVenc < Date()` era sempre .F.). Agora `data ± n` = data, `data - data` = dias e
  `<`, `>`, `<=`, `>=` comparam as datas (data vazia é menor que qualquer outra).
- **`U_Nome()` não achava a `User Function Nome`** ("unknown function"). Agora vale a
  convenção do Protheus em chamadas, `RunFunction` (rotas REST, `StartJob`) e
  `FindFunction`: `U_X` é a User Function X; uma `User Function U_X` só é alcançada
  por `U_U_X` (antes as duas colidiam e a rota REST caía na errada).
- **`#include` aninhado**: resolvia só pelos caminhos de include e pelo diretório
  corrente, e falhava em silêncio — rodando de outro diretório, `src/A.prw` com
  `#include "B.prw"` perdia B e as funções dele viravam "unknown function" só ao
  executar. Agora procura primeiro ao lado do arquivo que inclui; o mesmo arquivo
  incluído por grafias diferentes entra uma vez (deduplicação pelo caminho
  absoluto); fonte incluído (`.prw/.prx/.prg/.tlpp`) inexistente é erro com
  arquivo:linha. Headers `.ch/.th` ausentes seguem tolerados.
- **`Transform` ignorava a picture**: `Transform(1234.5, "@E 999,999.99")` dava
  `1234.50`. Agora `@E`, `@R` (template de texto, ex. CNPJ), `@!`, separadores de
  milhar, alinhamento no tamanho do template e `*` quando o número não cabe.
- **`JsonObject:FromJson`** devolvia `.T./.F.`; como no Protheus, agora devolve `Nil`
  quando leu e o texto do erro quando não (código que testa `ValType(xErr) == "C"`
  não via o erro).

### Added
- **`ADVPP_WEBUI_HIDE_LOG=1`** no `advplc serve`: o `ConOut` não vai ao browser (só
  ao stdout do processo) e o painel "Log técnico" não aparece — ele mostrava detalhes
  internos (banco, migrações) a qualquer visitante de um sistema exposto. Sem linhas,
  o painel também deixa de aparecer.

## [4.8.2] - 2026-11-21

### Added
- **`PoButton` com ícone e destaque**: `oBtn:Set("icon", "an an-code")` mostra um ícone
  Animalia do PO-UI no botão e `oBtn:Set("kind", "primary"|"secondary"|"tertiary")` muda o
  destaque (padrão `secondary`, como antes). Exemplo em `examples/poui/fase1.prw`; teste de
  navegador em `tests/poui/ao-vivo.sh`.

## [4.8.1] - 2026-11-21

### Added
- **`advplc check` avisa terminador de bloco órfão**: `EndIf`/`Next`/`End`/`EndDo` (e
  afins) sem bloco aberto continuam compilando, como no Protheus real, mas agora saem como
  `Aviso: arquivo:linha:coluna: EndIf sem bloco aberto (ignorado)` no stderr e em
  `"warnings"` no `--format json` (`ok` segue `true`). Antes eram ignorados em silêncio,
  inclusive depois do `Return` de uma função. API Go: `parser.Aviso`, `(*Parser).Avisos()`.

### Fixed
- **Texto digitado se perdia ao clicar logo em seguida** (componentes PO-UI): o valor de
  `PoInput`/`PoPassword`/`PoTextarea`/`PoNumber`/`AdvCodeEditor` só ia ao programa 300 ms
  (500 ms no editor) depois da última tecla, e um clique nesse intervalo chegava antes —
  o handler lia o valor antigo. Agora qualquer evento descarrega a digitação pendente
  antes, e os `POST /ui` saem em fila, na ordem dos eventos. Teste de navegador em
  `tests/poui/ao-vivo.sh`.

## [4.8.0] - 2026-11-21

### Added
- **`AdvCodeEditor` e `AdvTerminal`** (fase 4 dos componentes): editor CodeMirror 6 com
  realce AdvPL/TLPP, marcações de erro, Ctrl+S (`OnSave`) e `OnChange`; terminal xterm.js
  sobre pseudo-terminal real (`creack/pty`, sem shell implícito), entrada, redimensionamento
  e `OnExit` — no Windows, erro claro nesta versão. Exemplo `examples/poui/fase4.prw`
  (mini IDE). Dependências novas: `github.com/creack/pty`; npm `codemirror`,
  `@codemirror/*`, `@xterm/xterm`, `@xterm/addon-fit`.
- **`advplc check --format json`** traz `line`, `col` e `message` quando o erro tem posição.
- **`advplc serve --apps <dir> [--home <nome>]` — vários programas num servidor** (fase 3
  dos componentes PO-UI): cada `<dir>/<nome>.prw|.tlpp` em `/app/<nome>`, uma VM por
  janela, compilação sob demanda com recompilação quando o fonte muda, erro de
  compilação isolado na janela do app, nome validado (sem path traversal).
  `PoApp:Open(cNome)` abre outro app numa janela nova; `ADVPP_APPS_DIR` disponível aos
  programas. API Go: `webui.NewApps`, `webui.AppResolver`. Exemplos em
  `examples/poui/apps/`.
- **Componentes PO-UI — fase 2**: `PoMenu` (menu lateral) e várias páginas por
  aplicação (`PoApp:Navigate`); `PoContainer`, `PoWidget`, `PoDivider`, `PoTabs`/`PoTab`;
  `PoTree`, `PoList`, `PoInfo`, `PoProgress`, `PoGauge`, `PoChart` (com `AddPoint` em
  janela deslizante); campos tipados `PoPassword`, `PoTextarea`, `PoNumber`, `PoSelect`,
  `PoCheckbox`, `PoSwitch`; grade de 12 colunas (`SetCols`). Retorno ao usuário:
  `PoApp:Notify`, `Confirm` (não bloqueante) e `Loading`. **`PoApp:Spawn`**: processo
  filho sem shell, com stdout/stderr linha a linha sem perda (contrapressão) e
  encerrado se a janela fechar. Exemplo: `examples/poui/fase2.prw`.
- `tests/poui/ao-vivo.sh`: teste de navegador (Chromium + DevTools) que confere que
  uma atualização vinda do servidor aparece na tela sem interação.
- **Componentes PO-UI orientados a eventos (fase 1)** no `advplc serve`: classes nativas
  `PoApp`, `PoPage`, `PoButton`, `PoInput`, `PoTable` e `PoTag`. O programa monta a tela
  como objetos, `PoApp:Activate()` entra num laço de eventos não bloqueante (handlers em
  codeblocks, patches de propriedade enviados em lote) e `PoApp:SetTimer` atualiza a
  tela sozinho. Protocolo: evento SSE `ui` (mount/batch/close) e rota `POST /ui`, com a
  mesma checagem de origem e limite de corpo de `/reply`. Os diálogos bloqueantes, o
  MSDIALOG e o FWMBrowse seguem iguais. Nomes `Po*` cedem a funções do próprio programa
  com o mesmo nome. Exemplo em `examples/poui/fase1.prw`; design em
  `docs/superpowers/specs/2026-11-20-poui-componentes-design.md`.

### Fixed
- **Parser aceitava argumentos sem vírgula e parêntese sem fechar**: `ConOut("oi" + x`
  seguido de outras linhas virava uma única chamada, com as linhas seguintes como
  argumentos, e o programa rodava imprimindo lixo. Agora é erro de compilação com
  posição, como no Protheus. Fixture `tests/stability_error_paths_test.prw` corrigido
  (usava `1_000_000`, que não é número em AdvPL).
- **GitHub Pages**: o build do site falhava porque o Liquid do Jekyll lia o código AdvPL
  dos documentos internos em `docs/superpowers/` como tags `{{ }}`. `docs/_config.yml`
  exclui essa pasta; o restante do site sai idêntico.
- **`Static` de arquivo**: qualquer `Static` declarado no nível do arquivo (comum no
  Protheus) fazia o compilador tratar o programa como "corpo" e a função de entrada nunca
  era chamada — o programa não fazia nada. E, mesmo chamada, uma função não enxergava o
  `Static` (lia uma variável dinâmica vazia). Agora o corpo inicializa os `Static` e a
  entrada é chamada; dentro das funções o `Static` é lido e gravado (inclusive em `For`).
  Em VM sem programa principal (ex.: função rodada isoladamente) ele vale NIL.
- **Componentes PO-UI — pendências das fases 1 e 2**: `PoList` sem detalhe não mostra
  mais linha vazia; `PoMenu:AddItem` com ícone inexistente no PO-UI embutido é erro claro
  (antes o item ficava sem ícone, em silêncio) e aceita `"an an-x"`, `"an-x"` ou `"x"`;
  `PoApp:Activate()` aninhado (de dentro de um evento) é recusado em vez de remontar a
  tela; `Get()` de propriedades estruturadas devolve arrays/`JsonObject` (antes NIL);
  `POST /ui` com a fila de eventos cheia responde 503 na hora (antes segurava a conexão
  até o cliente desistir); o alvo `make web`, citado no README, agora existe.
- **Componentes PO-UI (fase 1): atualizações do servidor não apareciam na tela** até a
  próxima interação do usuário — os patches mudavam objetos que o Angular não observa.
  Cada lote agora incrementa um signal lido no template.
- **`Val` lia o texto inteiro em vez do prefixo numérico**: `Val("2.88 3.11")` dava 0 e
  `Val("12abc")` dava 0; no Protheus são 2.88 e 12. Também deixou de aceitar expoente,
  `NaN`, `Inf` e hexadecimal (`Val("1e5")` é 1).
- **`#include` de fonte em CP-1252 chegava cru ao lexer.** Só o arquivo raiz era convertido
  para UTF-8; o conteúdo incluído não, então um `"ã"` vindo de um include nunca era igual ao
  `"ã"` do arquivo principal. A conversão saiu de `cmd/advplc` para
  `preprocessor.SourceToUTF8` e passou a valer para todo `#include` e também para o
  `advplc ast`, que tinha o mesmo problema.
- **`WaitRun` voltou a capturar saída com `> arquivo`.** Desde a correção de injeção de
  comando (CWE-78) ele executava sem shell, mas o `>` virava argumento e o padrão documentado
  `WaitRun("cmd > saida.txt")` parou de gravar o arquivo, sem nenhum erro. Agora os
  redirecionamentos `>`, `>>`, `<`, `2>`, `2>>` e `2>&1` são feitos em Go, e as aspas agrupam
  argumentos (antes `strings.Fields` quebrava `"meu arquivo.txt"`). Continua sem shell:
  `;`, `&&`, `|`, `$VAR` etc. viram texto dos argumentos. O `WinExec` usa a mesma divisão
  com aspas. A documentação, que ainda falava em `sh -c`, foi corrigida.
- **`GetEnv` de variável inexistente devolvia `"Nil"`** em vez de `""`: o default ausente
  passava por `ToString(NIL)`. Com isso `!Empty(GetEnv("X"))` era verdadeiro para `X` não
  definida.

## [4.7.1] - 2026-11-20

### Fixed
- **Pacotes da release corrompidos.** Os jobs `cli-cross` e `native` do
  `.github/workflows/release.yml` publicavam arquivos com o mesmo nome
  (`advpp-<versão>-linux-amd64.tar.gz`, `-windows-amd64.zip`, `-darwin-arm64.tar.gz`) e o
  `download-artifact` com `merge-multiple` gravava os dois no mesmo caminho, misturando os
  bytes: a v4.6.1 saiu com "trailing garbage", a v4.7.0 com tar.gz ilegível — com o
  workflow verde. Os pacotes só-CLI agora se chamam `advpp-cli-<versão>-<so>-<arch>`
  (linux-amd64, linux-arm64, darwin-arm64, windows-amd64), como já documentado em
  `docs/MANUAL_ADVPLC.md`; os pacotes `advpp-<versão>-<plataforma>` são a suíte completa.
- **`install.sh` não funcionava:** baixava `advpp-cli-<versão>-…tar.gz`, nome que nenhuma
  release publicava; no macOS o binário dentro do pacote ainda se chamava `advplc-darwin`.
- **`make package` apagava o `./advplc` da raiz do repositório** (build e `rm -f` no
  diretório raiz). Agora compila em `dist/cli-<so>-<arch>/`.
- **Release:** novo passo "Integridade dos pacotes" (`gzip -t`/`tar`/`unzip -t`/`dpkg-deb`)
  falha o workflow em pacote corrompido; criação da release não engole mais erro (antes,
  `2>/dev/null` + loop terminando em `sleep` deixava o passo verde mesmo falhando); a tag
  não é mais apagada e recriada a partir do HEAD do `master` (`--verify-tag`).

## [4.7.0] - 2026-11-20

### Added
- **Garantia de compatibilidade Linux/Windows/macOS na CI** (`.github/workflows/test.yml`):
  matriz ganha `ubuntu-24.04-arm` (linux-arm64 já era embutido na extensão VS Code sem
  ser testado); cross-compile (`vet` + `build`) dos 6 alvos do CLI; job `determinism`
  exige sha256 idêntico do bytecode de todo fonte de `examples/` e `tests/` nos 4 runners
  (`TestBytecodeDeterminism`, `cmd/advplc/bytecode_determinism_test.go`).
- **`ADVPL_SERVE_HOST`**: host do `advplc serve` configurável (padrão `localhost`).
- **`ADVPP_WEBUI_ALLOWED_HOSTS`**: nomes extras aceitos no `Host` do `advplc serve`.
- Fuzz tests: `FuzzPipeline` (`pkg/parser`) e `FuzzParse` (`pkg/rpo`).
- Relatório `docs/security/SECURITY_AUDIT_2026_11_20.md`.

### Changed
- `.gitattributes`: fontes AdvPL/TLPP (`*.prw`, `*.tlpp`, `*.ch`, `*.th`, `*.prx`,
  `*.prg`) como `-text` — checkout no Windows não converte mais LF→CRLF (o fim de linha
  entra no bytecode via string `[colchetes]` multi-linha).
- `ci.yml` removido: duplicava o `test.yml` e o lint tinha `continue-on-error` (nunca
  falhava). `test.yml` é o workflow único de validação.
- **Comportamento:** `advplc serve` acessado pela rede por **nome** de máquina precisa
  desse nome em `ADVPP_WEBUI_ALLOWED_HOSTS` (IP literal e `localhost` seguem aceitos).

### Security
- **`advplc serve`: bloqueio de CSRF e DNS rebinding.** Qualquer página aberta no
  browser podia disparar `GET /events` (o programa executa a cada requisição) e, via
  DNS rebinding, ler eventos e responder diálogos. Agora o `Host` precisa ser
  loopback/IP literal (ou `ADVPL_SERVE_HOST` / `ADVPP_WEBUI_ALLOWED_HOSTS`, separados
  por vírgula) e o `Origin`, quando presente, o mesmo host:porta. Também: `/reply` só
  aceita POST com corpo ≤ 8 MB, teto de 64 sessões simultâneas, `ReadHeaderTimeout`.
  Teste: `pkg/webui/origin_test.go`.
- **DAP (`--debug-port`): requisição HTTP cross-protocol rejeitada.** O leitor de
  frames ignorava linhas desconhecidas, então um `fetch()` de página web para a porta
  TCP virava comando DAP (inclusive `launch`). Agora só `Content-Length`/`Content-Type`
  são aceitos e o corpo tem teto de 16 MB (antes, `make()` sem limite → OOM).
- **GGUF malformado não derruba mais o processo.** Contagens/tamanhos do header
  limitados ao tamanho do arquivo, `nDims ≤ 4`, arrays aninhados ≤ 8,
  `general.alignment = 0` vira erro, tensores fora do arquivo checados antes de alocar.
- **`advplc exec` valida o `.bc`.** `numGlobals`/`NumLocals`/offsets negativos ou
  gigantes causavam pânico ou OOM fatal; agora viram erro de carga.

### Fixed
- **`advplc compile` falhava com `json: unsupported value: +Inf`** quando um literal
  estoura `float64` (ex.: `1.8e308` em `tests/stability_edge_case_matrix_test.prw`).
  `Constant` serializa ±Inf/NaN como string; `.bc` antigos continuam carregando.
  Achado pelo job de determinismo. Teste: `TestBytecodeNonFiniteConstants`.
- **Preprocessor: `#command` com `[` sem `]` causava pânico** (slice out of range em
  `compilePattern`). Achado por fuzzing; fuzz tests novos em `pkg/parser/fuzz_test.go`
  (preprocessor→lexer→parser) e `pkg/rpo/fuzz_test.go` (RPO/APO).

## [4.6.1] - 2026-11-07

### Fixed
- **`TCSQLEXEC` e `TCSQLQUERY` falavam com o engine local (SQLite) mesmo com
  `DbConnection:Connect()` bem-sucedido.** Os dois natives liam `v.dbEngine`, que
  só muda com `DBSetDriver`. Agora usam o helper `tcSQLEngine()`, que prefere a
  conexão remota aberta pela própria VM (`ownConnID`) e só então o engine local.
  Nunca usa a conexão ativa global de outra sessão. Conexão própria fechada cai no
  engine local. Testes em `pkg/vm/tcsql_routing_test.go`.

## [4.6.0] - 2026-10-11

### Added
- **Injeção de bytecode RPO (`advplc rpo inject`).** Parser de framing APO, injetor (`pkg/rpo/injector`: `injector.go`, `reencrypt.go`, `admin_index.go`), handshake e codec do AppServer (`pkg/rpo/wire`) e sessão DAP (`pkg/rpo/dap`). Testes em `pkg/rpo/...`.
- **Scripts de apoio ao RPO:** `tools/deploy-rpo.sh`, `tools/build-integration/auto-inject.sh`, `tools/build-integration/inject-bytecode.sh`, `tools/rpo-live-inspect/capture_and_inject.sh`.
- **Suítes TIR de SA1 e SC5** e setup do TIR (`tests/tir/`).
- **Documentação da injeção RPO** em `docs/RPO-*.md` e `docs/rpo-seeds/`.

### Changed
- `tools/vscode-advpl/package.json` acompanha a versão 4.6.0.

## [4.4.6] - 2026-10-07

### Added
- **`advplc check --format json` (M1).** Um objeto JSON por linha (`{"file":…,"ok":true|false,"error"?…}`), ordem estável de entrada, erros no próprio JSON via stdout, exit 0/1 igual ao humano, formato desconhecido falha alto. Pensado para agentes (PiG/pig-advpp) e CI parseável; modo humano inalterado. Testes: `TestRunCheckJSONMixed`, `TestRunCheckJSONAllOK`, `TestParseOptionsFormat` (`cmd/advplc/check_format_test.go`).
- **Botão "Perguntar ao PiG" no advpp-ide (Passo 3).** Menu Tools → diálogo → `pig -p --mode json --no-session` em goroutine com o arquivo em contexto (teto 12k chars), resposta final extraída do JSONL (`message_end` autoritativo, `turn_end` fallback) no console. Config por `ADVPP_PIG_BIN/MODEL/TIMEOUT_SECS`; sem `-a` automático (trust é do usuário). `OutputConsole.Append` com mutex (fyne 2.4.4 não tem `fyne.Do`). Testes: parser (incl. contra saída real do pig), prompt-truncate (`cmd/advpp-ide/agent_test.go`).
- **Teto de geração do LLM configurável.** Não havia watchdog nenhum (a tabela 2.0.3 citava 5 min inexistentes): deadline no prefill e por token, default moderno de 30 min, `ADVPP_LLM_TIMEOUT_SECS` sobrescreve (`0` = sem teto), erro capturável via `Try/Catch`. Teste: `TestLlmTimeoutSeconds`.
- **Supressão de pensamento (técnicas do PiG em inferência crua).** `pkg/llm/thinking.go`: fechar `<think>` no prefill + remover blocos da saída; método `LLM:SuprimePensamento([lAtivo])`, default desligado. Testes: `TestCloseThinkingPrefill`, `TestStripThinking`.
- **Leitura de linha Q6_K, clamp de RoPE, HeadDim do tensor, atenção larga, fallback tied-embeddings, default `eps=1e-6`, `TensorNames()`, tokens especiais no Encode, flag `ComBOS`, infra QK-norm (qwen2/qwen3).** Robustez do motor para arquivos reais fora do par validado (ex.: MiniCPM5-1B, Llama-3.2-3B). Suite `pkg/llm` verde.

### Fixed
- **Tabela de limites de recursos modernizada** (era 2.0.3): timeout do LLM agora real e configurável (ver acima).
- **`GetEnv` sem default retornava a string `"Nil"`** (de `ToString(nil)`), que `Empty()` não pega — fallback de PATH nunca acionava. Forma suportada: `GetEnv("VAR", "")`.

### Em investigação
- **Qwen3 gera degenerado no motor nativo** (distribuição flat; referência dá Paris no mesmo prompt). Carrega sem crash; isolado com harness de logits, ablação de QK-norm, sweep de escala e leitura do graph no fonte do llama.cpp — dossiê em notas Joplin (`Qwen3 divergencia comprovada`). Llama-3.2-3B e MiniCPM íntegros e sem regressão.

## [4.4.5] - 2026-10-04

### Added
- **`advplc rpo apo`** — desmontagem de blobs APO extraídos do RPO: identificadores, literais, snippets de código e call-graph candidato, com relatório JSON/Markdown (`--catalog`, `--out`, `--format`). Parser do framing APO (magic `75 00 00 46 46` + kind `F`/`T`) validado contra 4 fixtures reais — 3.465/3.465 blobs do catálogo de teste (977.618 identificadores, 124.944 candidatos). Testes: `pkg/rpo/apo_blob_test.go`, `cmd/advplc/cmd_rpo_apo_test.go`.
- **`advplc rpo pull`** — extração ao vivo de recursos do RPO via sessão DAP (`GetApoRes`): lista por manifesto (`--manifest`) ou listing `--res`, saída em `--out`, senha via prompt ou `ADVPP_RPO_PASS` (nunca hardcoded). Smoke comprovado: recurso real baixado byte-a-byte idêntico (md5 conferido, evidência em `docs/rpo-evidence/rpo-pull-smoke.log`). Testes: `pkg/rpo/wire/*_test.go`, `pkg/rpo/dap/*_test.go`, `cmd/advplc/cmd_rpo_pull_test.go`.
- **Pacotes `pkg/rpo/wire` e `pkg/rpo/dap`** — codec de frames do protocolo do AppServer (`<IHIIHH>`, magic `0xab21`, zlib opcional), handshake autenticado de 5 mensagens (validado ao vivo contra build `7.00.210324P`), banner de identificação, decode de resultado DAP (aspas/base64/`NIL`); RPC stdio LSP/DAP com framing `Content-Length`, obtenção de token via Language Server e sessão de depuração (launch → breakpoints → stop → evaluate).
- **`docs/RPO-EXTRACTION-METHODOLOGY.md`** — documentação canônica das técnicas de extração RPO (T1–T14 com status de evidência, protocolo wire/DAP, métricas, gotchas e análise de gaps) + evidências curadas em `docs/rpo-evidence/`.

### Fixed
- `advplc rpo inject` executava o bloco duas vezes (bloco órfão pós-switch removido). (2a3996a)
- Handshake do AppServer com deadline por passo (um deadline único de 1s estourava o RTT via proxy de captura). (a407683)

### Security
- Credenciais do operador redigidas em documentos e comentários do repositório (`[REDACTED]`); senha do AppServer nunca presente em log ou artefato. (d7cc702)
## [4.4.0] - 2026-10-03

### Fixed
- **Isolamento entre sessões em banco remoto (multiempresa por `search_path`).** Cada conexão aberta por `DbConnection:Connect()` fica presa a uma conexão física (`SetMaxOpenConns(1)`): `SET search_path` passa a valer para todo comando seguinte daquela sessão — antes o pool do `database/sql` espalhava os comandos entre conexões. Conexão reposta pelo driver nasce em `public` (falha fechada: "tabela não existe"). Teste: `TestPinPool`. (073024c)
- **Cada VM usa a conexão que ela mesma abriu.** `DBSetDriver("TOPCONN")` pegava a conexão "ativa" global do processo — a da última sessão que conectou —, misturando sessões do `advplc serve`. Teste: `TestApplyRDDUsesOwnConnection`. Ao fim de cada sessão do `serve` as conexões dela são fechadas (`TestCloseOwnedConnections`). (247cb7b)
- **Requisição REST, `StartJob`, `FWJobStart`, `FWGridProcess`, gRPC e MCP ganham conexão própria.** A requisição REST compartilhava o engine do pai (requisições concorrentes trocavam a empresa umas das outras) e os jobs caíam no SQLite local vazio quando o banco era remoto. Agora cada VM-filha recebe um clone da configuração do pai, devolvido ao terminar. Testes: `TestChildVMClonesRemote`, `TestChildVMLocalEngineKeepsFactory`, `TestChildVMInheritsRDD`; prova ponta a ponta no GEBAN (`T_GEBISO`, 6 workers concorrentes em 2 empresas, Postgres real). (bfbda8c)

- **Sessão do `advplc serve` abandonada não segura mais a conexão.** Browser que recarrega ou fecha com menu/diálogo/browse aberto deixava a goroutine da sessão presa para sempre, com a conexão de banco dela. Agora, sem nenhum browser conectado por `ADVPP_WEBUI_ABANDON_SECONDS` (padrão 300), a sessão é encerrada e as conexões fechadas. Testes: `TestAbandonReleasesBlockedDialog`, `TestAbandonDropsOutput`.
- VM sem conexão própria fica no engine local (falha fechada) em vez de herdar a "ativa" global de outra sessão (`TestApplyRDDNoOwnConnDoesNotTakeOthers`); `EVAL`/`AEVAL`/ações de `MSDIALOG` reaproveitam a conexão do pai em vez de abrir uma por avaliação (`TestSharedChildVMDoesNotClone`); o estado `DB*` de cada VM-filha é liberado ao terminar (`TestChildVMDoneReleasesState`).

### Added
- `ADVPP_MAX_REMOTE_CONNS` (padrão 50): limite de conexões remotas abertas por processo; acima dele, erro `limite de conexoes remotas atingido (N)`. Teste: `TestRemoteSlots`. (073024c)
- `FWMBrowse:SetMenuDef("")` deixa o browse somente leitura: a tela esconde Incluir/Editar/Excluir e o servidor recusa `save`/`delete` mesmo se o cliente enviar. Teste: `TestBrowseReadOnlyRejects`. (dc1152c)

## [4.3.2] - 2026-10-01

### Fixed
- `FWMBrowse` funcionava no SQLite e morria no Postgres remoto: `browseItems`/`browseSave`/`browseDelete` (`pkg/vm/browse.go`) endereçavam a linha pelo pseudo-campo `rowid`, que não existe no Postgres (nem no Oracle/MSSQL) — qualquer `Activate()` contra banco remoto abortava a sessão com erro. A coluna-chave agora é resolvida por `browseKeyColumn`: `R_E_C_N_O_` (coluna real nos dois motores — no SQLite ela É o rowid) quando a tabela tem, `rowid` como fallback para tabelas sem `R_E_C_N_O_`. Validado com `go test ./pkg/vm/ -run Browse` (7 PASS, 2 testes novos).

## [4.3.1] - 2026-09-30

### Added
- ReplaceAPO dinâmico com suporte a mudanças de tamanho
- Recriptografia automática após modificação
- Heurística aprimorada de identificação de ciphers
- Testes unitários para injector (7 testes)
- Makefile com targets RPO
- Scripts de automação (capture_and_inject.sh)

### Changed
- Parser APO com validação melhorada
- Documentation atualizada
- Error handling aprimorado

### Fixed
- Bug de compilação `-compile` (falta RPODB no INI)
- Problemas de slice bounds no parser
- Tratamento de edge cases na reconstrução do body

### Performance
- Parse RPO 22KB: 40µs
- Parse RPO 9.5MB: 9.6ms
- Memória otimizada

## [4.3.0] - 2026-09-30

### Added
- Parser completo de estrutura RPO
- Parser de registros APO
- CLI `advplc rpo inject`
- Support para 24+ ciphers OpenSSL
- Sistema de captura LD_PRELOAD

### Changed
- Refatoração do código para modularidade
- Melhorias na documentação

## [4.2.0] - 2026-09-29

### Added
- Heurística de identificação de cipher
- Mapeamento key/IV size → cipher
- Testes de heurística

## [4.1.0] - 2026-09-28

### Added
- Serializer JSON→binário
- Geração de bytecodes
- Comandos CLI básicos

## [4.0.0] - 2026-09-26

### Added
- Infraestrutura inicial
- Parser RPO básico
- Hook LD_PRELOAD
- Testes unitários

---

## Notas de Versão

### 4.3.1
- **Destaque:** ReplaceAPO dinâmico
- **Testes:** 17/17 passing
- **Performance:** <10ms para RPOs até 10MB

### 4.3.0
- **Destaque:** Parser completo
- **Novo:** Suporte a múltiplos ciphers
- **Testes:** Infraestrutura de testes

---

*Changelog do projeto AdvPP*
