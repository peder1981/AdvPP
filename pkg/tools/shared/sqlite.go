package shared

import (
	"database/sql"

	_ "modernc.org/sqlite" // driver SQLite puro-Go (sem CGO, portável Linux/Win64/macOS)
)

// OpenSQLite abre um banco SQLite com os pragmas padrão do AdvPP.
// Ponto único de abertura: todas as ferramentas (advplc, advcfg,
// adveditor, advpp-ide) devem usar esta função para enxergar o mesmo
// banco com o mesmo comportamento.
func OpenSQLite(path string) (*sql.DB, error) {
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
