package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestRestServerFixture builda o advplc e roda tests/rest_server_test.prw
// como servidor REST real sobre HTTP, exercitando GET, GET com path param,
// POST com corpo JSON, 404 e 405 — a mesma pilha que qualquer cliente HTTP
// real usaria contra um WSRESTFUL do Protheus.
func TestRestServerFixture(t *testing.T) {
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

	const baseURL = "http://127.0.0.1:18321"

	cmd := exec.Command(binPath, "run", "tests/rest_server_test.prw")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer cmd.Process.Kill()

	client := &http.Client{Timeout: 3 * time.Second}
	if !waitForServer(client, baseURL+"/_routes", 10*time.Second) {
		t.Fatal("timeout esperando o servidor REST subir em " + baseURL)
	}

	t.Run("GET lista rotas registradas (auto-discovery @Get/@Post + AddRoute)", func(t *testing.T) {
		resp, body := doGet(t, client, baseURL+"/_routes")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
		}
		var routes []map[string]string
		if err := json.Unmarshal(body, &routes); err != nil {
			t.Fatalf("resposta inválida: %v (%s)", err, body)
		}
		if len(routes) < 4 {
			t.Fatalf("esperava >= 4 rotas, veio %d: %v", len(routes), routes)
		}
	})

	t.Run("GET /clientes retorna array (função @Get sem path param)", func(t *testing.T) {
		resp, body := doGet(t, client, baseURL+"/clientes")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
		}
		var list []map[string]any
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatalf("resposta inválida: %v (%s)", err, body)
		}
		if len(list) != 2 {
			t.Fatalf("esperava 2 clientes, veio %d: %s", len(list), body)
		}
		if list[0]["NOME"] != "Ana" {
			t.Errorf("clientes[0].NOME = %v, want Ana", list[0]["NOME"])
		}
	})

	t.Run("GET /clientes/{id} popula path param no oParam da função", func(t *testing.T) {
		resp, body := doGet(t, client, baseURL+"/clientes/42")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("resposta inválida: %v (%s)", err, body)
		}
		if m["id"] != "42" {
			t.Errorf("id = %v, want 42", m["id"])
		}
		if m["nome"] != "Cliente 42" {
			t.Errorf("nome = %v, want 'Cliente 42'", m["nome"])
		}
	})

	t.Run("POST /clientes decodifica corpo JSON no oParam da função", func(t *testing.T) {
		reqBody := bytes.NewBufferString(`{"nome":"Carla"}`)
		resp, err := client.Post(baseURL+"/clientes", "application/json", reqBody)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()
		var m map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			t.Fatalf("resposta inválida: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%v", resp.StatusCode, m)
		}
		if m["criado"] != true {
			t.Errorf("criado = %v, want true", m["criado"])
		}
		if m["nome"] != "Carla" {
			t.Errorf("nome = %v, want Carla", m["nome"])
		}
	})

	t.Run("GET /manual atende rota registrada manualmente via AddRoute", func(t *testing.T) {
		resp, body := doGet(t, client, baseURL+"/manual")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
		}
		if string(bytes.TrimSpace(body)) != `"rota manual OK"` {
			t.Errorf("body = %s, want \"rota manual OK\"", body)
		}
	})

	t.Run("404 para path não registrado", func(t *testing.T) {
		resp, _ := doGet(t, client, baseURL+"/nao-existe")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("405 para verbo não registrado num path existente", func(t *testing.T) {
		resp, err := client.Post(baseURL+"/manual", "application/json", bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})
}

func waitForServer(client *http.Client, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func doGet(t *testing.T, client *http.Client, url string) (*http.Response, []byte) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	return resp, body
}
