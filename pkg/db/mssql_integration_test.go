//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenMSSQLIntegration só roda com `go test -tags=integration` contra
// um SQL Server real (ex.: mcr.microsoft.com/mssql/server no Docker).
func TestOpenMSSQLIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_MSSQL_HOST", "localhost"),
		Port:     1433,
		Service:  envOr("ADVPP_TEST_MSSQL_DB", "master"),
		User:     envOr("ADVPP_TEST_MSSQL_USER", "sa"),
		Password: os.Getenv("ADVPP_TEST_MSSQL_PASSWORD"),
	}
	db, dialect, err := openMSSQL(cfg)
	if err != nil {
		t.Fatalf("openMSSQL: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != "@p1" {
		t.Errorf("placeholder = %q, want @p1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
