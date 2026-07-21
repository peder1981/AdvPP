# Changelog

Todas as mudanças notáveis deste projeto são documentadas aqui.

## [1.12.0] — 2026-07-21

Fechamento de lacunas da VM/compilador descobertas ao escrever um LLM de Markov
em AdvPL puro (`pt_llm.prw`): três bugs de controle de fluxo e a ausência de I/O
de disco e chamada de sistema.

### Correções de controle de fluxo

- **`Loop` (continue) e `Exit` (break) agora funcionam** — `pkg/compiler/codegen.go`.
  Antes, ambos emitiam um `OP_JUMP` para sentinelas `-998`/`-999` que **nunca
  eram corrigidas**, então `Loop` estourava a VM com índice de instrução
  negativo e `Exit` saltava para lixo. Adicionada uma pilha de `loopContext`
  que acumula os jumps de break/continue de cada loop e os corrige no fim de
  `compileFor`/`compileWhile` (continue → incremento/condição, break → fim do
  loop). Funciona com loops aninhados — `Exit` quebra apenas o loop interno.
- **`For ... Step` negativo (descendente) agora itera** — novo opcode
  `OP_FORLOOP_CMP`. A condição de continuação era fixa em `var <= end`
  (`OP_LTE`), então qualquer `For i := N To 1 Step -1` executava zero vezes. O
  novo opcode recebe `[var, end, step]` e escolhe `<=` ou `>=` **pelo sinal do
  step em tempo de execução** — cobre `Step -1`, `Step -2` e até step com valor
  variável. Como efeito colateral, o step passou a ser avaliado uma única vez
  (numa local escondida), em vez de reavaliado a cada iteração.

### Correção de semântica de hash

- **Chaves de `JsonObject` por bracket agora são case-sensitive** —
  `pkg/vm/vm.go`. `OP_ARRAY_GET`/`OP_ARRAY_SET` aplicavam `strings.ToUpper` na
  chave, colapsando `h["Brasil"]` e `h["brasil"]` na mesma entrada e inutilizando
  o objeto como dicionário. Removido o upper apenas do acesso por bracket; o
  acesso por ponto (`obj:Prop`) continua case-insensitive (semântica correta de
  objeto AdvPL). Efeito colateral positivo: `toJson()` agora preserva o case
  original das chaves.

### Novas funções nativas — I/O de disco, arquivo e sistema

- **I/O de disco**: `MemoRead(cArq)`, `MemoWrite(cArq, cTexto)` (+ alias
  `MemoWrit`), `FErase(cArq)`.
- **API de handle de arquivo (streaming)**: `FCreate`, `FOpen`, `FReadStr`,
  `FWrite`, `FSeek`, `FClose`, `FError` — com tabela de handles na VM
  (`fileHandles map[int]*os.File`). Permite ler/gravar arquivos grandes em
  blocos sem carregar tudo na memória. A leitura usa `FReadStr` (retorna a
  string) em vez de `FRead` com buffer por referência, porque os natives da VM
  recebem valores, não lvalues.
- **Chamada de sistema**: `WaitRun(cCmd)` executa no shell do SO
  (cross-platform `sh -c` / `cmd /c`), herda stdio, espera e retorna o exit
  code. A captura de stdout é feita pelo padrão AdvPL de redirecionar para
  arquivo e ler com a API de handle (funciona em streaming, para saída grande).

### Exemplo novo

- **`pt_llm.prw`** — modelo de linguagem de Markov de ordem variável (nível de
  byte, ordens 1–6 com backoff) escrito inteiramente em AdvPL, que lê e gera
  português do Brasil. Roda com `advplc run pt_llm.prw`, com auto-teste incluído.

### Testes

- Suíte Go completa verde (`go test ./...`): `pkg/compiler`, `pkg/vm` (via
  `cmd/advplc`), `pkg/runtime` e todos os demais pacotes. Fixtures `.prw` novos
  cobrindo os três bugs de loop, o case-sensitivity, e as APIs de I/O/handle/sistema.

## [1.11.0] — 2026-07-11

### Debugger real via DAP (breakpoints/step/variáveis) — launch e attach

Pedido do usuário: testes de compilação/debug completos antes de estender o
mesmo suporte ao plugin de VS Code na frente web. Resultado: `advplc` ganha
um adaptador completo de Debug Adapter Protocol, cobrindo os dois modos de
execução que o compilador já suportava.

- **`advplc debug`** (modo *launch*, sobre stdio): compila e executa um
  único fonte com breakpoints, step over/in/out, call stack e inspeção de
  variáveis locais. Implementado como hook por instrução na `runLoop` da VM
  (`pkg/vm/debug.go`) — usa a linha-fonte que cada instrução de bytecode já
  carregava e o call-frame stack já existente, custo zero quando nenhum
  debugger está anexado (`if v.debugger != nil`). Servidor DAP em
  `pkg/dap/`, stdlib puro, mesmo padrão de framing do LSP
  (Content-Length). `os.Stdout` é redirecionado para `/dev/null` dentro do
  adaptador — `ConOut`/`ConOutW` sempre espelham no stdout real, que aqui é
  o próprio transporte DAP; sem isso o primeiro `ConOut()` do programa
  depurado corrompia o stream JSON no meio da sessão.
- **`advplc serve --debug-port N`** (modo *attach*): `advplc serve` é um
  processo de vida longa com uma VM por sessão de browser — arquitetura
  diferente de `debug`, que expõe um listener TCP DAP separado da porta
  HTTP. Cada nova sessão de browser oferece sua VM ao servidor DAP anexado
  (`dap.Server.OfferSession`), que só assume a VM depois que o cliente
  concluiu `attach` + `configurationDone` — deliberadamente **uma sessão
  depurada por vez** (sem multiplexação de threads DAP; uma segunda aba
  conectando durante uma depuração roda normal, sem debugger). Saída do
  console espelha simultaneamente no browser e no Debug Console do editor.
- Testado ponta a ponta duas vezes: cliente DAP bruto em Python (breakpoint
  bate na linha certa, variáveis corretas ANTES da instrução executar, call
  stack aninhado correto) e ao vivo pela GUI real do NeuralInverse/VS Code
  (toolbar mostra ícone de "disconnect" pra attach vs "stop" pra launch —
  o editor distingue os dois modos automaticamente).

### Extensão VS Code (`advpl-tlpp`) publicada no Marketplace

`tools/vscode-advpl/` — antes só sideload via `.vsix`, agora publicada em
`marketplace.visualstudio.com/items?itemName=PederMunksgaard.advpl-tlpp`.
Cobre: syntax highlighting completo, keybindings reais (`Ctrl+F9` build,
`F9` run, `Ctrl+Shift+F9` compile, `Ctrl+Alt+F9` serve), e o debugger DAP
(launch zero-config via `F5`, attach via comando dedicado). Motivo deste
release: a extensão publicada já chamava `advplc debug`/`--debug-port`, que
não existiam no compilador oficial (v1.10.3) — esse release fecha essa
lacuna entre o que estava publicado no Marketplace e o que o compilador
publicado sabia fazer.

### Verificação antes do release

Sweep completo contra os 2 corpora reais (811R4 + Protheus 12.1.2510, 500
arquivos amostrados): 96,2% de aprovação, **as mesmas 19 falhas** que já
existiam no v1.10.3 publicado — zero regressão introduzida. Cross-compile
verificado para os 4 alvos oficiais (linux/amd64, linux/arm64,
windows/amd64, darwin/arm64). Diff revisado linha a linha contra v1.10.3:
mudanças estritamente aditivas, nenhum caminho existente (`run`/`compile`/
`build`/`check`/`ast`/`bytecode`) alterado.

## [1.10.3] — 2026-07-11

### 3 bugs reais do executável standalone no Windows, encontrados testando de verdade

Pedido do usuário depois da v1.10.2: "testar o binário standalone gerado no
Windows também". Os binários das ferramentas (advplc.exe/adveditor.exe/
advpp-ide.exe) já eram compilados nativamente no Windows pelo workflow de
release, mas o RECURSO de gerar um executável standalone a partir de um
programa AdvPL nunca tinha sido exercitado de verdade nesse SO — só
localmente em Linux. Adicionado um step de CI dedicado (Windows apenas,
`.github/workflows/test.yml`) que builda E RODA um executável standalone
gerado a partir de um fixture simples (`tests/standalone_console_test.prw`)
num runner `windows-latest` de verdade — não bastava só compilar, o teste
efetivamente executa o binário e verifica a saída. Isso encontrou 3 bugs
reais, um atrás do outro:

1. **Mover o executável entre drives diferentes falhava**: `os.Rename`
   dá erro `"cannot move the file to a different disk drive"` no Windows
   quando origem e destino estão em volumes diferentes — exatamente o
   layout dos runners `windows-latest` do GitHub Actions (temp em `C:`,
   checkout em `D:`), plausível também em máquinas reais. Corrigido com
   fallback copy+remove (`moveFile`) quando o rename falha — cobre também
   o `EXDEV` equivalente no Unix para mounts diferentes.
2. **O fallback de cópia então falhava ao remover a origem**: o handle do
   arquivo de origem ainda estava aberto (só fechado via `defer`, que roda
   DEPOIS do `return`) quando `os.Remove` tentava apagá-lo — o Windows
   recusa apagar um arquivo com handle aberto (ao contrário do POSIX, que
   permite `unlink` em arquivo aberto). Corrigido fechando o handle
   explicitamente antes do `os.Remove`, não via `defer`.
3. **O executável gerado rodava corretamente (MSDIALOG funcionava, saída
   aparecia) mas nunca se fechava sozinho ao terminar** — ficava
   pendurado precisando ser morto à força. Rastreado com um trace opt-in
   (`ADVPP_STUB_TRACE=1`) instrumentando cada etapa do stub: `a.Quit()`
   (chamado pela goroutine de fundo da VM ao terminar) retornava
   normalmente, mas o event loop do Fyne/GLFW dentro de `w.ShowAndRun()`
   nunca notava o canal de encerramento fechado e nunca retornava —
   especificamente neste ambiente. Uma primeira tentativa (remover uma
   chamada duplicada a `Show()` antes de `ShowAndRun()`, que também já
   chama `Show()` internamente) não resolveu. Corrigido de forma mais
   robusta: como o executável standalone é um script de vida curta, não
   um app interativo de longa duração, não há necessidade de um handshake
   gracioso de fechamento de janela via Fyne — a goroutine agora chama
   `os.Exit(0)` diretamente assim que `v.Run()` termina, encerrando o
   processo incondicionalmente independente do que `w.ShowAndRun()`
   estiver fazendo. Caminho de erro inalterado: a janela continua aberta
   para o usuário ver o erro, fechando manualmente.

Cada um dos 3 bugs foi confirmado corrigido rodando de verdade no runner
Windows do CI (não just localmente) antes de seguir para o próximo — o
teste de CI também foi corrigido no meio do caminho para falhar
corretamente quando o processo precisa ser morto à força (a versão
anterior só conferia o conteúdo do log, deixando passar silenciosamente
um hang que precisou de kill manual).

Sem regressão: `go build ./...`, `go vet ./...`, `go test ./...`,
`make test` (31 fixtures, incluindo o novo fixture de smoke test),
sweep completo do corpus de 500 arquivos (500/500) a cada commit.

## [1.10.2] — 2026-07-11

### MSDIALOG renderiza de verdade no advpp-ide (não só no modo web)

Pedido do usuário: testou o FrameworkClassesTest e o botão Compilar no
advpp-ide e achou dois problemas reais. Investigação mostrou que o segundo
era bem maior do que parecia: `DEFINE MSDIALOG`/`ACTIVATE MSDIALOG` (o DSL
clássico `@ x,y SAY/GET/BUTTON`) e `FWMBrowse` dependem de interfaces
opcionais (`DialogUI`/`BrowseUI`, `pkg/vm/dialog.go`/`pkg/vm/browse.go`) que
só o renderer web (`advplc serve`) implementava — `pkg/ui.FyneUIProvider`
(usado por advpp-ide) só tinha os 4 primitivos de mensagem
(`MsgInfo`/`MsgStop`/`MsgAlert`/`MsgYesNo`). Qualquer programa com MSDIALOG
literalmente **falhava** ("MSDIALOG: requer o modo web") ao rodar pelo
IDE desktop.

- **`FyneUIProvider.Dialog`** (`pkg/ui/msdialog.go`, novo): implementa
  `vm.DialogUI` nativamente em Fyne — renderiza o spec (linhas SAY/GET
  agrupadas pela heurística de grade y/x já existente no VM, botões no
  rodapé) como um `dialog.CustomDialog` de verdade, com um `chan
  dialogAction` bloqueando a goroutine chamadora até o clique de um botão
  (ou Escape/fechamento), devolvendo os valores digitados para writeback
  nas variáveis AdvPL — mesmo protocolo JSON que o renderer web já usa,
  então o VM não precisou mudar nada.
- **`v.Run()` precisou sair da goroutine principal do Fyne**
  (`cmd/advpp-ide`, `run()`): como `Dialog()` bloqueia quem chamou até o
  clique do botão, e esse clique só é processado pelo event loop principal
  do Fyne, rodar `v.Run()` direto no handler do menu "Run" causava
  deadlock garantido assim que um programa abrisse um MSDIALOG. Corrigido
  rodando a VM em `go func() { v.Run() ... }()`.
- **Bug real encontrado de bônus, mesma causa raiz**: `MsgYesNo` sempre
  devolvia `false` independente da escolha do usuário —
  `dialog.ShowConfirm` é assíncrono (o callback só roda depois), mas o
  código antigo lia a variável de resultado ANTES do callback rodar.
  Corrigido com o mesmo padrão de canal bloqueante — seguro agora que
  `v.Run()` não roda mais na goroutine principal.
- **`ConOut`/console não aparecia em lugar nenhum visível no IDE** — a VM
  escrevia direto em `os.Stdout` (sem terminal visível num app GUI
  empacotado) porque `run()` nunca chamava `v.SetOutputWriter`. Corrigido
  roteando para o próprio painel de saída do IDE.
- **Botão "Compile" agora gera um arquivo de verdade**: antes só reportava
  contagem de funções/classes e descartava o bytecode compilado. Agora
  salva `<arquivo>.bytecode` (mesmo formato que `advplc compile` já
  gerava), carregável depois via `advplc run` ou o próprio botão Run do
  IDE sem recompilar.
- **Novo item de menu "Build standalone executable..."**: mesmo mecanismo
  de `advplc build`, extraído para `compiler.BuildStandalone`
  (`pkg/compiler/standalone.go`) e compartilhado entre CLI e IDE. Só
  funciona rodando de dentro de (ou apontando via `ADVPP_SRC` para) um
  checkout completo do código-fonte do AdvPP com o toolchain Go instalado
  — o stub gerado importa `pkg/compiler`/`pkg/vm` deste módulo, que não
  está publicado em lugar nenhum que `go build` consiga buscar sozinho;
  não é algo que funcione a partir de um pacote de release baixado
  isoladamente. Bug real corrigido de quebra: a detecção antiga da raiz do
  projeto (`filepath.Dir(filepath.Dir(caminhoDoFonte))`) só funcionava por
  coincidência para fontes dentro de `tests/` deste repo — quebrava para
  qualquer arquivo real de usuário. Substituída por busca robusta (sobe a
  árvore de diretórios a partir do cwd e do executável em execução,
  procurando o `go.mod` do módulo certo).
Sem regressão: `go build ./...`, `go vet ./...`, `go test ./...`
(testes novos em `pkg/ui` para o contrato JSON do diálogo e em
`pkg/compiler` para a busca de módulo), `make test` (30 fixtures), sweep
completo do corpus de 500 arquivos (500/500). Verificado visualmente via
Xvfb: MSDIALOG abre, campos são editáveis, clique em botão fecha o
diálogo e grava os valores de volta corretamente nas variáveis AdvPL.

### Executáveis standalone ganham UI Fyne e banco de dados completos

Pedido do usuário: quem compila um binário standalone via `advplc build`
(ou o novo botão do advpp-ide) deveria conseguir usar TODAS as
características da linguagem, não só a parte headless. O stub gerado
(`pkg/compiler/stub_template.go`) rodava com `uiEnabled=false` e sem
nenhum provider de UI nem banco de dados anexado — um programa usando
`MsgInfo`, `MSDIALOG`, `FWMBrowse` ou acesso a tabela simplesmente perdia
essa funcionalidade quando virava um `.exe`/binário standalone, sobrando
só a saída de console.

- O stub agora abre uma janela Fyne pequena que funciona ao mesmo tempo
  como console de saída (`ui.OutputConsole`, via `v.SetOutputWriter`) e
  como parent dos diálogos (`ui.FyneUIProvider`) — a MESMA dupla função
  que corrige de brinde um problema real no Windows: um binário Fyne
  gerado como GUI-subsystem não tem terminal anexado, então `ConOut`
  simplesmente desaparecia sem essa janela.
- Banco de dados conectado com o mesmo `SetDBFactory`/`ResolveDatabasePath`
  local (`./advpp.db`) usado por advplc/adveditor/advpp-ide.
- `pkg/ui.ConsoleWriter` extraído (antes privado em `cmd/advpp-ide`) para
  ser reaproveitado pelo stub também.
- Verificado visualmente via Xvfb: binário standalone gerado a partir de
  `tests/msdialog_test.prw` abre a janela, renderiza o MSDIALOG, grava os
  valores digitados e encerra sozinho ao fim da execução (`a.Quit()`), sem
  deixar processo pendurado; um programa 100% console (só `ConOut`)
  também abre a janela (double função de terminal), mostra a saída e
  encerra sozinho sem exigir interação do usuário.

### FWMBrowse renderiza de verdade no advpp-ide + bug real de recno corrigido

Continuação do item anterior: `FWMBrowse` também dependia da mesma
`BrowseUI` só implementada pelo renderer web.

- **`FyneUIProvider.Browse`** (`pkg/ui/browse.go`, novo): grid real
  (`widget.Table`, linha 0 = cabeçalho) com botões Novo/Editar/Excluir/
  Fechar, mesmo protocolo JSON (`browseSpec`/`browseAction`) que o
  renderer web já usa. Editar/Novo abrem `dialog.ShowForm` pré-preenchido;
  Excluir pede confirmação antes.
- **Bug real encontrado e corrigido em `pkg/vm/browse.go`** (compartilhado
  pelos dois renderers, não é algo introduzido agora): `browseItems`
  selecionava o pseudo-campo `rowid` sem apelido e procurava o resultado
  pela chave `"ROWID"` — mas o SQLite devolve o NOME da coluna de
  resultado usando o nome da própria coluna `INTEGER PRIMARY KEY` da
  tabela quando ela tem uma (`R_E_C_N_O_`, em toda tabela gerenciada pelo
  AdvPP desde a convenção de exclusão lógica da v1.10.1) — não literalmente
  `"rowid"`. Na prática, `recno` sempre voltava como 0, e todo "Editar"
  virava um INSERT duplicado em vez de um UPDATE. Corrigido apelidando a
  coluna explicitamente (`AS browse_recno_`), o que torna o nome do
  resultado independente do schema da tabela.
- Verificado visualmente via Xvfb, ciclo completo: abrir browse com dados
  reais (SX3 + SA1), selecionar+editar um registro (grava no lugar certo,
  sem duplicar), Novo (insere), Excluir (soft-delete via `D_E_L_E_T_`),
  Fechar (retorna o controle ao programa AdvPL).

### 8 classes complexas do framework: adiadas por decisão explícita do usuário

Pedido original também citava `FWWizardControl`, `FWDynDialog`, `FWPanel`,
`FWGroupBox`, `FWTabs`, `FWSplitter`, `FWTreeView`, `FWListView`. Antes de
investir nelas, verificado o uso real nos dois corpora completos do
usuário (811R4 + 12.1.2510, ~30.000 arquivos): aparecem em **0 a 1
arquivo no total** (`FWWizardControl` em 1 arquivo do 12.1.2510; as
outras 7, zero). Além disso, nenhuma delas tem hoje um jeito de ser
populada a partir de AdvPL real — não existe `TButton():New()`,
`TabPage():New()` etc. construível em AdvPL; existem só como structs Go
internos (`pkg/mvc/view.go`) nunca conectados ao VM/bytecode. Renderizá-las
de verdade exigiria inventar uma API de construção de widgets do zero,
sem nenhum uso real nos corpora para validar contra. Apresentados os
números, o usuário optou por não investir nisso agora — permanecem como
estruturas de dados Go sem renderer (nem web, nem desktop), documentado
aqui como decisão consciente, não lacuna esquecida.

Sem regressão em todo o arco: `go build ./...`, `go vet ./...`,
`go test ./...`, `make test` (30 fixtures), sweep completo do corpus de
500 arquivos (500/500) a cada passo.

## [1.10.0] — 2026-07-11

### Banco de dados local automático — RetSqlName/DbSelectArea/GetArea funcionam sem dicionário configurado

Pedido do usuário: funções de acesso a tabela (`RetSqlName`, `DbSelectArea`,
...) deveriam funcionar independente de existir um dicionário de dados
configurado, e cada diretório de trabalho deveria ganhar seu próprio banco
SQLite automaticamente — sem exigir configuração prévia via `advcfg` — de
forma que `advcfg`/`adveditor` rodados no MESMO diretório logo depois já
enxerguem esse banco e permitam criar tabelas/campos/índices nele.

- **`ResolveDatabasePath`** (`pkg/tools/shared`, usado por advplc/advcfg/
  adveditor/advpp-ide): quando não há caminho explícito, variável de
  ambiente `ADVPP_DB`, nem um `~/.advpp/advpp_config.json` que REALMENTE
  exista em disco (antes o valor sintético que `LoadConfig` sempre devolve
  mascarava "nada configurado" como se fosse o banco global já escolhido),
  o padrão agora é um banco LOCAL `./advpp.db` no diretório de trabalho
  atual — não mais o global `~/.advpp/ADVPP.db`. O banco global só volta a
  valer depois que o usuário configura explicitamente via `advcfg`.
  Removidos os candidatos legados `./data/ADVPP.db`/`./data/
  advpl_dictionary.db` (simplificação).
- **`OpenSQLite`** (ponto único de abertura, `pkg/tools/shared`): agora
  cria um arquivo vazio ANTES de abrir quando o caminho não existe —
  `sql.Open`+`Ping` sozinhos não garantiam que o arquivo aparecesse em
  disco imediatamente (o driver só materializa no primeiro INSERT/CREATE
  real, que podia nunca acontecer se a primeira operação fosse uma leitura
  que falha por tabela inexistente). Sem isso, `advcfg`/`adveditor`
  rodados logo em seguida no mesmo diretório não viam banco nenhum para
  abrir.
- **`attachDatabase`** (`cmd/advplc`): removido o `os.Stat` que pulava a
  conexão inteira quando o arquivo ainda não existia — o VM agora sempre
  anexa um banco (criado na hora se preciso), em vez de rodar sem nenhum
  quando nada foi configurado.
- **Novas nativas**: `RetSqlName(alias)` devolve o próprio alias em
  maiúsculas (sem um dicionário SX2 carregado, é assim que as tabelas
  locais deste VM são nomeadas — funciona mesmo sem dicionário nenhum,
  como pedido). `GetArea()`/`RestArea(alias)` salvam/restauram a área de
  trabalho atual. `Alias()` (antes sempre `""`) agora devolve a área atual
  de verdade.

`pkg/tools/shared` ganhou testes unitários pela primeira vez (isolados via
`HOME`/diretório de trabalho temporários — nunca tocam a config real do
usuário). Sem regressão: `go test ./...`, 30 fixtures, sweep completo do
corpus de 500 arquivos.

## [1.10.1] — 2026-07-11

### Exclusão lógica (recno/D_E_L_E_T_/R_E_C_D_E_L_) e consolidação advcfg → adveditor

Pedido do usuário: adotar o padrão clássico Protheus de exclusão lógica nas
tabelas gerenciadas pelo compilador/ferramentas, e descontinuar o `advcfg`
por ser funcionalmente redundante com o `adveditor` — mantendo só uma
ferramenta gráfica de banco de dados, evoluída para cobrir tudo que o
`advcfg` fazia (criar/editar/excluir tabelas, campos e índices), com o
`adveditor` como ponto único de evolução futura (ex.: outros bancos além de
SQLite).

- **Exclusão lógica** (`pkg/tools/shared/database.go`): toda tabela criada
  via `CreateTable` ganha 3 colunas de sistema automáticas —
  `R_E_C_N_O_` (INTEGER PRIMARY KEY AUTOINCREMENT), `D_E_L_E_T_` (`' '`/`'*'`)
  e `R_E_C_D_E_L_` (0/1, espelho booleano para filtro SQL nativo). Leituras
  (`GetData`/`GetRecord`/`Count`/`Sum`) filtram `R_E_C_D_E_L_ = 0` por
  padrão; `DeleteRecord` passou a fazer UPDATE (marca como excluído) em vez
  de DELETE físico; `RecallRecord` reverte a exclusão; `Pack` purga
  fisicamente os registros marcados e roda VACUUM. Colunas de sistema ficam
  ocultas em `loadStructure()` e nomes de coluna são validados contra
  SQL injection (`validIdentifier`) antes de entrar na query.
- **`cmd/advcfg` removido inteiramente** (402 linhas) — assim como
  `pkg/tools/shared/dictionary.go` (579 linhas, abstração SX2/SX3/SIX
  específica do advcfg, sem uso fora dele) e `docs/MANUAL_ADVCFG.md`.
  `Makefile` e o workflow de release não compilam/empacotam mais o binário.
- **`cmd/adveditor` ganhou os métodos que faltavam**: CRUD completo de
  registro (Incluir/Alterar/Excluir com diálogos de formulário), e um novo
  menu "Tabela" com Nova Tabela (editor de campos dinâmico), Excluir
  Tabela, Adicionar/Remover Coluna, Criar/Excluir Índice — tudo com
  diálogos reais em vez de stubs.
- **3 bugs reais encontrados por teste visual** (Xvfb + xdotool +
  screenshot, não apenas `go build`/`go test`): a árvore de navegação
  nunca havia renderizado nenhum dado nesta ferramenta — faltava
  `ExtendBaseWidget` no widget customizado, e o código tratava a raiz da
  árvore como um ID fixo em vez do ID vazio convencionado pelo framework
  de UI; a grade de dados nunca era recarregada depois de
  Incluir/Alterar/Excluir porque a rotina de reabrir tabela detectava
  "já aberta" e devolvia dados obsoletos em vez de recarregar; o diálogo
  de "ver estrutura" renderizava como uma fresta de ~1px por falta de
  `Resize()` explícito no contêiner de rolagem, e perdia a formatação
  porque reprocessava texto já convertido como se ainda fosse markdown.

Sem regressão: `go build ./...`, `go vet ./...`, `go test ./...`,
`make test` (30 fixtures), sweep completo do corpus de 500 arquivos
(500/500).

### advpp-ide: syntax highlight, banco auto-provisionado e integração com AdvEditor

Pedido do usuário: fazer o advpp-ide acompanhar a evolução do adveditor — não
só visualmente, mas também com syntax highlight, as mesmas características
de acesso/criação de banco de dados, e permitir abrir o AdvEditor a partir
do próprio IDE. Também pedido: auditar se o advpp-ide reflete todas as
evoluções do compilador até aqui.

- **Syntax highlight real** (`pkg/ui/editor.go`, `CodeEditor`): editor
  colorido para AdvPL/TLPP (palavras-chave, tipos, strings, comentários,
  números, diretivas `#`). `widget.Entry` do Fyne não suporta cor por
  token, então o editor alterna entre um `widget.Entry` nativo (100% do
  comportamento de edição — cursor, seleção, clipboard, IME) enquanto tem
  foco, e um preview colorido (`widget.RichText`) somente-leitura assim que
  perde o foco ou um arquivo é aberto; um clique no preview volta ao modo
  de edição. As listas `advplKeywords`/`advplTypes` já existiam no arquivo
  mas nunca tinham sido usadas — a UI mostrava um `widget.Entry` puro sem
  nenhuma coloração.
- **3 bugs reais de renderização Fyne encontrados via teste visual**
  (Xvfb+xdotool+screenshot, invisíveis a `go build`/`go test`) ao construir
  o alterna-preview: (1) `widget.RichText` só resolve seu
  `BaseWidget.impl` preguiçosamente, no primeiro `CreateRenderer()`/
  `MinSize()` — se o widget é escondido antes da primeira pintura, esse
  gatilho nunca ocorre e todo `Refresh()`/`Show()` posterior vira no-op
  silencioso para sempre (corrigido chamando `MinSize()` uma vez no
  construtor). (2) `Container.Refresh()` (Fyne v2.4.4) nunca chama
  `SetDirty()` no canvas — só `Container.Move()`/`Hide()` chamam — então
  trocar `Objects` de um `container.Stack` em runtime e chamar só
  `Refresh()` não repinta a tela (corrigido forçando um `Move()` para a
  própria posição atual logo depois). (3) o bug real, mais sutil: o
  widget "catcher" transparente usado para capturar o clique que volta ao
  modo edição estava pintando um retângulo *opaco* da cor de fundo por
  cima do preview colorido (`container.Stack` pinta objetos depois por
  cima dos anteriores) — cobria o próprio texto que deveria deixar
  visível. Corrigido trocando para `color.Transparent`.
- **Banco de dados auto-provisionado**: `run()` usava o padrão antigo
  (`os.Stat` + `SetDBEngine` condicional, pulando a conexão se o arquivo
  ainda não existisse) — trocado pelo mesmo padrão `SetDBFactory` de
  `attachDatabase` já usado em advplc desde a v1.10.0 anterior, então
  `RetSqlName`/`DbSelectArea` funcionam a partir do advpp-ide sem
  configuração prévia, com o mesmo banco local `./advpp.db`.
- **Menu Tools → "Open AdvEditor (database)"**: lança o binário `adveditor`
  como processo separado, procurando primeiro ao lado do executável do
  advpp-ide (mesmo layout dos pacotes de release) e depois no PATH.
- **Versão real no About/título**: `advplc`/`adveditor`/`advpp-ide` já
  recebiam `-X main.version=` via `make release`, mas só `advplc`
  declarava a variável `version` — nos outros dois o link `-X` era um
  no-op silencioso e a UI sempre mostrava texto hardcoded ("v1.0",
  "Version 1.0.0"). Corrigido nos dois.
- **Auditoria de paridade com o compilador**: advpp-ide importa
  `pkg/compiler`/`pkg/vm`/`pkg/parser` diretamente do mesmo módulo, então
  motor LLM, MCP e `#command` já chegavam automaticamente — o único gap
  real era o padrão de banco de dados acima.

Sem regressão: `go build ./...`, `go vet ./...`, `go test ./...` (incluindo
testes novos de `pkg/ui` para o tokenizer de highlight), `make test` (30
fixtures), sweep completo do corpus de 500 arquivos (500/500).

### Fix: testes de `ResolveDatabasePath` falhavam no CI do Windows

`config_test.go` comparava o resultado de `ResolveDatabasePath` contra
strings absolutas estilo Unix hardcoded (`/custom/path.db`), mas a função
sempre normaliza via `filepath.Abs`, que no Windows reescreve esse literal
como `C:\custom\path.db` (ancorado na unidade atual) em vez de preservar a
string literal — e o teste que checava a prioridade do config real em disco
usava um caminho que `filepath.IsAbs` nem reconhece como absoluto no
Windows (sem letra de unidade), então o próprio código de produção caía no
fallback local em vez de exercitar o caminho testado. Corrigido calculando
o valor esperado com o mesmo `filepath.Abs` (não mais um literal Unix) e
usando um caminho genuinamente absoluto (`t.TempDir()`) no teste do config.
Confirmado verde nos 3 SOs via GitHub Actions após o fix.

## [1.9.1] — 2026-07-11

### 4 bugs reais de parser encontrados em validação fora do corpus de amostra

- `SET DATE FORMAT "dd/mm/yyyy"` (Clipper clássico sem `TO`) não era
  reconhecido pelo dispatcher de `SET`, que só cobria `SET <opção>
  TO/ON/OFF/OF ...`.
- Nome de variável colidindo (case-insensitive) com uma keyword de tipo
  TLPP (`Date`, `Array`, `Object`, ...) era rejeitado em posição de
  declaração (`Local`/`Private`/`Public`/`Static`), que exigia
  `TOKEN_IDENT` estrito em vez de aceitar keyword também — já aceito em
  nome de parâmetro de função, agora consistente também em declaração.
- Keyword-colisão como valor puro no fim de expressão (ex.: `Return
  <nomeQueColideComKeyword>`, sem token de continuação depois na mesma
  linha para o heurístico existente reconhecer) — fallback final
  adicionado em `parsePrimary`: quando nenhuma outra forma bate, uma
  keyword em posição de operando vira identificador comum.
- `(ALIAS_MACRO)->campo := valor` onde `ALIAS_MACRO` é um `#define` para
  uma STRING (idioma real e comum para nomear alias de tabela temporária)
  — depois da expansão da macro, `alias->campo` só era reconhecido quando
  o lado esquerdo do `->` era um `*ast.Ident`; com string literal, o
  `->campo` era descartado silenciosamente e a atribuição sobrava com a
  STRING como alvo ("unsupported assignment target: *ast.StringLit").
  Corrigido em `parsePostfix` para os dois lugares que constroem
  `FieldAccess` a partir do lado esquerdo de `->`.
- Mensagem de erro do codegen para alvo de atribuição não suportado agora
  inclui o número da linha (facilita bisseção de bugs reais).

Sem regressão: `go test ./...`, 30 fixtures, sweep completo do corpus de
500 arquivos.

## [1.9.0] — 2026-07-11

### Rodada de otimizações de performance (compilador, VM, motor de inferência LLM)

Todas as mudanças abaixo são puramente de performance — comportamento e saída
idênticos, validados por toda a suíte de testes (incluindo `TestValidateAgainstLlamaCPP`,
que compara geração greedy token a token contra o llama.cpp de referência) e
pelo sweep completo do corpus Protheus (500/500, 100% mantido). Descobertas via
`pprof` sobre um forward pass real (Falcon3-3B-1.58bit) e uma compilação real
(arquivo Protheus de 3,6MB) — não especulação.

**Motor de inferência LLM (`pkg/llm/`) — ~14x mais rápido por forward pass
nesta máquina (5,27s → 0,36s):**
- `Σq` (correção do caminho AVX2 de `MatMulI2S`) era recalculada em TODA
  linha do peso via `sumInt8`, embora seja o mesmo `q` (ativação quantizada)
  para as `NRows` linhas — ~32% do tempo total, hoisted para fora do loop de
  linhas (calculada uma vez por chamada de `MatMulI2S`).
- `Float16ToFloat32` trocou a decodificação bit-a-bit por uma tabela de 65536
  entradas pré-computada em `init()` — usada por `MatMulF16` (a maior matmul
  do forward pass, projeção de saída com `vocab_size` linhas), que sozinha
  era ~34% do tempo antes desta troca. Validada exaustivamente contra a
  implementação de referência para os 65536 padrões de bit possíveis.
- Novo kernel `dotF16BlocksAVX2` (amd64, `simd_amd64.s`) usa F16C
  (`VCVTPH2PS`, conversão half→float em hardware) + FMA (`VFMADD231PS`) para
  `MatMulF16` — mesmo que a tabela já tivesse ajudado, essa matmul ainda
  dominava o profile. Detecção de CPU (`hasF16CFMA`) via CPUID, com
  fallback escalar (tabela) em CPUs sem F16C/FMA e em arquiteturas não-amd64.
- `gguf.go`: o parser do header GGUF (metadados + lista de tensores) fazia
  uma syscall `pread` por CAMPO (nome, cada dimensão, tipo, offset — de
  cada tensor, mais cada entrada de metadado) via `os.File.ReadAt` direto;
  para um modelo real (~150+ tensores) isso sozinho era mais tempo que
  ler os dados de peso em si. Trocado por `bufio.Reader` — `LoadModel`
  ficou ~2,2x mais rápido (2,75s → 1,26s).

**Compilador (`cmd/advplc`, `pkg/preprocessor`, `pkg/lexer`) — ~1,9x mais
rápido por arquivo grande (163ms → 85ms num arquivo Protheus real de 3,6MB):**
- `processFile` (preprocessor) calculava `strings.ToUpper` da LINHA INTEIRA
  para TODA linha do arquivo, só para checar prefixos de diretiva
  (`#INCLUDE`, `#DEFINE`, ...); ~31% do tempo de compilação. Agora só
  computa quando a linha já começa com `#` ou `BeginSql`.
- `convertWithGoEncoding` (conversão CP-1252→UTF-8) escrevia byte a byte
  num `bytes.Buffer` (~23-48% do tempo, dependendo da correção anterior já
  aplicada ou não); agora copia em BLOCOS as sequências contíguas de bytes
  ASCII (a esmagadora maioria de qualquer fonte real) com um único `Write`
  por trecho, só indo byte a byte nos raros bytes >=128 (acentos em
  comentários), via tabela pré-computada.
- `tryBeginContent` (lexer) fazia um `strings.EqualFold` de 12 bytes a cada
  identificador do arquivo inteiro para checar `BeginContent`; agora um
  `if` de 1 byte descarta a esmagadora maioria antes do EqualFold.

**VM/interpretador (`pkg/runtime`) — ~28% mais rápido num loop aritmético
sintético (2,65s → 1,90s para 2M iterações):**
- `NewNumber` alocava um `*NumberValue` novo no heap a cada chamada — TODA
  operação aritmética do VM (soma, subtração, módulo, comparação) aloca um
  resultado novo, e isso dominava o profile de qualquer loop
  (`runtime.mallocgc`/`newobject` bem acima de qualquer opcode). Adicionado
  cache de inteiros pequenos (-256..4096, cobrindo contadores de loop,
  índices de array, resultados de módulo/comparação comuns) — `NewNumber`
  devolve o ponteiro compartilhado em vez de alocar, seguro porque
  `NumberValue` é imutável depois de criado e `Equals` compara por valor
  (mesmo padrão já usado pelo singleton `Nil`). `pkg/runtime` ganhou testes
  unitários pela primeira vez (`values_test.go`) para essa mudança.
  **Achado, não resolvido nesta rodada**: a raiz do custo é `Value` ser uma
  interface (todo número é um ponteiro boxed no heap, nunca um valor
  inline) — resolver isso de verdade exigiria trocar a representação de
  `Value` para um tagged union/valor inline, uma mudança arquitetural que
  toca `pkg/vm`, `pkg/runtime` e `pkg/compiler` inteiros; fora do escopo
  seguro de uma rodada de perf num código que acabou de chegar a 100% de
  corretude. Documentado aqui para uma futura sessão dedicada.

**Portabilidade multi-SO**: nenhuma mudança usa `unsafe`, cgo, ou qualquer
API específica de SO — `bufio`/`bytes.Buffer`/tabelas são 100% Go puro,
idênticas em linux/darwin/windows. O kernel `dotF16BlocksAVX2` é amd64-only
(gated por `hasF16CFMA`, detecção em runtime) com fallback escalar já
testado em todas as arquiteturas; darwin/arm64 (Apple Silicon, o alvo
`darwin-arm64` do release) e demais arquiteturas usam o caminho escalar +
tabela, que já é ~6x mais rápido que antes desta rodada (as duas primeiras
otimizações do LLM não dependem de amd64). Um kernel NEON para arm64 foi
considerado mas não implementado nesta rodada: esta máquina de
desenvolvimento não tem hardware nem emulação de usuário arm64 disponível
(só QEMU full-system, sem binfmt_misc registrado; instalar
qemu-user-static ou usar `docker run --privileged` para registrar
emulação foram bloqueados pelo classificador de segurança do Auto Mode
por serem mudanças de sistema não solicitadas explicitamente) — escrever
assembly SIMD sem conseguir RODAR os testes contra hardware real não é
uma otimização segura de se enviar. Ver "Próximos passos" abaixo.

**Próximos passos possíveis** (não feitos nesta rodada, por escopo/risco):
kernel NEON arm64 para `dotI2SBlocksAVX2`/`dotF16BlocksAVX2` (precisa de
CI rodando em runner macOS arm64 real para validar — o repo já tem
`macos-latest` no `release.yml`, mas só builda, não roda `go test`; um
workflow de CI dedicado a testes, multi-SO, seria o pré-requisito seguro);
representação de `Value` sem boxing no VM (projeto arquitetural à parte).

## [1.8.7] — 2026-07-11

### Sweep de pass-rate no corpus Protheus real (98,6% → 100%, 500/500)

- **Bug estrutural (parser)**: cláusulas soltas de DSL (`ADJUST`/`NO BORDER`/
  `OF`/...) depois de uma expression statement eram consumidas mesmo
  quando o próximo token estava em outra linha — o comentário do código
  já dizia "same line only", mas faltava o check de `.Line` de fato.
  `isAtClauseWord` reconhece palavras genéricas como `ITEM`, `SIZE`, `TO`,
  `OF`, `COLOR`, `FONT` — nomes comuns de variável/campo — então um
  statement solto (`item`) seguido na linha seguinte por
  `item := {}` (ITEM é cláusula de RADIO/COMBOBOX) comia o segundo
  `item` como cláusula e sobrava um `:=` "inesperado". Agora exige
  `p.tokens[p.pos-1].Line == p.peek().Line` para entrar/continuar no loop.
- **Bug estrutural (lexer + cmd/advplc)**: `cmd/advplc` converte fontes
  CP-1252 para UTF-8 antes de lexar; um NBSP (0xA0) colado por editores de
  texto vira a sequência UTF-8 de 2 bytes `0xC2 0xA0`, não um `0xA0`
  solto. A heurística do lexer "byte >= 0x80 é letra acentuada CP-1252"
  engolia o `0xC2` para dentro do identificador seguinte
  (`MSGET\xc2\xa0cBanco` virava um único identificador corrompido),
  desalinhando a tokenização e quebrando o parse 1-2 statements depois —
  o clássico "drift bug" da sessão. Faixa de tokens idêntica entre a
  versão corrompida e a versão limpa em ferramentas de depuração que leem
  bytes crus (sem a conversão CP1252→UTF8) mascarou o diagnóstico por
  várias iterações. Corrigido reconhecendo a sequência de 2 bytes como
  espaço em branco tanto em `skipWhitespace` quanto no scanner de
  identificadores (`tokenizeIdentifier` para de consumir ao encontrá-la).
- Pass-rate final: **500/500 (100%)** no corpus de amostra (811R4 +
  12.1.2510).

## [1.8.6] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (96,8% → 98,6%)

- `WEB EXTENDED INIT <var> START <expr>` / `WEB EXTENDED END` (Portais).
- `TTALK "v1"` como cláusula de WSMETHOD (declaração e dispatch);
  `PRODUCES/CONSUMES` aceita constante que colide com keyword;
  `QUERYPARAM a, b` (lista) na implementação.
- `Replace <campo> With <expr> [, <campo> With <expr>...]` (Clipper).
- Flags/cláusulas de `@`: `CENTERED`, `RAISED`, `PROMPTS` (FOLDER),
  `FILENAME/FILE/DISK` (BITMAP).
- `COLORS` como sinônimo de COLOR no DEFINE.
- `Store Header/Cols ... TO ...` tolerado nativamente quando o .ch com o
  #command (PLSMGER.CH) não é resolvível.

## [1.8.5] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (95,8% → 96,8%)

- **Bug estrutural (preprocessor)**: diretivas `#IFDEF`/`#ENDIF` DENTRO de
  comentário de bloco (`/*#IFDEF TOP ... #ENDIF*/`, idioma real de
  desativar código) eram consumidas como diretivas de verdade, DELETANDO o
  `*/` da saída e invertendo a fase de comentários do resto do arquivo —
  fonte do drift bug mais antigo da sessão (WSAPD010). O preprocessor
  agora rastreia estado de comentário de bloco e passa essas linhas
  intactas.
- `Store <expr> To <var,...>` (atribuição múltipla Clipper).
- `DEFINE TIMER ... INTERVAL <n> ACTION <expr> OF <janela>`.
- Nome de parâmetro de função pode colidir com keyword
  (`Static Function GetSU5(oApi, Self)`).
- Vírgula pendurada antes de cláusula em lista de valores
  (`HEADER "a","b",;` + `SIZE w,h` na linha seguinte).

## [1.8.4] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (94,4% → 95,8%)

- **Bug estrutural**: `Static cVar := valor` DENTRO de uma função era
  tratado como fronteira de função (por causa de `Static Function`),
  truncando o corpo silenciosamente e corrompendo o parse do resto do
  arquivo (fonte dos piores "drift bugs"). STATIC agora só é boundary
  seguido de FUNCTION.
- DSL XML legado: `ADDNODE <expr> NODE <expr> ON <expr>`, `DELETENODE
  <expr> ON <expr>`, `CREATE <var> XMLFILE <expr> [SETASARRAY <lista>]`.
- Alvo de `Count/Sum/Average ... To` pode ser expressão (`self:nTotReg`).
- Keyword como identificador comum em posição de operando quando o token
  seguinte só continua expressão (`{|Panel| f(Panel, ...)}`).
- `Private &("nome"+var) := x` — memvar com nome computado por macro.
- `HEADERS` (plural) como cláusula de LISTBOX; WSMETHOD REST sem nome
  próprio (`WSMETHOD GET WSRECEIVE ... WSSERVICE X`).

## [1.8.3] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (92,6% → 94,4%)

- `@ nLin++` sozinho (forma degenerada real) tolerado como expressão.
- `End If` / `End Do` fechando If e Do Case (variantes de duas palavras).
- QUALQUER keyword seguida de `(` na mesma linha em contexto de expressão é
  chamada de função (`alias->(Add())`, `Select()`, ...) — generalização da
  lista IF/ARRAY/DATE/OBJECT/BREAK.
- Nome de função pode colidir com keyword (`Static Function Add`).
- `SEND MAIL FROM ... TO ... SUBJECT ... BODY ... [ATTACHMENT] [RESULT]` e
  `GET MAIL ERROR <var>` (DSL de e-mail do workflow).
- `MENU <var> POPUP ... MENUITEM ... ACTION ... ENDMENU` (menu de contexto).
- `DEFINE DBTREE ... CARGO ; ON CHANGE <expr>`; `DBADDTREE ... PROMPT ...
  RESOURCE ... CARGO ... OPENED` (árvores legadas).
- Cláusulas de `@`: `FROM` (METER FROM 0 TO 100), `PICT` (abrev. de
  PICTURE), `OPTION` (FOLDER), flag `RIGHT`; `FIELDS` pode levar lista de
  valores própria (`LISTBOX ... FIELDS "" ; HEADER ...`).

## [1.8.2] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (89,4% → 92,6%)

- `&&` (comentário Clipper) após `;` de continuação tratado como fim de
  linha para a continuação (lexer), igual ao `//` já suportado.
- `Begin Report Query <expr>` — a seção pode ser expressão completa
  (`oReport:Section(2)`), não só um nome, nos dois lados do bloco.
- Atribuição encadeada como valor dentro de item de codeblock
  (`x[9] := x[10] := ... := 0`).
- `COLORS` como cláusula de `@` (FOLDER ... COLORS 0,167...).
- `Release Object <nome>` / `Release All [Like <máscara>]`.
- `Break(oErro)` como chamada de função em expressão (idioma de
  ErrorBlock), distinto do statement BREAK.
- `@ y,x To y2,x2 MultiLine Object oMulti` — flags/cláusulas do TMultiget
  legado no ramo de caixa do `@`.
- `Data <keyword>` — nome de membro de classe pode colidir com palavra
  reservada (`Data size`, `Data default`); `::Default()` idem em acesso.
- `DEFINE SCROLLBAR ::oVScroll VERTICAL OF Self RANGE a,b` — alvo `::prop`
  em DEFINE e ACTIVATE, flags VERTICAL/HORIZONTAL e cláusula RANGE.
- Bytes de controle soltos (\x01, corrupção comum em fontes legados)
  tolerados pelo lexer, como o backtick.
- `Count To <var>` / `Sum <exprs> To <vars>` / `Average ... To ...`
  (comandos Clipper de agregação; parseados e descartados).
- Atribuição em resultado de chamada (`ATail(arr) := v`, semântica de
  referência do Clipper) tolerada no codegen (avalia e descarta), mesma
  tolerância já dada a `&macro := v`.

## [1.8.1] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (87,6% → 89,4%)

- Resolução de #include: tenta subpastas convencionais (`ch/`, `include/`,
  `includes/`) e fallback case-insensitive por diretório (fontes CP-1252
  vindos de Windows quase nunca batem o case do disco em Linux).
- Junção de linha lógica antes do casamento de #command (`Store COLS ... ;`
  + `While ...`), preservando contagem de linhas; comentário `//` após o
  `;` de continuação não esconde mais a marca (definições e uso); CRLF
  (`;\r`) tratado em todos os joins.
- Operador de exponenciação `^` e sinônimo `**` (parsePower, associativo à
  direita, acima da multiplicação; novo OP_POW na VM via math.Pow).
- `ACTIVATE FWMBROWSE/MBROWSE/REPORT <var>`.
- DSL mobile FDA: `ADD FOLDER ... CAPTION ... [ON ACTIVATE f()] OF ...`,
  `ADD COLUMN ... TO ... ARRAY ELEMENT n HEADER ... WIDTH n`,
  `SET BROWSE <var> ARRAY <expr>`, cláusulas `CAPTION`/`SYMBOL`/`ITEM`/
  `VSCROLL` e flags de duas palavras `NO SCROLL`/`NO UNDERLINE` em `@`.

## [1.8.0] — 2026-07-10

### Motor de #command reescrito para semântica Clipper real + 2 features de lexer (86,4% → 87,6%)

Rodada dirigida pelos arquivos que exigiam o pré-processador de verdade:

**Motor #command/#xcommand (pkg/preprocessor):**
- Junção de definição multi-linha agora REMOVE o `;` de continuação de fim
  de linha (antes ficava dentro do padrão como literal impossível de casar,
  desativando silenciosamente toda regra multi-linha); um `;` no meio/começo
  de linha do resultado é conteúdo (separador de comandos gerados).
- Regra definida por ÚLTIMO vence (ordem reversa de tentativa), semântica
  Clipper que permite um .ch especializar comando já definido.
- Cláusulas opcionais consecutivas casam em QUALQUER ORDEM (ex.: padrão
  declara `[OF <oWnd>] [PIXEL]`, fonte usa `PIXEL OF oPanel`).
- Marcador restrito multi-literal `<nome: LIT1, LIT2, ...>` (captura qual
  literal casou; `<.nome.>` vira .T./.F.).
- Marcador guloso para em QUALQUER literal alcançável à frente (união dos
  abridores de todos os grupos opcionais + próximo literal obrigatório),
  não só no primeiro.
- Marcador dentro de grupo opcional herda o ponto de parada do padrão
  externo (lista, não um único literal).
- Grupos opcionais `[...]` do lado do RESULTADO: emitidos sem os colchetes
  só se algum marcador interno capturou algo (antes viravam `[, ]` literal).
- Marcador de resultado `<"var">` (stringify: nome capturado entre aspas).
- Expansão re-processada recursivamente (o resultado pode conter novos
  comandos, ex.: VTSAY+VTGET expande para dois comandos `@...` que expandem
  de novo), segmentada por `;` de topo.
- Comentário `//` de fim de linha removido antes do casamento (um marcador
  guloso o capturava para dentro da expansão, comentando o resto do código
  gerado).
- Flag `[<nome: LITERAL>]` com espaços ao redor de `:`/`,` normalizada.

**Lexer:**
- Literal de string entre colchetes do Clipper: `[texto]`/`[]` em posição
  de operando é string (heurística clássica: `[` após token que encerra
  operando é indexação; caso contrário, literal até o `]` da mesma linha).
- `BeginContent var <nome> ...conteúdo cru... EndContent` (bloco TLPP de
  JSON/XML embutido) — corpo não é AdvPL e não deve ser tokenizado;
  consumido no lexer e emitido como `<nome> := "<corpo>"`.

**Parser:**
- Parâmetro de codeblock pode colidir com palavra reservada
  (`{|Self| ...}`) — aceita keyword como nome.

## [1.7.11] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (86,0% → 86,4%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `WSMETHOD GET <nome> PATHPARAM <param> WSRECEIVE ...` — cláusulas REST
  `PATHPARAM`/`QUERYPARAM` (binding de parâmetro de rota) não reconhecidas
  na implementação do WSMETHOD.
- `Default Self:Prop := valor` — alvo de `Default` explicitamente escrito
  como `Self:Prop` em vez do atalho `::Prop` já suportado.

## [1.7.10] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real

- `@ y,x BMPBUTTON TYPE n ACTION expr` — cláusula `TYPE` (número de estilo
  do botão bitmap) não reconhecida no laço de cláusulas de `@`.

## [1.7.9] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (85,2% → 86,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).

- **Bug estrutural (lexer)**: literal numérico com ponto decimal sem
  dígito antes (`.5`, `.7` — comum em coordenadas `@ .5,.7 ...`) não era
  reconhecido; o lexer só entrava no tokenizador de número ao ver um
  dígito primeiro, então um `.` inicial caía sempre no caminho de
  dot-literal/operador (`.T.`, `.AND.`, `.`), nunca no de número. Corrigido:
  um `.` seguido de dígito agora entra no tokenizador de número.

## [1.7.8] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (84,4% → 85,2%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `DEFINE CELL ... BLOCK{||...} ...` — cláusula de bloco de valor da
  coluna do TReport não reconhecida.
- `@ x1,y1 TO x2,y2 DIALOG <var> TITLE "..." [PIXEL]` — sintaxe legada de
  criação de diálogo via `@ ... TO ...` (equivalente a `DEFINE MSDIALOG
  <var> FROM x1,y1 TO x2,y2 TITLE ...`), confundida com o desenho de caixa
  (`@ ... TO ... BOX`); nenhum dos dois é o outro, precisavam de ramos
  separados.

## [1.7.7] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (83,6% → 84,4%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `Private M->NOME_CAMPO := valor` — idioma Clipper de qualificar
  explicitamente uma memvar com o alias "M" (memory), redundante mas usado
  em fontes reais para dar nome de memvar igual a um campo; `Local`/
  `Private`/`Public`/`Static` só aceitavam um nome simples, não o padrão
  `M->nome`.
- `@ ... MSGET ... HASBUTTON F3 "..." ...`, `@ ... WORKTIME ... RESOLUTION
  <expr> VALUE <expr> ...` — cláusulas de `@` não reconhecidas
  (`HASBUTTON` do MSGET com botão de F3; `RESOLUTION`/`VALUE` do controle
  WORKTIME).

## [1.7.6] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (83,0% → 83,6%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).

- `alias->END` (e qualquer outro campo cujo nome colide com palavra
  reservada, ex. `alias->DELETE`) — nome de campo após `->` exigia
  `TOKEN_IDENT`; agora usa `expectName()` (aceita `TOKEN_KEYWORD`
  também), mesma classe de bug já corrigida em outros pontos do parser
  para identificadores que colidem com keywords.

## [1.7.5] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (82,0% → 83,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- Elemento de array literal `{a, b := c, d}` (sem `||`, usado como
  sequência de expressões em cláusulas VALID/ACTION reais) não aceitava
  atribuição (`:=`) como elemento — usava `parseExpression` puro em vez de
  `parseCodeBlockItem`, mesma classe de bug já corrigida em outros pontos
  do parser para `:=` inline.
- `Return target := value` (atribuição usada inline como valor de retorno,
  ex. `Return self:oProp := {...}`) deixava o `:=` pendurado — corrigido
  nos DOIS parsers de RETURN existentes (`expressions.go` e o parser de
  corpo de método em `parser.go`, que tem sua própria cópia da lógica de
  RETURN) para usar `parseAssignRHS` em vez de `parseExpression`.

## [1.7.4] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (81,0% → 82,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Seis bugs reais adicionais de parser corrigidos:

- `@ ... RADIO/CHECKBOX ... 3D SIZE w,h ...` — flag de layout "3D"
  (tokeniza como NUMBER "3" + IDENT "D") não reconhecida no laço de
  cláusulas de `@` (só existia o caso equivalente em `DEFINE`).
- `LOCATE FOR <expr> [WHILE <expr>]` — comando Clipper de busca sequencial
  no alias atual, não suportado (nenhum dispatch existia).
- `Copy File <expr> To <expr>` — cópia de arquivo em disco (comando
  Clipper), distinto de `Copy To` (exportação de registros); confundia-se
  com este e quebrava o parsing.
- `Copy <alias-expr> To Memory <name> [Blank]` — copia a estrutura de
  campos de um alias para um array; forma de `COPY` não reconhecida.
- `@ ... GET ... MULTILINE ... HSCROLL ...` — cláusulas do GET multilinha
  (TGet memo) não reconhecidas.
- `DEFINE SBUTTON ... ONSTOP <expr> ...` — cláusula de tooltip do botão
  não reconhecida.

## [1.7.3] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (76,6% → 81,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs estruturais de alto impacto e mais quatro bugs pontuais de parser
corrigidos:

- **Bug estrutural**: strings `"..."` sem aspa de fechamento até o fim da
  linha (typo comum em fontes reais, ex. query SQL multi-linha via `+=`)
  travavam o lexer, que consumia até a próxima aspa em QUALQUER linha
  seguinte, engolindo o resto do arquivo. Clipper/AdvPL fecha strings
  implicitamente no fim da linha; o lexer agora faz o mesmo.
- **Bug estrutural**: um identificador seguido de `(` seguinte, sem guarda
  de mesma linha, era sempre tratado como chamada de função — já que
  newlines são removidas antes do parsing, um `(alias)->campo` no início da
  PRÓXIMA linha grudava como argumento da chamada do identificador da
  linha anterior (`var := f() \n (alias)->campo := x` virava
  `f()(alias)->campo`). Corrigido em dois pontos: `parsePrimary` (chamada
  direta `ident(`) e `parsePostfix` (chamada após expressão composta),
  ambos agora exigem que o `(` esteja na mesma linha do token anterior.
- `alias->(expr1, expr2, ...)` — sequência separada por vírgula dentro do
  escopo de alias só aceitava uma única expressão; agora usa a mesma
  produção de `(a, b, c)` (avalia todas, retorna a última).
- `DEFINE SECTION ... TABLES "A","B",...`, `DEFINE CELL ... PICTURE "..."`,
  `DEFINE BREAK ... WHEN {||...}`, `DEFINE FUNCTION ... FUNCTION SUM BREAK
  oBreak TITLE "..." NO END SECTION` — cláusulas do DSL de TReport
  (`DEFINE SECTION/CELL/BREAK/FUNCTION`) não reconhecidas: `TABLES`,
  `PICTURE`, `WHEN`, `FUNCTION` (como nome de cláusula, colide com o nome
  do próprio DEFINE kind), `BREAK`, e o flag de três palavras `NO END
  SECTION`.

## [1.7.2] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (73,8% → 76,6%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Seis bugs reais adicionais de parser corrigidos:

- `If x := cond` / `While x := cond` — atribuição usada inline como
  condição (idioma comum em AdvPL: "avança e testa") deixava o `:=`
  pendurado; a condição agora usa `parseAssignRHS` como o resto do parser.
- Caminho de namespace TLPP totalmente qualificado
  (`totvs.framework.treports.date.stringToTimeStamp(...)`) quebrava
  sempre que um segmento colidia com palavra reservada (`date`, que lexa
  como `TOKEN_KEYWORD`); o loop de segmentos só aceitava `TOKEN_IDENT`.
  Mesmo problema corrigido em `NAMESPACE`/`USING NAMESPACE`, agora via
  `parseNamespacePath` compartilhado (segmento só aceito logo após um
  ponto, nunca solto — não avança para além da declaração).
- `WSRESTFUL/WSSERVICE <nome> ... FORMAT <expr>` — cláusula de cabeçalho
  não reconhecida, quebrando o corpo inteiro do bloco.
- **Bug de colisão de nome em `WSDATA`**: o bypass "nome de método é
  opcional" (`WSMETHOD POST DESCRIPTION "..." ...`) se aplicava também a
  `WSDATA`, então um campo literalmente chamado `Description`
  (`WSDATA Description As String`) era confundido com a cláusula
  `DESCRIPTION` e o parser pulava o nome do campo — struct inteira
  corrompida a partir daí. `WSDATA` agora sempre exige nome explícito.
- `ParamType <n> Var <nome> As <tipo> [Default <expr>]` — declaração de
  metadados de parâmetro não suportada.

## [1.7.1] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (70,8% → 73,8%)

Continuação do sweep dirigido por corpus contra os fontes reais 811R4 e
12.1.2510 (amostra de 500 arquivos, ver [[advpp_corpus_locations]]).
Onze bugs reais de parser corrigidos:

- `SET KEY <nKey> TO [<uBlock>]` — o keycode antes do `TO` não era
  reconhecido pelo dispatcher de `SET`.
- `DEFINE CELL ... AUTO SIZE` (TReport) — flag `AUTO` sem valor antes de
  `SIZE` não era reconhecida, quebrando o parsing da cláusula seguinte.
- Drift em `SET FILTER TO` — a heurística "tem valor?" não detectava que
  um `x += ...` na linha seguinte não era o valor do `SET`, engolindo a
  variável errada.
- `DELETE FILE <expr>` e `DELETE [FOR/WHILE/RECORD/REST/ALL]` — comandos
  Clipper de arquivo/registro não eram suportados.
- `WSMETHOD ... WSRECEIVE a,b WSSEND c` — só aceitava `WSSEND` antes de
  `WSRECEIVE` e com valor único; real Protheus usa qualquer ordem e listas
  separadas por vírgula em ambas.
- `PREPARE ENVIRONMENT EMPRESA/FILIAL/MODULO/TABLES` — comando batch de
  abertura de ambiente não suportado.
- `SET DELETE ON` — "DELETE" é palavra reservada (`TOKEN_KEYWORD`), não
  identificador; o dispatcher de `SET` exigia `TOKEN_IDENT` e falhava.
- **Bug estrutural**: como quebras de linha são descartadas antes do
  parsing, um `++x` prefixo iniciando uma nova instrução colava-se ao
  fim da expressão da instrução anterior (`y := f()` seguido de `++x`
  virava `(f())++`, erro de compilação "unsupported assignment target").
  Corrigido exigindo que o operador pós-fixo `++`/`--` esteja na mesma
  linha do token anterior.
- Atribuição encadeada (`a:=b:=c:=valor`) dentro de `Local`/`Private` não
  era suportada (só funcionava em atribuição solta).
- `For ... EndFor` (além de `Next`/`End`) não fechava o loop.
- `Default a:=1, b:=2, c:=3` (múltiplas variáveis separadas por vírgula)
  só suportava uma única variável.


### Motor real de `#xcommand`/`#command`/`#xtranslate`/`#translate`

Até aqui, o pré-processador **reconhecia** a sintaxe destas diretivas mas
**descartava** as definições — nenhuma expansão de verdade acontecia. Isso
quebrava qualquer arquivo que dependesse de comandos customizados definidos
em headers `.ch` reais (padrão comum em código Protheus legado: `STORE
HEADER <cA> TO <aH> [FOR <for>]`, `COPY <cAC> TO MEMORY [<bl:BLANK>]`,
etc.). Agora o AdvPP implementa o pattern-matching de verdade, no estilo
Clipper:

- **Padrão de casamento**: palavras literais (case-insensitive), `<nome>`
  (captura uma cláusula até o próximo literal esperado), `<nome,...>`
  (captura uma lista), `[...]` (grupo opcional — só tenta se o primeiro
  literal dele aparecer na posição atual), `[<nome:LITERAL>]` (marcador de
  flag booleana).
- **Molde de resultado**: `<nome>` (substitui pelo texto capturado, ou
  vazio se ausente), `<{nome}>` (vira `{|| texto}` se presente, `NIL` se
  ausente — usado para condições `FOR`/`WHILE` que viram codeblock),
  `<.nome.>` (`.T.`/`.F.` conforme presença), `\[`/`\]` (colchete literal).
- Definições multi-linha via continuação com `;` (convenção usual do
  Clipper) são unidas antes de compilar a regra.

Três bugs reais adicionais encontrados e corrigidos no caminho (achados ao
validar contra headers `.ch` reais de um fork ApSoft/Protheus):

1. **`#define` com múltiplos espaços** (`#define  NOME    valor`) —
   `parseDefine` usava `strings.SplitN(line, " ", 3)`, que quebra quando
   há mais de um espaço entre `#define` e o nome (comum em código real),
   armazenando a macro com nome vazio.
2. **`#define` multi-linha** (`#define NOME { "a","b",;\n "c","d" }`) —
   sem juntar as linhas de continuação, o resto do array vazava como
   código bruto (token solto no meio de uma statement).
3. **Tokenização por espaço simples** grudava identificador com pontuação
   colada (`TCSQLEXEC("select 1")` virava um token só), fazendo até um
   `#translate` sem parâmetros nunca casar; e quando um padrão casava só
   o início da linha, o resto era descartado em vez de reanexado.

Validado com testes automatizados (`pkg/preprocessor/commands_test.go`) e
contra arquivos `.prw`/`.ch` reais de um corpus de ~30 mil fontes Protheus
(legado 811R4 + versão 12.1.2510 atual) cedido pelo usuário para esta
investigação — usado só localmente para validação, não redistribuído.
Sem regressões: `make test` continua 30/30, `go vet` e os demais pacotes
seguem limpos, cross-compile OK em linux/windows/darwin (amd64+arm64).

## [1.6.0] — 2026-07-09

### `tests/real_protheus_test.prw` totalmente resolvido

O dump de 3785 linhas de código Protheus real usado como fixture de
estresse — que tinha uma falha de parser documentada como conhecida
desde antes desta série de correções — agora **compila e interpreta
sem nenhum erro** (`advplc check` e `advplc run`, ambos saem limpo).
Oito bugs reais e distintos encontrados e corrigidos por bisecção
binária (truncar o fonte progressivamente até isolar a menor entrada
que ainda reproduz o erro), além dos cinco já corrigidos na versão
anterior:

- `++nome` — incremento **prefixo** (só o pós-fixado `nome++` estava
  implementado).
- `@ ... LISTBOX ... FIELDS HEADER a,b,c ... ON DBLCLICK expr
  NOSCROLL OF window PIXEL` — cláusulas do LISTBOX (`FIELDS`,
  `HEADER`, `ON <evento> <expr>`, `NOSCROLL`) não reconhecidas.
- `@ y,x BUTTON var PROMPT "texto" ...` — cláusula `PROMPT` do BUTTON
  não reconhecida.
- `IF ( aArray[ i , j ] )` — o lookahead que desambigua bloco `If`
  de `IF(cond,then,else)` (adicionado na correção anterior) contava
  a vírgula de um índice multi-dimensional `[i,j]` como se fosse a
  vírgula de topo do `IF(...)`, tratando incorretamente todo `If`
  cuja condição usa um array 2D como a forma de chamada.
- `f(aArray[i] := valor, ...)` — atribuição como argumento de função
  quando o alvo não é um identificador simples (só `ident := valor`
  virava atribuição; `array[i] := valor` ficava com o `:=` sobrando).
- `@ y,x RADIO var VAR nVar ITEMS v1,v2,...` — cláusula `ITEMS` do
  RADIO não reconhecida.
- `Do Case ... End Case` — só `EndCase` (uma palavra) era aceito como
  fechamento; `End Case` (duas palavras, forma clássica do Clipper)
  não.
- `FindFunction("Nome")` — nativa ausente (usada no Protheus real para
  checar a existência de funções opcionais/AddOn antes de chamá-las).
  Implementada: verifica natives registradas e funções do bytecode
  (com/sem prefixo `U_`).

Sem regressões: `make test` agora dá **30/30** fixtures (antes eram
29/30, com esta sendo a única falha conhecida); `go vet ./...` e os
testes de `pkg/llm`/`pkg/mcp`/`cmd/advplc` continuam limpos;
cross-compile OK em linux/windows/darwin (amd64+arm64).

## [1.5.0] — 2026-07-09

### Servidor MCP nativo (classe `MCPServer`)

O AdvPP agora fala **MCP (Model Context Protocol)** de verdade — ao
contrário do suporte a REST (`WSRESTFUL`/`@Get`/`@Post`), que hoje é só
sintaxe reconhecida e descartada (sem servidor HTTP nem despacho real), a
classe `MCPServer` sobe um servidor **funcional**: JSON-RPC 2.0 sobre
stdio, expondo funções AdvPL/TLPP como "tools" que qualquer cliente MCP
(Claude, outros agentes) pode listar e chamar.

- **`pkg/mcp`**: núcleo do protocolo em Go puro (sem CGO, sem
  dependências externas) — `initialize`, `notifications/initialized`,
  `tools/list`, `tools/call`, `ping`; transporte stdio com uma mensagem
  JSON por linha.
- **Classe `MCPServer`** (`pkg/vm/mcp_native.go`):
  ```advpl
  oMCP := MCPServer():New("meu-servidor", "1.0.0")
  oMCP:AddTool("soma", "Soma dois números", ;
      '{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}', ;
      "ToolSoma")
  oMCP:Serve() // bloqueia lendo/escrevendo em stdin/stdout

  User Function ToolSoma(oArgs)
  Return cValToChar(oArgs:A + oArgs:B)
  ```
  Cada chamada de tool roda a função registrada numa VM isolada (mesmo
  mecanismo do `StartJob`) — necessário porque `Serve()` já está no meio
  da execução da VM principal quando uma `tools/call` chega; chamar a
  função direto na mesma VM corromperia a pilha de chamadas em andamento
  (bug real encontrado e corrigido durante o desenvolvimento).
  `Serve()` redireciona `ConOut`/console para stderr automaticamente,
  para não misturar saída de depuração com as mensagens JSON-RPC no
  stdout.
- Funciona com **`advplc run`** normal — não precisa de um comando novo.

**Validado com o SDK oficial em Python do MCP** (não só testes internos):
handshake `initialize`, `list_tools`, `call_tool` — ver
`cmd/advplc/mcp_integration_test.go`.

### Correções no parser (encontradas caçando um bug pré-existente)

Investigando uma falha antiga documentada em
`tests/real_protheus_test.prw` (um dump de 3785 linhas de código
Protheus real usado como fixture de estresse) via bisecção binária
(truncar o fonte progressivamente até isolar a menor entrada que ainda
reproduz o erro), foram encontrados e corrigidos cinco bugs reais e
distintos de parsing:

1. `&nome.` — o ponto final (terminador explícito clássico do
   Clipper/AdvPL para a substituição de macro) não era consumido.
2. `&nome.()` / `&(expr)()` — chamada de função cujo nome vem de uma
   macro; os parênteses da chamada não tinham dono no parser (mesma
   simplificação já usada para `alias->&macro`: sintaxe consumida, sem
   modelar a invocação dinâmica — o VM não resolve função por nome em
   runtime).
3. `@ y,x GROUP var TO y2,x2 OF window LABEL "..." PIXEL` — a cláusula
   GROUP do comando `@` de diálogo (caixa de agrupamento) usa `TO` e
   `LABEL` como cláusulas, não reconhecidas antes.
4. `ACTIVATE DIALOG oDlg ON INIT ... CENTERED` — variante clássica (sem
   o prefixo "MS") do já suportado `ACTIVATE MSDIALOG`.
5. `IF(cond, then, else)` usado como **statement isolado** (resultado
   descartado) — sempre caía no parser de bloco `If/EndIf`, que não
   trata `(...)` com vírgulas como chamada. Novo lookahead
   (`isInlineIfCall`) desambigua da forma bloco `If (cond) ... EndIf`.

`tests/real_protheus_test.prw` avança de ~503 para ~2414 das 3785
linhas antes de esbarrar no próximo gap (não mais um bug de parsing,
uma feature genuinamente não implementada) — mantido como falha
conhecida documentada no Makefile.

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
