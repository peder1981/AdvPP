User Function FileApi()
    Local nH := 0
    Local nW := 0
    Local cChunk := ""
    Local cAll := ""
    Local nLidos := 0

    // 1. FCreate + FWrite: escreve em pedacos
    nH := FCreate("/tmp/advpp_fh.txt")
    ConOut("1 FCreate handle (>=1): " + Str(nH,2))
    nW := FWrite(nH, "linha um" + Chr(10))
    nW += FWrite(nH, "linha dois" + Chr(10))
    ConOut("1 FWrite bytes escritos (19): " + Str(nW,3))
    FClose(nH)

    // 2. FOpen + FReadStr streaming em blocos de 5 bytes
    nH := FOpen("/tmp/advpp_fh.txt", 0)
    ConOut("2 FOpen handle (>=1): " + Str(nH,2))
    cAll := ""
    cChunk := FReadStr(nH, 5)
    While Len(cChunk) > 0
        cAll += cChunk
        nLidos++
        cChunk := FReadStr(nH, 5)
    End
    FClose(nH)
    ConOut("2 blocos lidos (esperado 4): " + Str(nLidos,2))
    ConOut("2 conteudo total (19 bytes): " + Str(Len(cAll),3))

    // 3. FSeek: volta ao inicio e le 8 bytes
    nH := FOpen("/tmp/advpp_fh.txt", 0)
    FReadStr(nH, 4)
    FSeek(nH, 0, 0)             // rebobina
    cChunk := FReadStr(nH, 8)
    ConOut("3 FSeek+read (esperado 'linha um'): " + cChunk)
    FClose(nH)

    // 4. FError apos handle invalido
    FReadStr(9999, 10)
    ConOut("4 FError handle invalido (esperado 6): " + Str(FError(),1))

    // 5. Captura de stdout de comando via WaitRun + streaming (arquivo grande)
    WaitRun("seq 1 1000 > /tmp/advpp_cmd.txt")
    nH := FOpen("/tmp/advpp_cmd.txt", 0)
    cAll := ""
    cChunk := FReadStr(nH, 256)   // le em blocos de 256 bytes
    While Len(cChunk) > 0
        cAll += cChunk
        cChunk := FReadStr(nH, 256)
    End
    FClose(nH)
    ConOut("5 stdout capturado bytes (>3800): " + Str(Len(cAll),5))
    ConOut("5 ultimos chars (esperado '999<nl>1000<nl>'): " + StrTran(Right(cAll,9), Chr(10), "|"))

    FErase("/tmp/advpp_fh.txt")
    FErase("/tmp/advpp_cmd.txt")
Return
