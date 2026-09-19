package vm

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// jwtState é o estado Go da classe tJWT (TDN: Classes/Componentes/Nao-Visual/tJWT,
// RFC 7519). Header e Payload ficam em map[string]interface{} (não em
// advplrt.Value) porque precisam ser serializados em JSON byte-a-byte
// idêntico entre assinatura e verificação, e porque setToken() precisa
// popular as mesmas estruturas a partir de um token de terceiros via
// encoding/json puro.
type jwtState struct {
	header  map[string]interface{}
	payload map[string]interface{}

	pubKey    string
	privKey   string
	secretKey string

	token   string
	lastErr string

	withIssuer      string
	hasWithIssuer   bool
	withSubject     string
	hasWithSubject  bool
	withID          string
	hasWithID       bool
	withAudience    []string
	hasWithAudience bool
	withClaims      map[string]interface{}
}

func newJWTObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TJWT", nil)
	obj.Native = &jwtState{
		header:     map[string]interface{}{},
		payload:    map[string]interface{}{},
		withClaims: map[string]interface{}{},
	}
	obj.SetProp("TOKEN", advplrt.NewString(""))
	return obj
}

func jwtHasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

// advplToGo converte um Value AdvPL para o interface{} equivalente que
// encoding/json produziria ao decodificar o mesmo dado — necessário para que
// claims montadas via JsonObject (setPayloadClaim/setHeaderClaim/withClaim)
// comparem igual (reflect.DeepEqual) às claims decodificadas de um token real.
func advplToGo(val advplrt.Value) interface{} {
	if val == nil || val == advplrt.Nil {
		return nil
	}
	switch t := val.(type) {
	case *advplrt.StringValue:
		return t.Val
	case *advplrt.NumberValue:
		return t.Val
	case *advplrt.BoolValue:
		return t.Val
	case *advplrt.NilValue:
		return nil
	case *advplrt.ArrayValue:
		out := make([]interface{}, len(t.Elements))
		for i, e := range t.Elements {
			out[i] = advplToGo(e)
		}
		return out
	case *advplrt.ObjectValue:
		out := make(map[string]interface{}, len(t.Keys))
		for _, k := range t.Keys {
			out[k] = advplToGo(t.Props[k])
		}
		return out
	default:
		return advplrt.ToString(val)
	}
}

// jwtMergeClaims copia as propriedades de um JsonObject (ou objeto
// compatível) para dentro do map de claims (header ou payload).
func jwtMergeClaims(dst map[string]interface{}, val advplrt.Value) {
	obj, ok := val.(*advplrt.ObjectValue)
	if !ok {
		return
	}
	for _, k := range obj.Keys {
		dst[k] = advplToGo(obj.Props[k])
	}
}

func jwtDecodeToken(token string) (header, payload map[string]interface{}, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("malformed JWT (esperado 3 segmentos)")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, errors.New("header com base64url inválido")
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, errors.New("payload com base64url inválido")
	}
	header = map[string]interface{}{}
	payload = map[string]interface{}{}
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, nil, errors.New("header não é JSON válido")
	}
	if err := json.Unmarshal(pb, &payload); err != nil {
		return nil, nil, errors.New("payload não é JSON válido")
	}
	return header, payload, nil
}

func jwtParseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("chave privada RSA: PEM inválido")
	}
	key, err := parseAnyPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("chave privada informada não é RSA")
	}
	return rk, nil
}

func jwtParseECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("chave privada EC: PEM inválido")
	}
	key, err := parseAnyPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("chave privada informada não é EC")
	}
	return ek, nil
}

func jwtParseECPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("chave pública EC: PEM inválido")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if ek, ok := key.(*ecdsa.PublicKey); ok {
			return ek, nil
		}
	}
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		if ek, ok := cert.PublicKey.(*ecdsa.PublicKey); ok {
			return ek, nil
		}
	}
	return nil, errors.New("chave pública informada não é EC")
}

// jwtEncodeECDSASig serializa (r,s) no formato "raw R||S" fixo que a JWA
// (RFC 7518 §3.4) exige para ES256 — diferente do DER que crypto/ecdsa usaria
// por padrão em outros contextos.
func jwtEncodeECDSASig(r, s *big.Int, size int) []byte {
	out := make([]byte, 2*size)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(out[size-len(rb):size], rb)
	copy(out[2*size-len(sb):], sb)
	return out
}

func jwtSign(alg string, signingInput []byte, st *jwtState) ([]byte, error) {
	switch alg {
	case "HS256":
		if st.secretKey == "" {
			return nil, errors.New("secret key não definida (setSecretKey)")
		}
		mac := hmac.New(sha256.New, []byte(st.secretKey))
		mac.Write(signingInput)
		return mac.Sum(nil), nil

	case "RS256", "RS512":
		priv, err := jwtParseRSAPrivateKey(st.privKey)
		if err != nil {
			return nil, err
		}
		h, hf := sha256.New(), crypto.SHA256
		if alg == "RS512" {
			h, hf = sha512.New(), crypto.SHA512
		}
		h.Write(signingInput)
		return rsa.SignPKCS1v15(rand.Reader, priv, hf, h.Sum(nil))

	case "PS256", "PS384", "PS512":
		priv, err := jwtParseRSAPrivateKey(st.privKey)
		if err != nil {
			return nil, err
		}
		h, hf := pssHash(alg)
		h.Write(signingInput)
		return rsa.SignPSS(rand.Reader, priv, hf, h.Sum(nil), &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: hf})

	case "ES256":
		priv, err := jwtParseECPrivateKey(st.privKey)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(signingInput)
		r, s, err := ecdsa.Sign(rand.Reader, priv, sum[:])
		if err != nil {
			return nil, err
		}
		return jwtEncodeECDSASig(r, s, 32), nil

	default:
		return nil, fmt.Errorf("algoritmo não suportado: %s", alg)
	}
}

func jwtVerifySig(alg string, signingInput, sig []byte, st *jwtState) error {
	switch alg {
	case "HS256":
		if st.secretKey == "" {
			return errors.New("secret key não definida (setSecretKey)")
		}
		mac := hmac.New(sha256.New, []byte(st.secretKey))
		mac.Write(signingInput)
		if !hmac.Equal(mac.Sum(nil), sig) {
			return errors.New("assinatura inválida")
		}
		return nil

	case "RS256", "RS512":
		pub, err := parseRSAPublicKey([]byte(st.pubKey))
		if err != nil {
			return err
		}
		h, hf := sha256.New(), crypto.SHA256
		if alg == "RS512" {
			h, hf = sha512.New(), crypto.SHA512
		}
		h.Write(signingInput)
		return rsa.VerifyPKCS1v15(pub, hf, h.Sum(nil), sig)

	case "PS256", "PS384", "PS512":
		pub, err := parseRSAPublicKey([]byte(st.pubKey))
		if err != nil {
			return err
		}
		h, hf := pssHash(alg)
		h.Write(signingInput)
		return rsa.VerifyPSS(pub, hf, h.Sum(nil), sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: hf})

	case "ES256":
		pub, err := jwtParseECPublicKey(st.pubKey)
		if err != nil {
			return err
		}
		if len(sig) != 64 {
			return errors.New("assinatura ES256 com tamanho inválido")
		}
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		sum := sha256.Sum256(signingInput)
		if !ecdsa.Verify(pub, sum[:], r, s) {
			return errors.New("assinatura inválida")
		}
		return nil

	default:
		return fmt.Errorf("algoritmo não suportado: %s", alg)
	}
}

func pssHash(alg string) (hash.Hash, crypto.Hash) {
	switch alg {
	case "PS384":
		return sha512.New384(), crypto.SHA384
	case "PS512":
		return sha512.New(), crypto.SHA512
	default: // PS256
		return sha256.New(), crypto.SHA256
	}
}

// jwtAudienceContains verifica se todos os valores de want estão contidos na
// claim "aud" do payload (aud pode ser string única ou array — RFC 7519 §4.1.3).
func jwtAudienceContains(payload map[string]interface{}, want []string) bool {
	raw, ok := payload["aud"]
	if !ok {
		return len(want) == 0
	}
	present := map[string]bool{}
	switch t := raw.(type) {
	case string:
		present[t] = true
	case []interface{}:
		for _, e := range t {
			if s, ok := e.(string); ok {
				present[s] = true
			}
		}
	}
	for _, w := range want {
		if !present[w] {
			return false
		}
	}
	return true
}

func (v *VM) callTJWTMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*jwtState)
	if !ok {
		return fmt.Errorf("tJWT: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		v.push(obj)
		return nil

	// --- Header ---
	case "SETALGORITHM":
		st.header["alg"] = strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		v.push(advplrt.Nil)
		return nil
	case "SETTYPE":
		st.header["typ"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETCONTENTTYPE":
		st.header["cty"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETKEYID":
		st.header["kid"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETHEADERCLAIM":
		jwtMergeClaims(st.header, getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "HASALGORITHM":
		v.push(advplrt.NewBool(jwtHasKey(st.header, "alg")))
		return nil
	case "HASTYPE":
		v.push(advplrt.NewBool(jwtHasKey(st.header, "typ")))
		return nil
	case "HASCONTENTTYPE":
		v.push(advplrt.NewBool(jwtHasKey(st.header, "cty")))
		return nil
	case "HASKEYID":
		v.push(advplrt.NewBool(jwtHasKey(st.header, "kid")))
		return nil
	case "HASHEADERCLAIM":
		v.push(advplrt.NewBool(jwtHasKey(st.header, advplrt.ToString(getArg(args, 0)))))
		return nil

	// --- Payload (claims reservadas) ---
	case "SETISSUER":
		st.payload["iss"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETSUBJECT":
		st.payload["sub"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETID":
		st.payload["jti"] = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETAUDIENCE":
		st.payload["aud"] = jwtAudienceValue(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETEXPIRESAT":
		// ponytail: TDN documenta nSec como duração relativa ("configurado
		// para 24horas... setExpiresAt(86400)"); a âncora ("agora") é o
		// instante desta chamada, não o de createToken — diferença
		// irrelevante no uso real (mesmo fluxo de chamadas).
		st.payload["exp"] = float64(time.Now().Unix()) + toNumber(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETNOTBEFORE":
		st.payload["nbf"] = float64(time.Now().Unix()) + toNumber(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETISSUEDAT":
		// TDN: valor absoluto em epoch (ex. produzido por encodeTime), ao
		// contrário de setExpiresAt/setNotBefore que são duração.
		st.payload["iat"] = toNumber(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETPAYLOADCLAIM":
		jwtMergeClaims(st.payload, getArg(args, 0))
		v.push(advplrt.Nil)
		return nil

	case "HASISSUER":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "iss")))
		return nil
	case "HASSUBJECT":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "sub")))
		return nil
	case "HASAUDIENCE":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "aud")))
		return nil
	case "HASEXPIRESAT":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "exp")))
		return nil
	case "HASNOTBEFORE":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "nbf")))
		return nil
	case "HASISSUEDAT":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "iat")))
		return nil
	case "HASID":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, "jti")))
		return nil
	case "HASPAYLOADCLAIM":
		v.push(advplrt.NewBool(jwtHasKey(st.payload, advplrt.ToString(getArg(args, 0)))))
		return nil

	// --- Verificação (with*) ---
	case "WITHISSUER":
		st.withIssuer = advplrt.ToString(getArg(args, 0))
		st.hasWithIssuer = true
		v.push(advplrt.Nil)
		return nil
	case "WITHSUBJECT":
		st.withSubject = advplrt.ToString(getArg(args, 0))
		st.hasWithSubject = true
		v.push(advplrt.Nil)
		return nil
	case "WITHID":
		st.withID = advplrt.ToString(getArg(args, 0))
		st.hasWithID = true
		v.push(advplrt.Nil)
		return nil
	case "WITHAUDIENCE":
		st.withAudience = jwtStringList(getArg(args, 0))
		st.hasWithAudience = true
		v.push(advplrt.Nil)
		return nil
	case "WITHCLAIM":
		jwtMergeClaims(st.withClaims, getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "CLEARWITHCLAIM":
		st.withClaims = map[string]interface{}{}
		v.push(advplrt.Nil)
		return nil

	// --- Chaves ---
	case "SETPUBKEY":
		st.pubKey = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETPRIVKEY":
		st.privKey = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil
	case "SETSECRETKEY":
		st.secretKey = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil

	// --- Token ---
	case "SETTOKEN":
		st.token = advplrt.ToString(getArg(args, 0))
		if hdr, pl, err := jwtDecodeToken(st.token); err == nil {
			st.header, st.payload, st.lastErr = hdr, pl, ""
		} else {
			st.lastErr = err.Error()
		}
		v.push(advplrt.Nil)
		return nil

	case "CREATETOKEN":
		alg := strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		st.header["alg"] = alg
		if !jwtHasKey(st.payload, "iat") {
			st.payload["iat"] = float64(time.Now().Unix())
		}
		hb, err := json.Marshal(st.header)
		if err != nil {
			st.lastErr = err.Error()
			v.push(advplrt.NewString(""))
			return nil
		}
		pb, err := json.Marshal(st.payload)
		if err != nil {
			st.lastErr = err.Error()
			v.push(advplrt.NewString(""))
			return nil
		}
		signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
		sig, err := jwtSign(alg, []byte(signingInput), st)
		if err != nil {
			st.lastErr = err.Error()
			v.push(advplrt.NewString(""))
			return nil
		}
		token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
		st.token, st.lastErr = token, ""
		obj.SetProp("TOKEN", advplrt.NewString(token))
		v.push(advplrt.NewString(token))
		return nil

	case "VERIFYTOKEN":
		alg := strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		if st.token == "" {
			st.lastErr = "nenhum token definido (setToken)"
			v.push(advplrt.NewBool(false))
			return nil
		}
		parts := strings.Split(st.token, ".")
		if len(parts) != 3 {
			st.lastErr = "token malformado"
			v.push(advplrt.NewBool(false))
			return nil
		}
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			st.lastErr = "assinatura com base64url inválido"
			v.push(advplrt.NewBool(false))
			return nil
		}
		signingInput := parts[0] + "." + parts[1]
		if err := jwtVerifySig(alg, []byte(signingInput), sig, st); err != nil {
			st.lastErr = err.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		hdr, pl, err := jwtDecodeToken(st.token)
		if err != nil {
			st.lastErr = err.Error()
			v.push(advplrt.NewBool(false))
			return nil
		}
		st.header, st.payload = hdr, pl

		now := float64(time.Now().Unix())
		if exp, ok := st.payload["exp"].(float64); ok && now > exp {
			st.lastErr = "token expirado (exp)"
			v.push(advplrt.NewBool(false))
			return nil
		}
		if nbf, ok := st.payload["nbf"].(float64); ok && now < nbf {
			st.lastErr = "token ainda não válido (nbf)"
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.hasWithIssuer && advplrt.ToString(jwtToValue(st.payload["iss"])) != st.withIssuer {
			st.lastErr = "issuer não confere"
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.hasWithSubject && advplrt.ToString(jwtToValue(st.payload["sub"])) != st.withSubject {
			st.lastErr = "subject não confere"
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.hasWithID && advplrt.ToString(jwtToValue(st.payload["jti"])) != st.withID {
			st.lastErr = "id (jti) não confere"
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.hasWithAudience && !jwtAudienceContains(st.payload, st.withAudience) {
			st.lastErr = "audience não confere"
			v.push(advplrt.NewBool(false))
			return nil
		}
		for k, want := range st.withClaims {
			if got, ok := st.payload[k]; !ok || !jwtDeepEqual(got, want) {
				st.lastErr = "claim não confere: " + k
				v.push(advplrt.NewBool(false))
				return nil
			}
		}
		st.lastErr = ""
		v.push(advplrt.NewBool(true))
		return nil

	case "GETLASTERROR":
		v.push(advplrt.NewString(st.lastErr))
		return nil

	// --- Tempo ---
	case "ENCODETIME":
		a0 := getArg(args, 0)
		if n, ok := a0.(*advplrt.NumberValue); ok {
			v.push(advplrt.NewNumber(n.Val))
			return nil
		}
		cData := advplrt.ToString(a0)
		cHora := getArgString(args, 1, "00:00:00")
		t, err := time.ParseInLocation("2006/01/02 15:04:05", cData+" "+cHora, time.Local)
		if err != nil {
			st.lastErr = "data/hora inválida: " + err.Error()
			v.push(advplrt.NewNumber(0))
			return nil
		}
		v.push(advplrt.NewNumber(float64(t.Unix())))
		return nil

	case "DECODETIME":
		nSec := int64(toNumber(getArg(args, 0)))
		t := time.Unix(nSec, 0).In(time.Local)
		v.push(advplrt.NewString(t.Format("2006/01/02 15:04:05")))
		return nil

	default:
		return fmt.Errorf("unknown method %s on tJWT", method)
	}
}

// jwtAudienceValue implementa setAudience: array vira lista de strings
// (elementos não-string são descartados, per TDN); qualquer outro tipo vira
// uma única string.
func jwtAudienceValue(val advplrt.Value) interface{} {
	if arr, ok := val.(*advplrt.ArrayValue); ok {
		var list []interface{}
		for _, e := range arr.Elements {
			if s, ok := e.(*advplrt.StringValue); ok {
				list = append(list, s.Val)
			}
		}
		return list
	}
	return advplrt.ToString(val)
}

func jwtStringList(val advplrt.Value) []string {
	if arr, ok := val.(*advplrt.ArrayValue); ok {
		var list []string
		for _, e := range arr.Elements {
			if s, ok := e.(*advplrt.StringValue); ok {
				list = append(list, s.Val)
			}
		}
		return list
	}
	return []string{advplrt.ToString(val)}
}

// jwtToValue devolve "" para nil (claim ausente), preservando o tipo Go
// original nos demais casos — usado só para comparações textuais (iss/sub/jti).
func jwtToValue(v interface{}) advplrt.Value {
	if v == nil {
		return advplrt.NewString("")
	}
	if s, ok := v.(string); ok {
		return advplrt.NewString(s)
	}
	return advplrt.NewString(fmt.Sprintf("%v", v))
}

func jwtDeepEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}
