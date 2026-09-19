// Package rpo — cipher_dispatch.go implementa o mecanismo REAL de
// cifra do RPO do Protheus, confirmado por desmontagem real de
// `tCryptoEVP::Encrypt` (objdump/gdb `disassemble` + `info symbol` no
// ponteiro de função `do_cipher` de cada `EVP_CIPHER` retornado por uma
// tabela de despacho indireta) — ver docs/rpo-format-sonnet.md para a
// investigação completa, com controle estatístico e matches byte a
// byte contra RPOs reais.
//
// ACHADO CENTRAL (substitui qualquer documentação anterior que dizia
// "AES-128-CBC padrão" — essa afirmação foi testada e refutada por
// busca exaustiva, ver docs/rpo-format.md Fase 15): não existe UM
// algoritmo fixo. `tCryptoEVP::Encrypt` escolhe, por chamada, uma
// cifra de uma TABELA ROTATIVA de ~12 algoritmos legados do OpenSSL —
// DES, 3DES (DES-EDE de 2 chaves), RC4, RC5-32/12/16, CAST5, Blowfish,
// RC2 — em vários modos (ECB/CBC/CFB64/OFB), todas derivando do MESMO
// par chave+IV de 16+8 bytes fixado por sessão de compilação via
// `tCryptoEVP::SetKey`. IDEA também aparece na tabela mas não tem
// implementação verificada aqui (ver idea.go).
package rpo

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rc4"
	"fmt"
	"strings"

	"golang.org/x/crypto/blowfish"
	"golang.org/x/crypto/cast5"
)

// pkcs7Pad/pkcs7Unpad — confirmado via dado real capturado que ECB/CBC
// usam padding PKCS7 por padrão (comportamento padrão do OpenSSL EVP
// quando o padding não é explicitamente desabilitado via
// EVP_CIPHER_CTX_set_padding(ctx, 0) — não observado no caminho de
// tCryptoEVP::Encrypt via desmontagem). Verificado com Blowfish-ECB
// contra admin_section.bin real (ver cipher_dispatch_test.go).
func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	if padLen == 0 {
		padLen = blockSize
	}
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("rpo: pkcs7Unpad: tamanho inválido %d", len(data))
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, fmt.Errorf("rpo: pkcs7Unpad: padding inválido (%d)", padLen)
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, fmt.Errorf("rpo: pkcs7Unpad: bytes de padding inconsistentes")
		}
	}
	return data[:len(data)-padLen], nil
}

// cipherMode identifica o modo de operação de bloco extraído do nome
// simbólico retornado por `info symbol` no ponteiro `do_cipher`.
type cipherMode int

const (
	modeECB cipherMode = iota
	modeCBC
	modeCFB
	modeOFB
	modeStream // RC4 — não é cipher de bloco, não tem "modo" separado
)

// ParseCipherName decompõe um nome como "des_ede_cfb64_cipher" ou
// "rc5_32_12_16_ofb_cipher" (exatamente como `info symbol` do gdb
// resolve o ponteiro `do_cipher` da struct EVP_CIPHER real) em
// (algoritmo-base, modo). Retorna erro se o nome não for reconhecido.
func ParseCipherName(name string) (base string, mode cipherMode, err error) {
	n := strings.TrimSuffix(name, "_cipher")
	switch {
	case n == "rc4":
		return "rc4", modeStream, nil
	case strings.HasSuffix(n, "_cbc"):
		return strings.TrimSuffix(n, "_cbc"), modeCBC, nil
	case strings.HasSuffix(n, "_ecb"):
		return strings.TrimSuffix(n, "_ecb"), modeECB, nil
	case strings.HasSuffix(n, "_cfb64"):
		return strings.TrimSuffix(n, "_cfb64"), modeCFB, nil
	case strings.HasSuffix(n, "_ofb"):
		return strings.TrimSuffix(n, "_ofb"), modeOFB, nil
	default:
		return "", 0, fmt.Errorf("rpo: nome de cipher não reconhecido: %q", name)
	}
}

// blockCipherFor constrói o cipher.Block correspondente ao algoritmo
// base identificado, usando a chave nos bytes exatos capturados (cada
// algoritmo usa o tamanho de chave que aceitar — RC4 e RC2/CAST5/
// Blowfish/RC5 toleram o tamanho capturado diretamente; DES simples
// espera 8 bytes, DES-EDE de 2 chaves espera 16 e é expandido para 24
// (K1,K2,K1) antes de chamar des.NewTripleDESCipher, que só aceita 24).
func blockCipherFor(base string, key []byte) (cipher.Block, error) {
	switch base {
	case "des":
		return des.NewCipher(key)
	case "des_ede":
		if len(key) == 16 {
			k24 := make([]byte, 24)
			copy(k24, key)
			copy(k24[16:], key[:8])
			return des.NewTripleDESCipher(k24)
		}
		return des.NewTripleDESCipher(key)
	case "cast5":
		return cast5.NewCipher(key)
	case "bf":
		return blowfish.NewCipher(key)
	case "rc2":
		return NewRC2Cipher(key)
	case "rc5_32_12_16":
		return NewRC5Cipher(key)
	case "idea":
		return nil, fmt.Errorf("rpo: IDEA não tem implementação verificada nesta versão — ver aviso em idea.go; decodificação de segmentos IDEA não é suportada")
	default:
		return nil, fmt.Errorf("rpo: algoritmo base desconhecido: %q", base)
	}
}

// EncryptSegment cifra um segmento de plaintext usando o cipher/chave/IV
// reais identificados via captura ao vivo (ver
// tools/rpo-live-inspect/extract_rpo.py e docs/rpo-format-sonnet.md
// para como obter esses três valores de uma sessão de compilação real).
// `cipherName` é o nome exato como `info symbol` do gdb resolve
// (ex.: "cast5_ofb_cipher", "des_ede_cbc_cipher").
func EncryptSegment(cipherName string, key, iv, plaintext []byte) ([]byte, error) {
	base, mode, err := ParseCipherName(cipherName)
	if err != nil {
		return nil, err
	}
	if base == "rc4" {
		c, err := rc4.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("rpo: rc4.NewCipher: %w", err)
		}
		out := make([]byte, len(plaintext))
		c.XORKeyStream(out, plaintext)
		return out, nil
	}

	block, err := blockCipherFor(base, key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()

	switch mode {
	case modeECB:
		padded := pkcs7Pad(plaintext, bs)
		out := make([]byte, len(padded))
		for off := 0; off < len(padded); off += bs {
			block.Encrypt(out[off:off+bs], padded[off:off+bs])
		}
		return out, nil
	case modeCBC:
		if len(iv) != bs {
			return nil, fmt.Errorf("rpo: CBC requer IV de %d bytes, recebido %d", bs, len(iv))
		}
		padded := pkcs7Pad(plaintext, bs)
		out := make([]byte, len(padded))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
		return out, nil
	case modeCFB:
		if len(iv) != bs {
			return nil, fmt.Errorf("rpo: CFB requer IV de %d bytes, recebido %d", bs, len(iv))
		}
		out := make([]byte, len(plaintext))
		cipher.NewCFBEncrypter(block, iv).XORKeyStream(out, plaintext) //nolint:staticcheck // modo exigido para compatibilidade, não escolha de segurança
		return out, nil
	case modeOFB:
		if len(iv) != bs {
			return nil, fmt.Errorf("rpo: OFB requer IV de %d bytes, recebido %d", bs, len(iv))
		}
		out := make([]byte, len(plaintext))
		cipher.NewOFB(block, iv).XORKeyStream(out, plaintext) //nolint:staticcheck // modo exigido para compatibilidade, não escolha de segurança
		return out, nil
	default:
		return nil, fmt.Errorf("rpo: modo desconhecido para %q", cipherName)
	}
}

// DecryptSegment reverte EncryptSegment. Para os modos stream (OFB,
// RC4) a operação é idêntica a Encrypt (XOR do mesmo keystream); para
// ECB/CBC/CFB usa a direção de decriptação correta do cipher.Block.
func DecryptSegment(cipherName string, key, iv, ciphertext []byte) ([]byte, error) {
	base, mode, err := ParseCipherName(cipherName)
	if err != nil {
		return nil, err
	}
	if base == "rc4" {
		// RC4 é simétrico: decrypt = encrypt (XOR do mesmo keystream).
		return EncryptSegment(cipherName, key, iv, ciphertext)
	}
	if mode == modeOFB {
		// OFB também é simétrico.
		return EncryptSegment(cipherName, key, iv, ciphertext)
	}

	block, err := blockCipherFor(base, key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()

	switch mode {
	case modeECB:
		if len(ciphertext)%bs != 0 {
			return nil, fmt.Errorf("rpo: ECB requer ciphertext múltiplo de %d bytes, recebido %d", bs, len(ciphertext))
		}
		out := make([]byte, len(ciphertext))
		for off := 0; off < len(ciphertext); off += bs {
			block.Decrypt(out[off:off+bs], ciphertext[off:off+bs])
		}
		return pkcs7Unpad(out, bs)
	case modeCBC:
		if len(ciphertext)%bs != 0 {
			return nil, fmt.Errorf("rpo: CBC requer ciphertext múltiplo de %d bytes, recebido %d", bs, len(ciphertext))
		}
		if len(iv) != bs {
			return nil, fmt.Errorf("rpo: CBC requer IV de %d bytes, recebido %d", bs, len(iv))
		}
		out := make([]byte, len(ciphertext))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ciphertext)
		return pkcs7Unpad(out, bs)
	case modeCFB:
		if len(iv) != bs {
			return nil, fmt.Errorf("rpo: CFB requer IV de %d bytes, recebido %d", bs, len(iv))
		}
		out := make([]byte, len(ciphertext))
		cipher.NewCFBDecrypter(block, iv).XORKeyStream(out, ciphertext) //nolint:staticcheck
		return out, nil
	default:
		return nil, fmt.Errorf("rpo: modo desconhecido para %q", cipherName)
	}
}
