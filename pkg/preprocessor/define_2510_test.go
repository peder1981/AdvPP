package preprocessor

import (
	"strings"
	"testing"
)

// Regressões do corpus Protheus 12.1.2510 (avgeral.prw e afins).

// `#define NOME(p1, p2) corpo` — macro função-like. Antes o nome saía
// como "NOME(" e o corpo era injetado cru em todo identificador.
func TestDefineFuncLikeBasico(t *testing.T) {
	src := `#define MsgStopLM( cMensagem ) ( Iif(Empty(cMensagem), "vazio", cMensagem) )
Function T()
	Local x := MsgStopLM("oi")
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if strings.Contains(out, "MsgStopLM(") {
		t.Errorf("macro função-like não expandida:\n%s", out)
	}
	if !strings.Contains(out, `Iif(Empty("oi"), "vazio", "oi")`) {
		t.Errorf("substituição de parâmetro incorreta:\n%s", out)
	}
}

// Corpo de macro função-like chamando outra macro (expansão iterativa).
func TestDefineFuncLikeAninhado(t *testing.T) {
	src := `#define MsgStopM( c ) c
#define MsgStopLM( cMensagem ) MsgStopM(cMensagem)
Function T()
	Local x := MsgStopLM("oi")
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if strings.Contains(out, "MsgStopLM(") || strings.Contains(out, "MsgStopM(") {
		t.Errorf("expansão aninhada incompleta:\n%s", out)
	}
}

// `#define` multi-linha com ';' como marca de continuação — as linhas
// seguintes entram no valor e os ';' não vazam como separadores.
func TestDefineMultiLinhaPontoVirgula(t *testing.T) {
	src := `#define ARR { 1,;
	2,;
	3 }
Function T()
	Local a := ARR
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, "{ 1, 2, 3 }") {
		t.Errorf("define multi-linha não juntou linhas:\n%s", out)
	}
}

// Comentário `//` no fim do valor do define não pode entrar na expansão.
func TestDefineComentarioFinal(t *testing.T) {
	src := `#define X "1" // comentário
Function T()
	Local a := X
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, `"1"`) || strings.Contains(out, "comentário") {
		t.Errorf("comentário vazou para a expansão:\n%s", out)
	}
}

// `#undef` também remove macro função-like.
func TestUndefRemoveFuncDefine(t *testing.T) {
	src := `#define F( x ) x + 1
#undef F
Function T()
	Local a := F(1)
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, "F(1)") {
		t.Errorf("#undef não removeu macro função-like:\n%s", out)
	}
}

// #define não expande dentro de literais de string — `#define JURIDICA
// "J"` + `Set(JURIDICA,"JURIDICA")` não pode virar `Set("J",""J"")`
// (distribution.convergencia.*.adapter.tlpp do corpus 12.1.2510), e a
// iteração não pode re-expandir o próprio resultado (`#define CCOS
// "CCOS"` + `applyFilter(CCOS)` → `"CCOS"`, não `""""CCOS""""`).
func TestDefineNaoExpandeDentroDeString(t *testing.T) {
	src := `#define JURIDICA "J"
#define CCOS "CCOS"
Function T()
	Local oHash := .F.
	oHash:Set(JURIDICA,"JURIDICA")
	oRequest:applyFilter(CCOS)
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, `Set("J","JURIDICA")`) {
		t.Errorf("literal \"JURIDICA\" foi expandido:\n%s", out)
	}
	if !strings.Contains(out, `applyFilter("CCOS")`) {
		t.Errorf("CCOS não expandiu para \"CCOS\" uma única vez:\n%s", out)
	}
}
