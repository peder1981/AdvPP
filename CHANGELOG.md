# Changelog

Todas as mudanças notáveis deste projeto são documentadas aqui.

## [1.4.0] — 2026-07-09

### Motor de inferência LLM embutido (`pkg/llm` + classe `LLM`)

Novo: um motor de inferência para modelos de linguagem quantizados em
**I2_S** (ternário, formato BitNet), escrito 100% em Go — sem CGO, sem
`llama.cpp`, sem dependências de terceiros — compilando e rodando
identicamente em Linux, Windows e macOS (amd64 e arm64). Validado
**token a token** contra o `llama.cpp` de referência (fork BitNet do
projeto) usando o modelo `Falcon3-3B-Instruct-1.58bit`.

- **Parser GGUF** (`pkg/llm/gguf.go`): header, metadados e tensores lidos
  sob demanda (não carrega o arquivo inteiro em memória de uma vez).
- **Kernel ternário I2_S** (`pkg/llm/i2s.go`): dequantização e matmul
  contra ativações int8, replicando byte a byte o algoritmo de
  `ggml-quants.c`.
- **SIMD AVX2** (`pkg/llm/simd_amd64.s`, amd64): o dot-product ternário
  em assembly Go (VPMADDUBSW/VPSRLW), com detecção de CPU em runtime via
  CPUID e fallback automático para o caminho escalar em CPUs sem AVX2 —
  ou em qualquer arquitetura fora de amd64 (arm64 usa o escalar puro já
  validado; sem assembly não testável nesta arquitetura).
- **Forward pass completo** (`pkg/llm/model.go`): transformer arquitetura
  "llama" (GQA, RoPE, RMSNorm, FFN SwiGLU) com KV cache incremental.
- **Tokenizer BPE** (`pkg/llm/tokenizer.go`): byte-level estilo GPT-2,
  usando o vocabulário/merges já embutidos no próprio GGUF.
- **Amostragem** (`pkg/llm/sampling.go`): greedy, temperatura, top-k, top-p.
- **Classe AdvPL/TLPP `LLM`** (`pkg/vm/llm_native.go`): expõe o motor
  como native, no mesmo padrão de `FWMBrowse`/`MsDialog`:
  ```advpl
  oLLM := LLM():New("/caminho/modelo-i2_s.gguf")
  cTexto := oLLM:Generate("The capital of France is", 6, 0)  // prompt, nMaxTokens, nTemperatura
  aTokens := oLLM:Tokenize("algum texto")
  cTexto := oLLM:Decode(aTokens)
  oLLM:Close()
  ```

**Desempenho** (Falcon3-3B-1.58bit, 8 núcleos): ~5s/token com
paralelização por goroutines (matmul e atenção por faixa de
linhas/cabeças) + caminho rápido sem checagem de limite para blocos
ternários completos; AVX2 reduz mais ~1.6x sobre isso em amd64.

**Limitações conhecidas**: só arquitetura GGUF `"llama"` com pesos I2_S
(não `bitnet-b1.58` com as normas extras "SubLN"); pré-tokenizador
simplificado (não replica o split dígito-a-dígito específico da
Falcon3 — só afeta números com mais de um dígito); sem streaming
token-a-token na classe `LLM` (bloqueia até `Generate()` terminar); sem
suporte a outras quantizações (Q4_K, Q6_K etc.) nem outras arquiteturas.

## [1.3.0] — 2026-07-09

### Renderer web (`advplc serve`) — fases 1 a 4

Novo modo de execução: o programa AdvPL/TLPP roda no servidor (mesma VM,
mesmo `ADVPP.db`) e a interface é renderizada no browser. Basta o binário
`advplc` e um navegador — sem SmartClient, sem executável gráfico.

- **Fase 1 — console e diálogos**: `advplc serve <fonte> [--port N]`.
  `ConOut` é transmitido em tempo real; `MsgInfo`/`MsgStop`/`MsgAlert`/
  `MsgYesNo` bloqueiam a execução até a resposta do usuário no browser.
  Protocolo SSE + POST (stdlib pura, sem WebSocket). Cada aba/recarga é
  uma sessão com VM isolada e conexão própria ao banco.
- **Fase 2 — MVC → PO-UI**: frontend **PO-UI/Angular** (TOTVS) embutido
  no binário via `embed.FS`. `FWMBrowse():New()` + `SetAlias("SA1")` +
  `Activate()` renderiza um **`po-table`** com colunas e títulos vindos
  do dicionário **SX3** do `ADVPP.db`; Incluir/Editar abrem um
  **`po-dynamic-form`** gerado do dicionário; exclusão é soft-delete
  padrão Protheus (`D_E_L_E_T_='*'`). CRUD persistido no SQLite.
- **Fase 3 — hot reload**: `advplc serve <fonte> --watch` recompila a
  cada alteração do fonte e recarrega as sessões do browser
  automaticamente; erro de compilação aparece no console do browser.
- **Fase 4 — MSDIALOG legado**: `DEFINE MSDIALOG` + `@ linha,coluna
  SAY/GET/BUTTON` + `ACTIVATE MSDIALOG` viram um modal PO-UI por
  heurística de grade (controles agrupados em linhas por proximidade de
  `y`). O valor digitado nos `GET`s **escreve de volta nas variáveis**
  do programa (novo `FunctionInfo.LocalNames` no bytecode). `ACTION` de
  botão executa em VM isolada; `VALID`/`WHEN`/`ACTION` agora são lazy
  (embrulhados em codeblock, como o `#xcommand` real do Protheus).

### Infra

- `webui_port` na configuração compartilhada (`~/.advpp/advpp_config.json`);
  precedência: `--port` → config → 8080. Diretiva do projeto: toda nova
  configuração entra na Config compartilhada para futura edição via AdvCfg.
- Novo alvo `make web`: recompila o frontend PO-UI e embute em
  `pkg/webui/dist` (o dist é versionado — `go build` funciona sem Node).
- `SQLiteEngine` ganhou `QueryRows`/`Exec` (interface `vm.SQLEngine`).
- Fixtures novos: `tests/webui_test.prw`, `tests/mvc_browse_test.prw`,
  `tests/msdialog_test.prw`.

### Limitações conhecidas (fase 4)

- Codeblocks deste runtime não capturam variáveis locais: `ACTION
  {|| oDlg:End()}` não fecha o diálogo — por isso, qualquer clique de
  botão fecha o diálogo após executar o `ACTION`.
- `VALID` ainda não dispara round-trip por campo (planejado).

## [1.2.0] — 2026-07-08

### Multi-thread

- **`StartJob(cFunc, cEnv, lWait, params...)`** implementado no runtime:
  executa a função em uma VM isolada (semântica de work process do
  Protheus). Com `lWait=.F.` roda em goroutine e o processo aguarda os
  jobs pendentes antes de encerrar; cada job abre a própria conexão ao
  banco SQLite (WAL).
- **`FWGridProcess`** implementada conforme a documentação TDN:
  `New`, `SetThreadGrid`/`SetMaxThreadGrid` (pool de threads com
  backpressure), `CallExecute` (cada unidade em VM isolada com conexão
  própria), `Activate`/`Execute`, `StopExecute`, `IsFinished`,
  `SetAbort`, `SetAfterExecute`, meters (`SetMeters`/`SetMaxMeter`/
  `SetIncMeter`) e `SaveLog`/`GetLastLog`. Sem a interface gráfica de
  configuração (runtime headless).
- **`advplc check` paralelo**: aceita múltiplos arquivos (antes das
  flags) e verifica com 1 worker por CPU, com resumo `ok/failed`.

### Performance

- **Lexer ~95× mais rápido em arquivos grandes**: `tryDotLiteral` fazia
  `ToUpper` de todo o fonte restante a cada caractere `.` (O(n²)).
  Fonte real de 574KB: 9,1s → 0,095s. Corpus de 300 fontes reais do
  Protheus 12.1.2510 verificado em ~1,2s.

### Compatibilidade de linguagem

- Lexer tolera backtick solto fora de strings (typo presente em fontes
  reais da TOTVS aceito pelo compilador Protheus).

## [1.1.x] — 2026-07-08

### Banco de dados unificado

- **Banco padrão renomeado para `~/.advpp/ADVPP.db`** (era
  `./data/advpl_dictionary.db`, caminho relativo que quebrava fora do
  diretório do projeto).
- **Resolver único de caminho** (`shared.ResolveDatabasePath`) usado por
  todas as ferramentas: flag explícita → variável `ADVPP_DB` → config
  `~/.advpp/advpp_config.json` → legado `./data/` → padrão absoluto.
- **Ponto único de abertura** (`shared.OpenSQLite`) com pragmas WAL,
  `busy_timeout` e `foreign_keys` para todas as ferramentas.
- **VM conectado ao banco compartilhado**: `--db-path`/`ADVPP_DB` agora
  funcionam de fato no `advplc run`/`exec` (antes eram parseados e
  ignorados); a IDE também conecta o VM ao mesmo banco.
- Corrigido schema do dicionário: criação do zero falhava por colunas
  ausentes em SX2 (`X2_NOMEUSR`/`X2_MODULO`/`X2_TIPO`/`X2_DESCRIC`) e
  SX5 (`X5_TIPO`/`X5_TAMANHO`/`X5_DECIMAL`).
- Corrigida a heurística `banco.db/tabela` do driver SQLite, que
  quebrava qualquer caminho absoluto (agora só ativa quando o caminho
  não existe em disco; aceita `/` e `\`).

### Portabilidade (Linux / Windows 64 / macOS)

- **Driver SQLite trocado para `modernc.org/sqlite` (100% Go, sem
  CGO)**: o CLI cross-compila estaticamente para linux/windows/darwin,
  amd64 e arm64.
- **Removida a dependência do `iconv` externo**: conversão CP-1252 →
  UTF-8 é feita por conversor interno 100% Go, idêntico nas 3
  plataformas.
- `go.sum` versionado (estava incorretamente no `.gitignore`).

### Build, empacotamento e release

- **`Makefile`**: `make build` (4 ferramentas), `make test` (fixtures),
  `make cross` (CLI para 5 alvos), `make package VERSION=x.y.z`
  (pacotes em `dist/`), `make release VERSION=x.y.z` (tag + CI).
- **GitHub Actions** (`.github/workflows/release.yml`): a cada tag
  `v*`, builds nativos em Linux, Windows e macOS (incluindo as GUIs
  Fyne) e publicação automática dos pacotes `.tar.gz`/`.zip`/`.deb` na
  Release.
- `advplc version` mostra a versão embutida no build.
- Corrigido `.gitignore` que ignorava o diretório `cmd/advpp-ide`
  (o fonte da IDE não estava no repositório).

## [1.0.0]

- Versão inicial: compilador (lexer, preprocessador, parser, codegen),
  VM com natives, MVC, UI Fyne, ferramentas advcfg/adveditor/advpp-ide.
