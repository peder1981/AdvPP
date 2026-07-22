// dtype float64 selecionavel no Tensor (S6a).
User Function TF64Tst()
    Local oA := Tensor():New({2,2}, "float64")
    Local oB := Nil
    Local oC := Nil
    Local oF32 := Nil
    Local nFail := 0
    Local lPegou := .F.

    If oA:DType() != "float64"
        ConOut("FALHA: DType deveria ser float64, veio " + oA:DType())
        nFail++
    EndIf

    // aritmetica + matmul em float64
    oA := Tensor():FromArray({1,2,3,4}, {2,2}, "float64")
    oB := Tensor():FromArray({5,6,7,8}, {2,2}, "float64")
    oC := oA:MatMul(oB)               // [[19,22],[43,50]]
    If oC:DType() != "float64" .Or. oC:ToArray()[1] != 19 .Or. oC:ToArray()[4] != 50
        ConOut("FALHA: MatMul float64")
        nFail++
    EndIf
    If oA:Dot(oB) != 70               // 1*5+2*6+3*7+4*8
        ConOut("FALHA: Dot = " + Str(oA:Dot(oB), 10, 2))
        nFail++
    EndIf

    // conversoes
    oF32 := oA:ToFloat32()
    If oF32:DType() != "float32"
        ConOut("FALHA: ToFloat32 nao converteu")
        nFail++
    EndIf
    If oF32:ToFloat64():DType() != "float64"
        ConOut("FALHA: ToFloat64 nao converteu")
        nFail++
    EndIf

    // default continua float32
    If Tensor():New({2,2}):DType() != "float32"
        ConOut("FALHA: default deveria ser float32")
        nFail++
    EndIf

    // Norm em float64: |{3,4}| = 5
    If Tensor():FromArray({3,4}, {2}, "float64"):Norm() != 5
        ConOut("FALHA: Norm float64")
        nFail++
    EndIf

    // forma invalida -> ErrorValue capturavel
    Begin Sequence
        Tensor():FromArray({1,2,3}, {2,2}, "float64")
        ConOut("FALHA: forma invalida nao lancou erro")
        nFail++
    Recover
        lPegou := .T.
    End Sequence
    If !lPegou
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: Tensor float64 (dtype seletivo) verificado.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail, 2))
    EndIf
Return
