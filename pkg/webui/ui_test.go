package webui

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type uiRecebido struct {
	id    int
	event string
	value string
}

// abreSessao sobe o servidor com run, abre /events?s=<sid> e devolve um
// canal com as linhas "data: ..." do SSE.
func abreSessao(t *testing.T, run RunFunc, sid string) (*httptest.Server, <-chan string) {
	t.Helper()
	ts := httptest.NewServer(New("t.prw", run).Handler())
	t.Cleanup(ts.Close)
	resp, err := http.Get(ts.URL + "/events?s=" + sid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	linhas := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "data: ") {
				linhas <- strings.TrimPrefix(l, "data: ")
			}
		}
	}()
	return ts, linhas
}

func postUI(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func runQueEspera(got chan<- uiRecebido, n int) RunFunc {
	return func(ui *Provider, _ *OutWriter) error {
		for i := 0; i < n; i++ {
			id, ev, val := ui.UINext()
			got <- uiRecebido{id, ev, string(val)}
		}
		return nil
	}
}

func esperaUI(t *testing.T, got <-chan uiRecebido) uiRecebido {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("evento de UI não chegou ao programa")
	}
	return uiRecebido{}
}

func TestUIPostEntregaEvento(t *testing.T) {
	got := make(chan uiRecebido, 1)
	ts, _ := abreSessao(t, runQueEspera(got, 1), "a")
	if st := postUI(t, ts.URL+"/ui?s=a", `{"id":3,"event":"change","value":"abc"}`); st != http.StatusNoContent {
		t.Fatalf("status = %d", st)
	}
	r := esperaUI(t, got)
	if r.id != 3 || r.event != "change" || r.value != `"abc"` {
		t.Errorf("recebido %+v", r)
	}
}

func TestUISendViraEventoSSE(t *testing.T) {
	run := func(ui *Provider, _ *OutWriter) error {
		ui.UISend([]byte(`{"op":"close"}`))
		return nil
	}
	_, linhas := abreSessao(t, run, "b")
	fim := time.After(2 * time.Second)
	for {
		select {
		case l := <-linhas:
			if l == `{"type":"ui","data":{"op":"close"}}` {
				return
			}
		case <-fim:
			t.Fatal("evento ui não apareceu no SSE")
		}
	}
}

func TestUIPostInvalido(t *testing.T) {
	got := make(chan uiRecebido, 1)
	ts, _ := abreSessao(t, runQueEspera(got, 1), "c")
	casos := []struct {
		nome, url, body string
		want            []int
	}{
		{"json inválido", "/ui?s=c", `{`, []int{400}},
		{"sem id", "/ui?s=c", `{"event":"click"}`, []int{400}},
		{"sem event", "/ui?s=c", `{"id":1}`, []int{400}},
		{"sessão inexistente", "/ui?s=nao-existe", `{"id":1,"event":"click"}`, []int{404}},
		{"corpo grande", "/ui?s=c", `{"id":1,"event":"x","value":"` + strings.Repeat("a", 9<<20) + `"}`, []int{400, 413}},
	}
	for _, c := range casos {
		st := postUI(t, ts.URL+c.url, c.body)
		ok := false
		for _, w := range c.want {
			ok = ok || st == w
		}
		if !ok {
			t.Errorf("%s: status %d, esperado %v", c.nome, st, c.want)
		}
	}
	// A sessão continua recebendo eventos válidos
	postUI(t, ts.URL+"/ui?s=c", `{"id":7,"event":"click","value":null}`)
	if r := esperaUI(t, got); r.id != 7 || r.event != "click" {
		t.Errorf("recebido %+v", r)
	}
}

func TestUIPostOrigemCruzada(t *testing.T) {
	got := make(chan uiRecebido, 1)
	ts, _ := abreSessao(t, runQueEspera(got, 1), "d")
	req, _ := http.NewRequest("POST", ts.URL+"/ui?s=d", strings.NewReader(`{"id":1,"event":"click"}`))
	req.Header.Set("Origin", "http://evil.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, esperado 403", resp.StatusCode)
	}
}

func TestUIPostNaoBloqueiaAposAbandono(t *testing.T) {
	s := newSession()
	p := &Provider{s}
	s.abandon()
	feito := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ { // mais que o buffer
			p.UIPost(1, "timer", nil)
		}
		close(feito)
	}()
	select {
	case <-feito:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("UIPost bloqueou numa sessão abandonada")
	}
}

func TestUIPostWaitBloqueiaEDesbloqueia(t *testing.T) {
	s := newSession()
	p := &Provider{s}
	for i := 0; i < uiBuffer; i++ {
		p.UIPost(1, "x", nil) // enche a fila
	}
	ret := make(chan bool, 1)
	go func() { ret <- p.UIPostWait(1, "line", nil, nil) }()
	select {
	case <-ret:
		t.Fatal("UIPostWait não bloqueou com a fila cheia")
	case <-time.After(100 * time.Millisecond):
	}
	s.abandon()
	select {
	case ok := <-ret:
		if ok {
			t.Error("UIPostWait devolveu true numa sessão encerrada")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("UIPostWait continuou bloqueado após abandon()")
	}
}

// Fila de eventos cheia (programa fora do laço, ex.: preso num MsgYesNo):
// POST /ui responde na hora com 503 em vez de segurar a requisição.
func TestUIPostFilaCheia(t *testing.T) {
	got := make(chan uiRecebido, 1)
	bloqueia := make(chan struct{})
	run := func(ui *Provider, _ *OutWriter) error {
		<-bloqueia // nunca lê a fila
		return nil
	}
	ts, _ := abreSessao(t, run, "cheia")
	defer close(bloqueia)
	_ = got
	for i := 0; i < uiBuffer; i++ {
		postUI(t, ts.URL+"/ui?s=cheia", `{"id":1,"event":"click"}`)
	}
	ini := time.Now()
	if st := postUI(t, ts.URL+"/ui?s=cheia", `{"id":1,"event":"click"}`); st != http.StatusServiceUnavailable {
		t.Errorf("status = %d, esperado 503", st)
	}
	if time.Since(ini) > time.Second {
		t.Errorf("POST com fila cheia demorou %v", time.Since(ini))
	}
}

func TestIconesDoCSSEmbutido(t *testing.T) {
	ic := Icones()
	if len(ic) < 1000 || !ic["house"] || !ic["gauge"] || ic["nao-existe"] {
		t.Errorf("ícones: %d, house=%v gauge=%v", len(ic), ic["house"], ic["gauge"])
	}
}
