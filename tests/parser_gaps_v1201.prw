// tests/parser_gaps_v1201.prw
// Fixture de regressão dos gaps de parser/compilador achados numa varredura
// de corpus real (811R4 + Protheus 12.1.2510) após o v1.20.0. Cada bloco
// isola um padrão de linguagem que quebrava antes do fix; ver CHANGELOG.md
// seção [Não lançado] para a causa raiz de cada um.
#include "protheus.ch"

// `ident := valor` como NamedParam (feature v1.19) usado FORA de uma lista
// de argumentos de call — aqui, como um dos ramos de IIF/IF, um idioma
// comum em Clipper de "atribuir e usar o valor atribuído". Antes: crash no
// compilador ("unsupported expression type: *ast.NamedParam").
Function TestNamedParamAsIifBranch()
	Local cVar := ""
	Local lCond := .T.
	Local cResult := IIf(lCond, cVar := "A", cVar := "B")
Return cResult == "A" .And. cVar == "A"

// `ACTIVATE POPUP obj AT nRow, nCol` — cláusula AT de posição, ausente do
// parser de ACTIVATE (que só cobria ON INIT/VALID/CENTERED).
Function TestActivatePopupAt()
	Local oMenu, oDlg
	Menu oMenu PopUp
		MenuItem "x" Action Nil
	EndMenu
	// Sem engine real de UI, só precisa compilar (não executa Activate).
	If .F.
		Activate PopUp oMenu At oDlg:nTop, oDlg:nLeft
	EndIf
Return .T.

// `RELEASE OBJECTS a, b` (plural) — só "RELEASE OBJECT" (singular) era
// reconhecido.
Function TestReleaseObjectsPlural()
	Local oA, oB
Release Objects oA, oB
Return .T.

// `LISTBOX ... FIELDS ALIAS cAlias ...` — variante que referencia uma área
// de trabalho, distinta de `FIELDS "a","b","c"` (lista literal).
Function TestListboxFieldsAlias()
	Local oDlg, oLst, cVar
	@ 0,0 LISTBOX oLst VAR cVar FIELDS ALIAS "SA1" HEADER "a","b" OF oDlg PIXEL
Return .T.

// `DEFINE SECTION ... TABLE "a","b"` — variante no singular de TABLES.
Function TestDefineSectionTableSingular()
	Local oReport, oSec
	DEFINE REPORT oReport NAME "X" TITLE "x"
	DEFINE SECTION oSec OF oReport TITLE "y" TABLE "SA1","SA2"
Return .T.

// `@ ... METER ... BARCOLOR c1,c2` — cláusula não reconhecida (isAtClauseWord
// não incluía BARCOLOR), o que fazia o loop de cláusulas do `@` terminar
// cedo e sobrar "BARCOLOR c1,c2" como statement inválido.
Function TestMeterBarColor()
	Local oDlg, oMeter, nVal
	@ 0,0 METER oMeter VAR nVal TOTAL 10 SIZE 10,10 OF oDlg BARCOLOR 1,2 PIXEL
Return .T.

// `Default alias->campo := valor` — alvo alias->field num Default, tanto
// como primeiro quanto como item subsequente de uma lista separada por
// vírgula (`Private a:=1, M->campo:=2`).
Function TestDefaultAliasArrow()
	Default HttpGet->Page := "1"
	Private M->CAMPO1 := "x", M->CAMPO2 := "y"
Return .T.
