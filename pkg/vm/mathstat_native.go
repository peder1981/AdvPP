package vm

import (
	"math"
	"sort"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerMathStatNatives adiciona aritmética faltante e estatística.
func registerMathStatNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// --- aritmética escalar ---
	natives["ATAN2"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Atan2(advplrt.ToFloat(getArg(a, 0)), advplrt.ToFloat(getArg(a, 1)))), nil
	}
	natives["LOG10"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Log10(advplrt.ToFloat(getArg(a, 0)))), nil
	}
	natives["POW"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Pow(advplrt.ToFloat(getArg(a, 0)), advplrt.ToFloat(getArg(a, 1)))), nil
	}
	natives["CEIL"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Ceil(advplrt.ToFloat(getArg(a, 0)))), nil
	}
	natives["SIGN"] = func(a []advplrt.Value) (advplrt.Value, error) {
		x := advplrt.ToFloat(getArg(a, 0))
		s := 0.0
		if x > 0 {
			s = 1
		} else if x < 0 {
			s = -1
		}
		return advplrt.NewNumber(s), nil
	}
	natives["SINH"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Sinh(advplrt.ToFloat(getArg(a, 0)))), nil
	}
	natives["COSH"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Cosh(advplrt.ToFloat(getArg(a, 0)))), nil
	}
	natives["TANH"] = func(a []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(math.Tanh(advplrt.ToFloat(getArg(a, 0)))), nil
	}
	natives["GCD"] = func(a []advplrt.Value) (advplrt.Value, error) {
		x := int64(advplrt.ToFloat(getArg(a, 0)))
		y := int64(advplrt.ToFloat(getArg(a, 1)))
		return advplrt.NewNumber(float64(gcd(abs64(x), abs64(y)))), nil
	}
	natives["LCM"] = func(a []advplrt.Value) (advplrt.Value, error) {
		x := abs64(int64(advplrt.ToFloat(getArg(a, 0))))
		y := abs64(int64(advplrt.ToFloat(getArg(a, 1))))
		if x == 0 || y == 0 {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(x / gcd(x, y) * y)), nil
	}
	natives["FACT"] = func(a []advplrt.Value) (advplrt.Value, error) {
		n := int(advplrt.ToFloat(getArg(a, 0)))
		if n < 0 {
			return advplrt.Nil, advplrt.NewError("Fact: n negativo")
		}
		r := 1.0
		for i := 2; i <= n; i++ {
			r *= float64(i)
		}
		return advplrt.NewNumber(r), nil
	}

	// --- estatística sobre array ---
	natives["MEAN"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := vecFromArg(getArg(a, 0))
		if len(xs) == 0 {
			return advplrt.Nil, advplrt.NewError("Mean: array vazio")
		}
		return advplrt.NewNumber(mean(xs)), nil
	}
	natives["VARIANCE"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := vecFromArg(getArg(a, 0))
		if len(xs) < 2 {
			return advplrt.Nil, advplrt.NewError("Variance: precisa de >=2 elementos")
		}
		return advplrt.NewNumber(variance(xs)), nil
	}
	natives["STDDEV"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := vecFromArg(getArg(a, 0))
		if len(xs) < 2 {
			return advplrt.Nil, advplrt.NewError("StdDev: precisa de >=2 elementos")
		}
		return advplrt.NewNumber(math.Sqrt(variance(xs))), nil
	}
	natives["MEDIAN"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := append([]float64(nil), vecFromArg(getArg(a, 0))...)
		if len(xs) == 0 {
			return advplrt.Nil, advplrt.NewError("Median: array vazio")
		}
		sort.Float64s(xs)
		n := len(xs)
		if n%2 == 1 {
			return advplrt.NewNumber(xs[n/2]), nil
		}
		return advplrt.NewNumber((xs[n/2-1] + xs[n/2]) / 2), nil
	}
	// LinReg(aX, aY) -> {nA, nB} de y = a + b*x (mínimos quadrados).
	natives["LINREG"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := vecFromArg(getArg(a, 0))
		ys := vecFromArg(getArg(a, 1))
		if len(xs) != len(ys) || len(xs) < 2 {
			return advplrt.Nil, advplrt.NewError("LinReg: arrays de tamanhos incompatíveis ou <2")
		}
		n := float64(len(xs))
		var sx, sy, sxy, sxx float64
		for i := range xs {
			sx += xs[i]
			sy += ys[i]
			sxy += xs[i] * ys[i]
			sxx += xs[i] * xs[i]
		}
		den := n*sxx - sx*sx
		if den == 0 {
			return advplrt.Nil, advplrt.NewError("LinReg: x degenerado")
		}
		b := (n*sxy - sx*sy) / den
		aa := (sy - b*sx) / n
		return advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(aa), advplrt.NewNumber(b)}), nil
	}
	// Interp(aX, aY, x) -> interpolação linear (aX crescente).
	natives["INTERP"] = func(a []advplrt.Value) (advplrt.Value, error) {
		xs := vecFromArg(getArg(a, 0))
		ys := vecFromArg(getArg(a, 1))
		x := advplrt.ToFloat(getArg(a, 2))
		if len(xs) != len(ys) || len(xs) < 2 {
			return advplrt.Nil, advplrt.NewError("Interp: arrays incompatíveis ou <2")
		}
		if x <= xs[0] {
			return advplrt.NewNumber(ys[0]), nil
		}
		if x >= xs[len(xs)-1] {
			return advplrt.NewNumber(ys[len(ys)-1]), nil
		}
		for i := 0; i < len(xs)-1; i++ {
			if x >= xs[i] && x <= xs[i+1] {
				t := (x - xs[i]) / (xs[i+1] - xs[i])
				return advplrt.NewNumber(ys[i] + t*(ys[i+1]-ys[i])), nil
			}
		}
		return advplrt.NewNumber(ys[len(ys)-1]), nil
	}
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
func variance(xs []float64) float64 {
	m := mean(xs)
	var s float64
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return s / float64(len(xs)-1) // variância amostral
}
