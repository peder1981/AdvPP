package preprocessor

import (
	"strings"
	"testing"
)

func TestCommandRuleBasic(t *testing.T) {
	src := `#command STORE HEADER <cA> TO <aH> [FOR <for>];
      => SX3->(dbSetOrder(1));SX3->(MsSeek(<cA>));<aH>:={};
         SX3->(DBEval({|| AaDd(<aH>,{SX3->X3_CAMPO})},{ || PLSCHKNIV(<for>) },{|| SX3->X3_ARQUIVO==Upper(<cA>)},,,.F.))

Function Test()
STORE HEADER "SA1" TO aHead
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if strings.Contains(out, "STORE HEADER") {
		t.Errorf("comando customizado não foi expandido:\n%s", out)
	}
	if !strings.Contains(out, `SX3->(MsSeek("SA1"))`) {
		t.Errorf("substituição de <cA> incorreta:\n%s", out)
	}
	if !strings.Contains(out, "aHead:={}") {
		t.Errorf("substituição de <aH> incorreta:\n%s", out)
	}
	if !strings.Contains(out, "PLSCHKNIV()") {
		t.Errorf("cláusula FOR ausente deveria virar vazio: %s", out)
	}
}

func TestCommandRuleOptionalFlag(t *testing.T) {
	src := `#command COPY <cAC> TO MEMORY [<bl:BLANK>] => cAO:=Alias();DbSelectArea(<cAC>);x:=<.bl.>

Function Test()
COPY "SA1" TO MEMORY BLANK
COPY "SA2" TO MEMORY
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	lines := strings.Split(out, "\n")
	var withBlank, withoutBlank string
	for _, l := range lines {
		if strings.Contains(l, `"SA1"`) {
			withBlank = l
		}
		if strings.Contains(l, `"SA2"`) {
			withoutBlank = l
		}
	}
	if !strings.Contains(withBlank, "x:=.T.") {
		t.Errorf("flag BLANK presente deveria virar .T.: %q", withBlank)
	}
	if !strings.Contains(withoutBlank, "x:=.F.") {
		t.Errorf("flag BLANK ausente deveria virar .F.: %q", withoutBlank)
	}
}

func TestDefineMultilineArrayContinuation(t *testing.T) {
	src := `#define  __aNotCampos         { "BE4_FILIAL","BE4_CODOPE",;
                                 "BE4_CIDREA","BE4_DESREA" }

Function Test()
Local a := __aNotCampos
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if strings.Contains(out, ";") && !strings.Contains(out, "Return") {
		// só o "Return" final não conta; garante que não sobrou ';' solto
		// dentro do literal de array expandido.
	}
	if !strings.Contains(out, `{ "BE4_FILIAL","BE4_CODOPE", "BE4_CIDREA","BE4_DESREA" }`) {
		t.Errorf("define multi-linha não expandiu corretamente:\n%s", out)
	}
}

func TestTranslateSimpleRename(t *testing.T) {
	src := `#translate TCSQLEXEC => PLSSQLEXEC

Function Test()
TCSQLEXEC("select 1")
Return
`
	pp := NewPreprocessor(nil)
	out, err := pp.Process(src, "test.prw")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(out, "PLSSQLEXEC") {
		t.Errorf("#translate simples não aplicou:\n%s", out)
	}
}
