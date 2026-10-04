package rpo

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "apo", name))
	if err != nil {
		t.Fatalf("lendo fixture %s: %v", name, err)
	}
	return data
}

func TestParseApoBlob_MagicEKindAdvPL(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "EXTXDEF.PRW"))
	if err != nil {
		t.Fatalf("ParseApoBlob: %v", err)
	}
	if blob.Kind != ApoKindAdvPL {
		t.Errorf("Kind = %q, esperado %q", blob.Kind, ApoKindAdvPL)
	}
	if blob.Size != 39 {
		t.Errorf("Size = %d, esperado 39", blob.Size)
	}
}

func TestParseApoBlob_KindTLPP(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP"))
	if err != nil {
		t.Fatalf("ParseApoBlob: %v", err)
	}
	if blob.Kind != ApoKindTLPP {
		t.Errorf("Kind = %q, esperado %q", blob.Kind, ApoKindTLPP)
	}
}

func TestParseApoBlob_MagicInvalida(t *testing.T) {
	if _, err := ParseApoBlob([]byte{0x01, 0x02, 0x03}); err == nil {
		t.Error("esperava erro para magic inválida")
	}
}

func TestParseApoBlob_KindInvalido(t *testing.T) {
	data := []byte{0x75, 0x00, 0x00, 0x46, 0x46, 'X'}
	if _, err := ParseApoBlob(data); err == nil {
		t.Error("esperava erro para kind != F/T")
	}
}
