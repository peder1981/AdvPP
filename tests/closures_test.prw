// Fixture: closures de verdade — codeblocks capturam Locais do escopo envolvente
// por referência (leitura, escrita, e estado persistente que escapa da função).
User Function ClosureTst()
    Local nSoma := 0
    Local bAcc := Nil
    Local nR := 0

    // Escrita: acumulador externo capturado por referência
    AEval({10, 20, 30}, {|x| nSoma := nSoma + x })
    ConOut("AEval acumula em Local externo (esperado 60): " + Str(nSoma))

    // Leitura de múltiplas capturas num Eval
    ConOut("Eval le capturas (esperado 47): " + Str(CalcBF()))

    // Closure que escapa: contador com estado persistente entre chamadas
    bAcc := MakeCounter()
    Eval(bAcc)
    Eval(bAcc)
    nR := Eval(bAcc)
    ConOut("counter que escapou (esperado 3): " + Str(nR))
Return

Static Function MakeCounter()
    Local nN := 0
Return {|| nN := nN + 1 }        // captura nN por referência; sobrevive ao Return

Static Function CalcBF()
    Local nBase := 5
    Local nFator := 3
Return Eval({|x| nBase + x * nFator }, 14)   // 5 + 14*3 = 47
