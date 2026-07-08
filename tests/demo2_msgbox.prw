// Demo 2: Hello World com MsgBox
User Function Demo2MsgBox()
    ConOut("========================================")
    ConOut("Demo 2: Hello World com MsgBox")
    ConOut("========================================")
    
    Local cMensagem := "Hello World!"
    Local cTitulo := "AdvPP Demo"
    
    // Exibe mensagem em diálogo
    MsgInfo(cMensagem, cTitulo)
    
    ConOut("Mensagem exibida: " + cMensagem)
    ConOut("========================================")
Return
