package vm

import (
	"path/filepath"
	"strings"
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

// TestDBChangeAlias testa DBCHANGEALIAS movendo o estado para novo alias.
func TestDBChangeAlias(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "chg.db")
	defer eng.Close()
	createTSTable(t, eng, "T_CHG")

	// Abre a tabela como alias T_CHG
	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_CHG"),
		advplrt.NewString("T_CHG"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Muda o alias para T2
	if _, err := natives["DBCHANGEALIAS"]([]advplrt.Value{
		advplrt.NewString("T_CHG"), advplrt.NewString("T2"),
	}); err != nil {
		t.Fatalf("DBCHANGEALIAS erro: %v", err)
	}
	if v.currentAlias != "T2" {
		t.Fatalf("currentAlias=%q, esperado T2", v.currentAlias)
	}
	// O engine SQLite não tem conceito de alias lógico (SelectArea usa o nome
	// físico); DBInfo(33) reflete currentAlias do VM.
	got, err := natives["DBINFO"]([]advplrt.Value{advplrt.NewNumber(33)})
	if err != nil {
		t.Fatalf("DBINFO erro: %v", err)
	}
	if advplrt.ToString(got) != "T2" {
		t.Fatalf("DBINFO(33)=%q, esperado T2", advplrt.ToString(got))
	}
}

// TestDBCreate testa DBCREATE criando tabela real com tipos AdvPL.
func TestDBCreate(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "create.db")
	defer eng.Close()

	aStruct := advplrt.NewArray([]advplrt.Value{
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewString("Cod"), advplrt.NewString("N"), advplrt.NewNumber(3), advplrt.NewNumber(0),
		}),
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewString("Nome"), advplrt.NewString("C"), advplrt.NewNumber(10), advplrt.NewNumber(0),
		}),
	})
	if _, err := natives["DBCREATE"]([]advplrt.Value{
		advplrt.NewString("T_NEW"), aStruct, advplrt.NewString("SQLITE"),
	}); err != nil {
		t.Fatalf("DBCREATE erro: %v", err)
	}
	cols := eng.FieldPos("CODNUM")
	if cols != 0 {
		t.Fatalf("CODNUM não deveria existir, FieldPos=%d", cols)
	}
	// Verifica estrutura via PRAGMA indireto: usa DBUSEAREA na tabela criada
	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_NEW"),
		advplrt.NewString("T_NEW"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	if eng.FieldPos("COD") == 0 {
		t.Fatalf("campo COD não encontrado na tabela criada")
	}
	if eng.FieldPos("NOME") == 0 {
		t.Fatalf("campo NOME não encontrado na tabela criada")
	}
}

// TestDBFieldInfo testa DBFIELDINFO com as constantes DBS_* (1..4).
func TestDBFieldInfo(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "fieldinfo.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FI")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_FI"),
		advplrt.NewString("T_FI"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Colunas do usuário (sem R_E_C_N_O_/D_E_L_E_T_): CODNUM, NOME, VALOR
	name, err := natives["DBFIELDINFO"]([]advplrt.Value{advplrt.NewNumber(1), advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("DBFIELDINFO(1,1) erro: %v", err)
	}
	if advplrt.ToString(name) != "CODNUM" {
		t.Fatalf("DBFIELDINFO(NAME,1)=%q, esperado CODNUM", advplrt.ToString(name))
	}
	typ, err := natives["DBFIELDINFO"]([]advplrt.Value{advplrt.NewNumber(2), advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("DBFIELDINFO(2,1) erro: %v", err)
	}
	if advplrt.ToString(typ) != "N" {
		t.Fatalf("DBFIELDINFO(TYPE,1)=%q, esperado N", advplrt.ToString(typ))
	}
	// Campo 2 (NOME) deve ser "C"
	typ2, err := natives["DBFIELDINFO"]([]advplrt.Value{advplrt.NewNumber(2), advplrt.NewNumber(2)})
	if err != nil {
		t.Fatalf("DBFIELDINFO(2,2) erro: %v", err)
	}
	if advplrt.ToString(typ2) != "C" {
		t.Fatalf("DBFIELDINFO(TYPE,2)=%q, esperado C", advplrt.ToString(typ2))
	}
	// Campo 3 (VALOR) deve ser "N"
	typ3, err := natives["DBFIELDINFO"]([]advplrt.Value{advplrt.NewNumber(2), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("DBFIELDINFO(2,3) erro: %v", err)
	}
	if advplrt.ToString(typ3) != "N" {
		t.Fatalf("DBFIELDINFO(TYPE,3)=%q, esperado N", advplrt.ToString(typ3))
	}
	_ = v
}

// TestDBGetActFld testa DBGETACTFLD com e sem campos restritos.
func TestDBGetActFld(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "actfld.db")
	defer eng.Close()
	createTSTable(t, eng, "T_AF")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_AF"),
		advplrt.NewString("T_AF"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Todos ativos => "*"
	got, err := natives["DBGETACTFLD"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBGETACTFLD erro: %v", err)
	}
	if advplrt.ToString(got) != "*" {
		t.Fatalf("DBGETACTFLD()=%q, esperado *", advplrt.ToString(got))
	}
	// Desabilita NOME
	if _, err := natives["DBSETACTFLD"]([]advplrt.Value{
		advplrt.NewString("NOME"), advplrt.False,
	}); err != nil {
		t.Fatalf("DBSETACTFLD erro: %v", err)
	}
	got, err = natives["DBGETACTFLD"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBGETACTFLD erro: %v", err)
	}
	out := advplrt.ToString(got)
	if strings.Contains(out, "NOME") {
		t.Fatalf("DBGETACTFLD()=%q, NOME não deveria estar ativo", out)
	}
	if !strings.Contains(out, "CODNUM") {
		t.Fatalf("DBGETACTFLD()=%q, CODNUM deveria estar ativo", out)
	}
}

// TestDBGoToDBInInsert testa DBGOTO e DBININSERT (fluxo append->commit).
func TestDBGoToDBInInsert(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "goto.db")
	defer eng.Close()
	createTSTable(t, eng, "T_GO")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_GO"),
		advplrt.NewString("T_GO"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Antes de append: .F.
	got, err := natives["DBININSERT"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBININSERT erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("DBININSERT() antes=%v, esperado .F.", got)
	}
	// Append (engine diretamente — DBAPPEND vive no mapa inline de natives.go,
	// não no map de newDBGenVM) -> .T.
	if err := eng.Append(); err != nil {
		t.Fatalf("Append erro: %v", err)
	}
	got, err = natives["DBININSERT"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBININSERT erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DBININSERT() após append=%v, esperado .T.", got)
	}
	// Preenche e commit (SetInserting direto) -> .F.
	if err := eng.FieldPut("NOME", advplrt.NewString("ABC")); err != nil {
		t.Fatalf("FieldPut erro: %v", err)
	}
	eng.SetInserting(false)
	got, err = natives["DBININSERT"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBININSERT erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("DBININSERT() após commit=%v, esperado .F.", got)
	}
	// DBGOTO posiciona no recno 1
	if _, err := natives["DBGOTO"]([]advplrt.Value{advplrt.NewNumber(1)}); err != nil {
		t.Fatalf("DBGOTO erro: %v", err)
	}
	if eng.RecNo() != 1 {
		t.Fatalf("RecNo após DBGOTO(1)=%d, esperado 1", eng.RecNo())
	}
}

// TestDBFilterEDBClearAllFilter testa DBFILTER, DBFILTERCB e DBCLEARALLFILTER.
func TestDBFilterEDBClearAllFilter(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "filter.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FL")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_FL"),
		advplrt.NewString("T_FL"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Sem filtro => ""
	got, err := natives["DBFILTER"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBFILTER erro: %v", err)
	}
	if advplrt.ToString(got) != "" {
		t.Fatalf("DBFILTER()=%q, esperado \"\"", advplrt.ToString(got))
	}
	// Popula o estado de filtro via DBCLEARALLFILTER (garante map vazio e
	// registra implicitamente) e depois grava expressão direto no estado.
	s := v.dbGenStateFor()
	s.mu.Lock()
	s.filters["T_FL"] = "CODNUM=1"
	s.mu.Unlock()
	got, err = natives["DBFILTER"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBFILTER erro: %v", err)
	}
	if advplrt.ToString(got) != "CODNUM=1" {
		t.Fatalf("DBFILTER()=%q, esperado CODNUM=1", advplrt.ToString(got))
	}
	// DBFILTERCB sem codeblock => NIL
	cb, err := natives["DBFILTERCB"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBFILTERCB erro: %v", err)
	}
	if !advplrt.IsNil(cb) {
		t.Fatalf("DBFILTERCB()=%v, esperado NIL", cb)
	}
	// DBCLEARALLFILTER limpa tudo
	if _, err := natives["DBCLEARALLFILTER"]([]advplrt.Value{}); err != nil {
		t.Fatalf("DBCLEARALLFILTER erro: %v", err)
	}
	got, err = natives["DBFILTER"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DBFILTER erro: %v", err)
	}
	if advplrt.ToString(got) != "" {
		t.Fatalf("DBFILTER() após clear=%q, esperado \"\"", advplrt.ToString(got))
	}
}

// TestDBInfoDBClearIndexDBCloseAll testa DBINFO, DBCLEARINDEX e DBCLOSEALL.
func TestDBInfoDBClearIndexDBCloseAll(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "info.db")
	defer eng.Close()
	createTSTable(t, eng, "T_IN")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_IN"),
		advplrt.NewString("T_IN"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// DBI_ALIAS (33)
	got, err := natives["DBINFO"]([]advplrt.Value{advplrt.NewNumber(33)})
	if err != nil {
		t.Fatalf("DBINFO(33) erro: %v", err)
	}
	if advplrt.ToString(got) != "T_IN" {
		t.Fatalf("DBINFO(33)=%q, esperado T_IN", advplrt.ToString(got))
	}
	// DBI_EOF (27) em tabela vazia: current=0, len(records)=0 => EOF .T.
	got, err = natives["DBINFO"]([]advplrt.Value{advplrt.NewNumber(27)})
	if err != nil {
		t.Fatalf("DBINFO(27) erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("DBINFO(27)=%v, esperado .T. (EOF em tabela vazia)", got)
	}
	// DBCLEARINDEX não deve quebrar
	if _, err := natives["DBCLEARINDEX"]([]advplrt.Value{}); err != nil {
		t.Fatalf("DBCLEARINDEX erro: %v", err)
	}
	// DBCLOSEALL fecha a área
	if _, err := natives["DBCLOSEALL"]([]advplrt.Value{}); err != nil {
		t.Fatalf("DBCLOSEALL erro: %v", err)
	}
	if v.currentAlias != "" {
		t.Fatalf("currentAlias após DBCLOSEALL=%q, esperado \"\"", v.currentAlias)
	}
	// GetDBExtension
	ext, err := natives["GETDBEXTENSION"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("GETDBEXTENSION erro: %v", err)
	}
	if advplrt.ToString(ext) != ".dbf" {
		t.Fatalf("GETDBEXTENSION()=%q, esperado .dbf", advplrt.ToString(ext))
	}
}

// TestDBCommitAllAndDBFound testa DBCOMMITALL e o estado de Found via DBSEEK.
func TestDBCommitAllAndDBFound(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "commit.db")
	defer eng.Close()
	createTSTable(t, eng, "T_CM")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_CM"),
		advplrt.NewString("T_CM"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// DBCOMMITALL é no-op seguro
	if _, err := natives["DBCOMMITALL"]([]advplrt.Value{}); err != nil {
		t.Fatalf("DBCOMMITALL erro: %v", err)
	}
	// DBSEEK não encontrado (engine direto — DBSEEK vive no mapa inline de
	// natives.go; o fluxo real que popula lastFound é via dbGenSetFound) =>
	// Found .F.
	found, err := eng.Seek("NAO_EXISTE")
	if err != nil {
		t.Fatalf("Seek erro: %v", err)
	}
	if found {
		t.Fatalf("Seek(NAO_EXISTE) encontrou, esperado não")
	}
	got, err := natives["DBINFO"]([]advplrt.Value{advplrt.NewNumber(29)})
	if err != nil {
		t.Fatalf("DBINFO(29) erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("DBINFO(29) após seek falho=%v, esperado .F.", got)
	}
}

// TestField testa FIELD com nPos 1-based sobre campos do usuário.
func TestField(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "field.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FLD")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_FLD"),
		advplrt.NewString("T_FLD"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// Campo 1 = CODNUM (não considera R_E_C_N_O_/D_E_L_E_T_)
	got, err := natives["FIELD"]([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("FIELD(1) erro: %v", err)
	}
	if advplrt.ToString(got) != "CODNUM" {
		t.Fatalf("FIELD(1)=%q, esperado CODNUM", advplrt.ToString(got))
	}
	got, err = natives["FIELD"]([]advplrt.Value{advplrt.NewNumber(2)})
	if err != nil {
		t.Fatalf("FIELD(2) erro: %v", err)
	}
	if advplrt.ToString(got) != "NOME" {
		t.Fatalf("FIELD(2)=%q, esperado NOME", advplrt.ToString(got))
	}
	// nPos fora de range => ""
	got, err = natives["FIELD"]([]advplrt.Value{advplrt.NewNumber(99)})
	if err != nil {
		t.Fatalf("FIELD(99) erro: %v", err)
	}
	if advplrt.ToString(got) != "" {
		t.Fatalf("FIELD(99)=%q, esperado \"\"", advplrt.ToString(got))
	}
}

// TestFoundEHeaderELastRec testa FOUND (após seek), HEADER e LASTREC.
func TestFoundEHeaderELastRec(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "fh.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FH")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_FH"),
		advplrt.NewString("T_FH"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// LASTREC em tabela vazia => 0
	got, err := natives["LASTREC"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("LASTREC erro: %v", err)
	}
	if advplrt.ToFloat(got) != 0 {
		t.Fatalf("LASTREC()=%v, esperado 0", advplrt.ToFloat(got))
	}
	// FOUND sem busca => .F.
	got, err = natives["FOUND"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("FOUND erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("FOUND()=%v, esperado .F.", got)
	}
	// Simula busca falha via estado (DBSEEK vive no natives.go inline)
	v.dbGenSetFound(false)
	got, err = natives["FOUND"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("FOUND erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("FOUND()=%v, esperado .F.", got)
	}
	// Simula busca com sucesso
	v.dbGenSetFound(true)
	got, err = natives["FOUND"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("FOUND erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("FOUND()=%v, esperado .T.", got)
	}
	// HEADER = soma dos campos do usuário (CODNUM N 15 + NOME C 20 + VALOR N 15)
	got, err = natives["HEADER"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("HEADER erro: %v", err)
	}
	if advplrt.ToFloat(got) != 50 {
		t.Fatalf("HEADER()=%v, esperado 50 (15+20+15)", advplrt.ToFloat(got))
	}
}

// TestNetErrERecSize testa NETERR (get/set) e RECSIZE (soma +1 flag).
func TestNetErrERecSize(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "nr.db")
	defer eng.Close()
	createTSTable(t, eng, "T_NR")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_NR"),
		advplrt.NewString("T_NR"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	// NetErr() default .F.
	got, err := natives["NETERR"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("NETERR erro: %v", err)
	}
	if got != advplrt.False {
		t.Fatalf("NETERR()=%v, esperado .F.", got)
	}
	// NetErr(.T.) grava
	if _, err := natives["NETERR"]([]advplrt.Value{advplrt.True}); err != nil {
		t.Fatalf("NETERR(.T.) erro: %v", err)
	}
	got, err = natives["NETERR"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("NETERR erro: %v", err)
	}
	if got != advplrt.True {
		t.Fatalf("NETERR() após .T.=%v, esperado .T.", got)
	}
	// RecSize = campos usuário (CODNUM N15 + NOME C20 + VALOR N15) + 1 flag = 51
	got, err = natives["RECSIZE"]([]advplrt.Value{})
	if err != nil {
		t.Fatalf("RECSIZE erro: %v", err)
	}
	if advplrt.ToFloat(got) != 51 {
		t.Fatalf("RECSIZE()=%v, esperado 51 (15+20+15+1)", advplrt.ToFloat(got))
	}
}

// TestFieldBlockEWBlock testa FIELDBLOCK/FIELDWBLOCK (NIL documentado).
func TestFieldBlockEWBlock(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "fb.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FB")

	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString("SQLITE"), advplrt.NewString("T_FB"),
		advplrt.NewString("T_FB"), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA erro: %v", err)
	}
	got, err := natives["FIELDBLOCK"]([]advplrt.Value{advplrt.NewString("NOME")})
	if err != nil {
		t.Fatalf("FIELDBLOCK erro: %v", err)
	}
	if !advplrt.IsNil(got) {
		t.Fatalf("FIELDBLOCK()=%v, esperado NIL (infra de codeblock runtime inexistente)", got)
	}
	got, err = natives["FIELDWBLOCK"]([]advplrt.Value{advplrt.NewString("NOME"), advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("FIELDWBLOCK erro: %v", err)
	}
	if !advplrt.IsNil(got) {
		t.Fatalf("FIELDWBLOCK()=%v, esperado NIL (infra de codeblock runtime inexistente)", got)
	}
}

// openAlias abre a tabela teste como alias AL.
func openAlias(t *testing.T, natives map[string]func(args []advplrt.Value) (advplrt.Value, error), table, alias, rdd string) {
	t.Helper()
	if _, err := natives["DBUSEAREA"]([]advplrt.Value{
		advplrt.True, advplrt.NewString(rdd), advplrt.NewString(table),
		advplrt.NewString(alias), advplrt.False, advplrt.False,
	}); err != nil {
		t.Fatalf("DBUSEAREA(%s): %v", table, err)
	}
}

// TestIndexOrdEFamily testa IndexOrd, OrdName, OrdNumber e IndexKey com ordens
// criadas via DBSetIndex (sem expressão) e OrdCreate (com expressão).
func TestIndexOrdEFamily(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "idx.db")
	defer eng.Close()
	createTSTable(t, eng, "T_IDX")
	openAlias(t, natives, "T_IDX", "IDX", "SQLITE")

	// Sem índices: IndexOrd 0, OrdName "" , OrdNumber 0
	got, _ := natives["INDEXORD"]([]advplrt.Value{})
	if advplrt.ToFloat(got) != 0 {
		t.Fatalf("IndexOrd()=%v, esperado 0 (sem índice)", got)
	}
	got, _ = natives["ORDNAME"]([]advplrt.Value{advplrt.NewNumber(0)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("OrdName(0) sem índice=%q, esperado \"\"", advplrt.ToString(got))
	}
	got, _ = natives["ORDNUMBER"]([]advplrt.Value{advplrt.NewString("X")})
	if advplrt.ToFloat(got) != 0 {
		t.Fatalf("OrdNumber(X)=%v, esperado 0", got)
	}
	got, _ = natives["INDEXKEY"]([]advplrt.Value{advplrt.NewNumber(0)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("IndexKey() sem índice=%q, esperado \"\"", advplrt.ToString(got))
	}

	// DBSetIndex cria ordem SEM expressão -> IndexKey ""
	if _, err := natives["DBSETINDEX"]([]advplrt.Value{advplrt.NewString("T_IDX1")}); err != nil {
		t.Fatalf("DBSETINDEX: %v", err)
	}
	got, _ = natives["INDEXORD"](nil)
	if advplrt.ToFloat(got) != 1 {
		t.Fatalf("IndexOrd()=%v, esperado 1", got)
	}
	got, _ = natives["INDEXKEY"](nil)
	if advplrt.ToString(got) != "" {
		t.Fatalf("IndexKey() com ordem sem expressão=%q, esperado \"\"", advplrt.ToString(got))
	}

	// OrdCreate cria ordem COM expressão e a torna corrente
	if _, err := natives["ORDCREATE"]([]advplrt.Value{
		advplrt.NewString("T_IDX_ORD"), advplrt.NewString("TAGCOD"),
		advplrt.NewString("CODNUM"),
	}); err != nil {
		t.Fatalf("ORDCREATE: %v", err)
	}
	// Agora há 2 ordens; corrente = TAGCOD (posição 2)
	got, _ = natives["INDEXORD"](nil)
	if advplrt.ToFloat(got) != 2 {
		t.Fatalf("IndexOrd()=%v, esperado 2 (TAGCOD ativa)", got)
	}
	got, _ = natives["INDEXKEY"](nil)
	if advplrt.ToString(got) != "CODNUM" {
		t.Fatalf("IndexKey()=%q, esperado CODNUM", advplrt.ToString(got))
	}
	// OrdKey por nome
	got, _ = natives["ORDKEY"]([]advplrt.Value{advplrt.NewString("TAGCOD")})
	if advplrt.ToString(got) != "CODNUM" {
		t.Fatalf("OrdKey(TAGCOD)=%q, esperado CODNUM", advplrt.ToString(got))
	}
	// OrdKey por posição numérica 1-based (spec: detecta numérico)
	got, _ = natives["ORDKEY"]([]advplrt.Value{advplrt.NewNumber(2)})
	if advplrt.ToString(got) != "CODNUM" {
		t.Fatalf("OrdKey(2)=%q, esperado CODNUM (posição 2 = TAGCOD)", advplrt.ToString(got))
	}
	// Posição fora do range -> ""
	got, _ = natives["ORDKEY"]([]advplrt.Value{advplrt.NewNumber(99)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("OrdKey(99)=%q, esperado \"\"", advplrt.ToString(got))
	}
	// OrdName/OrdNumber
	got, _ = natives["ORDNAME"]([]advplrt.Value{advplrt.NewNumber(1)})
	if advplrt.ToString(got) != "T_IDX1" {
		t.Fatalf("OrdName(1)=%q, esperado T_IDX1", advplrt.ToString(got))
	}
	got, _ = natives["ORDNUMBER"]([]advplrt.Value{advplrt.NewString("TAGCOD")})
	if advplrt.ToFloat(got) != 2 {
		t.Fatalf("OrdNumber(TAGCOD)=%v, esperado 2", got)
	}
	// OrdName(0) = ordem corrente (TAGCOD)
	got, _ = natives["ORDNAME"]([]advplrt.Value{advplrt.NewNumber(0)})
	if advplrt.ToString(got) != "TAGCOD" {
		t.Fatalf("OrdName(0)=%q, esperado TAGCOD", advplrt.ToString(got))
	}
	// IndexKey(1) = expressão da 1ª ordem (T_IDX1 sem expressão -> "")
	got, _ = natives["INDEXKEY"]([]advplrt.Value{advplrt.NewNumber(1)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("IndexKey(1)=%q, esperado \"\"", advplrt.ToString(got))
	}
	// IndexKey(99) fora do range -> ""
	got, _ = natives["INDEXKEY"]([]advplrt.Value{advplrt.NewNumber(99)})
	if advplrt.ToString(got) != "" {
		t.Fatalf("IndexKey(99)=%q, esperado \"\"", advplrt.ToString(got))
	}
	_ = v
}

// TestOrdSetFocusEListAdd testa OrdSetFocus, OrdListAdd e OrdBagName.
func TestOrdSetFocusEListAdd(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "focus.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FCS")
	openAlias(t, natives, "T_FCS", "FCS", "SQLITE")

	// OrdSetFocus sem foco -> ""
	got, _ := natives["ORDSETFOCUS"](nil)
	if advplrt.ToString(got) != "" {
		t.Fatalf("OrdSetFocus()=%q, esperado \"\"", advplrt.ToString(got))
	}
	// OrdSetFocus(inexistente) -> "" e não muda
	got, _ = natives["ORDSETFOCUS"]([]advplrt.Value{advplrt.NewString("ZZZ")})
	if advplrt.ToString(got) != "" {
		t.Fatalf("OrdSetFocus(ZZZ)=%q, esperado \"\"", advplrt.ToString(got))
	}
	// OrdListAdd cria ordens
	if _, err := natives["ORDLISTADD"]([]advplrt.Value{advplrt.NewString("A_ORD")}); err != nil {
		t.Fatalf("ORDLISTADD(A_ORD): %v", err)
	}
	if _, err := natives["ORDLISTADD"]([]advplrt.Value{
		advplrt.NewString("B_ORD"), advplrt.NewString("TAGB"),
	}); err != nil {
		t.Fatalf("ORDLISTADD(B_ORD,TAGB): %v", err)
	}
	// Ordem corrente = primeira adicionada (A_ORD)
	got, _ = natives["ORDNAME"]([]advplrt.Value{advplrt.NewNumber(0)})
	if advplrt.ToString(got) != "A_ORD" {
		t.Fatalf("OrdName(0)=%q, esperado A_ORD", advplrt.ToString(got))
	}
	// OrdSetFocus(TAGB) seta foco e retorna TAGB
	got, _ = natives["ORDSETFOCUS"]([]advplrt.Value{advplrt.NewString("TAGB")})
	if advplrt.ToString(got) != "TAGB" {
		t.Fatalf("OrdSetFocus(TAGB)=%q, esperado TAGB", advplrt.ToString(got))
	}
	got, _ = natives["ORDNAME"]([]advplrt.Value{advplrt.NewNumber(0)})
	if advplrt.ToString(got) != "TAGB" {
		t.Fatalf("OrdName(0) após foco=%q, esperado TAGB", advplrt.ToString(got))
	}
	// OrdBagName busca substring
	got, _ = natives["ORDBAGNAME"]([]advplrt.Value{advplrt.NewString("TAGB")})
	if advplrt.ToString(got) != "TAGB" {
		t.Fatalf("OrdBagName(TAGB)=%q, esperado TAGB", advplrt.ToString(got))
	}
	// OrdBagName não encontrada -> ""
	got, _ = natives["ORDBAGNAME"]([]advplrt.Value{advplrt.NewString("NAO_EXISTE")})
	if advplrt.ToString(got) != "" {
		t.Fatalf("OrdBagName(NAO_EXISTE)=%q, esperado \"\"", advplrt.ToString(got))
	}
}

// TestRDDNameEDefault testa RDDName, RDDSetDefault e RealRDD.
func TestRDDNameEDefault(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "rdd.db")
	defer eng.Close()
	createTSTable(t, eng, "T_RDD")
	openAlias(t, natives, "T_RDD", "RDD", "SQLITE")

	// RDDName reflete a RDD de abertura (SQLITE)
	got, _ := natives["RDDNAME"](nil)
	if advplrt.ToString(got) != "SQLITE" {
		t.Fatalf("RDDName()=%q, esperado SQLITE", advplrt.ToString(got))
	}
	// RealRDD é sempre SQLITE
	got, _ = natives["REALRDD"](nil)
	if advplrt.ToString(got) != "SQLITE" {
		t.Fatalf("RealRDD()=%q, esperado SQLITE", advplrt.ToString(got))
	}
	// RDDSetDefault sem args -> default DBFCDX
	got, _ = natives["RDDSETDEFAULT"](nil)
	if advplrt.ToString(got) != "DBFCDX" {
		t.Fatalf("RDDSetDefault()=%q, esperado DBFCDX", advplrt.ToString(got))
	}
	// RDDSetDefault(TOPCONN) altera e retorna o valor ANTERIOR (spec)
	got, _ = natives["RDDSETDEFAULT"]([]advplrt.Value{advplrt.NewString("TOPCONN")})
	if advplrt.ToString(got) != "DBFCDX" {
		t.Fatalf("RDDSetDefault(TOPCONN)=%q, esperado DBFCDX (anterior)", advplrt.ToString(got))
	}
	// RDD inválida não altera
	got, _ = natives["RDDSETDEFAULT"]([]advplrt.Value{advplrt.NewString("NOTEXISTS")})
	if advplrt.ToString(got) != "TOPCONN" {
		t.Fatalf("RDDSetDefault(NOTEXISTS)=%q, esperado TOPCONN", advplrt.ToString(got))
	}
}

// TestDBCreateRejeitaNomeInvalido testa que DBCREATE rejeita nome de campo
// com caracteres fora de identificador (prevenção de injeção no DDL).
func TestDBCreateRejeitaNomeInvalido(t *testing.T) {
	v, natives, eng := newDBGenVM(t, "create_bad.db")
	defer eng.Close()

	// Nome de campo com espaço/vírgula tentaria injetar coluna extra no DDL.
	aStruct := advplrt.NewArray([]advplrt.Value{
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewString("X TEXT, Y INTEGER"), advplrt.NewString("C"),
			advplrt.NewNumber(10), advplrt.NewNumber(0),
		}),
	})
	if _, err := natives["DBCREATE"]([]advplrt.Value{
		advplrt.NewString("T_BAD"), aStruct, advplrt.NewString("SQLITE"),
	}); err != nil {
		t.Fatalf("DBCREATE erro: %v", err)
	}
	// A tabela NÃO deve ter sido criada.
	if _, err := eng.QueryRows("PRAGMA table_info(T_BAD)"); err != nil {
		t.Fatalf("PRAGMA T_BAD deveria falhar (tabela não criada), veio %v", err)
	}
	// Confirma que a coluna extra não existe e a tabela segue inexistente.
	rows, err := eng.QueryRows("SELECT name FROM sqlite_master WHERE type='table' AND name='T_BAD'")
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("tabela T_BAD deveria não existir, mas existe (%v)", rows)
	}
	_ = v
}


func TestFLockERLock(t *testing.T) {
	_, natives, eng := newDBGenVM(t, "flock.db")
	defer eng.Close()
	createTSTable(t, eng, "T_FL")
	openAlias(t, natives, "T_FL", "FL", "SQLITE")

	// Sem registro corrente, RLock ainda retorna .T. (engine tolera)
	got, err := natives["RLOCK"]([]advplrt.Value{})
	if err != nil || got != advplrt.True {
		t.Fatalf("RLOCK() got=%v err=%v, esperado .T.", got, err)
	}
	// FLock marca o arquivo -> .T.
	got, err = natives["FLOCK"]([]advplrt.Value{})
	if err != nil || got != advplrt.True {
		t.Fatalf("FLOCK() got=%v err=%v, esperado .T.", got, err)
	}
	// DBI_ISFLOCK (20) deve refletir o bloqueio de arquivo
	got, err = natives["DBINFO"]([]advplrt.Value{advplrt.NewNumber(20)})
	if err != nil || got != advplrt.True {
		t.Fatalf("DBInfo(20) após FLOCK=%v err=%v, esperado .T.", got, err)
	}
}

// TestFLockERLockNoArea testa FLock/RLock sem área aberta: retornam .F.
// (spec: erro "Work area not in use" tratado pela regra Nil-friendly).
func TestFLockERLockNoArea(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerDbgenericasNatives(natives)

	got, err := natives["FLOCK"]([]advplrt.Value{})
	if err != nil || got != advplrt.False {
		t.Fatalf("FLOCK sem área got=%v err=%v, esperado .F.", got, err)
	}
	got, err = natives["RLOCK"]([]advplrt.Value{})
	if err != nil || got != advplrt.False {
		t.Fatalf("RLOCK sem área got=%v err=%v, esperado .F.", got, err)
	}
}
