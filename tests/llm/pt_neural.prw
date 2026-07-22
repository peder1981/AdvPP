/*/{Protheus.doc} PtNeural
    LM neural char-level (byte-level) treinado 100% em AdvPP.
    NPLM estilo Bengio (2003) com Embedding real, sobre o stack S2+S3:
      Embedding(V,D):Forward(idx) -> [N*k, D]
        -> Reshape [N, k*D] -> Linear(k*D,H) -> Tanh -> Linear(H,V) -> logits
        -> SoftmaxCE(proximo char)          (treino: Adam via Fit)
    Geracao: seed -> ultimos k chars -> forward -> softmax+temperatura+amostragem.
    Corpus real em tests/llm/corpus.txt se existir; senao mini-corpus deterministico
    (auto-teste: loss_final << loss_inicial e geracao nao-vazia).
    @type  user function
    @author AdvPP
    @since 2026-07-22
/*/
User Function PtNeural()
    Local cCorpus := ""
    Local lReal   := .F.
    Local aVoc    := {}
    Local aId2Code := {}
    Local aCode2Id := {}
    Local aIds    := {}
    Local aEx     := {}
    Local aX      := {}
    Local aAlvo   := {}
    Local nN      := 0
    Local nK      := 3
    Local nD      := 8
    Local nH      := 16
    Local nV      := 0
    Local nLR     := 0.05
    Local nEpocas := 80
    Local nMax    := 400
    Local nGerar  := 40
    Local cSeed   := ""
    Local oEmb    := Nil
    Local oL1     := Nil
    Local oL2     := Nil
    Local oOpt    := Nil
    Local nInicial := 0
    Local nFinal  := 0
    Local cGerado := ""

    // Corpus real se existir; senao mini-corpus deterministico (auto-teste).
    If File("tests/llm/corpus.txt")
        cCorpus := MemoRead("tests/llm/corpus.txt")
        lReal   := .T.
        nK      := 6
        nD      := 24
        nH      := 96
        nEpocas := 150
        nMax    := 2500
        nGerar  := 240
    Else
        cCorpus := MiniCorpus()
    EndIf

    aVoc     := BuildVocab(cCorpus)
    aId2Code := aVoc[1]
    aCode2Id := aVoc[2]
    nV       := Len(aId2Code)
    aIds     := Encode(cCorpus, aCode2Id)
    aEx      := BuildExamples(aIds, nK, nMax)
    aX       := aEx[1]
    aAlvo    := aEx[2]
    nN       := aEx[3]

    ConOut("corpus=" + Str(Len(cCorpus),7) + " vocab=" + Str(nV,4) + " exemplos=" + Str(nN,6) + ;
           " (k=" + Str(nK,1) + " D=" + Str(nD,2) + " H=" + Str(nH,3) + ")")

    If nN < 2
        ConOut("FALHA: corpus pequeno demais para gerar exemplos.")
        Return
    EndIf

    oEmb := Embedding():New(nV, nD)
    oL1  := Linear():New(nK * nD, nH)
    oL2  := Linear():New(nH, nV)
    oOpt := Adam():New(Params3(oEmb, oL1, oL2), nLR)

    nInicial := LossOnly(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD)
    nFinal   := Fit({|| Passo(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD, oOpt) }, nEpocas)

    If lReal
        cSeed := "no "
    Else
        cSeed := "o gato "
    EndIf
    cGerado := Gera(oEmb, oL1, oL2, aId2Code, aCode2Id, cSeed, nK, nD, nGerar, 0.8)

    ConOut("loss " + Str(nInicial,11,5) + " -> " + Str(nFinal,11,5))
    ConOut("gerado: " + cGerado)
    If nFinal < nInicial * 0.5 .And. Len(cGerado) > 0
        ConOut("OK: pt_neural treinou e gerou (Embedding + Reshape + Adam + Fit).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente ou geracao vazia.")
    EndIf
Return

// ------------------------------------------------------------------
// Tokenizador byte-level: vocab = bytes distintos do corpus (ids 1-based).
// ------------------------------------------------------------------
Static Function BuildVocab(cT)
    Local aSeen    := Array(256)
    Local aId2Code := {}
    Local aCode2Id := Array(256)
    Local i        := 0
    Local n        := 0
    Local nLen     := Len(cT)

    For i := 1 To 256
        aSeen[i]    := .F.
        aCode2Id[i] := 0
    Next i
    For i := 1 To nLen
        n := Asc(SubStr(cT, i, 1))
        aSeen[n + 1] := .T.
    Next i
    For i := 1 To 256
        If aSeen[i]
            aAdd(aId2Code, i - 1)            // codigo do byte
            aCode2Id[i] := Len(aId2Code)     // id (1-based)
        EndIf
    Next i
Return {aId2Code, aCode2Id}

Static Function Encode(cT, aCode2Id)
    Local aIds := {}
    Local i    := 0
    Local nLen := Len(cT)
    Local nId  := 0

    For i := 1 To nLen
        nId := aCode2Id[Asc(SubStr(cT, i, 1)) + 1]
        If nId == 0
            nId := 1                          // byte fora do vocab -> id 1
        EndIf
        aAdd(aIds, nId)
    Next i
Return aIds

// Janela deslizante: k ids -> proximo id. aX achatado (N*k, exemplo-maior).
// Amostra por stride se ultrapassar nMax exemplos.
Static Function BuildExamples(aIds, nK, nMax)
    Local aX      := {}
    Local aAlvo   := {}
    Local nN      := 0
    Local nTot    := Len(aIds) - nK
    Local nStride := 1
    Local p       := 0
    Local j       := 0

    If nTot <= 0
        Return {aX, aAlvo, 0}
    EndIf
    If nTot > nMax
        nStride := Int(nTot / nMax) + 1
    EndIf
    p := 1
    Do While p <= nTot
        For j := 0 To nK - 1
            aAdd(aX, aIds[p + j])
        Next j
        aAdd(aAlvo, aIds[p + nK])
        nN++
        p := p + nStride
    EndDo
Return {aX, aAlvo, nN}

// ------------------------------------------------------------------
// Modelo: Embedding -> Reshape -> Linear -> Tanh -> Linear -> logits [N,V]
// ------------------------------------------------------------------
Static Function FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD)
    Local oE := oEmb:Forward(aX)                 // [N*k, D]
    Local oR := oE:Reshape({nN, nK * nD})        // [N, k*D]
    Local oH := oL1:Forward(oR):Tanh()           // [N, H]
Return oL2:Forward(oH)                           // [N, V] (logits)

Static Function LossOnly(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD)
Return FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD):SoftmaxCE(aAlvo):Value():ToArray()[1]

Static Function Passo(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD, oOpt)
    Local oLog  := FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD)
    Local oLoss := oLog:SoftmaxCE(aAlvo)
    oOpt:ZeroGrad()
    oLoss:Backward()
    oOpt:Step()
Return oLoss:Value():ToArray()[1]

Static Function Params3(oEmb, oL1, oL2)
    Local aP := {}
    Local aE := oEmb:Params()
    Local a1 := oL1:Params()
    Local a2 := oL2:Params()
    Local i  := 0

    For i := 1 To Len(aE)
        aAdd(aP, aE[i])
    Next i
    For i := 1 To Len(a1)
        aAdd(aP, a1[i])
    Next i
    For i := 1 To Len(a2)
        aAdd(aP, a2[i])
    Next i
Return aP

// ------------------------------------------------------------------
// Geracao: mantem janela de k ids; a cada passo forward N=1 -> amostra.
// ------------------------------------------------------------------
Static Function Gera(oEmb, oL1, oL2, aId2Code, aCode2Id, cSeed, nK, nD, nGerar, nTemp)
    Local cOut     := cSeed
    Local aCtx     := {}
    Local aSeedIds := Encode(cSeed, aCode2Id)
    Local nV       := Len(aId2Code)
    Local i        := 0
    Local j        := 0
    Local nId      := 0
    Local aLogits  := {}

    For i := 1 To nK
        aAdd(aCtx, 1)                            // padding inicial
    Next i
    For i := 1 To Len(aSeedIds)
        For j := 1 To nK - 1
            aCtx[j] := aCtx[j + 1]
        Next j
        aCtx[nK] := aSeedIds[i]
    Next i

    For i := 1 To nGerar
        aLogits := FwdModel(oEmb, oL1, oL2, aCtx, 1, nK, nD):Value():ToArray()
        nId     := Sample(aLogits, nV, nTemp)
        cOut    := cOut + Chr(aId2Code[nId])
        For j := 1 To nK - 1
            aCtx[j] := aCtx[j + 1]
        Next j
        aCtx[nK] := nId
    Next i
Return cOut

// Amostragem categorica com temperatura (softmax estavel + cumulativa).
Static Function Sample(aLogits, nV, nTemp)
    Local aP   := Array(nV)
    Local nMax := 0
    Local nSum := 0
    Local r    := 0
    Local acc  := 0
    Local i    := 0

    If nTemp <= 0
        nTemp := 1
    EndIf
    For i := 1 To nV
        aLogits[i] := aLogits[i] / nTemp
    Next i
    nMax := aLogits[1]
    For i := 2 To nV
        If aLogits[i] > nMax
            nMax := aLogits[i]
        EndIf
    Next i
    nSum := 0
    For i := 1 To nV
        aP[i] := Exp(aLogits[i] - nMax)
        nSum  := nSum + aP[i]
    Next i
    r   := (Random(1000000) / 1000000) * nSum
    acc := 0
    For i := 1 To nV
        acc := acc + aP[i]
        If acc >= r
            Return i
        EndIf
    Next i
Return nV

// Mini-corpus deterministico e repetitivo (auto-teste sem corpus externo).
Static Function MiniCorpus()
    Local c := ""
    Local i := 0
    For i := 1 To 40
        c := c + "o gato subiu no telhado e viu a lua. "
    Next i
Return c
