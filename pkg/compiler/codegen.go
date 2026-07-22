package compiler

import (
	"fmt"
	"strings"

	"github.com/advpl/compiler/pkg/ast"
)

type Compiler struct {
	bc              *Bytecode
	funcStack       []*funcContext
	currentFunc     *funcContext
	localIdx        map[string]int
	globalIdx       map[string]int
	funcIndices     map[string]int
	nextFuncIdx     int
	namespace       string
	usingNamespaces []string
	loopStack       []*loopContext
}

// loopContext acumula os jumps de Exit (break) e Loop (continue) pendentes de
// patch dentro do loop atual. Sao resolvidos ao final de compileFor/compileWhile.
type loopContext struct {
	breakJumps    []int // patch -> fim do loop
	continueJumps []int // patch -> alvo de continuacao (incremento/condicao)
}

type funcContext struct {
	name      string
	params    []string
	locals    map[string]int
	nextLocal int
	offset    int
	// closures: contexto envolvente e variáveis livres capturadas (upvalues)
	parent     *funcContext
	upvals     map[string]int // nome da variável livre → índice de upvalue
	upvalDescs []UpvalDesc    // índice de upvalue → origem (LOCAL/UPVAL)
}

// resolveUpvalue: se `name` é capturável do escopo envolvente (Local do pai ou
// upvalue do pai, recursivamente), aloca/reusa um upvalue e retorna seu índice.
func (c *Compiler) resolveUpvalue(name string) (int, bool) {
	return c.resolveUpvalueIn(c.currentFunc, name)
}

func (c *Compiler) resolveUpvalueIn(fc *funcContext, name string) (int, bool) {
	if fc == nil || fc.parent == nil {
		return 0, false
	}
	if idx, ok := fc.upvals[name]; ok {
		return idx, true
	}
	if slot, ok := fc.parent.locals[name]; ok && slot&0x8000 == 0 {
		return c.addUpval(fc, name, UpvalDesc{Kind: UpvalLocal, Index: slot})
	}
	if pidx, ok := c.resolveUpvalueIn(fc.parent, name); ok {
		return c.addUpval(fc, name, UpvalDesc{Kind: UpvalParent, Index: pidx})
	}
	return 0, false
}

func (c *Compiler) addUpval(fc *funcContext, name string, d UpvalDesc) (int, bool) {
	idx := len(fc.upvalDescs)
	if fc.upvals == nil {
		fc.upvals = make(map[string]int)
	}
	fc.upvals[name] = idx
	fc.upvalDescs = append(fc.upvalDescs, d)
	return idx, true
}

func New() *Compiler {
	return &Compiler{
		bc: &Bytecode{
			Constants: make([]Constant, 0),
			Functions: make(map[string]*FunctionInfo),
			Classes:   make(map[string]*ClassInfo),
			Code:      make([]Instruction, 0),
		},
		localIdx:    make(map[string]int),
		globalIdx:   make(map[string]int),
		funcIndices: make(map[string]int),
	}
}

func Compile(program *ast.Program) (*Bytecode, error) {
	c := New()
	if err := c.compileProgram(program); err != nil {
		return nil, err
	}
	return c.bc, nil
}

func (c *Compiler) compileProgram(prog *ast.Program) error {
	c.namespace = prog.Namespace
	c.usingNamespaces = prog.UsingNamespaces

	// Register all functions (with namespace prefix if declared)
	for _, fn := range prog.Functions {
		fullName := fn.Name
		if prog.Namespace != "" {
			fullName = prog.Namespace + "." + fn.Name
		}
		info := &FunctionInfo{
			Name:       fullName,
			NumParams:  len(fn.Params),
			IsUser:     fn.IsUser,
			IsStatic:   fn.IsStatic,
			ParamNames: make([]string, len(fn.Params)),
		}
		for i, p := range fn.Params {
			info.ParamNames[i] = p.Name
		}
		for _, ann := range fn.Annotations {
			info.Annotations = append(info.Annotations, AnnotationInfo{Name: ann.Name, Value: ann.Value})
		}
		c.bc.Functions[fullName] = info
		// Also register without namespace for backward compatibility
		c.bc.Functions[fn.Name] = info
	}

	// Register classes
	for _, cls := range prog.Classes {
		fullName := cls.Name
		if prog.Namespace != "" {
			fullName = prog.Namespace + "." + cls.Name
		}
		info := &ClassInfo{
			Name:       fullName,
			Parent:     cls.Parent,
			Properties: make(map[string]string),
			Methods:    make(map[string]*FunctionInfo),
		}
		for _, prop := range cls.Properties {
			info.Properties[prop.Name] = prop.Type
		}
		for _, ann := range cls.Annotations {
			info.Annotations = append(info.Annotations, AnnotationInfo{Name: ann.Name, Value: ann.Value})
		}
		c.bc.Classes[fullName] = info
		c.bc.Classes[cls.Name] = info
	}

	// Compile main body
	c.bc.MainOffset = len(c.bc.Code)
	for _, stmt := range prog.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	// If no body statements, auto-call the first user function
	if len(prog.Body) == 0 && len(prog.Functions) > 0 {
		for _, fn := range prog.Functions {
			if fn.IsUser {
				c.emit(OP_CALL_FUNC, 0, 0, fn.Name, fn.Loc.Line)
				c.emit(OP_POP, 0, 0, "", fn.Loc.Line)
				break
			}
		}
	}

	c.emit(OP_HALT, 0, 0, "", 0)

	// Compile each function
	for _, fn := range prog.Functions {
		if err := c.compileFunction(fn); err != nil {
			return err
		}
	}

	// Compile each method
	for _, m := range prog.Methods {
		if err := c.compileMethod(m); err != nil {
			return err
		}
	}

	return nil
}

func (c *Compiler) compileFunction(fn *ast.FunctionDecl) error {
	info := c.bc.Functions[fn.Name]
	info.Offset = len(c.bc.Code)

	ctx := &funcContext{
		name:      fn.Name,
		locals:    make(map[string]int),
		nextLocal: len(fn.Params),
	}
	for i, p := range fn.Params {
		ctx.locals[p.Name] = i
	}
	c.funcStack = append(c.funcStack, ctx)
	c.currentFunc = ctx

	for _, stmt := range fn.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	if fn.ReturnExpr != nil {
		if err := c.compileExpr(fn.ReturnExpr); err != nil {
			return err
		}
		c.emit(OP_RETURN_VALUE, 0, 0, "", 0)
	} else {
		c.emit(OP_RETURN, 0, 0, "", 0)
	}

	c.funcStack = c.funcStack[:len(c.funcStack)-1]
	if len(c.funcStack) > 0 {
		c.currentFunc = c.funcStack[len(c.funcStack)-1]
	} else {
		c.currentFunc = nil
	}

	info.NumLocals = ctx.nextLocal
	info.LocalNames = ctx.locals
	return nil
}

func (c *Compiler) compileMethod(m *ast.MethodImpl) error {
	funcName := m.ClassName + "::" + m.Name
	info := &FunctionInfo{
		Name:       funcName,
		NumParams:  len(m.Params) + 1, // +1 for self
		Offset:     len(c.bc.Code),
		ParamNames: make([]string, len(m.Params)+1),
	}
	info.ParamNames[0] = "self"
	for i, p := range m.Params {
		info.ParamNames[i+1] = p.Name
	}

	cls, ok := c.bc.Classes[m.ClassName]
	if !ok {
		cls = &ClassInfo{Name: m.ClassName, Properties: make(map[string]string), Methods: make(map[string]*FunctionInfo)}
		c.bc.Classes[m.ClassName] = cls
	}
	cls.Methods[m.Name] = info
	c.bc.Functions[funcName] = info

	ctx := &funcContext{
		name:      funcName,
		locals:    make(map[string]int),
		nextLocal: len(m.Params) + 1,
	}
	ctx.locals["self"] = 0
	for i, p := range m.Params {
		ctx.locals[p.Name] = i + 1
	}
	c.funcStack = append(c.funcStack, ctx)
	c.currentFunc = ctx

	for _, stmt := range m.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	if m.ReturnExpr != nil {
		if err := c.compileExpr(m.ReturnExpr); err != nil {
			return err
		}
		c.emit(OP_RETURN_VALUE, 0, 0, "", 0)
	} else {
		c.emit(OP_RETURN, 0, 0, "", 0)
	}

	c.funcStack = c.funcStack[:len(c.funcStack)-1]
	if len(c.funcStack) > 0 {
		c.currentFunc = c.funcStack[len(c.funcStack)-1]
	} else {
		c.currentFunc = nil
	}

	info.NumLocals = ctx.nextLocal
	info.LocalNames = ctx.locals
	return nil
}

func (c *Compiler) emit(op Opcode, arg, arg2 int, str string, line int) int {
	idx := len(c.bc.Code)
	c.bc.Code = append(c.bc.Code, Instruction{Op: op, Arg: arg, Arg2: arg2, Str: str, Line: line})
	return idx
}

func (c *Compiler) emitJump(op Opcode, line int) int {
	return c.emit(op, 0, 0, "", line)
}

func (c *Compiler) patchJump(idx int, target int) {
	c.bc.Code[idx].Arg = target
}

func (c *Compiler) addNumberConst(val float64) int {
	for i, k := range c.bc.Constants {
		if k.Type == "number" && k.Num == val {
			return i
		}
	}
	idx := len(c.bc.Constants)
	c.bc.Constants = append(c.bc.Constants, Constant{Type: "number", Num: val})
	return idx
}

func (c *Compiler) addStringConst(s string) int {
	for i, k := range c.bc.Constants {
		if k.Type == "string" && k.Str == s {
			return i
		}
	}
	idx := len(c.bc.Constants)
	c.bc.Constants = append(c.bc.Constants, Constant{Type: "string", Str: s})
	return idx
}

func (c *Compiler) resolveLocal(name string) (int, bool) {
	if c.currentFunc != nil {
		if idx, ok := c.currentFunc.locals[name]; ok {
			return idx, true
		}
	}
	return -1, false
}

func (c *Compiler) addLocal(name string) int {
	if c.currentFunc == nil {
		// Global scope
		if idx, ok := c.globalIdx[name]; ok {
			return idx
		}
		idx := len(c.globalIdx)
		c.globalIdx[name] = idx
		if idx >= c.bc.NumGlobals {
			c.bc.NumGlobals = idx + 1
		}
		return idx | 0x8000 // mark as global
	}
	if idx, ok := c.currentFunc.locals[name]; ok {
		return idx
	}
	idx := c.currentFunc.nextLocal
	c.currentFunc.locals[name] = idx
	c.currentFunc.nextLocal++
	return idx
}

func (c *Compiler) compileStatement(stmt ast.Statement) error {
	if stmt == nil {
		return nil
	}
	switch s := stmt.(type) {
	case *ast.VarDecl:
		return c.compileVarDecl(s)
	case *ast.VarDeclGroup:
		for _, d := range s.Decls {
			if err := c.compileVarDecl(d); err != nil {
				return err
			}
		}
		return nil
	case *ast.DefaultGroup:
		for _, d := range s.Defaults {
			if err := c.compileExpr(d); err != nil {
				return err
			}
		}
		return nil
	case *ast.AssignStmt:
		return c.compileAssign(s)
	case *ast.IfStmt:
		return c.compileIf(s)
	case *ast.ForStmt:
		return c.compileFor(s)
	case *ast.WhileStmt:
		return c.compileWhile(s)
	case *ast.DoCaseStmt:
		return c.compileDoCase(s)
	case *ast.ReturnStmt:
		return c.compileReturn(s)
	case *ast.ExitStmt:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("Exit fora de um loop (linha %d)", s.Loc.Line)
		}
		top := c.loopStack[len(c.loopStack)-1]
		top.breakJumps = append(top.breakJumps, c.emitJump(OP_JUMP, s.Loc.Line))
		return nil
	case *ast.LoopStmt:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("Loop fora de um loop (linha %d)", s.Loc.Line)
		}
		top := c.loopStack[len(c.loopStack)-1]
		top.continueJumps = append(top.continueJumps, c.emitJump(OP_JUMP, s.Loc.Line))
		return nil
	case *ast.BreakStmt:
		if s.Value != nil {
			if err := c.compileExpr(s.Value); err != nil {
				return err
			}
		}
		c.emit(OP_THROW, 0, 0, "break", s.Loc.Line)
		return nil
	case *ast.TryCatchStmt:
		return c.compileTryCatch(s)
	case *ast.ThrowStmt:
		if err := c.compileExpr(s.Value); err != nil {
			return err
		}
		c.emit(OP_THROW, 0, 0, "", s.Loc.Line)
		return nil
	case *ast.BeginSequenceStmt:
		// Treat like try/catch with break
		return c.compileBeginSequence(s)
	case *ast.ExprStmt:
		return c.compileExpr(s.Expr)
	case *ast.InterfaceDecl:
		// Interface declarations are compile-time only, no code to emit
		return nil
	case *ast.ClassDecl:
		// Class declarations are pre-registered, no code to emit
		return nil
	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (c *Compiler) compileVarDecl(s *ast.VarDecl) error {
	scope := strings.ToLower(s.Scope)
	if scope == "private" || scope == "public" {
		c.emit(OP_DECL_DYN, 0, 0, s.Name, s.Loc.Line)
		if s.Value != nil {
			if err := c.compileExpr(s.Value); err != nil {
				return err
			}
			c.emit(OP_STORE_DYN, 0, 0, s.Name, s.Loc.Line)
		}
		return nil
	}
	if s.Value != nil {
		if err := c.compileExpr(s.Value); err != nil {
			return err
		}
		idx := c.addLocal(s.Name)
		if idx&0x8000 != 0 {
			c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, s.Name, s.Loc.Line)
		} else {
			c.emit(OP_STORE_LOCAL, idx, 0, s.Name, s.Loc.Line)
		}
	} else if s.Type != "" {
		// Initialize with default value based on type
		emitDefaultValue(c, s.Type, s.Name, s.Loc.Line)
	}
	return nil
}

func emitDefaultValue(c *Compiler, typeName, varName string, line int) {
	switch strings.ToUpper(typeName) {
	case "NUMERIC", "INTEGER", "DOUBLE", "DECIMAL", "FLOAT":
		c.emit(OP_NUMBER, c.addNumberConst(0), 0, "", line)
	case "CHARACTER", "CHAR", "STRING":
		c.emit(OP_STRING, c.addStringConst(""), 0, "", line)
	case "LOGICAL", "BOOLEAN":
		c.emit(OP_FALSE, 0, 0, "", line)
	case "DATE":
		c.emit(OP_DATE, c.addNumberConst(0), 0, "", line)
	case "ARRAY":
		c.emit(OP_NEW_ARRAY, 0, 0, "", line)
	default:
		c.emit(OP_NIL, 0, 0, "", line)
	}
	idx := c.addLocal(varName)
	if idx&0x8000 != 0 {
		c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, varName, line)
	} else {
		c.emit(OP_STORE_LOCAL, idx, 0, varName, line)
	}
}

func (c *Compiler) compileAssign(s *ast.AssignStmt) error {
	if err := c.compileExpr(s.Value); err != nil {
		return err
	}
	return c.compileStoreTarget(s.Target, s.Loc.Line)
}

// compileStoreTarget emits the store sequence for an assignment target,
// consuming the value already on top of the stack. Shared by compileAssign
// (statement `x := v`) and AssignExpr (expression `(x := v)`, e.g. inside a
// codeblock or a `While (x := next()) > 0` condition).
func (c *Compiler) compileStoreTarget(target ast.Expression, line int) error {
	switch target := target.(type) {
	case *ast.Ident:
		if idx, ok := c.resolveLocal(target.Name); ok {
			c.emit(OP_STORE_LOCAL, idx, 0, target.Name, line)
		} else if uidx, ok := c.resolveUpvalue(target.Name); ok {
			c.emit(OP_STORE_UPVAL, uidx, 0, target.Name, line)
		} else if c.currentFunc == nil {
			idx := c.addLocal(target.Name)
			if idx&0x8000 != 0 {
				c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, target.Name, line)
			} else {
				c.emit(OP_STORE_LOCAL, idx, 0, target.Name, line)
			}
		} else {
			c.emit(OP_STORE_DYN, 0, 0, target.Name, line)
		}
	case *ast.PropertyAccess:
		if err := c.compileExpr(target.Object); err != nil {
			return err
		}
		c.emit(OP_SET_PROP, 0, 0, target.Property, line)
	case *ast.ArrayAccess:
		if err := c.compileExpr(target.Array); err != nil {
			return err
		}
		if err := c.compileExpr(target.Index); err != nil {
			return err
		}
		c.emit(OP_ARRAY_SET, 0, 0, "", line)
	case *ast.SelfRef:
		c.emit(OP_LOAD_SELF, 0, 0, "", line)
		c.emit(OP_SET_PROP, 0, 0, target.Property, line)
	case *ast.FieldAccess:
		c.emit(OP_FIELD_PUT, 0, 0, target.Field, line)
	case *ast.MacroExp:
		// Macro/dynamic assignment (&cVar := x) has no addressable storage
		// in this VM yet — drop the value rather than fail compilation.
		c.emit(OP_POP, 0, 0, "", line)
	case *ast.CallExpr, *ast.MethodCall:
		// Clipper permite atribuição em resultado de chamada com semântica
		// de referência (`ATail(arr) := valor`, `oObj:Metodo() := x`). O VM
		// não modela lvalues por referência — descarta o valor em vez de
		// falhar a compilação (mesma tolerância do MacroExp acima).
		c.emit(OP_POP, 0, 0, "", line)
	default:
		return fmt.Errorf("unsupported assignment target: %T at line %d", target, line)
	}
	return nil
}

func (c *Compiler) compileIf(s *ast.IfStmt) error {
	if err := c.compileExpr(s.Condition); err != nil {
		return err
	}
	jumpToEnd := c.emitJump(OP_JUMP_IF_FALSE, s.Loc.Line)

	for _, stmt := range s.ThenBody {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	if len(s.ElseIfs) > 0 || len(s.ElseBody) > 0 {
		jumpOverElse := c.emitJump(OP_JUMP, s.Loc.Line)
		c.patchJump(jumpToEnd, len(c.bc.Code))

		for _, elseif := range s.ElseIfs {
			if err := c.compileExpr(elseif.Condition); err != nil {
				return err
			}
			jumpPastElseIf := c.emitJump(OP_JUMP_IF_FALSE, elseif.Loc.Line)
			for _, stmt := range elseif.Body {
				if err := c.compileStatement(stmt); err != nil {
					return err
				}
			}
			c.patchJump(jumpOverElse, len(c.bc.Code))
			jumpOverElse = c.emitJump(OP_JUMP, elseif.Loc.Line)
			c.patchJump(jumpPastElseIf, len(c.bc.Code))
		}

		if len(s.ElseBody) > 0 {
			for _, stmt := range s.ElseBody {
				if err := c.compileStatement(stmt); err != nil {
					return err
				}
			}
		}
		c.patchJump(jumpOverElse, len(c.bc.Code))
	} else {
		c.patchJump(jumpToEnd, len(c.bc.Code))
	}

	return nil
}

func (c *Compiler) compileFor(s *ast.ForStmt) error {
	// Init: var = start
	if err := c.compileExpr(s.Start); err != nil {
		return err
	}
	c.emitStore(s.VarName, s.Loc.Line)

	// Avalia o step UMA vez numa local escondida (evita reavaliar expressoes com
	// efeito colateral a cada iteracao e permite comparar pelo sinal do step).
	stepVar := s.VarName + " step" // espaco: nome impossivel de colidir com identificador do usuario
	if s.Step != nil {
		if err := c.compileExpr(s.Step); err != nil {
			return err
		}
	} else {
		c.emit(OP_NUMBER, c.addNumberConst(1), 0, "", s.Loc.Line)
	}
	c.emitStore(stepVar, s.Loc.Line)

	// Condition: step>=0 ? var<=end : var>=end (via OP_FORLOOP_CMP)
	loopStart := len(c.bc.Code)
	c.emitLoad(s.VarName, s.Loc.Line)
	if err := c.compileExpr(s.End); err != nil {
		return err
	}
	c.emitLoad(stepVar, s.Loc.Line)
	c.emit(OP_FORLOOP_CMP, 0, 0, "", s.Loc.Line)
	exitJump := c.emitJump(OP_JUMP_IF_FALSE, s.Loc.Line)

	// Body
	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)
	for _, stmt := range s.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}
	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	// Loop (continue) cai aqui: no incremento.
	continueTarget := len(c.bc.Code)
	for _, j := range ctx.continueJumps {
		c.patchJump(j, continueTarget)
	}

	// Increment: var = var + step
	c.emitLoad(s.VarName, s.Loc.Line)
	c.emitLoad(stepVar, s.Loc.Line)
	c.emit(OP_ADD, 0, 0, "", s.Loc.Line)
	c.emitStore(s.VarName, s.Loc.Line)

	c.emit(OP_JUMP, loopStart, 0, "", s.Loc.Line)

	// Fim do loop: alvo do exitJump e dos Exit (break).
	loopEnd := len(c.bc.Code)
	c.patchJump(exitJump, loopEnd)
	for _, j := range ctx.breakJumps {
		c.patchJump(j, loopEnd)
	}

	return nil
}

// emitLoad/emitStore resolvem local vs global e emitem o opcode certo.
func (c *Compiler) emitLoad(name string, line int) {
	if idx, ok := c.resolveLocal(name); ok {
		c.emit(OP_LOAD_LOCAL, idx, 0, name, line)
	} else {
		idx := c.addLocal(name)
		c.emit(OP_LOAD_GLOBAL, idx&0x7FFF, 0, name, line)
	}
}

func (c *Compiler) emitStore(name string, line int) {
	idx := c.addLocal(name)
	if idx&0x8000 != 0 {
		c.emit(OP_STORE_GLOBAL, idx&0x7FFF, 0, name, line)
	} else {
		c.emit(OP_STORE_LOCAL, idx, 0, name, line)
	}
}

func (c *Compiler) compileWhile(s *ast.WhileStmt) error {
	loopStart := len(c.bc.Code)
	if err := c.compileExpr(s.Condition); err != nil {
		return err
	}
	exitJump := c.emitJump(OP_JUMP_IF_FALSE, s.Loc.Line)

	ctx := &loopContext{}
	c.loopStack = append(c.loopStack, ctx)
	for _, stmt := range s.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}
	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	// Loop (continue) reavalia a condicao.
	for _, j := range ctx.continueJumps {
		c.patchJump(j, loopStart)
	}

	c.emit(OP_JUMP, loopStart, 0, "", s.Loc.Line)

	loopEnd := len(c.bc.Code)
	c.patchJump(exitJump, loopEnd)
	for _, j := range ctx.breakJumps {
		c.patchJump(j, loopEnd)
	}
	return nil
}

func (c *Compiler) compileDoCase(s *ast.DoCaseStmt) error {
	var endJumps []int

	for _, clause := range s.Cases {
		if err := c.compileExpr(clause.Condition); err != nil {
			return err
		}
		nextCaseJump := c.emitJump(OP_JUMP_IF_FALSE, clause.Loc.Line)

		for _, stmt := range clause.Body {
			if err := c.compileStatement(stmt); err != nil {
				return err
			}
		}

		endJumps = append(endJumps, c.emitJump(OP_JUMP, clause.Loc.Line))
		c.patchJump(nextCaseJump, len(c.bc.Code))
	}

	if s.Otherwise != nil {
		for _, stmt := range s.Otherwise {
			if err := c.compileStatement(stmt); err != nil {
				return err
			}
		}
	}

	for _, j := range endJumps {
		c.patchJump(j, len(c.bc.Code))
	}

	return nil
}

func (c *Compiler) compileReturn(s *ast.ReturnStmt) error {
	if s.Value != nil {
		if err := c.compileExpr(s.Value); err != nil {
			return err
		}
		c.emit(OP_RETURN_VALUE, 0, 0, "", s.Loc.Line)
	} else {
		c.emit(OP_RETURN, 0, 0, "", s.Loc.Line)
	}
	return nil
}

func (c *Compiler) compileTryCatch(s *ast.TryCatchStmt) error {
	// Pre-allocate catch variable so we can store its index in TRY_BEGIN
	catchVarIdx := -1
	if s.CatchVar != "" {
		catchVarIdx = c.addLocal(s.CatchVar)
	}

	tryBeginIdx := c.emit(OP_TRY_BEGIN, 0, catchVarIdx, s.CatchVar, s.Loc.Line)

	for _, stmt := range s.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	// Jump over catch block after successful try body
	jumpOverCatch := c.emitJump(OP_JUMP, s.Loc.Line)
	catchStart := len(c.bc.Code)

	// Patch TRY_BEGIN with catch start IP
	c.patchJump(tryBeginIdx, catchStart)

	// TRY_END removes the try/catch handler
	c.emit(OP_TRY_END, 0, 0, "", s.Loc.Line)

	// OP_CATCH stores the error value into the catch variable local
	if s.CatchVar != "" {
		c.emit(OP_CATCH, catchVarIdx, 0, s.CatchVar, s.Loc.Line)
	}
	for _, stmt := range s.CatchBody {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	if len(s.FinallyBody) > 0 {
		for _, stmt := range s.FinallyBody {
			if err := c.compileStatement(stmt); err != nil {
				return err
			}
		}
	}

	// Patch jump over catch to land after catch/finally
	c.patchJump(jumpOverCatch, len(c.bc.Code))
	return nil
}

func (c *Compiler) compileBeginSequence(s *ast.BeginSequenceStmt) error {
	tryBeginIdx := c.emit(OP_TRY_BEGIN, 0, 0, "", s.Loc.Line)

	for _, stmt := range s.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	jumpOverRecover := c.emitJump(OP_JUMP, s.Loc.Line)
	recoverStart := len(c.bc.Code)
	c.patchJump(tryBeginIdx, recoverStart)
	c.emit(OP_TRY_END, 0, 0, "", s.Loc.Line)

	if s.UsingVar != "" {
		c.emit(OP_CATCH, c.addLocal(s.UsingVar), 0, s.UsingVar, s.Loc.Line)
	}
	for _, stmt := range s.RecoverBody {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	c.patchJump(jumpOverRecover, len(c.bc.Code))
	return nil
}

func (c *Compiler) compileExpr(expr ast.Expression) error {
	switch e := expr.(type) {
	case *ast.NumberLit:
		c.emit(OP_NUMBER, c.addNumberConst(e.Value), 0, "", e.Loc.Line)
	case *ast.StringLit:
		c.emit(OP_STRING, c.addStringConst(e.Value), 0, "", e.Loc.Line)
	case *ast.BoolLit:
		if e.Value {
			c.emit(OP_TRUE, 0, 0, "", e.Loc.Line)
		} else {
			c.emit(OP_FALSE, 0, 0, "", e.Loc.Line)
		}
	case *ast.NilLit:
		c.emit(OP_NIL, 0, 0, "", e.Loc.Line)
	case *ast.AssignExpr:
		// (x := expr) as a value: store, but leave a copy on the stack.
		if err := c.compileExpr(e.Value); err != nil {
			return err
		}
		c.emit(OP_DUP, 0, 0, "", e.Loc.Line)
		if err := c.compileStoreTarget(e.Target, e.Loc.Line); err != nil {
			return err
		}
	case *ast.SeqExpr:
		// (a, b, c): run all in order, value is the last one.
		for i, sub := range e.Exprs {
			if err := c.compileExpr(sub); err != nil {
				return err
			}
			if i < len(e.Exprs)-1 {
				c.emit(OP_POP, 0, 0, "", e.Loc.Line)
			}
		}
	case *ast.Ident:
		if idx, ok := c.resolveLocal(e.Name); ok {
			c.emit(OP_LOAD_LOCAL, idx, 0, e.Name, e.Loc.Line)
		} else if uidx, ok := c.resolveUpvalue(e.Name); ok {
			c.emit(OP_LOAD_UPVAL, uidx, 0, e.Name, e.Loc.Line)
		} else if c.currentFunc == nil {
			idx := c.addLocal(e.Name)
			if idx&0x8000 != 0 {
				c.emit(OP_LOAD_GLOBAL, idx&0x7FFF, 0, e.Name, e.Loc.Line)
			} else {
				c.emit(OP_LOAD_LOCAL, idx, 0, e.Name, e.Loc.Line)
			}
		} else {
			c.emit(OP_LOAD_DYN, 0, 0, e.Name, e.Loc.Line)
		}
	case *ast.BinaryOp:
		return c.compileBinaryOp(e)
	case *ast.UnaryOp:
		return c.compileUnaryOp(e)
	case *ast.CallExpr:
		return c.compileCallExpr(e)
	case *ast.MethodCall:
		return c.compileMethodCall(e)
	case *ast.PropertyAccess:
		if err := c.compileExpr(e.Object); err != nil {
			return err
		}
		c.emit(OP_GET_PROP, 0, 0, e.Property, e.Loc.Line)
	case *ast.SelfRef:
		c.emit(OP_LOAD_SELF, 0, 0, "", e.Loc.Line)
		if e.Property != "" {
			c.emit(OP_GET_PROP, 0, 0, e.Property, e.Loc.Line)
		}
	case *ast.SelfMethodCall:
		c.emit(OP_LOAD_SELF, 0, 0, "", e.Loc.Line)
		for _, arg := range e.Args {
			if err := c.compileExpr(arg); err != nil {
				return err
			}
		}
		c.emit(OP_CALL_METHOD, 0, len(e.Args), e.Method, e.Loc.Line)
	case *ast.FieldAccess:
		c.emit(OP_FIELD_GET, 0, 0, e.Field, e.Loc.Line)
	case *ast.ArrayLit:
		for _, elem := range e.Elements {
			if err := c.compileExpr(elem); err != nil {
				return err
			}
		}
		c.emit(OP_NEW_ARRAY, len(e.Elements), 0, "", e.Loc.Line)
	case *ast.ArrayAccess:
		if err := c.compileExpr(e.Array); err != nil {
			return err
		}
		if err := c.compileExpr(e.Index); err != nil {
			return err
		}
		c.emit(OP_ARRAY_GET, 0, 0, "", e.Loc.Line)
	case *ast.CodeBlock:
		return c.compileCodeBlock(e)
	case *ast.NewExpr:
		for _, arg := range e.Args {
			if err := c.compileExpr(arg); err != nil {
				return err
			}
		}
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), e.ClassName, e.Loc.Line)
	case *ast.MacroExp:
		if err := c.compileExpr(e.Expr); err != nil {
			return err
		}
		c.emit(OP_MACRO, 0, 0, "", e.Loc.Line)
	case *ast.DefaultExpr:
		if err := c.compileExpr(e.Value); err != nil {
			return err
		}
		if idx, ok := c.resolveLocal(e.Name); ok {
			c.emit(OP_STORE_LOCAL, idx, 0, e.Name, e.Loc.Line)
		}
	case *ast.JsonLit:
		for _, pair := range e.Pairs {
			c.emit(OP_STRING, c.addStringConst(pair.Key), 0, "", pair.Loc.Line)
			if err := c.compileExpr(pair.Value); err != nil {
				return err
			}
		}
		c.emit(OP_NEW_OBJECT, len(e.Pairs), 0, "json", e.Loc.Line)
	default:
		return fmt.Errorf("unsupported expression type: %T", expr)
	}
	return nil
}

func (c *Compiler) compileBinaryOp(e *ast.BinaryOp) error {
	if err := c.compileExpr(e.Left); err != nil {
		return err
	}
	if err := c.compileExpr(e.Right); err != nil {
		return err
	}
	switch e.Op {
	case "+":
		c.emit(OP_ADD, 0, 0, "", e.Loc.Line)
	case "-":
		c.emit(OP_SUB, 0, 0, "", e.Loc.Line)
	case "*":
		c.emit(OP_MUL, 0, 0, "", e.Loc.Line)
	case "/":
		c.emit(OP_DIV, 0, 0, "", e.Loc.Line)
	case "%":
		c.emit(OP_MOD, 0, 0, "", e.Loc.Line)
	case "^", "**":
		c.emit(OP_POW, 0, 0, "", e.Loc.Line)
	case "==":
		c.emit(OP_EQ, 0, 0, "", e.Loc.Line)
	case "!=", "<>":
		c.emit(OP_NEQ, 0, 0, "", e.Loc.Line)
	case "<":
		c.emit(OP_LT, 0, 0, "", e.Loc.Line)
	case ">":
		c.emit(OP_GT, 0, 0, "", e.Loc.Line)
	case "<=":
		c.emit(OP_LTE, 0, 0, "", e.Loc.Line)
	case ">=":
		c.emit(OP_GTE, 0, 0, "", e.Loc.Line)
	case ".And.":
		c.emit(OP_AND, 0, 0, "", e.Loc.Line)
	case ".Or.":
		c.emit(OP_OR, 0, 0, "", e.Loc.Line)
	case "$":
		c.emit(OP_DOLLAR, 0, 0, "", e.Loc.Line)
	default:
		return fmt.Errorf("unknown operator: %s", e.Op)
	}
	return nil
}

func (c *Compiler) compileUnaryOp(e *ast.UnaryOp) error {
	if err := c.compileExpr(e.Operand); err != nil {
		return err
	}
	switch e.Op {
	case "-":
		c.emit(OP_NEG, 0, 0, "", e.Loc.Line)
	case ".Not.", "!":
		c.emit(OP_NOT, 0, 0, "", e.Loc.Line)
	default:
		return fmt.Errorf("unknown unary operator: %s", e.Op)
	}
	return nil
}

func (c *Compiler) compileCallExpr(e *ast.CallExpr) error {
	// If()/IIF() com 3 argumentos: forma especial de curto-circuito — avalia só
	// o ramo escolhido (o native avaliaria os dois, pois args são calculados antes).
	if up := strings.ToUpper(e.Name); (up == "IF" || up == "IIF") && len(e.Args) == 3 {
		if err := c.compileExpr(e.Args[0]); err != nil {
			return err
		}
		jFalse := c.emitJump(OP_JUMP_IF_FALSE, e.Loc.Line)
		if err := c.compileExpr(e.Args[1]); err != nil {
			return err
		}
		jEnd := c.emitJump(OP_JUMP, e.Loc.Line)
		c.patchJump(jFalse, len(c.bc.Code))
		if err := c.compileExpr(e.Args[2]); err != nil {
			return err
		}
		c.patchJump(jEnd, len(c.bc.Code))
		return nil
	}

	// Check if this is a class constructor call (e.g., Calculator():New())
	if _, isClass := c.bc.Classes[e.Name]; isClass {
		c.compileArgs(e.Args)
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), e.Name, e.Loc.Line)
		return nil
	}
	// Check known built-in classes
	if isBuiltinClass(e.Name) {
		c.compileArgs(e.Args)
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), e.Name, e.Loc.Line)
		return nil
	}
	c.compileArgs(e.Args)
	if strings.ToUpper(e.Name) == "EVAL" {
		c.emit(OP_EVAL_CODEBLOCK, 0, len(e.Args)-1, "", e.Loc.Line)
		return nil
	}
	if _, ok := c.bc.Functions[e.Name]; ok {
		c.emit(OP_CALL_FUNC, 0, len(e.Args), e.Name, e.Loc.Line)
	} else {
		c.emit(OP_CALL_NATIVE, 0, len(e.Args), e.Name, e.Loc.Line)
	}
	return nil
}

func (c *Compiler) compileArgs(args []ast.Expression) {
	for _, arg := range args {
		if np, ok := arg.(*ast.NamedParam); ok {
			c.emit(OP_NAMED_ARG, 0, 0, np.Name, np.Loc.Line)
			c.compileExpr(np.Value)
		} else {
			c.emit(OP_NAMED_ARG, 0, 0, "", arg.Pos().Line)
			c.compileExpr(arg)
		}
	}
}

func (c *Compiler) compileMethodCall(e *ast.MethodCall) error {
	if err := c.compileExpr(e.Object); err != nil {
		return err
	}
	c.compileArgs(e.Args)
	c.emit(OP_CALL_METHOD, 0, len(e.Args), e.Method, e.Loc.Line)
	return nil
}

var builtinClasses = map[string]bool{
	"ERRORCLASS":    true,
	"JSONOBJECT":    true,
	"JSONARRAY":     true,
	"FWFORMVIEW":    true,
	"FWFORMMODEL":   true,
	"FWFORMBROWSE":  true,
	"FWGRIDPROCESS": true,
	"FWMBROWSE":     true,
	"LLM":           true,
	"MCPSERVER":     true,
}

func isBuiltinClass(name string) bool {
	return builtinClasses[strings.ToUpper(name)]
}

func (c *Compiler) compileCodeBlock(e *ast.CodeBlock) error {
	// Nome único monotônico: não usar len(c.bc.Functions) aqui, pois um
	// codeblock aninhado é compilado (e registrado) ANTES de seu pai (a
	// compilação do corpo do pai roda antes do próprio pai se registrar em
	// c.bc.Functions), o que colidiria os nomes de pai e filho.
	funcName := fmt.Sprintf("__codeblock_%d", c.nextFuncIdx)
	c.nextFuncIdx++

	savedFunc := c.currentFunc

	// Jump over the codeblock body (it will be called via Eval)
	jumpOver := c.emit(OP_JUMP, 0, 0, "", e.Loc.Line)

	c.currentFunc = &funcContext{
		name:      funcName,
		locals:    make(map[string]int),
		nextLocal: 1,
		parent:    savedFunc, // escopo envolvente, para capturar upvalues (closures)
		upvals:    make(map[string]int),
	}

	for _, param := range e.Params {
		c.currentFunc.locals[param] = c.currentFunc.nextLocal
		c.currentFunc.nextLocal++
	}

	info := &FunctionInfo{
		Name:       funcName,
		NumParams:  len(e.Params) + 1,
		NumLocals:  c.currentFunc.nextLocal,
		IsUser:     false,
		Offset:     len(c.bc.Code),
		ParamNames: make([]string, len(e.Params)+1),
	}
	info.ParamNames[0] = "self"
	for i, p := range e.Params {
		info.ParamNames[i+1] = p
	}

	for _, stmt := range e.Body {
		if err := c.compileStatement(stmt); err != nil {
			c.currentFunc = savedFunc
			return err
		}
	}

	if e.Expr != nil {
		if err := c.compileExpr(e.Expr); err != nil {
			c.currentFunc = savedFunc
			return err
		}
		c.emit(OP_RETURN_VALUE, 0, 0, "", e.Loc.Line)
	} else {
		c.emit(OP_RETURN, 0, 0, "", e.Loc.Line)
	}

	info.NumLocals = c.currentFunc.nextLocal
	info.Upvals = c.currentFunc.upvalDescs // origem dos upvalues (closures aninhadas)
	c.bc.Functions[funcName] = info

	// Patch the jump to skip over the codeblock body
	c.patchJump(jumpOver, len(c.bc.Code))

	c.currentFunc = savedFunc

	c.emit(OP_NEW_CODEBLOCK, 0, len(e.Params), funcName, e.Loc.Line)
	return nil
}
