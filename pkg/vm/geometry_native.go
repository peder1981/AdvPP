package vm

import (
	"math"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// vecFromArg lê um array AdvPL de números como []float64 (vetor/ponto).
func vecFromArg(val advplrt.Value) []float64 {
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

func vecToArray(v []float64) *advplrt.ArrayValue {
	el := make([]advplrt.Value, len(v))
	for i, x := range v {
		el[i] = advplrt.NewNumber(x)
	}
	return advplrt.NewArray(el)
}

// registerGeometryNatives adiciona as funções de geometria espacial (vetores 2D/3D).
func registerGeometryNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	natives["VECDOT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, b := vecFromArg(getArg(args, 0)), vecFromArg(getArg(args, 1))
		if len(a) != len(b) {
			return advplrt.Nil, advplrt.NewError("VecDot: dimensões diferentes")
		}
		var s float64
		for i := range a {
			s += a[i] * b[i]
		}
		return advplrt.NewNumber(s), nil
	}
	natives["VECCROSS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, b := vecFromArg(getArg(args, 0)), vecFromArg(getArg(args, 1))
		if len(a) != 3 || len(b) != 3 {
			return advplrt.Nil, advplrt.NewError("VecCross: requer vetores 3D")
		}
		return vecToArray([]float64{
			a[1]*b[2] - a[2]*b[1],
			a[2]*b[0] - a[0]*b[2],
			a[0]*b[1] - a[1]*b[0],
		}), nil
	}
	natives["VECNORM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := vecFromArg(getArg(args, 0))
		var s float64
		for _, x := range a {
			s += x * x
		}
		return advplrt.NewNumber(math.Sqrt(s)), nil
	}
	natives["VECNORMALIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := vecFromArg(getArg(args, 0))
		var s float64
		for _, x := range a {
			s += x * x
		}
		n := math.Sqrt(s)
		if n == 0 {
			return advplrt.Nil, advplrt.NewError("VecNormalize: vetor nulo")
		}
		out := make([]float64, len(a))
		for i, x := range a {
			out[i] = x / n
		}
		return vecToArray(out), nil
	}
	natives["VECDIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, b := vecFromArg(getArg(args, 0)), vecFromArg(getArg(args, 1))
		if len(a) != len(b) {
			return advplrt.Nil, advplrt.NewError("VecDist: dimensões diferentes")
		}
		var s float64
		for i := range a {
			d := a[i] - b[i]
			s += d * d
		}
		return advplrt.NewNumber(math.Sqrt(s)), nil
	}
	natives["VECANGLE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a, b := vecFromArg(getArg(args, 0)), vecFromArg(getArg(args, 1))
		if len(a) != len(b) {
			return advplrt.Nil, advplrt.NewError("VecAngle: dimensões diferentes")
		}
		var dot, na, nb float64
		for i := range a {
			dot += a[i] * b[i]
			na += a[i] * a[i]
			nb += b[i] * b[i]
		}
		den := math.Sqrt(na) * math.Sqrt(nb)
		if den == 0 {
			return advplrt.Nil, advplrt.NewError("VecAngle: vetor nulo")
		}
		c := dot / den
		if c > 1 {
			c = 1
		} else if c < -1 {
			c = -1
		}
		return advplrt.NewNumber(math.Acos(c)), nil
	}
	natives["VECADD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return vecBin(args, func(x, y float64) float64 { return x + y }, "VecAdd")
	}
	natives["VECSUB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return vecBin(args, func(x, y float64) float64 { return x - y }, "VecSub")
	}
	natives["VECSCALE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := vecFromArg(getArg(args, 0))
		s := advplrt.ToFloat(getArg(args, 1))
		out := make([]float64, len(a))
		for i, x := range a {
			out[i] = x * s
		}
		return vecToArray(out), nil
	}
	natives["ROTATEVEC2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := vecFromArg(getArg(args, 0))
		if len(a) != 2 {
			return advplrt.Nil, advplrt.NewError("RotateVec2: requer vetor 2D")
		}
		th := advplrt.ToFloat(getArg(args, 1))
		c, s := math.Cos(th), math.Sin(th)
		return vecToArray([]float64{c*a[0] - s*a[1], s*a[0] + c*a[1]}), nil
	}
	natives["ROTATEVEC3"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := vecFromArg(getArg(args, 0))
		if len(a) != 3 {
			return advplrt.Nil, advplrt.NewError("RotateVec3: requer vetor 3D")
		}
		axis := getArgString(args, 1, "z")
		th := advplrt.ToFloat(getArg(args, 2))
		c, s := math.Cos(th), math.Sin(th)
		x, y, z := a[0], a[1], a[2]
		switch axis {
		case "x", "X":
			return vecToArray([]float64{x, c*y - s*z, s*y + c*z}), nil
		case "y", "Y":
			return vecToArray([]float64{c*x + s*z, y, -s*x + c*z}), nil
		default: // z
			return vecToArray([]float64{c*x - s*y, s*x + c*y, z}), nil
		}
	}
}

func vecBin(args []advplrt.Value, f func(x, y float64) float64, name string) (advplrt.Value, error) {
	a, b := vecFromArg(getArg(args, 0)), vecFromArg(getArg(args, 1))
	if len(a) != len(b) {
		return advplrt.Nil, advplrt.NewError(name + ": dimensões diferentes")
	}
	out := make([]float64, len(a))
	for i := range a {
		out[i] = f(a[i], b[i])
	}
	return vecToArray(out), nil
}
