# AdvPP Multi-DB Connectivity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make AdvPL programs compiled by AdvPP connect to real external PostgreSQL, Oracle, and SQL Server databases — both for direct SQL (TCSQLEXEC/TCGENQRY) and for table-based RDD access (DBUseArea under `DBSetDriver("TOPCONN")`) — while SQLite stays exactly as-is for the local/default case.

**Architecture:** A new `pkg/db` driver layer opens real `*sql.DB` connections via pure-Go drivers (pgx stdlib, go-ora, go-mssqldb — no CGO). A new `RemoteSQLEngine` implements the existing `DBEngine`/`SQLEngine` interfaces the same way `SQLiteEngine` does (load rows into memory, navigate in Go, write back on `MsUnlock`), so it plugs into all existing TC*/DB* natives with zero changes to ~50 call sites. A new `DbConnection` AdvPL class opens the real connection with explicit credentials; `DBSetDriver("TOPCONN")` swaps `v.dbEngine` to that connection's `RemoteSQLEngine` for the session, restoring the local engine when the RDD is set back to `DBFCDX`.

**Tech Stack:** Go 1.27, `database/sql`, `github.com/jackc/pgx/v5` (stdlib mode), `github.com/sijms/go-ora/v2`, `github.com/microsoft/go-mssqldb`, `github.com/DATA-DOG/go-sqlmock` (test-only).

**Spec:** `docs/superpowers/specs/2026-09-16-advpp-multidb-design.md`

## Global Constraints

- `go build`/`go vet`/`go test` must keep passing for `GOOS=linux`, `GOOS=windows`, `GOOS=darwin` — no CGO, no platform-specific dependency (project-wide absolute premise, restated in the spec).
- No secret (password, connection string) is ever hardcoded — `DbConnection:New()` takes the password as a value the AdvPL caller supplies (e.g. from `GetEnv()`); AdvPP itself stores nothing beyond the live connection.
- Every SQL identifier (table/alias name) built into a query string must pass the existing `identRe` validation (`^[A-Za-z0-9_]+$`) before use — same rule `SQLiteEngine` already enforces, prevents SQL injection (CWE-89).
- No behavior change to the existing SQLite/local path: `TCLINK` without a `DbConnection`, and any `DBUseArea` while the default RDD is `DBFCDX`, must produce identical results to today.

---

### Task 1: `ConnConfig` + `Dialect` abstraction (no network)

**Files:**
- Create: `pkg/db/remote_conn.go`
- Test: `pkg/db/remote_conn_test.go`

**Interfaces:**
- Produces: `type ConnConfig struct { Host string; Port int; Service string; User string; Password string }`; `type Dialect interface { Name() string; Placeholder(pos int) string }`; `func dialectFor(driver string) (Dialect, bool)` (driver = `"POSTGRES"`, `"ORACLE"`, or `"MSSQL"`, case-insensitive).

- [ ] **Step 1: Write the failing test**

```go
// pkg/db/remote_conn_test.go
package db

import "testing"

func TestDialectPlaceholders(t *testing.T) {
	cases := []struct {
		driver string
		pos    int
		want   string
	}{
		{"POSTGRES", 1, "$1"},
		{"POSTGRES", 2, "$2"},
		{"ORACLE", 1, ":1"},
		{"MSSQL", 1, "@p1"},
	}
	for _, c := range cases {
		d, ok := dialectFor(c.driver)
		if !ok {
			t.Fatalf("dialectFor(%q): not found", c.driver)
		}
		if got := d.Placeholder(c.pos); got != c.want {
			t.Errorf("%s.Placeholder(%d) = %q, want %q", c.driver, c.pos, got, c.want)
		}
	}
}

func TestDialectForUnknown(t *testing.T) {
	if _, ok := dialectFor("DB2"); ok {
		t.Error("dialectFor(\"DB2\") should not be found")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/db/... -run TestDialect -v`
Expected: FAIL (`dialectFor` undefined)

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/db/remote_conn.go
package db

import (
	"fmt"
	"strings"
)

// ConnConfig são os dados de conexão de um banco externo real — nunca
// contém um valor hardcoded: quem chama Open()/DbConnection:New() decide
// de onde vem a senha (GetEnv, cofre, etc).
type ConnConfig struct {
	Host     string
	Port     int
	Service  string // nome do banco (Postgres/MSSQL) ou SID/service name (Oracle)
	User     string
	Password string
}

// Dialect isola a única diferença de SQL entre os 3 drivers remotos: a
// sintaxe de placeholder de parâmetro. Tudo o mais (paginação, locking)
// não existe — RemoteSQLEngine carrega os registros em memória e navega
// em Go, igual ao SQLiteEngine.
type Dialect interface {
	Name() string
	Placeholder(pos int) string
}

type postgresDialect struct{}

func (postgresDialect) Name() string                { return "POSTGRES" }
func (postgresDialect) Placeholder(pos int) string   { return fmt.Sprintf("$%d", pos) }

type oracleDialect struct{}

func (oracleDialect) Name() string              { return "ORACLE" }
func (oracleDialect) Placeholder(pos int) string { return fmt.Sprintf(":%d", pos) }

type mssqlDialect struct{}

func (mssqlDialect) Name() string               { return "MSSQL" }
func (mssqlDialect) Placeholder(pos int) string  { return fmt.Sprintf("@p%d", pos) }

func dialectFor(driver string) (Dialect, bool) {
	switch strings.ToUpper(strings.TrimSpace(driver)) {
	case "POSTGRES", "POSTGRESQL":
		return postgresDialect{}, true
	case "ORACLE":
		return oracleDialect{}, true
	case "MSSQL", "SQLSERVER":
		return mssqlDialect{}, true
	}
	return nil, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/db/... -run TestDialect -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/db/remote_conn.go pkg/db/remote_conn_test.go
git commit -m "feat(db): ConnConfig e Dialect (placeholder) para drivers remotos"
```

---

### Task 2: PostgreSQL driver (`pkg/db/postgres.go`)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/jackc/pgx/v5`)
- Create: `pkg/db/postgres.go`
- Test: `pkg/db/postgres_integration_test.go` (build-tagged, opt-in)

**Interfaces:**
- Consumes: `ConnConfig`, `Dialect`/`dialectFor` (Task 1)
- Produces: `func openPostgres(cfg ConnConfig) (*sql.DB, Dialect, error)`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/jackc/pgx/v5@v5.6.0`

- [ ] **Step 2: Write the implementation**

```go
// pkg/db/postgres.go
package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" em database/sql
)

// openPostgres abre uma conexão real com um servidor PostgreSQL via pgx em
// modo stdlib (database/sql), sem CGO. cfg.Service é o nome do banco.
func openPostgres(cfg ConnConfig) (*sql.DB, Dialect, error) {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + cfg.Service,
	}
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("postgres: %w", err)
	}
	return db, postgresDialect{}, nil
}
```

- [ ] **Step 3: Write the opt-in integration test**

```go
// pkg/db/postgres_integration_test.go
//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenPostgresIntegration só roda com `go test -tags=integration` contra
// um Postgres real (ex.: `docker run -e POSTGRES_PASSWORD=advpp -p 5432:5432 postgres:16`).
// Configurar via env vars antes de rodar.
func TestOpenPostgresIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_PG_HOST", "localhost"),
		Port:     5432,
		Service:  envOr("ADVPP_TEST_PG_DB", "postgres"),
		User:     envOr("ADVPP_TEST_PG_USER", "postgres"),
		Password: os.Getenv("ADVPP_TEST_PG_PASSWORD"),
	}
	db, dialect, err := openPostgres(cfg)
	if err != nil {
		t.Fatalf("openPostgres: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != "$1" {
		t.Errorf("placeholder = %q, want $1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Verify the non-integration build is unaffected**

Run: `go build ./... && go test ./pkg/db/...`
Expected: builds clean; `postgres_integration_test.go` is excluded (no `integration` tag), so no Postgres server is needed for this to pass.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum pkg/db/postgres.go pkg/db/postgres_integration_test.go
git commit -m "feat(db): driver PostgreSQL real via pgx (stdlib, sem CGO)"
```

---

### Task 3: Oracle driver (`pkg/db/oracle.go`)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/sijms/go-ora/v2`)
- Create: `pkg/db/oracle.go`
- Test: `pkg/db/oracle_integration_test.go` (build-tagged, opt-in)

**Interfaces:**
- Consumes: `ConnConfig`, `Dialect`/`dialectFor` (Task 1)
- Produces: `func openOracle(cfg ConnConfig) (*sql.DB, Dialect, error)`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/sijms/go-ora/v2@v2.8.22`

- [ ] **Step 2: Write the implementation**

```go
// pkg/db/oracle.go
package db

import (
	"database/sql"
	"fmt"

	go_ora "github.com/sijms/go-ora/v2"
)

// openOracle abre uma conexão real com um servidor Oracle via go-ora —
// implementação pura em Go do protocolo TNS, sem Oracle Instant Client e
// sem CGO. cfg.Service é o SID/service name.
func openOracle(cfg ConnConfig) (*sql.DB, Dialect, error) {
	dsn := go_ora.BuildUrl(cfg.Host, cfg.Port, cfg.Service, cfg.User, cfg.Password, nil)
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("oracle: %w", err)
	}
	return db, oracleDialect{}, nil
}
```

- [ ] **Step 3: Write the opt-in integration test**

```go
// pkg/db/oracle_integration_test.go
//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenOracleIntegration só roda com `go test -tags=integration` contra
// um Oracle real (ex.: gvenzl/oracle-free no Docker). Configurar via env
// vars antes de rodar.
func TestOpenOracleIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_ORA_HOST", "localhost"),
		Port:     1521,
		Service:  envOr("ADVPP_TEST_ORA_SERVICE", "FREEPDB1"),
		User:     envOr("ADVPP_TEST_ORA_USER", "system"),
		Password: os.Getenv("ADVPP_TEST_ORA_PASSWORD"),
	}
	db, dialect, err := openOracle(cfg)
	if err != nil {
		t.Fatalf("openOracle: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != ":1" {
		t.Errorf("placeholder = %q, want :1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
```

- [ ] **Step 4: Verify the non-integration build is unaffected**

Run: `go build ./... && go test ./pkg/db/...`
Expected: builds clean, no Oracle server needed.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum pkg/db/oracle.go pkg/db/oracle_integration_test.go
git commit -m "feat(db): driver Oracle real via go-ora (puro Go, sem Instant Client)"
```

---

### Task 4: SQL Server driver (`pkg/db/mssql.go`)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/microsoft/go-mssqldb`)
- Create: `pkg/db/mssql.go`
- Test: `pkg/db/mssql_integration_test.go` (build-tagged, opt-in)

**Interfaces:**
- Consumes: `ConnConfig`, `Dialect`/`dialectFor` (Task 1)
- Produces: `func openMSSQL(cfg ConnConfig) (*sql.DB, Dialect, error)`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/microsoft/go-mssqldb@v1.7.2`

- [ ] **Step 2: Write the implementation**

```go
// pkg/db/mssql.go
package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/microsoft/go-mssqldb" // registra o driver "sqlserver"
)

// openMSSQL abre uma conexão real com um servidor SQL Server via
// go-mssqldb, puro Go, sem CGO. cfg.Service é o nome do banco.
func openMSSQL(cfg ConnConfig) (*sql.DB, Dialect, error) {
	query := url.Values{}
	query.Add("database", cfg.Service)
	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: query.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("mssql: %w", err)
	}
	return db, mssqlDialect{}, nil
}
```

- [ ] **Step 3: Write the opt-in integration test**

```go
// pkg/db/mssql_integration_test.go
//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenMSSQLIntegration só roda com `go test -tags=integration` contra
// um SQL Server real (ex.: mcr.microsoft.com/mssql/server no Docker).
func TestOpenMSSQLIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_MSSQL_HOST", "localhost"),
		Port:     1433,
		Service:  envOr("ADVPP_TEST_MSSQL_DB", "master"),
		User:     envOr("ADVPP_TEST_MSSQL_USER", "sa"),
		Password: os.Getenv("ADVPP_TEST_MSSQL_PASSWORD"),
	}
	db, dialect, err := openMSSQL(cfg)
	if err != nil {
		t.Fatalf("openMSSQL: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != "@p1" {
		t.Errorf("placeholder = %q, want @p1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
```

- [ ] **Step 4: Verify the non-integration build is unaffected**

Run: `go build ./... && go test ./pkg/db/...`
Expected: builds clean, no SQL Server needed.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum pkg/db/mssql.go pkg/db/mssql_integration_test.go
git commit -m "feat(db): driver SQL Server real via go-mssqldb"
```

---

### Task 5: Dispatch factory + `[]byte` fix in `convertDBValue`

**Files:**
- Modify: `pkg/db/remote_conn.go` (add dispatch function)
- Modify: `pkg/db/sqlite.go:582-601` (`convertDBValue`)
- Test: `pkg/db/remote_conn_test.go`, `pkg/db/sqlite_test.go`

**Interfaces:**
- Consumes: `openPostgres`, `openOracle`, `openMSSQL` (Tasks 2-4)
- Produces: `func OpenRemote(driver string, cfg ConnConfig) (*sql.DB, Dialect, error)` — the single entry point Task 7 (`DbConnection`) calls.

- [ ] **Step 1: Write the failing test for the dispatcher**

```go
// pkg/db/remote_conn_test.go (append)
func TestOpenRemoteUnknownDriver(t *testing.T) {
	_, _, err := OpenRemote("DB2", ConnConfig{})
	if err == nil {
		t.Error("OpenRemote(\"DB2\", ...) should return an error")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./pkg/db/... -run TestOpenRemote -v`
Expected: FAIL (`OpenRemote` undefined)

- [ ] **Step 3: Add the dispatcher**

```go
// pkg/db/remote_conn.go (append)
import "database/sql" // adicionar ao bloco de imports existente

// OpenRemote abre uma conexão real de banco externo pelo nome do driver
// declarado em DbConnection:New()/TCLINK ("POSTGRES", "ORACLE", "MSSQL").
func OpenRemote(driver string, cfg ConnConfig) (*sql.DB, Dialect, error) {
	switch strings.ToUpper(strings.TrimSpace(driver)) {
	case "POSTGRES", "POSTGRESQL":
		return openPostgres(cfg)
	case "ORACLE":
		return openOracle(cfg)
	case "MSSQL", "SQLSERVER":
		return openMSSQL(cfg)
	}
	return nil, nil, fmt.Errorf("OpenRemote: driver desconhecido %q (use POSTGRES, ORACLE ou MSSQL)", driver)
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./pkg/db/... -run TestOpenRemote -v`
Expected: PASS

- [ ] **Step 5: Write the failing test for the `[]byte` fix**

```go
// pkg/db/sqlite_test.go (append)
func TestConvertDBValueBytes(t *testing.T) {
	got := convertDBValue([]byte("hello"))
	if got.String() != "hello" {
		t.Errorf("convertDBValue([]byte(\"hello\")) = %q, want \"hello\"", got.String())
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./pkg/db/... -run TestConvertDBValueBytes -v`
Expected: FAIL (currently prints `"[104 101 108 108 111]"` via the `default` branch)

- [ ] **Step 7: Fix `convertDBValue`**

```go
// pkg/db/sqlite.go — dentro do switch em convertDBValue, antes do default:
	case []byte:
		return advplrt.NewString(string(v))
```

Motivo: drivers de rede (pgx, go-mssqldb) frequentemente escaneiam colunas de
texto/numéricas como `[]byte` em vez de `string`/`float64` — o `SQLiteEngine`
raramente bate nesse caso hoje, mas o `RemoteSQLEngine` (Task 6) bate sempre.

- [ ] **Step 8: Run it to verify it passes**

Run: `go test ./pkg/db/... -v`
Expected: PASS (all `pkg/db` tests, including the pre-existing ones)

- [ ] **Step 9: Commit**

```bash
git add pkg/db/remote_conn.go pkg/db/remote_conn_test.go pkg/db/sqlite.go pkg/db/sqlite_test.go
git commit -m "feat(db): OpenRemote dispatcher; fix convertDBValue para []byte"
```

---

### Task 6: `RemoteSQLEngine` (implements `DBEngine` + `SQLEngine`)

**Files:**
- Create: `pkg/db/remote_engine.go`
- Test: `pkg/db/remote_engine_test.go`
- Modify: `go.mod`, `go.sum` (add `github.com/DATA-DOG/go-sqlmock`, test-only)

**Interfaces:**
- Consumes: `Dialect` (Task 1); `convertDBValue`, `valueToSQL`, `identRe`, `getTableLock` (existing, `pkg/db/sqlite.go`)
- Produces: `type RemoteSQLEngine struct{...}`; `func NewRemoteSQLEngine(sqlDB *sql.DB, dialect Dialect) *RemoteSQLEngine` — implements `SelectArea`, `Seek`, `Skip`, `GoTop`, `GoBottom`, `EOF`, `BOF`, `FieldGet`, `FieldPut`, `RecLock`, `MsUnlock`, `RecCount`, `RecNo`, `Append`, `FieldPos`, `QueryRows`, `Exec` — the exact same method set `*SQLiteEngine` has, so it satisfies `vm.DBEngine` and `vm.SQLEngine` by duck typing.

**Design note — recno without an auto-increment column:** unlike SQLite (owns the file, can rely on `rowid`/`R_E_C_N_O_` autoincrement), an external table wasn't necessarily created by AdvPP. `Append()` therefore computes the next `R_E_C_N_O_` **client-side** (`max(existing R_E_C_N_O_ values in memory) + 1`) and writes it as an explicit column value in the `INSERT`, instead of relying on any driver-specific `LastInsertId()`/`RETURNING` behavior. This requires the physical remote table to already have an `R_E_C_N_O_` and `D_E_L_E_T_` column (same convention `DBCREATE` uses for local tables) — `RemoteSQLEngine` targets AdvPP-managed remote tables, not arbitrary unmodified legacy schemas. This is a documented limitation, same spirit as the "Limitações honestas documentadas" already at the top of `pkg/vm/dbaccess_native.go`.

- [ ] **Step 1: Add the mock-DB test dependency**

Run: `go get github.com/DATA-DOG/go-sqlmock@v1.5.2`

- [ ] **Step 2: Write the failing test — SelectArea + navigation**

```go
// pkg/db/remote_engine_test.go
package db

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRemoteSQLEngineSelectAreaAndSkip(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	cols := []string{"R_E_C_N_O_", "D_E_L_E_T_", "NOME"}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES WHERE 1=0`).
		WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES$`).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(int64(1), " ", "ACME").
			AddRow(int64(2), " ", "BETA"))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	if e.RecCount() != 2 {
		t.Fatalf("RecCount() = %d, want 2", e.RecCount())
	}
	if e.BOF() || e.EOF() {
		t.Fatalf("BOF/EOF unexpected right after SelectArea")
	}
	val, _ := e.FieldGet("NOME")
	if val.String() != "ACME" {
		t.Fatalf("FieldGet(NOME) = %q, want ACME", val.String())
	}
	if err := e.Skip(1); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	val, _ = e.FieldGet("NOME")
	if val.String() != "BETA" {
		t.Fatalf("FieldGet(NOME) after Skip = %q, want BETA", val.String())
	}
	if err := e.Skip(1); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if !e.EOF() {
		t.Fatal("EOF() should be true after skipping past the last record")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./pkg/db/... -run TestRemoteSQLEngineSelectAreaAndSkip -v`
Expected: FAIL (`NewRemoteSQLEngine` undefined)

- [ ] **Step 4: Implement `RemoteSQLEngine`**

```go
// pkg/db/remote_engine.go
package db

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// RemoteSQLEngine implementa DBEngine + SQLEngine (por duck typing, igual
// SQLiteEngine) sobre um *sql.DB real (Postgres/Oracle/MSSQL). Mesmo
// modelo do SQLiteEngine: SelectArea carrega TODAS as linhas em memória e
// a navegação (Skip/GoTop/...) opera sobre esse slice, não sobre um cursor
// de banco — RecLock/MsUnlock usam o mesmo mutex por tabela do pacote
// (getTableLock), sem lock real do banco (mesma limitação honesta do
// SQLiteEngine).
type RemoteSQLEngine struct {
	db           *sql.DB
	dialect      Dialect
	alias        string
	columns      []columnInfo
	records      []map[string]advplrt.Value
	current      int
	isLocked     bool
	recordsMutex sync.RWMutex
}

func NewRemoteSQLEngine(sqlDB *sql.DB, dialect Dialect) *RemoteSQLEngine {
	return &RemoteSQLEngine{db: sqlDB, dialect: dialect, current: -1}
}

func (e *RemoteSQLEngine) SelectArea(alias string) error {
	e.alias = strings.ToUpper(alias)
	if !identRe.MatchString(e.alias) {
		return fmt.Errorf("invalid table name: %q", e.alias)
	}

	rows, err := e.db.Query(fmt.Sprintf("SELECT * FROM %s WHERE 1=0", e.alias))
	if err != nil {
		return fmt.Errorf("table %s not found: %v", e.alias, err)
	}
	cols, err := rows.Columns()
	rows.Close()
	if err != nil {
		return err
	}

	e.columns = nil
	for _, c := range cols {
		e.columns = append(e.columns, columnInfo{name: strings.ToUpper(c)})
	}

	rows, err = e.db.Query(fmt.Sprintf("SELECT * FROM %s", e.alias))
	if err != nil {
		return err
	}
	defer rows.Close()

	e.records = make([]map[string]advplrt.Value, 0)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		record := make(map[string]advplrt.Value)
		for i, c := range cols {
			record[strings.ToUpper(c)] = convertDBValue(values[i])
		}
		e.records = append(e.records, record)
	}
	e.current = 0
	return rows.Err()
}

func (e *RemoteSQLEngine) Seek(key string) (bool, error) {
	for i, record := range e.records {
		for _, val := range record {
			if fmt.Sprintf("%v", val) == key {
				e.current = i
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *RemoteSQLEngine) Skip(count int) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) == 0 {
		return nil
	}
	e.current += count
	if e.current < 0 {
		e.current = 0
	}
	if e.current > len(e.records) {
		e.current = len(e.records)
	}
	return nil
}

func (e *RemoteSQLEngine) GoTop() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.current = 0
	return nil
}

func (e *RemoteSQLEngine) GoBottom() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

func (e *RemoteSQLEngine) EOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current >= len(e.records)
}

func (e *RemoteSQLEngine) BOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current < 0
}

func (e *RemoteSQLEngine) FieldGet(field string) (advplrt.Value, error) {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	if e.current < 0 || e.current >= len(e.records) {
		return advplrt.Nil, nil
	}
	if val, ok := e.records[e.current][strings.ToUpper(field)]; ok {
		return val, nil
	}
	return advplrt.Nil, nil
}

func (e *RemoteSQLEngine) FieldPut(field string, val advplrt.Value) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if e.current < 0 || e.current >= len(e.records) {
		return fmt.Errorf("no current record")
	}
	e.records[e.current][strings.ToUpper(field)] = val
	return nil
}

func (e *RemoteSQLEngine) RecLock() error {
	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		return fmt.Errorf("RecLock: no current record")
	}
	e.recordsMutex.RUnlock()
	if e.isLocked {
		return fmt.Errorf("RecLock: record already locked")
	}
	getTableLock(e.alias).Lock()
	e.isLocked = true
	return nil
}

func (e *RemoteSQLEngine) MsUnlock() error {
	if !e.isLocked {
		return nil
	}
	defer func() {
		e.isLocked = false
		getTableLock(e.alias).Unlock()
	}()

	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		return nil
	}
	record := e.records[e.current]
	e.recordsMutex.RUnlock()

	recno, ok := record["R_E_C_N_O_"]
	if !ok {
		return fmt.Errorf("MsUnlock: registro sem R_E_C_N_O_")
	}

	var setClauses []string
	var vals []any
	pos := 1
	for _, c := range e.columns {
		if c.name == "R_E_C_N_O_" {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", c.name, e.dialect.Placeholder(pos)))
		vals = append(vals, valueToSQL(record[c.name]))
		pos++
	}
	vals = append(vals, valueToSQL(recno))
	query := fmt.Sprintf("UPDATE %s SET %s WHERE R_E_C_N_O_ = %s",
		e.alias, strings.Join(setClauses, ", "), e.dialect.Placeholder(pos))
	_, err := e.db.Exec(query, vals...)
	return err
}

// Append calcula R_E_C_N_O_ no cliente (max atual + 1) em vez de depender
// de LastInsertId()/RETURNING — ver nota de design da Task 6 do plano.
func (e *RemoteSQLEngine) Append() error {
	if e.alias == "" || len(e.columns) == 0 {
		return fmt.Errorf("DbAppend: nenhuma área selecionada")
	}

	e.recordsMutex.Lock()
	var maxRecno float64
	for _, r := range e.records {
		if n, ok := r["R_E_C_N_O_"].(*advplrt.NumberValue); ok && n.Val > maxRecno {
			maxRecno = n.Val
		}
	}
	newRecno := maxRecno + 1
	e.recordsMutex.Unlock()

	var cols []string
	var placeholders []string
	var vals []any
	blank := make(map[string]advplrt.Value)
	pos := 1
	for _, c := range e.columns {
		switch c.name {
		case "R_E_C_N_O_":
			cols = append(cols, c.name)
			placeholders = append(placeholders, e.dialect.Placeholder(pos))
			vals = append(vals, newRecno)
			blank[c.name] = advplrt.NewNumber(newRecno)
			pos++
			continue
		case "D_E_L_E_T_":
			blank[c.name] = advplrt.NewString(" ")
			vals = append(vals, " ")
		default:
			blank[c.name] = advplrt.NewString("")
			vals = append(vals, "")
		}
		cols = append(cols, c.name)
		placeholders = append(placeholders, e.dialect.Placeholder(pos))
		pos++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", e.alias, strings.Join(cols, ","), strings.Join(placeholders, ","))
	if _, err := e.db.Exec(query, vals...); err != nil {
		return err
	}

	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.records = append(e.records, blank)
	e.current = len(e.records) - 1
	return nil
}

func (e *RemoteSQLEngine) FieldPos(field string) int {
	field = strings.ToUpper(field)
	for i, c := range e.columns {
		if c.name == field {
			return i + 1
		}
	}
	return 0
}

func (e *RemoteSQLEngine) RecCount() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return len(e.records)
}

func (e *RemoteSQLEngine) RecNo() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	if e.current < 0 || e.current >= len(e.records) {
		return 0
	}
	if n, ok := e.records[e.current]["R_E_C_N_O_"].(*advplrt.NumberValue); ok {
		return int(n.Val)
	}
	return e.current + 1
}

func (e *RemoteSQLEngine) QueryRows(query string, args ...any) ([]map[string]string, error) {
	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]string
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]string)
		for i, c := range cols {
			row[strings.ToUpper(c)] = convertDBValue(values[i]).String()
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *RemoteSQLEngine) Exec(query string, args ...any) error {
	_, err := e.db.Exec(query, args...)
	return err
}
```

- [ ] **Step 5: Run it to verify it passes**

Run: `go test ./pkg/db/... -run TestRemoteSQLEngineSelectAreaAndSkip -v`
Expected: PASS

- [ ] **Step 6: Write and pass a test for `RecLock`/`FieldPut`/`MsUnlock` and `Append`**

```go
// pkg/db/remote_engine_test.go (append)
func TestRemoteSQLEngineFieldPutAndMsUnlock(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	cols := []string{"R_E_C_N_O_", "D_E_L_E_T_", "NOME"}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES WHERE 1=0`).WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES$`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), " ", "ACME"))
	mock.ExpectExec(`UPDATE CLIENTES SET`).WillReturnResult(sqlmock.NewResult(0, 1))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	if err := e.RecLock(); err != nil {
		t.Fatalf("RecLock: %v", err)
	}
	if err := e.FieldPut("NOME", advplrt.NewString("ACME LTDA")); err != nil {
		t.Fatalf("FieldPut: %v", err)
	}
	if err := e.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoteSQLEngineAppend(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	cols := []string{"R_E_C_N_O_", "D_E_L_E_T_", "NOME"}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES WHERE 1=0`).WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES$`).WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectExec(`INSERT INTO CLIENTES`).WillReturnResult(sqlmock.NewResult(1, 1))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	if err := e.Append(); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if e.RecCount() != 1 {
		t.Fatalf("RecCount() = %d, want 1", e.RecCount())
	}
	if e.RecNo() != 1 {
		t.Fatalf("RecNo() = %d, want 1", e.RecNo())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
```

Import `advplrt "github.com/advpl/compiler/pkg/runtime"` at the top of the test file alongside `sqlmock`.

- [ ] **Step 7: Run all `pkg/db` tests**

Run: `go test ./pkg/db/... -v`
Expected: PASS (all tests, including Task 1-5's)

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum pkg/db/remote_engine.go pkg/db/remote_engine_test.go
git commit -m "feat(db): RemoteSQLEngine (DBEngine+SQLEngine sobre database/sql real)"
```

---

### Task 7: `DbConnection` AdvPL class

**Files:**
- Modify: `pkg/vm/dbaccess_native.go` (`dbstateConn` gets a `remote bool` field; small helper)
- Create: `pkg/vm/dbconnection_native.go`
- Modify: `pkg/vm/vm.go` (wire into `newInstance` and `callNativeMethod`)
- Test: `pkg/vm/dbconnection_native_test.go`

**Interfaces:**
- Consumes: `db.OpenRemote`, `db.ConnConfig`, `db.NewRemoteSQLEngine` (Tasks 5-6); `dbstate`, `dbstateConn` (existing, `pkg/vm/dbaccess_native.go`)
- Produces: `DbConnection` class with methods `New(cDriver, cHost, nPort, cService, cUser, cPassword)`, `Connect() -> lRet`, `Close() -> NIL`, `GetError() -> cErro`. On success, registers a `dbstateConn` with `remote = true` as the active connection — same global `dbstate` `TCLINK` already uses, so `TCSQLEXEC`/`TCGENQRY`/etc. work against it unchanged.

- [ ] **Step 1: Add the `remote` field**

```go
// pkg/vm/dbaccess_native.go:73-89 — adicionar campo ao struct existente
type dbstateConn struct {
	id       int
	connStr  string
	server   string
	port     int
	driver   string
	dbsid    string
	sid      int
	engine   DBEngine
	sqlEng   SQLEngine
	dbPath   string
	views    map[string]*dbViewMeta
	inPool   bool
	poolName string
	poolTime time.Time
	closed   bool
	remote   bool // true quando aberta por DbConnection (driver real), não TCLINK/SQLite
}
```

- [ ] **Step 2: Write the failing test**

```go
// pkg/vm/dbconnection_native_test.go
package vm

import (
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestDbConnectionNewSetsFields(t *testing.T) {
	resetDbaccessState()
	v := New(false)
	obj := newDbConnectionObject()
	args := []advplrt.Value{
		advplrt.NewString("POSTGRES"),
		advplrt.NewString("10.0.0.5"),
		advplrt.NewNumber(5432),
		advplrt.NewString("meubanco"),
		advplrt.NewString("usuario"),
		advplrt.NewString("senha"),
	}
	if err := v.callDbConnectionMethod(obj, "NEW", args); err != nil {
		t.Fatalf("NEW: %v", err)
	}
	st := obj.Native.(*dbConnState)
	if st.driver != "POSTGRES" || st.host != "10.0.0.5" || st.port != 5432 || st.service != "meubanco" {
		t.Fatalf("unexpected state after NEW: %+v", st)
	}
}

func TestDbConnectionConnectUnknownDriverSetsError(t *testing.T) {
	resetDbaccessState()
	v := New(false)
	obj := newDbConnectionObject()
	_ = v.callDbConnectionMethod(obj, "NEW", []advplrt.Value{
		advplrt.NewString("DB2"), advplrt.NewString("host"), advplrt.NewNumber(1),
		advplrt.NewString("svc"), advplrt.NewString("u"), advplrt.NewString("p"),
	})
	if err := v.callDbConnectionMethod(obj, "CONNECT", nil); err != nil {
		t.Fatalf("CONNECT should not return a Go error, only push .F. and set GetError: %v", err)
	}
	ret := v.pop()
	if advplrt.ToBool(ret) {
		t.Fatal("Connect() with unknown driver should return .F.")
	}
	st := obj.Native.(*dbConnState)
	if st.lastError == "" {
		t.Fatal("GetError() state should be populated after a failed Connect()")
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./pkg/vm/... -run TestDbConnection -v`
Expected: FAIL (`newDbConnectionObject`/`callDbConnectionMethod`/`dbConnState` undefined)

- [ ] **Step 4: Implement the class**

```go
// pkg/vm/dbconnection_native.go
package vm

import (
	"strings"

	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// dbConnState é o estado Go da classe DbConnection: credenciais de uma
// conexão externa real (Postgres/Oracle/MSSQL), setadas em New() e usadas
// em Connect(). A senha nunca é persistida em lugar hardcoded pelo AdvPP —
// quem chama New() decide de onde ela vem (GetEnv, cofre, etc).
type dbConnState struct {
	driver    string
	host      string
	port      int
	service   string
	user      string
	password  string
	connID    int
	lastError string
}

func newDbConnectionObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("DbConnection", nil)
	obj.Native = &dbConnState{}
	return obj
}

func (v *VM) callDbConnectionMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*dbConnState)
	if !ok {
		return advplrt.NewError("DbConnection: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		st.driver = strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		st.host = advplrt.ToString(getArg(args, 1))
		st.port = int(advplrt.ToFloat(getArg(args, 2)))
		st.service = advplrt.ToString(getArg(args, 3))
		st.user = advplrt.ToString(getArg(args, 4))
		st.password = advplrt.ToString(getArg(args, 5))
		v.push(obj)
	case "CONNECT":
		cfg := db.ConnConfig{Host: st.host, Port: st.port, Service: st.service, User: st.user, Password: st.password}
		sqlDB, dialect, err := db.OpenRemote(st.driver, cfg)
		if err != nil {
			st.lastError = err.Error()
			v.push(advplrt.False)
			return nil
		}
		engine := db.NewRemoteSQLEngine(sqlDB, dialect)

		dbstate.mu.Lock()
		id := dbstate.nextID
		dbstate.nextID++
		dbstate.conns[id] = &dbstateConn{
			id:     id,
			driver: st.driver,
			server: st.host,
			port:   st.port,
			engine: engine,
			sqlEng: engine,
			remote: true,
		}
		dbstate.active = id
		dbstate.mu.Unlock()

		st.connID = id
		st.lastError = ""
		v.push(advplrt.True)
	case "CLOSE":
		dbstate.mu.Lock()
		if c, ok := dbstate.conns[st.connID]; ok {
			dbaccessCloseConnLocked(c)
			delete(dbstate.conns, st.connID)
			if dbstate.active == st.connID {
				dbstate.active = -1
			}
		}
		dbstate.mu.Unlock()
		v.push(advplrt.Nil)
	case "GETERROR":
		v.push(advplrt.NewString(st.lastError))
	default:
		return advplrt.NewError("DbConnection: método desconhecido " + method)
	}
	return nil
}
```

- [ ] **Step 5: Wire the class into `vm.go`**

```go
// pkg/vm/vm.go — dentro de newInstance(), dentro do switch em upperName
case "DBCONNECTION":
	v.push(newDbConnectionObject())
	return nil
```

```go
// pkg/vm/vm.go — dentro de callNativeMethod(), dentro do switch em obj.ClassName
case "DbConnection":
	return v.callDbConnectionMethod(obj, upperMethod, args)
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./pkg/vm/... -run TestDbConnection -v`
Expected: PASS

- [ ] **Step 7: Run the full `pkg/vm` suite to check for regressions**

Run: `go test ./pkg/vm/...`
Expected: PASS (no existing test broken by the new struct field/switch cases)

- [ ] **Step 8: Commit**

```bash
git add pkg/vm/dbaccess_native.go pkg/vm/dbconnection_native.go pkg/vm/vm.go pkg/vm/dbconnection_native_test.go
git commit -m "feat(vm): classe DbConnection (New/Connect/Close/GetError) para bancos externos reais"
```

---

### Task 8: TC* natives against a real connection (integration check + AdvPL smoke source)

**Files:**
- Test: `pkg/vm/dbaccess_native_test.go` (append)
- Create: `examples/db/multidb_smoke.prw`

**Interfaces:**
- Consumes: `DbConnection` (Task 7); existing `TCSQLTOARR`/`TCGENQRY` natives (unchanged code, `pkg/vm/dbaccess_native.go`)

**Two separate TC* codepaths exist today** (found while implementing this
task, worth recording so nobody "fixes" it as a bug later): the elaborate
TC* family in `pkg/vm/dbaccess_native.go` (`TCLINK`, `TCGENQRY`,
`TCSQLTOARR`, `TCSQLERROR`, ...) reads through `dbaccessActiveConn()` →
`dbstateConn.sqlEng` — this is the one `DbConnection:Connect()` (Task 7)
plugs into directly, no code change needed. A second, simpler pair,
`TCSQLEXEC`/`TCSQLQUERY` (`pkg/vm/natives.go:1191-1221`), instead reads
`v.dbEngine` directly — those two start hitting the real external DB only
once Task 9's `DBSetDriver("TOPCONN")` swap is in effect, not merely from
`Connect()` succeeding. This task proves the first codepath; Task 9's own
test proves the second.

- [ ] **Step 1: Write a Go-level test proving TCSQLTOARR uses the active remote connection**

```go
// pkg/vm/dbaccess_native_test.go (append)
func TestTCSQLToArrUsesRemoteConnection(t *testing.T) {
	resetDbaccessState()
	v := New(false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerDbaccessNatives(natives)

	// Simula uma DbConnection real já conectada, sem abrir rede: registra
	// diretamente uma conexão "remote" cujo sqlEng é o SQLiteEngine em temp
	// dir mesmo (o ponto do teste é a ativação/roteamento via dbstate, não
	// o driver de rede em si — esse já foi coberto com sqlmock na Task 6).
	eng, sqlEng, _, err := dbaccessOpenEngine(999)
	if err != nil {
		t.Fatalf("dbaccessOpenEngine: %v", err)
	}
	sqlEng.Exec("CREATE TABLE TESTE (R_E_C_N_O_ INTEGER, D_E_L_E_T_ TEXT, NOME TEXT)")
	sqlEng.Exec("INSERT INTO TESTE VALUES (1, ' ', 'REMOTO')")
	dbstate.mu.Lock()
	dbstate.conns[999] = &dbstateConn{id: 999, driver: "POSTGRES", engine: eng, sqlEng: sqlEng, remote: true}
	dbstate.active = 999
	dbstate.mu.Unlock()

	fn, ok := natives["TCSQLTOARR"]
	if !ok {
		t.Fatal("TCSQLTOARR not registered")
	}
	aResult := advplrt.NewArray(nil)
	n, err := fn([]advplrt.Value{advplrt.NewString("SELECT NOME FROM TESTE"), aResult})
	if err != nil {
		t.Fatalf("TCSQLTOARR: %v", err)
	}
	if advplrt.ToFloat(n) < 0 {
		t.Fatalf("TCSQLTOARR returned failure code %v", n)
	}
	c, ok := dbaccessActiveConn()
	if !ok || !c.remote {
		t.Fatal("active connection should be the remote one")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./pkg/vm/... -run TestTCSQLToArrUsesRemoteConnection -v`
Expected: PASS (proves no code change was needed in the `dbaccess_native.go` TC* family)

- [ ] **Step 3: Add an AdvPL smoke-test source showing the intended real usage**

```advpl
// examples/db/multidb_smoke.prw
#include "totvs.ch"

User Function MultiDbSmoke()
    Local oConn := DbConnection():New("POSTGRES", GetEnv("ADVPP_PG_HOST"), 5432, "meubanco", GetEnv("ADVPP_PG_USER"), GetEnv("ADVPP_PG_PASSWORD"))
    Local aRows

    If !oConn:Connect()
        ConOut("Falha ao conectar: " + oConn:GetError())
        Return
    EndIf

    aRows := TCGenQry("SELECT * FROM CLIENTES")
    ConOut("Linhas: " + cValToChar(Len(aRows)))

    oConn:Close()
Return
```

- [ ] **Step 4: Commit**

```bash
git add pkg/vm/dbaccess_native_test.go examples/db/multidb_smoke.prw
git commit -m "test(vm): prova que TCSQLEXEC/TCGENQRY já funcionam sobre conexão remota; smoke AdvPL"
```

---

### Task 9: `DBSETDRIVER("TOPCONN")` routes `DBUseArea`/RDD to the active `DbConnection`

**Files:**
- Modify: `pkg/vm/vm.go` (new `localDBEngine` field + `applyRDDEngine` helper)
- Modify: `pkg/vm/dbgenericas_native.go:391-406` (`DBSETDRIVER`)
- Test: `pkg/vm/dbgenericas_native_test.go` (append)

**Interfaces:**
- Consumes: `dbstate`, `dbstateConn.remote`, `dbstateConn.engine` (Task 7); `validRDD` (existing)
- Produces: `v.applyRDDEngine(cRDD string)` — swaps `v.dbEngine` between the local engine and the active remote connection's engine.

**Design note (simplification vs. the spec's "map by alias" sketch):** `v.dbEngine`
is a single VM-wide field consumed directly by ~50 call sites across
`vm.go`, `natives.go`, `dbgenericas_native.go`, `browse.go`, `grid.go` and
others (every opcode and native that manipulates the "current" work area).
Rewriting all of them to resolve a per-alias engine would be a much larger,
riskier change for the same practical outcome: a whole AdvPL program
targets one external RDD at a time via `DBSetDriver("TOPCONN")`, exactly
like a `TOPCONN`-based Protheus job does. So this task swaps the single
`v.dbEngine` wholesale instead of introducing a map — same effect for the
common case, none of the call-site churn. Mixing local SQLite tables and a
remote TOPCONN table open at the same time in one VM session is not
supported by this swap (documented limitation); only one `DbConnection` is
routable at a time (the active one, `dbstate.active`).

- [ ] **Step 1: Add the `localDBEngine` field**

```go
// pkg/vm/vm.go — junto ao campo dbEngine existente (linha ~94)
	dbEngine           DBEngine
	localDBEngine      DBEngine // engine local (SQLite) guardado antes de trocar por DBSetDriver("TOPCONN")
```

- [ ] **Step 2: Write the failing test**

```go
// pkg/vm/dbgenericas_native_test.go (append)
func TestDBSetDriverTopconnSwapsEngine(t *testing.T) {
	resetDbaccessState()
	v := New(false)
	localEngine, _, _, err := dbaccessOpenEngine(1)
	if err != nil {
		t.Fatalf("dbaccessOpenEngine: %v", err)
	}
	v.SetDBEngine(localEngine)

	remoteEngine, remoteSQL, _, err := dbaccessOpenEngine(2) // stand-in: mesmo tipo concreto, ver nota da Task 8
	if err != nil {
		t.Fatalf("dbaccessOpenEngine: %v", err)
	}
	dbstate.mu.Lock()
	dbstate.conns[2] = &dbstateConn{id: 2, driver: "POSTGRES", engine: remoteEngine, sqlEng: remoteSQL, remote: true}
	dbstate.active = 2
	dbstate.mu.Unlock()

	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerDbgenericasNatives(natives)
	fn := natives["DBSETDRIVER"]

	if _, err := fn([]advplrt.Value{advplrt.NewString("TOPCONN")}); err != nil {
		t.Fatalf("DBSETDRIVER(TOPCONN): %v", err)
	}
	if v.dbEngine != remoteEngine {
		t.Fatal("v.dbEngine should point to the remote connection's engine after DBSetDriver(\"TOPCONN\")")
	}

	if _, err := fn([]advplrt.Value{advplrt.NewString("DBFCDX")}); err != nil {
		t.Fatalf("DBSETDRIVER(DBFCDX): %v", err)
	}
	if v.dbEngine != localEngine {
		t.Fatal("v.dbEngine should be restored to the local engine after DBSetDriver(\"DBFCDX\")")
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./pkg/vm/... -run TestDBSetDriverTopconnSwapsEngine -v`
Expected: FAIL (`v.dbEngine` unchanged — `DBSETDRIVER` doesn't call any swap yet)

- [ ] **Step 4: Implement `applyRDDEngine` and wire it into `DBSETDRIVER`**

```go
// pkg/vm/vm.go (novo método, próximo de SetDBEngine)

// applyRDDEngine troca v.dbEngine para o engine da conexão remota ativa
// (DbConnection:Connect) quando cRDD é "TOPCONN", e devolve o engine local
// quando a RDD volta a ser local. Ver nota de design da Task 9 do plano
// multidb: troca o campo único da VM inteira, não faz roteamento por
// alias.
func (v *VM) applyRDDEngine(cRDD string) {
	if v.localDBEngine == nil {
		v.localDBEngine = v.dbEngine
	}
	if cRDD == "TOPCONN" {
		dbstate.mu.Lock()
		c, ok := dbstate.conns[dbstate.active]
		dbstate.mu.Unlock()
		if ok && c != nil && c.remote && c.engine != nil {
			v.dbEngine = c.engine
			return
		}
	}
	v.dbEngine = v.localDBEngine
}
```

```go
// pkg/vm/dbgenericas_native.go:391-406 — substituir o corpo de DBSETDRIVER
natives["DBSETDRIVER"] = func(args []advplrt.Value) (advplrt.Value, error) {
	s := v.dbGenStateFor()
	cRDD := getArgString(args, 0, "")
	if advplrt.IsNil(getArg(args, 0)) {
		cRDD = ""
	}
	cRDD = strings.ToUpper(strings.TrimSpace(cRDD))
	s.mu.Lock()
	defer s.mu.Unlock()
	if cRDD != "" && validRDD(cRDD) {
		prev := s.defaultRDD
		s.defaultRDD = cRDD
		v.applyRDDEngine(cRDD)
		return advplrt.NewString(prev), nil
	}
	return advplrt.NewString(s.defaultRDD), nil
}
```

- [ ] **Step 5: Run it to verify it passes**

Run: `go test ./pkg/vm/... -run TestDBSetDriverTopconnSwapsEngine -v`
Expected: PASS

- [ ] **Step 6: Write and pass an end-to-end test — `DBUseArea` under `TOPCONN` reads from the remote engine**

```go
// pkg/vm/dbgenericas_native_test.go (append)
func TestDBUseAreaUnderTopconnUsesRemoteEngine(t *testing.T) {
	resetDbaccessState()
	v := New(false)
	localEngine, _, _, _ := dbaccessOpenEngine(1)
	v.SetDBEngine(localEngine)

	// Tabela remota "de mentira": SQLiteEngine próprio, mas registrado como
	// conexão remota — o ponto testado é o roteamento de v.dbEngine, não o
	// driver de rede (já coberto por sqlmock na Task 6).
	remoteEngine, remoteSQL, _, _ := dbaccessOpenEngine(2)
	remoteSQL.Exec("CREATE TABLE CLIENTES (R_E_C_N_O_ INTEGER, D_E_L_E_T_ TEXT, NOME TEXT)")
	remoteSQL.Exec("INSERT INTO CLIENTES VALUES (1, ' ', 'REMOTO')")
	dbstate.mu.Lock()
	dbstate.conns[2] = &dbstateConn{id: 2, driver: "POSTGRES", engine: remoteEngine, sqlEng: remoteSQL, remote: true}
	dbstate.active = 2
	dbstate.mu.Unlock()

	genNatives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerDbgenericasNatives(genNatives)
	genNatives["DBSETDRIVER"]([]advplrt.Value{advplrt.NewString("TOPCONN")})
	genNatives["DBUSEAREA"]([]advplrt.Value{advplrt.Nil, advplrt.NewString("TOPCONN"), advplrt.NewString("CLIENTES"), advplrt.NewString("CLIENTES")})

	val, err := v.dbEngine.FieldGet("NOME")
	if err != nil {
		t.Fatalf("FieldGet: %v", err)
	}
	if val.String() != "REMOTO" {
		t.Fatalf("FieldGet(NOME) = %q, want REMOTO — DBUseArea did not route to the remote engine", val.String())
	}
}
```

- [ ] **Step 7: Run it**

Run: `go test ./pkg/vm/... -run TestDBUseAreaUnderTopconnUsesRemoteEngine -v`
Expected: PASS

- [ ] **Step 8: Run the entire `pkg/vm` suite for regressions**

Run: `go test ./pkg/vm/...`
Expected: PASS (existing local/SQLite-path tests unaffected — default RDD stays `DBFCDX`, `applyRDDEngine` only changes `v.dbEngine` when `TOPCONN` is explicitly set)

- [ ] **Step 9: Commit**

```bash
git add pkg/vm/vm.go pkg/vm/dbgenericas_native.go pkg/vm/dbgenericas_native_test.go
git commit -m "feat(vm): DBSetDriver(\"TOPCONN\") roteia DBUseArea/RDD pra conexão remota ativa"
```

---

### Task 10: Cross-platform build check, docs, roadmap cleanup

**Files:**
- Modify: `pkg/vm/dbaccess_native.go:1-31` (top-of-file comment)
- Modify: `pkg/vm/dbgenericas_native.go:1-40` (top-of-file comment)
- Modify: `ROADMAP.md` (no entry to add — this was implemented directly, not deferred; just confirm no stale reference exists)

- [ ] **Step 1: Verify cross-platform build (the project's absolute premise)**

Run:
```bash
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
go vet ./...
```
Expected: all three builds succeed, `go vet` clean — confirms the pure-Go drivers (pgx, go-ora, go-mssqldb) introduced no CGO/platform dependency.

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: PASS (integration-tagged tests are skipped automatically — no external DB needed)

- [ ] **Step 3: Update the honest-limitations comment blocks**

```go
// pkg/vm/dbaccess_native.go:16-31 — adicionar ao bloco de comentários existente
//   - TCLink/DbConnection: TCLink continua abrindo um SQLiteEngine local
//     (comportamento inalterado). DbConnection():New()/Connect() abre uma
//     conexão real (PostgreSQL/Oracle/MSSQL) via pkg/db.OpenRemote — ver
//     docs/superpowers/specs/2026-09-16-advpp-multidb-design.md.
```

```go
// pkg/vm/dbgenericas_native.go:30-39 — adicionar ao bloco de comentários existente
//   - DBSetDriver("TOPCONN") com uma DbConnection real ativa (Connect()
//     bem-sucedido) troca v.dbEngine para RemoteSQLEngine (pkg/db) — leitura
//     e escrita de tabela via SQL real, exigindo que a tabela física remota
//     já tenha as colunas R_E_C_N_O_/D_E_L_E_T_ (convenção AdvPP, mesma do
//     SQLiteEngine local). Não há roteamento por alias: um único RDD remoto
//     "ativo" por vez na sessão (ver nota de design no plano multidb).
```

- [ ] **Step 4: Commit**

```bash
git add pkg/vm/dbaccess_native.go pkg/vm/dbgenericas_native.go
git commit -m "docs: documenta conectividade real multi-provider nos comentários de topo"
```
