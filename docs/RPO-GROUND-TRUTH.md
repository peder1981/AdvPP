# RPO — Verdade Empírica (Ground Truth)

**Documento autoritativo.** Onde qualquer outro documento RPO divergir deste,
este prevalece. Cada afirmação abaixo é marcada com o nível de evidência:

- 🟢 **CONFIRMADO** — reproduzido por teste automatizado ou dado medido.
- 🟡 **INFERIDO** — baseado em indício; pode estar errado.
- 🔴 **REFUTADO** — foi testado e mostrou-se falso.
- ⚪ **DESCONHECIDO** — sem evidência suficiente.

**Versão:** 4.3.1 (unstable) · **Data:** 2026-09-20

---

## 1. Veredito de uma linha

O RPO do Protheus tem um **container legível** (cabeçalho/footer/regiões) e um
**miolo cifrado de alta entropia**. O container é lido/reescrito com segurança;
o miolo só é decodificável com uma **captura de chave feita durante a MESMA
compilação** que gerou o arquivo. Sem essa captura, **nada de estrutura interna
é recuperável** — e qualquer "nome de função/rotina" extraído por regex do
arquivo em disco é **falso positivo de ruído**.

---

## 2. O que está CONFIRMADO 🟢

| # | Afirmação | Evidência |
|---|-----------|-----------|
| C1 | Container tem cabeçalho (ponteiro de auto-referência + bloco de nome/sentinela) e footer (magic ASCII + 24 bytes de trailer). | `pkg/rpo/rpo.go`; testes `TestParseCustomRPO`, `TestParseTTTM120`, `TestParseTLPP`, `TestRoundTrip*` |
| C2 | Round-trip do container é byte-a-byte fiel. | `TestRoundTripCustom`, `TestRoundTripTTTM120`, `TestRoundTripSynthetic` |
| C3 | Magics de footer identificam o tipo: `APNSRM0419`=custom, `APNSRM0420`=tlpp, `APNSRM0421`=tttm120. | `pkg/rpo/identify.go`; `docs/rpo-format.md` |
| C4 | O miolo é **cifrado com alta entropia** (~7.95–8.00 bits/byte), 256/256 bytes únicos. | `advplc rpo regions` nesta versão; testes `forensics_test.go` |
| C5 | A cifra **não é um algoritmo fixo**: é uma **tabela rotativa de cifras legadas do OpenSSL**, escolhida por chamada. | `pkg/rpo/cipher_dispatch.go`; `TestCipherDispatch_RealCapture` (10/10 segmentos batem contra RPO real) |
| C6 | Os algoritmos observados incluem: DES-EDE (3DES), CAST5, Blowfish (bf), RC4, RC5-32/12/16, IDEA, RC2 — em modos ECB/CBC/CFB64/OFB. | `pkg/rpo/testdata/live_capture.json` (nomes de cifra literais capturados) |
| C7 | O **payload antes de cifrar é zlib** (magic `78 9c`). | `cmd/advplc/cmd_rpo_decrypt.go` faz `zlib.NewReader` e infla com sucesso |
| C8 | Chave/IV são **efêmeros por sessão de compilação** (16+8 bytes), fixados uma vez via `SetKey` e reusados por todas as cifras. | `docs/rpo-format-sonnet.md`; captura ao vivo |
| C9 | Dada a captura, a decodificação é **real e verificada** (`advplc rpo decrypt`). | `TestCipherDispatch_RealCapture`; execução do comando contra `live_capture` |
| C10 | O **diretório interno** (após decifrar+inflar) contém entradas nomeadas, ex.: `RPORC5_TRIGGER.PRW`, `SIGA.MAP`, `SIGAANNOT.MAP`, `SIGABAD.MAP`, `SIGACLS.MAP`, `SIGAEXIT.MAP`, `SIGAFC.MAP`, `SIGAINIT.MAP`, `SIGAPCLS.MAP`. | Saída real de `advplc rpo decrypt pkg/rpo/testdata/live_capture.rpo ...` |
| C11 | `tlpp.rpo` é **idêntico** entre 12.1.2310 e 12.1.2510. | MD5 `1755a366c890999d665b040992a470de` em ambos |
| C12 | `tttm120.rpo` é **idêntico** entre 12.1.2310 e 12.1.2510. | MD5 `f35f1ec791e4985ff7d2690f8e94bbeb` em ambos |

---

## 3. O que está REFUTADO 🔴

| # | Alegação antiga | Por que é falsa | Prova |
|---|-----------------|-----------------|-------|
| R1 | "O RPO usa **AES-128-CBC**." | Nenhuma cifra AES aparece; são cifras legadas OpenSSL. | `docs/rpo-format.md` Fase 15; `cipher_dispatch.go` |
| R2 | "**Rotinas identificadas**: AP448, DK158, DW619, FT535, IH358, JU392, TS3140, VA995, ZQ761." | Regex `[A-Z]{2,4}[0-9]{3,5}` sobre 12 MB de cifra casa com ruído. Dados aleatórios do mesmo tamanho produzem contagem equivalente. | `forensics_test.go::TestRegexFalsePositiveOnCiphertext`; `dismantle_rpo.py` (100% ciphertext) |
| R3 | "**Funções U_ identificadas**: U_4SH, U_B0H, U_PXYU." | Regex `U_[A-Z0-9_]{3,10}` sobre cifra; taxa indistinguível de ruído. | idem |
| R4 | "**Candidatos APO** encontrados no arquivo cifrado." | O scanner heurístico antigo tinha confiança-base 0.5 e aceitava ~100% do ruído. | `apo_parser.go` reescrito; `TestAPOScannerRejectsCiphertext` |
| R5 | "Trailer de 24 bytes é **SHA-1**." | Nunca foi verificado que seja SHA-1; é apenas 24 bytes opacos. | `rpo.go` (comentário original já dizia "opaco") |
| R6 | "**10/15 segmentos** decodificados ⇒ extração de RPOs reais funciona." | A captura com sucesso é de um **fixture sintético** (contém a string-teste `"verify rc5 implementation real data test string here"`), não de um RPO de produção. | inspeção de `live_capture.json`; saída do `decrypt` |
| R7 | "Tabela de **~12 cifras**" como se todas fossem implementadas. | 10 de 12 confirmadas; **IDEA não tem implementação validada** (retorna erro explícito, nunca decodifica errado). | `pkg/rpo/idea.go`; `TestIDEA_KnownBroken` |

---

## 4. O que é DESCONHECIDO ⚪

- Formato byte-a-byte exato do diretório APO (só os **nomes** das entradas foram vistos).
- Se o trailer de 24 bytes é checksum/assinatura e qual algoritmo.
- Conteúdo dos `.MAP` (SIGA\*.MAP) — vimos os nomes, não o payload de cada um.
- Layout exato das entradas no `admin_section` vs `body`.
- Se existe derivação de chave a partir de algum dado do arquivo (evidência diz que **não** — chaves são aleatórias).

---

## 5. Como usar (fluxo honesto)

```bash
# 1. Ver o que dá para saber SEM chave (sempre):

advplc rpo info      arquivo.rpo     # metadados do container
advplc rpo identify  arquivo.rpo     # tipo (custom/tlpp/tttm120)
advplc rpo regions   arquivo.rpo     # classificação honesta do conteúdo
advplc rpo analyze   arquivo.rpo     # entropia, bytes, strings
python3 tools/rpo-live-inspect/dismantle_rpo.py arquivo.rpo  # relatório

# 2. Para DECODIFICAR o miolo, é preciso capturar a chave DURANTE a
#    compilação que gerou o arquivo (mesma sessão):

LD_PRELOAD=tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook.so \
  ./appsrvlinux -compile -files=fonte.prw -env=ambiente
# -> gera /tmp/rpo_keys_export.json

advplc rpo decrypt arquivo.rpo /tmp/rpo_keys_export.json
```

O que **não** funciona: rodar `decrypt` de um RPO de produção (ex.:
`tttm120.rpo` de 379 MB) com uma captura de outra sessão — a chave não bate.

---

## 6. Por que "extração de código-fonte" é impossível offline

As chaves vêm de `OpenSSL RAND_bytes()` + entropia do sistema durante a
compilação e são **descartadas** ao final. Recompilar o mesmo fonte gera
bytes **completamente diferentes**. Portanto:

- **Não existe** um "decriptador offline universal de RPO".
- **O que existe** (e está implementado) é: (a) leitura segura do container;
  (b) decodificação **dada** uma captura ao vivo; (c) análise honesta do
  conteúdo quando não há captura (entropia/classe de regiões, sem inventar
  estrutura).

---

## 7. Ferramentas e onde estão

| Ferramenta | O que faz | Arquivo |
|-----------|-----------|---------|
| `advplc rpo info/identify/decompose/build` | Container | `cmd/advplc/cmd_rpo.go`, `pkg/rpo/rpo.go` |
| `advplc rpo decrypt` | Decodifica **com captura** | `cmd/advplc/cmd_rpo_decrypt.go`, `pkg/rpo/cipher_dispatch.go` |
| `advplc rpo regions` | Classifica conteúdo (honesto) | `cmd/advplc/cmd_rpo_regions.go`, `pkg/rpo/forensics.go` |
| `advplc rpo analyze` | Entropia/bytes/strings | `cmd/advplc/cmd_rpo_analyze.go` |
| `advplc rpo extract` | Lista funções via appserver real | `pkg/rpo/extract.go` |
| `dismantle_rpo.py` | Relatório honesto offline | `tools/rpo-live-inspect/dismantle_rpo.py` |
| `rpo_extractor.py` | Relatório JSON offline | `tools/rpo-live-inspect/rpo_extractor.py` |

---

## 8. Testes que sustentam este documento

```
pkg/rpo/forensics_test.go   — prova dos falsos positivos (R2, R3)
pkg/rpo/apo_parser_test.go  — scanner estrito rejeita cifra (R4)
pkg/rpo/cipher_dispatch_test.go — 10/10 cifras reais (C5, C9)
pkg/rpo/rpo_test.go         — container/round-trip (C1, C2)
pkg/rpo/rc2_test.go, rc5_test.go, idea_test.go — cifras individuais
```

Rodar: `go test ./pkg/rpo/ -count=1 -v`

---

## 9. Documentos RETRATADOS (contêm alegações falsas)

Os documentos abaixo foram escritos em sessões anteriores com alegações não
verificadas (seções 3.deste documento). **Não use como fonte.** Carregam um
aviso no topo apontando para cá:

`FINAL-SUMMARY.md`, `MISSION-ACCOMPLISHED.md`, `RPO-DISMANTLING-FINAL-REPORT.md`,
`RPO-EXTRACTION-FINAL-REPORT.md`, `RPO-EXTRACTION-FINAL-SUMMARY.md`,
`RPO-EXTRACTION-INTEGRATION.md`, `RPO-EXTRACTION-TWO-FRONT.md`,
`RPO-EXTRACTION-README.md`, `RPO-INVESTIGATION-FINAL-REPORT.md`,
`RPO-16MB-ANALYSIS.md`, `RPO-16MB-HASHMAP.md`,
`RPO-REVERSE-ENGINEERING-COMPLETE-GUIDE.md`, `rpo-getSx3Cache-FINAL.md`,
`rpo-getSx3Cache-extraction.md`.

---

*Este documento é mantido com evidência, não com narrativa. Se você mudar
o código, atualize a evidência correspondente.*

---

## 10. Limitações Identificadas (2026-09-20)

### 10.1 AppServer 24.3.1.1 — Comportamento Diferente

**Observação:** A versão 24.3.1.1 do appserver (build 7.00.240223P)
apresenta comportamento diferente da versão usada nos testes originais:

- `EVP_EncryptInit_ex` **não é chamado** durante operações de cifragem
- O hook LD_PRELOAD captura `SetKey` (chave mestra) e `EncryptUpdate`
  (dados de entrada), mas **não captura o cipher name**
- Sem o cipher name, a decodificação automática é impossível

**Workaround temporário:**
1. Usar appserver de versão anterior (12.1.2310) para capturas
2. Ou usar gdb manual para break em `tCryptoEVP::Encrypt`
3. Ou implementar hook em nível diferente (ex: `tCryptoEVP::Encrypt`)

**Status:** Bloqueio conhecido,文档ado em `docs/RPO-LIMITATIONS.md`.
