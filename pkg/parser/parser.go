package parser

import (
	"fmt"
	"strings"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/lexer"
)

const (
	// MaxRecursionDepth prevents stack exhaustion from deeply nested expressions
	MaxRecursionDepth = 1000
)

// Parser builds an abstract syntax tree (AST) from a token stream.
// It enforces AdvPL/TLPP grammar rules and validates structure.
type Parser struct {
	tokens         []lexer.Token
	pos            int
	fileName       string
	defines        map[string]string
	recursionDepth int
	avisos         []Aviso
	// wsClassKind registra o opener (WSCLIENT/WSSTRUCT/WSRESTFUL/WSSERVICE)
	// de cada classe WS* já declarada no arquivo — parseWSMethodImpl usa
	// para saber se a implementação é REST antes de consumir o verbo HTTP.
	wsClassKind map[string]string
}

// Aviso é um problema que não impede a compilação (o Protheus também aceita),
// mas que o `advplc check` mostra com a posição.
type Aviso struct {
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

// Avisos devolve os avisos acumulados pelo Parse, na ordem do fonte.
func (p *Parser) Avisos() []Aviso {
	return p.avisos
}

// ignoraOrfao consome um terminador de bloco sem bloco aberto (EndIf, Next,
// End, EndDo...), que o Protheus real tolera, registrando um aviso.
func (p *Parser) ignoraOrfao() {
	tok := p.advance()
	p.avisos = append(p.avisos, Aviso{Line: tok.Line, Col: tok.Col, Message: tok.Value + " sem bloco aberto (ignorado)"})
}

// NewParser creates a new parser for the given token stream.
// The defines map contains preprocessor defines for conditional parsing.
func NewParser(tokens []lexer.Token, fileName string, defines map[string]string) *Parser {
	tokens = lexer.FilterTokens(tokens)
	return &Parser{
		tokens:      tokens,
		pos:         0,
		fileName:    fileName,
		defines:     defines,
		wsClassKind: make(map[string]string),
	}
}

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TOKEN_EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekAt(offset int) lexer.Token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return lexer.Token{Type: lexer.TOKEN_EOF}
	}
	return p.tokens[idx]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected %v, got %v (%q) at %s:%d:%d",
			tt, tok.Type, tok.Value, tok.FileName, tok.Line, tok.Col)
	}
	p.pos++
	return tok, nil
}

// expectName accepts a name that may collide with a reserved word (e.g. a
// WSMETHOD named "Main" — MAIN is reserved) — anywhere an identifier is
// expected but the reserved-word table might plausibly clash with a real
// user-chosen name.
func (p *Parser) expectName() (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != lexer.TOKEN_IDENT && tok.Type != lexer.TOKEN_KEYWORD {
		return tok, fmt.Errorf("expected name, got %v (%q) at %s:%d:%d",
			tok.Type, tok.Value, tok.FileName, tok.Line, tok.Col)
	}
	p.pos++
	return tok, nil
}

// checkRecursionDepth prevents stack exhaustion from deeply nested expressions.
// Must be called at the start of recursive parsing functions.
func (p *Parser) checkRecursionDepth() error {
	p.recursionDepth++
	if p.recursionDepth > MaxRecursionDepth {
		return fmt.Errorf("maximum recursion depth exceeded (%d) at %s:%d:%d",
			MaxRecursionDepth, p.peek().FileName, p.peek().Line, p.peek().Col)
	}
	return nil
}

// decreaseRecursionDepth should be called when exiting recursive functions.
func (p *Parser) decreaseRecursionDepth() {
	if p.recursionDepth > 0 {
		p.recursionDepth--
	}
}

func (p *Parser) isKeyword(tok lexer.Token, kw string) bool {
	return tok.Type == lexer.TOKEN_KEYWORD && strings.EqualFold(tok.Value, kw)
}

// isWord matches kw whether the lexer classified it as a reserved keyword or
// a plain identifier — clause words in #xcommand-style DSLs (OPTION,
// OPERATION, ACCESS, ...) aren't all in the reserved-word table.
func (p *Parser) isWord(tok lexer.Token, kw string) bool {
	return (tok.Type == lexer.TOKEN_KEYWORD || tok.Type == lexer.TOKEN_IDENT) && strings.EqualFold(tok.Value, kw)
}

// isWSClientOpener matches any spelling of the WSCLIENT-family opener
// keyword (WSCLIENT/WSSTRUCT/WSRESTFUL/WSSERVICE — all TOTVS webservice
// declaration DSLs sharing the same WSMETHOD/WSDATA...END<X> shape).
func (p *Parser) isWSClientOpener(tok lexer.Token) bool {
	for _, kw := range []string{"WSCLIENT", "WSSTRUCT", "WSRESTFUL", "WSSERVICE"} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
}

// isEndWSClient matches the WSCLIENT-family block closer, spelled either
// as one word (ENDWSCLIENT) or two (END WSCLIENT).
func (p *Parser) isEndWSClient() bool {
	if p.isWord(p.peek(), "ENDWSCLIENT") || p.isWord(p.peek(), "ENDWSSTRUCT") ||
		p.isWord(p.peek(), "ENDWSRESTFUL") || p.isWord(p.peek(), "ENDWSSERVICE") {
		return true
	}
	if p.isWord(p.peek(), "END") {
		return p.isWSClientOpener(p.peekAt(1))
	}
	return false
}

// isRestMethodClauseWord matches the REST WSMETHOD metadata clause
// keywords, used to tell whether a name follows the HTTP verb or the
// declaration jumps straight into clauses (`WSMETHOD POST DESCRIPTION ...`).
func (p *Parser) isRestMethodClauseWord(tok lexer.Token) bool {
	for _, kw := range []string{"DESCRIPTION", "PATH", "WSSYNTAX", "PRODUCES", "CONSUMES", "TTALK",
		// clauses reais do corpus 12.1.2510 — sem elas `WSMETHOD GET
		// WSRECEIVE Page WSSERVICE x` (sem nome) lia WSRECEIVE como nome.
		"WSRECEIVE", "WSSEND", "QUERYPARAM", "PATHPARAM", "WSSERVICE", "WSREST", "WSCLIENT",
		"HEADERPARAM"} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
}

// isWSClauseWord é o conjunto de palavras que abrem uma cláusula do DSL
// WS — usado por parseWSNameList para saber onde a lista de nomes de
// WSRECEIVE/WSSEND/QUERYPARAM/PATHPARAM/WSSERVICE termina.
func (p *Parser) isWSClauseWord(tok lexer.Token) bool {
	for _, kw := range []string{
		"WSMETHOD", "WSDATA", "WSRECEIVE", "WSSEND", "QUERYPARAM", "PATHPARAM",
		"HEADERPARAM",
		"WSSERVICE", "WSREST", "WSCLIENT", "DESCRIPTION", "WSSYNTAX", "PATH",
		"PRODUCES", "CONSUMES", "TTALK", "RESPONSE", "END", "ENDWSRESTFUL",
		"ENDWSSERVICE", "ENDWSCLIENT", "ENDWSSTRUCT", "FORMAT", "NAMESPACE",
	} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
}

// parseWSMemberName consome o nome de um membro WS (método WSMETHOD ou
// campo WSDATA). Nomes do corpus 12.1.2510 podem começar com dígito
// (`WSMETHOD GET 1customers ...`), o que o lexer parte em NUMBER+IDENT
// colados — refaz o nome juntando tokens adjacentes na mesma linha.
// `WSMETHOD GET 0001 DESCRIPTION` continua "0001": DESCRIPTION não é
// adjacente (há espaço), então não é incorporado.
func (p *Parser) parseWSMemberName() (lexer.Token, error) {
	tok := p.peek()
	if tok.Type != lexer.TOKEN_NUMBER {
		return p.expectName()
	}
	p.advance()
	name := tok.Value
	endCol := tok.Col + len(tok.Value)
	for {
		n := p.peek()
		if (n.Type != lexer.TOKEN_IDENT && n.Type != lexer.TOKEN_KEYWORD) ||
			n.Line != tok.Line || n.Col != endCol {
			break
		}
		name += n.Value
		endCol = n.Col + len(n.Value)
		p.advance()
	}
	tok.Value = name
	return tok, nil
}

// parseWSNameList consome uma lista de nomes separada por vírgulas do DSL
// WS (`WSRECEIVE branchId, productionOrder, ...`). Itens podem ser
// IDENT, KEYWORD (`order`, `date`, `status` são nomes de parâmetro
// comuns) ou NUMBER; para no primeiro token que não é nome ou que é uma
// palavra de cláusula.
func (p *Parser) parseWSNameList() {
	// afterComma: depois de uma vírgula a lista EXIGE um item — uma
	// palavra de cláusula nessa posição é nome de parâmetro, não
	// terminador (`QUERYPARAM Page, PageSize, format WSSERVICE x` do
	// corpus 2510 usa `format` como parâmetro). Sem vírgula anterior,
	// palavra de cláusula encerra a lista normalmente.
	afterComma := false
	for {
		t := p.peek()
		if t.Type != lexer.TOKEN_IDENT && t.Type != lexer.TOKEN_KEYWORD && t.Type != lexer.TOKEN_NUMBER {
			return
		}
		if p.isWSClauseWord(t) && !afterComma {
			return
		}
		p.advance()
		if p.peek().Type != lexer.TOKEN_COMMA {
			return
		}
		p.advance()
		afterComma = true
	}
}

// wsHTTPVerbs são os verbos HTTP aceitos na forma REST do WSMETHOD
// (`WSMETHOD GET hasAPI ...`), compartilhados pela declaração
// (parseWSClient) e pela implementação (parseWSMethodImpl).
var wsHTTPVerbs = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

// isWSClassBoundary reporta se o próximo token fecha o bloco WS* ou abre
// outro membro dele — usado para reconhecer `WSMETHOD GET` sem nome
// seguido de ENDWSRESTFUL/END WSRESTFUL, outro WSMETHOD ou WSDATA.
func (p *Parser) isWSClassBoundary() bool {
	tok := p.peek()
	if tok.Type == lexer.TOKEN_EOF {
		return true
	}
	if p.isWord(tok, "WSMETHOD") || p.isWord(tok, "WSDATA") {
		return true
	}
	return p.isEndWSClient()
}

func (p *Parser) matchKeyword(kw string) bool {
	if p.isKeyword(p.peek(), kw) {
		p.pos++
		return true
	}
	return false
}

func (p *Parser) posFromToken(tok lexer.Token) ast.Position {
	return ast.Position{Line: tok.Line, Col: tok.Col, FileName: tok.FileName}
}

// parseNamespacePath consumes a dotted namespace path (`totvs.framework.x`)
// after NAMESPACE/USING NAMESPACE. A segment can collide with a reserved
// word (`date`, etc. lex as TOKEN_KEYWORD), so both IDENT and KEYWORD are
// accepted — but only right after a DOT (or as the first segment), never as
// a bare follow-on token, so this can't run past the declaration into the
// next real statement (e.g. `NAMESPACE totvs` \n `Function Foo()`).
func (p *Parser) parseNamespacePath() []string {
	parts := []string{}
	if p.peek().Type != lexer.TOKEN_IDENT && p.peek().Type != lexer.TOKEN_KEYWORD {
		return parts
	}
	parts = append(parts, p.advance().Value)
	for p.peek().Type == lexer.TOKEN_DOT &&
		(p.peekAt(1).Type == lexer.TOKEN_IDENT || p.peekAt(1).Type == lexer.TOKEN_KEYWORD) {
		p.advance()
		parts = append(parts, p.advance().Value)
	}
	return parts
}

func (p *Parser) isFunctionBoundary(tok lexer.Token) bool {
	// STATIC só abre função quando seguido de FUNCTION — `Static cVar := x`
	// dentro de um corpo é declaração de variável, não boundary. (Todos os
	// chamadores passam p.peek(), então olhar p.peekAt(1) é seguro.)
	if p.isKeyword(tok, "STATIC") {
		return p.isKeyword(p.peekAt(1), "FUNCTION")
	}
	return p.isKeyword(tok, "FUNCTION") || p.isKeyword(tok, "USER") ||
		p.isKeyword(tok, "MAIN") ||
		p.isKeyword(tok, "PROCEDURE") || p.isKeyword(tok, "CLASS") ||
		p.isKeyword(tok, "METHOD") ||
		p.isWSClientOpener(tok) || p.isWord(tok, "WSMETHOD")
}

// isStatementBoundary reports whether tok closes an enclosing block
// (If/For/While/DoCase/Try/Class/...). A bare RETURN/EXIT/BREAK immediately
// followed by one of these must not try to parse it as an expression.
// isEndIf matches ENDIF or the generic Clipper block-closer "End", which
// real code commonly uses to close an If instead of EndIf.
func (p *Parser) isEndIf(tok lexer.Token) bool {
	if p.isKeyword(tok, "ENDIF") {
		return true
	}
	if p.isKeyword(tok, "END") {
		// `END REPORT QUERY <var>` — not an EndIf, it's a statement.
		if p.isWord(p.peekAt(1), "REPORT") {
			return false
		}
		return true
	}
	return false
}

func (p *Parser) isStatementBoundary(tok lexer.Token) bool {
	if p.isFunctionBoundary(tok) {
		return true
	}
	if p.isWord(tok, "ENDFOR") {
		return true
	}
	for _, kw := range []string{
		"ENDIF", "ELSE", "ELSEIF",
		"NEXT", "ENDDO", "END",
		"CASE", "OTHERWISE", "ENDCASE",
		"RECOVER", "ENDTRY", "FINALLY",
		"ENDCLASS", "ENDINTERFACE",
	} {
		if p.isKeyword(tok, kw) {
			// `END REPORT QUERY <var>` — not a block terminator, handled
			// as a statement in parseStatement.
			if kw == "END" && p.isWord(p.peekAt(1), "REPORT") {
				return false
			}
			return true
		}
	}
	return false
}

// Parse parses the entire program
// Parse parses the token stream and returns an abstract syntax tree.
// Returns an error if syntax is invalid or recursion depth is exceeded.
func (p *Parser) Parse() (*ast.Program, error) {
	prog := &ast.Program{
		Loc:       ast.Position{FileName: p.fileName},
		FileName:  p.fileName,
		Defines:   p.defines,
		Functions: make([]*ast.FunctionDecl, 0),
		Classes:   make([]*ast.ClassDecl, 0),
		Methods:   make([]*ast.MethodImpl, 0),
		Body:      make([]ast.Statement, 0),
	}

	var pendingAnnotations []*ast.Annotation

	for p.peek().Type != lexer.TOKEN_EOF {
		tok := p.peek()

		if tok.Type >= lexer.TOKEN_PREPROC_INCLUDE && tok.Type <= lexer.TOKEN_PREPROC_UNDEFINE {
			if tok.Type == lexer.TOKEN_PREPROC_INCLUDE {
				prog.Includes = append(prog.Includes, tok.Value)
			}
			p.advance()
			continue
		}

		if tok.Type == lexer.TOKEN_DIRECTIVE {
			p.advance()
			continue
		}

		if p.isKeyword(tok, "NAMESPACE") {
			p.advance()
			prog.Namespace = strings.Join(p.parseNamespacePath(), ".")
			continue
		}

		if p.isKeyword(tok, "USING") && p.isKeyword(p.peekAt(1), "NAMESPACE") {
			p.advance() // USING
			p.advance() // NAMESPACE
			prog.UsingNamespaces = append(prog.UsingNamespaces, strings.Join(p.parseNamespacePath(), "."))
			continue
		}

		// Top-level annotations (`@Get`, `@Post(...)`) — but real legacy
		// code can also have `@ x,y TO x2,y2 ...` screen-drawing statements
		// at top level (implicit Main, no enclosing Function), e.g.
		// `@0,0 TO 380,520 DIALOG ...`. An annotation name is never a bare
		// number, and is never immediately followed by a comma (that's the
		// x,y coordinate pair's separator) — use both to tell them apart.
		if tok.Type == lexer.TOKEN_AT && p.peekAt(1).Type != lexer.TOKEN_NUMBER &&
			p.peekAt(2).Type != lexer.TOKEN_COMMA {
			p.advance()
			annNameTok := p.advance()
			annValue := ""
			if p.peek().Type == lexer.TOKEN_LPAREN {
				p.advance()
				var sb strings.Builder
				for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
					sb.WriteString(p.advance().Value)
				}
				if p.peek().Type == lexer.TOKEN_RPAREN {
					p.advance()
				}
				annValue = sb.String()
			}
			ann := &ast.Annotation{Loc: p.posFromToken(tok), Name: annNameTok.Value, Value: annValue}
			pendingAnnotations = append(pendingAnnotations, ann)
			continue
		}

		// `Main Function Nome()` — forma clássica do Clipper/AdvPL para
		// marcar o ponto de entrada do programa; tratada como equivalente
		// a User Function (IsUser=true), já que é o mesmo papel de "função
		// invocada automaticamente quando não há statements de topo".
		if p.isKeyword(tok, "MAIN") && p.isKeyword(p.peekAt(1), "FUNCTION") {
			// parseFunction(isUser=true) já pula um token antes de exigir
			// FUNCTION (pensado para "USER FUNCTION"); não consumir MAIN
			// aqui manualmente, ou dois tokens seriam pulados.
			fn, err := p.parseFunction(true, false)
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				fn.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			prog.Functions = append(prog.Functions, fn)
			continue
		}

		if p.isKeyword(tok, "USER") {
			fn, err := p.parseFunction(true, false)
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				fn.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			prog.Functions = append(prog.Functions, fn)
			continue
		}

		if p.isKeyword(tok, "STATIC") && p.isKeyword(p.peekAt(1), "FUNCTION") {
			p.advance()
			fn, err := p.parseFunction(false, true)
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				fn.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			prog.Functions = append(prog.Functions, fn)
			continue
		}

		if p.isKeyword(tok, "FUNCTION") {
			fn, err := p.parseFunction(false, false)
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				fn.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			prog.Functions = append(prog.Functions, fn)
			continue
		}

		if p.isKeyword(tok, "PROCEDURE") {
			fn, err := p.parseProcedure()
			if err != nil {
				return nil, err
			}
			prog.Functions = append(prog.Functions, fn)
			continue
		}

		if p.isKeyword(tok, "CLASS") {
			class, err := p.parseClass()
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				class.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			prog.Classes = append(prog.Classes, class)
			continue
		}

		if p.isKeyword(tok, "METHOD") {
			method, err := p.parseMethodImpl()
			if err != nil {
				return nil, err
			}
			prog.Methods = append(prog.Methods, method)
			continue
		}

		if p.isWSClientOpener(tok) {
			class, err := p.parseWSClient()
			if err != nil {
				return nil, err
			}
			// Anotações `@` pendentes entram antes das anotações internas
			// do DSL (WSRESTFUL/WSMETHOD), como em parseClass.
			if len(pendingAnnotations) > 0 {
				class.Annotations = append(pendingAnnotations, class.Annotations...)
				pendingAnnotations = nil
			}
			prog.Classes = append(prog.Classes, class)
			continue
		}

		if p.isWord(tok, "WSMETHOD") {
			method, err := p.parseWSMethodImpl()
			if err != nil {
				return nil, err
			}
			prog.Methods = append(prog.Methods, method)
			continue
		}

		if p.isKeyword(tok, "OPERATOR") {
			method, err := p.parseOperatorImpl()
			if err != nil {
				return nil, err
			}
			prog.Methods = append(prog.Methods, method)
			continue
		}

		if p.isKeyword(tok, "INTERFACE") {
			p.advance()
			nameTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			iface := &ast.InterfaceDecl{Loc: p.posFromToken(tok), Name: nameTok.Value}
			for !p.isKeyword(p.peek(), "ENDINTERFACE") && p.peek().Type != lexer.TOKEN_EOF {
				if p.isKeyword(p.peek(), "METHOD") {
					m, err := p.parseMethodDecl()
					if err != nil {
						return nil, err
					}
					iface.Methods = append(iface.Methods, m)
				} else {
					p.advance()
				}
			}
			if p.isKeyword(p.peek(), "ENDINTERFACE") {
				p.advance()
			}
			prog.Body = append(prog.Body, iface)
			continue
		}

		// Terminador órfão fora de função (ex.: EndIf depois do Return).
		if p.isStatementBoundary(tok) && !p.isFunctionBoundary(tok) {
			p.ignoraOrfao()
			continue
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			prog.Body = append(prog.Body, stmt)
		}
	}

	return prog, nil
}

func (p *Parser) parseFunction(isUser, isStatic bool) (*ast.FunctionDecl, error) {
	startTok := p.peek()
	if isUser {
		p.advance()
	}
	if _, err := p.expect(lexer.TOKEN_KEYWORD); err != nil {
		return nil, err
	}

	// nome de função pode colidir com keyword (`Static Function Add`)
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	fn := &ast.FunctionDecl{
		Loc:      p.posFromToken(startTok),
		Name:     nameTok.Value,
		IsUser:   isUser,
		IsStatic: isStatic,
		Params:   make([]*ast.Parameter, 0),
	}

	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			fn.Params = append(fn.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		fn.ReturnType = p.parseTypeName()
	}

	body, retExpr, err := p.parseFunctionBody()
	if err != nil {
		return nil, err
	}
	fn.Body = body
	fn.ReturnExpr = retExpr
	return fn, nil
}

func (p *Parser) parseProcedure() (*ast.FunctionDecl, error) {
	startTok := p.advance()
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}

	fn := &ast.FunctionDecl{
		Loc:         p.posFromToken(startTok),
		Name:        nameTok.Value,
		IsProcedure: true,
		Params:      make([]*ast.Parameter, 0),
	}

	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			fn.Params = append(fn.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}

	body, retExpr, err := p.parseFunctionBody()
	if err != nil {
		return nil, err
	}
	fn.Body = body
	fn.ReturnExpr = retExpr
	return fn, nil
}

func (p *Parser) parseParameter() (*ast.Parameter, error) {
	tok := p.peek()
	byRef := false
	if tok.Type == lexer.TOKEN_AT {
		byRef = true
		p.advance()
	}

	// nome de parâmetro pode colidir com keyword (`GetSU5(oApi, Self)`)
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	param := &ast.Parameter{
		Loc:   p.posFromToken(tok),
		Name:  nameTok.Value,
		ByRef: byRef,
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		param.Type = p.parseTypeName()
	}

	if p.isKeyword(p.peek(), "DEFAULT") {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		param.Default = val
	}

	return param, nil
}

func (p *Parser) parseTypeName() string {
	tok := p.peek()
	if tok.Type == lexer.TOKEN_KEYWORD || tok.Type == lexer.TOKEN_IDENT {
		p.advance()
		return tok.Value
	}
	return ""
}

func (p *Parser) parseFunctionBody() ([]ast.Statement, ast.Expression, error) {
	var body []ast.Statement
	var retExpr ast.Expression

	for p.peek().Type != lexer.TOKEN_EOF && !p.isFunctionBoundary(p.peek()) {
		tok := p.peek()

		// Orphaned block terminators (EndIf, End, EndDo, Next, etc.) sem
		// bloco aberto correspondente — o Protheus real tolera. Consome e
		// ignora em vez de falhar com "unexpected token".
		if p.isStatementBoundary(tok) && !p.isFunctionBoundary(tok) {
			p.ignoraOrfao()
			continue
		}

		if p.isKeyword(tok, "RETURN") {
			p.advance()
			if p.peek().Type != lexer.TOKEN_EOF &&
				p.peek().Type != lexer.TOKEN_SEMICOLON &&
				p.peek().Type != lexer.TOKEN_AT &&
				!p.isStatementBoundary(p.peek()) {
				// `Return target := value` — see the same fix in
				// expressions.go's top-level RETURN handling.
				expr, err := p.parseAssignRHS()
				if err != nil {
					return body, nil, err
				}
				retExpr = expr
			}
			break
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return body, nil, err
		}
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	return body, retExpr, nil
}

func (p *Parser) parseClass() (*ast.ClassDecl, error) {
	startTok := p.advance()
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}

	class := &ast.ClassDecl{
		Loc:        p.posFromToken(startTok),
		Name:       nameTok.Value,
		Properties: make([]*ast.PropertyDecl, 0),
		Methods:    make([]*ast.MethodDecl, 0),
	}

	if p.isKeyword(p.peek(), "FROM") {
		p.advance()
		parentTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		class.Parent = parentTok.Value
	}

	var pendingAnnotations []*ast.Annotation

	for !p.isKeyword(p.peek(), "ENDCLASS") && p.peek().Type != lexer.TOKEN_EOF {
		tok := p.peek()

		if p.isKeyword(tok, "DATA") {
			p.advance()
			prop, err := p.parseProperty()
			if err != nil {
				return nil, err
			}
			class.Properties = append(class.Properties, prop)
			continue
		}

		if p.isKeyword(tok, "METHOD") {
			method, err := p.parseMethodDecl()
			if err != nil {
				return nil, err
			}
			if len(pendingAnnotations) > 0 {
				method.Annotations = pendingAnnotations
				pendingAnnotations = nil
			}
			class.Methods = append(class.Methods, method)
			continue
		}

		if p.isKeyword(tok, "PUBLIC") || p.isKeyword(tok, "PRIVATE") || p.isKeyword(tok, "PROTECTED") {
			modifier := strings.ToUpper(tok.Value)
			p.advance()

			if p.isKeyword(p.peek(), "DATA") {
				p.advance()
				prop, err := p.parseProperty()
				if err != nil {
					return nil, err
				}
				switch modifier {
				case "PUBLIC":
					prop.IsPublic = true
				case "PRIVATE":
					prop.IsPrivate = true
				case "PROTECTED":
					prop.IsProtected = true
				}
				class.Properties = append(class.Properties, prop)
				continue
			}

			if p.isKeyword(p.peek(), "METHOD") {
				method, err := p.parseMethodDecl()
				if err != nil {
					return nil, err
				}
				// `@GET(...) public method x()` — a anotação pendente vale
				// para método com modificador igual ao `method` puro
				// (controladores REST TLPP do corpus 12.1.2510).
				if len(pendingAnnotations) > 0 {
					method.Annotations = pendingAnnotations
					pendingAnnotations = nil
				}
				switch modifier {
				case "PUBLIC":
					method.IsPublic = true
				case "PRIVATE":
					method.IsPrivate = true
				case "PROTECTED":
					method.IsProtected = true
				}
				class.Methods = append(class.Methods, method)
				continue
			}
		}

		if tok.Type == lexer.TOKEN_AT {
			p.advance()
			annNameTok := p.advance()
			annValue := ""
			if p.peek().Type == lexer.TOKEN_LPAREN {
				p.advance()
				var sb strings.Builder
				for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
					sb.WriteString(p.advance().Value)
				}
				if p.peek().Type == lexer.TOKEN_RPAREN {
					p.advance()
				}
				annValue = sb.String()
			}
			ann := &ast.Annotation{Loc: p.posFromToken(tok), Name: annNameTok.Value, Value: annValue}
			pendingAnnotations = append(pendingAnnotations, ann)
			continue
		}

		p.advance()
	}

	if p.isKeyword(p.peek(), "ENDCLASS") {
		p.advance()
	}

	return class, nil
}

func (p *Parser) parseProperty() (*ast.PropertyDecl, error) {
	tok := p.peek()
	// Nome de membro pode colidir com palavra reservada (`Data size`,
	// `Data default`) — aceita keyword também.
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}

	prop := &ast.PropertyDecl{
		Loc:  p.posFromToken(tok),
		Name: nameTok.Value,
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		prop.Type = p.parseTypeName()
	}

	return prop, nil
}

func (p *Parser) parseMethodDecl() (*ast.MethodDecl, error) {
	startTok := p.advance()
	// Method name can be a keyword (e.g., ADD, DELETE)
	nameTok := p.peek()
	if nameTok.Type != lexer.TOKEN_IDENT && nameTok.Type != lexer.TOKEN_KEYWORD {
		return nil, fmt.Errorf("expected method name, got %v (%q) at %s:%d:%d",
			nameTok.Type, nameTok.Value, nameTok.FileName, nameTok.Line, nameTok.Col)
	}
	p.advance()

	method := &ast.MethodDecl{
		Loc:    p.posFromToken(startTok),
		Name:   nameTok.Value,
		Params: make([]*ast.Parameter, 0),
	}

	if p.isKeyword(p.peek(), "CONSTRUCTOR") {
		method.IsConstructor = true
		p.advance()
	}

	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			method.Params = append(method.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		method.ReturnType = p.parseTypeName()
	}

	// `Method New(...) Constructor` — real code puts it after the params
	// as often as before (checked above the param list too).
	if p.isKeyword(p.peek(), "CONSTRUCTOR") {
		method.IsConstructor = true
		p.advance()
	}

	return method, nil
}

func (p *Parser) parseMethodImpl() (*ast.MethodImpl, error) {
	startTok := p.advance()
	nameTok := p.peek()
	if nameTok.Type != lexer.TOKEN_IDENT && nameTok.Type != lexer.TOKEN_KEYWORD {
		return nil, fmt.Errorf("expected method name, got %v (%q) at %s:%d:%d",
			nameTok.Type, nameTok.Value, nameTok.FileName, nameTok.Line, nameTok.Col)
	}
	p.advance()

	method := &ast.MethodImpl{
		Loc:    p.posFromToken(startTok),
		Name:   nameTok.Value,
		Params: make([]*ast.Parameter, 0),
	}

	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			method.Params = append(method.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		method.ReturnType = p.parseTypeName()
	}

	if p.isKeyword(p.peek(), "CLASS") {
		p.advance()
		classTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		method.ClassName = classTok.Value
	}

	// `method nome() class Fulano as Tipo` — a anotação de tipo de retorno
	// também aparece DEPOIS de "class ClassName" em código real (ordem
	// inversa do caso já tratado acima, antes do "class").
	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		method.ReturnType = p.parseTypeName()
	}

	body, retExpr, err := p.parseFunctionBody()
	if err != nil {
		return nil, err
	}
	method.Body = body
	method.ReturnExpr = retExpr
	return method, nil
}

// parseWSClient handles the WSCLIENT/WSSTRUCT/WSRESTFUL declarative DSL for
// webservice consumer stubs (TOTVS WSDL/REST client codegen), e.g.:
//
//	WSCLIENT MyService
//	    WSMETHOD DoThing
//	    WSDATA   cField AS String
//	ENDWSCLIENT
//
// Structurally the same as CLASS/ENDCLASS — WSMETHOD prototypes and WSDATA
// fields map onto ast.ClassDecl's Methods/Properties so the rest of the
// compiler doesn't need to know this DSL exists.
func (p *Parser) parseWSClient() (*ast.ClassDecl, error) {
	startTok := p.advance() // WSCLIENT / WSSTRUCT / WSRESTFUL
	// Nome pode colidir com keyword (`WSRESTFUL Data`, RHNP02 do corpus
	// 2510) — expectName aceita IDENT e KEYWORD.
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}
	// WSRESTFUL/WSSERVICE <name> [DESCRIPTION <expr>] [NAMESPACE <expr>]
	// [FORMAT <expr>] — header clauses, order and presence both vary in
	// real code. Corpus 2510 também usa SECURITY/API/OBJECT/AUTH.
	for p.isWord(p.peek(), "DESCRIPTION") || p.isWord(p.peek(), "NAMESPACE") || p.isWord(p.peek(), "FORMAT") ||
		p.isWord(p.peek(), "SECURITY") || p.isWord(p.peek(), "API") || p.isWord(p.peek(), "OBJECT") ||
		p.isWord(p.peek(), "AUTH") {
		p.advance()
		if _, err := p.parseOr(); err != nil {
			return nil, err
		}
	}
	// WSRESTFUL é o DSL de serviço REST do Protheus: o compilador precisa
	// saber disso para montar as rotas (verbo+PATH de cada WSMETHOD vão em
	// anotações da classe — ClassInfo.Annotations no bytecode).
	isRESTFUL := p.isWord(startTok, "WSRESTFUL")
	// Registra o tipo da classe para parseWSMethodImpl decidir se a
	// implementação é REST (verbo HTTP) ou WSCLIENT/WSSTRUCT comum.
	p.wsClassKind[strings.ToUpper(nameTok.Value)] = strings.ToUpper(startTok.Value)
	class := &ast.ClassDecl{
		Loc:        p.posFromToken(startTok),
		Name:       nameTok.Value,
		Properties: make([]*ast.PropertyDecl, 0),
		Methods:    make([]*ast.MethodDecl, 0),
	}
	if isRESTFUL {
		class.Annotations = append(class.Annotations, &ast.Annotation{
			Loc:  class.Loc,
			Name: "WSRESTFUL",
		})
	}

	for !p.isEndWSClient() && p.peek().Type != lexer.TOKEN_EOF {
		tok := p.peek()
		if p.isWord(tok, "WSMETHOD") || p.isWord(tok, "WSDATA") {
			p.advance()
			isWSMethod := strings.EqualFold(tok.Value, "WSMETHOD")
			// Só WSRESTFUL tem verbo HTTP na frente do nome
			// (`WSMETHOD GET hasAPI ...`). Em WSCLIENT/WSSTRUCT/WSSERVICE,
			// GET/POST/etc. são nomes de método comuns — não consumir.
			var verb string
			if isWSMethod && isRESTFUL {
				for _, v := range wsHTTPVerbs {
					if p.isKeyword(p.peek(), v) {
						verb = v
						p.advance()
						break
					}
				}
				if verb == "" {
					p.avisos = append(p.avisos, Aviso{
						Line:    tok.Line,
						Col:     tok.Col,
						Message: "WSMETHOD em WSRESTFUL sem verbo HTTP",
					})
				}
			}
			// The method name itself is optional in the REST form — some
			// real code goes straight from the verb into clauses:
			// `WSMETHOD POST DESCRIPTION "..." WSSYNTAX "..."`. Também é
			// sem nome quando o verbo é seguido direto do fim da classe ou
			// de outro membro (`WSMETHOD GET` + ENDWSRESTFUL/WSMETHOD...).
			// WSDATA has no such form — a field is always named, even when
			// that name happens to collide with a WSMETHOD clause word
			// like "Description" (`WSDATA Description As String`). Fora de
			// WSRESTFUL não existe forma sem nome: toda palavra (mesmo
			// PATH/DESCRIPTION) é o nome do método.
			unnamedREST := isRESTFUL && isWSMethod &&
				(p.isRestMethodClauseWord(p.peek()) || p.isWSClassBoundary())
			nameTok := tok
			named := true
			if strings.EqualFold(tok.Value, "WSDATA") || !unnamedREST {
				var err error
				// Nome de método WS pode ser numérico (`WSMETHOD GET 0001
				// DESCRIPTION ...`, TAFWSGARBAGE do corpus 2510) ou começar
				// com dígito (`1customers` — NUMBER+IDENT colados).
				if isWSMethod && p.peek().Type == lexer.TOKEN_NUMBER {
					nameTok, err = p.parseWSMemberName()
				} else {
					nameTok, err = p.expectName()
				}
				if err != nil {
					return nil, err
				}
			} else {
				named = false
			}
			// Método REST sem nome chama-se pelo verbo ("GET", "POST"...) —
			// antes virava "WSMETHOD" aqui e "REST" na implementação, o que
			// fazia dois métodos sem nome colidirem.
			declName := nameTok.Value
			if isWSMethod && !named && verb != "" {
				declName = verb
			}
			var path string
			if isWSMethod {
				// Optional REST metadata clauses: DESCRIPTION/WSSYNTAX
				// <expr>, PATH <expr> (só literal string é guardado),
				// PRODUCES/CONSUMES <ident> — parsed and dropped, like the
				// SOAP WSSEND/WSRECEIVE clauses above.
				for {
					switch {
					case p.isWord(p.peek(), "PATH"):
						pathTok := p.peek()
						p.advance()
						expr, err := p.parseOr()
						if err != nil {
							return nil, err
						}
						// Só WSRESTFUL guarda PATH (vira anotação); nos demais
						// DSLs a cláusula é descartada como antes, sem aviso.
						if !isRESTFUL {
							break
						}
						if lit, ok := expr.(*ast.StringLit); ok {
							path = lit.Value
						} else {
							p.avisos = append(p.avisos, Aviso{
								Line:    pathTok.Line,
								Col:     pathTok.Col,
								Message: "PATH de WSMETHOD não é literal string (ignorado)",
							})
						}
					case p.isWord(p.peek(), "DESCRIPTION"), p.isWord(p.peek(), "WSSYNTAX"):
						p.advance()
						if _, err := p.parseOr(); err != nil {
							return nil, err
						}
					case p.isWord(p.peek(), "PRODUCES"), p.isWord(p.peek(), "CONSUMES"):
						p.advance()
						// Payload é expressão: ident (`APPLICATION_JSON`)
						// OU string literal (`'application/json;
						// charset=utf-8'`, corpus 2510) — e pode vir em
						// lista separada por vírgula.
						for {
							if _, err := p.parseOr(); err != nil {
								return nil, err
							}
							if p.peek().Type != lexer.TOKEN_COMMA {
								break
							}
							p.advance()
						}
						// `PRODUCES APPLICATION_JSON RESPONSE EaiObj` — RESPONSE
						// (o objeto de retorno) é opcional e vem colado logo
						// depois do valor de PRODUCES/CONSUMES, mesma linha.
						if p.isWord(p.peek(), "RESPONSE") {
							p.advance()
							if _, err := p.expectName(); err != nil {
								return nil, err
							}
						}
					case p.isWord(p.peek(), "WSRECEIVE"), p.isWord(p.peek(), "WSSEND"),
						p.isWord(p.peek(), "QUERYPARAM"), p.isWord(p.peek(), "PATHPARAM"),
						p.isWord(p.peek(), "HEADERPARAM"):
						// `WSRECEIVE branchId, productionOrder, ...` — lista de
						// nomes separados por vírgula (corpus 2510).
						p.advance()
						p.parseWSNameList()
					case p.isWord(p.peek(), "WSSERVICE"), p.isWord(p.peek(), "WSREST"),
						p.isWord(p.peek(), "WSCLIENT"):
						// `WSSERVICE productionorders` / `WSREST Auth` — um nome.
						p.advance()
						p.parseWSNameList()
					case p.isWord(p.peek(), "TTALK"):
						p.advance()
						if _, err := p.parseOr(); err != nil {
							return nil, err
						}
					default:
						goto doneClauses
					}
				}
			doneClauses:
			}
			if strings.EqualFold(tok.Value, "WSDATA") {
				prop := &ast.PropertyDecl{Loc: p.posFromToken(nameTok), Name: nameTok.Value}
				if p.isKeyword(p.peek(), "AS") {
					p.advance()
					prop.Type = p.parseTypeName()
					if p.isWord(p.peek(), "OF") { // `Array of String`
						p.advance()
						prop.Type += " of " + p.parseTypeName()
					}
					if p.isWord(p.peek(), "OPTIONAL") {
						p.advance()
					}
				}
				class.Properties = append(class.Properties, prop)
			} else {
				class.Methods = append(class.Methods, &ast.MethodDecl{
					Loc:    p.posFromToken(nameTok),
					Name:   declName,
					Params: make([]*ast.Parameter, 0),
				})
				if isRESTFUL {
					// "<VERBO>|<NomeDoMetodo>|<PATH>" — o dispatch REST lê
					// estas anotações (ClassInfo.Annotations no bytecode).
					class.Annotations = append(class.Annotations, &ast.Annotation{
						Loc:   p.posFromToken(tok),
						Name:  "WSMETHOD",
						Value: verb + "|" + declName + "|" + path,
					})
				}
			}
			continue
		}
		// Unrecognized WS* clause: real WSCLIENT bodies (TOTVS-generated)
		// only ever contain WSMETHOD/WSDATA, so stop rather than guess how
		// far to skip — newline tokens are already filtered out by this
		// point, so there's no safe "skip to end of line".
		break
	}
	if p.isWord(p.peek(), "END") {
		p.advance()
		p.advance() // WSCLIENT / WSSTRUCT / WSRESTFUL
	} else if p.peek().Type != lexer.TOKEN_EOF {
		p.advance() // ENDWSCLIENT / ENDWSSTRUCT / ENDWSRESTFUL
	}
	return class, nil
}

// wsImplIsREST determina, antes de consumir qualquer token do cabeçalho,
// se a implementação WSMETHOD em curso é REST: procura o primeiro
// linking de classe à frente (WSREST / WSSERVICE / WSCLIENT / WSSTRUCT /
// WSRESTFUL) e resolve o tipo da classe alvo em wsClassKind. WSREST é
// sempre REST; WSCLIENT/WSSTRUCT nunca são; WSSERVICE/WSRESTFUL dependem
// do tipo declarado da classe — desconhecida (outro arquivo), assume REST,
// que é o caso de uso majoritário do linkage WSSERVICE.
// wsImplScanMax limita a varredura de wsImplIsREST: o cabeçalho de um
// WSMETHOD é curto (verbo, nome, cláusulas, linkage), então passar de
// ~128 tokens sem achar o opener significa que não há linkage à frente
// (implementação sem classe) — trata como não-REST em vez de varrer o
// resto do arquivo.
const wsImplScanMax = 128

func (p *Parser) wsImplIsREST() bool {
	limite := p.pos + wsImplScanMax
	for i := p.pos; i < len(p.tokens) && i < limite; i++ {
		tok := p.tokens[i]
		if tok.Type == lexer.TOKEN_EOF {
			return false
		}
		if p.isWord(tok, "WSREST") {
			return true
		}
		if p.isWord(tok, "WSCLIENT") || p.isWord(tok, "WSSTRUCT") {
			return false
		}
		if p.isWord(tok, "WSSERVICE") || p.isWord(tok, "WSRESTFUL") {
			if i+1 < len(p.tokens) {
				if kind, ok := p.wsClassKind[strings.ToUpper(p.tokens[i+1].Value)]; ok {
					return kind == "WSRESTFUL"
				}
			}
			return true
		}
	}
	return false
}

// parseWSMethodImpl handles a WSMETHOD implementation:
//
//	WSMETHOD Init WSCLIENT MyService
//	    ...body...
//	Return
//
// Same shape as `METHOD name() class ClassName`, just spelled with WSMETHOD
// and WSCLIENT and (in practice) no parameter list.
func (p *Parser) parseWSMethodImpl() (*ast.MethodImpl, error) {
	startTok := p.advance() // WSMETHOD
	// Só implementação REST (classe WSRESTFUL, ou linkage WSREST/WSSERVICE
	// apontando para ela) tem verbo HTTP antes do nome —
	// `WSMETHOD GET hasAPI WSSERVICE ...`. Em WSCLIENT/WSSTRUCT, GET/POST
	// etc. são nomes de método comuns e não podem ser consumidos aqui
	// (`WSMETHOD GET(...) WSCLIENT Foo` é um método chamado GET).
	verb := ""
	if p.wsImplIsREST() {
		for _, v := range wsHTTPVerbs {
			// `WSMETHOD GET(...)` — o verbo seguido de "(" é o próprio nome
			// do método com lista de parâmetros, não um verbo HTTP.
			if p.isKeyword(p.peek(), v) && p.peekAt(1).Type != lexer.TOKEN_LPAREN {
				verb = v
				p.advance()
				break
			}
		}
	}
	// `WSMETHOD GET WSRECEIVE a,b WSSERVICE X` — REST sem nome próprio
	// (só o verbo HTTP): se o que vem depois do verbo já é uma cláusula,
	// o nome do método é o próprio verbo (antes era "REST", o que fazia
	// dois métodos sem nome na mesma classe colidirem).
	var methodName string
	if p.isWord(p.peek(), "WSRECEIVE") || p.isWord(p.peek(), "WSSEND") ||
		p.isWord(p.peek(), "PATHPARAM") || p.isWord(p.peek(), "QUERYPARAM") ||
		p.isWSClientOpener(p.peek()) || p.isWord(p.peek(), "WSREST") {
		methodName = verb
		if methodName == "" {
			methodName = "REST"
		}
	} else if p.peek().Type == lexer.TOKEN_NUMBER {
		// `WSMETHOD GET 0001 WSSERVICE X` / `1customers` — nome numérico
		// ou iniciando com dígito (corpus 2510).
		nameTok, err := p.parseWSMemberName()
		if err != nil {
			return nil, err
		}
		methodName = nameTok.Value
	} else {
		nameTok, err := p.expectName()
		if err != nil {
			return nil, err
		}
		methodName = nameTok.Value
	}
	method := &ast.MethodImpl{
		Loc:    p.posFromToken(startTok),
		Name:   methodName,
		Params: make([]*ast.Parameter, 0),
	}
	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			method.Params = append(method.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}
	// Cláusulas de binding do DSL WS em QUALQUER ordem e quantas vezes
	// couber (`WSRECEIVE a,b QUERYPARAM c WSSERVICE X`, `PATHPARAM ...
	// QUERYPARAM ...` — o corpus 2510 usa todas as combinações). Cada
	// cláusula de lista consome nomes separados por vírgula; WSSERVICE/
	// WSREST/WSCLIENT consomem um nome e viram ClassName do método.
	for {
		switch {
		case p.isWord(p.peek(), "PATHPARAM") || p.isWord(p.peek(), "QUERYPARAM") ||
			p.isWord(p.peek(), "HEADERPARAM") ||
			p.isWord(p.peek(), "WSRECEIVE") || p.isWord(p.peek(), "WSSEND"):
			p.advance()
			p.parseWSNameList()
		case p.isWSClientOpener(p.peek()) || p.isWord(p.peek(), "WSREST"):
			p.advance()
			classTok, err := p.expectName()
			if err != nil {
				return nil, err
			}
			method.ClassName = classTok.Value
		default:
			goto doneWSClauses
		}
	}
doneWSClauses:
	body, retExpr, err := p.parseFunctionBody()
	if err != nil {
		return nil, err
	}
	method.Body = body
	method.ReturnExpr = retExpr
	return method, nil
}

func (p *Parser) parseOperatorImpl() (*ast.MethodImpl, error) {
	startTok := p.advance() // consume OPERATOR
	opTok := p.peek()
	if opTok.Type != lexer.TOKEN_IDENT && opTok.Type != lexer.TOKEN_KEYWORD {
		return nil, fmt.Errorf("expected operator name (Add/Sub/Mult/Div/Compare/ToString), got %v (%q)",
			opTok.Type, opTok.Value)
	}
	p.advance()

	method := &ast.MethodImpl{
		Loc:    p.posFromToken(startTok),
		Name:   "OPERATOR_" + strings.ToUpper(opTok.Value),
		Params: make([]*ast.Parameter, 0),
	}

	if p.peek().Type == lexer.TOKEN_LPAREN {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
			param, err := p.parseParameter()
			if err != nil {
				return nil, err
			}
			method.Params = append(method.Params, param)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		p.advance()
	}

	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		method.ReturnType = p.parseTypeName()
	}

	if p.isKeyword(p.peek(), "CLASS") {
		p.advance()
		classTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		method.ClassName = classTok.Value
	}

	body, retExpr, err := p.parseFunctionBody()
	if err != nil {
		return nil, err
	}
	method.Body = body
	method.ReturnExpr = retExpr
	return method, nil
}
