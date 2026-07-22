User Function AutogradTst()
    Local oX  := Variable():FromArray({1,2,3,4,5,6}, {2,3})   // [2,3]
    Local oW  := Variable():FromArray({0.1,0.2,0.1,0.2,0.1,0.2}, {3,2})  // [3,2]
    Local ob  := Variable():FromArray({0.5,0.5}, {2})         // bias linha
    Local oAlvo := Variable():FromArray({1,0,0,1}, {2,2})
    Local oY, oL, aGW, nFail := 0

    oY := oX:MatMul(oW):Add(ob):Relu()     // [2,2]
    oL := oY:MSE(oAlvo)                     // escalar
    oL:Backward()

    aGW := oW:Grad():Shape()
    If aGW[1] != 3 .Or. aGW[2] != 2
        ConOut("FALHA forma do grad de W: " + Str(aGW[1]) + "," + Str(aGW[2])); nFail++
    EndIf
    If ob:Grad():Size() != 2
        ConOut("FALHA tamanho do grad de b"); nFail++
    EndIf
    // loss é escalar >= 0
    If oL:Value():ToArray()[1] < 0
        ConOut("FALHA loss negativa"); nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1))
    EndIf
Return
