package shared

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate pontos HOME e o diretório de trabalho para um diretório
// temporário isolado, para que os testes nunca leiam/escrevam a config
// real do usuário (~/.advpp) nem o diretório real de onde `go test` roda.
func isolate(t *testing.T) (home, work string) {
	t.Helper()
	home = t.TempDir()
	work = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ADVPP_DB", "")
	t.Chdir(work)
	return home, work
}

func TestResolveDatabasePathExplicit(t *testing.T) {
	isolate(t)
	got := ResolveDatabasePath("/custom/path.db")
	if got != "/custom/path.db" {
		t.Errorf("ResolveDatabasePath(explicit) = %q, want /custom/path.db", got)
	}
}

func TestResolveDatabasePathEnvVar(t *testing.T) {
	isolate(t)
	t.Setenv("ADVPP_DB", "/env/path.db")
	got := ResolveDatabasePath("")
	if got != "/env/path.db" {
		t.Errorf("ResolveDatabasePath() = %q, want /env/path.db (from ADVPP_DB)", got)
	}
}

// TestResolveDatabasePathNothingConfigured é o caso central do pedido do
// usuário: sem flag, sem env, sem advpp_config.json salvo (arquivo nem
// existe) — deve cair para um banco LOCAL (./advpp.db) do diretório de
// trabalho atual, não para o banco global ~/.advpp/ADVPP.db.
func TestResolveDatabasePathNothingConfigured(t *testing.T) {
	_, work := isolate(t)
	got := ResolveDatabasePath("")
	want := filepath.Join(work, LocalDatabaseName)
	if got != want {
		t.Errorf("ResolveDatabasePath() = %q, want %q (banco local do diretório atual)", got, want)
	}
}

// TestResolveDatabasePathHonorsRealConfig confere que um advpp_config.json
// que REALMENTE existe em disco (não o valor sintético que LoadConfig
// devolve por padrão) tem prioridade sobre o banco local.
func TestResolveDatabasePathHonorsRealConfig(t *testing.T) {
	isolate(t)
	if err := SaveConfig(&Config{DefaultDatabase: "/configured/shared.db"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := ResolveDatabasePath("")
	if got != "/configured/shared.db" {
		t.Errorf("ResolveDatabasePath() = %q, want /configured/shared.db (config real em disco)", got)
	}
}

// TestResolveDatabasePathIsAbsolute confere que o resultado é sempre
// absoluto mesmo quando o candidato resolvido (banco local) é relativo.
func TestResolveDatabasePathIsAbsolute(t *testing.T) {
	isolate(t)
	got := ResolveDatabasePath("")
	if !filepath.IsAbs(got) {
		t.Errorf("ResolveDatabasePath() = %q, esperado caminho absoluto", got)
	}
}

// TestOpenSQLiteCreatesFileImmediately confere que OpenSQLite materializa
// o arquivo em disco assim que aberto, mesmo sem nenhuma tabela criada —
// sql.Open+Ping sozinhos não garantiam isso (o driver só escreve o
// arquivo na primeira escrita real), o que deixava adveditor sem enxergar
// o banco criado por `advplc run` até a primeira tabela existir.
func TestOpenSQLiteCreatesFileImmediately(t *testing.T) {
	work := t.TempDir()
	dbPath := filepath.Join(work, "fresh.db")
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("arquivo já existia antes do teste: %s", dbPath)
	}
	db, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("arquivo do banco não existe logo após OpenSQLite: %v", err)
	}
}
