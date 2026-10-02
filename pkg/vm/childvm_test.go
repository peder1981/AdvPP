package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
)

func TestChildVMLocalEngineKeepsFactory(t *testing.T) {
	resetDbaccessState()
	parent := NewVM(&compiler.Bytecode{}, false)
	calls := 0
	parent.SetDBFactory(func() DBEngine { calls++; return &fakeEngine{} })
	child, done, err := parent.newChildVM()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if child.dbEngine == nil || calls != 2 {
		t.Fatalf("filha com engine local deveria abrir via dbFactory (calls=%d)", calls)
	}
}

func TestChildVMInheritsRDD(t *testing.T) {
	resetDbaccessState()
	parent := NewVM(&compiler.Bytecode{}, false)
	parent.dbGenStateFor().defaultRDD = "TOPCONN"
	child, done, err := parent.newChildVM()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if child.dbGenStateFor().defaultRDD != "TOPCONN" {
		t.Fatal("filha deveria herdar o RDD padrao do pai")
	}
}

// Engine remoto do pai: a filha recebe um clone PRÓPRIO (nunca o do pai),
// registrado como conexão dela e fechado no done().
func TestChildVMClonesRemote(t *testing.T) {
	resetDbaccessState()
	parentEng := &fakeEngine{}
	var cloned *fakeEngine
	old := cloneRemote
	cloneRemote = func(e DBEngine) (DBEngine, bool, error) {
		if e != DBEngine(parentEng) {
			return nil, false, nil
		}
		cloned = &fakeEngine{}
		return cloned, true, nil
	}
	defer func() { cloneRemote = old }()

	parent := NewVM(&compiler.Bytecode{}, false)
	parent.dbEngine = parentEng
	child, done, err := parent.newChildVM()
	if err != nil {
		t.Fatal(err)
	}
	if child.dbEngine == DBEngine(parentEng) || child.dbEngine != DBEngine(cloned) {
		t.Fatal("filha deveria usar o clone, nao a conexao do pai")
	}
	child.applyRDDEngine("TOPCONN")
	if child.dbEngine != DBEngine(cloned) {
		t.Fatal("DBSetDriver TOPCONN na filha deveria manter o clone dela")
	}
	done()
	if !cloned.closed || parentEng.closed {
		t.Fatal("done() deveria fechar so o clone da filha")
	}
}
