package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/lexer"
)

func (p *Parser) parseStatement() (ast.Statement, error) {
	// `;` also serves as a same-line statement separator (`Return ; EndIf`
	// isn't continuation — the lexer only treats a trailing ';' before a
	// newline as that). Skip empty statements between separators. Only
	// short-circuit on a block terminator if a ';' was actually consumed:
	// most caller loops only re-check their own boundary keyword, not every
	// possible one, so bailing out here without consuming anything would
	// spin forever on a boundary a caller's loop condition doesn't expect.
	skippedSemi := false
	for p.peek().Type == lexer.TOKEN_SEMICOLON {
		p.advance()
		skippedSemi = true
	}
	if skippedSemi && (p.peek().Type == lexer.TOKEN_EOF || p.isStatementBoundary(p.peek())) {
		return nil, nil
	}
	tok := p.peek()

	if p.isKeyword(tok, "LOCAL") || p.isKeyword(tok, "PRIVATE") || p.isKeyword(tok, "PUBLIC") || p.isKeyword(tok, "STATIC") {
		return p.parseVarDecl()
	}
	if p.isKeyword(tok, "IF") {
		return p.parseIf()
	}
	if p.isKeyword(tok, "FOR") {
		return p.parseFor()
	}
	if p.isKeyword(tok, "WHILE") {
		return p.parseWhile()
	}
	if p.isKeyword(tok, "DO") && p.isKeyword(p.peekAt(1), "WHILE") {
		p.advance() // DO — parseWhile expects to start right at WHILE
		return p.parseWhile()
	}
	if p.isKeyword(tok, "DO") && p.isKeyword(p.peekAt(1), "CASE") {
		return p.parseDoCase()
	}
	if p.isWord(tok, "SET") && p.peekAt(1).Type == lexer.TOKEN_IDENT &&
		(p.isKeyword(p.peekAt(2), "TO") || p.isWord(p.peekAt(2), "ON") || p.isWord(p.peekAt(2), "OFF")) {
		return p.parseSetCommand()
	}
	if p.isKeyword(tok, "ADD") && p.isWord(p.peekAt(1), "OPTION") {
		return p.parseAddOption()
	}
	if p.isWord(tok, "DEFINE") {
		return p.parseDefine()
	}
	if p.isWord(tok, "PUBLISH") && p.isWord(p.peekAt(1), "MODEL") {
		return p.parsePublishModel()
	}
	if p.isKeyword(tok, "ACTIVATE") && p.isWord(p.peekAt(1), "MSDIALOG") {
		return p.parseActivateDialog()
	}
	if tok.Type == lexer.TOKEN_AT {
		return p.parseAtCommand()
	}
	if p.isKeyword(tok, "RETURN") {
		p.advance()
		var retExpr ast.Expression
		if p.peek().Type != lexer.TOKEN_EOF && p.peek().Type != lexer.TOKEN_SEMICOLON && !p.isStatementBoundary(p.peek()) {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			retExpr = expr
		}
		return &ast.ReturnStmt{Loc: p.posFromToken(tok), Value: retExpr}, nil
	}
	if p.isKeyword(tok, "EXIT") {
		p.advance()
		return &ast.ExitStmt{Loc: p.posFromToken(tok)}, nil
	}
	if p.isKeyword(tok, "LOOP") {
		p.advance()
		return &ast.LoopStmt{Loc: p.posFromToken(tok)}, nil
	}
	if p.isKeyword(tok, "BREAK") {
		p.advance()
		var val ast.Expression
		if p.peek().Type != lexer.TOKEN_EOF && p.peek().Type != lexer.TOKEN_SEMICOLON && !p.isStatementBoundary(p.peek()) {
			val, _ = p.parseExpression()
		}
		return &ast.BreakStmt{Loc: p.posFromToken(tok), Value: val}, nil
	}
	if p.isKeyword(tok, "BEGIN") && p.isKeyword(p.peekAt(1), "SEQUENCE") {
		return p.parseBeginSequence()
	}
	if p.isKeyword(tok, "BEGIN") && p.isWord(p.peekAt(1), "TRANSACTION") {
		return p.parseBeginTransaction()
	}
	if p.isKeyword(tok, "BEGIN") && p.isWord(p.peekAt(1), "WSMETHOD") {
		return p.parseBeginEndBlock("WSMETHOD")
	}
	if p.isKeyword(tok, "BEGIN") && p.isKeyword(p.peekAt(1), "REPORT") && p.isWord(p.peekAt(2), "QUERY") {
		return p.parseBeginReportQuery()
	}
	if p.isKeyword(tok, "TRY") {
		return p.parseTryCatch()
	}
	if p.isKeyword(tok, "THROW") {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.ThrowStmt{Loc: p.posFromToken(tok), Value: val}, nil
	}
	if p.isKeyword(tok, "DEFAULT") {
		return p.parseDefault()
	}

	return p.parseExprStatement()
}

// parseArrayDeclSize handles `[10]`, `[3][2]`, and `[3,2]` right after a
// declared variable name — Clipper's fixed-size array declaration, sugar
// for `name := Array(10)` / `Array(3,2)`. Returns nil if there's no bracket.
func (p *Parser) parseArrayDeclSize(nameTok lexer.Token) (ast.Expression, error) {
	if p.peek().Type != lexer.TOKEN_LBRACKET {
		return nil, nil
	}
	var dims []ast.Expression
	for p.peek().Type == lexer.TOKEN_LBRACKET {
		p.advance()
		for p.peek().Type != lexer.TOKEN_RBRACKET && p.peek().Type != lexer.TOKEN_EOF {
			dim, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			dims = append(dims, dim)
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		if _, err := p.expect(lexer.TOKEN_RBRACKET); err != nil {
			return nil, err
		}
	}
	return &ast.CallExpr{Loc: p.posFromToken(nameTok), Name: "ARRAY", Args: dims}, nil
}

func (p *Parser) parseVarDecl() (ast.Statement, error) {
	tok := p.advance()
	scope := strings.ToLower(tok.Value)

	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}

	decl := &ast.VarDecl{
		Loc:   p.posFromToken(tok),
		Scope: scope,
		Name:  nameTok.Value,
	}

	// Fixed-size array declaration: `Local aArr[10]` / `aArr[3][2]` /
	// `aArr[3,2]` — sugar for `aArr := Array(10)` etc.
	if arr, err := p.parseArrayDeclSize(nameTok); err != nil {
		return nil, err
	} else if arr != nil {
		decl.Value = arr
	}

	// Check for 'as Type' before ':=' (TLPP allows: Local x as numeric := 42)
	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		decl.Type = p.parseTypeName()
	}

	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		decl.Value = val
	}

	// Also check for 'as Type' after ':=' (AdvPL style: Local x := 42 as numeric)
	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		decl.Type = p.parseTypeName()
	}

	// Handle comma-separated declarations (`Local a, b := 1, c`) — each gets
	// its own VarDecl; a single-var line still returns a bare *VarDecl so
	// existing callers/tests see no shape change for the common case.
	decls := []*ast.VarDecl{decl}
	for p.peek().Type == lexer.TOKEN_COMMA {
		p.advance()
		if p.peek().Type != lexer.TOKEN_IDENT {
			break
		}
		extraName := p.advance()
		extraDecl := &ast.VarDecl{
			Loc:   p.posFromToken(extraName),
			Scope: scope,
			Name:  extraName.Value,
		}
		if arr, err := p.parseArrayDeclSize(extraName); err != nil {
			return nil, err
		} else if arr != nil {
			extraDecl.Value = arr
		}
		if p.isKeyword(p.peek(), "AS") {
			p.advance()
			extraDecl.Type = p.parseTypeName()
		}
		if p.peek().Type == lexer.TOKEN_ASSIGN {
			p.advance()
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			extraDecl.Value = val
		}
		if p.isKeyword(p.peek(), "AS") {
			p.advance()
			extraDecl.Type = p.parseTypeName()
		}
		decls = append(decls, extraDecl)
	}

	if len(decls) == 1 {
		return decl, nil
	}
	return &ast.VarDeclGroup{Loc: p.posFromToken(tok), Decls: decls}, nil
}

func (p *Parser) parseIf() (ast.Statement, error) {
	startTok := p.advance()

	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	ifStmt := &ast.IfStmt{
		Loc:       p.posFromToken(startTok),
		Condition: cond,
		ThenBody:  make([]ast.Statement, 0),
		ElseIfs:   make([]*ast.ElseIfClause, 0),
	}

	for !p.isKeyword(p.peek(), "ELSEIF") && !p.isKeyword(p.peek(), "ELSE") &&
		!p.isEndIf(p.peek()) && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			ifStmt.ThenBody = append(ifStmt.ThenBody, stmt)
		}
	}

	for p.isKeyword(p.peek(), "ELSEIF") {
		p.advance()
		elseifCond, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		clause := &ast.ElseIfClause{Loc: p.posFromToken(startTok), Condition: elseifCond, Body: make([]ast.Statement, 0)}
		for !p.isKeyword(p.peek(), "ELSEIF") && !p.isKeyword(p.peek(), "ELSE") &&
			!p.isEndIf(p.peek()) && p.peek().Type != lexer.TOKEN_EOF {
			if p.isFunctionBoundary(p.peek()) {
				break
			}
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				clause.Body = append(clause.Body, stmt)
			}
		}
		ifStmt.ElseIfs = append(ifStmt.ElseIfs, clause)
	}

	if p.isKeyword(p.peek(), "ELSE") {
		p.advance()
		ifStmt.ElseBody = make([]ast.Statement, 0)
		for !p.isEndIf(p.peek()) && p.peek().Type != lexer.TOKEN_EOF {
			if p.isFunctionBoundary(p.peek()) {
				break
			}
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				ifStmt.ElseBody = append(ifStmt.ElseBody, stmt)
			}
		}
	}

	if p.isEndIf(p.peek()) {
		p.advance()
	}

	return ifStmt, nil
}

func (p *Parser) parseFor() (ast.Statement, error) {
	startTok := p.advance()
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}

	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
	} else if p.peek().Type == lexer.TOKEN_EQ {
		p.advance()
	}

	startExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if !p.matchKeyword("TO") {
		return nil, fmt.Errorf("expected TO in For at %s:%d:%d", startTok.FileName, startTok.Line, startTok.Col)
	}

	endExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	var stepExpr ast.Expression
	if p.isKeyword(p.peek(), "STEP") {
		p.advance()
		stepExpr, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	forStmt := &ast.ForStmt{
		Loc: p.posFromToken(startTok), VarName: nameTok.Value,
		Start: startExpr, End: endExpr, Step: stepExpr,
		Body: make([]ast.Statement, 0),
	}

	for !p.isKeyword(p.peek(), "NEXT") && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			forStmt.Body = append(forStmt.Body, stmt)
		}
	}

	if p.isKeyword(p.peek(), "NEXT") {
		nextTok := p.advance()
		// `Next nX` (optional loop-var name) only counts on the SAME
		// PHYSICAL LINE as `Next` — NEWLINE tokens are filtered out before
		// the parser ever sees the stream, so this is the only way to tell
		// "Next nX" apart from a bare "Next" immediately followed by an
		// unrelated later statement that happens to target a variable with
		// the same name (`Next` \n `nX := 0` inside the loop's own body —
		// real, reproducible bug: this used to eat that `nX` as if it were
		// the loop-var echo, leaving a bare `:= 0` for the next statement).
		if p.peek().Type == lexer.TOKEN_IDENT && p.peek().Line == nextTok.Line &&
			strings.EqualFold(p.peek().Value, forStmt.VarName) {
			p.advance()
		}
	}

	return forStmt, nil
}

func (p *Parser) parseWhile() (ast.Statement, error) {
	startTok := p.advance()

	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	// Rare `While cond Do ... EndDo` form has `Do` trailing the condition
	// on the same line. Without the same-line check this swallows the
	// `Do` off `Do Case` when that's the loop's first body statement —
	// real, reproducible bug (`While lCont` / `Do Case` on the next line).
	if p.isKeyword(p.peek(), "DO") && p.peek().Line == p.peekAt(-1).Line {
		p.advance()
	}

	whileStmt := &ast.WhileStmt{
		Loc: p.posFromToken(startTok), Condition: cond,
		Body: make([]ast.Statement, 0),
	}

	for !p.isKeyword(p.peek(), "ENDDO") && !p.isKeyword(p.peek(), "END") && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			whileStmt.Body = append(whileStmt.Body, stmt)
		}
	}

	if p.isKeyword(p.peek(), "ENDDO") {
		p.advance()
	} else if p.isKeyword(p.peek(), "END") {
		endTok := p.advance()
		// "End" alone, "End Do", or "End While" all close a While — but
		// only if DO/WHILE is on the same line as "End". Otherwise a bare
		// "End" immediately followed by a *new* `While ...` statement on
		// the next line would wrongly eat that statement's own `While`.
		if (p.isKeyword(p.peek(), "DO") || p.isKeyword(p.peek(), "WHILE")) && p.peek().Line == endTok.Line {
			p.advance()
		}
	}

	return whileStmt, nil
}

func (p *Parser) parseDoCase() (ast.Statement, error) {
	p.advance() // DO
	p.advance() // CASE

	doCase := &ast.DoCaseStmt{Loc: p.posFromToken(p.peek()), Cases: make([]*ast.CaseClause, 0)}

	for p.isKeyword(p.peek(), "CASE") {
		p.advance()
		cond, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		clause := &ast.CaseClause{Loc: p.posFromToken(p.peek()), Condition: cond, Body: make([]ast.Statement, 0)}
		for !p.isKeyword(p.peek(), "CASE") && !p.isKeyword(p.peek(), "OTHERWISE") &&
			!p.isKeyword(p.peek(), "ENDCASE") && p.peek().Type != lexer.TOKEN_EOF {
			if p.isFunctionBoundary(p.peek()) {
				break
			}
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				clause.Body = append(clause.Body, stmt)
			}
		}
		doCase.Cases = append(doCase.Cases, clause)
	}

	if p.isKeyword(p.peek(), "OTHERWISE") {
		p.advance()
		doCase.Otherwise = make([]ast.Statement, 0)
		for !p.isKeyword(p.peek(), "ENDCASE") && p.peek().Type != lexer.TOKEN_EOF {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				doCase.Otherwise = append(doCase.Otherwise, stmt)
			}
		}
	}

	if p.isKeyword(p.peek(), "ENDCASE") {
		p.advance()
	}

	return doCase, nil
}

func (p *Parser) parseBeginSequence() (ast.Statement, error) {
	startTok := p.advance() // BEGIN
	p.advance()             // SEQUENCE

	bs := &ast.BeginSequenceStmt{Loc: p.posFromToken(startTok), Body: make([]ast.Statement, 0)}

	for !p.isKeyword(p.peek(), "RECOVER") && !p.isKeyword(p.peek(), "END") && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			bs.Body = append(bs.Body, stmt)
		}
	}

	if p.isKeyword(p.peek(), "RECOVER") {
		p.advance()
		if p.isKeyword(p.peek(), "USING") {
			p.advance()
			varTok, _ := p.expect(lexer.TOKEN_IDENT)
			bs.UsingVar = varTok.Value
		}
		bs.RecoverBody = make([]ast.Statement, 0)
		for !p.isKeyword(p.peek(), "END") && p.peek().Type != lexer.TOKEN_EOF {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				bs.RecoverBody = append(bs.RecoverBody, stmt)
			}
		}
	}

	if p.isKeyword(p.peek(), "END") {
		p.advance()
		if p.isKeyword(p.peek(), "SEQUENCE") {
			p.advance()
		}
	}

	return bs, nil
}

// parseBeginTransaction handles `Begin Transaction ... End Transaction`.
// Real commit/rollback semantics need DB-layer support this interpreter
// doesn't have yet, so this just runs the body as a plain block for now.
func (p *Parser) parseBeginTransaction() (ast.Statement, error) {
	return p.parseBeginEndBlock("TRANSACTION")
}

// parseBeginEndBlock handles `BEGIN <marker> ... END <marker>` — plain
// scope/region markers (Begin Transaction, Begin WSMethod) with no control
// flow of their own; the body just runs as a straight sequence.
func (p *Parser) parseBeginEndBlock(marker string) (ast.Statement, error) {
	startTok := p.advance() // BEGIN
	p.advance()             // marker keyword

	bs := &ast.BeginSequenceStmt{Loc: p.posFromToken(startTok), Body: make([]ast.Statement, 0)}
	for !(p.isKeyword(p.peek(), "END")) && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			bs.Body = append(bs.Body, stmt)
		}
	}
	if p.isKeyword(p.peek(), "END") {
		endTok := p.advance()
		// Same-line guard as parseWhile's End Do/While: a bare "End"
		// immediately followed by a new `WSMethod ...` declaration on the
		// next line must not eat that declaration's own marker word.
		if p.isWord(p.peek(), marker) && p.peek().Line == endTok.Line {
			p.advance()
		}
	}
	return bs, nil
}

// parseBeginReportQuery handles TReport's query-definition block:
//
//	BEGIN REPORT QUERY oSection1
//	    ...body (often a BeginSql block)...
//	END REPORT QUERY oSection1
//
// Same "plain sequence, no control flow" shape as parseBeginEndBlock, just
// with a two-word marker and an optional trailing section-var name on
// both ends.
func (p *Parser) parseBeginReportQuery() (ast.Statement, error) {
	startTok := p.advance() // BEGIN
	p.advance()             // REPORT
	p.advance()             // QUERY
	if p.peek().Type == lexer.TOKEN_IDENT {
		p.advance() // section var
	}

	isEndReportQuery := func() bool {
		return p.isKeyword(p.peek(), "END") && p.isKeyword(p.peekAt(1), "REPORT") && p.isWord(p.peekAt(2), "QUERY")
	}

	bs := &ast.BeginSequenceStmt{Loc: p.posFromToken(startTok), Body: make([]ast.Statement, 0)}
	for !isEndReportQuery() && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			bs.Body = append(bs.Body, stmt)
		}
	}
	if isEndReportQuery() {
		p.advance()
		p.advance()
		queryTok := p.advance()
		// Same-line guard: don't eat a genuinely new statement's leading
		// identifier as if it were the trailing section-var echo.
		if p.peek().Type == lexer.TOKEN_IDENT && p.peek().Line == queryTok.Line {
			p.advance()
		}
	}
	return bs, nil
}

func (p *Parser) parseTryCatch() (ast.Statement, error) {
	startTok := p.advance()

	tc := &ast.TryCatchStmt{Loc: p.posFromToken(startTok), Body: make([]ast.Statement, 0)}

	for !p.isKeyword(p.peek(), "CATCH") && !p.isKeyword(p.peek(), "FINALLY") &&
		!p.isKeyword(p.peek(), "ENDTRY") && p.peek().Type != lexer.TOKEN_EOF {
		if p.isFunctionBoundary(p.peek()) {
			break
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			tc.Body = append(tc.Body, stmt)
		}
	}

	if p.isKeyword(p.peek(), "CATCH") {
		p.advance()
		varTok, _ := p.expect(lexer.TOKEN_IDENT)
		tc.CatchVar = varTok.Value
		tc.CatchBody = make([]ast.Statement, 0)
		for !p.isKeyword(p.peek(), "FINALLY") && !p.isKeyword(p.peek(), "ENDTRY") && p.peek().Type != lexer.TOKEN_EOF {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				tc.CatchBody = append(tc.CatchBody, stmt)
			}
		}
	}

	if p.isKeyword(p.peek(), "FINALLY") {
		p.advance()
		tc.FinallyBody = make([]ast.Statement, 0)
		for !p.isKeyword(p.peek(), "ENDTRY") && p.peek().Type != lexer.TOKEN_EOF {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				tc.FinallyBody = append(tc.FinallyBody, stmt)
			}
		}
	}

	if p.isKeyword(p.peek(), "ENDTRY") {
		p.advance()
	}

	return tc, nil
}

func (p *Parser) parseDefault() (ast.Statement, error) {
	tok := p.advance()
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}
	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
	}
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{
		Loc:  p.posFromToken(tok),
		Expr: &ast.DefaultExpr{Loc: p.posFromToken(tok), Name: nameTok.Value, Value: val},
	}, nil
}

// parseAddOption handles the MenuDef() `#xcommand` idiom:
//
//	ADD OPTION aRotina TITLE "x" ACTION "Y" OPERATION 2 ACCESS 0
//
// desugared to AAdd(aRotina, {Title, Action, 0, Operation, Access, ...}),
// the standard aRotina row shape. Clause order/presence varies in real code,
// so clauses are collected by keyword rather than assumed positional.
func (p *Parser) parseAddOption() (ast.Statement, error) {
	tok := p.advance() // ADD
	p.advance()         // OPTION

	arrTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}
	arr := &ast.Ident{Loc: p.posFromToken(arrTok), Name: arrTok.Value}

	clauses := map[string]ast.Expression{}
	for {
		cur := p.peek()
		var name string
		switch {
		case p.isKeyword(cur, "TITLE"):
			name = "TITLE"
		case p.isKeyword(cur, "ACTION"):
			name = "ACTION"
		case p.isWord(cur, "OPERATION"):
			name = "OPERATION"
		case p.isWord(cur, "ACCESS"):
			name = "ACCESS"
		case p.isWord(cur, "DISABLE"):
			name = "DISABLE"
		case p.isWord(cur, "ID"):
			name = "ID"
		case p.isWord(cur, "TOOLBAR"):
			name = "TOOLBAR"
		default:
			goto done
		}
		p.advance()
		val, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		clauses[name] = val
	}
done:
	nilExpr := func() ast.Expression { return &ast.NilLit{Loc: p.posFromToken(tok)} }
	field := func(name string) ast.Expression {
		if v, ok := clauses[name]; ok {
			return v
		}
		return nilExpr()
	}
	elems := []ast.Expression{
		field("TITLE"), field("ACTION"),
		&ast.NumberLit{Loc: p.posFromToken(tok), Value: 0, Str: "0"},
		field("OPERATION"), field("ACCESS"),
	}
	for _, name := range []string{"DISABLE", "ID", "TOOLBAR"} {
		if v, ok := clauses[name]; ok {
			elems = append(elems, v)
		}
	}

	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "AADD", Args: []ast.Expression{arr, &ast.ArrayLit{Loc: p.posFromToken(tok), Elements: elems}}}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseDefine handles the classic dialog-construction DSL:
//
//	DEFINE MSDIALOG var TITLE "x" FROM 0,0 TO 480,640 OF oWnd PIXEL
//
// desugared to `var := MSDIALOG(x1,y1,x2,y2,title)`. This interpreter has no
// real dialog engine behind it yet, so the goal here is just to consume the
// clauses so `check` succeeds — not full runtime fidelity.
// parsePublishModel handles the MVC REST-publishing declaration:
//
//	PUBLISH MODEL REST NAME FISA160D SOURCE FISA160D RESOURCE OBJECT oModel
//
// Desugars to a dropped `PUBLISH_MODEL(name, source, resource)` call — this
// interpreter doesn't have a real REST dispatch layer to register with.
func (p *Parser) parsePublishModel() (ast.Statement, error) {
	tok := p.advance() // PUBLISH
	p.advance()         // MODEL
	if p.isKeyword(p.peek(), "REST") {
		p.advance()
	}

	clauses := map[string]string{}
	for {
		switch {
		case p.isWord(p.peek(), "NAME"):
			p.advance()
			nameTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			clauses["NAME"] = nameTok.Value
		case p.isWord(p.peek(), "SOURCE"):
			p.advance()
			srcTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			clauses["SOURCE"] = srcTok.Value
		case p.isWord(p.peek(), "RESOURCE") && p.isKeyword(p.peekAt(1), "OBJECT"):
			p.advance()
			p.advance()
			objTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			clauses["RESOURCE"] = objTok.Value
		default:
			goto done
		}
	}
done:
	toArg := func(name string) ast.Expression {
		if v, ok := clauses[name]; ok {
			return &ast.StringLit{Loc: p.posFromToken(tok), Value: v}
		}
		return &ast.NilLit{Loc: p.posFromToken(tok)}
	}
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "PUBLISH_MODEL",
		Args: []ast.Expression{toArg("NAME"), toArg("SOURCE"), toArg("RESOURCE")}}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// isDefineClauseWord matches DEFINE's clause keywords, used to tell whether
// the token right after DEFINE <kind> is a target variable or the DSL jumps
// straight into clauses (`DEFINE CELL NAME "x" OF oSection ...`).
func (p *Parser) isDefineClauseWord(tok lexer.Token) bool {
	for _, kw := range []string{
		"TITLE", "FROM", "TO", "OF", "PIXEL", "ENABLE", "DISABLE", "COLOR",
		"STYLE", "ICON", "NAME", "SIZE", "TYPE", "ACTION", "ALIAS",
		"BOLD", "ITALIC", "UNDERLINE", "PARAMETER", "PARAMETERS", "DESCRIPTION",
	} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
}

func (p *Parser) parseDefine() (ast.Statement, error) {
	tok := p.advance()     // DEFINE
	kindTok := p.advance() // MSDIALOG / WINDOW / FONT / ...
	kind := strings.ToUpper(kindTok.Value)

	// The target var is usually present (`DEFINE MSDIALOG oDlg ...`), but
	// e.g. `DEFINE SBUTTON FROM ... ACTION ...` / `DEFINE CELL NAME "x" OF
	// oSection ...` (no variable, straight into clauses) don't have one —
	// only treat the next identifier as a target if it isn't itself a
	// clause word.
	var target *ast.Ident
	if p.peek().Type == lexer.TOKEN_IDENT && !p.isDefineClauseWord(p.peek()) {
		varTok := p.advance()
		target = &ast.Ident{Loc: p.posFromToken(varTok), Name: varTok.Value}
	}

	clauses := map[string][]ast.Expression{}
	for {
		cur := p.peek()
		var name string
		switch {
		case p.isKeyword(cur, "TITLE"):
			name = "TITLE"
		case p.isKeyword(cur, "FROM"):
			name = "FROM"
		case p.isKeyword(cur, "TO"):
			name = "TO"
		case p.isKeyword(cur, "OF"):
			name = "OF"
		case p.isKeyword(cur, "PIXEL"), p.isWord(cur, "ENABLE"), p.isWord(cur, "DISABLE"):
			p.advance() // flag clauses, no value
			continue
		case p.isWord(cur, "COLOR"):
			name = "COLOR"
		case p.isWord(cur, "STYLE"):
			name = "STYLE"
		case p.isWord(cur, "ICON"):
			name = "ICON"
		case p.isWord(cur, "NAME"):
			name = "NAME"
		case p.isKeyword(cur, "SIZE"):
			name = "SIZE"
		case p.isWord(cur, "TYPE"):
			name = "TYPE"
		case p.isKeyword(cur, "ACTION"):
			name = "ACTION"
		case p.isWord(cur, "ALIAS"):
			name = "ALIAS"
		case p.isKeyword(cur, "PARAMETER"), p.isKeyword(cur, "PARAMETERS"):
			name = "PARAMETER"
		case p.isWord(cur, "DESCRIPTION"):
			name = "DESCRIPTION"
		case p.isWord(cur, "BOLD"), p.isWord(cur, "ITALIC"), p.isWord(cur, "UNDERLINE"):
			p.advance() // DEFINE FONT style flags, no value
			continue
		default:
			goto done
		}
		p.advance()
		vals, err := p.parseCommaValues()
		if err != nil {
			return nil, err
		}
		clauses[name] = vals
	}
done:
	nilExpr := func() ast.Expression { return &ast.NilLit{Loc: p.posFromToken(tok)} }
	at := func(name string, i int) ast.Expression {
		if v, ok := clauses[name]; ok && i < len(v) {
			return v[i]
		}
		return nilExpr()
	}
	args := []ast.Expression{at("FROM", 0), at("FROM", 1), at("TO", 0), at("TO", 1), at("TITLE", 0)}
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: kind, Args: args}
	if target == nil {
		return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
	}
	return &ast.AssignStmt{Loc: p.posFromToken(tok), Target: target, Value: call, Op: ":="}, nil
}

// parseCommaValues parses one expression, then any further comma-separated
// expressions (e.g. the "x,y" in `FROM x,y`).
func (p *Parser) parseCommaValues() ([]ast.Expression, error) {
	first, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	vals := []ast.Expression{first}
	for p.peek().Type == lexer.TOKEN_COMMA {
		p.advance()
		v, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// parseSetCommand handles Clipper's `SET <option> TO [<value>]` /
// `SET <option> ON|OFF` family (SET DEVICE TO SCREEN, SET FILTER TO ...,
// SET DELETED ON, ...), desugaring to a dropped `SET_<OPTION>(value)` call —
// this interpreter doesn't model any of these runtime options.
func (p *Parser) parseSetCommand() (ast.Statement, error) {
	tok := p.advance() // SET
	nameTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{}
	if p.isKeyword(p.peek(), "TO") {
		p.advance()
		// `SET FILTER TO` with nothing after (clearing the option) is also
		// common; only a STRING/NUMBER/LPAREN is unambiguously a value. A
		// bare IDENT is only treated as one if it isn't itself the start of
		// the next `SET ...` statement.
		hasValue := false
		switch p.peek().Type {
		case lexer.TOKEN_STRING, lexer.TOKEN_NUMBER, lexer.TOKEN_LPAREN:
			hasValue = true
		case lexer.TOKEN_IDENT:
			hasValue = !p.isWord(p.peek(), "SET")
		}
		if hasValue {
			vals, err := p.parseCommaValues()
			if err != nil {
				return nil, err
			}
			args = vals
		}
	} else if p.isWord(p.peek(), "ON") || p.isWord(p.peek(), "OFF") {
		args = append(args, &ast.BoolLit{Loc: p.posFromToken(tok), Value: p.isWord(p.peek(), "ON")})
		p.advance()
	}
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "SET_" + strings.ToUpper(nameTok.Value), Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseAtCommand handles the `@ x,y VERB ...` screen-positioning DSL
// (SAY/GET/BUTTON/...), desugaring to a no-op-ish `AT_<VERB>(x,y,mainValue)`
// call so `check` succeeds. Like parseDefine, this doesn't have a rendering
// engine behind it — clauses are parsed and dropped, not threaded through.
func (p *Parser) parseAtCommand() (ast.Statement, error) {
	tok := p.advance() // @
	x, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == lexer.TOKEN_COMMA {
		p.advance()
	}
	y, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	// `@ x1,y1 TO x2,y2 [BOX expr] [...]` draws a box/rectangle — a second
	// coordinate pair instead of a SAY/GET/BUTTON verb.
	if p.isKeyword(p.peek(), "TO") {
		p.advance()
		x2, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
		y2, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args := []ast.Expression{x, y, x2, y2}
		if p.isWord(p.peek(), "BOX") {
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		for p.isAtClauseWord(p.peek()) {
			clauseTok := p.advance()
			if p.isWord(clauseTok, "PIXEL") || p.isWord(clauseTok, "CLICKFOCUS") || p.isWord(clauseTok, "RESET") || p.isWord(clauseTok, "MEMO") {
				continue
			}
			vals, err := p.parseCommaValues()
			if err != nil {
				return nil, err
			}
			args = append(args, vals...)
		}
		call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "AT_BOX", Args: args}
		return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
	}

	verbTok := p.peek()
	verb := strings.ToUpper(verbTok.Value)
	if verbTok.Type != lexer.TOKEN_KEYWORD && verbTok.Type != lexer.TOKEN_IDENT {
		return nil, fmt.Errorf("expected @ command verb (SAY/GET/BUTTON/...), got %q at %s:%d:%d",
			verbTok.Value, verbTok.FileName, verbTok.Line, verbTok.Col)
	}
	p.advance()

	args := []ast.Expression{x, y}
	if !p.isAtClauseWord(p.peek()) && p.peek().Type != lexer.TOKEN_EOF && !p.isStatementBoundary(p.peek()) {
		mainVal, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		// `@ x,y GET var` precisa do NOME da variável para o diálogo web
		// escrever o valor digitado de volta (via FunctionInfo.LocalNames);
		// vai como string antes do valor: AT_GET(x, y, "cNome", cNome, ...)
		if verb == "GET" {
			name := ""
			if ident, ok := mainVal.(*ast.Ident); ok {
				name = ident.Name
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(tok), Value: name})
		}
		args = append(args, mainVal)
	}

	for p.isAtClauseWord(p.peek()) {
		clauseTok := p.advance()
		if p.isWord(clauseTok, "PIXEL") || p.isWord(clauseTok, "CLICKFOCUS") || p.isWord(clauseTok, "RESET") || p.isWord(clauseTok, "MEMO") {
			continue // flag clauses, no value
		}
		clauseName := strings.ToUpper(clauseTok.Value)
		vals, err := p.parseCommaValues()
		if err != nil {
			return nil, err
		}
		// VALID/WHEN/ACTION são lazy no Protheus (#xcommand embrulha em
		// codeblock); replica isso para não avaliar a expressão na hora
		// de montar o controle
		if clauseName == "VALID" || clauseName == "WHEN" || clauseName == "ACTION" {
			for i, v := range vals {
				if _, isBlock := v.(*ast.CodeBlock); !isBlock {
					vals[i] = &ast.CodeBlock{Loc: p.posFromToken(clauseTok), Expr: v}
				}
			}
		}
		// etiqueta a cláusula: AT_SAY(x, y, txt, "SIZE", w, h, "PICTURE", pic)
		args = append(args, &ast.StringLit{Loc: p.posFromToken(clauseTok), Value: clauseName})
		args = append(args, vals...)
	}

	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "AT_" + verb, Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

func (p *Parser) isAtClauseWord(tok lexer.Token) bool {
	for _, kw := range []string{
		"SIZE", "PICTURE", "VALID", "WHEN", "COLOR", "FONT", "PIXEL",
		"MESSAGE", "ACTION", "OF", "DECODE", "F3", "CLICKFOCUS", "RANGE",
		"MAXLENGTH", "MASK", "RESET", "TITLE", "VAR", "MEMO",
	} {
		if p.isWord(tok, kw) {
			return true
		}
	}
	return false
}

// parseActivateDialog handles:
//
//	ACTIVATE MSDIALOG oDlg ON INIT <block> VALID <expr> CENTERED
//
// desugared to a dropped `ACTIVATE_MSDIALOG(oDlg, onInit, valid)` call —
// same rationale as parseDefine/parseAtCommand: no real dialog engine yet,
// so the goal is just consuming the clauses so `check` succeeds.
func (p *Parser) parseActivateDialog() (ast.Statement, error) {
	tok := p.advance() // ACTIVATE
	p.advance()         // MSDIALOG

	varTok, err := p.expect(lexer.TOKEN_IDENT)
	if err != nil {
		return nil, err
	}
	target := &ast.Ident{Loc: p.posFromToken(varTok), Name: varTok.Value}

	clauses := map[string]ast.Expression{}
	for {
		cur := p.peek()
		switch {
		case p.isKeyword(cur, "ON") && p.isWord(p.peekAt(1), "INIT"):
			p.advance()
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			clauses["INIT"] = val
		case p.isKeyword(cur, "VALID"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			clauses["VALID"] = val
		case p.isWord(cur, "CENTERED"):
			p.advance()
		default:
			goto done
		}
	}
done:
	args := []ast.Expression{target}
	for _, name := range []string{"INIT", "VALID"} {
		if v, ok := clauses[name]; ok {
			args = append(args, v)
		}
	}
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "ACTIVATE_MSDIALOG", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseCodeBlockItem parses one comma-separated item of a `{|| ...}` body.
// Codeblocks are expressions, but Clipper freely uses assignment inside them
// (`{|| x := 1}` is the single most common codeblock shape), so this mirrors
// parseExprStatement's assignment desugaring, just producing an AssignExpr
// instead of an AssignStmt.
func (p *Parser) parseCodeBlockItem() (ast.Expression, error) {
	tok := p.peek()
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &ast.AssignExpr{Loc: p.posFromToken(tok), Target: expr, Value: val}, nil
	}

	if p.peek().Type == lexer.TOKEN_PLUS || p.peek().Type == lexer.TOKEN_MINUS ||
		p.peek().Type == lexer.TOKEN_STAR || p.peek().Type == lexer.TOKEN_SLASH {
		if p.peekAt(1).Type == lexer.TOKEN_ASSIGN {
			opTok := p.advance()
			p.advance()
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			combined := &ast.BinaryOp{Loc: p.posFromToken(tok), Op: opTok.Value, Left: expr, Right: val}
			return &ast.AssignExpr{Loc: p.posFromToken(tok), Target: expr, Value: combined}, nil
		}
	}

	if p.peek().Type == lexer.TOKEN_INCREMENT || p.peek().Type == lexer.TOKEN_DECREMENT {
		opTok := p.advance()
		op := "+"
		if opTok.Type == lexer.TOKEN_DECREMENT {
			op = "-"
		}
		one := &ast.NumberLit{Loc: p.posFromToken(opTok), Value: 1, Str: "1"}
		combined := &ast.BinaryOp{Loc: p.posFromToken(tok), Op: op, Left: expr, Right: one}
		return &ast.AssignExpr{Loc: p.posFromToken(tok), Target: expr, Value: combined}, nil
	}

	return expr, nil
}

// parseAssignRHS parses the value side of `target := value`, recursing on
// further ':=' so chained assignment (`a := b := expr`, seen in real code)
// builds nested AssignExprs instead of leaving a trailing ':=' unconsumed.
func (p *Parser) parseAssignRHS() (ast.Expression, error) {
	tok := p.peek()
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
		val, err := p.parseAssignRHS()
		if err != nil {
			return nil, err
		}
		return &ast.AssignExpr{Loc: p.posFromToken(tok), Target: expr, Value: val}, nil
	}
	return expr, nil
}

func (p *Parser) parseExprStatement() (ast.Statement, error) {
	tok := p.peek()

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.peek().Type == lexer.TOKEN_ASSIGN {
		op := p.advance().Value
		val, err := p.parseAssignRHS()
		if err != nil {
			return nil, err
		}
		return &ast.AssignStmt{Loc: p.posFromToken(tok), Target: expr, Value: val, Op: op}, nil
	}

	// Compound assignment (+=, -=, *=, /=) desugars to `target := target op value`
	// here at parse time — codegen only knows plain ':=' assignment, and this
	// reuses it as-is instead of threading the op through every target kind
	// (Ident/PropertyAccess/ArrayAccess/...) in the compiler.
	if p.peek().Type == lexer.TOKEN_PLUS || p.peek().Type == lexer.TOKEN_MINUS ||
		p.peek().Type == lexer.TOKEN_STAR || p.peek().Type == lexer.TOKEN_SLASH {
		next := p.peekAt(1)
		if next.Type == lexer.TOKEN_ASSIGN {
			opTok := p.advance()
			p.advance()
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			combined := &ast.BinaryOp{Loc: p.posFromToken(tok), Op: opTok.Value, Left: expr, Right: val}
			return &ast.AssignStmt{Loc: p.posFromToken(tok), Target: expr, Value: combined, Op: ":="}, nil
		}
	}

	// x++ / x-- desugars to x := x + 1 / x := x - 1.
	if p.peek().Type == lexer.TOKEN_INCREMENT || p.peek().Type == lexer.TOKEN_DECREMENT {
		opTok := p.advance()
		op := "+"
		if opTok.Type == lexer.TOKEN_DECREMENT {
			op = "-"
		}
		one := &ast.NumberLit{Loc: p.posFromToken(opTok), Value: 1, Str: "1"}
		combined := &ast.BinaryOp{Loc: p.posFromToken(tok), Op: op, Left: expr, Right: one}
		return &ast.AssignStmt{Loc: p.posFromToken(tok), Target: expr, Value: combined, Op: ":="}, nil
	}

	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: expr}, nil
}

// --- Expression parsing (Pratt precedence) ---

func (p *Parser) parseExpression() (ast.Expression, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (ast.Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == lexer.TOKEN_DOT_OR || p.isKeyword(p.peek(), "OR") {
		tok := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: ".Or.", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expression, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == lexer.TOKEN_DOT_AND || p.isKeyword(p.peek(), "AND") {
		tok := p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: ".And.", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (ast.Expression, error) {
	if p.peek().Type == lexer.TOKEN_DOT_NOT {
		tok := p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryOp{Loc: p.posFromToken(tok), Op: ".Not.", Operand: operand}, nil
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() (ast.Expression, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		var op string
		switch {
		case tok.Type == lexer.TOKEN_EQ:
			op = "=="
		case tok.Type == lexer.TOKEN_ASSIGN && tok.Value == "=":
			// Bare '=' inside an expression (e.g. `IF x = 1`) is equality in
			// AdvPL/Clipper, not assignment — assignment statements are
			// parsed separately before general expression parsing runs.
			// TOKEN_ASSIGN also covers ':=', which must NOT be swallowed
			// here (it's a statement-level operator, handled by the caller).
			op = "=="
		case tok.Type == lexer.TOKEN_NEQ:
			op = "!="
		case tok.Type == lexer.TOKEN_LT:
			op = "<"
		case tok.Type == lexer.TOKEN_GT:
			op = ">"
		case tok.Type == lexer.TOKEN_LTE:
			op = "<="
		case tok.Type == lexer.TOKEN_GTE:
			op = ">="
		case tok.Type == lexer.TOKEN_DOLLAR:
			op = "$"
		default:
			goto done
		}
		p.advance()
		right, err := p.parseAddition()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: op, Left: left, Right: right}
	}
done:
	return left, nil
}

func (p *Parser) parseAddition() (ast.Expression, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for (p.peek().Type == lexer.TOKEN_PLUS || p.peek().Type == lexer.TOKEN_MINUS) && p.peekAt(1).Type != lexer.TOKEN_ASSIGN {
		tok := p.advance()
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: tok.Value, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseMultiplication() (ast.Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for (p.peek().Type == lexer.TOKEN_STAR || p.peek().Type == lexer.TOKEN_SLASH || p.peek().Type == lexer.TOKEN_PERCENT) && p.peekAt(1).Type != lexer.TOKEN_ASSIGN {
		tok := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: tok.Value, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (ast.Expression, error) {
	if p.peek().Type == lexer.TOKEN_MINUS {
		tok := p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryOp{Loc: p.posFromToken(tok), Op: "-", Operand: operand}, nil
	}
	if p.peek().Type == lexer.TOKEN_PLUS {
		p.advance()
		return p.parseUnary()
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (ast.Expression, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		switch tok.Type {
		case lexer.TOKEN_COLON:
			p.advance()
			propTok := p.peek()
			if propTok.Type != lexer.TOKEN_IDENT && propTok.Type != lexer.TOKEN_KEYWORD {
				return nil, fmt.Errorf("expected property name after ':', got %v (%q) at %s:%d:%d",
					propTok.Type, propTok.Value, propTok.FileName, propTok.Line, propTok.Col)
			}
			p.advance()
			if p.peek().Type == lexer.TOKEN_LPAREN {
				p.advance()
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				expr = &ast.MethodCall{Loc: p.posFromToken(tok), Object: expr, Method: propTok.Value, Args: args}
			} else {
				expr = &ast.PropertyAccess{Loc: p.posFromToken(tok), Object: expr, Property: propTok.Value}
			}
		case lexer.TOKEN_LBRACKET:
			p.advance()
			idx, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			expr = &ast.ArrayAccess{Loc: p.posFromToken(tok), Array: expr, Index: idx}
			// arr[i,j] is Clipper/AdvPL sugar for arr[i][j] (multi-dim arrays
			// are arrays of arrays).
			for p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
				idx2, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				expr = &ast.ArrayAccess{Loc: p.posFromToken(tok), Array: expr, Index: idx2}
			}
			if _, err := p.expect(lexer.TOKEN_RBRACKET); err != nil {
				return nil, err
			}
		case lexer.TOKEN_ARROW:
			p.advance()
			// alias->(expr) evaluates expr in the alias's work area, as
			// opposed to alias->field. Codegen doesn't track work-area
			// context yet (plain alias->field already ignores Alias), so
			// this just evaluates the inner expression directly.
			if p.peek().Type == lexer.TOKEN_LPAREN {
				p.advance()
				inner, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TOKEN_RPAREN); err != nil {
					return nil, err
				}
				expr = inner
				break
			}
			// alias->&(expr) / alias->&ident: macro-computed field name.
			// The field name is only known at runtime (it's the whole point
			// of the macro), which this VM can't resolve dynamically yet —
			// consume the syntax so `check` succeeds, field left blank.
			if p.peek().Type == lexer.TOKEN_AMPERSAND {
				p.advance()
				if p.peek().Type == lexer.TOKEN_LPAREN {
					p.advance()
					if _, err := p.parseExpression(); err != nil {
						return nil, err
					}
					if _, err := p.expect(lexer.TOKEN_RPAREN); err != nil {
						return nil, err
					}
				} else if _, err := p.expect(lexer.TOKEN_IDENT); err != nil {
					return nil, err
				}
				if ident, ok := expr.(*ast.Ident); ok {
					expr = &ast.FieldAccess{Loc: p.posFromToken(tok), Alias: ident.Name, Field: ""}
				}
				break
			}
			fieldTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			if ident, ok := expr.(*ast.Ident); ok {
				expr = &ast.FieldAccess{Loc: p.posFromToken(tok), Alias: ident.Name, Field: fieldTok.Value}
			}
		case lexer.TOKEN_LPAREN:
			if ident, ok := expr.(*ast.Ident); ok {
				p.advance()
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				expr = &ast.NewExpr{Loc: p.posFromToken(tok), ClassName: ident.Name, Args: args}
			} else {
				goto done
			}
		case lexer.TOKEN_INCREMENT, lexer.TOKEN_DECREMENT:
			// `nParam++` used inline as a call argument, not just as its own
			// statement. Desugars to `(expr := expr +/- 1)` — evaluates to
			// the new value, so this is pre- rather than post-increment
			// semantics, but real usage here is always a running counter
			// where callers don't depend on which one they get.
			opTok := p.advance()
			op := "+"
			if opTok.Type == lexer.TOKEN_DECREMENT {
				op = "-"
			}
			one := &ast.NumberLit{Loc: p.posFromToken(opTok), Value: 1, Str: "1"}
			combined := &ast.BinaryOp{Loc: p.posFromToken(opTok), Op: op, Left: expr, Right: one}
			expr = &ast.AssignExpr{Loc: p.posFromToken(opTok), Target: expr, Value: combined}
		default:
			goto done
		}
	}
done:
	return expr, nil
}

func (p *Parser) parseArguments() ([]ast.Expression, error) {
	args := make([]ast.Expression, 0)
	for p.peek().Type != lexer.TOKEN_RPAREN && p.peek().Type != lexer.TOKEN_EOF {
		if p.peek().Type == lexer.TOKEN_AT {
			p.advance()
		}
		// Omitted argument: `f(, x)`, `f(x,, y)`, `f(x,)` all default the gap to NIL.
		if p.peek().Type == lexer.TOKEN_COMMA || p.peek().Type == lexer.TOKEN_RPAREN {
			tok := p.peek()
			args = append(args, &ast.NilLit{Loc: p.posFromToken(tok)})
		} else if p.peek().Type == lexer.TOKEN_IDENT && p.peekAt(1).Type == lexer.TOKEN_ASSIGN {
			// Check for named parameter: ident = expr
			nameTok := p.advance()
			p.advance() // consume =
			valExpr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.NamedParam{Loc: p.posFromToken(nameTok), Name: nameTok.Value, Value: valExpr})
		} else {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		if p.peek().Type == lexer.TOKEN_COMMA {
			p.advance()
		}
	}
	if p.peek().Type == lexer.TOKEN_RPAREN {
		p.advance()
	}
	return args, nil
}

func (p *Parser) parsePrimary() (ast.Expression, error) {
	tok := p.peek()

	switch tok.Type {
	case lexer.TOKEN_NUMBER:
		p.advance()
		val, _ := strconv.ParseFloat(tok.Value, 64)
		return &ast.NumberLit{Loc: p.posFromToken(tok), Value: val, Str: tok.Value}, nil

	case lexer.TOKEN_STRING:
		p.advance()
		return &ast.StringLit{Loc: p.posFromToken(tok), Value: tok.Value}, nil

	case lexer.TOKEN_TRUE:
		p.advance()
		return &ast.BoolLit{Loc: p.posFromToken(tok), Value: true}, nil

	case lexer.TOKEN_FALSE:
		p.advance()
		return &ast.BoolLit{Loc: p.posFromToken(tok), Value: false}, nil

	case lexer.TOKEN_NIL:
		p.advance()
		return &ast.NilLit{Loc: p.posFromToken(tok)}, nil

	case lexer.TOKEN_IDENT:
		p.advance()
		// Fully qualified TLPP namespace path: totvs.framework.x.Func(...).
		// Dot-literals (.T., .AND., etc) already tokenize as their own single
		// token in the lexer, so a bare TOKEN_DOT here only ever separates
		// namespace segments.
		name := tok.Value
		for p.peek().Type == lexer.TOKEN_DOT && p.peekAt(1).Type == lexer.TOKEN_IDENT {
			p.advance()
			name += "." + p.advance().Value
		}
		if p.peek().Type == lexer.TOKEN_LPAREN {
			p.advance()
			args, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			return &ast.CallExpr{Loc: p.posFromToken(tok), Name: name, Args: args}, nil
		}
		return &ast.Ident{Loc: p.posFromToken(tok), Name: name}, nil

	case lexer.TOKEN_DOUBLECOLON:
		p.advance()
		propTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		if p.peek().Type == lexer.TOKEN_LPAREN {
			p.advance()
			args, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			return &ast.SelfMethodCall{Loc: p.posFromToken(tok), Method: propTok.Value, Args: args}, nil
		}
		return &ast.SelfRef{Loc: p.posFromToken(tok), Property: propTok.Value}, nil

	case lexer.TOKEN_LPAREN:
		p.advance()
		// `()` (seen after Return in real code, e.g. `Return()`) has no
		// inner expression to parse — treat it as NIL.
		if p.peek().Type == lexer.TOKEN_RPAREN {
			p.advance()
			return &ast.NilLit{Loc: p.posFromToken(tok)}, nil
		}
		// Plain grouping `(expr)`, `(x := expr)` (Clipper's "assign and
		// test" idiom — assignment is otherwise a statement, not an
		// expression), and `(a, b, c)` (a codeblock body without the `{||
		// }`, evaluates all and yields the last) are all one production:
		// comma-separated codeblock-style items inside parens.
		items := []ast.Expression{}
		for {
			item, err := p.parseCodeBlockItem()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			if p.peek().Type != lexer.TOKEN_COMMA {
				break
			}
			p.advance()
		}
		if _, err := p.expect(lexer.TOKEN_RPAREN); err != nil {
			return nil, err
		}
		if len(items) == 1 {
			return items[0], nil
		}
		return &ast.SeqExpr{Loc: p.posFromToken(tok), Exprs: items}, nil

	case lexer.TOKEN_LBRACE:
		p.advance()
		// Code block: {|| ...} or {|params| ...}
		if p.peek().Type == lexer.TOKEN_PIPE {
			p.advance()
			params := []string{}
			if p.peek().Type == lexer.TOKEN_PIPE {
				p.advance()
			} else {
				for p.peek().Type != lexer.TOKEN_PIPE && p.peek().Type != lexer.TOKEN_EOF {
					paramTok, err := p.expect(lexer.TOKEN_IDENT)
					if err != nil {
						return nil, err
					}
					params = append(params, paramTok.Value)
					if p.peek().Type == lexer.TOKEN_COMMA {
						p.advance()
					}
				}
				if p.peek().Type == lexer.TOKEN_PIPE {
					p.advance()
				}
			}
			var bodyStmts []ast.Statement
			var singleExpr ast.Expression
			for p.peek().Type != lexer.TOKEN_RBRACE && p.peek().Type != lexer.TOKEN_EOF {
				expr, err := p.parseCodeBlockItem()
				if err != nil {
					return nil, err
				}
				if singleExpr == nil {
					singleExpr = expr
				} else {
					bodyStmts = append(bodyStmts, &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: singleExpr})
					singleExpr = expr
				}
				if p.peek().Type == lexer.TOKEN_COMMA {
					p.advance()
				}
			}
			if _, err := p.expect(lexer.TOKEN_RBRACE); err != nil {
				return nil, err
			}
			return &ast.CodeBlock{Loc: p.posFromToken(tok), Params: params, Expr: singleExpr, Body: bodyStmts}, nil
		}

		// JSON inline: { "key" : value, ... }
		if p.peek().Type == lexer.TOKEN_STRING && p.peekAt(1).Type == lexer.TOKEN_COLON {
			pairs := make([]ast.JsonPair, 0)
			for p.peek().Type != lexer.TOKEN_RBRACE && p.peek().Type != lexer.TOKEN_EOF {
				keyTok, err := p.expect(lexer.TOKEN_STRING)
				if err != nil {
					return nil, err
				}
				if _, err := p.expect(lexer.TOKEN_COLON); err != nil {
					return nil, err
				}
				val, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				pairs = append(pairs, ast.JsonPair{Loc: p.posFromToken(keyTok), Key: keyTok.Value, Value: val})
				if p.peek().Type == lexer.TOKEN_COMMA {
					p.advance()
				}
			}
			if _, err := p.expect(lexer.TOKEN_RBRACE); err != nil {
				return nil, err
			}
			return &ast.JsonLit{Loc: p.posFromToken(tok), Pairs: pairs}, nil
		}

		// Regular array literal
		elements := make([]ast.Expression, 0)
		for p.peek().Type != lexer.TOKEN_RBRACE && p.peek().Type != lexer.TOKEN_EOF {
			// Omitted element: `{a,, c}` leaves a gap that defaults to NIL,
			// same idiom as omitted call arguments.
			if p.peek().Type == lexer.TOKEN_COMMA {
				elements = append(elements, &ast.NilLit{Loc: p.posFromToken(p.peek())})
			} else {
				elem, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				elements = append(elements, elem)
			}
			if p.peek().Type == lexer.TOKEN_COMMA {
				p.advance()
			}
		}
		if _, err := p.expect(lexer.TOKEN_RBRACE); err != nil {
			return nil, err
		}
		return &ast.ArrayLit{Loc: p.posFromToken(tok), Elements: elements}, nil

	case lexer.TOKEN_AMPERSAND:
		p.advance()
		if p.peek().Type == lexer.TOKEN_LPAREN {
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TOKEN_RPAREN); err != nil {
				return nil, err
			}
			return &ast.MacroExp{Loc: p.posFromToken(tok), Expr: expr}, nil
		}
		identTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		return &ast.MacroExp{Loc: p.posFromToken(tok), Expr: &ast.Ident{Loc: p.posFromToken(tok), Name: identTok.Value}}, nil

	case lexer.TOKEN_AT:
		p.advance()
		return p.parsePrimary()

	default:
		if tok.Type == lexer.TOKEN_KEYWORD {
			if strings.EqualFold(tok.Value, "SELF") {
				p.advance()
				return &ast.SelfRef{Loc: p.posFromToken(tok), Property: ""}, nil
			}
			if strings.EqualFold(tok.Value, "NIL") {
				p.advance()
				return &ast.NilLit{Loc: p.posFromToken(tok)}, nil
			}
			// `If(cond, then, else)` is Clipper/AdvPL's inline conditional,
			// an alias for IIF() usable as an expression (distinct from the
			// If/EndIf statement, which is handled at the statement level).
			if strings.EqualFold(tok.Value, "IF") && p.peekAt(1).Type == lexer.TOKEN_LPAREN {
				p.advance()
				p.advance()
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				return &ast.CallExpr{Loc: p.posFromToken(tok), Name: "IIF", Args: args}, nil
			}
			// TLPP reserves ARRAY/DATE/OBJECT as type-annotation keywords, but
			// classic AdvPL also calls them as constructors: Array(10),
			// Date(2024,1,1). Same shape as If()/IIF() above.
			if (strings.EqualFold(tok.Value, "ARRAY") || strings.EqualFold(tok.Value, "DATE") || strings.EqualFold(tok.Value, "OBJECT")) &&
				p.peekAt(1).Type == lexer.TOKEN_LPAREN {
				p.advance()
				p.advance()
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				return &ast.CallExpr{Loc: p.posFromToken(tok), Name: strings.ToUpper(tok.Value), Args: args}, nil
			}
		}
		return nil, fmt.Errorf("unexpected token %v (%q) at %s:%d:%d",
			tok.Type, tok.Value, tok.FileName, tok.Line, tok.Col)
	}
}
