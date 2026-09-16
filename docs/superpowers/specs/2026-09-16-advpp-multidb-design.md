# AdvPP — Conectividade Real Multi-Provider (PostgreSQL/Oracle/MSSQL) (Design)

## Objetivo

Hoje `TCLINK`/`TCUNLINK`/`TCSQLEXEC`/`TCGENQRY` (e toda a família TC*) e
`DBUseArea`/RDD aceitam um nome de driver (`"ORACLE/..."`, `"MSSQL/..."`,
`DBSetDriver("TOPCONN")`) mas **sempre** abrem um `SQLiteEngine` local por
baixo — o nome do driver fica só armazenado como string em `dbstateConn`
(`pkg/vm/dbaccess_native.go:78`), nunca usado para conectar de fato. Não
existe hoje nenhuma conexão de rede real com PostgreSQL, Oracle ou SQL
Server.

O objetivo deste trabalho é fazer com que programas AdvPL compilados pelo
AdvPP consigam se conectar de verdade a servidores externos PostgreSQL,
Oracle e SQL Server (host/porta/usuário/senha reais, rede real), tanto para
SQL direto (TCSQLEXEC/TCGENQRY/TCQuery) quanto para acesso a tabelas via
RDD (DBUseArea/DBSkip/RecLock/... roteado para a tabela física remota, como
`DBSetDriver("TOPCONN")` faz no Protheus real). SQLite continua sendo o
motor local/default — nada muda no caminho que já funciona hoje.

## Estado atual (levantado em 2026-09-16)

- `dbstateConn` (`pkg/vm/dbaccess_native.go:72-90`) guarda `driver` como
  string livre, mas `TCLINK` (linha 395) sempre chama
  `dbaccessOpenEngine(id)` — SQLite local, ignorando o driver declarado.
- `TCSQLEXEC`/`TCGENQRY`/demais TC* já operam sobre `dbstateConn.sqlEng`
  (`SQLEngine`), então uma vez que a conexão carregue um driver real, essas
  natives passam a funcionar contra o backend real sem mudança de
  assinatura.
- `DBEngine` (`pkg/vm/vm.go:171`) é a interface de navegação de área
  (SelectArea/Seek/Skip/GoTop/GoBottom/FieldGet/FieldPut/RecLock/
  MsUnlock/RecCount/RecNo/Append/FieldPos) — pequena, já desenhada para
  múltiplas implementações ("SQLite, in-memory ou motores nativos
  Protheus", conforme o comentário na própria interface).
- Hoje `v.dbEngine` é **um único** engine por VM (sempre SQLite), setado
  uma vez. `DBUseArea` (`pkg/vm/dbgenericas_native.go:682`) só chama
  `v.dbEngine.SelectArea(cFile)` — não escolhe engine por alias.
- `SQLEngine` (`pkg/vm/browse.go:23`) é a interface mínima já usada pelo
  browse para SQL direto: `QueryRows(query, args...) ([]map[string]string,
  error)` e `Exec(query, args...) error`. Essa mesma interface serve de
  contrato para os novos drivers remotos.
- `validRDD()` (`pkg/vm/dbgenericas_native.go:1684`) já reconhece nomes de
  RDD reais do Protheus (inclui `TOPCONN`), mas não há comportamento
  diferente associado a cada um — é só validação de string.
- Premissa absoluta do projeto (memória compartilhada): nenhuma mudança
  pode quebrar `go build`/`go vet`/`go test` cross-platform
  (linux/windows/darwin), e dependências novas devem ser 100%
  multiplataforma. Isso descarta qualquer driver que exija CGO ou
  bibliotecas cliente nativas (ex.: Oracle Instant Client via `godror`).

## Arquitetura

### Camada de drivers (`pkg/db/`)

Novo arquivo por provider, cada um implementando o mesmo par de interfaces
já usado pelo SQLite (`SQLEngine` para SQL direto; um novo `DBEngine` para
RDD — ver seção seguinte), sobre `database/sql` com drivers 100% Go, sem
CGO:

| Provider | Driver Go | Pacote |
|---|---|---|
| PostgreSQL | `pgx` (modo stdlib) | `github.com/jackc/pgx/v5/stdlib` |
| Oracle | `go-ora` (puro Go, sem Oracle Instant Client) | `github.com/sijms/go-ora/v2` |
| SQL Server | `go-mssqldb` | `github.com/microsoft/go-mssqldb` |
| SQLite | inalterado | `pkg/db/sqlite.go` (existente) |

Cada driver expõe uma função `Open(cfg ConnConfig) (*sql.DB, error)` onde
`ConnConfig` é um struct simples: `Host, Port, User, Password, Service`
(SID/database name conforme o provider). Nenhuma lógica de retry/pool
própria — `database/sql` já faz pooling; usamos o padrão da lib.

### API AdvPL: classe `DbConnection`

Nova classe intrínseca (não substitui `TCLINK`, que continua igual para o
caso local/SQLite):

```advpl
oConn := DbConnection():New("POSTGRES", "10.0.0.5", 5432, "meubanco", "usuario", cSenhaVindaDeGetEnv)
If oConn:Connect()
    // conexão real ativa; TCSQLEXEC/TCGENQRY/DBUseArea passam a usá-la
Else
    ConOut(oConn:GetError())
EndIf
oConn:Close()
```

`DbConnection:Connect()` registra um `dbstateConn` real (mesmo struct de
`dbaccess_native.go`, campo novo `driver db.SQLDriver` no lugar do sempre-
SQLite) e o marca como conexão ativa da sessão — mesmo papel que `TCLINK`
já cumpre hoje, só que com um backend de verdade. A senha nunca é
armazenada em lugar hardcoded pelo AdvPP: é responsabilidade de quem chama
`New()` obtê-la de `GetEnv()`, um cofre, ou parâmetro externo — igual a
qualquer outro secret hoje no projeto.

### SQL direto (TCSQLEXEC/TCGENQRY/TCQuery)

Sem mudança de assinatura. Essas natives já leem `dbstateConn.sqlEng`; uma
vez que `sqlEng` seja a implementação real (Postgres/Oracle/MSSQL) em vez
do `SQLiteEngine`, os resultados passam a vir do banco externo de verdade.

### RDD / acesso por tabela (DBUseArea, TOPCONN-style)

Esta é a parte mais ampla do trabalho:

1. `v.dbEngine` (hoje um único campo, sempre SQLite) passa a ser
   `map[string]DBEngine` por alias, com fallback para o engine SQLite
   padrão quando o alias não tem entrada — nenhum código existente muda de
   comportamento.
2. Cada driver remoto (`pkg/db/postgres.go` etc.) ganha uma segunda
   implementação, `RemoteSQLEngine`, satisfazendo `DBEngine`:
   `SelectArea` vira `SELECT * FROM <tabela>` com paginação por cursor;
   `Skip`/`GoTop`/`GoBottom` avançam o cursor; `Seek` usa o índice
   declarado (SIX/campo) como `WHERE`; `RecLock` vira `SELECT ... FOR
   UPDATE` (Postgres/Oracle) ou `UPDLOCK` (MSSQL); `MsUnlock` faz
   commit/release; `FieldGet`/`FieldPut` leem/escrevem a linha corrente em
   memória, `Append` faz `INSERT`. Metadados de campo vêm do dicionário
   SX3 já usado pelo browse — nenhum sistema de metadados novo.
3. `DBUseArea` (`dbgenericas_native.go:682`) passa a decidir qual engine
   usar para o alias sendo aberto: se `DBSetDriver`/RDD ativa for
   `TOPCONN` (ou outro nome de RDD remota reconhecido) **e** houver uma
   `DbConnection` ativa, constrói um `RemoteSQLEngine` sobre ela; caso
   contrário, comportamento idêntico ao atual (SQLite).

### Erros

`dbstate.sqlErr` / `TCSQLError()` / `NetErr()` (já existentes, hoje sempre
vazios no caminho simulado) passam a ser populados com o erro real
devolvido pelo `database/sql` do driver ativo — nenhuma API nova de erro.

## Fora de escopo

- Otimizações específicas de provider (ex.: `ROWID` do Oracle, `OUTPUT`
  do MSSQL) — usar o SQL padrão suficiente para CRUD via RDD.
- Pool de conexões próprio — usar o pool default de `database/sql`.
- Suporte a outros bancos (MySQL, DB2, etc.) — os 4 providers listados são
  o escopo completo pedido.
- Migração de dados entre engines — fora do pedido original.

## Testes

- Testes unitários da camada de driver continuam rodando contra SQLite
  (padrão atual), sem infraestrutura externa.
- Um arquivo `//go:build integration` por provider
  (`pkg/db/postgres_integration_test.go` etc.), fora do `go test ./...`
  padrão e fora do CI automático — roda só sob `go test -tags=integration`
  contra um container Docker local levantado manualmente. Containers
  Oracle/MSSQL são pesados e com licenciamento restrito demais para rodar
  em toda execução de CI.
  <!-- ponytail: cobertura de integração fica opt-in; promover pra CI
  automatizado só se surgir flakiness real de campo que justifique o
  custo do container em toda run. -->
- `go build`/`go vet` para `GOOS=linux`, `GOOS=windows`, `GOOS=darwin`
  continuam sendo o gate de compatibilidade (premissa absoluta do
  projeto) — os 3 drivers escolhidos são puro Go, sem CGO, então isso não
  deveria quebrar, mas é o primeiro check antes de qualquer merge.

## Riscos conhecidos

- `go-ora` (Oracle) é uma implementação de terceiros do protocolo TNS, não
  a lib oficial da Oracle — pode não cobrir 100% dos recursos de um client
  oficial (ex.: alguns tipos de dado exóticos, RAC avançado). Suficiente
  para CRUD padrão via RDD e SQL direto, que é o escopo aqui.
- Mudar `v.dbEngine` de campo único para mapa por alias toca código que já
  assume um único engine por VM — precisa de varredura cuidadosa em todo
  `pkg/vm/*.go` que lê `v.dbEngine` diretamente (não só via `DBUseArea`)
  antes de considerar o Phase de RDD concluído.
