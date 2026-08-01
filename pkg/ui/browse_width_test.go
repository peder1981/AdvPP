package ui

import "testing"

// O titulo do SX3 e quem estoura: X3_TAMANHO descreve o dado, nao o
// cabecalho. "Competencia" (11 caracteres) sobre uma coluna de 7 saia por
// cima da coluna vizinha na grade de Despesas.
func TestBrowseColumnWidth(t *testing.T) {
	casos := []struct {
		nome    string
		label   string
		size    int
		querMin float32
	}{
		// 7*8+20 = 76, largo demais para 11 caracteres em bold.
		{"titulo maior que o dado", "Competência", 7, 100},
		{"titulo acentuado nao infla pelo byte", "Fração Ideal", 8, 0},
		{"dado maior que o titulo manda", "Valor", 14, 132},
		{"size fora da faixa cai no padrao", "Nome", 60, 120},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := browseColumnWidth(c.label, c.size)
			if got < c.querMin {
				t.Fatalf("browseColumnWidth(%q, %d) = %v; queria >= %v", c.label, c.size, got, c.querMin)
			}
		})
	}

	// Acento nao pode custar largura: mesmo numero de caracteres, mesma
	// largura -- e o que separa RuneCount de len.
	if a, b := browseColumnWidth("Fração Ideal", 8), browseColumnWidth("Fracao Ideal", 8); a != b {
		t.Fatalf("acento mudou a largura: %v vs %v", a, b)
	}

	// O caso que motivou tudo: o titulo tem que caber, nao a coluna do dado.
	if browseColumnWidth("Competência", 7) <= browseColumnWidth("", 7) {
		t.Fatal("titulo comprido nao alargou a coluna")
	}
}
