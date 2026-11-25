# AdvPP DBAccess Bridge — Design

**Data:** 2026-11-25
**Status:** Em discussão / Fase 0 (prova de vida)
**Branch alvo:** a definir (provável `feat/dbaccess-bridge` a partir de `master` ou `unstable`)

## Objetivo

Permitir que a VM AdvPP execute AdvPL real contra um **servidor dbAccess TOTVS real**
(`dbaccess64`), obtendo compatibilidade máxima com Protheus: o middleware oficial
faz a tradução de dialeto (PostgreSQL/Oracle/MSSQL), stored procedures, locks,
numeração e views — tudo que o `RemoteSQLEngine` atual (database/sql nativo)
não cobre.

Visão de longo prazo (decidida pelo operador): o mesmo núcleo serve para
ferramentas externas (ex.: migração Protheus → GEBAN-ERP) consumirem o banco
via dbAccess.

## Contexto verificado (evidências)

### Ambiente disponível

- Container Docker `dbaccess` (`feliperaposo/protheus-dbaccess`) **rodando** em
  `localhost:7890`, Postgres 15 em `localhost:5432`, ambos up.
- `dbaccess.ini` do server mapeia `[POSTGRES/protheus]` e `[POSTGRES/p1212510]`
  → `postgresql:5432` (dbs `protheus` e `p1212510`, user/pass `protheus`).
- Server opera em **`TOP_NO_LICENSE` / STANDALONE** (sem license server;
  `ALLOWOUTDATED=1`, "SSL: Not using secure connections").
- Log mostra tráfego real Protheus: `DBEnv [POSTGRES/P1212510]`,
  `ROP_SQLEXEC`, `ROP_PING`, `TCVersion 24.1.1.3`, build `20260522`.

### Binários

| Artefato | Local | Notas |
|---|---|---|
| `dbapi.so` (client) | `/dbaccess/library/dbapi.so` no container; cópia em `.scratch/dbaccess-poc/` | 47MB, **not stripped**, deps mínimas (libc/libstdc++/pthread/dl/rt) |
| `dbaccess64` (server) | `/dbaccess/multi/dbaccess64` | daemon TCP :7890 |
| Build oficial | `~/juice/Downloads/26-05-22-TOTVS_DBACCESS_BUILD_24.1.1.3_LINUX_X64.TAR.GZ` | mesmo layout |
| AppServer real | `~/GhidraProjects/appserver/` (appsrvlinux, dbapi.so, dbsrv.so, libaplinux.so) | referência de como o appserver carrega a lib |
| Cópia antiga p/ RE | `~/GhidraProjects/dbaccess/client/dbapi.so` | build jan/2026, API provavelmente igual |

### Superfície de API exportada (verificado via `nm -D` + `c++filt`)

`dbapi.so` exporta a classe **`tTOPClient`** — API RPC completa sobre o
protocolo dbAccess (≈90 métodos), mais famílias de clientes:

- **Conexão/meta**: `tBasicClient::tBasicClient(short, unsigned char)`,
  `ConnectToServer` (via tTOPClient), `Disconnect`, `Ping`, `Checkup`,
  `GetIO`, `GetSID`, `GetDBSID`, `GetDrivers`, `APICache`, `GetInfo(2)`,
  `SetLocation`, `SetThreadIDOri`, `AddConfig`, `SendConfig`, `Config`,
  `SetParam`, `SendParams`, `SrvMap`, `GetTargetBuild`, `InternalF`,
  `ServerUsingEncriptedConnection`, `SendEncryptedString`,
  `ReceiveFirstBuffer`, `TopClientShutDown()` (free function)
- **Workarea**: `OpenFile`, `JustClose`, `CanOpen`, `RefreshFile`,
  `FindWorkArea`, `RecNo`, `RecCount`, `Bof`, `Eof`, `GoTop`, `GoBottom`,
  `GoTo`, `Skip`, `GetInternalRec`, `GetField`, `GetFieldN`, `SetField`,
  `SetFieldN`, `GetFieldDesc(2)`, `GetColCount`, `GetKeyNames`, `GetKeys`,
  `SetKey`, `SetKeyC`, `SetFilter`, `SetFilterEx`, `SetDeletedFilter`,
  `SetDeleOn`, `UniqueKey`, `Pack`, `RecallR`, `Update`, `Write`, `Write2`,
  `InitRecord`, `CreateFile`, `DeleteFile`, `CreateIndex`, `CreateVIndex`,
  `DeleteIndex`
- **SQL**: `SQLExec`, `SQLExec2`, `SqlPrepare`, `SqlBind`, `SqlFree`,
  `SPExec`, `SPExist`, `ExecQueryToArray`, `Query`, `GetStruct`,
  `GetDBObject`, `GetDBSID`
- **Locks/transações**: `Lock`, `UnLock`, `MultLock`, `SoftLock`,
  `SoftFree`, `SoftFlush`, `SoftList`, `FreeAllSoftLockNodes`,
  `FlushAllSoftLockNodes`, `FLock`, `FUnLock`, `StrCmtCtl`, `EndCmtCtl`,
  `Commit`, `Rollback`
- **Views**: `ViewOne`, `ViewMulti`, `ViewStruct`, `View2DB`, `ViewRename`,
  `IsView`, `TableHasView`
- **Outros clientes**: `tLockClient`, `tNumberClient`, `tManagerClient`,
  `tAuditClient`, `tAuditMonitorClient` (cada um `tBasicClient`-like,
  ctor `(short, unsigned char)`)
- **API C de gerência** (`TC_*` flat exports): `TC_ConnectManager`,
  `TC_DisconnectManager`, `TC_GetManager`, `TC_ManagerIsConnected`,
  `TC_ConnectLockClient`, `TC_LockClient*` (~30 fns), `TC_ConnectAudit`,
  `TC_AuditUser`, `TC_GetConnLic`, `TC_SetLicense`, `TC_ReadIniString`,
  `TC_WriteIniString`, `TC_DBApiSetLocation(void*, int&)`, etc.

Strings relevantes: `dbapi.ini` (lido de `.` — cwd), seções `GENERAL`,
`ClientLog`; log `SET_DBAPI_LOCATION`; fontes Bamboo
`TPGW-OF2411X-TOTVSDBAPILINUXX64` (client/, lib_common/, lib_base/).

### O que NÃO está resolvido (objetivo da Fase 0)

1. Semântica exata do ctor `tTOPClient(short, unsigned char)` e de
   `tBasicClient(short, unsigned char)` — provável `(porta, flags)` ou
   `(tipo, flags)`; confirmar via decompilação.
2. Como o cliente informa **host:porta do server** e **DBEnv**
   (`POSTGRES/P1212510`): candidatos `dbapi.ini` `[GENERAL]`,
   `TC_DBApiSetLocation`, `SetLocation(int)`, env vars (`DBACCESS`,
   `SERVERCOM`), ou parâmetros de `SetParam/AddConfig/SendParams`.
3. Se o cliente precisa de licença própria ou herda a do server
   (server roda `TOP_NO_LICENSE` — bom sinal).
4. Versão/ABI: build do container (mai/2026) vs. cópias GhidraProjects
   (jan/2026) — API aparentemente estável, mas fixar versão do .so no bridge.

## Arquitetura proposta

```
AdvPL (TCLink, TCSqlExec, DBUseArea, DbConnection, RDD)
   │  natives TC* existentes em pkg/vm/dbaccess_native.go
   ▼
DbAccessEngine (novo backend em pkg/db)
   │  implementa DBEngine/SQLEngine (mesma interface do RemoteSQLEngine)
   │  protocolo interno: TCP localhost (JSON ou protobuf) — processo separado
   ▼
advpp-dbapi-bridge (binário C++ host)
   │  dlopen("dbapi.so"); instancia tTOPClient por conexão lógica
   │  mapeia chamadas SQL/workarea → métodos tTOPClient
   ▼
dbaccess64 (server TOTVS real, licenciado pelo cliente)
   │  dialeto nativo + ODBC/OCI
   ▼
PostgreSQL / Oracle / MSSQL  ← banco do Protheus do cliente
```

**Decisões já tomadas (operador):**
- Bridge = **processo separado + socket** (isolamento de crashes/ABI,
  licenciamento limpo, reutilizável por ferramentas externas/GEBAN).
- Primeira fatia funcional = **decidir após a prova de vida (Fase 0)**.

**Por que não cgo in-process:** exceção C++/crash de ABI derruba a VM;
licença e .so TOTVS acoplados ao processo; bridge separado também serve o
cenário GEBAN depois.

## Fases

- **Fase 0 — prova de vida** (atual): RE da sequência init/connect +
  hello-world C++ que conecta em `localhost:7890` e executa query.
  Critério de saída: `SELECT` real retornando rows via `tTOPClient`.
- **Fase 1 — read path**: bridge expõe connect/query/close; VM roteia
  `TCSqlExec`/`TCGenQry`/`TCSqlQuery` para o bridge quando configurado
  (`DBSetDriver("DBACCESS")` ou `DbConnection` apontando para bridge).
- **Fase 2 — workarea parity**: `OpenFile`/`Seek`/`GetField`/índices via
  `tTOPClient` → `DBUseArea`/RDD. Aqui mora a "máxima compatibilidade".
- **Fase 3 — escrita**: `Write/Update/Write2`, `Commit/Rollback`,
  locks (`Lock/SoftLock`), stored procedures (`SPExec`), views.
- **Fase 4 — empacotamento**: bridge como binário companion do advplc,
  versionamento de dbapi.so, docs de deploy no cliente.

## Riscos e ressalvas honestas

1. **Licença**: `dbapi.so` contém `TC_SetLicense`; appserver real pode
   exigir licenciamento que nosso host C++ não replica. Fase 0 mede isso.
   Server em `TOP_NO_LICENSE` sugere que builds STANDALONE não exigem.
2. **ABI C++**: chamamos métodos de classe via ponteiros para mangled
   names — frágil entre builds TOTVS. Mitigação: pinar versão do dbapi.so
   por bridge; alternativa futura = RE do protocolo TCP e cliente Go nativo
   (clean-room), se ABI se mostrar intratável.
3. **Paridade ≠ regra de negócio**: dbAccess dá acesso a dados com
   dialeto traduzido, mas SX7 triggers, GETMV, campos criptografados,
   `D_E_L_E_T_` semântico e índice SIX continuam sendo responsabilidade
   do RPO/appserver — fora do escopo do bridge.
4. **Segurança**: dbapi.ini pode conter credenciais; bridge deve manter
   segredos fora do código, seguindo as regras do projeto.
5. **Escopo GEBAN**: migração Protheus→GEBAN provavelmente se beneficia
   mais de SQL direto por provider do que do dbAccess — manter como
   caso de uso separado a desenhar depois.

## Não-objetivos (YAGNI)

- Reimplementar o protocolo TCP em Go (só se Fase 0 falhar por ABI).
- Suportar dbAccess Windows (dbapi.dll) nesta fase.
- Substituir o `RemoteSQLEngine` atual — ele continua como fallback
  sem licença/dbAccess.

## Fase 0 — resultados da prova de vida (2026-11-25) ✅

**Veredito: dbapi.so funciona standalone, fora do appsrvlinux, sem
ambiente Protheus.** Conectou no `dbaccess64` real (container
`feliperaposo/protheus-dbaccess`, build TOTVS 24.1.1.3 de mai/2026) e
leu dados reais do dicionário SX2 via query.

### Sequência de init validada (PoC C, dlopen)

1. `dlopen("dbapi.so", RTLD_NOW|RTLD_GLOBAL)` — deps apenas
   libc/libstdc++/libpthread/libdl/librt (OpenSSL/ICU estáticos embutidos;
   ODBC/OCI8 carregados pelo server, não pelo client).
2. `v40TC_GetTopConn(int type=0, short port=7890)` → aloca e constrói
   `tTOPClient` (0x218 bytes) via `_topmalloc`.
3. `tTOPClient::Connect(server, environment, user, port, locktype)` —
   mangled `_ZN10tTOPClient7ConnectEPKcPcS2_si`.
   **`environment` deve ser `"<DRIVER>/<env>"`**, ex.:
   `"POSTGRES/protheus"`, `"POSTGRES/p1212510"` — formato do
   `dbaccess.ini` do server (`[POSTGRES/protheus]`). Bare `"protheus"`
   retorna erro 65501.
4. `tTOPClient::Query(sql, alias)` → retorna workarea id (short);
   `GoTop(wa)` posiciona no primeiro registro real (recno 1);
   `GetColCount(wa)`; `GetFieldN(wa, fld)` **1-based**;
   `GetFieldDesc(wa, fld,...)` **0-based**; `Skip(wa,n)`;
   `Eof/Bof/RecNo/RecCount`; `GetField(wa,"NOME")` por nome;
   `GetError(buf,len)` para diagnóstico.
5. `v40TC_Release(ptr)` para destruir.

### Workarea completa confirmada (métodos exportados)

Navegação: `GoTop/GoBottom/GoTo/Skip/Seek/Seek2`; descrição:
`GetFieldDesc/GetFieldDesc2/GetColCount/GetKeys/GetKeyNames`;
escrita: `SetField/SetFieldN/Write/Write2/Update/DeleteR/RecallR/Pack`;
índices: `SetKey/SetKeyC/CreateIndex/CreateVIndex/DeleteIndex`;
locks/transações: `Lock/UnLock/MultLock/SoftLock/FLock/FUnLock/Commit/
Rollback/StrCmtCtl`; SQL: `SQLExec/SQLExec2/SqlPrepare/SqlBind/SqlFree/
SPExec/SPExist/ExecQueryToArray/GetStruct/ViewOne/ViewMulti/View2DB`;
meta: `Ping/GetInfo/GetInfo2/GetDBSID/GetDrivers/AddConfig/SetParam/
Config/SendConfig/SetLocation/APICache/Checkup/CanOpen/IsView`.

Clientes auxiliares exportados: `tLockClient`, `tNumberClient`,
`tManagerClient`, `tAuditClient`, `tAuditMonitorClient`,
`tBasicClient::IsConnected`.

### Observações operacionais

- **Licença**: o server usa `LicenseServer=license-server:5555`;
  o client não exigiu licença própria nesta prova — handshake e query
  passaram. Validar depois se há limite de conexões/recursos.
- **Erros**: SQL inválido (`ROWNUM` em Postgres) faz o server lançar
  `tAssertException` que **derruba o processo host** (exceção C++
  não capturada atravessa dlopen). O bridge DEVE rodar cada chamada
  dentro de processo isolado ou handler de exceção dedicado —
  reforça a decisão "processo separado".
- **Row fantasma**: `Query` abre workarea posicionada em recno=0
  (vazia); `GoTop` move para recno=1 = primeiro dado. Iteração
  correta: `GoTop` → loop `!Eof` → `Skip(1)`.
- **Campos char** vêm com padding de tamanho fixo; numéricos tipo `F`
  podem precisar de tratamento de escala (dec=15 em `SELECT 1`).

### Artefatos

- `.scratch/dbaccess-poc/poc5.c` — PoC mínimo funcional
  (connect + query + navegação + leitura de campos).
- `.scratch/dbaccess-poc/dbapi.so` — build 24.1.1.3 (mai/2026),
  não-stripado (~18k símbolos).
- `.scratch/dbaccess-poc/dbaccess64-server` — server standalone
  extraído do container para referência.

### Próxima decisão (aguardando operador)

Com a prova de vida confirmada, candidatas à primeira fatia:
- **A. Read path**: bridge socket mínimo (connect/query/fetch) +
  `DbAccessEngine` no AdvPP roteando `TCSqlExec`/`TCGenQry`.
- **B. Workarea path**: `OpenFile`/`Seek`/`GetField` → `DBUseArea` —
  maior compatibilidade, mas bem maior.
- **C. Bridge genérico primeiro**: definir protocolo socket +
  `advpp-dbapi-bridge` com subconjunto SQL, depois plugar na VM.

## Implementação — fatia C→A entregue (2026-11-25) ✅

Primeira fatia implementada conforme decidido: **bridge genérico +
read path na VM**. E2E validado de ponta a ponta.

### Componentes

- **`tools/dbapi-bridge/bridge.cpp`** — processo C++ que dlopen
  `dbapi.so` e serve protocolo binário em `127.0.0.1:17890`
  (`ADVPP_DBAPI_PORT`/`ADVPP_DBAPI_SO` configuráveis). Thread por
  cliente TCP, mutex global serializando chamadas dbapi, `try/catch`
  por request (exceção C++ vira resposta ERR — provado: SQL inválido
  não derruba o bridge). Build: `tools/dbapi-bridge/build.sh`.
- **`pkg/db/dbapi_bridge.go`** — cliente do protocolo +
  `DbapiSQLEngine` (DBEngine+SQLEngine por duck typing), mesmo modelo
  em-memória do `RemoteSQLEngine`. Driver `"DBACCESS"` mapeado em
  `ConnConfig`: Host/Port=dbaccess64, Service=environment
  `"DRIVER/env"`, User=usuário, locktype via `ADVPP_DBAPI_LOCKTYPE`.
  Endereço do bridge via `ADVPP_DBAPI_BRIDGE` (default
  `127.0.0.1:17890`).
- **`pkg/vm/dbconnection_native.go`** — `DbConnection:Connect` roteia
  driver `DBACCESS` para `NewDbapiEngineFrom`.

### Protocolo (resumo)

frame `[u32 len][payload]`; request = op + fields length-prefix;
resposta terminal status OK/ERR; push frames COLS(0x83)/ROW(0x82)/
DONE(0x84). Ops: CONNECT/QUERY/FETCH/EXEC/CLOSEWA/DISCONNECT/PING.
Campos NULL = len 0xFFFFFFFF. `GetFieldDesc` 0-based, `GetFieldN`
1-based — assimetria absorvida no bridge.

### Evidência E2E (ambiente real)

`advplc run tests/dbaccess_bridge_test.prw` contra dbaccess64
(container) + Postgres `p1212510`:

```
connect=true          DbConnection:New("DBACCESS","127.0.0.1",7890,"POSTGRES/p1212510")
rows=3                TCSQLQUERY("SELECT X2_CHAVE,X2_NOME FROM SX2990 LIMIT 3")
reccount=10436        DBSetDriver("TOPCONN") + DBUseArea(.T.,"TOPCONN","SX2990")
recno=1 / nome=Itens de Venda da NF   navegação + leitura SX2990->X2_NOME
baderr=excecao interna dbapi         erro SQL tratável via TCSqlError()
fim=true
```

Testes: `TestDbapiBridgeCodec` + `TestDbapiSQLEngineIntegration`
(pkg/db, gated `ADVPP_DBAPI_IT=1`) e `TestDbAccessBridgeFixture`
(cmd/advplc, idem). `go test ./pkg/db ./pkg/vm` verde.

### Fase 2 parcial — workarea real (entregue)

`DbapiSQLEngine.SelectArea` agora tenta `WACMD open` (OpenFile remoto)
antes do fallback `SELECT *` em memória. Navegação (`GoTop`/`GoBottom`/
`GoTo`/`Skip`/`EOF`/`BOF`/`RecNo`), `FieldGet` (`fieldn` 1-based) e
`RecCount` (`COUNT(*)` cacheado — o RecCount ISAM do dbapi devolve
sentinela 999930006) operam sobre a workarea remota sem materializar
linhas. `Seek` usa comparação sequencial campo-a-campo (O(n)
roundtrips — ponytail); índices/`SetKey` ficam para depois.

**Decodificação de tipos (achado crítico)**: `GetFieldN` devolve
`char*` para dentro do record buffer. Para tipo `C` é string que
corre até o fim do registro (fatiar por `size` do fielddesc). Para
tipo `F` o ponteiro aponta um **double IEEE-754 little-endian de 8
bytes** — não é string; o bridge decodifica para texto numérico.
Demais tipos (`D`,`L`,`N`,`M`,`B`...) ainda não verificados (base de
teste só tem SX2990/SX6990, sem datas/logicals): o bridge devolve hex
dos bytes crus — `ponytail:` revisitar com base real. No Go,
`dbapiFieldSliceT` só fatia tipos texto; `F`/hex passam inteiros.

### Fase 3 parcial — escrita (entregue)

WACMD ganhou comandos de escrita/locks: `setfieldn`, `setfield`,
`update`, `write`, `append`, `delete`, `recall`, `commit`, `rollback`,
`lock`, `unlock` — mapeados de `tTOPClient::SetFieldN/SetField/Update/
Write/DeleteR/RecallR/Commit/Rollback/Lock/UnLock`. Na engine:

- `FieldPut` (workarea remota) → `setfieldn` no buffer do record;
- `RecLock` → `lock` no recno corrente; `MsUnlock` → `update` (ou
  `write` se veio de `Append`) + `unlock` — gravação real no SGBD;
- `Append` → posiciona além do fim (GoBottom+Skip) e marca
  `waAppending`; `MsUnlock` faz `Write`.

**Cuidados empiricamente verificados**: `SetFieldN` NÃO faz
padding/truncate — o bridge normaliza campos `C`/`M` para exatamente
`size` bytes (resto do valor antigo ficava no buffer). `SetFieldN`
retorna tamanho escrito, não 0=sucesso. Recno de append vem do espaço
de sentinela (999930007+ com `UseLargeRecno=1`). Update+restore e
append+write+delete validados em SX2990 com commit.

### Limitações desta fatia (próximas)

- **Seek**: duas rotas — `seek2` indexado numa **workarea auxiliar**
  (SetKey muda a ordem de navegação da wa; a auxiliar tem índice 1
  ativo, devolve recno, wa principal faz GoTo+verificação) e fallback
  `find` (scan sequencial **server-side** no bridge — sem o loop
  Go de fieldn-por-campo, um miss em 10k registros leva segundos em
  vez de minutos). Seek2 é soft seek: rc=0 só posiciona; a engine
  verifica o registro corrente antes de declarar match.
- **Índices**: `keys`/`setkey`/`setkeyc`/`pack` expostos; engine usa
  sempre o índice 1 (não há SET ORDER por campo — ponytail).
- **Transação explícita** (entregue): semântica real mapeada e
  verificada — fora de transação, Update/Write fazem **autocommit**;
  `StrCmtCtl` abre escopo transacional (writes ficam pendentes,
  invisíveis fora da sessão), `Commit` persiste o pendente,
  `Rollback` descarta, `EndCmtCtl` fecha o escopo. `TCCommit` agora
  despacha por duck-typing: 1→`BeginTx`(strcmtctl, só na profundidade
  0), 2→`CommitTx`, 3→`RollbackTx`, 4→`EndTx`; engines sem suporte
  mantêm o no-op anterior. Validado no IT: rollback descartou e
  commit persistiu (verificação via SQL server-side — o buffer da
  workarea não é invalidado por rollback).
- **Locks**: `lock`/`unlock` por recno; sem `FLock`/`MultLock`/`SoftLock`.
- **Delete/Recall**: comandos no bridge, mas a engine DBEngine não tem
  Delete nativo — uso direto via WaCmd hoje.
- **FieldPut de tipos não-C**: `F` recebe texto numérico (funciona);
  `D` verificado de ponta a ponta — leitura: 8 bytes ASCII "YYYYMMDD"
  no record buffer → `DateValue`; escrita: `DateValue` → "YYYYMMDD",
  data zero → espaços. `L`/`N`/`M`/`B` seguem sem verificação
  empírica: nesta base não há tabela populada com esses tipos (L só
  existe em tabelas custom vazias; `N` do dicionário chega como `F`
  no dbapi; `M` memo retorna vazio/inconsistente). ponytail: escrever
  `L`/`M` hoje sai `Value.String()` — provavelmente errado.
- **OpenFile naming — RESOLVIDO**: o "arquivo" é o nome físico
  (`SE1990`, `SA1990`, `SX2990`), não o alias (`SE1` retorna rc 65517).
  O dbapi **não** resolve alias — quem resolve é o appserver via SX2.
  `DbapiSQLEngine.resolveSQLName` consulta `SX2.X2_ARQUIVO`
  (candidatos `sx2`/`sx2990`/`sx2x3199`, cacheado por alias) e guarda o
  resultado em `e.fileName`, usado por `openRemoteWA`, `waSeekIndex`
  (workarea auxiliar do Seek2) e `RecCount` (`COUNT(*)`). O alias AdvPL
  (`e.alias`) permanece intacto para FieldGet/Navegação. `RetSqlName`
  nativo da VM delega ao engine via duck-typing
  (`interface{ RetSqlName(string) string }`) — `RetSqlName("SE1")` →
  `"SE1990"`, então `%table:SE1%`/SQL embutido também funcionam.
  Validado E2E: `DBUseArea("SE1")` abre SE1990 (RecCount=102,
  `E1_EMISSAO` tipo D → `23/12/2023`).
- **QueryRows/Exec sem args** — placeholders não expandidos (o
  dbapi não os aceita via SQLExec simples); usar literais.
- **SelectArea fallback** carrega tudo em memória quando `OpenFile`
  falha (ex.: alias sem tabela física); quando workarea abre, não
  materializa — mas `SelectArea` em si ainda pré-carrega `WHERE` via
  SELECT para filtros que OpenFile não expressa (ponytail).
- **Concorrência**: mutex por connID no bridge — cada `tTOPClient`
  só é tocado pela thread dona do connID; CONNECT segue no mutex
  global (init compartilhado). Validado com 3 conexões × 20 queries
  concorrentes. ponytail: mutexes de conns mortas não são removidos.
- **Licença**: `LicenseServer=license-server`, `LicensePort=5555` no
  dbaccess.ini — o container `license-server` (totvslicensevirtual)
  precisa estar de pé; se cair, connect retorna "No licenses
  available" até o dbaccess reiniciar. **Cada CONNECT consome uma
  seat de licença** — a licença de teste tem poucas seats; socket
  fechado sem DISCONNECT vazava a seat até timeout do server.
  Mitigado: o bridge agora libera automaticamente as conexões de um
  fd quando o socket morre (`g_fdConns` cleanup no clientThread).
- **Versão TOTVS**: ABI validada no build 24.1.1.3 (mai/2026);
  pinning de dbapi.so por release do bridge.

### Empacotamento (parcial — auto-spawn)

`dialDbapiBridge` agora tenta subir o bridge sozinho quando o dial
falha (só em endereço loopback): procura `advpp-dbapi-bridge` em
`ADVPP_DBAPI_BRIDGE_BIN` → mesmo dir do executável → PATH, e
`dbapi.so` em `ADVPP_DBAPI_SO` → mesmo dir do bridge/executável →
`/usr/lib` → `/usr/local/lib` → `/opt/dbapi`. `make dbapi-bridge`
compila o C++ e copia o binário para a raiz (ao lado do `advplc`).
O `dbapi.so` é proprietário TOTVS — não pode ser redistribuído; o
usuário aponta via env ou copia para o mesmo diretório.

Cenário zero-config validado: `advplc` + `advpp-dbapi-bridge` +
`dbapi.so` no mesmo dir → `DbConnection("DBACCESS",...)` funciona sem
nenhuma env extra. Corridas de spawn entre processos são benignas
(perdedor falha no bind).
