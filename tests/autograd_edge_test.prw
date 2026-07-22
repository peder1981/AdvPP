User Function AutogradEdge()
    Local nOk := 0
    Local nTotal := 2

    If NonScalarBackward()
        nOk++
    Else
        ConOut("FALHA: Variable():FromArray({1,2,3},{3}):Backward() nao lancou erro capturavel")
    EndIf

    If ScalarBackwardOk()
        nOk++
    Else
        ConOut("FALHA: Backward() em Variable escalar deveria funcionar sem erro")
    EndIf

    If nOk == nTotal
        ConOut("OK: " + Str(nOk,1) + "/" + Str(nTotal,1))
    Else
        ConOut("TESTE FALHOU: " + Str(nOk,1) + "/" + Str(nTotal,1))
    EndIf
Return

// Caso 1: Backward() em Variable NAO-escalar (shape {3}) deve ser capturavel,
// nunca executar silenciosamente semeando onesLike(v.Value).
Static Function NonScalarBackward()
    Local lCaught := .F.
    Begin Sequence
        Variable():FromArray({1,2,3}, {3}):Backward()
    Recover
        lCaught := .T.
    End Sequence
Return lCaught

// Caso 2 (controle): Backward() em Variable escalar (resultado de Sum()) segue
// funcionando sem lancar erro.
Static Function ScalarBackwardOk()
    Local lOk := .F.
    Begin Sequence
        Variable():FromArray({1,2,3}, {3}):Sum():Backward()
        lOk := .T.
    Recover
        lOk := .F.
    End Sequence
Return lOk
