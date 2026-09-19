package rpo

import (
	"crypto/cipher"
	"encoding/hex"
	"testing"
)

// Vetores gerados com pycryptodome (Crypto.Cipher.ARC2, biblioteca de
// terceiros independente, versão 3.23.0) em 2026-09-19, não fabricados:
//
//	key := bytes.fromhex("000102030405060708090a0b0c0d0e0f")
//	c := ARC2.new(key, ARC2.MODE_ECB, effective_keylen=128)
//	c.encrypt(bytes.fromhex("0011223344556677"))  -> "5ab3337c2c72b69f"
func TestRC2_ECB_KnownAnswer(t *testing.T) {
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	pt, _ := hex.DecodeString("0011223344556677")
	want, _ := hex.DecodeString("5ab3337c2c72b69f")

	c, err := NewRC2Cipher(key)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 8)
	c.Encrypt(got, pt)
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("RC2 ECB encrypt = %x, want %x", got, want)
	}

	back := make([]byte, 8)
	c.Decrypt(back, got)
	if hex.EncodeToString(back) != hex.EncodeToString(pt) {
		t.Fatalf("RC2 ECB decrypt = %x, want %x", back, pt)
	}
}

// Vetor OFB, mesma referência:
//
//	iv = bytes.fromhex("0102030405060708")
//	c := ARC2.new(key, ARC2.MODE_OFB, iv, effective_keylen=128)
//	c.encrypt(b"Hello, RC2 world!!") -> "437f599bc0f8ed2371fa6768c1be020da8d1"
func TestRC2_OFB_KnownAnswer(t *testing.T) {
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	iv, _ := hex.DecodeString("0102030405060708")
	pt := []byte("Hello, RC2 world!!")
	want, _ := hex.DecodeString("437f599bc0f8ed2371fa6768c1be020da8d1")

	c, err := NewRC2Cipher(key)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(pt))
	cipher.NewOFB(c, iv).XORKeyStream(got, pt)
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("RC2 OFB encrypt = %x, want %x", got, want)
	}
}
