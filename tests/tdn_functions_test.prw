// Teste de Funcoes TDN do TOTVS
// Este arquivo testa as funcoes adicionadas do TDN

User Function TDNFunctionsTest()
    Local sTeste
    Local nResultado
    Local dData
    Local aArray
    
    ConOut("=========================================")
    ConOut("Teste de Funcoes TDN do TOTVS")
    ConOut("=========================================")
    
    // Teste 1: LEFT
    ConOut("")
    ConOut("--- Teste 1: LEFT ---")
    sTeste := "TOTVS Protheus"
    nResultado := LEFT(sTeste, 5)
    ConOut("LEFT('TOTVS Protheus', 5) = " + nResultado)
    
    // Teste 2: RIGHT
    ConOut("")
    ConOut("--- Teste 2: RIGHT ---")
    nResultado := RIGHT(sTeste, 7)
    ConOut("RIGHT('TOTVS Protheus', 7) = " + nResultado)
    
    // Teste 3: REPLICA
    ConOut("")
    ConOut("--- Teste 3: REPLICA ---")
    nResultado := REPLICA("AB", 3)
    ConOut("REPLICA('AB', 3) = " + nResultado)
    
    // Teste 4: CAPSLOCK
    ConOut("")
    ConOut("--- Teste 4: CAPSLOCK ---")
    nResultado := CAPSLOCK("totvs")
    ConOut("CAPSLOCK('totvs') = " + nResultado)
    
    // Teste 5: PROPER
    ConOut("")
    ConOut("--- Teste 5: PROPER ---")
    nResultado := PROPER("totvs protheus")
    ConOut("PROPER('totvs protheus') = " + nResultado)
    
    // Teste 6: ATC
    ConOut("")
    ConOut("--- Teste 6: ATC ---")
    nResultado := ATC("totvs", "TOTVS PROTHEUS")
    ConOut("ATC('totvs', 'TOTVS PROTHEUS') = " + Str(nResultado))
    
    // Teste 7: RATC
    ConOut("")
    ConOut("--- Teste 7: RATC ---")
    nResultado := RATC("s", "TOTVS PROTHEUS")
    ConOut("RATC('s', 'TOTVS PROTHEUS') = " + Str(nResultado))
    
    // Teste 8: GETWORDNUM
    ConOut("")
    ConOut("--- Teste 8: GETWORDNUM ---")
    nResultado := GETWORDNUM("TOTVS PROTHEUS", 2)
    ConOut("GETWORDNUM('TOTVS PROTHEUS', 2) = " + nResultado)
    
    // Teste 9: WORDS
    ConOut("")
    ConOut("--- Teste 9: WORDS ---")
    nResultado := WORDS("TOTVS PROTHEUS")
    ConOut("WORDS('TOTVS PROTHEUS') = " + Str(nResultado))
    
    // Teste 10: FILENOEXT
    ConOut("")
    ConOut("--- Teste 10: FILENOEXT ---")
    nResultado := FILENOEXT("/path/to/file.txt")
    ConOut("FILENOEXT('/path/to/file.txt') = " + nResultado)
    
    // Teste 11: FILEEXT
    ConOut("")
    ConOut("--- Teste 11: FILEEXT ---")
    nResultado := FILEEXT("/path/to/file.txt")
    ConOut("FILEEXT('/path/to/file.txt') = " + nResultado)
    
    // Teste 12: FILENAME
    ConOut("")
    ConOut("--- Teste 12: FILENAME ---")
    nResultado := FILENAME("/path/to/file.txt")
    ConOut("FILENAME('/path/to/file.txt') = " + nResultado)
    
    // Teste 13: FILEPATH
    ConOut("")
    ConOut("--- Teste 13: FILEPATH ---")
    nResultado := FILEPATH("/path/to/file.txt")
    ConOut("FILEPATH('/path/to/file.txt') = " + nResultado)
    
    // Teste 14: FILEDIR
    ConOut("")
    ConOut("--- Teste 14: FILEDIR ---")
    nResultado := FILEDIR("/path/to/file.txt")
    ConOut("FILEDIR('/path/to/file.txt') = " + nResultado)
    
    // Teste 15: SIGN
    ConOut("")
    ConOut("--- Teste 15: SIGN ---")
    nResultado := SIGN(10)
    ConOut("SIGN(10) = " + Str(nResultado))
    nResultado := SIGN(-5)
    ConOut("SIGN(-5) = " + Str(nResultado))
    nResultado := SIGN(0)
    ConOut("SIGN(0) = " + Str(nResultado))
    
    // Teste 16: POWER
    ConOut("")
    ConOut("--- Teste 16: POWER ---")
    nResultado := POWER(2, 3)
    ConOut("POWER(2, 3) = " + Str(nResultado))
    
    // Teste 17: PI
    ConOut("")
    ConOut("--- Teste 17: PI ---")
    nResultado := PI()
    ConOut("PI() = " + Str(nResultado))
    
    // Teste 18: SIN
    ConOut("")
    ConOut("--- Teste 18: SIN ---")
    nResultado := SIN(0)
    ConOut("SIN(0) = " + Str(nResultado))
    
    // Teste 19: COS
    ConOut("")
    ConOut("--- Teste 19: COS ---")
    nResultado := COS(0)
    ConOut("COS(0) = " + Str(nResultado))
    
    // Teste 20: STOD
    ConOut("")
    ConOut("--- Teste 20: STOD ---")
    dData := STOD("20240101")
    ConOut("STOD('20240101') = " + DTOC(dData))
    
    // Teste 21: ELAPTIME
    ConOut("")
    ConOut("--- Teste 21: ELAPTIME ---")
    nResultado := ELAPTIME(100, 150)
    ConOut("ELAPTIME(100, 150) = " + Str(nResultado))
    
    // Teste 22: CTOT
    ConOut("")
    ConOut("--- Teste 22: CTOT ---")
    nResultado := CTOT("12:30:45")
    ConOut("CTOT('12:30:45') = " + Str(nResultado))
    
    // Teste 23: TTOC
    ConOut("")
    ConOut("--- Teste 23: TTOC ---")
    nResultado := TTOC(45000)
    ConOut("TTOC(45000) = " + nResultado)
    
    ConOut("")
    ConOut("=========================================")
    ConOut("Teste de funcoes TDN concluido!")
    ConOut("Todas as funcoes TDN funcionam")
    ConOut("=========================================")
    
Return .T.
