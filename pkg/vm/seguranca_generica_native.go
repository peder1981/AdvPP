package vm

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"hash"
	"os"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerSegurancaGenericaNatives registra as funções de Segurança / Genérica
// do Protheus: extração de informações de certificados PEM/PFX (PEMInfo,
// PFXInfo) e conversores de certificados/chaves para o formato PEM
// (PFX2PEM, PFXCA2PEM, PFXCert2PEM, PFXKey2PEM, PK7Key2PEM, PK8Key2PEM).
//
// LIMITAÇÕES documentadas desta implementação:
//   - PKCS#12 (arquivos .PFX) NÃO é suportado pela stdlib Go. As funções PFX*
//     aceitam como fallback arquivos que sejam realmente bundles PEM (certificados
//     + chave) ou DER PKCS#8/PKCS#1 — usados no lugar do .PFX — e degradam com
//     graça (Nil/.F.) diante de um container PKCS#12 real, em vez de inventar um
//     parser ASN.1 proprietário. Para PFX reais, usar OpenSSL (openssl pkcs12).
//   - PKCS#7 (arquivos .PK7) também não é parseável pela stdlib. PK7Key2PEM
//     converte o bundle DER para PEM (bloco "PKCS7") intacto, sem extrair os
//     certificados individuais — ferramentas OpenSSL conseguem consumir o
//     resultado.
//   - Parâmetros @cError (por referência) não são gravados: esta VM embutida não
//     propaga referências de argumentos (apenas arrays). O retorno lógico já
//     sinaliza o resultado.
func (v *VM) registerSegurancaGenericaNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// =========================================================================
	// PEMInfo(cFile, [cPassword], [nHashAlgorithm]) -> aRet
	//   Extrai informações de todos os certificados de um arquivo .PEM.
	//   Cada item do vetor de retorno é um vetor com:
	//     [1] nVersão (0=V1, 1=V2, 2=V3)
	//     [2] cDestinatário (Subject)
	//     [3] cEmissor (Issuer)
	//     [4] cDataValidadeInicial
	//     [5] cDataValidadeFinal
	//     [6] cNúmeroSerial
	//     [7] cFingerprint/Thumbprint (Base64)
	//     [8] cFingerprint/Thumbprint (Hexadecimal)
	//   nHashAlgorithm: 3=SHA1 (default), 4=SHA224, 5=SHA256, 6=SHA384,
	//   7=SHA512. cPassword não é usado (PEM não tem senha neste contexto).
	//   Retorna vetor vazio quando o arquivo não contém certificados e Nil em
	//   caso de erro (arquivo inexistente/ilegível ou PEM inválido).
	//   A função também aceita o conteúdo PEM diretamente (convenção do projeto,
	//   igual a readKeyMaterial), não apenas o caminho do arquivo.
	// =========================================================================
	natives["PEMINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		input := getArgString(args, 0, "")
		data, ok := readKeyMaterial(input)
		if !ok {
			return advplrt.Nil, nil
		}
		certs := parseCertificatesFromPEM(data)
		result := make([]advplrt.Value, 0, len(certs))
		nHash := advplrt.ToFloat(getArg(args, 2))
		for _, cert := range certs {
			b64, hexStr, ok := certificateFingerprint(cert, nHash)
			if !ok {
				return advplrt.Nil, nil
			}
			result = append(result, advplrt.NewArray([]advplrt.Value{
				advplrt.NewNumber(float64(certDNVersion(cert))),
				advplrt.NewString(cert.Subject.String()),
				advplrt.NewString(cert.Issuer.String()),
				advplrt.NewString(certDateStr(cert.NotBefore)),
				advplrt.NewString(certDateStr(cert.NotAfter)),
				advplrt.NewString(cert.SerialNumber.String()),
				advplrt.NewString(b64),
				advplrt.NewString(hexStr),
			}))
		}
		return advplrt.NewArray(result), nil
	}

	// =========================================================================
	// PFXInfo(cFile, [cPassword]) -> aRet
	//   Extrai as informações do certificado de cliente e dos certificados de CA
	//   de um arquivo .PFX. O vetor de retorno informa primeiro o certificado de
	//   cliente e depois cada CA. Cada item é um vetor com:
	//     [1] nVersão, [2] cDestinatário, [3] cEmissor,
	//     [4] cDataValidadeInicial, [5] cDataValidadeFinal.
	//   Se não houver certificado de cliente, o primeiro item é Nil. Em caso de
	//   erro de leitura/parse retorna Nil (spec TDN). Container PKCS#12 real não
	//   é parseável pela stdlib — retorna Nil com a limitação documentada; o
	//   fallback aceita bundles PEM/DER chamados de .pfx.
	// =========================================================================
	natives["PFXINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		certs, _, err := loadPfxMaterial(getArgString(args, 0, ""), getArgString(args, 1, ""))
		if err != nil {
			return advplrt.Nil, nil
		}
		if len(certs) == 0 {
			return advplrt.NewArray([]advplrt.Value{advplrt.Nil}), nil
		}
		result := make([]advplrt.Value, 0, len(certs))
		result = append(result, certInfoArray(certs[0]))
		for _, ca := range certs[1:] {
			result = append(result, certInfoArray(ca))
		}
		return advplrt.NewArray(result), nil
	}

	// =========================================================================
	// PFXCert2PEM(cPFXFile, cPEMFile, @cError, [cPassword]) -> lRet
	//   Extrai o certificado de cliente de um .PFX e grava em cPEMFile.
	//   @cError não é gravado (limitação de referência desta VM). Retorna .T. em
	//   caso de sucesso; .F. em falha.
	// =========================================================================
	natives["PFXCERT2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		certs, _, err := loadPfxMaterial(getArgString(args, 0, ""), getArgString(args, 3, ""))
		if err != nil || len(certs) == 0 {
			return advplrt.NewBool(false), nil
		}
		if err := writeCertsPEM(certs[:1], getArgString(args, 1, "")); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// PFXCA2PEM(cPFXFile, cPEMFile, @cError, [cPassword]) -> lRet
	//   Extrai os certificados de autorização (CA) de um .PFX e grava em
	//   cPEMFile (todos os certificados exceto o de cliente). Retorna .F.
	//   quando não há CAs.
	// =========================================================================
	natives["PFXCA2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		certs, _, err := loadPfxMaterial(getArgString(args, 0, ""), getArgString(args, 3, ""))
		if err != nil || len(certs) < 2 {
			return advplrt.NewBool(false), nil
		}
		if err := writeCertsPEM(certs[1:], getArgString(args, 1, "")); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// PFXKey2PEM(cPFXFile, cPEMFile, @cError, [cPassword]) -> lRet
	//   Extrai a chave privada de um .PFX e grava em cPEMFile (PEM "RSA PRIVATE
	//   KEY" ou "EC PRIVATE KEY" conforme o tipo). Retorna .F. sem chave.
	// =========================================================================
	natives["PFXKEY2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_, priv, err := loadPfxMaterial(getArgString(args, 0, ""), getArgString(args, 3, ""))
		if err != nil || priv == nil {
			return advplrt.NewBool(false), nil
		}
		keyPEM := marshalPrivateKeyPEM(priv)
		if keyPEM == nil {
			return advplrt.NewBool(false), nil
		}
		if err := os.WriteFile(getArgString(args, 1, ""), keyPEM, 0644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// PFX2PEM(cPFXFile, cPEMFile, @cError, [cPassword]) -> lRet
	//   Extrai certificado de cliente + CAs + chave privada de um .PFX e grava
	//   o bundle PEM completo em cPEMFile. Retorna .F. quando nada é extraído.
	// =========================================================================
	natives["PFX2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		certs, priv, err := loadPfxMaterial(getArgString(args, 0, ""), getArgString(args, 3, ""))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		var buf bytes.Buffer
		if len(certs) > 0 {
			if err := encodeCertsPEM(&buf, certs); err != nil {
				return advplrt.NewBool(false), nil
			}
		}
		if priv != nil {
			keyPEM := marshalPrivateKeyPEM(priv)
			if keyPEM == nil {
				return advplrt.NewBool(false), nil
			}
			buf.Write(keyPEM)
		}
		if buf.Len() == 0 {
			return advplrt.NewBool(false), nil
		}
		if err := os.WriteFile(getArgString(args, 1, ""), buf.Bytes(), 0644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// PK8Key2PEM(cPK8File, cPEMFile, @cError, [cPassword]) -> lRet
	//   Converte uma chave privada PKCS#8 em DER para PEM (bloco "PRIVATE KEY").
	//   Aceita também DER PKCS#1 e PEM "RSA PRIVATE KEY"/"PRIVATE KEY" como
	//   entrada (o resultado é sempre normalizado para PKCS#8 PEM).
	//   cPassword (senha que será usada no arquivo .PEM de saída) NÃO é
	//   suportado: a stdlib Go não gera PKCS#8 criptografado — com senha
	//   informada retorna .F. documentando a limitação.
	// =========================================================================
	natives["PK8KEY2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if getArgString(args, 3, "") != "" {
			return advplrt.NewBool(false), nil
		}
		data, err := os.ReadFile(getArgString(args, 0, ""))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		der := data
		if strings.Contains(string(data), "-----BEGIN") {
			block, _ := pem.Decode(data)
			if block == nil {
				return advplrt.NewBool(false), nil
			}
			der = block.Bytes
		}
		priv, err := parseAnyPrivateKey(der)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		out := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
		if err := os.WriteFile(getArgString(args, 1, ""), out, 0644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// PK7Key2PEM(cPK7File, cPEMFile, @cError) -> lRet
	//   Converte um arquivo PKCS#7 em DER para PEM. A stdlib Go não parseia
	//   PKCS#7: o bundle é convertido intacto para um bloco PEM "PKCS7"
	//   (consumível por OpenSSL), sem extrair os certificados individuais.
	//   Entrada já em PEM ("-----BEGIN PKCS7-----") é normalizada e re-escrita.
	// =========================================================================
	natives["PK7KEY2PEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		data, err := os.ReadFile(getArgString(args, 0, ""))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		var out bytes.Buffer
		if strings.Contains(string(data), "-----BEGIN") {
			rest := data
			found := false
			for {
				var block *pem.Block
				block, rest = pem.Decode(rest)
				if block == nil {
					break
				}
				if strings.EqualFold(block.Type, "PKCS7") {
					if err := pem.Encode(&out, block); err != nil {
						return advplrt.NewBool(false), nil
					}
					found = true
				}
			}
			if !found {
				return advplrt.NewBool(false), nil
			}
		} else {
			if len(data) == 0 || data[0] != 0x30 { // mínimo sanity check de ASN.1 DER (SEQUENCE)
				return advplrt.NewBool(false), nil
			}
			if err := pem.Encode(&out, &pem.Block{Type: "PKCS7", Bytes: data}); err != nil {
				return advplrt.NewBool(false), nil
			}
		}
		if err := os.WriteFile(getArgString(args, 1, ""), out.Bytes(), 0644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}
}

// ----------------------------------------------------------------------------
// Helpers internos (não exportados)
// ----------------------------------------------------------------------------

// parseCertificatesFromPEM extrai todos os certificados X.509 de um arquivo PEM
// (blocos "CERTIFICATE"/"X509 CERTIFICATE").
func parseCertificatesFromPEM(data []byte) []*x509.Certificate {
	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE", "X509 CERTIFICATE":
			if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				certs = append(certs, cert)
			}
		}
	}
	return certs
}

// certificateFingerprint calcula o fingerprint/thumbprint do certificado com o
// algoritmo informado: 3=SHA1 (default), 4=SHA224, 5=SHA256, 6=SHA384,
// 7=SHA512. Retorna Base64 e Hexadecimal do digest.
func certificateFingerprint(cert *x509.Certificate, nHashAlgorithm float64) (b64, hexStr string, ok bool) {
	var h hash.Hash
	switch int(nHashAlgorithm) {
	case 0, 3:
		h = sha1.New()
	case 4:
		h = sha256.New224()
	case 5:
		h = sha256.New()
	case 6:
		h = sha512.New384()
	case 7:
		h = sha512.New()
	default:
		return "", "", false
	}
	h.Write(cert.Raw)
	sum := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(sum), hex.EncodeToString(sum), true
}

// certDateStr formata uma data de certificado como string (formato ISO
// "2006-01-02 15:04:05").
func certDateStr(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// certDNVersion converte a versão do certificado (Go a reporta 1-indexada:
// parser.go "for backwards compat reasons Version is one-indexed") para a
// convenção do TDN: 0=Versão 1, 1=Versão 2, 2=Versão 3.
func certDNVersion(cert *x509.Certificate) int {
	v := cert.Version - 1
	if v < 0 {
		return 0
	}
	return v
}

// certInfoArray monta o vetor de 5 elementos usado pelo PFXInfo.
func certInfoArray(cert *x509.Certificate) advplrt.Value {
	return advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(float64(certDNVersion(cert))),
		advplrt.NewString(cert.Subject.String()),
		advplrt.NewString(cert.Issuer.String()),
		advplrt.NewString(certDateStr(cert.NotBefore)),
		advplrt.NewString(certDateStr(cert.NotAfter)),
	})
}

// loadPfxMaterial lê um arquivo .PFX e extrai certificados e chave privada.
// Como a stdlib Go não suporta PKCS#12 (o formato real dos .PFX), o conteúdo é
// interpretado como bundle PEM ou DER PKCS#8/PKCS#1 (fallback documentado).
// Retorna erro quando nada é reconhecido — diante de um container PKCS#12 real.
func loadPfxMaterial(file, password string) ([]*x509.Certificate, crypto.PrivateKey, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	certs, priv := parsePEMOrDER(data, password)
	if len(certs) == 0 && priv == nil {
		return nil, nil, errors.New("PFX (PKCS#12) não suportado pela stdlib Go; forneça arquivo PEM ou DER PKCS#8")
	}
	return certs, priv, nil
}

// parsePEMOrDER interpreta bytes como bundle PEM ou DER (PKCS#8/PKCS#1/EC) e
// devolve certificados e a primeira chave privada encontrada.
func parsePEMOrDER(data []byte, password string) ([]*x509.Certificate, crypto.PrivateKey) {
	var certs []*x509.Certificate
	var priv crypto.PrivateKey
	if strings.Contains(string(data), "-----BEGIN") {
		rest := data
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			cert, key := parsePEMBlock(block, password)
			if cert != nil {
				certs = append(certs, cert)
			}
			if key != nil && priv == nil {
				priv = key
			}
		}
		return certs, priv
	}
	// DER direto.
	if key, err := parseAnyPrivateKey(data); err == nil {
		return certs, key
	}
	if cert, err := x509.ParseCertificate(data); err == nil {
		return []*x509.Certificate{cert}, nil
	}
	return nil, nil
}

// parsePEMBlock interpreta um bloco PEM como certificado ou chave privada
// (tratando blocos criptografados com o password).
func parsePEMBlock(block *pem.Block, password string) (*x509.Certificate, crypto.PrivateKey) {
	der := block.Bytes
	if x509.IsEncryptedPEMBlock(block) {
		dec, err := x509.DecryptPEMBlock(block, []byte(password))
		if err != nil {
			return nil, nil
		}
		der = dec
	}
	switch block.Type {
	case "CERTIFICATE", "X509 CERTIFICATE":
		if cert, err := x509.ParseCertificate(der); err == nil {
			return cert, nil
		}
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY":
		if key, err := parseAnyPrivateKey(der); err == nil {
			return nil, key
		}
	}
	return nil, nil
}

// parseAnyPrivateKey tenta interpretar DER como PKCS#8, PKCS#1 ou EC.
func parseAnyPrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("chave privada não suportada")
}

// encodeCertsPEM serializa certificados em formato PEM.
func encodeCertsPEM(dst *bytes.Buffer, certs []*x509.Certificate) error {
	for _, cert := range certs {
		if err := pem.Encode(dst, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
			return err
		}
	}
	return nil
}

// writeCertsPEM grava os certificados em um arquivo PEM.
func writeCertsPEM(certs []*x509.Certificate, outFile string) error {
	var buf bytes.Buffer
	if err := encodeCertsPEM(&buf, certs); err != nil {
		return err
	}
	return os.WriteFile(outFile, buf.Bytes(), 0644)
}

// marshalPrivateKeyPEM serializa uma chave privada em formato PEM
// ("RSA PRIVATE KEY" ou "EC PRIVATE KEY"); Nil para tipos não suportados.
func marshalPrivateKeyPEM(priv crypto.PrivateKey) []byte {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	case *ecdsa.PrivateKey:
		if der, err := x509.MarshalECPrivateKey(k); err == nil {
			return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
		}
	}
	return nil
}
