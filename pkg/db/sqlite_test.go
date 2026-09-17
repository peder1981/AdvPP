package db

import (
	"testing"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestSkipReachesEOF is a regression test for a bug found while writing an
// invocation-proof test for DBEval (pkg/vm/blococodigo_native_test.go,
// TestDBEValInvokesCodeblockPerRecord — see task-7-report.md, 2026-08-10
// escalation entry).
//
// Skip() used to clamp the cursor at len(records)-1, so once positioned on
// the last record, further Skip(1) calls were silent no-ops and EOF() could
// never become true. Every AdvPL loop of the canonical form
// `While !Eof() ... dbSkip() ... EndDo` (and DBEval's own internal
// iteration) would spin forever on real data, saved only by DBEval's
// unrelated 100000-iteration safety cap.
func TestSkipReachesEOF(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := NewSQLiteEngine(tmpDir + "/skip_eof_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE SKIP_EOF_TEST (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		NAME TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if err := eng.Exec("INSERT INTO SKIP_EOF_TEST (NAME) VALUES (?)", "R"); err != nil {
			t.Fatalf("insert record %d: %v", i, err)
		}
	}
	if err := eng.SelectArea("SKIP_EOF_TEST"); err != nil {
		t.Fatalf("SelectArea: %v", err)
	}

	eng.GoTop()

	// The canonical AdvPL loop shape: While !Eof() ... dbSkip() ... EndDo.
	// With 3 records this must terminate in exactly 3 iterations.
	const maxIterations = 10 // generous bound; a correct engine needs exactly 3
	iterations := 0
	for !eng.EOF() && iterations < maxIterations {
		iterations++
		if err := eng.Skip(1); err != nil {
			t.Fatalf("Skip(1) on iteration %d: %v", iterations, err)
		}
	}

	if iterations != 3 {
		t.Fatalf("loop ran %d iteration(s), want exactly 3 (one per record); EOF()=%v after loop", iterations, eng.EOF())
	}
	if !eng.EOF() {
		t.Fatalf("expected EOF() == true after skipping past the last of 3 records, got false (RecNo=%d)", eng.RecNo())
	}
}

func TestConvertDBValueBytes(t *testing.T) {
	got := convertDBValue([]byte("hello"))
	if got.String() != "hello" {
		t.Errorf("convertDBValue([]byte(\"hello\")) = %q, want \"hello\"", got.String())
	}
}

// TestConvertDBValueTimeRoundTrip prova o achado #4 da revisão final da
// branch multidb: drivers de rede (pgx/go-mssqldb/go-ora) devolvem
// time.Time para colunas DATE/TIMESTAMP; sem um case dedicado,
// convertDBValue caía no default (fmt.Sprintf("%v", v)) e produzia uma
// string tipo "2026-09-16 00:00:00 +0000 UTC" em vez de um valor D
// (DateValue) real — e valueToSQL, ao regravar via MsUnlock, devia enviar
// o time.Time nativo pro driver, não a formatação AdvPL (DD/MM/YYYY).
func TestConvertDBValueTimeRoundTrip(t *testing.T) {
	when := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)

	got := convertDBValue(when)
	dv, ok := got.(*advplrt.DateValue)
	if !ok {
		t.Fatalf("convertDBValue(time.Time) = %T, want *advplrt.DateValue", got)
	}
	if !dv.Val.Equal(when) {
		t.Errorf("convertDBValue(time.Time).Val = %v, want %v", dv.Val, when)
	}
	if got.Type() != "D" {
		t.Errorf("convertDBValue(time.Time).Type() = %q, want \"D\"", got.Type())
	}

	// Round-trip: valueToSQL deve devolver o time.Time nativo (bindável
	// direto pelo driver), não a string DD/MM/YYYY do String() do DateValue.
	back := valueToSQL(dv)
	tv, ok := back.(time.Time)
	if !ok {
		t.Fatalf("valueToSQL(DateValue) = %T, want time.Time", back)
	}
	if !tv.Equal(when) {
		t.Errorf("valueToSQL(DateValue) = %v, want %v", tv, when)
	}
}
