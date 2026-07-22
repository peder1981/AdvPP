/*
 * pt_nn.prw - LLM híbrido Markov + rede neural TERNÁRIA, 100% em AdvPL (AdvPP).
 *
 * Arquitetura (Caminho B - "topo" que fica integralmente em AdvPL):
 *   - Camada escondida = projeção aleatória TERNÁRIA fixa (W_proj em {-1,0,+1}).
 *     Não é treinada; projeta o contexto em add/sub via o native BLAS MatVecTern.
 *   - Ativação ternária: h = sign(W_proj . x)  em {-1,0,+1}.
 *   - Camada de saída U (aprendida) treinada por PERCEPTRON MÉDIO (Collins 2002)
 *     — puro add/sub, sem gradiente/float. scores = U . h (também via MatVecTern).
 *   Isto é uma Extreme Learning Machine ternária: hidden aleatório, saída linear
 *   aprendida — treina E infere sem multiplicação, tudo em AdvPL.
 *   - Markov n-grama (nível de palavra) INTERPOLADO (Jelinek-Mercer: mistura as
 *     ordens 1..NCTX) dá o prior; a rede generaliza para contextos não vistos.
 *     A geração mistura os dois e amostra por NUCLEUS (top-p).
 *
 * JANELA LONGA (entrada e saída até CTXMAX=4096 tokens CADA):
 *   - Entrada ternária = NCTX blocos LOCAIS posicionais (a ordem importa,
 *     "a tecnologia" != "tecnologia a") + 1 bloco BAG de presença sobre as
 *     últimas BAGW (até 4096) palavras — o long-context. O bag é mantido
 *     INCREMENTALMENTE (contagem por palavra, desliza a janela em O(delta)),
 *     custo amortizado O(1) por token.
 *   - SeedSeq aceita um seed de até CTXMAX tokens; Gera produz um DOCUMENTO
 *     multi-frase de até CTXMAX tokens.
 *   Nota honesta: atenção real sobre 4096 tokens exigiria ponto flutuante
 *   (inviável multiply-free em AdvPL); o bag é a aproximação de custo limitado.
 *
 * Algoritmos modernos: perceptron médio, suavização interpolada, amostragem
 * nucleus, contexto posicional + bag long-context, vocabulário limitado por
 * frequência (top-N + <unk>) e amostra de treino por stride — os dois últimos
 * deixam o custo do treino LIMITADO, independente do tamanho do corpus.
 *
 * O "BLAS ternário" é o native MatVecTern(aMat, aVecTern): result[i] =
 * Σ_j sign(vec[j])*mat[i][j], multiply-free. Estado do modelo vive num JsonObject
 * passado explícito (a VM não propaga Private entre chamadas).
 *
 * Corpus: usa corpus.txt (via MemoRead) se existir; senão o Corpus() embutido.
 * Rodar/testar: ./advplc run pt_nn.prw
 */

#define NHID     64
#define NCTX     3        // ordem do Markov e contexto local posicional do neural
#define BAGW     4096     // janela do "saco de contexto" (long-context do neural): até 4096 tokens
#define CTXMAX   4096     // máximo de tokens de ENTRADA (seed) e de SAÍDA (geração)
#define PASSES   20
#define MAXVOCAB 1200     // teto do vocabulário (top-N por frequência; resto -> <unk>)
#define MAXTRAIN 1000     // teto de posições de treino do perceptron (stride no corpus)
#define TOPP     0.92     // amostragem nucleus: menor conjunto com massa >= TOPP
#define REPWIN   14       // janela de anti-repetição (palavras recentes)
#define REPPEN   9        // força da penalidade de repetição por ocorrência recente

User Function PtNN()
    Local oM := JsonObject():New()

    ConOut("=================================================================")
    ConOut(" LLM híbrido Markov + rede TERNÁRIA (ELM) - AdvPL puro (AdvPP)")
    ConOut("=================================================================")

    BuildVocab(oM, LoadCorpus())
    ConOut("Vocabulário: " + Str(oM["V"],4) + " palavras | tokens: " + Str(Len(oM["stream"]),5))

    InitParams(oM)
    BuildMarkov(oM, oM["stream"])
    ConOut("Entrada ternária " + Str(NHID,3) + "x" + Str((NCTX + 1) * oM["V"],6) + " (" + Str(NCTX,1) + " blocos locais + bag ate " + Str(BAGW,4) + " tokens) | saída U " + Str(oM["V"],4) + "x" + Str(NHID,3))
    ConOut("Janela: contexto até " + Str(CTXMAX,4) + " tokens (entrada) e geração até " + Str(CTXMAX,4) + " tokens (saída).")
    ConOut("")

    Train(oM)
    ConOut("")

    ConOut("--- Geração híbrida (documento multi-frase, janela até " + Str(CTXMAX,4) + " tokens) ---")
    Gera(oM, "uma noite", 200)                 // seeds ajustados ao corpus atual
    Gera(oM, "os olhos", 120)
    Gera(oM, "minha mae", 120)

    SelfTest(oM)
Return

// Corpus externo se existir, senão o embutido. Escala via MemoRead.
// Procura tests/llm/corpus.txt (rodando da raiz) e corpus.txt (rodando da pasta).
Static Function LoadCorpus()
    If File("tests/llm/corpus.txt")
        Return MemoRead("tests/llm/corpus.txt")
    ElseIf File("corpus.txt")
        Return MemoRead("corpus.txt")
    EndIf
Return Corpus()

// ---------------------------------------------------------------- Vocabulário
// Conta frequências, mantém as MAXVOCAB palavras mais frequentes (ASort por
// contagem) e mapeia o resto para <unk>. Vocabulário limitado => custo por
// passo do forward fixo, independente do tamanho do corpus.
Static Function BuildVocab(oM, cCorpus)
    Local aTok := Tokenize(cCorpus)
    Local jSeen := JsonObject():New()   // palavra -> índice em aPairs
    Local aPairs := {}                  // { palavra, contagem }
    Local jW2Id := JsonObject():New()
    Local aId2W := {}
    Local aStream := {}
    Local i := 0
    Local cW := ""
    Local nIx := 0

    // conta frequências (uma passada)
    For i := 1 To Len(aTok)
        cW := aTok[i]
        If ValType(jSeen[cW]) == "U"
            aAdd(aPairs, {cW, 1})
            jSeen[cW] := Len(aPairs)
        Else
            nIx := jSeen[cW]
            aPairs[nIx][2] := aPairs[nIx][2] + 1
        EndIf
    Next i

    // ordena por frequência decrescente (bloco só-parâmetro)
    ASort(aPairs, , , {|x, y| x[2] > y[2] })

    // ids reservados: 1=<s> (padding), 2=<unk> (fora do vocabulário)
    aAdd(aId2W, "<s>")  ; jW2Id["<s>"]  := 1
    aAdd(aId2W, "<unk>"); jW2Id["<unk>"] := 2
    For i := 1 To Len(aPairs)
        If Len(aId2W) >= MAXVOCAB + 2
            Exit
        EndIf
        cW := aPairs[i][1]
        aAdd(aId2W, cW)
        jW2Id[cW] := Len(aId2W)
    Next i

    // stream de ids; palavra fora do vocabulário vira <unk> (2)
    For i := 1 To Len(aTok)
        cW := aTok[i]
        If ValType(jW2Id[cW]) == "U"
            aAdd(aStream, 2)
        Else
            aAdd(aStream, jW2Id[cW])
        EndIf
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

    // Entrada = NCTX blocos locais posicionais + 1 bloco "bag" (long-context).
    For i := 1 To NHID
        aRow := {}
        For j := 1 To (NCTX + 1) * nV
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

    oM["xbuf"] := Array((NCTX + 1) * nV)    // buffer de entrada reutilizado no Forward
    AFill(oM["xbuf"], 0)
    oM["xactloc"] := {}                     // posições ativas dos blocos LOCAIS (limpas por forward)

    // Estado incremental do bag (long-context): contagem por palavra na janela.
    oM["bagcnt"] := Array(nV)               // quantos tokens da janela == cada id
    AFill(oM["bagcnt"], 0)
    oM["bagend"] := 0                       // tEnd que o bag reflete no momento
Return

// ------------------------------------------------------------------- Markov
// Guarda uma hash por ordem k=1..NCTX. mk[k][chave-de-k-palavras] -> próximos
// ids (multiset = frequência). A geração faz BACKOFF da ordem NCTX até a 1.
Static Function BuildMarkov(oM, aStream)
    Local aMk := {}
    Local k := 0
    Local i := 0
    Local cKey := ""
    Local nNext := 0

    For k := 1 To NCTX
        aAdd(aMk, JsonObject():New())
    Next k

    For i := 1 To Len(aStream) - NCTX
        nNext := aStream[i + NCTX]
        For k := 1 To NCTX                         // últimas k palavras do contexto
            cKey := KeyRange(aStream, i + NCTX - k, k)
            If ValType(aMk[k][cKey]) == "U"
                aMk[k][cKey] := {}
            EndIf
            aAdd(aMk[k][cKey], nNext)
        Next k
    Next i
    oM["mk"] := aMk
Return

// Chave = nLen ids consecutivos a partir de nStart.
Static Function KeyRange(aStream, nStart, nLen)
    Local c := ""
    Local i := 0
    For i := 0 To nLen - 1
        c += Str(aStream[nStart + i], 5) + " "
    Next i
Return c

// ------------------------------------------------------------------ Forward
// Prediz o token após a posição tEnd em aSeq. Entrada ternária =
//   NCTX blocos LOCAIS posicionais (últimas NCTX palavras, ordem importa)
//   + 1 bloco BAG de presença sobre as últimas BAGW palavras (long-context,
//     até CTXMAX tokens). Tudo {0,1} => multiply-free na BLAS.
// Retorna { aScores (nV), aH (NHID ternário) }.
Static Function Forward(oM, aSeq, tEnd)
    Local nV := oM["V"]
    Local aX := oM["xbuf"]
    Local aOld := oM["xactloc"]
    Local aNew := {}
    Local aHraw := Nil
    Local aH := {}
    Local aScores := Nil
    Local i := 0
    Local p := 0
    Local id := 0
    Local nPos := 0

    BagSyncTo(oM, aSeq, tEnd)               // mantém o bloco bag (long-context) incremental

    For i := 1 To Len(aOld)                 // limpa só os blocos LOCAIS do forward anterior
        aX[aOld[i]] := 0
    Next i
    // Blocos locais: posição p = 1..NCTX (p=NCTX é a palavra mais recente).
    For p := 1 To NCTX
        i := tEnd - (NCTX - p)              // índice em aSeq da palavra do bloco p
        id := 1                             // <s> (padding) se antes do início
        If i >= 1 .And. i <= tEnd
            id := aSeq[i]
        EndIf
        If id >= 1 .And. id <= nV
            nPos := (p - 1) * nV + id
            If aX[nPos] == 0
                aX[nPos] := 1
                aAdd(aNew, nPos)
            EndIf
        EndIf
    Next p
    oM["xactloc"] := aNew

    aHraw := MatVecTern(oM["wproj"], aX)        // BLAS ternária: W_proj . x
    For i := 1 To Len(aHraw)
        aAdd(aH, If(aHraw[i] > 0, 1, If(aHraw[i] < 0, -1, 0)))   // ativação ternária
    Next i

    aScores := MatVecTern(oM["U"], aH)          // BLAS ternária: U . h
Return {aScores, aH}

// BagSyncTo: mantém o bloco "bag" do xbuf refletindo a presença de cada palavra
// nas últimas BAGW posições de aSeq[1..tEnd]. Desliza a janela em O(delta) quando
// tEnd avança (caso comum: treino/geração monotônicos); reconstrói só quando há
// salto para trás ou muito grande. Custo amortizado O(1) por token.
Static Function BagSyncTo(oM, aSeq, tEnd)
    Local aCnt := oM["bagcnt"]
    Local aX   := oM["xbuf"]
    Local nBase := NCTX * oM["V"]
    Local nPrev := oM["bagend"]
    Local nStart := 0
    Local t := 0
    Local tOut := 0
    Local id := 0

    If tEnd < nPrev .Or. tEnd - nPrev > BAGW
        // reconstrói do zero (raro: início de passada/geração)
        AFill(aCnt, 0)
        For id := 1 To oM["V"]
            aX[nBase + id] := 0
        Next id
        nStart := tEnd - BAGW + 1
        If nStart < 1
            nStart := 1
        EndIf
        For t := nStart To tEnd
            id := aSeq[t]
            aCnt[id] := aCnt[id] + 1
            aX[nBase + id] := 1
        Next t
    Else
        // desliza para frente: adiciona os novos, remove os que saíram da janela
        For t := nPrev + 1 To tEnd
            id := aSeq[t]
            aCnt[id] := aCnt[id] + 1
            aX[nBase + id] := 1
            tOut := t - BAGW
            If tOut >= 1
                id := aSeq[tOut]
                aCnt[id] := aCnt[id] - 1
                If aCnt[id] <= 0
                    aX[nBase + id] := 0
                EndIf
            EndIf
        Next t
    EndIf
    oM["bagend"] := tEnd
Return

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
// Perceptron multiclasse MÉDIO (Collins 2002): erra -> U[alvo]+=h, U[prev]-=h,
// e mantém um acumulador Uc[..]+=t*delta; ao final usa a média U-Uc/T, que
// generaliza melhor que os pesos finais. Tudo add/sub (h é ternário).
// Para escalar a corpus grande, treina numa AMOSTRA por stride (teto MAXTRAIN);
// o Markov já usa o corpus inteiro. Custo do treino neural fica limitado.
Static Function Train(oM)
    Local aStream := oM["stream"]
    Local aU  := oM["U"]
    Local nV  := oM["V"]
    Local aUc := ZeroMatrix(nV, NHID)      // acumulador do perceptron médio
    Local nP := 0
    Local i := 0
    Local j := 0
    Local nErr := 0
    Local nErr1 := -1
    Local nT := 0                          // contador global de exemplos (peso da média)
    Local nPos := Len(aStream) - NCTX
    Local nStep := 1
    Local aCtx := {}
    Local aFwd := Nil
    Local aH := Nil
    Local nPred := 0
    Local nAlvo := 0
    Local aRowA := Nil
    Local aRowP := Nil
    Local aCcA := Nil
    Local aCcP := Nil

    If nPos > MAXTRAIN
        nStep := Int((nPos - 1) / MAXTRAIN) + 1   // divisão-teto: amostra <= MAXTRAIN posições
    EndIf

    ConOut("Treinando (perceptron médio ternário, " + Str(PASSES,2) + " passadas, stride " + Str(nStep,2) + "):")
    For nP := 1 To PASSES
        nErr := 0
        i := 1
        While i <= nPos
            nT++
            nAlvo := aStream[i + NCTX]        // alvo = palavra após o contexto

            aFwd := Forward(oM, aStream, i + NCTX - 1)   // contexto termina em i+NCTX-1
            nPred := Argmax(aFwd[1])
            aH := aFwd[2]

            If nPred != nAlvo
                nErr++
                aRowA := aU[nAlvo]
                aRowP := aU[nPred]
                aCcA := aUc[nAlvo]
                aCcP := aUc[nPred]
                For j := 1 To NHID
                    aRowA[j] := aRowA[j] + aH[j]
                    aRowP[j] := aRowP[j] - aH[j]
                    aCcA[j]  := aCcA[j]  + nT * aH[j]
                    aCcP[j]  := aCcP[j]  - nT * aH[j]
                Next j
            EndIf
            i += nStep
        End
        If nErr1 == -1
            nErr1 := nErr
        EndIf
        If nP == 1 .Or. nP == PASSES .Or. Mod(nP, 5) == 0
            ConOut("  passada " + Str(nP,2) + ": " + Str(nErr,5) + " erros")
        EndIf
    Next nP

    // Pesos médios: U := U - Uc / T
    If nT > 0
        For i := 1 To nV
            aRowA := aU[i]
            aCcA := aUc[i]
            For j := 1 To NHID
                aRowA[j] := aRowA[j] - aCcA[j] / nT
            Next j
        Next i
    EndIf

    oM["err_first"] := nErr1
    oM["err_last"]  := nErr
Return

// ZeroMatrix: matriz nR x nC preenchida com zeros.
Static Function ZeroMatrix(nR, nC)
    Local a := {}
    Local i := 0
    Local j := 0
    Local aRow := Nil
    For i := 1 To nR
        aRow := {}
        For j := 1 To nC
            aAdd(aRow, 0)
        Next j
        aAdd(a, aRow)
    Next i
Return a

// ------------------------------------------------------------------ Geração
// Gera um DOCUMENTO de até nMax tokens (multi-frase) a partir de um seed que
// pode ter até CTXMAX tokens. Mantém a sequência inteira aSeq, que alimenta o
// contexto local (últimas NCTX palavras) e o bag (últimas BAGW). Continua após
// cada ".", começando nova frase, até atingir nMax.
Static Function Gera(oM, cSeed, nMax)
    Local aSeq := SeedSeq(oM, cSeed)       // ids do seed (aceita seed longo)
    Local cOut := cSeed
    Local i := 0
    Local nNext := 0
    Local nPrev := 0
    Local nSent := 0                       // palavras na frase atual (libera "." após >=4)

    If nMax > CTXMAX
        nMax := CTXMAX
    EndIf
    For i := 1 To nMax
        nNext := NextWord(oM, aSeq, nPrev, nSent >= 4)
        If nNext == 0
            Exit
        EndIf
        aAdd(aSeq, nNext)
        nPrev := nNext
        If oM["id2w"][nNext] == "."
            cOut += "."
            nSent := 0
        Else
            cOut += " " + oM["id2w"][nNext]
            nSent++
        EndIf
    Next i
    ConOut("  [" + cSeed + "] (" + Str(i - 1, 4) + " tokens):")
    ConOut("  " + cOut)
    ConOut("")
Return

// NextWord: mistura Markov INTERPOLADO (Jelinek-Mercer: combina TODAS as ordens
// 1..NCTX, cada uma normalizada por frequência e pesada por lambda_k favorecendo
// a ordem maior) com o rerank aprendido da rede ternária; aplica anti-repetição
// e sharpening, e amostra por NUCLEUS (top-p). <s>/<unk> nunca são gerados.
Static Function NextWord(oM, aSeq, nPrev, lAllowEnd)
    Local nEnd    := Len(aSeq)
    Local aMkAll  := oM["mk"]
    Local aScores := Forward(oM, aSeq, nEnd)[1]
    Local aTop    := TopK(aScores, 6)
    Local cKey    := ""
    Local aMk     := Nil
    Local aIds    := {}    // candidatos únicos
    Local aW      := {}    // peso paralelo
    Local j       := 0
    Local k       := 0
    Local id      := 0
    Local nPeriod := IdOf(oM, ".")
    Local nUnk    := 2
    Local nLambda := 0

    // Interpolação: soma a contribuição de cada ordem k (maior ordem = maior peso).
    For k := NCTX To 1 Step -1
        cKey := ""
        For j := nEnd - k + 1 To nEnd            // últimas k palavras da sequência
            id := 1
            If j >= 1
                id := aSeq[j]
            EndIf
            cKey += Str(id, 5) + " "
        Next j
        aMk := aMkAll[k][cKey]
        If ValType(aMk) == "A" .And. Len(aMk) > 0
            nLambda := k * 2.0 / Len(aMk)        // lambda_k ~ k, normalizado pela contagem total
            For j := 1 To Len(aMk)
                AddCand(aIds, aW, aMk[j], nLambda)
            Next j
        EndIf
    Next k

    // Rede ternária: bônus por rank (top-1 vale mais); cobre contexto não visto.
    For j := 1 To Len(aTop)
        AddCand(aIds, aW, aTop[j], 1.0 * (Len(aTop) - j + 1) / Len(aTop))
    Next j

    // Filtros: descarta <s>/<unk>; segura ponto final; penaliza repetição na
    // janela recente (mata loops "se se se"); sharpening.
    For j := 1 To Len(aIds)
        id := aIds[j]
        If id == 1 .Or. id == nUnk
            aW[j] := 0
        ElseIf id == nPeriod .And. !lAllowEnd
            aW[j] := 0
        Else
            aW[j] := aW[j] / (1 + REPPEN * RecentCount(aSeq, nEnd, id))
        EndIf
        aW[j] := aW[j] * aW[j]                   // sharpening: favorece os fortes
    Next j
Return NucleusSample(aIds, aW, TOPP)

// RecentCount: quantas vezes id aparece nas últimas REPWIN posições de aSeq.
Static Function RecentCount(aSeq, nEnd, id)
    Local n := 0
    Local r := nEnd - REPWIN + 1
    If r < 1
        r := 1
    EndIf
    While r <= nEnd
        If aSeq[r] == id
            n++
        EndIf
        r++
    End
Return n

// IdOf: id de uma palavra (0 se não existir no vocabulário).
Static Function IdOf(oM, cW)
    If ValType(oM["w2id"][cW]) == "U"
        Return 0
    EndIf
Return oM["w2id"][cW]

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

// NucleusSample (top-p): ordena por peso desc, mantém o menor conjunto cuja
// massa acumulada atinge nP da massa total, e amostra proporcional dentro dele.
// Corta a cauda de baixa probabilidade — a técnica moderna de amostragem.
Static Function NucleusSample(aIds, aW, nP)
    Local aPar := {}
    Local nTot := 0
    Local nAcc := 0
    Local nCut := 0
    Local nR := 0
    Local i := 0

    For i := 1 To Len(aIds)
        If aW[i] > 0
            aAdd(aPar, {aIds[i], aW[i]})
            nTot += aW[i]
        EndIf
    Next i
    If nTot <= 0 .Or. Len(aPar) == 0
        Return If(Len(aIds) > 0, aIds[1], 0)
    EndIf

    ASort(aPar, , , {|x, y| x[2] > y[2] })       // maior peso primeiro
    // núcleo: menor prefixo com massa >= nP * total
    nCut := Len(aPar)
    For i := 1 To Len(aPar)
        nAcc += aPar[i][2]
        If nAcc >= nP * nTot
            nCut := i
            Exit
        EndIf
    Next i

    // amostra proporcional dentro do núcleo (1..nCut)
    nTot := 0
    For i := 1 To nCut
        nTot += aPar[i][2]
    Next i
    nR := (Random(100000) / 100000) * nTot
    nAcc := 0
    For i := 1 To nCut
        nAcc += aPar[i][2]
        If nR <= nAcc
            Return aPar[i][1]
        EndIf
    Next i
Return aPar[1][1]

// SeedSeq: converte o seed (de qualquer tamanho, até CTXMAX) num array de ids,
// com NCTX <s> de padding à esquerda. Palavra fora do vocabulário vira <unk>.
Static Function SeedSeq(oM, cSeed)
    Local aTok := Tokenize(cSeed)
    Local jW := oM["w2id"]
    Local aSeq := {}
    Local i := 0
    For i := 1 To NCTX
        aAdd(aSeq, 1)                      // padding <s>
    Next i
    For i := 1 To Len(aTok)
        If i > CTXMAX
            Exit
        EndIf
        If ValType(jW[aTok[i]]) == "U"
            aAdd(aSeq, 2)                  // <unk>
        Else
            aAdd(aSeq, jW[aTok[i]])
        EndIf
    Next i
Return aSeq

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

    // 2. Forward produz vetores dos tamanhos certos (aSeq com padding, tEnd=NCTX)
    aFwd := Forward(oM, {1, 1, 1}, NCTX)
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
    c += "a capital do brasil e brasilia uma cidade planejada no centro do pais. "
    c += "sao paulo e a maior cidade do pais e um grande centro financeiro e cultural. "
    c += "o rio de janeiro e conhecido por suas praias suas montanhas e sua alegria. "
    c += "a cidade de salvador guarda muito da historia e da cultura afro brasileira. "
    c += "o nordeste do brasil tem praias lindas um sol forte e um povo acolhedor. "
    c += "o sul do pais tem um clima mais frio e uma forte tradicao europeia. "
    c += "a regiao amazonica concentra a maior floresta tropical de todo o planeta. "
    c += "o cerrado e o pantanal abrigam uma fauna e uma flora muito ricas e variadas. "
    c += "os rios da amazonia sao imensos e cortam a floresta por milhares de quilometros. "
    c += "muitas especies de animais e de plantas ainda esperam para ser descobertas. "
    c += "a natureza do brasil e um patrimonio que pertence a toda a humanidade. "
    c += "proteger o meio ambiente e uma tarefa de todos os cidadaos e governos. "
    c += "o desmatamento ameaca a floresta e a vida de muitas comunidades locais. "
    c += "a reciclagem do lixo ajuda a preservar os recursos naturais do planeta. "
    c += "o sol o vento e a agua sao fontes de energia limpa e renovavel. "
    c += "a ciencia estuda a natureza para entender o mundo e melhorar a vida. "
    c += "os pesquisadores das universidades produzem conhecimento novo todos os anos. "
    c += "a medicina moderna salva vidas e aumenta o tempo de vida das pessoas. "
    c += "as vacinas protegem as criancas contra muitas doencas perigosas. "
    c += "a alimentacao saudavel e a pratica de exercicios trazem mais qualidade de vida. "
    c += "beber agua dormir bem e caminhar todos os dias fazem bem para o corpo. "
    c += "o computador e o celular fazem parte da rotina de quase todas as pessoas. "
    c += "os programas de computador resolvem problemas e automatizam tarefas do dia a dia. "
    c += "a inteligencia artificial aprende com os dados e ajuda em muitas areas. "
    c += "escrever um bom programa exige logica clareza e muita atencao aos detalhes. "
    c += "a linguagem advpl e usada para criar sistemas de gestao nas empresas. "
    c += "um compilador traduz o codigo escrito pelo programador em instrucoes da maquina. "
    c += "o brasil produz cafe soja milho e muitos outros alimentos para o mundo. "
    c += "a agricultura e a pecuaria movimentam boa parte da economia nacional. "
    c += "o comercio e a industria geram empregos nas cidades grandes e pequenas. "
    c += "as pequenas empresas sao muito importantes para a economia de cada regiao. "
    c += "o trabalho honesto e a educacao abrem portas para um futuro melhor. "
    c += "muitos jovens estudam a noite e trabalham durante o dia para vencer na vida. "
    c += "a escola prepara as criancas e os jovens para os desafios do futuro. "
    c += "ler bons livros desde cedo desperta a curiosidade e a imaginacao das criancas. "
    c += "a literatura brasileira tem autores famosos lidos em todo o mundo. "
    c += "a musica popular brasileira mistura ritmos de muitas origens diferentes. "
    c += "o samba o forro e a bossa nova nasceram da alma criativa do povo. "
    c += "o carnaval e a maior festa popular do brasil e atrai turistas de todo lugar. "
    c += "as festas juninas animam o interior com comidas dancas e muita fogueira. "
    c += "a comida brasileira e rica variada e cheia de sabores de cada regiao. "
    c += "o arroz com feijao e o prato mais presente nas mesas das familias. "
    c += "a feijoada a moqueca e o acaraje sao pratos famosos da cozinha do pais. "
    c += "as frutas tropicais como a manga o caju e o maracuja encantam quem prova. "
    c += "o futebol reune amigos nos campos nas praias e nas ruas de todo o brasil. "
    c += "a selecao brasileira e a maior campea da copa do mundo de futebol. "
    c += "o esporte ensina disciplina trabalho em equipe e respeito ao adversario. "
    c += "praticar um esporte melhora a saude o humor e a disposicao das pessoas. "
    c += "as familias se reunem nos fins de semana para almocar e conversar juntas. "
    c += "os avos contam historias antigas que passam de uma geracao para outra. "
    c += "a amizade e a solidariedade tornam a vida em comunidade mais leve e feliz. "
    c += "ajudar o proximo e cuidar de quem precisa e um gesto de grandeza. "
    c += "viajar pelo pais e uma forma de conhecer novas paisagens e novas culturas. "
    c += "cada estado do brasil tem sua propria historia seus costumes e sua beleza. "
    c += "as cidades historicas de minas gerais guardam igrejas antigas e ruas de pedra. "
    c += "o pantanal atrai visitantes que buscam observar animais em seu ambiente natural. "
    c += "o povo brasileiro e conhecido pela hospitalidade e pela alegria de viver. "
    c += "a diversidade de povos e de culturas e uma das maiores riquezas do brasil. "
    c += "o respeito as diferencas fortalece a democracia e a paz entre as pessoas. "
    c += "o voto e um direito e um dever de todo cidadao consciente e responsavel. "
    c += "a informacao de qualidade ajuda as pessoas a tomar melhores decisoes. "
    c += "a internet aproxima as pessoas mas exige atencao e uso responsavel. "
    c += "o futuro do pais depende da educacao da ciencia e do trabalho de todos. "
    c += "sonhar estudar e trabalhar com dedicacao transforma a vida das pessoas. "
Return c
