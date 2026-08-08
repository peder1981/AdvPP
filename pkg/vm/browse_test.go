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
