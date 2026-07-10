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
	flagLits []string   // patMarker/patOptional: literais do marcador restrito "<nome: LIT1, LIT2, ...>"
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
		// Marcador restrito "<nome: LIT1, LIT2, ...>": casa exatamente UMA
		// das palavras da lista (captura a que casou). Espaços ao redor de
		// ':' e ',' são comuns e não fazem parte dos literais.
		var lits []string
		for _, l := range strings.Split(inner[ci+1:], ",") {
			if l = strings.TrimSpace(l); l != "" {
				lits = append(lits, l)
			}
		}
		return patToken{kind: patMarker, name: strings.TrimSpace(inner[:ci]), flagLits: lits}
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
	if len(sub) == 1 && sub[0].kind == patMarker && len(sub[0].flagLits) > 0 {
		return patToken{kind: patOptional, flagName: sub[0].name, flagLits: sub[0].flagLits, sub: sub}
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
// consumindo a partir de srcTokens[0]. `outerStop` é o literal em que um
// marcador do FIM deste padrão deve parar quando o padrão em si não tem mais
// literais — necessário quando o padrão é o sub-padrão de um grupo opcional
// e o literal de parada real vem DEPOIS do grupo no padrão externo (ex.:
// `[<sayClauses,...>] VTGET ...`: o marcador de lista dentro do grupo tem de
// parar em "VTGET", que só o chamador conhece). Retorna o número de tokens
// consumidos e as capturas, ou ok=false se não bateu.
func matchPattern(pattern []patToken, srcTokens []string, outerStops []string) (int, matchResult, bool) {
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
			// Cláusulas opcionais consecutivas casam em QUALQUER ordem
			// (semântica Clipper: `[ OF <oWnd> ] [ PIXEL ]` aceita tanto
			// `OF oDlg PIXEL` quanto `PIXEL OF oDlg`). Junta a corrida de
			// opcionais e tenta cada grupo repetidamente até nenhum casar.
			runEnd := pi
			for runEnd < len(pattern) && pattern[runEnd].kind == patOptional {
				runEnd++
			}
			run := pattern[pi:runEnd]
			// pontos de parada para marcadores gulosos dentro dos grupos:
			// os literais de abertura de TODOS os grupos da corrida + o que
			// vier depois dela (ou o herdado do chamador).
			runStops := nextLiterals(pattern[pi:])
			if after := nextLiterals(pattern[runEnd:]); len(after) == 0 {
				runStops = append(runStops, outerStops...)
			}
			used := make([]bool, len(run))
			for progress := true; progress; {
				progress = false
				for gi := range run {
					if used[gi] {
						continue
					}
					g := run[gi]
					if len(g.flagLits) > 0 && len(g.sub) == 1 {
						// grupo é só uma flag: "[<nome: LIT[, LIT...]>]"
						if pos < len(srcTokens) && matchesAnyFold(srcTokens[pos], g.flagLits) {
							res.flags[g.flagName] = true
							res.vars[g.flagName] = srcTokens[pos]
							pos++
							used[gi] = true
							progress = true
						}
						continue
					}
					// grupo genérico: só tenta se um literal de abertura
					// dele aparecer na posição atual.
					openLits := firstLiterals(g.sub)
					if len(openLits) > 0 && (pos >= len(srcTokens) || !matchesAnyFold(srcTokens[pos], openLits)) {
						continue
					}
					consumed, subRes, ok := matchPattern(g.sub, srcTokens[pos:], runStops)
					if !ok || consumed == 0 {
						continue
					}
					pos += consumed
					used[gi] = true
					progress = true
					for k, v := range subRes.vars {
						res.vars[k] = v
					}
					for k, v := range subRes.flags {
						res.flags[k] = v
					}
				}
			}
			// flags de grupos que não apareceram ficam explicitamente .F.
			for gi := range run {
				if !used[gi] && run[gi].flagName != "" {
					if _, seen := res.flags[run[gi].flagName]; !seen {
						res.flags[run[gi].flagName] = false
					}
				}
			}
			pi = runEnd - 1 // o for externo pula a corrida inteira
		case patMarker:
			// marcador restrito "<nome: LIT1, LIT2>": casa exatamente uma
			// das palavras, capturando a que casou.
			if len(pt.flagLits) > 0 {
				if pos >= len(srcTokens) || !matchesAnyFold(srcTokens[pos], pt.flagLits) {
					return 0, res, false
				}
				res.flags[pt.name] = true
				res.vars[pt.name] = srcTokens[pos]
				pos++
				continue
			}
			// consome tokens até QUALQUER literal alcançável à frente no
			// padrão (literais de grupos opcionais são todos candidatos, já
			// que grupos podem ser pulados) ou até o fim da linha/token ';'.
			stops := nextLiterals(pattern[pi+1:])
			if len(stops) == 0 {
				stops = outerStops
			}
			start := pos
			for pos < len(srcTokens) {
				if srcTokens[pos] == ";" {
					break
				}
				if matchesAnyFold(srcTokens[pos], stops) {
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

// firstLiterals devolve os literais que podem ABRIR um padrão: o texto de um
// patLit inicial, ou os literais de um marcador restrito inicial
// ("<of: WINDOW, DIALOG, OF> <oWnd>" abre com WINDOW/DIALOG/OF).
func firstLiterals(pattern []patToken) []string {
	if len(pattern) == 0 {
		return nil
	}
	switch pattern[0].kind {
	case patLit:
		return []string{pattern[0].text}
	case patMarker:
		return pattern[0].flagLits // vazio para marcador comum
	}
	return nil
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

// nextLiterals acumula TODOS os literais alcançáveis a partir daqui: os
// literais de abertura de cada grupo opcional (qualquer um pode ser o
// próximo token real, já que grupos são puláveis) e — parando nele — o
// primeiro literal obrigatório. Um marcador guloso tem de parar em QUALQUER
// um deles (ex.: `<var> [PICTURE <pic>] [VALID <v>] [WHEN <w>]`: var para
// em PICTURE, VALID ou WHEN, o que aparecer primeiro).
func nextLiterals(pattern []patToken) []string {
	var lits []string
	for _, pt := range pattern {
		switch pt.kind {
		case patLit:
			return append(lits, pt.text)
		case patMarker:
			if len(pt.flagLits) > 0 {
				// marcador restrito obrigatório: são literais de parada e,
				// como é obrigatório, nada além dele pode vir — para aqui.
				return append(lits, pt.flagLits...)
			}
			// marcador comum guloso à frente: não contribui literal
		case patOptional:
			lits = append(lits, firstLiterals(pt.sub)...)
		}
	}
	return lits
}

func matchesAnyFold(tok string, lits []string) bool {
	for _, l := range lits {
		if strings.EqualFold(tok, l) {
			return true
		}
	}
	return false
}

// expandResult substitui os marcadores de resultado (<nome>, <{nome}>,
// <.nome.>, <"nome">), os grupos opcionais `[...]` (emitidos só se algum
// marcador dentro capturou algo — sem os colchetes) e os colchetes
// escapados (\[ \]) no molde de resultado.
func expandResult(result string, m matchResult) string {
	out, _, _ := expandResultSeg(result, m)
	return out
}

// expandResultSeg faz a expansão de um trecho do molde e reporta se o trecho
// continha marcadores e se algum deles capturou valor não-vazio — é o que
// decide se um grupo opcional `[...]` do resultado aparece na saída.
func expandResultSeg(result string, m matchResult) (string, bool, bool) {
	var out strings.Builder
	hasMarker := false
	anyCaptured := false
	i := 0
	for i < len(result) {
		switch {
		case strings.HasPrefix(result[i:], `\[`):
			out.WriteByte('[')
			i += 2
		case strings.HasPrefix(result[i:], `\]`):
			out.WriteByte(']')
			i += 2
		case result[i] == '[':
			// grupo opcional no resultado: emite o conteúdo expandido só se
			// algum marcador do grupo capturou algo; senão o grupo some.
			depth := 1
			j := i + 1
			for j < len(result) && depth > 0 {
				if strings.HasPrefix(result[j:], `\[`) || strings.HasPrefix(result[j:], `\]`) {
					j += 2
					continue
				}
				if result[j] == '[' {
					depth++
				} else if result[j] == ']' {
					depth--
				}
				j++
			}
			innerSeg := result[i+1 : j-1]
			i = j
			seg, segHas, segCap := expandResultSeg(innerSeg, m)
			if !segHas || segCap {
				out.WriteString(seg)
			}
			if segHas {
				hasMarker = true
				if segCap {
					anyCaptured = true
				}
			}
		case result[i] == '<':
			j := strings.IndexByte(result[i:], '>')
			if j < 0 {
				out.WriteByte(result[i])
				i++
				continue
			}
			inner := result[i+1 : i+j]
			i += j + 1
			exp := expandMarkerResult(inner, m)
			hasMarker = true
			if v, ok := m.vars[markerBaseName(inner)]; ok && v != "" {
				anyCaptured = true
			}
			out.WriteString(exp)
		default:
			out.WriteByte(result[i])
			i++
		}
	}
	return out.String(), hasMarker, anyCaptured
}

// markerBaseName extrai o nome da variável de um marcador de resultado em
// qualquer das formas: nome, {nome}, .nome., "nome".
func markerBaseName(inner string) string {
	inner = strings.TrimSpace(inner)
	inner = strings.Trim(inner, `.{}"`)
	return strings.TrimSpace(inner)
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
	case strings.HasPrefix(inner, `"`) && strings.HasSuffix(inner, `"`):
		// "dumb stringify": <"var"> vira o texto capturado entre aspas
		// (idioma clássico para passar o NOME da variável a um runtime,
		// ex.: VTSetGet(@<var>, <"var">, ...)).
		name := strings.Trim(inner, `"`)
		return `"` + m.vars[name] + `"`
	default:
		return m.vars[inner]
	}
}

// applyCommandRules tenta cada regra registrada (na ordem em que foram
// definidas) contra o início da linha; a primeira que bater vence. Linhas
// sem nenhum comando customizado voltam inalteradas.
func (p *Preprocessor) applyCommandRules(line string) string {
	return p.applyCommandRulesDepth(line, 0)
}

func (p *Preprocessor) applyCommandRulesDepth(line string, depth int) string {
	if depth > 8 || len(p.commandRules) == 0 {
		return line
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		return line
	}
	// Comentário de fim de linha não participa do casamento — sem tirar,
	// um marcador guloso o captura para DENTRO da expansão e o `//`
	// comenta o resto do código gerado (`TSay():New(..., oPanel//"x", ...)`).
	trimmed = strings.TrimSpace(stripLineComment(trimmed))
	if trimmed == "" {
		return line
	}
	leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	tokens := tokenizeLine(trimmed)
	// Semântica Clipper: a regra definida por ÚLTIMO vence (permite um .ch
	// especializar um comando já definido — ex.: apvt100.ch define
	// `@...VTSAY <xpr>` simples e depois `@...VTSAY <xpr> VTGET <var>`
	// combinado; o combinado, mais específico e definido depois, tem de
	// ganhar). Itera em ordem reversa de definição.
	for i := len(p.commandRules) - 1; i >= 0; i-- {
		rule := p.commandRules[i]
		consumed, res, ok := matchPattern(rule.pattern, tokens, nil)
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
		// A expansão pode gerar novos comandos (ex.: o VTSAY+VTGET combinado
		// expande para DOIS comandos `@...` separados por ';', cada um
		// coberto por outra regra) — reprocessa cada segmento de topo.
		segs := splitTopSemicolons(expanded)
		for si := range segs {
			segs[si] = p.applyCommandRulesDepth(strings.TrimSpace(segs[si]), depth+1)
		}
		return leading + strings.Join(segs, " ; ")
	}
	return line
}

// stripLineComment corta um comentário `//` de fim de linha (fora de
// aspas). `&&` (comentário Clipper) fica de fora de propósito: é raro e
// ambíguo com o operador .AND. abreviado em macros.
func stripLineComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			return s[:i]
		}
	}
	return s
}

// splitTopSemicolons divide uma linha expandida nos ';' de separação de
// comandos (fora de aspas). Um ';' dentro de string fica intacto.
func splitTopSemicolons(s string) []string {
	var segs []string
	start := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == ';' {
			segs = append(segs, s[start:i])
			start = i + 1
		}
	}
	segs = append(segs, s[start:])
	return segs
}

func endsWithWordChar(s string) bool {
	return len(s) > 0 && isWordChar(s[len(s)-1])
}
