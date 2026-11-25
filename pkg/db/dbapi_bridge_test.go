package db

import (
	"os"
	"strings"
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestDbapiBridgeCodec valida o codec de frames sem bridge real:
// encode/decode simétrico de campos, incluindo NULL.
func TestDbapiBridgeCodec(t *testing.T) {
	fields := []dbapiField{
		dbapiStr("127.0.0.1"),
		{isNull: true},
		dbapiStr("SELECT 1"),
	}
	payload := []byte{dbapiOpConnect}
	payload = dbapiAppendU32(payload, uint32(len(fields)))
	for _, f := range fields {
		payload = dbapiAppendField(payload, f)
	}

	// decodifica como o bridge faria
	if payload[0] != dbapiOpConnect {
		t.Fatalf("opcode errado: %d", payload[0])
	}
	p := &dbapiParser{buf: payload[1:]}
	nf, err := p.u32()
	if err != nil || nf != 3 {
		t.Fatalf("nfields: %d err=%v", nf, err)
	}
	fs, err := p.fields(nf)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if dbapiFieldStr(fs[0]) != "127.0.0.1" {
		t.Fatalf("field0 = %q", dbapiFieldStr(fs[0]))
	}
	if !fs[1].isNull {
		t.Fatal("field1 deveria ser NULL")
	}
	if dbapiFieldStr(fs[2]) != "SELECT 1" {
		t.Fatalf("field2 = %q", dbapiFieldStr(fs[2]))
	}
}

// TestDbapiSQLEngineIntegration é o teste E2E real: bridge + dbaccess64 +
// Postgres. Só roda quando ADVPP_DBAPI_IT=1 e o ambiente está de pé:
//
//	ADVPP_DBAPI_BRIDGE=127.0.0.1:17890  (advpp-dbapi-bridge rodando)
//	DbConnection cfg: Host=127.0.0.1 Port=7890 Service=POSTGRES/p1212510
func TestDbapiSQLEngineIntegration(t *testing.T) {
	if os.Getenv("ADVPP_DBAPI_IT") != "1" {
		t.Skip("defina ADVPP_DBAPI_IT=1 com bridge+dbaccess no ar")
	}
	cfg := ConnConfig{
		Host:    "127.0.0.1",
		Port:    7890,
		Service: "POSTGRES/p1212510",
	}
	eng, err := NewDbapiEngineFrom(cfg)
	if err != nil {
		t.Fatalf("NewDbapiEngineFrom: %v", err)
	}
	defer eng.Close()

	// read path: SelectArea sobre tabela real do Protheus
	if err := eng.SelectArea("SX2990"); err != nil {
		t.Fatalf("SelectArea SX2990: %v", err)
	}
	if eng.RecCount() == 0 {
		t.Fatal("SX2990 vazio — esperado dicionário populado")
	}
	t.Logf("SX2990: %d registros, %d colunas", eng.RecCount(), len(eng.columns))

	if err := eng.GoTop(); err != nil {
		t.Fatalf("GoTop: %v", err)
	}
	v, err := eng.FieldGet("X2_NOME")
	if err != nil {
		t.Fatalf("FieldGet: %v", err)
	}
	if v.String() == "" {
		t.Fatal("X2_NOME vazio no primeiro registro")
	}
	t.Logf("primeiro X2_NOME: %q", v.String())

	// navegação
	if err := eng.Skip(1); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if eng.RecNo() != 2 {
		t.Fatalf("RecNo após Skip(1): %d, esperado 2", eng.RecNo())
	}

	// SQLEngine.QueryRows
	rows, err := eng.QueryRows("SELECT X2_CHAVE, X2_NOME FROM SX2990 LIMIT 2")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("QueryRows retornou %d linhas, esperado 2", len(rows))
	}
	for _, r := range rows {
		if r["X2_CHAVE"] == "" {
			t.Fatal("X2_CHAVE vazio no resultado")
		}
	}
	t.Logf("QueryRows OK: %d linhas", len(rows))

	// erro SQL deve voltar como erro Go, não crash
	if _, err := eng.QueryRows("SELECT * FROM TABELA_INEXISTENTE_XX"); err == nil {
		t.Fatal("QueryRows em tabela inexistente deveria falhar")
	} else {
		t.Logf("erro esperado capturado: %v", err)
	}

	// engine continua viva após o erro
	if _, err := eng.QueryRows("SELECT 1 AS UM"); err != nil {
		t.Fatalf("engine morreu após erro anterior: %v", err)
	}

	// seek indexado: "SD2" é prefixo do índice 1 (X2_CHAVE...) — Seek2
	// posiciona direto; "ZZZINEXISTENTE" deve dar miss via fallback.
	if ok, err := eng.Seek("SD2"); err != nil || !ok {
		t.Fatalf("Seek(SD2): ok=%v err=%v", ok, err)
	}
	if eng.RecNo() != 7840 {
		t.Logf("Seek(SD2) recno=%d (esperado ~7840 se índice pegou)", eng.RecNo())
	}
	if ok, err := eng.Seek("ZZZINEXISTENTE123"); err != nil || ok {
		t.Fatalf("Seek inexistente: ok=%v err=%v", ok, err)
	}

	// escrita: update+restore em SX2990->X2_NOME (workarea remota,
	// RecLock/FieldPut/MsUnlock → Lock/SetFieldN/Update/UnLock no dbapi)
	if err := eng.SelectArea("SX2990"); err != nil {
		t.Fatalf("SelectArea p/ escrita: %v", err)
	}
	if err := eng.GoTop(); err != nil {
		t.Fatalf("GoTop p/ escrita: %v", err)
	}
	orig, err := eng.FieldGet("X2_NOME")
	if err != nil || orig.String() == "" {
		t.Fatalf("FieldGet orig: %v %q", err, orig)
	}
	if err := eng.RecLock(); err != nil {
		t.Fatalf("RecLock: %v", err)
	}
	if err := eng.FieldPut("X2_NOME", advplrt.NewString("TESTE WRITE FASE3")); err != nil {
		t.Fatalf("FieldPut: %v", err)
	}
	if err := eng.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock: %v", err)
	}
	chk, err := eng.FieldGet("X2_NOME")
	if err != nil {
		t.Fatalf("FieldGet pós-update: %v", err)
	}
	if !strings.HasPrefix(chk.String(), "TESTE WRITE FASE3") {
		t.Fatalf("update não persistiu: %q", chk.String())
	}
	// restaura o valor original (campo C de 30 bytes, padding inclusos)
	if err := eng.RecLock(); err != nil {
		t.Fatalf("RecLock restore: %v", err)
	}
	if err := eng.FieldPut("X2_NOME", orig); err != nil {
		t.Fatalf("FieldPut restore: %v", err)
	}
	if err := eng.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock restore: %v", err)
	}
	restored, err := eng.FieldGet("X2_NOME")
	if err != nil {
		t.Fatalf("FieldGet restore: %v", err)
	}
	if restored.String() != orig.String() {
		t.Fatalf("restore divergiu: %q != %q", restored.String(), orig.String())
	}
	t.Logf("write path OK: update+restore em X2_NOME")

	// transação explícita (TCCommit 1/2/3/4 → StrCmtCtl/Commit/Rollback/
	// EndCmtCtl): escrita dentro da tx fica pendente; RollbackTx descarta,
	// CommitTx persiste. Verificação via SQL (o buffer da workarea não é
	// invalidado por rollback — leitura própria continuaria stale).
	nomeSQL := func() string {
		rows, err := eng.QueryRows("SELECT X2_NOME FROM SX2990 WHERE R_E_C_N_O_=1")
		if err != nil || len(rows) == 0 {
			t.Fatalf("QueryRows p/ tx check: %v rows=%d", err, len(rows))
		}
		return rows[0]["X2_NOME"]
	}
	base := nomeSQL()

	if err := eng.BeginTx(); err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := eng.RecLock(); err != nil {
		t.Fatalf("RecLock tx: %v", err)
	}
	if err := eng.FieldPut("X2_NOME", advplrt.NewString("TXPENDENTE")); err != nil {
		t.Fatalf("FieldPut tx: %v", err)
	}
	if err := eng.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock tx: %v", err)
	}
	if err := eng.RollbackTx(); err != nil {
		t.Fatalf("RollbackTx: %v", err)
	}
	if err := eng.EndTx(); err != nil {
		t.Fatalf("EndTx rollback: %v", err)
	}
	if got := nomeSQL(); got != base {
		t.Fatalf("rollback não descartou: %q != %q", got, base)
	}

	if err := eng.BeginTx(); err != nil {
		t.Fatalf("BeginTx 2: %v", err)
	}
	if err := eng.RecLock(); err != nil {
		t.Fatalf("RecLock tx2: %v", err)
	}
	if err := eng.FieldPut("X2_NOME", advplrt.NewString("TXCOMMITED")); err != nil {
		t.Fatalf("FieldPut tx2: %v", err)
	}
	if err := eng.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock tx2: %v", err)
	}
	if err := eng.CommitTx(); err != nil {
		t.Fatalf("CommitTx: %v", err)
	}
	if err := eng.EndTx(); err != nil {
		t.Fatalf("EndTx commit: %v", err)
	}
	if got := nomeSQL(); !strings.HasPrefix(got, "TXCOMMITED") {
		t.Fatalf("commit não persistiu: %q", got)
	}

	// restaura fora de tx (autocommit)
	if err := eng.RecLock(); err != nil {
		t.Fatalf("RecLock restore2: %v", err)
	}
	if err := eng.FieldPut("X2_NOME", orig); err != nil {
		t.Fatalf("FieldPut restore2: %v", err)
	}
	if err := eng.MsUnlock(); err != nil {
		t.Fatalf("MsUnlock restore2: %v", err)
	}
	t.Logf("tx path OK: rollback descartou, commit persistiu")
}
