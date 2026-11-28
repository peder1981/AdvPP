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
git tag -a vX.Y.Z -m "AdvPP X.Y.Z: <resumo>" <commit-do-master>
git push origin vX.Y.Z
```

(Não existe alvo `make release`.) Desde a v4.7.0 o `master` é protegido:
tudo entra por PR com a matriz `Test` verde (4 SOs + `determinism`) —
inclusive o bump da extensão (passo 9), que precisa estar no PR antes da tag. Dispara
`.github/workflows/release.yml`: cross-compile CLI (3 plataformas) +
build nativo das GUIs Fyne (runners Linux/Windows/macOS) +
`gh release create --generate-notes`.

## 7. Acompanhar o CI até o fim

Monitorar `Release`, `CI` e `Test` (uma notificação só ao final via
poll em background — nunca ficar checando a cada 15s). Se algo
quebrar: investigar log, corrigir, re-tag se precisar. Nunca anunciar
release pronta com CI vermelho.

**Preferir `gh run watch`** — já bloqueia até o run terminar e imprime
progresso, sem loop escrito à mão:

```bash
run_id=$(gh run list --branch vX.Y.Z --limit 1 --json databaseId -q '.[0].databaseId')
gh run watch "$run_id" --exit-status
```

Se precisar de um loop manual (ex.: esperar vários workflows
disparados pelo mesmo push), usar o **nome do arquivo em minúsculo**
em `--workflow` (`release.yml`/`test.yml` — o nome de
exibição capitalizado, tipo `Release.yml`, dá `HTTP 404` silencioso se
tiver `2>/dev/null` no comando, e o loop nunca sai). **Nunca** silenciar
stderr num loop de polling — é assim que um 404 vira loop infinito sem
ninguém perceber. Sempre com timeout/nº máximo de iterações:

```bash
i=0
while [ "$(gh run list --branch vX.Y.Z --workflow release.yml --limit 1 --json status -q '.[0].status')" != "completed" ]; do
    i=$((i + 1))
    [ "$i" -gt 60 ] && { echo "timeout esperando o workflow"; exit 1; }  # ~30min a 30s/iteração
    sleep 30
done
```

Ao terminar (por `gh run watch` ou pelo loop), **conferir o `ps aux`**
por processo de polling esquecido antes de seguir pro próximo passo —
já aconteceu de o loop nunca sair e ficar rodando em segundo plano
sem ninguém notar.

## 8. Conferir os artefatos publicados

```bash
gh release view vX.Y.Z
```

Confirmar que os binários das 3 plataformas + instaladores foram
anexados de verdade, não só que o workflow terminou verde. **Baixar e
abrir** pelo menos um pacote (`gh release download vX.Y.Z -p
'advpp-cli-*linux-amd64*'` + `tar xzf` + `./advplc --version`) e rodar o
`install.sh` com `ADVPP_INSTALL_DIR` apontando pra um diretório temporário
— até a v4.7.0 a release saiu verde com tar.gz corrompido.

## 9. Extensão VS Code (sempre — mesma versão, mesmo run)

Desde a v4.10.3 o `.vsix` é gerado pelo job `vsix` do `release.yml`, na
mesma tag, e anexado à release. O job falha se o `package.json` não tiver a
versão da tag ou se o `advplc` embutido não for byte a byte o dos pacotes
CLI. Por isso o bump de `tools/vscode-advpl/package.json` para `X.Y.Z` vai
**no PR da release, antes da tag** — nunca existe release sem extensão.

Publicar exatamente o asset da release (nunca um `.vsix` gerado local):

```bash
gh release download vX.Y.Z -p 'advpl-tlpp-advpp-*.vsix' -D /tmp/vsix
vsce publish --packagePath /tmp/vsix/advpl-tlpp-advpp-X.Y.Z.vsix
```

Não existe mais tag `vscode-X.Y.Z` separada: a tag `vX.Y.Z` cobre os dois.
O Marketplace leva alguns minutos para mostrar a versão nova (`vsce show`).

## 10. Deploy nas máquinas reais (só se o usuário pedir)

Ordem já usada: `lab-local` (local) → `homelab` (Proxmox) →
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
