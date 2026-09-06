# Roadmap

Ideias e pedidos de funcionalidade futura para o AdvPP, ainda não
implementados. Cada item nasce de uma necessidade real encontrada num
projeto que roda sobre o compilador — não é brainstorm especulativo.

## Suporte nativo a Windows Service

**Origem:** projeto `monitor` (monitor de disponibilidade de unidades
Protheus), 2026-09-05 — precisa de um processo AdvPL rodando sempre em
segundo plano no Windows, sobrevivendo a reboot e troca de sessão RDP,
sem depender de ferramenta de terceiro (NSSM/WinSW) nem de escrever um
wrapper Go separado por fora do AdvPP.

**O que seria:** um programa AdvPL compilado via `advplc build`
conseguir se registrar como Windows Service de verdade — implementando
o protocolo do SCM (Service Control Manager) nativamente no runtime do
AdvPP, via `golang.org/x/sys/windows/svc` (biblioteca padrão do
ecossistema Go pra isso). Provavelmente uma nova flag de build (ex:
`--service`) que gera um binário respondendo a `sc.exe create/start/
stop/query`, com um ponto de entrada AdvPL rodando dentro do laço de
vida do serviço (start/stop/pause callbacks).

**Por que interessa além do caso original:** qualquer projeto AdvPL que
precise rodar sem supervisão (o próprio `monitor`, e potencialmente o
`GesCon` se algum dia precisar de um componente de fundo) se beneficia,
sem reintroduzir a mesma decisão de "qual wrapper de serviço usar" a
cada novo projeto.

**Status:** não iniciado. Registrado aqui a pedido do operador para
retomar quando fizer sentido priorizar.

## Bug: `JsonObject:ToJson()` não serializa `JsonObject` aninhado

**Origem:** projeto `monitor`, 2026-09-05 — descoberto testando antes de
escrever um plano de implementação (o formato de `state.json` desenhado
na spec dependia de um `JsonObject` como valor de outro `JsonObject`).

**O que acontece:** `jsonValueString` (`pkg/vm/vm.go:1756`) trata
`StringValue`, `NumberValue`, `BoolValue` e `NilValue` como casos
especiais; qualquer outro tipo — incluindo `*advplrt.ObjectValue`
(`JsonObject`, ou qualquer outra classe) — cai no `default` e vira
`advplrt.ToString(val)` entre aspas. Para um objeto isso produz a
string literal `"Object:JsonObject"`, descartando todas as chaves e
valores internos. Reproduzido:

```advpl
Local oState := JsonObject():New()
Local oEntrada := JsonObject():New()
oEntrada["status"] := "UP"
oState["TCPSP"] := oEntrada
ConOut(oState:ToJson())   // {"TCPSP": "Object:JsonObject"} -- dados perdidos
```

**Fix:** em `jsonValueString`, adicionar um `case *advplrt.ObjectValue`
(ou o tipo concreto usado por `JsonObject`) que serialize
recursivamente as `Props`/`Keys` do objeto como um objeto JSON de
verdade, em vez de cair no `default`. Provavelmente o mesmo tratamento
vale para arrays de objetos aninhados, se ainda não cobertos.

**Impacto no `monitor`:** contornado sem esperar o fix — `state.json`
guarda duas chaves paralelas por unidade (`"<CHAVE>"` e
`"<CHAVE>_LATENCIA"`) em vez de aninhar um objeto `{status, latenciaMs}`
por chave.

**Status:** não iniciado.

## Gap: `FWMBrowse` sem forma de restringir CRUD (somente leitura)

**Origem:** projeto `GesCon` (`src/portal.prw`, `GcPortalBrowse`), achado
ao investigar uma tela pensada como "só consulta".

**O que acontece:** `callFormBrowseMethod` (`pkg/vm/browse.go`) só
implementa `NEW`, `SETALIAS`, `SETDESCRIPTION`/`SETTITLE`, `ACTIVATE`,
`DEACTIVATE`, `DESTROY`. Não existe `SetReadOnly()` nem qualquer outro
método pra desabilitar Incluir/Editar/Excluir. Toda `FWMBrowse` aberta
com o AdvPP expõe CRUD completo por padrão, mesmo quando a tela real do
Protheus (via `MenuDef`/`BrowseDef`) só oferece consulta.

**O que seria:** adicionar `SetReadOnly([lReadOnly])` na classe (default
`.T.` quando chamado sem argumento, espelhando o real) e fazer
`ACTIVATE` respeitar a flag, ocultando/desabilitando as opções de
Incluir/Editar/Excluir do menu de contexto do browse gerado.

**Status:** não iniciado.

## Gap: sem função nativa de comprimento/substring UTF-8-aware

**Origem:** projeto `shortcoder` (renderização de respostas de LLM em
terminal, com acentuação), contornado com uma `Utf8Len()` própria no
fonte AdvPL do projeto.

**O que acontece:** `Len()`, `SubStr()`, `Left()`, `Right()` operam
sobre bytes crus da string Go (`len(string)`), não sobre codepoints
Unicode. Uma string com acentuação (`"é"`, `"ã"`, etc.) conta 2 bytes
por caractere acentuado, quebrando alinhamento de UI em terminal
(box-drawing) e qualquer truncamento por tamanho visual. Não existe
equivalente a um `Len()` ciente de UTF-8 no runtime.

**O que seria:** natives novas (ex.: `U8Len()`, `U8SubStr()`) que operem
sobre `[]rune` (contagem de codepoints via `utf8.RuneCountInString` /
iteração de rune do Go), sem alterar o comportamento existente de
`Len`/`SubStr`/`Left`/`Right` (que devem continuar byte-orientados, por
compatibilidade com AdvPL real). Fora de escopo inicial: largura visual
de caracteres largos (emoji/CJK) — outro nível de complexidade
(East Asian Width), tratar só se algum projeto realmente precisar.

**Status:** não iniciado.

## Levantamento: Node.js embarcado (toolchain de build do `web/`)

**Origem:** pedido direto do operador (2026-09-05) — avaliar
atualização ou substituição do Node.js usado para compilar o frontend
Angular/PO-UI (`web/`, usado por `advplc serve`) por Bun ou Deno,
"o que for mais estável, seguro, compatível e imutável".

**Estado atual (levantado nesta sessão):** Node não é embarcado no
binário do AdvPP — é só uma dependência de build do frontend. `make web`
(`Makefile:23-26`) roda `cd web && npx ng build`, copia o resultado pra
`pkg/webui/dist/` (5,2 MB), que é versionado no git e embutido via
`go:embed` — o binário final (`advplc serve`) não depende de Node em
runtime, só quem re-gera o dist precisa dele. Nenhum workflow de CI
builda `web/` (não há `actions/setup-node` em `.github/workflows/`); o
`dist/` commitado é a fonte da verdade em produção. Node instalado
localmente: v24.14.1. `web/package.json` não declara `engines`
(sem piso mínimo de versão travado). Stack: Angular 21.2 + `@po-ui/
ng-components` 21.23 + `zone.js` 0.16 + Angular CLI/`@angular/build`
21.2 (usa `esbuild` por baixo). `web/node_modules` pesa 369 MB (não
versionado).

**Atualizar o Node atual:** 🟢 baixo risco/esforço — trocar só a versão
instalada localmente (v24 já é recente; conferir se é a LTS ativa na
época) e opcionalmente travar `engines` no `package.json` +
`.nvmrc`/`.node-version` versionado, pra não depender da versão que
"por acaso" está instalada na máquina de quem builda.

**Substituir por Bun:** 🟡 viável com ressalvas — Bun implementa
compatibilidade ampla com o ecossistema npm (resolve `node_modules`,
`package.json`, a maior parte da API `node:*`) e roda `esbuild`/Vite
sem drama; o próprio Angular CLI já revisou compatibilidade com Bun em
versões recentes, mas o encadeamento `@angular/cli` → `@angular/build`
→ builders internos ainda assume Node como runtime "oficialmente
suportado" — Bun tende a funcionar (`bunx ng build`) mas sem garantia
formal da Angular/PO-UI contra regressão silenciosa a cada versão nova
de qualquer uma das duas. Ganho real: binário único, startup mais
rápido, sem `node_modules` de 369 MB (Bun tem cache global de pacotes).

**Substituir por Deno:** 🔴 maior atrito — suporte a `npm:` specifiers
existe, mas o ecossistema Angular CLI depende pesado de resolução
`node_modules` clássica, scripts de `package.json` e APIs `node:fs`/
`node:child_process` que o Deno replica parcialmente e sob permissões
explícitas (`--allow-read`/`--allow-write`/`--allow-run`) — a promessa
de "seguro por padrão" do Deno colide com o jeito que Angular CLI/PO-UI
esperam rodar (acesso livre a arquivo/processo). Provável que builders
do Angular quebrem ou exijam flags extras não documentadas por ninguém
ainda (combinação Angular 21 + PO-UI + Deno é praticamente não testada
publicamente).

**Recomendação (a validar na prática antes de decidir):** não vale
trocar `npx ng build` por Bun/Deno até haver um motivo concreto de dor
(tempo de build, tamanho de `node_modules`, CVE em dependência do
Node) — hoje o Node só roda na máquina de quem regenera `pkg/webui/
dist/`, nunca em produção. Se decidir migrar, começar por Bun (`bunx
ng build` no lugar de `npx ng build`) num branch isolado, comparando
build de saída byte-a-byte contra o `dist/` atual antes de trocar o
`Makefile`; Deno só faria sentido se o Bun falhar.

**Status:** levantamento concluído; nenhuma migração pra Bun/Deno
iniciada (decisão de seguir ou não pendente do operador). O Node local
foi atualizado nesta sessão (2026-09-05): v24.14.1 → **v24.20.0
"Krypton"** (LTS ativa, confirmada via `nodejs.org/dist/index.json`),
tarball oficial baixado e checksum SHA-256 validado contra
`SHASUMS256.txt` antes de substituir `~/.holaboss/node`. Globais
reinstalados nas mesmas versões (`@angular/cli@21.2.16`,
`corepack@0.34.6`, `little-coder@1.14.0`, `prettier@3.8.3`,
`vsce@2.15.0`). `make web` validado com o Node novo: build de
`pkg/webui/dist/` saiu **byte-idêntico** ao já commitado (zero diff),
confirmando que a atualização de patch não alterou o artefato final.
Backup do Node anterior preservado em `~/.holaboss/node.bak-v24.14.1`
(remover quando não precisar mais rebobinar). `engines`/`.nvmrc` ainda
não travados no `web/package.json` — pendência menor, útil se algum
dia mais de uma pessoa/máquina builda o frontend.

## Atualização de Go: v1.24.2 → v1.27.1

**Origem:** pedido direto do operador (2026-09-05) — verificar se o
compilador estava na última versão do Go e, se não, levantar ganhos
relevantes antes de decidir migrar.

**Levantamento (relevante ao AdvPP, das release notes oficiais go.dev):**
- **Green Tea GC** (default desde 1.26): 10-40% menos overhead de GC
  em programas alloc-heavy — perfil direto da VM do AdvPP, que aloca
  muito objeto pequeno por instrução de bytecode.
- **Alocador de memória mais rápido** (1.27): até 30% mais rápido em
  alocações <80 bytes, mesmo perfil da VM.
- **Container-aware GOMAXPROCS** (1.25): respeita o CPU limit real do
  cgroup — relevante pro deploy em LXC 101 (Proxmox).
- **Goroutine leak profile** (estável em 1.27, via `runtime/pprof`):
  detecta goroutine travada e inalcançável — auditoria de graça pros
  pontos que sobem goroutine (`StartJob`, `GRPCServer`, `WSRestServer`).
- **`encoding/json` agora backed por v2** (1.27, automático, mesma
  API): unmarshal bem mais rápido, sem mudar nada de código — usado
  pesado em `TJsonParser`/`GRPCServer`/`MCPServer`.
- Pacote `uuid` novo na stdlib (1.27) — poderia simplificar
  `UUIDRANDOM`/`UUIDRANDOMSEQ` (`ambiente_native.go`) no futuro, hoje
  implementados à mão.
- `go vet` ganhou os analisadores `waitgroup` e `hostport` (1.25).

**Executado nesta sessão:** `go.mod` (`go 1.27.0` / `toolchain
go1.27.1`) e os 3 workflows do GitHub Actions (`GO_VERSION`/
`go-version: '1.24'` → `'1.27'` em `test.yml`, `release.yml`, `ci.yml`)
atualizados. Validado com o Go novo: `go build ./...` (OK), `go vet
-unsafeptr=false ./...` (OK — os warnings de `unsafe.Pointer` que a
versão nova passou a reportar em `dyncall_native.go` já eram
conhecidos e são exatamente o motivo do `-unsafeptr=false` já existir
no `Makefile`/CI, nenhuma regressão real), `go test ./...` (todos os
pacotes OK, nenhuma fixture quebrada), cross-compile limpo pros 3 SOs
(`GOOS=linux/windows/darwin`) do `advplc`, e execução real de um
binário compilado (`advplc run tests/autograd_edge_test.prw` → `OK:
2/2`).

**Toolchain do Go na máquina (laptop-peder) — o que foi atualizado e o
que ficou pendente de root:**
- `~/.local/go-sdks/go1.27.1` — instalação nova, tarball oficial com
  SHA-256 validado contra `go.dev/dl/?mode=json`.
- `~/.local/bin/go` (symlink ativo, resolve antes de tudo no `PATH`) →
  repontado pra `go1.27.1`.
- `~/.bashrc`/`~/.profile` (`GOROOT`/`PATH`) → repontados pra
  `~/.local/go-sdks/go1.27.1` (antes apontavam pro `/usr/local/go`
  root-owned).
- `~/.local/go-1.26` (instalação órfã, go1.26.4, fora do `PATH` ativo)
  → substituída por uma cópia de `go1.27.1`, renomeada pra
  `~/.local/go-1.27.1`; backup antigo em
  `~/.local/go-1.26.bak-go1.26.4`.
- **`/usr/local/go` (root-owned)** — exigia `sudo`, sem senha
  configurada pra automação; o operador rodou manualmente
  (`sudo rm -rf /usr/local/go` + tarball oficial `go1.27.1.linux-
  amd64.tar.gz`, checksum SHA-256 validado antes). Confirmado depois:
  `/usr/local/go/bin/go version` → `go1.27.1`. **Atualizado.**
- **`/usr/lib/go-1.22` (pacote apt `golang-1.22`/`golang-go`, ainda
  1.22.2) — não atualizado, por decisão de escopo.** É gerenciado pelo
  `dpkg`/`apt`; sobrescrever os arquivos na mão corromperia a
  integridade do pacote, e o Ubuntu não empacota Go 1.27 nos
  repositórios padrão. Não está no caminho ativo do `go` (PATH
  resolve por `~/.local/bin/go` primeiro) e não afeta o AdvPP.

**Status:** concluído em todos os locais da máquina (`laptop-peder`)
que fazem sentido atualizar sem quebrar gerenciamento de pacote do SO
— `~/.local/go-sdks/go1.27.1`, `~/.local/bin/go`, `~/.bashrc`/
`~/.profile`, `~/.local/go-1.27.1` (ex-`go-1.26`) e `/usr/local/go`
(root, atualizado pelo operador). Único remanescente em 1.22 é o
pacote apt `golang-1.22`, deixado de propósito por ser gerenciado pelo
sistema operacional, não pelo projeto.
