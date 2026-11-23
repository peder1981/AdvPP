package lexer

import "testing"

// Variável com nome de palavra-chave (oF, In, Menu, Title...) colada em
// `[` é indexação; com espaço, `[texto]` continua string literal (SAY [x]).
// Casos reais do corpus 811R4: `Menu[n] = aPrincipal`, `{|MENU| MENU[3]}`.
func TestKeywordColadaEmColcheteIndexa(t *testing.T) {
	casos := []struct {
		src      string
		indexa   bool // true: há TOKEN_LBRACKET; false: virou TOKEN_STRING
	}{
		{`ConOut(oF["k"])`, true},
		{`x := Title[1]`, true},
		{`Menu[n] = aPrincipal`, true},
		{`{|MENU| MENU[3]}`, true},
		{`@ 1,1 SAY [abc]`, false},
		{`RETURN [texto]`, false},
		{`x := oJson[1]`, true},
	}
	for _, c := range casos {
		toks, err := Tokenize(c.src, "t.prw")
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		temColchete := false
		for _, tk := range toks {
			if tk.Type == TOKEN_LBRACKET {
				temColchete = true
			}
		}
		if temColchete != c.indexa {
			t.Errorf("%q: indexação = %v, quer %v (tokens %v)", c.src, temColchete, c.indexa, toks)
		}
	}
}
