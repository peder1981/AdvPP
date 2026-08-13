# Limitações arquiteturais conhecidas (VM AdvPP)

Descobertas durante a implementação da série de cobertura íntegra do TDN
AdvPL (`docs/superpowers/plans/2026-08-09-advpp-tdn-integral-plan.md`).
Funções cujo comportamento documentado no TDN depende de uma dessas
limitações devem implementar o que for possível e documentar o gap
explicitamente no código — não inventar workarounds ad-hoc por função.

## Parâmetros por referência (`@var`) em natives

Native functions recebem `[]advplrt.Value` (cópias) e retornam um único
`advplrt.Value`. Não existe, em nenhum lugar do VM/compilador, um
mecanismo para uma native mutar diretamente uma variável do chamador
passada com `@`. Descoberto na Task 5 (`IPCWaitEx`, que segundo o TDN
recupera dados via parâmetros por referência).

Funções afetadas até agora: `IPCWaitEx` (pkg/vm/execucaoprocessos_native.go);
`GetGlbVars` (pkg/vm/varglobais_native.go) — os parâmetros `@xValue1...N` não são
populados com os valores armazenados; o valor de retorno (.T./.F.) permanece correto
e é a forma suportada de checar sucesso/falha; `SFTPDirLs`, `SFTPDwld1`, `SFTPDwld2`,
`SFTPUpld1`, `SFTPUpld2` (pkg/vm/sftp_native.go) — o parâmetro final `@sError`/`@cError`
de todas as cinco não é populado; o valor de retorno principal (array/código de status)
permanece correto e é a forma suportada de checar sucesso/falha; `GetFuncArray`
(pkg/vm/rpo_native.go, Task 17) — os parâmetros opcionais `@aTipo`/`@aArquivo`/
`@aLinha`/`@aData`/`@aHora` não são populados; o retorno principal `aScr` (array
de nomes de função que casam com a máscara, derivado de `v.bc.Functions` +
`v.natives`) permanece correto e é a forma suportada de usar a função.

`HMGet`/`HMGetN` (pkg/vm/matrizhashmap_native.go, Task 19) — o parâmetro `@aVal`
não é populado quando o chamador segue o exemplo literal da TDN (variável escalar,
ex.: `Local oVal := nil; HMGet(oHash,chave,oVal)`); o retorno `lRet` (achou/não achou)
permanece correto e é a forma suportada de checar o resultado. Exceção parcial: como
arrays já são tipo referência neste VM (sem precisar de `@`, igual ao AdvPL real), se o
chamador passar um `*advplrt.ArrayValue` como terceiro argumento (a própria tabela de
parâmetros da TDN documenta `aVal` como tipo "vetor"), a mutação in-place funciona e
é aplicada. `HMList` (mesmo arquivo) não sofre a limitação: seu `@aElem` é
inequivocamente array na TDN, então a lista é sempre populada de volta.

`VarGet`, `VarGetX`, `VarGetXD`, `VarGetD` (pkg/vm/varglobaishashmap_native.go,
Task 28) — o parâmetro escalar `@xValor` (N/C/D/L) não é populado de volta na
variável do chamador; o valor de retorno lógico (.T./.F.) permanece correto e é a
forma suportada de checar sucesso/falha, e o conteúdo armazenado na "Tabela X"
continua acessível via `VarGetXA` (listagem que inclui os valores). Exceção
parcial: os parâmetros de array `@aValor` (em `VarGetA`/`VarGetAD`/`VarGetD`/
`VarGet`) e `@aListCV`/`@aListCV_X`/`@aListCV_A` (em `VarGet_A`/`VarGetXA`/
`VarGetAA`) são populados in-place, pois arrays são tipo referência neste VM
(mesmo mecanismo de `HMGet`/`HMList`, Task 19).

`XmlC14N`, `XmlC14NFile`, `XmlFVldSch`, `XmlParser`, `XmlParserFile`
(pkg/vm/xml_native.go, Task 23) — todas têm, na TDN, os últimos 1-2
parâmetros como saída por referência (`@cError`/`@cWarning`), nunca
populados; o valor de retorno principal de cada uma (string canonicalizada,
lógico de validação, ou objeto/NIL) permanece correto e é a forma suportada
de checar sucesso/falha.

`SFTPDirLs`/`SFTPDwld1`/`SFTPDwld2`/`SFTPUpld1`/`SFTPUpld2` também documentam, no
código (`pkg/vm/sftp_native.go`), um trade-off de segurança deliberado: a
verificação de host key usa `ssh.InsecureIgnoreHostKey()` por padrão (a TDN não
oferece parâmetro de known_hosts/fingerprint nestas 5 funções), com um escape
hatch opcional via `ADVPP_SFTP_KNOWN_HOSTS` para quem quiser verificação estrita.

Se uma task futura encontrar uma função cujo comportamento central depende
de mutar `@var`, documente o gap da mesma forma (comentário explicando
o que funciona e o que não funciona) em vez de tentar implementar às
cegas — e adicione uma linha aqui.

## Autenticação contra Active Directory (`ADUserValid`)

A função `ADUserValid(cDomainName, cUserName, cPassword)` do TDN realiza
autenticação contra o Active Directory do Windows via APIs nativas
(advapi32.dll — `LogonUserA`, netapi32.dll — `ConvertStringSidToSidA`).

AdvPP, sendo um compilador Go headless, não possui:
1. Acesso a APIs nativas Windows (advapi32/netapi32)
2. Conectividade LDAP/AD em tempo de compilação/teste
3. Contexto de usuário/domínio (é um processo standalone sem sessão Windows)

**Comportamento em AdvPP:** `ADUserValid()` sempre retorna `.F.` (autenticação
falha). Esta é uma escolha conservadora: um código que verifica "if
ADUserValid(...)" recebe "falso" (seguro, nega acesso por padrão) em vez
de um falso-positivo "verdadeiro" (perigoso).

**Função:** A validação de argumentos ocorre (não aceita todos vazios), mas
nenhuma autenticação real é realizada. Em produção no Protheus (Windows),
a função autentica contra AD. Em AdvPP (qualquer OS), sempre falha.

**Afetado:** `ADUserValid` (pkg/vm/controleacesso_native.go, Task 11).

**Alternativa:** Se integração real com AD/LDAP fosse necessária, seria
preciso: (1) adicionar dependência Go (ex: `github.com/go-ldap/ldap`),
(2) conectar a um domínio/controlador real (exigindo infra), (3) aceitar
que testes automatizados não podem validar a autenticação real de forma
portável. Portanto, a implementação atual (fail-closed, retorna .F.) é
apropriada para um compilador standalone.

## Ausência de índice físico de RPO (`ChkRpoChg`, `GetApoRes`, `GetDependency`, `GetRpoLog`, `GetSrcArray`)

O Protheus real compila fontes AdvPL para um RPO binário próprio (formato
proprietário TOTVS, "Repositório de Programas Objeto"), que retém em disco
um índice físico com: fonte de origem e linha de cada função, patches
(.upd/.pak/.ptm) aplicados em sequência com data/build de cada um, e um
`SourcePath` configurável em `totvsappserver.ini` que pode ser trocado em
tempo de execução do TOTVS Application Server.

AdvPP compila para o seu próprio bytecode Go (`pkg/compiler.Bytecode`), que
não é um arquivo/container persistente análogo ao RPO: `FunctionInfo`
(`pkg/compiler/opcodes.go`) não guarda o nome do arquivo-fonte de origem
nem a lista de chamadas de primeiro nível de cada função, não existe
estrutura de "patch aplicado" em lugar nenhum do compilador/VM, e o
bytecode carregado no início do processo é o único que existe durante toda
a vida do processo (não há recarga a partir de um `SourcePath`
configurável).

**Comportamento em AdvPP** (Task 17, `pkg/vm/rpo_native.go`):
- `ChkRpoChg()` sempre retorna `.T.` — como não existe mecanismo de recarga
  de config/SourcePath em tempo de execução, "nenhuma mudança detectada" é
  a resposta real e honesta, não uma simulação.
- `GetApoRes(cRes)` valida o argumento (`cRes` não pode ser vazio) e sempre
  devolve `""`. Diferente das outras três desta seção, o motivo aqui não é
  só "não existe container de resources": `cRes`, no fluxo real do TDN, é
  um identificador de resource INTERNO ao container do RPO, obtido
  previamente via `GetResArray("*.per")` — que enumera os nomes válidos e
  que AdvPP também não implementa. Ou seja, nenhum chamador seguindo o
  fluxo documentado pelo TDN conseguiria alcançar `GetApoRes` com um
  argumento válido. Por isso `cRes` NÃO é reinterpretado como um caminho
  de disco arbitrário (uma primeira versão desta função fazia
  `os.ReadFile(cRes)`, o que foi revertido por responder a uma pergunta
  diferente da que o TDN especifica — ver histórico do commit desta task).
- `GetDependency(sFonte)` valida o argumento (`sFonte` não pode ser vazio)
  e sempre devolve array vazio — não há grafo de chamadas por arquivo
  retido no bytecode; reconstruí-lo exigiria reanalisar o AST do parser
  por fonte, fora do escopo desta task.
- `GetRpoLog([nRPO])` valida `nRPO` (aceita 1 = Padrão ou 3 = Custom,
  conforme TDN) e sempre devolve `{% raw %}{{"", <data vazia>}, 0}{% endraw %}` — versão/data de
  RPO vazias e contagem de patches 0, que é verdadeiro dentro da
  arquitetura AdvPP (não existe sistema de patches, logo a contagem real é
  sempre zero).
- `GetSrcArray(cNome, [nRPO])` valida os argumentos (`cNome` não pode ser
  vazio; `nRPO` quando informado deve ser 1/2/3 conforme TDN) e sempre
  devolve array vazio — não há índice de nomes de arquivo-fonte compilados
  para consultar.

Nenhuma das quatro tenta simular o formato binário do RPO real ou inventar
dados de patch/dependência/resource que não existem. `GetFuncArray`, em
contraste, tem equivalente real em AdvPP porque não depende de um índice
físico: o conjunto de funções conhecidas pelo VM em execução
(`v.bc.Functions` + `v.natives`) é a fonte da verdade real do compilador
para "funções compiladas no repositório em uso", então essa função foi
implementada com lógica real (ver seção de parâmetros por referência acima
para a limitação dos parâmetros `@`). **Divergência conhecida de
`GetFuncArray`:** o casamento inclui também `v.natives` — as funções
nativas do motor AdvPP (ex: `CONOUT`, `MSGALERT`). No Protheus real essas
nativas ficam compiladas dentro do binário do TOTVS Application Server, e
nunca residem dentro do RPO — logo `GetFuncArray` real nunca as retornaria.
AdvPP não tem uma fronteira engine/user-function equivalente à do
AppServer, então essa divergência é uma escolha defensável (dado que
"funções conhecidas pelo VM" é a melhor aproximação disponível), mas é
comportamento diferente do TDN e fica registrado aqui.

`GetAPOInfo` e `RetImgType` também não se enquadram nesta seção: operam
sobre um único arquivo nomeado por parâmetro, e AdvPP lê esse arquivo real
do disco quando ele existe — comportamento real, não simulado. Nota sobre
`GetAPOInfo(cFonte)`: `cFonte` é resolvido como caminho literal relativo
ao diretório de trabalho (cwd) do processo, não via mecanismo `SourcePath`
do Protheus — o próprio exemplo do TDN, `GetAPOInfo("ExemplosTDN.prw")`
(nome de arquivo sem path), só encontra o arquivo se ele estiver no cwd do
processo AdvPP; caso contrário, devolve array vazio em silêncio.

**Afetadas:** `ChkRpoChg`, `GetApoRes`, `GetDependency`, `GetRpoLog`,
`GetSrcArray` (pkg/vm/rpo_native.go, Task 17).

## Canonicalização C14N não-exclusiva incompleta (`XmlC14N`, `XmlC14NFile`)

`XmlC14N`/`XmlC14NFile` (pkg/vm/xml_native.go, Task 23) implementam de fato
o núcleo do algoritmo W3C REC-xml-c14n-20010315 (não é stub): remoção de
comentários e da declaração XML/demais PIs, conversão de CDATA para
conteúdo literal, ordenação alfabética de atributos e de declarações de
namespace, elementos vazios sempre com tag de fechamento explícita (nunca
self-closing), e escaping de caracteres exatamente conforme a especificação
(texto: `&`,`<`,`>`,CR; atributo: `&`,`<`,`"`,TAB,LF,CR).

**O que NÃO é implementado** (verificado: `go.mod` não lista nenhuma
dependência de C14N/xmlsec — checado antes de escrever a implementação;
escrever um C14N 100% conforme ao "namespace axis" completo é um projeto
maior que esta task): o eixo de namespace do C14N não-exclusivo, que exige
re-renderizar em CADA elemento todo namespace herdado que esteja em escopo
— mesmo quando declarado só em um ancestral distante e nunca redeclarado no
caminho até o elemento atual. A implementação em `pkg/vm/xml_native.go`
(`canonicalizeXML`) só resolve um prefixo se o `xmlns:*` correspondente
aparece em algum elemento realmente visitado na pilha do documento sendo
processado (o que cobre o caso comum de documentos com namespaces
declarados uma vez perto da raiz e reutilizados abaixo, mas não o caso
adversarial de reconstrução do eixo completo por elemento). Atributos
`xml:lang`/`xml:space` também não recebem tratamento de herança dedicado.
Nenhum destes casos aparece nos exemplos da própria página TDN de
`XmlC14N`/`XmlC14NFile` (que não incluem o conteúdo do arquivo XML de
exemplo usado, apenas chamam a função sobre ele) — não há, portanto,
ground-truth documentado pela própria TDN para validar contra.

## Ausência de validador XML Schema (XSD) completo (`XmlFVldSch`)

`XmlFVldSch` (pkg/vm/xml_native.go, Task 23) segundo o TDN valida um
arquivo XML contra um XSD e retorna `.T.`/`.F.` populando `@cError`/
`@cWarning` com o motivo de uma eventual falha (ex.: no exemplo da própria
TDN, `invalid.xml` com `<Quantidade>ABC</Quantidade>` deveria falhar porque
`ABC` não é um `xs:integer` válido).

AdvPP não tem, em nenhum lugar, um validador de XML Schema completo: não há
suporte no stdlib Go (`encoding/xml` só faz parsing, não validação de
schema) e `go.mod` não lista nenhuma dependência de schema/XSD (checado
antes de implementar). Escrever um validador XSD 1.1 completo (tipos
simples/complexos derivados por restrição, `xs:pattern`/`xs:enumeration`,
`xs:choice`/`xs:all`, cardinalidade avançada, atributos, etc.) é um projeto
por si só, fora do escopo desta task.

**Revisão (code review, Task 23):** uma primeira versão desta função
verificava só boa-formação XML e retornava `.T.` para qualquer par de
arquivos bem-formados — o que respondia **errado** exatamente o exemplo de
maior destaque da própria página do TDN (schema tipando `Quantidade` como
`xs:integer`, XML com `Quantidade='ABC'` deveria reprovar). Isso foi
corrigido: em vez de tratar a função inteira como fora de escopo, foi
implementado um **verificador estrutural mínimo, mas real**
(`xsdCheckSchema`/`xsdValidateInstance`/`buildXsdNode` em
`pkg/vm/xml_native.go`) cobrindo exatamente os dois mecanismos de falha que
os exemplos da TDN exercitam:
1. **Presença de elemento obrigatório** — `xs:sequence` com `minOccurs`
   (default 1); elemento ausente na instância XML → inválido.
2. **Tipo primitivo do conteúdo de um elemento tipado** — `xs:integer`
   (e as demais variantes inteiras XSD), `xs:decimal`/`xs:float`/`xs:double`,
   `xs:boolean`, `xs:date`, `xs:dateTime`; conteúdo que não faz parse como o
   tipo declarado → inválido. `xs:string` (e qualquer tipo não reconhecido)
   nunca reprova por formato.

Isto já resolve corretamente o exemplo de `Quantidade`/`xs:integer` da
própria TDN (coberto por `TestXmlFVldSchExemploTDNQuantidadeXsInteger` em
`pkg/vm/xml_native_test.go`, que reproduz o par valid.xml/invalid.xml
literalmente descrito na página, incluindo a mensagem de erro exata
"Element 'Quantidade': 'ABC' is not a valid value of the atomic type
'xs:integer'." — gerada internamente por `xsdValidateInstance`, ainda que
não possa ser escrita em `@cError` por causa da limitação de parâmetros por
referência, ver seção acima).

**O que continua fora de escopo** (documentado, não fingido):
`xs:choice`/`xs:all` (só `xs:sequence` é interpretado), `xs:pattern`/
`xs:enumeration`/`xs:minLength`/etc., tipos simples nomeados derivados por
restrição (exceto o caso trivial de `<xs:simpleType><xs:restriction
base="xs:integer">` inline, que É resolvido), `complexType` referenciado com
prefixo de namespace não resolvido para um tipo primitivo/complexType de
topo conhecido, e atributos XML (só elementos são checados, não `@attr`).
Quando o schema usa algum desses recursos, ou quando a forma raiz do
documento XML não bate com nenhum `<xs:element>` de topo do schema, o
verificador não consegue se aplicar e `xsdCheckSchema` cai de volta na
postura anterior — retorna `.T.` (schema fora do subconjunto reconhecido
não reprova o XML só por isso; a checagem de boa-formação + existência de
arquivo continua valendo). `XmlFVldSch` não deve ser usado como gate de
validação de schema completo em produção fora do subconjunto documentado
acima.

## Controle de processo: pool de threads de AppServer inexistente (`ManualJob`, `SmartJob`)

As funções `ManualJob` e `SmartJob` (pkg/vm/controleprocessamento_native.go,
Task 25) no Protheus real dependem da infraestrutura de Jobs/pool de threads
do AppServer (seções `[SMARTJOB]`/OnStart do ini, alocação de `nMin`/`nMax`/
`nIncr` threads, fila FIFO com controle de recursos `memload`/`minjobs`/
`maxjobs`). AdvPP é um runtime headless **sem AppServer** e sem essa
infraestrutura — portanto:

- **`ManualJob`** executa a função-alvo do job num job isolado (via
  `StartJob`, um VM novo em goroutine) em vez de gerenciar um pool de
  threads. Para `cJobType='MDI'` executa `cOnStart` com `cSSKey` como
  argumento; para qualquer outro tipo executa `cOnConnect`. Os parâmetros
  de pool (`nInactive`, `nMin`, `nMax`, `nMinFree`, `nIncr`, `nWaitTime`)
  são aceitos para preservar a assinatura, mas **ignorados** — não há
  escalonamento multi-thread a controlar.
- **`SmartJob`** dispara `cName` num job isolado não-bloqueante (mesma
  semântica de `StartJob(wait=.F.)`). O `lWait` é sempre tratado como `.F.`
  internamente (conforme TDN). A **fila FIFO e o controle de recursos**
  (memload/minjobs/maxjobs da seção `[SMARTJOB]`) **não existem**: a função
  valida a existência do alvo (`VM.functionExists`) e dispara imediatamente,
  retornando `.T.`; o limite global de jobs concorrentes é o `MaxConcurrentJobs`
  já usado pelo `StartJob`.
- **`ExUserException`** no Protheus exibe a janela de Error log antes de
  abortar. No runtime headless não há janela; a semântica observável é a
  interrupção da execução com a mensagem (propagada como erro).

### Stubs mantidos (sem appserver para implementar)

`KillApp`, `setFinishAppHandler`, `KillUser`, `SysRefresh`, `JobInfo`,
`ProcLine`, `UserException`, `ProcName` continuam como stubs (ver
`docs/tdn-gap-stubs.md`) — exigem estado de processo/sessão do AppServer
que o runtime embutido não mantém.


## Impressão: sem AppServer/SmartClient nem spooler (`GetPortActive`)

`GetPortActive` (pkg/vm/controleimpressao_native.go, Task 26) no Protheus real
distingue portas do Application Server (`lDirect=.T.`) das do Smart Client
(`lDirect=.F.`). O runtime embutido é headless, sem AppServer nem SmartClient:
a função aceita `lDirect` mas ambas as direções retornam a mesma enumeração —
as portas seriais/paralelas reais do host (lidas de `/dev` via
`enumerateSerialPorts`: `ttyS*`, `ttyUSB*`, `ttyACM*`, `ttyAMA*`,
`ttyXRUSB*`, `lp*`), ou `{}` quando nenhuma existe (comportamento das builds
>7.00.111010P, conforme TDN).

As demais 18 funções de Controle de impressão (`__Eject`, `_PCol`, `_PRow`,
`DevOut`, `DevOutPict`, `DevPos`, `FechaRel`, `GetConnStatus`, `GetImpInf`,
`InitPrint`, `PreparePrint`, `PrintOut`, `PrnFlush`, `QOut`, `QQOut`,
`RmvToken`, `SetPrc`, `SndToPrnWin`) não foram implementadas por serem stubs
genuínos do TDN (páginas com corpo vazio, sem spec) — ver
`docs/tdn-gap-stubs.md`. Além disso, a maior parte delas exigiria um spooler
de impressão/UI de relatório que não existe no runtime.

## Conversão de tipos: convenções fixadas a partir da fonte

Decisões de conformidade da Task 27 (`pkg/vm/conversaotipos_native.go`),
validadas contra a fonte (colors.ch real da TOTVS) e contra os exemplos
numéricos do TDN — registradas aqui para revisão:

- **Endianness**: todas as conversões binárias (`Bin2I/L/W/F/D`, `I2Bin`,
  `L2Bin`, `W2Bin`, `F2Bin`, `D2Bin`, `Bin2Str`) usam **little-endian**
  (convenção Clipper/Harbour base do AdvPL). Os exemplos do TDN (`L2Bin
  (1145258561)` = "ABCD", `Bin2L("ABCD")` = 1145258561, `Bin2I("AB")` =
  16961) só batem com LE.
- **Formato de cor**: `ColorToRGB` interpreta `nColor` como **0x00BBGGRR**
  (BGR, COLORREF Windows). Confirmado com o `colors.ch` real
  (`CLR_HBLUE=16711680=0xFF0000` → `{0,0,255,0}`; `CLR_HRED=255` →
  `{255,0,0,0}`). Alpha no byte alto (bits 24-31).
- **Serial de data OLE**: `Dbl2Dt`/`Dt2Dbl` usam epoch **1899-12-30** e
  fração = milissegundos/86400000. Derivado dos exemplos do TDN:
  `DBL2DT(40544.52426839)` → "20110101 12:34:56.789". A fração é
  **arredondada** no `Dbl2Dt` (ms inteiro); `Dt2Dbl` devolve o double
  natural de `days + ms/86400000` (a representação float64 pode divergir
  na 8ª casa decimal — ex. `40544.5242683912` vs TDN `40544.52426839`).
- **GetDtoDate**: usa formato **US `mm/dd/yy`** (com ou sem separadores),
  conforme exemplo TDN `GetDtoDate("021605")`/"02/16/05" → 16/fev/2005.
  Diverge do `CToD` pré-existente (formato brasileiro `dd/mm/yyyy`).
- **BmpToJpg**: usa `gobmp` (decode) + `image/jpeg` (encode). Aceita BMP
  de qualquer BPP que o gobmp decodifique (o TDN restringe a 8/16/24 BPP
  com BITMAPV3INFOHEADER conforme a build do AppServer; aqui não há essa
  restrição de build). Não há UI para exibir erro — apenas o retorno `-1`.

## Interface-HTTP: família HTTP*/HTTPS* legada (Task 29)

As funções `HTTPGet`, `HTTPCGet`, `HTTPPost`, `HTTPCPost`, `HTTPQuote`,
`HTTPSGet`, `HTTPSPost`, `HTTPSQuote`, `HTTPGetStatus`, `HTTPSetPass`,
`SetProxy`, `SetNoProxyFor` (pkg/vm/interfacehttp_native.go) fazem
requisições HTTP/HTTPS **reais** via `net/http` (GET/POST/método arbitrário
via `HTTPQuote`/`HTTPSQuote`, headers customizados no formato `"Nome| Valor"`
ou `"Nome: Valor"`, time-out configurável com default de 120s, e suporte a
proxy com lista de exceções por domínio). Retornam a string do documento
solicitado, ou `Nil` em caso de time-out/falha de DNS/erro de URL (conforme
TDN). Limitações e divergências conhecidas:

- **`@cHeaderGet`/`@cHeaderRet` (por referência) não são populados.** O
  header de resposta fica gravado em `v.legacyHTTPHeader` e o status em
  `v.legacyHTTPStatus`, consultáveis via `HTTPGetStatus()` — mas a variável
  do chamador passada com `@` não recebe o valor (mesma limitação arquitetural
  de `@var` documentada no topo deste arquivo). `HTTPGetStatus(@cError)`
  também não popula a descrição do erro por referência; ela fica em
  `v.legacyHTTPError` e o retorno numérico (status HTTP, `<100` = erro) é a
  forma suportada de checar o resultado.
- **`HTTPSetPass`/`SetProxy`/`SetNoProxyFor` são estados do VM** válidos
  apenas dentro da sessão (não configuram proxy/autenticação a nível de
  sistema operacional). O `lClient` (SmartClient vs AppServer) é aceito e
  ignorado — o runtime embutido não tem SmartClient. `SetProxy` aplica o
  proxy a todas as requisições HTTP*/HTTPS* subsequentes (honrando a lista
  de `SetNoProxyFor`, com curingas `*.domínio`/`prefixo.*`), exceto quando
  a variável de ambiente `HTTP_PROXY`/`HTTPS_PROXY` do processo já define um
  proxy (ProxyFromEnvironment tem precedência).
- **Certificados nas variantes HTTPS***: `cCertificate`/`cPrivKey` são
  arquivos PEM lidos do disco local (paths estilo Windows `"\certs\..."` são
  rejeitados com a mensagem "only server path are allowed", espelhando o
  TDN). A verificação da cadeia de certificados do servidor é **habilitada**
  por padrão (TLS 1.2+); para testes contra servidores self-signed existe o
  escape hatch `ADVPP_HTTP_INSECURE=1` (mesmo padrão opt-in do
  `ADVPP_SFTP_KNOWN_HOSTS`).
- **`Valores-de-Content-Types`** é uma tabela de referência MIME do TDN, não
  uma função — nada a implementar (o `Content-Type` é informado via
  `aHeadStr`, ex. `"Content-Type| application/json"`).

## Manipulação de string: by-ref e formatos proprietários (Task 30)

As 25 funções novas de `Manipulacao-de-string`
(`pkg/vm/string_native.go`, Task 30) implementam o comportamento
documentado no TDN com as seguintes divergências conhecidas:

- **`@cBufferOut`/`@nLenghtOut` (por referência) não são populados.** Em
  `NotBit`, `StuffBit`, `UnStuff`, `Compress`, `UnCompress`, `GzStrComp`,
  `GzStrDecomp` o buffer/string **modificado é devolvido como valor de
  retorno** (única forma de resultado suportada neste VM — ver a limitação
  arquitetural de `@var` no topo deste arquivo). `UnStuff`/`NotBit`
  adicionam `@nResultPos` como parâmetro final opcional (índice da string
  de retorno, `.T.` na TDN) — como não há by-ref, o índice é aceito mas a
  posição é sempre a de retorno natural. `Compress`/`UnCompress`/`GzStrComp`/
  `GzStrDecomp` retornam `Nil` em caso de erro (fonte/destino inválidos,
  dados corrompidos) — mesmo mapeamento de erro que as demais natives.
- **Compress/UnCompress usam zlib (RFC 1950)**, não o algoritmo proprietário
  TOTVS (que não é reproduzível sem o binário original). O round-trip
  `Compress`→`UnCompress` é fiel, mas buffers produzidos pelo Protheus real
  (e vice-versa) **não** são compatíveis — documentado, não tentado.
- **GzStrComp/GzStrDecomp usam gzip real (`compress/gzip`, RFC 1952)**, com
  round-trip fiel — estas sim são compatíveis com arquivos `.gz` do Protheus.
- **ANSIToOEM/OEMToANSI usam CP850** (a tabela DOS do português/ibérica),
  não CP437: o CP437 não encoda `ã`/`õ`/`À`, e o exemplo da TDN não
  discrimina qual tabela usar. Para texto estritamente US-ASCII as duas
  tabelas coincidem; para acentuação pt-BR, CP850 é a escolha correta.
- **STRICONV/Encode/Decode UTF-8/UTF-16** usam `golang.org/x/text`
  (dependência transitiva já existente). `STRICONV("...","UTF-8")` trata o
  codepage UTF-8 como passthrough (sem transcodificação). Codepages
  desconhecidos em `STRICONV`/`Encode*` retornam `Nil`; em `DecodeUTF8`/
  `DecodeUTF16` retornam `Nil` quando o bytes não formam UTF-8/UTF-16 válido.
- **`Encode64`/`Decode64` com `cFilePath`** (variante de arquivo em disco)
  retornam `Nil` — não suportada (só a variante string).
- **`Pad`** preenche com espaço por padrão quando `cChar` não é informado,
  e **trunca** o resultado se o tamanho final exceder `nLen` (comportamento
  TDN "preenche e trunca", diferentemente de `PADC/PADL/PADR` que truncam
  só o prefixo/sufixo).
- **`Match`** implementa curingas Clipper/Harbour: `*` (qualquer sequência,
  inclui string vazia), `?` (um caractere qualquer), `!` (nega o próximo
  padrão), `[classe]` (conjunto, com intervalos `a-z` e negação `[!...]`) e
  `#` (um dígito). **Não** suporta o operador de alternância Clipper
  `|` (listas de padrões separadas por `|`) nem classes predefinidas como
  `$`/`~`.
- **`MLCount`** usa largura de linha default 79 quando `nLinLen` é 0 ou não
  informado, e `nTabSize` default 4; com `lQuebra=.F.` trima o espaço
  inicial da linha seguinte ao quebrar no limite (comportamento observado
  no exemplo da TDN, que espera 9 linhas para um texto de 362 chars).
- **`GetDToVal`** aceita `nType` 0/1/2 (0=double, 1=int, 2=string) e `nDec`
  default 10 quando não informado — `nType` inválido cai para double.
- **`StrTokArr2`** aceita `cSep` default `";"` e `nEsc` default `"\x1a"`
  (caractere de escape SUB/CTRL-Z) quando não informados; o escape é
  preservado literalmente na saída (igual à TDN, que documenta "o caractere
  escape é removido da string de resultado").
- **`DecodeUTF16`/`EncodeUTF16`** usam UTF-16 little-endian com BOM
  (`0xFEFF`), que é a variante do Windows usada pelo Protheus.
- **`Descend`** ordena bytes em ordem decrescente (ordem reversa dos bytes,
  ex.: "AAAA" → "aaaa"; igual ao exemplo do TDN) — documentado pois
  funciona por reordenação de bytes, não por collation.

## DynCall (`tRunDll`): chamada dinâmica de DLL/SO

Implementado em `pkg/vm/dyncall_native.go` com
`github.com/ebitengine/purego` (sem cgo): `New`/`Free` fazem
dlopen/dlclose real; `CallFunction`/`CallMethod` montam a ABI real
(inclusive double/float no registrador correto) via `reflect.FuncOf` +
`purego.RegisterFunc`, cobrindo toda a legenda de tipos da TDN
(`V,B,C,c,S,s,I,i,L,l,G,g,F,D,P,A,T`); `GetVar`/`SetVar` leem/escrevem
memória bruta de uma variável global exportada pela DLL via `Dlsym` +
`unsafe.Pointer`; `NewPointer`/`NewObj`/`FreeObj`/`StrLen`/`StrCpy`/
`MemCpy` operam sobre handles opacos (endereços C reais, incluindo
alocação Go real e fixada com `runtime.Pinner` para o caso 2 de
`NewObj(nBytes)` — "TLPP aloca e chama o construtor"). Testado
end-to-end contra bibliotecas C e C++ reais compiladas por `gcc`/`g++`
em tempo de teste (`pkg/vm/dyncall_native_test.go` +
`pkg/vm/testdata/dyncall/`), não simulado — inclusive a chamada real ao
construtor `tArith::tArith()` sobre memória alocada por `NewObj(64)`.

**Todo método documenta na TDN "retorno: lógico"** (`CallFunction`,
`CallMethod`, `GetVar`, `SetVar`, `Free`, `FreeObj`, `SetTimeout`,
`StrLen`) — o valor de fato é sempre um parâmetro de
saída por referência (`xRet`/`nRet`/`cRet`), nunca o retorno do método.
Esta VM preserva o retorno lógico documentado tal como é (nunca o
substitui pelo valor computado) e simplesmente não popula o `xRet` — a
mesma limitação de `@var` documentada no topo deste arquivo, aplicada
aqui com o mesmo critério de honestidade já usado em `GetGlbVars`/
`IPCWaitEx`/`SFTP*`/`XmlC14N`. Exceção real (não um substituto): quando
o `xRet` passado é um objeto `TRunDllPointer` (de `NewPointer`/`NewObj`/
`CallMethod` anterior), seu ponteiro interno É atualizado in-place —
objetos são tipo referência neste VM, mesmo mecanismo já usado por
`HMGet`/`VarGet` para parâmetros de array. Na prática, isso significa
que o valor numérico/string retornado por uma função/método C só é
observável através deste VM quando o tipo de retorno é `P` (ponteiro) —
para `I`/`D`/outros escalares, o resultado é real internamente (a
chamada FFI ocorre e o valor é computado corretamente, coberto pelos
testes de `dynCallInvoke`) mas fica inacessível ao código AdvPL/TLPP
chamador, exatamente como os demais casos de `@var` já documentados.
- **Exceção deliberada: `StrCpy(oPointer, nMaxSize)` e
  `MemCpy(oPointer, nBytes)` desviam do contrato "lógico com cRet mudo"
  documentado pela TDN — retornam diretamente a `String` lida do buffer
  C (até `\0`/`nMaxSize` no primeiro caso, `nBytes` brutos no segundo),
  em vez de um `Logical` inútil. Sem essa exceção os dois métodos eram
  funcionalmente inertes (não havia nenhuma forma de ler o conteúdo de
  um buffer escrito por uma DLL de volta para AdvPL/TLPP). `StrLen`
  permanece com o contrato antigo (só sinaliza ponteiro válido via
  `.T./.F.`, o tamanho real não é observável) por ainda não ter sido
  necessário para nenhum caso de uso real.
- **`CallMethod` (DLL C++) só resolve mangling Itanium** (GCC/Clang —
  Linux, macOS, MinGW), verificado contra o símbolo real emitido por
  `g++` (`nm -D`) para os exemplos da própria TDN
  (`tArith::Add(double,double)` → `_ZN6tArith3AddEdd`;
  `tArith::tArith()` → `_ZN6tArithC1Ev`, marcador de construtor
  Itanium, nunca o nome da classe repetido). Construtor/destrutor têm
  fallback automático C1→C2/D1→D2: clang (default no macOS) pode não
  emitir o "complete object constructor" (C1) como símbolo próprio
  quando idêntico ao "base object constructor" (C2) para uma classe sem
  bases virtuais — achado real via CI macOS (gcc, em Linux/MinGW, emite
  os dois). `CallMethod` tenta C1/D1 primeiro (escolha ABI-correta) e só
  cai para C2/D2 se o Dlsym do símbolo primário falhar (ver
  `itaniumMangleCtorDtorFallback` em `pkg/vm/dyncall_native.go`).
  Mangling MSVC (`cl.exe`, o
  compilador nativo mais comum para DLLs de produção em Windows) é um
  esquema proprietário não documentado publicamente pela Microsoft em
  sua totalidade — não implementado; `CallMethod` contra uma DLL
  compilada com MSVC devolve `.F.` + `GetErrorMsg()` explicando o
  símbolo não encontrado, em vez de arriscar um mangling incorreto (que
  causaria crash/corrupção silenciosa em vez de um erro honesto).
- **Tipos de parâmetro em `CallMethod` limitados a primitivos C
  (+ 1 nível de ponteiro/`const`)**: referências, classes por valor,
  templates e sobrecarga de operadores não são suportados no mangling —
  erro explícito em vez de símbolo errado.
- **`long`/`unsigned long` (`L`/`l`)** seguem o tamanho real da ABI da
  plataforma de execução (`runtime.GOOS`): 8 bytes em Linux/macOS
  (LP64), 4 bytes em Windows (LLP64) — mesma convenção do compilador C
  nativo de cada SO. `long long`/`unsigned long long` (`G`/`g`) são
  sempre 8 bytes, em qualquer SO.
- **`FreeObj` só libera memória alocada por este VM** (caso 2 de
  `NewObj(nBytes)`, via `runtime.Pinner`). Um objeto obtido de dentro da
  DLL (caso 1, `NewObj()` + factory/`malloc`) não é liberado por
  `FreeObj` — este VM não é o dono dessa memória e chamar `free()` sobre
  um ponteiro alocado por um alocador C desconhecido seria arriscado;
  documentado em vez de simulado.
- **`GetTimeout`/`SetTimeout`** são aceitos e armazenados (default 60s,
  igual ao documentado em "DynCall - Configuração de timeout"), sem
  efeito real — a chamada FFI é síncrona in-process (mesma goroutine),
  sem mecanismo seguro para abortar uma chamada C travada a meio
  caminho (o exemplo da TDN espera a thread terminar "de forma
  graciosa" após o timeout, o que exigiria abandonar/matar uma thread
  do SO no meio de código C arbitrário — inseguro de fazer de forma
  genérica). A terceira forma de configuração documentada pela TDN —
  seção `[dyncall]` / `timeout = N` num `.ini` do AppServer — não é
  suportada: este VM não tem (em lugar nenhum, não só para DynCall) um
  mecanismo real de carregar configuração de um `appserver.ini`/
  `totvsappserver.ini` externo no start (ver `GetAdvplIni`/`GetMV` em
  `pkg/vm/ambiente_native.go`, que devolvem apenas o NOME do arquivo
  convencional, nunca leem/aplicam seu conteúdo). `SetTimeout` é a
  única forma real de configurar o valor neste VM.
- **Passagem de parâmetro por referência via `@` (TDN: DynCall -
  Passagem de parâmetros) não é suportada e pode causar chamada
  ABI-incorreta.** A TDN reusa a MESMA letra de assinatura escalar
  (ex. `'I'`) tanto para `int x` (por valor) quanto para `int* x`/
  `int& x` (por referência) — a diferença é sinalizada só pelo `@` no
  lado TLPP (`callFunction("sucessor", "II", nRet, @nValue)`), nunca na
  `cSignature`. Confirmado em `pkg/lexer/lexer.go` (`TOKEN_AT`) e
  `pkg/parser/expressions.go` (`case lexer.TOKEN_AT: p.advance(); return
  p.parsePrimary()`): o compilador AdvPP **descarta** o `@` ao compilar
  um argumento de chamada — `@nValue` compila exatamente igual a
  `nValue`, em qualquer posição de chamada, não só DynCall. Não há
  nenhum lugar no VM/compilador onde essa informação sobreviva até
  `pkg/vm/dyncall_native.go`. Por isso, uma chamada real a uma
  função/método C/C++ que espere um ponteiro numa posição de letra
  escalar (`int*`, `int&`) recebe o valor por cópia em vez do endereço —
  ABI incorreta, que pode ler/escrever memória arbitrária ou crashar o
  processo, não apenas "perder o dado" como as demais limitações de
  `@var` deste arquivo. Não há correção possível sem adicionar rastreio
  de `@` ao compilador inteiro (fora do escopo desta classe). Único
  caminho seguro hoje: usar a letra `'P'` explicitamente na assinatura e
  passar um `TRunDllPointer` real (`NewPointer`/`NewObj`) como
  argumento, que já é inequivocamente um ponteiro e não depende do `@`.
- `go vet` sinaliza "possible misuse of unsafe.Pointer" em várias linhas
  de `dyncall_native.go` — esperado e documentado no próprio arquivo:
  é a natureza inevitável de uma FFI acessando memória fora do heap Go.
