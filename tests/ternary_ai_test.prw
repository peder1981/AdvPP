// Fixture: BLAS ternária (MatVecTern) e console interativo (ConIn).
User Function TernAiTst()
    Local aMat := { {5, 7, 9}, {2, 4, 6} }
    Local aVec := { 1, 0, -1 }              // ternário
    Local aR   := MatVecTern(aMat, aVec)
    Local cLinha := ""

    // linha0: +5 -9 = -4 ; linha1: +2 -6 = -4
    ConOut("MatVecTern r0 (esperado -4): " + Str(aR[1]))
    ConOut("MatVecTern r1 (esperado -4): " + Str(aR[2]))

    // ConIn le uma linha do stdin (nao-bloqueante em pipe; "" no EOF)
    cLinha := ConIn("digite algo> ")
    ConOut("leu do stdin: [" + cLinha + "]")
Return
