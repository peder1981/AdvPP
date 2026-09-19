package rpo

import (
	"fmt"
	"math/bits"
)

// RC5-32/12/16 (Rivest, 1994) — implementado a partir da especificação
// pública original (word size w=32, rounds r=12, key bytes b=16), que é
// exatamente a variante confirmada via desmontagem real de
// tCryptoEVP::Encrypt (docs/rpo-format-sonnet.md, cipher
// "rc5_32_12_16_*"). Um dos ~12 algoritmos legados da tabela rotativa
// usada pelo RPO do Protheus.
//
// Diferente de RC2 (rc2.go), não há biblioteca de referência disponível
// neste ambiente (nem OpenSSL, nem pycryptodome implementam RC5) para
// cross-validar contra uma segunda implementação. Em vez disso, a
// implementação abaixo foi validada contra dados REAIS: capturei ao
// vivo (gdb, tCryptoEVP::Encrypt + EVP_EncryptInit_ex) uma sessão de
// compilação Protheus que selecionou "rc5_32_12_16_ofb_cipher" para dois
// registros (4 e 33 bytes), cifrei o plaintext capturado com esta
// implementação usando a chave/IV capturados, e o resultado bateu byte
// a byte com o conteúdo real do `admin_section.bin` do RPO gerado (ver
// docs/rpo-format-sonnet.md). Como o modo OFB gera seu keystream
// chamando diretamente Encrypt() (o mesmo núcleo usado por CBC/ECB),
// essa validação cobre a função de bloco em si, não só o modo OFB.

const (
	rc5P32 uint32 = 0xb7e15163
	rc5Q32 uint32 = 0x9e3779b9
	rc5W   = 32 // word size em bits
	rc5R   = 12 // rounds
)

// RC5Cipher implementa cipher.Block (BlockSize=8, 2 palavras de 32 bits).
type RC5Cipher struct {
	s [2 * (rc5R + 1)]uint32 // tabela de subchaves expandida, S[0..2r+1]
}

// NewRC5Cipher cria um cipher RC5-32/12/16 (r=12 fixo, b=len(key), tipicamente 16).
func NewRC5Cipher(key []byte) (*RC5Cipher, error) {
	b := len(key)
	if b < 1 || b > 255 {
		return nil, fmt.Errorf("rpo: RC5 key size %d fora do intervalo 1..255", b)
	}

	// 1. Converte a chave em palavras de 32 bits little-endian (c = ceil(b/u), u=4).
	u := rc5W / 8
	c := (b + u - 1) / u
	if c == 0 {
		c = 1
	}
	l := make([]uint32, c)
	for i := b - 1; i >= 0; i-- {
		l[i/u] = (l[i/u] << 8) + uint32(key[i])
	}

	// 2. Inicializa a tabela S com a progressão P32/Q32.
	t := 2 * (rc5R + 1)
	var s [2 * (rc5R + 1)]uint32
	s[0] = rc5P32
	for i := 1; i < t; i++ {
		s[i] = s[i-1] + rc5Q32
	}

	// 3. Mistura L em S (3*max(t,c) iterações).
	iters := 3 * t
	if c > t {
		iters = 3 * c
	}
	var a, bw uint32
	i, j := 0, 0
	for k := 0; k < iters; k++ {
		s[i] = bits.RotateLeft32(s[i]+a+bw, 3)
		a = s[i]
		rot := int(a+bw) & 31
		l[j] = bits.RotateLeft32(l[j]+a+bw, rot)
		bw = l[j]
		i = (i + 1) % t
		j = (j + 1) % c
	}

	return &RC5Cipher{s: s}, nil
}

// BlockSize implementa cipher.Block.
func (c *RC5Cipher) BlockSize() int { return 8 }

// Encrypt implementa cipher.Block — bloco de 2 palavras de 32 bits (little-endian).
func (c *RC5Cipher) Encrypt(dst, src []byte) {
	a := le32(src[0:4]) + c.s[0]
	b := le32(src[4:8]) + c.s[1]
	for i := 1; i <= rc5R; i++ {
		a = bits.RotateLeft32(a^b, int(b&31)) + c.s[2*i]
		b = bits.RotateLeft32(b^a, int(a&31)) + c.s[2*i+1]
	}
	putLe32(dst[0:4], a)
	putLe32(dst[4:8], b)
}

// Decrypt implementa cipher.Block.
func (c *RC5Cipher) Decrypt(dst, src []byte) {
	a := le32(src[0:4])
	b := le32(src[4:8])
	for i := rc5R; i >= 1; i-- {
		b = bits.RotateLeft32(b-c.s[2*i+1], -int(a&31)) ^ a
		a = bits.RotateLeft32(a-c.s[2*i], -int(b&31)) ^ b
	}
	putLe32(dst[0:4], a-c.s[0])
	putLe32(dst[4:8], b-c.s[1])
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func putLe32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
