User Function IfShortTst()
    Local aPos := {}
    Local aNeg := {}
    Local aNums := {3, -2, 5, -8}
    Local i := 0
    For i := 1 To Len(aNums)
        If(aNums[i] > 0, aAdd(aPos, aNums[i]), aAdd(aNeg, aNums[i]))
    Next i
    ConOut("pos (esperado 2): " + Str(Len(aPos)))
    ConOut("neg (esperado 2): " + Str(Len(aNeg)))
Return
