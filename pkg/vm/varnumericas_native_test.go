package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestNAnd(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		args []advplrt.Value
		want float64
	}{
		// Example from TDN: NAnd(65535, 61695, 65520, 65295) => 61440
		{
			"TDN example",
			[]advplrt.Value{
				advplrt.NewNumber(65535),
				advplrt.NewNumber(61695),
				advplrt.NewNumber(65520),
				advplrt.NewNumber(65295),
			},
			61440,
		},
		// Two numbers: NAnd(15, 7) => 7 (1111 AND 0111 = 0111 = 7)
		{
			"two numbers simple",
			[]advplrt.Value{
				advplrt.NewNumber(15),
				advplrt.NewNumber(7),
			},
			7,
		},
		// Single number: NAnd(5) => 5
		{
			"single number",
			[]advplrt.Value{
				advplrt.NewNumber(5),
			},
			5,
		},
		// All zeros: NAnd(0, 0) => 0
		{
			"all zeros",
			[]advplrt.Value{
				advplrt.NewNumber(0),
				advplrt.NewNumber(0),
			},
			0,
		},
		// No bits in common: NAnd(12, 3) => 0 (1100 AND 0011 = 0000 = 0)
		{
			"no bits in common",
			[]advplrt.Value{
				advplrt.NewNumber(12),
				advplrt.NewNumber(3),
			},
			0,
		},
	}

	for _, c := range cases {
		got, err := v.natives["NAND"].Fn(c.args)
		if err != nil {
			t.Fatalf("NAnd(%s) retornou erro: %v", c.name, err)
		}
		n, ok := got.(*advplrt.NumberValue)
		if !ok || n.Val != c.want {
			t.Errorf("NAnd(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestNOr(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		args []advplrt.Value
		want float64
	}{
		// Example from TDN: NOr(15, 240, 3840, 61440) => 65535
		{
			"TDN example",
			[]advplrt.Value{
				advplrt.NewNumber(15),
				advplrt.NewNumber(240),
				advplrt.NewNumber(3840),
				advplrt.NewNumber(61440),
			},
			65535,
		},
		// Two numbers: NOr(12, 3) => 15 (1100 OR 0011 = 1111 = 15)
		{
			"two numbers simple",
			[]advplrt.Value{
				advplrt.NewNumber(12),
				advplrt.NewNumber(3),
			},
			15,
		},
		// Single number: NOr(5) => 5
		{
			"single number",
			[]advplrt.Value{
				advplrt.NewNumber(5),
			},
			5,
		},
		// All zeros: NOr(0, 0) => 0
		{
			"all zeros",
			[]advplrt.Value{
				advplrt.NewNumber(0),
				advplrt.NewNumber(0),
			},
			0,
		},
		// Same numbers: NOr(5, 5) => 5 (0101 OR 0101 = 0101 = 5)
		{
			"same numbers",
			[]advplrt.Value{
				advplrt.NewNumber(5),
				advplrt.NewNumber(5),
			},
			5,
		},
	}

	for _, c := range cases {
		got, err := v.natives["NOR"].Fn(c.args)
		if err != nil {
			t.Fatalf("NOr(%s) retornou erro: %v", c.name, err)
		}
		n, ok := got.(*advplrt.NumberValue)
		if !ok || n.Val != c.want {
			t.Errorf("NOr(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestNXor(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		args []advplrt.Value
		want float64
	}{
		// Example from TDN: NXor(9, 15, 80, 192) => 150
		{
			"TDN example",
			[]advplrt.Value{
				advplrt.NewNumber(9),
				advplrt.NewNumber(15),
				advplrt.NewNumber(80),
				advplrt.NewNumber(192),
			},
			150,
		},
		// Two numbers: NXor(12, 3) => 15 (1100 XOR 0011 = 1111 = 15)
		{
			"two numbers simple",
			[]advplrt.Value{
				advplrt.NewNumber(12),
				advplrt.NewNumber(3),
			},
			15,
		},
		// Single number: NXor(5) => 5
		{
			"single number",
			[]advplrt.Value{
				advplrt.NewNumber(5),
			},
			5,
		},
		// Same numbers: NXor(5, 5) => 0 (0101 XOR 0101 = 0000 = 0)
		{
			"same numbers",
			[]advplrt.Value{
				advplrt.NewNumber(5),
				advplrt.NewNumber(5),
			},
			0,
		},
		// All zeros: NXor(0, 0) => 0
		{
			"all zeros",
			[]advplrt.Value{
				advplrt.NewNumber(0),
				advplrt.NewNumber(0),
			},
			0,
		},
	}

	for _, c := range cases {
		got, err := v.natives["NXOR"].Fn(c.args)
		if err != nil {
			t.Fatalf("NXor(%s) retornou erro: %v", c.name, err)
		}
		n, ok := got.(*advplrt.NumberValue)
		if !ok || n.Val != c.want {
			t.Errorf("NXor(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestRandomize(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name string
		args []advplrt.Value
		// We can't test the exact value, but we can test the range
		minBound float64
		maxBound float64
	}{
		// Randomize(10, 1000) should return value in [10, 999]
		{
			"TDN example range 10-1000",
			[]advplrt.Value{
				advplrt.NewNumber(10),
				advplrt.NewNumber(1000),
			},
			10,
			999, // exclusive upper bound
		},
		// Randomize(1, 34000) should return value in [1, 32766]
		{
			"TDN example range 1-34000",
			[]advplrt.Value{
				advplrt.NewNumber(1),
				advplrt.NewNumber(34000),
			},
			1,
			32766,
		},
		// Randomize(-20000, 25000) should return value in [-20000, 12766]
		{
			"TDN example negative range",
			[]advplrt.Value{
				advplrt.NewNumber(-20000),
				advplrt.NewNumber(25000),
			},
			-20000,
			12766,
		},
		// Randomize(1, 2) should return 1 (as per TDN: "a chamada da função randomize (1,2) sempre retornará 1")
		{
			"single value range",
			[]advplrt.Value{
				advplrt.NewNumber(1),
				advplrt.NewNumber(2),
			},
			1,
			1, // should always return 1
		},
	}

	for _, c := range cases {
		// Test multiple times to ensure consistency
		for i := 0; i < 10; i++ {
			got, err := v.natives["RANDOMIZE"].Fn(c.args)
			if err != nil {
				t.Fatalf("Randomize(%s) retornou erro: %v", c.name, err)
			}
			n, ok := got.(*advplrt.NumberValue)
			if !ok {
				t.Errorf("Randomize(%s) retornou tipo incorreto: %v", c.name, got)
				continue
			}
			// Check bounds: range is [minBound, maxBound] after clamping to 32767-value window
			// (i.e., result in [nMinimo, nMinimo+32766] when upper bound exceeds nMinimo+32767)
			if n.Val < c.minBound || n.Val > c.maxBound {
				t.Errorf("Randomize(%s) = %v, esperado entre [%v, %v]", c.name, n.Val, c.minBound, c.maxBound)
			}
		}
	}
}
