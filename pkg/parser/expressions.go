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
		// `IF(cond, then, else)` — o condicional em linha do Clipper/AdvPL,
		// usado como statement isolado (resultado descartado), ex.:
		// `IF(VALTYPE(d) != 'D', d := Date(), )`. Distingue do bloco
		// `If (cond) ... EndIf` por lookahead: uma vírgula no nível
		// superior dentro dos parênteses só existe na forma de chamada
		// (`If (cond)` sozinho nunca tem vírgula de topo).
		if p.peekAt(1).Type == lexer.TOKEN_LPAREN && p.isInlineIfCall() {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: expr}, nil
		}
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
	if p.isWord(tok, "SET") && (p.peekAt(1).Type == lexer.TOKEN_IDENT || p.peekAt(1).Type == lexer.TOKEN_KEYWORD) &&
		(p.isKeyword(p.peekAt(2), "TO") || p.isWord(p.peekAt(2), "ON") || p.isWord(p.peekAt(2), "OFF") || p.isWord(p.peekAt(2), "OF")) {
		return p.parseSetCommand()
	}
	// `SET KEY <nKey> TO [<uBlock>]` — o keycode vem antes do TO, ao
	// contrário de `SET <opção> TO ...`.
	if p.isWord(tok, "SET") && p.isWord(p.peekAt(1), "KEY") {
		return p.parseSetCommand()
	}
	// `SET BROWSE <var> ARRAY <expr>` — DSL mobile FDA: vincula o array de
	// dados ao browse; parseado e descartado.
	if p.isWord(tok, "SET") && p.isWord(p.peekAt(1), "BROWSE") {
		p.advance() // SET
		p.advance() // BROWSE
		target, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args := []ast.Expression{target}
		if p.isKeyword(p.peek(), "ARRAY") {
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "SET_BROWSE", Args: args}
		return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
	}
	if p.isKeyword(tok, "ADD") && p.isWord(p.peekAt(1), "OPTION") {
		return p.parseAddOption()
	}
	// `ADD FOLDER <var> CAPTION <expr> OF <expr>` / `ADD COLUMN <var> TO
	// <expr> ARRAY ELEMENT <n> HEADER <expr> WIDTH <n>` — DSL do FDA/mobile;
	// parseado e descartado (sem UI mobile por trás).
	if p.isKeyword(tok, "ADD") && (p.isWord(p.peekAt(1), "FOLDER") || p.isWord(p.peekAt(1), "COLUMN")) {
		return p.parseAddWidget()
	}
	if p.isWord(tok, "DEFINE") {
		return p.parseDefine()
	}
	// `CREATE PANEL oWizard HEADER ... MESSAGE ... BACK {||...} NEXT
	// {||...} FINISH {||...}` — mesma forma de DEFINE WIZARD (var +
	// cláusulas), só com "CREATE PANEL" em vez de "DEFINE WIZARD".
	if p.isWord(tok, "CREATE") && p.isWord(p.peekAt(1), "PANEL") {
		return p.parseDefine()
	}
	// `Append From (expr) [Via expr] [Fields ...] [For expr] [While expr]`
	// — comando Clipper clássico de importação de registros de outro
	// arquivo/RDD; parseado e descartado (sem engine de banco por trás),
	// mesmo espírito do SET/DEFINE.
	if p.isWord(tok, "APPEND") && p.isKeyword(p.peekAt(1), "FROM") {
		return p.parseAppendFrom()
	}
	// `Copy To (expr) [For expr] [While expr] [Via expr] [Fields ...]
	// [SDF] [DELIMITED [WITH expr]] [REST] [NEXT expr]` — comando Clipper
	// de exportação de registros; parseado e descartado.
	if p.isWord(tok, "COPY") && p.isKeyword(p.peekAt(1), "TO") {
		return p.parseCopyTo()
	}
	// `Copy <alias-expr> To Memory <name> [Blank]` — copia a estrutura de
	// campos do alias para um array (distinto de `Copy To` que exporta
	// registros); parseado e descartado.
	if p.isWord(tok, "COPY") {
		save := p.pos
		p.advance() // COPY
		if _, err := p.parseOr(); err == nil && p.isKeyword(p.peek(), "TO") && p.isWord(p.peekAt(1), "MEMORY") {
			p.advance() // TO
			p.advance() // MEMORY
			nameTok, err := p.expectName()
			if err != nil {
				return nil, err
			}
			if p.isWord(p.peek(), "BLANK") {
				p.advance()
			}
			call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "COPY_TO_MEMORY", Args: []ast.Expression{&ast.StringLit{Loc: p.posFromToken(nameTok), Value: nameTok.Value}}}
			return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
		}
		p.pos = save
	}
	// `Copy File <expr> To <expr>` — copia arquivo no disco (comando
	// Clipper), distinto de `Copy To` (exportação de registros do alias
	// atual); parseado e descartado, mesmo espírito de DELETE FILE.
	if p.isWord(tok, "COPY") && p.isWord(p.peekAt(1), "FILE") {
		copyTok := p.advance() // COPY
		p.advance()            // FILE
		source, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args := []ast.Expression{source}
		if p.isKeyword(p.peek(), "TO") {
			p.advance()
			dest, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, dest)
		}
		call := &ast.CallExpr{Loc: p.posFromToken(copyTok), Name: "COPY_FILE", Args: args}
		return &ast.ExprStmt{Loc: p.posFromToken(copyTok), Expr: call}, nil
	}
	// `Index On <expr> [Tag <expr>] [For <expr>] [Unique] [Descending]
	// [Additive] To <expr>` — comando Clipper de criação de índice.
	if p.isWord(tok, "INDEX") && p.isKeyword(p.peekAt(1), "ON") {
		return p.parseIndexOn()
	}
	// `Delete File <expr>` — apaga um arquivo do disco (comando Clipper,
	// diferente de `Delete [For expr] [While expr]` que marca registros do
	// alias atual para deleção); ambos parseados e descartados.
	if p.isWord(tok, "DELETE") {
		return p.parseDeleteCommand()
	}
	// `Locate For <expr> [While <expr>]` — posiciona no primeiro registro do
	// alias atual que satisfaz a condição (Found() reflete o resultado).
	if p.isWord(tok, "LOCATE") {
		return p.parseLocateCommand()
	}
	// `Release Object <name>` / `Release All [Like <mask>]` — libera
	// memvars/objetos; parseado e descartado. Só com OBJECT/ALL para não
	// engolir um identificador "Release" usado como nome comum.
	if p.isWord(tok, "RELEASE") && (p.isWord(p.peekAt(1), "OBJECT") || p.isKeyword(p.peekAt(1), "ALL")) {
		relTok := p.advance() // RELEASE
		if p.isWord(p.peek(), "OBJECT") {
			p.advance()
			for {
				if _, err := p.expectName(); err != nil {
					return nil, err
				}
				if p.peek().Type != lexer.TOKEN_COMMA {
					break
				}
				p.advance()
			}
		} else {
			p.advance() // ALL
			if p.isWord(p.peek(), "LIKE") {
				p.advance()
				if _, err := p.parseOr(); err != nil {
					return nil, err
				}
			}
		}
		call := &ast.CallExpr{Loc: p.posFromToken(relTok), Name: "RELEASE_VARS", Args: nil}
		return &ast.ExprStmt{Loc: p.posFromToken(relTok), Expr: call}, nil
	}
	if p.isWord(tok, "PUBLISH") && p.isWord(p.peekAt(1), "MODEL") {
		return p.parsePublishModel()
	}
	// `Prepare Environment Empresa <expr> Filial <expr> [Modulo <expr>]
	// [Tables <expr>,...]` — abre ambiente/tabelas de um job batch fora do
	// contexto de uma rotina interativa; parseado e descartado.
	if p.isWord(tok, "PREPARE") && p.isWord(p.peekAt(1), "ENVIRONMENT") {
		return p.parsePrepareEnvironment()
	}
	// `ParamType <n> Var <name> As <type> [Default <expr>]` — declaração de
	// metadados de parâmetro de rotina (objeto de negócio/REST), distinta
	// do include obsoleto ParmType.ch; parseada e descartada.
	if p.isWord(tok, "PARAMTYPE") {
		return p.parseParamType()
	}
	if p.isKeyword(tok, "ACTIVATE") && (p.isWord(p.peekAt(1), "MSDIALOG") || p.isWord(p.peekAt(1), "DIALOG") || p.isWord(p.peekAt(1), "WIZARD") || p.isWord(p.peekAt(1), "WINDOW") || p.isWord(p.peekAt(1), "FWMBROWSE") || p.isWord(p.peekAt(1), "MBROWSE") || p.isWord(p.peekAt(1), "REPORT")) {
		return p.parseActivateDialog()
	}
	if tok.Type == lexer.TOKEN_AT {
		return p.parseAtCommand()
	}
	if p.isKeyword(tok, "RETURN") {
		p.advance()
		var retExpr ast.Expression
		if p.peek().Type != lexer.TOKEN_EOF && p.peek().Type != lexer.TOKEN_SEMICOLON && !p.isStatementBoundary(p.peek()) {
			// `Return target := value` (assignment used inline as the
			// return value, e.g. `Return self:oProp := {...}`) — same
			// idiom as If/While conditions, needs parseAssignRHS instead
			// of plain parseExpression to not leave the ':=' dangling.
			expr, err := p.parseAssignRHS()
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
	// `Private M->AKR_ORCAME := ...` — Clipper idiom explicitly qualifying
	// a memvar with the "M" (memory) alias, redundant scope prefix that
	// only real-alias field access (`SomeAlias->field`) normally uses;
	// the declared variable's actual name is the part after '->'.
	if strings.EqualFold(nameTok.Value, "M") && p.peek().Type == lexer.TOKEN_ARROW {
		p.advance() // ->
		realName, err := p.expectName()
		if err != nil {
			return nil, err
		}
		nameTok = realName
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
		// parseAssignRHS (not parseExpression) so chained assignment
		// (`Private cL2:=cL3:=cL4:="x"`) parses as nested AssignExpr instead
		// of leaving the extra ':='s dangling.
		val, err := p.parseAssignRHS()
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
			val, err := p.parseAssignRHS()
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

// isInlineIfCall faz lookahead a partir de "IF(" (chamador já garantiu
// peekAt(1)==LPAREN) e retorna true se houver uma vírgula no nível
// superior de parênteses antes do fechamento correspondente — sinal de
// que é a forma de chamada `IF(cond,then,else)`, não `If (cond)` bloco.
func (p *Parser) isInlineIfCall() bool {
	depth := 0
	bracketDepth := 0
	for i := 1; ; i++ {
		tok := p.peekAt(i)
		switch tok.Type {
		case lexer.TOKEN_LPAREN:
			depth++
		case lexer.TOKEN_RPAREN:
			depth--
			if depth == 0 {
				return false
			}
		case lexer.TOKEN_LBRACKET:
			bracketDepth++
		case lexer.TOKEN_RBRACKET:
			bracketDepth--
		case lexer.TOKEN_COMMA:
			// vírgula dentro de `[i,j]` (índice multi-dimensional) não
			// conta — só a vírgula de topo dentro dos parênteses do IF.
			if depth == 1 && bracketDepth == 0 {
				return true
			}
		case lexer.TOKEN_EOF:
			return false
		}
	}
}

func (p *Parser) parseIf() (ast.Statement, error) {
	startTok := p.advance()

	// parseAssignRHS (not parseExpression) so `If x := cond` — assignment
	// used inline as the condition, real and common in AdvPL — parses as an
	// AssignExpr instead of leaving the ':=' dangling.
	cond, err := p.parseAssignRHS()
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

	// "End" genérico (sem "Next") e "EndFor" também fecham o For no
	// Clipper/AdvPL clássico, igual ao If/While/Case.
	for !p.isKeyword(p.peek(), "NEXT") && !p.isKeyword(p.peek(), "END") &&
		!p.isWord(p.peek(), "ENDFOR") && p.peek().Type != lexer.TOKEN_EOF {
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

	if p.isKeyword(p.peek(), "END") || p.isWord(p.peek(), "ENDFOR") {
		p.advance()
		return forStmt, nil
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

	// parseAssignRHS so `While x := next()` / `While (x := next()) > 0` —
	// assignment inline in the condition, a common AdvPL idiom for
	// "advance and test" loops — parses correctly instead of leaving the
	// ':=' dangling.
	cond, err := p.parseAssignRHS()
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

// isEndCase reconhece tanto "ENDCASE" (uma palavra) quanto "End Case"
// (duas palavras, forma clássica do Clipper).
func (p *Parser) isEndCase() bool {
	return p.isKeyword(p.peek(), "ENDCASE") ||
		(p.isKeyword(p.peek(), "END") && p.isWord(p.peekAt(1), "CASE"))
}

func (p *Parser) advanceEndCase() {
	p.advance()
	if p.isWord(p.peek(), "CASE") {
		p.advance()
	}
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
			!p.isEndCase() && p.peek().Type != lexer.TOKEN_EOF {
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
		for !p.isEndCase() && p.peek().Type != lexer.TOKEN_EOF {
			stmt, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			if stmt != nil {
				doCase.Otherwise = append(doCase.Otherwise, stmt)
			}
		}
	}

	if p.isEndCase() {
		p.advanceEndCase()
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
	// A "seção" pode ser uma expressão completa (`oReport:Section(2)`),
	// não só um nome — parseia como expressão postfix e descarta.
	if p.peek().Type == lexer.TOKEN_IDENT {
		if _, err := p.parsePostfix(); err != nil {
			return nil, err
		}
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
		// identifier as if it were the trailing section-var echo. O eco
		// também pode ser uma expressão (`oReport:Section(2)`).
		if p.peek().Type == lexer.TOKEN_IDENT && p.peek().Line == queryTok.Line {
			if _, err := p.parsePostfix(); err != nil {
				return nil, err
			}
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

// parseDefault handles both the single form `Default var := val` and the
// comma-separated multi-var form `Default a := 1, b := 2, c := 3` — common
// in real AdvPL for setting up several optional parameters at once.
func (p *Parser) parseDefault() (ast.Statement, error) {
	tok := p.advance()
	def, err := p.parseOneDefault(tok)
	if err != nil {
		return nil, err
	}
	defaults := []*ast.DefaultExpr{def}
	for p.peek().Type == lexer.TOKEN_COMMA {
		p.advance()
		nextTok := p.peek()
		d, err := p.parseOneDefault(nextTok)
		if err != nil {
			return nil, err
		}
		defaults = append(defaults, d)
	}
	if len(defaults) == 1 {
		return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: defaults[0]}, nil
	}
	return &ast.DefaultGroup{Loc: p.posFromToken(tok), Defaults: defaults}, nil
}

func (p *Parser) parseOneDefault(tok lexer.Token) (*ast.DefaultExpr, error) {
	// `Default ::PageLen := 0` — target can be a self property, not just a
	// plain local/private var. Recorded with the "::" prefix so it never
	// collides with (and silently no-ops against, same as any other
	// unresolvable name) a real local of the bare property name.
	name := ""
	if p.peek().Type == lexer.TOKEN_DOUBLECOLON {
		p.advance()
		propTok, err := p.expectName()
		if err != nil {
			return nil, err
		}
		name = "::" + propTok.Value
	} else if p.isWord(p.peek(), "SELF") && p.peekAt(1).Type == lexer.TOKEN_COLON {
		// `Default Self:Prop := 0` — same self-property target as `::Prop`,
		// spelled out explicitly instead of the `::` shorthand.
		p.advance() // Self
		p.advance() // :
		propTok, err := p.expectName()
		if err != nil {
			return nil, err
		}
		name = "::" + propTok.Value
	} else {
		nameTok, err := p.expect(lexer.TOKEN_IDENT)
		if err != nil {
			return nil, err
		}
		name = nameTok.Value
	}
	if p.peek().Type == lexer.TOKEN_ASSIGN {
		p.advance()
	}
	val, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.DefaultExpr{Loc: p.posFromToken(tok), Name: name, Value: val}, nil
}

// parseAddWidget handles the FDA/mobile `ADD FOLDER/COLUMN` DSL:
//
//	ADD FOLDER <var> CAPTION <expr> OF <expr>
//	ADD COLUMN <var> TO <expr> ARRAY ELEMENT <n> HEADER <expr> WIDTH <n>
//
// parsed into a dropped ADD_<kind>(...) call.
func (p *Parser) parseAddWidget() (ast.Statement, error) {
	tok := p.advance()     // ADD
	kindTok := p.advance() // FOLDER / COLUMN
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{&ast.Ident{Loc: p.posFromToken(nameTok), Name: nameTok.Value}}
	for {
		cur := p.peek()
		switch {
		case p.isWord(cur, "CAPTION"), p.isKeyword(cur, "OF"), p.isKeyword(cur, "TO"),
			p.isWord(cur, "HEADER"), p.isWord(cur, "WIDTH"), p.isKeyword(cur, "SIZE"),
			p.isWord(cur, "ELEMENT"):
			p.advance()
			vals, err := p.parseCommaValues()
			if err != nil {
				return nil, err
			}
			args = append(args, vals...)
		// `ARRAY ELEMENT <n>` — ARRAY é só um qualificador sem valor próprio.
		case p.isKeyword(cur, "ARRAY"):
			p.advance()
		// `ON ACTIVATE <expr>` — callback do folder; consome o par de
		// palavras e a expressão.
		case p.isWord(cur, "ON") && p.isWord(p.peekAt(1), "ACTIVATE"):
			p.advance()
			p.advance()
			val, err := p.parseAssignableExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		default:
			call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "ADD_" + strings.ToUpper(kindTok.Value), Args: args}
			return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
		}
	}
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
	p.advance()        // OPTION

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
	p.advance()        // MODEL
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
		"TABLES", "PICTURE", "WHEN", "ONSTOP", "BLOCK",
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
	} else if p.peek().Type == lexer.TOKEN_DOUBLECOLON {
		// `DEFINE SCROLLBAR ::oVScroll VERTICAL OF Self` — alvo pode ser
		// propriedade do próprio objeto; consome e descarta (o codegen só
		// modela alvo simples).
		p.advance()
		if _, err := p.expectName(); err != nil {
			return nil, err
		}
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
		case p.isKeyword(cur, "PIXEL"), p.isWord(cur, "ENABLE"), p.isWord(cur, "DISABLE"),
			p.isWord(cur, "PANEL"), p.isWord(cur, "NOFIRSTPANEL"), p.isWord(cur, "TOP"), p.isWord(cur, "GROUP"),
			p.isWord(cur, "VERTICAL"), p.isWord(cur, "HORIZONTAL"):
			p.advance() // flag clauses, no value
			continue
		// `DEFINE BUTTONBAR ... 3D TOP OF ...` — "3D" tokeniza como NUMBER
		// "3" + IDENT "D" separados (identificador não pode começar com
		// dígito); trata o par junto como uma única flag.
		case cur.Type == lexer.TOKEN_NUMBER && cur.Value == "3" &&
			p.peekAt(1).Type == lexer.TOKEN_IDENT && strings.EqualFold(p.peekAt(1).Value, "D"):
			p.advance()
			p.advance()
			continue
		// `DEFINE FUNCTION ... NO END SECTION` — TReport column aggregate,
		// three-word flag with no value (don't force a page break after the
		// section's aggregate line).
		case p.isWord(cur, "NO") && p.isWord(p.peekAt(1), "END") && p.isWord(p.peekAt(2), "SECTION"):
			p.advance()
			p.advance()
			p.advance()
			continue
		// `DEFINE WIZARD ... HEADER expr MESSAGE expr NEXT {|lOk|...}
		// BACK {||...} FINISH {||...} PANEL NOFIRSTPANEL` — TWizard/
		// ApWizard (API obsoleta, mas ainda usada em fontes legados; só
		// consome a sintaxe, sem modelar o assistente de verdade).
		case p.isWord(cur, "HEADER"):
			name = "HEADER"
		case p.isWord(cur, "MESSAGE"):
			name = "MESSAGE"
		case p.isKeyword(cur, "NEXT"):
			name = "NEXT"
		case p.isWord(cur, "BACK"):
			name = "BACK"
		case p.isWord(cur, "FINISH"):
			name = "FINISH"
		case p.isWord(cur, "EXEC"):
			name = "EXEC"
		case p.isWord(cur, "COLOR"):
			name = "COLOR"
		case p.isWord(cur, "STYLE"):
			name = "STYLE"
		case p.isWord(cur, "ICON"):
			name = "ICON"
		case p.isWord(cur, "NAME"):
			name = "NAME"
		case p.isWord(cur, "RESOURCE"), p.isWord(cur, "RESNAME"):
			name = "RESOURCE"
		case p.isWord(cur, "TOOLTIP"):
			name = "TOOLTIP"
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
		case p.isWord(cur, "TABLES"):
			name = "TABLES"
		// `DEFINE SCROLLBAR ... RANGE min, max`
		case p.isWord(cur, "RANGE"):
			name = "RANGE"
		case p.isWord(cur, "PICTURE"):
			name = "PICTURE"
		case p.isWord(cur, "WHEN"):
			name = "WHEN"
		// `DEFINE SBUTTON ... ONSTOP <expr> OF ...` — texto de tooltip do
		// botão (TButton "OnStop"), clause de valor único.
		case p.isWord(cur, "ONSTOP"):
			name = "ONSTOP"
		// `DEFINE CELL ... BLOCK{||...} ...` — TReport column value block
		// (equivalente a NAME quando o conteúdo não é um campo direto).
		case p.isWord(cur, "BLOCK"):
			name = "BLOCK"
		// `DEFINE FUNCTION ... FUNCTION SUM ...` — TReport column aggregate
		// function (SUM/AVG/...); clause name collides with the DEFINE kind
		// itself, only ever seen after `DEFINE FUNCTION <target> FROM ...`.
		case p.isWord(cur, "FUNCTION"):
			name = "AGGFUNCTION"
		case p.isWord(cur, "BREAK"):
			name = "BREAK"
		case p.isWord(cur, "PROMPT"):
			name = "PROMPT"
		case p.isWord(cur, "BOLD"), p.isWord(cur, "ITALIC"), p.isWord(cur, "UNDERLINE"):
			p.advance() // DEFINE FONT style flags, no value
			continue
		// `DEFINE CELL ... AUTO SIZE` — coluna de relatório com largura
		// automática (TReport): par de flags sem valor, "SIZE" aqui não é
		// seguido de w,h como na cláusula normal de SIZE.
		case p.isWord(cur, "AUTO") && p.isKeyword(p.peekAt(1), "SIZE"):
			p.advance()
			p.advance()
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

// parseAppendFrom handles `Append From <expr> [Via <expr>] [Fields
// f1,f2,...] [For <expr>] [While <expr>]`, desugaring to a dropped
// `APPEND_FROM(source, via, fields..., for, while)` call.
func (p *Parser) parseAppendFrom() (ast.Statement, error) {
	tok := p.advance() // APPEND
	p.advance()        // FROM

	source, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{source}

	for {
		cur := p.peek()
		switch {
		case p.isWord(cur, "VIA"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "VIA"}, val)
		case p.isWord(cur, "FIELDS"):
			p.advance()
			vals, err := p.parseCommaValues()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "FIELDS"})
			args = append(args, vals...)
		case p.isKeyword(cur, "FOR"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "FOR"}, val)
		case p.isKeyword(cur, "WHILE"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "WHILE"}, val)
		case p.isKeyword(cur, "REST"):
			p.advance()
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "REST"})
		case p.isKeyword(cur, "NEXT"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "NEXT"}, val)
		default:
			goto done
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "APPEND_FROM", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseCopyTo handles `Copy To <expr> [For expr] [While expr] [Via expr]
// [Fields f1,f2,...] [SDF] [DELIMITED [WITH expr]] [REST] [NEXT expr]`,
// desugaring to a dropped `COPY_TO(dest, ...)` call.
// parseIndexOn handles `Index On <expr> [Tag <expr>] [For <expr>] [Unique]
// [Descending] [Additive] To <expr>`, desugarado para `INDEX_ON(key, ...)`.
func (p *Parser) parseIndexOn() (ast.Statement, error) {
	tok := p.advance() // INDEX
	p.advance()        // ON

	key, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{key}

	for {
		cur := p.peek()
		switch {
		case p.isWord(cur, "TAG"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "TAG"}, val)
		case p.isKeyword(cur, "FOR"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "FOR"}, val)
		case p.isWord(cur, "UNIQUE"), p.isWord(cur, "DESCENDING"), p.isWord(cur, "ADDITIVE"):
			p.advance()
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: strings.ToUpper(cur.Value)})
		case p.isKeyword(cur, "TO"):
			p.advance()
			val, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "TO"}, val)
		default:
			goto done
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "INDEX_ON", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseDeleteCommand handles the two Clipper "DELETE" forms:
//
//	Delete File <expr>                         — apaga arquivo do disco
//	Delete [For <expr>] [While <expr>] [Next <expr>] [Record <expr>] [Rest] [All]
//
// — marca registro(s) do alias atual para deleção. Ambas descartadas (sem
// engine de banco por trás), mesmo espírito de COPY TO/APPEND FROM.
func (p *Parser) parseDeleteCommand() (ast.Statement, error) {
	tok := p.advance() // DELETE
	if p.isWord(p.peek(), "FILE") {
		p.advance()
		target, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "DELETE_FILE", Args: []ast.Expression{target}}
		return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
	}
	args := []ast.Expression{}
	clauseVal := func(cur lexer.Token, name string) error {
		p.advance()
		val, err := p.parseOr()
		if err != nil {
			return err
		}
		args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: name}, val)
		return nil
	}
	for {
		cur := p.peek()
		var err error
		switch {
		case p.isKeyword(cur, "FOR"):
			err = clauseVal(cur, "FOR")
		case p.isKeyword(cur, "WHILE"):
			err = clauseVal(cur, "WHILE")
		case p.isKeyword(cur, "NEXT"):
			err = clauseVal(cur, "NEXT")
		case p.isWord(cur, "RECORD"):
			err = clauseVal(cur, "RECORD")
		case p.isWord(cur, "REST"), p.isWord(cur, "ALL"):
			p.advance()
		default:
			goto done
		}
		if err != nil {
			return nil, err
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "DELETE_RECORD", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseLocateCommand handles Clipper's `Locate For <expr> [While <expr>]`
// — finds the first record in the current alias matching the condition
// (positions the record pointer; Found() reflects the result). Parsed and
// discarded, same spirit as DELETE FOR/WHILE.
func (p *Parser) parseLocateCommand() (ast.Statement, error) {
	tok := p.advance() // LOCATE
	args := []ast.Expression{}
	clauseVal := func(cur lexer.Token, name string) error {
		p.advance()
		val, err := p.parseOr()
		if err != nil {
			return err
		}
		args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: name}, val)
		return nil
	}
	for {
		cur := p.peek()
		var err error
		switch {
		case p.isKeyword(cur, "FOR"):
			err = clauseVal(cur, "FOR")
		case p.isKeyword(cur, "WHILE"):
			err = clauseVal(cur, "WHILE")
		default:
			goto done
		}
		if err != nil {
			return nil, err
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "LOCATE_RECORD", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parsePrepareEnvironment handles:
//
//	Prepare Environment Empresa <expr> Filial <expr> [Modulo <expr>]
//	    [Tables <expr>,...]
//
// — comando batch (fora de rotina interativa) que abre empresa/filial/
// tabelas; desugarizado para uma chamada solta e descartada, mesmo espírito
// de DEFINE/COPY TO.
// parseParamType handles:
//
//	ParamType <n> Var <name> As <type> [Default <expr>]
//
// — declaração de metadados de parâmetro (tipo/obrigatoriedade) usada em
// objetos de negócio/REST; desugarizada para uma chamada solta e
// descartada, mesmo espírito de PREPARE ENVIRONMENT/DEFINE.
func (p *Parser) parseParamType() (ast.Statement, error) {
	tok := p.advance() // PARAMTYPE
	args := []ast.Expression{}
	if p.peek().Type == lexer.TOKEN_NUMBER {
		n, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		args = append(args, n)
	}
	if p.isWord(p.peek(), "VAR") {
		p.advance()
		nameTok, err := p.expectName()
		if err != nil {
			return nil, err
		}
		args = append(args, &ast.StringLit{Loc: p.posFromToken(nameTok), Value: nameTok.Value})
	}
	if p.isKeyword(p.peek(), "AS") {
		p.advance()
		typeTok := p.peek()
		typeName := p.parseTypeName()
		args = append(args, &ast.StringLit{Loc: p.posFromToken(typeTok), Value: typeName})
	}
	if p.isWord(p.peek(), "OPTIONAL") {
		p.advance()
	}
	if p.isWord(p.peek(), "DEFAULT") {
		p.advance()
		val, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "PARAM_TYPE", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

func (p *Parser) parsePrepareEnvironment() (ast.Statement, error) {
	tok := p.advance() // PREPARE
	p.advance()         // ENVIRONMENT
	args := []ast.Expression{}
	clauseVal := func(cur lexer.Token, name string) error {
		p.advance()
		vals, err := p.parseCommaValues()
		if err != nil {
			return err
		}
		args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: name})
		args = append(args, vals...)
		return nil
	}
	for {
		cur := p.peek()
		var err error
		switch {
		case p.isWord(cur, "EMPRESA"):
			err = clauseVal(cur, "EMPRESA")
		case p.isWord(cur, "FILIAL"):
			err = clauseVal(cur, "FILIAL")
		case p.isWord(cur, "MODULO"):
			err = clauseVal(cur, "MODULO")
		case p.isWord(cur, "TABLES"):
			err = clauseVal(cur, "TABLES")
		default:
			goto done
		}
		if err != nil {
			return nil, err
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "PREPARE_ENVIRONMENT", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

func (p *Parser) parseCopyTo() (ast.Statement, error) {
	tok := p.advance() // COPY
	p.advance()        // TO

	dest, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{dest}

	flag := func(name string) {
		args = append(args, &ast.StringLit{Loc: p.posFromToken(tok), Value: name})
	}
	clauseVal := func(cur lexer.Token, name string) error {
		p.advance()
		val, err := p.parseOr()
		if err != nil {
			return err
		}
		args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: name}, val)
		return nil
	}

	for {
		cur := p.peek()
		var err error
		switch {
		case p.isKeyword(cur, "FOR"):
			err = clauseVal(cur, "FOR")
		case p.isKeyword(cur, "WHILE"):
			err = clauseVal(cur, "WHILE")
		case p.isWord(cur, "VIA"):
			err = clauseVal(cur, "VIA")
		case p.isKeyword(cur, "NEXT"):
			err = clauseVal(cur, "NEXT")
		case p.isWord(cur, "FIELDS"):
			p.advance()
			vals, verr := p.parseCommaValues()
			if verr != nil {
				return nil, verr
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(cur), Value: "FIELDS"})
			args = append(args, vals...)
		case p.isWord(cur, "SDF"):
			p.advance()
			flag("SDF")
		case p.isKeyword(cur, "REST"):
			p.advance()
			flag("REST")
		case p.isWord(cur, "DELIMITED"):
			p.advance()
			flag("DELIMITED")
			if p.isWord(p.peek(), "WITH") {
				err = clauseVal(p.peek(), "WITH")
			}
		default:
			goto done
		}
		if err != nil {
			return nil, err
		}
	}
done:
	call := &ast.CallExpr{Loc: p.posFromToken(tok), Name: "COPY_TO", Args: args}
	return &ast.ExprStmt{Loc: p.posFromToken(tok), Expr: call}, nil
}

// parseSetCommand handles Clipper's `SET <option> TO [<value>]` /
// `SET <option> ON|OFF` family (SET DEVICE TO SCREEN, SET FILTER TO ...,
// SET DELETED ON, ...), desugaring to a dropped `SET_<OPTION>(value)` call —
// this interpreter doesn't model any of these runtime options.
func (p *Parser) parseSetCommand() (ast.Statement, error) {
	tok := p.advance() // SET
	// nameTok pode ser reservado (`SET DELETE ON`, `SET FUNCTION ...`), não
	// só identificador — expectName aceita ambos.
	nameTok, err := p.expectName()
	if err != nil {
		return nil, err
	}
	args := []ast.Expression{}
	// `SET KEY <nKey> TO [<uBlock>]` — o keycode é um argumento posicional
	// antes do TO, diferente das demais opções de SET.
	if p.isWord(nameTok, "KEY") && !p.isKeyword(p.peek(), "TO") {
		keyExpr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args = append(args, keyExpr)
	}
	// `SET MESSAGE OF oWnd TO expr [NOINSET] [FONT oFont]` — cláusula OF
	// (janela/controle alvo) antes do TO, e clausulas finais soltas.
	if p.isWord(p.peek(), "OF") {
		p.advance()
		target, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		args = append(args, target)
	}
	if p.isKeyword(p.peek(), "TO") {
		p.advance()
		// `SET FILTER TO` with nothing after (clearing the option) is also
		// common; only a STRING/NUMBER/LPAREN is unambiguously a value. A
		// bare IDENT is only treated as one if it isn't itself the start of
		// the next `SET ...` statement, nor the target of an assignment on
		// the next line (`Set Filter to` \n `cChave := ...` is two
		// statements, not `SET_FILTER(cChave)` followed by a stray `:=`).
		hasValue := false
		switch p.peek().Type {
		case lexer.TOKEN_STRING, lexer.TOKEN_NUMBER, lexer.TOKEN_LPAREN, lexer.TOKEN_LBRACE:
			hasValue = true
		case lexer.TOKEN_IDENT:
			hasValue = !p.isWord(p.peek(), "SET") && p.peekAt(1).Type != lexer.TOKEN_ASSIGN &&
				!((p.peekAt(1).Type == lexer.TOKEN_PLUS || p.peekAt(1).Type == lexer.TOKEN_MINUS ||
					p.peekAt(1).Type == lexer.TOKEN_STAR || p.peekAt(1).Type == lexer.TOKEN_SLASH) &&
					p.peekAt(2).Type == lexer.TOKEN_ASSIGN)
		}
		if hasValue {
			vals, err := p.parseCommaValues()
			if err != nil {
				return nil, err
			}
			args = append(args, vals...)
		}
	} else if p.isWord(p.peek(), "ON") || p.isWord(p.peek(), "OFF") {
		args = append(args, &ast.BoolLit{Loc: p.posFromToken(tok), Value: p.isWord(p.peek(), "ON")})
		p.advance()
	}
	// Cláusulas finais soltas (NOINSET flag, FONT expr, ...) — parseadas e
	// descartadas, mesmo espírito das cláusulas de @ e DEFINE.
	for {
		switch {
		case p.isWord(p.peek(), "NOINSET"):
			p.advance()
		case p.isWord(p.peek(), "FONT"):
			p.advance()
			if _, err := p.parseOr(); err != nil {
				return nil, err
			}
		default:
			goto doneClauses
		}
	}
doneClauses:
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
		// `@ x1,y1 TO x2,y2 DIALOG <var> TITLE "..." ...` — legacy dialog
		// creation syntax (equivalente a `DEFINE MSDIALOG <var> FROM
		// x1,y1 TO x2,y2 TITLE ...`), distinto do desenho de caixa (BOX).
		if p.isWord(p.peek(), "DIALOG") {
			dlgTok := p.advance()
			varTok, err := p.expect(lexer.TOKEN_IDENT)
			if err != nil {
				return nil, err
			}
			clauses := map[string][]ast.Expression{}
			for {
				cur := p.peek()
				var name string
				switch {
				case p.isKeyword(cur, "TITLE"):
					name = "TITLE"
				case p.isWord(cur, "PIXEL"):
					p.advance()
					continue
				default:
					goto dlgDone
				}
				p.advance()
				vals, err := p.parseCommaValues()
				if err != nil {
					return nil, err
				}
				clauses[name] = vals
			}
		dlgDone:
			nilExpr := func() ast.Expression { return &ast.NilLit{Loc: p.posFromToken(dlgTok)} }
			at := func(name string, i int) ast.Expression {
				if v, ok := clauses[name]; ok && i < len(v) {
					return v[i]
				}
				return nilExpr()
			}
			call := &ast.CallExpr{Loc: p.posFromToken(dlgTok), Name: "MSDIALOG",
				Args: []ast.Expression{x, y, x2, y2, at("TITLE", 0)}}
			target := &ast.Ident{Loc: p.posFromToken(varTok), Name: varTok.Value}
			return &ast.AssignStmt{Loc: p.posFromToken(tok), Target: target, Value: call, Op: ":="}, nil
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
			if p.isWord(clauseTok, "PIXEL") || p.isWord(clauseTok, "CLICKFOCUS") || p.isWord(clauseTok, "RESET") || p.isWord(clauseTok, "MEMO") || p.isWord(clauseTok, "MULTILINE") {
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

	for p.isAtClauseWord(p.peek()) ||
		(p.peek().Type == lexer.TOKEN_NUMBER && p.peek().Value == "3" &&
			p.peekAt(1).Type == lexer.TOKEN_IDENT && strings.EqualFold(p.peekAt(1).Value, "D")) ||
		(p.isWord(p.peek(), "NO") && (p.isWord(p.peekAt(1), "SCROLL") || p.isWord(p.peekAt(1), "UNDERLINE"))) {
		// `@ ... RADIO ... ITEMS ... 3D SIZE w,h ...` — "3D" tokeniza como
		// NUMBER "3" + IDENT "D" (identificador não pode começar com
		// dígito); flag de layout do RADIO/CHECKBOX, sem valor.
		if p.peek().Type == lexer.TOKEN_NUMBER && p.peek().Value == "3" &&
			p.peekAt(1).Type == lexer.TOKEN_IDENT && strings.EqualFold(p.peekAt(1).Value, "D") {
			p.advance()
			p.advance()
			continue
		}
		// `@ ... BROWSE ... NO SCROLL ...` / `@ ... GET ... NO UNDERLINE ...`
		// — flags de duas palavras (DSL mobile FDA).
		if p.isWord(p.peek(), "NO") && (p.isWord(p.peekAt(1), "SCROLL") || p.isWord(p.peekAt(1), "UNDERLINE")) {
			p.advance()
			p.advance()
			continue
		}
		clauseTok := p.advance()
		if p.isWord(clauseTok, "PIXEL") || p.isWord(clauseTok, "CLICKFOCUS") || p.isWord(clauseTok, "RESET") || p.isWord(clauseTok, "MEMO") || p.isWord(clauseTok, "FIELDS") || p.isWord(clauseTok, "NOSCROLL") || p.isWord(clauseTok, "NOBORDER") || p.isWord(clauseTok, "PASSWORD") || p.isWord(clauseTok, "LOWERED") || p.isWord(clauseTok, "READONLY") || p.isWord(clauseTok, "VERTICAL") || p.isWord(clauseTok, "HORIZONTAL") || p.isWord(clauseTok, "MULTILINE") || p.isWord(clauseTok, "HSCROLL") || p.isWord(clauseTok, "VSCROLL") || p.isWord(clauseTok, "HASBUTTON") || p.isWord(clauseTok, "SYMBOL") {
			continue // flag clauses, no value
		}
		// `@ ... LISTBOX ... ON DBLCLICK <expr> ...` — o nome do evento
		// (DBLCLICK, ENTER, ...) vem junto de ON formando o nome da
		// cláusula, e o valor é uma única expressão (callback), não uma
		// lista separada por vírgula.
		if p.isWord(clauseTok, "ON") {
			eventTok := p.advance()
			clauseName := "ON_" + strings.ToUpper(eventTok.Value)
			val, err := p.parseAssignableExpr()
			if err != nil {
				return nil, err
			}
			if _, isBlock := val.(*ast.CodeBlock); !isBlock {
				val = &ast.CodeBlock{Loc: p.posFromToken(clauseTok), Expr: val}
			}
			args = append(args, &ast.StringLit{Loc: p.posFromToken(clauseTok), Value: clauseName}, val)
			continue
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
		"SIZE", "PICTURE", "VALID", "WHEN", "COLOR", "COLORS", "FONT", "PIXEL",
		"MESSAGE", "ACTION", "OF", "DECODE", "F3", "CLICKFOCUS", "RANGE",
		"MAXLENGTH", "MASK", "RESET", "TITLE", "VAR", "MEMO",
		// `@ y,x GROUP var TO y2,x2 OF window LABEL "..." PIXEL` — GROUP
		// (caixa de agrupamento) usa TO para a segunda coordenada e LABEL
		// para o texto, como cláusulas normais (não como o `@ TO` de caixa
		// sem verbo, tratado antes de chegar aqui).
		"TO", "LABEL",
		// `@ ... LISTBOX ... FIELDS HEADER a,b,c ... ON DBLCLICK expr
		// NOSCROLL OF window PIXEL` — mais cláusulas do LISTBOX.
		"FIELDS", "HEADER", "ON", "NOSCROLL", "FIELDSIZES", "MULTILINE", "HSCROLL", "VSCROLL", "HASBUTTON",
		"RESOLUTION", "VALUE",
		// `@ y,x BUTTON var PROMPT "texto" SIZE w,h OF window PIXEL ACTION
		// expr` — PROMPT é o texto do botão.
		"PROMPT",
		// `@ y,x RADIO var VAR nVar ITEMS v1,v2,... SIZE w,h OF window PIXEL`
		// (o DSL mobile FDA usa o singular ITEM para COMBOBOX)
		"ITEMS", "ITEM",
		// `@ y,x BITMAP var RESOURCE|RESNAME "nome" SIZE w,h PIXEL NOBORDER OF window`
		"RESOURCE", "RESNAME", "NOBORDER",
		// `@ y,x MSGET var SIZE w,h PASSWORD OF window PIXEL`
		"PASSWORD",
		// `@ y,x MSPANEL var OF window SIZE w,h LOWERED`
		"LOWERED",
		// `@ y,x GET var VAR nome SIZE w,h OF window PICTURE p READONLY`
		"READONLY",
		// `@ y,x LISTBOX var FIELDS HEADER a,b,c SIZES w1,w2,w3 SIZE w,h`
		"SIZES",
		// `@ y,x METER var VAR n TOTAL 100 SIZE w,h`
		"TOTAL",
		// `@ y,x SCROLLBOX var VERTICAL|HORIZONTAL OF window PIXEL`
		"VERTICAL", "HORIZONTAL",
		// `@ y,x BMPBUTTON TYPE n ACTION expr` — botão bitmap com número de
		// estilo predefinido (TYPE), distinto da @ BUTTON comum.
		"TYPE",
		// `@ y,x TO y2,x2 CAPTION expr OF oFolder` — expansão de folder do
		// DSL mobile (FDA); `@ y,x BUTTON o CAPTION x SYMBOL ACTION f()` —
		// botão com bitmap simbólico do mesmo DSL.
		"CAPTION", "SYMBOL",
		// `@ y,x To y2,x2 MultiLine Object oMulti` — caixa multi-linha
		// (TMultiget legado) com var de saída via OBJECT.
		"OBJECT",
		// `@ y,x VTSAY cTexto VTGET var VALID expr` — DSL VT100 (coletores
		// de dados): VTGET encadeia um campo de entrada na mesma linha @.
		"VTGET",
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
	p.advance()        // MSDIALOG

	// alvo pode ser `::oDlg` (propriedade do próprio objeto)
	if p.peek().Type == lexer.TOKEN_DOUBLECOLON {
		p.advance()
	}
	varTok, err := p.expectName()
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
		case p.isWord(cur, "CENTERED"), p.isWord(cur, "CENTER"), p.isWord(cur, "ICONIZED"), p.isWord(cur, "ICONIZE"):
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
		// parseAssignRHS: o valor pode ser outra atribuição encadeada
		// (`x[9] := x[10] := ... := 0` dentro de codeblock real).
		val, err := p.parseAssignRHS()
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

	// Same-line guard: newlines are stripped before parsing, so without this
	// a `++x`/`--x` prefix statement on the next source line would glue onto
	// whatever expression this codeblock item just finished parsing.
	if (p.peek().Type == lexer.TOKEN_INCREMENT || p.peek().Type == lexer.TOKEN_DECREMENT) &&
		p.pos > 0 && p.tokens[p.pos-1].Line == p.peek().Line {
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

	// x++ / x-- desugars to x := x + 1 / x := x - 1. Same-line guard as in
	// parsePostfix: a `++x` prefix statement on the next source line must
	// not glue onto this statement's already-parsed expr.
	if (p.peek().Type == lexer.TOKEN_INCREMENT || p.peek().Type == lexer.TOKEN_DECREMENT) &&
		p.pos > 0 && p.tokens[p.pos-1].Line == p.peek().Line {
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
	left, err := p.parsePower()
	if err != nil {
		return nil, err
	}
	for (p.peek().Type == lexer.TOKEN_STAR || p.peek().Type == lexer.TOKEN_SLASH || p.peek().Type == lexer.TOKEN_PERCENT) && p.peekAt(1).Type != lexer.TOKEN_ASSIGN {
		tok := p.advance()
		right, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: tok.Value, Left: left, Right: right}
	}
	return left, nil
}

// parsePower trata `^` (e `**`, sinônimo Clipper) — exponenciação, acima da
// multiplicação e associativa à direita (2^3^2 = 2^(3^2)).
func (p *Parser) parsePower() (ast.Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if p.peek().Type == lexer.TOKEN_CARET {
		tok := p.advance()
		right, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Loc: p.posFromToken(tok), Op: "^", Left: left, Right: right}
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
	// `++x` / `--x` — forma prefixa (a pós-fixa `x++`/`x--` já é tratada em
	// parseCodeBlockItem/parseExprStatement). Mesma semântica: incrementa e
	// devolve o novo valor, açúcar para `x := x + 1`.
	if p.peek().Type == lexer.TOKEN_INCREMENT || p.peek().Type == lexer.TOKEN_DECREMENT {
		opTok := p.advance()
		op := "+"
		if opTok.Type == lexer.TOKEN_DECREMENT {
			op = "-"
		}
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		one := &ast.NumberLit{Loc: p.posFromToken(opTok), Value: 1, Str: "1"}
		combined := &ast.BinaryOp{Loc: p.posFromToken(opTok), Op: op, Left: operand, Right: one}
		return &ast.AssignExpr{Loc: p.posFromToken(opTok), Target: operand, Value: combined}, nil
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
				// alias->(expr1, expr2, ...) — same comma-sequence production
				// as plain `(a, b, c)`, evaluates all and yields the last.
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
					expr = items[0]
				} else {
					expr = &ast.SeqExpr{Loc: p.posFromToken(tok), Exprs: items}
				}
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
			// Field name can collide with a reserved word (`alias->END`,
			// `alias->DELETE`, ...) — accept KEYWORD too, like expectName.
			fieldTok, err := p.expectName()
			if err != nil {
				return nil, err
			}
			if ident, ok := expr.(*ast.Ident); ok {
				expr = &ast.FieldAccess{Loc: p.posFromToken(tok), Alias: ident.Name, Field: fieldTok.Value}
			}
		case lexer.TOKEN_LPAREN:
			// Newlines are stripped before parsing, so a call-like `(` that
			// starts a NEW statement on the next source line (e.g. a bare
			// `(alias)->field` statement right after this expression) must
			// not glue onto the end of THIS expression as a call. Require
			// the '(' to be on the same line as the token right before it.
			if p.pos == 0 || p.tokens[p.pos-1].Line != tok.Line {
				goto done
			}
			if ident, ok := expr.(*ast.Ident); ok {
				p.advance()
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				expr = &ast.NewExpr{Loc: p.posFromToken(tok), ClassName: ident.Name, Args: args}
			} else if _, ok := expr.(*ast.MacroExp); ok {
				// `&cFunc.()` / `&(expr)()` — chamada de função cujo nome vem
				// de uma macro (`&`), idioma clássico do Clipper/AdvPL. O VM
				// ainda não resolve/chama uma função por nome dinâmico em
				// runtime (mesma simplificação já feita para alias->&macro,
				// ver TOKEN_ARROW acima): consome os parênteses/argumentos
				// para o parsing suceder, mas não modela a invocação — o
				// resultado da expressão continua sendo só a leitura da
				// macro, sem efetivamente chamá-la.
				p.advance()
				if _, err := p.parseArguments(); err != nil {
					return nil, err
				}
			} else {
				goto done
			}
		case lexer.TOKEN_INCREMENT, lexer.TOKEN_DECREMENT:
			// `nParam++` used inline as a call argument, not just as its own
			// statement. Desugars to `(expr := expr +/- 1)` — evaluates to
			// the new value, so this is pre- rather than post-increment
			// semantics, but real usage here is always a running counter
			// where callers don't depend on which one they get.
			//
			// Newlines are stripped before parsing, so nothing else marks a
			// statement boundary here — without this check, a `++x` prefix
			// statement on the NEXT source line gets glued onto whatever
			// expression ended the previous statement (e.g. `y := f()` \n
			// `++x` misparses as `(f())++`). Require the operator to be on
			// the same line as the token right before it.
			if p.pos == 0 || p.tokens[p.pos-1].Line != tok.Line {
				goto done
			}
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

// parseAssignableExpr faz o mesmo papel que parseCodeBlockItem para o valor
// de uma cláusula/argumento avulso: parseia uma expressão e, se sobrar um
// operador de atribuição (:=, +=, -=, *=, /=) logo depois — o parser de
// binário aditivo/multiplicativo já evita consumi-lo sozinho — completa
// como AssignExpr. Usado em contextos fora de codeblock/argumentos de
// função que também aceitam atribuição como valor (ex.: `ON evento expr`).
func (p *Parser) parseAssignableExpr() (ast.Expression, error) {
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
	if (p.peek().Type == lexer.TOKEN_PLUS || p.peek().Type == lexer.TOKEN_MINUS ||
		p.peek().Type == lexer.TOKEN_STAR || p.peek().Type == lexer.TOKEN_SLASH) &&
		p.peekAt(1).Type == lexer.TOKEN_ASSIGN {
		opTok := p.advance()
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		combined := &ast.BinaryOp{Loc: p.posFromToken(tok), Op: opTok.Value, Left: expr, Right: val}
		return &ast.AssignExpr{Loc: p.posFromToken(tok), Target: expr, Value: combined}, nil
	}
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
			// `f(aArray[i] := val, ...)` / `f(nOrd += 1, ...)` — atribuição
			// (simples ou composta) como argumento onde o alvo não é um
			// identificador simples (a checagem de parâmetro nomeado acima
			// só cobre `ident := valor`).
			arg, err := p.parseAssignableExpr()
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
		// namespace segments. A segment can collide with a reserved word
		// (`totvs.framework.treports.date.stringToTimeStamp` — "date" lexes
		// as TOKEN_KEYWORD), so accept either token type.
		name := tok.Value
		for p.peek().Type == lexer.TOKEN_DOT &&
			(p.peekAt(1).Type == lexer.TOKEN_IDENT || p.peekAt(1).Type == lexer.TOKEN_KEYWORD) {
			p.advance()
			name += "." + p.advance().Value
		}
		// Newlines are stripped before parsing, so a `(` starting a NEW
		// statement on the next source line (e.g. a bare `(alias)->field`
		// statement right after a bare identifier expression) must not glue
		// onto this identifier as a call. Require same line.
		if p.peek().Type == lexer.TOKEN_LPAREN && p.peek().Line == tok.Line {
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
		// nome de propriedade/método pode colidir com keyword (::Default()).
		propTok, err := p.expectName()
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
					// nome de parâmetro pode colidir com palavra reservada
					// ({|Self| ...} em código real) — aceita keyword também.
					paramTok, err := p.expectName()
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
				// parseCodeBlockItem (not plain parseExpression) so an
				// element can itself be an assignment (`{a, b := c, d}`,
				// seen as a `{||...}`-less block body used as an array in
				// real Protheus VALID/ACTION clauses).
				elem, err := p.parseCodeBlockItem()
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
		// `&nome.` — o ponto final é um terminador explícito clássico do
		// Clipper/AdvPL para a substituição de macro (`&cFunc.()`), usado
		// quando o que segue poderia ser confundido com o resto do nome.
		// Não carrega significado semântico próprio; só é consumido aqui.
		if p.peek().Type == lexer.TOKEN_DOT {
			p.advance()
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
			// Date(2024,1,1). Same shape as If()/IIF() above. BREAK is a
			// statement keyword, but `Break(oError)` inside a codeblock
			// (idioma de ErrorBlock) é uma chamada de função.
			if (strings.EqualFold(tok.Value, "ARRAY") || strings.EqualFold(tok.Value, "DATE") || strings.EqualFold(tok.Value, "OBJECT") || strings.EqualFold(tok.Value, "BREAK")) &&
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
