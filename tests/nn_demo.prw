// Define um MLP 2-8-2 com dois modulos Linear + Tanh, e o treina com Adam via Fit,
// classificando 4 pontos (XOR-como-classes). Verifica loss caindo e acuracia 100%.
User Function NnDemo()
    Local oL1 := Linear():New(2, 8)
    Local oL2 := Linear():New(8, 2)
    Local oX  := Variable():FromArray({0,0, 0,1, 1,0, 1,1}, {4,2})
    Local aAlvo := {1, 2, 2, 1}
    Local aParams := Concat2(oL1:Params(), oL2:Params())
    Local oOpt := Adam():New(aParams, 0.05)
    Local nInicial := LossOnly(oL1, oL2, oX, aAlvo)
    Local nFinal := 0
    Local oLog := Nil
    Local aPred := {}
    Local nOk := 0
    Local i := 0

    // Treina: cada passo faz forward + zerograd + backward + step, devolve a loss
    nFinal := Fit({|| Passo(oL1, oL2, oX, aAlvo, oOpt) }, 600)

    // Acuracia final
    oLog := oL2:Forward(oL1:Forward(oX):Tanh())
    aPred := oLog:Value():Argmax(2):ToArray()   // 1-based
    For i := 1 To 4
        If aPred[i] == aAlvo[i]
            nOk++
        EndIf
    Next i

    ConOut("loss " + Str(nInicial,9,5) + " -> " + Str(nFinal,9,5) + " | acuracia " + Str(nOk,1) + "/4")
    If nFinal < nInicial * 0.5 .And. nOk == 4
        ConOut("OK: modelo com modulos treinou (Linear + Adam + Fit).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente ou acuracia < 4/4")
    EndIf
Return

Static Function Passo(oL1, oL2, oX, aAlvo, oOpt)
    Local oLog := oL2:Forward(oL1:Forward(oX):Tanh())
    Local oLoss := oLog:SoftmaxCE(aAlvo)
    oOpt:ZeroGrad()
    oLoss:Backward()
    oOpt:Step()
Return oLoss:Value():ToArray()[1]

Static Function LossOnly(oL1, oL2, oX, aAlvo)
    Local oLog := oL2:Forward(oL1:Forward(oX):Tanh())
Return oLog:SoftmaxCE(aAlvo):Value():ToArray()[1]

Static Function Concat2(a, b)
    Local r := {}
    Local i := 0
    For i := 1 To Len(a)
        aAdd(r, a[i])
    Next i
    For i := 1 To Len(b)
        aAdd(r, b[i])
    Next i
Return r
