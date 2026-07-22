package vm

import (
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tensor"
)

func newTensorObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Tensor", nil)
	obj.Native = &tensor.Tensor{Shape: []int{0}, Data: []float32{}}
	return obj
}

func wrapTensor(t *tensor.Tensor) *advplrt.ObjectValue {
	obj := advplrt.NewObject("Tensor", nil)
	obj.Native = t
	return obj
}

// shapeFromArg lê um array AdvPL de inteiros como []int.
func shapeFromArg(val advplrt.Value) []int {
	a, ok := val.(*advplrt.ArrayValue)
	if !ok {
		return nil
	}
	out := make([]int, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = int(advplrt.ToFloat(e))
	}
	return out
}

// floatsFromArg lê um array AdvPL de números como []float32.
func floatsFromArg(val advplrt.Value) []float32 {
	a, ok := val.(*advplrt.ArrayValue)
	if !ok {
		return nil
	}
	out := make([]float32, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = float32(advplrt.ToFloat(e))
	}
	return out
}

// floats64FromArg lê um array AdvPL de números como []float64.
func floats64FromArg(val advplrt.Value) []float64 {
	a, ok := val.(*advplrt.ArrayValue)
	if !ok {
		return nil
	}
	out := make([]float64, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = advplrt.ToFloat(e)
	}
	return out
}

// dtypeFromArg lê "float64"/"float32" de um argumento string (default Float32).
func dtypeFromArg(val advplrt.Value) tensor.DType {
	if s, ok := val.(*advplrt.StringValue); ok && s.Val == "float64" {
		return tensor.Float64
	}
	return tensor.Float32
}

// tensorToAdvplArray serializa os elementos (dtype-neutro) como array AdvPL.
func tensorToAdvplArray(t *tensor.Tensor) *advplrt.ArrayValue {
	el := make([]advplrt.Value, t.Size())
	for i := range el {
		el[i] = advplrt.NewNumber(t.Get(i))
	}
	return advplrt.NewArray(el)
}

func intsToAdvplArray(xs []int) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(xs))
	for i, x := range xs {
		el[i] = advplrt.NewNumber(float64(x))
	}
	return advplrt.NewArray(el)
}

func floatsToAdvplArray(xs []float32) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(xs))
	for i, x := range xs {
		el[i] = advplrt.NewNumber(float64(x))
	}
	return advplrt.NewArray(el)
}

// argTensor lê o *tensor.Tensor de um argumento que deve ser um objeto Tensor.
func argTensor(args []advplrt.Value, i int) (*tensor.Tensor, error) {
	o, ok := getArg(args, i).(*advplrt.ObjectValue)
	if !ok {
		return nil, advplrt.NewError("Tensor: argumento não é um objeto Tensor")
	}
	t, ok := o.Native.(*tensor.Tensor)
	if !ok {
		return nil, advplrt.NewError("Tensor: objeto sem estado interno de tensor")
	}
	return t, nil
}

// terr converte um erro de kernel num ErrorValue catchável.
func terr(err error) error { return advplrt.NewError("Tensor: " + err.Error()) }

// validShape rejeita formas com dimensão negativa, evitando panic em make()
// dentro dos kernels de tensor.New/tensor.Rand.
func validShape(shape []int) error {
	for _, d := range shape {
		if d < 0 {
			return fmt.Errorf("forma com dimensão negativa: %v", shape)
		}
	}
	return nil
}

// axisArg lê nAxis (1-based) e devolve 0-based, e se foi informado.
func axisArg(args []advplrt.Value, i int) (axis int, given bool) {
	if i >= len(args) {
		return 0, false
	}
	if _, ok := getArg(args, i).(*advplrt.NumberValue); !ok {
		return 0, false
	}
	return int(advplrt.ToFloat(getArg(args, i))) - 1, true
}

func (v *VM) callTensorMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	t, _ := obj.Native.(*tensor.Tensor)

	switch method {
	case "NEW":
		shape := shapeFromArg(getArg(args, 0))
		if err := validShape(shape); err != nil {
			return terr(err)
		}
		obj.Native = tensor.NewDType(shape, dtypeFromArg(getArg(args, 1)))
		v.push(obj)
	case "FROMARRAY":
		shape := shapeFromArg(getArg(args, 1))
		if err := validShape(shape); err != nil {
			return terr(err)
		}
		var nt *tensor.Tensor
		var err error
		if dtypeFromArg(getArg(args, 2)) == tensor.Float64 {
			nt, err = tensor.FromData64(floats64FromArg(getArg(args, 0)), shape)
		} else {
			nt, err = tensor.FromData(floatsFromArg(getArg(args, 0)), shape)
		}
		if err != nil {
			return terr(err)
		}
		obj.Native = nt
		v.push(obj)
	case "RAND":
		shape := shapeFromArg(getArg(args, 0))
		if err := validShape(shape); err != nil {
			return terr(err)
		}
		scale := float32(1)
		if _, ok := getArg(args, 1).(*advplrt.NumberValue); ok {
			scale = float32(advplrt.ToFloat(getArg(args, 1)))
		}
		obj.Native = tensor.Rand(shape, scale)
		v.push(obj)

	case "SHAPE":
		v.push(intsToAdvplArray(t.Shape))
	case "SIZE":
		v.push(advplrt.NewNumber(float64(t.Size())))
	case "TOARRAY":
		v.push(tensorToAdvplArray(t))
	case "DTYPE":
		v.push(advplrt.NewString(t.DType.String()))
	case "TOFLOAT64":
		v.push(wrapTensor(t.AsDType(tensor.Float64)))
	case "TOFLOAT32":
		v.push(wrapTensor(t.AsDType(tensor.Float32)))
	case "DOT":
		b, err := argTensor(args, 0)
		if err != nil {
			return err
		}
		d, err := t.Dot(b)
		if err != nil {
			return terr(err)
		}
		v.push(advplrt.NewNumber(d))
	case "NORM":
		v.push(advplrt.NewNumber(t.Norm()))
	case "GET":
		val, err := t.At(idxFromArg(getArg(args, 0)))
		if err != nil {
			return terr(err)
		}
		v.push(advplrt.NewNumber(float64(val)))
	case "SET":
		if err := t.SetAt(idxFromArg(getArg(args, 0)), float32(advplrt.ToFloat(getArg(args, 1)))); err != nil {
			return terr(err)
		}
		v.push(obj)

	case "ADD", "SUB", "MUL", "DIV":
		b, err := argTensor(args, 0)
		if err != nil {
			return err
		}
		var r *tensor.Tensor
		switch method {
		case "ADD":
			r, err = t.Add(b)
		case "SUB":
			r, err = t.Sub(b)
		case "MUL":
			r, err = t.Mul(b)
		case "DIV":
			r, err = t.Div(b)
		}
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "ADDSCALAR":
		v.push(wrapTensor(t.AddScalar(float32(advplrt.ToFloat(getArg(args, 0))))))
	case "MULSCALAR":
		v.push(wrapTensor(t.MulScalar(float32(advplrt.ToFloat(getArg(args, 0))))))

	case "MATMUL":
		b, err := argTensor(args, 0)
		if err != nil {
			return err
		}
		r, err := t.MatMul(b)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "TRANSPOSE":
		r, err := t.Transpose()
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))
	case "RESHAPE":
		r, err := t.Reshape(shapeFromArg(getArg(args, 0)))
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "SUM", "MEAN", "MAX", "ARGMAX":
		axis, given := axisArg(args, 0)
		if !given {
			if t.Size() == 0 {
				return terr(fmt.Errorf("tensor vazio"))
			}
			switch method {
			case "SUM":
				v.push(advplrt.NewNumber(float64(t.SumAll())))
			case "MEAN":
				v.push(advplrt.NewNumber(float64(t.MeanAll())))
			case "MAX":
				v.push(advplrt.NewNumber(float64(t.MaxAll())))
			case "ARGMAX":
				v.push(advplrt.NewNumber(float64(t.ArgmaxAll() + 1))) // 1-based
			}
			return nil
		}
		var r *tensor.Tensor
		var err error
		switch method {
		case "SUM":
			r, err = t.SumAxis(axis)
		case "MEAN":
			r, err = t.MeanAxis(axis)
		case "MAX":
			r, err = t.MaxAxis(axis)
		case "ARGMAX":
			r, err = t.ArgmaxAxis(axis)
			if err == nil { // 1-based na saída
				r = r.AddScalar(1)
			}
		}
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "EXP":
		v.push(wrapTensor(t.Exp()))
	case "LOG":
		v.push(wrapTensor(t.Log()))
	case "SQRT":
		v.push(wrapTensor(t.Sqrt()))
	case "RELU":
		v.push(wrapTensor(t.Relu()))
	case "TANH":
		v.push(wrapTensor(t.Tanh()))
	case "SIGMOID":
		v.push(wrapTensor(t.Sigmoid()))
	case "GELU":
		v.push(wrapTensor(t.Gelu()))

	case "SOFTMAX":
		if t.Size() == 0 {
			return terr(fmt.Errorf("tensor vazio"))
		}
		axis, given := axisArg(args, 0)
		if !given {
			axis = len(t.Shape) - 1 // última dim, 0-based
		}
		r, err := t.Softmax(axis)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	case "INDEXROWS":
		idx := shapeFromArg(getArg(args, 0)) // reusa leitura de ints
		for i := range idx {
			idx[i]-- // 1-based -> 0-based
		}
		r, err := t.IndexRows(idx)
		if err != nil {
			return terr(err)
		}
		v.push(wrapTensor(r))

	default:
		return advplrt.NewError("Tensor: método desconhecido " + method)
	}
	return nil
}

// idxFromArg lê um índice multi-dim AdvPL (1-based) como []int 0-based.
func idxFromArg(val advplrt.Value) []int {
	xs := shapeFromArg(val)
	for i := range xs {
		xs[i]--
	}
	return xs
}
