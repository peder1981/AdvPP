package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodeJSONLines lê o JSONL do runCheckJSON em ordem.
func decodeJSONLines(t *testing.T, raw string) []checkJSONResult {
	t.Helper()
	var out []checkJSONResult
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		var r checkJSONResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("linha não-JSON: %q (%v)", line, err)
		}
		out = append(out, r)
	}
	return out
}

func TestRunCheckJSONMixed(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.prw")
	if err := os.WriteFile(good, []byte("User Function Tst()\nReturn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.prw")
	if err := os.WriteFile(bad, []byte("defeito((("), 0o600); err != nil {
		t.Fatal(err)
	}
	// Captura stdout via pipe (runCheckJSON recebe *os.File).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := runCheckJSON([]string{good, bad}, &Options{}, w)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if code != 1 {
		t.Fatalf("mistura deveria sair 1, saiu %d", code)
	}
	got := decodeJSONLines(t, buf.String())
	if len(got) != 2 {
		t.Fatalf("esperava 2 linhas, veio %d", len(got))
	}
	if got[0].File != good || !got[0].OK || got[0].Error != "" {
		t.Fatalf("primeira linha (bom) errada: %+v", got[0])
	}
	if got[1].File != bad || got[1].OK || got[1].Error == "" {
		t.Fatalf("segunda linha (ruim) errada: %+v", got[1])
	}
}

func TestRunCheckJSONAllOK(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.prw")
	if err := os.WriteFile(good, []byte("User Function Tst()\nReturn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := runCheckJSON([]string{good}, &Options{}, w)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if code != 0 {
		t.Fatalf("tudo-ok deveria sair 0, saiu %d", code)
	}
	got := decodeJSONLines(t, buf.String())
	if len(got) != 1 || !got[0].OK {
		t.Fatalf("linha única errada: %+v", got)
	}
}

func TestParseOptionsFormat(t *testing.T) {
	if got := parseOptions([]string{"--format", "json"}).format; got != "json" {
		t.Fatalf("forma espaço: %q", got)
	}
	if got := parseOptions([]string{"--format=json"}).format; got != "json" {
		t.Fatalf("forma =: %q", got)
	}
	if got := parseOptions([]string{"--include", "./x"}).format; got != "" {
		t.Fatalf("default deveria ser vazio: %q", got)
	}
}

func checkJSONDe(t *testing.T, src string) checkJSONResult {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.prw")
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	r, w, _ := os.Pipe()
	runCheckJSON([]string{p}, &Options{}, w)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return decodeJSONLines(t, buf.String())[0]
}

// O editor marca o erro na posição: line/col/message ao lado do error.
func TestCheckJSONPosicao(t *testing.T) {
	got := checkJSONDe(t, "User Function Main(\n\tConOut(\"x\"\nReturn\n")
	if got.OK || got.Line != 2 || got.Col != 8 {
		t.Fatalf("posição: %+v", got)
	}
	if got.Message == "" || strings.Contains(got.Message, "x.prw") || !strings.Contains(got.Error, "x.prw:2:8") {
		t.Errorf("message/error: %+v", got)
	}
}

// Sem posição reconhecível no erro: nenhuma linha/coluna inventada.
func TestCheckJSONSemPosicao(t *testing.T) {
	if l, c, m := posicaoDoErro("falha qualquer sem posição"); l != 0 || c != 0 || m != "falha qualquer sem posição" {
		t.Errorf("posicaoDoErro = %d %d %q", l, c, m)
	}
	data, _ := json.Marshal(checkJSONResult{File: "a", Error: "e", Message: "e"})
	if strings.Contains(string(data), `"line"`) || strings.Contains(string(data), `"col"`) {
		t.Errorf("JSON sem posição não deve ter line/col: %s", data)
	}
}

// Terminador órfão: ok continua true (compila, como no Protheus) e o aviso
// sai em "warnings" com linha e coluna.
func TestRunCheckJSONAvisos(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "orfao.prw")
	if err := os.WriteFile(f, []byte("User Function Tst()\nReturn\n\tEndIf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	code := runCheckJSON([]string{f}, &Options{}, w)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	if code != 0 {
		t.Fatalf("aviso não é falha: saiu %d (%s)", code, buf.String())
	}
	var got checkJSONResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || len(got.Warnings) != 1 || got.Warnings[0].Line != 3 || got.Warnings[0].Message != "EndIf sem bloco aberto (ignorado)" {
		t.Fatalf("resultado inesperado: %s", buf.String())
	}
}
