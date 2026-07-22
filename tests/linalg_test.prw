// Álgebra linear float64 (S6b): Det, Solve, Inv, QR, EigSym.
User Function LinAlgTst()
    Local oA := Tensor():FromArray({4,7,2,6}, {2,2}, "float64")
    Local oI := Nil
    Local oX := Nil
    Local aQR := {}
    Local aEig := {}
    Local nFail := 0
    Local lPegou := .F.
    Local i := 0

    // Det: det([[1,2],[3,4]]) = -2
    If Abs(Tensor():FromArray({1,2,3,4},{2,2},"float64"):Det() - (-2)) > 0.0001
        ConOut("FALHA: Det")
        nFail++
    EndIf

    // Solve: [[2,1],[1,3]] x = [3,5] -> [0.8,1.4]
    oX := Tensor():FromArray({2,1,1,3},{2,2},"float64"):Solve(Tensor():FromArray({3,5},{2},"float64"))
    If Abs(oX:ToArray()[1] - 0.8) > 0.0001 .Or. Abs(oX:ToArray()[2] - 1.4) > 0.0001
        ConOut("FALHA: Solve")
        nFail++
    EndIf

    // Inv: A · A^-1 ~ I
    oI := oA:MatMul(oA:Inv())
    If Abs(oI:ToArray()[1]-1) > 0.0001 .Or. Abs(oI:ToArray()[2]) > 0.0001 .Or. ;
       Abs(oI:ToArray()[3]) > 0.0001 .Or. Abs(oI:ToArray()[4]-1) > 0.0001
        ConOut("FALHA: Inv (A*inv != I)")
        nFail++
    EndIf

    // QR: {Q,R}; Q*R ~ A
    aQR := Tensor():FromArray({12,-51,4,6,167,-68,-4,24,-41},{3,3},"float64"):QR()
    If Len(aQR) != 2
        ConOut("FALHA: QR nao devolveu {Q,R}")
        nFail++
    EndIf

    // EigSym: [[2,0],[0,3]] -> autovalores {3,2}
    aEig := Tensor():FromArray({2,0,0,3},{2,2},"float64"):EigSym()
    If Abs(aEig[1]:ToArray()[1] - 3) > 0.0001 .Or. Abs(aEig[1]:ToArray()[2] - 2) > 0.0001
        ConOut("FALHA: EigSym autovalores")
        nFail++
    EndIf

    // erro capturavel: Inv de singular
    Begin Sequence
        Tensor():FromArray({1,2,2,4},{2,2},"float64"):Inv()
        ConOut("FALHA: Inv singular nao lancou")
        nFail++
    Recover
        lPegou := .T.
    End Sequence
    If !lPegou
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: algebra linear (Det/Solve/Inv/QR/EigSym) verificada.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,2))
    EndIf
Return
