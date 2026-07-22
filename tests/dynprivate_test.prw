// Fixture: escopo dinâmico de Private — visibilidade descendente entre
// funções (a função chamada enxerga o Private do chamador) e restauração do
// binding sombreado quando o frame que o declarou retorna.
// Auto-verificável: compara valor esperado x obtido, imprime
// "OK: N/N verificacoes passaram" (padrão de pt_nn.prw).
User Function DynPrivTst()
    Local nFail := 0
    Private cCtx := "A"

    // 1. ModB() enxerga o Private "cCtx" declarado no chamador (escopo dinâmico)
    If ModB() != "A"
        ConOut("FALHA: ModB nao viu cCtx=A do chamador")
        nFail++
    EndIf

    // 2. ModB() alterou cCtx (mesmo binding dinâmico, sem Private próprio novo)
    If cCtx != "Z"
        ConOut("FALHA: alteracao de cCtx em ModB nao refletiu no chamador (obtido: " + cCtx + ")")
        nFail++
    EndIf

    // 3. ModC() declara seu PRÓPRIO Private cCtx (sombreando); ao retornar,
    //    o binding do chamador deve ser restaurado intacto.
    ModC()
    If cCtx != "Z"
        ConOut("FALHA: Private sombreado em ModC nao foi restaurado (obtido: " + cCtx + ")")
        nFail++
    EndIf

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("FALHA: " + Str(nFail,1) + " verificacao(oes) falharam.")
    EndIf
Return

// Enxerga cCtx do chamador via escopo dinâmico (nenhum Private próprio) e o altera.
Static Function ModB()
    Local cSeen := cCtx
    cCtx := "Z"
Return cSeen

// Declara seu próprio Private cCtx (sombreia o do chamador); some ao retornar.
Static Function ModC()
    Private cCtx := "TEMP-C"
Return
