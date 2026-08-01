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

// Um app distribuido tem que achar o MESMO banco venha de onde vier. Este
// teste existe porque a versao anterior seguia o diretorio de trabalho: o
// mesmo app lancado da Area de Trabalho e de Documentos usava dois bancos,
// e o dado do condominio ficava partido sem aviso nenhum.
func TestStandaloneDBEstavelIndependenteDoDiretorio(t *testing.T) {
	t.Setenv("ADVPP_DB", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dirA := t.TempDir()
	entraEm(t, dirA)
	a := ResolveStandaloneDatabasePath("AppExemplo")

	dirB := t.TempDir()
	if err := os.Chdir(dirB); err != nil {
		t.Fatal(err)
	}
	b := ResolveStandaloneDatabasePath("AppExemplo")

	if a != b {
		t.Fatalf("banco mudou com o diretorio: %q vs %q", a, b)
	}
	if strings.HasPrefix(a, dirA) || strings.HasPrefix(a, dirB) {
		t.Fatalf("banco caiu no diretorio de trabalho: %q", a)
	}
	if !strings.Contains(a, "AppExemplo") {
		t.Fatalf("esperava o nome do app na pasta de dados: %q", a)
	}
	if err := os.WriteFile(a, []byte("x"), 0o644); err != nil {
		t.Fatalf("caminho escolhido nao e gravavel: %v", err)
	}
	os.Remove(a)
}

// Dois apps diferentes nao podem dividir o mesmo arquivo: era o que fazia
// GesCon e advpp-ide nao abrirem ao mesmo tempo quando lancados da mesma
// pasta.
func TestStandaloneDBSeparaPorApp(t *testing.T) {
	t.Setenv("ADVPP_DB", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	entraEm(t, t.TempDir())

	if a, b := ResolveStandaloneDatabasePath("AppExemplo"), ResolveStandaloneDatabasePath("OutroApp"); a == b {
		t.Fatalf("dois apps dividindo o mesmo banco: %q", a)
	}
}

func TestStandaloneDBRespeitaADVPPDB(t *testing.T) {
	dir := t.TempDir()
	entraEm(t, dir)
	escolhido := filepath.Join(t.TempDir(), "meu.db")
	t.Setenv("ADVPP_DB", escolhido)

	if got := ResolveStandaloneDatabasePath("AppExemplo"); got != escolhido {
		t.Fatalf("ADVPP_DB tem precedência: got %q, quer %q", got, escolhido)
	}
}

func TestSanitizaNomeApp(t *testing.T) {
	casos := map[string]string{
		"AppExemplo":            "AppExemplo",
		"App — Exemplo":         "AppExemplo",
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

// O arquivo de configuração substitui o executável lançador que definia
// ADVPP_DB: transportar uma string não vale um processo intermediário.
func TestStandaloneDBLeArquivoDeConfig(t *testing.T) {
	t.Setenv("ADVPP_DB", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// O arquivo é procurado ao lado do executável — nos testes, o binário do
	// próprio `go test`.
	exe, err := os.Executable()
	if err != nil {
		t.Skip("sem os.Executable neste ambiente")
	}
	conf := filepath.Join(filepath.Dir(exe), bancoConfigNome)
	escolhido := filepath.Join(t.TempDir(), "compartilhado", "GesCon.db")
	if err := os.WriteFile(conf, []byte(escolhido+"\r\n"), 0o644); err != nil {
		t.Skip("diretório do binário de teste não é gravável")
	}
	t.Cleanup(func() { os.Remove(conf) })

	if got := ResolveStandaloneDatabasePath("AppExemplo"); got != escolhido {
		t.Fatalf("devia usar o caminho do arquivo: got %q, quer %q", got, escolhido)
	}
	// A pasta tem que existir depois: quem lê o arquivo é quem vai abrir o
	// banco, e SQLite não cria diretório.
	if st, err := os.Stat(filepath.Dir(escolhido)); err != nil || !st.IsDir() {
		t.Fatalf("a pasta do banco não foi criada: %v", err)
	}
}

func TestADVPPDBTemPrecedenciaSobreOArquivo(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("sem os.Executable neste ambiente")
	}
	conf := filepath.Join(filepath.Dir(exe), bancoConfigNome)
	if err := os.WriteFile(conf, []byte(filepath.Join(t.TempDir(), "do-arquivo.db")), 0o644); err != nil {
		t.Skip("diretório do binário de teste não é gravável")
	}
	t.Cleanup(func() { os.Remove(conf) })

	daVariavel := filepath.Join(t.TempDir(), "da-variavel.db")
	t.Setenv("ADVPP_DB", daVariavel)
	if got := ResolveStandaloneDatabasePath("AppExemplo"); got != daVariavel {
		t.Fatalf("ADVPP_DB devia vencer o arquivo: got %q", got)
	}
}
