User Function TensorTst()
    Local oA := Tensor():FromArray({1,2,3,4}, {2,2})
    Local oB := Tensor():FromArray({5,6,7,8}, {2,2})
    Local oC := oA:MatMul(oB)            // [[19,22],[43,50]]
    Local aC := oC:ToArray()
    Local oBias := Tensor():FromArray({10,20}, {2})   // broadcast por linha
    Local oS := oC:Add(oBias):Softmax(2) // softmax por linha (eixo 2)
    Local nPred := oC:Argmax()           // maior valor global -> offset 1-based
    Local nFail := 0

    If aC[1] != 19 .Or. aC[4] != 50
        ConOut("FALHA MatMul: " + Str(aC[1]) + "," + Str(aC[4])); nFail++
    EndIf
    If Abs(oS:ToArray()[1] + oS:ToArray()[2] - 1) > 0.001
        ConOut("FALHA Softmax linha nao soma 1"); nFail++
    EndIf
    If nPred != 4                        // 50 é o maior, offset row-major 1-based = 4
        ConOut("FALHA Argmax global: " + Str(nPred)); nFail++
    EndIf
    // erro de forma é capturável
    Begin Sequence
        oA:MatMul(Tensor():New({3,3}))
        ConOut("FALHA: matmul incompativel nao lancou"); nFail++
    Recover
    End Sequence

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
