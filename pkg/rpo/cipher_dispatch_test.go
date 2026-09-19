package rpo

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

// TestCipherDispatch_RealCapture é o teste central deste pacote: usa
// uma captura ao vivo REAL (testdata/live_capture.json — chave, IV,
// cipher e plaintext de tCryptoEVP::Encrypt/EVP_EncryptInit_ex numa
// compilação Protheus real, 2026-09-19) e o RPO resultante
// (testdata/live_capture.rpo) para verificar que EncryptSegment
// reproduz byte a byte o conteúdo real gravado no arquivo, e que
// DecryptSegment reverte corretamente. Não são vetores inventados —
// são exatamente os dados que produziram o RPO commitado.
func TestCipherDispatch_RealCapture(t *testing.T) {
	events, err := LoadCaptureEvents("testdata/live_capture.json")
	if err != nil {
		t.Fatal(err)
	}
	segments := MergeCaptureSegments(events)

	rpoData, err := os.ReadFile("testdata/live_capture.rpo")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(rpoData)
	if err != nil {
		t.Fatalf("Parse(testdata/live_capture.rpo): %v", err)
	}

	tested := 0
	for _, m := range segments {
		if m.Cipher == "" || m.Key == "" || m.Plaintext == "" {
			continue
		}
		base, _, err := ParseCipherName(m.Cipher)
		if err != nil || base == "idea" {
			continue // nome desconhecido, ou IDEA (sem implementação verificada — ver idea.go)
		}
		key, err := hex.DecodeString(m.Key)
		if err != nil {
			t.Fatalf("#%d: chave hex inválida: %v", m.N, err)
		}
		var iv []byte
		if m.IV != "" {
			iv, err = hex.DecodeString(m.IV)
			if err != nil {
				t.Fatalf("#%d: iv hex inválido: %v", m.N, err)
			}
		}
		plain, err := hex.DecodeString(m.Plaintext)
		if err != nil {
			t.Fatalf("#%d: plaintext hex inválido: %v", m.N, err)
		}

		ct, err := EncryptSegment(m.Cipher, key, iv, plain)
		if err != nil {
			t.Errorf("#%d %s: EncryptSegment falhou: %v", m.N, m.Cipher, err)
			continue
		}
		if !bytes.Contains(f.Body, ct) && !bytes.Contains(f.AdminSection, ct) {
			t.Errorf("#%d %s: ciphertext calculado não aparece no RPO real (len=%d)", m.N, m.Cipher, len(plain))
			continue
		}

		back, err := DecryptSegment(m.Cipher, key, iv, ct)
		if err != nil {
			t.Errorf("#%d %s: DecryptSegment falhou: %v", m.N, m.Cipher, err)
			continue
		}
		if !bytes.Equal(back, plain) {
			t.Errorf("#%d %s: DecryptSegment não reverteu corretamente (got %x, want %x)", m.N, m.Cipher, back, plain)
			continue
		}
		tested++
	}

	if tested == 0 {
		t.Fatal("nenhum segmento com cipher suportado foi testado — capture.json ou dispatcher quebrados")
	}
	t.Logf("%d segmentos verificados contra o RPO real (chave/iv/plaintext capturados ao vivo)", tested)
}

func TestParseCipherName(t *testing.T) {
	cases := []struct {
		in       string
		wantBase string
		wantMode cipherMode
		wantErr  bool
	}{
		{"des_cbc_cipher", "des", modeCBC, false},
		{"des_ede_cfb64_cipher", "des_ede", modeCFB, false},
		{"rc5_32_12_16_ofb_cipher", "rc5_32_12_16", modeOFB, false},
		{"cast5_ecb_cipher", "cast5", modeECB, false},
		{"rc4_cipher", "rc4", modeStream, false},
		{"algo_desconhecido_xyz_cipher", "", 0, true},
	}
	for _, tc := range cases {
		base, mode, err := ParseCipherName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseCipherName(%q): esperava erro", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCipherName(%q): erro inesperado: %v", tc.in, err)
			continue
		}
		if base != tc.wantBase || mode != tc.wantMode {
			t.Errorf("ParseCipherName(%q) = (%q, %v), want (%q, %v)", tc.in, base, mode, tc.wantBase, tc.wantMode)
		}
	}
}
