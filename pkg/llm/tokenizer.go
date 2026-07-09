package llm

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Tokenizer implementa BPE byte-level no estilo GPT-2, usando o vocabulário
// e as regras de merge embutidos no próprio GGUF (sem downloads externos).
//
// ponytail: o pré-tokenizador da Falcon3 usa um pipeline de 3 regexes do
// llama.cpp (incluindo um look-ahead que a stdlib regexp do Go, baseada em
// RE2, não suporta) que separa dígitos um a um. Aqui implementamos o
// splitter GPT-2 padrão (letras/dígitos agrupados, pontuação em runs) —
// diverge da Falcon3 só em números com mais de um dígito. A validação
// numérica contra o C++ (task 17) usa tokens já tokenizados pela referência
// para não depender dessa fidelidade.
type Tokenizer struct {
	tokenToID  map[string]int32
	idToToken  []string
	mergeRank  map[string]int
	byteToRune [256]rune
	runeToByte map[rune]byte
	bos, eos   int32
}

// NewTokenizer carrega o vocabulário e as regras de merge de um GGUF gpt2/BPE.
func NewTokenizer(g *File) (*Tokenizer, error) {
	model, _ := g.String("tokenizer.ggml.model")
	if model != "gpt2" {
		return nil, fmt.Errorf("llm: tokenizer.ggml.model=%q não suportado (só gpt2)", model)
	}
	tokens, ok := g.StringArray("tokenizer.ggml.tokens")
	if !ok {
		return nil, fmt.Errorf("llm: tokenizer.ggml.tokens ausente")
	}
	merges, ok := g.StringArray("tokenizer.ggml.merges")
	if !ok {
		return nil, fmt.Errorf("llm: tokenizer.ggml.merges ausente")
	}

	t := &Tokenizer{
		tokenToID:  make(map[string]int32, len(tokens)),
		idToToken:  tokens,
		mergeRank:  make(map[string]int, len(merges)),
		runeToByte: make(map[rune]byte, 256),
	}
	for i, tok := range tokens {
		t.tokenToID[tok] = int32(i)
	}
	for i, m := range merges {
		t.mergeRank[m] = i
	}
	t.byteToRune, t.runeToByte = buildByteUnicodeTable()

	if bos, ok := g.Uint32("tokenizer.ggml.bos_token_id"); ok {
		t.bos = int32(bos)
	}
	if eos, ok := g.Uint32("tokenizer.ggml.eos_token_id"); ok {
		t.eos = int32(eos)
	}
	return t, nil
}

func (t *Tokenizer) BOS() int32 { return t.bos }
func (t *Tokenizer) EOS() int32 { return t.eos }

// buildByteUnicodeTable replica bytes_to_unicode() do encoder GPT-2 original:
// bytes imprimíveis mapeiam para si mesmos, os demais para code points a
// partir de U+0100, garantindo que todo byte tenha uma representação
// textual visível para o BPE operar em cima de strings.
func buildByteUnicodeTable() (byteToRune [256]rune, runeToByte map[rune]byte) {
	runeToByte = make(map[rune]byte, 256)
	printable := map[int]bool{}
	add := func(lo, hi int) {
		for b := lo; b <= hi; b++ {
			printable[b] = true
		}
	}
	add('!', '~')
	add(0xA1, 0xAC)
	add(0xAE, 0xFF)

	n := 0
	for b := 0; b < 256; b++ {
		var r rune
		if printable[b] {
			r = rune(b)
		} else {
			r = rune(256 + n)
			n++
		}
		byteToRune[b] = r
		runeToByte[r] = byte(b)
	}
	return
}

var contractions = []string{"'s", "'t", "'re", "'ve", "'m", "'ll", "'d"}

// pretokenize divide o texto em pedaços no estilo GPT-2 (contrações,
// [espaço?]+letras, [espaço?]+dígitos, espaço em branco final, e o resto
// como "outro" — pontuação/símbolos).
func pretokenize(text string) []string {
	runes := []rune(text)
	var out []string
	i := 0
	for i < len(runes) {
		rest := string(runes[i:])
		matchedContraction := false
		for _, c := range contractions {
			if strings.HasPrefix(rest, c) {
				out = append(out, c)
				i += len([]rune(c))
				matchedContraction = true
				break
			}
		}
		if matchedContraction {
			continue
		}

		r := runes[i]
		switch {
		case r == ' ' && i+1 < len(runes) && unicode.IsLetter(runes[i+1]):
			start := i
			i++
			for i < len(runes) && unicode.IsLetter(runes[i]) {
				i++
			}
			out = append(out, string(runes[start:i]))
		case r == ' ' && i+1 < len(runes) && unicode.IsNumber(runes[i+1]):
			start := i
			i++
			for i < len(runes) && unicode.IsNumber(runes[i]) {
				i++
			}
			out = append(out, string(runes[start:i]))
		case unicode.IsLetter(r):
			start := i
			for i < len(runes) && unicode.IsLetter(runes[i]) {
				i++
			}
			out = append(out, string(runes[start:i]))
		case unicode.IsNumber(r):
			start := i
			for i < len(runes) && unicode.IsNumber(runes[i]) {
				i++
			}
			out = append(out, string(runes[start:i]))
		case unicode.IsSpace(r) && allSpaceFrom(runes, i):
			out = append(out, string(runes[i:]))
			i = len(runes)
		default:
			start := i
			if r == ' ' && i+1 < len(runes) {
				i++
			}
			for i < len(runes) && !unicode.IsSpace(runes[i]) && !unicode.IsLetter(runes[i]) && !unicode.IsNumber(runes[i]) {
				i++
			}
			if i == start {
				i++
			}
			out = append(out, string(runes[start:i]))
		}
	}
	return out
}

func allSpaceFrom(runes []rune, i int) bool {
	for ; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) {
			return false
		}
	}
	return true
}

// Encode converte texto em uma sequência de token ids via BPE byte-level.
func (t *Tokenizer) Encode(text string) []int32 {
	var ids []int32
	for _, piece := range pretokenize(text) {
		symbols := t.byteEncodeSymbols(piece)
		symbols = t.applyBPE(symbols)
		for _, s := range symbols {
			if id, ok := t.tokenToID[s]; ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// byteEncodeSymbols converte uma string UTF-8 em uma lista de símbolos, um
// por byte, cada um representado pelo caractere GPT-2 correspondente.
func (t *Tokenizer) byteEncodeSymbols(s string) []string {
	b := []byte(s)
	out := make([]string, len(b))
	for i, c := range b {
		out[i] = string(t.byteToRune[c])
	}
	return out
}

// applyBPE aplica merges repetidamente, sempre no par adjacente de maior
// prioridade (menor rank), até não haver mais nenhum merge aplicável —
// algoritmo BPE clássico.
func (t *Tokenizer) applyBPE(symbols []string) []string {
	if len(symbols) < 2 {
		return symbols
	}
	for {
		bestRank := -1
		bestIdx := -1
		for i := 0; i < len(symbols)-1; i++ {
			key := symbols[i] + " " + symbols[i+1]
			if rank, ok := t.mergeRank[key]; ok {
				if bestRank == -1 || rank < bestRank {
					bestRank = rank
					bestIdx = i
				}
			}
		}
		if bestIdx == -1 {
			break
		}
		merged := symbols[bestIdx] + symbols[bestIdx+1]
		next := make([]string, 0, len(symbols)-1)
		next = append(next, symbols[:bestIdx]...)
		next = append(next, merged)
		next = append(next, symbols[bestIdx+2:]...)
		symbols = next
	}
	return symbols
}

// Decode converte uma sequência de token ids de volta em texto. Tokens BPE
// normais são revertidos pelo mapeamento byte<->unicode do GPT-2; tokens
// especiais/de controle (ex.: "\n", "\t", "<|endoftext|>") costumam ser
// gravados no vocabulário como texto literal, não através desse mapeamento
// — se algum caractere do token não existir na tabela reversa, usamos os
// bytes UTF-8 literais do token em vez de descartar o caractere.
func (t *Tokenizer) Decode(ids []int32) string {
	var raw []byte
	for _, id := range ids {
		if int(id) < 0 || int(id) >= len(t.idToToken) {
			continue
		}
		piece := t.idToToken[id]
		if b, ok := t.decodeBPEPiece(piece); ok {
			raw = append(raw, b...)
		} else {
			raw = append(raw, piece...)
		}
	}
	return string(raw)
}

// decodeBPEPiece reverte o mapeamento byte<->unicode do GPT-2 para um token,
// retornando ok=false se algum caractere não pertencer ao alfabeto de 256
// símbolos (sinal de que o token é literal, não byte-BPE).
func (t *Tokenizer) decodeBPEPiece(piece string) ([]byte, bool) {
	out := make([]byte, 0, len(piece))
	for _, r := range piece {
		b, ok := t.runeToByte[r]
		if !ok {
			return nil, false
		}
		out = append(out, b)
	}
	return out, true
}

// TopTokens retorna os n tokens de maior logit (usado só para depuração).
func TopTokens(t *Tokenizer, logits []float32, n int) []string {
	type pair struct {
		id  int
		val float32
	}
	pairs := make([]pair, len(logits))
	for i, v := range logits {
		pairs[i] = pair{i, v}
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].val > pairs[b].val })
	if n > len(pairs) {
		n = len(pairs)
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = t.idToToken[pairs[i].id]
	}
	return out
}
