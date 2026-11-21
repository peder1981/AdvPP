/*/{Protheus.doc} inicio.prw
Página inicial de `advplc serve --apps examples/poui/apps --home inicio`:
lista os outros apps do diretório (ADVPP_APPS_DIR) e abre cada um numa
janela própria (PoApp:Open) — cada janela é uma sessão com VM própria.
@type file
@author Peder Munksgaard
@since 20/11/2026
/*/

User Function Main()
	Local oApp   := PoApp():New("Aplicativos")
	Local oPg    := PoPage():New(oApp, "Aplicativos")
	Local cDir   := GetEnv("ADVPP_APPS_DIR") as Character
	Local aArqs  := {} as Array
	Local cNome  := "" as Character
	Local nI     := 0  as Numeric

	If Empty(cDir)
		PoTag():New(oPg, "Rode com: advplc serve --apps <diretório> --home inicio", "warning")
	Else
		aArqs := Directory(cDir + "/*.prw")
		For nI := 1 To Len(aArqs)
			cNome := StrTran(Lower(aArqs[nI][1]), ".prw", "")
			If cNome != "inicio"
				Cartao(oApp, oPg, cNome)
			EndIf
		Next nI
	EndIf
	oApp:Activate()
Return

/*/{Protheus.doc} Cartao
Um cartão por app, com o botão que o abre noutra janela.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Cartao(oApp, oPg, cNome)
	Local oW := PoWidget():New(oPg, Upper(Left(cNome, 1)) + SubStr(cNome, 2))
	Local oB := PoButton():New(oW, "Abrir " + cNome, {|| oApp:Open(cNome)})

	oW:SetCols(4)
Return
