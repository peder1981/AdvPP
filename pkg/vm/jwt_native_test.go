package vm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestJWTHS256CreateAndVerify(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	obj := newJWTObject()

	if err := v.callTJWTMethod(obj, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("secret")}); err != nil {
		t.Fatalf("SetSecretKey: %v", err)
	}
	if err := v.callTJWTMethod(obj, "SETISSUER", []advplrt.Value{advplrt.NewString("auth0")}); err != nil {
		t.Fatalf("SetIssuer: %v", err)
	}
	if err := v.callTJWTMethod(obj, "SETEXPIRESAT", []advplrt.Value{advplrt.NewNumber(3600)}); err != nil {
		t.Fatalf("SetExpiresAt: %v", err)
	}

	claims := advplrt.NewObject("JsonObject", nil)
	claims.SetProp("idade", advplrt.NewNumber(10))
	claims.SetProp("root", advplrt.NewBool(true))
	claims.SetProp("dir", advplrt.NewArray([]advplrt.Value{
		advplrt.NewString("home"), advplrt.NewString("user"), advplrt.NewNumber(10),
	}))
	if err := v.callTJWTMethod(obj, "SETPAYLOADCLAIM", []advplrt.Value{claims}); err != nil {
		t.Fatalf("SetPayloadClaim: %v", err)
	}

	if err := v.callTJWTMethod(obj, "CREATETOKEN", []advplrt.Value{advplrt.NewString("hs256")}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	tokenVal := v.pop()
	token := advplrt.ToString(tokenVal)
	if token == "" {
		st := obj.Native.(*jwtState)
		t.Fatalf("CreateToken retornou token vazio, lastErr=%q", st.lastErr)
	}

	// Verifica com um segundo objeto, como no exemplo da TDN (setToken +
	// setSecretKey + verifyToken), garantindo que o token é autocontido.
	obj2 := newJWTObject()
	_ = v.callTJWTMethod(obj2, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("secret")})
	_ = v.callTJWTMethod(obj2, "SETTOKEN", []advplrt.Value{advplrt.NewString(token)})

	if !advplrt.ToBool(mustCall(t, v, obj2, "HASISSUER", nil)) {
		t.Error("HasIssuer() deveria ser .T. após SetToken")
	}
	if !advplrt.ToBool(mustCall(t, v, obj2, "HASPAYLOADCLAIM", []advplrt.Value{advplrt.NewString("root")})) {
		t.Error("HasPayloadClaim(\"root\") deveria ser .T.")
	}

	_ = v.callTJWTMethod(obj2, "WITHISSUER", []advplrt.Value{advplrt.NewString("auth0")})
	ret := mustCall(t, v, obj2, "VERIFYTOKEN", []advplrt.Value{advplrt.NewString("hs256")})
	if !advplrt.ToBool(ret) {
		st := obj2.Native.(*jwtState)
		t.Fatalf("VerifyToken deveria ser .T., lastErr=%q", st.lastErr)
	}

	// Issuer errado deve falhar.
	obj3 := newJWTObject()
	_ = v.callTJWTMethod(obj3, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("secret")})
	_ = v.callTJWTMethod(obj3, "SETTOKEN", []advplrt.Value{advplrt.NewString(token)})
	_ = v.callTJWTMethod(obj3, "WITHISSUER", []advplrt.Value{advplrt.NewString("outro")})
	ret3 := mustCall(t, v, obj3, "VERIFYTOKEN", []advplrt.Value{advplrt.NewString("hs256")})
	if advplrt.ToBool(ret3) {
		t.Error("VerifyToken com issuer incorreto deveria ser .F.")
	}

	// Secret errado deve falhar (assinatura inválida).
	obj4 := newJWTObject()
	_ = v.callTJWTMethod(obj4, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("outrosegredo")})
	_ = v.callTJWTMethod(obj4, "SETTOKEN", []advplrt.Value{advplrt.NewString(token)})
	ret4 := mustCall(t, v, obj4, "VERIFYTOKEN", []advplrt.Value{advplrt.NewString("hs256")})
	if advplrt.ToBool(ret4) {
		t.Error("VerifyToken com secret errado deveria ser .F.")
	}
}

func TestJWTRS256CreateAndVerify(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	v := NewVM(&compiler.Bytecode{}, false)
	obj := newJWTObject()
	_ = v.callTJWTMethod(obj, "SETPRIVKEY", []advplrt.Value{advplrt.NewString(string(privPEM))})
	_ = v.callTJWTMethod(obj, "SETSUBJECT", []advplrt.Value{advplrt.NewString("1234567890")})

	if err := v.callTJWTMethod(obj, "CREATETOKEN", []advplrt.Value{advplrt.NewString("RS256")}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	token := advplrt.ToString(v.pop())
	if token == "" {
		st := obj.Native.(*jwtState)
		t.Fatalf("token vazio, lastErr=%q", st.lastErr)
	}

	obj2 := newJWTObject()
	_ = v.callTJWTMethod(obj2, "SETPUBKEY", []advplrt.Value{advplrt.NewString(string(pubPEM))})
	_ = v.callTJWTMethod(obj2, "SETTOKEN", []advplrt.Value{advplrt.NewString(token)})
	ret := mustCall(t, v, obj2, "VERIFYTOKEN", []advplrt.Value{advplrt.NewString("RS256")})
	if !advplrt.ToBool(ret) {
		st := obj2.Native.(*jwtState)
		t.Fatalf("VerifyToken RS256 deveria ser .T., lastErr=%q", st.lastErr)
	}
}

func TestJWTExpiredToken(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	obj := newJWTObject()
	_ = v.callTJWTMethod(obj, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("s")})
	_ = v.callTJWTMethod(obj, "SETEXPIRESAT", []advplrt.Value{advplrt.NewNumber(-10)}) // já expirado
	_ = v.callTJWTMethod(obj, "CREATETOKEN", []advplrt.Value{advplrt.NewString("HS256")})
	token := advplrt.ToString(v.pop())

	obj2 := newJWTObject()
	_ = v.callTJWTMethod(obj2, "SETSECRETKEY", []advplrt.Value{advplrt.NewString("s")})
	_ = v.callTJWTMethod(obj2, "SETTOKEN", []advplrt.Value{advplrt.NewString(token)})
	ret := mustCall(t, v, obj2, "VERIFYTOKEN", []advplrt.Value{advplrt.NewString("HS256")})
	if advplrt.ToBool(ret) {
		t.Error("token expirado deveria falhar na verificação")
	}
}

func TestJWTEncodeDecodeTime(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	obj := newJWTObject()

	ret := mustCall(t, v, obj, "ENCODETIME", []advplrt.Value{advplrt.NewString("2020/02/13"), advplrt.NewString("20:10:35")})
	secs := advplrt.ToFloat(ret)
	if secs == 0 {
		t.Fatal("EncodeTime retornou 0 para data válida")
	}

	back := mustCall(t, v, obj, "DECODETIME", []advplrt.Value{advplrt.NewNumber(secs)})
	if advplrt.ToString(back) != "2020/02/13 20:10:35" {
		t.Errorf("DecodeTime(EncodeTime(...)) = %q, quer 2020/02/13 20:10:35", advplrt.ToString(back))
	}

	// Passar um número direto é passthrough (já é "segundos").
	ret2 := mustCall(t, v, obj, "ENCODETIME", []advplrt.Value{advplrt.NewNumber(300)})
	if advplrt.ToFloat(ret2) != 300 {
		t.Errorf("EncodeTime(300) = %v, quer 300 (passthrough)", ret2)
	}
}

func mustCall(t *testing.T, v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) advplrt.Value {
	t.Helper()
	if err := v.callTJWTMethod(obj, method, args); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return v.pop()
}
