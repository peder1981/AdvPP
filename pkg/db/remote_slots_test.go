package db

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRemoteSlots(t *testing.T) {
	old := maxRemoteConns
	maxRemoteConns = 2
	defer func() { maxRemoteConns = old }()
	if err := acquireRemoteSlot(); err != nil {
		t.Fatal(err)
	}
	if err := acquireRemoteSlot(); err != nil {
		t.Fatal(err)
	}
	err := acquireRemoteSlot()
	if err == nil || !strings.Contains(err.Error(), "limite de conexoes remotas atingido (2)") {
		t.Fatalf("terceira vaga deveria falhar, veio %v", err)
	}
	releaseRemoteSlot()
	if err := acquireRemoteSlot(); err != nil {
		t.Fatalf("vaga liberada deveria ser reaproveitada: %v", err)
	}
	releaseRemoteSlot()
	releaseRemoteSlot()
}

// sql.Open não conecta: dá para conferir a configuração do pool sem servidor.
func TestPinPool(t *testing.T) {
	sqlDB, err := sql.Open("pgx", "postgres://x@127.0.0.1:1/x")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	pinPool(sqlDB)
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}
}
