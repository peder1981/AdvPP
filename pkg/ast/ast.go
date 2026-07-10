package ast

import "fmt"

type Position struct {
	Line     int
	Col      int
	FileName string
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.FileName, p.Line, p.Col)
}

type Node interface {
	Pos() Position
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// Program is the root AST node
type Program struct {
	Loc             Position
	FileName        string
	Functions       []*FunctionDecl
	Classes         []*ClassDecl
	Methods         []*MethodImpl
	Body            []Statement
	Includes        []string
	Defines         map[string]string
	Namespace       string
	UsingNamespaces []string
}

func (p *Program) Pos() Position  { return p.Loc }
func (p *Program) String() string { return "Program" }
func (p *Program) statementNode() {}

// FunctionDecl represents User Function, Static Function, Function, Procedure
type FunctionDecl struct {
	Loc         Position
	Name        string
	IsUser      bool
	IsStatic    bool
	IsProcedure bool
	Params      []*Parameter
	ReturnType  string
	Body        []Statement
	ReturnExpr  Expression
	Annotations []*Annotation
}

func (f *FunctionDecl) Pos() Position  { return f.Loc }
func (f *FunctionDecl) String() string { return fmt.Sprintf("Function %s", f.Name) }
func (f *FunctionDecl) statementNode() {}

type Parameter struct {
	Loc     Position
	Name    string
	Type    string
	ByRef   bool
	Default Expression
}

// ClassDecl represents Class ... EndClass
type ClassDecl struct {
	Loc         Position
	Name        string
	Parent      string
	Namespace   string
	Properties  []*PropertyDecl
	Methods     []*MethodDecl
	Interfaces  []string
	Annotations []*Annotation
}

func (c *ClassDecl) Pos() Position  { return c.Loc }
func (c *ClassDecl) String() string { return fmt.Sprintf("Class %s", c.Name) }
func (c *ClassDecl) statementNode() {}

type PropertyDecl struct {
	Loc         Position
	Name        string
	Type        string
	IsPublic    bool
	IsPrivate   bool
	IsProtected bool
	Default     Expression
}

type MethodDecl struct {
	Loc           Position
	Name          string
	Params        []*Parameter
	ReturnType    string
	IsConstructor bool
	IsPublic      bool
	IsPrivate     bool
	IsProtected   bool
	IsStatic      bool
	Annotations   []*Annotation
}

type Annotation struct {
	Loc   Position
	Name  string
	Value string
}

// MethodImpl is a method implementation outside class block
type MethodImpl struct {
	Loc        Position
	Name       string
	ClassName  string
	Params     []*Parameter
	ReturnType string
	Body       []Statement
	ReturnExpr Expression
}

func (m *MethodImpl) Pos() Position  { return m.Loc }
func (m *MethodImpl) String() string { return fmt.Sprintf("Method %s Class %s", m.Name, m.ClassName) }
func (m *MethodImpl) statementNode() {}

// InterfaceDecl represents Interface ... EndInterface
type InterfaceDecl struct {
	Loc     Position
	Name    string
	Methods []*MethodDecl
}

func (i *InterfaceDecl) Pos() Position  { return i.Loc }
func (i *InterfaceDecl) String() string { return fmt.Sprintf("Interface %s", i.Name) }
func (i *InterfaceDecl) statementNode() {}

// --- Statements ---

type VarDecl struct {
	Loc   Position
	Scope string // local, private, public, static
	Name  string
	Type  string
	Value Expression
}

func (v *VarDecl) Pos() Position  { return v.Loc }
func (v *VarDecl) String() string { return fmt.Sprintf("%s %s", v.Scope, v.Name) }
func (v *VarDecl) statementNode() {}

// VarDeclGroup holds a comma-separated declaration line with more than one
// variable (`Local a, b := 1, c`) — extremely common in real AdvPL. Each
// name is its own VarDecl; the group just keeps them together as the one
// ast.Statement a single parseStatement() call must return.
type VarDeclGroup struct {
	Loc   Position
	Decls []*VarDecl
}

func (v *VarDeclGroup) Pos() Position  { return v.Loc }
func (v *VarDeclGroup) String() string { return "VarDeclGroup" }
func (v *VarDeclGroup) statementNode() {}

// DefaultGroup holds a comma-separated `Default` line with more than one
// variable (`Default a := 1, b := 2, c := 3`) — same rationale as
// VarDeclGroup: each pair is its own DefaultExpr, kept together as the one
// ast.Statement a single parseStatement() call must return.
type DefaultGroup struct {
	Loc      Position
	Defaults []*DefaultExpr
}

func (d *DefaultGroup) Pos() Position  { return d.Loc }
func (d *DefaultGroup) String() string { return "DefaultGroup" }
func (d *DefaultGroup) statementNode() {}

type AssignStmt struct {
	Loc    Position
	Target Expression
	Value  Expression
	Op     string // :=, =, +=, -=, *=, /=
}

func (a *AssignStmt) Pos() Position  { return a.Loc }
func (a *AssignStmt) String() string { return "Assign" }
func (a *AssignStmt) statementNode() {}

type IfStmt struct {
	Loc       Position
	Condition Expression
	ThenBody  []Statement
	ElseIfs   []*ElseIfClause
	ElseBody  []Statement
}

type ElseIfClause struct {
	Loc       Position
	Condition Expression
	Body      []Statement
}

func (i *IfStmt) Pos() Position  { return i.Loc }
func (i *IfStmt) String() string { return "If" }
func (i *IfStmt) statementNode() {}

type ForStmt struct {
	Loc     Position
	VarName string
	Start   Expression
	End     Expression
	Step    Expression
	Body    []Statement
}

func (f *ForStmt) Pos() Position  { return f.Loc }
func (f *ForStmt) String() string { return "For" }
func (f *ForStmt) statementNode() {}

type ForInStmt struct {
	Loc      Position
	VarName  string
	Iterable Expression
	Body     []Statement
}

func (f *ForInStmt) Pos() Position  { return f.Loc }
func (f *ForInStmt) String() string { return "ForIn" }
func (f *ForInStmt) statementNode() {}

type WhileStmt struct {
	Loc       Position
	Condition Expression
	Body      []Statement
}

func (w *WhileStmt) Pos() Position  { return w.Loc }
func (w *WhileStmt) String() string { return "While" }
func (w *WhileStmt) statementNode() {}

type DoCaseStmt struct {
	Loc       Position
	Cases     []*CaseClause
	Otherwise []Statement
}

type CaseClause struct {
	Loc       Position
	Condition Expression
	Body      []Statement
}

func (d *DoCaseStmt) Pos() Position  { return d.Loc }
func (d *DoCaseStmt) String() string { return "DoCase" }
func (d *DoCaseStmt) statementNode() {}

type ReturnStmt struct {
	Loc   Position
	Value Expression
}

func (r *ReturnStmt) Pos() Position  { return r.Loc }
func (r *ReturnStmt) String() string { return "Return" }
func (r *ReturnStmt) statementNode() {}

type ExitStmt struct {
	Loc Position
}

func (e *ExitStmt) Pos() Position  { return e.Loc }
func (e *ExitStmt) String() string { return "Exit" }
func (e *ExitStmt) statementNode() {}

type LoopStmt struct {
	Loc Position
}

func (l *LoopStmt) Pos() Position  { return l.Loc }
func (l *LoopStmt) String() string { return "Loop" }
func (l *LoopStmt) statementNode() {}

type BreakStmt struct {
	Loc   Position
	Value Expression
}

func (b *BreakStmt) Pos() Position  { return b.Loc }
func (b *BreakStmt) String() string { return "Break" }
func (b *BreakStmt) statementNode() {}

type BeginSequenceStmt struct {
	Loc         Position
	Body        []Statement
	RecoverBody []Statement
	UsingVar    string
}

func (b *BeginSequenceStmt) Pos() Position  { return b.Loc }
func (b *BeginSequenceStmt) String() string { return "BeginSequence" }
func (b *BeginSequenceStmt) statementNode() {}

type TryCatchStmt struct {
	Loc         Position
	Body        []Statement
	CatchVar    string
	CatchBody   []Statement
	FinallyBody []Statement
}

func (t *TryCatchStmt) Pos() Position  { return t.Loc }
func (t *TryCatchStmt) String() string { return "TryCatch" }
func (t *TryCatchStmt) statementNode() {}

type ThrowStmt struct {
	Loc   Position
	Value Expression
}

func (t *ThrowStmt) Pos() Position  { return t.Loc }
func (t *ThrowStmt) String() string { return "Throw" }
func (t *ThrowStmt) statementNode() {}

type ExprStmt struct {
	Loc  Position
	Expr Expression
}

func (e *ExprStmt) Pos() Position  { return e.Loc }
func (e *ExprStmt) String() string { return "ExprStmt" }
func (e *ExprStmt) statementNode() {}

// --- Expressions ---

type NumberLit struct {
	Loc   Position
	Value float64
	Str   string
}

func (n *NumberLit) Pos() Position   { return n.Loc }
func (n *NumberLit) String() string  { return fmt.Sprintf("%v", n.Value) }
func (n *NumberLit) expressionNode() {}

type StringLit struct {
	Loc   Position
	Value string
}

func (s *StringLit) Pos() Position   { return s.Loc }
func (s *StringLit) String() string  { return fmt.Sprintf("%q", s.Value) }
func (s *StringLit) expressionNode() {}

type BoolLit struct {
	Loc   Position
	Value bool
}

func (b *BoolLit) Pos() Position   { return b.Loc }
func (b *BoolLit) String() string  { return fmt.Sprintf("%v", b.Value) }
func (b *BoolLit) expressionNode() {}

// AssignExpr is assignment used as an expression value: `While (nAt :=
// AScan(aArr, x)) > 0` (Clipper's "assign and test" idiom), and codeblock
// bodies like `{|| x := 1}` where assignment is one of the block's
// comma-separated expressions rather than a statement.
type AssignExpr struct {
	Loc    Position
	Target Expression
	Value  Expression
}

func (a *AssignExpr) Pos() Position   { return a.Loc }
func (a *AssignExpr) String() string  { return "(assign)" }
func (a *AssignExpr) expressionNode() {}

// SeqExpr is Clipper's comma-expression-list-in-parens idiom, e.g.
// `If(cond, (a:Show(), b:Show(), c), NIL)` — a codeblock body without the
// `{|| }` wrapper. All expressions run in order; the value is the last one.
type SeqExpr struct {
	Loc   Position
	Exprs []Expression
}

func (s *SeqExpr) Pos() Position   { return s.Loc }
func (s *SeqExpr) String() string  { return "(seq)" }
func (s *SeqExpr) expressionNode() {}

type NilLit struct {
	Loc Position
}

func (n *NilLit) Pos() Position   { return n.Loc }
func (n *NilLit) String() string  { return "Nil" }
func (n *NilLit) expressionNode() {}

type DateLit struct {
	Loc   Position
	Value string // dd/mm/yyyy
}

func (d *DateLit) Pos() Position   { return d.Loc }
func (d *DateLit) String() string  { return d.Value }
func (d *DateLit) expressionNode() {}

type Ident struct {
	Loc  Position
	Name string
}

func (i *Ident) Pos() Position   { return i.Loc }
func (i *Ident) String() string  { return i.Name }
func (i *Ident) expressionNode() {}

type BinaryOp struct {
	Loc   Position
	Op    string
	Left  Expression
	Right Expression
}

func (b *BinaryOp) Pos() Position   { return b.Loc }
func (b *BinaryOp) String() string  { return fmt.Sprintf("(%s %s %s)", b.Left, b.Op, b.Right) }
func (b *BinaryOp) expressionNode() {}

type UnaryOp struct {
	Loc     Position
	Op      string
	Operand Expression
}

func (u *UnaryOp) Pos() Position   { return u.Loc }
func (u *UnaryOp) String() string  { return fmt.Sprintf("(%s%s)", u.Op, u.Operand) }
func (u *UnaryOp) expressionNode() {}

type CallExpr struct {
	Loc  Position
	Name string
	Args []Expression
}

func (c *CallExpr) Pos() Position   { return c.Loc }
func (c *CallExpr) String() string  { return fmt.Sprintf("%s(...)", c.Name) }
func (c *CallExpr) expressionNode() {}

type MethodCall struct {
	Loc    Position
	Object Expression
	Method string
	Args   []Expression
}

func (m *MethodCall) Pos() Position   { return m.Loc }
func (m *MethodCall) String() string  { return fmt.Sprintf("%s:%s(...)", m.Object, m.Method) }
func (m *MethodCall) expressionNode() {}

type PropertyAccess struct {
	Loc      Position
	Object   Expression
	Property string
}

func (p *PropertyAccess) Pos() Position   { return p.Loc }
func (p *PropertyAccess) String() string  { return fmt.Sprintf("%s:%s", p.Object, p.Property) }
func (p *PropertyAccess) expressionNode() {}

type SelfRef struct {
	Loc      Position
	Property string
}

func (s *SelfRef) Pos() Position   { return s.Loc }
func (s *SelfRef) String() string  { return fmt.Sprintf("::%s", s.Property) }
func (s *SelfRef) expressionNode() {}

type SelfMethodCall struct {
	Loc    Position
	Method string
	Args   []Expression
}

func (s *SelfMethodCall) Pos() Position   { return s.Loc }
func (s *SelfMethodCall) String() string  { return fmt.Sprintf("::%s(...)", s.Method) }
func (s *SelfMethodCall) expressionNode() {}

type FieldAccess struct {
	Loc   Position
	Alias string
	Field string
}

func (f *FieldAccess) Pos() Position   { return f.Loc }
func (f *FieldAccess) String() string  { return fmt.Sprintf("%s->%s", f.Alias, f.Field) }
func (f *FieldAccess) expressionNode() {}

type ArrayLit struct {
	Loc      Position
	Elements []Expression
}

func (a *ArrayLit) Pos() Position   { return a.Loc }
func (a *ArrayLit) String() string  { return "{...}" }
func (a *ArrayLit) expressionNode() {}

type ArrayAccess struct {
	Loc   Position
	Array Expression
	Index Expression
}

func (a *ArrayAccess) Pos() Position   { return a.Loc }
func (a *ArrayAccess) String() string  { return "arr[idx]" }
func (a *ArrayAccess) expressionNode() {}

type CodeBlock struct {
	Loc    Position
	Params []string
	Body   []Statement
	Expr   Expression
}

func (c *CodeBlock) Pos() Position   { return c.Loc }
func (c *CodeBlock) String() string  { return "{|| ... }" }
func (c *CodeBlock) expressionNode() {}

type NewExpr struct {
	Loc       Position
	ClassName string
	Args      []Expression
}

func (n *NewExpr) Pos() Position   { return n.Loc }
func (n *NewExpr) String() string  { return fmt.Sprintf("%s():New()", n.ClassName) }
func (n *NewExpr) expressionNode() {}

type MacroExp struct {
	Loc  Position
	Expr Expression
}

func (m *MacroExp) Pos() Position   { return m.Loc }
func (m *MacroExp) String() string  { return "&(...)" }
func (m *MacroExp) expressionNode() {}

type DefaultExpr struct {
	Loc   Position
	Name  string
	Value Expression
}

func (d *DefaultExpr) Pos() Position   { return d.Loc }
func (d *DefaultExpr) String() string  { return fmt.Sprintf("Default %s", d.Name) }
func (d *DefaultExpr) expressionNode() {}

type NamedParam struct {
	Loc   Position
	Name  string
	Value Expression
}

func (n *NamedParam) Pos() Position   { return n.Loc }
func (n *NamedParam) String() string  { return fmt.Sprintf("%s=%s", n.Name, n.Value) }
func (n *NamedParam) expressionNode() {}

type JsonLit struct {
	Loc   Position
	Pairs []JsonPair
}

type JsonPair struct {
	Loc   Position
	Key   string
	Value Expression
}

func (j *JsonLit) Pos() Position   { return j.Loc }
func (j *JsonLit) String() string  { return "{json}" }
func (j *JsonLit) expressionNode() {}

type TernaryExpr struct {
	Loc       Position
	Condition Expression
	ThenExpr  Expression
	ElseExpr  Expression
}

func (t *TernaryExpr) Pos() Position   { return t.Loc }
func (t *TernaryExpr) String() string  { return "ternary" }
func (t *TernaryExpr) expressionNode() {}
