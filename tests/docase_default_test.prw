// Fixture de regressão da issue #3: `Default` sozinho na linha dentro de
// `Do Case` é o ramo otherwise (alias tolerado de `Otherwise`).
// Sem o fix, o `Default` era consumido por parseDefault como atribuição
// `Default <prox-ident> := ...`, engolindo a primeira linha do ramo.
User Function Main()
    Local cModo := "X"

    // 4 Cases + Default (caso exato do relato)
    Do Case
        Case cModo == "A"
            ConOut("BRANCH_A")
        Case cModo == "B"
            ConOut("BRANCH_B")
        Case cModo == "C"
            ConOut("BRANCH_C")
        Case cModo == "D"
            ConOut("BRANCH_D")
        Default
            ConOut("BRANCH_DEFAULT")
    EndCase

    // Otherwise continua funcionando
    Do Case
        Case cModo == "A"
            ConOut("OTHER_A")
        Otherwise
            ConOut("OTHERWISE_OK")
    EndCase

    // Default com atribuição continua sendo statement Default
    Local nVal := 0
    Default nVal := 42
    ConOut("DEFAULT_STMT=" + AllTrim(Str(nVal)))
Return
