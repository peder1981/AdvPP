package vm

import (
	"strconv"
	"strings"
)

// resolveFilial computa a filial "efetiva" pra uma tabela: a filial ativa
// da sessão (RpcSetEnv), truncada/espacada conforme o NIVEL configurado em
// X2_FILIAL_COMPART pra essa tabela. Sem engine conectado, sem a tabela de
// config, ou sem linha pro alias -- default NIVEL=6 (mais restritivo,
// falha segura: nunca vaza dado achando que uma tabela e compartilhada
// quando ninguem configurou nada).
func (v *VM) resolveFilial(eng SQLEngine, alias string) string {
	nivel := 6
	if eng != nil {
		rows, err := eng.QueryRows("SELECT NIVEL FROM X2_FILIAL_COMPART WHERE TABELA = ?", strings.ToUpper(alias))
		if err == nil && len(rows) > 0 {
			if n, convErr := strconv.Atoi(strings.TrimSpace(rows[0]["NIVEL"])); convErr == nil {
				nivel = n
			}
		}
	}
	return truncarFilial(v.filialAtiva, nivel)
}

// truncarFilial mantem os primeiros nivel caracteres da filial e completa
// o resto com espaco ate 6 -- mesma regra usada pro valor GRAVADO em cada
// linha (ver browse.go, Task 3), por isso uma comparacao de igualdade
// simples (WHERE FILIAL = ?) funciona pra qualquer nivel sem CASE/lógica
// condicional na query.
func truncarFilial(filial string, nivel int) string {
	if nivel < 0 {
		nivel = 0
	}
	if nivel > 6 {
		nivel = 6
	}
	base := filial
	for len(base) < 6 {
		base += " "
	}
	base = base[:6]
	return base[:nivel] + strings.Repeat(" ", 6-nivel)
}
