// Aritmética faltante + estatística (S6d).
User Function MStatTst()
    Local nFail := 0
    Local aLR := {}

    // aritmetica
    If Abs(Pow(2,10) - 1024) > 0.0001
        ConOut("FALHA: Pow"); nFail++
    EndIf
    If Abs(Log10(1000) - 3) > 0.0001
        ConOut("FALHA: Log10"); nFail++
    EndIf
    If Ceil(7.1) != 8
        ConOut("FALHA: Ceil"); nFail++
    EndIf
    If Sign(-5) != -1 .Or. Sign(5) != 1 .Or. Sign(0) != 0
        ConOut("FALHA: Sign"); nFail++
    EndIf
    If Gcd(48,36) != 12
        ConOut("FALHA: Gcd"); nFail++
    EndIf
    If Lcm(4,6) != 12
        ConOut("FALHA: Lcm"); nFail++
    EndIf
    If Fact(5) != 120
        ConOut("FALHA: Fact"); nFail++
    EndIf
    If Abs(Atan2(1,1) - 3.14159265/4) > 0.001
        ConOut("FALHA: Atan2"); nFail++
    EndIf

    // estatistica
    If Abs(Mean({2,4,6}) - 4) > 0.0001
        ConOut("FALHA: Mean"); nFail++
    EndIf
    If Abs(Median({3,1,2}) - 2) > 0.0001
        ConOut("FALHA: Median"); nFail++
    EndIf
    If Abs(StdDev({2,4,6}) - 2) > 0.0001    // variancia amostral = 4 -> desvio 2
        ConOut("FALHA: StdDev = " + Str(StdDev({2,4,6}),10,4)); nFail++
    EndIf
    // LinReg de y=2x exato -> {a~0, b~2}
    aLR := LinReg({1,2,3}, {2,4,6})
    If Abs(aLR[1]) > 0.0001 .Or. Abs(aLR[2] - 2) > 0.0001
        ConOut("FALHA: LinReg"); nFail++
    EndIf
    // Interp: entre (0,0) e (2,10), x=1 -> 5
    If Abs(Interp({0,2},{0,10}, 1) - 5) > 0.0001
        ConOut("FALHA: Interp"); nFail++
    EndIf

    If nFail == 0
        ConOut("OK: aritmetica + estatistica verificadas.")
    Else
        ConOut("TESTE FALHOU: " + Str(nFail,2))
    EndIf
Return
