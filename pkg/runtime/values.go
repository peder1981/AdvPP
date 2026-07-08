package advplrt

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Value interface {
	Type() string
	String() string
	IsTruthy() bool
	Equals(other Value) bool
}

// NilValue
type NilValue struct{}

func (n *NilValue) Type() string   { return "U" }
func (n *NilValue) String() string { return "Nil" }
func (n *NilValue) IsTruthy() bool { return false }
func (n *NilValue) Equals(other Value) bool {
	_, ok := other.(*NilValue)
	return ok
}

var Nil = &NilValue{}

// NumberValue
type NumberValue struct{ Val float64 }

func (n *NumberValue) Type() string { return "N" }
func (n *NumberValue) String() string {
	if n.Val == math.Trunc(n.Val) && !math.IsInf(n.Val, 0) {
		return fmt.Sprintf("%d", int64(n.Val))
	}
	return fmt.Sprintf("%g", n.Val)
}
func (n *NumberValue) IsTruthy() bool { return n.Val != 0 }
func (n *NumberValue) Equals(other Value) bool {
	if o, ok := other.(*NumberValue); ok {
		return n.Val == o.Val
	}
	return false
}

func NewNumber(v float64) *NumberValue { return &NumberValue{Val: v} }

// StringValue
type StringValue struct{ Val string }

func (s *StringValue) Type() string   { return "C" }
func (s *StringValue) String() string { return s.Val }
func (s *StringValue) IsTruthy() bool { return len(s.Val) > 0 }
func (s *StringValue) Equals(other Value) bool {
	if o, ok := other.(*StringValue); ok {
		return s.Val == o.Val
	}
	return false
}

func NewString(s string) *StringValue { return &StringValue{Val: s} }

// BoolValue
type BoolValue struct{ Val bool }

func (b *BoolValue) Type() string { return "L" }
func (b *BoolValue) String() string {
	if b.Val {
		return ".T."
	}
	return ".F."
}
func (b *BoolValue) IsTruthy() bool { return b.Val }
func (b *BoolValue) Equals(other Value) bool {
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
type DateValue struct{ Val time.Time }

func (d *DateValue) Type() string   { return "D" }
func (d *DateValue) String() string { return d.Val.Format("02/01/2006") }
func (d *DateValue) IsTruthy() bool { return !d.Val.IsZero() }
func (d *DateValue) Equals(other Value) bool {
	if o, ok := other.(*DateValue); ok {
		return d.Val.Equal(o.Val)
	}
	return false
}

func NewDate(t time.Time) *DateValue { return &DateValue{Val: t} }

// ArrayValue
type ArrayValue struct{ Elements []Value }

func (a *ArrayValue) Type() string { return "A" }
func (a *ArrayValue) String() string {
	parts := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		parts[i] = e.String()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (a *ArrayValue) IsTruthy() bool { return len(a.Elements) > 0 }
func (a *ArrayValue) Equals(other Value) bool {
	if o, ok := other.(*ArrayValue); ok {
		if len(a.Elements) != len(o.Elements) {
			return false
		}
		for i := range a.Elements {
			if !a.Elements[i].Equals(o.Elements[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func NewArray(elements []Value) *ArrayValue { return &ArrayValue{Elements: elements} }

// CodeBlockValue
type CodeBlockValue struct {
	Params   []string
	Body     interface{}
	Env      *Environment
	FuncName string // for bytecode VM
}

func (c *CodeBlockValue) Type() string   { return "B" }
func (c *CodeBlockValue) String() string { return "{|| ... }" }
func (c *CodeBlockValue) IsTruthy() bool { return true }
func (c *CodeBlockValue) Equals(other Value) bool {
	return c == other
}

// ObjectValue
type ObjectValue struct {
	ClassName string
	Props     map[string]Value
	Class     *ClassDef
}

func (o *ObjectValue) Type() string   { return "O" }
func (o *ObjectValue) String() string { return fmt.Sprintf("Object:%s", o.ClassName) }
func (o *ObjectValue) IsTruthy() bool { return true }
func (o *ObjectValue) Equals(other Value) bool {
	return o == other
}

func NewObject(className string, class *ClassDef) *ObjectValue {
	return &ObjectValue{
		ClassName: className,
		Props:     make(map[string]Value),
		Class:     class,
	}
}

// FunctionValue
type FunctionValue struct {
	Name string
	Fn   func(args []Value) (Value, error)
}

func (f *FunctionValue) Type() string   { return "F" }
func (f *FunctionValue) String() string { return fmt.Sprintf("Function:%s", f.Name) }
func (f *FunctionValue) IsTruthy() bool { return true }
func (f *FunctionValue) Equals(other Value) bool {
	return f == other
}

// ErrorValue
type ErrorValue struct {
	Description string
	Severity    string
	Stack       string
	ClassName   string
	GenCode     int
}

func (e *ErrorValue) Type() string   { return "O" }
func (e *ErrorValue) String() string { return e.Description }
func (e *ErrorValue) IsTruthy() bool { return true }
func (e *ErrorValue) Equals(other Value) bool {
	return e == other
}
func (e *ErrorValue) Error() string { return e.Description }

func NewError(desc string) *ErrorValue {
	return &ErrorValue{Description: desc, Severity: "ERROR", ClassName: "ErrorClass"}
}

// ClassDef
type ClassDef struct {
	Name       string
	Parent     string
	Properties map[string]string // name -> type
	Methods    map[string]*MethodDef
}

type MethodDef struct {
	Name       string
	ClassName  string
	Params     []*ParamDef
	Body       interface{}
	ReturnExpr interface{}
}

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
