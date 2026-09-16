package db

import (
	"testing"

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
