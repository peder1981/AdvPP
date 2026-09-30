package rpo

import (
	"crypto/rand"
	"os"
	"strings"
	"testing"
)

// TestAPOScannerRejectsCiphertext prova que o scanner estrito NÃO produz
// candidatos a partir de conteúdo cifrado (fonte dos falsos positivos
// antigos).
func TestAPOScannerRejectsCiphertext(t *testing.T) {
	t.Parallel()

	rpoData, err := os.ReadFile("testdata/live_capture.rpo")
	if err != nil {
		t.Skipf("testdata indisponível: %v", err)
	}
	f, err := Parse(rpoData)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	content := append(append([]byte{}, f.AdminSection...), f.Body...)

	parser := NewAPOParser(content)
	recs := parser.ParseRecords(DefaultAPOMinConfidence)
	if len(recs) != 0 {
		t.Errorf("esperado 0 registros de ciphertext, obtido %d: %+v", len(recs), recs)
	}

	random := make([]byte, 2<<20)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	recs = NewAPOParser(random).ParseRecords(DefaultAPOMinConfidence)
	if len(recs) != 0 {
		t.Errorf("esperado 0 registros de dados aleatórios, obtido %d", len(recs))
	}
}

// TestAPOScannerFindsSynthetic prova que o scanner ENCONTRA um registro
// sintético bem-formado (controle positivo).
func TestAPOScannerFindsSynthetic(t *testing.T) {
	t.Parallel()

	// Cabeçalho: size(4 LE) + type(1) + "MinhaFuncao\0" + nextLen(4 LE)
	name := []byte("MinhaFuncao\x00")
	next := []byte{4, 0, 0, 0}
	payloadLen := 4 + 1 + len(name) + len(next)
	buf := make([]byte, 5+len(name)+len(next)+8)
	buf[0] = byte(payloadLen)
	buf[1] = 0
	buf[2] = 0
	buf[3] = 0
	buf[4] = APO_TYPE_FUNCTION
	copy(buf[5:], name)
	copy(buf[5+len(name):], next)

	recs := NewAPOParser(buf).ParseRecords(DefaultAPOMinConfidence)
	if len(recs) != 1 {
		t.Fatalf("esperado 1 registro sintético, obtido %d", len(recs))
	}
	if recs[0].Name != "MinhaFuncao" {
		t.Errorf("nome: esperado MinhaFuncao, obtido %q", recs[0].Name)
	}
	if recs[0].Type != APO_TYPE_FUNCTION {
		t.Errorf("tipo: esperado Function, obtido %s", recs[0].TypeName)
	}
}

func TestGetTypeName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   uint8
		want string
	}{
		{APO_TYPE_FUNCTION, "Function"},
		{APO_TYPE_METHOD, "Method"},
		{APO_TYPE_CLASS, "Class"},
		{0xFF, "Unknown_0xFF"},
	}
	for _, c := range cases {
		if got := GetTypeName(c.id); got != c.want {
			t.Errorf("GetTypeName(%d)=%q, want %q", c.id, got, c.want)
		}
	}
}

func TestIsValidAdvplName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ok   bool
	}{
		{"GetSx3Cache", true},
		{"U_MyFunction", true},
		{"Invalid Name", false},
		{"123Start", false},
		{"", false},
		{"A", true},
		{"X"+strings.Repeat("a",70), false},
	}
	for _, c := range cases {
		if got := isValidAdvplName(c.name); got != c.ok {
			t.Errorf("isValidAdvplName(%q)=%v, want %v", c.name, got, c.ok)
		}
	}
}
