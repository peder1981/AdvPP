// Fixture: vazamento de escopo dinâmico (Private/Public) quando um frame é
// descartado por um caminho de erro que NÃO passa por doReturn — tanto no
// loop síncrono de blocos de código (AEval/ASort/AScan, via callBlockSync)
// quanto no desenrolamento de Try/Catch do runLoop principal quando o Throw
// ocorre em uma função aninhada mais profunda que a que contém o Try.
// Em ambos os casos, um Private declarado no frame descartado (que sombreia
// um Private de mesmo nome no chamador) deve ser removido de v.dynEnv, e o
// binding anterior do chamador deve ser restaurado.
// Auto-verificável: compara valor esperado x obtido, imprime
// "OK: N/N verificacoes passaram" (padrão de pt_nn.prw).
User Function DynLeakTst()
    Local nFail := 0

    // 1. Throw dentro de um bloco de código (AEval), capturado pelo Try do
    //    chamador. O frame do bloco é descartado pelo caminho de ERRO de
    //    callBlockSync (não passa por doReturn).
    nFail += TestBlockLeak()

    // 2. Throw em função chamada normalmente (mais profunda que o Try),
    //    capturado pelo Try do chamador via desenrolamento de handleCatch.
    nFail += TestNestedCatchLeak()

    If nFail == 0
        ConOut("OK: 2/2 verificacoes passaram.")
    Else
        ConOut("FALHA: " + Str(nFail,1) + " verificacao(oes) falharam.")
    EndIf
Return

Static Function TestBlockLeak()
    Local nErr := 0
    Private cCtx := "A"
    Try
        AEval({1}, {|e| BlkBody(e) })
    Catch oErr
        If cCtx != "A"
            ConOut("FALHA (bloco/catch): esperado A, obtido " + cCtx)
            nErr++
        EndIf
    EndTry
    If cCtx != "A"
        ConOut("FALHA (bloco/depois): esperado A, obtido " + cCtx)
        nErr++
    EndIf
Return nErr

Static Function BlkBody(e)
    Private cCtx := "TEMP-BLK"
    Throw("boom")
Return

Static Function TestNestedCatchLeak()
    Local nErr := 0
    Private cCtx := "B"
    Try
        InnerFn()
    Catch oErr
        If cCtx != "B"
            ConOut("FALHA (nested/catch): esperado B, obtido " + cCtx)
            nErr++
        EndIf
    EndTry
    If cCtx != "B"
        ConOut("FALHA (nested/depois): esperado B, obtido " + cCtx)
        nErr++
    EndIf
Return nErr

Static Function InnerFn()
    Private cCtx := "TEMP-NESTED"
    Throw("boom")
Return
