package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// entraEm troca o diretório de trabalho pela duração do teste.
func entraEm(t *testing.T, dir string) {
	t.Helper()
	anterior, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(anterior) })
}

func TestStandaloneDBUsaDiretorioAtualQuandoGravavel(t *testing.T) {
	dir := t.TempDir()
	entraEm(t, dir)
	t.Setenv("ADVPP_DB", "")

	got := ResolveStandaloneDatabasePath("GesCon")
	// macOS resolve TempDir por /var, link para /private/var: compara o
	// diretório resolvido, não a string.
	quer, _ := filepath.EvalSymlinks(filepath.Join(dir, LocalDatabaseName))
	gotResolvido, _ := filepath.EvalSymlinks(got)
	if gotResolvido != quer {
		t.Fatalf("diretório gravável devia mandar: got %q, quer %q", gotResolvido, quer)
	}
}

func TestStandaloneDBCaiParaAppDataQuandoDiretorioNaoGravavel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0555 não vale como ACL no Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root escreve em diretório 0555")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	entraEm(t, dir)
	t.Setenv("ADVPP_DB", "")

	got := ResolveStandaloneDatabasePath("GesCon")
	if strings.HasPrefix(got, dir) {
		t.Fatalf("caiu no diretório sem escrita, o defeito que este teste existe para pegar: %q", got)
	}
	if !strings.Contains(got, "GesCon") {
		t.Fatalf("esperava o nome do app na pasta de dados: %q", got)
	}
	// Só serve se der para escrever de verdade.
	if err := os.WriteFile(got, []byte("x"), 0o644); err != nil {
		t.Fatalf("caminho escolhido não é gravável: %v", err)
	}
	os.Remove(got)
}

func TestStandaloneDBPreservaBancoJaExistenteNoDiretorio(t *testing.T) {
	dir := t.TempDir()
	entraEm(t, dir)
	t.Setenv("ADVPP_DB", "")
	existente := filepath.Join(dir, LocalDatabaseName)
	if err := os.WriteFile(existente, []byte("banco em uso"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, _ := filepath.EvalSymlinks(ResolveStandaloneDatabasePath("GesCon"))
	quer, _ := filepath.EvalSymlinks(existente)
	if got != quer {
		t.Fatalf("trocou o banco em uso por outro caminho: got %q, quer %q", got, quer)
	}
}

func TestStandaloneDBRespeitaADVPPDB(t *testing.T) {
	dir := t.TempDir()
	entraEm(t, dir)
	escolhido := filepath.Join(t.TempDir(), "meu.db")
	t.Setenv("ADVPP_DB", escolhido)

	if got := ResolveStandaloneDatabasePath("GesCon"); got != escolhido {
		t.Fatalf("ADVPP_DB tem precedência: got %q, quer %q", got, escolhido)
	}
}

func TestSanitizaNomeApp(t *testing.T) {
	casos := map[string]string{
		"GesCon":                "GesCon",
		"GesCon — Gestão":       "GesConGesto",
		"../../etc":             "etc",
		"":                      "app",
		"C:\\Windows\\System32": "CWindowsSystem32",
	}
	for entrada, quer := range casos {
		if got := sanitizaNomeApp(entrada); got != quer {
			t.Errorf("sanitizaNomeApp(%q) = %q, quer %q", entrada, got, quer)
		}
	}
}

// O caso do adveditor/advpp-ide: eles usam ResolveDatabasePath, nao o
// resolver de standalone. Aberto por um atalho apontando para dentro de
// Program Files, o diretorio de trabalho e a pasta de instalacao -- e era ali
// que o "./advpp.db" ia parar, sem nunca abrir.
func TestResolveDatabasePathNaoDevolveCaminhoSemEscrita(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0555 não vale como ACL no Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root escreve em diretório 0555")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	entraEm(t, dir)
	t.Setenv("ADVPP_DB", "")
	// HOME próprio: sem isto o passo 3 acharia a config real da máquina.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got := ResolveDatabasePath("")
	if strings.HasPrefix(got, dir) {
		t.Fatalf("devolveu caminho sem escrita: %q", got)
	}
	if err := os.WriteFile(got, []byte("x"), 0o644); err != nil {
		t.Fatalf("caminho escolhido não é gravável: %v", err)
	}
	os.Remove(got)
}
