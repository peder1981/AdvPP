/*
 * pt_llm.prw - Um "LLM" pequeno e simples que le e fala portugues do Brasil.
 *
 * Nao e uma rede neural: e um modelo de linguagem de Markov de ordem variavel
 * em nivel de BYTE (ordens 1..N numa mesma hash, com backoff). Le portugues
 * (condiciona no prompt) e fala portugues (continua o texto) com estatistica
 * de n-gramas aprendida do corpus embutido em Corpus().
 *
 * Byte-level: as strings aqui sao UTF-8 (Len("cafe com acento")>letras), entao
 * as sequencias multibyte dos acentos (a,e,o,c cedilha, til) emergem sozinhas
 * das estatisticas de bytes. Sem IO de arquivo: corpus embutido = auto-contido.
 *
 * Rodar/testar: ./advplc run pt_llm.prw   (o auto-teste roda ao final)
 *
 * ponytail: append de string por byte em hash e O(n^2) no pior caso; troca por
 * array de sufixos se o corpus passar de ~50KB. Neste tamanho roda instantaneo.
 */

#define ORDEM 6

User Function PtLLM()
    Local h        := Nil
    Local aPrompts := {}
    Local i        := 0

    ConOut("=========================================================")
    ConOut(" LLM pequeno em AdvPL - portugues do Brasil (Markov byte) ")
    ConOut("=========================================================")

    // "Ler": treina lendo todo o corpus em portugues
    h := TrainModel(PtCorpus(), ORDEM)
    ConOut("Modelo treinado (ordem " + Str(ORDEM,1) + ").")
    ConOut("")

    // "Falar": recebe prompts em portugues e continua o texto
    aAdd(aPrompts, "O Brasil e um pais")
    aAdd(aPrompts, "Na cidade de")
    aAdd(aPrompts, "A tecnologia")
    aAdd(aPrompts, "Bom dia, ")

    For i := 1 To Len(aPrompts)
        ConOut("Prompt : " + aPrompts[i])
        ConOut("Fala   : " + aPrompts[i] + Speak(h, aPrompts[i], ORDEM, 180))
        ConOut("")
    Next i

    // Geracao livre (sem prompt): reseeda de um trecho aleatorio do corpus
    ConOut("Fala livre:")
    ConOut(Speak(h, "", ORDEM, 240))
    ConOut("")

    PtTest()
Return

/*
 * Treina: para cada ordem k de 1..nOrder, aprende quais bytes seguem cada
 * contexto de k bytes. O valor da hash e a MULTISET de seguintes como string,
 * entao um sorteio uniforme ja fica ponderado pela frequencia. Elegante e vago.
 */
Static Function TrainModel(cText, nOrder)
    Local h  := JsonObject():New()
    Local nL := Len(cText)
    Local k  := 0
    Local i  := 0
    Local cCtx := ""
    Local cNxt := ""

    For k := 1 To nOrder
        For i := 1 To nL - k
            cCtx := SubStr(cText, i, k)
            cNxt := SubStr(cText, i + k, 1)
            If ValType(h[cCtx]) == "U"
                h[cCtx] := cNxt
            Else
                h[cCtx] := h[cCtx] + cNxt
            EndIf
        Next i
    Next k
Return h

/*
 * Fala nLen bytes a partir de cSeed. Se cSeed vazio, reseeda do corpus.
 * Desliza a janela de contexto e escolhe o proximo byte com backoff.
 */
Static Function Speak(h, cSeed, nOrder, nLen)
    Local cOut := ""
    Local cCtx := ""
    Local cNxt := ""
    Local i    := 0

    If Len(cSeed) >= nOrder
        cCtx := Right(cSeed, nOrder)
    Else
        cCtx := Reseed(nOrder)
    EndIf

    For i := 1 To nLen
        cNxt := PickNext(h, cCtx, nOrder)
        If Empty(cNxt)
            cCtx := Reseed(nOrder)              // contexto sem saida: recomeca
            cNxt := PickNext(h, cCtx, nOrder)   // contexto do corpus sempre tem saida
        EndIf
        If .Not. Empty(cNxt)
            cOut := cOut + cNxt
            cCtx := Right(cCtx + cNxt, nOrder)
        EndIf
    Next i
Return cOut

/*
 * Backoff: tenta o contexto mais longo presente na hash; encurta ate achar.
 * Retorna um byte sorteado dos seguintes, ou "" se nada bater (raro: ordem-1).
 */
Static Function PickNext(h, cCtx, nOrder)
    Local k     := 0
    Local cTry  := ""
    Local cFoll := ""
    Local nPos  := 0

    k := Len(cCtx)
    While k >= 1
        cTry := Right(cCtx, k)
        cFoll := h[cTry]
        If ValType(cFoll) == "C" .And. Len(cFoll) > 0
            nPos := Random(Len(cFoll))
            Return SubStr(cFoll, nPos, 1)
        EndIf
        k := k - 1
    EndDo
Return ""

// Reseed: janela de nOrder bytes de uma posicao aleatoria do corpus (sempre vista no treino)
Static Function Reseed(nOrder)
    Local cC := PtCorpus()
    Local nP := Random(Len(cC) - nOrder)
Return SubStr(cC, nP, nOrder)

/*
 * Auto-teste (ponytail): garante que o modelo treinou e que a geracao respeita
 * as transicoes do corpus. Falha ruidosamente se a logica quebrar.
 */
User Function PtTest()
    Local h    := TrainModel(PtCorpus(), ORDEM)
    Local nFail := 0
    Local cCtx := "Brasil"
    Local cNxt := ""

    ConOut("--- auto-teste ---")

    // 1. contexto conhecido tem seguintes
    If ValType(h[cCtx]) != "C"
        ConOut("FALHA: contexto conhecido sem seguintes"); nFail++
    EndIf

    // 2. PickNext sempre devolve um byte que realmente segue o contexto no corpus.
    cNxt := PickNext(h, cCtx, ORDEM)
    If Empty(cNxt) .Or. At(cCtx + cNxt, PtCorpus()) == 0
        ConOut("FALHA: byte gerado nao segue o contexto no corpus"); nFail++
    EndIf

    // 3. Speak produz saida nao-vazia
    If Empty(Speak(h, "O Brasil", ORDEM, 50))
        ConOut("FALHA: geracao vazia"); nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,1) + " erro(s).")
    EndIf
Return

// Corpus de treino: portugues do Brasil embutido (quanto maior, mais fluente).
User Function PtCorpus()
    Local c := ""
    c += "O Brasil e um pais de dimensoes continentais, com uma cultura rica e diversa. "
    c += "A lingua portuguesa e falada por milhoes de pessoas em todo o territorio nacional. "
    c += "Na cidade de Sao Paulo, o transito costuma ser intenso durante toda a semana. "
    c += "No Rio de Janeiro, as praias atraem turistas do mundo inteiro o ano todo. "
    c += "A tecnologia mudou a forma como as pessoas trabalham e se comunicam hoje em dia. "
    c += "Bom dia, tudo bem com voce? Espero que o seu dia seja tranquilo e produtivo. "
    c += "Boa tarde, como foi o seu almoco na empresa nesta segunda-feira ensolarada? "
    c += "Boa noite, e hora de descansar depois de um longo dia de trabalho e estudo. "
    c += "A educacao e a base para o desenvolvimento de qualquer sociedade moderna. "
    c += "As escolas publicas precisam de mais investimento e de professores valorizados. "
    c += "O cafe da manha e a refeicao mais importante para muitas familias brasileiras. "
    c += "No interior do pais, a agricultura movimenta a economia de pequenas cidades. "
    c += "A floresta amazonica abriga uma biodiversidade unica e precisa ser preservada. "
    c += "Os rios brasileiros sao fontes de agua, energia e vida para o povo do campo. "
    c += "A musica popular brasileira encanta pessoas de todas as idades e regioes. "
    c += "O futebol e uma paixao nacional que une torcedores em todo o pais aos domingos. "
    c += "A culinaria regional oferece pratos saborosos feitos com ingredientes locais. "
    c += "Muitas pessoas gostam de viajar durante as ferias para conhecer novos lugares. "
    c += "A saude publica e um direito de todos os cidadaos brasileiros garantido por lei. "
    c += "Os hospitais atendem milhares de pacientes todos os dias em cada estado do pais. "
    c += "O trabalho em equipe e fundamental para alcancar bons resultados em qualquer projeto. "
    c += "A leitura de bons livros amplia o conhecimento e estimula a imaginacao das criancas. "
    c += "As bibliotecas guardam historias, saberes e memorias de muitas geracoes passadas. "
    c += "O comercio local gera empregos e fortalece a economia dos bairros das cidades. "
    c += "A internet conecta pessoas de diferentes lugares e facilita o acesso a informacao. "
    c += "Os jovens buscam oportunidades de estudo e de trabalho para construir o futuro. "
    c += "A natureza oferece paisagens belas que devem ser cuidadas com muito respeito. "
    c += "Cada regiao do Brasil tem costumes, sotaques e tradicoes que enriquecem a cultura. "
    c += "A familia se reune no fim de semana para conversar, cozinhar e celebrar a vida. "
    c += "O respeito ao proximo e a base de uma convivencia pacifica entre as pessoas. "
    c += "Obrigado pela atencao, e desejo a voce muita saude, paz e felicidade sempre. "
Return c
