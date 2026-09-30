package rpo

// forensics.go — análise de conteúdo do RPO que NÃO depende de
// descriptografia e que NÃO inventa estrutura onde só existe ruído.
//
// MOTIVAÇÃO (lição empírica): versões anteriores desta base tentaram
// "identificar rotinas" (ex.: AP448, DK158) e "funções" (U_...) aplicando
// regex diretamente sobre o conteúdo do RPO. Como o Body/AdminSection
// são, na esmagadora maioria, CIFRA + ZLIB (alta entropia), esses regex
// casam com COINCIDÊNCIAS ALEATÓRIAS. Ver forensics_test.go, que prova
// estatisticamente que a contagem de matches em dados aleatórios do mesmo
// tamanho é equivalente (mesma ordem de grandeza) à contagem no RPO —
// ou seja, os nomes citados em relatórios antigos eram falsos positivos.
//
// O que este arquivo oferece no lugar:
//   - ClassifyRegions: segmenta o arquivo em janelas e mede entropia de
//     Shannon, classificando cada janela como Zero, Ciphertext (alta
//     entropia), ou Plaintext (baixa entropia).
//   - PlaintextWindows: retorna SOMENTE as janelas onde faz sentido
//     procurar nomes/strings, evitando o paradeiro de falsos positivos.
//   - ExtractStringsFromPlaintext: extrai strings apenas de janelas
//     classificadas como Plaintext.

import (
	"math"
	"sort"
)

// RegionKind classifica a natureza de uma janela de conteúdo.
type RegionKind string

const (
	// RegionZero — janela inteiramente zerada (padding/área vazia).
	RegionZero RegionKind = "zero"
	// RegionCiphertext — entropia alta: cifra ou dados comprimidos/aleatórios.
	// Não contém estrutura legível; qualquer "nome" achado aqui é ruído.
	RegionCiphertext RegionKind = "ciphertext"
	// RegionPlaintext — entropia baixa: pode conter texto/estrutura.
	RegionPlaintext RegionKind = "plaintext"
	// RegionMixed — entropia intermediária.
	RegionMixed RegionKind = "mixed"
)

// DefaultEntropyThreshold é o limiar de entropia (bits/byte) abaixo do
// qual uma janela é considerada Plaintext o suficiente para busca de
// nomes. 4.5 bits/byte separa texto latino/ASCII (≈3.5–4.5) de cifra
// (≈7.9–8.0) com folga confortável.
const DefaultEntropyThreshold = 4.5

// Region é uma janela classificada do conteúdo do RPO.
type Region struct {
	Offset     int        `json:"offset"`
	Size       int        `json:"size"`
	Entropy    float64    `json:"entropy"`
	UniqueByte int        `json:"unique_bytes"`
	ZeroRatio  float64    `json:"zero_ratio"`
	Kind       RegionKind `json:"kind"`
}

// ShannonEntropy calcula a entropia de Shannon (bits/byte) de data.
// Retorna 0 para entrada vazia.
func ShannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	total := float64(len(data))
	var ent float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / total
		ent -= p * math.Log2(p)
	}
	return ent
}

// ClassifyRegions segmenta data em janelas de windowSize bytes e
// classifica cada uma. windowSize <= 0 usa 4096.
func ClassifyRegions(data []byte, windowSize int) []Region {
	if windowSize <= 0 {
		windowSize = 4096
	}
	var regions []Region
	for off := 0; off < len(data); off += windowSize {
		end := off + windowSize
		if end > len(data) {
			end = len(data)
		}
		win := data[off:end]
		if len(win) == 0 {
			break
		}
		ent := ShannonEntropy(win)
		uniq := uniqueByteCount(win)
		zeros := zeroCount(win)
		zeroRatio := float64(zeros) / float64(len(win))

		kind := RegionMixed
		switch {
		case zeros == len(win):
			kind = RegionZero
		case ent >= 7.5:
			kind = RegionCiphertext
		case ent <= DefaultEntropyThreshold && uniq > 1:
			kind = RegionPlaintext
		}
		regions = append(regions, Region{
			Offset:     off,
			Size:       len(win),
			Entropy:    ent,
			UniqueByte: uniq,
			ZeroRatio:  zeroRatio,
			Kind:       kind,
		})
	}
	return regions
}

// RegionSummary resume a classificação de um arquivo.
type RegionSummary struct {
	TotalBytes      int     `json:"total_bytes"`
	ZeroBytes       int     `json:"zero_bytes"`
	CiphertextBytes int     `json:"ciphertext_bytes"`
	PlaintextBytes  int     `json:"plaintext_bytes"`
	MixedBytes      int     `json:"mixed_bytes"`
	CiphertextPct   float64 `json:"ciphertext_pct"`
	MeanEntropy     float64 `json:"mean_entropy"`
}

// SummarizeRegions agrega as regiões num resumo.
func SummarizeRegions(regions []Region) RegionSummary {
	var s RegionSummary
	var entSum float64
	for _, r := range regions {
		s.TotalBytes += r.Size
		entSum += r.Entropy * float64(r.Size)
		switch r.Kind {
		case RegionZero:
			s.ZeroBytes += r.Size
		case RegionCiphertext:
			s.CiphertextBytes += r.Size
		case RegionPlaintext:
			s.PlaintextBytes += r.Size
		default:
			s.MixedBytes += r.Size
		}
	}
	if s.TotalBytes > 0 {
		s.CiphertextPct = 100 * float64(s.CiphertextBytes) / float64(s.TotalBytes)
		s.MeanEntropy = entSum / float64(s.TotalBytes)
	}
	return s
}

// PlaintextWindows retorna somente as regiões classificadas como
// Plaintext — o único lugar onde buscar nomes/strings faz sentido.
func PlaintextWindows(data []byte, windowSize int) []Region {
	var out []Region
	for _, r := range ClassifyRegions(data, windowSize) {
		if r.Kind == RegionPlaintext {
			out = append(out, r)
		}
	}
	return out
}

// ExtractStringsFromPlaintext extrai strings ASCII imprimíveis (>= minLen)
// SOMENTE das janelas classificadas como Plaintext. Diferente de aplicar
// regex no arquivo inteiro, isto não produz falsos positivos a partir de
// cifra — se não houver janela Plaintext, o retorno é vazio.
func ExtractStringsFromPlaintext(data []byte, windowSize, minLen int) []StringMatch {
	if minLen < 1 {
		minLen = 4
	}
	var out []StringMatch
	for _, r := range PlaintextWindows(data, windowSize) {
		win := data[r.Offset : r.Offset+r.Size]
		out = append(out, extractStringsRaw(win, r.Offset, minLen)...)
	}
	return out
}

// StringMatch é uma string imprimível localizada.
type StringMatch struct {
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

func extractStringsRaw(data []byte, base, minLen int) []StringMatch {
	var out []StringMatch
	var cur []byte
	flush := func(end int) {
		if len(cur) >= minLen {
			out = append(out, StringMatch{Offset: base + end - len(cur), Text: string(cur)})
		}
		cur = nil
	}
	for i, b := range data {
		if b >= 32 && b <= 126 {
			cur = append(cur, b)
		} else {
			flush(i)
		}
	}
	flush(len(data))
	return out
}

func uniqueByteCount(data []byte) int {
	var seen [256]bool
	n := 0
	for _, b := range data {
		if !seen[b] {
			seen[b] = true
			n++
		}
	}
	return n
}

func zeroCount(data []byte) int {
	n := 0
	for _, b := range data {
		if b == 0 {
			n++
		}
	}
	return n
}

// SortRegionsByEntropy ordena regiões por entropia (decrescente).
func SortRegionsByEntropy(regions []Region) {
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].Entropy > regions[j].Entropy
	})
}
