package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
)

// fakeSQLEngine: engine de teste que tambem implementa SQLEngine.
type fakeSQLEngine struct {
	DBEngine
	name string
}

func (f *fakeSQLEngine) Exec(query string, args ...any) error { return nil }
func (f *fakeSQLEngine) QueryRows(query string, args ...any) ([]map[string]string, error) {
	return nil, nil
}

// Regressao: DbConnection:Connect e' a conexao da VM, mas TCSQLQUERY lia o engine da
// RDD corrente (local). Com a conexao propria viva, TCSQL* tem de falar com ela.
func TestTCSQLUsesOwnRemoteConnectionEvenWhenRDDIsLocal(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
	remote := &fakeSQLEngine{name: "remote"}
	local := &fakeSQLEngine{name: "local"}
	v.dbEngine = local
	v.localDBEngine = local
	v.localDBEngineCaptured = true
	v.registerOwnedConn(remote, "POSTGRES", "h", 1)
	// RDD corrente ainda e' local (sem DBSetDriver("TOPCONN")).
	got, ok := v.tcSQLEngine()
	if !ok || got != SQLEngine(remote) {
		t.Fatalf("TCSQL devia usar a conexao propria remota, obteve %#v", got)
	}
}

// Sem conexao propria viva, TCSQL* cai no engine local (falha fechada, nao herda outra sessao).
func TestTCSQLFallsBackToLocalWithoutOwnConnection(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
	local := &fakeSQLEngine{name: "local"}
	v.dbEngine = local
	got, ok := v.tcSQLEngine()
	if !ok || got != SQLEngine(local) {
		t.Fatal("sem conexao propria TCSQL devia usar o engine local")
	}
}

// Isolamento de sessao: a conexao "ativa" global de outra VM nunca e' usada.
func TestTCSQLNeverUsesOtherSessionActiveConnection(t *testing.T) {
	resetDbaccessState()
	a := NewVM(&compiler.Bytecode{}, false)
	b := NewVM(&compiler.Bytecode{}, false)
	local := &fakeSQLEngine{name: "local"}
	a.dbEngine = local
	b.registerOwnedConn(&fakeSQLEngine{name: "b"}, "POSTGRES", "h", 1) // "ativa" global e' de b
	got, _ := a.tcSQLEngine()
	if got == SQLEngine(&fakeSQLEngine{name: "b"}) || got != SQLEngine(local) {
		t.Fatal("VM a nao pode usar a conexao da sessao b")
	}
}

// Conexao propria fechada: volta ao engine local, nao usa um *sql.DB fechado.
func TestTCSQLIgnoresClosedOwnConnection(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
	local := &fakeSQLEngine{name: "local"}
	v.dbEngine = local
	id := v.registerOwnedConn(&fakeSQLEngine{name: "remote"}, "POSTGRES", "h", 1)
	dbstate.mu.Lock()
	dbstate.conns[id].closed = true
	dbstate.mu.Unlock()
	got, ok := v.tcSQLEngine()
	if !ok || got != SQLEngine(local) {
		t.Fatal("conexao propria fechada nao pode ser usada por TCSQL")
	}
}
