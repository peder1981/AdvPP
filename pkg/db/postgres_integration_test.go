//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenPostgresIntegration só roda com `go test -tags=integration` contra
// um Postgres real (ex.: `docker run -e POSTGRES_PASSWORD=advpp -p 5432:5432 postgres:16`).
// Configurar via env vars antes de rodar.
func TestOpenPostgresIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_PG_HOST", "localhost"),
		Port:     5432,
		Service:  envOr("ADVPP_TEST_PG_DB", "postgres"),
		User:     envOr("ADVPP_TEST_PG_USER", "postgres"),
		Password: os.Getenv("ADVPP_TEST_PG_PASSWORD"),
	}
	db, dialect, err := openPostgres(cfg)
	if err != nil {
		t.Fatalf("openPostgres: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != "$1" {
		t.Errorf("placeholder = %q, want $1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
