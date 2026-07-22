User Function ReshpTst()
    Local oV := Variable():FromArray({1,2,3,4,5,6}, {2,3})
    Local oR := oV:Reshape({3,2})
    Local aSh := oR:Value():Shape()
    Local nFail := 0
    Local lPegou := .F.

    If aSh[1] != 3 .Or. aSh[2] != 2
        ConOut("FALHA forma pos-reshape")
        nFail++
    EndIf

    oR:Sum():Backward()
    aSh := oV:Grad():Shape()
    If aSh[1] != 2 .Or. aSh[2] != 3
        ConOut("FALHA grad nao voltou na forma original")
        nFail++
    EndIf

    // Reshape com contagem incompativel deve ser ErrorValue capturavel
    Begin Sequence
        oV:Reshape({4,4})
    Recover
        lPegou := .T.
    End Sequence
    If !lPegou
        ConOut("FALHA reshape invalido nao lancou erro")
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
