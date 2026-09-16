package db

import (
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	advplrt "github.com/advpl/compiler/pkg/runtime"
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

// TestRemoteSQLEngineAppendConcurrent regression test for the TOCTOU race
// in Append(): with getTableLock scoped only around the in-memory scan
// (not around the full scan -> INSERT -> append-to-slice sequence), two
// concurrent Append() calls on the same alias could read the same
// max(R_E_C_N_O_) and write duplicate synthetic recnos to the remote
// table. Fix: getTableLock(e.alias) now spans the whole critical section.
func TestRemoteSQLEngineAppendConcurrent(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()
	mock.MatchExpectationsInOrder(false)

	const n = 8
	cols := []string{"R_E_C_N_O_", "D_E_L_E_T_", "NOME"}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES2 WHERE 1=0`).WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES2$`).WillReturnRows(sqlmock.NewRows(cols))
	for i := 0; i < n; i++ {
		mock.ExpectExec(`INSERT INTO CLIENTES2`).WillReturnResult(sqlmock.NewResult(int64(i+1), 1))
	}

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES2"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = e.Append()
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Append() goroutine %d: %v", i, err)
		}
	}

	if e.RecCount() != n {
		t.Fatalf("RecCount() = %d, want %d", e.RecCount(), n)
	}

	seen := make(map[float64]bool)
	if err := e.GoTop(); err != nil {
		t.Fatalf("GoTop: %v", err)
	}
	for i := 0; i < n; i++ {
		val, err := e.FieldGet("R_E_C_N_O_")
		if err != nil {
			t.Fatalf("FieldGet: %v", err)
		}
		nv, ok := val.(*advplrt.NumberValue)
		if !ok {
			t.Fatalf("R_E_C_N_O_ is not numeric: %v", val)
		}
		if seen[nv.Val] {
			t.Fatalf("duplicate R_E_C_N_O_ = %v across concurrent Append() calls", nv.Val)
		}
		seen[nv.Val] = true
		if err := e.Skip(1); err != nil {
			t.Fatalf("Skip: %v", err)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestRemoteSQLEngineAppendAfterRecLockNoDeadlock regression test: RecLock()
// holds getTableLock(alias) until MsUnlock() is called. If Append() reused
// that same mutex (as an earlier version of the TOCTOU fix did), calling
// RecLock() then Append() on the same alias from the same goroutine without
// an intervening MsUnlock() would self-deadlock forever (Go mutexes aren't
// reentrant) — a plausible "clone this record while it's locked" AdvPL
// idiom, and also the shape of a forgotten-MsUnlock bug. Append() must use
// its own lock (getAppendLock) so this sequence completes promptly instead
// of hanging. Guarded with a timeout so a real regression fails the test
// instead of hanging the whole test run.
func TestRemoteSQLEngineAppendAfterRecLockNoDeadlock(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	cols := []string{"R_E_C_N_O_", "D_E_L_E_T_", "NOME"}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES3 WHERE 1=0`).WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES3$`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), " ", "ACME"))
	mock.ExpectExec(`INSERT INTO CLIENTES3`).WillReturnResult(sqlmock.NewResult(2, 1))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES3"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	if err := e.RecLock(); err != nil {
		t.Fatalf("RecLock: %v", err)
	}
	// Deliberately no MsUnlock() here — getTableLock(CLIENTES3) stays held
	// by this same goroutine, exactly the sequence that used to deadlock.

	done := make(chan error, 1)
	go func() {
		done <- e.Append()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Append() after RecLock (no MsUnlock) returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Append() deadlocked while RecLock was held on the same alias without MsUnlock")
	}

	if e.RecCount() != 2 {
		t.Fatalf("RecCount() = %d, want 2", e.RecCount())
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

// TestRemoteSQLEngineSelectAreaCapturesSQLType prova o achado #3 da revisão
// final da branch multidb: SelectArea agora chama rows.ColumnTypes() além
// de rows.Columns(), populando columnInfo.sqlType — sem isso, e.columns
// tinha só o nome, e Append() não tinha como saber que "SALDO" é numérico
// (caía sempre no branch de string vazia).
func TestRemoteSQLEngineSelectAreaCapturesSQLType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	colsDef := []*sqlmock.Column{
		sqlmock.NewColumn("R_E_C_N_O_").OfType("NUMERIC", 0),
		sqlmock.NewColumn("D_E_L_E_T_").OfType("TEXT", ""),
		sqlmock.NewColumn("SALDO").OfType("NUMERIC", 0.0),
	}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES4 WHERE 1=0`).
		WillReturnRows(sqlmock.NewRowsWithColumnDefinition(colsDef...))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES4$`).
		WillReturnRows(sqlmock.NewRowsWithColumnDefinition(colsDef...))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES4"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}

	var got string
	for _, c := range e.columns {
		if c.name == "SALDO" {
			got = c.sqlType
		}
	}
	if got != "NUMERIC" {
		t.Fatalf("columns[SALDO].sqlType = %q, want %q", got, "NUMERIC")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestRemoteSQLEngineAppendSendsTypedNumericValue prova o achado #3 da
// revisão final da branch multidb: Append() enviava "" (string vazia) pra
// TODA coluna, incluindo numéricas — Postgres/Oracle/MSSQL reais rejeitam
// '' num INT/NUMERIC/etc. Com columnInfo.sqlType capturado em SelectArea
// (ver teste acima), Append() agora manda um 0 numérico pra "SALDO".
func TestRemoteSQLEngineAppendSendsTypedNumericValue(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer mockDB.Close()

	colsDef := []*sqlmock.Column{
		sqlmock.NewColumn("R_E_C_N_O_").OfType("NUMERIC", 0),
		sqlmock.NewColumn("D_E_L_E_T_").OfType("TEXT", ""),
		sqlmock.NewColumn("SALDO").OfType("NUMERIC", 0.0),
	}
	mock.ExpectQuery(`SELECT \* FROM CLIENTES5 WHERE 1=0`).
		WillReturnRows(sqlmock.NewRowsWithColumnDefinition(colsDef...))
	mock.ExpectQuery(`SELECT \* FROM CLIENTES5$`).
		WillReturnRows(sqlmock.NewRowsWithColumnDefinition(colsDef...))
	// WithArgs: R_E_C_N_O_ calculado (AnyArg), D_E_L_E_T_ = " ", SALDO = 0
	// (int) — NÃO "" (string). Se Append() regredir pro branch de texto,
	// este WithArgs deixa de casar e o teste falha.
	mock.ExpectExec(`INSERT INTO CLIENTES5`).
		WithArgs(sqlmock.AnyArg(), " ", 0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.SelectArea("CLIENTES5"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}
	if err := e.Append(); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations (Append não mandou o valor numérico tipado esperado): %v", err)
	}
}

// TestRemoteSQLEngineCloseClosesUnderlyingDB prova o achado #5 da revisão
// final da branch multidb: RemoteSQLEngine não tinha Close(), então o type
// assertion `interface{ Close() error }` em dbaccessCloseConnLocked
// (pkg/vm/dbaccess_native.go) nunca casava e a conexão real vazava.
func TestRemoteSQLEngineCloseClosesUnderlyingDB(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectClose()

	e := NewRemoteSQLEngine(mockDB, postgresDialect{})
	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
