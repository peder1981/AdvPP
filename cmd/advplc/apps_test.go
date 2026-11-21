package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/webui"
)

func escreveApp(t *testing.T, dir, nome, src string) string {
	t.Helper()
	p := filepath.Join(dir, nome)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func appConOut(texto string) string {
	return "User Function Main()\n\tConOut(\"" + texto + "\")\nReturn\n"
}

// eventoDoApp abre o SSE e devolve a 1ª linha de saída do programa
// (output que não é o cabeçalho, ou error).
func eventoDoApp(t *testing.T, base, query string) string {
	t.Helper()
	resp, err := http.Get(base + "/events?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "status " + resp.Status
	}
	linhas := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			linhas <- sc.Text()
		}
	}()
	fim := time.After(3 * time.Second)
	for {
		select {
		case l := <-linhas:
			if (strings.Contains(l, `"type":"output"`) && !strings.Contains(l, "executando")) || strings.Contains(l, `"type":"error"`) {
				return l
			}
		case <-fim:
			t.Fatalf("%s: sem saída", query)
		}
	}
}

func servidorDeApps(t *testing.T, dir, home string) string {
	t.Helper()
	c := novoCatalogo(dir, home, &Options{})
	ts := httptest.NewServer(webui.NewApps("teste", c.resolver()).Handler())
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestCatalogoRodaAppPeloNome(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir, "a.prw", appConOut("sou a"))
	if l := eventoDoApp(t, servidorDeApps(t, dir, ""), "s=1&app=a"); !strings.Contains(l, "sou a") {
		t.Errorf("evento = %s", l)
	}
}

func TestCatalogoHome(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir, "a.prw", appConOut("sou a"))
	url := servidorDeApps(t, dir, "a")
	if l := eventoDoApp(t, url, "s=1"); !strings.Contains(l, "sou a") {
		t.Errorf("evento = %s", l)
	}
	semHome := servidorDeApps(t, dir, "")
	if l := eventoDoApp(t, semHome, "s=1"); !strings.Contains(l, "404") {
		t.Errorf("sem --home, a raiz deveria dar 404: %s", l)
	}
}

func TestCatalogoTlpp(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir, "b.tlpp", appConOut("sou b"))
	if l := eventoDoApp(t, servidorDeApps(t, dir, ""), "s=1&app=b"); !strings.Contains(l, "sou b") {
		t.Errorf("evento = %s", l)
	}
}

func TestCatalogoErroDeCompilacao(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir, "ruim.prw", "User Function Main(\n\tConOut(\"x\"\nReturn\n")
	escreveApp(t, dir, "a.prw", appConOut("sou a"))
	url := servidorDeApps(t, dir, "")
	if l := eventoDoApp(t, url, "s=1&app=ruim"); !strings.Contains(l, `"type":"error"`) || !strings.Contains(l, "erro de compilação em ruim") {
		t.Errorf("evento = %s", l)
	}
	if l := eventoDoApp(t, url, "s=2&app=a"); !strings.Contains(l, "sou a") {
		t.Errorf("depois do erro, app a: %s", l)
	}
}

func TestCatalogoRecompilaPorMtime(t *testing.T) {
	dir := t.TempDir()
	p := escreveApp(t, dir, "a.prw", appConOut("sou a"))
	url := servidorDeApps(t, dir, "")
	if l := eventoDoApp(t, url, "s=1&app=a"); !strings.Contains(l, "sou a") {
		t.Fatalf("evento = %s", l)
	}
	escreveApp(t, dir, "a.prw", appConOut("sou a v2"))
	futuro := time.Now().Add(2 * time.Second)
	os.Chtimes(p, futuro, futuro)
	if l := eventoDoApp(t, url, "s=2&app=a"); !strings.Contains(l, "sou a v2") {
		t.Errorf("depois da alteração: %s", l)
	}
}

func TestCatalogoSessoesIndependentes(t *testing.T) {
	dir := t.TempDir()
	escreveApp(t, dir, "cont.prw", "Static nCont := 0\n\nUser Function Main()\n\tnCont++\n\tConOut(\"n=\" + cValToChar(nCont))\nReturn\n")
	url := servidorDeApps(t, dir, "")
	for i, s := range []string{"s=1&app=cont", "s=2&app=cont"} {
		if l := eventoDoApp(t, url, s); !strings.Contains(l, "n=1") {
			t.Errorf("sessão %d: %s", i+1, l)
		}
	}
}

func TestServeAppsEnvDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ADVPP_APPS_DIR", "")
	if err := serveAppsPrepara(&Options{apps: dir}); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	if got := os.Getenv("ADVPP_APPS_DIR"); got != abs {
		t.Errorf("ADVPP_APPS_DIR = %q, esperado %q", got, abs)
	}
	if err := serveAppsPrepara(&Options{apps: filepath.Join(dir, "nao-existe")}); err == nil {
		t.Error("diretório inexistente deveria dar erro")
	}
}

func TestParseOptionsApps(t *testing.T) {
	o := parseOptions([]string{"--apps", "d", "--home", "x"})
	if o.apps != "d" || o.home != "x" {
		t.Errorf("apps=%q home=%q", o.apps, o.home)
	}
}
