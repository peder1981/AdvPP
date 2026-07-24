// tests/menu_test.prw — FWMenuSelect/FWGetText em execução headless
// (advplc run, sem UIProvider): nunca devem bloquear esperando input que
// não vai vir — devem retornar o "sem escolha" (0 / valor default) na
// hora. O caminho interativo real (advplc serve) não é testável aqui —
// depende de uma sessão de browser de verdade respondendo ao /reply.
User Function MenuTest()
    Local aOpcoes := {"Unidades", "Condôminos", "Sair"}
    Local nEscolha := FWMenuSelect(aOpcoes, "Menu de teste")
    ConOut("escolha=" + Str(nEscolha))

    Local cTexto := FWGetText("Competência?", "2026-08")
    ConOut("texto=" + cTexto)
Return
