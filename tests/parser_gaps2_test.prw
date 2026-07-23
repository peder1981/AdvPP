#include "totvs.ch"

// Fixture de regressao para gaps de linguagem achados numa rodada de
// varredura contra corpus real (811R4 + Protheus 12.1.2510). Cada função
// isola um padrão que já quebrou o parser/compilador antes do fix
// correspondente. So precisa compilar (`advplc check`) sem erro — o
// conteúdo semântico das expressões não importa.

// NamedParam usado fora de compileArgs (ramo do IIf/If de 3 args de
// curto-circuito) — `ident := expr` como argumento é atribuição real, não
// parâmetro nomeado.
Function GapNamedParamInIif()
	Local lConfirma := .F.
	Local nOpcDlg := 0
	Local oDlg
	Local bAction := {{"BTN", {|| nOpcDlg := 1, IIf(lConfirma := HS_ObrgPer(1, 2), oDlg:End(), nOpcDlg := 0)}}}
Return Nil

Static Function HS_ObrgPer(a, b)
Return .T.

// Atribuição usada como condição em ElseIf/Case/While (não só If).
Function GapAssignInConditions()
	Local x := 0
	Local lRet := 0
	If x == 1
		lRet := 1
	ElseIf lRet := (x == 2)
		lRet := 2
	EndIf

	Do Case
	Case x == 0
		lRet := 3
	Case lRet := (x == 1)
		lRet := 4
	EndCase

	While lRet := (x < 3)
		x++
	EndDo
Return Nil

// `@ident := expr` sem coordenadas — o `@` é só um marcador tolerado.
Function GapAtAssignNoCoord()
	Local cPicture := ""
	@cPicture := "999" + cPicture
Return Nil

// ACTION/MENUITEM com lista de expressões separadas por vírgula, incluindo
// atribuição.
Function GapActionCommaList()
	Local oMenu, cCargoAtu
	MENU oMenu POPUP
		MENUITEM "x" ACTION cCargoAtu := DummyInc(), DummyRefresh()
	ENDMENU
Return Nil

Static Function DummyInc()
Return 1

Static Function DummyRefresh()
Return Nil

// DEFINE SBUTTON com variável-alvo depois de FROM (não logo após o kind).
Function GapDefineLateTarget()
	Local oButton, oDlg
	DEFINE SBUTTON FROM 250,100 oButton TYPE 1 ACTION cProd := Space(15) OF oDlg ENABLE
Return Nil

// Atribuição composta (+=) como lado direito de outra atribuição.
Function GapCompoundAssignAsRHS()
	Local nIniDim := 0
	Local aCol := {1, 2, 3}
	nIniDim := aCol[1] += 5
Return Nil

// Coordenada do `@` como atribuição composta.
Function GapAtCompoundCoord()
	Local nLin := 1
	Local nCo1 := 1
	@ nLin, nCo1 SAY "x"
	@ nLin+=1, nCo1 SAY "y"
Return Nil

// SET FILTER TO vazio seguido de DEFINE com continuação `;` — heurística de
// valor do SET não pode vazar pro statement seguinte.
Function GapSetFilterToThenDefine()
	Local oDlg, oMainWnd
	Set Filter To
	DEFINE MSDIALOG oDlg TITLE "x" ;
		FROM 1,2 TO 3,4 OF oMainWnd PIXEL
Return Nil

// DEFINE SECTION com TABLE (lista de tabelas) e BREAK como flag (PAGE/CELL/
// LINE BREAK), não cláusula com valor.
Function GapDefineSectionTableBreak()
	Local oReport, oSection1, oSection3
	DEFINE SECTION oSection1 OF oReport TITLE "x" LINE STYLE TABLES "QE6" PAGE BREAK
	DEFINE CELL NAME "c1" OF oSection1 SIZE 4 CELL BREAK
	DEFINE SECTION oSection3 OF oReport TITLE "q" TABLES "QE7","QE8"
Return Nil

// REDEFINE com VAR/ID/DIALOGS/HEAD/FIELDS/SIZES/ITEMS.
Function GapRedefineClauses()
	Local oObj, oPages, oDlg, nType, nIcon, aItems := {}
	REDEFINE PAGES oPages ID 101 OF oDlg DIALOGS "P1","P2"
	REDEFINE RADIO oObj VAR nType ID 102,103 OF oPages:aDialogs[1] ON CHANGE nType := 1
	REDEFINE LISTBOX oObj FIELDS "","","" HEAD "","","c" SIZES 10,10,50 ID 102 OF oPages:aDialogs[1]
	REDEFINE LISTBOX oObj VAR nIcon ITEMS aItems ID 102 OF oPages:aDialogs[1]
Return Nil

// RELEASE OBJECTS (plural).
Function GapReleaseObjects()
	Local oFont, oTree
	RELEASE OBJECTS oFont, oTree
Return Nil

// ACTIVATE MENU/POPUP com AT/WINDOW.
Function GapActivateMenuAt()
	Local oMenu, oWnd
	MENU oMenu POPUP
		MENUITEM "x" ACTION 1
	ENDMENU
	ACTIVATE MENU oMenu AT 10, 20
	ACTIVATE POPUP oMenu WINDOW oWnd AT 10, 20
Return Nil

// LOCATE REST FOR (REST é flag, não deve confundir o FOR de loop seguinte).
Function GapLocateRestFor()
	Local nFound := 0
	LOCATE REST FOR ( nFound == 0 )
Return Nil

// CREATE SCOPE ... FOR (mesma razão do LOCATE REST FOR).
Function GapCreateScopeFor()
	Local aScope
	CREATE SCOPE aScope FOR ( .T. )
Return Nil

// DEFAULT com alvo alias->campo e com índice de array.
Function GapDefaultAliasAndIndex()
	Default HttpSession->cRetPag := ""
	Local oValidationError := JsonObject():New()
	Default oValidationError["k"] := {}
Return Nil

// String delimitada por colchetes span multi-linha via continuação `;`.
Function GapBracketStringContinuation()
	Local cCampo := "X"
	Local lOk := (cCampo $ [AAA/BBB/CCC;
		/DDD/EEE])
Return lOk

// `@ x,y TO ...` no nível top-level (fora de função) não pode ser
// confundido com anotação `@Nome`.
@0,0 TO 10,10 DIALOG oTopDlg TITLE "x" PIXEL

Function GapTopLevelAtCommand()
Return Nil

// WSMETHOD com RESPONSE depois de PRODUCES, e WSRECEIVE com nomes de
// parâmetro que colidem com palavras de cláusula do DSL `@` (Type, Size).
WSRESTFUL GapRest DESCRIPTION 'x'

	WSDATA Page AS INTEGER OPTIONAL

	WSMETHOD GET Item;
	DESCRIPTION "x";
	PATH "/x";
	PRODUCES APPLICATION_JSON RESPONSE EaiObj

END WSRESTFUL

WSMETHOD GET Item QUERYPARAM Page WSREST GapRest
Return .T.

WSMETHOD GetHeaderPart WSRECEIVE UserCode,ParticipantId,Type,CodMap WSSEND PartHeader WSSERVICE GapRest
Return .T.

// `:MethodName(args)` sem receiver, no topo do corpo de um método — idioma
// de invocar a versão da superclasse.
CLASS GapBareColonCall
	METHOD New(nTop) CONSTRUCTOR
ENDCLASS

METHOD New(nTop) CLASS GapBareColonCall
	:New(nTop)
Return Self

// `NO VSCROLL`/`NO HSCROLL` como flags de duas palavras no DSL `@`.
Function GapNoVScroll()
	Local oGet, cTexto := "", oDlg
	@ 6,4 GET oGet VAR cTexto MEMO READONLY SIZE 10,60 PIXEL OF oDlg NO VSCROLL
Return Nil
