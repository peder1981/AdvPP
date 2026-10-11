package llm

import (
	"testing"
)

func testSpecialTokenizer() *Tokenizer {
	return &Tokenizer{
		tokenToID:   map[string]int32{},
		specials:    map[string]int32{"<|im_start|>": 100, "<|im_end|>": 101, "<|im_start|>x": 102},
		specialList: []string{"<|im_start|>x", "<|im_start|>", "<|im_end|>"},
	}
}

func TestSplitSpecialsLongestMatch(t *testing.T) {
	tok := testSpecialTokenizer()
	spans := tok.splitSpecials("a<|im_start|>xB<|im_end|>c")
	if len(spans) != 5 {
		t.Fatalf("esperava 5 spans, veio %d: %q", len(spans), spans)
	}
	if spans[0] != "a" || spans[1] != int32(102) || spans[2] != "B" || spans[3] != int32(101) || spans[4] != "c" {
		t.Fatalf("spans errados: %q", spans)
	}
}

func TestSplitSpecialsNoSpecials(t *testing.T) {
	tok := &Tokenizer{}
	spans := tok.splitSpecials("texto puro çã")
	if len(spans) != 1 || spans[0] != "texto puro çã" {
		t.Fatalf("sem specials deveria passar intacto: %q", spans)
	}
}

func TestSplitSpecialsUTF8Safe(t *testing.T) {
	tok := testSpecialTokenizer()
	// "é" (2 bytes) imediatamente antes do special: o corte não pode partir o rune.
	spans := tok.splitSpecials("café<|im_end|>")
	if len(spans) != 2 || spans[0] != "café" || spans[1] != int32(101) {
		t.Fatalf("corte quebrou UTF-8 ou errou: %q", spans)
	}
}
