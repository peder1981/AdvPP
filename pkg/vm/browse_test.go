package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/db"
)

func TestBrowseFiltraPorFilial(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('999', '010102')"); err != nil {
		t.Fatalf("seed B: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "UNI")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if !hasFilial {
		t.Fatal("hasFilial deveria ser true (tabela tem coluna FILIAL)")
	}

	items, err := browseItems(eng, "UNI", cols, hasDelete, hasFilial, "010101")
	if err != nil {
		t.Fatalf("browseItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("browseItems devolveu %d itens, quer 1 (só a filial 010101)", len(items))
	}
	if items[0]["UNI_CODIGO"] != "101" {
		t.Errorf("UNI_CODIGO = %v, quer '101'", items[0]["UNI_CODIGO"])
	}

	// Incluir sob filial 010102 -- deve estampar FILIAL sozinho.
	err = browseSave(eng, "UNI", cols, hasDelete, hasFilial, "010102",
		browseAction{Action: "save", Recno: 0, Data: map[string]string{"UNI_CODIGO": "202"}})
	if err != nil {
		t.Fatalf("browseSave (incluir): %v", err)
	}
	rows, _ := eng.QueryRows("SELECT FILIAL FROM UNI WHERE UNI_CODIGO = '202'")
	if len(rows) != 1 || rows[0]["FILIAL"] != "010102" {
		t.Errorf("linha incluida com FILIAL = %v, quer '010102'", rows)
	}

	// Sob a filial 010102, browseItems agora deve ver 2 linhas (999 + 202 novo).
	items2, _ := browseItems(eng, "UNI", cols, hasDelete, hasFilial, "010102")
	if len(items2) != 2 {
		t.Errorf("browseItems sob 010102 devolveu %d itens, quer 2", len(items2))
	}
}

func TestBrowseAlterarRecusaCrossFilial(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test3.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "UNI")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if !hasFilial {
		t.Fatal("hasFilial deveria ser true (tabela tem coluna FILIAL)")
	}

	// A linha '101' pertence à filial 010101 (recno 1). Tenta alterá-la
	// passando cFilial = 010102 (outra filial) -- deve ser recusado
	// silenciosamente (0 linhas afetadas, sem erro), preservando os dados.
	err = browseSave(eng, "UNI", cols, hasDelete, hasFilial, "010102",
		browseAction{Action: "save", Recno: 1, Data: map[string]string{"UNI_CODIGO": "FORJADO"}})
	if err != nil {
		t.Fatalf("browseSave (alterar cross-filial): %v", err)
	}

	rows, err := eng.QueryRows("SELECT UNI_CODIGO, FILIAL FROM UNI WHERE rowid = 1")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("esperava 1 linha, veio %d", len(rows))
	}
	if rows[0]["UNI_CODIGO"] != "101" {
		t.Errorf("UNI_CODIGO foi alterado para %q por um update cross-filial, quer permanecer '101'", rows[0]["UNI_CODIGO"])
	}
	if rows[0]["FILIAL"] != "010101" {
		t.Errorf("FILIAL foi alterado para %q por um update cross-filial, quer permanecer '010101'", rows[0]["FILIAL"])
	}
}

func TestBrowseFilialNaoEhColunaEditavel(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test4.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "UNI")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if !hasFilial {
		t.Fatal("hasFilial deveria ser true (tabela tem coluna FILIAL)")
	}
	for _, c := range cols {
		if c.Property == "FILIAL" {
			t.Fatal("FILIAL não deveria aparecer em cols -- é campo gerido pelo sistema, não editável pelo cliente")
		}
	}

	// Incluir: cliente forja um FILIAL diferente do cFilial ativo em
	// act.Data. Como FILIAL não está em cols, esse valor forjado deve ser
	// ignorado; a linha deve ficar estampada com o cFilial ATIVO (010101),
	// não com o valor forjado.
	err = browseSave(eng, "UNI", cols, hasDelete, hasFilial, "010101",
		browseAction{Action: "save", Recno: 0, Data: map[string]string{
			"UNI_CODIGO": "202",
			"FILIAL":     "999999", // forjado
		}})
	if err != nil {
		t.Fatalf("browseSave (incluir com FILIAL forjado): %v", err)
	}
	rows, _ := eng.QueryRows("SELECT FILIAL FROM UNI WHERE UNI_CODIGO = '202'")
	if len(rows) != 1 || rows[0]["FILIAL"] != "010101" {
		t.Errorf("linha incluida com FILIAL = %v, quer '010101' (cFilial ativo, não o forjado)", rows)
	}

	// Alterar: mesma tentativa, agora sobre a linha existente (recno 1,
	// filial real 010101 == cFilial ativo, então o UPDATE deve ser
	// aplicado -- mas o FILIAL forjado em Data deve ser ignorado).
	err = browseSave(eng, "UNI", cols, hasDelete, hasFilial, "010101",
		browseAction{Action: "save", Recno: 1, Data: map[string]string{
			"UNI_CODIGO": "101-EDITADO",
			"FILIAL":     "999999", // forjado
		}})
	if err != nil {
		t.Fatalf("browseSave (alterar com FILIAL forjado): %v", err)
	}
	rows2, _ := eng.QueryRows("SELECT UNI_CODIGO, FILIAL FROM UNI WHERE rowid = 1")
	if len(rows2) != 1 {
		t.Fatalf("esperava 1 linha, veio %d", len(rows2))
	}
	if rows2[0]["UNI_CODIGO"] != "101-EDITADO" {
		t.Errorf("UNI_CODIGO = %q, quer '101-EDITADO' (o resto do update deveria ter sido aplicado)", rows2[0]["UNI_CODIGO"])
	}
	if rows2[0]["FILIAL"] != "010101" {
		t.Errorf("FILIAL = %q por um Alterar com FILIAL forjado em Data, quer permanecer '010101'", rows2[0]["FILIAL"])
	}
}

func TestBrowseDeleteRecusaCrossFilial(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test5.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE UNI (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		UNI_CODIGO TEXT,
		FILIAL TEXT
	)`); err != nil {
		t.Fatalf("create UNI: %v", err)
	}
	if err := eng.Exec("INSERT INTO UNI (UNI_CODIGO, FILIAL) VALUES ('101', '010101')"); err != nil {
		t.Fatalf("seed A: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "UNI")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	_ = cols

	// Tenta excluir o recno 1 (filial real 010101) sob cFilial=010102 --
	// deve ser recusado silenciosamente (0 linhas afetadas, sem erro).
	if err := browseDelete(eng, "UNI", hasDelete, hasFilial, "010102", 1); err != nil {
		t.Fatalf("browseDelete (cross-filial): %v", err)
	}

	rows, err := eng.QueryRows("SELECT D_E_L_E_T_ FROM UNI WHERE rowid = 1")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("esperava 1 linha, veio %d", len(rows))
	}
	if rows[0]["D_E_L_E_T_"] != " " {
		t.Errorf("D_E_L_E_T_ = %q, quer ' ' (linha NÃO deveria ter sido soft-deletada por um browseDelete cross-filial)", rows[0]["D_E_L_E_T_"])
	}
}

func TestBrowseSemColunaFilialComportamentoInalterado(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/browse_test2.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec(`CREATE TABLE SEM_FILIAL (
		R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT,
		D_E_L_E_T_ TEXT DEFAULT ' ',
		NOME TEXT
	)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := eng.Exec("INSERT INTO SEM_FILIAL (NOME) VALUES ('x')"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cols, hasDelete, hasFilial, err := (&VM{}).browseColumns(eng, "SEM_FILIAL")
	if err != nil {
		t.Fatalf("browseColumns: %v", err)
	}
	if hasFilial {
		t.Fatal("hasFilial deveria ser false (tabela sem coluna FILIAL)")
	}
	items, err := browseItems(eng, "SEM_FILIAL", cols, hasDelete, hasFilial, "")
	if err != nil {
		t.Fatalf("browseItems: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("tabela sem FILIAL: browseItems devolveu %d itens, quer 1 (sem filtro)", len(items))
	}
}
