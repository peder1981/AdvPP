package compiler

type Opcode int

const (
	OP_NIL Opcode = iota
	OP_TRUE
	OP_FALSE
	OP_NUMBER
	OP_STRING
	OP_DATE

	OP_LOAD_LOCAL
	OP_STORE_LOCAL
	OP_LOAD_GLOBAL
	OP_STORE_GLOBAL
	OP_LOAD_SELF
	OP_STORE_SELF

	OP_NEW_ARRAY
	OP_ARRAY_GET
	OP_ARRAY_SET
	OP_ARRAY_LEN

	OP_NEW_OBJECT
	OP_GET_PROP
	OP_SET_PROP
	OP_CALL_METHOD
	OP_NEW_INSTANCE

	OP_CALL_FUNC
	OP_CALL_NATIVE
	OP_RETURN
	OP_RETURN_VALUE
	OP_POP

	OP_JUMP
	OP_JUMP_IF_FALSE
	OP_JUMP_IF_TRUE

	OP_ADD
	OP_SUB
	OP_MUL
	OP_DIV
	OP_MOD
	OP_POW
	OP_NEG
	OP_EQ
	OP_NEQ
	OP_LT
	OP_GT
	OP_LTE
	OP_GTE
	OP_AND
	OP_OR
	OP_NOT
	OP_DOLLAR
	OP_CONCAT

	OP_NEW_CODEBLOCK
	OP_EVAL_CODEBLOCK

	OP_TRY_BEGIN
	OP_TRY_END
	OP_THROW
	OP_CATCH

	OP_DB_SELECT
	OP_DB_SEEK
	OP_DB_SKIP
	OP_DB_GOTOP
	OP_DB_GOBOTTOM
	OP_EOF
	OP_BOF
	OP_FIELD_GET
	OP_FIELD_PUT
	OP_REC_LOCK
	OP_MS_UNLOCK

	OP_MVC_NEW_MODEL
	OP_MVC_NEW_VIEW
	OP_MVC_NEW_BROWSE
	OP_MVC_ADD_FIELD
	OP_MVC_ADD_COMPONENT
	OP_MVC_ADD_COLUMN
	OP_MVC_SET_PROPERTY
	OP_MVC_GET_PROPERTY
	OP_MVC_VALIDATE
	OP_MVC_SHOW

	OP_MACRO
	OP_HALT
	OP_DUP
	OP_SWAP

	OP_JUMP_IF_FALSE_OR_POP
	OP_POP_AND_JUMP

	OP_NAMED_ARG

	// OP_FORLOOP_CMP: compara [var, end, step] no topo da pilha e empilha o
	// resultado da condicao de continuacao do For: step>=0 ? var<=end : var>=end.
	// Permite For descendente (Step negativo) sem fixar o operador em tempo de compilacao.
	OP_FORLOOP_CMP

	// Closures: acessam variaveis capturadas do frame envolvente (upvalues).
	// Arg = indice na lista Upvalues do CodeBlockValue (self = Locals[0]).
	OP_LOAD_UPVAL
	OP_STORE_UPVAL
)

var opcodeNames = map[Opcode]string{
	OP_NIL: "NIL", OP_TRUE: "TRUE", OP_FALSE: "FALSE",
	OP_NUMBER: "NUMBER", OP_STRING: "STRING", OP_DATE: "DATE",
	OP_LOAD_LOCAL: "LOAD_LOCAL", OP_STORE_LOCAL: "STORE_LOCAL",
	OP_LOAD_GLOBAL: "LOAD_GLOBAL", OP_STORE_GLOBAL: "STORE_GLOBAL",
	OP_LOAD_SELF: "LOAD_SELF", OP_STORE_SELF: "STORE_SELF",
	OP_NEW_ARRAY: "NEW_ARRAY", OP_ARRAY_GET: "ARRAY_GET",
	OP_ARRAY_SET: "ARRAY_SET", OP_ARRAY_LEN: "ARRAY_LEN",
	OP_NEW_OBJECT: "NEW_OBJECT", OP_GET_PROP: "GET_PROP",
	OP_SET_PROP: "SET_PROP", OP_CALL_METHOD: "CALL_METHOD",
	OP_NEW_INSTANCE: "NEW_INSTANCE",
	OP_CALL_FUNC:    "CALL_FUNC", OP_CALL_NATIVE: "CALL_NATIVE",
	OP_RETURN: "RETURN", OP_RETURN_VALUE: "RETURN_VALUE", OP_POP: "POP",
	OP_JUMP: "JUMP", OP_JUMP_IF_FALSE: "JUMP_IF_FALSE", OP_JUMP_IF_TRUE: "JUMP_IF_TRUE",
	OP_ADD: "ADD", OP_SUB: "SUB", OP_MUL: "MUL", OP_DIV: "DIV", OP_MOD: "MOD", OP_POW: "POW",
	OP_NEG: "NEG", OP_EQ: "EQ", OP_NEQ: "NEQ", OP_LT: "LT", OP_GT: "GT",
	OP_LTE: "LTE", OP_GTE: "GTE", OP_AND: "AND", OP_OR: "OR", OP_NOT: "NOT",
	OP_DOLLAR: "DOLLAR", OP_CONCAT: "CONCAT",
	OP_NEW_CODEBLOCK: "NEW_CODEBLOCK", OP_EVAL_CODEBLOCK: "EVAL_CODEBLOCK",
	OP_TRY_BEGIN: "TRY_BEGIN", OP_TRY_END: "TRY_END",
	OP_THROW: "THROW", OP_CATCH: "CATCH",
	OP_HALT: "HALT", OP_DUP: "DUP", OP_SWAP: "SWAP",
	OP_JUMP_IF_FALSE_OR_POP: "JUMP_IF_FALSE_OR_POP",
	OP_POP_AND_JUMP:         "POP_AND_JUMP",
	OP_NAMED_ARG:            "NAMED_ARG",
	OP_MVC_NEW_MODEL:        "MVC_NEW_MODEL",
	OP_MVC_NEW_VIEW:         "MVC_NEW_VIEW",
	OP_MVC_NEW_BROWSE:       "MVC_NEW_BROWSE",
	OP_MVC_ADD_FIELD:        "MVC_ADD_FIELD",
	OP_MVC_ADD_COMPONENT:    "MVC_ADD_COMPONENT",
	OP_MVC_ADD_COLUMN:       "MVC_ADD_COLUMN",
	OP_MVC_SET_PROPERTY:     "MVC_SET_PROPERTY",
	OP_MVC_GET_PROPERTY:     "MVC_GET_PROPERTY",
	OP_MVC_VALIDATE:         "MVC_VALIDATE",
	OP_MVC_SHOW:             "MVC_SHOW",
	OP_FORLOOP_CMP:          "FORLOOP_CMP",
	OP_LOAD_UPVAL:           "LOAD_UPVAL",
	OP_STORE_UPVAL:          "STORE_UPVAL",
}

func (op Opcode) String() string {
	if name, ok := opcodeNames[op]; ok {
		return name
	}
	return "UNKNOWN"
}

// Instruction is a single bytecode instruction
type Instruction struct {
	Op   Opcode
	Arg  int    // index into constant pool, local slot, jump target, or function index
	Arg2 int    // secondary argument (e.g. arg count)
	Str  string // string argument (for function names, property names)
	Line int    // source line for debug info
}

// Constant represents a value in the constant pool
type Constant struct {
	Type string // "number", "string", "date"
	Num  float64
	Str  string
}

// FunctionInfo describes a compiled function
type FunctionInfo struct {
	Name        string
	NumParams   int
	NumLocals   int
	IsUser      bool
	IsStatic    bool
	IsNative    bool
	Offset      int // bytecode offset
	ParamNames  []string
	LocalNames  map[string]int // nome da local → slot (writeback do @ GET web)
	Annotations []AnnotationInfo
	Upvals      []UpvalDesc // closures: origem de cada upvalue capturado por este codeblock
}

// UpvalDesc descreve a origem de um upvalue capturado por um codeblock (closure).
type UpvalDesc struct {
	Kind  uint8 // UpvalLocal (slot do frame envolvente) ou UpvalParent (upvalue do bloco pai)
	Index int
}

const (
	UpvalLocal  uint8 = 0 // captura frame.Locals[Index]
	UpvalParent uint8 = 1 // captura parentBlock.Upvalues[Index]
)

// AnnotationInfo stores annotation metadata
type AnnotationInfo struct {
	Name  string
	Value string
}

// ClassInfo describes a compiled class
type ClassInfo struct {
	Name        string
	Parent      string
	Properties  map[string]string // name -> type
	Methods     map[string]*FunctionInfo
	Annotations []AnnotationInfo
}

// Bytecode is the compiled output
type Bytecode struct {
	Constants  []Constant
	Functions  map[string]*FunctionInfo
	Classes    map[string]*ClassInfo
	Code       []Instruction
	MainOffset int
	NumGlobals int
}
