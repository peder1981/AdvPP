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

func TestExtractStrings_ExtXdef(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "EXTXDEF.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	if len(blob.Strings) != 1 || blob.Strings[0].Text != "EXTXDEF.PRW" {
		t.Fatalf("Strings = %+v, esperado único [EXTXDEF.PRW]", blob.Strings)
	}
	if blob.FileName != "EXTXDEF.PRW" {
		t.Errorf("FileName = %q, esperado %q", blob.FileName, "EXTXDEF.PRW")
	}
}

func TestExtractStrings_AbsloggerTemTabelaDeNomes(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "ABSLOGGER.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	got := map[string]bool{}
	for _, s := range blob.Strings {
		got[s.Text] = true
	}
	for _, want := range []string{"ABSLOGGER", "CPROCNAME", "LFFACTIVE", "USELF", "ABSLOGGER.PRW"} {
		if !got[want] {
			t.Errorf("string %q não encontrada em %+v", want, blob.Strings)
		}
	}
	if blob.FileName != "ABSLOGGER.PRW" {
		t.Errorf("FileName = %q, esperado ABSLOGGER.PRW", blob.FileName)
	}
}

func TestExtractStrings_TLPPFilename(t *testing.T) {
	blob, err := ParseApoBlob(loadFixture(t, "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP"))
	if err != nil {
		t.Fatal(err)
	}
	blob.ExtractStrings()
	if blob.FileName != "BACKOFFICE.SV.EST.SOLDPRODUCTS.BRA.TLPP" {
		t.Errorf("FileName = %q", blob.FileName)
	}
}
