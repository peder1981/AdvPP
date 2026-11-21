package webui

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// resolverDeTeste: cada app imprime o próprio nome; "" é "inicio".
func resolverDeTeste(chamadas *atomic.Int32) AppResolver {
	return func(app string) (RunFunc, error) {
		chamadas.Add(1)
		if app == "sumido" {
			return nil, ErrAppNaoEncontrado
		}
		nome := app
		if nome == "" {
			nome = "inicio"
		}
		return func(ui *Provider, console *OutWriter) error {
			console.Write([]byte(nome + "\n"))
			return nil
		}, nil
	}
}

// primeiraSaida abre o SSE e devolve a primeira linha "output" do programa.
func primeiraSaida(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", url, resp.StatusCode)
	}
	linhas := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			linhas <- sc.Text()
		}
	}()
	fim := time.After(2 * time.Second)
	for {
		select {
		case l := <-linhas:
			if strings.Contains(l, `"type":"output"`) && !strings.Contains(l, "executando") {
				return l
			}
		case <-fim:
			t.Fatalf("%s: sem saída do programa", url)
		}
	}
}

func TestAppsSessaoUsaRunDoApp(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(NewApps("AdvOS", resolverDeTeste(&n)).Handler())
	defer ts.Close()
	if l := primeiraSaida(t, ts.URL+"/events?s=1&app=relogio"); !strings.Contains(l, `"text":"relogio"`) {
		t.Errorf("app relogio: %s", l)
	}
	if l := primeiraSaida(t, ts.URL+"/events?s=2"); !strings.Contains(l, `"text":"inicio"`) {
		t.Errorf("página inicial: %s", l)
	}
}

func TestAppsNomeInvalido(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(NewApps("AdvOS", resolverDeTeste(&n)).Handler())
	defer ts.Close()
	for _, nome := range []string{"..%2Fx", "a%2Fb", "a%00b", strings.Repeat("a", 65), "a.b"} {
		resp, err := http.Get(ts.URL + "/events?s=x&app=" + nome)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("app=%s: status %d, esperado 400", nome, resp.StatusCode)
		}
	}
	if n.Load() != 0 {
		t.Errorf("o resolver foi chamado %d vezes com nome inválido", n.Load())
	}
}

func TestAppsNaoEncontrado(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(NewApps("AdvOS", resolverDeTeste(&n)).Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/events?s=x&app=sumido")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, esperado 404", resp.StatusCode)
	}
}

func TestAppsRotaServeIndex(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(NewApps("AdvOS", resolverDeTeste(&n)).Handler())
	defer ts.Close()
	corpo := func(url string) (int, string) {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	stRaiz, raiz := corpo(ts.URL + "/")
	stApp, app := corpo(ts.URL + "/app/relogio")
	if stApp != 200 || app != raiz || stRaiz != 200 || !strings.Contains(raiz, "<app-root") {
		t.Errorf("/app/relogio: status %d, igual ao index: %v", stApp, app == raiz)
	}
	if st, _ := corpo(ts.URL + "/app/a.b"); st != http.StatusBadRequest {
		t.Errorf("/app/a.b: status %d, esperado 400", st)
	}
}

func TestServeUnicoRecusaApp(t *testing.T) {
	run := func(ui *Provider, console *OutWriter) error {
		console.Write([]byte("unico\n"))
		return nil
	}
	ts := httptest.NewServer(New("p.prw", run).Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/events?s=1&app=x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("app=x no serve de um programa: status %d, esperado 404", resp.StatusCode)
	}
	if l := primeiraSaida(t, ts.URL+"/events?s=2"); !strings.Contains(l, `"text":"unico"`) {
		t.Errorf("serve de um programa: %s", l)
	}
}
