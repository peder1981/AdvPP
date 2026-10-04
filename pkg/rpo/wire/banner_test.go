package wire

import (
	"bytes"
	"testing"
)

func TestBuildBannerLayoutCapturado(t *testing.T) {
	b, err := BuildBanner("u1", "h1", "20210324103317")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 134 {
		t.Fatalf("tamanho=%d, esperado 134", len(b))
	}
	if !bytes.Equal(b[0:15], []byte("--ADVANCEDPR--\x00")) {
		t.Fatalf("magic errado: %q", b[0:15])
	}
	if b[15] != 0x03 {
		t.Fatalf("flag=%#x, esperado 0x03", b[15])
	}
	if string(bytes.TrimRight(b[16:82], "\x00")) != "u1" {
		t.Fatalf("user field errado: %q", b[16:82])
	}
	if string(bytes.TrimRight(b[82:115], "\x00")) != "h1" {
		t.Fatalf("host field errado: %q", b[82:115])
	}
	if string(bytes.TrimRight(b[115:130], "\x00")) != "20210324103317" {
		t.Fatalf("build field errado: %q", b[115:130])
	}
	if !bytes.Equal(b[130:134], []byte{0x00, 0x00, 0x05, 0x01}) {
		t.Fatalf("tail errado: %x", b[130:134])
	}
}

func TestBuildBannerCamposGrandes(t *testing.T) {
	if _, err := BuildBanner(stringsX(66), "h", "b"); err == nil {
		t.Fatal("user de 66 bytes deveria falhar (cabeça 65 + NUL)")
	}
	if _, err := BuildBanner("u", stringsX(33), "b"); err == nil {
		t.Fatal("host de 33 bytes deveria falhar")
	}
	if _, err := BuildBanner("u", "h", stringsX(15)); err == nil {
		t.Fatal("build de 15 bytes deveria falhar")
	}
}

// stringsX devolve n bytes 'x' (helper de teste).
func stringsX(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
