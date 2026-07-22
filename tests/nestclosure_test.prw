User Function NestClosTst()
    Local nSoma := 0
    Local aX := {10, 20}
    Local aY := {1, 2}
    // bloco interno captura nSoma (Local da função, 2 níveis acima) e x (param do bloco externo)
    AEval(aX, {|x| AEval(aY, {|y| nSoma := nSoma + x + y }) })
    // (10+1)+(10+2)+(20+1)+(20+2) = 66
    ConOut("soma aninhada (esperado 66): " + Str(nSoma))
Return
