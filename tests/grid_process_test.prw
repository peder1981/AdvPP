User Function GridTst()
    Local oGrid
    Local nX := 0
    oGrid := FWGridProcess():New("TSTGRID", "Teste", "Teste de grid", Nil, "", "U_GridWk", .T.)
    oGrid:SetMaxThreadGrid(8)
    oGrid:SetThreadGrid(4)
    oGrid:SetMeters(1)
    oGrid:SetMaxMeter(6, 1, "processando")
    For nX := 1 To 6
        If !oGrid:CallExecute(nX, "lote-" + cValToChar(nX))
            Exit
        EndIf
        oGrid:SetIncMeter(1)
    Next nX
    oGrid:Activate()
    If oGrid:IsFinished()
        ConOut("[main] grid finalizado com sucesso")
    Else
        ConOut("[main] grid interrompido")
    EndIf
    oGrid:SaveLog("processamento concluido")
    ConOut("[main] ultimo log: " + oGrid:GetLastLog())
Return

User Function GridWk(nId, cLote)
    DBSelectArea("SA1")
    Sleep(100)
    ConOut("[thread " + cValToChar(nId) + "] " + cLote + " SA1=" + cValToChar(RecCount()))
Return
