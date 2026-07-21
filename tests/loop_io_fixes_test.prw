User Function Gaps()
    Local i := 0
    Local s := ""
    Local h := JsonObject():New()
    Local nRc := 0

    // 1. Loop (continue): pula i==2
    s := ""
    For i := 1 To 5
        If i == 2
            Loop
        EndIf
        s += Str(i,1)
    Next i
    ConOut("1 Loop/continue (esperado 1345): " + s)

    // 2. Exit (break): para em i==3
    s := ""
    For i := 1 To 9
        If i == 3
            Exit
        EndIf
        s += Str(i,1)
    Next i
    ConOut("2 Exit/break (esperado 12): " + s)

    // 3. For Step -1 (descendente)
    s := ""
    For i := 5 To 1 Step -1
        s += Str(i,1)
    Next i
    ConOut("3 For Step -1 (esperado 54321): " + s)

    // 3b. For Step -2
    s := ""
    For i := 10 To 0 Step -2
        s += Str(i,2)
    Next i
    ConOut("3b For Step -2 (esperado 10 8 6 4 2 0): " + s)

    // 4. JsonObject case-sensitive
    h["Brasil"] := "maiusculo"
    h["brasil"] := "minusculo"
    ConOut("4 case-sensitive (esperado maiusculo/minusculo): " + h["Brasil"] + "/" + h["brasil"])

    // 5a. Escrita e leitura em disco
    MemoWrite("/tmp/advpp_io_test.txt", "ola mundo em disco")
    ConOut("5a MemoRead (esperado 'ola mundo em disco'): " + MemoRead("/tmp/advpp_io_test.txt"))

    // 5b. Chamada de sistema + captura via redirecionamento
    nRc := WaitRun("echo saida-do-shell > /tmp/advpp_sys_test.txt")
    ConOut("5b WaitRun rc (esperado 0): " + Str(nRc,1))
    ConOut("5b captura (esperado 'saida-do-shell'): " + AllTrim(MemoRead("/tmp/advpp_sys_test.txt")))

    // 6. Loop aninhado: Exit so quebra o interno
    s := ""
    For i := 1 To 3
        Local j := 0
        For j := 1 To 3
            If j == 2
                Exit
            EndIf
            s += Str(i,1) + Str(j,1) + " "
        Next j
    Next i
    ConOut("6 Exit aninhado (esperado 11 21 31): " + s)

    FErase("/tmp/advpp_io_test.txt")
    FErase("/tmp/advpp_sys_test.txt")
Return
