package vm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newCryptoVM cria uma VM com as natives de Segurança/Criptografia registradas
// manualmente (o registro central em natives.go é feito à parte).
func newCryptoVM() (*VM, map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerSegurancaCriptografiaNatives(natives)
	return v, natives
}

// cryptoCall invoca uma native registrada com os argumentos dados.
func cryptoCall(natives map[string]func(args []advplrt.Value) (advplrt.Value, error), name string, args []advplrt.Value) (advplrt.Value, error) {
	return natives[name](args)
}

func TestMD5(t *testing.T) {
	_, natives := newCryptoVM()
	got, err := cryptoCall(natives, "MD5", []advplrt.Value{advplrt.NewString("abc")})
	if err != nil {
		t.Fatalf("MD5 retornou erro: %v", err)
	}
	if advplrt.ToString(got) != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("MD5(\"abc\") = %q, quer 900150983cd24fb0d6963f7d28e17f72", advplrt.ToString(got))
	}
	// RAW_DIGEST (nType=1): 16 bytes binários
	got, _ = cryptoCall(natives, "MD5", []advplrt.Value{advplrt.NewString("123456"), advplrt.NewNumber(1)})
	if len(advplrt.ToString(got)) != 16 {
		t.Errorf("MD5 raw de \"123456\" deveria ter 16 bytes, tem %d", len(advplrt.ToString(got)))
	}
}

func TestMD5File(t *testing.T) {
	_, natives := newCryptoVM()
	got, err := cryptoCall(natives, "MD5FILE", []advplrt.Value{advplrt.NewString("/arquivo/que/nao/existe.txt")})
	if err != nil {
		t.Fatalf("MD5File retornou erro: %v", err)
	}
	if advplrt.ToString(got) != "" {
		t.Errorf("MD5File de arquivo inexistente = %q, quer \"\"", advplrt.ToString(got))
	}
}

func TestSHA1(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "SHA1", []advplrt.Value{advplrt.NewString("abc")})
	if advplrt.ToString(got) != "a9993e364706816aba3e25717850c26c9cd0d89d" {
		t.Errorf("SHA1(\"abc\") = %q", advplrt.ToString(got))
	}
}

func TestSHA256(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "SHA256", []advplrt.Value{advplrt.NewString("abc")})
	if advplrt.ToString(got) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("SHA256(\"abc\") = %q", advplrt.ToString(got))
	}
}

func TestSHA384(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "SHA384", []advplrt.Value{advplrt.NewString("abc")})
	if advplrt.ToString(got) != "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7" {
		t.Errorf("SHA384(\"abc\") = %q", advplrt.ToString(got))
	}
}

func TestSHA512(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "SHA512", []advplrt.Value{advplrt.NewString("abc")})
	if advplrt.ToString(got) != "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f" {
		t.Errorf("SHA512(\"abc\") = %q", advplrt.ToString(got))
	}
}

func TestEVPDigest(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "EVPDIGEST", []advplrt.Value{advplrt.NewString("abc"), advplrt.NewNumber(1)})
	if hex.EncodeToString([]byte(advplrt.ToString(got))) != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("EVPDigest(\"abc\",1) = %q", advplrt.ToString(got))
	}
	got, _ = cryptoCall(natives, "EVPDIGEST", []advplrt.Value{advplrt.NewString("abc"), advplrt.NewNumber(5)})
	if hex.EncodeToString([]byte(advplrt.ToString(got))) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("EVPDigest(\"abc\",5) = %q", advplrt.ToString(got))
	}
}

func TestHMAC(t *testing.T) {
	_, natives := newCryptoVM()
	// HMAC-SHA256 hex: HMAC("abc","key",5,2,1,1)
	got, _ := cryptoCall(natives, "HMAC", []advplrt.Value{
		advplrt.NewString("abc"), advplrt.NewString("key"),
		advplrt.NewNumber(5), advplrt.NewNumber(2), advplrt.NewNumber(1), advplrt.NewNumber(1),
	})
	if advplrt.ToString(got) != "9c196e32dc0175f86f4b1cb89289d6619de6bee699e4c378e68309ed97a1a6ab" {
		t.Errorf("HMAC-SHA256 = %q", advplrt.ToString(got))
	}
	// HMAC-MD5 hex
	got, _ = cryptoCall(natives, "HMAC", []advplrt.Value{
		advplrt.NewString("abc"), advplrt.NewString("key"),
		advplrt.NewNumber(1), advplrt.NewNumber(2), advplrt.NewNumber(1), advplrt.NewNumber(1),
	})
	if advplrt.ToString(got) != "d2fe98063f876b03193afb49b4979591" {
		t.Errorf("HMAC-MD5 = %q", advplrt.ToString(got))
	}
	// HMAC-SHA1 raw (nRetType=1) sobre "secret text"/"d key used"
	got, _ = cryptoCall(natives, "HMAC", []advplrt.Value{
		advplrt.NewString("secret text"), advplrt.NewString("d key used"),
		advplrt.NewNumber(3), advplrt.NewNumber(1), advplrt.NewNumber(1), advplrt.NewNumber(1),
	})
	if len(advplrt.ToString(got)) != 20 {
		t.Errorf("HMAC-SHA1 raw deveria ter 20 bytes, tem %d", len(advplrt.ToString(got)))
	}
	// HMAC-SHA512 hex
	got, _ = cryptoCall(natives, "HMAC", []advplrt.Value{
		advplrt.NewString("abc"), advplrt.NewString("key"),
		advplrt.NewNumber(7), advplrt.NewNumber(2), advplrt.NewNumber(1), advplrt.NewNumber(1),
	})
	if advplrt.ToString(got) != "3926a207c8c42b0c41792cbd3e1a1aaaf5f7a25704f62dfc939c4987dd7ce060009c5bb1c2447355b3216f10b537e9afa7b64a4e5391b0d631172d07939e087a" {
		t.Errorf("HMAC-SHA512 = %q", advplrt.ToString(got))
	}
}

func TestCRCCalc(t *testing.T) {
	_, natives := newCryptoVM()
	// CRC16_CCITT_KERMIT (8) de "123456789" = 0x2189 = 8585
	got, _ := cryptoCall(natives, "CRCCALC", []advplrt.Value{advplrt.NewNumber(8), advplrt.NewString("123456789")})
	if advplrt.ToFloat(got) != 0x2189 {
		t.Errorf("CRCCalc(8,\"123456789\") = %v, quer 8585 (0x2189)", advplrt.ToFloat(got))
	}
	// CRC16_MODBUS (3) de "123456789" = 0x4B37 = 19255
	got, _ = cryptoCall(natives, "CRCCALC", []advplrt.Value{advplrt.NewNumber(3), advplrt.NewString("123456789")})
	if advplrt.ToFloat(got) != 0x4B37 {
		t.Errorf("CRCCalc(3,\"123456789\") = %v, quer 19255 (0x4B37)", advplrt.ToFloat(got))
	}
}

func TestAESEncryptDecrypt(t *testing.T) {
	_, natives := newCryptoVM()
	const plain = "mensagem de teste AES-128 CBC"
	key := "chave12345678901" // 16 bytes
	iv := "iv12345678901234"  // 16 bytes

	enc, err := cryptoCall(natives, "AESENCRYPT", []advplrt.Value{
		advplrt.NewNumber(0), advplrt.NewString(plain),
		advplrt.Nil, advplrt.NewString(key), advplrt.NewString(iv),
	})
	if err != nil {
		t.Fatalf("AESEncrypt erro: %v", err)
	}
	encArr, ok := enc.(*advplrt.ArrayValue)
	if !ok || len(encArr.Elements) != 4 {
		t.Fatalf("AESEncrypt deveria retornar array de 4 elementos, retornou %v", enc)
	}
	if advplrt.ToFloat(encArr.Elements[0]) != 0 {
		t.Fatalf("AESEncrypt retornou código de erro %v", advplrt.ToFloat(encArr.Elements[0]))
	}
	cipherText := advplrt.ToString(encArr.Elements[1])
	if cipherText == "" {
		t.Fatal("AESEncrypt retornou texto cifrado vazio")
	}

	dec, _ := cryptoCall(natives, "AESDECRYPT", []advplrt.Value{
		advplrt.NewNumber(0), advplrt.NewString(cipherText),
		advplrt.NewString(key), advplrt.NewString(iv),
	})
	decArr, ok := dec.(*advplrt.ArrayValue)
	if !ok || len(decArr.Elements) != 2 {
		t.Fatalf("AESDecrypt deveria retornar array de 2 elementos, retornou %v", dec)
	}
	if advplrt.ToFloat(decArr.Elements[0]) != 0 {
		t.Fatalf("AESDecrypt retornou código de erro %v", advplrt.ToFloat(decArr.Elements[0]))
	}
	if advplrt.ToString(decArr.Elements[1]) != plain {
		t.Errorf("AES roundtrip falhou: %q != %q", advplrt.ToString(decArr.Elements[1]), plain)
	}

	// Chave errada -> código de erro
	dec, _ = cryptoCall(natives, "AESDECRYPT", []advplrt.Value{
		advplrt.NewNumber(0), advplrt.NewString(cipherText),
		advplrt.NewString("0123456789abcdef"), advplrt.NewString(iv),
	})
	decArr, _ = dec.(*advplrt.ArrayValue)
	if advplrt.ToFloat(decArr.Elements[0]) == 0 {
		t.Error("AESDecrypt com chave errada deveria retornar código de erro != 0")
	}
}

func TestAESPasswordRoundtrip(t *testing.T) {
	_, natives := newCryptoVM()
	const plain = "senha secreta"
	enc, _ := cryptoCall(natives, "AESENCRYPT", []advplrt.Value{
		advplrt.NewNumber(2), advplrt.NewString(plain), advplrt.NewString("minha-senha"),
	})
	encArr := enc.(*advplrt.ArrayValue)
	if advplrt.ToFloat(encArr.Elements[0]) != 0 {
		t.Fatalf("AESEncrypt(password) erro %v", advplrt.ToFloat(encArr.Elements[0]))
	}
	keyUsed := advplrt.ToString(encArr.Elements[2])
	ivUsed := advplrt.ToString(encArr.Elements[3])
	dec, _ := cryptoCall(natives, "AESDECRYPT", []advplrt.Value{
		advplrt.NewNumber(2), encArr.Elements[1], advplrt.NewString(keyUsed), advplrt.NewString(ivUsed),
	})
	decArr := dec.(*advplrt.ArrayValue)
	if advplrt.ToFloat(decArr.Elements[0]) != 0 || advplrt.ToString(decArr.Elements[1]) != plain {
		t.Errorf("AES password roundtrip falhou: %v %q", advplrt.ToFloat(decArr.Elements[0]), advplrt.ToString(decArr.Elements[1]))
	}
}

func TestRC4Crypt(t *testing.T) {
	_, natives := newCryptoVM()
	// Vetor conhecido do TDN: rc4crypt("abcde","123456789",.T.) = "55AB394524"
	got, _ := cryptoCall(natives, "RC4CRYPT", []advplrt.Value{
		advplrt.NewString("abcde"), advplrt.NewString("123456789"), advplrt.NewBool(true),
	})
	if advplrt.ToString(got) != "55AB394524" {
		t.Errorf("RC4Crypt(\"abcde\",\"123456789\",.T.) = %q, quer 55AB394524", advplrt.ToString(got))
	}
	// Roundtrip em texto plano
	enc, _ := cryptoCall(natives, "RC4CRYPT", []advplrt.Value{
		advplrt.NewString("texto original"), advplrt.NewString("chave"), advplrt.NewBool(false),
	})
	dec, _ := cryptoCall(natives, "RC4CRYPT", []advplrt.Value{
		enc, advplrt.NewString("chave"), advplrt.NewBool(false),
	})
	if advplrt.ToString(dec) != "texto original" {
		t.Errorf("RC4 roundtrip falhou: %q", advplrt.ToString(dec))
	}
	// Roundtrip com entrada ASCII hex (segue exemplo do TDN: .F. retorno texto,
	// .T. entrada hex)
	encHex, _ := cryptoCall(natives, "RC4CRYPT", []advplrt.Value{
		advplrt.NewString("abcde"), advplrt.NewString("123456789"), advplrt.NewBool(true),
	})
	decHex, _ := cryptoCall(natives, "RC4CRYPT", []advplrt.Value{
		encHex, advplrt.NewString("123456789"), advplrt.NewBool(false), advplrt.NewBool(true),
	})
	if advplrt.ToString(decHex) != "abcde" {
		t.Errorf("RC4 roundtrip (input hex) falhou: %q", advplrt.ToString(decHex))
	}
}

func TestArc4(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "ARC4", []advplrt.Value{advplrt.NewString("abcde"), advplrt.NewString("123456789")})
	if advplrt.ToString(got) != "55-AB-39-45-24" {
		t.Errorf("ARC4(\"abcde\",\"123456789\") = %q, quer 55-AB-39-45-24", advplrt.ToString(got))
	}
}

func TestWebEncript(t *testing.T) {
	_, natives := newCryptoVM()
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	enc, _ := cryptoCall(natives, "WEBENCRIPT", []advplrt.Value{advplrt.NewString(content), advplrt.NewBool(false)})
	if advplrt.ToString(enc) == "" {
		t.Fatal("WebEncript retornou vazio")
	}
	dec, _ := cryptoCall(natives, "WEBENCRIPT", []advplrt.Value{enc, advplrt.NewBool(true)})
	if advplrt.ToString(dec) != content {
		t.Errorf("WebEncript roundtrip falhou: %q != %q", advplrt.ToString(dec), content)
	}
}

func TestEmbaralha(t *testing.T) {
	_, natives := newCryptoVM()
	text := "TOTVS APPSERVER"
	scrambled, _ := cryptoCall(natives, "EMBARALHA", []advplrt.Value{advplrt.NewString(text), advplrt.NewNumber(0)})
	unscrambled, _ := cryptoCall(natives, "EMBARALHA", []advplrt.Value{scrambled, advplrt.NewNumber(1)})
	if advplrt.ToString(unscrambled) != text {
		t.Errorf("Embaralha roundtrip falhou: %q != %q", advplrt.ToString(unscrambled), text)
	}
	// A permutação preserva o multiset de caracteres (mesmas letras).
	if len(advplrt.ToString(scrambled)) != len(text) || !sameChars(advplrt.ToString(scrambled), text) {
		t.Errorf("Embaralha deveria ser uma permutação de caracteres: %q", advplrt.ToString(scrambled))
	}
}

func TestHSMStubs(t *testing.T) {
	_, natives := newCryptoVM()
	init, _ := cryptoCall(natives, "HSMINITIALIZE", nil)
	if advplrt.ToBool(init) {
		t.Error("HSMInitialize deveria retornar .F. sem dispositivo HSM")
	}
	slots, _ := cryptoCall(natives, "HSMSLOTLIST", nil)
	if arr, ok := slots.(*advplrt.ArrayValue); !ok || len(arr.Elements) != 0 {
		t.Errorf("HSMSlotList deveria retornar {} , retornou %v", slots)
	}
	mod, _ := cryptoCall(natives, "HSMMODULUS", []advplrt.Value{advplrt.NewString("slot_1-label_X")})
	if advplrt.ToString(mod) != "" {
		t.Errorf("HSMModulus deveria retornar \"\" sem HSM, retornou %q", advplrt.ToString(mod))
	}
}

func TestHTTPSSLClient(t *testing.T) {
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "HTTPSSLCLIENT", []advplrt.Value{
		advplrt.NewNumber(0), advplrt.NewNumber(0), advplrt.NewNumber(1),
		advplrt.NewString(""), advplrt.NewString(""), advplrt.NewString(""),
	})
	if advplrt.ToBool(got) {
		t.Error("HTTPSSLClient deveria retornar .F. sem infraestrutura SSL")
	}
}

func TestRSAExponentModulus(t *testing.T) {
	// Gera um par de chaves RSA em memória e valida extração de módulo/expoente.
	key := testRSAPrivatePEM(t)
	_, natives := newCryptoVM()
	pub, _ := cryptoCall(natives, "RSAMODULUS", []advplrt.Value{advplrt.NewString(key), advplrt.NewBool(true)})
	exp, _ := cryptoCall(natives, "RSAEXPONENT", []advplrt.Value{advplrt.NewString(key), advplrt.NewBool(true)})
	if advplrt.IsNil(pub) || advplrt.IsNil(exp) {
		t.Fatal("RSAModulus/RSAExponent deveriam retornar valores para chave válida")
	}
	// Expoente público padrão 65537 -> bytes 01 00 01.
	if strings.ToUpper(hex.EncodeToString([]byte(advplrt.ToString(exp)))) != "010001" {
		t.Errorf("RSAExponent = %x, quer 010001 (65537)", []byte(advplrt.ToString(exp)))
	}
	// Módulo deve ter o tamanho esperado da chave (1024 bits = 128 bytes).
	if len([]byte(advplrt.ToString(pub))) != 128 {
		t.Errorf("RSAModulus deveria ter 128 bytes para chave de 1024 bits, tem %d", len([]byte(advplrt.ToString(pub))))
	}
}

func TestDecryptRSA(t *testing.T) {
	// Encripta com a chave pública (PKCS1v15) e decripta via DecryptRSA.
	key := testRSAPrivatePEM(t)
	priv, err := parseRSAPrivateKey([]byte(key), "")
	if err != nil {
		t.Fatalf("parseRSAPrivateKey: %v", err)
	}
	ciphertext, err := rsaEncryptTest(&priv.PublicKey, []byte("PASSWORD"))
	if err != nil {
		t.Fatalf("rsa.EncryptPKCS1v15: %v", err)
	}
	path := writeTempPEM(t, []byte(key))
	_, natives := newCryptoVM()
	got, _ := cryptoCall(natives, "DECRYPTRSA", []advplrt.Value{advplrt.NewString(path), advplrt.NewString(string(ciphertext))})
	if advplrt.ToString(got) != "PASSWORD" {
		t.Errorf("DecryptRSA = %q, quer PASSWORD", advplrt.ToString(got))
	}
}

func TestPrivSignVerifyRSA(t *testing.T) {
	privPEM := testRSAPrivatePEM(t)
	priv, _ := parseRSAPrivateKey([]byte(privPEM), "")
	pubPEM := pemEncodeTest("PUBLIC KEY", func() []byte {
		der, _ := x509MarshalPKIXPublicKey(&priv.PublicKey)
		return der
	}())

	_, natives := newCryptoVM()
	// Assina o hash MD5 hex de um conteúdo (como no exemplo do TDN).
	hashMD5, _ := cryptoCall(natives, "MD5", []advplrt.Value{advplrt.NewString("01234567890123456789")})
	sign, _ := cryptoCall(natives, "PRIVSIGNRSA", []advplrt.Value{
		advplrt.NewString(privPEM), hashMD5, advplrt.NewNumber(1), advplrt.NewString("senha"),
	})
	if advplrt.IsNil(sign) || advplrt.ToString(sign) == "" {
		t.Fatal("PrivSignRSA retornou Nil/vazio para chave válida")
	}
	ok, _ := cryptoCall(natives, "PRIVVERYRSA", []advplrt.Value{
		advplrt.NewString(pubPEM), hashMD5, advplrt.NewNumber(1), sign,
	})
	if !advplrt.ToBool(ok) {
		t.Error("PrivVeryRSA deveria validar a assinatura gerada")
	}
	// Assinatura alterada deve falhar
	badSig := strings.Repeat("0", len(advplrt.ToString(sign)))
	ok, _ = cryptoCall(natives, "PRIVVERYRSA", []advplrt.Value{
		advplrt.NewString(pubPEM), hashMD5, advplrt.NewNumber(1), advplrt.NewString(badSig),
	})
	if advplrt.ToBool(ok) {
		t.Error("PrivVeryRSA validou assinatura adulterada")
	}
}

func TestWriteRSAPK(t *testing.T) {
	priv, err := rsaGenerateKey(1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	der := x509MarshalPKCS1PrivateKey(priv)
	derPath := writeTempFile(t, der)
	pemPath := writeTempFile(t, nil)
	_, natives := newCryptoVM()
	ok, _ := cryptoCall(natives, "WRITERSAPK", []advplrt.Value{advplrt.NewString(derPath), advplrt.NewString(pemPath)})
	if !advplrt.ToBool(ok) {
		t.Fatal("WriteRSAPK deveria retornar .T. para DER válido")
	}
	data, err := readTempFile(pemPath)
	if err != nil || !strings.Contains(string(data), "BEGIN RSA PRIVATE KEY") {
		t.Errorf("WriteRSAPK não gravou PEM válido: %v %q", err, string(data))
	}
}

// sameChars verifica se duas strings têm exatamente o mesmo multiset de bytes.
func sameChars(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[byte]int)
	for i := 0; i < len(a); i++ {
		counts[a[i]]++
	}
	for i := 0; i < len(b); i++ {
		counts[b[i]]--
		if counts[b[i]] < 0 {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------------
// Helpers de teste para RSA (stdlib apenas)
// ----------------------------------------------------------------------------

// rsaGenerateKey gera uma chave RSA de 1024 bits para os testes.
func rsaGenerateKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// testRSAPrivatePEM gera uma chave RSA privada e a serializa em PEM PKCS#1.
func testRSAPrivatePEM(t *testing.T) string {
	t.Helper()
	priv, err := rsaGenerateKey(1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return pemEncodeTest("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))
}

// x509MarshalPKCS1PrivateKey é um alias direto de x509.MarshalPKCS1PrivateKey.
func x509MarshalPKCS1PrivateKey(key *rsa.PrivateKey) []byte {
	return x509.MarshalPKCS1PrivateKey(key)
}

// x509MarshalPKIXPublicKey serializa a chave pública em formato PKIX DER.
func x509MarshalPKIXPublicKey(pub *rsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pub)
}

// rsaEncryptTest cifra dados com a chave pública usando PKCS#1 v1.5.
func rsaEncryptTest(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, pub, data)
}

// pemEncodeTest envolve bytes DER em um bloco PEM.
func pemEncodeTest(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}

// writeTempPEM grava bytes PEM em um arquivo temporário e retorna o path.
func writeTempPEM(t *testing.T, content []byte) string {
	t.Helper()
	return writeTempFile(t, content)
}

// writeTempFile grava bytes em um arquivo temporário e retorna o path.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "advpp-test-*")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	defer f.Close()
	if content != nil {
		if _, err := f.Write(content); err != nil {
			t.Fatalf("f.Write: %v", err)
		}
	}
	return f.Name()
}

// readTempFile lê o conteúdo de um arquivo.
func readTempFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("readTempFile: " + err.Error())
	}
	return data, nil
}
