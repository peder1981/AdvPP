# RPO — Metodologia de Extração (canônico)

**Escopo:** todas as técnicas de extração de conteúdo de RPO/POI testadas nas ondas 1–44,
com status de evidência, protocolo operacional ao vivo e limites conhecidos.

**Relação com outros documentos:**
- `docs/RPO-GROUND-TRUTH.md` — **autoritativo** para vereditos (CONFIRMADO/REFUTADO).
  Em conflito, o Ground Truth prevalece; este documento descreve *como* extrair.
- `docs/superpowers/plans/2026-10-04-rpo-apo-parser.md` — plano do `advplc rpo apo`.
- `docs/superpowers/plans/2026-10-04-rpo-pull.md` — plano do `advplc rpo pull`.
- `docs/rpo-evidence/` — evidências curadas (relatório APO, log do smoke do pull).

**Data:** 2026-10-04 · **Versão do produto:** branch `unstable`

---

## 1. Tabela de técnicas

Status: 🟢 CONFIRMADO (reproduzido) · 🔴 REFUTADO (testado e falso) ·
🟡 INFERIDO (indício, pode estar errado) · ⚪ BLOQUEADO (sem caminho conhecido)

| # | Técnica | Canal | Status | Evidência | Comando/Artefato |
|---|---------|-------|--------|-----------|------------------|
| T1 | Parse do container (cabeçalho/footer/regiões) | disco | 🟢 | `pkg/rpo/rpo.go`; `TestParseCustomRPO`, `TestRoundTrip*` | `advplc rpo info/identify/decompose/build` |
| T2 | Round-trip byte-a-byte do container | disco | 🟢 | `TestRoundTripCustom/TTTM120/Synthetic` | — |
| T3 | Classificação honesta por entropia | disco | 🟢 | `pkg/rpo/forensics.go`; `TestForensics*` | `advplc rpo regions` |
| T4 | **Regex sobre o miolo cifrado** p/ nomes de rotina/função | disco | 🔴 | Falsos positivos provados (ondas 10–15); Ground Truth §REF | — (nunca usar) |
| T5 | Decodificação com **chave capturada na mesma compilação** | disco+captura | 🟢 | `TestCipherDispatch_RealCapture` (10/10 cifras); zlib `78 9c` | `advplc rpo decrypt <rpo> <captura.json>` |
| T6 | Extração de **source-line** do RPO em disco | disco | 🔴 | Esgotado (ondas 19–38): o miolo cifrado não entrega linhas | — |
| T7 | **APO in-disk (framing + parser)** | disco | 🟢 | `pkg/rpo/apo_blob.go` + 4 fixtures reais; 3.465/3.465 blobs | `advplc rpo apo <dir>` |
| T8 | Handshake de 5 mensagens do AppServer | ao vivo (wire) | 🟢 | `TestHandshakeLive` → `build=7.00.210324P env=TOTVSTEC` | `pkg/rpo/wire.Handshake` |
| T9 | **`GetApoRes` via sessão DAP** (recurso do RPO → arquivo) | ao vivo (DAP) | 🟢 | Smoke: `PULL FIM: ok=1`; `1.PNG` md5 idêntico byte-a-byte | `advplc rpo pull` |
| T10 | Eval ao vivo **com debuggee** (`evaluate` + frameId) | ao vivo (DAP) | 🟢 | Ondas 20–39: `GetApoRes`, `ProcName`, etc. | `pkg/rpo/dap.Session.Evaluate` |
| T11 | Eval cru `0x8149` em conexão fresh | ao vivo (wire) | 🔴 | Retorna `U\0` (NIL) sem debuggee (`probe8149.out`) | — (nunca usar) |
| T12 | Listing `--res` via `GetApoRes` em chunks | ao vivo (DAP) | 🟢 | Onda 24: `GetResArray` + `AEval` em chunks de 3000 | `advplc rpo pull --res` |
| T13 | `.treports` (552 relatórios) | disco | ⚪ | Criptografados sem chave pública conhecida | — |
| T14 | Live-capture via LD_PRELOAD hook | ao vivo | 🟢 | `tools/rpo-live-inspect/rpo_key_hook/` (ondas 6–9) | — |

---

## 2. Protocolo wire ao vivo (AppServer AdvPL)

Implementação de referência: **`pkg/rpo/wire`** (observado ao vivo, ondas 19–38;
reproduções de captura: `tcp_proxy2` em `/tmp/opencode`).

### 2.1 Frame

Layout de 18 bytes + corpo, little-endian (`pkg/rpo/wire/frame.go`):

```
u32  total-4          # offset 0
u16  magic 0xab21     # offset 4
u32  total-4          # offset 6
u32  total-14         # offset 10
u16  seq = 1          # offset 14  (único valor observado em todas as capturas)
u16  msgID            # offset 16
...  body
```

`total` = 18 + len(body). O magic é validado em `ParseFrames`.

### 2.2 Corpo comprimido

Respostas grandes chegam como: `u32(compLen | 0x80000000)` + `u32(uncompLen)` +
zlib desde o byte 8. **Atenção:** o teste é o **bit de alta do u32 inteiro**
(`raw & 0x80000000`), **não** `body[0] & 0x80` (confusão que custou um ciclo de
debug — ver `DecodeBody`).

### 2.3 Banner do cliente (134 bytes)

Layout medido em `captures2/0222_L7_C2S.bin` (sem senha) — `pkg/rpo/wire/banner.go`:

| Offset | Conteúdo |
|--------|----------|
| `[0:15]` | `"--ADVANCEDPR--\x00"` |
| `[15]` | `0x03` |
| `[16:82]` | usuário (slot 66 B, `len ≥ 66` falha) |
| `[82:115]` | host (slot 33 B) |
| `[115:130]` | build stamp (slot 15 B; p/ este appserver: `"20210324103317"`) |
| `[130:134]` | `00 00 05 01` |

### 2.4 Handshake (5 passos, captura L334)

| Passo | msgID | Body | Resposta esperada |
|-------|-------|------|-------------------|
| — | — | banner (2.3) | descartar (janela ~1,5 s) |
| 1 | `0xa22b` | `''` | `20.3.2.14 - 42368\0` (versão) |
| 2 | `0x8451` | `''` | `7.00.210324P\0` (build) |
| 3 | `0xa241` | `"P12\0"` | `\x01\0\0\0` (código de ambiente) |
| 4 | `0x0051` | `"P12\0user\0pass\0"` | `\x01N0\0` (login; primeiro byte `0x01` = OK) |
| 5 | `0xa1c4` | `''` | `TOTVSTEC\0` (marca do produto) |

**Segurança (Lei 6):** a senha aparece **somente** no frame `0x0051` e nunca em
log/erro. **Deadline por passo** (1 s único estourava com RTT via proxy de
captura — commit `a407683`).

---

## 3. Fluxo DAP ao vivo (extração de recursos e eval)

Implementação de referência: **`pkg/rpo/dap`** (`rpc.go`, `token.go`, `session.go`).

1. **Token:** Language Server `advpls-linux-226 language-server` → `initialize` /
   `initialized` → `$totvsserver/connect` → `authentication`
   (`connectionToken`, `environment`, `user`, `password`, `encoding: "CP1252"`) →
   `serverInformations`. Tipo de ambiente: `1`=protheus, `2`=logix, outro=totvs.
2. **Sessão:** Debug Adapter `debugAdapter-linux` → `initialize` (`adapterID:
   "totvs_language_debug"`) → `launch{program, token, server, port, build,
   workspaceFolders, cwb, smartclientBin, ...}` (timeout 90 s) → drena 2 s →
   `setBreakpoints` (todas as linhas do programa-alvo) → `configurationDone` →
   `WaitStopped` (drena + clique `xdotool` se aparecer diálogo) → `threads` →
   `stackTrace` → `evaluate{expression, frameId, context:"repl"}`.
3. **Decode do eval:** remove aspas → `NIL`→nil, `""`→vazio, senão base64.
4. **Listing `--res`:** `aAll := GetResArray('<padrão>')`; chunks de 3000 com
   `AEval(aAll, {|x,n| IIf(n >= S .and. n <= E, cOut := cOut + x + Chr(10), NIL)})`
   — fim do chunk **clampado em n**.

### Gotchas operacionais (todos custaram um ciclo cada)

| Gotcha | Sintoma | Solução |
|--------|---------|---------|
| Smartclient em instância única | `setBreakpoints: processo encerrado`; novo processo sai `ExitCode=0` em ~1 s | Matar sobras (1 smartclient + DAs + LSs) antes do launch — **só com autorização (Lei 1.2)** |
| Workspace incompleto | DA não resolve o programa | Workspace precisa de `<PROG>.prw` + `MSLIB.PRW` + `includes/` |
| Deadline único de 1 s | Handshake falha no passo 4 via proxy | Deadline **por passo** (10 s) |
| Zumbis `go` | `TestFgtFixture` trava | Checar `ps aux \| grep '[g]o '` antes da suíte |
| Eval sem debuggee | `0x8149` → `U\` | Usar sessão DAP completa (T11 🔴 vs T10 🟢) |

---

## 4. Métricas do RPO de teste (desmontagem APO)

**Definição:** "arquivos com ≥1 *snippet* extraído / total de blobs da extensão".
**Origem:** recálculo de `docs/rpo-evidence/apo_report.md` (gerado pelo
`advplc rpo apo` sobre os 3.465 blobs do catálogo) em 2026-10-04.
O JSON completo (44 MB) fica só no scratchpad; agregados em
`docs/rpo-evidence/apo_summary.json`.

| Extensão | Blobs | Com identificador | Com literais | Com snippet | Com candidato |
|----------|------:|------------------:|-------------:|------------:|--------------:|
| `.prw` | 2.070 | 2.045 | 1.794 | **779** | 1.518 |
| `.tlpp` | 1.345 | 1.344 | 1.339 | **721** | 1.112 |
| `.prx` | 35 | 35 | 33 | **22** | 33 |
| `.apw` | 5 | 5 | 5 | **3** | 5 |
| `.prg` | 4 | 4 | 4 | **1** | 4 |
| `.aph` | 5 | 0 | 0 | **0** | 0 |
| `.tres` | 1 | 1 | 1 | **0** | 1 |
| **Total** | **3.465** | | | | |

Totais absolutos: **977.618 identificadores**, **496.903 literais**,
**28.567 snippets**, **124.944 call-candidatos**.

> Os "snippets" são trechos de código reais recuperados dos blobs APO pela
> heurística de framing — **não** são linhas-fonte completas do arquivo original.
> Isso é honesto: o layout interno do APO não está 100% reverso (ver §6).

---

## 5. Comandos de referência

```bash
# Container (disco)
advplc rpo info      arquivo.rpo
advplc rpo identify  arquivo.rpo
advplc rpo regions   arquivo.rpo          # classificação honesta por entropia
advplc rpo analyze   arquivo.rpo          # entropia/bytes/strings
advplc rpo decompose arquivo.rpo --out dir/
advplc rpo build     dir/ -o novo.rpo
advplc rpo decrypt   arquivo.rpo captura.json   # exige chave da MESMA compilação

# APO in-disk (ondas 40–44)
advplc rpo apo [--catalog f] [--out dir] [--format json|md] <arquivo|dir>

# Extração ao vivo (ondas 39–44) — senha NUNCA no argumento:
export ADVPP_RPO_PASS='...'             # ou prompt interativo
advplc rpo pull --out DIR --manifest lista.txt --workspace WS \
    [--host 172.17.0.3 --port 1234 --user peder --env P12 ...]
```

Evidências: `docs/rpo-evidence/rpo-pull-smoke.log` (smoke `1.PNG`, md5
`dcfdf6a6b39af58d6f0de5af25125420`).

---

## 6. Limites conhecidos e trabalhos futuros

| Limite | Status | Observação |
|--------|--------|-----------|
| Layout interno do registro APO | 🟡 parcial | Parser = heurística honesta com score estrito (não emite ruído); reverse completo do record layout é trabalho futuro |
| Source-line do RPO em disco | 🔴 impossível | Miolo cifrado de alta entropia sem chave da mesma compilação; regex = falso positivo (T4) |
| `.treports` | ⚪ bloqueado | 552 arquivos criptografados; sem chave pública |
| Eval sem sessão | 🔴 refutado | T11; exige DAP completo |
| `--res` | 🟢 parcial | Cobertura via `GetResArray` em chunks; recursos binários grandes saem íntegros (provado com PNG) |
| APO: `.aph` sem snippet | ⚪ desconhecido | 0/5 — formato do blob `.aph` ainda não mapeado |

---

## 7. Análise de gaps (completude — regra global PARTE 10)

Comparação escopo (spec `2026-10-04-rpo-extraction-techniques-design.md`, S3–S5) ×
entregue:

| Item do spec | Rótulo | Nota |
|--------------|--------|------|
| Doc de metodologia canônico com tabela de técnicas | ✅ entregue | §1–§6 deste arquivo |
| Evidências curadas versionadas | ✅ entregue | `docs/rpo-evidence/` (3 arquivos, ≤ 300 KB) |
| Gap analysis explícita | ✅ entregue | §7 |
| Nota Joplin (REGRA #1) | ✅ entregue | Onda 44: `7f65f227…` + nota complementar em `Fact` |
| Ground Truth atualizado com os novos canais | ✅ entregue | C12–C15 |
| Índice imagem→rotina p/ Reversa Visor (3.561 PNGs) | ⚠️ fora deste spec | Tarefa separada enfileirada (pós-planes) |
| Inventário call-graph/literais (item 1b do operador) | ✅ entregue como `rpo apo` | 124.944 candidatos, relatório JSON/MD |
| Cross-ref BIRT↔APO (37 designs × 56 ds_refs) | ⚠️ fora deste spec | Tarefa separada enfileirada |
| Remoção de credenciais do **histórico** git | ⚠️ pendente de decisão | Requer `git filter-repo` + force-push — aguarda autorização explícita (Lei 5) |

---

**Fim do documento.** Atualizações devem preservar a tabela §1 como índice vivo
e refletir mudanças no `RPO-GROUND-TRUTH.md` antes deste arquivo.
