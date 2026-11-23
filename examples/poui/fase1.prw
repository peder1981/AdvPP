/*/{Protheus.doc} fase1.prw
Exemplo dos componentes PO-UI do AdvPP (fase 1): página com ação, campo,
botão, tabela e tag atualizada por timer.

Rodar: advplc serve examples/poui/fase1.prw
@type file
@author Peder Munksgaard
@since 20/11/2026
/*/

User Function Main()
	Local oApp  := PoApp():New("Componentes PO-UI — fase 1")
	Local oPg   := PoPage():New(oApp, "Contatos")
	Local oHora := PoTag():New(oPg, Time(), "info")
	Local oNome := PoInput():New(oPg, "Nome do contato")
	Local oTab  := PoTable():New(oPg)
	Local oMsg  := PoTag():New(oPg, "Digite um nome e clique em Adicionar", "neutral")
	Local aLin  := {{"Ana", "ana@exemplo.com"}, {"Bruno", "bruno@exemplo.com"}}
	Local oBtn  := PoButton():New(oPg, "Adicionar", {|| Adiciona(oNome, oTab, oMsg, aLin)})

	oTab:SetColumns({{"nome", "Nome"}, {"email", "E-mail"}})
	oTab:SetItems(aLin)
	oTab:OnSelect({|o, aLinha| oMsg:SetValue("Selecionado: " + aLinha[1]), oMsg:Set("type", "success")})
	oPg:AddAction("Limpar", {|| aSize(aLin, 0), oTab:SetItems(aLin), oMsg:SetValue("Lista limpa")})
	oApp:SetTimer(1000, {|| oHora:SetValue(Time())})

	oBtn:Set("kind", "primary")
	oBtn:Set("icon", "an an-plus")
	oApp:Activate()
Return

/*/{Protheus.doc} Adiciona
Inclui o nome digitado na tabela.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Adiciona(oNome, oTab, oMsg, aLin)
	Local cNome := AllTrim(oNome:GetValue()) as Character

	If Empty(cNome)
		oMsg:SetValue("Informe um nome antes de adicionar")
		oMsg:Set("type", "warning")
		Return
	EndIf
	aAdd(aLin, {cNome, Lower(cNome) + "@exemplo.com"})
	oTab:SetItems(aLin)
	oNome:SetValue("")
	oMsg:SetValue(cNome + " adicionado")
	oMsg:Set("type", "success")
Return
