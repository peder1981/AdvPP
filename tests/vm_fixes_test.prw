// Regressao dos dois fixes da VM:
//  1. Operadores relacionais em string sao lexicograficos (nao jogam em ToFloat).
//  2. JsonObject:HasProperty com chave-variavel (com letras) e case-sensitive e funciona.
User Function VmFixTst()
    Local nFail := 0
    Local oJ := JsonObject():New()
    Local aT := {"Local", "nX", "Local", "nX", "Local"}
    Local i := 0
    Local t := ""
    Local nDist := 0

    // --- 1. relacional de string lexicografico ---
    If " " >= "A"
        ConOut("FALHA: ' ' >= 'A' deveria ser falso")
        nFail++
    EndIf
    If !("A" < "a")           // 'A'=65 < 'a'=97
        ConOut("FALHA: 'A' < 'a' deveria ser verdadeiro")
        nFail++
    EndIf
    If !("abc" < "abd")
        ConOut("FALHA: 'abc' < 'abd' deveria ser verdadeiro")
        nFail++
    EndIf
    If !("9" < "A")           // '9'=57 < 'A'=65
        ConOut("FALHA: '9' < 'A' deveria ser verdadeiro")
        nFail++
    EndIf
    // numerico continua numerico
    If !(2 < 10)
        ConOut("FALHA: 2 < 10 deveria ser verdadeiro")
        nFail++
    EndIf

    // --- 2. HasProperty case-sensitive com chave-variavel ---
    For i := 1 To Len(aT)
        t := aT[i]
        If oJ:HasProperty(t)
            oJ[t] := oJ[t] + 1
        Else
            oJ[t] := 1
            nDist++
        EndIf
    Next i
    If nDist != 2
        ConOut("FALHA: HasProperty nao dedup por chave-variavel (distintos=" + Str(nDist,2) + ", esperado 2)")
        nFail++
    EndIf
    If oJ["Local"] != 3 .Or. oJ["nX"] != 2
        ConOut("FALHA: contagem via HasProperty errada (Local=" + Str(oJ["Local"],2) + " nX=" + Str(oJ["nX"],2) + ")")
        nFail++
    EndIf
    // case-sensitive: chave com casing diferente NAO existe
    If oJ:HasProperty("local")
        ConOut("FALHA: HasProperty('local') deveria ser falso (case-sensitive)")
        nFail++
    EndIf
    If !oJ:HasProperty("Local")
        ConOut("FALHA: HasProperty('Local') deveria ser verdadeiro")
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: fixes da VM (relacional string + HasProperty) verificados.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,2))
    EndIf
Return
