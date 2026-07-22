package vm

import (
	"github.com/advpl/compiler/pkg/autograd"
	"github.com/advpl/compiler/pkg/nn"
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tensor"
)

func newVariableObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Variable", nil)
	obj.Native = autograd.NewLeaf(&tensor.Tensor{Shape: []int{0}, Data: []float32{}})
	return obj
}

func wrapVariable(v *autograd.Variable) *advplrt.ObjectValue {
	obj := advplrt.NewObject("Variable", nil)
	obj.Native = v
	return obj
}

// verr rotula um erro de operação de Variable com o prefixo "Variable: ".
func verr(err error) error { return advplrt.NewError("Variable: " + err.Error()) }

// argVariable lê o *autograd.Variable de um argumento que deve ser um objeto Variable.
func argVariable(args []advplrt.Value, i int) (*autograd.Variable, error) {
	o, ok := getArg(args, i).(*advplrt.ObjectValue)
	if !ok {
		return nil, advplrt.NewError("Variable: argumento não é um objeto Variable")
	}
	vv, ok := o.Native.(*autograd.Variable)
	if !ok {
		return nil, advplrt.NewError("Variable: objeto sem estado interno de Variable")
	}
	return vv, nil
}

func (v *VM) callVariableMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	self, _ := obj.Native.(*autograd.Variable)

	switch method {
	case "NEW":
		// New(oTensor): folha a partir de um objeto Tensor do S2
		to, ok := getArg(args, 0).(*advplrt.ObjectValue)
		if !ok {
			return advplrt.NewError("Variable:New requer um objeto Tensor")
		}
		tt, ok := to.Native.(*tensor.Tensor)
		if !ok {
			return advplrt.NewError("Variable:New: objeto não é Tensor")
		}
		obj.Native = autograd.NewLeaf(tt)
		v.push(obj)
	case "FROMARRAY":
		shp := shapeFromArg(getArg(args, 1))
		if err := validShape(shp); err != nil {
			return err
		}
		tt, err := tensor.FromData(floatsFromArg(getArg(args, 0)), shp)
		if err != nil {
			return verr(err)
		}
		obj.Native = autograd.NewLeaf(tt)
		v.push(obj)

	case "MATMUL", "ADD", "MUL", "MSE":
		b, err := argVariable(args, 0)
		if err != nil {
			return err
		}
		var r *autograd.Variable
		switch method {
		case "MATMUL":
			r, err = self.MatMul(b)
		case "ADD":
			r, err = self.Add(b)
		case "MUL":
			r, err = self.Mul(b)
		case "MSE":
			r, err = self.MSE(b)
		}
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "RELU":
		v.push(wrapVariable(self.Relu()))
	case "SUM":
		v.push(wrapVariable(self.Sum()))
	case "MEAN":
		v.push(wrapVariable(self.Mean()))

	case "BACKWARD":
		if err := self.Backward(); err != nil {
			return verr(err)
		}
		v.push(obj)
	case "VALUE":
		v.push(wrapTensor(self.Value))
	case "GRAD":
		if self.Grad == nil {
			v.push(wrapTensor(tensor.New(self.Value.Shape)))
		} else {
			v.push(wrapTensor(self.Grad))
		}

	case "TANH":
		v.push(wrapVariable(self.Tanh()))
	case "SIGMOID":
		v.push(wrapVariable(self.Sigmoid()))
	case "GELU":
		v.push(wrapVariable(self.Gelu()))
	case "INDEXROWS":
		idx := shapeFromArg(getArg(args, 0))
		for i := range idx {
			idx[i]--
		}
		r, err := self.IndexRows(idx)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "SOFTMAXCE":
		tg := shapeFromArg(getArg(args, 0))
		for i := range tg {
			tg[i]--
		}
		r, err := self.SoftmaxCE(tg)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))

	default:
		return advplrt.NewError("Variable: método desconhecido " + method)
	}
	return nil
}

// --- SGD ---

type sgdState struct{ opt *autograd.SGD }

func newSGDObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("SGD", nil)
	obj.Native = &sgdState{}
	return obj
}

func (v *VM) callSGDMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*sgdState)

	switch method {
	case "NEW":
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewError("SGD:New requer um array de Variables")
		}
		params := make([]*autograd.Variable, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			o, ok := e.(*advplrt.ObjectValue)
			if !ok {
				return advplrt.NewError("SGD:New: elemento não é Variable")
			}
			vv, ok := o.Native.(*autograd.Variable)
			if !ok {
				return advplrt.NewError("SGD:New: elemento não é Variable")
			}
			params = append(params, vv)
		}
		lr := float32(advplrt.ToFloat(getArg(args, 1)))
		st.opt = autograd.NewSGD(params, lr)
		v.push(obj)
	case "STEP":
		if st.opt != nil {
			st.opt.Step()
		}
		v.push(obj)
	case "ZEROGRAD":
		if st.opt != nil {
			st.opt.ZeroGrad()
		}
		v.push(obj)
	default:
		return advplrt.NewError("SGD: método desconhecido " + method)
	}
	return nil
}

// --- Adam ---

type adamState struct{ opt *autograd.Adam }

func newAdamObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Adam", nil)
	obj.Native = &adamState{}
	return obj
}

func (v *VM) callAdamMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*adamState)
	switch method {
	case "NEW":
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return advplrt.NewError("Adam:New requer um array de Variables")
		}
		params := make([]*autograd.Variable, 0, len(arr.Elements))
		for _, e := range arr.Elements {
			o, ok := e.(*advplrt.ObjectValue)
			if !ok {
				return advplrt.NewError("Adam:New: elemento não é Variable")
			}
			vv, ok := o.Native.(*autograd.Variable)
			if !ok {
				return advplrt.NewError("Adam:New: elemento não é Variable")
			}
			params = append(params, vv)
		}
		st.opt = autograd.NewAdam(params, float32(advplrt.ToFloat(getArg(args, 1))))
		v.push(obj)
	case "STEP":
		if st.opt != nil {
			st.opt.Step()
		}
		v.push(obj)
	case "ZEROGRAD":
		if st.opt != nil {
			st.opt.ZeroGrad()
		}
		v.push(obj)
	default:
		return advplrt.NewError("Adam: método desconhecido " + method)
	}
	return nil
}

// --- Módulos NN: Linear e Embedding ---

type linearState struct{ m *nn.Linear }

func newLinearObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Linear", nil)
	obj.Native = &linearState{}
	return obj
}

// optScale lê o scale opcional (default 0.1) do i-ésimo argumento.
func optScale(args []advplrt.Value, i int) float32 {
	if num, ok := getArg(args, i).(*advplrt.NumberValue); ok {
		return float32(num.Val)
	}
	return 0.1
}

func paramsArray(ps []*autograd.Variable) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(ps))
	for i, p := range ps {
		el[i] = wrapVariable(p)
	}
	return advplrt.NewArray(el)
}

func (v *VM) callLinearMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*linearState)
	switch method {
	case "NEW":
		nIn := int(advplrt.ToFloat(getArg(args, 0)))
		nOut := int(advplrt.ToFloat(getArg(args, 1)))
		if nIn <= 0 || nOut <= 0 {
			return advplrt.NewError("Linear:New: dimensões devem ser > 0")
		}
		st.m = nn.NewLinear(nIn, nOut, optScale(args, 2))
		v.push(obj)
	case "FORWARD":
		x, err := argVariable(args, 0)
		if err != nil {
			return err
		}
		r, err := st.m.Forward(x)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "PARAMS":
		v.push(paramsArray(st.m.Params()))
	default:
		return advplrt.NewError("Linear: método desconhecido " + method)
	}
	return nil
}

type embeddingState struct{ m *nn.Embedding }

func newEmbeddingObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("Embedding", nil)
	obj.Native = &embeddingState{}
	return obj
}

func (v *VM) callEmbeddingMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, _ := obj.Native.(*embeddingState)
	switch method {
	case "NEW":
		nVocab := int(advplrt.ToFloat(getArg(args, 0)))
		nDim := int(advplrt.ToFloat(getArg(args, 1)))
		if nVocab <= 0 || nDim <= 0 {
			return advplrt.NewError("Embedding:New: dimensões devem ser > 0")
		}
		st.m = nn.NewEmbedding(nVocab, nDim, optScale(args, 2))
		v.push(obj)
	case "FORWARD":
		idx := shapeFromArg(getArg(args, 0))
		for i := range idx {
			idx[i]-- // 1-based (AdvPL) -> 0-based
		}
		r, err := st.m.Forward(idx)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
	case "PARAMS":
		v.push(paramsArray(st.m.Params()))
	default:
		return advplrt.NewError("Embedding: método desconhecido " + method)
	}
	return nil
}
