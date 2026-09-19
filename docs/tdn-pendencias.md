# Pendências reais (functions TDN sem native no AdvPP) — 2026-08-11

Método: cruzamento das folhas Functions/ do mirror (H1 real) contra as natives
registradas via NewVM (dump runtime). Excluídos stubs confirmados
(ConErr, TCConType) e sem-spec (ProcessMessages).

## Status: 0 pendências do lote original (tasks 32/33a/33b/33c/34) — TODAS implementadas

As 41 funções abaixo foram implementadas (commits 83d1bbe, 072d863, 542fcff,
a7d9ebf, a8bf70b; fixes de review em 45375e3). Runtime atual: 729 natives.

- ~~CmpBuildStr, GetBuild, GetEndPoint~~ ✅ (Task 32)
- ~~DBChangeAlias, DBClearAllFilter, DBClearIndex, DBCloseAll, DBCommitAll,
  DBCreate, DBFieldInfo, DBFilter, DBFilterCB, DBGetActFld, DBGoTo, DBInInsert,
  DBInfo, FLock, Field, FieldBlock, FieldWBlock, Found, GetDBExtension, Header,
  IndexKey, IndexOrd, LastRec, NetErr, OrdBagName, OrdCreate, OrdDescend, OrdKey,
  OrdListAdd, OrdName, OrdNumber, OrdSetFocus, RDDName, RDDSetDefault, RLock,
  RealRDD, RecSize~~ ✅ (Task 33)
- ~~SMIMESign~~ ✅ (Task 34)

TOTAL: 41 ✅ implementadas

## Backlog natural — 2026-09-19: revalidado item a item, 0 restam implementáveis

Cruzamento completo (2026-08-11): 710 folhas Functions/ vs 729 natives
(535 cobertas; 7 páginas não-função excluídas; 168 reais sem native
naquela data). `Resource2File` foi implementada em 2026-08-23
(`pkg/vm/rpo_native.go`, ver `docs/tdn-known-limitations.md`) — contagem
ajustada para 167.

Em 2026-09-19 as 167 funções foram checadas uma a uma contra o corpo real
das páginas em `~/tdn-advpl-mirror/` (não contra o índice, contra o
conteúdo). Resultado:

- **4 já estavam implementadas** sob o native correto, só desatualizadas
  no backlog: GetProfInt/GetProfString/WritePProString/WriteProfString
  (agora formalizados como aliases, ver abaixo) e **AttlsMemberOf** (slug
  do TDN; o H1 real da página e a sintaxe da função são `AttIsMemberOf` —
  já coberto por `pkg/vm/manipclasse_native.go` via `ATTISMEMBEROF`).
- **As 163 restantes têm página TDN confirmada vazia** ("Tempo aproximado
  para leitura" / "Sem rótulos", sem corpo) — sem sintaxe, sem parâmetros,
  sem retorno documentado. Implementar qualquer uma delas exigiria
  inventar comportamento a partir de memória genérica de Clipper/AdvPL, o
  que este projeto não faz (ver `docs/tdn-gap-stubs.md` e o precedente já
  registrado em `pkg/vm/controleprocessamento_native.go`: "Stubs
  confirmados ... NÃO são registrados"). As 20 páginas de
  Functions/Interface-HTTP não estavam na lista de stubs (o script de
  detecção não pegou o padrão delas) — inspecionadas manualmente agora,
  são igualmente vazias; adicionadas ao `docs/tdn-gap-stubs.md`.

Ou seja: **não há mais nenhuma função nesta lista implementável sem
fabricar spec.** Path a seguir, se/quando aparecer necessidade real:
1. Um brief explícito do usuário descrevendo o comportamento esperado
   (equivalente a uma spec), ou
2. TOTVS publicar/atualizar a página TDN, ou
3. Encontrar a função em uso real num fonte AdvPL do corpus (`consultar_base_direta`)
   com contexto suficiente pra inferir o contrato com confiança.

### ✅ Implementadas nesta rodada (2026-09-19)

- ~~Manipulacao-do-arquivo-INI (4)~~: GetProfInt, GetProfString, WritePProString,
  WriteProfString — aliases de GetPvProfileInt/GetPvProfString/
  WriteSrvProfString reaproveitando iniGetKey/iniSetKey (`pkg/vm/arquivoini_native.go`)

### 🔴 Bloqueado por limitação arquitetural (não é gap de spec)

- **Verificacao-dos-tipos (5)**: ClearVarSetGet, ContType, VarRef, VarSetGet,
  VarUnref — estas TÊM spec real na TDN (ao contrário do resto da lista),
  mas exigem semântica de referência a variável (getter/setter ligado a
  uma variável por ponteiro) que o compilador AdvPP não tem para
  escalares — `@param` é descartado no parser (só array/objeto propagam
  por referência, ver `pkg/vm/vm.go`). Requer mudança arquitetural no
  compilador/VM, não é um native isolado.

### ⚪ Confirmado sem spec real na TDN (página vazia) — não implementável sem fabricar

- **Ambiente/Funcoes-genericas (14)**: GetEnvHost, GetRemoteType, GetSrvVersion,
  GetWebJob, IsPlugin, IsPrinter2, IsSecure, IsSrv64, IsSrvBigE, SrvDisplay,
  ThreadCount, ThreadID, __Quit, __SetPicture
- **Interface-HTTP (20)**: HTTPCTDisp, HTTPCTLen, HTTPCTType, HTTPExitProc,
  HTTPFreeSession, HTTPGetPart, HTTPIsAPW, HTTPIsConnected, HTTPLeaveSession,
  HTTPLogonUser, HTTPOtherContent, HTTPPostXml, HTTPPragma, HTTPRCTDisp,
  HTTPRCTLen, HTTPRCTType, HTTPSend, HTTPSetPart, HttpCache, HttpCountSession
- **Componentes-de-interface-visual (43)**: AppBringToFront, CalcFieldSize,
  ChkBmpRlt, CreateSession, CursorArrow, CursorWait, ExecInClient, MSCalculator,
  MessageBox, MsgNoYes, MsgRetryCal, MsgRun, PtGetTheme, PtSetAcento, PtSetTheme,
  SendToFore, SetDefCaption, SetDefFont, SetFlatControls, SetFocus, SetRmtDate,
  SetTransparentColor, SetWndDefault, ShowHelpCpo, ShowHelpDlg, AddCSSRule,
  AddFontAlias, Beep, CSSDictAdd, GetFocus, GetFontList, GetFontPixWidths,
  GetHeightFont, GetScreenRes, GetSenhAp, GetStringPixSize, GetWndDefault,
  IntIncProc, PtGetSessions, PtKillSession, PtRunInSession, SetCSS, SetKey,
  SetKeyBlock
- **Controle-de-impressao (18)**: DevOut, DevOutPict, DevPos, FechaRel,
  GetConnStatus, GetImpInf, InitPrint, PreparePrint, PrintOut, PrnFlush, QOut,
  QQOut, RmvToken, SetPrc, SndToPrnWin, _PCol, _PRow, __Eject
- **Tratamento-de-XML (8)**: XmlChildEx, XmlCloneNode, XmlDelNode, XmlGetChild,
  XmlGetParent, XmlNewNode, XmlNode2Arr, XmlSVldSch
- **SAML (8)**: getSAMLID, getSAMLSvc, reloadSAML, saveIDPXML, setIDPConf,
  setSAMLID, setSAMLSvc, setSPCert
- **Manipulacao-de-arquivos-discos-IO (6)**: FT_FGoTop, FT_FLastRec, FT_FReadLn,
  FT_FRecno, FT_FSkip, MemoLine, SplitPath
- **Web-Services (4)**: WSClassNew, WSDL2Parser, WSDLParser, WSDescData
- **Manipulacao-de-memoria (4)**: __ClearRmt, __ListRmt, __LoadRmt, __SaveRmt
- **Seguranca/Criptografia (6)**: EVPPrivSign, EVPPrivVery, MsCRC32, MsCRC32Str,
  GetSslObj, SetSslObj
- **Outros (22)**: Dbf, OrdBagExt, CTUpdateIntName, ctreeDelIdxs, ctreeDelInt,
  SocketConn, GetCredential, GetUserFromSID, JobInfo, KillApp, KillUser,
  SysRefresh, setFinishAppHandler, __HEXTODEC, DelClassIntf, GetParentTree,
  HMDel, ClearGlbValue, MemGlbSize, TimeGlbValue, GlbLock, GlbUnlock,
  MailVersion (TCConType já era stub confirmado à parte, fora da contagem)

### Já implementado (item stale removido da contagem)

- ~~AttlsMemberOf~~ — já coberto por `ATTISMEMBEROF` (`pkg/vm/manipclasse_native.go`)

## Auditoria estendida a Classes/ — 2026-09-19

A pedido do usuário, a checagem foi ampliada além de `Functions/` (escopo
original deste doc) para `Classes/` (97 páginas), cruzando contra os 35 nomes
de classe com dispatch de `NEW` já implementados em `pkg/vm/vm.go`
(`newInstance`). 96 páginas não batem à primeira vista, mas quase todas caem
em uma destas categorias, nenhuma é gap real:

- Páginas de categoria/índice (ex.: `Componentes`, `Visual`, `Nao-Visual`,
  `Janelas`, `Mobile`) — não são classes.
- Duplicatas de nome (a página TDN usa prefixo `Classe-`, ex.
  `Classe-TFTPCLIENT.md`, `Classe-TJSONPARSER.md`) — a classe já está
  implementada sob o nome sem prefixo.
- Componentes visuais de verdade (TBUTTON, TDIALOG, TGET, TPANEL, TSBROWSE,
  MsCalend, TCBrowse, ~40 no total) — fora de escopo: AdvPP é um runtime
  headless/servidor, sem sistema de janelas; mesma decisão já tomada para as
  8 classes MVC complexas (ver memória do projeto) e para todo o backlog de
  Componentes-de-interface-visual em `Functions/`.
- Protocolos que exigiriam implementar um wire format inteiro sem spec
  pública suficiente ou sem dependência já presente: **TRpc** (protocolo
  proprietário TOTVS de RPC entre Application Servers — a página documenta
  só a API AdvPL, não o protocolo de rede) e **WDClient** (idem). **TAMQP**
  (663 linhas, spec real e completa, mas implementar AMQP 0-9-1 do zero
  exigiria ou uma dependência nova ou reimplementar um protocolo binário
  inteiro — não decidi sozinho, ver pergunta abaixo).

### ✅ Implementado nesta auditoria: classe `tJWT`

`Classes/Componentes/Nao-Visual/tJWT.md` (1094 linhas) tinha spec real e
completa (RFC 7519 + RFC 7518), não-visual, sem qualquer necessidade de I/O
de rede/GUI — só criptografia via `crypto/*` da stdlib. Implementada em
`pkg/vm/jwt_native.go` (+ 2 casos de dispatch em `pkg/vm/vm.go`):

- Construtor `tJWT():New()`.
- Header: setAlgorithm/setType/setContentType/setKeyId/setHeaderClaim +
  hasAlgorithm/hasType/hasContentType/hasKeyId/hasHeaderClaim.
- Payload: setIssuer/setSubject/setAudience/setExpiresAt/setNotBefore/
  setIssuedAt/setId/setPayloadClaim + has* equivalentes.
- Verificação: withIssuer/withSubject/withAudience/withId/withClaim/
  clearWithClaim.
- Chaves: setPubKey/setPrivKey/setSecretKey (PEM, mesmo padrão dos demais
  natives de criptografia do projeto).
- `createToken(cAlgo)` / `verifyToken(cAlgo)`: HS256, RS256, RS512, PS256,
  PS384, PS512, ES256 — todos via `crypto/hmac`, `crypto/rsa`, `crypto/ecdsa`
  da stdlib (zero dependências novas, zero código específico de SO/arch).
- `setToken`/`getLastError`/`encodeTime`/`decodeTime`, propriedade `token`.

Decisões tomadas onde a prosa da TDN é ambígua (documentadas em comentário no
código): `setExpiresAt`/`setNotBefore` recebem duração em segundos a partir
de "agora" (confirmado pelos dois exemplos "configurado para 24h/1h"),
`setIssuedAt` recebe um epoch absoluto (confirmado pelo exemplo que usa
`encodeTime()` antes de chamá-lo).

Testado em `pkg/vm/jwt_native_test.go` (HS256 create+verify com segredo
certo/errado, RS256 com chave gerada em runtime, token expirado, round-trip
de encodeTime/decodeTime). Build e suíte completa verdes; cross-compile
limpo nos 4 alvos oficiais do `make cross` (linux/amd64, linux/arm64,
windows/amd64, darwin/arm64) — código é criptografia pura, sem
particularidade de plataforma.

### ✅ Implementado nesta auditoria: classe `tAMQP` (com dependência nova, aprovada pelo usuário)

`Classes/Componentes/Nao-Visual/tAMQP.md` (663 linhas) documenta um wrapper
AMQP 0-9-1 para RabbitMQ. Diferente de tJWT, implementar o protocolo AMQP do
zero seria grande demais e arriscado (frames, canais, heartbeats,
negociação); perguntei ao usuário e ele optou por adicionar
`github.com/rabbitmq/amqp091-go` (a lib oficial pura-Go do RabbitMQ, mesma
usada pelo próprio time do RabbitMQ, zero cgo) como dependência nova do
`go.mod`.

Implementado em `pkg/vm/amqp_native.go` (+ 2 casos de dispatch em
`pkg/vm/vm.go`): New (conecta + abre canal), QueueDeclare/
QueueDeclarePassive, ExchangeDeclare/ExchangeDeclarePassive, QueueBind,
BasicPublish, BasicConsume (com timeout via propriedade `ConsumeTimeout` ou
30s se `bWaitingEvent`), BasicQos, BasicAck, CorrelationID, ReplyTo, Tag,
QueueName, MessageCount, ConsumerCount, Error, Status; propriedades
ChannelNumber/Body/ConsumeTimeout.

Cuidado documentado no código: a ordem de parâmetros de `BasicQos` e de
`QueueDeclare`/`ExchangeDeclare` na TDN não bate com a ordem da assinatura
Go da lib (`Qos(prefetchCount, prefetchSize, global)` inverte os dois
primeiros; `QueueDeclare(name, durable, autoDelete, exclusive, ...)` troca a
posição de exclusive/autodelete) — os remapeamentos estão comentados
inline em `pkg/vm/amqp_native.go` para não se perderem numa refatoração
futura.

Testado em `pkg/vm/amqp_native_test.go` **contra um broker RabbitMQ real**
(container `rabbitmq:3-alpine` efêmero, subido e removido só para rodar o
teste): publish com persistência+correlationId+replyTo, consume, leitura de
Body/CorrelationID/ReplyTo/Tag, ack, e timeout de consumo em fila vazia. Os
testes fazem skip automático se não houver broker em `localhost:5672` (não
quebram CI sem RabbitMQ). Cross-compile limpo nos 4 alvos oficiais — a lib é
pura Go, sem cgo, sem código específico de SO/arch.
