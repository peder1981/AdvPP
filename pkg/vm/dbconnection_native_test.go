package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestDbConnectionNewSetsFields(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
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
	v := NewVM(&compiler.Bytecode{}, false)
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

// TestDbConnectionCloseRestoresLocalEngine prova o achado #6 da revisão
// final da branch multidb: DbConnection:Close() só apagava
// dbstate.active, sem re-rodar applyRDDEngine — v.dbEngine ficava
// "pendurado" apontando pro RemoteSQLEngine agora fechado enquanto
// DBSetDriver("TOPCONN") continuasse em efeito, e a próxima leitura via
// TCSQLEXEC/DBUseArea bateria num *sql.DB já fechado em vez de cair de
// volta pro engine local.
func TestDbConnectionCloseRestoresLocalEngine(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)

	// Engine local "de verdade" (SQLiteEngine em temp dir) registrado como
	// o engine da VM antes de qualquer TOPCONN — é o que Close() deve
	// restaurar.
	localEngine, _, _, err := dbaccessOpenEngine(1)
	if err != nil {
		t.Fatalf("dbaccessOpenEngine(local): %v", err)
	}
	v.SetDBEngine(localEngine)

	// Simula uma DbConnection já conectada (sem dial de rede — reaproveita
	// SQLiteEngine como "remoto de mentira", mesmo padrão do teste de
	// roteamento em dbgenericas_native_test.go): registra a conexão
	// diretamente em dbstate e ativa TOPCONN.
	remoteEngine, remoteSQL, _, err := dbaccessOpenEngine(2)
	if err != nil {
		t.Fatalf("dbaccessOpenEngine(remote): %v", err)
	}
	dbstate.mu.Lock()
	dbstate.conns[2] = &dbstateConn{id: 2, driver: "POSTGRES", engine: remoteEngine, sqlEng: remoteSQL, remote: true}
	dbstate.active = 2
	dbstate.mu.Unlock()

	s := v.dbGenStateFor()
	s.defaultRDD = "TOPCONN"
	v.applyRDDEngine("TOPCONN")
	if v.dbEngine != remoteEngine {
		t.Fatal("setup: v.dbEngine deveria apontar pro engine remoto antes do Close()")
	}

	obj := newDbConnectionObject()
	st := obj.Native.(*dbConnState)
	st.connID = 2

	if err := v.callDbConnectionMethod(obj, "CLOSE", nil); err != nil {
		t.Fatalf("CLOSE: %v", err)
	}

	if v.dbEngine == remoteEngine {
		t.Fatal("v.dbEngine ainda aponta pro engine remoto (agora fechado) depois de Close() — deveria ter voltado pro engine local")
	}
	if v.dbEngine != localEngine {
		t.Fatalf("v.dbEngine = %v, want o engine local restaurado (%v)", v.dbEngine, localEngine)
	}
}
