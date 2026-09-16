package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestTCSQLToArrUsesRemoteConnection proves that TCSQLTOARR uses the active
// remote connection (registered via DbConnection:Connect, Task 7) without
// requiring any changes to the TC* family in dbaccess_native.go.
func TestTCSQLToArrUsesRemoteConnection(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
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
	defer func() {
		if c, ok := eng.(interface{ Close() error }); ok && c != nil {
			_ = c.Close()
		}
	}()

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
