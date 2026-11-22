package webui

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func ctxTeste(t *testing.T) *PouiCtx {
	t.Helper()
	c, err := NovoPouiCtx("meuapp", "01", "01", "EST", "admin", "pt-br", "/rest", "http://localhost:8080")
	if err != nil {
		t.Fatalf("NovoPouiCtx: %v", err)
	}
	return c
}

func TestSessionStorageChaves(t *testing.T) {
	ss := ctxTeste(t).SessionStorage()
	for _, k := range []string{
		"ProCompany", "ProBranch", "ProAppConfig", "ERPAPPCONFIG",
		"ProModule", "appName", "ProSystemModule", "ProDatabase",
		"ProIdiom", "ProRole", "ProUser", "RemoteType", "SocketPort", "ERPTOKEN",
	} {
		if _, ok := ss[k]; !ok {
			t.Errorf("sessionStorage sem chave %q", k)
		}
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(ss["ERPAPPCONFIG"]), &cfg); err != nil {
		t.Fatalf("ERPAPPCONFIG não-JSON: %v", err)
	}
	if cfg["api_baseUrl"] != "http://localhost:8080/rest" {
		t.Fatalf("api_baseUrl = %q", cfg["api_baseUrl"])
	}
	if cfg["restEntryPoint"] != "/rest" || cfg["serverBackend"] != "http://localhost:8080" {
		t.Fatalf("ERPAPPCONFIG = %v", cfg)
	}
	var comp map[string]string
	if err := json.Unmarshal([]byte(ss["ProCompany"]), &comp); err != nil {
		t.Fatalf("ProCompany não-JSON: %v", err)
	}
	if comp["Code"] != "01" {
		t.Fatalf("ProCompany = %v", comp)
	}
	if ss["RemoteType"] != "1" || ss["SocketPort"] != "0" {
		t.Fatalf("RemoteType/SocketPort = %q/%q", ss["RemoteType"], ss["SocketPort"])
	}
}

func TestTokenAleatorioPorSessao(t *testing.T) {
	a, b := ctxTeste(t), ctxTeste(t)
	re := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, c := range []*PouiCtx{a, b} {
		if !re.MatchString(c.Token) {
			t.Fatalf("token %q não é hex de 32 bytes", c.Token)
		}
	}
	if a.Token == b.Token {
		t.Fatal("tokens iguais entre sessões")
	}
}

func TestInjetaShimAntesDoPrimeiroScript(t *testing.T) {
	out := InjetaShim([]byte(`<html><head><script src="x.js"></script></head><body></body></html>`))
	s := string(out)
	shim := strings.Index(s, "poui_sid")
	orig := strings.Index(s, `src="x.js"`)
	if shim < 0 || orig < 0 || shim > orig {
		t.Fatalf("shim não está antes do script original:\n%s", s)
	}
	// O <script> original continua depois do shim.
	if !strings.Contains(s, `src="x.js"`) {
		t.Fatal("script original sumiu")
	}
}

func TestInjetaShimSemScript(t *testing.T) {
	out := InjetaShim([]byte(`<html><body>oi</body></html>`))
	s := string(out)
	shim := strings.Index(s, "poui_sid")
	body := strings.Index(s, "</body>")
	if shim < 0 || shim > body {
		t.Fatalf("shim não está antes de </body:\n%s", s)
	}
}

func TestShimSemIdentificadores(t *testing.T) {
	data, err := os.ReadFile("poui_shim.js")
	if err != nil {
		t.Fatalf("ler poui_shim.js: %v", err)
	}
	s := string(data)
	for _, proibido := range []string{"advpp", "AdvPP", "claude", "cognition", "http://", "https://"} {
		if strings.Contains(s, proibido) {
			t.Errorf("poui_shim.js contém %q", proibido)
		}
	}
	for _, r := range s {
		if r >= 0x200B && r <= 0x200D || r == 0x2060 || r == 0xFEFF {
			t.Error("poui_shim.js contém caractere invisível")
		}
	}
	if n := len(strings.Split(strings.TrimSpace(s), "\n")); n > 120 {
		t.Errorf("poui_shim.js com %d linhas (>120)", n)
	}
}
