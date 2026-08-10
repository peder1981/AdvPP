package vm

import (
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Tests das 25 funções novas de Manipulação de String (TDN):
// ANSIToOEM, BitOn, Compress, Decode64, DecodeUTF8, DecodeUTF16, Descend,
// Encode64, EncodeUTF8, EncodeUTF16, GetDToVal, GzStrComp, GzStrDecomp,
// Look4Bit, Match, MathC, MLCount, NotBit, OEMToANSI, Pad, STRICONV,
// StrTokArr2, StuffBit, UnCompress, UnStuff.
//
// Nota de arquitetura: parâmetros por referência (`@cStr`, `@cBufferOut`) não
// são graváveis na VM; as funções que dependem deles devolvem o buffer
// modificado como valor de retorno (forma suportada de uso).

func newStringTestVM(t *testing.T) *VM {
	t.Helper()
	return NewVM(&compiler.Bytecode{}, false)
}

func callStringNative(t *testing.T, v *VM, name string, args []advplrt.Value) advplrt.Value {
	t.Helper()
	fn, ok := v.natives[name]
	if !ok {
		t.Fatalf("native %s não registrada", name)
	}
	got, err := fn.Fn(args)
	if err != nil {
		t.Fatalf("%s retornou erro: %v", name, err)
	}
	return got
}

func strVal(t *testing.T, v advplrt.Value) string {
	t.Helper()
	s, ok := v.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("valor = %T, quer string", v)
	}
	return s.Val
}

func boolVal(t *testing.T, v advplrt.Value) bool {
	t.Helper()
	b, ok := v.(*advplrt.BoolValue)
	if !ok {
		t.Fatalf("valor = %T, quer bool", v)
	}
	return b.Val
}

func numVal(t *testing.T, v advplrt.Value) float64 {
	t.Helper()
	n, ok := v.(*advplrt.NumberValue)
	if !ok {
		t.Fatalf("valor = %T, quer número", v)
	}
	return n.Val
}

func arrStr(t *testing.T, v advplrt.Value) []string {
	t.Helper()
	a, ok := v.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("valor = %T, quer array", v)
	}
	out := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		out[i] = advplrt.ToString(e)
	}
	return out
}

// --- ANSIToOEM / OEMToANSI -------------------------------------------------

func TestANSItoOEMRoundTrip(t *testing.T) {
	v := newStringTestVM(t)
	// "ação cê" em CP1252: ç=0xE7, ã=0xE3, ê=0xEA
	in := "a\xe7\xe3o c\xea"
	oem := strVal(t, callStringNative(t, v, "ANSITOOEM", []advplrt.Value{advplrt.NewString(in)}))
	// CP850: ç=0x87, ã=0xC6, ê=0x88 (codepage OEM pt-BR)
	exp := "a\x87\xc6o c\x88"
	if oem != exp {
		t.Errorf("ANSIToOEM(%q) = %q, quer %q", in, oem, exp)
	}
	back := strVal(t, callStringNative(t, v, "OEMTOANSI", []advplrt.Value{advplrt.NewString(oem)}))
	if back != in {
		t.Errorf("OEMToANSI(ANSIToOEM(%q)) = %q, quer %q", in, back, in)
	}
}

func TestOEMToANSITDN(t *testing.T) {
	v := newStringTestVM(t)
	// Exemplo da TDN: OEM "Ateno" -> ANSI "Atençäo" (ç=0xE7 CP1252)
	got := strVal(t, callStringNative(t, v, "OEMTOANSI", []advplrt.Value{advplrt.NewString("Aten\x87o")}))
	if !strings.HasPrefix(got, "Aten") || !strings.Contains(got, "\xe7") {
		t.Errorf("OEMToANSI(Aten\\x87o) = %q, quer conter ç (0xE7)", got)
	}
}

// --- Encode64 / Decode64 ---------------------------------------------------

func TestEncode64Decode64RoundTrip(t *testing.T) {
	v := newStringTestVM(t)
	in := "à noite, vovô kowalsky vê o ímã cair no pé do pingüim"
	enc := strVal(t, callStringNative(t, v, "ENCODE64", []advplrt.Value{advplrt.NewString(in)}))
	dec := strVal(t, callStringNative(t, v, "DECODE64", []advplrt.Value{advplrt.NewString(enc)}))
	if dec != in {
		t.Errorf("Decode64(Encode64(%q)) = %q", in, dec)
	}
}

func TestEncode64ValoresBase(t *testing.T) {
	v := newStringTestVM(t)
	// "a" -> "YQ==", "ab" -> "YWI=", "abc" -> "YWJj" (RFC 4648)
	for in, want := range map[string]string{
		"a":   "YQ==",
		"ab":  "YWI=",
		"abc": "YWJj",
		"":    "",
	} {
		got := strVal(t, callStringNative(t, v, "ENCODE64", []advplrt.Value{advplrt.NewString(in)}))
		if got != want {
			t.Errorf("Encode64(%q) = %q, quer %q", in, got, want)
		}
	}
}

// --- EncodeUTF8 / DecodeUTF8 ----------------------------------------------

func TestEncodeUTF8DecodeUTF8RoundTrip(t *testing.T) {
	v := newStringTestVM(t)
	in := "à noite, vovô kowalsky vê o ímã"
	enc := strVal(t, callStringNative(t, v, "ENCODEUTF8", []advplrt.Value{advplrt.NewString(in), advplrt.NewString("cp1252")}))
	if !strings.Contains(enc, "ç") && !strings.Contains(enc, "\xc3") {
		t.Errorf("EncodeUTF8(%q) = %q, esperado UTF-8 com acento", in, enc)
	}
	dec := strVal(t, callStringNative(t, v, "DECODEUTF8", []advplrt.Value{advplrt.NewString(enc), advplrt.NewString("cp1252")}))
	if dec != in {
		t.Errorf("DecodeUTF8(EncodeUTF8(%q)) = %q", in, dec)
	}
}

func TestEncodeUTF8DefaultEncoding(t *testing.T) {
	v := newStringTestVM(t)
	// Sem o 2º parâmetro (default cp1252), "é" (0xE9) vira 2 bytes UTF-8 C3 A9
	in := "caf\xe9"
	enc := strVal(t, callStringNative(t, v, "ENCODEUTF8", []advplrt.Value{advplrt.NewString(in)}))
	if !strings.Contains(enc, "\xc3\xa9") {
		t.Errorf("EncodeUTF8(%q) default = %q, esperado C3 A9", in, enc)
	}
}

// --- EncodeUTF16 / DecodeUTF16 --------------------------------------------

func TestEncodeUTF16DecodeUTF16BigEndian(t *testing.T) {
	v := newStringTestVM(t)
	in := "abc\xe9" // é = U+00E9
	enc := strVal(t, callStringNative(t, v, "ENCODEUTF16", []advplrt.Value{advplrt.NewString(in)}))
	// Big-endian (default): cada char 2 bytes, é = 0x00 0xE9
	want := "\x00a\x00b\x00c\x00\xe9"
	if enc != want {
		t.Errorf("EncodeUTF16(%q) BE = %q, quer %q", in, enc, want)
	}
	dec := strVal(t, callStringNative(t, v, "DECODEUTF16", []advplrt.Value{advplrt.NewString(enc)}))
	if dec != in {
		t.Errorf("DecodeUTF16(EncodeUTF16(%q)) = %q", in, dec)
	}
}

func TestEncodeUTF16LittleEndian(t *testing.T) {
	v := newStringTestVM(t)
	enc := strVal(t, callStringNative(t, v, "ENCODEUTF16", []advplrt.Value{advplrt.NewString("a"), advplrt.NewNumber(2)}))
	// Little-endian: 'a' = 0x61 0x00
	if enc != "a\x00" && enc != "\x61\x00" {
		t.Errorf("EncodeUTF16(a, LE) = %q, quer \\x61\\x00", enc)
	}
}

// --- Descend ---------------------------------------------------------------

func TestDescendTdn(t *testing.T) {
	v := newStringTestVM(t)
	// CHR(0) -> CHR(0); demais bytes invertidos (255 - byte)
	got := strVal(t, callStringNative(t, v, "DESCEND", []advplrt.Value{advplrt.NewString("\x00\x01\x7f\xff")}))
	// 0->0, 1->0xFE, 0x7F->0x80, 0xFF->0x00
	want := "\x00\xfe\x80\x00"
	if got != want {
		t.Errorf("Descend(\\x00\\x01\\x7f\\xff) = %q, quer %q", got, want)
	}
}

// --- GetDToVal -------------------------------------------------------------

func TestGetDtoValTdn(t *testing.T) {
	v := newStringTestVM(t)
	cases := map[string]float64{
		"123456":      123456,
		"1/2/3/4/5/6": 123456,
		"fim.123456":  0.123456,
		"teste":       0,
	}
	for in, want := range cases {
		got := numVal(t, callStringNative(t, v, "GETDTOVAL", []advplrt.Value{advplrt.NewString(in)}))
		if got != want {
			t.Errorf("GetDtoVal(%q) = %v, quer %v", in, got, want)
		}
	}
}

// --- Match -----------------------------------------------------------------

func TestMatchTdn(t *testing.T) {
	v := newStringTestVM(t)
	cases := []struct {
		val, mask string
		want      bool
	}{
		{"BAAA", "b*", true},
		{"baaa", "b*a", true},
		{"baaa", "b?a", false},
		{"ba", "b?a", false},
		{"bxa", "b?a", true},
		{"Automatic", "*m?t*i*", true},
		{"qualquer", "", true},
		{"AB", "ab", true},
		{"abc", "a*c", true},
	}
	for _, c := range cases {
		got := boolVal(t, callStringNative(t, v, "MATCH", []advplrt.Value{advplrt.NewString(c.val), advplrt.NewString(c.mask)}))
		if got != c.want {
			t.Errorf("Match(%q, %q) = %v, quer %v", c.val, c.mask, got, c.want)
		}
	}
}

// --- MathC -----------------------------------------------------------------

func TestMathCTdn(t *testing.T) {
	v := newStringTestVM(t)
	cases := []struct{ n1, op, n2, want string }{
		{"10", "/", "2", "5"},
		{"100", "+", "10", "110"},
		{"100", "*", "10", "1000"},
		{"100", "-", "10", "90"},
		{"4", "e", "2", "16"},
	}
	for _, c := range cases {
		got := strVal(t, callStringNative(t, v, "MATHC", []advplrt.Value{advplrt.NewString(c.n1), advplrt.NewString(c.op), advplrt.NewString(c.n2)}))
		if strings.TrimRight(got, ".0") != strings.TrimRight(c.want, ".0") {
			t.Errorf("MathC(%q, %q, %q) = %q, quer %q", c.n1, c.op, c.n2, got, c.want)
		}
	}
}

// --- Pad -------------------------------------------------------------------

func TestPadTdn(t *testing.T) {
	v := newStringTestVM(t)
	got := strVal(t, callStringNative(t, v, "PAD", []advplrt.Value{advplrt.NewString("Light"), advplrt.NewNumber(9)}))
	if got != "Light    " {
		t.Errorf(`Pad("Light",9) = %q, quer "Light    "`, got)
	}
	got = strVal(t, callStringNative(t, v, "PAD", []advplrt.Value{advplrt.NewString("Light"), advplrt.NewNumber(9), advplrt.NewString("@")}))
	if got != "Light@@@@" {
		t.Errorf(`Pad("Light",9,"@") = %q, quer "Light@@@@"`, got)
	}
	got = strVal(t, callStringNative(t, v, "PAD", []advplrt.Value{advplrt.NewString("Light"), advplrt.NewNumber(3)}))
	if got != "Lig" {
		t.Errorf(`Pad("Light",3) = %q, quer "Lig"`, got)
	}
	got = strVal(t, callStringNative(t, v, "PAD", []advplrt.Value{advplrt.NewNumber(123), advplrt.NewNumber(9)}))
	if got != "123      " {
		t.Errorf(`Pad(123,9) = %q, quer "123      "`, got)
	}
	got = strVal(t, callStringNative(t, v, "PAD", []advplrt.Value{advplrt.NewString("Light"), advplrt.NewNumber(0)}))
	if got != "" {
		t.Errorf(`Pad("Light",0) = %q, quer ""`, got)
	}
}

// --- StrTokArr2 ------------------------------------------------------------

func TestStrTokArr2Tdn(t *testing.T) {
	v := newStringTestVM(t)

	got := arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString("A,"), advplrt.NewString(",")}))
	if len(got) != 1 || got[0] != "A" {
		t.Errorf("StrTokArr2('A,',',') = %v, quer {A}", got)
	}

	got = arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString(",B"), advplrt.NewString(",")}))
	if len(got) != 1 || got[0] != "B" {
		t.Errorf("StrTokArr2(',B',',') = %v, quer {B}", got)
	}

	got = arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString(","), advplrt.NewString(",")}))
	if len(got) != 0 {
		t.Errorf("StrTokArr2(',',',') = %v, quer {}", got)
	}

	got = arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString("A,,B"), advplrt.NewString(",")}))
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("StrTokArr2('A,,B',',') = %v, quer {A,B}", got)
	}

	// token é a sequência inteira, não cada caractere
	got = arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString("ABRACADABRA"), advplrt.NewString("BRA")}))
	if len(got) != 2 || got[0] != "A" || got[1] != "CADA" {
		t.Errorf("StrTokArr2('ABRACADABRA','BRA') = %v, quer {A,CADA}", got)
	}

	// lEmptyStr = .T.
	got = arrStr(t, callStringNative(t, v, "STRTOKARR2", []advplrt.Value{advplrt.NewString("A,,B"), advplrt.NewString(","), advplrt.NewBool(true)}))
	if len(got) != 3 || got[0] != "A" || got[1] != "" || got[2] != "B" {
		t.Errorf("StrTokArr2('A,,B',',',.T.) = %v, quer {A,'',B}", got)
	}
}

// --- Compress / UnCompress ------------------------------------------------

func TestCompressUnCompressRoundTrip(t *testing.T) {
	v := newStringTestVM(t)
	in := strings.Repeat("Linha do buffer de Teste ", 200)
	// Assinatura TDN: Compress(@cBufferOut, @nLenghtOut, cBufferIn, nLenghtIn)
	comp := strVal(t, callStringNative(t, v, "COMPRESS", []advplrt.Value{
		advplrt.Nil, advplrt.NewNumber(0), advplrt.NewString(in), advplrt.NewNumber(float64(len(in))),
	}))
	if comp == "" {
		t.Fatal("Compress() retornou string vazia")
	}
	decomp := strVal(t, callStringNative(t, v, "UNCOMPRESS", []advplrt.Value{
		advplrt.Nil, advplrt.NewNumber(0), advplrt.NewString(comp), advplrt.NewNumber(float64(len(comp))),
	}))
	if decomp != in {
		t.Errorf("UnCompress(Compress(%d bytes)) não fez round-trip", len(in))
	}
}

// --- GzStrComp / GzStrDecomp ----------------------------------------------

func TestGzStrCompDecompRoundTrip(t *testing.T) {
	v := newStringTestVM(t)
	in := "Teste da funcao GzStrComp."
	comp := strVal(t, callStringNative(t, v, "GZSTRCOMP", []advplrt.Value{advplrt.NewString(in)}))
	if comp == "" {
		t.Fatal("GzStrComp() retornou string vazia")
	}
	decomp := strVal(t, callStringNative(t, v, "GZSTRDECOMP", []advplrt.Value{advplrt.NewString(comp), advplrt.NewNumber(float64(len(comp)))}))
	if decomp != in {
		t.Errorf("GzStrDecomp(GzStrComp(%q)) = %q", in, decomp)
	}
}

// --- BitOn / Look4Bit / NotBit / StuffBit / UnStuff -----------------------

func TestLook4BitTdn(t *testing.T) {
	v := newStringTestVM(t)
	// chr(240)=11110000 (4 bits), chr(240) (4), chr(10)=00001010 (2), chr(160)=10100000 (2) -> 12
	cStr := "\xf0\xf0\x0a\xa0"
	got := numVal(t, callStringNative(t, v, "LOOK4BIT", []advplrt.Value{
		advplrt.NewString(cStr), advplrt.NewNumber(1), advplrt.NewNumber(32), advplrt.NewNumber(3),
	}))
	if got != 12 {
		t.Errorf("Look4Bit = %v, quer 12", got)
	}
}

func TestBitOnTdn(t *testing.T) {
	v := newStringTestVM(t)
	// chr(0)+chr(15)+chr(255)+chr(255); nStart=1, nTest=12, nLen=3
	cStr := "\x00\x0f\xff\xff"
	got := numVal(t, callStringNative(t, v, "BITON", []advplrt.Value{
		advplrt.NewString(cStr), advplrt.NewNumber(1), advplrt.NewNumber(12), advplrt.NewNumber(3),
	}))
	if got != 1 {
		t.Errorf("BitOn = %v, quer 1", got)
	}
}

func TestNotBitTdn(t *testing.T) {
	v := newStringTestVM(t)
	// chr(255)*4, nLength=4 -> tudo 0 (bytes viram 0x00)
	cStr := "\xff\xff\xff\xff"
	got := strVal(t, callStringNative(t, v, "NOTBIT", []advplrt.Value{advplrt.NewString(cStr), advplrt.NewNumber(4)}))
	if got != "\x00\x00\x00\x00" {
		t.Errorf("NotBit(4x0xFF) = %q, quer 4x0x00", got)
	}
}

func TestStuffBitTdn(t *testing.T) {
	v := newStringTestVM(t)
	// chr(0)*4; nStart=5, nTest=8, nLen=3 -> "00001111111100000000000000000000"
	cStr := "\x00\x00\x00\x00"
	got := strVal(t, callStringNative(t, v, "STUFFBIT", []advplrt.Value{
		advplrt.NewString(cStr), advplrt.NewNumber(5), advplrt.NewNumber(8), advplrt.NewNumber(3),
	}))
	// byte0 = 00001111 = 0x0F, byte1 = 11110000 = 0xF0
	if got != "\x0f\xf0\x00\x00" {
		t.Errorf("StuffBit = %q, quer 0x0F 0xF0 0x00 0x00", got)
	}
}

func TestUnStuffTdn(t *testing.T) {
	v := newStringTestVM(t)
	// chr(255)*4; nStart=5, nTest=8, nLen=3 -> "11110000000011111111111111111111"
	cStr := "\xff\xff\xff\xff"
	got := strVal(t, callStringNative(t, v, "UNSTUFF", []advplrt.Value{
		advplrt.NewString(cStr), advplrt.NewNumber(5), advplrt.NewNumber(8), advplrt.NewNumber(3),
	}))
	// byte0 = 11110000 = 0xF0, byte1 = 00001111 = 0x0F (bits 5-12 zerados)
	if got != "\xf0\x0f\xff\xff" {
		t.Errorf("UnStuff = %q, quer 0xF0 0x0F 0xFF 0xFF", got)
	}
}

// --- STRICONV --------------------------------------------------------------

func TestStrIConv(t *testing.T) {
	v := newStringTestVM(t)
	in := "Era um gato chin\xeas" // "ês" em CP1252: ê=0xEA, s
	got := strVal(t, callStringNative(t, v, "STRICONV", []advplrt.Value{
		advplrt.NewString(in), advplrt.NewString("CP1252"), advplrt.NewString("UTF-8"),
	}))
	if !strings.Contains(got, "\xc3\xaa") {
		t.Errorf("StrIConv(cp1252->utf-8) = %q, esperado ê como C3 AA", got)
	}
}

// --- MLCount ---------------------------------------------------------------

func TestMLCountTdn(t *testing.T) {
	v := newStringTestVM(t)
	cString := "Lorem ipsum dolor sit amet, urna nullafusce vehicula porttitor lobortis "
	cString += "sapien, eget taciti nam tincidunt viverra saepe, eleifend et neque "
	cString += "justonunc adipiscing. Eget eu ut sed est sed accumsan, sit sed ultrices id "
	cString += "scelerisque ullamcorper at, sodales accumsan et per blandit et, enim "
	cString += "porta metus voluptatem luctus wisi, vel nunc tellus pellentesque "
	cString += "tincidunt urn."
	nLin := numVal(t, callStringNative(t, v, "MLCOUNT", []advplrt.Value{advplrt.NewString(cString), advplrt.NewNumber(40)}))
	if nLin != 10 {
		t.Errorf("MLCount(c,40) = %v, quer 10 (sem quebrar palavras)", nLin)
	}
	nLin2 := numVal(t, callStringNative(t, v, "MLCOUNT", []advplrt.Value{advplrt.NewString(cString), advplrt.NewNumber(40), advplrt.NewNumber(4), advplrt.NewBool(false)}))
	if nLin2 != 9 {
		t.Errorf("MLCount(c,40,,.F.) = %v, quer 9 (quebrando palavras)", nLin2)
	}
}
