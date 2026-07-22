User Function NnTst()
    Local oLin := Linear():New(2, 3)
    Local oX := Variable():FromArray({1,2, 3,4}, {2,2})   // [2,2]
    Local oY := oLin:Forward(oX)                          // [2,3]
    Local aP := oLin:Params()
    Local oEmb := Embedding():New(4, 2)
    Local oPick := oEmb:Forward({3, 1})                   // linhas 3 e 1 (1-based)
    Local aC := {0}
    Local nFinal := 0
    Local nFail := 0

    If oY:Value():Shape()[2] != 3
        ConOut("FALHA Linear Forward")
        nFail++
    EndIf
    If Len(aP) != 2
        ConOut("FALHA Linear Params")
        nFail++
    EndIf
    If oPick:Value():Shape()[1] != 2 .Or. oPick:Value():Shape()[2] != 2
        ConOut("FALHA Embedding Forward")
        nFail++
    EndIf
    // Fit avalia o codeblock N vezes; o array capturado (tipo referência, como o
    // otimizador/modelo no treino real) acumula e a ultima avaliacao devolve 5.
    nFinal := Fit({|| ContaEsoma(aC) }, 5)
    If aC[1] != 5 .Or. nFinal != 5
        ConOut("FALHA Fit: cont=" + Str(aC[1],1) + " final=" + Str(nFinal,3))
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 4/4 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return

Static Function ContaEsoma(aC)
    aC[1] := aC[1] + 1
Return aC[1]
