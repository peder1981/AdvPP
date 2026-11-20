package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
)

// TestBytecodeDeterminism compila todos os fontes de examples/ e tests/ duas
// vezes e exige bytecode idêntico. Com ADVPP_BYTECODE_HASHES=<arquivo>, grava
// "sha256  caminho" (ordenado, caminhos com "/") — o job `determinism` do
// .github/workflows/test.yml roda isso em Linux/Windows/macOS e compara os
// três arquivos: a mesma revisão tem que gerar o mesmo bytecode em todo SO.
func TestBytecodeDeterminism(t *testing.T) {
	var files []string
	for _, root := range []string{"../../examples", "../../tests"} {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if ext := strings.ToLower(filepath.Ext(p)); ext == ".prw" || ext == ".tlpp" {
					files = append(files, p)
				}
			}
			return nil
		})
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("nenhum fonte encontrado em examples/ e tests/")
	}

	hash := func(src string) string {
		bc, err := loadAndCompile(src, &Options{})
		if err != nil {
			return "ERR" // fonte que não compila também tem que falhar igual em todo SO
		}
		out := filepath.Join(t.TempDir(), "x.bc")
		if err := compiler.SaveBytecode(bc, out); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}

	var lines []string
	for _, f := range files {
		h1, h2 := hash(f), hash(f)
		if h1 != h2 {
			t.Errorf("%s: bytecode não determinístico (%s != %s)", f, h1, h2)
		}
		rel := filepath.ToSlash(strings.TrimPrefix(f, filepath.Join("..", "..")+string(filepath.Separator)))
		lines = append(lines, h1+"  "+rel)
	}

	if out := os.Getenv("ADVPP_BYTECODE_HASHES"); out != "" {
		if err := os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%d hashes gravados em %s", len(lines), out)
	}
}
