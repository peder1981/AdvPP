// Teste de Codificação CP1252
// Este arquivo contém caracteres especiais CP1252 para testar conversão automática

User Function CP1252Test()
    Local cTexto := "Teste com caracteres especiais: ç ã é í ó ú"
    Local cEuro := "Símbolo Euro: €"
    Local cAspas := "Aspas: " " " " "
    Local cHifen := "Hífens: – —"
    
    ConOut("=========================================")
    ConOut("Teste de Codificação CP1252")
    ConOut("=========================================")
    ConOut("Texto: " + cTexto)
    ConOut("Euro: " + cEuro)
    ConOut("Aspas: " + cAspas)
    ConOut("Hífens: " + cHifen)
    ConOut("=========================================")
    ConOut("Teste concluído com sucesso!")
    ConOut("Conversão CP1252 -> UTF-8 funcionou")
    ConOut("=========================================")
    
Return .T.
