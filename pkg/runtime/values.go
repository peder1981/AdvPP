package advplrt

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Value represents any AdvPL/TLPP runtime value (scalar, array, object, function).
type Value interface {
	Type() string
	String() string
	IsTruthy() bool
	Equals(other Value) bool
}

// NilValue represents nil/empty value.
type NilValue struct{}

func (n *NilValue) Type() string   { return "U" }
func (n *NilValue) String() string { return "Nil" }
func (n *NilValue) IsTruthy() bool { return false }
func (n *NilValue) Equals(other Value) bool {
	// Guard against nil interface (CWE-476: null pointer dereference)
	if other == nil {
		return true  // nil == nil
	}
	_, ok := other.(*NilValue)
	return ok
}

var Nil = &NilValue{}

// NumberValue represents a numeric value (cached for common integers).
type NumberValue struct{ Val float64 }

func (n *NumberValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if n == nil {
		return "U"
	}
	return "N"
}
func (n *NumberValue) String() string {
	// Guard against nil receiver (CWE-476)
	if n == nil {
		return "Nil"
	}
	if n.Val == math.Trunc(n.Val) && !math.IsInf(n.Val, 0) {
		return fmt.Sprintf("%d", int64(n.Val))
	}
	return fmt.Sprintf("%g", n.Val)
}
func (n *NumberValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if n == nil {
		return false
	}
	return n.Val != 0
}
func (n *NumberValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if n == nil || other == nil {
		return n == other
	}
	if o, ok := other.(*NumberValue); ok {
		return n.Val == o.Val
	}
	return false
}

// numberCache pré-aloca *NumberValue para os inteiros pequenos mais comuns
// (contadores de loop, índices de array, resultados de comparação/módulo,
// 0/1 booleanos) — NewNumber devolve o ponteiro compartilhado em vez de
// alocar um novo a cada chamada. Seguro porque NumberValue é imutável
// depois de criado (nada no VM escreve em .Val) e Equals compara por
// VALOR, não por identidade de ponteiro (mesmo padrão já usado por `Nil`,
// o singleton de NilValue, logo acima). Em profile real, cada operação
// aritmética do VM (ADD/SUB/MOD/comparação) alocava um NumberValue novo no
// heap — para um loop com milhões de iterações isso dominava o tempo total
// (mallocgc bem acima de qualquer opcode individual); esta é a otimização
// mais ampla e barata sem mudar a representação de Value (interface
// boxed) — que é a raiz real do custo, mas trocar isso é um projeto à
// parte, não algo pra fazer no meio de uma rodada de perf.
const numberCacheMin = -256
const numberCacheMax = 4096

var numberCache = buildNumberCache()

func buildNumberCache() [numberCacheMax - numberCacheMin + 1]*NumberValue {
	var c [numberCacheMax - numberCacheMin + 1]*NumberValue
	for i := range c {
		c[i] = &NumberValue{Val: float64(i + numberCacheMin)}
	}
	return c
}

func NewNumber(v float64) *NumberValue {
	if v >= numberCacheMin && v <= numberCacheMax {
		if iv := int(v); float64(iv) == v {
			return numberCache[iv-numberCacheMin]
		}
	}
	return &NumberValue{Val: v}
}

// StringValue represents a string literal.
type StringValue struct{ Val string }

func (s *StringValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if s == nil {
		return "U"
	}
	return "C"
}
func (s *StringValue) String() string {
	// Guard against nil receiver (CWE-476)
	if s == nil {
		return "Nil"
	}
	return s.Val
}
func (s *StringValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if s == nil {
		return false
	}
	return len(s.Val) > 0
}
func (s *StringValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if s == nil || other == nil {
		return s == other
	}
	if o, ok := other.(*StringValue); ok {
		return s.Val == o.Val
	}
	return false
}

func NewString(s string) *StringValue { return &StringValue{Val: s} }

// BoolValue represents a boolean value (.T. or .F.).
type BoolValue struct{ Val bool }

func (b *BoolValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if b == nil {
		return "U"
	}
	return "L"
}
func (b *BoolValue) String() string {
	// Guard against nil receiver (CWE-476)
	if b == nil {
		return "Nil"
	}
	if b.Val {
		return ".T."
	}
	return ".F."
}
func (b *BoolValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if b == nil {
		return false
	}
	return b.Val
}
func (b *BoolValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if b == nil || other == nil {
		return b == other
	}
	if o, ok := other.(*BoolValue); ok {
		return b.Val == o.Val
	}
	return false
}

var True = &BoolValue{Val: true}
var False = &BoolValue{Val: false}

func NewBool(b bool) *BoolValue {
	if b {
		return True
	}
	return False
}

// DateValue
// DateValue represents a date/time value (DD/MM/YYYY format).
type DateValue struct{ Val time.Time }

func (d *DateValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if d == nil {
		return "U"
	}
	return "D"
}
func (d *DateValue) String() string {
	// Guard against nil receiver (CWE-476)
	if d == nil {
		return "Nil"
	}
	return d.Val.Format("02/01/2006")
}
func (d *DateValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if d == nil {
		return false
	}
	return !d.Val.IsZero()
}
func (d *DateValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if d == nil || other == nil {
		return d == other
	}
	if o, ok := other.(*DateValue); ok {
		return d.Val.Equal(o.Val)
	}
	return false
}

// NewDate creates a date value from a time.Time.
func NewDate(t time.Time) *DateValue { return &DateValue{Val: t} }

// ArrayValue represents an array/table (zero-indexed collection of values).
type ArrayValue struct{ Elements []Value }

func (a *ArrayValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if a == nil {
		return "U"
	}
	return "A"
}
func (a *ArrayValue) String() string {
	// Guard against nil receiver (CWE-476)
	if a == nil {
		return "Nil"
	}
	parts := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		if e != nil {
			parts[i] = e.String()
		} else {
			parts[i] = "Nil"
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (a *ArrayValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if a == nil {
		return false
	}
	return len(a.Elements) > 0
}
func (a *ArrayValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if a == nil || other == nil {
		return a == other
	}
	if o, ok := other.(*ArrayValue); ok {
		if len(a.Elements) != len(o.Elements) {
			return false
		}
		for i := range a.Elements {
			if a.Elements[i] == nil && o.Elements[i] == nil {
				continue
			}
			if a.Elements[i] == nil || o.Elements[i] == nil {
				return false
			}
			if !a.Elements[i].Equals(o.Elements[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// NewArray creates an array value from a slice of values.
func NewArray(elements []Value) *ArrayValue { return &ArrayValue{Elements: elements} }

// CodeBlockValue represents an executable code block/closure {|| ... }.
type CodeBlockValue struct {
	Params   []string
	Body     interface{}
	Env      *Environment
	FuncName string  // for bytecode VM
	Upvalues []*Value // closures: ponteiros para os slots capturados do frame envolvente
}

func (c *CodeBlockValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if c == nil {
		return "U"
	}
	return "B"
}
func (c *CodeBlockValue) String() string {
	// Guard against nil receiver (CWE-476)
	if c == nil {
		return "Nil"
	}
	return "{|| ... }"
}
func (c *CodeBlockValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if c == nil {
		return false
	}
	return true
}
func (c *CodeBlockValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if c == nil || other == nil {
		return c == other
	}
	return c == other
}

// ObjectValue represents an object instance with properties and methods.
type ObjectValue struct {
	ClassName string
	Props     map[string]Value
	Keys      []string // ordem de inserção das chaves (para GetNames)
	Class     *ClassDef
	Native    interface{} // estado Go de classes de framework nativas (ex.: FWGridProcess)
}

// SetProp grava uma propriedade preservando a ordem de inserção das chaves.
// Guard against nil receiver (CWE-476).
func (o *ObjectValue) SetProp(key string, val Value) {
	if o == nil {
		return  // Cannot set property on nil object
	}
	if _, exists := o.Props[key]; !exists {
		o.Keys = append(o.Keys, key)
	}
	o.Props[key] = val
}

// DelProp remove uma propriedade de Props e de Keys, mantendo os dois em
// sincronia (contraparte de SetProp). Retorna false se a chave não existia.
// Guard against nil receiver (CWE-476).
func (o *ObjectValue) DelProp(key string) bool {
	if o == nil {
		return false  // Cannot delete property from nil object
	}
	if _, exists := o.Props[key]; !exists {
		return false
	}
	delete(o.Props, key)
	for i, k := range o.Keys {
		if k == key {
			o.Keys = append(o.Keys[:i], o.Keys[i+1:]...)
			break
		}
	}
	return true
}

func (o *ObjectValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if o == nil {
		return "U"
	}
	return "O"
}
func (o *ObjectValue) String() string {
	// Guard against nil receiver (CWE-476)
	if o == nil {
		return "Nil"
	}
	return fmt.Sprintf("Object:%s", o.ClassName)
}
func (o *ObjectValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if o == nil {
		return false
	}
	return true
}
func (o *ObjectValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if o == nil || other == nil {
		return o == other
	}
	return o == other
}

func NewObject(className string, class *ClassDef) *ObjectValue {
	return &ObjectValue{
		ClassName: className,
		Props:     make(map[string]Value),
		Class:     class,
	}
}

// FunctionValue represents a callable native function.
type FunctionValue struct {
	Name string
	Fn   func(args []Value) (Value, error)
}

func (f *FunctionValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if f == nil {
		return "U"
	}
	return "F"
}
func (f *FunctionValue) String() string {
	// Guard against nil receiver (CWE-476)
	if f == nil {
		return "Nil"
	}
	return fmt.Sprintf("Function:%s", f.Name)
}
func (f *FunctionValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if f == nil {
		return false
	}
	return true
}
func (f *FunctionValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if f == nil || other == nil {
		return f == other
	}
	return f == other
}

// ErrorValue represents an error/exception with stack trace.
type ErrorValue struct {
	Description string
	Severity    string
	Stack       string
	ClassName   string
	GenCode     int
}

func (e *ErrorValue) Type() string {
	// Guard against nil receiver (CWE-476)
	if e == nil {
		return "U"
	}
	return "O"
}
func (e *ErrorValue) String() string {
	// Guard against nil receiver (CWE-476)
	if e == nil {
		return "Nil"
	}
	return e.Description
}
func (e *ErrorValue) IsTruthy() bool {
	// Guard against nil receiver (CWE-476)
	if e == nil {
		return false
	}
	return true
}
func (e *ErrorValue) Equals(other Value) bool {
	// Guard against nil receiver and parameter (CWE-476)
	if e == nil || other == nil {
		return e == other
	}
	return e == other
}
func (e *ErrorValue) Error() string {
	// Guard against nil receiver (CWE-476)
	if e == nil {
		return "Nil"
	}
	return e.Description
}

// NewError creates an error value with the given description.
func NewError(desc string) *ErrorValue {
	return &ErrorValue{Description: desc, Severity: "ERROR", ClassName: "ErrorClass"}
}

// ClassDef defines a class structure with properties and methods.
type ClassDef struct {
	Name       string
	Parent     string
	Properties map[string]string // name -> type
	Methods    map[string]*MethodDef
}

// MethodDef defines a method within a class.
type MethodDef struct {
	Name       string
	ClassName  string
	Params     []*ParamDef
	Body       interface{}
	ReturnExpr interface{}
}

// ParamDef defines a parameter in a method or function signature.
type ParamDef struct {
	Name string
	Type string
}

// Helper functions

func IsNumber(v Value) bool { _, ok := v.(*NumberValue); return ok }
func IsString(v Value) bool { _, ok := v.(*StringValue); return ok }
func IsBool(v Value) bool   { _, ok := v.(*BoolValue); return ok }
func IsNil(v Value) bool    { _, ok := v.(*NilValue); return ok }
func IsArray(v Value) bool  { _, ok := v.(*ArrayValue); return ok }
func IsObject(v Value) bool { _, ok := v.(*ObjectValue); return ok }

func ToFloat(v Value) float64 {
	switch val := v.(type) {
	case *NumberValue:
		return val.Val
	case *StringValue:
		var f float64
		fmt.Sscanf(val.Val, "%f", &f)
		return f
	case *BoolValue:
		if val.Val {
			return 1
		}
		return 0
	}
	return 0
}

func ToString(v Value) string {
	if v == nil {
		return "Nil"
	}
	return v.String()
}

func ToBool(v Value) bool {
	if v == nil {
		return false
	}
	return v.IsTruthy()
}

func ValType(v Value) string {
	if v == nil {
		return "U"
	}
	return v.Type()
}
