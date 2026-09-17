package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" em database/sql
)

// openPostgres abre uma conexão real com um servidor PostgreSQL via pgx em
// modo stdlib (database/sql), sem CGO. cfg.Service é o nome do banco.
func openPostgres(cfg ConnConfig) (*sql.DB, Dialect, error) {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + cfg.Service,
	}
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("postgres: %w", err)
	}
	return db, postgresDialect{}, nil
}
