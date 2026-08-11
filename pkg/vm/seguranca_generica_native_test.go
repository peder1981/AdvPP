package vm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newSegGenericaVM cria uma VM com as natives de Segurança/Genérica registradas
// manualmente (o registro central em natives.go é feito à parte).
func newSegGenericaVM() (*VM, map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerSegurancaGenericaNatives(natives)
	return v, natives
}

func segGenericaCall(natives map[string]func(args []advplrt.Value) (advplrt.Value, error), name string, args []advplrt.Value) (advplrt.Value, error) {
	return natives[name](args)
}

// makeTestCert gera um certificado X.509 auto-assinado (RSA) e retorna o PEM do
// certificado, o PEM da chave e o certificado parseado.
func makeTestCert(t *testing.T, cn string, serial int64) (certPEM, keyPEM []byte, cert *x509.Certificate) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"AdvPP Org"}},
		Issuer:       pkix.Name{CommonName: cn, Organization: []string{"AdvPP Org"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM, cert
}

// certFingerprints devolve (base64, hex) do fingerprint SHA1 do certificado.
func certFingerprints(t *testing.T, got advplrt.Value) (string, string) {
	t.Helper()
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 1 {
		t.Fatalf("PEMInfo deveria retornar 1 certificado, veio %v", got.Type())
	}
	info, ok := arr.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(info.Elements) != 8 {
		t.Fatalf("info do certificado deveria ter 8 itens, veio %d", len(info.Elements))
	}
	return advplrt.ToString(info.Elements[6]), advplrt.ToString(info.Elements[7])
}

// TestPEMInfo valida extração de informações de um certificado auto-assinado
// (spec TDN: vetor de 8 itens por certificado).
func TestPEMInfo(t *testing.T) {
	_, natives := newSegGenericaVM()
	certPEM, _, cert := makeTestCert(t, "AdvPP Test Cert", 424242)
	dir := t.TempDir()
	pemFile := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(pemFile, certPEM, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := segGenericaCall(natives, "PEMINFO", []advplrt.Value{advplrt.NewString(pemFile)})
	if err != nil {
		t.Fatalf("PEMInfo retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 1 {
		t.Fatalf("PEMInfo deveria retornar vetor com 1 certificado, veio %v", got.Type())
	}
	info, ok := arr.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(info.Elements) != 8 {
		t.Fatalf("info do certificado deveria ter 8 itens, veio %d", len(info.Elements))
	}
	if advplrt.ToString(info.Elements[0]) != "2" { // X.509 v3
		t.Errorf("versão = %s, quer 2", advplrt.ToString(info.Elements[0]))
	}
	if !strings.Contains(advplrt.ToString(info.Elements[1]), "AdvPP Test Cert") {
		t.Errorf("subject = %q, deveria conter CN", advplrt.ToString(info.Elements[1]))
	}
	if advplrt.ToString(info.Elements[5]) != "424242" {
		t.Errorf("serial = %q, quer 424242", advplrt.ToString(info.Elements[5]))
	}
	// Fingerprint SHA1: base64 não vazio e hex com 40 chars.
	b64, hexStr := certFingerprints(t, got)
	if b64 == "" {
		t.Error("fingerprint base64 não deveria ser vazio")
	}
	if len(hexStr) != 40 {
		t.Errorf("fingerprint SHA1 hex deveria ter 40 chars, tem %d", len(hexStr))
	}
	_ = cert

	// nHashAlgorithm = 5 (SHA256): fingerprint hex com 64 chars.
	got, _ = segGenericaCall(natives, "PEMINFO", []advplrt.Value{advplrt.NewString(pemFile), advplrt.NewString(""), advplrt.NewNumber(5)})
	_, hexStr = certFingerprints(t, got)
	if len(hexStr) != 64 {
		t.Errorf("fingerprint SHA256 hex deveria ter 64 chars, tem %d", len(hexStr))
	}

	// Arquivo inexistente -> Nil.
	got, _ = segGenericaCall(natives, "PEMINFO", []advplrt.Value{advplrt.NewString(filepath.Join(dir, "nao-existe.pem"))})
	if !advplrt.IsNil(got) {
		t.Errorf("PEMInfo de arquivo inexistente deveria retornar Nil, retornou %q", advplrt.ToString(got))
	}

	// Conteúdo PEM passado diretamente (não path).
	got, _ = segGenericaCall(natives, "PEMINFO", []advplrt.Value{advplrt.NewString(string(certPEM))})
	if advplrt.IsNil(got) {
		t.Error("PEMInfo com conteúdo PEM deveria funcionar")
	}

	// Arquivo PEM sem certificados -> vetor vazio.
	emptyFile := filepath.Join(dir, "empty.pem")
	if err := os.WriteFile(emptyFile, []byte("texto sem blocos"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PEMINFO", []advplrt.Value{advplrt.NewString(emptyFile)})
	if arr, ok := got.(*advplrt.ArrayValue); !ok || len(arr.Elements) != 0 {
		t.Errorf("PEMInfo sem certificados deveria retornar vetor vazio, veio %v", got.Type())
	}
}

// TestPK8Key2PEM valida a conversão PKCS#8 DER -> PEM (spec TDN).
func TestPK8Key2PEM(t *testing.T) {
	_, natives := newSegGenericaVM()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	dir := t.TempDir()
	pk8File := filepath.Join(dir, "key.pk8")
	outFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(pk8File, pkcs8DER, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := segGenericaCall(natives, "PK8KEY2PEM", []advplrt.Value{advplrt.NewString(pk8File), advplrt.NewString(outFile)})
	if err != nil {
		t.Fatalf("PK8Key2PEM retornou erro: %v", err)
	}
	if !advplrt.ToBool(got) {
		t.Fatalf("PK8Key2PEM deveria retornar .T., retornou .F.")
	}
	out, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("leitura do PEM de saída: %v", err)
	}
	block, _ := pem.Decode(out)
	if block == nil || block.Type != "PRIVATE KEY" {
		t.Fatalf("saída deveria ser bloco PEM PRIVATE KEY, veio tipo %v", blockTypeOf(block))
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey da saída: %v", err)
	}
	if parsed == nil {
		t.Fatal("chave parseada é nil")
	}

	// Entrada PKCS#1 DER também deve converter para PRIVATE KEY (PKCS#8).
	pkcs1DER := x509.MarshalPKCS1PrivateKey(priv)
	pk1File := filepath.Join(dir, "key.pk1")
	if err := os.WriteFile(pk1File, pkcs1DER, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PK8KEY2PEM", []advplrt.Value{advplrt.NewString(pk1File), advplrt.NewString(outFile)})
	if !advplrt.ToBool(got) {
		t.Error("PK8Key2PEM com DER PKCS#1 deveria retornar .T.")
	}

	// cPassword informado -> .F. (saída criptografada não suportada).
	got, _ = segGenericaCall(natives, "PK8KEY2PEM", []advplrt.Value{advplrt.NewString(pk8File), advplrt.NewString(outFile), advplrt.NewString(""), advplrt.NewString("1234")})
	if advplrt.ToBool(got) {
		t.Error("PK8Key2PEM com senha de saída deveria retornar .F. (limitação documentada)")
	}

	// Arquivo inexistente -> .F.
	got, _ = segGenericaCall(natives, "PK8KEY2PEM", []advplrt.Value{advplrt.NewString(filepath.Join(dir, "nao.pk8")), advplrt.NewString(outFile)})
	if advplrt.ToBool(got) {
		t.Error("PK8Key2PEM de arquivo inexistente deveria retornar .F.")
	}
}

func blockTypeOf(block *pem.Block) interface{} {
	if block == nil {
		return nil
	}
	return block.Type
}

// TestPFXExtraiCertChave valida os conversores PFX* com fallback PEM
// (a stdlib Go não suporta PKCS#12 — limitação documentada).
func TestPFXExtraiCertChave(t *testing.T) {
	_, natives := newSegGenericaVM()
	certPEM, keyPEM, _ := makeTestCert(t, "AdvPP PFX Cert", 7)
	dir := t.TempDir()
	pfxFile := filepath.Join(dir, "bundle.pfx")
	bundle := append(append(append([]byte{}, certPEM...), keyPEM...), '\n')
	if err := os.WriteFile(pfxFile, bundle, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// PFXCert2PEM -> .T., saída contém CERTIFICATE.
	outCert := filepath.Join(dir, "cert.pem")
	got, err := segGenericaCall(natives, "PFXCERT2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outCert), advplrt.NewString("")})
	if err != nil {
		t.Fatalf("PFXCert2PEM retornou erro: %v", err)
	}
	if !advplrt.ToBool(got) {
		t.Fatal("PFXCert2PEM deveria retornar .T.")
	}
	out, _ := os.ReadFile(outCert)
	if !strings.Contains(string(out), "BEGIN CERTIFICATE") {
		t.Error("PFXCert2PEM deveria gravar um certificado PEM")
	}

	// PFXKey2PEM -> .T., saída contém RSA PRIVATE KEY.
	outKey := filepath.Join(dir, "key.pem")
	got, _ = segGenericaCall(natives, "PFXKEY2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outKey), advplrt.NewString("")})
	if !advplrt.ToBool(got) {
		t.Fatal("PFXKey2PEM deveria retornar .T.")
	}
	out, _ = os.ReadFile(outKey)
	if !strings.Contains(string(out), "BEGIN RSA PRIVATE KEY") {
		t.Error("PFXKey2PEM deveria gravar uma chave privada PEM")
	}

	// PFX2PEM -> .T., saída contém certificado E chave.
	outBoth := filepath.Join(dir, "both.pem")
	got, _ = segGenericaCall(natives, "PFX2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outBoth), advplrt.NewString("")})
	if !advplrt.ToBool(got) {
		t.Fatal("PFX2PEM deveria retornar .T.")
	}
	out, _ = os.ReadFile(outBoth)
	if !strings.Contains(string(out), "BEGIN CERTIFICATE") || !strings.Contains(string(out), "BEGIN RSA PRIVATE KEY") {
		t.Error("PFX2PEM deveria gravar certificado e chave")
	}

	// PFXCA2PEM sem CA -> .F.
	outCA := filepath.Join(dir, "ca.pem")
	got, _ = segGenericaCall(natives, "PFXCA2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outCA), advplrt.NewString("")})
	if advplrt.ToBool(got) {
		t.Error("PFXCA2PEM sem CA deveria retornar .F.")
	}

	// Container PKCS#12 real (binário qualquer) -> .F. nos conversores PFX*.
	realPFX := filepath.Join(dir, "real.pfx")
	if err := os.WriteFile(realPFX, []byte{0x30, 0x82, 0x01, 0x02, 0x03, 0x04}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PFXCERT2PEM", []advplrt.Value{advplrt.NewString(realPFX), advplrt.NewString(outCert), advplrt.NewString("")})
	if advplrt.ToBool(got) {
		t.Error("PFXCERT2PEM com PKCS#12 real deveria retornar .F. (stdlib não suporta)")
	}
	got, _ = segGenericaCall(natives, "PFXKEY2PEM", []advplrt.Value{advplrt.NewString(realPFX), advplrt.NewString(outKey), advplrt.NewString("")})
	if advplrt.ToBool(got) {
		t.Error("PFXKEY2PEM com PKCS#12 real deveria retornar .F. (stdlib não suporta)")
	}

	// Arquivo inexistente -> .F.
	got, _ = segGenericaCall(natives, "PFXCERT2PEM", []advplrt.Value{advplrt.NewString(filepath.Join(dir, "nao.pfx")), advplrt.NewString(outCert), advplrt.NewString("")})
	if advplrt.ToBool(got) {
		t.Error("PFXCERT2PEM de arquivo inexistente deveria retornar .F.")
	}
}

// TestPFXCA2PEMComCA valida extração de CAs quando há 2+ certificados.
func TestPFXCA2PEMComCA(t *testing.T) {
	_, natives := newSegGenericaVM()
	certPEM, _, _ := makeTestCert(t, "Client Cert", 1)
	caPEM, _, _ := makeTestCert(t, "Root CA", 2)
	dir := t.TempDir()
	pfxFile := filepath.Join(dir, "chain.pfx")
	chain := append(append(append([]byte{}, certPEM...), caPEM...), '\n')
	if err := os.WriteFile(pfxFile, chain, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	outCA := filepath.Join(dir, "ca.pem")
	got, _ := segGenericaCall(natives, "PFXCA2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outCA), advplrt.NewString("")})
	if !advplrt.ToBool(got) {
		t.Fatal("PFXCA2PEM com CA deveria retornar .T.")
	}
	out, _ := os.ReadFile(outCA)
	if !strings.Contains(string(out), "BEGIN CERTIFICATE") {
		t.Error("PFXCA2PEM deveria gravar o CA")
	}
}

// TestPFXInfo valida o vetor de informações do PFX (cliente primeiro, depois CAs).
func TestPFXInfo(t *testing.T) {
	_, natives := newSegGenericaVM()
	certPEM, keyPEM, _ := makeTestCert(t, "Client Info", 5)
	caPEM, _, _ := makeTestCert(t, "CA Info", 6)
	dir := t.TempDir()
	pfxFile := filepath.Join(dir, "info.pfx")
	bundle := append(append(append(append([]byte{}, certPEM...), caPEM...), keyPEM...), '\n')
	if err := os.WriteFile(pfxFile, bundle, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := segGenericaCall(natives, "PFXINFO", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString("")})
	if err != nil {
		t.Fatalf("PFXInfo retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 2 {
		t.Fatalf("PFXInfo deveria retornar 2 itens (cliente + CA), veio %v", got.Type())
	}
	client, ok := arr.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(client.Elements) != 5 {
		t.Fatalf("item de PFXInfo deveria ter 5 campos, veio %d", len(client.Elements))
	}
	if !strings.Contains(advplrt.ToString(client.Elements[1]), "Client Info") {
		t.Errorf("destinatário do cliente = %q", advplrt.ToString(client.Elements[1]))
	}
	ca, ok := arr.Elements[1].(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("item do CA deveria ser vetor, veio %v", arr.Elements[1].Type())
	}
	if !strings.Contains(advplrt.ToString(ca.Elements[1]), "CA Info") {
		t.Errorf("destinatário do CA = %q", advplrt.ToString(ca.Elements[1]))
	}

	// Sem nenhum certificado -> vetor com item Nil.
	onlyKey := filepath.Join(dir, "key-only.pfx")
	if err := os.WriteFile(onlyKey, keyPEM, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PFXINFO", []advplrt.Value{advplrt.NewString(onlyKey), advplrt.NewString("")})
	if arr, ok := got.(*advplrt.ArrayValue); !ok || len(arr.Elements) != 1 || !advplrt.IsNil(arr.Elements[0]) {
		t.Errorf("PFXInfo sem certificados deveria retornar vetor com Nil, veio %v", got.Type())
	}

	// Container PKCS#12 real -> Nil.
	realPFX := filepath.Join(dir, "real.pfx")
	if err := os.WriteFile(realPFX, []byte{0x30, 0x82, 0x01, 0x02, 0x03, 0x04}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PFXINFO", []advplrt.Value{advplrt.NewString(realPFX), advplrt.NewString("")})
	if !advplrt.IsNil(got) {
		t.Errorf("PFXInfo com PKCS#12 real deveria retornar Nil, retornou %q", advplrt.ToString(got))
	}
}

// TestPK7Key2PEM valida a conversão PKCS#7 DER -> PEM (bloco PKCS7).
func TestPK7Key2PEM(t *testing.T) {
	_, natives := newSegGenericaVM()
	dir := t.TempDir()
	// DER sintético: sequência ASN.1 mínima (não parseável pela stdlib).
	pk7File := filepath.Join(dir, "test.pk7")
	if err := os.WriteFile(pk7File, []byte{0x30, 0x82, 0x00, 0x00, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x07, 0x02}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	outFile := filepath.Join(dir, "pk7.pem")
	got, err := segGenericaCall(natives, "PK7KEY2PEM", []advplrt.Value{advplrt.NewString(pk7File), advplrt.NewString(outFile)})
	if err != nil {
		t.Fatalf("PK7Key2PEM retornou erro: %v", err)
	}
	if !advplrt.ToBool(got) {
		t.Fatal("PK7Key2PEM deveria retornar .T.")
	}
	out, _ := os.ReadFile(outFile)
	if !strings.Contains(string(out), "BEGIN PKCS7") {
		t.Error("PK7Key2PEM deveria gravar bloco PEM PKCS7")
	}

	// Entrada PEM PKCS7 -> .T. normalizado.
	pem7 := pem.EncodeToMemory(&pem.Block{Type: "PKCS7", Bytes: []byte{0x30, 0x82, 0x00, 0x01}})
	pemFile := filepath.Join(dir, "pk7.pem.in")
	if err := os.WriteFile(pemFile, pem7, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PK7KEY2PEM", []advplrt.Value{advplrt.NewString(pemFile), advplrt.NewString(outFile)})
	if !advplrt.ToBool(got) {
		t.Error("PK7Key2PEM com PEM PKCS7 deveria retornar .T.")
	}

	// Arquivo inexistente -> .F.
	got, _ = segGenericaCall(natives, "PK7KEY2PEM", []advplrt.Value{advplrt.NewString(filepath.Join(dir, "nao.pk7")), advplrt.NewString(outFile)})
	if advplrt.ToBool(got) {
		t.Error("PK7Key2PEM de arquivo inexistente deveria retornar .F.")
	}
}

// TestSegGenericaECKey valida a conversão de chave EC (ECDSA) para PEM.
func TestSegGenericaECKey(t *testing.T) {
	_, natives := newSegGenericaVM()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	dir := t.TempDir()
	pk8File := filepath.Join(dir, "ec.pk8")
	outFile := filepath.Join(dir, "ec.pem")
	if err := os.WriteFile(pk8File, pkcs8DER, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ := segGenericaCall(natives, "PK8KEY2PEM", []advplrt.Value{advplrt.NewString(pk8File), advplrt.NewString(outFile)})
	if !advplrt.ToBool(got) {
		t.Fatal("PK8Key2PEM com chave EC deveria retornar .T.")
	}
	out, _ := os.ReadFile(outFile)
	block, _ := pem.Decode(out)
	if block == nil || block.Type != "PRIVATE KEY" {
		t.Fatalf("saída EC deveria ser bloco PRIVATE KEY, veio %v", blockTypeOf(block))
	}
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		t.Fatalf("ParsePKCS8PrivateKey da saída EC: %v", err)
	}

	// PFXKey2PEM com chave EC (PEM) deve gravar "EC PRIVATE KEY".
	ecPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8DER})
	pfxFile := filepath.Join(dir, "ec.pfx")
	if err := os.WriteFile(pfxFile, ecPEM, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ = segGenericaCall(natives, "PFXKEY2PEM", []advplrt.Value{advplrt.NewString(pfxFile), advplrt.NewString(outFile)})
	if !advplrt.ToBool(got) {
		t.Fatal("PFXKey2PEM com chave EC deveria retornar .T.")
	}
	out, _ = os.ReadFile(outFile)
	if !strings.Contains(string(out), "BEGIN EC PRIVATE KEY") {
		t.Error("PFXKey2PEM com EC deveria gravar EC PRIVATE KEY")
	}
}