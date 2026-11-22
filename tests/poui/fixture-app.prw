// Fixture do plano PO-UI onda 2, Task 2.7: FwCallApp abre o app
// tests/poui/apps/fixture-app (dist Angular do fixture-app-src) numa aba
// da sessão web. O JsToAdvpl responde ao "ping" do botão "Pingar AdvPL"
// devolvendo "pong" pelo canal AdvPLToJS — o receiveId viaja em cContent.
// Os #include no fim trazem o endpoint /api/est/v1/armazens (fixture da
// onda 1) para a tabela do app ter dados no serve de arquivo único —
// ficam depois das funções para FixtureApp continuar sendo a entrada.

User Function FixtureApp()
	Local cUrl
	ConOut("[fixture-app] iniciando FwCallApp")
	cUrl := FwCallApp("fixture-app")
	ConOut("[fixture-app] FwCallApp retornou: " + cUrl)
Return Nil

Static Function JsToAdvpl(oWebChannel, cType, cContent)
	If Upper(cType) == "PING"
		oWebChannel:AdvPLToJS(cContent, "pong")
	EndIf
Return Nil

#include "../rest_totvs/armazens_repo.tlpp"
#include "../rest_totvs/armazens_api.tlpp"
