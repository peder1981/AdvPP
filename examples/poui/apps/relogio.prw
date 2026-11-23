/*/{Protheus.doc} relogio.prw
App de exemplo: relógio atualizado por timer.
@type file
@author Peder Munksgaard
@since 20/11/2026
/*/

User Function Main()
	Local oApp  := PoApp():New("Relógio")
	Local oPg   := PoPage():New(oApp, "Relógio")
	Local oHora := PoTag():New(oPg, Time(), "info")
	Local oData := PoInfo():New(oPg, "Hoje", DToC(Date()))

	oApp:SetTimer(1000, {|| oHora:SetValue(Time())})
	oApp:Activate()
Return
