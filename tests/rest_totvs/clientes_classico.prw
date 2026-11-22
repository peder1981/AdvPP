// Fixture do dispatch WSRESTFUL classico (plano PO-UI onda 1, Task 5).
// Cobre PATH com parametro, WSDATA (querystring), aURLParms, aQueryString,
// GetContent/SetResponse/SetStatus/SetHeader/GetHeader e SetRestFault.

WSRESTFUL Clientes DESCRIPTION "Clientes REST classico"

	WSDATA page AS INTEGER OPTIONAL
	WSDATA ativo AS BOOLEAN OPTIONAL
	WSDATA nome AS CHARACTER OPTIONAL

	WSMETHOD GET lista DESCRIPTION "lista" PATH "/api/v1/clientes" PRODUCES APPLICATION_JSON
	WSMETHOD GET umCliente DESCRIPTION "um" PATH "/api/v1/clientes/{id}" PRODUCES APPLICATION_JSON
	WSMETHOD POST incluir DESCRIPTION "inclui" PATH "/api/v1/clientes" PRODUCES APPLICATION_JSON
	WSMETHOD GET falha DESCRIPTION "erro" PATH "/api/v1/clientes/falha" PRODUCES APPLICATION_JSON
	WSMETHOD GET resumo DESCRIPTION "resumo sem path"
	WSMETHOD GET qs DESCRIPTION "querystring" PATH "/api/v1/qs" PRODUCES APPLICATION_JSON
	WSMETHOD GET headers DESCRIPTION "headers" PATH "/api/v1/headers" PRODUCES APPLICATION_JSON

END WSRESTFUL

WSMETHOD GET lista WSRECEIVE page,ativo,nome WSSERVICE Clientes
Local nPage := ::page
Local lAtivo := ::ativo
Local cNome := ::nome
Local cAtivo := "null"
Local cBody := ""
If ValType(nPage) == "U"
	nPage := 1
EndIf
If ValType(lAtivo) == "L"
	If lAtivo == .T.
		cAtivo := "true"
	Else
		cAtivo := "false"
	EndIf
EndIf
If ValType(cNome) != "C"
	cNome := ""
EndIf
cBody := '{"page":' + cValToChar(nPage) + ',"ativo":' + cAtivo + ',"nome":"' + cNome + '"}'
::SetResponse(cBody)
Return .T.

WSMETHOD GET umCliente WSRECEIVE id WSSERVICE Clientes
Local cId := ""
Local cBody := ""
If Len(::aURLParms) > 0
	cId := ::aURLParms[1]
EndIf
cBody := '{"id":"' + cId + '"}'
::SetResponse(cBody)
Return .T.

WSMETHOD POST incluir WSRECEIVE nome WSSERVICE Clientes
::SetContentType("text/plain")
::SetStatus(201)
::SetResponse(::GetContent())
Return .T.

WSMETHOD GET falha WSRECEIVE page WSSERVICE Clientes
Local nPage := ::page
Local x
Local cBody := ""
If nPage == 404
	SetRestFault(404, "nao achei")
	Return .F.
EndIf
If nPage == 500
	x := FuncaoQueNaoExiste()
EndIf
cBody := '{"page":' + cValToChar(nPage) + '}'
::SetResponse(cBody)
Return .T.

WSMETHOD GET resumo WSRECEIVE resto WSSERVICE Clientes
Local cA := ""
Local cB := ""
Local cBody := ""
If Len(::aURLParms) > 0
	cA := ::aURLParms[1]
EndIf
If Len(::aURLParms) > 1
	cB := ::aURLParms[2]
EndIf
cBody := '{"n":' + cValToChar(Len(::aURLParms)) + ',"a":"' + cA + '","b":"' + cB + '"}'
::SetResponse(cBody)
Return .T.

WSMETHOD GET qs WSRECEIVE filtro WSSERVICE Clientes
Local aQS := ::aQueryString
Local n := Len(aQS)
Local cK1 := ""
Local cV1 := ""
Local cK2 := ""
Local cV2 := ""
Local cBody := ""
If n > 0
	cK1 := aQS[1][1]
	cV1 := aQS[1][2]
EndIf
If n > 1
	cK2 := aQS[n][1]
	cV2 := aQS[n][2]
EndIf
cBody := '{"n":' + cValToChar(n) + ',"k1":"' + cK1 + '","v1":"' + cV1 + '","k2":"' + cK2 + '","v2":"' + cV2 + '"}'
::SetResponse(cBody)
Return .T.

WSMETHOD GET headers WSRECEIVE nada WSSERVICE Clientes
Local cBody := ""
::SetHeader("X-Resp", "ok")
::SetStatus(202)
cBody := '{"rid":"' + ::GetHeader("X-Request-Id") + '"}'
::SetResponse(cBody)
Return .T.
