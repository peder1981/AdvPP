User Function TensorEdge()
    Local nOk := 0
    Local nTotal := 5

    If NewNegDim()
        nOk++
    Else
        ConOut("FALHA: Tensor():New({-1,2}) nao lancou erro capturavel")
    EndIf

    If RandNegDim()
        nOk++
    Else
        ConOut("FALHA: Tensor():Rand({-2},1) nao lancou erro capturavel")
    EndIf

    If EmptyMax()
        nOk++
    Else
        ConOut("FALHA: Tensor():New({0}):Max() nao lancou erro capturavel")
    EndIf

    If EmptyArgmax()
        nOk++
    Else
        ConOut("FALHA: Tensor():New({0}):Argmax() nao lancou erro capturavel")
    EndIf

    If EmptySoftmax()
        nOk++
    Else
        ConOut("FALHA: Tensor():New({0}):Softmax() nao lancou erro capturavel")
    EndIf

    If nOk == nTotal
        ConOut("OK: " + Str(nOk,1) + "/" + Str(nTotal,1))
    Else
        ConOut("TESTE FALHOU: " + Str(nOk,1) + "/" + Str(nTotal,1))
    EndIf
Return

// Caso 1: forma com dimensao negativa em New() deve ser capturavel (nao derrubar a VM)
Static Function NewNegDim()
    Local lCaught := .F.
    Begin Sequence
        Tensor():New({-1,2})
    Recover
        lCaught := .T.
    End Sequence
Return lCaught

// Caso 2: forma com dimensao negativa em Rand() deve ser capturavel
Static Function RandNegDim()
    Local lCaught := .F.
    Begin Sequence
        Tensor():Rand({-2}, 1)
    Recover
        lCaught := .T.
    End Sequence
Return lCaught

// Caso 3: Max() em tensor vazio deve ser capturavel
Static Function EmptyMax()
    Local lCaught := .F.
    Begin Sequence
        Tensor():New({0}):Max()
    Recover
        lCaught := .T.
    End Sequence
Return lCaught

// Caso 4: Argmax() em tensor vazio deve ser capturavel
Static Function EmptyArgmax()
    Local lCaught := .F.
    Begin Sequence
        Tensor():New({0}):Argmax()
    Recover
        lCaught := .T.
    End Sequence
Return lCaught

// Caso 5: Softmax() em tensor vazio deve ser capturavel
Static Function EmptySoftmax()
    Local lCaught := .F.
    Begin Sequence
        Tensor():New({0}):Softmax()
    Recover
        lCaught := .T.
    End Sequence
Return lCaught
