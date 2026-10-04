# Design — Incorporação das técnicas de extração RPO ao `advplc rpo` + documentação e persistência (branch `unstable`)

**Data:** 2026-10-04
**Status:** aprovado pelo operador (design em 5 seções; emenda em S4 aplicada antes da gravação)
**Branch alvo:** `unstable` (base: `d1b876a` = merge `master` → `unstable`, preservando `0397429`)

---

## 1. Contexto

As sessões de engenharia reversa do RPO Protheus (ondas 6–43, todas persistidas em
Joplin sob `Agentes > _Mem0 > Fact`) provaram:

- **Canais de texto de fonte esgotados** (DAP `source`, `GETINCLUDE/GETSOURCE`,
  `srget*`, `GetSrcCode`, `.CH`) — duplamente verificados, marcados REFUTADOS em
  `docs/RPO-GROUND-TRUTH.md`.
- **Canal APO vivo validado**: `GetApoRes(nome)` + `Encode64()` devolve o blob APO
  íntegro por resource; extração massiva (onda 39) gravou **3.465 blobs custom**
  (`mass/apo/`, 37 MB) + **5.634 recursos** (`mass/res/`, 156 MB) + **99.197
  recursos .tres** (`mass/tres/`, 445 MB) + TRP/BIRT (onda 40).
- **Framing do blob APO parcialmente decifrado** (onda 41–43 + análise empírica):
  header `75 00 00 46 46` + tipo (`F`=AdvPL, `T`=TLPP — correlação 100% com
  extensão nos 3.465), nome, tabelas de strings (identificadores, literais).
- **Evidência de "trecho de código" mensurada**: literais reais de código em
  PRW 788/2073, TLPP 723/1347, PRX 22/35, PRG 1/4, APW 0/1 (ex.: SQL com
  `D_E_L_E_T_`, `RD0->RD0_CODIGO`, codeblocks `{ |x| x:Code == ... }`).
- O `advplc rpo` atual cobre container (`info/identify/decompose/build`),
  forense (`analyze/regions`), chaves (`extract/decrypt`) e injeção
  (`inject`), mas **não** a extração wire das ondas 19–43 nem o parser dos
  blobs APO.

**Decisão do operador:** opção **B** (completo) — tudo do híbrido + port da
extração wire para Go puro, fatiado: parser APO primeiro, wire depois.

## 2. Escopo (seções aprovadas)

### S1 — `advplc rpo apo` (fatia 1 — desmontagem APO em Go)
- Novo `pkg/rpo/apo_blob.go`: framing (`75 00 00 46 46` + tipo F/T), nome do
  resource, extração de strings null-terminated, classificação
  identificador/literal/**snippet** (regex de conteúdo de código: `:=`, `->`,
  `%notDel%`, `BeginSql`, `SELECT … FROM`, etc.), call-graph **candidato**
  por oráculo opcional (`--catalog <arquivo>` = `wire/catalog_sec2.txt`,
  113.431 nomes) com marcação de confiança 🟢/🟡 (nunca afirmar como fato o
  que é heurística).
- CLI: `advplc rpo apo <arquivo|dir> [--catalog f] [--out dir] [--format json|md]`.
- Testes Go (`pkg/rpo/apo_blob_test.go`) com fixtures reais: EXTXDEF.PRW
  (40 B), ABSLOGGER.PRW (292 B), amostra TLPP.
- Corrigir bug `cmdRpoInject` duplicado (`cmd/advplc/cmd_rpo.go` linhas 60–63:
  bloco pós-switch reexecuta `cmdRpoInject`).

### S2 — `advplc rpo pull` (fatia 2 — extração wire em Go)
- Novo pacote `pkg/rpo/wire` (port fiel do `tcp_proxy2.py`/`dap_lib.py`):
  frame `'<IHIIHH'` (total-4, 0xab21, total-14, seq=1, msg id), compressão
  zlib (`I4 compLen|0x80000000` + `I4 uncompLen`), handshake de auth,
  injeção de eval (`Encode64(GetApoRes/GetResArray)`) com strip de aspas do
  DAP, decodificação base64 local.
- CLI: `advplc rpo pull --host H --port P --user U --out DIR [--manifest lista]
  [--res '*']` — credencial via prompt/`ADVPP_RPO_PASS` (Lei 6: nunca
  hardcoded, nunca logada).
- Reproduz ondas 39/40 nativamente; usa `Resource2File` como alternativa
  validada (binário type==1 apenas).
- Smoke-test read-only obrigatório contra `protheus-2310` (172.17.0.3:1234)
  antes de declarar pronto; mensagens de erro id de lethais (0x845b etc.)
  documentados no código.

### S3 — Documentação (PT-BR)
- Novo `docs/RPO-EXTRACTION-METHODOLOGY.md`: metodologia onda-a-onda, tabela
  de **recursos por bloco** (`mass/apo`, `mass/res`, `mass/tres`, `mass/trp`,
  `mass/birt`, `wire/`, `apores/`), técnicas (frame layout, Encode64,
  Resource2File, disassembly `SaveApo`), **evidências de trecho por extensão**
  (números e exemplos verados), nota honesta (linhas de fonte = canal
  esgotado; snippets = literais reais + `.CH`/`.th`), mapa *técnica →
  comando `rpo`*, cross-ref `RPO-GROUND-TRUTH.md`.
- Atualizar uso/ajuda de `cmd_rpo.go` com os novos subcomandos.

### S4 — Artefatos `/tmp/opencode/advpls-test` → `unstable` (com emenda)
| Tier | Conteúdo | Destino |
|---|---|---|
| Commit normal | `wire/` (12 MB, catálogos), `apores/` (4,3 MB), `mass/birt/`, `mass/trp/` (21 MB), scripts curados (76 → ~25 únicos; proxies duplicados → só manifest SHA-256), evidências (`saveapo.dis`, amostras, banners), `manifests/SHA256SUMS` | git |
| Git LFS | `mass/apo/` (37 MB — dataset do parser) | git-lfs (disponível) |
| **Emenda do operador** | **logs `da*.log` (~1 GB) e disasms (`server.disasm` 536 M, `disasm_full` 354 M, `da.asm` 119 M, `adapter.disasm` 114 M)** | **cópia em pasta do projeto + `.gitignore`** (fora do git) |
| Não versionar (manifest + regeneração) | `mass/res`, `mass/tres` (regeneráveis via `rpo pull`), `src/includes` (852 M) | manifest SHA-256 + comando de regeneração documentado |
- Execução em **worktree temporário** `/tmp/opencode/advpp-unstable-wt` — o
  working tree principal (alterações pré-existentes do operador) **não é
  tocado**. **Sem push** (Lei 5).

### S5 — Persistência e entrega
- Nota Joplin (`Agentes > _Mem0 > Fact`) com as descobertas desta onda.
- Commits atômicos em `unstable`, formato `[FEAT|DOC|TEST|CFG] — descrição`,
  **sem** trailers de atribuição (Lei 2/Anexo D).
- Verificação antes de declarar: `go build ./...` e `go test ./...` no
  worktree; smoke-test `rpo pull` read-only; gap analysis S1/S2 apresentada.

## 3. Fora de escopo
- Decifrar `.treports` (chave desconhecida, busca negativa persistida).
- Port da UI DAP/debug completo para Go.
- `git push` ou qualquer ato externo.
- Alterar o working tree principal do operador.

## 4. Riscos e mitigações
- **Handshake auth em Go** pode divergir do Python → mitigar com captura de
  referência (`captures2/banner`) como fixture e comparação byte-a-byte.
- **Framing APO incompleto** → saída com graus de confiança; fallback para
  extração de strings brutas sempre disponível.
- **Conflitos futuros master↔unstable** → spec gravado após merge `d1b876a`.
- **Espaço em disco**: 135 GB livres; cópia de ~2,5 GB (logs/disasms +
  pendentes) é segura.

## 5. Critérios de aceitação
1. `advplc rpo apo` processa os 3.465 blobs e emite JSON/MD com literais +
   snippets + call-graph candidato (fixtures verdes em `go test`).
2. `advplc rpo pull` baixa ao menos um `GetApoRes` real do appserver vivo
   (evidência em log) sem credencial no código.
3. `docs/RPO-EXTRACTION-METHODOLOGY.md` existe, com evidências por extensão.
4. Artefatos tier-commit + LFS + logs/disasms (gitignored) no `unstable`.
5. Nota Joplin criada; commits sem trailers; `go build/test` verdes.
