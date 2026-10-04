package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPullOne_OKDecodificaEGrava(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "1.PNG")
	calls := 0
	eval := func(expr string, d time.Duration) (string, error) {
		calls++
		if expr != "Encode64(GetApoRes('1.PNG'))" {
			t.Errorf("expr=%q", expr)
		}
		return "\"cG9rZQ==\"", nil // "poke"
	}
	st, err := pullOne("1.PNG", dst, eval)
	if err != nil || st != pullOK {
		t.Fatalf("st=%v err=%v", st, err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "poke" {
		t.Fatalf("conteúdo=%q", b)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	// .tmp não pode sobrar
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp residual")
	}
}

func TestPullOne_NIL(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "x.bin")
	eval := func(string, time.Duration) (string, error) { return "NIL", nil }
	st, err := pullOne("x.bin", dst, eval)
	if err != nil || st != pullNil {
		t.Fatalf("st=%v err=%v", st, err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("não deveria gravar em NIL")
	}
}

func TestPullOne_SkipPreExistente(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "prev.bin")
	if err := os.WriteFile(dst, []byte("já-tem"), 0644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	eval := func(string, time.Duration) (string, error) { calls++; return "", nil }
	st, err := pullOne("prev.bin", dst, eval)
	if err != nil || st != pullSkip {
		t.Fatalf("st=%v err=%v", st, err)
	}
	if calls != 0 {
		t.Fatal("não deveria avaliar arquivo existente")
	}
}

func TestPullOne_RejeitaNomePerigoso(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "bad")
	eval := func(string, time.Duration) (string, error) {
		t.Fatal("não deveria avaliar")
		return "", nil
	}
	if st, _ := pullOne("a'b", dst, eval); st != pullErr {
		t.Fatalf("st=%v", st)
	}
	if st, _ := pullOne("a\nb", dst, eval); st != pullErr {
		t.Fatalf("st=%v", st)
	}
}

func TestPullReadManifest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lista.txt")
	if err := os.WriteFile(p, []byte("1.PNG\n\n#comentario\nB.TLPP\n"), 0644); err != nil {
		t.Fatal(err)
	}
	names, err := pullReadManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "1.PNG" || names[1] != "B.TLPP" {
		t.Fatalf("names=%v", names)
	}
}

func TestPullResList_MontaManifesto(t *testing.T) {
	var exprs []string
	eval := func(expr string, d time.Duration) (string, error) {
		exprs = append(exprs, expr)
		switch {
		case strings.HasPrefix(expr, "aAll := GetResArray("):
			return "\"Array size=3\"", nil
		case expr == "Len(aAll)":
			return "\"3\"", nil
		case strings.Contains(expr, "n >= 1"):
			return "\"A.PNG\nB.TLPP\nC.TRP\"", nil
		case strings.Contains(expr, "n >= 3001"):
			return "\"\"", nil
		}
		return "\"\"", nil
	}
	names, err := pullResList(eval, "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[0] != "A.PNG" || names[2] != "C.TRP" {
		t.Fatalf("names=%v", names)
	}
	// expressão de chunk idêntica à onda 24
	if !strings.Contains(exprs[2],
		"cOut := ''; AEval(aAll, {|x,n| IIf(n >= 1 .and. n <= 3, cOut := cOut + x + Chr(10), NIL)}); cOut") {
		t.Fatalf("expr chunk=%q", exprs[2])
	}
}

func TestCmdRpoPull_UsoSemOut(t *testing.T) {
	err := cmdRpoPull([]string{"--manifest", "x.txt"})
	if err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("err=%v", err)
	}
}
