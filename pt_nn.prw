/*
 * pt_nn.prw - LLM híbrido Markov + rede neural TERNÁRIA, 100% em AdvPL (AdvPP).
 *
 * Arquitetura (Caminho B - "topo" que fica integralmente em AdvPL):
 *   - Camada escondida = projeção aleatória TERNÁRIA fixa (W_proj em {-1,0,+1}).
 *     Não é treinada; projeta o contexto em add/sub via o native BLAS MatVecTern.
 *     Contexto POSICIONAL: x tem NCTX blocos de V dims (a ordem importa,
 *     "a tecnologia" != "tecnologia a").
 *   - Ativação ternária: h = sign(W_proj . x)  em {-1,0,+1}.
 *   - Camada de saída U (aprendida) treinada por PERCEPTRON multiclasse — puro
 *     add/sub, sem gradiente/float. scores = U . h  (também via MatVecTern).
 *   Isto é uma Extreme Learning Machine ternária: hidden aleatório, saída linear
 *   aprendida — treina E infere sem multiplicação, tudo em AdvPL.
 *   - Markov n-grama (nível de palavra) dá o prior local; a rede generaliza para
 *     contextos não vistos. A geração MISTURA os dois.
 *
 * O "BLAS ternário" é o native MatVecTern(aMat, aVecTern): result[i] =
 * Σ_j sign(vec[j])*mat[i][j], multiply-free. Estado do modelo vive num JsonObject
 * passado explícito (a VM não propaga Private entre chamadas).
 *
 * Rodar/testar: ./advplc run pt_nn.prw
 *
 * ponytail: dims modestas (hidden 32, contexto 2 palavras) e corpus embutido;
 * qualidade escala com dados — troque Corpus() por MemoRead("corpus.txt").
 */

#define NHID   64
#define NCTX   2
#define PASSES 30

User Function PtNN()
    Local oM := JsonObject():New()

    ConOut("=================================================================")
    ConOut(" LLM híbrido Markov + rede TERNÁRIA (ELM) - AdvPL puro (AdvPP)")
    ConOut("=================================================================")

    BuildVocab(oM, LoadCorpus())
    ConOut("Vocabulário: " + Str(oM["V"],4) + " palavras | tokens: " + Str(Len(oM["stream"]),5))

    InitParams(oM)
    BuildMarkov(oM, oM["stream"])
    ConOut("Projeção ternária " + Str(NHID,3) + "x" + Str(NCTX * oM["V"],5) + " (posicional) | saída U " + Str(oM["V"],4) + "x" + Str(NHID,3))
    ConOut("")

    Train(oM)
    ConOut("")

    ConOut("--- Geração híbrida (Markov + rede ternária) ---")
    Gera(oM, "o brasil")
    Gera(oM, "a tecnologia")
    Gera(oM, "na cidade")
    ConOut("")

    SelfTest(oM)
Return

// Corpus externo (corpus.txt) se existir, senão o embutido. Escala via MemoRead.
Static Function LoadCorpus()
    If File("corpus.txt")
        Return MemoRead("corpus.txt")
    EndIf
Return Corpus()

// ---------------------------------------------------------------- Vocabulário
Static Function BuildVocab(oM, cCorpus)
    Local aTok := Tokenize(cCorpus)
    Local jW2Id := JsonObject():New()
    Local aId2W := {}
    Local aStream := {}
    Local i := 0
    Local cW := ""

    // id 1 reservado para <s> (início de frase / padding de contexto)
    aAdd(aId2W, "<s>")
    jW2Id["<s>"] := 1

    For i := 1 To Len(aTok)
        cW := aTok[i]
        If ValType(jW2Id[cW]) == "U"
            aAdd(aId2W, cW)
            jW2Id[cW] := Len(aId2W)
        EndIf
        aAdd(aStream, jW2Id[cW])
    Next i

    oM["V"]      := Len(aId2W)
    oM["id2w"]   := aId2W
    oM["w2id"]   := jW2Id
    oM["stream"] := aStream
Return

// Tokeniza: minúsculas+sem acento; palavras [a-z0-9] e "." como token próprio.
Static Function Tokenize(cText)
    Local aW  := {}
    Local cN  := Fold(cText)
    Local nL  := Len(cN)
    Local i   := 0
    Local cCh := ""
    Local cCur := ""
    Local n := 0

    For i := 1 To nL
        cCh := SubStr(cN, i, 1)
        n := Asc(cCh)
        If (n >= 97 .And. n <= 122) .Or. (n >= 48 .And. n <= 57)
            cCur += cCh
        Else
            If Len(cCur) > 0
                aAdd(aW, cCur)
                cCur := ""
            EndIf
            If cCh == "."
                aAdd(aW, ".")
            EndIf
        EndIf
    Next i
    If Len(cCur) > 0
        aAdd(aW, cCur)
    EndIf
Return aW

Static Function Fold(c)
    c := Lower(c)
    c := StrTran(c, "á", "a") ; c := StrTran(c, "à", "a") ; c := StrTran(c, "â", "a") ; c := StrTran(c, "ã", "a")
    c := StrTran(c, "é", "e") ; c := StrTran(c, "ê", "e")
    c := StrTran(c, "í", "i")
    c := StrTran(c, "ó", "o") ; c := StrTran(c, "ô", "o") ; c := StrTran(c, "õ", "o")
    c := StrTran(c, "ú", "u") ; c := StrTran(c, "ü", "u")
    c := StrTran(c, "ç", "c")
Return c

// --------------------------------------------------------------- Parâmetros
Static Function InitParams(oM)
    Local nV := oM["V"]
    Local aWP := {}    // NHID linhas x nV colunas, ternária {-1,0,+1}, fixa
    Local aU  := {}    // nV linhas x NHID colunas, inteira, aprendida (zeros)
    Local i := 0
    Local j := 0
    Local aRow := {}
    Local nR := 0

    For i := 1 To NHID
        aRow := {}
        For j := 1 To NCTX * nV             // contexto posicional: NCTX blocos de nV
            nR := Random(3)                 // 1,2,3
            aAdd(aRow, If(nR == 1, -1, If(nR == 2, 0, 1)))
        Next j
        aAdd(aWP, aRow)
    Next i

    For i := 1 To nV
        aRow := {}
        For j := 1 To NHID
            aAdd(aRow, 0)
        Next j
        aAdd(aU, aRow)
    Next i

    oM["wproj"] := aWP
    oM["U"]     := aU
Return

// ------------------------------------------------------------------- Markov
// Hash "id1 id2" -> array de próximos ids (multiset = frequência embutida).
Static Function BuildMarkov(oM, aStream)
    Local jM := JsonObject():New()
    Local i := 0
    Local cKey := ""
    Local nNext := 0

    For i := 1 To Len(aStream) - NCTX
        cKey := CtxKey(aStream, i)
        nNext := aStream[i + NCTX]
        If ValType(jM[cKey]) == "U"
            jM[cKey] := {}
        EndIf
        aAdd(jM[cKey], nNext)
    Next i
    oM["markov"] := jM
Return

Static Function CtxKey(aStream, nPos)
    Local c := ""
    Local k := 0
    For k := 0 To NCTX - 1
        c += Str(aStream[nPos + k], 5) + " "
    Next k
Return c

// ------------------------------------------------------------------ Forward
// Retorna { aScores (nV), aH (NHID ternário) } para um array de ids de contexto.
Static Function Forward(oM, aCtxIds)
    Local nV := oM["V"]
    Local aX := Array(NCTX * nV)
    Local aHraw := Nil
    Local aH := {}
    Local aScores := Nil
    Local i := 0

    AFill(aX, 0)
    For i := 1 To Len(aCtxIds)              // posição i -> bloco (i-1)*nV
        If aCtxIds[i] >= 1 .And. aCtxIds[i] <= nV
            aX[(i - 1) * nV + aCtxIds[i]] := 1
        EndIf
    Next i

    aHraw := MatVecTern(oM["wproj"], aX)        // BLAS ternária: W_proj . x
    For i := 1 To Len(aHraw)
        aAdd(aH, If(aHraw[i] > 0, 1, If(aHraw[i] < 0, -1, 0)))   // ativação ternária
    Next i

    aScores := MatVecTern(oM["U"], aH)          // BLAS ternária: U . h
Return {aScores, aH}

Static Function Argmax(aScores)
    Local nBest := aScores[1]
    Local nIdx := 1
    Local i := 0
    For i := 2 To Len(aScores)
        If aScores[i] > nBest
            nBest := aScores[i]
            nIdx := i
        EndIf
    Next i
Return nIdx

// -------------------------------------------------------------------- Treino
// Perceptron multiclasse: erra -> U[alvo]+=h, U[previsto]-=h. Puro add/sub.
Static Function Train(oM)
    Local aStream := oM["stream"]
    Local aU := oM["U"]
    Local nP := 0
    Local i := 0
    Local j := 0
    Local nErr := 0
    Local nErr1 := -1
    Local aCtx := {}
    Local aFwd := Nil
    Local aH := Nil
    Local nPred := 0
    Local nAlvo := 0
    Local aRowA := Nil
    Local aRowP := Nil

    ConOut("Treinando (perceptron ternário, " + Str(PASSES,2) + " passadas):")
    For nP := 1 To PASSES
        nErr := 0
        For i := 1 To Len(aStream) - NCTX
            aCtx := {}
            For j := 0 To NCTX - 1
                aAdd(aCtx, aStream[i + j])
            Next j
            nAlvo := aStream[i + NCTX]

            aFwd := Forward(oM, aCtx)
            nPred := Argmax(aFwd[1])
            aH := aFwd[2]

            If nPred != nAlvo
                nErr++
                aRowA := aU[nAlvo]
                aRowP := aU[nPred]
                For j := 1 To NHID
                    aRowA[j] := aRowA[j] + aH[j]
                    aRowP[j] := aRowP[j] - aH[j]
                Next j
            EndIf
        Next i
        If nErr1 == -1
            nErr1 := nErr
        EndIf
        If nP == 1 .Or. nP == PASSES .Or. Mod(nP, 5) == 0
            ConOut("  passada " + Str(nP,2) + ": " + Str(nErr,5) + " erros")
        EndIf
    Next nP
    oM["err_first"] := nErr1
    oM["err_last"]  := nErr
Return

// ------------------------------------------------------------------ Geração
// A cada passo mistura Markov (frequência) + rede ternária (rerank aprendido),
// com anti-repetição e sharpening, e amostra proporcional ao peso.
Static Function Gera(oM, cSeed)
    Local aCtx := SeedCtx(oM, cSeed)
    Local cOut := cSeed
    Local i := 0
    Local nNext := 0
    Local nPrev := 0

    For i := 1 To 30
        nNext := NextWord(oM, aCtx, nPrev, i > 5)   // so permite "." apos 5 palavras
        If nNext == 0 .Or. oM["id2w"][nNext] == "."
            cOut += "."
            Exit
        EndIf
        cOut += " " + oM["id2w"][nNext]
        nPrev := nNext
        aCtx := ShiftCtx(AClone(aCtx), nNext)
    Next i
    ConOut("  [" + cSeed + "] -> " + cOut)
Return

// NextWord: constrói pesos por candidato e amostra. Markov dá peso = frequência;
// a rede soma um bônus por rank (top-K); penaliza a palavra anterior (anti-loop);
// sharpening (peso^2) reduz ruído. Contexto não visto => só a rede decide.
Static Function NextWord(oM, aCtx, nPrev, lAllowEnd)
    Local jM      := oM["markov"]
    Local aScores := Forward(oM, aCtx)[1]
    Local aTop    := TopK(aScores, 6)
    Local cKey    := ""
    Local aMk     := Nil
    Local aIds    := {}    // candidatos únicos
    Local aW      := {}    // peso paralelo
    Local j       := 0
    Local id      := 0
    Local nPeriod := 0

    If ValType(oM["w2id"]["."]) != "U"
        nPeriod := oM["w2id"]["."]
    EndIf

    For j := 1 To NCTX
        cKey += Str(aCtx[j], 5) + " "
    Next j

    // Markov: cada ocorrência soma 1.0 ao peso do id
    aMk := jM[cKey]
    If ValType(aMk) == "A"
        For j := 1 To Len(aMk)
            AddCand(aIds, aW, aMk[j], 1.0)
        Next j
    EndIf
    // Rede ternária: bônus por rank (top-1 vale mais). Peso 1.6 calibra vs Markov.
    For j := 1 To Len(aTop)
        AddCand(aIds, aW, aTop[j], 1.6 * (Len(aTop) - j + 1) / Len(aTop))
    Next j

    // Anti-repetição + descarta <s>; sharpening por quadrado.
    For j := 1 To Len(aIds)
        id := aIds[j]
        If id == 1                          // <s> nunca é gerado
            aW[j] := 0
        ElseIf id == nPeriod .And. !lAllowEnd
            aW[j] := 0                       // frase curta demais: sem ponto final ainda
        ElseIf id == nPrev
            aW[j] := aW[j] * 0.10           // pune repetir a palavra anterior
        EndIf
        aW[j] := aW[j] * aW[j]              // sharpening: favorece os fortes
    Next j
Return WSample(aIds, aW)

// AddCand: acumula peso do id na lista de candidatos (in-place).
Static Function AddCand(aIds, aW, id, nAdd)
    Local p := aScan(aIds, id)
    If p == 0
        aAdd(aIds, id)
        aAdd(aW, nAdd)
    Else
        aW[p] := aW[p] + nAdd
    EndIf
Return

// WSample: amostra um id proporcional ao peso (roleta). 0 se lista vazia.
Static Function WSample(aIds, aW)
    Local nTot := 0
    Local i := 0
    Local nR := 0
    Local nAcc := 0
    For i := 1 To Len(aW)
        nTot += aW[i]
    Next i
    If nTot <= 0
        Return If(Len(aIds) > 0, aIds[1], 0)
    EndIf
    nR := (Random(100000) / 100000) * nTot
    For i := 1 To Len(aIds)
        nAcc += aW[i]
        If nR <= nAcc
            Return aIds[i]
        EndIf
    Next i
Return aIds[Len(aIds)]

Static Function SeedCtx(oM, cSeed)
    Local aTok := Tokenize(cSeed)
    Local jW := oM["w2id"]
    Local aCtx := {}
    Local i := 0
    Local id := 0
    // preenche com <s> (id 1) à esquerda se o seed for curto
    For i := 1 To NCTX
        aAdd(aCtx, 1)
    Next i
    For i := 1 To Len(aTok)
        If ValType(jW[aTok[i]]) != "U"
            id := jW[aTok[i]]
            aCtx := ShiftCtx(aCtx, id)
        EndIf
    Next i
Return aCtx

Static Function ShiftCtx(aCtx, nNew)
    Local i := 0
    For i := 1 To NCTX - 1
        aCtx[i] := aCtx[i + 1]
    Next i
    aCtx[NCTX] := nNew
Return aCtx

Static Function TopK(aScores, k)
    Local aIdx := {}
    Local aUsed := Array(Len(aScores))
    Local n := 0
    Local i := 0
    Local nBest := 0
    Local nBI := 0
    AFill(aUsed, .F.)
    For n := 1 To k
        nBest := -999999
        nBI := 0
        For i := 1 To Len(aScores)
            If !aUsed[i] .And. aScores[i] > nBest
                nBest := aScores[i]
                nBI := i
            EndIf
        Next i
        If nBI > 0
            aUsed[nBI] := .T.
            aAdd(aIdx, nBI)
        EndIf
    Next n
Return aIdx

// ----------------------------------------------------------------- Auto-teste
Static Function SelfTest(oM)
    Local nFail := 0
    Local aFwd := Nil

    ConOut("--- auto-teste ---")

    // 1. BLAS ternária: sanidade do multiply-free
    If MatVecTern({{5, 7, 9}}, {1, 0, -1})[1] != -4    // 5 - 9
        ConOut("FALHA: MatVecTern incorreto"); nFail++
    EndIf

    // 2. Forward produz vetores dos tamanhos certos
    aFwd := Forward(oM, {1, 1})
    If Len(aFwd[1]) != oM["V"] .Or. Len(aFwd[2]) != NHID
        ConOut("FALHA: dimensoes do forward"); nFail++
    EndIf

    // 3. APRENDIZADO: o perceptron reduziu erros da 1a para a ultima passada
    If oM["err_last"] >= oM["err_first"]
        ConOut("FALHA: modelo nao aprendeu (erros nao cairam)"); nFail++
    Else
        ConOut("aprendizado OK: erros " + Str(oM["err_first"],5) + " -> " + Str(oM["err_last"],5))
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1) + " erro(s).")
    EndIf
Return

// Corpus PT-BR embutido (troque por MemoRead para escalar).
Static Function Corpus()
    Local c := ""
    c += "o brasil e um pais de dimensoes continentais com uma cultura rica e diversa. "
    c += "a lingua portuguesa e falada por milhoes de pessoas em todo o territorio nacional. "
    c += "na cidade de sao paulo o transito costuma ser intenso durante toda a semana. "
    c += "no rio de janeiro as praias atraem turistas do mundo inteiro o ano todo. "
    c += "a tecnologia mudou a forma como as pessoas trabalham e se comunicam hoje em dia. "
    c += "a educacao e a base para o desenvolvimento de qualquer sociedade moderna. "
    c += "o cafe da manha e a refeicao mais importante para muitas familias brasileiras. "
    c += "no interior do pais a agricultura movimenta a economia de pequenas cidades. "
    c += "a floresta amazonica abriga uma biodiversidade unica e precisa ser preservada. "
    c += "a musica popular brasileira encanta pessoas de todas as idades e regioes. "
    c += "o futebol e uma paixao nacional que une torcedores em todo o pais. "
    c += "a saude publica e um direito de todos os cidadaos brasileiros. "
    c += "o trabalho em equipe e fundamental para alcancar bons resultados. "
    c += "a leitura de bons livros amplia o conhecimento e estimula a imaginacao. "
    c += "a internet conecta pessoas de diferentes lugares e facilita o acesso a informacao. "
    c += "os jovens buscam oportunidades de estudo e de trabalho para construir o futuro. "
    c += "cada regiao do brasil tem costumes sotaques e tradicoes que enriquecem a cultura. "
    c += "a familia se reune no fim de semana para conversar e celebrar a vida. "
    c += "a tecnologia moderna transformou a economia e o mercado de trabalho no pais. "
    c += "o brasil e um pais jovem cheio de energia e de oportunidades para todos. "
    c += "as escolas publicas precisam de mais investimento e de professores valorizados. "
    c += "os rios brasileiros sao fontes de agua e de energia para o povo do campo. "
    c += "a culinaria regional oferece pratos saborosos feitos com ingredientes locais. "
    c += "muitas pessoas gostam de viajar durante as ferias para conhecer novos lugares. "
    c += "os hospitais atendem milhares de pacientes todos os dias em cada estado. "
    c += "as bibliotecas guardam historias e memorias de muitas geracoes passadas. "
    c += "o comercio local gera empregos e fortalece a economia dos bairros das cidades. "
    c += "a natureza oferece paisagens belas que devem ser cuidadas com muito respeito. "
    c += "o respeito ao proximo e a base de uma convivencia pacifica entre as pessoas. "
    c += "a ciencia e a pesquisa impulsionam o progresso e melhoram a vida das pessoas. "
    c += "o transporte publico e essencial para a mobilidade nas grandes cidades. "
    c += "a arte e a cultura expressam a identidade e a alma de um povo diverso. "
    c += "o campo e a cidade se completam na economia e na vida do pais. "
    c += "a agua limpa e um recurso precioso que precisa ser usado com consciencia. "
    c += "as festas populares reunem familias e amigos em todas as regioes do brasil. "
    c += "o conhecimento se constroi com estudo trabalho e muita dedicacao diaria. "
    c += "a energia solar e uma fonte limpa que cresce no brasil a cada ano. "
    c += "as criancas aprendem brincando e descobrindo o mundo ao seu redor. "
Return c
