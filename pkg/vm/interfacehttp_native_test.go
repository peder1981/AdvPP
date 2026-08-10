package vm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func newHTTPTestVM() *VM {
	return NewVM(&compiler.Bytecode{}, false)
}

func callHTTPNative(t *testing.T, v *VM, name string, args []advplrt.Value) advplrt.Value {
	t.Helper()
	fn, ok := v.natives[name]
	if !ok {
		t.Fatalf("native %s não registrada", name)
	}
	got, err := fn.Fn(args)
	if err != nil {
		t.Fatalf("%s retornou erro: %v", name, err)
	}
	return got
}

func TestHTTPGetExemploTDN(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pageteste.htm" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		io.WriteString(w, "<html>pagina de teste</html>")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGET", []advplrt.Value{advplrt.NewString(srv.URL + "/pageteste.htm")})
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("HTTPGet() = %T, quer string", got)
	}
	if s.Val != "<html>pagina de teste</html>" {
		t.Errorf("HTTPGet() = %q, quer corpo esperado", s.Val)
	}
}

func TestHTTPGetComParmsEPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "Id=123&Nome=Teste" {
			t.Errorf("query inesperada: %q", r.URL.RawQuery)
		}
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGET", []advplrt.Value{
		advplrt.NewString(srv.URL + "/funteste.asp"),
		advplrt.NewString("Id=123&Nome=Teste"),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "ok" {
		t.Errorf("HTTPGet com parms = %v, quer \"ok\"", got)
	}
}

func TestHTTPGetURLInvalidaRetornaNil(t *testing.T) {
	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGET", []advplrt.Value{advplrt.NewString("http://host-inexistente-advpp.invalid/x")})
	if got != advplrt.Nil {
		t.Errorf("HTTPGet URL invalida = %v, quer Nil", got)
	}
}

func TestHTTPGetTimeoutRetornaNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		io.WriteString(w, "tarde demais")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGET", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString(""),
		advplrt.NewNumber(0.1),
	})
	if got != advplrt.Nil {
		t.Errorf("HTTPGet com timeout curto = %v, quer Nil", got)
	}
}

func TestHTTPGetNaoSegueRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redir", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/destino", http.StatusFound)
	})
	mux.HandleFunc("/destino", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "destino")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGET", []advplrt.Value{
		advplrt.NewString(srv.URL + "/redir"),
		advplrt.NewString(""),
		advplrt.NewNumber(5),
		advplrt.NewArray(nil),
		advplrt.NewString(""),
		advplrt.NewBool(false),
	})
	s, _ := got.(*advplrt.StringValue)
	if s != nil && s.Val == "destino" {
		t.Errorf("HTTPGet sem seguir redirect retornou o conteudo do destino: %q", s.Val)
	}
	if v.legacyHTTPStatus != http.StatusFound {
		t.Errorf("HTTPGet sem seguir redirect status = %d, quer 302", v.legacyHTTPStatus)
	}
}

func TestHTTPPostExemploTDN(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("metodo inesperado: %s", r.Method)
		}
		if r.URL.RawQuery != "REQUEST=1212" {
			t.Errorf("query inesperada: %q", r.URL.RawQuery)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "EXAMPLEFIELD=DUMMY" {
			t.Errorf("post body inesperado: %q", string(body))
		}
		io.WriteString(w, "post-ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPPOST", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString("REQUEST=1212"),
		advplrt.NewString("EXAMPLEFIELD=DUMMY"),
		advplrt.NewNumber(2),
		advplrt.NewArray(nil),
		advplrt.NewString(""),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "post-ok" {
		t.Errorf("HTTPPost = %v, quer \"post-ok\"", got)
	}
}

func TestHTTPPostComHeadersPipes(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	callHTTPNative(t, v, "HTTPPOST", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString(""),
		advplrt.NewString("a=b"),
		advplrt.NewNumber(2),
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewString("Content-Type| application/x-www-form-urlencoded"),
			advplrt.NewString("User-Agent: AdvPP-Test"),
		}),
		advplrt.NewString(""),
	})
	if gotCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type header = %q, quer application/x-www-form-urlencoded", gotCT)
	}
}

func TestHTTPCGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("metodo inesperado: %s", r.Method)
		}
		io.WriteString(w, "cget-ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPCGET", []advplrt.Value{advplrt.NewString(srv.URL)})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "cget-ok" {
		t.Errorf("HTTPCGet = %v, quer \"cget-ok\"", got)
	}
}

func TestHTTPCPostExemploTDN(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("metodo inesperado: %s", r.Method)
		}
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, "cpost-ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPCPOST", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString("Conteudo a ser enviado via Post"),
		advplrt.NewNumber(2),
		advplrt.NewArray([]advplrt.Value{
			advplrt.NewString("Content-Type| application/json"),
		}),
		advplrt.NewString(""),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "cpost-ok" {
		t.Errorf("HTTPCPost = %v, quer \"cpost-ok\"", got)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, quer application/json", gotCT)
	}
	if gotBody != "Conteudo a ser enviado via Post" {
		t.Errorf("body = %q, quer conteudo postado", gotBody)
	}
}

func TestHTTPQuoteMethodCustom(t *testing.T) {
	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, "quote-ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPQUOTE", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString("DELETE"),
		advplrt.NewString(""),
		advplrt.NewString("payload"),
		advplrt.NewNumber(2),
		advplrt.NewArray(nil),
		advplrt.NewString(""),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "quote-ok" {
		t.Errorf("HTTPQuote = %v, quer \"quote-ok\"", got)
	}
	if gotMethod != "DELETE" {
		t.Errorf("metodo = %q, quer DELETE", gotMethod)
	}
	if gotBody != "payload" {
		t.Errorf("body = %q, quer payload", gotBody)
	}
}

func TestHTTPGetStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	v := newHTTPTestVM()
	callHTTPNative(t, v, "HTTPGET", []advplrt.Value{advplrt.NewString(srv.URL)})

	got := callHTTPNative(t, v, "HTTPGETSTATUS", []advplrt.Value{advplrt.NewString("")})
	n, _ := got.(*advplrt.NumberValue)
	if n == nil || int(n.Val) != http.StatusOK {
		t.Errorf("HTTPGetStatus apos 200 = %v, quer 200", got)
	}

	callHTTPNative(t, v, "HTTPGET", []advplrt.Value{advplrt.NewString("http://host-inexistente-advpp.invalid/x")})
	got = callHTTPNative(t, v, "HTTPGETSTATUS", nil)
	n, _ = got.(*advplrt.NumberValue)
	if n != nil && n.Val >= 100 {
		t.Errorf("HTTPGetStatus apos falha = %v, quer < 100", got)
	}
}

func TestHTTPSetPassSetProxySetNoProxyFor(t *testing.T) {
	v := newHTTPTestVM()

	got := callHTTPNative(t, v, "HTTPSETPASS", []advplrt.Value{advplrt.NewString("user"), advplrt.NewString("pass")})
	if got != advplrt.Nil {
		t.Errorf("HTTPSetPass = %v, quer Nil", got)
	}

	got = callHTTPNative(t, v, "SETPROXY", []advplrt.Value{advplrt.NewString("myproxyserver.com"), advplrt.NewNumber(8080)})
	if got != advplrt.Nil {
		t.Errorf("SetProxy = %v, quer Nil", got)
	}

	got = callHTTPNative(t, v, "SETNOPROXYFOR", []advplrt.Value{advplrt.NewString("*.domain.*, *.domain2")})
	if got != advplrt.Nil {
		t.Errorf("SetNoProxyFor = %v, quer Nil", got)
	}
}

// genTestCert gera um par certificado/chave PEM self-signed em diretório temp
// para os testes das variantes HTTPS.
func genTestCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gerar chave: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "advpp-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("criar certificado: %v", err)
	}
	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("escrever cert: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("escrever key: %v", err)
	}
	return certPath, keyPath
}

func TestHTTPSGet(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("metodo inesperado: %s", r.Method)
		}
		io.WriteString(w, "https-get-ok")
	}))
	defer srv.Close()

	certPath, keyPath := genTestCert(t)

	// ADVPP_HTTP_INSECURE habilita skip de verificacao para teste contra
	// servidor TLS self-signed (mesmo padrao opt-in do ADVPP_SFTP_KNOWN_HOSTS).
	t.Setenv("ADVPP_HTTP_INSECURE", "1")
	_ = certPath
	_ = keyPath

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPSGET", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "https-get-ok" {
		t.Errorf("HTTPSGet = %v, quer \"https-get-ok\"", got)
	}
}

func TestHTTPSPost(t *testing.T) {
	var gotBody string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("metodo inesperado: %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, "https-post-ok")
	}))
	defer srv.Close()

	t.Setenv("ADVPP_HTTP_INSECURE", "1")

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPSPOST", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString("corpo-post"),
		advplrt.NewNumber(2),
		advplrt.NewArray(nil),
		advplrt.NewString(""),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "https-post-ok" {
		t.Errorf("HTTPSPost = %v, quer \"https-post-ok\"", got)
	}
	if gotBody != "corpo-post" {
		t.Errorf("body = %q, quer corpo-post", gotBody)
	}
}

func TestHTTPSQuote(t *testing.T) {
	var gotMethod string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		io.WriteString(w, "https-quote-ok")
	}))
	defer srv.Close()

	t.Setenv("ADVPP_HTTP_INSECURE", "1")

	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPSQUOTE", []advplrt.Value{
		advplrt.NewString(srv.URL),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString("PUT"),
	})
	s, _ := got.(*advplrt.StringValue)
	if s == nil || s.Val != "https-quote-ok" {
		t.Errorf("HTTPSQuote = %v, quer \"https-quote-ok\"", got)
	}
	if gotMethod != "PUT" {
		t.Errorf("metodo = %q, quer PUT", gotMethod)
	}
}

func TestHTTPGetStatusSemRequisicao(t *testing.T) {
	v := newHTTPTestVM()
	got := callHTTPNative(t, v, "HTTPGETSTATUS", nil)
	n, _ := got.(*advplrt.NumberValue)
	if n != nil && n.Val != 0 {
		t.Errorf("HTTPGetStatus sem requisicao = %v, quer 0", got)
	}
}
