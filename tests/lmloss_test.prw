User Function LmLossTst()
    Local oLog := Variable():FromArray({2,1,0.1, 0.3,0.2,3}, {2,3})   // logits [2,3]
    Local oL := oLog:SoftmaxCE({1, 3})   // alvos classe 1 e 3 (1-based)
    Local oEmb := Variable():FromArray({1,2, 3,4, 5,6}, {3,2})        // tabela [3,2]
    Local oPick := oEmb:IndexRows({3, 1})                             // linhas 3 e 1
    Local oH := Variable():FromArray({-1,0.5, 2,-0.3}, {2,2})
    Local nFail := 0

    oL:Backward()
    If oLog:Grad():Size() != 6
        ConOut("FALHA grad softmaxce"); nFail++
    EndIf
    If oPick:Value():Shape()[1] != 2 .Or. oPick:Value():Shape()[2] != 2
        ConOut("FALHA forma IndexRows"); nFail++
    EndIf
    If oL:Value():ToArray()[1] < 0
        ConOut("FALHA loss negativa"); nFail++
    EndIf
    // Adam roda sem erro
    Begin Sequence
        Local oOpt := Adam():New({oEmb}, 0.01)
        oEmb:IndexRows({1,2,3}):Sum():Backward()
        oOpt:Step()
    Recover
        ConOut("FALHA: Adam lancou erro"); nFail++
    End Sequence

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
