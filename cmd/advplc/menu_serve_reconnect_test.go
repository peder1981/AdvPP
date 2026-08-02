package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMenuServeReconnectResumesSession reproduz o bug relatado (2ª chamada de
// FWMenuSelect dentro de um loop, depois de já ter passado por outra tela):
// o EventSource do browser reconecta sozinho em qualquer queda de conexão
// (proxy, aba suspensa, blip de rede) reusando o MESMO sid — sem que a
// página tenha recarregado e sem que o usuário tenha feito nada. Antes da
// correção, o handler de /events tratava toda reconexão como sessão nova e
// reexecutava o programa do zero, enquanto a goroutine anterior (bloqueada
// esperando a resposta do menu) ficava presa pra sempre. Este teste conecta,
// avança até o 2º FWMenuSelect, DERRUBA a conexão SSE sem responder,
// reconecta com o mesmo sid e confirma que: (1) o programa NÃO reiniciou
// (não reaparece o "── executando" nem o menu "Menu 1"); e (2) a resposta ao
// diálogo que ficou pendente antes da queda ainda é aceita pela VM — prova
// de que a sessão foi retomada, não recriada. Ver pkg/webui/server.go.
func TestMenuServeReconnectResumesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("builda o binário e sobe um servidor HTTP; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	const baseURL = "http://127.0.0.1:18323"
	const sid = "recon1"

	cmd := exec.Command(binPath, "serve", "tests/menu_reconnect_test.prw", "--port", "18323")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer cmd.Process.Kill()

	client := &http.Client{Timeout: 3 * time.Second}
	if !waitForServer(client, baseURL+"/", 10*time.Second) {
		t.Fatal("timeout esperando o servidor web subir em " + baseURL)
	}

	type sseEvent struct {
		Type string          `json:"type"`
		ID   int             `json:"id"`
		Data json.RawMessage `json:"data"`
		Text string          `json:"text"`
	}
	// makeReader devolve um closure de leitura de eventos SSE ligado a um
	// bufio.Reader específico, com timeout — usado tanto pra ler
	// normalmente (timeout longo) quanto pra provar ausência de evento
	// (timeout curto, no passo que verifica que NÃO houve reinício). Uma
	// única goroutine "bombeia" o Reader pro canal pela vida inteira da
	// conexão — nunca duas goroutines lendo o mesmo bufio.Reader ao mesmo
	// tempo (uma leitura que estoura o timeout não pode simplesmente ser
	// abandonada e recriada a cada chamada: o evento que chega depois
	// seria consumido por uma goroutine cujo canal ninguém mais lê,
	// desaparecendo silenciosamente).
	makeReader := func(r *bufio.Reader) func(timeout time.Duration) (sseEvent, bool) {
		ch := make(chan sseEvent, 16)
		go func() {
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					close(ch)
					return
				}
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var ev sseEvent
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
					close(ch)
					return
				}
				ch <- ev
			}
		}()
		return func(timeout time.Duration) (sseEvent, bool) {
			t.Helper()
			select {
			case ev, ok := <-ch:
				return ev, ok
			case <-time.After(timeout):
				return sseEvent{}, false
			}
		}
	}
	reply := func(id int, result string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"id": id, "result": result})
		r, err := client.Post(baseURL+"/reply?s="+sid, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /reply: %v", err)
		}
		r.Body.Close()
	}
	connect := func(ctx context.Context) *http.Response {
		t.Helper()
		streamClient := &http.Client{Timeout: 0}
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/events?s="+sid, nil)
		resp, err := streamClient.Do(req)
		if err != nil {
			t.Fatalf("GET /events: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status /events = %d, want 200", resp.StatusCode)
		}
		return resp
	}

	// --- Conexão A: avança até o 2º FWMenuSelect, sem responder o 2º ---
	ctxA, cancelA := context.WithCancel(context.Background())
	respA := connect(ctxA)
	readA := makeReader(bufio.NewReader(respA.Body))

	var menu1ID, menu2ID int
	var menu2Title string
	sawPrimeira := false
	for menu2ID == 0 || !sawPrimeira {
		ev, ok := readA(5 * time.Second)
		if !ok {
			t.Fatal("conexão A: timeout ou erro esperando eventos")
		}
		switch ev.Type {
		case "menu":
			var d struct {
				Title string `json:"title"`
			}
			json.Unmarshal(ev.Data, &d)
			if menu1ID == 0 {
				menu1ID = ev.ID
				if d.Title != "Menu 1" {
					t.Fatalf("1º menu title = %q, want %q", d.Title, "Menu 1")
				}
				reply(menu1ID, "1")
			} else if menu2ID == 0 {
				menu2ID = ev.ID
				menu2Title = d.Title
				if d.Title != "Menu 2" {
					t.Fatalf("2º menu title = %q, want %q", d.Title, "Menu 2")
				}
				// Não responde — este é o diálogo que fica pendente
				// quando a conexão cai, exatamente o cenário relatado.
			}
		case "output":
			if strings.Contains(ev.Text, "primeira=1") {
				sawPrimeira = true
			}
		case "error":
			t.Fatalf("erro da VM: %s", ev.Text)
		}
	}
	if menu2Title != "Menu 2" {
		t.Fatalf("estado inesperado antes da queda: menu2Title=%q", menu2Title)
	}

	// Derruba a conexão A sem responder ao 2º menu — simula queda de rede.
	cancelA()
	respA.Body.Close()

	// --- Conexão B: reconecta com o MESMO sid ---
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	respB := connect(ctxB)
	defer respB.Body.Close()
	readB := makeReader(bufio.NewReader(respB.Body))

	// Prova de que NÃO houve reinício: se o bug estivesse presente, o
	// servidor recriaria a sessão e a VM reexecutaria do zero, emitindo
	// de novo "── executando" e o 1º menu ("Menu 1") nesta nova conexão.
	if ev, ok := readB(2 * time.Second); ok {
		t.Fatalf("conexão B recebeu evento inesperado logo ao reconectar (indício de reinício da sessão): type=%q text=%q data=%s",
			ev.Type, ev.Text, ev.Data)
	}

	// Responde ao diálogo que ficou pendente ANTES da queda, usando o id
	// capturado na conexão A — só funciona se a sessão foi retomada
	// (mesmo goroutine, mesmo canal `waiting`), não recriada.
	reply(menu2ID, "1")

	sawSegunda := false
	for !sawSegunda {
		ev, ok := readB(5 * time.Second)
		if !ok {
			t.Fatal("conexão B: timeout esperando a saída 'segunda=1' — sessão não foi retomada corretamente")
		}
		switch ev.Type {
		case "output":
			if strings.Contains(ev.Text, "segunda=1") {
				sawSegunda = true
			}
			if strings.Contains(ev.Text, "── executando") {
				t.Fatal("conexão B recebeu '── executando' — a sessão reiniciou em vez de retomar")
			}
		case "menu":
			var d struct {
				Title string `json:"title"`
			}
			json.Unmarshal(ev.Data, &d)
			t.Fatalf("conexão B recebeu um novo evento 'menu' (title=%q) — a sessão reiniciou em vez de retomar", d.Title)
		case "error":
			t.Fatalf("erro da VM: %s", ev.Text)
		}
	}
}
