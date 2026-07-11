package shared

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // driver SQLite puro-Go (sem CGO, portável Linux/Win64/macOS)
)

// OpenSQLite abre um banco SQLite com os pragmas padrão do AdvPP.
// Ponto único de abertura: todas as ferramentas (advplc, advcfg,
// adveditor, advpp-ide) devem usar esta função para enxergar o mesmo
// banco com o mesmo comportamento.
//
// Se o arquivo ainda não existe, cria um arquivo vazio ANTES de abrir —
// sql.Open + Ping sozinhos não garantem que o arquivo apareça em disco de
// imediato (o driver só materializa o arquivo na primeira escrita real,
// que pode nunca acontecer se o chamador só faz leituras que falham por
// tabela inexistente). Um arquivo SQLite de 0 bytes é válido para abrir
// como banco novo — garante que `advplc run/check/compile/serve` deixa o
// banco local visível (para advcfg/adveditor no mesmo diretório
// enxergarem) mesmo que nenhuma tabela tenha sido criada ainda.
func OpenSQLite(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		f.Close()
	}
	dsn := path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
