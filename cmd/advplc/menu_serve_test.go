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

// TestMenuServeFixture roda tests/menu_serve_test.prw via `advplc serve` e
// exercita FWMenuSelect/FWGetText contra uma sessão real: conecta em
// /events?s=ID (SSE), lê os eventos "menu" e "input" que a VM emite
// (bloqueada esperando resposta), responde via POST /reply — mesmo
// protocolo que o app Angular usa — e confirma que a VM recebeu a
// resposta e seguiu a execução com o valor escolhido. Ver
// pkg/webui/server.go (Provider.Menu/InputText) e
// web/src/app/app.ts (handlers dos eventos "menu"/"input").
func TestMenuServeFixture(t *testing.T) {
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

	const baseURL = "http://127.0.0.1:18322"

	cmd := exec.Command(binPath, "serve", "tests/menu_serve_test.prw", "--port", "18322")
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

	// Timeout: 0 no client — /events é uma conexão SSE de longa duração;
	// um Timeout fixo no client corta a leitura do corpo no meio do
	// stream, não só a resposta inicial (foi o que quebrou esta linha na
	// primeira versão do teste, mesmo com a VM respondendo corretamente
	// do outro lado). O bound geral do teste vem do contexto abaixo.
	streamClient := &http.Client{Timeout: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/events?s=teste1", nil)
	resp, err := streamClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status /events = %d, want 200", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	type sseEvent struct {
		Type string          `json:"type"`
		ID   int             `json:"id"`
		Data json.RawMessage `json:"data"`
		Text string          `json:"text"`
	}
	nextEvent := func() sseEvent {
		t.Helper()
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("lendo SSE: %v", err)
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var ev sseEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
				t.Fatalf("evento SSE inválido %q: %v", line, err)
			}
			return ev
		}
	}
	reply := func(id int, result string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"id": id, "result": result})
		r, err := client.Post(baseURL+"/reply?s=teste1", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /reply: %v", err)
		}
		r.Body.Close()
	}

	// Um único loop despachando por tipo — nunca descarta um evento "output"
	// que chegue entre um "menu" e um "input" (foi exatamente esse descarte,
	// numa versão anterior deste teste que lia por tipo esperado isolado,
	// que fazia o teste falhar mesmo com a VM respondendo certo do outro
	// lado: o ConOut de "escolhido=2" chega ANTES do evento "input" seguinte,
	// e ficava perdido enquanto o teste só procurava por "input").
	var gotMenuTitle string
	var gotMenuItems []string
	var gotInputPrompt string
	var sawEscolhido, sawCompetencia, repliedMenu, repliedInput bool
	for !(sawEscolhido && sawCompetencia) {
		ev := nextEvent()
		switch ev.Type {
		case "menu":
			var d struct {
				Title string   `json:"title"`
				Items []string `json:"items"`
			}
			if err := json.Unmarshal(ev.Data, &d); err != nil {
				t.Fatalf("menu data inválido: %v", err)
			}
			gotMenuTitle, gotMenuItems = d.Title, d.Items
			reply(ev.ID, "2") // escolhe a segunda opção
			repliedMenu = true
		case "input":
			var d struct {
				Prompt string `json:"prompt"`
				Def    string `json:"def"`
			}
			if err := json.Unmarshal(ev.Data, &d); err != nil {
				t.Fatalf("input data inválido: %v", err)
			}
			gotInputPrompt = d.Prompt
			reply(ev.ID, "2026-08")
			repliedInput = true
		case "output":
			if strings.Contains(ev.Text, "escolhido=2") {
				sawEscolhido = true
			}
			if strings.Contains(ev.Text, "competencia=2026-08") {
				sawCompetencia = true
			}
		case "error":
			t.Fatalf("erro da VM: %s", ev.Text)
		}
	}

	if !repliedMenu || !repliedInput {
		t.Fatalf("não recebeu os eventos menu/input esperados (menu=%v input=%v)", repliedMenu, repliedInput)
	}
	if len(gotMenuItems) != 2 || gotMenuItems[0] != "Primeira Opção" || gotMenuItems[1] != "Segunda Opção" {
		t.Errorf("menu items = %v, want [Primeira Opção Segunda Opção]", gotMenuItems)
	}
	if gotMenuTitle != "Escolha uma tela" {
		t.Errorf("menu title = %q, want %q", gotMenuTitle, "Escolha uma tela")
	}
	if gotInputPrompt != "Qual competência?" {
		t.Errorf("input prompt = %q, want %q", gotInputPrompt, "Qual competência?")
	}
	if !sawEscolhido {
		t.Error("esperava saída 'escolhido=2' (segunda opção do menu) — VM não recebeu a resposta do /reply corretamente")
	}
	if !sawCompetencia {
		t.Error("esperava saída 'competencia=2026-08' — VM não recebeu o texto respondido corretamente")
	}
}
