package vm

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/mvc"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

type SignalKind int

const (
	SigNone SignalKind = iota
	SigReturn
	SigExit
	SigLoop
	SigBreak
)

type Signal struct {
	Kind  SignalKind
	Value advplrt.Value
}

type CallFrame struct {
	FuncName   string
	Code       []compiler.Instruction
	IP         int
	Locals     []advplrt.Value
	StackBase  int
	Self       advplrt.Value
	TryDepth   int
	TryCatches []*TryCatch
}

type TryCatch struct {
	CatchIP     int
	CatchVar    string
	CatchVarIdx int
	FinallyIP   int
	StackBase   int
}

type VM struct {
	bc           *compiler.Bytecode
	stack        []advplrt.Value
	frames       []*CallFrame
	current      *CallFrame
	natives      map[string]*advplrt.FunctionValue
	classes      map[string]*advplrt.ClassDef
	methodBodies map[string]interface{}
	uiEnabled    bool
	dbEngine     DBEngine
	currentAlias string // último alias passado para DbSelectArea, para GetArea()/RestArea()
	uiProvider   UIProvider
	output       strings.Builder
	namedArgs    []namedArgInfo // tracks named parameter info for current call
	argCounter   int            // counts args pushed for current call
	mvcModels    map[int]*mvc.FWFormModel
	mvcViews     map[int]*mvc.FWFormView
	mvcBrowses   map[int]*mvc.FWFormBrowse
	mvcNextID    int
	dbFactory    func() DBEngine // cria um engine próprio por job (StartJob)
	jobs         sync.WaitGroup  // jobs em background pendentes
	outWriter    io.Writer       // espelho opcional da saída de console (modo web)
	curDialog    *webDialog      // MSDIALOG em construção (fase 4 do renderer web)
	debugger     *Debugger       // hook opcional (advplc debug); nil em execução normal
	fileHandles  map[int]*os.File // handles abertos por FOpen/FCreate
	nextFH       int              // próximo handle a distribuir
	lastFError   int              // último erro de IO (FError())
}

type namedArgInfo struct {
	name     string
	argIndex int // position in the args array (0-based)
}

type DBEngine interface {
	SelectArea(alias string) error
	Seek(key string) (bool, error)
	Skip(count int) error
	GoTop() error
	GoBottom() error
	EOF() bool
	BOF() bool
	FieldGet(field string) (advplrt.Value, error)
	FieldPut(field string, val advplrt.Value) error
	RecLock() error
	MsUnlock() error
	RecCount() int
	RecNo() int
}

type UIProvider interface {
	MsgInfo(msg, title string)
	MsgStop(msg, title string)
	MsgAlert(msg, title string)
	MsgYesNo(msg, title string) bool
}

func NewVM(bc *compiler.Bytecode, uiEnabled bool) *VM {
	v := &VM{
		bc:           bc,
		stack:        make([]advplrt.Value, 0, 256),
		frames:       make([]*CallFrame, 0, 32),
		natives:      make(map[string]*advplrt.FunctionValue),
		classes:      make(map[string]*advplrt.ClassDef),
		methodBodies: make(map[string]interface{}),
		uiEnabled:    uiEnabled,
		mvcModels:    make(map[int]*mvc.FWFormModel),
		mvcViews:     make(map[int]*mvc.FWFormView),
		mvcBrowses:   make(map[int]*mvc.FWFormBrowse),
		mvcNextID:    1,
		fileHandles:  make(map[int]*os.File),
		nextFH:       1,
	}
	v.registerClasses()
	v.registerNatives()
	return v
}

func (v *VM) SetDBEngine(engine DBEngine) {
	v.dbEngine = engine
}

// SetDBFactory registra como abrir uma nova conexão de banco. Cada job
// disparado via StartJob() recebe seu próprio engine (o SQLite em WAL
// suporta leitores concorrentes), evitando corrida no engine principal.
func (v *VM) SetDBFactory(factory func() DBEngine) {
	v.dbFactory = factory
	if v.dbEngine == nil && factory != nil {
		v.dbEngine = factory()
	}
}

func (v *VM) SetUIProvider(provider UIProvider) {
	v.uiProvider = provider
}

// SetOutputWriter espelha a saída de console (ConOut etc.) para um writer
// adicional — usado pelo modo web (advplc serve) para transmitir ao browser.
func (v *VM) SetOutputWriter(w io.Writer) {
	v.outWriter = w
}

func (v *VM) writeOut(line string) {
	if v.outWriter != nil {
		fmt.Fprintln(v.outWriter, line)
	}
}

func (v *VM) GetOutput() string {
	return v.output.String()
}

func (v *VM) registerClasses() {
	for name, cls := range v.bc.Classes {
		cd := &advplrt.ClassDef{
			Name:       name,
			Parent:     cls.Parent,
			Properties: cls.Properties,
			Methods:    make(map[string]*advplrt.MethodDef),
		}
		for mname, minfo := range cls.Methods {
			cd.Methods[mname] = &advplrt.MethodDef{
				Name:      mname,
				ClassName: name,
				Params:    convertParams(minfo.ParamNames),
			}
		}
		v.classes[name] = cd
	}

	// Register built-in ErrorClass
	v.classes["ERRORCLASS"] = &advplrt.ClassDef{
		Name:       "ErrorClass",
		Properties: map[string]string{"description": "character", "genCode": "numeric"},
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {Name: "NEW", ClassName: "ErrorClass"},
		},
	}

	// Register MVC classes as built-in classes
	v.classes["FWFORMMODEL"] = &advplrt.ClassDef{
		Name:       "FWFormModel",
		Properties: map[string]string{"name": "character", "_modelId": "numeric"},
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {Name: "NEW", ClassName: "FWFormModel"},
		},
	}

	v.classes["FWFORMVIEW"] = &advplrt.ClassDef{
		Name:       "FWFormView",
		Properties: map[string]string{"name": "character", "_viewId": "numeric"},
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {Name: "NEW", ClassName: "FWFormView"},
		},
	}

	v.classes["FWFORMBROWSE"] = &advplrt.ClassDef{
		Name:       "FWFormBrowse",
		Properties: map[string]string{"name": "character", "_browseId": "numeric"},
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {Name: "NEW", ClassName: "FWFormBrowse"},
		},
	}
}

func convertParams(names []string) []*advplrt.ParamDef {
	params := make([]*advplrt.ParamDef, len(names))
	for i, n := range names {
		params[i] = &advplrt.ParamDef{Name: n}
	}
	return params
}

func (v *VM) push(val advplrt.Value) {
	v.stack = append(v.stack, val)
}

func (v *VM) pop() advplrt.Value {
	if len(v.stack) == 0 {
		return advplrt.Nil
	}
	val := v.stack[len(v.stack)-1]
	v.stack = v.stack[:len(v.stack)-1]
	return val
}

func (v *VM) peek() advplrt.Value {
	if len(v.stack) == 0 {
		return advplrt.Nil
	}
	return v.stack[len(v.stack)-1]
}

func (v *VM) Run() (advplrt.Value, error) {
	// Create main frame
	frame := &CallFrame{
		FuncName:  "main",
		Code:      v.bc.Code,
		IP:        v.bc.MainOffset,
		Locals:    make([]advplrt.Value, v.bc.NumGlobals),
		StackBase: 0,
	}
	v.frames = append(v.frames, frame)
	v.current = frame

	result, err := v.runLoop()

	// Espera jobs em background (StartJob lWait=.F.) antes de encerrar —
	// num processo CLI, sair da main mataria os jobs silenciosamente.
	v.jobs.Wait()

	return result, err
}

// RunFunction executa uma função específica do bytecode (busca
// case-insensitive) num VM já inicializado. Usada por StartJob.
func (v *VM) RunFunction(name string, args []advplrt.Value) (advplrt.Value, error) {
	info, ok := v.bc.Functions[name]
	if !ok {
		// Busca case-insensitive; o prefixo U_ de User Function é aceito
		// (o bytecode guarda o nome declarado, sem o prefixo)
		upper := strings.ToUpper(name)
		trimmed := strings.TrimPrefix(upper, "U_")
		for fname, finfo := range v.bc.Functions {
			fupper := strings.ToUpper(fname)
			if fupper == upper || fupper == trimmed {
				info, ok = finfo, true
				break
			}
		}
	}
	if !ok {
		return advplrt.Nil, fmt.Errorf("function %s not found", name)
	}

	locals := make([]advplrt.Value, info.NumLocals)
	for i := 0; i < len(args) && i < info.NumParams; i++ {
		locals[i] = args[i]
	}
	frame := &CallFrame{
		FuncName:  name,
		Code:      v.bc.Code,
		IP:        info.Offset,
		Locals:    locals,
		StackBase: len(v.stack),
	}
	v.frames = append(v.frames, frame)
	v.current = frame

	return v.runLoop()
}

// StartJob dispara a execução de uma função em um VM isolado (memória
// própria, como um work process do Protheus). Com wait=true o chamador
// bloqueia até o término; com wait=false roda em goroutine e o Run()
// principal espera a conclusão antes de encerrar o processo.
func (v *VM) StartJob(funcName string, wait bool, args []advplrt.Value) error {
	job := NewVM(v.bc, false)
	job.dbFactory = v.dbFactory
	if v.dbFactory != nil {
		job.dbEngine = v.dbFactory()
	}

	if wait {
		_, err := job.RunFunction(funcName, args)
		return err
	}

	v.jobs.Add(1)
	go func() {
		defer v.jobs.Done()
		if _, err := job.RunFunction(funcName, args); err != nil {
			fmt.Printf("StartJob(%s) error: %v\n", funcName, err)
		}
	}()
	return nil
}

func (v *VM) runLoop() (advplrt.Value, error) {
	for {
		// v.current fica nil quando o último frame retorna (RunFunction)
		if v.current == nil || v.current.IP >= len(v.current.Code) {
			break
		}

		instr := v.current.Code[v.current.IP]
		v.current.IP++

		if v.debugger != nil {
			v.debugger.checkBreak(v, instr)
		}

		if err := v.execute(instr); err != nil {
			// Check try/catch
			if advErr, ok := err.(*advplrt.ErrorValue); ok {
				if v.handleCatch(advErr) {
					continue
				}
				return advplrt.Nil, fmt.Errorf("%s", advErr.String())
			}
			return advplrt.Nil, err
		}

		// Check for HALT
		if instr.Op == compiler.OP_HALT {
			break
		}
	}

	if len(v.stack) > 0 {
		return v.pop(), nil
	}
	return advplrt.Nil, nil
}

func (v *VM) handleCatch(errVal *advplrt.ErrorValue) bool {
	frame := v.current
	for i := len(frame.TryCatches) - 1; i >= 0; i-- {
		tc := frame.TryCatches[i]
		frame.TryCatches = frame.TryCatches[:i]
		if len(v.stack) > tc.StackBase {
			v.stack = v.stack[:tc.StackBase]
		}
		if tc.CatchVarIdx >= 0 && tc.CatchVarIdx < len(frame.Locals) {
			frame.Locals[tc.CatchVarIdx] = errVal
		}
		frame.IP = tc.CatchIP
		return true
	}
	return false
}

func (v *VM) getLocalIndex(name string) int {
	// In bytecode VM, locals are indexed by position
	// We need to find the index from function info
	if fn, ok := v.bc.Functions[v.current.FuncName]; ok {
		for i, p := range fn.ParamNames {
			if strings.EqualFold(p, name) {
				return i
			}
		}
	}
	return -1
}

func (v *VM) execute(instr compiler.Instruction) error {
	switch instr.Op {
	case compiler.OP_NIL:
		v.push(advplrt.Nil)
	case compiler.OP_TRUE:
		v.push(advplrt.True)
	case compiler.OP_FALSE:
		v.push(advplrt.False)
	case compiler.OP_NUMBER:
		if instr.Arg < len(v.bc.Constants) {
			c := v.bc.Constants[instr.Arg]
			v.push(advplrt.NewNumber(c.Num))
		}
	case compiler.OP_STRING:
		if instr.Arg < len(v.bc.Constants) {
			c := v.bc.Constants[instr.Arg]
			v.push(advplrt.NewString(c.Str))
		}
	case compiler.OP_DATE:
		if instr.Arg < len(v.bc.Constants) {
			c := v.bc.Constants[instr.Arg]
			v.push(advplrt.NewDate(time.Unix(int64(c.Num), 0)))
		} else {
			v.push(advplrt.NewDate(timeZero()))
		}
	case compiler.OP_LOAD_LOCAL:
		if instr.Arg < len(v.current.Locals) {
			v.push(v.current.Locals[instr.Arg])
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_STORE_LOCAL:
		val := v.pop()
		if instr.Arg < len(v.current.Locals) {
			v.current.Locals[instr.Arg] = val
		}
	case compiler.OP_LOAD_GLOBAL:
		if instr.Arg < len(v.current.Locals) {
			v.push(v.current.Locals[instr.Arg])
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_STORE_GLOBAL:
		val := v.pop()
		if instr.Arg < len(v.current.Locals) {
			v.current.Locals[instr.Arg] = val
		}
	case compiler.OP_LOAD_SELF:
		v.push(v.current.Self)
	case compiler.OP_STORE_SELF:
		v.current.Self = v.pop()
	case compiler.OP_POP:
		v.pop()
	case compiler.OP_DUP:
		v.push(v.peek())
	case compiler.OP_ADD:
		return v.opAdd()
	case compiler.OP_SUB:
		return v.opBinary(func(a, b float64) float64 { return a - b }, "OPERATOR_SUB")
	case compiler.OP_MUL:
		return v.opBinary(func(a, b float64) float64 { return a * b }, "OPERATOR_MULT")
	case compiler.OP_DIV:
		return v.opBinary(func(a, b float64) float64 { return a / b }, "OPERATOR_DIV")
	case compiler.OP_MOD:
		return v.opBinary(func(a, b float64) float64 { return float64(int64(a) % int64(b)) }, "")
	case compiler.OP_POW:
		return v.opBinary(math.Pow, "")
	case compiler.OP_NEG:
		val := v.pop()
		v.push(advplrt.NewNumber(-advplrt.ToFloat(val)))
	case compiler.OP_EQ:
		return v.opComparison(func(a, b advplrt.Value) bool { return a.Equals(b) }, "OPERATOR_COMPARE")
	case compiler.OP_NEQ:
		return v.opComparison(func(a, b advplrt.Value) bool { return !a.Equals(b) }, "")
	case compiler.OP_LT:
		return v.opComparison(func(a, b advplrt.Value) bool { return advplrt.ToFloat(a) < advplrt.ToFloat(b) }, "")
	case compiler.OP_GT:
		return v.opComparison(func(a, b advplrt.Value) bool { return advplrt.ToFloat(a) > advplrt.ToFloat(b) }, "")
	case compiler.OP_LTE:
		return v.opComparison(func(a, b advplrt.Value) bool { return advplrt.ToFloat(a) <= advplrt.ToFloat(b) }, "")
	case compiler.OP_GTE:
		return v.opComparison(func(a, b advplrt.Value) bool { return advplrt.ToFloat(a) >= advplrt.ToFloat(b) }, "")
	case compiler.OP_FORLOOP_CMP:
		step := advplrt.ToFloat(v.pop())
		end := advplrt.ToFloat(v.pop())
		vv := advplrt.ToFloat(v.pop())
		var cont bool
		if step >= 0 {
			cont = vv <= end
		} else {
			cont = vv >= end
		}
		v.push(advplrt.NewBool(cont))
	case compiler.OP_AND:
		return v.opLogic(true)
	case compiler.OP_OR:
		return v.opLogic(false)
	case compiler.OP_NOT:
		val := v.pop()
		v.push(advplrt.NewBool(!val.IsTruthy()))
	case compiler.OP_DOLLAR:
		return v.opDollar()
	case compiler.OP_CONCAT:
		return v.opConcat()
	case compiler.OP_NEW_ARRAY:
		count := instr.Arg
		elems := make([]advplrt.Value, count)
		for i := count - 1; i >= 0; i-- {
			elems[i] = v.pop()
		}
		v.push(advplrt.NewArray(elems))
	case compiler.OP_ARRAY_GET:
		idx := v.pop()
		arr := v.pop()
		if a, ok := arr.(*advplrt.ArrayValue); ok {
			i := int(advplrt.ToFloat(idx))
			if i >= 1 && i <= len(a.Elements) {
				v.push(a.Elements[i-1])
			} else {
				v.push(advplrt.Nil)
			}
		} else if o, ok := arr.(*advplrt.ObjectValue); ok {
			// Chave de bracket em JsonObject/hash: case-sensitive (semantica JSON).
			if s, ok := idx.(*advplrt.StringValue); ok {
				if val, exists := o.Props[s.Val]; exists {
					v.push(val)
				} else {
					v.push(advplrt.Nil)
				}
			} else {
				v.push(advplrt.Nil)
			}
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_ARRAY_SET:
		idx := v.pop()
		arr := v.pop()
		val := v.pop()
		if a, ok := arr.(*advplrt.ArrayValue); ok {
			i := int(advplrt.ToFloat(idx))
			if i >= 1 && i <= len(a.Elements) {
				a.Elements[i-1] = val
			}
		} else if o, ok := arr.(*advplrt.ObjectValue); ok {
			// Chave de bracket em JsonObject/hash: case-sensitive (semantica JSON).
			if s, ok := idx.(*advplrt.StringValue); ok {
				o.Props[s.Val] = val
			}
		}
	case compiler.OP_ARRAY_LEN:
		val := v.pop()
		if a, ok := val.(*advplrt.ArrayValue); ok {
			v.push(advplrt.NewNumber(float64(len(a.Elements))))
		} else if s, ok := val.(*advplrt.StringValue); ok {
			v.push(advplrt.NewNumber(float64(len(s.Val))))
		} else {
			v.push(advplrt.NewNumber(0))
		}
	case compiler.OP_NEW_OBJECT:
		count := instr.Arg
		obj := advplrt.NewObject("json", nil)
		for i := 0; i < count; i++ {
			val := v.pop()
			key := v.pop()
			if s, ok := key.(*advplrt.StringValue); ok {
				obj.Props[strings.ToUpper(s.Val)] = val
			}
		}
		v.push(obj)
	case compiler.OP_GET_PROP:
		propName := instr.Str
		obj := v.pop()
		if o, ok := obj.(*advplrt.ObjectValue); ok {
			if val, exists := o.Props[strings.ToUpper(propName)]; exists {
				v.push(val)
			} else {
				v.push(advplrt.Nil)
			}
		} else if e, ok := obj.(*advplrt.ErrorValue); ok {
			switch strings.ToUpper(propName) {
			case "DESCRIPTION":
				v.push(advplrt.NewString(e.Description))
			case "GENCODE":
				v.push(advplrt.NewNumber(float64(e.GenCode)))
			case "SEVERITY":
				v.push(advplrt.NewString(e.Severity))
			default:
				v.push(advplrt.Nil)
			}
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_SET_PROP:
		propName := instr.Str
		obj := v.pop()
		val := v.pop()
		if o, ok := obj.(*advplrt.ObjectValue); ok {
			o.Props[strings.ToUpper(propName)] = val
		} else if e, ok := obj.(*advplrt.ErrorValue); ok {
			switch strings.ToUpper(propName) {
			case "DESCRIPTION":
				e.Description = advplrt.ToString(val)
			case "GENCODE":
				e.GenCode = int(advplrt.ToFloat(val))
			case "SEVERITY":
				e.Severity = advplrt.ToString(val)
			}
		}
	case compiler.OP_NEW_INSTANCE:
		className := instr.Str
		argCount := instr.Arg2
		args := make([]advplrt.Value, argCount)
		for i := argCount - 1; i >= 0; i-- {
			args[i] = v.pop()
		}
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return v.newInstance(className, args)
	case compiler.OP_CALL_FUNC:
		return v.callFunc(instr.Str, instr.Arg2)
	case compiler.OP_CALL_NATIVE:
		return v.callNative(instr.Str, instr.Arg2)
	case compiler.OP_CALL_METHOD:
		return v.callMethod(instr.Str, instr.Arg2)
	case compiler.OP_RETURN:
		return v.doReturn(advplrt.Nil)
	case compiler.OP_RETURN_VALUE:
		val := v.pop()
		return v.doReturn(val)
	case compiler.OP_JUMP:
		v.current.IP = instr.Arg
	case compiler.OP_JUMP_IF_FALSE:
		val := v.pop()
		if !val.IsTruthy() {
			v.current.IP = instr.Arg
		}
	case compiler.OP_JUMP_IF_TRUE:
		val := v.pop()
		if val.IsTruthy() {
			v.current.IP = instr.Arg
		}
	case compiler.OP_TRY_BEGIN:
		v.current.TryDepth++
		tc := &TryCatch{
			CatchIP:   instr.Arg,
			StackBase: len(v.stack),
		}
		if instr.Arg2 >= 0 && instr.Str != "" {
			tc.CatchVar = instr.Str
			tc.CatchVarIdx = instr.Arg2
		}
		v.current.TryCatches = append(v.current.TryCatches, tc)
	case compiler.OP_TRY_END:
		if len(v.current.TryCatches) > 0 {
			v.current.TryCatches = v.current.TryCatches[:len(v.current.TryCatches)-1]
		}
		v.current.TryDepth--
	case compiler.OP_THROW:
		val := v.pop()
		if errVal, ok := val.(*advplrt.ErrorValue); ok {
			return errVal
		}
		// Check if it's an ErrorClass object
		if obj, ok := val.(*advplrt.ObjectValue); ok && strings.EqualFold(obj.ClassName, "ErrorClass") {
			desc := ""
			if d, ok := obj.Props["DESCRIPTION"]; ok {
				desc = advplrt.ToString(d)
			}
			genCode := 0
			if g, ok := obj.Props["GENCODE"]; ok {
				genCode = int(advplrt.ToFloat(g))
			}
			return &advplrt.ErrorValue{Description: desc, GenCode: genCode, Severity: "ERROR", ClassName: "ErrorClass"}
		}
		return advplrt.NewError(advplrt.ToString(val))
	case compiler.OP_CATCH:
		// Error value is already stored in local by handleCatch
		// This opcode is a no-op marker
	case compiler.OP_DB_SELECT:
		v.currentAlias = instr.Str
		if v.dbEngine != nil {
			v.dbEngine.SelectArea(instr.Str)
		}
	case compiler.OP_DB_SEEK:
		if v.dbEngine != nil {
			key := v.pop()
			if s, ok := key.(*advplrt.StringValue); ok {
				v.dbEngine.Seek(s.Val)
			}
		}
	case compiler.OP_DB_SKIP:
		if v.dbEngine != nil {
			count := int(advplrt.ToFloat(v.pop()))
			v.dbEngine.Skip(count)
		}
	case compiler.OP_DB_GOTOP:
		if v.dbEngine != nil {
			v.dbEngine.GoTop()
		}
	case compiler.OP_DB_GOBOTTOM:
		if v.dbEngine != nil {
			v.dbEngine.GoBottom()
		}
	case compiler.OP_EOF:
		if v.dbEngine != nil {
			v.push(advplrt.NewBool(v.dbEngine.EOF()))
		} else {
			v.push(advplrt.True)
		}
	case compiler.OP_BOF:
		if v.dbEngine != nil {
			v.push(advplrt.NewBool(v.dbEngine.BOF()))
		} else {
			v.push(advplrt.True)
		}
	case compiler.OP_FIELD_GET:
		if v.dbEngine != nil {
			val, err := v.dbEngine.FieldGet(instr.Str)
			if err != nil {
				return err
			}
			v.push(val)
		} else {
			v.push(advplrt.Nil)
		}
	case compiler.OP_FIELD_PUT:
		val := v.pop()
		if v.dbEngine != nil {
			if err := v.dbEngine.FieldPut(instr.Str, val); err != nil {
				return err
			}
		}
	case compiler.OP_REC_LOCK:
		if v.dbEngine != nil {
			v.dbEngine.RecLock()
		}
	case compiler.OP_MS_UNLOCK:
		if v.dbEngine != nil {
			v.dbEngine.MsUnlock()
		}
	case compiler.OP_MVC_NEW_MODEL:
		name := v.pop()
		if s, ok := name.(*advplrt.StringValue); ok {
			obj := advplrt.NewObject("FWFormModel", nil)
			obj.Props["name"] = s
			v.push(obj)
		}
	case compiler.OP_MVC_NEW_VIEW:
		name := v.pop()
		model := v.pop()
		if s, ok := name.(*advplrt.StringValue); ok {
			obj := advplrt.NewObject("FWFormView", nil)
			obj.Props["name"] = s
			obj.Props["model"] = model
			v.push(obj)
		}
	case compiler.OP_MVC_NEW_BROWSE:
		name := v.pop()
		model := v.pop()
		if s, ok := name.(*advplrt.StringValue); ok {
			obj := advplrt.NewObject("FWFormBrowse", nil)
			obj.Props["name"] = s
			obj.Props["model"] = model
			v.push(obj)
		}
	case compiler.OP_MVC_ADD_FIELD:
		v.pop()
	case compiler.OP_MVC_ADD_COMPONENT:
		v.pop()
	case compiler.OP_MVC_ADD_COLUMN:
		v.pop()
	case compiler.OP_MVC_SET_PROPERTY:
		value := v.pop()
		prop := v.pop()
		obj := v.pop()
		if o, ok := obj.(*advplrt.ObjectValue); ok {
			if p, ok := prop.(*advplrt.StringValue); ok {
				o.Props[p.Val] = value
			}
		}
	case compiler.OP_MVC_GET_PROPERTY:
		prop := v.pop()
		obj := v.pop()
		if o, ok := obj.(*advplrt.ObjectValue); ok {
			if p, ok := prop.(*advplrt.StringValue); ok {
				if val, exists := o.Props[p.Val]; exists {
					v.push(val)
				} else {
					v.push(advplrt.Nil)
				}
			}
		}
	case compiler.OP_MVC_VALIDATE:
		v.push(advplrt.True)
	case compiler.OP_MVC_SHOW:
		fmt.Println("[MVC] Show called")
	case compiler.OP_HALT:
		// Stop execution
	case compiler.OP_NAMED_ARG:
		if instr.Str != "" {
			v.namedArgs = append(v.namedArgs, namedArgInfo{name: instr.Str, argIndex: v.argCounter})
		}
		v.argCounter++
	case compiler.OP_NEW_CODEBLOCK:
		funcName := instr.Str
		paramCount := instr.Arg2
		params := make([]string, paramCount)
		cb := &advplrt.CodeBlockValue{
			Params:   params,
			FuncName: funcName,
		}
		v.push(cb)
	case compiler.OP_EVAL_CODEBLOCK:
		argCount := instr.Arg2
		args := make([]advplrt.Value, argCount)
		for i := argCount - 1; i >= 0; i-- {
			args[i] = v.pop()
		}
		cb := v.pop()
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		if cbVal, ok := cb.(*advplrt.CodeBlockValue); ok {
			info, ok := v.bc.Functions[cbVal.FuncName]
			if !ok {
				return fmt.Errorf("codeblock function %s not found", cbVal.FuncName)
			}
			locals := make([]advplrt.Value, info.NumLocals)
			locals[0] = cbVal // self
			for i := 0; i < len(args) && i+1 < info.NumParams; i++ {
				locals[i+1] = args[i]
			}
			frame := &CallFrame{
				FuncName:  cbVal.FuncName,
				Code:      v.bc.Code,
				IP:        info.Offset,
				Locals:    locals,
				StackBase: len(v.stack),
			}
			v.frames = append(v.frames, frame)
			v.current = frame
		}
	case compiler.OP_SWAP:
		if len(v.stack) >= 2 {
			n := len(v.stack)
			v.stack[n-1], v.stack[n-2] = v.stack[n-2], v.stack[n-1]
		}
	default:
		return fmt.Errorf("unknown opcode: %s", instr.Op)
	}
	return nil
}

func (v *VM) opAdd() error {
	right := v.pop()
	left := v.pop()
	// Check operator overloading on objects
	if result, handled := v.tryOperatorOverload(left, right, "OPERATOR_ADD"); handled {
		v.push(result)
		return nil
	}
	// String concatenation: if either side is a string, concatenate
	if ls, ok := left.(*advplrt.StringValue); ok {
		v.push(advplrt.NewString(ls.Val + advplrt.ToString(right)))
		return nil
	}
	if rs, ok := right.(*advplrt.StringValue); ok {
		v.push(advplrt.NewString(advplrt.ToString(left) + rs.Val))
		return nil
	}
	v.push(advplrt.NewNumber(advplrt.ToFloat(left) + advplrt.ToFloat(right)))
	return nil
}

func (v *VM) tryOperatorOverload(left, right advplrt.Value, operatorMethod string) (advplrt.Value, bool) {
	o, ok := left.(*advplrt.ObjectValue)
	if !ok {
		return nil, false
	}
	funcName := v.findMethod(o.ClassName, operatorMethod)
	if funcName == "" {
		return nil, false
	}
	// Use the existing callMethod mechanism
	v.push(left)
	v.push(right)
	if err := v.callMethod(operatorMethod, 1); err != nil {
		return advplrt.Nil, true
	}
	// After callMethod sets up the frame, we need to run it
	// But callMethod changes v.current to the method's frame
	// We need to execute the method and get the return value
	// Save current frame and run the method
	savedFrame := v.current
	for v.current != savedFrame && v.current != nil {
		instr := v.current.Code[v.current.IP]
		v.current.IP++
		if err := v.execute(instr); err != nil {
			// Error or return - check if we're back to saved frame
			if v.current == savedFrame {
				break
			}
			// Try/catch might handle it
			if advErr, ok := err.(*advplrt.ErrorValue); ok {
				if v.handleCatch(advErr) {
					continue
				}
			}
			v.push(advplrt.Nil)
			return advplrt.Nil, true
		}
	}
	if len(v.stack) > 0 {
		return v.pop(), true
	}
	return advplrt.Nil, true
}

func (v *VM) opBinary(fn func(a, b float64) float64, operatorMethod string) error {
	right := v.pop()
	left := v.pop()
	if result, handled := v.tryOperatorOverload(left, right, operatorMethod); handled {
		v.push(result)
		return nil
	}
	v.push(advplrt.NewNumber(fn(advplrt.ToFloat(left), advplrt.ToFloat(right))))
	return nil
}

func (v *VM) opComparison(fn func(a, b advplrt.Value) bool, operatorMethod string) error {
	right := v.pop()
	left := v.pop()
	if result, handled := v.tryOperatorOverload(left, right, operatorMethod); handled {
		v.push(result)
		return nil
	}
	v.push(advplrt.NewBool(fn(left, right)))
	return nil
}

func (v *VM) opLogic(isAnd bool) error {
	right := v.pop()
	left := v.pop()
	if isAnd {
		v.push(advplrt.NewBool(left.IsTruthy() && right.IsTruthy()))
	} else {
		v.push(advplrt.NewBool(left.IsTruthy() || right.IsTruthy()))
	}
	return nil
}

func (v *VM) opDollar() error {
	right := v.pop()
	left := v.pop()
	if ls, ok := left.(*advplrt.StringValue); ok {
		if rs, ok := right.(*advplrt.StringValue); ok {
			v.push(advplrt.NewBool(strings.Contains(rs.Val, ls.Val)))
			return nil
		}
	}
	v.push(advplrt.False)
	return nil
}

func (v *VM) opConcat() error {
	right := v.pop()
	left := v.pop()
	v.push(advplrt.NewString(advplrt.ToString(left) + advplrt.ToString(right)))
	return nil
}

func (v *VM) callFunc(name string, argCount int) error {
	info, ok := v.bc.Functions[name]
	if !ok {
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return fmt.Errorf("function %s not found", name)
	}

	args := make([]advplrt.Value, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = v.pop()
	}

	// Reorder named args
	if len(v.namedArgs) > 0 {
		args = v.reorderNamedArgs(args, info.ParamNames)
	}
	v.namedArgs = v.namedArgs[:0]
	v.argCounter = 0

	// Create new frame
	locals := make([]advplrt.Value, info.NumLocals)
	for i := 0; i < len(args) && i < info.NumParams; i++ {
		locals[i] = args[i]
	}

	frame := &CallFrame{
		FuncName:  name,
		Code:      v.bc.Code,
		IP:        info.Offset,
		Locals:    locals,
		StackBase: len(v.stack),
	}
	v.frames = append(v.frames, frame)
	v.current = frame
	return nil
}

func (v *VM) callNative(name string, argCount int) error {
	upperName := strings.ToUpper(name)
	fn, ok := v.natives[upperName]
	if !ok {
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return fmt.Errorf("unknown function: %s", name)
	}

	args := make([]advplrt.Value, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = v.pop()
	}
	v.namedArgs = v.namedArgs[:0]
	v.argCounter = 0

	result, err := fn.Fn(args)
	if err != nil {
		return err
	}
	v.push(result)
	return nil
}

func (v *VM) callMethod(methodName string, argCount int) error {
	args := make([]advplrt.Value, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = v.pop()
	}
	obj := v.pop()

	o, ok := obj.(*advplrt.ObjectValue)
	if !ok {
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return fmt.Errorf("cannot call method %s on non-object (type %T)", methodName, obj)
	}

	// Find method in class hierarchy
	funcName := v.findMethod(o.ClassName, methodName)
	if funcName == "" {
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return v.callNativeMethod(o, methodName, args)
	}

	info, ok := v.bc.Functions[funcName]
	if !ok {
		v.namedArgs = v.namedArgs[:0]
		v.argCounter = 0
		return v.callNativeMethod(o, methodName, args)
	}

	// Reorder named args
	if len(v.namedArgs) > 0 {
		args = v.reorderNamedArgs(args, info.ParamNames)
	}
	v.namedArgs = v.namedArgs[:0]
	v.argCounter = 0

	locals := make([]advplrt.Value, info.NumLocals)
	locals[0] = o // self
	for i := 0; i < argCount && i+1 < info.NumParams; i++ {
		locals[i+1] = args[i]
	}

	frame := &CallFrame{
		FuncName:  funcName,
		Code:      v.bc.Code,
		IP:        info.Offset,
		Locals:    locals,
		StackBase: len(v.stack),
		Self:      o,
	}
	v.frames = append(v.frames, frame)
	v.current = frame
	return nil
}

func (v *VM) reorderNamedArgs(args []advplrt.Value, paramNames []string) []advplrt.Value {
	if len(v.namedArgs) == 0 || len(paramNames) == 0 {
		return args
	}
	// Build result array sized to paramNames
	result := make([]advplrt.Value, len(paramNames))
	for i := range result {
		result[i] = advplrt.Nil
	}
	// Create a set of named arg indices for quick lookup
	namedMap := make(map[int]string) // argIndex -> name
	for _, na := range v.namedArgs {
		namedMap[na.argIndex] = na.name
	}
	// Place each arg
	positionalIdx := 0
	for i, arg := range args {
		if name, isNamed := namedMap[i]; isNamed {
			// Named arg - find its position in paramNames
			upperName := strings.ToUpper(name)
			for j, pname := range paramNames {
				if strings.ToUpper(pname) == upperName {
					result[j] = arg
					break
				}
			}
		} else {
			// Positional arg - place in order, skipping positions used by named args
			for positionalIdx < len(result) {
				result[positionalIdx] = arg
				positionalIdx++
				break
			}
		}
	}
	return result
}

func (v *VM) findMethod(className, methodName string) string {
	upperMethod := strings.ToUpper(methodName)
	// Check user-defined classes in bytecode
	for {
		if cls, ok := v.bc.Classes[className]; ok {
			// Try original case first, then uppercase
			if m, ok := cls.Methods[methodName]; ok {
				return m.Name
			}
			if m, ok := cls.Methods[upperMethod]; ok {
				return m.Name
			}
			className = cls.Parent
		} else {
			break
		}
		if className == "" {
			break
		}
	}
	// Check native classes registered in v.classes
	if cls, ok := v.classes[strings.ToUpper(className)]; ok {
		if m, ok := cls.Methods[upperMethod]; ok {
			return m.Name
		}
	}
	return ""
}

func (v *VM) callNativeMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	upperMethod := strings.ToUpper(method)
	switch obj.ClassName {
	case "ErrorClass":
		return v.callErrorClassMethod(obj, upperMethod, args)
	case "JsonObject":
		return v.callJsonObjectMethod(obj, upperMethod, args)
	case "FWGridProcess":
		return v.callGridProcessMethod(obj, upperMethod, args)
	case "FWMBrowse":
		return v.callFormBrowseMethod(obj, upperMethod, args)
	case "MsDialog":
		return v.callMsDialogMethod(obj, upperMethod, args)
	case "LLM":
		return v.callLLMMethod(obj, upperMethod, args)
	case "MCPServer":
		return v.callMCPServerMethod(obj, upperMethod, args)
	default:
		return fmt.Errorf("unknown method %s on object %s", method, obj.ClassName)
	}
}

// args não é usado (ErrorClass:New() não recebe parâmetro; as propriedades
// Description/GenCode/Severity são setadas via property access, não pelo
// construtor) — mantido por uniformidade com os demais dispatchers.
func (v *VM) callErrorClassMethod(obj *advplrt.ObjectValue, method string, _ []advplrt.Value) error {
	switch method {
	case "NEW":
		v.push(obj)
		return nil
	default:
		return fmt.Errorf("unknown method %s on ErrorClass", method)
	}
}

func (v *VM) callJsonObjectMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	switch method {
	case "TOSTRING", "TOJSON":
		var sb strings.Builder
		sb.WriteString("{")
		first := true
		for k, val := range obj.Props {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("\"%s\": %s", k, jsonValueString(val)))
			first = false
		}
		sb.WriteString("}")
		v.push(advplrt.NewString(sb.String()))
		return nil
	case "HASPROPERTY":
		if len(args) > 0 {
			if s, ok := args[0].(*advplrt.StringValue); ok {
				_, exists := obj.Props[strings.ToUpper(s.Val)]
				v.push(advplrt.NewBool(exists))
				return nil
			}
		}
		v.push(advplrt.False)
		return nil
	case "FROMJSON":
		if len(args) > 0 {
			if s, ok := args[0].(*advplrt.StringValue); ok {
				// Simple JSON parse - just return Nil for now, full parser TODO
				_ = s
				v.push(advplrt.Nil)
				return nil
			}
		}
		v.push(advplrt.Nil)
		return nil
	case "GETNAMES":
		elems := make([]advplrt.Value, 0)
		for k := range obj.Props {
			elems = append(elems, advplrt.NewString(k))
		}
		v.push(advplrt.NewArray(elems))
		return nil
	case "DELNAME":
		if len(args) > 0 {
			if s, ok := args[0].(*advplrt.StringValue); ok {
				key := strings.ToUpper(s.Val)
				if _, exists := obj.Props[key]; exists {
					delete(obj.Props, key)
					v.push(advplrt.True)
					return nil
				}
			}
		}
		v.push(advplrt.False)
		return nil
	case "GETJSONTEXT":
		if len(args) > 0 {
			if s, ok := args[0].(*advplrt.StringValue); ok {
				key := strings.ToUpper(s.Val)
				if val, exists := obj.Props[key]; exists {
					v.push(advplrt.NewString(advplrt.ToString(val)))
					return nil
				}
			}
		}
		v.push(advplrt.NewString("NULL"))
		return nil
	case "NEW":
		v.push(obj)
		return nil
	default:
		return fmt.Errorf("unknown method %s on JsonObject", method)
	}
}

func jsonValueString(val advplrt.Value) string {
	switch v := val.(type) {
	case *advplrt.StringValue:
		return fmt.Sprintf("\"%s\"", v.Val)
	case *advplrt.NumberValue:
		return fmt.Sprintf("%g", v.Val)
	case *advplrt.BoolValue:
		if v.Val {
			return "true"
		}
		return "false"
	case *advplrt.NilValue:
		return "null"
	default:
		return fmt.Sprintf("\"%s\"", advplrt.ToString(val))
	}
}

// args não é usado por design: a convenção deste VM é sempre construir com
// ClassName() vazio e inicializar via :New(args) chamado à parte (ver
// comentário "Don't auto-call constructor" mais abaixo) — os args de
// OP_NEW_INSTANCE são descartados aqui de propósito.
func (v *VM) newInstance(className string, _ []advplrt.Value) error {
	cls, ok := v.classes[className]
	if !ok {
		// Check if it's a known framework class
		upperName := strings.ToUpper(className)
		switch upperName {
		case "JSONOBJECT":
			obj := advplrt.NewObject("JsonObject", nil)
			v.push(obj)
			return nil
		case "FWGRIDPROCESS":
			v.push(newGridObject())
			return nil
		case "FWMBROWSE":
			v.push(newBrowseObject())
			return nil
		case "LLM":
			v.push(newLLMObject())
			return nil
		case "MCPSERVER":
			v.push(newMCPServerObject())
			return nil
		case "ERRORCLASS":
			obj := advplrt.NewObject("ErrorClass", cls)
			obj.Props["DESCRIPTION"] = advplrt.NewString("")
			obj.Props["GENCODE"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWFORMMODEL":
			obj := advplrt.NewObject("FWFormModel", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["_MODELID"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWFORMVIEW":
			obj := advplrt.NewObject("FWFormView", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["_VIEWID"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWFORMBROWSE":
			obj := advplrt.NewObject("FWFormBrowse", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["_BROWSEID"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWWIZARDCONTROL":
			obj := advplrt.NewObject("FWWizardControl", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["TITLE"] = advplrt.NewString("")
			obj.Props["CURRENTSTEP"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWDYNDIALOG":
			obj := advplrt.NewObject("FWDynDialog", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["TITLE"] = advplrt.NewString("")
			v.push(obj)
			return nil
		case "FWPANEL":
			obj := advplrt.NewObject("FWPanel", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			v.push(obj)
			return nil
		case "FWGROUPBOX":
			obj := advplrt.NewObject("FWGroupBox", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["TITLE"] = advplrt.NewString("")
			v.push(obj)
			return nil
		case "FWTABS":
			obj := advplrt.NewObject("FWTabs", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["ACTIVETAB"] = advplrt.NewNumber(0)
			v.push(obj)
			return nil
		case "FWSPLITTER":
			obj := advplrt.NewObject("FWSplitter", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["ORIENTATION"] = advplrt.NewString("")
			v.push(obj)
			return nil
		case "FWTREEVIEW":
			obj := advplrt.NewObject("FWTreeView", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			v.push(obj)
			return nil
		case "FWLISTVIEW":
			obj := advplrt.NewObject("FWListView", cls)
			obj.Props["NAME"] = advplrt.NewString("")
			obj.Props["VIEWSTYLE"] = advplrt.NewString("REPORT")
			v.push(obj)
			return nil
		default:
			return fmt.Errorf("unknown class: %s", className)
		}
	}

	obj := advplrt.NewObject(className, cls)

	// Initialize properties with defaults
	for propName, propType := range cls.Properties {
		obj.Props[strings.ToUpper(propName)] = defaultValueForType(propType)
	}

	// Don't auto-call constructor here - :New() will be called explicitly
	// via CALL_METHOD if the code does Calculator():New()
	v.push(obj)
	return nil
}

func (v *VM) callConstructor(className string, obj *advplrt.ObjectValue, args []advplrt.Value) error {
	funcName := v.findMethod(className, "NEW")
	if funcName == "" {
		return nil
	}
	info := v.bc.Functions[funcName]
	locals := make([]advplrt.Value, info.NumLocals)
	locals[0] = obj
	for i := 0; i < len(args) && i+1 < info.NumParams; i++ {
		locals[i+1] = args[i]
	}
	frame := &CallFrame{
		FuncName:  funcName,
		Code:      v.bc.Code,
		IP:        info.Offset,
		Locals:    locals,
		StackBase: len(v.stack),
		Self:      obj,
	}
	v.frames = append(v.frames, frame)
	v.current = frame
	return nil
}

func defaultValueForType(typeName string) advplrt.Value {
	switch strings.ToUpper(typeName) {
	case "NUMERIC", "INTEGER", "DOUBLE", "DECIMAL", "FLOAT":
		return advplrt.NewNumber(0)
	case "CHARACTER", "CHAR", "STRING":
		return advplrt.NewString("")
	case "LOGICAL", "BOOLEAN":
		return advplrt.False
	case "DATE":
		return advplrt.NewDate(timeZero())
	case "ARRAY":
		return advplrt.NewArray([]advplrt.Value{})
	default:
		return advplrt.Nil
	}
}

func timeZero() time.Time {
	return time.Time{}
}

// MVC registration methods
func (v *VM) registerMVCModel(model *mvc.FWFormModel) int {
	id := v.mvcNextID
	v.mvcNextID++
	v.mvcModels[id] = model
	return id
}

func (v *VM) getMVCModel(id int) *mvc.FWFormModel {
	return v.mvcModels[id]
}

func (v *VM) registerMVCView(view *mvc.FWFormView) int {
	id := v.mvcNextID
	v.mvcNextID++
	v.mvcViews[id] = view
	return id
}

func (v *VM) getMVCView(id int) *mvc.FWFormView {
	return v.mvcViews[id]
}

func (v *VM) registerMVCBrowse(browse *mvc.FWFormBrowse) int {
	id := v.mvcNextID
	v.mvcNextID++
	v.mvcBrowses[id] = browse
	return id
}

func (v *VM) getMVCBrowse(id int) *mvc.FWFormBrowse {
	return v.mvcBrowses[id]
}

func (v *VM) doReturn(val advplrt.Value) error {
	// Pop frame
	oldFrame := v.current
	v.frames = v.frames[:len(v.frames)-1]
	if len(v.frames) > 0 {
		v.current = v.frames[len(v.frames)-1]
	} else {
		v.current = nil
	}
	// Trim stack to frame base
	if len(v.stack) > oldFrame.StackBase {
		v.stack = v.stack[:oldFrame.StackBase]
	}
	v.push(val)
	return nil
}
