// Treina um MLP 2-4-1 (Relu) para ajustar y = x1 + 2*x2, com pesos fixos, SGD e MSE.
// A loss deve cair bem abaixo da inicial — prova autodiff + treino ponta a ponta.
User Function TrainDemo()
    Local oX  := Variable():FromArray({1,0, 0,1, 1,1, 2,1}, {4,2})     // 4 exemplos
    Local oY  := Variable():FromArray({1, 2, 3, 4}, {4,1})             // y = x1 + 2*x2
    // Pesos iniciais fixos e ASSIMÉTRICOS (quebram a simetria entre unidades ocultas
    // -> mais capacidade). Bias positivo mantém as ReLU ativas nos inputs positivos.
    Local oW1 := Variable():FromArray({0.10,0.15,0.20,0.05, 0.05,0.10,0.15,0.20}, {2,4})
    Local ob1 := Variable():FromArray({0.50,0.40,0.60,0.50}, {4})
    Local oW2 := Variable():FromArray({0.10,0.20,0.15,0.05}, {4,1})
    Local ob2 := Variable():FromArray({0}, {1})
    Local oOpt := SGD():New({oW1, ob1, oW2, ob2}, 0.05)
    Local nEpoca := 0
    Local oH, oPred, oLoss
    Local nInicial := 0, nAtual := 0

    For nEpoca := 1 To 1000
        // forward
        oH    := oX:MatMul(oW1):Add(ob1):Relu()
        oPred := oH:MatMul(oW2):Add(ob2)
        oLoss := oPred:MSE(oY)
        nAtual := oLoss:Value():ToArray()[1]
        If nEpoca == 1
            nInicial := nAtual
        EndIf
        If nEpoca == 1 .Or. Mod(nEpoca, 200) == 0
            ConOut("epoca " + Str(nEpoca,4) + " loss " + Str(nAtual,10,5))
        EndIf
        // backward + passo
        oOpt:ZeroGrad()
        oLoss:Backward()
        oOpt:Step()
    Next nEpoca

    ConOut("loss inicial " + Str(nInicial,10,5) + " -> final " + Str(nAtual,10,5))
    If nAtual < nInicial * 0.2
        ConOut("OK: treino reduziu a loss (autodiff + SGD funcionam).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente (final >= 20% da inicial)")
    EndIf
Return
