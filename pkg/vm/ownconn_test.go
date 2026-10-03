package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
)

type fakeEngine struct {
	DBEngine
	closed bool
}

func (f *fakeEngine) Close() error { f.closed = true; return nil }

func TestApplyRDDUsesOwnConnection(t *testing.T) {
	resetDbaccessState()
	a := NewVM(&compiler.Bytecode{}, false)
	b := NewVM(&compiler.Bytecode{}, false)
	ea, eb := &fakeEngine{}, &fakeEngine{}
	a.registerOwnedConn(ea, "POSTGRES", "h", 1)
	b.registerOwnedConn(eb, "POSTGRES", "h", 1) // a "ativa" global agora é a de b
	a.applyRDDEngine("TOPCONN")
	if a.dbEngine != DBEngine(ea) {
		t.Fatal("VM a deveria usar a propria conexao, nao a ativa global (de b)")
	}
}

func TestCloseOwnedConnections(t *testing.T) {
	resetDbaccessState()
	a := NewVM(&compiler.Bytecode{}, false)
	e1, e2 := &fakeEngine{}, &fakeEngine{}
	id1 := a.registerOwnedConn(e1, "POSTGRES", "h", 1)
	id2 := a.registerOwnedConn(e2, "POSTGRES", "h", 1)
	a.CloseOwnedConnections()
	if !e1.closed || !e2.closed {
		t.Fatal("conexoes da sessao deveriam ser fechadas")
	}
	dbstate.mu.Lock()
	_, ok1 := dbstate.conns[id1]
	_, ok2 := dbstate.conns[id2]
	dbstate.mu.Unlock()
	if ok1 || ok2 {
		t.Fatal("conexoes fechadas deveriam sair do dbstate")
	}
}
