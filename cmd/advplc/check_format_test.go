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
