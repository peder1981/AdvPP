package rpo

import "fmt"

// RC2 (RFC 2268) — implementação portada linha a linha do algoritmo de
// referência (pycryptodome ARC2.c, key expansion + encrypt/decrypt),
// necessária porque nem a stdlib do Go nem golang.org/x/crypto
// implementam RC2. Validado contra vetores gerados pela mesma
// referência (ver rc2_test.go) antes de ser usado em qualquer decodificação
// real do RPO — não é uma tentativa não verificada.
//
// Um dos cerca de 12 algoritmos legados confirmados via desmontagem real
// de tCryptoEVP::Encrypt (ver docs/rpo-format-sonnet.md) na tabela
// rotativa de cifras usada pelo RPO do Protheus.

var rc2Permute = [256]byte{
	217, 120, 249, 196, 25, 221, 181, 237, 40, 233, 253, 121, 74, 160, 216, 157,
	198, 126, 55, 131, 43, 118, 83, 142, 98, 76, 100, 136, 68, 139, 251, 162,
	23, 154, 89, 245, 135, 179, 79, 19, 97, 69, 109, 141, 9, 129, 125, 50,
	189, 143, 64, 235, 134, 183, 123, 11, 240, 149, 33, 34, 92, 107, 78, 130,
	84, 214, 101, 147, 206, 96, 178, 28, 115, 86, 192, 20, 167, 140, 241, 220,
	18, 117, 202, 31, 59, 190, 228, 209, 66, 61, 212, 48, 163, 60, 182, 38,
	111, 191, 14, 218, 70, 105, 7, 87, 39, 242, 29, 155, 188, 148, 67, 3,
	248, 17, 199, 246, 144, 239, 62, 231, 6, 195, 213, 47, 200, 102, 30, 215,
	8, 232, 234, 222, 128, 82, 238, 247, 132, 170, 114, 172, 53, 77, 106, 42,
	150, 26, 210, 113, 90, 21, 73, 116, 75, 159, 208, 94, 4, 24, 164, 236,
	194, 224, 65, 110, 15, 81, 203, 204, 36, 145, 175, 80, 161, 244, 112, 57,
	153, 124, 58, 133, 35, 184, 180, 122, 252, 2, 54, 91, 37, 85, 151, 49,
	45, 93, 250, 152, 227, 138, 146, 174, 5, 223, 41, 16, 103, 108, 186, 201,
	211, 0, 230, 207, 225, 158, 168, 44, 99, 22, 1, 63, 88, 226, 137, 169,
	13, 56, 52, 27, 171, 51, 255, 176, 187, 72, 12, 95, 185, 177, 205, 46,
	197, 243, 219, 71, 229, 165, 156, 119, 10, 166, 32, 104, 254, 127, 193, 173,
}

// RC2Cipher implementa cipher.Block (BlockSize=8) para RC2/ARC2.
type RC2Cipher struct {
	expKey [64]uint16
}

// NewRC2Cipher cria um cipher RC2 com bits de chave efetiva = len(key)*8
// (o padrão usado pelo OpenSSL EVP_rc2_*() quando nenhum parâmetro de
// bits efetivos é passado explicitamente — é o caso do RPO, confirmado
// via desmontagem: key_len=16 sem nenhum EVP_CIPHER_CTX_ctrl de override).
func NewRC2Cipher(key []byte) (*RC2Cipher, error) {
	return NewRC2CipherEffective(key, len(key)*8)
}

// NewRC2CipherEffective cria um cipher RC2 com bits de chave efetiva
// explícitos (5..1024), para os casos em que o RPO usar uma variante
// truncada (ex.: rc2-40).
func NewRC2CipherEffective(key []byte, effectiveBits int) (*RC2Cipher, error) {
	t := len(key)
	if t < 1 || t > 128 {
		return nil, fmt.Errorf("rpo: RC2 key size %d fora do intervalo 1..128", t)
	}
	if effectiveBits < 5 || effectiveBits > 1024 {
		return nil, fmt.Errorf("rpo: RC2 effectiveBits %d fora do intervalo 5..1024", effectiveBits)
	}

	var bkey [128]byte
	copy(bkey[:], key)

	t8 := byte((effectiveBits + 7) / 8)
	tm := byte((1 << uint(8-(int(t8)*8-effectiveBits))) - 1)

	for i := t; i < 128; i++ {
		bkey[i] = rc2Permute[(int(bkey[i-1])+int(bkey[i-t]))%256]
	}
	bkey[128-int(t8)] = rc2Permute[bkey[128-int(t8)]&tm]
	for i := 127 - int(t8); i >= 0; i-- {
		bkey[i] = rc2Permute[bkey[i+1]^bkey[i+int(t8)]]
	}

	c := &RC2Cipher{}
	for i := 0; i < 64; i++ {
		c.expKey[i] = uint16(bkey[2*i]) + 256*uint16(bkey[2*i+1])
	}
	return c, nil
}

// BlockSize implementa cipher.Block.
func (c *RC2Cipher) BlockSize() int { return 8 }

func rol16(x uint16, p uint) uint16 { return (x << p) | (x >> (16 - p)) }
func ror16(x uint16, p uint) uint16 { return (x >> p) | (x << (16 - p)) }

func rc2MixRound(r *[4]uint16, k [64]uint16, j *int) {
	r[0] += k[*j] + (r[3] & r[2]) + (^r[3] & r[1])
	*j++
	r[0] = rol16(r[0], 1)
	r[1] += k[*j] + (r[0] & r[3]) + (^r[0] & r[2])
	*j++
	r[1] = rol16(r[1], 2)
	r[2] += k[*j] + (r[1] & r[0]) + (^r[1] & r[3])
	*j++
	r[2] = rol16(r[2], 3)
	r[3] += k[*j] + (r[2] & r[1]) + (^r[2] & r[0])
	*j++
	r[3] = rol16(r[3], 5)
}

func rc2InvMixRound(r *[4]uint16, k [64]uint16, j *int) {
	r[3] = ror16(r[3], 5)
	r[3] -= k[*j] + (r[2] & r[1]) + (^r[2] & r[0])
	*j--
	r[2] = ror16(r[2], 3)
	r[2] -= k[*j] + (r[1] & r[0]) + (^r[1] & r[3])
	*j--
	r[1] = ror16(r[1], 2)
	r[1] -= k[*j] + (r[0] & r[3]) + (^r[0] & r[2])
	*j--
	r[0] = ror16(r[0], 1)
	r[0] -= k[*j] + (r[3] & r[2]) + (^r[3] & r[1])
	*j--
}

func rc2MashRound(r *[4]uint16, k [64]uint16) {
	r[0] += k[r[3]&63]
	r[1] += k[r[0]&63]
	r[2] += k[r[1]&63]
	r[3] += k[r[2]&63]
}

func rc2InvMashRound(r *[4]uint16, k [64]uint16) {
	r[3] -= k[r[2]&63]
	r[2] -= k[r[1]&63]
	r[1] -= k[r[0]&63]
	r[0] -= k[r[3]&63]
}

// Encrypt implementa cipher.Block.
func (c *RC2Cipher) Encrypt(dst, src []byte) {
	var r [4]uint16
	for i := 0; i < 4; i++ {
		r[i] = uint16(src[2*i]) + 256*uint16(src[2*i+1])
	}
	j := 0
	for i := 0; i < 5; i++ {
		rc2MixRound(&r, c.expKey, &j)
	}
	rc2MashRound(&r, c.expKey)
	for i := 0; i < 6; i++ {
		rc2MixRound(&r, c.expKey, &j)
	}
	rc2MashRound(&r, c.expKey)
	for i := 0; i < 5; i++ {
		rc2MixRound(&r, c.expKey, &j)
	}
	for i := 0; i < 4; i++ {
		dst[2*i] = byte(r[i] & 255)
		dst[2*i+1] = byte(r[i] >> 8)
	}
}

// Decrypt implementa cipher.Block.
func (c *RC2Cipher) Decrypt(dst, src []byte) {
	var r [4]uint16
	for i := 0; i < 4; i++ {
		r[i] = uint16(src[2*i]) + 256*uint16(src[2*i+1])
	}
	j := 63
	for i := 0; i < 5; i++ {
		rc2InvMixRound(&r, c.expKey, &j)
	}
	rc2InvMashRound(&r, c.expKey)
	for i := 0; i < 6; i++ {
		rc2InvMixRound(&r, c.expKey, &j)
	}
	rc2InvMashRound(&r, c.expKey)
	for i := 0; i < 5; i++ {
		rc2InvMixRound(&r, c.expKey, &j)
	}
	for i := 0; i < 4; i++ {
		dst[2*i] = byte(r[i] & 255)
		dst[2*i+1] = byte(r[i] >> 8)
	}
}
