package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/advpl/compiler/pkg/ast"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
)

// Compiler generates optimized bytecode from an abstract syntax tree.
// It tracks function contexts, loop boundaries, and symbol scopes.
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
	// fileStatics: Static declarados no nível do arquivo -> índice global.
	// Dentro das funções esses nomes leem/gravam a global (antes caíam na
	// busca dinâmica de Private/Public e não enxergavam o valor).
	fileStatics map[string]int
	// proj: estado do CompileProject (nil no Compile de fonte único). Vários
	// programas num só bytecode, como um RPO: User Function e classes são
	// globais; Static Function fica restrita ao arquivo.
	proj *projectState
	// methodDeclAnns: anotações dos MethodDecl (dentro do class ... endclass)
	// indexadas por "CLASSE::METODO" em maiúsculas — o MethodImpl não
	// carrega anotações, então o compileMethod mescla daqui no FunctionInfo
	// (controladores REST TLPP: @GET em método de classe).
	methodDeclAnns map[string][]AnnotationInfo
}

// projectProgram é um fonte dentro do CompileProject: suas Static Function
// resolvem para chaves "STATIC:<ARQUIVO>:<NOME>" e seus Static de arquivo
// têm slot global próprio. O corpo principal é ignorado, exceto os
// inicializadores dos Static de arquivo — emitidos por
// compileProjectStaticInit no início do "main" do projeto: MainOffset
// aponta para o bloco de inicialização, que termina em OP_HALT (sem
// Static de arquivo, MainOffset aponta direto para o OP_HALT).
type projectProgram struct {
	prog        *ast.Program
	fileTag     string            // basename do arquivo em maiúsculas (com extensão)
	statics     map[string]string // nome da Static Function (maiúsculas) -> chave qualificada
	fileStatics map[string]int    // Static de arquivo -> slot global
}

// projectState acumula o que o CompileProject precisa entre o passe de
// registro (todos os programas) e o passe de compilação (um por vez).
type projectState struct {
	programs   []*projectProgram
	funcFiles  map[string]string // chave de função em maiúsculas -> arquivo de origem
	classFiles map[string]string // chave de classe em maiúsculas -> arquivo de origem
	methodFiles map[string]string // "CLASSE::METODO" em maiúsculas -> arquivo
	current    *projectProgram   // programa cujas funções estão sendo compiladas
}

// progFileName: nome do arquivo-fonte do programa, com fallback para a
// posição (Program montado à mão pode não ter FileName preenchido).
func progFileName(prog *ast.Program) string {
	if prog.FileName != "" {
		return prog.FileName
	}
	return prog.Loc.FileName
}

// loopContext acumula os jumps de Exit (break) e Loop (continue) pendentes de
// patch dentro do loop atual. Sao resolvidos ao final de compileFor/compileWhile.
type loopContext struct {
	breakJumps    []int // patch -> fim do loop
	continueJumps []int // patch -> alvo de continuacao (incremento/condicao)
}

type funcContext struct {
	name      string
	className string // classe do método em compilação ("" em funções); base do _Super:
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

// New performs a core operation.
func New() *Compiler {
	return &Compiler{
		bc: &Bytecode{
			Constants: make([]Constant, 0),
			Functions: make(map[string]*FunctionInfo),
			Classes:   make(map[string]*ClassInfo),
			Code:      make([]Instruction, 0),
		},
		localIdx:        make(map[string]int),
		globalIdx:       make(map[string]int),
		funcIndices:     make(map[string]int),
		methodDeclAnns:  make(map[string][]AnnotationInfo),
	}
}

// Compile performs a core operation.
func Compile(program *ast.Program) (*Bytecode, error) {
	c := New()
	if err := c.compileProgram(program); err != nil {
		return nil, err
	}
	return c.bc, nil
}

// CompileProject compila vários programas num único bytecode, como um RPO:
// mesmo pool de constantes e mesmo Code, sem relocação. User Function e
// classes são globais — nome duplicado entre arquivos é erro citando os
// dois arquivos. Static Function fica restrita ao arquivo que a declara,
// registrada como "STATIC:<ARQUIVO>:<NOME>" (arquivo = basename em
// maiúsculas, com extensão, sem diretório); chamadas dentro do mesmo
// arquivo saem já qualificadas. O corpo principal de cada programa é
// ignorado, exceto os inicializadores dos Static de arquivo, que rodam
// uma vez no "main" do projeto (MainOffset) antes do OP_HALT.
func CompileProject(progs []*ast.Program) (*Bytecode, error) {
	if len(progs) == 0 {
		return nil, fmt.Errorf("CompileProject: nenhum programa")
	}
	for i, prog := range progs {
		if prog == nil {
			return nil, fmt.Errorf("CompileProject: programa %d é nil", i)
		}
	}
	c := New()
	ps := &projectState{
		funcFiles:   make(map[string]string),
		classFiles:  make(map[string]string),
		methodFiles: make(map[string]string),
	}
	c.proj = ps

	// MainOffset -> início do "main" do projeto: inicializadores dos Static
	// de arquivo (na ordem dos programas) e um OP_HALT no fim.
	c.bc.MainOffset = len(c.bc.Code)

	// Passe 1: registra funções, classes e métodos de TODOS os programas
	// antes de compilar qualquer um — chamadas e heranças (_Super:) enxerem
	// símbolos de arquivos em qualquer posição da lista.
	for _, prog := range progs {
		pp, err := c.registerProjectProgram(prog, ps)
		if err != nil {
			return nil, err
		}
		ps.programs = append(ps.programs, pp)
	}

	// Passe 1.5: inicializadores dos Static de arquivo, por programa.
	for _, pp := range ps.programs {
		if err := c.compileProjectStaticInit(pp); err != nil {
			return nil, err
		}
	}
	c.emit(OP_HALT, 0, 0, "", 0)

	// Passe 2: compila funções e métodos de cada programa.
	for _, pp := range ps.programs {
		if err := c.compileProjectProgram(pp); err != nil {
			return nil, err
		}
	}
	c.bc.ProjectStatics = true
	return c.bc, nil
}

// staticFuncKey monta a chave qualificada de uma Static Function:
// "STATIC:<ARQUIVO>:<NOME>".
func staticFuncKey(fileTag, name string) string {
	return "STATIC:" + fileTag + ":" + name
}

// registerProjectProgram registra no pool global as funções e classes de
// um programa do projeto e devolve o contexto por-arquivo (estáticas e
// Static de arquivo). Não emite código.
func (c *Compiler) registerProjectProgram(prog *ast.Program, ps *projectState) (*projectProgram, error) {
	file := progFileName(prog)
	pp := &projectProgram{
		prog:        prog,
		fileTag:     strings.ToUpper(filepath.Base(file)),
		statics:     make(map[string]string),
		fileStatics: make(map[string]int),
	}

	// Static Function: só com o nome qualificado — o nome puro não entra
	// no pool, senão dois arquivos com a mesma Static colidiriam.
	for _, fn := range prog.Functions {
		if !fn.IsStatic {
			continue
		}
		key := staticFuncKey(pp.fileTag, strings.ToUpper(fn.Name))
		if prev, dup := ps.funcFiles[key]; dup {
			return nil, fmt.Errorf("função %s duplicada em %s e %s", fn.Name, prev, file)
		}
		info := &FunctionInfo{
			Name:       key,
			NumParams:  len(fn.Params),
			IsUser:     fn.IsUser,
			IsStatic:   true,
			ParamNames: make([]string, len(fn.Params)),
		}
		for i, p := range fn.Params {
			info.ParamNames[i] = p.Name
		}
		for _, ann := range fn.Annotations {
			info.Annotations = append(info.Annotations, AnnotationInfo{Name: ann.Name, Value: ann.Value})
		}
		c.bc.Functions[key] = info
		ps.funcFiles[key] = file
		pp.statics[strings.ToUpper(fn.Name)] = key
	}

	// Demais funções (User Function, Function, Procedure): globais. As
	// chaves de detecção vão em maiúsculas — "Foo" e "FOO" em arquivos
	// diferentes são a mesma função global (AdvPL não diferencia caixa).
	for _, fn := range prog.Functions {
		if fn.IsStatic {
			continue
		}
		fullName := fn.Name
		if prog.Namespace != "" {
			fullName = prog.Namespace + "." + fn.Name
		}
		keys := []string{fullName}
		if fn.Name != fullName {
			keys = append(keys, fn.Name)
		}
		for _, k := range keys {
			if prev, dup := ps.funcFiles[strings.ToUpper(k)]; dup {
				return nil, fmt.Errorf("função %s duplicada: declarada em %s e %s", fn.Name, prev, file)
			}
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
		for _, k := range keys {
			c.bc.Functions[k] = info
			ps.funcFiles[strings.ToUpper(k)] = file
		}
	}

	// Classes globais (mesma regra case-insensitive das funções).
	for _, cls := range prog.Classes {
		fullName := cls.Name
		if prog.Namespace != "" {
			fullName = prog.Namespace + "." + cls.Name
		}
		keys := []string{fullName}
		if cls.Name != fullName {
			keys = append(keys, cls.Name)
		}
		for _, k := range keys {
			if prev, dup := ps.classFiles[strings.ToUpper(k)]; dup {
				return nil, fmt.Errorf("classe %s duplicada: declarada em %s e %s", cls.Name, prev, file)
			}
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
		for _, k := range keys {
			c.bc.Classes[k] = info
			ps.classFiles[strings.ToUpper(k)] = file
		}
		c.collectMethodDeclAnns(cls, prog.Namespace)
	}

	// Implementações de método (Classe::Metodo) também são globais:
	// duplicata entre arquivos sobrescreveria silenciosamente em
	// compileMethod — detecta aqui, antes de emitir código.
	for _, m := range prog.Methods {
		key := strings.ToUpper(m.ClassName + "::" + m.Name)
		if prev, dup := ps.methodFiles[key]; dup {
			return nil, fmt.Errorf("método %s::%s duplicado: declarado em %s e %s",
				m.ClassName, m.Name, prev, file)
		}
		ps.methodFiles[key] = file
	}

	// Static de arquivo (variáveis): reserva slot global com chave
	// qualificada para não dividir estado entre arquivos. O inicializador
	// roda no "main" do projeto (compileProjectStaticInit).
	for _, st := range prog.Body {
		var decls []*ast.VarDecl
		switch d := st.(type) {
		case *ast.VarDecl:
			decls = []*ast.VarDecl{d}
		case *ast.VarDeclGroup:
			decls = d.Decls
		default:
			continue
		}
		for _, d := range decls {
			if !strings.EqualFold(d.Scope, "static") {
				continue
			}
			key := staticFuncKey(pp.fileTag, strings.ToUpper(d.Name))
			pp.fileStatics[strings.ToUpper(d.Name)] = c.addLocal(key) & 0x7FFF
		}
	}

	return pp, nil
}

// compileProjectStaticInit emite, no "main" do projeto, os inicializadores
// dos Static de arquivo de um programa — equivalente ao que o corpo
// principal faria num Compile de fonte único, mas restrito às declarações
// Static (o resto do corpo segue ignorado). Roda uma vez, na ordem dos
// programas, antes do OP_HALT.
func (c *Compiler) compileProjectStaticInit(pp *projectProgram) error {
	prog := pp.prog
	file := progFileName(prog)
	c.namespace = prog.Namespace
	c.usingNamespaces = prog.UsingNamespaces
	c.proj.current = pp
	c.fileStatics = pp.fileStatics
	defer func() {
		c.proj.current = nil
		c.fileStatics = nil
	}()

	for _, st := range prog.Body {
		var decls []*ast.VarDecl
		switch d := st.(type) {
		case *ast.VarDecl:
			decls = []*ast.VarDecl{d}
		case *ast.VarDeclGroup:
			decls = d.Decls
		default:
			continue
		}
		for _, d := range decls {
			if !strings.EqualFold(d.Scope, "static") {
				continue
			}
			idx, ok := pp.fileStatics[strings.ToUpper(d.Name)]
			if !ok {
				continue // registrado no passe 1; não deveria acontecer
			}
			if d.Value != nil {
				if err := c.compileExpr(d.Value); err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
			} else if d.Type != "" {
				emitDefaultExpr(c, d.Type, d.Loc.Line)
			} else {
				continue // Static sem valor nem tipo: global fica Nil
			}
			c.emit(OP_STORE_GLOBAL, idx, 0, d.Name, d.Loc.Line)
		}
	}
	return nil
}

// compileProjectProgram compila as funções e métodos de um programa do
// projeto, com o contexto por-arquivo ativo (estáticas e Static de
// arquivo). O corpo principal não é compilado. Erros de compilação são
// embrulhados com o nome do arquivo para localizar a origem.
func (c *Compiler) compileProjectProgram(pp *projectProgram) error {
	prog := pp.prog
	file := progFileName(prog)
	c.namespace = prog.Namespace
	c.usingNamespaces = prog.UsingNamespaces
	c.proj.current = pp
	c.fileStatics = pp.fileStatics
	defer func() {
		c.proj.current = nil
		c.fileStatics = nil
	}()

	for _, fn := range prog.Functions {
		if err := c.compileFunction(fn); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	for _, m := range prog.Methods {
		if err := c.compileMethod(m); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

// functionKey devolve a chave de registro da função no pool global. No
// CompileProject, Static Function usa o nome qualificado do arquivo — o
// nome puro não existe em bc.Functions.
func (c *Compiler) functionKey(fn *ast.FunctionDecl) string {
	if c.proj != nil && c.proj.current != nil && fn.IsStatic {
		return staticFuncKey(c.proj.current.fileTag, strings.ToUpper(fn.Name))
	}
	return fn.Name
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
		c.collectMethodDeclAnns(cls, prog.Namespace)
	}

	// Compile main body
	c.bc.MainOffset = len(c.bc.Code)
	for _, stmt := range prog.Body {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
	}

	// If no body statements, auto-call the first user function declared
	// no próprio arquivo raiz (Loc.Line >= RootBoundaryLine). Funções
	// trazidas via #include (de bibliotecas auxiliares, coladas ANTES
	// desse ponto pela convenção real de colocar includes no topo do
	// arquivo) não contam para essa escolha — uma lib incluída não deve
	// virar o ponto de entrada silencioso do programa que a inclui. Se
	// nenhuma função "própria" existir (ex.: o arquivo raiz é só um
	// #include), cai no comportamento antigo (primeira função de todas),
	// preservando compatibilidade.
	if soDeclaracoesStatic(prog.Body) && len(prog.Functions) > 0 {
		var entry *ast.FunctionDecl
		for _, fn := range prog.Functions {
			if fn.IsUser && fn.Loc.Line >= prog.RootBoundaryLine {
				entry = fn
				break
			}
		}
		if entry == nil {
			for _, fn := range prog.Functions {
				if fn.IsUser {
					entry = fn
					break
				}
			}
		}
		if entry != nil {
			c.emit(OP_CALL_FUNC, 0, 0, entry.Name, entry.Loc.Line)
			c.emit(OP_POP, 0, 0, "", entry.Loc.Line)
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
	info := c.bc.Functions[c.functionKey(fn)]
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

// collectMethodDeclAnns indexa as anotações dos métodos declarados dentro
// do class (@GET/@Post de controladores REST TLPP) por "CLASSE::METODO"
// em maiúsculas — bare e namespaced, para casar com o ClassName do
// MethodImpl seja ele escrito de qual forma for.
func (c *Compiler) collectMethodDeclAnns(cls *ast.ClassDecl, namespace string) {
	for _, m := range cls.Methods {
		if len(m.Annotations) == 0 {
			continue
		}
		anns := make([]AnnotationInfo, 0, len(m.Annotations))
		for _, a := range m.Annotations {
			anns = append(anns, AnnotationInfo{Name: a.Name, Value: a.Value})
		}
		c.methodDeclAnns[strings.ToUpper(cls.Name)+"::"+strings.ToUpper(m.Name)] = anns
		if namespace != "" {
			c.methodDeclAnns[strings.ToUpper(namespace+"."+cls.Name)+"::"+strings.ToUpper(m.Name)] = anns
		}
	}
}

func (c *Compiler) compileMethod(m *ast.MethodImpl) error {
	// Classe/método com caixa diferente da declarada resolvem para a chave
	// canônica — o OP_NEW_INSTANCE carrega o nome que a VM encontra em
	// bc.Classes e o _Super: (ctx.className) aponta para a classe real.
	className := m.ClassName
	if key, ok := c.classKey(m.ClassName); ok {
		className = key
	}
	funcName := className + "::" + m.Name
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

	cls, ok := c.bc.Classes[className]
	if !ok {
		cls = &ClassInfo{Name: className, Properties: make(map[string]string), Methods: make(map[string]*FunctionInfo)}
		c.bc.Classes[className] = cls
	}
	cls.Methods[m.Name] = info
	c.bc.Functions[funcName] = info

	// Anotações do MethodDecl correspondente (@GET de controlador REST
	// TLPP) — indexadas no registro da classe, pois o MethodImpl não as
	// carrega. Tenta o ClassName como escrito e a chave resolvida.
	for _, ck := range []string{m.ClassName, className} {
		if anns, ok := c.methodDeclAnns[strings.ToUpper(ck)+"::"+strings.ToUpper(m.Name)]; ok {
			info.Annotations = append(info.Annotations, anns...)
			break
		}
	}

	ctx := &funcContext{
		name:      funcName,
		className: className,
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
		// Método sem Return explícito: cai no mesmo retorno implícito de
		// `return` bare — Self (ver compileReturn).
		c.emit(OP_LOAD_SELF, 0, 0, "", 0)
		c.emit(OP_RETURN_VALUE, 0, 0, "", 0)
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

// fileStatic: dentro de uma função, o índice global de um Static de arquivo.
// Também vale no nível do arquivo quando há Static declarados (init de
// Static do CompileProject: `Static b := a + 1` precisa enxergar `a`).
// Lookup case-insensitive (AdvPL não diferencia maiúsculas): chave exata,
// depois maiúsculas e por fim comparação dobrada — cobre Static declarados
// com caixa mista no fonte único e as chaves em maiúsculas do projeto.
func (c *Compiler) fileStatic(name string) (int, bool) {
	if c.fileStatics == nil {
		return 0, false
	}
	if idx, ok := c.fileStatics[name]; ok {
		return idx, true
	}
	if idx, ok := c.fileStatics[strings.ToUpper(name)]; ok {
		return idx, true
	}
	for k, idx := range c.fileStatics {
		if strings.EqualFold(k, name) {
			return idx, true
		}
	}
	return 0, false
}

// classKey resolve o nome de uma classe para a chave canônica registrada
// em bc.Classes, ignorando caixa — AdvPL não diferencia maiúsculas, então
// `Method X() Class REPO` e `Class Repo` são a mesma classe, e
// `REPO():New()` instancia a `Repo` declarada. Devolve ("", false) quando
// não há classe registrada; o chamador decide o fallback.
func (c *Compiler) classKey(name string) (string, bool) {
	if _, ok := c.bc.Classes[name]; ok {
		return name, true
	}
	upper := strings.ToUpper(name)
	for k := range c.bc.Classes {
		if strings.ToUpper(k) == upper {
			return k, true
		}
	}
	return "", false
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
			return idx | 0x8000 // mark as global
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
		if err := c.compileExpr(s.Expr); err != nil {
			return err
		}
		// O valor de uma expressão usada como comando (chamada de função,
		// x++, x--) é descartado. Sem este POP cada iteração de laço vazava
		// um valor na pilha da VM até estourar MaxStackSize, e o programa
		// entrava em loop infinito silencioso.
		// DefaultExpr é exceção: ele já consome o próprio valor (STORE_LOCAL
		// ou POP interno) e não deixa nada na pilha — um POP aqui comeria
		// um slot do frame chamador.
		if _, isDefault := s.Expr.(*ast.DefaultExpr); !isDefault {
			c.emit(OP_POP, 0, 0, "", s.Loc.Line)
		}
		return nil
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
	if c.currentFunc == nil && scope == "static" {
		idx := c.addLocal(s.Name) & 0x7FFF
		if c.fileStatics == nil {
			c.fileStatics = map[string]int{}
		}
		c.fileStatics[s.Name] = idx
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

// emitDefaultExpr empilha na pilha o valor default de um tipo declarado
// (Numeric->0, Character->"", Logical->.F., Date, Array, demais->Nil).
// Separado de emitDefaultValue para reuso pelo init de Static de arquivo
// no CompileProject, que grava num slot global já alocado.
func emitDefaultExpr(c *Compiler, typeName string, line int) {
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
}

func emitDefaultValue(c *Compiler, typeName, varName string, line int) {
	emitDefaultExpr(c, typeName, line)
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
		} else if gi, ok := c.fileStatic(target.Name); ok {
			c.emit(OP_STORE_GLOBAL, gi, 0, target.Name, line)
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
	} else if gi, ok := c.fileStatic(name); ok {
		c.emit(OP_LOAD_GLOBAL, gi, 0, name, line)
	} else {
		idx := c.addLocal(name)
		c.emit(OP_LOAD_GLOBAL, idx&0x7FFF, 0, name, line)
	}
}

func (c *Compiler) emitStore(name string, line int) {
	if _, local := c.resolveLocal(name); !local {
		if gi, ok := c.fileStatic(name); ok {
			c.emit(OP_STORE_GLOBAL, gi, 0, name, line)
			return
		}
	}
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
	} else if c.currentFunc != nil && c.currentFunc.className != "" {
		// `Return` sem expressão dentro de método devolve Self — sem isso
		// `Classe():new()` com `return` bare (padrão onipresente no corpus
		// TOTVS) devolveria Nil e todo `o := Classe():new()` quebraria.
		c.emit(OP_LOAD_SELF, 0, 0, "", s.Loc.Line)
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
	case *ast.NamedParam:
		// NamedParam só é consumido especialmente dentro de compileArgs (lista de
		// argumentos de chamada comum). Quando aparece via compileExpr direto —
		// ex.: ramo do IF()/IIF() de 3 args, que evita compileArgs pra manter o
		// curto-circuito — o nó É sempre uma atribuição real (`ident := expr`
		// como argumento, idioma comum em AdvPL/Clipper), nunca parâmetro
		// nomeado de fato. Trata como AssignExpr comum: calcula o valor, guarda
		// em ident, e deixa o valor (não a atribuição) no topo da pilha.
		return c.compileExpr(&ast.AssignExpr{Loc: e.Loc, Target: &ast.Ident{Loc: e.Loc, Name: e.Name}, Value: e.Value})
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
		} else if gi, ok := c.fileStatic(e.Name); ok {
			c.emit(OP_LOAD_GLOBAL, gi, 0, e.Name, e.Loc.Line)
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
	case *ast.RefArg:
		// `@nome`: mesma resolução do Ident (local, upvalue, Static de arquivo,
		// local do corpo principal, dinâmica), mas empilha a referência.
		if idx, ok := c.resolveLocal(e.Name); ok {
			c.emit(OP_REF_LOCAL, idx, 0, e.Name, e.Loc.Line)
		} else if uidx, ok := c.resolveUpvalue(e.Name); ok {
			c.emit(OP_REF_UPVAL, uidx, 0, e.Name, e.Loc.Line)
		} else if gi, ok := c.fileStatic(e.Name); ok {
			c.emit(OP_REF_GLOBAL, gi, 0, e.Name, e.Loc.Line)
		} else if c.currentFunc == nil {
			idx := c.addLocal(e.Name)
			if idx&0x8000 != 0 {
				c.emit(OP_REF_GLOBAL, idx&0x7FFF, 0, e.Name, e.Loc.Line)
			} else {
				c.emit(OP_REF_LOCAL, idx, 0, e.Name, e.Loc.Line)
			}
		} else {
			c.emit(OP_REF_DYN, 0, 0, e.Name, e.Loc.Line)
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
		if err := c.compileArgs(e.Args); err != nil { // via compileArgs p/ tratar NamedParam como nos demais calls
			return err
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
		if err := c.compileArgs(e.Args); err != nil { // via compileArgs p/ tratar NamedParam como nos demais calls
			return err
		}
		// Chave canônica da classe (caixa-insensível) — a VM resolve
		// OP_NEW_INSTANCE por bc.Classes e falharia com caixa divergente.
		className := e.ClassName
		if key, ok := c.classKey(e.ClassName); ok {
			className = key
		}
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), className, e.Loc.Line)
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
		} else {
			// Alvo não-local (::Prop, aArr[i], oObj:k, alias->campo): o
			// DefaultExpr é statement — deve deixar a pilha neutra. Sem
			// este POP o valor vazava para o POP do ExprStmt, que comia um
			// slot do frame chamador (bug que corrompia pilha em métodos
			// com `Default param := v`).
			c.emit(OP_POP, 0, 0, "", e.Loc.Line)
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
		return fmt.Errorf("unsupported expression type: %T (linha %d)", expr, expr.Pos().Line)
	}
	return nil
}

func (c *Compiler) compileBinaryOp(e *ast.BinaryOp) error {
	if e.Op == ".And." || e.Op == ".Or." {
		return c.compileLogicOp(e)
	}
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

// compileLogicOp: .And./.Or. com curto-circuito, como no Protheus — o operando
// direito só é avaliado quando decide o resultado (`x != Nil .And. x:Foo()` não
// chama Foo com x Nil). O resultado é sempre lógico, pelo mesmo critério do
// OP_AND/OP_OR (IsTruthy); o duplo OP_NOT converte o operando direito.
func (c *Compiler) compileLogicOp(e *ast.BinaryOp) error {
	if err := c.compileExpr(e.Left); err != nil {
		return err
	}
	salto, curto := OP_JUMP_IF_FALSE, OP_FALSE
	if e.Op == ".Or." {
		salto, curto = OP_JUMP_IF_TRUE, OP_TRUE
	}
	jCurto := c.emitJump(salto, e.Loc.Line)
	if err := c.compileExpr(e.Right); err != nil {
		return err
	}
	c.emit(OP_NOT, 0, 0, "", e.Loc.Line)
	c.emit(OP_NOT, 0, 0, "", e.Loc.Line)
	jFim := c.emitJump(OP_JUMP, e.Loc.Line)
	c.patchJump(jCurto, len(c.bc.Code))
	c.emit(curto, 0, 0, "", e.Loc.Line)
	c.patchJump(jFim, len(c.bc.Code))
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

	// Static Function do arquivo em compilação (CompileProject): resolve
	// para a chave qualificada "STATIC:<ARQUIVO>:<NOME>". Vem antes das
	// classes/built-ins porque a static é o vínculo mais específico do
	// arquivo — mesmo motivo pelo qual hasFunction faz PoUI ceder a uma
	// função do programa.
	if c.proj != nil && c.proj.current != nil {
		if key, ok := c.proj.current.statics[strings.ToUpper(e.Name)]; ok {
			if err := c.compileArgs(e.Args); err != nil {
				return err
			}
			c.emit(OP_CALL_FUNC, 0, len(e.Args), key, e.Loc.Line)
			return nil
		}
	}
	// Check if this is a class constructor call (e.g., Calculator():New()).
	// classKey resolve a caixa divergente (`REPO():New()` acha `Class Repo`).
	if key, isClass := c.classKey(e.Name); isClass {
		if err := c.compileArgs(e.Args); err != nil {
			return err
		}
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), key, e.Loc.Line)
		return nil
	}
	// Check known built-in classes
	if isBuiltinClass(e.Name) || (isPoClassName(e.Name) && !c.hasFunction(e.Name)) {
		if err := c.compileArgs(e.Args); err != nil {
			return err
		}
		c.emit(OP_NEW_INSTANCE, 0, len(e.Args), e.Name, e.Loc.Line)
		return nil
	}
	if err := c.compileArgs(e.Args); err != nil {
		return err
	}
	if strings.ToUpper(e.Name) == "EVAL" {
		c.emit(OP_EVAL_CODEBLOCK, 0, len(e.Args)-1, "", e.Loc.Line)
		return nil
	}
	if k, ok := c.bc.ResolveFunc(e.Name); ok {
		c.emit(OP_CALL_FUNC, 0, len(e.Args), k, e.Loc.Line)
	} else {
		c.emit(OP_CALL_NATIVE, 0, len(e.Args), e.Name, e.Loc.Line)
	}
	return nil
}

// compileArgs compila a lista de argumentos de uma chamada. Propaga o
// primeiro erro de compileExpr — antes o retorno era descartado, o que
// permitia que um argumento inválido (ex.: _Super: fora de método) sumisse
// silenciosamente e corrompesse a pilhaa de operandos do bytecode.
func (c *Compiler) compileArgs(args []ast.Expression) error {
	for _, arg := range args {
		if np, ok := arg.(*ast.NamedParam); ok {
			c.emit(OP_NAMED_ARG, 0, 0, np.Name, np.Loc.Line)
			if err := c.compileExpr(np.Value); err != nil {
				return err
			}
		} else {
			c.emit(OP_NAMED_ARG, 0, 0, "", arg.Pos().Line)
			if err := c.compileExpr(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Compiler) compileMethodCall(e *ast.MethodCall) error {
	// `_Super:Metodo(args)` — chamada ao método da superclasse sobre o self
	// do frame atual. Str leva "Classe:Metodo" da classe que define o método
	// (c.currentFunc.className); a resolução do pai fica para a VM, que a faz
	// em runtime porque o pai pode estar em outro fonte (CompileProject) ou
	// ser uma classe nativa (registerNativeBase).
	if id, ok := e.Object.(*ast.Ident); ok && strings.EqualFold(id.Name, "_SUPER") {
		className := ""
		if c.currentFunc != nil {
			className = c.currentFunc.className
		}
		// _Super: só existe dentro de método de classe. Em função comum,
		// codeblock solto ou corpo top-level não há "classe que define o
		// método" — rejeitar aqui evita que a VM precise adivinhar um defClass
		// e despachar contra o self dinâmico de um frame pai.
		if className == "" {
			return fmt.Errorf("_Super: so pode ser usado dentro de metodo de classe (linha %d)", e.Loc.Line)
		}
		if err := c.compileArgs(e.Args); err != nil {
			return err
		}
		c.emit(OP_CALL_SUPER, 0, len(e.Args), className+":"+e.Method, e.Loc.Line)
		return nil
	}
	if err := c.compileExpr(e.Object); err != nil {
		return err
	}
	if err := c.compileArgs(e.Args); err != nil {
		return err
	}
	c.emit(OP_CALL_METHOD, 0, len(e.Args), e.Method, e.Loc.Line)
	return nil
}

var builtinClasses = map[string]bool{
	"ERRORCLASS":        true,
	"JSONOBJECT":        true,
	"JSONARRAY":         true,
	"FWFORMVIEW":        true,
	"FWFORMMODEL":       true,
	"FWFORMBROWSE":      true,
	"FWGRIDPROCESS":     true,
	"FWMBROWSE":         true,
	"LLM":               true,
	"MCPSERVER":         true,
	"WSRESTSERVER":      true,
	"TMAILMESSAGE":      true,
	"TENSOR":            true,
	"VARIABLE":          true,
	"SGD":               true,
	"ADAM":              true,
	"LINEAR":            true,
	"EMBEDDING":         true,
	"TRUNDLL":           true,
	"TPBKDF2":           true,
	"THASHMAP":          true,
	"TJSONPARSER":       true,
	"TUNICODE":          true,
	"TFTPCLIENT":        true,
	"TGRPC":             true,
	"FWTOTVSLINKCLIENT": true,
	"GRPCSERVER":        true,
	"DBCONNECTION":      true,
	// Canal SmartClient↔Navegador do Protheus (corpus 12.1.2510):
	// TWebChannel():New() / TWebEngine():New(...) — ver pkg/vm/twebengine.go.
	"TWEBCHANNEL": true,
	"TWEBENGINE":  true,
}

// soDeclaracoesStatic: o corpo de nível superior só declara Static de
// arquivo (comum no Protheus) — não é um programa em si. O corpo roda
// primeiro (inicializa os Static) e a função de entrada é chamada depois.
// Corpo vazio também conta (comportamento de sempre).
func soDeclaracoesStatic(body []ast.Statement) bool {
	for _, st := range body {
		switch d := st.(type) {
		case *ast.VarDecl:
			if !strings.EqualFold(d.Scope, "static") {
				return false
			}
		case *ast.VarDeclGroup:
			for _, v := range d.Decls {
				if !strings.EqualFold(v.Scope, "static") {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func isBuiltinClass(name string) bool {
	return builtinClasses[strings.ToUpper(name)]
}

// poClassNames: componentes PO-UI nativos (pkg/vm/poui*.go). Ficam fora de
// builtinClasses porque cedem a uma função do próprio programa com o mesmo
// nome: programas que já tinham, por exemplo, uma Static Function PoTag()
// continuam chamando a sua função.
var poClassNames = map[string]bool{
	"POAPP": true, "POPAGE": true, "POBUTTON": true, "POINPUT": true, "POTABLE": true, "POTAG": true,
	"POMENU": true, "POCONTAINER": true, "POWIDGET": true, "PODIVIDER": true, "POTABS": true, "POTAB": true,
	"POTREE": true, "POLIST": true, "POINFO": true, "POPROGRESS": true, "POGAUGE": true, "POCHART": true,
	"POPASSWORD": true, "POTEXTAREA": true, "PONUMBER": true, "POSELECT": true, "POCHECKBOX": true, "POSWITCH": true,
	"ADVCODEEDITOR": true, "ADVTERMINAL": true,
	// Onda 3a.1 — seleção
	"POCOMBO": true, "POMULTISELECT": true, "PORADIOGROUP": true, "POCHECKBOXGROUP": true, "POLOOKUP": true,
	// Onda 3a.2 — data e hora
	"PODATEPICKER": true, "PODATEPICKERRANGE": true, "PODATETIMEPICKER": true, "POTIMEPICKER": true,
	// Onda 3a.3 — texto especializado
	"POEMAIL": true, "POURL": true, "PODECIMAL": true, "PORICHTEXT": true, "POSEARCH": true,
	// Onda 3a.4 — upload
	"POUPLOAD": true,
	// Onda 3a.5 — templates estáticos
	"POPAGELIST": true, "POPAGEEDIT": true, "POPAGEDETAIL": true,
	// Onda 3a.6 — templates dinâmicos
	"POPAGEDYNAMICTABLE": true, "POPAGEDYNAMICEDIT": true,
	"POPAGEDYNAMICDETAIL": true, "POPAGEDYNAMICSEARCH": true,
	// Onda 3b.1 — agrupamento e sobreposição
	"POACCORDION": true, "POACCORDIONITEM": true, "POMODAL": true,
	"POSTEPPER": true, "POSTEP": true, "POSLIDE": true,
	// Onda 3b.2 — ações
	"POBUTTONGROUP": true, "PODROPDOWN": true, "POPOPUP": true,
	"POPOPOVER": true, "POCONTEXTMENU": true, "POTOOLBAR": true,
}

func isPoClassName(name string) bool {
	return poClassNames[strings.ToUpper(name)]
}

// hasFunction diz se o programa declara uma função com esse nome (as
// funções são todas registradas antes da geração de código).
func (c *Compiler) hasFunction(name string) bool {
	for fn := range c.bc.Functions {
		if strings.EqualFold(fn, name) {
			return true
		}
	}
	return false
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

	// _Super: dentro de closure resolve a classe do método que a contém.
	// savedFunc pode ser nil (codeblock no corpo principal do arquivo,
	// fora de qualquer função/método) — sem a guarda isso dereferenciava.
	cbClassName := ""
	if savedFunc != nil {
		cbClassName = savedFunc.className
	}
	c.currentFunc = &funcContext{
		name:      funcName,
		className: cbClassName,
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

// Compile takes AdvPL source code and returns bytecode.
// It handles tokenization, parsing, and compilation in one call.
func (c *Compiler) Compile(source string) (*Bytecode, error) {
	// Tokenize the source
	tokens, err := lexer.Tokenize(source, "source")
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Parse the tokens into an AST
	p := parser.NewParser(tokens, "source", nil)
	prog, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing failed: %w", err)
	}

	// Compile the AST to bytecode
	if err := c.compileProgram(prog); err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	return c.bc, nil
}
