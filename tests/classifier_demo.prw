// Classificador: 4 pontos 2D, 2 classes linearmente separaveis. MLP 2-8-2 com Tanh,
// loss SoftmaxCE, otimizador Adam. Verifica loss caindo e acuracia 100%.
User Function ClassifierDemo()
    Local oX := Variable():FromArray({0,0, 0,1, 1,0, 1,1}, {4,2})
    Local aAlvo := {1, 2, 2, 1}   // classe 1-based (XOR-como-classes: 0/1)
    Local oW1 := Variable():FromArray({0.3,-0.2,0.1,0.4,-0.3,0.2,0.5,-0.1, -0.4,0.3,0.2,-0.5,0.1,0.4,-0.2,0.3}, {2,8})
    Local ob1 := Variable():FromArray({0,0,0,0,0,0,0,0}, {8})
    Local oW2 := Variable():FromArray({0.2,-0.3, 0.4,0.1, -0.2,0.5, 0.3,-0.4, 0.1,0.2, -0.5,0.3, 0.4,-0.1, 0.2,0.3}, {8,2})
    Local ob2 := Variable():FromArray({0,0}, {2})
    Local oOpt := Adam():New({oW1, ob1, oW2, ob2}, 0.05)
    Local nEpoca := 0, oH, oLog, oLoss
    Local nInicial := 0, nAtual := 0

    For nEpoca := 1 To 500
        oH   := oX:MatMul(oW1):Add(ob1):Tanh()
        oLog := oH:MatMul(oW2):Add(ob2)        // logits [4,2]
        oLoss := oLog:SoftmaxCE(aAlvo)
        nAtual := oLoss:Value():ToArray()[1]
        If nEpoca == 1
            nInicial := nAtual
        EndIf
        If nEpoca == 1 .Or. Mod(nEpoca,100) == 0
            ConOut("epoca " + Str(nEpoca,4) + " loss " + Str(nAtual,9,5))
        EndIf
        oOpt:ZeroGrad()
        oLoss:Backward()
        oOpt:Step()
    Next nEpoca

    // acuracia: argmax por linha dos logits vs alvo
    Local aPred := oLog:Value():Argmax(2):ToArray()   // Argmax por eixo 2 -> [4], 1-based
    Local nOk := 0, i := 0
    For i := 1 To 4
        If aPred[i] == aAlvo[i]
            nOk++
        EndIf
    Next i
    ConOut("loss " + Str(nInicial,9,5) + " -> " + Str(nAtual,9,5) + " | acuracia " + Str(nOk,1) + "/4")
    If nAtual < nInicial * 0.5 .And. nOk == 4
        ConOut("OK: classificador treinou (softmax-CE + Adam funcionam).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente ou acuracia < 4/4")
    EndIf
Return
