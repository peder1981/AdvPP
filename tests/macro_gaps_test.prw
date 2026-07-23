// Fixture de regressão pros gaps encontrados na varredura de 2026-07-23 no
// corpus real (811R4 + 12.1.2510): BEGIN REPORT QUERY, macro-eval em
// runtime (&expr), composição de nome via ident&macro, e NamedParam fora de
// lista de argumentos. Ver CHANGELOG [Não lançado].
User Function MacroGapsTest()
    Local cProcesso := "1"
    Local cPeriodo := "2"
    Local cMyAlias
    Local cIdx := "42"
    Local nVal := &cIdx
    Local cSuf := "A"
    Local nx
    Local aArr

    ConOut("macro_eval=" + Str(nVal))

    Private K2A
    K2A := 99
    nx := K2&cSuf
    ConOut("ident_macro=" + Str(nx))

    // NamedParam fora de call args: elemento de array via `nome := valor`
    aArr := { 1, nx := 100, 3 }
    ConOut("named_param=" + Str(nx))

    BEGIN REPORT QUERY oSecFil
        BeginSql alias cMyAlias
            SELECT A, B
            FROM %table:RFT% RFT
            WHERE RFT_PROCES = %exp:cProcesso% AND RFT_PERIOD = %exp:cPeriodo%
        EndSql
    END REPORT QUERY oSecFil PARAM cProcesso, cPeriodo
    ConOut("report_query=OK")
Return
