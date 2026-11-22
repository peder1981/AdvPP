package db

import (
	"path/filepath"
	"testing"
)

// Um Exec com "BEGIN; ...; COMMIT;" em que um comando do meio falha deixava
// a transação aberta numa conexão do pool: o que veio antes do erro ficava
// pendente e as escritas seguintes esperavam o busy_timeout e falhavam
// ("database is locked"). Exec agora desfaz na mesma conexão.
func TestSQLiteExecFalhaNoMeioNaoDeixaTransacaoAberta(t *testing.T) {
	eng, err := NewSQLiteEngine(filepath.Join(t.TempDir(), "rb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.db.Close()
	if err := eng.Exec("CREATE TABLE T (A INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	err = eng.Exec("BEGIN;\nINSERT INTO T VALUES (1);\nINSERT INTO T VALUES (NULL);\nCOMMIT;")
	if err == nil {
		t.Fatal("esperava erro de NOT NULL")
	}
	// Escrita seguinte não pode ficar travada pela transação do comando anterior
	if err := eng.Exec("INSERT INTO T VALUES (2)"); err != nil {
		t.Fatalf("escrita depois do erro falhou (transação pendurada?): %v", err)
	}
	rows, err := eng.QueryRows("SELECT COUNT(*) AS Q FROM T")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["Q"] != "1" {
		t.Fatalf("esperava só a linha 2 (a 1 desfeita junto com o BEGIN), achou %s linhas", rows[0]["Q"])
	}
}
