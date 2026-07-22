User Function DynPrivTst()
    Private cCtx := "A"
    ModB()
    ConOut("apos ModB (esperado Z): " + cCtx)
Return

Static Function ModB()
    ConOut("B ve (esperado A): " + cCtx)
    cCtx := "Z"
Return
