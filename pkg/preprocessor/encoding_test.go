package preprocessor

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCP1252TableExhaustive confere, para os 256 valores de byte possíveis,
// que a tabela pré-computada (cp1252ToUTF8) é idêntica à conversão de
// referência (cp1252ByteToUTF8) — cobertura total, não amostragem, já que
// o espaço de entrada (um byte) é pequeno o bastante para testar por
// inteiro.
func TestCP1252TableExhaustive(t *testing.T) {
	for b := 0; b < 256; b++ {
		want := cp1252ByteToUTF8(byte(b))
		got := cp1252ToUTF8[b]
		if !bytes.Equal(want, got) {
			t.Fatalf("byte 0x%02x: table=%v, reference=%v", b, got, want)
		}
	}
}

// TestConvertWithGoEncodingKnownChars confere alguns casos reais do
// intervalo especial 0x80-0x9F (aspas curvas, travessão, reticências) que
// aparecem em comentários de fontes Protheus reais.
func TestConvertWithGoEncodingKnownChars(t *testing.T) {
	// 0x93/0x94 = aspas curvas esquerda/direita, 0x96 = en dash, 0x85 = reticências
	got, err := convertWithGoEncoding([]byte{0x93, 'x', 0x94, 0x96, 0x85})
	if err != nil {
		t.Fatalf("convertWithGoEncoding: %v", err)
	}
	want := "“x”–…"
	if got != want {
		t.Errorf("convertWithGoEncoding = %q, want %q", got, want)
	}
}

// TestConvertWithGoEncodingVsPerByteReference confere, com entradas
// aleatórias de vários tamanhos e densidades de byte alto (incluindo runs
// longos de ASCII puro, só bytes altos, e tudo intercalado), que a versão
// que copia em blocos (convertWithGoEncoding) produz exatamente a mesma
// saída que aplicar cp1252ByteToUTF8 byte a byte — a otimização por blocos
// tem aritmética de índice (start/i) que é fácil de errar por 1 nas
// bordas; isso cobre esse risco especificamente.
func TestConvertWithGoEncodingVsPerByteReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	sizes := []int{0, 1, 2, 3, 10, 100, 1000}
	// probabilidade de cada byte ser >=128 (denso vs esparso vs nenhum)
	highByteProbs := []float64{0, 0.01, 0.3, 1.0}

	for _, n := range sizes {
		for _, p := range highByteProbs {
			source := make([]byte, n)
			for i := range source {
				if rng.Float64() < p {
					source[i] = byte(128 + rng.Intn(128))
				} else {
					source[i] = byte(rng.Intn(128))
				}
			}

			var want bytes.Buffer
			for _, b := range source {
				want.Write(cp1252ByteToUTF8(b))
			}

			got, err := convertWithGoEncoding(source)
			if err != nil {
				t.Fatalf("n=%d p=%v: convertWithGoEncoding: %v", n, p, err)
			}
			if got != want.String() {
				t.Fatalf("n=%d p=%v: convertWithGoEncoding = %q, want %q", n, p, got, want.String())
			}
		}
	}
}

// TestIncludeCP1252ConvertidoComoORaiz: um #include em CP-1252 tem que chegar
// ao lexer em UTF-8, igual ao arquivo raiz. Antes, loadInclude devolvia os
// bytes crus: "ã" (0xE3) do include nunca era igual ao "ã" do raiz.
func TestIncludeCP1252ConvertidoComoORaiz(t *testing.T) {
	dir := t.TempDir()
	// 'Return "ã"' em CP-1252
	if err := os.WriteFile(filepath.Join(dir, "inc.prw"), []byte("Static Function F()\nReturn \"\xe3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, nome := range []string{"inc.prw", "INC.PRW"} { // caminho exato e case-insensitive
		pp := NewPreprocessor([]string{dir})
		out, err := pp.Process("#include \""+nome+"\"\n", filepath.Join(dir, "main.prw"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Return \"ã\"") {
			t.Errorf("%s: include não convertido para UTF-8: %q", nome, out)
		}
	}
	// Include que já é UTF-8 continua intacto
	if got := SourceToUTF8([]byte("ã€")); got != "ã€" {
		t.Errorf("SourceToUTF8(UTF-8) = %q", got)
	}
}
