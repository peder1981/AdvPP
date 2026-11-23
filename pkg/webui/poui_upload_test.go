package webui

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/tools/shared"
)

func postUpload(t *testing.T, token, filename, conteudo string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(conteudo))
	_ = w.Close()

	ts := httptest.NewServer(New("t.prw", nil).Handler())
	defer ts.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/poui-upload?token="+token, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Body.Write(body)
	return rec
}

func TestUploadForaDoDiretorio403(t *testing.T) {
	token := shared.NovoTokenUpload()
	// /tmp está fora do cwd do teste? cwd do `go test` é o pacote — /tmp
	// só é permitido se estiver sob cwd ou poui_upload_dirs. Usa um dir
	// garantidamente fora: a raiz do filesystem.
	fora := string(filepath.Separator)
	shared.RegistrarUploadPoui(token, fora, 10, nil)
	defer shared.RemoverUploadPoui(token)

	rec := postUpload(t, token, "x.txt", "oi")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestUploadNomeComBarras(t *testing.T) {
	// Nome com separadores deve virar filepath.Base antes de gravar.
	cwd, _ := os.Getwd()
	dentro := filepath.Join(cwd, ".upload-teste-"+t.Name())
	defer os.RemoveAll(dentro)
	token := shared.NovoTokenUpload()
	shared.RegistrarUploadPoui(token, dentro, 10, nil)
	defer shared.RemoverUploadPoui(token)

	rec := postUpload(t, token, "..\\..\\evil.txt", "conteudo")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	// Deve ter gravado "evil.txt" dentro do destino, não fora.
	if _, err := os.Stat(filepath.Join(dentro, "evil.txt")); err != nil {
		t.Fatalf("arquivo não gravado com nome sanitizado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "evil.txt")); err == nil {
		t.Fatal("arquivo escapou do diretório de destino")
	}
}

func TestUploadExtensaoProibida415(t *testing.T) {
	cwd, _ := os.Getwd()
	dentro := filepath.Join(cwd, ".upload-teste-ext-"+t.Name())
	defer os.RemoveAll(dentro)

	token := shared.NovoTokenUpload()
	shared.RegistrarUploadPoui(token, dentro, 10, []string{".png"})
	defer shared.RemoverUploadPoui(token)

	rec := postUpload(t, token, "virus.exe", "MZ")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestUploadOkGravaEDevolveJSON(t *testing.T) {
	cwd, _ := os.Getwd()
	dentro := filepath.Join(cwd, ".upload-teste-ok-"+t.Name())
	defer os.RemoveAll(dentro)

	token := shared.NovoTokenUpload()
	shared.RegistrarUploadPoui(token, dentro, 10, []string{".txt"})
	defer shared.RemoverUploadPoui(token)

	rec := postUpload(t, token, "nota.txt", "olá")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "cNome") || !strings.Contains(body, "nota.txt") || !strings.Contains(body, "nBytes") {
		t.Fatalf("resposta = %s", body)
	}
	if _, err := os.Stat(filepath.Join(dentro, "nota.txt")); err != nil {
		t.Fatalf("arquivo não gravado: %v", err)
	}
}
