# Plano: Metodologia de extração RPO + curadoria de artefatos + gap analysis

**Data:** 2026-10-04 · **Spec:** `2026-10-04-rpo-extraction-techniques-design.md` (S3–S5)
**Modo:** execução inline · **Base:** Plan 1 (`rpo apo`) e Plan 2 (`rpo pull`) concluídos

## Goal

Persistir na branch `unstable` (1) o documento canônico de metodologia de extração RPO,
(2) os artefatos de evidência curados de `/tmp/opencode` **sem credenciais nem capturas
sensíveis** (Lei 6), e (3) a análise de completude (gap analysis) exigida pelas regras
globais antes de declarar as tarefas concluídas.

## Motivo

`/tmp/opencode` é scratchpad volátil; os planos 1–2 provaram novos canais (APO in-disk,
GetApoRes via wire, eval DAP ao vivo). A evidência e o método precisam viver no repo
(versionado, revisável) e o `RPO-GROUND-TRUTH.md` precisa refletir o estado novo.

## Constraints

- **Lei 6:** nada de senha (`ADVPP_RPO_PASS` nunca em arquivo), frames de captura com
  senha (`285529`), `live_capture.json` (chaves), `.pull-da.log` (pode conter ambiente).
- **Lei 3:** todos os números recalculados dos artefatos reais (não reusar números de
  memória); origem e definição de cada métrica declaradas.
- **Lei 5:** push autorizado pelo operador; gate = suíte 100% PASS (código inalterado
  por estes tasks — sanity `go build` + vet basta; se qualquer teste tocar docs, rodar suíte).
- Idioma PT-BR; sem trailers em commit (Lei 2 / Anexo D).

## Fatos verificados (pré-condições, 2026-10-04)

- Métricas recomputadas de `/tmp/opencode/apo-report/apo_report.json` (3.465 itens):
  - identificadores **977.618** · literais **496.903** · snippets **28.567** ·
    candidatos **124.944**
  - arquivos com ≥1 snippet / total por extensão: **prw 779/2070 · tlpp 721/1345 ·
    prx 22/35 · apw 3/5 · prg 1/4 · aph 0/5 · tres 0/1**
- `apo_report.md` = 218.940 B; `apo_report.json` = 44.568.620 B (NÃO commitar integral).
- `rpo-pull-smoke.log` = 106 B, verificado sem credenciais.
- `docs/RPO-GROUND-TRUTH.md` existe (12K, autoritativo, v2026-09-20) — sem seções dos
  canais APO/DAP.
- Nota Joplin da onda 44 criada: id `7f65f227-1970-44ba-9d1a-edd455cfa6b8`.

---

- [x] **Task 1: `docs/RPO-EXTRACTION-METHODOLOGY.md` (canônico)**

Criar com heredoc (`cat > docs/RPO-EXTRACTION-METHODOLOGY.md <<'EOF'`), PT-BR, seções:

1. **Propósito e escopo** — índice das técnicas testadas nas ondas 1–44; ponteiro para
   `RPO-GROUND-TRUTH.md` (autoritativo para vereditos) e para os planos em
   `docs/superpowers/plans/`.
2. **Tabela de técnicas** — colunas: técnica · canal · status
   (🟢 CONFIRMADO / 🔴 REFUTADO / 🟡 INFERIDO / ⚪ BLOQUEADO) · evidência
   (arquivo/commit/teste) · comando. Linhas obrigatórias:
   - Container/round-trip (`pkg/rpo/rpo.go`, tests) 🟢
   - Regex sobre cifra p/ nomes de rotina (ondas antigas) 🔴 — falso positivo
   - AES/OpenSSL live-capture (`rpo decrypt` + `live_capture.json`) 🟢 — chave efêmera
   - `rpo regions/analyze/info/identify/decompose/build` 🟢
   - **APO in-disk via regex/framing (Plan 1)** 🟢 — magic `75 00 00 46 46`
   - **Handshake 5-msg AppServer (Plan 2, `pkg/rpo/wire`)** 🟢 — `TestHandshakeLive`
   - **GetApoRes via sessão DAP (Plan 2, `pkg/rpo/dap`)** 🟢 — smoke `ok=1`
   - **Eval cru `0x8149` sem debuggee** 🔴 — retorna NIL (`probe8149.out`)
   - Eval via DAP (`evaluate` com frameId) 🟢
   - `.treports` (552 criptografados) ⚪ — sem chave pública
   - Extração de source-line do RPO em disco 🔴 — esgotado ( Ground Truth)
3. **Protocolo vivo (resumo operacional)** — wire: header `<IHIIHH` 18 B, magic
   `0xab21`, seq 1, corpo comprimido `u32(len|0x80000000)`+`u32(uncomp)`+zlib;
   banner 134 B (layout de offsets); passos do handshake; fluxo DAP
   (LS→token, DA→launch→bp→WaitStopped→evaluate); decode eval
   (aspas/base64/`NIL`); gotchas: deadline por passo, instância única do
   smartclient, workspace com `MSLIB.PRW`+`includes/`.
4. **Métricas do RPO de teste** — tabela por extensão com definição exata
   ("arquivos com ≥1 snippet extraído / total de blobs") + totais; origem:
   recompute de `apo_report.json` em 2026-10-04; artefato: `docs/rpo-evidence/apo_report.md`.
5. **Comandos de referência** — `advplc rpo {info,identify,decompose,build,regions,
   analyze,decrypt,apo,pull}` com um exemplo cada (senhas só via env/prompt).
6. **Limites conhecidos e trabalhos futuros** — layout APO não 100% reverso
   (heurística honesta), source-line impossível sem debuggee, `--res` limitado a
   `GetResArray`.

Verificação: `test -f` + `grep -c` das seções obrigatórias ≥ 6 + nenhum
`grep -E "Pdr\.laptop1|manezinho"` no arquivo.

- [x] **Task 2: Curadoria de artefatos → `docs/rpo-evidence/`**

```bash
mkdir -p docs/rpo-evidence
cp /tmp/opencode/apo-report/apo_report.md docs/rpo-evidence/apo_report.md
cp /tmp/opencode/rpo-pull-smoke.log docs/rpo-evidence/rpo-pull-smoke.log
```

`apo_summary.json` gerado por python inline: agregados por extensão (total, com
ident/lit/snip/cand), somatórios, `catalogo`, `gerado_em`, `fonte` — sem
conteúdo por arquivo (o JSON completo de 44 MB fica só no scratchpad).

**Proibido copiar:** capturas (`captures*`, `285529`), `live_capture.json`,
`*.pull-da.log`, qualquer `.err`, `sc_real.log`, `apo_report.json` (44 MB).

Verificação: `git status` lista apenas os 3 arquivos em `docs/rpo-evidence/`;
`grep -rE` de credenciais no diretório → vazio; `du -sh` ≤ 300K.

- [x] **Task 3: Gap analysis (completude, regra global PARTE 10)**

Seção final do próprio `RPO-EXTRACTION-METHODOLOGY.md` ("Análise de gaps"):
comparar item a item o escopo do spec S3–S5 vs entregue, com rótulos
✅ migrado/entregue · ⚠️ não-entregue (justificativa) · 🔄 preservado em
outra camada. Cobrir explicitamente: technique index para Reversa (1a) — NÃO
entregue aqui (tarefa separada enfileirada), inventário de call-graph (1b) —
entregue como `rpo apo`, cross-ref BIRT (2) — não entregue (fora do spec S3–S5).
Apresentar o resumo no chat ao final.

- [x] **Task 4: Atualizar `RPO-GROUND-TRUTH.md` + nota Joplin**

- Ground Truth: adicionar linhas **C12–C15** 🟢 (APO framing+parser com testes e
  fixtures; handshake live com build real; GetApoRes byte-a-byte via DAP smoke;
  eval DAP com frameId) + rodapé apontando `RPO-EXTRACTION-METHODOLOGY.md`;
  data → 2026-10-04, versão mantida.
- Joplin: nota da onda 44 **já existe** (`7f65f227…`) — criar **nota complementar**
  em `Fact` com o caminho da metodologia + evidências curadas (dedup SHA-256
  automático).

Verificação: `grep -c "C12\|C13\|C14\|C15"` ≥ 4; nota Joplin retorna `status: created`.

- [x] **Task 5: Gate final + commit + push**

```bash
cd /tmp/opencode/advpp-unstable-wt
go build ./cmd/advplc && go vet ./pkg/rpo/... ./cmd/advplc/   # sanity (docs não afetam binário)
git status --short                                             # só arquivos pretendidos
git grep -nE "Pdr\.laptop1|manezinho|laptop-peder|442d5780|b55ee224" -- docs/ || echo LIMPO
git add docs/RPO-EXTRACTION-METHODOLOGY.md docs/rpo-evidence/ \
        docs/superpowers/plans/2026-10-04-rpo-methodology-artifacts.md
# (+ RPO-GROUND-TRUTH.md se Task 4 alterou)
git commit -m "DOC — metodologia de extração RPO, evidências curadas e gap analysis"
git push origin unstable
```

Se `git push` rejeitar (remoto avançou): `git fetch` + `git merge origin/unstable`
(zero sobreposição esperada em docs) + re-validar vet/build + push.
Expected: push aceito; working tree limpo ao final.

---

**Execução real (2026-10-04) — desvio:** `C12` já existia no Ground Truth
(`tttm120.rpo` idêntico) → novas linhas numeradas **C13–C16** + **R8**
(refutação do eval cru `0x8149`), conforme renumeração automática na edição.
Joplin: nota onda 44 `7f65f227…` + nota Fact `0875efb4…` (Task 4).
Tarefas 1a (índice imagem→rotina) e 2 (BIRT↔APO) permanecem enfileiradas
fora deste spec (§7 do documento canônico).

---

## Self-Review

**1. Spec coverage (S3–S5):** S3 metodologia → Task 1; S4 artefatos → Task 2;
S5 gap/Joplin → Tasks 3–4; aceitação = push com evidência → Task 5.
**2. Placeholders:** nenhum TBD — todos os passos com comando.
**3. Type consistency:** sem tipos novos (doc/curadoria); métricas únicas e
recalculadas; IDs Joplin conferidos nas pré-condições.
