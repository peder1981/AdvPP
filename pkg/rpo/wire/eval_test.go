package wire

import "testing"

func TestDecodeEvalValueAspasBase64(t *testing.T) {
	// "poke" em base64 = cG9rZQ== — DAP devolve entre aspas duplas.
	data, st, err := DecodeEvalValue("\"cG9rZQ==\"")
	if err != nil || st != EvalOK {
		t.Fatalf("err=%v st=%v", err, st)
	}
	if string(data) != "poke" {
		t.Fatalf("data=%q", data)
	}
}

func TestDecodeEvalValueSemAspas(t *testing.T) {
	data, st, err := DecodeEvalValue("cG9rZQ==")
	if err != nil || st != EvalOK || string(data) != "poke" {
		t.Fatalf("err=%v st=%v data=%q", err, st, data)
	}
}

func TestDecodeEvalValueNIL(t *testing.T) {
	_, st, err := DecodeEvalValue("NIL")
	if err != nil || st != EvalNil {
		t.Fatalf("err=%v st=%v", err, st)
	}
	// NIL com aspas (caso DAP) também deve virar EvalNil após UnquoteDAP.
	_, st, err = DecodeEvalValue("\"NIL\"")
	if err != nil || st != EvalNil {
		t.Fatalf("aspas: err=%v st=%v", err, st)
	}
}

func TestDecodeEvalValueVazio(t *testing.T) {
	_, st, err := DecodeEvalValue("")
	if err != nil || st != EvalEmpty {
		t.Fatalf("err=%v st=%v", err, st)
	}
}

func TestDecodeEvalValueBase64Invalido(t *testing.T) {
	if _, _, err := DecodeEvalValue("!!!nao-base64!!!"); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestUnquoteDAP(t *testing.T) {
	cases := map[string]string{
		"\"abc\"":      "abc",
		"abc":          "abc",
		"\"a\\nb\"":    "a\\nb",
		"":             "",
		"\"":           "\"", // aspas solta: não toca
	}
	for in, want := range cases {
		if got := UnquoteDAP(in); got != want {
			t.Errorf("UnquoteDAP(%q)=%q, esperado %q", in, got, want)
		}
	}
}
