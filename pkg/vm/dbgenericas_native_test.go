package vm

import (
	"path/filepath"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newDBGenVM cria uma VM com as natives de Banco de Dados genéricas registradas
// manualmente (o registro central em natives.go é feito à parte) e um engine
// SQLite real.
func newDBGenVM(t *testing.T, dbName string) (*VM, map[string]func(args []advplrt.Value) (advplrt.Value, error), *db.SQLiteEngine) {
	t.Helper()
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(filepath.Join(tmpDir, dbName))
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerDbgenericasNatives(natives)
	return v, natives, eng
}

// createTSTable cria a tabela de teste com estrutura AdvPL típica
// (R_E_C_N_O_, D_E_L_E_T_, CODNUM N, NOME C, VALOR N).
func createTSTable(t *testing.T, eng *db.SQLiteEngine, name string) {
	t.Helper()
	if err := eng.Exec(`CREATE TABLE ` + name + ` (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		CODNUM INTEGER,
		NOME TEXT,
		VALOR REAL
	)`); err != nil {
		t.Fatalf("create table %s: %v", name, err)
	}
}

// TestDBSqlExecCriaInsereSeleciona testa DBSQLEXEC: cria tabela, insere linha e
// seleciona via engine (a área corrente passa a apontar para o resultado).
func TestDBSqlExecCriaInsereSeleciona(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "sqlexec.db")
	defer eng.Close()

	// CREATE TABLE
	got, err := natives["DBSQLEXEC"]([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString("CREATE TABLE T_Q (A INTEGER, B TEXT)"),
		advplrt.NewString("SQLITE_SYS"),
	})
	if err != nil {
		t.Fatalf("DBSQLEXEC(CREATE) erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DBSQLEXEC(CREATE) retornou %v, esperado .T.", got)
	}

	// INSERT
	got, err = natives["DBSQLEXEC"]([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString("INSERT INTO T_Q (A, B) VALUES (1, 'x')"),
		advplrt.NewString("SQLITE_SYS"),
	})
	if err != nil || got != advplrt.True {
		t.Fatalf("DBSQLEXEC(INSERT) got=%v err=%v", got, err)
	}

	// SELECT materializa a área de trabalho T_RES
	got, err = natives["DBSQLEXEC"]([]advplrt.Value{
		advplrt.NewString("T_RES"),
		advplrt.NewString("SELECT A, B FROM T_Q"),
		advplrt.NewString("SQLITE_SYS"),
	})
	if err != nil || got != advplrt.True {
		t.Fatalf("DBSQLEXEC(SELECT) got=%v err=%v", got, err)
	}
	if v.currentAlias != "T_RES" {
		t.Fatalf("currentAlias=%q, esperado T_RES", v.currentAlias)
	}
	if v.dbEngine.RecCount() != 1 {
		t.Fatalf("RecCount=%d, esperado 1", v.dbEngine.RecCount())
	}
	fv, _ := v.dbEngine.FieldGet("B")
	if advplrt.ToString(fv) != "x" {
		t.Fatalf("FieldGet(B)=%q, esperado x", advplrt.ToString(fv))
	}
}

// TestDBUseAreaEFCount testa DBUSEAREA abrindo a tabela e FCOUNT contando campos.
func TestDBUseAreaEFCount(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "usearea.db")
	defer eng.Close()
	createTSTable(t, eng, "T_PEOPLE")

	got, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True,
		advplrt.NewString("TOPCONN"),
		advplrt.NewString("T_PEOPLE"),
		advplrt.NewString("PES"),
		advplrt.False,
		advplrt.False,
	})
	if err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	if got != advplrt.Nil {
		t.Fatalf("DBUSEAREA retornou %v, esperado Nil", got)
	}
	if v.currentAlias != "PES" {
		t.Fatalf("currentAlias=%q, esperado PES", v.currentAlias)
	}

	// FCOUNT: 4 campos de negócio (CODNUM, NOME, VALOR + D_E_L_E_T_ físico)
	got, err = natives["FCOUNT"](nil)
	if err != nil {
		t.Fatalf("FCOUNT erro: %v", err)
	}
	n := int(advplrt.ToFloat(got))
	if n < 3 {
		t.Fatalf("FCOUNT=%d, esperado >= 3 (CODNUM, NOME, VALOR)", n)
	}
}

// TestDBStruct testa a estrutura da tabela (nome, tipo, tamanho, decimais).
func TestDBStruct(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "struct.db")
	defer eng.Close()
	createTSTable(t, eng, "T_STR")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_STR"),
		advplrt.NewString("STR"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	got, err := natives["DBSTRUCT"](nil)
	if err != nil {
		t.Fatalf("DBSTRUCT erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) == 0 {
		t.Fatalf("DBSTRUCT retornou %v, esperado array não vazio", got)
	}
	// Procura o campo NOME na estrutura
	found := false
	for _, row := range arr.Elements {
		r, ok := row.(*advplrt.ArrayValue)
		if !ok || len(r.Elements) < 4 {
			continue
		}
		if advplrt.ToString(r.Elements[0]) == "NOME" {
			found = true
			if advplrt.ToString(r.Elements[1]) != "C" {
				t.Errorf("NOME type=%q, esperado C", advplrt.ToString(r.Elements[1]))
			}
			if advplrt.ToFloat(r.Elements[2]) <= 0 {
				t.Errorf("NOME size=%v, esperado > 0", r.Elements[2])
			}
			break
		}
	}
	if !found {
		t.Fatalf("campo NOME não encontrado em DBSTRUCT")
	}
}

// TestDBTblCopy testa a cópia de dados entre tabelas SQLite.
func TestDBTblCopy(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "tblcopy.db")
	defer eng.Close()
	createTSTable(t, eng, "T_SRC")
	createTSTable(t, eng, "T_DST")

	// Dados na origem
	if err := eng.Exec("INSERT INTO T_SRC (CODNUM, NOME, VALOR) VALUES (1, 'A', 10.5), (2, 'B', 20)"); err != nil {
		t.Fatalf("insert src: %v", err)
	}

	got, err := natives["DBTBLCOPY"]([]advplrt.Value{
		advplrt.NewString("T_SRC"),
		advplrt.NewString("T_DST"),
	})
	if err != nil {
		t.Fatalf("DBTBLCOPY erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DBTBLCOPY retornou %v, esperado .T.", got)
	}
	rows, err := eng.QueryRows("SELECT NOME, VALOR FROM T_DST ORDER BY CODNUM")
	if err != nil {
		t.Fatalf("query dest: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("dest tem %d linhas, esperado 2", len(rows))
	}
	if rows[0]["NOME"] != "A" || rows[1]["NOME"] != "B" {
		t.Errorf("dest NOMEs = %v/%v, esperado A/B", rows[0]["NOME"], rows[1]["NOME"])
	}
}

// TestDBUnlockAndSetDriver testa DBUNLOCK/DBUNLOCKALL/DBRLOCK/DBRLOCKLIST/DBRUNLOCK
// e DBSETDRIVER retornando sem erro.
func TestDBUnlockAndSetDriver(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "lock.db")
	defer eng.Close()
	createTSTable(t, eng, "T_LOCK")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_LOCK"),
		advplrt.NewString("LCK"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	// DBSETDRIVER: default é DBFCDX
	got, err := natives["DBSETDRIVER"](nil)
	if err != nil {
		t.Fatalf("DBSETDRIVER erro: %v", err)
	}
	if advplrt.ToString(got) != "DBFCDX" {
		t.Fatalf("DBSETDRIVER()=%q, esperado DBFCDX", advplrt.ToString(got))
	}
	// Altera para TOPCONN, retorna o anterior
	got, err = natives["DBSETDRIVER"]([]advplrt.Value{advplrt.NewString("TOPCONN")})
	if err != nil || advplrt.ToString(got) != "DBFCDX" {
		t.Fatalf("DBSETDRIVER(TOPCONN) got=%q err=%v, esperado anterior DBFCDX", advplrt.ToString(got), err)
	}
	// RDD inválida não altera
	got, _ = natives["DBSETDRIVER"]([]advplrt.Value{advplrt.NewString("NOTEXISTS")})
	if advplrt.ToString(got) != "TOPCONN" {
		t.Fatalf("DBSETDRIVER(NOTEXISTS)=%q, esperado TOPCONN (inalterado)", advplrt.ToString(got))
	}

	// DBRLock sem nRec -> .T.
	got, err = natives["DBRLOCK"](nil)
	if err != nil || got != advplrt.True {
		t.Fatalf("DBRLOCK() got=%v err=%v, esperado .T.", got, err)
	}
	// DBRLockList contém o recno corrente (1)
	got, err = natives["DBRLOCKLIST"](nil)
	if err != nil {
		t.Fatalf("DBRLOCKLIST erro: %v", err)
	}
	if arr, ok := got.(*advplrt.ArrayValue); ok {
		if len(arr.Elements) == 0 {
			t.Errorf("DBRLOCKLIST vazio após DBRLOCK, esperado [1]")
		}
	} else {
		t.Errorf("DBRLOCKLIST não retornou array: %v", got)
	}
	// DBRUnlock -> .T., lista volta a vazio
	got, err = natives["DBRUNLOCK"](nil)
	if err != nil || got != advplrt.True {
		t.Fatalf("DBRUNLOCK() got=%v err=%v, esperado .T.", got, err)
	}
	got, _ = natives["DBRLOCKLIST"](nil)
	if arr, ok := got.(*advplrt.ArrayValue); ok {
		if len(arr.Elements) != 0 {
			t.Errorf("DBRLOCKLIST após DBRUNLOCK=%d elem, esperado 0", len(arr.Elements))
		}
	}

	// DBUNLOCK / DBUNLOCKALL sem erro
	if _, err := natives["DBUNLOCK"](nil); err != nil {
		t.Fatalf("DBUNLOCK erro: %v", err)
	}
	if _, err := natives["DBUNLOCKALL"](nil); err != nil {
		t.Fatalf("DBUNLOCKALL erro: %v", err)
	}
}

// TestDeletedFalse verifica DELETED() = .F. para registro não marcado e .T.
// para registro marcado para exclusão (soft-delete D_E_L_E_T_ = '*').
func TestDeletedFalse(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "deleted.db")
	defer eng.Close()
	createTSTable(t, eng, "T_DEL")
	if err := eng.Exec("INSERT INTO T_DEL (CODNUM, NOME, VALOR) VALUES (1, 'A', 1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_DEL"),
		advplrt.NewString("DEL"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	got, err := natives["DELETED"](nil)
	if err != nil {
		t.Fatalf("DELETED erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("DELETED()=%v, esperado .F.", got)
	}

	// Marca o registro como excluído (soft-delete) e verifica .T.
	if err := eng.Exec("UPDATE T_DEL SET D_E_L_E_T_ = '*' WHERE R_E_C_N_O_ = 1"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// Recarrega a área (records em memória não são atualizados) e reposiciona
	v.dbEngine.SelectArea("T_DEL")
	got, err = natives["DELETED"](nil)
	if err != nil {
		t.Fatalf("DELETED erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DELETED()=%v, esperado .T. após marcar exclusão", got)
	}
}

// TestDBSetActFldEFCount testa que DBSETACTFLD desativa um campo e FCOUNT passa
// a desconsiderá-lo.
func TestDBSetActFldEFCount(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "actfld.db")
	defer eng.Close()
	createTSTable(t, eng, "T_ACT")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_ACT"),
		advplrt.NewString("ACT"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	before, _ := natives["FCOUNT"](nil)
	nBefore := int(advplrt.ToFloat(before))

	got, err := natives["DBSETACTFLD"]([]advplrt.Value{
		advplrt.NewString("NOME"),
		advplrt.False,
	})
	if err != nil {
		t.Fatalf("DBSETACTFLD erro: %v", err)
	}
	if got != advplrt.Nil {
		t.Fatalf("DBSETACTFLD retornou %v, esperado Nil", got)
	}

	after, _ := natives["FCOUNT"](nil)
	nAfter := int(advplrt.ToFloat(after))
	if nAfter != nBefore-1 {
		t.Fatalf("FCOUNT antes=%d depois=%d, esperado %d (campo NOME oculto)", nBefore, nAfter, nBefore-1)
	}
}

// TestDBOrderInfoENickname testa DBSetIndex/DBSetNickname/DBOrderNickname/DBOrderInfo.
func TestDBOrderInfoENickname(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "order.db")
	defer eng.Close()
	createTSTable(t, eng, "T_ORD")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_ORD"),
		advplrt.NewString("ORD"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	// Sem índices: DBOrderInfo(7) -> "", (9) -> 0
	got, _ := natives["DBORDERINFO"]([]advplrt.Value{advplrt.NewNumber(7)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("DBOrderInfo(7) sem índice=%q, esperado \"\"", advplrt.ToString(got))
	}
	got, _ = natives["DBORDERINFO"]([]advplrt.Value{advplrt.NewNumber(9)})
	if advplrt.ToFloat(got) != 0 {
		t.Fatalf("DBOrderInfo(9) sem índice=%v, esperado 0", got)
	}

	// DBSetIndex + DBSetNickname + DBOrderNickname
	if _, err := natives["DBSETINDEX"]([]advplrt.Value{advplrt.NewString("T_ORD_IND")}); err != nil {
		t.Fatalf("DBSETINDEX: %v", err)
	}
	got, err := natives["DBSETNICKNAME"]([]advplrt.Value{
		advplrt.NewString("T_ORD_IND"),
		advplrt.NewString("NICK1"),
	})
	if err != nil || advplrt.ToString(got) != "NICK1" {
		t.Fatalf("DBSETNICKNAME got=%q err=%v, esperado NICK1", advplrt.ToString(got), err)
	}
	got, err = natives["DBORDERNICKNAME"]([]advplrt.Value{advplrt.NewString("NICK1")})
	if err != nil || got != advplrt.True {
		t.Fatalf("DBORDERNICKNAME got=%v err=%v, esperado .T.", got, err)
	}
	got, _ = natives["DBORDERINFO"]([]advplrt.Value{advplrt.NewNumber(7)})
	if advplrt.ToString(got) != "T_ORD_IND" {
		t.Fatalf("DBOrderInfo(7)=%q, esperado T_ORD_IND", advplrt.ToString(got))
	}
	got, _ = natives["DBORDERINFO"]([]advplrt.Value{advplrt.NewNumber(9)})
	if advplrt.ToFloat(got) != 1 {
		t.Fatalf("DBOrderInfo(9)=%v, esperado 1", got)
	}
	// Apelido inexistente -> .F.
	got, _ = natives["DBORDERNICKNAME"]([]advplrt.Value{advplrt.NewString("NAO_EXISTE")})
	if got != advplrt.False {
		t.Fatalf("DBORDERNICKNAME inexistente=%v, esperado .F.", got)
	}
}

// TestDBRecordInfo testa os 3 tipos de DBRecordInfo e retorno Nil para tipo inválido.
func TestDBRecordInfo(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "recinfo.db")
	defer eng.Close()
	createTSTable(t, eng, "T_REC")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_REC"),
		advplrt.NewString("REC"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	// DBRI_DELETED (1) -> .F.
	got, err := natives["DBRECORDINFO"]([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil || got != advplrt.False {
		t.Fatalf("DBRecordInfo(1) got=%v err=%v, esperado .F.", got, err)
	}
	// DBRI_RECSIZE (3) -> número > 0
	got, err = natives["DBRECORDINFO"]([]advplrt.Value{advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("DBRecordInfo(3) erro: %v", err)
	}
	if advplrt.ToFloat(got) <= 0 {
		t.Fatalf("DBRecordInfo(3)=%v, esperado > 0", got)
	}
	// DBRI_UPDATED (5) -> .F.
	got, err = natives["DBRECORDINFO"]([]advplrt.Value{advplrt.NewNumber(5)})
	if err != nil || got != advplrt.False {
		t.Fatalf("DBRecordInfo(5) got=%v err=%v, esperado .F.", got, err)
	}
	// Tipo inválido -> Nil
	got, err = natives["DBRECORDINFO"]([]advplrt.Value{advplrt.NewNumber(99)})
	if err != nil {
		t.Fatalf("DBRecordInfo(99) erro: %v", err)
	}
	if got != advplrt.Nil {
		t.Fatalf("DBRecordInfo(99)=%v, esperado Nil", got)
	}
}

// TestDBRecall testa DBRECALL revertendo a marca de exclusão do registro corrente.
func TestDBRecall(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "recall.db")
	defer eng.Close()
	createTSTable(t, eng, "T_RCL")
	if err := eng.Exec("INSERT INTO T_RCL (CODNUM, NOME, VALOR) VALUES (1, 'A', 1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Marca como excluído para que DBRECALL tenha efeito
	if err := eng.Exec("UPDATE T_RCL SET D_E_L_E_T_ = '*' WHERE R_E_C_N_O_ = 1"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("TOPCONN"), advplrt.NewString("T_RCL"),
		advplrt.NewString("RCL"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA: %v", err)
	}

	got, err := natives["DELETED"](nil)
	if err != nil || got != advplrt.True {
		t.Fatalf("DELETED antes=%v err=%v, esperado .T.", got, err)
	}

	got, err = natives["DBRECALL"](nil)
	if err != nil {
		t.Fatalf("DBRECALL erro: %v", err)
	}
	if got != advplrt.Nil {
		t.Fatalf("DBRECALL retornou %v, esperado Nil", got)
	}

	got, err = natives["DELETED"](nil)
	if err != nil || got != advplrt.False {
		t.Fatalf("DELETED depois=%v err=%v, esperado .F.", got, err)
	}
}

// TestDBSqlPlan testa DBSQLPLAN preenchendo aResult (array) com o plano.
func TestDBSqlPlan(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "plan.db")
	defer eng.Close()
	createTSTable(t, eng, "T_PLAN")

	result := advplrt.NewArray([]advplrt.Value{})
	got, err := natives["DBSQLPLAN"]([]advplrt.Value{
		advplrt.NewString("SELECT * FROM T_PLAN"),
		advplrt.NewString("SQLITE_SYS"),
		result,
		advplrt.NewNumber(1),
	})
	if err != nil {
		t.Fatalf("DBSQLPLAN erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DBSQLPLAN retornou %v, esperado .T.", got)
	}
	if len(result.Elements) < 2 {
		t.Fatalf("aResult com %d elems, esperado >= 2 (header + linhas)", len(result.Elements))
	}
	// Cabeçalho
	header, ok := result.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(header.Elements) != 4 {
		t.Fatalf("header=%v, esperado array de 4 (id,parent,notused,detail)", result.Elements[0])
	}
}
