package shared

import (
	"path/filepath"
	"testing"
)

func newTestDriver(t *testing.T) (*SQLiteDriver, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d := NewSQLiteDriver()
	if err := d.Open(dbPath, false, false); err != nil {
		t.Fatalf("Open (empty db): %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d, dbPath
}

// TestOpenEmptyDatabase confere que abrir um banco SQLite recém-criado (0
// tabelas — o caso do banco local auto-provisionado) não é mais erro, só
// deixa a tabela ativa vazia até CreateTable/SelectTable.
func TestOpenEmptyDatabase(t *testing.T) {
	newTestDriver(t) // já falha via t.Fatalf se Open() errar
}

func TestCreateTableAndSelect(t *testing.T) {
	d, _ := newTestDriver(t)
	fields := []Field{
		{Name: "cod", Type: FieldTypeChar},
		{Name: "qtd", Type: FieldTypeNum, Decimal: 2},
	}
	if err := d.CreateTable("produtos", fields); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("produtos"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	got, err := d.GetStructure()
	if err != nil {
		t.Fatalf("GetStructure: %v", err)
	}
	if len(got) != 2 || got[0].Name != "cod" || got[1].Name != "qtd" {
		t.Fatalf("GetStructure = %+v, want cod+qtd", got)
	}
}

func TestCreateTableRejectsBadIdentifiers(t *testing.T) {
	d, _ := newTestDriver(t)
	cases := []struct {
		table string
		field string
	}{
		{"ok tabela com espaco", "campo"},
		{"tabela; DROP TABLE x; --", "campo"},
		{"1comecaComDigito", "campo"},
		{"tabela", "campo com espaco"},
		{"tabela", "campo;--"},
	}
	for _, c := range cases {
		err := d.CreateTable(c.table, []Field{{Name: c.field, Type: FieldTypeChar}})
		if err == nil {
			t.Errorf("CreateTable(%q, field=%q) succeeded, want rejection", c.table, c.field)
		}
	}
}

func TestCreateTableEmptyFieldsRejected(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("vazia", nil); err == nil {
		t.Error("CreateTable com 0 campos deveria falhar")
	}
}

func TestDropTable(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "a", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.DropTable("t1"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	tables, err := d.ListTables()
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	for _, tb := range tables {
		if tb == "t1" {
			t.Fatal("t1 ainda existe depois de DropTable")
		}
	}
}

func TestAddColumnAndDropColumn(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "a", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	if err := d.AddColumn(Field{Name: "b", Type: FieldTypeNum}); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}
	structure, _ := d.GetStructure()
	if len(structure) != 2 {
		t.Fatalf("depois de AddColumn, GetStructure = %+v, want 2 campos", structure)
	}

	if err := d.DropColumn("b"); err != nil {
		t.Fatalf("DropColumn: %v", err)
	}
	structure, _ = d.GetStructure()
	if len(structure) != 1 {
		t.Fatalf("depois de DropColumn, GetStructure = %+v, want 1 campo", structure)
	}
}

// TestRecordCRUD confere o ciclo completo Add/Update/Delete de linhas —
// os métodos já existiam antes desta rodada, mas nunca tinham teste.
func TestRecordCRUD(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "nome", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}

	recno, err := d.AddRecord(Record{Fields: map[string]interface{}{"nome": "primeiro"}})
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	records, err := d.GetData(0, 10)
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if len(records) != 1 || records[0].Fields["nome"] != "primeiro" {
		t.Fatalf("GetData = %+v, want 1 registro 'primeiro'", records)
	}

	if err := d.UpdateRecord(recno, Record{Fields: map[string]interface{}{"nome": "segundo"}}); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	records, _ = d.GetData(0, 10)
	if records[0].Fields["nome"] != "segundo" {
		t.Fatalf("depois de UpdateRecord, nome = %v, want 'segundo'", records[0].Fields["nome"])
	}

	if err := d.DeleteRecord(recno); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	records, _ = d.GetData(0, 10)
	if len(records) != 0 {
		t.Fatalf("depois de DeleteRecord, GetData = %+v, want vazio", records)
	}
}

// TestDeleteRecordIsLogical confere que DeleteRecord NÃO remove a linha de
// verdade (estilo Protheus: D_E_L_E_T_='*', R_E_C_D_E_L_=1) — só some das
// leituras normais (GetData/Count) até um Pack ou um RecallRecord.
func TestDeleteRecordIsLogical(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "nome", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	recno, err := d.AddRecord(Record{Fields: map[string]interface{}{"nome": "x"}})
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if err := d.DeleteRecord(recno); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}

	// GetData/Count escondem a linha deletada...
	if records, _ := d.GetData(0, 10); len(records) != 0 {
		t.Fatalf("GetData depois de DeleteRecord = %+v, want vazio", records)
	}
	if count, _ := d.Count(); count != 0 {
		t.Fatalf("Count depois de DeleteRecord = %d, want 0", count)
	}

	// ...mas a linha continua fisicamente na tabela (consulta bruta).
	var raw int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM t1").Scan(&raw); err != nil {
		t.Fatalf("consulta bruta: %v", err)
	}
	if raw != 1 {
		t.Fatalf("linha física depois de DeleteRecord = %d, want 1 (delete deveria ser lógico)", raw)
	}

	// RecallRecord desfaz a exclusão lógica.
	if err := d.RecallRecord(recno); err != nil {
		t.Fatalf("RecallRecord: %v", err)
	}
	records, err := d.GetData(0, 10)
	if err != nil {
		t.Fatalf("GetData depois de RecallRecord: %v", err)
	}
	if len(records) != 1 || records[0].Fields["nome"] != "x" {
		t.Fatalf("GetData depois de RecallRecord = %+v, want 1 registro 'x'", records)
	}
}

// TestPackPurgesDeletedRows confere que Pack remove de vez as linhas
// marcadas com exclusão lógica.
func TestPackPurgesDeletedRows(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "nome", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	recno, err := d.AddRecord(Record{Fields: map[string]interface{}{"nome": "x"}})
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if err := d.DeleteRecord(recno); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if err := d.Pack(); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	var raw int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM t1").Scan(&raw); err != nil {
		t.Fatalf("consulta bruta: %v", err)
	}
	if raw != 0 {
		t.Fatalf("linha física depois de Pack = %d, want 0 (Pack deveria purgar)", raw)
	}
}

// TestCreateTableRejectsSystemColumnName confere que CreateTable recusa um
// campo do usuário que colida com uma coluna de sistema reservada
// (R_E_C_N_O_/D_E_L_E_T_/R_E_C_D_E_L_).
func TestCreateTableRejectsSystemColumnName(t *testing.T) {
	d, _ := newTestDriver(t)
	for _, name := range []string{"R_E_C_N_O_", "D_E_L_E_T_", "R_E_C_D_E_L_", "d_e_l_e_t_"} {
		if err := d.CreateTable("t_"+name, []Field{{Name: name, Type: FieldTypeChar}}); err == nil {
			t.Errorf("CreateTable com campo %q deveria falhar (nome reservado)", name)
		}
	}
}

// TestStructureHidesSystemColumns confere que GetStructure não expõe as
// colunas de sistema injetadas por CreateTable — só os campos do usuário.
func TestStructureHidesSystemColumns(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{
		{Name: "a", Type: FieldTypeChar},
		{Name: "b", Type: FieldTypeNum},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	structure, err := d.GetStructure()
	if err != nil {
		t.Fatalf("GetStructure: %v", err)
	}
	if len(structure) != 2 {
		t.Fatalf("GetStructure = %+v, want só os 2 campos do usuário (sem colunas de sistema)", structure)
	}
}

func TestCreateAndDropIndex(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{
		{Name: "a", Type: FieldTypeChar},
		{Name: "b", Type: FieldTypeChar},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	if err := d.CreateIndex("idx1", "a+b"); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	idxs, err := d.GetIndexes()
	if err != nil {
		t.Fatalf("GetIndexes: %v", err)
	}
	found := false
	for _, ix := range idxs {
		if ix.Name == "idx1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("índice idx1 não encontrado em GetIndexes: %+v", idxs)
	}

	if err := d.DropIndex("idx1"); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}
}

func TestCreateIndexRejectsBadFieldName(t *testing.T) {
	d, _ := newTestDriver(t)
	if err := d.CreateTable("t1", []Field{{Name: "a", Type: FieldTypeChar}}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := d.SelectTable("t1"); err != nil {
		t.Fatalf("SelectTable: %v", err)
	}
	if err := d.CreateIndex("idx1", "a; DROP TABLE t1; --"); err == nil {
		t.Error("CreateIndex com expressão maliciosa deveria falhar")
	}
}

func TestValidIdentifier(t *testing.T) {
	valid := []string{"a", "abc", "A_1", "_x", "campo123"}
	invalid := []string{"", "1abc", "a b", "a;b", "a-b", "a.b", "a'b"}
	for _, v := range valid {
		if !validIdentifier(v) {
			t.Errorf("validIdentifier(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if validIdentifier(v) {
			t.Errorf("validIdentifier(%q) = true, want false", v)
		}
	}
}
