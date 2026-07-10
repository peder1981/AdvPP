package preprocessor

import "strings"

// commandRule é uma definição #command/#xcommand/#translate/#xtranslate
// compilada: um padrão de casamento (do lado esquerdo do "=>") e um molde
// de resultado (do lado direito), no estilo do pré-processador do Clipper.
//
// Sintaxe suportada do padrão:
//   - palavras literais: casadas sem diferenciar maiúsculas/minúsculas
//   - <nome>            : marcador que captura o que aparecer até o
//     próximo literal esperado (uma "cláusula")
//   - <nome,...>        : marcador de lista (captura tudo, vírgulas
//     inclusas — a expansão é sempre pelo texto cru)
//   - [ ... ]           : grupo opcional; se o primeiro literal dentro
//     dele não aparecer na posição atual, o grupo
//     inteiro é pulado (nenhuma captura)
//   - [<nome:LITERAL>]  : marcador de flag — vira <.nome.> = .T./.F. no
//     resultado conforme LITERAL aparecer ou não
//
// Sintaxe suportada do resultado:
//   - <nome>   : substitui pelo texto cru capturado (ou "" se ausente)
//   - <{nome}> : vira {|| texto capturado} se presente, NIL se ausente
//   - <.nome.> : vira .T. se capturado/flag presente, .F. caso contrário
//   - \[ \]    : colchete literal (equivalente a `[`/`]` fora de padrão)
type commandRule struct {
	pattern []patToken
	result  string
}

type patTokenKind int

const (
	patLit patTokenKind = iota
	patMarker
	patOptional
)

type patToken struct {
	kind     patTokenKind
	text     string     // patLit: a palavra literal
	name     string     // patMarker: nome da variável capturada
	isList   bool       // patMarker: era "<nome,...>"
	sub      []patToken // patOptional: padrão dentro do "[...]"
	flagName string     // patOptional: se o grupo é só um marcador de flag "<nome:LITERAL>", o nome
	flagLit  string     // patOptional: ... e o literal que ativa a flag
}

// parseCommandDef compila uma definição de #command/#xcommand/#translate/
// #xtranslate já com continuações de linha unidas (um "=>" a mais dentro de
// uma string literal do resultado quebraria isto, mas não ocorre nestas
// definições reais). Retorna ok=false se não achar "=>" ou o padrão vier
// vazio.
func parseCommandDef(def string) (commandRule, bool) {
	idx := strings.Index(def, "=>")
	if idx < 0 {
		return commandRule{}, false
	}
	// o ";" no fim do padrão é só a marca de continuação de linha da
	// PRÓPRIA definição multi-linha (junção feita antes de chegar aqui),
	// não faz parte da gramática do comando — sem tirar, viraria um
	// token literal exigido que nunca bate com uso real.
	patStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(def[:idx]), ";"))
	resStr := strings.TrimSpace(def[idx+2:])
	if patStr == "" {
		return commandRule{}, false
	}
	pattern := compilePattern(patStr)
	if len(pattern) == 0 {
		return commandRule{}, false
	}
	return commandRule{pattern: pattern, result: resStr}, true
}

// compilePattern tokeniza uma string de padrão em literais/marcadores/
// grupos opcionais.
func compilePattern(s string) []patToken {
	var toks []patToken
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '<':
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				i = len(s)
				break
			}
			inner := s[i+1 : i+j]
			i += j + 1
			toks = append(toks, compileMarker(inner))
		case c == '[':
			depth := 1
			j := i + 1
			for j < len(s) && depth > 0 {
				if s[j] == '[' {
					depth++
				} else if s[j] == ']' {
					depth--
				}
				j++
			}
			inner := s[i+1 : j-1]
			i = j
			toks = append(toks, compileOptional(inner))
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '<' && s[j] != '[' {
				j++
			}
			toks = append(toks, patToken{kind: patLit, text: s[i:j]})
			i = j
		}
	}
	return toks
}

// compileMarker interpreta o conteúdo de "<...>": "nome", "nome,..." ou
// "nome:LITERAL" (esta última só faz sentido dentro de um "[...]").
func compileMarker(inner string) patToken {
	if ci := strings.IndexByte(inner, ':'); ci >= 0 {
		return patToken{kind: patMarker, name: inner[:ci], flagLit: inner[ci+1:]}
	}
	name := inner
	isList := false
	if strings.HasSuffix(name, ",...") {
		name = strings.TrimSuffix(name, ",...")
		isList = true
	}
	return patToken{kind: patMarker, name: name, isList: isList}
}

// compileOptional compila o conteúdo de um grupo "[...]". Caso especial
// muito comum: o grupo é só um marcador de flag "<nome:LITERAL>" — vira um
// patOptional "raso" com flagName/flagLit direto, sem precisar casar um
// sub-padrão completo.
func compileOptional(inner string) patToken {
	sub := compilePattern(inner)
	if len(sub) == 1 && sub[0].kind == patMarker && sub[0].flagLit != "" {
		return patToken{kind: patOptional, flagName: sub[0].name, flagLit: sub[0].flagLit, sub: sub}
	}
	return patToken{kind: patOptional, sub: sub}
}

// tokenizeLine tokeniza uma linha de código real para casamento de comando.
// Não pode ser um split por espaço simples: identificadores quase sempre
// colam direto em pontuação ("TCSQLEXEC(\"select 1\")", "cAO:=Alias()"),
// e um split ingênuo grudaria "TCSQLEXEC(\"select" como um token só,
// fazendo até um casamento de literal simples (ex.: #translate sem
// parâmetros) nunca bater. Trata strings entre aspas como um token único
// (aspas inclusas), pontuação isolada como token de 1 caractere cada, e
// operadores de 2 caracteres comuns (":=", "->", "==", "<>", "<=", ">=")
// como um token só.
func tokenizeLine(s string) []string {
	var toks []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(s) && s[j] != c {
				j++
			}
			if j < len(s) {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		case i+1 < len(s) && isTwoCharOp(s[i:i+2]):
			toks = append(toks, s[i:i+2])
			i += 2
		case isWordChar(c):
			j := i
			for j < len(s) && isWordChar(s[j]) {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		default:
			toks = append(toks, string(c))
			i++
		}
	}
	return toks
}

// joinTokens rejunta tokens preservando espaço só entre dois tokens que são
// puramente "palavra" (identificador/palavra-chave) — é o único caso em
// que omitir o espaço corromperia o sentido (ex.: "TO"+"aHead" precisa
// ficar "TO aHead", não "TOaHead"). Ao redor de pontuação/operadores/
// strings, nunca insere espaço (o estilo real varia e nenhum dos dois é
// obrigatório para o lexer).
func joinTokens(toks []string) string {
	var sb strings.Builder
	for i, t := range toks {
		if i > 0 && isWordToken(toks[i-1]) && isWordToken(t) {
			sb.WriteByte(' ')
		}
		sb.WriteString(t)
	}
	return sb.String()
}

func isWordToken(t string) bool {
	if t == "" {
		return false
	}
	for i := 0; i < len(t); i++ {
		if !isWordChar(t[i]) {
			return false
		}
	}
	return true
}

func isTwoCharOp(s string) bool {
	switch s {
	case ":=", "->", "==", "<>", "<=", ">=", "++", "--":
		return true
	}
	return false
}

// isWordChar já existe em preprocessor.go (mesma checagem, reaproveitada).

// matchResult acumula o que foi capturado ao casar um padrão contra uma
// linha real de código.
type matchResult struct {
	vars  map[string]string
	flags map[string]bool
}

// matchPattern tenta casar `pattern` inteiro contra os tokens de origem,
// consumindo a partir de srcTokens[0]. Retorna o número de tokens
// consumidos e as capturas, ou ok=false se não bateu.
func matchPattern(pattern []patToken, srcTokens []string) (int, matchResult, bool) {
	res := matchResult{vars: map[string]string{}, flags: map[string]bool{}}
	pos := 0
	for pi := 0; pi < len(pattern); pi++ {
		pt := pattern[pi]
		switch pt.kind {
		case patLit:
			if pos >= len(srcTokens) || !strings.EqualFold(srcTokens[pos], pt.text) {
				return 0, res, false
			}
			pos++
		case patOptional:
			if pt.flagLit != "" && len(pt.sub) == 1 {
				// grupo é só uma flag: "[<nome:LITERAL>]"
				if pos < len(srcTokens) && strings.EqualFold(srcTokens[pos], pt.flagLit) {
					res.flags[pt.flagName] = true
					pos++
				} else {
					res.flags[pt.flagName] = false
				}
				continue
			}
			// grupo opcional genérico: só tenta se o primeiro literal do
			// sub-padrão aparecer na posição atual; senão pula o grupo
			// inteiro (marcadores dele ficam ausentes).
			firstLit := firstLiteral(pt.sub)
			if firstLit != "" && (pos >= len(srcTokens) || !strings.EqualFold(srcTokens[pos], firstLit)) {
				continue
			}
			consumed, subRes, ok := matchPattern(pt.sub, srcTokens[pos:])
			if !ok {
				continue // não bateu o resto do grupo: trata como ausente
			}
			pos += consumed
			for k, v := range subRes.vars {
				res.vars[k] = v
			}
			for k, v := range subRes.flags {
				res.flags[k] = v
			}
		case patMarker:
			// consome tokens até o próximo literal esperado (olhando à
			// frente no padrão) ou até o fim da linha/token ';'.
			stop := nextLiteral(pattern[pi+1:])
			start := pos
			for pos < len(srcTokens) {
				if srcTokens[pos] == ";" {
					break
				}
				if stop != "" && strings.EqualFold(srcTokens[pos], stop) {
					break
				}
				pos++
			}
			res.vars[pt.name] = joinTokens(srcTokens[start:pos])
		}
	}
	return pos, res, true
}

// firstLiteral devolve o texto do primeiro token literal de um padrão (usado
// para decidir se um grupo opcional deve ser tentado), ou "" se o padrão
// começa com um marcador (caso raro nestas definições reais).
func firstLiteral(pattern []patToken) string {
	for _, pt := range pattern {
		if pt.kind == patLit {
			return pt.text
		}
		return ""
	}
	return ""
}

// nextLiteral acha o texto do próximo token literal em uma sequência de
// padrão (pulando marcadores/opcionais), usado como ponto de parada para um
// marcador guloso.
func nextLiteral(pattern []patToken) string {
	for _, pt := range pattern {
		switch pt.kind {
		case patLit:
			return pt.text
		case patOptional:
			if l := firstLiteral(pt.sub); l != "" {
				return l
			}
		}
	}
	return ""
}

// expandResult substitui os marcadores de resultado (<nome>, <{nome}>,
// <.nome.>) e os colchetes escapados (\[ \]) no molde de resultado.
func expandResult(result string, m matchResult) string {
	var out strings.Builder
	i := 0
	for i < len(result) {
		switch {
		case strings.HasPrefix(result[i:], `\[`):
			out.WriteByte('[')
			i += 2
		case strings.HasPrefix(result[i:], `\]`):
			out.WriteByte(']')
			i += 2
		case result[i] == '<':
			j := strings.IndexByte(result[i:], '>')
			if j < 0 {
				out.WriteByte(result[i])
				i++
				continue
			}
			inner := result[i+1 : i+j]
			i += j + 1
			out.WriteString(expandMarkerResult(inner, m))
		default:
			out.WriteByte(result[i])
			i++
		}
	}
	return out.String()
}

func expandMarkerResult(inner string, m matchResult) string {
	switch {
	case strings.HasPrefix(inner, ".") && strings.HasSuffix(inner, "."):
		name := strings.Trim(inner, ".")
		if v, ok := m.vars[name]; ok && v != "" {
			return ".T."
		}
		if b, ok := m.flags[name]; ok && b {
			return ".T."
		}
		return ".F."
	case strings.HasPrefix(inner, "{") && strings.HasSuffix(inner, "}"):
		name := strings.TrimSuffix(strings.TrimPrefix(inner, "{"), "}")
		if v, ok := m.vars[name]; ok && v != "" {
			return "{|| " + v + "}"
		}
		return "NIL"
	default:
		return m.vars[inner]
	}
}

// applyCommandRules tenta cada regra registrada (na ordem em que foram
// definidas) contra o início da linha; a primeira que bater vence. Linhas
// sem nenhum comando customizado voltam inalteradas.
func (p *Preprocessor) applyCommandRules(line string) string {
	if len(p.commandRules) == 0 {
		return line
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		return line
	}
	leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	tokens := tokenizeLine(trimmed)
	for _, rule := range p.commandRules {
		consumed, res, ok := matchPattern(rule.pattern, tokens)
		if !ok || consumed == 0 {
			continue
		}
		expanded := expandResult(rule.result, res)
		// Um padrão sem marcador de "resto da linha" (ex.: #translate
		// simples, um só literal) pode casar só o começo da linha; o que
		// sobrar (a chamada "(args)" etc.) precisa voltar, não sumir.
		if consumed < len(tokens) {
			remainder := joinTokens(tokens[consumed:])
			if endsWithWordChar(expanded) && isWordToken(tokens[consumed]) {
				expanded += " "
			}
			expanded += remainder
		}
		return leading + expanded
	}
	return line
}

func endsWithWordChar(s string) bool {
	return len(s) > 0 && isWordChar(s[len(s)-1])
}
