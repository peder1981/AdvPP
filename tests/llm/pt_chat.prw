/*
 * pt_chat.prw - Assistente que LE e RESPONDE em portugues do Brasil,
 *               escrito integralmente em AdvPL (roda no advplc/AdvPP).
 *
 * Nao e rede neural nem cadeia de Markov: e um respondedor por RECUPERACAO
 * (retrieval). Le a pergunta, normaliza (minusculas + remove acentos), tokeniza
 * em palavras, descarta stopwords e pontua cada item da base de conhecimento
 * pela sobreposicao de palavras-conteudo. Responde com o item mais relevante;
 * se nada casar bem, devolve um fallback educado em vez de inventar.
 *
 * REPL interativo via ConIn() (leitura de stdin). Rodar:
 *     ./advplc run pt_chat.prw
 *   ou nao-interativo:
 *     printf 'o que e advpp?\ncapital do brasil?\nsair\n' | ./advplc run pt_chat.prw
 *
 * ponytail: scoring O(base * palavras) por pergunta; a base e pequena, roda
 * instantaneo. Se crescer pra milhares de itens, trocar por indice invertido.
 */

User Function PtChat()
    Local cPerg := ""
    Local lOn   := .T.

    SelfTest()  // auto-verificacao ao subir (uma linha OK/FALHA)

    ConOut("=====================================================")
    ConOut(" Assistente em portugues do Brasil - AdvPL puro (AdvPP)")
    ConOut(" Pergunte algo. Digite 'sair' para encerrar.")
    ConOut("=====================================================")

    While lOn
        cPerg := ConIn("voce > ")
        If Empty(AllTrim(cPerg))
            If Empty(cPerg)   // EOF (stdin acabou)
                lOn := .F.
            EndIf
            Loop
        EndIf
        If IsSair(cPerg)
            ConOut("bot  > Ate mais! Foi bom conversar.")
            lOn := .F.
            Loop
        EndIf
        ConOut("bot  > " + Answer(cPerg))
    End
Return

// Answer: le a pergunta e devolve a melhor resposta da base (ou fallback).
Static Function Answer(cPerg)
    Local aKB   := KB()
    Local aQ    := Tokenize(cPerg)
    Local nBest := 0
    Local nIdx  := 0
    Local nS    := 0
    Local i     := 0

    For i := 1 To Len(aKB)
        nS := ScoreEntry(aQ, aKB[i][1])
        If nS > nBest
            nBest := nS
            nIdx  := i
        EndIf
    Next i

    If nIdx == 0
        Return "Ainda nao sei responder isso. Pode reformular, ou pergunte 'o que posso perguntar?'."
    EndIf
Return aKB[nIdx][2]

// ScoreEntry: soma o tamanho de cada palavra-conteudo da pergunta que aparece
// (como palavra inteira) nas chaves do item. Palavra maior = mais especifica.
Static Function ScoreEntry(aQ, cChaves)
    Local cK := " " + Fold(cChaves) + " "
    Local nScore := 0
    Local i := 0
    Local cW := ""

    For i := 1 To Len(aQ)
        cW := aQ[i]
        If IsStop(cW) .Or. Len(cW) < 2
            Loop
        EndIf
        If At(" " + cW + " ", cK) > 0
            nScore += Len(cW)
        EndIf
    Next i
Return nScore

// Tokenize: normaliza e quebra em palavras (a-z, 0-9).
Static Function Tokenize(cText)
    Local aW  := {}
    Local cN  := Fold(cText)
    Local nL  := Len(cN)
    Local i   := 0
    Local cCh := ""
    Local cCur := ""

    For i := 1 To nL
        cCh := SubStr(cN, i, 1)
        If IsWordByte(cCh)
            cCur += cCh
        Else
            If Len(cCur) > 0
                aAdd(aW, cCur)
                cCur := ""
            EndIf
        EndIf
    Next i
    If Len(cCur) > 0
        aAdd(aW, cCur)
    EndIf
Return aW

// Fold: minusculas + remove acentos do PT-BR (casamento robusto a acentos).
Static Function Fold(c)
    c := Lower(c)
    c := StrTran(c, "á", "a") ; c := StrTran(c, "à", "a") ; c := StrTran(c, "â", "a") ; c := StrTran(c, "ã", "a")
    c := StrTran(c, "é", "e") ; c := StrTran(c, "ê", "e")
    c := StrTran(c, "í", "i")
    c := StrTran(c, "ó", "o") ; c := StrTran(c, "ô", "o") ; c := StrTran(c, "õ", "o")
    c := StrTran(c, "ú", "u") ; c := StrTran(c, "ü", "u")
    c := StrTran(c, "ç", "c")
Return c

Static Function IsWordByte(cCh)
    Local n := Asc(cCh)
Return (n >= 97 .And. n <= 122) .Or. (n >= 48 .And. n <= 57)

// Stopwords PT-BR: palavras vazias que nao ajudam a discriminar o topico.
Static Function IsStop(cW)
    Local cStop := " o a os as um uma uns umas de do da dos das em no na nos nas "
    cStop += "e ou que qual quais quem como onde quando porque por para com sem "
    cStop += "eu voce tu ele ela nos eles me te se meu minha seu sua isso isto "
    cStop += "eh e sao esta estao ser tem ter foi era mais muito sobre ao aos "
Return At(" " + cW + " ", cStop) > 0

Static Function IsSair(cPerg)
    Local c := Fold(AllTrim(cPerg))
Return c == "sair" .Or. c == "tchau" .Or. c == "adeus" .Or. c == "exit" .Or. c == "quit" .Or. c == "fim"

// Base de conhecimento: { chaves-de-busca , resposta em PT-BR }.
Static Function KB()
    Local a := {}
    aAdd(a, {"oi ola opa eai bom dia boa tarde noite fala hey", ;
        "Ola! Sou um assistente escrito em AdvPL puro. Pergunte sobre o AdvPP, AdvPL/TLPP ou o Brasil."})
    aAdd(a, {"quem voce assistente o que faz apresente nome", ;
        "Sou um respondedor em portugues feito 100% em AdvPL, rodando no compilador AdvPP. Leio sua pergunta e busco a resposta mais parecida na minha base."})
    aAdd(a, {"como funciona retrieval recuperacao busca similaridade tecnica", ;
        "Funciono por recuperacao: normalizo e tokenizo sua pergunta, descarto palavras vazias e pontuo cada item da base pela sobreposicao de palavras. Devolvo o mais relevante."})
    aAdd(a, {"advpp projeto compilador vm maquina virtual go", ;
        "O AdvPP e um compilador e maquina virtual open-source de AdvPL/TLPP escritos em Go, sem CGO. Compila, roda e depura fontes .prw/.tlpp fora do Protheus."})
    aAdd(a, {"advpl tlpp linguagem protheus totvs o que", ;
        "AdvPL e a linguagem do ERP TOTVS Protheus; TLPP e sua evolucao com tipagem estatica, namespaces e OOP moderna. O AdvPP implementa as duas."})
    aAdd(a, {"como compilar rodar executar check verificar arquivo comando advplc", ;
        "Use: 'advplc check arq.prw' valida a sintaxe; 'advplc run arq.prw' compila e executa; 'advplc build arq.prw -o saida' gera um executavel standalone."})
    aAdd(a, {"notacao hungara prefixo variavel c n l a o d b tipo", ;
        "Notacao hungara prefixa a variavel pelo tipo: c=character, n=numeric, l=logical, a=array, o=object, d=date, b=codeblock, j=json. Ex: cNome, nTotal, aItens."})
    aAdd(a, {"mvc modelo view browse fwformmodel fwformview", ;
        "O padrao MVC do Protheus usa FWFormModel (dados), FWFormView (tela) e FWFormBrowse (listagem), amarrados pelo MenuDef. O AdvPP suporta os modelos 1 e 3."})
    aAdd(a, {"markov llm modelo linguagem pt_llm gera texto ia", ;
        "O pt_llm.prw e um modelo de linguagem de Markov em nivel de byte, escrito em AdvPL: aprende n-gramas de um corpus e gera texto em portugues. Eu (pt_chat) respondo por busca."})
    aAdd(a, {"capital brasil brasilia", ;
        "A capital do Brasil e Brasilia, no Distrito Federal, inaugurada em 1960."})
    aAdd(a, {"maior cidade populosa sao paulo", ;
        "A maior cidade do Brasil e Sao Paulo, com mais de 11 milhoes de habitantes."})
    aAdd(a, {"lingua idioma falado oficial brasil portugues", ;
        "A lingua oficial do Brasil e o portugues, falado por praticamente toda a populacao."})
    aAdd(a, {"quantos estados unidades federacao brasil", ;
        "O Brasil tem 26 estados mais o Distrito Federal, totalizando 27 unidades federativas."})
    aAdd(a, {"floresta amazonia rio amazonas natureza", ;
        "A Amazonia e a maior floresta tropical do mundo, e o rio Amazonas o mais volumoso do planeta; ambos ficam no norte do Brasil."})
    aAdd(a, {"futebol esporte selecao copa", ;
        "O futebol e a paixao nacional; a selecao brasileira e a maior campea da Copa do Mundo, com cinco titulos."})
    aAdd(a, {"obrigado obrigada valeu agradeco grato", ;
        "De nada! Se tiver outra pergunta, e so mandar."})
    aAdd(a, {"ajuda posso perguntar topicos temas ajudar duvida", ;
        "Posso falar sobre: o AdvPP e o AdvPL/TLPP, como compilar/rodar, notacao hungara, MVC, o modelo pt_llm, e fatos gerais do Brasil (capital, cidades, estados, lingua)."})
Return a

// SelfTest: verificacao minima que roda ao subir (ponytail: um check runnable).
Static Function SelfTest()
    Local nFail := 0

    If At("Brasilia", Answer("qual a capital do brasil?")) == 0
        nFail++
    EndIf
    If At("portugues", Lower(Answer("que lingua se fala no brasil"))) == 0
        nFail++
    EndIf
    If At("compilador", Lower(Answer("o que e o advpp"))) == 0
        nFail++
    EndIf
    // pergunta fora do dominio cai no fallback
    If At("reformular", Answer("qual a receita de brigadeiro")) == 0
        nFail++
    EndIf

    If nFail == 0
        ConOut("[auto-teste: 4/4 OK]")
    Else
        ConOut("[auto-teste FALHOU: " + Str(nFail,1) + " erro(s)]")
    EndIf
Return
