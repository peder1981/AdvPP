// tests/menu_reconnect_test.prw — fixture pra TestMenuServeReconnectResumesSession
// (webui): reproduz o cenário reportado (2ª chamada de FWMenuSelect, dentro
// de um loop, depois de já ter passado por outra tela) em que uma queda de
// conexão SSE seguida de reconexão do EventSource fazia o programa inteiro
// reiniciar do zero. Ver pkg/webui/server.go (resumo de sessão em /events).
User Function MenuReconnectTest()
    Local aOpcoes := {"Primeira", "Segunda"}
    Local nEscolha

    nEscolha := FWMenuSelect(aOpcoes, "Menu 1")
    ConOut("primeira=" + Str(nEscolha))

    nEscolha := FWMenuSelect(aOpcoes, "Menu 2")
    ConOut("segunda=" + Str(nEscolha))
Return
