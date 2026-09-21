package rpo

import (
	"crypto/rand"
	"math"
	"os"
	"regexp"
	"testing"
)

// ---------------------------------------------------------------------------
// Estes testes existem para IMPEDIR a regressão documental que produziu
// "rotinas identificadas: AP448, DK158, ..." e "funções U_..." em relatórios
// antigos. Eles demonstram, com dado real, que tais nomes eram falsos
// positivos de regex sobre conteúdo cifrado.
// ---------------------------------------------------------------------------

var (
	reUserFunc = regexp.MustCompile(`U_[A-Z0-9_]{3,10}`)
	reRoutine  = regexp.MustCompile(`[A-Z]{2,4}[0-9]{3,5}`)
)

// TestRegexFalsePositiveOnCiphertext prova que a taxa de matches de regex
// por byte no conteúdo cifrado de um RPO real é indistinguível da taxa em
// dados aleatórios — ou seja, os nomes citados em relatórios antigos eram
// coincidência estatística, não estrutura.
func TestRegexFalsePositiveOnCiphertext(t *testing.T) {
	t.Parallel()

	rpoData, err := os.ReadFile("testdata/live_capture.rpo")
	if err != nil {
		t.Skipf("testdata indisponível: %v", err)
	}
	f, err := Parse(rpoData)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	real := append(append([]byte{}, f.AdminSection...), f.Body...)

	// Modelo nulo: 4 MiB de dados uniformemente aleatórios.
	const nullSize = 4 << 20
	null := make([]byte, nullSize)
	if _, err := rand.Read(null); err != nil {
		t.Fatal(err)
	}

	rateRealR := float64(len(reRoutine.FindAll(real, -1))) / float64(len(real))
	rateNullR := float64(len(reRoutine.FindAll(null, -1))) / float64(nullSize)
	rateRealU := float64(len(reUserFunc.FindAll(real, -1))) / float64(len(real))
	rateNullU := float64(len(reUserFunc.FindAll(null, -1))) / float64(nullSize)

	t.Logf("amostra RPO: %d bytes | modelo nulo: %d bytes", len(real), nullSize)
	t.Logf("rotina/byte: real=%.3e nulo=%.3e", rateRealR, rateNullR)
	t.Logf("U_/byte:     real=%.3e nulo=%.3e", rateRealU, rateNullU)

	// Sem ordem de magnitude: taxas dentro de fator 15 (tolerante a ruído
	// de amostra pequena). O ponto é NÃO haver sinal estrutural.
	if rateRealR > rateNullR*15 && rateRealR > 1e-5 {
		t.Errorf("rotina/byte real (%.3e) muito acima do nulo (%.3e): possível estrutura",
			rateRealR, rateNullR)
	}
	if rateRealU > rateNullU*15 && rateRealU > 1e-5 {
		t.Errorf("U_/byte real (%.3e) muito acima do nulo (%.3e): possível estrutura",
			rateRealU, rateNullU)
	}

	if ent := ShannonEntropy(real); ent < 7.0 {
		t.Errorf("esperado conteúdo cifrado com entropia >= 7.0, obtido %.3f", ent)
	}
}

// TestPlaintextGateBlocosStrings prova que ExtractStringsFromPlaintext
// NÃO extrai strings de conteúdo cifrado (entropia alta), eliminando a
// fonte dos falsos positivos.
func TestPlaintextGateBlocosStrings(t *testing.T) {
	t.Parallel()

	random := make([]byte, 256<<10)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}

	got := ExtractStringsFromPlaintext(random, 4096, 4)
	if len(got) != 0 {
		t.Errorf("esperado 0 strings de ciphertext, obtido %d (ex.: %+v)", len(got), firstN(got, 3))
	}

	plain := []byte("User Function MinhaRotina()\n    Local cNome := \"teste\"\nReturn\n")
	got = ExtractStringsFromPlaintext(plain, 4096, 4)
	joined := ""
	for _, m := range got {
		joined += m.Text + "\n"
	}
	for _, want := range []string{"User Function MinhaRotina", "Local cNome"} {
		found := false
		for _, m := range got {
			if contains(m.Text, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("esperado conter %q; obtido: %q", want, joined)
		}
	}
}

// TestClassifyRegionsSeparaCifraDeTexto verifica a classificação.
func TestClassifyRegionsSeparaCifraDeTexto(t *testing.T) {
	t.Parallel()

	// 8 KiB de zeros + 8 KiB de texto ASCII repetitivo (baixa entropia)
	// + 8 KiB de "cifra" (aleatório).
	sentence := "User Function Teste()\n    Local cVar := \"valor\"\nReturn cVar\n"
	text := make([]byte, 8192)
	for i := range text {
		text[i] = sentence[i%len(sentence)]
	}
	cipher := make([]byte, 8192)
	if _, err := rand.Read(cipher); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 0, 24576)
	data = append(data, make([]byte, 8192)...)
	data = append(data, text...)
	data = append(data, cipher...)

	regions := ClassifyRegions(data, 8192)
	if len(regions) != 3 {
		t.Fatalf("esperado 3 regiões, obtido %d", len(regions))
	}
	if regions[0].Kind != RegionZero {
		t.Errorf("região 0: esperado zero, obtido %s (ent=%.2f)", regions[0].Kind, regions[0].Entropy)
	}
	if regions[1].Kind != RegionPlaintext {
		t.Errorf("região 1: esperado plaintext, obtido %s (ent=%.2f)", regions[1].Kind, regions[1].Entropy)
	}
	if regions[2].Kind != RegionCiphertext {
		t.Errorf("região 2: esperado ciphertext, obtido %s (ent=%.2f)", regions[2].Kind, regions[2].Entropy)
	}

	sum := SummarizeRegions(regions)
	if sum.PlaintextBytes != 8192 || sum.CiphertextBytes != 8192 || sum.ZeroBytes != 8192 {
		t.Errorf("resumo inesperado: %+v", sum)
	}
}

// TestShannonEntropySanity valida o cálculo básico.
func TestShannonEntropySanity(t *testing.T) {
	t.Parallel()
	if got := ShannonEntropy(nil); got != 0 {
		t.Errorf("nil: esperado 0, obtido %v", got)
	}
	if got := ShannonEntropy([]byte{0, 0, 0, 0}); got != 0 {
		t.Errorf("constante: esperado 0, obtido %v", got)
	}
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	if got := ShannonEntropy(all); math.Abs(got-8.0) > 1e-9 {
		t.Errorf("uniforme 256: esperado 8.0, obtido %v", got)
	}
}

func withinFactor(a, b int, factor float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	hi, lo := float64(a), float64(b)
	if lo > hi {
		hi, lo = lo, hi
	}
	if lo == 0 {
		return hi <= float64(factor)
	}
	return hi/lo <= factor
}

func firstN(m []StringMatch, n int) []StringMatch {
	if len(m) < n {
		return m
	}
	return m[:n]
}

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
