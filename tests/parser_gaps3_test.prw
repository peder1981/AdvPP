// Fixture de regressão pra rodada de 2026-07-23 "tente ao menos novamente":
// 10 gaps reais achados analisando os fontes-padrão do corpus que sobraram
// de rodadas anteriores. Ver CHANGELOG [Não lançado].
//
// SyntaxOnlyGaps cobre construções que são "parseadas e descartadas"
// (sem native real por trás, mesmo espírito de vários outros comandos
// deste compilador) — só precisam compilar (advplc check), não rodar.
// RuntimeGaps cobre as que têm semântica real e precisam rodar certo.
User Function ParserGaps3Test()
    RuntimeGaps()
Return

Static Function SyntaxOnlyGaps()
    Local nCnt, aCabUsr, cAlias1

    // BEGIN REPORT QUERY / STORE HEADER ... FOR seguido de For de loop real
    Store Header "H" TO aCabUsr For .T.
    For nCnt := 1 To 3
    Next

    // Copy To Memory sem nome de array (idioma real sem argumento)
    Copy "SX3" To Memory

    // ParamType ... seguido, na linha de baixo, de um Default de verdade
    ParamType 1 Var cAlias1 As Character
    Default cAlias1 := ""

    // REPLACE ... ALL FOR <cond> (cláusulas de escopo do Clipper)
    REPLACE X3_ORDEM WITH 'XX' ALL FOR X3_PYME == 'N'

    // End For (duas palavras) fechando o loop
    For nCnt := 1 To 2
    End For

    // M->&macro. — campo via alias com nome computado por macro (resolução
    // dinâmica de nome não modelada neste VM, mesma tolerância de outros
    // alvos não endereçáveis — só não pode travar o PARSER)
    Local cCampo := "X"
    M->&cCampo. := 5
Return

Static Function RuntimeGaps()
    Local cSuf := "A"
    Private K2A
    K2A := 7
    Local nOut := K2&cSuf
    ConOut("ident_macro_for_bound=" + Str(nOut))

    Local aM := {10, 20, 30}
    Local nAux := 0
    Local nVal := aM[nAux+=1]
    ConOut("array_index_compound_assign=" + Str(nVal) + "," + Str(nAux))
Return
