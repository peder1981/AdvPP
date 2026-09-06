---
name: advpp-release
description: Passo a passo completo pra publicar uma nova release do compilador AdvPP (CLI advplc + GUIs + extensão VS Code + deploy). Use quando o usuário pedir "nova release", "publicar release", "lançar versão", "release do AdvPP/compilador" ou equivalente, neste repositório.
metadata:
  author: peder1981
  version: "1.0.0"
---

# advpp-release

Processo de release do compilador AdvPP (`~/Projetos/AdvPP`). Seguir
NA ORDEM — cada passo é um gate pro próximo, nenhum é opcional sem o
usuário dizer explicitamente pra pular.

## 0. Congelar escopo e decidir a versão

- `git log <última-tag>..HEAD --oneline` e `git diff --stat` — o que
  realmente mudou desde a última release.
- SemVer: gap TDN novo/feature nova → `MINOR`; só fix/doc → `PATCH`;
  breaking change (raro, projeto valoriza compatibilidade) → `MAJOR`.
- **Regra permanente do operador: nunca versão ≤ à última já
  publicada.** Conferir com `git tag --sort=-v:refname | head -1`
  antes de decidir o número. Sem pre-release abaixo da latest.

## 1. Validação técnica (gate antes de qualquer doc)

```bash
go build ./...
go vet -unsafeptr=false ./...      # flag já existe por causa do DynCall (unsafe.Pointer legítimo)
go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/adv-win.exe ./cmd/advplc
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/adv-mac    ./cmd/advplc
```

Zero falha em qualquer um. Premissa absoluta do projeto: nunca quebrar
a capacidade de compilar AdvPL e gerar binário nos 3 SOs. Se a mudança
tocou natives/VM, rodar também `make test` (fixtures reais em
`tests/`) e, se fizer sentido, um teste manual ponta-a-ponta da
feature nova (subir um server de teste, rodar contra um `.prw` real —
não confiar só em `go test`).

## 2. CHANGELOG.md

Fechar a seção `[Não lançado]` (se houver) como
`## [X.Y.Z] — AAAA-MM-DD`, bullets técnicos do que mudou de verdade —
sem prosa de marketing, sem inflar.

## 3. Documentação afetada

Atualizar só o que a mudança realmente toca, entre:
`README.md`, `docs/GUIA_DO_DESENVOLVEDOR_PARA_ADVPP.md`,
`docs/COMPONENT_STATUS.md`, `docs/PROTHEUS_COMPATIBILITY.md`,
`docs/MANUAL_ADVPLC.md`, `docs/tdn-known-limitations.md`,
`docs/tdn-pendencias.md`, `ROADMAP.md` (fechar item se a release
resolveu algo que estava lá).

Conferir: tudo em pt_BR, sem link quebrado pra arquivo que tenha sido
movido/renomeado.

## 4. PDFs

Usar a skill `docs-pdf-regen` — ela decide sozinha quais `.md`/`.pdf`
do repo mudaram desde a última geração e regenera só esses, com o
template já em uso no projeto (`jmpm`, desde 2026-09-05). Não chamar
`md2pdf` manualmente fora dela pra esse passo.

## 5. Commit + push

Commit(s) coeso(s), mensagem descrevendo o conteúdo — **sem
`Co-Authored-By:`/`Claude-Session:`/qualquer trailer de atribuição do
assistente**, por instrução permanente do operador. Se estiver na
`master` (padrão de trabalho deste repo), seguir direto; `git push`.

## 6. Tag + release automática

```bash
make release VERSION=X.Y.Z
```

Cria a tag `vX.Y.Z` e dá push. Dispara
`.github/workflows/release.yml`: cross-compile CLI (3 plataformas) +
build nativo das GUIs Fyne (runners Linux/Windows/macOS) +
`gh release create --generate-notes`.

## 7. Acompanhar o CI até o fim

Monitorar `Release`, `CI` e `Test` (uma notificação só ao final via
poll em background — nunca ficar checando a cada 15s). Se algo
quebrar: investigar log, corrigir, re-tag se precisar. Nunca anunciar
release pronta com CI vermelho.

```bash
gh run list --limit 5
gh run watch <run-id>   # ou poll em background
```

## 8. Conferir os artefatos publicados

```bash
gh release view vX.Y.Z
```

Confirmar que os binários das 3 plataformas + instaladores foram
anexados de verdade, não só que o workflow terminou verde.

## 9. Extensão VS Code (se o compilador embutido nela mudou)

```bash
# tools/vscode-advpl/package.json: bump "version" pra X.Y.Z (ou compatível)
./tools/vscode-advpl/build-vsix.sh X.Y.Z    # cross-compile, embute os 4 binários
vsce publish --packagePath tools/vscode-advpl/advpl-tlpp-advpp-X.Y.Z.vsix
```

Commit `release(vscode): extensão AdvPL/TLPP vX.Y.Z` + tag
`vscode-X.Y.Z` (não colide com o padrão `v[0-9]*.[0-9]*.[0-9]*` do
workflow do compilador). `.vsix` não é versionado no git (gitignored).

## 10. Deploy nas máquinas reais (só se o usuário pedir)

Ordem já usada: `laptop-peder` (local) → `homelab` (Proxmox) →
`lxc101`. Por máquina: backup do binário anterior
(`cp advplc advplc.bak-<data>`), copiar o novo, validar com execução
real (`advplc run <algo>.prw`, nunca só `--version`).

## 11. Persistir na memória cross-agent (mem0)

Buscar antes (`mem0_search`, anti-duplicação). Depois `mem0_add` com o
resultado final: versão, data, o que mudou de essencial, bug real
achado no processo (se houve) — pra nenhum outro agente do ecossistema
refazer essa investigação.

## 12. Reportar ao operador

Resumo com evidência real: link da release, versão confirmada em cada
máquina onde fez deploy, status final do CI. Nunca "pronto" sem ter
visto a saída do comando que prova isso.

## Non-goals

- Não pula o passo 1 (validação) mesmo pra release "só de doc" — o
  gate existe pra pegar regressão que ninguém pediu.
- Não versiona abaixo da última tag existente, nunca, mesmo em
  reescrita de histórico (ver regra do passo 0).
- Não commita nem dá push sem o pedido do usuário pra fazer a release
  (este skill assume que o pedido já veio).
