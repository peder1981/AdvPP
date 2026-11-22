package vm

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// PFX de verdade (PKCS#12) com a cadeia dentro, como o A1 da ICP-Brasil
// exportado pelo Windows (cifra legada). testdata/pfx: AC de teste + cliente
// "CONDOMINIO TESTE", senha "teste" — gerados com
//   openssl pkcs12 -export -legacy -inkey cli.key -in cli.pem -certfile ca.pem
// (e sem -legacy para o de AES).
func TestPFXPKCS12ComCadeia(t *testing.T) {
	_, natives := newSegGenericaVM()
	pfx := filepath.Join("testdata", "pfx", "cadeia-3des.pfx")
	dir := t.TempDir()
	s := advplrt.NewString

	info, _ := segGenericaCall(natives, "PFXINFO", []advplrt.Value{s(pfx), s("teste")})
	arr, ok := info.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 2 {
		t.Fatalf("PFXInfo deveria trazer cliente + 1 CA, veio %v", info)
	}
	if dest := advplrt.ToString(arr.Elements[0].(*advplrt.ArrayValue).Elements[1]); !strings.Contains(dest, "CONDOMINIO TESTE") {
		t.Fatalf("primeiro item deveria ser o certificado de cliente, veio %q", dest)
	}

	chave := filepath.Join(dir, "chave.pem")
	if got, _ := segGenericaCall(natives, "PFXKEY2PEM", []advplrt.Value{s(pfx), s(chave), s(""), s("teste")}); !advplrt.ToBool(got) {
		t.Fatal("PFXKey2PEM deveria extrair a chave do PKCS#12")
	}
	if st, _ := os.Stat(chave); st.Mode().Perm() != 0o600 {
		t.Fatalf("chave privada gravada com %v, esperado 0600", st.Mode().Perm())
	}
	cert := filepath.Join(dir, "cert.pem")
	if got, _ := segGenericaCall(natives, "PFXCERT2PEM", []advplrt.Value{s(pfx), s(cert), s(""), s("teste")}); !advplrt.ToBool(got) {
		t.Fatal("PFXCert2PEM deveria extrair o certificado")
	}
	// Certificado e chave extraídos formam um par válido.
	if _, err := tls.LoadX509KeyPair(cert, chave); err != nil {
		t.Fatalf("certificado e chave não formam par: %v", err)
	}
	ca := filepath.Join(dir, "ca.pem")
	if got, _ := segGenericaCall(natives, "PFXCA2PEM", []advplrt.Value{s(pfx), s(ca), s(""), s("teste")}); !advplrt.ToBool(got) {
		t.Fatal("PFXCA2PEM deveria extrair a AC da cadeia")
	}

	if got, _ := segGenericaCall(natives, "PFXKEY2PEM", []advplrt.Value{s(pfx), s(chave), s(""), s("errada")}); advplrt.ToBool(got) {
		t.Fatal("senha errada deveria falhar")
	}
	if _, _, err := loadPfxMaterial(filepath.Join("testdata", "pfx", "cadeia-aes.pfx"), "teste"); err == nil ||
		!strings.Contains(err.Error(), "-legacy") {
		t.Fatalf("PFX AES deveria falhar explicando como reexportar, veio %v", err)
	}
}
