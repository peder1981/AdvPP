package vm

import (
	"fmt"
	"strconv"
	"strings"
)

// MapaCampo descreve uma propriedade exposta pelo adapter FWAdapterBaseV2:
// Prop é o nome público (JSON/URL), Coluna a coluna física, Tipo o tipo
// SX3 (C/N/D/L...) para conversão e Filtra/Ordena se o campo pode ser
// usado em filtro e ordenação.
type MapaCampo struct {
	Prop   string
	Coluna string
	Tipo   string
	Filtra bool
	Ordena bool
}

// ---------------------------------------------------------------------------
// Filtro OData ("campo eq 'x' and contains(descri,'y')")
// ---------------------------------------------------------------------------

// TraduzFiltro converte uma expressão de filtro OData-like em SQL
// parametrizado ("?" para cada valor — nunca concatena literal do usuário).
// Propriedades devem estar em `mapa` com Filtra=true; função LIKE usa
// ESCAPE '\' para que %, _ e \ no termo sejam literais.
func TraduzFiltro(filtro string, mapa map[string]MapaCampo) (string, []any, error) {
	p := &filtroParser{mapa: mapa}
	if err := p.tokenize(filtro); err != nil {
		return "", nil, err
	}
	sql, err := p.parseExpr()
	if err != nil {
		return "", nil, err
	}
	if p.pos != len(p.toks) {
		return "", nil, fmt.Errorf("filtro OData: token inesperado %q", p.toks[p.pos].text)
	}
	return sql, p.args, nil
}

type ftokKind int

const (
	ftLParen ftokKind = iota
	ftRParen
	ftComma
	ftString
	ftNumber
	ftWord
	ftOp // operadores simbólicos: = <> != >= <= > <
)

type ftok struct {
	kind ftokKind
	text string
}

type filtroParser struct {
	toks []ftok
	pos  int
	mapa map[string]MapaCampo
	args []any
	sql  strings.Builder
}

func (p *filtroParser) tokenize(src string) error {
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			p.toks = append(p.toks, ftok{ftLParen, "("})
			i++
		case c == ')':
			p.toks = append(p.toks, ftok{ftRParen, ")"})
			i++
		case c == ',':
			p.toks = append(p.toks, ftok{ftComma, ","})
			i++
		case c == '\'':
			j := i + 1
			var sb strings.Builder
			for j < len(src) {
				if src[j] == '\'' {
					if j+1 < len(src) && src[j+1] == '\'' { // '' = aspa literal
						sb.WriteByte('\'')
						j += 2
						continue
					}
					break
				}
				sb.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return fmt.Errorf("filtro OData: string sem fechamento")
			}
			p.toks = append(p.toks, ftok{ftString, sb.String()})
			i = j + 1
		case c == '=' || c == '!' || c == '>' || c == '<':
			j := i + 1
			for j < len(src) && (src[j] == '=' || src[j] == '>') {
				j++
			}
			p.toks = append(p.toks, ftok{ftOp, src[i:j]})
			i = j
		case c >= '0' && c <= '9' || c == '-' || c == '+':
			j := i + 1
			for j < len(src) && ((src[j] >= '0' && src[j] <= '9') || src[j] == '.') {
				j++
			}
			p.toks = append(p.toks, ftok{ftNumber, src[i:j]})
			i = j
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_':
			j := i + 1
			for j < len(src) && ((src[j] >= 'a' && src[j] <= 'z') || (src[j] >= 'A' && src[j] <= 'Z') || (src[j] >= '0' && src[j] <= '9') || src[j] == '_') {
				j++
			}
			p.toks = append(p.toks, ftok{ftWord, src[i:j]})
			i = j
		default:
			return fmt.Errorf("filtro OData: caractere inesperado %q", string(c))
		}
	}
	return nil
}

func (p *filtroParser) peek() *ftok {
	if p.pos < len(p.toks) {
		return &p.toks[p.pos]
	}
	return nil
}

func (p *filtroParser) next() *ftok {
	t := p.peek()
	if t != nil {
		p.pos++
	}
	return t
}

// isWord reporta se o próximo token é a palavra w (case-insensitive) sem
// consumi-lo.
func (p *filtroParser) isWord(w string) bool {
	t := p.peek()
	return t != nil && t.kind == ftWord && strings.EqualFold(t.text, w)
}

// parseExpr: termo { ("and"|"or") termo } — associativo à esquerda, cada
// junção embrulhada em parênteses para preservar a ordem literal.
func (p *filtroParser) parseExpr() (string, error) {
	left, err := p.parseTerm()
	if err != nil {
		return "", err
	}
	for p.isWord("and") || p.isWord("or") {
		op := strings.ToUpper(p.next().text)
		right, err := p.parseTerm()
		if err != nil {
			return "", err
		}
		left = "(" + left + " " + op + " " + right + ")"
	}
	return left, nil
}

// parseTerm: "(" expr ")" | "not" termo | funcao | prop op valor.
func (p *filtroParser) parseTerm() (string, error) {
	t := p.peek()
	if t == nil {
		return "", fmt.Errorf("filtro OData: expressão incompleta")
	}
	switch {
	case t.kind == ftLParen:
		p.next()
		inner, err := p.parseExpr()
		if err != nil {
			return "", err
		}
		if p.peek() == nil || p.peek().kind != ftRParen {
			return "", fmt.Errorf("filtro OData: ')' esperado")
		}
		p.next()
		return "(" + inner + ")", nil
	case t.kind == ftWord && strings.EqualFold(t.text, "not"):
		p.next()
		inner, err := p.parseTerm()
		if err != nil {
			return "", err
		}
		return "(NOT " + inner + ")", nil
	case t.kind == ftWord && (strings.EqualFold(t.text, "contains") ||
		strings.EqualFold(t.text, "startswith") || strings.EqualFold(t.text, "endswith")):
		return p.parseFuncao()
	case t.kind == ftWord:
		return p.parseComparacao()
	default:
		return "", fmt.Errorf("filtro OData: termo inesperado %q", t.text)
	}
}

// parseComparacao: prop op valor. prop precisa estar mapeado com Filtra.
func (p *filtroParser) parseComparacao() (string, error) {
	prop := p.next()
	mc, err := p.campo(prop.text, true)
	if err != nil {
		return "", err
	}
	opTok := p.next()
	if opTok == nil {
		return "", fmt.Errorf("filtro OData: operador esperado após %q", prop.text)
	}
	op := filtroOpSQL(opTok)
	if op == "" {
		return "", fmt.Errorf("filtro OData: operador %q não suportado", opTok.text)
	}
	valTok := p.next()
	if valTok == nil {
		return "", fmt.Errorf("filtro OData: valor esperado após %q %s", prop.text, opTok.text)
	}
	// null só combina com igualdade/desigualdade (IS [NOT] NULL); os
	// demais operadores não fazem sentido com ele.
	if valTok.kind == ftWord && strings.EqualFold(valTok.text, "null") {
		switch op {
		case "=":
			return mc.Coluna + " IS NULL", nil
		case "<>":
			return mc.Coluna + " IS NOT NULL", nil
		default:
			return "", fmt.Errorf("filtro OData: operador %q não combina com null", opTok.text)
		}
	}
	var arg any
	switch valTok.kind {
	case ftString:
		arg = valTok.text
	case ftNumber:
		f, err := strconv.ParseFloat(valTok.text, 64)
		if err != nil {
			return "", fmt.Errorf("filtro OData: número inválido %q", valTok.text)
		}
		arg = f
	case ftWord:
		switch strings.ToLower(valTok.text) {
		case "true":
			arg = true
		case "false":
			arg = false
		default:
			return "", fmt.Errorf("filtro OData: valor %q não suportado (use 'texto', número, true/false ou null)", valTok.text)
		}
	default:
		return "", fmt.Errorf("filtro OData: valor inesperado %q", valTok.text)
	}
	p.args = append(p.args, arg)
	return mc.Coluna + " " + op + " ?", nil
}

// filtroOpSQL mapeia o token de operador para o SQL.
func filtroOpSQL(t *ftok) string {
	if t.kind == ftOp {
		switch t.text {
		case "=", "<>", "!=", ">=", "<=", ">", "<":
			if t.text == "!=" {
				return "<>"
			}
			return t.text
		}
		return ""
	}
	switch strings.ToLower(t.text) {
	case "eq":
		return "="
	case "ne":
		return "<>"
	case "gt":
		return ">"
	case "ge":
		return ">="
	case "lt":
		return "<"
	case "le":
		return "<="
	}
	return ""
}

// parseFuncao: contains(prop,'x') | startswith(prop,'x') | endswith(prop,'x')
// -> COLUNA LIKE ? ESCAPE '\' com o padrão %x% (escape de \ % _ no termo).
func (p *filtroParser) parseFuncao() (string, error) {
	fn := strings.ToLower(p.next().text)
	if p.peek() == nil || p.peek().kind != ftLParen {
		return "", fmt.Errorf("filtro OData: '(' esperado após %s(", fn)
	}
	p.next()
	prop := p.next()
	if prop == nil || prop.kind != ftWord {
		return "", fmt.Errorf("filtro OData: propriedade esperada em %s(", fn)
	}
	mc, err := p.campo(prop.text, true)
	if err != nil {
		return "", err
	}
	if p.peek() == nil || p.peek().kind != ftComma {
		return "", fmt.Errorf("filtro OData: ',' esperado em %s(", fn)
	}
	p.next()
	val := p.next()
	if val == nil || val.kind != ftString {
		return "", fmt.Errorf("filtro OData: termo string esperado em %s(prop,'x')", fn)
	}
	if p.peek() == nil || p.peek().kind != ftRParen {
		return "", fmt.Errorf("filtro OData: ')' esperado em %s(", fn)
	}
	p.next()
	var pat string
	switch fn {
	case "contains":
		pat = "%" + escapeLike(val.text) + "%"
	case "startswith":
		pat = escapeLike(val.text) + "%"
	case "endswith":
		pat = "%" + escapeLike(val.text)
	}
	p.args = append(p.args, pat)
	return mc.Coluna + " LIKE ? ESCAPE '\\'", nil
}

// escapeLike escapa os curingas do LIKE (\, % e _) com barra invertida —
// combinada com ESCAPE '\' no SQL, o termo casa literalmente.
func escapeLike(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 4)
	for _, c := range s {
		if c == '\\' || c == '%' || c == '_' {
			sb.WriteByte('\\')
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

// campo resolve o nome da propriedade (case-insensitive) no mapa. Com
// filtra=true exige MapaCampo.Filtra; com false exige Ordena.
func (p *filtroParser) campo(prop string, filtra bool) (MapaCampo, error) {
	mc, err := filtroCampoLookup(p.mapa, prop)
	if err != nil {
		return MapaCampo{}, err
	}
	if filtra && !mc.Filtra {
		return MapaCampo{}, fmt.Errorf("campo %q não é filtrável", prop)
	}
	if !filtra && !mc.Ordena {
		return MapaCampo{}, fmt.Errorf("campo %q não é ordenável", prop)
	}
	return mc, nil
}

// TraduzOrdem converte "-nome,codigo" em "COL_NOME DESC, COL_COD ASC".
// Cada item precisa estar no mapa com Ordena=true; "-" prefixa DESC.
func TraduzOrdem(ordem string, mapa map[string]MapaCampo) (string, error) {
	ordem = strings.TrimSpace(ordem)
	if ordem == "" {
		return "", nil
	}
	var parts []string
	for _, item := range strings.Split(ordem, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		desc := strings.HasPrefix(item, "-")
		name := strings.TrimPrefix(item, "-")
		mc, err := filtroCampoLookup(mapa, name)
		if err != nil {
			return "", err
		}
		if !mc.Ordena {
			return "", fmt.Errorf("campo %q não é ordenável", name)
		}
		s := mc.Coluna
		if desc {
			s += " DESC"
		} else {
			s += " ASC"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", "), nil
}

// filtroCampoLookup resolve prop no mapa (case-insensitive, pela chave ou
// por mc.Prop) sem exigir Filtra/Ordena — os flags são checados por quem
// chama (filtro, ordenação).
func filtroCampoLookup(mapa map[string]MapaCampo, prop string) (MapaCampo, error) {
	for k, mc := range mapa {
		if strings.EqualFold(k, prop) || strings.EqualFold(mc.Prop, prop) {
			return mc, nil
		}
	}
	return MapaCampo{}, fmt.Errorf("campo %q não mapeado", prop)
}

// TraduzCampos converte "a,b" (nomes de Prop) na lista de colunas
// físicas. Lista vazia devolve todas as colunas mapeadas. Nome não
// mapeado é erro — nunca deixa coluna arbitrária vazar para o SQL.
func TraduzCampos(campos string, mapa map[string]MapaCampo) ([]string, error) {
	if strings.TrimSpace(campos) == "" {
		out := make([]string, 0, len(mapa))
		for _, mc := range mapa {
			out = append(out, mc.Coluna)
		}
		return out, nil
	}
	var cols []string
	for _, item := range strings.Split(campos, ",") {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		mc, err := filtroCampoLookup(mapa, name)
		if err != nil {
			return nil, err
		}
		cols = append(cols, mc.Coluna)
	}
	return cols, nil
}
