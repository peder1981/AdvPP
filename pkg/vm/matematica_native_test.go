package vm

import (
	"math"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Test Ceiling - arredondamento para cima
func TestCeiling_PositiveNumber(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["CEILING"].Fn([]advplrt.Value{advplrt.NewNumber(3.14159)})
	if err != nil {
		t.Fatalf("Ceiling retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 4 {
		t.Errorf("Ceiling(3.14159) = %v, quer 4", got)
	}
}

func TestCeiling_NegativeNumber(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["CEILING"].Fn([]advplrt.Value{advplrt.NewNumber(-3.14159)})
	if err != nil {
		t.Fatalf("Ceiling retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != -3 {
		t.Errorf("Ceiling(-3.14159) = %v, quer -3", got)
	}
}

func TestCeiling_Integer(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["CEILING"].Fn([]advplrt.Value{advplrt.NewNumber(5)})
	if err != nil {
		t.Fatalf("Ceiling retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 5 {
		t.Errorf("Ceiling(5) = %v, quer 5", got)
	}
}

// Test Exp - exponencial (e^x)
func TestExp_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["EXP"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("Exp retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.E) > 0.0001 {
		t.Errorf("Exp(1) = %v, quer ~%.4f", got, math.E)
	}
}

func TestExp_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["EXP"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Exp retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 1 {
		t.Errorf("Exp(0) = %v, quer 1", got)
	}
}

func TestExp_Negative(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["EXP"].Fn([]advplrt.Value{advplrt.NewNumber(-1)})
	if err != nil {
		t.Fatalf("Exp retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-1/math.E) > 0.0001 {
		t.Errorf("Exp(-1) = %v, quer ~%.4f", got, 1/math.E)
	}
}

// Test Log - logaritmo natural
func TestLog_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("Log retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Log(1) = %v, quer 0", got)
	}
}

func TestLog_E(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG"].Fn([]advplrt.Value{advplrt.NewNumber(math.E)})
	if err != nil {
		t.Fatalf("Log retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-1) > 0.0001 {
		t.Errorf("Log(e) = %v, quer ~1", got)
	}
}

func TestLog_Ten(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG"].Fn([]advplrt.Value{advplrt.NewNumber(10)})
	if err != nil {
		t.Fatalf("Log retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Log(10)) > 0.0001 {
		t.Errorf("Log(10) = %v, quer ~%.4f", got, math.Log(10))
	}
}

// Test Log10 - logaritmo de base 10
func TestLog10_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG10"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("Log10 retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Log10(1) = %v, quer 0", got)
	}
}

func TestLog10_Ten(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG10"].Fn([]advplrt.Value{advplrt.NewNumber(10)})
	if err != nil {
		t.Fatalf("Log10 retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 1 {
		t.Errorf("Log10(10) = %v, quer 1", got)
	}
}

func TestLog10_Hundred(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["LOG10"].Fn([]advplrt.Value{advplrt.NewNumber(100)})
	if err != nil {
		t.Fatalf("Log10 retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 2 {
		t.Errorf("Log10(100) = %v, quer 2", got)
	}
}

// Test Mod - resto da divisão
func TestMod_Basic(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["MOD"].Fn([]advplrt.Value{advplrt.NewNumber(7), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("Mod retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 1 {
		t.Errorf("Mod(7, 3) = %v, quer 1", got)
	}
}

func TestMod_Exact(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["MOD"].Fn([]advplrt.Value{advplrt.NewNumber(6), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("Mod retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Mod(6, 3) = %v, quer 0", got)
	}
}

func TestMod_DivideByZero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["MOD"].Fn([]advplrt.Value{advplrt.NewNumber(5), advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Mod retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Mod(5, 0) = %v, quer 0", got)
	}
}

func TestMod_Negative(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["MOD"].Fn([]advplrt.Value{advplrt.NewNumber(-7), advplrt.NewNumber(3)})
	if err != nil {
		t.Fatalf("Mod retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-(-1)) > 0.0001 {
		t.Errorf("Mod(-7, 3) = %v, quer ~-1", got)
	}
}

// Test Sqrt - raiz quadrada
func TestSqrt_Four(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SQRT"].Fn([]advplrt.Value{advplrt.NewNumber(4)})
	if err != nil {
		t.Fatalf("Sqrt retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 2 {
		t.Errorf("Sqrt(4) = %v, quer 2", got)
	}
}

func TestSqrt_Two(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SQRT"].Fn([]advplrt.Value{advplrt.NewNumber(2)})
	if err != nil {
		t.Fatalf("Sqrt retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Sqrt(2)) > 0.0001 {
		t.Errorf("Sqrt(2) = %v, quer ~%.4f", got, math.Sqrt(2))
	}
}

func TestSqrt_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SQRT"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Sqrt retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Sqrt(0) = %v, quer 0", got)
	}
}

// Test ACos - arcocosseno
func TestACos_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ACOS"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("ACos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Pi/2) > 0.0001 {
		t.Errorf("ACos(0) = %v, quer ~%.4f (PI/2)", got, math.Pi/2)
	}
}

func TestACos_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ACOS"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("ACos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-0) > 0.0001 {
		t.Errorf("ACos(1) = %v, quer ~0", got)
	}
}

func TestACos_NegativeOne(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ACOS"].Fn([]advplrt.Value{advplrt.NewNumber(-1)})
	if err != nil {
		t.Fatalf("ACos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Pi) > 0.0001 {
		t.Errorf("ACos(-1) = %v, quer ~%.4f (PI)", got, math.Pi)
	}
}

// Test ASin - arcosseno
func TestASin_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ASIN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("ASin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("ASin(0) = %v, quer 0", got)
	}
}

func TestASin_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ASIN"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("ASin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Pi/2) > 0.0001 {
		t.Errorf("ASin(1) = %v, quer ~%.4f (PI/2)", got, math.Pi/2)
	}
}

func TestASin_NegativeOne(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ASIN"].Fn([]advplrt.Value{advplrt.NewNumber(-1)})
	if err != nil {
		t.Fatalf("ASin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-(-math.Pi/2)) > 0.0001 {
		t.Errorf("ASin(-1) = %v, quer ~%.4f (-PI/2)", got, -math.Pi/2)
	}
}

// Test ATan - arcotangente
func TestATan_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ATAN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("ATan retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("ATan(0) = %v, quer 0", got)
	}
}

func TestATan_One(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["ATAN"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("ATan retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-math.Pi/4) > 0.0001 {
		t.Errorf("ATan(1) = %v, quer ~%.4f (PI/4)", got, math.Pi/4)
	}
}

// Test Atn2 - ângulo de seno e cosseno
func TestAtn2_BasicTDNExample(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Atn2(Sin(0), Cos(0)) = 0
	got, err := v.natives["ATN2"].Fn([]advplrt.Value{
		advplrt.NewNumber(math.Sin(0)),
		advplrt.NewNumber(math.Cos(0)),
	})
	if err != nil {
		t.Fatalf("Atn2 retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-0) > 0.0001 {
		t.Errorf("Atn2(Sin(0), Cos(0)) = %v, quer ~0", got)
	}
}

func TestAtn2_PiHalf(t *testing.T) {
	pi := math.Pi
	v := NewVM(&compiler.Bytecode{}, false)
	// Atn2(Sin(PI/2), Cos(PI/2)) = PI/2
	got, err := v.natives["ATN2"].Fn([]advplrt.Value{
		advplrt.NewNumber(math.Sin(pi / 2)),
		advplrt.NewNumber(math.Cos(pi / 2)),
	})
	if err != nil {
		t.Fatalf("Atn2 retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-pi/2) > 0.0001 {
		t.Errorf("Atn2(Sin(PI/2), Cos(PI/2)) = %v, quer ~%.4f", got, pi/2)
	}
}

// Test Cos - cosseno
func TestCos_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["COS"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Cos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 1 {
		t.Errorf("Cos(0) = %v, quer 1", got)
	}
}

func TestCos_PiHalf(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["COS"].Fn([]advplrt.Value{advplrt.NewNumber(math.Pi / 2)})
	if err != nil {
		t.Fatalf("Cos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-0) > 0.0001 {
		t.Errorf("Cos(PI/2) = %v, quer ~0", got)
	}
}

func TestCos_Pi(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["COS"].Fn([]advplrt.Value{advplrt.NewNumber(math.Pi)})
	if err != nil {
		t.Fatalf("Cos retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-(-1)) > 0.0001 {
		t.Errorf("Cos(PI) = %v, quer ~-1", got)
	}
}

// Test Sin - seno
func TestSin_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SIN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Sin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Sin(0) = %v, quer 0", got)
	}
}

func TestSin_PiHalf(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SIN"].Fn([]advplrt.Value{advplrt.NewNumber(math.Pi / 2)})
	if err != nil {
		t.Fatalf("Sin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-1) > 0.0001 {
		t.Errorf("Sin(PI/2) = %v, quer ~1", got)
	}
}

func TestSin_Pi(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["SIN"].Fn([]advplrt.Value{advplrt.NewNumber(math.Pi)})
	if err != nil {
		t.Fatalf("Sin retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-0) > 0.0001 {
		t.Errorf("Sin(PI) = %v, quer ~0", got)
	}
}

// Test Tan - tangente
func TestTan_Zero(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["TAN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("Tan retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || num.Val != 0 {
		t.Errorf("Tan(0) = %v, quer 0", got)
	}
}

func TestTan_PiQuarter(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["TAN"].Fn([]advplrt.Value{advplrt.NewNumber(math.Pi / 4)})
	if err != nil {
		t.Fatalf("Tan retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-1) > 0.0001 {
		t.Errorf("Tan(PI/4) = %v, quer ~1", got)
	}
}

func TestTan_NegativePiQuarter(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["TAN"].Fn([]advplrt.Value{advplrt.NewNumber(-math.Pi / 4)})
	if err != nil {
		t.Fatalf("Tan retornou erro: %v", err)
	}
	num, ok := got.(*advplrt.NumberValue)
	if !ok || math.Abs(num.Val-(-1)) > 0.0001 {
		t.Errorf("Tan(-PI/4) = %v, quer ~-1", got)
	}
}
