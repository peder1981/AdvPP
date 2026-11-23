// Package encoding garante a regra do projeto: todo fonte AdvPL/TLPP do
// repositório em CP-1252 (nunca UTF-8).
package encoding

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

var extFonte = map[string]bool{".prw": true, ".prx": true, ".prg": true, ".tlpp": true, ".ch": true, ".th": true}

// TestFontesEmCP1252: arquivo com bytes acima de ASCII que formam UTF-8
// válido está em UTF-8. CP-1252 com acentos quase nunca é UTF-8 válido.
func TestFontesEmCP1252(t *testing.T) {
	raiz := filepath.Join("..", "..")
	var utf8s []string
	err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", ".worktrees", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !extFonte[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if temNaoASCII(b) && utf8.Valid(b) {
			utf8s = append(utf8s, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range utf8s {
		t.Errorf("%s está em UTF-8; converta para CP-1252 (iconv -f utf-8 -t cp1252)", p)
	}
}

func temNaoASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return true
		}
	}
	return false
}
