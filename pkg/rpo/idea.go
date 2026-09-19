package rpo

import "fmt"

// IDEA (Lai/Massey, 1991) — implementado a partir da especificação
// pública (8 rounds + transformação de saída, bloco de 64 bits, chave
// de 128 bits). Necessário porque nenhuma biblioteca disponível neste
// ambiente (stdlib Go, golang.org/x/crypto, OpenSSL do host,
// pycryptodome) implementa IDEA — foi excluída de builds padrão por
// décadas por causa de patentes hoje expiradas.
//
// [!] STATUS: NÃO VERIFICADA — diferente de rc2.go (cross-validado
// contra pycryptodome) e rc5.go (validado contra dado real capturado ao
// vivo), esta implementação de IDEA NÃO reproduz o ciphertext real
// capturado de uma sessão de compilação Protheus real, e o teste de
// round-trip (encrypt→decrypt) também falha — há pelo menos um bug
// remanescente na expansão/inversão de chave ou na estrutura de round
// que não foi isolado com o tempo disponível. Um bug real já foi
// encontrado e corrigido nesta implementação (troca das posições 2/3 na
// saída de cada round), mas não foi suficiente para fechar o resultado.
//
// NÃO use esta implementação para decodificar dados reais — o
// dispatcher em cipher_dispatch.go retorna erro explícito para
// "idea_*_cipher" em vez de silenciosamente produzir garbage. Fica
// aqui como ponto de partida documentado para quem quiser terminar a
// depuração (ver idea_test.go, TestIDEA_RoundTrip e
// TestIDEA_CFB64_RealCapture — ambos falham no estado atual).
//
// Confirmado via desmontagem real de tCryptoEVP::Encrypt que "idea_*"
// realmente faz parte da tabela rotativa de cifras do RPO (não é uma
// suposição) — só a implementação aqui ainda não está correta.

const ideaRounds = 8

// IDEACipher implementa cipher.Block (BlockSize=8).
type IDEACipher struct {
	encKeys [52]uint16
	decKeys [52]uint16
}

// NewIDEACipher cria um cipher IDEA a partir de uma chave de 16 bytes (128 bits).
func NewIDEACipher(key []byte) (*IDEACipher, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("rpo: IDEA requer chave de 16 bytes, recebido %d", len(key))
	}
	c := &IDEACipher{}
	c.encKeys = ideaExpandKey(key)
	c.decKeys = ideaInvertKey(c.encKeys)
	return c, nil
}

func be16(b []byte) uint16      { return uint16(b[0])<<8 | uint16(b[1]) }
func putBe16(b []byte, v uint16) { b[0] = byte(v >> 8); b[1] = byte(v) }

// ideaMul é a multiplicação modular usada pelo IDEA: (a*b) mod 65537,
// com a convenção de que 0 representa 2^16 (0x10000) nessa aritmética.
func ideaMul(a, b uint16) uint16 {
	const m = 0x10001
	aa, bb := uint32(a), uint32(b)
	if aa == 0 {
		aa = 0x10000
	}
	if bb == 0 {
		bb = 0x10000
	}
	p := (aa * bb) % m
	if p == 0x10000 {
		p = 0
	}
	return uint16(p)
}

// ideaInv é o inverso multiplicativo de a módulo 65537 (para a chave de
// decriptação), via exponenciação rápida (a^(m-2) mod m, Fermat).
func ideaInv(a uint16) uint16 {
	if a <= 1 {
		return a // inverso de 0 (=2^16) e 1 é ele mesmo nessa convenção
	}
	const m = 0x10001
	aa := uint32(a)
	var result uint32 = 1
	exp := uint32(m - 2)
	base := aa
	for exp > 0 {
		if exp&1 == 1 {
			result = (result * base) % m
		}
		base = (base * base) % m
		exp >>= 1
	}
	return uint16(result)
}

// rotateLeft128 gira à esquerda um bloco de 128 bits (8 palavras de 16
// bits, big-endian) por n bits — usado na expansão de chave do IDEA.
func rotateLeft128(k [8]uint16, n uint) [8]uint16 {
	var buf [16]byte
	for i := 0; i < 8; i++ {
		putBe16(buf[2*i:2*i+2], k[i])
	}
	n %= 128
	byteShift := n / 8
	bitShift := n % 8

	var doubled [32]byte
	copy(doubled[:16], buf[:])
	copy(doubled[16:], buf[:])
	var shifted [16]byte
	copy(shifted[:], doubled[byteShift:byteShift+16])

	var out [16]byte
	if bitShift == 0 {
		out = shifted
	} else {
		for i := 0; i < 16; i++ {
			cur := shifted[i]
			next := shifted[(i+1)%16]
			out[i] = (cur << bitShift) | (next >> (8 - bitShift))
		}
	}

	var res [8]uint16
	for i := 0; i < 8; i++ {
		res[i] = be16(out[2*i : 2*i+2])
	}
	return res
}

// ideaExpandKey gera as 52 subchaves de 16 bits a partir de uma chave
// de 128 bits: os 8 primeiros bloco de 16 bits são a própria chave;
// cada bloco seguinte é o anterior girado à esquerda em 25 bits.
func ideaExpandKey(key []byte) [52]uint16 {
	var sk [52]uint16
	var k [8]uint16
	for i := 0; i < 8; i++ {
		k[i] = be16(key[2*i : 2*i+2])
	}
	copy(sk[0:8], k[:])
	for block := 1; block < 7; block++ {
		k = rotateLeft128(k, 25)
		n := copy(sk[block*8:], k[:])
		_ = n
	}
	return sk
}

// ideaInvertKey deriva as subchaves de decriptação a partir das de
// encriptação — inverte a ordem dos rounds e troca multiplicação por
// inverso multiplicativo / adição por seu negativo, conforme a
// estrutura padrão do IDEA.
func ideaInvertKey(ek [52]uint16) [52]uint16 {
	var dk [52]uint16
	p := 0
	// output transform do encrypt vira o primeiro "round" do decrypt (invertido)
	dk[p] = ideaInv(ek[48])
	p++
	dk[p] = -ek[49] // negação em uint16 = complemento aritmético mod 2^16
	p++
	dk[p] = -ek[50]
	p++
	dk[p] = ideaInv(ek[51])
	p++

	for round := ideaRounds - 1; round >= 1; round-- {
		base := round * 6
		dk[p] = ek[base+4]
		p++
		dk[p] = ek[base+5]
		p++
		dk[p] = ideaInv(ek[base+0])
		p++
		dk[p] = -ek[base+2]
		p++
		dk[p] = -ek[base+1]
		p++
		dk[p] = ideaInv(ek[base+3])
		p++
	}
	// último round (round=0) usa as subchaves iniciais, sem swap de 4/5 com o round transform
	dk[p] = ek[4]
	p++
	dk[p] = ek[5]
	p++
	dk[p] = ideaInv(ek[0])
	p++
	dk[p] = -ek[2]
	p++
	dk[p] = -ek[1]
	p++
	dk[p] = ideaInv(ek[3])
	return dk
}

// BlockSize implementa cipher.Block.
func (c *IDEACipher) BlockSize() int { return 8 }

func ideaCryptBlock(sk [52]uint16, in, out []byte) {
	x1 := be16(in[0:2])
	x2 := be16(in[2:4])
	x3 := be16(in[4:6])
	x4 := be16(in[6:8])

	p := 0
	for round := 0; round < ideaRounds; round++ {
		x1 = ideaMul(x1, sk[p])
		p++
		x2 = x2 + sk[p]
		p++
		x3 = x3 + sk[p]
		p++
		x4 = ideaMul(x4, sk[p])
		p++
		t0 := ideaMul(x1^x3, sk[p])
		p++
		t1 := ideaMul(t0+(x2^x4), sk[p])
		p++
		t0 = t0 + t1

		x1 = x1 ^ t1
		x4 = x4 ^ t0
		o2 := x3 ^ t1
		o3 := x2 ^ t0
		// A saída de cada round é (O1, O3, O2, O4) — as posições 2 e 3
		// trocam de lugar antes de entrar no próximo round (ou na
		// transformação de saída, no round 8). Inverter isso foi o bug
		// que quebrava tanto o round-trip quanto o cross-check contra
		// dado real capturado.
		x2, x3 = o3, o2
	}

	y1 := ideaMul(x1, sk[p])
	p++
	y2 := x3 + sk[p]
	p++
	y3 := x2 + sk[p]
	p++
	y4 := ideaMul(x4, sk[p])

	putBe16(out[0:2], y1)
	putBe16(out[2:4], y2)
	putBe16(out[4:6], y3)
	putBe16(out[6:8], y4)
}

// Encrypt implementa cipher.Block.
func (c *IDEACipher) Encrypt(dst, src []byte) {
	ideaCryptBlock(c.encKeys, src, dst)
}

// Decrypt implementa cipher.Block.
func (c *IDEACipher) Decrypt(dst, src []byte) {
	ideaCryptBlock(c.decKeys, src, dst)
}
