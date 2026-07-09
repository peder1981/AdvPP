User Function WebTst()
    Local lOk := .F.
    ConOut("Bem-vindo ao AdvPP Web!")
    DBSelectArea("SA1")
    ConOut("Clientes na base: " + cValToChar(RecCount()))
    lOk := MsgYesNo("Deseja processar os clientes?", "Confirmação")
    If lOk
        ConOut("Processando...")
        MsgInfo("Processamento concluído com sucesso!", "Sucesso")
    Else
        MsgAlert("Processamento cancelado pelo usuário.", "Atenção")
    EndIf
    ConOut("Fim do programa.")
Return
