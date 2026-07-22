/*/{Protheus.doc} DevNN
    LM neural de CODIGO AdvPL, token-level, treinado 100% em AdvPP. Orientado a
    desenvolvimento: aprende a estrutura de tokens AdvPL e os idiomas algoritmicos do
    corpus (fontes do repo + algos_advpl.prw), gerando/completando codigo AdvPL.
    Mesmo NPLM do pt_neural (Embedding->Reshape->Linear->Tanh->Linear->SoftmaxCE,
    Adam via Fit), mas a unidade e o TOKEN AdvPL (lexer proprio), nao o byte.
    Uso: treina, auto-teste (loss cai), gera de prefixos e abre um REPL de autocomplete.
    Teto honesto: modelo pequeno em VM interpretado — gera codigo plausivel enviesado
    a logica, NAO resolve problemas novos (isso exige um LLM grande pre-treinado).
    @type  user function
    @author AdvPP
    @since 2026-07-22
/*/
User Function DevNN()
    Local cCorpus  := ""
    Local lReal    := .F.
    Local aTokens  := {}
    Local aVoc     := {}
    Local aId2Tok  := {}
    Local oTok2Id  := Nil
    Local aIds     := {}
    Local aEx      := {}
    Local aX       := {}
    Local aAlvo    := {}
    Local nN       := 0
    Local nK       := 3
    Local nD       := 16
    Local nH       := 32
    Local nV       := 0
    Local nTopN    := 120
    Local nLR      := 0.05
    Local nEpocas  := 120
    Local nMax     := 600
    Local nGerar   := 30
    Local oEmb     := Nil
    Local oL1      := Nil
    Local oL2      := Nil
    Local oOpt     := Nil
    Local nInicial := 0
    Local nFinal   := 0
    Local cSeed    := ""

    If File("tests/llm/code_corpus.txt")
        cCorpus := MemoRead("tests/llm/code_corpus.txt")
        lReal   := .T.
        nK      := 4
        nD      := 32
        nH      := 128
        nTopN   := 300
        nEpocas := 120
        nMax    := 2500
        nGerar  := 40
    Else
        cCorpus := MiniCode()
    EndIf

    aTokens := Lex(cCorpus)
    aVoc    := BuildVocab(aTokens, nTopN)
    aId2Tok := aVoc[1]
    oTok2Id := aVoc[2]
    nV      := Len(aId2Tok)
    aIds    := Encode(aTokens, oTok2Id)
    aEx     := BuildExamples(aIds, nK, nMax)
    aX      := aEx[1]
    aAlvo   := aEx[2]
    nN      := aEx[3]

    ConOut("tokens=" + Str(Len(aTokens),7) + " vocab=" + Str(nV,4) + " exemplos=" + Str(nN,6) + ;
           " (k=" + Str(nK,1) + " D=" + Str(nD,3) + " H=" + Str(nH,4) + ")")
    If nN < 2
        ConOut("FALHA: corpus pequeno demais.")
        Return
    EndIf

    oEmb := Embedding():New(nV, nD)
    oL1  := Linear():New(nK * nD, nH)
    oL2  := Linear():New(nH, nV)
    oOpt := Adam():New(Params3(oEmb, oL1, oL2), nLR)

    nInicial := LossOnly(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD)
    nFinal   := Fit({|| Passo(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD, oOpt) }, nEpocas)

    ConOut("loss " + Str(nInicial,11,5) + " -> " + Str(nFinal,11,5))

    // Demonstracao: gera a partir de prefixos AdvPL fixos.
    ConOut("--- geracao ---")
    ConOut("[Local nTotal] " + Gera(oEmb,oL1,oL2,aId2Tok,oTok2Id,"Local nTotal", nK,nD,nGerar,0.7))
    ConOut("[For i := 1] "   + Gera(oEmb,oL1,oL2,aId2Tok,oTok2Id,"For i := 1", nK,nD,nGerar,0.7))
    ConOut("[If a[j] >] "    + Gera(oEmb,oL1,oL2,aId2Tok,oTok2Id,"If a[j] >", nK,nD,nGerar,0.7))

    // Auto-teste deterministico (nao depende do REPL).
    If nFinal < nInicial * 0.5
        ConOut("OK: dev_nn treinou (token-level Embedding + Reshape + Adam + Fit).")
    Else
        ConOut("FALHA: loss nao caiu o suficiente.")
    EndIf

    // REPL de autocomplete: digite um prefixo AdvPL; enter vazio ou 'sair' encerra.
    // Sem stdin (EOF) o loop encerra na hora — o auto-teste acima ja rodou.
    ConOut("--- REPL autocomplete (prefixo AdvPL; 'sair' para encerrar) ---")
    Do While .T.
        cSeed := ConIn("advpl> ")
        If Empty(cSeed) .Or. Lower(AllTrim(cSeed)) == "sair"
            Exit
        EndIf
        ConOut("  " + Gera(oEmb,oL1,oL2,aId2Tok,oTok2Id, cSeed, nK, nD, nGerar, 0.7))
    EndDo
Return

// ================================================================ Lexer AdvPL
// Quebra o fonte em tokens: keywords/identificadores, numeros, strings, operadores
// multi/mono-char e <nl> para quebras de linha. Comentarios sao ignorados.
Static Function Lex(cSrc)
    Local aTok := {}
    Local i    := 1
    Local nLen := Len(cSrc)
    Local ch   := ""
    Local c2   := ""
    Local cTk  := ""
    Local nLin := 0

    Do While i <= nLen
        ch := SubStr(cSrc, i, 1)
        c2 := SubStr(cSrc, i, 2)

        // comentario de linha //
        If c2 == "//"
            Do While i <= nLen .And. SubStr(cSrc, i, 1) != Chr(10)
                i++
            EndDo
            Loop
        EndIf
        // comentario de bloco /* ... */
        If c2 == "/*"
            i += 2
            Do While i <= nLen .And. SubStr(cSrc, i, 2) != "*/"
                i++
            EndDo
            i += 2
            Loop
        EndIf
        // quebra de linha -> <nl> (colapsa multiplas)
        If ch == Chr(10)
            If Len(aTok) > 0 .And. aTok[Len(aTok)] != "<nl>"
                aAdd(aTok, "<nl>")
            EndIf
            i++
            Loop
        EndIf
        // espacos em branco
        If ch == " " .Or. ch == Chr(9) .Or. ch == Chr(13)
            i++
            Loop
        EndIf
        // string "..." ou '...'
        If ch == '"' .Or. ch == "'"
            cTk := ch
            nLin := i + 1
            Do While nLin <= nLen .And. SubStr(cSrc, nLin, 1) != ch
                cTk += SubStr(cSrc, nLin, 1)
                nLin++
            EndDo
            cTk += ch
            aAdd(aTok, cTk)
            i := nLin + 1
            Loop
        EndIf
        // operador logico .T. .F. .And. .Or. .Not.
        If ch == "."
            cTk := LogicOp(cSrc, i, nLen)
            If !Empty(cTk)
                aAdd(aTok, cTk)
                i += Len(cTk)
                Loop
            EndIf
        EndIf
        // numero
        If EhDigito(ch)
            cTk := ""
            Do While i <= nLen .And. (EhDigito(SubStr(cSrc,i,1)) .Or. SubStr(cSrc,i,1) == ".")
                cTk += SubStr(cSrc, i, 1)
                i++
            EndDo
            aAdd(aTok, cTk)
            Loop
        EndIf
        // identificador / keyword
        If EhAlfa(ch)
            cTk := ""
            Do While i <= nLen .And. EhAlnum(SubStr(cSrc,i,1))
                cTk += SubStr(cSrc, i, 1)
                i++
            EndDo
            aAdd(aTok, cTk)
            Loop
        EndIf
        // operador de 2 chars
        If EhOp2(c2)
            aAdd(aTok, c2)
            i += 2
            Loop
        EndIf
        // operador / pontuacao de 1 char
        aAdd(aTok, ch)
        i++
    EndDo
Return aTok

// Classificacao por codigo (Asc): o operador >= em strings no AdvPP nao e
// lexicografico (quirk SET EXACT), entao comparamos os codigos numericos.
Static Function EhDigito(ch)
    Local n := Asc(ch)
Return n >= 48 .And. n <= 57

Static Function EhAlfa(ch)
    Local n := Asc(ch)
Return (n >= 65 .And. n <= 90) .Or. (n >= 97 .And. n <= 122) .Or. n == 95

Static Function EhAlnum(ch)
Return EhAlfa(ch) .Or. EhDigito(ch)

Static Function EhOp2(c2)
Return c2 == ":=" .Or. c2 == "==" .Or. c2 == "!=" .Or. c2 == "<=" .Or. c2 == ">=" .Or. ;
       c2 == "->" .Or. c2 == "++" .Or. c2 == "--" .Or. c2 == "+=" .Or. c2 == "-=" .Or. ;
       c2 == "*=" .Or. c2 == "/=" .Or. c2 == "::" .Or. c2 == "<>"

// Reconhece .T. .F. .And. .Or. .Not. a partir de i; devolve o token ou "".
Static Function LogicOp(cSrc, i, nLen)
    Local cUp := Upper(SubStr(cSrc, i, 6))
    If SubStr(cUp, 1, 3) == ".T." .Or. SubStr(cUp, 1, 3) == ".F."
        Return SubStr(cSrc, i, 3)
    ElseIf SubStr(cUp, 1, 5) == ".AND." .Or. SubStr(cUp, 1, 5) == ".NOT."
        Return SubStr(cSrc, i, 5)
    ElseIf SubStr(cUp, 1, 4) == ".OR."
        Return SubStr(cSrc, i, 4)
    EndIf
Return ""

// ================================================================ Vocab / ids
// Frequencia via JsonObject (hash). Top-N por contagem + <unk>. Ids 1-based.
Static Function BuildVocab(aTokens, nTopN)
    Local oCnt    := JsonObject():New()
    Local aChaves := {}
    Local aFreq   := {}
    Local aOrd    := {}
    Local aId2Tok := {}
    Local oTok2Id := JsonObject():New()
    Local i       := 0
    Local t       := ""
    Local nLim    := 0

    For i := 1 To Len(aTokens)
        t := aTokens[i]
        // Nil-check em vez de HasProperty (HasProperty com chave-variavel falha no AdvPP)
        If oCnt[t] == Nil
            oCnt[t] := 1
            aAdd(aChaves, t)
        Else
            oCnt[t] := oCnt[t] + 1
        EndIf
    Next i
    // ordena chaves por frequencia desc (indices num array paralelo)
    For i := 1 To Len(aChaves)
        aAdd(aFreq, oCnt[aChaves[i]])
        aAdd(aOrd, i)
    Next i
    aSort(aOrd, , , {|x, y| aFreq[x] > aFreq[y] })

    aAdd(aId2Tok, "<unk>")
    oTok2Id["<unk>"] := 1
    nLim := nTopN
    If nLim > Len(aOrd)
        nLim := Len(aOrd)
    EndIf
    For i := 1 To nLim
        t := aChaves[aOrd[i]]
        aAdd(aId2Tok, t)
        oTok2Id[t] := Len(aId2Tok)
    Next i
Return {aId2Tok, oTok2Id}

Static Function Encode(aTokens, oTok2Id)
    Local aIds := {}
    Local i    := 0
    Local t    := ""
    Local nId  := 0
    For i := 1 To Len(aTokens)
        t := aTokens[i]
        nId := oTok2Id[t]
        If nId == Nil
            aAdd(aIds, 1)                 // <unk>
        Else
            aAdd(aIds, nId)
        EndIf
    Next i
Return aIds

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

// ================================================================ NPLM (reusa o stack)
Static Function FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD)
    Local oE := oEmb:Forward(aX)
    Local oR := oE:Reshape({nN, nK * nD})
    Local oH := oL1:Forward(oR):Tanh()
Return oL2:Forward(oH)

Static Function LossOnly(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD)
Return FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD):SoftmaxCE(aAlvo):Value():ToArray()[1]

Static Function Passo(oEmb, oL1, oL2, aX, aAlvo, nN, nK, nD, oOpt)
    Local oLoss := FwdModel(oEmb, oL1, oL2, aX, nN, nK, nD):SoftmaxCE(aAlvo)
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

// ================================================================ Geracao / des-tokenizacao
Static Function Gera(oEmb, oL1, oL2, aId2Tok, oTok2Id, cSeed, nK, nD, nGerar, nTemp)
    Local aCtx     := {}
    Local aSeedIds := Encode(Lex(cSeed), oTok2Id)
    Local nV       := Len(aId2Tok)
    Local i        := 0
    Local j        := 0
    Local nId      := 0
    Local cTok     := ""
    Local cOut     := cSeed

    For i := 1 To nK
        aAdd(aCtx, 1)
    Next i
    For i := 1 To Len(aSeedIds)
        For j := 1 To nK - 1
            aCtx[j] := aCtx[j + 1]
        Next j
        aCtx[nK] := aSeedIds[i]
    Next i

    For i := 1 To nGerar
        nId  := Sample(FwdModel(oEmb, oL1, oL2, aCtx, 1, nK, nD):Value():ToArray(), nV, nTemp)
        cTok := aId2Tok[nId]
        cOut += Detok(cTok)
        For j := 1 To nK - 1
            aCtx[j] := aCtx[j + 1]
        Next j
        aCtx[nK] := nId
    Next i
Return cOut

// Junta um token ao texto: <nl>->newline; pontuacao de fechamento cola; resto com espaco.
Static Function Detok(cTok)
    If cTok == "<nl>"
        Return Chr(10)
    ElseIf cTok == "<unk>"
        Return " ?"
    ElseIf cTok $ ")]},;."
        Return cTok
    EndIf
Return " " + cTok

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

// Mini-corpus de codigo deterministico (auto-teste sem corpus externo).
Static Function MiniCode()
    Local c := ""
    Local i := 0
    For i := 1 To 30
        c += "Static Function Soma(a, b)" + Chr(10)
        c += "Local nTotal := 0" + Chr(10)
        c += "nTotal := a + b" + Chr(10)
        c += "Return nTotal" + Chr(10)
    Next i
Return c
