package vm

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"os"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerSegurancaCriptografiaNatives registra as funções de Segurança /
// Criptografia do Protheus: hashes (MD5, SHA*, EVPDigest, HMAC), cifras
// simétricas (AES, ARC4/RC4), RSA (assinatura, decriptação, extração de
// chave), CRCCalc, WebEncript, Embaralha, MD5File, WriteRSAPK e os stubs
// honestos das funções HSM (que dependem de infraestrutura de servidor
// Protheus inexistente nesta VM embutida) e HTTPSSLClient.
func (v *VM) registerSegurancaCriptografiaNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// =========================================================================
	// MD5(cValor, [nType]) -> cRet
	//   nType: 1 = RAW_DIGEST (bytes binários), 2 = HEX_DIGEST (default).
	//   RFC 1321.
	// =========================================================================
	natives["MD5"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sum := md5.Sum([]byte(getArgString(args, 0, "")))
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}

	// MD5File(cFile, [nTipo], [nWhere]) -> cHash
	//   nTipo: 1 = RAW_DIGEST, 2 = HEX_DIGEST (default). Em caso de falha na
	//   abertura do arquivo retorna '' (vazio). O parâmetro nWhere (local da
	//   procura no AppServer/SmartClient) não se aplica nesta VM embutida: o
	//   arquivo é lido diretamente do filesystem.
	natives["MD5FILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		data, err := os.ReadFile(getArgString(args, 0, ""))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		sum := md5.Sum(data)
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}

	// SHA1(cContent, [nRetType]) -> cDigest (FIPS 180-1; default HEX).
	// SHA256(cContent, [nRetType]) -> cDigest (FIPS 180-4; default HEX).
	// SHA384(cContent, [nRetType]) -> cDigest (FIPS 180-4; default HEX).
	// SHA512(cContent, [nRetType]) -> cDigest (FIPS 180-4; default HEX).
	natives["SHA1"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sum := sha1.Sum([]byte(getArgString(args, 0, "")))
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}
	natives["SHA256"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sum := sha256.Sum256([]byte(getArgString(args, 0, "")))
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}
	natives["SHA384"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sum := sha512.Sum384([]byte(getArgString(args, 0, "")))
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}
	natives["SHA512"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sum := sha512.Sum512([]byte(getArgString(args, 0, "")))
		return digestResult(sum[:], advplrt.ToFloat(getArg(args, 1))), nil
	}

	// EVPDigest(cContent, nType) -> cRet
	//   Calcula o hash (digest) de um conteúdo, retornando string binária
	//   AdvPL (cada caractere é um byte 0..255).
	//   nType: 1=MD5, 2=RIPEMD160, 3=SHA1, 4=SHA224, 5=SHA256, 6=SHA384,
	//          7=SHA512.
	//   OBS: RIPEMD-160 não está disponível na stdlib Go; retorna '' (honesto).
	natives["EVPDIGEST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		data := []byte(getArgString(args, 0, ""))
		var h hash.Hash
		switch int(advplrt.ToFloat(getArg(args, 1))) {
		case 1:
			h = md5.New()
		case 2:
			return advplrt.NewString(""), nil // RIPEMD160 fora da stdlib
		case 3:
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
			return advplrt.NewString(""), nil
		}
		h.Write(data)
		return advplrt.NewString(string(h.Sum(nil))), nil
	}

	// HMAC(cContent, cKey, nCryptoType, [nRetType], [nContentType], [nKeyType]) -> cDigest
	//   nCryptoType: 1=MD5, 3=SHA1, 5=SHA256, 7=SHA512.
	//   nRetType:    1 = Raw, 2 = Hex (default).
	//   nContentType / nKeyType: 1 = Texto, 2 = Base64, 3 = Hexadecimal.
	natives["HMAC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		hf, err := hmacHashFunc(advplrt.ToFloat(getArg(args, 2)))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		content, err := decodeCryptoInput(getArgString(args, 0, ""), advplrt.ToFloat(getArg(args, 4)))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		key, err := decodeCryptoInput(getArgString(args, 1, ""), advplrt.ToFloat(getArg(args, 5)))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		mac := hmac.New(hf, key)
		mac.Write(content)
		sum := mac.Sum(nil)
		if int(advplrt.ToFloat(getArg(args, 3))) == 1 {
			return advplrt.NewString(string(sum)), nil
		}
		return advplrt.NewString(hex.EncodeToString(sum)), nil
	}

	// AESEncrypt(nCipherID, cPlainText, [cPassword], [cKey], [cIV]) -> aResEnc
	//   nCipherID: 0 = AES-128 CBC, 1 = AES-192 CBC, 2 = AES-256 CBC.
	//   Regra da chave: se cPassword for fornecido, a chave é derivada do
	//   password (ignora cKey); senão usa cKey (deve ter o tamanho do modo);
	//   se nenhum for fornecido, gera uma chave aleatória.
	//   Regra do IV: se cIV for fornecido, usa (CBC exige 16 bytes); senão
	//   gera um IV aleatório.
	//   Retorno: {nResultCode, cCipherText, cKeyUsed, cIVUsed}. O texto e a
	//   chave/IV são retornados como string binária AdvPL.
	//   Derivação da chave a partir do password: primeiros keySize bytes de
	//   SHA-256(password) — determinística e reversível nesta implementação.
	natives["AESENCRYPT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		keySize, ok := aesKeySize(advplrt.ToFloat(getArg(args, 0)))
		if !ok {
			return aesEncResult(1, "", "", ""), nil
		}
		plain := []byte(getArgString(args, 1, ""))
		password := cryptoArgString(args, 2, "")
		cKey := cryptoArgString(args, 3, "")
		cIV := cryptoArgString(args, 4, "")

		var key []byte
		switch {
		case password != "":
			key = deriveAESKey(password, keySize)
		case cKey != "":
			key = []byte(cKey)
			if len(key) != keySize {
				return aesEncResult(2, "", "", ""), nil
			}
		default:
			key = make([]byte, keySize)
			if _, err := rand.Read(key); err != nil {
				return aesEncResult(5, "", "", ""), nil
			}
		}

		var iv []byte
		switch {
		case cIV != "":
			iv = []byte(cIV)
			if len(iv) != aes.BlockSize {
				return aesEncResult(3, "", "", ""), nil
			}
		default:
			iv = make([]byte, aes.BlockSize)
			if _, err := rand.Read(iv); err != nil {
				return aesEncResult(6, "", "", ""), nil
			}
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			return aesEncResult(8, "", "", ""), nil
		}
		padded := pkcs7Pad(plain, block.BlockSize())
		ciphertext := make([]byte, len(padded))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
		return aesEncResult(0, string(ciphertext), string(key), string(iv)), nil
	}

	// AESDecrypt(nCipherID, cCipherText, cKey, [cIV]) -> aResDec
	//   nCipherID: 0/1/2 (AES-128/192/256 CBC). Para CBC o cIV é obrigatório.
	//   Retorno: {nResultCode, cPlainText}.
	natives["AESDECRYPT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		keySize, ok := aesKeySize(advplrt.ToFloat(getArg(args, 0)))
		if !ok {
			return aesDecResult(1, ""), nil
		}
		ciphertext := []byte(getArgString(args, 1, ""))
		if len(ciphertext) == 0 {
			return aesDecResult(7, ""), nil
		}
		key := []byte(cryptoArgString(args, 2, ""))
		if len(key) != keySize {
			return aesDecResult(2, ""), nil
		}
		iv := []byte(cryptoArgString(args, 3, ""))
		if len(iv) != aes.BlockSize {
			return aesDecResult(3, ""), nil
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return aesDecResult(8, ""), nil
		}
		if len(ciphertext)%block.BlockSize() != 0 {
			return aesDecResult(9, ""), nil
		}
		decrypted := make([]byte, len(ciphertext))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, ciphertext)
		plain, err := pkcs7Unpad(decrypted)
		if err != nil {
			return aesDecResult(9, ""), nil
		}
		return aesDecResult(0, string(plain)), nil
	}

	// ARC4(cBase, cChave) -> cStringArc4
	//   Cifra de fluxo RC4. Obsoleta no Protheus; retorna os bytes cifrados em
	//   ASCII hexadecimal separados por hífen (ex.: "55-AB-39-45-24").
	natives["ARC4"] = func(args []advplrt.Value) (advplrt.Value, error) {
		out := rc4Crypt([]byte(getArgString(args, 1, "")), []byte(getArgString(args, 0, "")))
		parts := make([]string, len(out))
		for i, b := range out {
			parts[i] = fmt.Sprintf("%02X", b)
		}
		return advplrt.NewString(strings.Join(parts, "-")), nil
	}

	// RC4Crypt(cInput, cKey, [lIsReturnASCII], [lIsInputASCII]) -> cRet
	//   RC4 é simétrica: encripta e decripta com a mesma operação.
	//   lIsReturnASCII default .T.: retorno em hex (2 chars/byte, sem "0x").
	//   lIsInputASCII default .F.: quando .T., a entrada é decodificada de hex.
	natives["RC4CRYPT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Defaults AdvPL: retorno ASCII (.T.), entrada texto (.F.).
		input := getArgString(args, 0, "")
		key := getArgString(args, 1, "")
		lReturnASCII := true
		if len(args) > 2 {
			lReturnASCII = advplrt.ToBool(getArg(args, 2))
		}
		lInputASCII := false
		if len(args) > 3 {
			lInputASCII = advplrt.ToBool(getArg(args, 3))
		}

		data := []byte(input)
		if lInputASCII {
			decoded, err := hex.DecodeString(input)
			if err != nil {
				return advplrt.Nil, nil
			}
			data = decoded
		}
		out := rc4Crypt([]byte(key), data)
		if lReturnASCII {
			return advplrt.NewString(strings.ToUpper(hex.EncodeToString(out))), nil
		}
		return advplrt.NewString(string(out)), nil
	}

	// CRCCalc(nAlgoritmo, cInput, [@cRetHex]) -> nCRC
	//   Algoritmos CRC16: 2=None, 3=MODBUS, 4=SICK, 5=CCITT_XMODEM,
	//   6=CCITT_FFFF, 7=CCITT_1D0F, 8=CCITT_KERMIT, 9=DNP.
	//   Retorna o valor decimal do CRC. O parâmetro opcional @cRetHex (por
	//   referência) NÃO é propagado nesta VM embutida (argumentos @ não têm
	//   propagação de referência fora de arrays) — o valor hexadecimal pode
	//   ser obtido com StrZero / convertendo o retorno numérico.
	natives["CRCCALC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		algo := int(advplrt.ToFloat(getArg(args, 0)))
		data := []byte(getArgString(args, 1, ""))
		crc, ok := crc16ForAlgorithm(algo, data)
		if !ok {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(crc)), nil
	}

	// WebEncript(cContent, [lDecript], [lUseinjava]) -> cRet
	//   O algoritmo real da TOTVS é proprietário e não divulgado no TDN.
	//   Esta implementação é um substituto reversível e determinístico:
	//   encripta com XOR de chave fixa + Base64; decripta o inverso.
	//   Garante o roundtrip WebEncript(WebEncript(s,.F.),.T.) == s.
	natives["WEBENCRIPT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		content := getArgString(args, 0, "")
		lDecript := false
		if len(args) > 1 {
			lDecript = advplrt.ToBool(getArg(args, 1))
		}
		if lDecript {
			enc, err := base64.StdEncoding.DecodeString(content)
			if err != nil {
				return advplrt.NewString(""), nil
			}
			return advplrt.NewString(string(webEncriptXOR(enc))), nil
		}
		xored := webEncriptXOR([]byte(content))
		return advplrt.NewString(base64.StdEncoding.EncodeToString(xored)), nil
	}

	// Embaralha(cTexto, nTipo) -> cRet
	//   nTipo: 0 embaralha, 1 desembaralha.
	//   O algoritmo exato da TOTVS é proprietário (a permutação não é
	//   divulgada e não corresponde a cifras simples). Esta implementação é
	//   um scramble reversível determinístico (permutação dos caracteres:
	//   toma alternadamente do final e do início) que garante o roundtrip
	//   Embaralha(Embaralha(s,0),1) == s.
	natives["EMBARALHA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		text := []byte(getArgString(args, 0, ""))
		if int(advplrt.ToFloat(getArg(args, 1))) == 1 {
			return advplrt.NewString(string(unscrambleBytes(text))), nil
		}
		return advplrt.NewString(string(scrambleBytes(text))), nil
	}

	// =========================================================================
	// RSA — infraestrutura de chaves (stdlib crypto/rsa + crypto/x509).
	// =========================================================================

	// DecryptRSA(cKeyFile, cInfo) -> cRet
	//   Descriptografa cInfo (string binária, ou Base64 previamente
	//   decodificada via Decode64) usando a chave privada PEM em cKeyFile.
	//   Retorna Nil em caso de erro (ex.: arquivo inexistente ou chave inválida).
	natives["DECRYPTRSA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		pemData, ok := readKeyMaterial(getArgString(args, 0, ""))
		if !ok {
			return advplrt.Nil, nil
		}
		priv, err := parseRSAPrivateKey(pemData, "")
		if err != nil {
			return advplrt.Nil, nil
		}
		info := []byte(getArgString(args, 1, ""))
		plain, err := rsa.DecryptPKCS1v15(nil, priv, info)
		if err != nil {
			// fallback: OAEP (alguns pares são gerados com OAEP)
			if oaep, oerr := rsa.DecryptOAEP(sha256.New(), nil, priv, info, nil); oerr == nil {
				return advplrt.NewString(string(oaep)), nil
			}
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(plain)), nil
	}

	// RSAExponent(cKey, lPublic, [cPassword]) -> cRet
	// RSAModulus(cKey, lPublic, [cPassword]) -> cRet
	//   Retornam expoente / módulo públicos de uma chave PEM (path ou conteúdo)
	//   em string binária AdvPL (big-endian). Nil em caso de erro.
	natives["RSAEXPONENT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key, ok := rsaPublicKeyFromArgs(args)
		if !ok {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(bigEndianBytes(big.NewInt(int64(key.E))))), nil
	}
	natives["RSAMODULUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key, ok := rsaPublicKeyFromArgs(args)
		if !ok {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(bigEndianBytes(key.N))), nil
	}

	// PrivSignRSA(cKeyOrPathKey, cContent, nType, cSenha, [@cErrStr], [nPad]) -> cRet
	//   Assina o hash cContent (gerado antes, ex.: MD5()) com a chave privada
	//   PEM (path ou conteúdo). nType: 1=MD5, 2=SHA1, 3=RIPEMD160,
	//   4=MD5_SHA1, 5=SHA256WithRSA, 6=SHA256. nPad default 1=PKCS1
	//   (2=SSL é aproximado por PKCS1; 3=NO é RSA puro; 4/5 não suportados).
	//   Retorna a assinatura como string binária; Nil em caso de erro.
	natives["PRIVSIGNRSA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		keyOrPath := getArgString(args, 0, "")
		pemData, ok := readKeyMaterial(keyOrPath)
		if !ok {
			return advplrt.Nil, nil
		}
		priv, err := parseRSAPrivateKey(pemData, getArgString(args, 3, ""))
		if err != nil {
			return advplrt.Nil, nil
		}
		hf, err := privSignHashFunc(advplrt.ToFloat(getArg(args, 2)))
		if err != nil {
			return advplrt.Nil, nil
		}
		digest, err := privSignDigest(getArgString(args, 1, ""), hf)
		if err != nil {
			return advplrt.Nil, nil
		}
		pad := int(advplrt.ToFloat(getArg(args, 5)))
		if pad == 0 {
			pad = 1
		}
		switch pad {
		case 1, 2: // PKCS1 (2=SSL aproximado)
			sig, err := rsa.SignPKCS1v15(rand.Reader, priv, hf, digest)
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewString(string(sig)), nil
		case 3: // NO padding — RSA puro (m^d mod n)
			sig := rawRSASign(priv, digest)
			return advplrt.NewString(string(sig)), nil
		default:
			return advplrt.NewString(""), nil
		}
	}

	// PrivVeryRSA(cKeyOrPathKey, cContent, nType, cAssinatura, [@cErrStr], [nPad]) -> lRet
	//   Verifica a assinatura cAssinatura usando a chave pública PEM (path ou
	//   conteúdo). Retorna .T./.F.
	natives["PRIVVERYRSA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		keyOrPath := getArgString(args, 0, "")
		pemData, ok := readKeyMaterial(keyOrPath)
		if !ok {
			return advplrt.NewBool(false), nil
		}
		pub, err := parseRSAPublicKey(pemData)
		if err != nil {
			if priv, perr := parseRSAPrivateKey(pemData, getArgString(args, 4, "")); perr == nil {
				pub = &priv.PublicKey
			} else {
				return advplrt.NewBool(false), nil
			}
		}
		hf, err := privSignHashFunc(advplrt.ToFloat(getArg(args, 2)))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		digest, err := privSignDigest(getArgString(args, 1, ""), hf)
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		signature := []byte(getArgString(args, 3, ""))
		pad := int(advplrt.ToFloat(getArg(args, 5)))
		if pad == 0 {
			pad = 1
		}
		switch pad {
		case 1, 2:
			err := rsa.VerifyPKCS1v15(pub, hf, digest, signature)
			return advplrt.NewBool(err == nil), nil
		case 3:
			return advplrt.NewBool(rawRSAVerify(pub, digest, signature)), nil
		default:
			return advplrt.NewBool(false), nil
		}
	}

	// WriteRSAPK(cDERFile, cRSAFile, @cError) -> lRet
	//   Converte uma chave privada DER (PKCS#1 ou PKCS#8) para PEM "RSA PRIVATE
	//   KEY" gravando em cRSAFile. Retorna .T. em caso de sucesso. O parâmetro
	//   @cError não é propagado (limitação de referência da VM embutida).
	natives["WRITERSAPK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		der, err := os.ReadFile(getArgString(args, 0, ""))
		if err != nil {
			return advplrt.NewBool(false), nil
		}
		var priv *rsa.PrivateKey
		if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			priv = k
		} else if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
			var ok bool
			priv, ok = key.(*rsa.PrivateKey)
			if !ok {
				return advplrt.NewBool(false), nil
			}
		} else {
			return advplrt.NewBool(false), nil
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
		if err := os.WriteFile(getArgString(args, 1, ""), pemBytes, 0644); err != nil {
			return advplrt.NewBool(false), nil
		}
		return advplrt.NewBool(true), nil
	}

	// =========================================================================
	// HSM (Hardware Security Module) e HTTPSSLClient — stubs honestos.
	//
	// Estas funções dependem de infraestrutura de servidor Protheus (módulo
	// PKCS#11 / dispositivo HSM) que não existe nesta VM embutida. Em vez de
	// falhar a execução, retornam valores que não quebram programas de exemplo
	// (que tipicamente checam o retorno e abortam com graça):
	//   HSMInitialize/HSMFinalize -> .F. (sem HSM);
	//   HSMSlotList/HSMObjList    -> {} (nenhum slot/objeto);
	//   HSMModulus/HSMExponent    -> "" ;
	//   HSMGetCertFile/HSMGetKeyFile -> .F.;
	//   HSMPrivSign -> Nil; HSMPrivVery -> .F.
	// =========================================================================
	natives["HSMINITIALIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}
	natives["HSMFINALIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}
	natives["HSMSLOTLIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray(nil), nil
	}
	natives["HSMOBJLIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray(nil), nil
	}
	natives["HSMMODULUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}
	natives["HSMEXPONENT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}
	natives["HSMGETCERTFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}
	natives["HSMGETKEYFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}
	natives["HSMPRIVSIGN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil
	}
	natives["HSMPRIVVERY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}

	// HTTPSSLClient(...) -> lRet
	//   Define em memória as configurações de conexão SSL. Sem infraestrutura
	//   SSL/HSM do AppServer, é um no-op honesto retornando .F..
	natives["HTTPSSLCLIENT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(false), nil
	}
}

// ----------------------------------------------------------------------------
// Helpers internos (não exportados)
// ----------------------------------------------------------------------------

// digestResult converte um digest em string binária (nType==1) ou hexadecimal
// (nType==2 / default).
func digestResult(sum []byte, nType float64) advplrt.Value {
	if int(nType) == 1 {
		return advplrt.NewString(string(sum))
	}
	return advplrt.NewString(hex.EncodeToString(sum))
}

// hmacHashFunc mapeia o nCryptoType do HMAC para a função hash.
func hmacHashFunc(nCryptoType float64) (func() hash.Hash, error) {
	switch int(nCryptoType) {
	case 1:
		return md5.New, nil
	case 3:
		return sha1.New, nil
	case 5:
		return sha256.New, nil
	case 7:
		return sha512.New, nil
	}
	return nil, errors.New("algoritmo HMAC inválido")
}

// decodeCryptoInput decodifica conteúdo/chave do HMAC conforme o tipo:
// 1 = Texto, 2 = Base64, 3 = Hexadecimal.
func decodeCryptoInput(data string, kind float64) ([]byte, error) {
	switch int(kind) {
	case 2:
		return base64.StdEncoding.DecodeString(data)
	case 3:
		return hex.DecodeString(data)
	default:
		return []byte(data), nil
	}
}

// aesKeySize retorna o tamanho da chave AES conforme nCipherID
// (0=AES-128, 1=AES-192, 2=AES-256), todos CBC com IV de 16 bytes.
func aesKeySize(nCipherID float64) (int, bool) {
	switch int(nCipherID) {
	case 0:
		return 16, true
	case 1:
		return 24, true
	case 2:
		return 32, true
	}
	return 0, false
}

// deriveAESKey deriva a chave de criptografia a partir do password:
// primeiros size bytes de SHA-256(password) (determinístico).
func deriveAESKey(password string, size int) []byte {
	sum := sha256.Sum256([]byte(password))
	if len(sum) <= size {
		return sum[:]
	}
	return sum[:size]
}

// pkcs7Pad aplica o padding PKCS#7.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

// pkcs7Unpad remove o padding PKCS#7.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("dados vazios")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, errors.New("padding inválido")
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, errors.New("padding inválido")
		}
	}
	return data[:len(data)-padLen], nil
}

// aesEncResult monta o array de retorno do AESEncrypt.
func aesEncResult(code int, cipherText, key, iv string) advplrt.Value {
	return advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(float64(code)),
		advplrt.NewString(cipherText),
		advplrt.NewString(key),
		advplrt.NewString(iv),
	})
}

// aesDecResult monta o array de retorno do AESDecrypt.
func aesDecResult(code int, plain string) advplrt.Value {
	return advplrt.NewArray([]advplrt.Value{
		advplrt.NewNumber(float64(code)),
		advplrt.NewString(plain),
	})
}

// rc4Crypt implementa a cifra de fluxo RC4 (KSA + PRGA). RC4 é simétrica:
// encriptar e decriptar usam a mesma operação.
func rc4Crypt(key, data []byte) []byte {
	if len(key) == 0 {
		return append([]byte(nil), data...)
	}
	s := make([]byte, 256)
	for i := 0; i < 256; i++ {
		s[i] = byte(i)
	}
	j := 0
	for i := 0; i < 256; i++ {
		j = (j + int(s[i]) + int(key[i%len(key)])) % 256
		s[i], s[j] = s[j], s[i]
	}
	out := make([]byte, len(data))
	i := 0
	j = 0
	for k := range data {
		i = (i + 1) % 256
		j = (j + int(s[i])) % 256
		s[i], s[j] = s[j], s[i]
		out[k] = data[k] ^ s[(int(s[i])+int(s[j]))%256]
	}
	return out
}

// crc16ForAlgorithm calcula o CRC16 conforme o nAlgoritmo do CRCCalc
// (tabela do TDN): 2=None, 3=MODBUS, 4=SICK, 5=CCITT_XMODEM, 6=CCITT_FFFF,
// 7=CCITT_1D0F, 8=CCITT_KERMIT, 9=DNP.
func crc16ForAlgorithm(algo int, data []byte) (uint16, bool) {
	switch algo {
	case 2: // CRC16_None — 0x8005, init 0, sem reflexão, sem XOR
		return crc16Generic(0x8005, 0x0000, 0x0000, false, false, data), true
	case 3: // CRC16_MODBUS
		return crc16Generic(0x8005, 0xFFFF, 0x0000, true, true, data), true
	case 4: // CRC16_SICK — 0x8005, init 0, sem reflexão, sem XOR
		return crc16Generic(0x8005, 0x0000, 0x0000, false, false, data), true
	case 5: // CRC16_CCITT_XMODEM
		return crc16Generic(0x1021, 0x0000, 0x0000, false, false, data), true
	case 6: // CRC16_CCITT_FFFF (CCITT-FALSE)
		return crc16Generic(0x1021, 0xFFFF, 0x0000, false, false, data), true
	case 7: // CRC16_CCITT_1D0F (AUG-CCITT)
		return crc16Generic(0x1021, 0x1D0F, 0x0000, false, false, data), true
	case 8: // CRC16_CCITT_KERMIT
		return crc16Generic(0x1021, 0x0000, 0x0000, true, true, data), true
	case 9: // CRC16_DNP
		return crc16Generic(0x3D65, 0x0000, 0xFFFF, true, true, data), true
	}
	return 0, false
}

// crc16Generic implementa CRC16 bit a bit. Os parâmetros usam a notação
// "normal" (não refletida) do polinômio. O resultado já sai na ordem de bits
// correta do algoritmo — não há reflexão final adicional.
func crc16Generic(poly, init, xorout uint16, refin, refout bool, data []byte) uint16 {
	_ = refout
	crc := init
	if refin {
		rpoly := reflect16(poly)
		for _, b := range data {
			crc ^= uint16(b)
			for i := 0; i < 8; i++ {
				if crc&1 != 0 {
					crc = (crc >> 1) ^ rpoly
				} else {
					crc >>= 1
				}
			}
		}
	} else {
		for _, b := range data {
			crc ^= uint16(b) << 8
			for i := 0; i < 8; i++ {
				if crc&0x8000 != 0 {
					crc = (crc << 1) ^ poly
				} else {
					crc <<= 1
				}
			}
		}
	}
	return crc ^ xorout
}

// reflect16 inverte a ordem dos 16 bits (usado para polinômios refletidos).
func reflect16(v uint16) uint16 {
	var r uint16
	for i := 0; i < 16; i++ {
		r = (r << 1) | (v & 1)
		v >>= 1
	}
	return r
}

// webEncriptXOR aplica/reverte o XOR de chave fixa usado pelo WebEncript.
func webEncriptXOR(data []byte) []byte {
	const key = "totvs-web-encript-key"
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ key[i%len(key)]
	}
	return out
}

// scrambleBytes embaralha tomando alternadamente caracteres do final e do
// início da string.
func scrambleBytes(in []byte) []byte {
	n := len(in)
	out := make([]byte, 0, n)
	lo, hi := 0, n-1
	for lo < hi {
		out = append(out, in[hi], in[lo])
		lo++
		hi--
	}
	if lo == hi {
		out = append(out, in[lo])
	}
	return out
}

// unscrambleBytes desfaz a permutação aplicada por scrambleBytes.
func unscrambleBytes(in []byte) []byte {
	n := len(in)
	order := make([]int, 0, n)
	lo, hi := 0, n-1
	for lo < hi {
		order = append(order, hi, lo)
		lo++
		hi--
	}
	if lo == hi {
		order = append(order, lo)
	}
	out := make([]byte, n)
	for i, pos := range order {
		out[pos] = in[i]
	}
	return out
}

// cryptoArgString lê um argumento string tratando valor ausente ou NilValue
// como a string default (evita o literal "Nil" do advplrt.ToString).
func cryptoArgString(args []advplrt.Value, idx int, def string) string {
	if idx >= len(args) || advplrt.IsNil(getArg(args, idx)) {
		return def
	}
	return advplrt.ToString(getArg(args, idx))
}

// readKeyMaterial retorna os bytes PEM de uma chave. Se cKey parecer conteúdo
// PEM ("-----BEGIN"), é usado diretamente; senão é tratado como path de arquivo.
func readKeyMaterial(cKey string) ([]byte, bool) {
	if strings.Contains(cKey, "-----BEGIN") {
		return []byte(cKey), true
	}
	data, err := os.ReadFile(cKey)
	if err != nil {
		return nil, false
	}
	return data, true
}

// parseRSAPrivateKey analisa uma chave privada RSA em PEM (PKCS#1, PKCS#8 ou
// PKCS#1 criptografada com password).
func parseRSAPrivateKey(pemBytes []byte, password string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("PEM inválido")
	}
	der := block.Bytes
	if x509.IsEncryptedPEMBlock(block) {
		decrypted, err := x509.DecryptPEMBlock(block, []byte(password))
		if err != nil {
			return nil, err
		}
		der = decrypted
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if rk, ok := key.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("não é chave RSA")
	}
	return nil, errors.New("chave privada não suportada")
}

// parseRSAPublicKey analisa uma chave pública RSA em PEM (PKIX "PUBLIC KEY",
// PKCS#1 "RSA PUBLIC KEY" ou certificado X.509).
func parseRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("PEM inválido")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := key.(*rsa.PublicKey); ok {
			return rk, nil
		}
		return nil, errors.New("não é chave pública RSA")
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if rk, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return rk, nil
		}
	}
	return nil, errors.New("chave pública não suportada")
}

// rsaPublicKeyFromArgs extrai a chave pública de uma chave RSA (path ou PEM),
// considerando o parâmetro lPublic (public vs private) da chamada.
func rsaPublicKeyFromArgs(args []advplrt.Value) (*rsa.PublicKey, bool) {
	keyOrPath := getArgString(args, 0, "")
	pemData, ok := readKeyMaterial(keyOrPath)
	if !ok {
		return nil, false
	}
	if advplrt.ToBool(getArg(args, 1)) {
		pub, err := parseRSAPublicKey(pemData)
		if err != nil {
			if priv, perr := parseRSAPrivateKey(pemData, getArgString(args, 2, "")); perr == nil {
				return &priv.PublicKey, true
			}
			return nil, false
		}
		return pub, true
	}
	priv, err := parseRSAPrivateKey(pemData, getArgString(args, 2, ""))
	if err != nil {
		return nil, false
	}
	return &priv.PublicKey, true
}

// privSignHashFunc mapeia o nType do PrivSignRSA/PrivVeryRSA para crypto.Hash.
func privSignHashFunc(nType float64) (crypto.Hash, error) {
	switch int(nType) {
	case 1:
		return crypto.MD5, nil
	case 2:
		return crypto.SHA1, nil
	case 3: // RIPEMD160 fora da stdlib
		return crypto.Hash(0), errors.New("RIPEMD160 não suportado")
	case 4: // MD5_SHA1 — aproximado por SHA1 nesta implementação
		return crypto.SHA1, nil
	case 5, 6:
		return crypto.SHA256, nil
	}
	return crypto.Hash(0), errors.New("tipo de hash inválido")
}

// privSignDigest normaliza o cContent para o digest a assinar: se for hex
// válido com o tamanho exato do hash, é usado diretamente; senão o conteúdo é
// hasheado com o algoritmo informado.
func privSignDigest(content string, ch crypto.Hash) ([]byte, error) {
	if d, err := hex.DecodeString(content); err == nil && len(d) == ch.Size() {
		return d, nil
	}
	h := ch.New()
	h.Write([]byte(content))
	return h.Sum(nil), nil
}

// rawRSASign implementa assinatura RSA sem padding (nPad=3): sig = m^d mod n.
func rawRSASign(priv *rsa.PrivateKey, digest []byte) []byte {
	m := new(big.Int).SetBytes(digest)
	if m.Cmp(priv.N) >= 0 {
		m.Mod(m, priv.N)
	}
	sig := new(big.Int).Exp(m, priv.D, priv.N)
	return sig.Bytes()
}

// rawRSAVerify implementa verificação RSA sem padding: compara m == sig^e mod n.
func rawRSAVerify(pub *rsa.PublicKey, digest, signature []byte) bool {
	sig := new(big.Int).SetBytes(signature)
	if sig.Cmp(pub.N) >= 0 {
		return false
	}
	m := new(big.Int).Exp(sig, big.NewInt(int64(pub.E)), pub.N)
	return m.Cmp(new(big.Int).SetBytes(digest)) == 0
}

// bigEndianBytes serializa um inteiro grande em bytes big-endian (mínimos).
func bigEndianBytes(n *big.Int) []byte {
	return n.Bytes()
}
