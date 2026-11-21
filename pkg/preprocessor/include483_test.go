package preprocessor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escreve(t *testing.T, path, txt string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(txt), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Include aninhado relativo resolve ao lado do arquivo que inclui, e o mesmo
// arquivo incluído por duas grafias entra uma vez só.
func TestIncludeAninhadoRelativo(t *testing.T) {
	d := t.TempDir()
	escreve(t, filepath.Join(d, "src", "base.prw"), "Static Function Base()\nReturn 1\n")
	escreve(t, filepath.Join(d, "src", "meio.prw"), "#include \"base.prw\"\nStatic Function Meio()\nReturn 2\n")
	raiz := filepath.Join(d, "tests", "t.prw")
	escreve(t, raiz, "#include \"../src/base.prw\"\n#include \"../src/meio.prw\"\nUser Function T()\nReturn\n")

	p := NewPreprocessor(nil)
	out, err := p.Process(string(mustRead(t, raiz)), raiz)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !strings.Contains(out, "Function Meio") {
		t.Errorf("meio.prw nao entrou:\n%s", out)
	}
	if n := strings.Count(out, "Function Base"); n != 1 {
		t.Errorf("base.prw entrou %d vezes (esperado 1):\n%s", n, out)
	}
}

// Fonte incluído inexistente vira aviso com arquivo e linha (não erro, para
// não quebrar programa existente com include morto); .ch ausente segue sem aviso.
func TestIncludeFonteInexistente(t *testing.T) {
	d := t.TempDir()
	raiz := filepath.Join(d, "t.prw")
	escreve(t, raiz, "#include \"totvs.ch\"\n#include \"nao_existe.prw\"\nUser Function T()\nReturn\n")
	p := NewPreprocessor(nil)
	if _, err := p.Process(string(mustRead(t, raiz)), raiz); err != nil {
		t.Fatalf("nao devia falhar: %v", err)
	}
	av := p.Avisos()
	if len(av) != 1 || av[0].Linha != 2 || !strings.Contains(av[0].Mensagem, "nao_existe.prw") {
		t.Fatalf("esperava 1 aviso na linha 2 sobre nao_existe.prw, veio: %+v", av)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
