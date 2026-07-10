package parser

import (
	"fmt"
	"strings"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/lexer"
)

type Parser struct {
	tokens   []lexer.Token
	pos      int
	fileName string
	defines  map[string]string
}

func NewParser(tokens []lexer.Token, fileName string, defines map[string]string) *Parser {
	tokens = lexer.FilterTokens(tokens)
	return &Parser{
		tokens:   tokens,
		pos:      0,
		fileName: fileName,
		defines:  defines,
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
	for _, kw := range []string{"DESCRIPTION", "PATH", "WSSYNTAX", "PRODUCES", "CONSUMES"} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
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

func (p *Parser) isFunctionBoundary(tok lexer.Token) bool {
	return p.isKeyword(tok, "FUNCTION") || p.isKeyword(tok, "USER") ||
		p.isKeyword(tok, "MAIN") ||
		p.isKeyword(tok, "PROCEDURE") || p.isKeyword(tok, "CLASS") ||
		p.isKeyword(tok, "METHOD") || p.isKeyword(tok, "STATIC") ||
		p.isWSClientOpener(tok) || p.isWord(tok, "WSMETHOD")
}

// isStatementBoundary reports whether tok closes an enclosing block
// (If/For/While/DoCase/Try/Class/...). A bare RETURN/EXIT/BREAK immediately
// followed by one of these must not try to parse it as an expression.
// isEndIf matches ENDIF or the generic Clipper block-closer "End", which
// real code commonly uses to close an If instead of EndIf.
func (p *Parser) isEndIf(tok lexer.Token) bool {
	return p.isKeyword(tok, "ENDIF") || p.isKeyword(tok, "END")
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
			return true
		}
	}
	return false
}

// Parse parses the entire program
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
			nsParts := []string{}
			for p.peek().Type == lexer.TOKEN_IDENT || p.peek().Type == lexer.TOKEN_DOT {
				if p.peek().Type == lexer.TOKEN_DOT {
					p.advance()
					continue
				}
				nsParts = append(nsParts, p.advance().Value)
			}
			prog.Namespace = strings.Join(nsParts, ".")
			continue
		}

		if p.isKeyword(tok, "USING") && p.isKeyword(p.peekAt(1), "NAMESPACE") {
			p.advance() // USING
			p.advance() // NAMESPACE
			nsParts := []string{}
			for p.peek().Type == lexer.TOKEN_IDENT || p.peek().Type == lexer.TOKEN_DOT {
				if p.peek().Type == lexer.TOKEN_DOT {
					p.advance()
					continue
				}
				nsParts = append(nsParts, p.advance().Value)
			}
			prog.UsingNamespaces = append(prog.UsingNamespaces, strings.Join(nsParts, "."))
			continue
		}

		// Top-level annotations
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

	nameTok, err := p.expect(lexer.TOKEN_IDENT)
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

	nameTok, err := p.expect(lexer.TOKEN_IDENT)
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

		if p.isKeyword(tok, "RETURN") {
			p.advance()
			if p.peek().Type != lexer.TOKEN_EOF &&
				p.peek().Type != lexer.TOKEN_SEMICOLON &&
				p.peek().Type != lexer.TOKEN_AT &&
				!p.isStatementBoundary(p.peek()) {
				expr, err := p.parseExpression()
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
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
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
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}
	// WSRESTFUL/WSSERVICE <name> [DESCRIPTION <expr>] [NAMESPACE <expr>] —
	// header clauses, order and presence both vary in real code.
	for p.isWord(p.peek(), "DESCRIPTION") || p.isWord(p.peek(), "NAMESPACE") {
		p.advance()
		if _, err := p.parseOr(); err != nil {
			return nil, err
		}
	}
	class := &ast.ClassDecl{
		Loc:        p.posFromToken(startTok),
		Name:       nameTok.Value,
		Properties: make([]*ast.PropertyDecl, 0),
		Methods:    make([]*ast.MethodDecl, 0),
	}

	for !p.isEndWSClient() && p.peek().Type != lexer.TOKEN_EOF {
		tok := p.peek()
		if p.isWord(tok, "WSMETHOD") || p.isWord(tok, "WSDATA") {
			p.advance()
			// WSRESTFUL spells a method `WSMETHOD GET hasAPI ...` — the
			// HTTP verb (all reserved keywords) comes before the name.
			if strings.EqualFold(tok.Value, "WSMETHOD") {
				for _, verb := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
					if p.isKeyword(p.peek(), verb) {
						p.advance()
						break
					}
				}
			}
			// The method name itself is optional in the REST form — some
			// real code goes straight from the verb into clauses:
			// `WSMETHOD POST DESCRIPTION "..." WSSYNTAX "..."`.
			nameTok := tok
			if !p.isRestMethodClauseWord(p.peek()) {
				var err error
				nameTok, err = p.expectName()
				if err != nil {
					return nil, err
				}
			}
			if strings.EqualFold(tok.Value, "WSMETHOD") {
				// Optional REST metadata clauses: DESCRIPTION/PATH <expr>,
				// PRODUCES/CONSUMES <ident> — parsed and dropped, like the
				// SOAP WSSEND/WSRECEIVE clauses above.
				for {
					switch {
					case p.isWord(p.peek(), "DESCRIPTION"), p.isWord(p.peek(), "PATH"), p.isWord(p.peek(), "WSSYNTAX"):
						p.advance()
						if _, err := p.parseOr(); err != nil {
							return nil, err
						}
					case p.isWord(p.peek(), "PRODUCES"), p.isWord(p.peek(), "CONSUMES"):
						p.advance()
						if _, err := p.expect(lexer.TOKEN_IDENT); err != nil {
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
					Name:   nameTok.Value,
					Params: make([]*ast.Parameter, 0),
				})
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
	// WSRESTFUL spells an implementation `WSMETHOD GET hasAPI WSSERVICE
	// ...` too — same HTTP-verb-before-name shape as the declaration.
	for _, verb := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if p.isKeyword(p.peek(), verb) {
			p.advance()
			break
		}
	}
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}
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
	// Optional `WSSEND arg[,arg...]` / `WSRECEIVE arg[,arg...]` clauses
	// (SOAP request/response binding) between the name and WSCLIENT, in
	// either order and each with a comma-separated arg list. Parsed and
	// dropped — this VM doesn't do real SOAP marshaling.
	for p.isWord(p.peek(), "WSSEND") || p.isWord(p.peek(), "WSRECEIVE") {
		p.advance()
		if _, err := p.parseCommaValues(); err != nil {
			return nil, err
		}
	}
	if p.isWSClientOpener(p.peek()) || p.isWord(p.peek(), "WSREST") {
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
