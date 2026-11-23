/*/{Protheus.doc} fase4.prw
Exemplo dos componentes próprios do AdvPP (fase 4): uma "mini IDE".
Aba Editor: AdvCodeEditor com realce AdvPL; Ctrl+S grava o fonte, roda
`advplc check --format json` (Spawn) e marca o erro na linha/coluna.
Aba Terminal: AdvTerminal com o shell do usuário (pty real; no Windows,
erro claro nesta versão).

Rodar: advplc serve examples/poui/fase4.prw
(o `advplc` usado na verificação vem de $ADVPLC, ou do PATH)
@type file
@author Peder Munksgaard
@since 20/11/2026
/*/

User Function Main()
	Local oApp  := PoApp():New("Mini IDE")
	Local oPg   := PoPage():New(oApp, "Mini IDE AdvPL")
	Local oTabs := PoTabs():New(oPg)
	Local oTEd  := PoTab():New(oTabs, "Editor")
	Local oTTe  := PoTab():New(oTabs, "Terminal")
	Local oSt   := PoTag():New(oTEd, "Ctrl+S salva e verifica o fonte", "info")
	Local oEd   := AdvCodeEditor():New(oTEd, FonteExemplo())
	Local oTerm := AdvTerminal():New(oTTe, Shell())
	Local cArq  := ArqTemp() as Character

	oEd:SetHeight("60vh")
	oTerm:SetHeight("60vh")
	oEd:OnSave({|o, cTexto| Verifica(oApp, oEd, oSt, cArq, cTexto)})
	oTerm:OnExit({|o, n| oApp:Notify("Terminal encerrado (código " + cValToChar(n) + ")", "warning")})
	oApp:Activate()
Return

/*/{Protheus.doc} Verifica
Grava o fonte e roda o advplc check em segundo plano.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Verifica(oApp, oEd, oSt, cArq, cTexto)
	Local aOut := {} as Array

	MemoWrite(cArq, cTexto)
	oSt:SetValue("Verificando...")
	oSt:Set("type", "info")
	oApp:Spawn(Advplc(), {"check", cArq, "--format", "json"}, ;
		{|c| aAdd(aOut, c)}, ;
		{|n| Resultado(oEd, oSt, aOut)})
Return

/*/{Protheus.doc} Resultado
Aplica o resultado do check: marcação no editor e a mensagem na tag.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Resultado(oEd, oSt, aOut)
	Local oJ    := JsonObject():New()
	Local nLin  := 1 as Numeric
	Local nCol  := 1 as Numeric

	If Len(aOut) == 0
		oSt:SetValue("Não consegui rodar o advplc (defina ADVPLC ou ponha no PATH)")
		oSt:Set("type", "warning")
		Return
	EndIf
	oJ:FromJson(aOut[1])
	If oJ["ok"]
		oEd:ClearMarkers()
		oSt:SetValue("Sem erros")
		oSt:Set("type", "success")
		Return
	EndIf
	If ValType(oJ["line"]) == "N"
		nLin := oJ["line"]
	EndIf
	If ValType(oJ["col"]) == "N"
		nCol := oJ["col"]
	EndIf
	oEd:SetMarkers({{nLin, nCol, oJ["message"]}})
	oSt:SetValue("Linha " + cValToChar(nLin) + ": " + oJ["message"])
	oSt:Set("type", "danger")
Return

/*/{Protheus.doc} FonteExemplo
Programa inicial do editor.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function FonteExemplo()
	Local cF := "" as Character

	cF += "// Edite e pressione Ctrl+S para verificar" + Chr(10)
	cF += "User Function Ola()" + Chr(10)
	cF += Chr(9) + 'Local cNome := "AdvPP" as Character' + Chr(10)
	cF += Chr(9) + 'ConOut("Olá, " + cNome)' + Chr(10)
	cF += "Return" + Chr(10)
Return cF

/*/{Protheus.doc} ArqTemp
Arquivo temporário do fonte desta sessão.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function ArqTemp()
	Local cDir := GetEnv("TMPDIR") as Character

	If Empty(cDir)
		cDir := "/tmp"
	EndIf
Return cDir + "/advpp-ide-" + StrTran(cValToChar(Seconds()), ".", "") + ".prw"

/*/{Protheus.doc} Shell
Shell do usuário, ou /bin/sh.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Shell()
	Local cSh := GetEnv("SHELL") as Character

	If Empty(cSh)
		cSh := "/bin/sh"
	EndIf
Return cSh

/*/{Protheus.doc} Advplc
Executável do compilador: $ADVPLC ou "advplc" (PATH).
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Advplc()
	Local cBin := GetEnv("ADVPLC") as Character

	If Empty(cBin)
		cBin := "advplc"
	EndIf
Return cBin
