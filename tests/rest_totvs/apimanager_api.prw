// Endpoint FwApiManager (leitura) — plano PO-UI onda 1, Task 8.
// A query string vira aQueryString {chave, valor} e alimenta
// SetApiQString; a resposta sai por GetJsonSerialize.

@Get("/api/est/v1/apimgr")
User Function ApiMgrList(oParam)
	Local oQ := oRest:getQueryRequest()
	Local aQS := {}
	aQS := {{"filter", oQ:FILTER}, {"page", oQ:PAGE}, {"pageSize", oQ:PAGESIZE},;
		{"order", oQ:ORDER}, {"fields", oQ:FIELDS}}
	Local oApi := FwApiManager():New("est", "v1")
	oApi:SetApiAlias("NNR")
	oApi:SetApiMap({{"codigo", "NNR_CODIGO"}, {"descri", "NNR_DESCRI"},;
		{"estoque", "NNR_ESTOQUE"}, {"filial", "NNR_FILIAL"}})
	oApi:SetApiQString(aQS)
	oApi:SetQuery("SELECT #QueryFields# FROM NNR #QueryWhere#")
	If !oApi:Activate()
		oRest:setStatusCode(400)
		oRest:setResponse({"message": "erro na api"})
		Return Nil
	EndIf
	oRest:setResponse(oApi:GetJsonSerialize())
Return Nil
