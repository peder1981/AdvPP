// tests/menu_serve_test.prw — fixture pra TestMenuServeFixture (webui),
// exercita FWMenuSelect/FWGetText de verdade contra uma sessão real de
// browser (via SSE + POST /reply), não só o fallback headless.
User Function MenuServeTest()
    Local aOpcoes := {"Primeira Opção", "Segunda Opção"}
    Local nEscolha := FWMenuSelect(aOpcoes, "Escolha uma tela")
    ConOut("escolhido=" + Str(nEscolha))

    Local cCompetencia := FWGetText("Qual competência?", "0000-00")
    ConOut("competencia=" + cCompetencia)
Return
