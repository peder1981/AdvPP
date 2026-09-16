//go:build integration

package db

import (
	"os"
	"testing"
)

// TestOpenOracleIntegration só roda com `go test -tags=integration` contra
// um Oracle real (ex.: gvenzl/oracle-free no Docker). Configurar via env
// vars antes de rodar.
func TestOpenOracleIntegration(t *testing.T) {
	cfg := ConnConfig{
		Host:     envOr("ADVPP_TEST_ORA_HOST", "localhost"),
		Port:     1521,
		Service:  envOr("ADVPP_TEST_ORA_SERVICE", "FREEPDB1"),
		User:     envOr("ADVPP_TEST_ORA_USER", "system"),
		Password: os.Getenv("ADVPP_TEST_ORA_PASSWORD"),
	}
	db, dialect, err := openOracle(cfg)
	if err != nil {
		t.Fatalf("openOracle: %v", err)
	}
	defer db.Close()
	if dialect.Placeholder(1) != ":1" {
		t.Errorf("placeholder = %q, want :1", dialect.Placeholder(1))
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
