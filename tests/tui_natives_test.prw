// tests/tui_natives_test.prw — fixture para TestTuiNativesFixture (cmd/advplc):
// exercita as novas primitivas de TUI (pkg/vm/ui_render.go) e as natives que
// as acompanham (ConOutRaw, PROCRUN, JsonObject:FromJson real, ConIn EOF).
User Function TuiNativesTest()
    Local cBox     := UiBox("Titulo", "Corpo da caixa", "39", 40)
    Local cStream1 := ""
    Local cMd      := UiMarkdown("**negrito** e item:" + Chr(10) + "- um", 40)
    Local nWidth   := UiTermWidth(80)
    Local oJson    := JsonObject():New()
    Local lOk
    Local aVals
    Local nExit
    Local cSelf    := GetEnv("ADVPLC_SELF_PATH")

    // UiBox: caixa com borda, titulo e corpo — confere presenca literal do
    // texto (o estilo lipgloss usa codigos ANSI ao redor, nao apaga o texto)
    ConOut("BOX_HAS_TITLE=" + cValToChar(".T." $ cBox .Or. "Titulo" $ cBox))
    ConOut("BOX_HAS_BODY=" + cValToChar("Corpo da caixa" $ cBox))

    // UiStreamBox / UiStreamReset: streaming redesenha por cima (ESC[nA)
    UiStreamBox("Turno", "parcial", "212", 40)
    UiStreamBox("Turno", "parcial completo", "212", 40)
    UiStreamReset()
    ConOut("STREAM_OK=.T.")

    // UiMarkdown: glamour renderiza -- o texto "negrito" e "item" sobrevivem
    // ao processamento (com codigos ANSI ao redor, mas o conteudo continua)
    ConOut("MD_HAS_TEXT=" + cValToChar("negrito" $ cMd .And. "item" $ cMd))

    // UiTermWidth: headless (sem tty) cai no default passado
    ConOut("TERMWIDTH=" + Str(nWidth))

    // UiAltScreenEnter/Exit: sequencias ANSI especificas de tela alternativa
    UiAltScreenEnter()
    UiAltScreenExit()
    ConOut("ALTSCREEN_OK=.T.")

    // ConOutRaw: sem newline, concatena na mesma linha
    ConOutRaw("raw1-")
    ConOutRaw("raw2")
    ConOut("")

    // JsonObject:FromJson - parser real (nao mais stub), aninhado
    lOk := oJson:FromJson('{"nome":"AdvPP","versao":2.18,"ok":true,"nulo":null,"itens":["a","b"],"sub":{"x":1}}')
    ConOut("JSON_PARSE_OK=" + cValToChar(lOk))
    ConOut("JSON_NOME=" + oJson["nome"])
    ConOut("JSON_VERSAO=" + Str(oJson["versao"]))
    ConOut("JSON_OK_BOOL=" + cValToChar(oJson["ok"]))
    ConOut("JSON_ITEM2=" + oJson["itens"][2])
    ConOut("JSON_SUB_X=" + Str(oJson["sub"]["x"]))

    // FromJson com JSON invalido: nao crasha, devolve .F.
    lOk := oJson:FromJson("{invalido")
    ConOut("JSON_INVALID_OK=" + cValToChar(!lOk))

    // ConIn no EOF real (stdin fechado/vazio): Nil, nao mais "" -- deixa
    // um REPL distinguir "Enter vazio" de "stdin acabou" e sair do loop
    ConOut("CONIN_EOF_IS_NIL=" + cValToChar(IsNil(ConIn())))

    // ProcRun: executa o proprio advplc (caminho via env ADVPLC_SELF_PATH)
    // com --version, capturando a linha de stdout via codeblock
    If !Empty(cSelf)
        aVals := {}
        nExit := ProcRun(cSelf, {"--version"}, {|cLinha| AAdd(aVals, cLinha)})
        ConOut("PROCRUN_EXIT=" + Str(nExit))
        ConOut("PROCRUN_LINES=" + Str(Len(aVals)))
        If Len(aVals) > 0
            ConOut("PROCRUN_FIRSTLINE=" + aVals[1])
        EndIf
    EndIf
Return
