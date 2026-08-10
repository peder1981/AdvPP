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
  conforme TDN) e sempre devolve `{{"", <data vazia>}, 0}` — versão/data de
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

## Ausência de validador XML Schema (XSD) real (`XmlFVldSch`)

`XmlFVldSch` (pkg/vm/xml_native.go, Task 23) segundo o TDN valida um
arquivo XML contra um XSD e retorna `.T.`/`.F.` populando `@cError`/
`@cWarning` com o motivo de uma eventual falha (ex.: no exemplo da própria
TDN, `invalid.xml` com `<Quantidade>ABC</Quantidade>` deveria falhar porque
`ABC` não é um `xs:integer` válido).

AdvPP não tem, em nenhum lugar, um validador de XML Schema: não há suporte
no stdlib Go (`encoding/xml` só faz parsing, não validação de schema) e
`go.mod` não lista nenhuma dependência de schema/XSD (checado antes de
implementar). Escrever um validador XSD completo (tipos simples/complexos,
`xs:integer`/`xs:enumeration`/`xs:pattern`/cardinalidade/`xs:sequence` vs.
`xs:choice`, etc.) é um projeto por si só, fora do escopo desta task — e a
própria página TDN não inclui o conteúdo dos arquivos de exemplo
(`schema_definition.xsd`, `valid.xml`, `invalid.xml` são apenas
referenciados como anexos, não mostrados inline), então não haveria
ground-truth para validar uma implementação parcial mesmo que se tentasse.

**Comportamento em AdvPP:** `XmlFVldSch` lê de fato os dois arquivos do
disco (`cXML`, `cXSD`) e retorna `.F.` se qualquer um não existir/não for
legível. Se ambos existirem, checa boa-formação XML sintática de ambos
(via `encoding/xml`) — não checa NENHUM constraint de schema (tipo,
obrigatoriedade, enumeração, cardinalidade). **Isto significa que, ao
contrário do segundo exemplo da própria TDN, esta implementação retornaria
`.T.` para um `invalid.xml` bem-formado mesmo violando o schema** — a
função só prova boa-formação + existência dos arquivos, não conformidade
real de schema. Documentado aqui explicitamente para não ser confundido com
um validador XSD funcional: não deve ser usado como gate de validação de
schema em produção.
