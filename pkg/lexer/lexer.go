package lexer

import (
	"fmt"
	"strings"
)

type Lexer struct {
	source   string
	pos      int
	line     int
	col      int
	fileName string
	tokens   []Token
}

func NewLexer(source, fileName string) *Lexer {
	return &Lexer{
		source:   source,
		pos:      0,
		line:     1,
		col:      1,
		fileName: fileName,
		tokens:   make([]Token, 0),
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peekAt(offset int) byte {
	idx := l.pos + offset
	if idx >= len(l.source) {
		return 0
	}
	return l.source[idx]
}

func (l *Lexer) advance() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		ch := l.peek()
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else if ch == '\n' {
			l.tokens = append(l.tokens, Token{
				Type: TOKEN_NEWLINE, Value: "\n",
				Line: l.line, Col: l.col, FileName: l.fileName,
			})
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) skipLineComment() {
	for l.pos < len(l.source) && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() {
	for l.pos < len(l.source) {
		if l.peek() == '*' && l.peekAt(1) == '/' {
			l.advance()
			l.advance()
			return
		}
		l.advance()
	}
}

func (l *Lexer) isAlpha(ch byte) bool {
	// >= 0x80 covers CP-1252 accented letters (á, ã, ç, ...) — real source
	// is CP-1252, not UTF-8, and occasionally has them in identifiers/const
	// names outside comments/strings.
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch >= 0x80
}

func (l *Lexer) isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func (l *Lexer) isAlphaNum(ch byte) bool {
	return l.isAlpha(ch) || l.isDigit(ch)
}

func (l *Lexer) lastTokenIsNewlineOrEOF() bool {
	if len(l.tokens) == 0 {
		return true
	}
	last := l.tokens[len(l.tokens)-1]
	return last.Type == TOKEN_NEWLINE || last.Type == TOKEN_EOF || last.Type == TOKEN_SEMICOLON
}

// lastTokenEndsOperand reports whether the previous meaningful token can
// end an operand — used to disambiguate `[` (array indexing) from `[texto]`
// (Clipper bracket string literal). A newline between counts as a break.
func (l *Lexer) lastTokenEndsOperand() bool {
	for i := len(l.tokens) - 1; i >= 0; i-- {
		t := l.tokens[i]
		switch t.Type {
		case TOKEN_LINECOMMENT, TOKEN_BLOCKCOMMENT:
			continue
		case TOKEN_IDENT, TOKEN_NUMBER, TOKEN_STRING,
			TOKEN_RPAREN, TOKEN_RBRACKET, TOKEN_RBRACE,
			TOKEN_TRUE, TOKEN_FALSE, TOKEN_NIL:
			return true
		default:
			return false
		}
	}
	return false
}

func (l *Lexer) Tokenize() ([]Token, error) {
	for l.pos < len(l.source) {
		l.skipWhitespace()
		if l.pos >= len(l.source) {
			break
		}

		line, col := l.line, l.col
		ch := l.peek()

		// Comments
		if ch == '/' && l.peekAt(1) == '/' {
			l.skipLineComment()
			continue
		}
		if ch == '&' && l.peekAt(1) == '&' {
			// `&&` is a legacy end-of-line comment marker in older Clipper
			// code, same as `//`. A single '&' (macro expansion) is always
			// followed by an identifier or '(', never another '&'.
			l.skipLineComment()
			continue
		}
		if ch == '/' && l.peekAt(1) == '*' {
			l.advance()
			l.advance()
			l.skipBlockComment()
			continue
		}
		if ch == '*' && l.lastTokenIsNewlineOrEOF() {
			l.skipLineComment()
			continue
		}

		// Preprocessor directives — only when '#' starts a line; mid-line
		// '#' is Clipper's legacy "not equal" operator (`SA1->FIELD # 0`,
		// same as `<>`/`!=`).
		if ch == '#' && l.lastTokenIsNewlineOrEOF() {
			if err := l.tokenizeDirective(line, col); err != nil {
				return nil, err
			}
			continue
		}

		// Numbers
		if l.isDigit(ch) {
			l.tokenizeNumber(line, col)
			continue
		}

		// Strings - double quote
		if ch == '"' {
			if err := l.tokenizeString(line, col, '"'); err != nil {
				return nil, err
			}
			continue
		}

		// Strings - single quote
		if ch == '\'' {
			if err := l.tokenizeString(line, col, '\''); err != nil {
				return nil, err
			}
			continue
		}

		// Dot-prefixed literals and operators
		if ch == '.' {
			if l.tryDotLiteral(line, col) {
				continue
			}
			// Leading-dot float literal: `.5` / `.7` (no digit before the
			// dot), valid AdvPL/Clipper numeric syntax — real coordinate
			// literals in `@ .5,.7 ...` commands use this form.
			if l.pos+1 < len(l.source) && l.isDigit(l.source[l.pos+1]) {
				l.tokenizeNumber(line, col)
				continue
			}
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_DOT, Value: ".", Line: line, Col: col, FileName: l.fileName})
			continue
		}

		// Identifiers and keywords
		if l.isAlpha(ch) {
			// `BeginContent var <name> ...raw... EndContent` — TLPP raw
			// content block (JSON/XML embutido). The body is NOT AdvPL and
			// must not be tokenized; consume the whole construct here and
			// emit the equivalent of `<name> := "<raw>"`.
			if l.tryBeginContent(line, col) {
				continue
			}
			l.tokenizeIdentifier(line, col)
			continue
		}

		// Operators and punctuation
		if err := l.tokenizeOperator(line, col); err != nil {
			return nil, err
		}
	}

	l.tokens = append(l.tokens, Token{
		Type: TOKEN_EOF, Value: "",
		Line: l.line, Col: l.col, FileName: l.fileName,
	})

	return l.tokens, nil
}

// tryBeginContent detects `BeginContent var <name>` at the current position
// and, when found, consumes everything up to (and including) the line whose
// first word is `EndContent`, emitting `<name> := "<raw body>"` as three
// tokens. Returns false (consuming nothing) when the current word is not
// BeginContent or the construct is malformed.
func (l *Lexer) tryBeginContent(line, col int) bool {
	const kw = "begincontent"
	// Match the word BeginContent case-insensitively without consuming.
	if l.pos+len(kw) > len(l.source) {
		return false
	}
	if !strings.EqualFold(l.source[l.pos:l.pos+len(kw)], kw) {
		return false
	}
	if l.pos+len(kw) < len(l.source) && l.isAlphaNum(l.source[l.pos+len(kw)]) {
		return false // longer identifier that merely starts with it
	}
	// Scan ahead (indices only) for: ws+ "var" ws+ <name> ws* \n
	i := l.pos + len(kw)
	for i < len(l.source) && (l.source[i] == ' ' || l.source[i] == '\t') {
		i++
	}
	if i+3 > len(l.source) || !strings.EqualFold(l.source[i:i+3], "var") {
		return false
	}
	i += 3
	for i < len(l.source) && (l.source[i] == ' ' || l.source[i] == '\t') {
		i++
	}
	nameStart := i
	for i < len(l.source) && l.isAlphaNum(l.source[i]) {
		i++
	}
	if i == nameStart {
		return false
	}
	name := l.source[nameStart:i]
	// Body: from just after this line's newline to the line starting with
	// EndContent (first non-ws word).
	for i < len(l.source) && l.source[i] != '\n' {
		i++
	}
	if i < len(l.source) {
		i++ // consume the newline
	}
	bodyStart := i
	end := -1     // index of the line that holds EndContent
	endAfter := i // index just past EndContent's line
	for i < len(l.source) {
		lineStart := i
		j := i
		for j < len(l.source) && (l.source[j] == ' ' || l.source[j] == '\t') {
			j++
		}
		if j+10 <= len(l.source) && strings.EqualFold(l.source[j:j+10], "endcontent") &&
			(j+10 == len(l.source) || !l.isAlphaNum(l.source[j+10])) {
			end = lineStart
			endAfter = j + 10
			break
		}
		for i < len(l.source) && l.source[i] != '\n' {
			i++
		}
		if i < len(l.source) {
			i++
		}
	}
	if end < 0 {
		return false // no EndContent — not the construct after all
	}
	body := l.source[bodyStart:end]
	// Consume through EndContent, keeping line/col bookkeeping via advance.
	for l.pos < endAfter {
		l.advance()
	}
	l.tokens = append(l.tokens,
		Token{Type: TOKEN_IDENT, Value: name, Line: line, Col: col, FileName: l.fileName},
		Token{Type: TOKEN_ASSIGN, Value: ":=", Line: line, Col: col, FileName: l.fileName},
		Token{Type: TOKEN_STRING, Value: body, Line: line, Col: col, FileName: l.fileName},
	)
	return true
}

func (l *Lexer) tokenizeDirective(line, col int) error {
	l.advance() // consume #

	start := l.pos
	for l.pos < len(l.source) && l.peek() != '\n' && l.peek() != ' ' && l.peek() != '\t' {
		l.advance()
	}
	directive := strings.ToUpper(l.source[start:l.pos])

	for l.pos < len(l.source) && (l.peek() == ' ' || l.peek() == '\t') {
		l.advance()
	}

	contentStart := l.pos
	for l.pos < len(l.source) && l.peek() != '\n' {
		l.advance()
	}
	content := strings.TrimSpace(l.source[contentStart:l.pos])

	var tt TokenType
	switch directive {
	case "INCLUDE":
		tt = TOKEN_PREPROC_INCLUDE
	case "DEFINE":
		tt = TOKEN_PREPROC_DEFINE
	case "UNDEFINE", "UNDEF":
		tt = TOKEN_PREPROC_UNDEFINE
	case "IFDEF":
		tt = TOKEN_PREPROC_IFDEF
	case "IFNDEF":
		tt = TOKEN_PREPROC_IFNDEF
	case "ENDIF":
		tt = TOKEN_PREPROC_ENDIF
	case "ELSE":
		tt = TOKEN_PREPROC_ELSE
	case "XCOMMAND":
		tt = TOKEN_PREPROC_XCOMMAND
	case "XTRANSLATE":
		tt = TOKEN_PREPROC_XTRANSLATE
	case "COMMAND":
		tt = TOKEN_PREPROC_COMMAND
	case "TRANSLATE":
		tt = TOKEN_PREPROC_TRANSLATE
	default:
		tt = TOKEN_DIRECTIVE
	}

	l.tokens = append(l.tokens, Token{
		Type: tt, Value: content,
		Line: line, Col: col, FileName: l.fileName,
	})
	return nil
}

func (l *Lexer) tokenizeNumber(line, col int) {
	start := l.pos
	hasDot := false
	hasExp := false

	for l.pos < len(l.source) {
		ch := l.peek()
		if l.isDigit(ch) {
			l.advance()
		} else if ch == '.' && !hasDot && !hasExp {
			if l.peekAt(1) >= '0' && l.peekAt(1) <= '9' {
				hasDot = true
				l.advance()
			} else {
				break
			}
		} else if (ch == 'e' || ch == 'E') && !hasExp {
			hasExp = true
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
		} else {
			break
		}
	}

	l.tokens = append(l.tokens, Token{
		Type: TOKEN_NUMBER, Value: l.source[start:l.pos],
		Line: line, Col: col, FileName: l.fileName,
	})
}

func (l *Lexer) tokenizeString(line, col int, quote byte) error {
	l.advance() // consume opening quote
	var sb strings.Builder
	for l.pos < len(l.source) {
		ch := l.peek()
		// Clipper/AdvPL strings implicitly close at end-of-line if the
		// closing quote is missing (real legacy sources rely on this).
		if ch == '\n' {
			break
		}
		if ch == quote {
			l.advance()
			// Doubled quote = escaped quote
			if l.peek() == quote {
				sb.WriteByte(quote)
				l.advance()
				continue
			}
			break
		}
		// AdvPL/Clipper string literals have no backslash escapes — '\' is
		// always a literal character (common in paths: "C:\Temp\").
		sb.WriteByte(ch)
		l.advance()
	}

	l.tokens = append(l.tokens, Token{
		Type: TOKEN_STRING, Value: sb.String(),
		Line: line, Col: col, FileName: l.fileName,
	})
	return nil
}

func (l *Lexer) tryDotLiteral(line, col int) bool {
	// Só os próximos bytes importam (maior literal: ".NULL." = 6) —
	// uppercase do fonte inteiro aqui tornava o lexer O(n²) em arquivos grandes
	end := l.pos + 6
	if end > len(l.source) {
		end = len(l.source)
	}
	upper := strings.ToUpper(l.source[l.pos:end])

	literals := []struct {
		text string
		tt   TokenType
	}{
		{".AND.", TOKEN_DOT_AND},
		{".OR.", TOKEN_DOT_OR},
		{".NOT.", TOKEN_DOT_NOT},
		{".NULL.", TOKEN_NIL},
		{".NIL.", TOKEN_NIL},
		{".T.", TOKEN_TRUE},
		{".Y.", TOKEN_TRUE},
		{".F.", TOKEN_FALSE},
		{".N.", TOKEN_FALSE},
	}

	for _, lit := range literals {
		if strings.HasPrefix(upper, lit.text) {
			for i := 0; i < len(lit.text); i++ {
				l.advance()
			}
			l.tokens = append(l.tokens, Token{
				Type: lit.tt, Value: lit.text,
				Line: line, Col: col, FileName: l.fileName,
			})
			return true
		}
	}
	return false
}

func (l *Lexer) tokenizeIdentifier(line, col int) {
	start := l.pos
	for l.pos < len(l.source) {
		ch := l.peek()
		if l.isAlphaNum(ch) || ch == '_' {
			l.advance()
		} else {
			break
		}
	}

	word := l.source[start:l.pos]
	upper := strings.ToUpper(word)

	if Keywords[upper] {
		l.tokens = append(l.tokens, Token{
			Type: TOKEN_KEYWORD, Value: word,
			Line: line, Col: col, FileName: l.fileName,
		})
	} else {
		l.tokens = append(l.tokens, Token{
			Type: TOKEN_IDENT, Value: word,
			Line: line, Col: col, FileName: l.fileName,
		})
	}
}

func (l *Lexer) tokenizeOperator(line, col int) error {
	ch := l.peek()

	switch ch {
	case '-':
		if l.peekAt(1) == '>' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_ARROW, Value: "->", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		if l.peekAt(1) == '-' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_DECREMENT, Value: "--", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_MINUS, Value: "-", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '+':
		if l.peekAt(1) == '+' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_INCREMENT, Value: "++", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_PLUS, Value: "+", Line: line, Col: col, FileName: l.fileName})
		return nil
	case ':':
		if l.peekAt(1) == ':' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_DOUBLECOLON, Value: "::", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		if l.peekAt(1) == '=' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_ASSIGN, Value: ":=", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_COLON, Value: ":", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '=':
		if l.peekAt(1) == '=' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_EQ, Value: "==", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_ASSIGN, Value: "=", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '!':
		if l.peekAt(1) == '=' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_NEQ, Value: "!=", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_DOT_NOT, Value: "!", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '<':
		if l.peekAt(1) == '=' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_LTE, Value: "<=", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		if l.peekAt(1) == '>' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_NEQ, Value: "<>", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_LT, Value: "<", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '>':
		if l.peekAt(1) == '=' {
			l.advance()
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_GTE, Value: ">=", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_GT, Value: ">", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '*':
		l.advance()
		// `**` é sinônimo Clipper de `^` (exponenciação).
		if l.peek() == '*' {
			l.advance()
			l.tokens = append(l.tokens, Token{Type: TOKEN_CARET, Value: "**", Line: line, Col: col, FileName: l.fileName})
			return nil
		}
		l.tokens = append(l.tokens, Token{Type: TOKEN_STAR, Value: "*", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '/':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_SLASH, Value: "/", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '%':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_PERCENT, Value: "%", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '(':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_LPAREN, Value: "(", Line: line, Col: col, FileName: l.fileName})
		return nil
	case ')':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_RPAREN, Value: ")", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '[':
		// Clipper's bracket-delimited string literal: `[texto]` in operand
		// position (start of expression) is a string, same as "texto".
		// Disambiguation heuristic (the classic Clipper rule): `[` right
		// after a token that ENDS an operand (ident, number, string, `)`,
		// `]`, `}`, `.T.`/`.F.`) is array indexing; anywhere else — after
		// an operator, comma, `(`, `:=`, keyword, or at line start — it
		// begins a string literal running to the next `]` on the same line.
		if !l.lastTokenEndsOperand() {
			// Look ahead (without consuming) for the closing ']' on the
			// same line before committing to the string interpretation.
			end := l.pos + 1
			for end < len(l.source) && l.source[end] != ']' && l.source[end] != '\n' {
				end++
			}
			if end < len(l.source) && l.source[end] == ']' {
				content := l.source[l.pos+1 : end]
				for l.pos <= end { // consume '[', content, ']'
					l.advance()
				}
				l.tokens = append(l.tokens, Token{Type: TOKEN_STRING, Value: content, Line: line, Col: col, FileName: l.fileName})
				return nil
			}
			// No closing ']' on this line — fall through as a plain bracket.
		}
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_LBRACKET, Value: "[", Line: line, Col: col, FileName: l.fileName})
		return nil
	case ']':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_RBRACKET, Value: "]", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '{':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_LBRACE, Value: "{", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '}':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_RBRACE, Value: "}", Line: line, Col: col, FileName: l.fileName})
		return nil
	case ';':
		l.advance()
		// A ';' with only whitespace before the next newline is Clipper/AdvPL
		// line continuation: join with the next physical line instead of
		// emitting a statement separator.
		lookahead := l.pos
		for lookahead < len(l.source) && (l.source[lookahead] == ' ' || l.source[lookahead] == '\t' || l.source[lookahead] == '\r') {
			lookahead++
		}
		// A trailing `// comment` or `&& comment` (Clipper) still counts as
		// end-of-line for continuation.
		if lookahead+1 < len(l.source) &&
			((l.source[lookahead] == '/' && l.source[lookahead+1] == '/') ||
				(l.source[lookahead] == '&' && l.source[lookahead+1] == '&')) {
			for lookahead < len(l.source) && l.source[lookahead] != '\n' {
				lookahead++
			}
		}
		if lookahead >= len(l.source) || l.source[lookahead] == '\n' {
			for l.pos < lookahead {
				l.advance()
			}
			if l.pos < len(l.source) {
				l.advance() // consume the newline itself
			}
			return nil
		}
		l.tokens = append(l.tokens, Token{Type: TOKEN_SEMICOLON, Value: ";", Line: line, Col: col, FileName: l.fileName})
		return nil
	case ',':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_COMMA, Value: ",", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '@':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_AT, Value: "@", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '&':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_AMPERSAND, Value: "&", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '|':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_PIPE, Value: "|", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '^':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_CARET, Value: "^", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '~':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_TILDE, Value: "~", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '$':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_DOLLAR, Value: "$", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '?':
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_QUESTION, Value: "?", Line: line, Col: col, FileName: l.fileName})
		return nil
	case '#':
		// Mid-line '#' (a directive-line '#' never reaches here, see the
		// Tokenize loop's lastTokenIsNewlineOrEOF guard) is Clipper's
		// legacy not-equal operator.
		l.advance()
		l.tokens = append(l.tokens, Token{Type: TOKEN_NEQ, Value: "#", Line: line, Col: col, FileName: l.fileName})
		return nil
	}

	// Backtick não tem significado em AdvPL; fontes reais da TOTVS contêm
	// backticks soltos (typos) que o compilador Protheus tolera — ignora.
	if ch == '`' {
		l.advance()
		return nil
	}

	return fmt.Errorf("unexpected character %q at %s:%d:%d", ch, l.fileName, line, col)
}

func Tokenize(source, fileName string) ([]Token, error) {
	l := NewLexer(source, fileName)
	return l.Tokenize()
}

func FilterTokens(tokens []Token) []Token {
	result := make([]Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Type == TOKEN_NEWLINE || t.Type == TOKEN_LINECOMMENT || t.Type == TOKEN_BLOCKCOMMENT {
			continue
		}
		result = append(result, t)
	}
	return result
}
