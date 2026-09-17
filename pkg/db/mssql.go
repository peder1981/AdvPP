package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/microsoft/go-mssqldb" // registra o driver "sqlserver"
)

// openMSSQL abre uma conexão real com um servidor SQL Server via
// go-mssqldb, puro Go, sem CGO. cfg.Service é o nome do banco.
func openMSSQL(cfg ConnConfig) (*sql.DB, Dialect, error) {
	query := url.Values{}
	query.Add("database", cfg.Service)
	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: query.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("mssql: %w", err)
	}
	return db, mssqlDialect{}, nil
}
