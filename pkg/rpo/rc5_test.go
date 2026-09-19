package rpo

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

// TestRC5_RoundTrip é uma verificação de AUTOCONSISTÊNCIA (encrypt então
// decrypt reproduz o original) — não há biblioteca de referência
// independente disponível neste ambiente para RC5 (nem OpenSSL nem
// pycryptodome o implementam). Ver aviso em rc5.go.
func TestRC5_RoundTrip(t *testing.T) {
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	c, err := NewRC5Cipher(key)
	if err != nil {
		t.Fatal(err)
	}

	for trial := 0; trial < 200; trial++ {
		pt := make([]byte, 8)
		rand.Read(pt)
		ct := make([]byte, 8)
		c.Encrypt(ct, pt)
		back := make([]byte, 8)
		c.Decrypt(back, ct)
		if !bytes.Equal(back, pt) {
			t.Fatalf("round-trip falhou: pt=%x ct=%x back=%x", pt, ct, back)
		}
	}
}

func TestRC5_OFB_RoundTrip(t *testing.T) {
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	iv, _ := hex.DecodeString("0102030405060708")
	c, err := NewRC5Cipher(key)
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte("mensagem de teste RC5 com tamanho arbitrario, nao multiplo de 8")
	ct := make([]byte, len(pt))
	cipher.NewOFB(c, iv).XORKeyStream(ct, pt)
	back := make([]byte, len(pt))
	cipher.NewOFB(c, iv).XORKeyStream(back, ct)
	if !bytes.Equal(back, pt) {
		t.Fatalf("OFB round-trip falhou")
	}
	if bytes.Equal(ct, pt) {
		t.Fatalf("ciphertext identico ao plaintext — cifra nao fez nada")
	}
}

// TestRC5_OFB_RealCapture usa dados capturados AO VIVO (gdb, sessão de
// compilação Protheus real, container protheus-compile, 2026-09-19) —
// não são vetores inventados. O plaintext e a chave/IV vieram de
// tCryptoEVP::Encrypt/EVP_EncryptInit_ex; o ciphertext esperado é o
// conteúdo REAL do admin_section.bin do custom.rpo gerado por aquela
// compilação (ver docs/rpo-format-sonnet.md, verificação RC5).
func TestRC5_OFB_RealCapture(t *testing.T) {
	key, _ := hex.DecodeString("d34f361958b31d500c21ce08ff3da723")
	iv, _ := hex.DecodeString("0ee3e10597b267d5")

	cases := []struct {
		plaintext string
		wantCT    string
	}{
		{"00000000", "13ac4f66"},
		{"0100000052504f5243355f545249474745522e50525700000000000000000000", "12ac4f66fbbfb32adc36763257b2dbc3fdce6de5009e877aa742ea0aca615217"},
	}

	c, err := NewRC5Cipher(key)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		pt, _ := hex.DecodeString(tc.plaintext)
		ct := make([]byte, len(pt))
		cipher.NewOFB(c, iv).XORKeyStream(ct, pt)
		if hex.EncodeToString(ct) != tc.wantCT {
			t.Errorf("case %d: RC5-OFB = %x, want %s", i, ct, tc.wantCT)
		}
	}
}

// TestRC5_KeyScheduleDeterministic garante que a mesma chave sempre
// produz o mesmo resultado (sem estado global vazando entre instâncias).
func TestRC5_KeyScheduleDeterministic(t *testing.T) {
	key, _ := hex.DecodeString("2d588ec72afc2109f00a5a3f6b6497af")
	pt, _ := hex.DecodeString("0011223344556677")

	c1, _ := NewRC5Cipher(key)
	c2, _ := NewRC5Cipher(key)
	ct1 := make([]byte, 8)
	ct2 := make([]byte, 8)
	c1.Encrypt(ct1, pt)
	c2.Encrypt(ct2, pt)
	if !bytes.Equal(ct1, ct2) {
		t.Fatalf("mesma chave produziu ciphertexts diferentes: %x vs %x", ct1, ct2)
	}
}
