// Demo 3: Hello World com MsgYesNo
User Function Demo3MsgYesNo()
    ConOut("========================================")
    ConOut("Demo 3: Hello World com MsgYesNo")
    ConOut("========================================")
    
    Local cPergunta := "Hello World?"
    Local cTitulo := "Confirmação AdvPP"
    Local lResposta
    
    // Exibe diálogo de confirmação e aguarda clique do usuário
    lResposta := MsgYesNo(cPergunta, cTitulo)
    
    ConOut("Pergunta: " + cPergunta)
    
    If lResposta
        ConOut("Usuário clicou em SIM")
        MsgInfo("Você escolheu SIM!", "Resultado")
    Else
        ConOut("Usuário clicou em NÃO")
        MsgInfo("Você escolheu NÃO!", "Resultado")
    EndIf
    
    ConOut("========================================")
Return
