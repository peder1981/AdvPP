package vm

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/jsummers/gobmp"
)

// Bin2I — LE int16 signed. TDN: Bin2I("A"+Chr(0)) difere de Bin2I("A") pois o
// segundo byte 0x00 muda o high byte (LE).
func TestBin2I(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		in   string
		want float64
	}{
		{"AB", 16961},               // 0x41, 0x42 -> 0x4241 = 16961
		{"A" + string(rune(0)), 65}, // 0x41, 0x00 -> 0x0041 = 65
		{"A", 65},                   // padding 0x00
	}
	for _, c := range cases {
		got, err := v.natives["BIN2I"].Fn([]advplrt.Value{advplrt.NewString(c.in)})
		if err != nil {
			t.Fatalf("Bin2I(%q) erro: %v", c.in, err)
		}
		if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != c.want {
			t.Errorf("Bin2I(%q) = %v, esperado %v", c.in, advplrt.ToString(got), c.want)
		}
	}
	// sinalizado: 0xFFFF -> -1
	got, err := v.natives["BIN2I"].Fn([]advplrt.Value{advplrt.NewString("\xff\xff")})
	if err != nil {
		t.Fatalf("Bin2I negativo erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != -1 {
		t.Errorf("Bin2I(0xFFFF) = %v, esperado -1", advplrt.ToString(got))
	}
}

// Bin2W — LE uint16. TDN: Bin2W("A"+Chr(0)) -> 65 (high byte 0x00).
func TestBin2W(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["BIN2W"].Fn([]advplrt.Value{advplrt.NewString("AB")})
	if err != nil {
		t.Fatalf("Bin2W erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 16961 {
		t.Errorf("Bin2W('AB') = %v, esperado 16961", advplrt.ToString(got))
	}
	// unsigned: 0xFFFF -> 65535
	got, err = v.natives["BIN2W"].Fn([]advplrt.Value{advplrt.NewString("\xff\xff")})
	if err != nil {
		t.Fatalf("Bin2W unsigned erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 65535 {
		t.Errorf("Bin2W(0xFFFF) = %v, esperado 65535", advplrt.ToString(got))
	}
}

// Bin2L — LE int32 signed.
func TestBin2L(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["BIN2L"].Fn([]advplrt.Value{advplrt.NewString("ABCD")})
	if err != nil {
		t.Fatalf("Bin2L erro: %v", err)
	}
	// "ABCD" = 0x41,0x42,0x43,0x44 -> LE = 0x44434241 = 1145258561
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 1145258561 {
		t.Errorf("Bin2L('ABCD') = %v, esperado 1145258561", advplrt.ToString(got))
	}
}

// Bin2Str — cada bit vira um char: ' ' para 0, 'x' para 1 (MSB primeiro).
func TestBin2Str(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		in   string
		want string
	}{
		{"A", " x     x"},                   // 01000001
		{"AB", " x     x x    x "},          // 01000001 01000010
		{"ABC", " x     x x    x  x    xx"}, // 01000001 01000010 01000011
	}
	for _, c := range cases {
		got, err := v.natives["BIN2STR"].Fn([]advplrt.Value{advplrt.NewString(c.in)})
		if err != nil {
			t.Fatalf("Bin2Str(%q) erro: %v", c.in, err)
		}
		if s, ok := got.(*advplrt.StringValue); !ok || s.Val != c.want {
			t.Errorf("Bin2Str(%q) = %q, esperado %q", c.in, advplrt.ToString(got), c.want)
		}
	}
}

// Round-trips: Bin2D(D2Bin(x)) == x, Bin2F(F2Bin(x)) == x (aprox).
func TestBin2D_D2Bin(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	for _, x := range []float64{12.6, 14.6, 123.456} {
		bin, err := v.natives["D2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(x)})
		if err != nil {
			t.Fatalf("D2Bin(%v) erro: %v", x, err)
		}
		back, err := v.natives["BIN2D"].Fn([]advplrt.Value{bin})
		if err != nil {
			t.Fatalf("Bin2D erro: %v", err)
		}
		n, ok := back.(*advplrt.NumberValue)
		if !ok {
			t.Fatalf("Bin2D retornou %T", back)
		}
		diff := n.Val - x
		if diff < 0 {
			diff = -diff
		}
		if diff > 1e-9 {
			t.Errorf("D2Bin/Bin2D round-trip de %v = %v", x, n.Val)
		}
	}
	// Bin2D deve ler 8 bytes
	got, err := v.natives["D2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("D2Bin(0) erro: %v", err)
	}
	if s, ok := got.(*advplrt.StringValue); !ok || len(s.Val) != 8 {
		t.Errorf("D2Bin(0) len = %v, esperado 8 bytes", advplrt.ToString(got))
	}
}

func TestBin2F_F2Bin(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	for _, x := range []float64{12.6, 14.6, 123.456} {
		bin, err := v.natives["F2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(x)})
		if err != nil {
			t.Fatalf("F2Bin(%v) erro: %v", x, err)
		}
		back, err := v.natives["BIN2F"].Fn([]advplrt.Value{bin})
		if err != nil {
			t.Fatalf("Bin2F erro: %v", err)
		}
		n, ok := back.(*advplrt.NumberValue)
		if !ok {
			t.Fatalf("Bin2F retornou %T", back)
		}
		diff := n.Val - x
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			t.Errorf("F2Bin/Bin2F round-trip de %v = %v", x, n.Val)
		}
	}
	got, err := v.natives["F2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("F2Bin(0) erro: %v", err)
	}
	if s, ok := got.(*advplrt.StringValue); !ok || len(s.Val) != 4 {
		t.Errorf("F2Bin(0) len = %v, esperado 4 bytes", advplrt.ToString(got))
	}
}

// I2Bin — LE int16 (trunca decimais). L2Bin — LE int32.
func TestI2Bin(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		in   float64
		want string
	}{
		{16962, "BB"},   // 0x4242
		{16705, "AA"},   // 0x4141
		{4276803, "CB"}, // 0x414243 -> trunca 0x4243 -> LE "CB"
		{16962.9, "BB"}, // decimais truncados
	}
	for _, c := range cases {
		got, err := v.natives["I2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(c.in)})
		if err != nil {
			t.Fatalf("I2Bin(%v) erro: %v", c.in, err)
		}
		if s, ok := got.(*advplrt.StringValue); !ok || s.Val != c.want {
			t.Errorf("I2Bin(%v) = %q, esperado %q", c.in, advplrt.ToString(got), c.want)
		}
	}
}

func TestL2Bin(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		in   float64
		want string
	}{
		{1094795585, "AAAA"}, // 0x41414141
		{1111638594, "BBBB"}, // 0x42424242
		{1128481603, "CCCC"}, // 0x43434343
		{1145258561, "ABCD"}, // 0x44434241 -> LE bytes "ABCD"
	}
	for _, c := range cases {
		got, err := v.natives["L2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(c.in)})
		if err != nil {
			t.Fatalf("L2Bin(%v) erro: %v", c.in, err)
		}
		if s, ok := got.(*advplrt.StringValue); !ok || s.Val != c.want {
			t.Errorf("L2Bin(%v) = %q, esperado %q", c.in, advplrt.ToString(got), c.want)
		}
	}
}

// W2Bin — LE uint16.
func TestW2Bin(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["W2BIN"].Fn([]advplrt.Value{advplrt.NewNumber(16962)})
	if err != nil {
		t.Fatalf("W2Bin erro: %v", err)
	}
	if s, ok := got.(*advplrt.StringValue); !ok || s.Val != "BB" {
		t.Errorf("W2Bin(16962) = %q, esperado 'BB'", advplrt.ToString(got))
	}
}

// BmpToJpg — converte BMP para JPG via gobmp + image/jpeg; 0 sucesso, -1 erro.
func TestBmpToJpg(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	dir := t.TempDir()
	bmpPath := filepath.Join(dir, "test.bmp")
	// gera um BMP de teste (8x8, vermelho)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(bmpPath)
	if err != nil {
		t.Fatalf("criar BMP: %v", err)
	}
	if err := gobmp.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("gobmp.Encode: %v", err)
	}
	f.Close()

	// sucesso -> 0
	jpgPath := filepath.Join(dir, "out.jpg")
	got, err := v.natives["BMPTOJPG"].Fn([]advplrt.Value{
		advplrt.NewString(bmpPath),
		advplrt.NewString(jpgPath),
	})
	if err != nil {
		t.Fatalf("BmpToJpg erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != 0 {
		t.Errorf("BmpToJpg = %v, esperado 0", advplrt.ToString(got))
	}
	if _, err := os.Stat(jpgPath); err != nil {
		t.Errorf("arquivo JPG não criado: %v", err)
	}

	// arquivo inexistente -> -1
	got, err = v.natives["BMPTOJPG"].Fn([]advplrt.Value{
		advplrt.NewString(filepath.Join(dir, "naoexiste.bmp")),
		advplrt.NewString(filepath.Join(dir, "nao2.jpg")),
	})
	if err != nil {
		t.Fatalf("BmpToJpg arquivo inexistente erro: %v", err)
	}
	if n, ok := got.(*advplrt.NumberValue); !ok || n.Val != -1 {
		t.Errorf("BmpToJpg inexistente = %v, esperado -1", advplrt.ToString(got))
	}
}

// ColorToRGB — formato BGR (0x00BBGGRR), retorna {R,G,B,Alpha}.
func TestColorToRGB(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		n    float64
		want []float64
	}{
		{255, []float64{255, 0, 0, 0}},          // CLR_HRED = RGB(255,0,0)
		{65280, []float64{0, 255, 0, 0}},        // CLR_HGREEN = RGB(0,255,0)
		{16711680, []float64{0, 0, 255, 0}},     // CLR_HBLUE = RGB(0,0,255)
		{16777215, []float64{255, 255, 255, 0}}, // CLR_WHITE
		{0x800000FF, []float64{255, 0, 0, 128}}, // alpha no byte alto
	}
	for _, c := range cases {
		got, err := v.natives["COLORTORGB"].Fn([]advplrt.Value{advplrt.NewNumber(c.n)})
		if err != nil {
			t.Fatalf("ColorToRGB(%v) erro: %v", c.n, err)
		}
		arr, ok := got.(*advplrt.ArrayValue)
		if !ok || len(arr.Elements) != 4 {
			t.Fatalf("ColorToRGB(%v) = %v, esperado vetor de 4", c.n, advplrt.ToString(got))
		}
		for i := 0; i < 4; i++ {
			n, ok := arr.Elements[i].(*advplrt.NumberValue)
			if !ok || n.Val != c.want[i] {
				t.Errorf("ColorToRGB(%v)[%d] = %v, esperado %v", c.n, i+1, advplrt.ToString(arr.Elements[i]), c.want[i])
			}
		}
	}
}

// Dbl2Dt — exemplo TDN: DBL2DT(40544.52426839) -> "20110101 12:34:56.789".
func TestDbl2Dt(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["DBL2DT"].Fn([]advplrt.Value{advplrt.NewNumber(40544.52426839)})
	if err != nil {
		t.Fatalf("Dbl2Dt erro: %v", err)
	}
	if s, ok := got.(*advplrt.StringValue); !ok || s.Val != "20110101 12:34:56.789" {
		t.Errorf("Dbl2Dt(40544.52426839) = %q, esperado '20110101 12:34:56.789'", advplrt.ToString(got))
	}
}

// Dt2Dbl — exemplo TDN: DT2DBL("20101206 00:00:00.001") -> 40518.00000001.
func TestDt2Dbl(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["DT2DBL"].Fn([]advplrt.Value{advplrt.NewString("20101206 00:00:00.001")})
	if err != nil {
		t.Fatalf("Dt2Dbl erro: %v", err)
	}
	n, ok := got.(*advplrt.NumberValue)
	if !ok {
		t.Fatalf("Dt2Dbl retornou %T", got)
	}
	diff := n.Val - 40518.00000001157 // 1ms/86400000 = 1.1574e-8
	if diff < 0 {
		diff = -diff
	}
	if diff > 1e-8 {
		t.Errorf("Dt2Dbl('20101206 00:00:00.001') = %v, esperado ~40518.00000001157", n.Val)
	}
	// round-trip: Dt2Dbl('20110101 12:34:56.789') ~= 40544.52426839
	// (float64 não representa 0.52426839 exatamente; tolerância).
	rt, err := v.natives["DT2DBL"].Fn([]advplrt.Value{
		advplrt.NewString("20110101 12:34:56.789"),
	})
	if err != nil {
		t.Fatalf("Dt2Dbl roundtrip erro: %v", err)
	}
	if n2, ok := rt.(*advplrt.NumberValue); !ok {
		t.Fatalf("Dt2Dbl roundtrip retornou %T", rt)
	} else {
		diff := n2.Val - 40544.52426839
		if diff < 0 {
			diff = -diff
		}
		if diff > 1e-6 {
			t.Errorf("Dt2Dbl('20110101 12:34:56.789') = %v, esperado ~40544.52426839", n2.Val)
		}
	}
}

// GetDtoDate — aceita "021605" (mmddyy) ou "02/16/05".
func TestGetDtoDate(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	for _, in := range []string{"021605", "02/16/05"} {
		got, err := v.natives["GETDTODATE"].Fn([]advplrt.Value{advplrt.NewString(in)})
		if err != nil {
			t.Fatalf("GetDtoDate(%q) erro: %v", in, err)
		}
		dtoc, err := v.natives["DTOC"].Fn([]advplrt.Value{got})
		if err != nil {
			t.Fatalf("DTOC erro: %v", err)
		}
		if s, ok := dtoc.(*advplrt.StringValue); !ok || !strings.HasPrefix(s.Val, "16/02/2005") {
			t.Errorf("GetDtoDate(%q) -> DToC = %q, esperado prefixo 16/02/2005", in, advplrt.ToString(dtoc))
		}
	}
}
