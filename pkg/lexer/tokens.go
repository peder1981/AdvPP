package lexer

import "fmt"

type TokenType int

const (
	TOKEN_EOF TokenType = iota
	TOKEN_NEWLINE

	// Literals
	TOKEN_NUMBER
	TOKEN_STRING
	TOKEN_IDENT
	TOKEN_KEYWORD

	// Logical literals
	TOKEN_TRUE
	TOKEN_FALSE
	TOKEN_NIL

	// Operators
	TOKEN_PLUS
	TOKEN_MINUS
	TOKEN_STAR
	TOKEN_SLASH
	TOKEN_PERCENT
	TOKEN_ASSIGN    // :=
	TOKEN_EQ        // ==
	TOKEN_NEQ       // != or <>
	TOKEN_LT        // <
	TOKEN_GT        // >
	TOKEN_LTE       // <=
	TOKEN_GTE       // >=
	TOKEN_DOT_AND   // .And.
	TOKEN_DOT_OR    // .Or.
	TOKEN_DOT_NOT   // .Not.
	TOKEN_INCREMENT // ++
	TOKEN_DECREMENT // --

	// Punctuation
	TOKEN_LPAREN      // (
	TOKEN_RPAREN      // )
	TOKEN_LBRACKET    // [
	TOKEN_RBRACKET    // ]
	TOKEN_LBRACE      // {
	TOKEN_RBRACE      // }
	TOKEN_SEMICOLON   // ;
	TOKEN_COMMA       // ,
	TOKEN_DOT         // .
	TOKEN_COLON       // :
	TOKEN_DOUBLECOLON // ::
	TOKEN_ARROW       // ->
	TOKEN_AT          // @
	TOKEN_AMPERSAND   // &
	TOKEN_PIPE        // |
	TOKEN_CARET       // ^
	TOKEN_TILDE       // ~
	TOKEN_DOLLAR      // $
	TOKEN_QUESTION    // ?
	TOKEN_HASH        // #

	// Preprocessor
	TOKEN_PREPROC_INCLUDE
	TOKEN_PREPROC_DEFINE
	TOKEN_PREPROC_UNDEFINE
	TOKEN_PREPROC_IFDEF
	TOKEN_PREPROC_IFNDEF
	TOKEN_PREPROC_ELSE
	TOKEN_PREPROC_ENDIF
	TOKEN_PREPROC_XCOMMAND
	TOKEN_PREPROC_XTRANSLATE
	TOKEN_PREPROC_COMMAND
	TOKEN_PREPROC_TRANSLATE
	TOKEN_DIRECTIVE

	// Special
	TOKEN_LINECOMMENT
	TOKEN_BLOCKCOMMENT
)

type Token struct {
	Type     TokenType
	Value    string
	Line     int
	Col      int
	FileName string
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d: %q at %s:%d:%d)", t.Type, t.Value, t.FileName, t.Line, t.Col)
}

func (t Token) IsKeyword(kw string) bool {
	if t.Type != TOKEN_KEYWORD {
		return false
	}
	return equalIgnoreCase(t.Value, kw)
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 32
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

var Keywords = map[string]bool{
	// Function/Procedure declarations
	"USER": true, "FUNCTION": true, "STATIC": true, "PROCEDURE": true,
	"RETURN": true, "MAIN": true,

	// Control flow
	"IF": true, "ELSEIF": true, "ELSE": true, "ENDIF": true,
	"FOR": true, "TO": true, "STEP": true, "NEXT": true,
	"WHILE": true, "ENDDO": true, "END": true, "DO": true,
	"CASE": true, "ENDCASE": true, "OTHERWISE": true,
	"EXIT": true, "LOOP": true, "BREAK": true, "CONTINUE": true,

	// Error handling
	"BEGIN": true, "SEQUENCE": true, "RECOVER": true, "USING": true,
	"TRY": true, "CATCH": true, "FINALLY": true, "ENDTRY": true,
	"THROW": true,

	// Variable scopes
	"LOCAL": true, "PRIVATE": true, "PUBLIC": true, "GLOBAL": true,
	"PARAMETERS": true, "PARAMETER": true, "DEFAULT": true,

	// Class system
	"CLASS": true, "ENDCLASS": true, "DATA": true, "METHOD": true,
	"CONSTRUCTOR": true, "FROM": true, "AS": true,
	"PUBLIC_MOD": true, "PRIVATE_MOD": true, "PROTECTED": true,
	"SELF": true, "OPERATOR": true,

	// Interfaces
	"INTERFACE": true, "ENDINTERFACE": true, "IMPLEMENTS": true,

	// Types
	"NIL": true, "VARIANT": true, "VARIADIC": true, "JSON": true,
	"INTEGER": true, "DOUBLE": true, "DECIMAL": true,
	"CHARACTER": true, "NUMERIC": true, "LOGICAL": true,
	"DATE": true, "ARRAY": true, "OBJECT": true,
	"MEMO": true, "FIXED": true, "UNDEFINED": true,

	// Logical operators (word form)
	"AND": true, "OR": true, "NOT": true,

	// Misc
	"IN": true, "IS": true, "OF": true, "ON": true,
	"THREAD": true, "JOB": true,
	"EXPORT": true, "ENUM": true, "ENDENUM": true,
	"REST": true, "GET": true, "POST": true, "PUT": true,
	"DELETE": true, "PATCH": true,
	"SAY": true, "SIZE": true, "PIXEL": true, "COORD": true,
	"TITLE": true, "ACTION": true, "WHEN": true, "VALID": true,
	"UPDATE": true, "READ": true, "ADD": true, "ACTIVATE": true,
	"WINDOW": true, "DIALOG": true, "BUTTON": true, "SBUTTON": true,
	"MENU": true, "MENUITEM": true, "BROWSE": true,
	"REPORT": true, "SECTION": true, "PANEL": true,
	"BLOCK": true, "VARNAME": true,
	"INCLUDE": true, "TRANSLATE": true, "COMMAND": true,
	"UNDEFINE": true, "IFDEF": true, "IFNDEF": true,
	"NAMESPACE": true,
}
