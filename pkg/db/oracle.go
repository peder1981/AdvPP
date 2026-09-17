package db

import (
	"database/sql"
	"fmt"

	go_ora "github.com/sijms/go-ora/v2"
)

// openOracle abre uma conexão real com um servidor Oracle via go-ora —
// implementação pura em Go do protocolo TNS, sem Oracle Instant Client e
// sem CGO. cfg.Service é o SID/service name.
func openOracle(cfg ConnConfig) (*sql.DB, Dialect, error) {
	dsn := go_ora.BuildUrl(cfg.Host, cfg.Port, cfg.Service, cfg.User, cfg.Password, nil)
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("oracle: %w", err)
	}
	return db, oracleDialect{}, nil
}
