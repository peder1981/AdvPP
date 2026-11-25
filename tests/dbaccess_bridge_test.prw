// Fixture E2E do driver DBACCESS (bridge -> dbapi.so -> dbaccess64).
// Só produz saida util quando ADVPP_DBAPI_BRIDGE esta no ar e o dbaccess64
// responde — o teste Go que a invoca e' gated por ADVPP_DBAPI_IT=1.
User Function DbAccessBridgeTest()
    Local oConn := DbConnection():New("DBACCESS", "127.0.0.1", 7890, "POSTGRES/p1212510", "", "")

    If !oConn:Connect()
        ConOut("connect=false")
        ConOut("erro=" + oConn:GetError())
        Return
    EndIf
    ConOut("connect=true")

    // TCSQLQUERY usa a conexao propria da VM (ownConnID).
    Local aRows := TCSQLQUERY("SELECT X2_CHAVE AS CHAVE, X2_NOME AS NOME FROM SX2990 LIMIT 3")
    ConOut("rows=" + cValToChar(Len(aRows)))
    If Len(aRows) > 0
        ConOut("row0.chave=" + aRows[1]["CHAVE"])
        ConOut("row0.nome=" + aRows[1]["NOME"])
    EndIf

    // Workarea: DBSetDriver TOPCONN + DBUseArea cai no engine dbapi da
    // conexao propria. DBUseArea retorna Nil sempre — detecta sucesso
    // por RecCount()>0.
    DBSetDriver("TOPCONN")
    DBUseArea(.T., "TOPCONN", "SX2990", "SX2990")
    DbGoTop()
    ConOut("reccount=" + cValToChar(RecCount()))
    If RecCount() > 0
        ConOut("usearea=true")
        ConOut("recno=" + cValToChar(RecNo()))
        ConOut("nome=" + SX2990->X2_NOME)
        DbCloseArea()
    Else
        ConOut("usearea=false")
    EndIf

    // Workarea por ALIAS: SE1 resolve para o nome físico SE1990 via
    // SX2.X2_ARQUIVO (dbapi/OpenFile não resolve alias sozinho).
    ConOut("sqlname=" + RetSqlName("SE1"))
    DBUseArea(.T., "TOPCONN", "SE1", "SE1")
    DbGoTop()
    ConOut("se1count=" + cValToChar(RecCount()))
    If RecCount() > 0
        ConOut("se1=true")
        ConOut("se1doc=" + SE1->E1_DOC)
        ConOut("se1emis=" + DtoC(SE1->E1_EMISSAO))

        // FLock/FUnLock reais no dbAccess + DBDelete/DBRecall ISAM
        // (DeleteR/RecallR — marcação imediata de D_E_L_E_T_).
        ConOut("flock=" + Iif(FLock(), "ok", "fail"))
        ConOut("funlock=" + Iif(FUnLock(), "ok", "fail"))
        DBDelete()
        ConOut("se1del=" + Iif(Deleted(), "sim", "nao"))
        DBRecall()
        ConOut("se1rec=" + Iif(Deleted(), "sim", "nao"))
        DbCloseArea()
    Else
        ConOut("se1=false")
    EndIf

    // Erro de SQL deve voltar tratavel, nao derrubar o programa.
    Local aBad := TCSQLQUERY("SELECT * FROM TABELA_INEXISTENTE_XX")
    ConOut("badrows=" + cValToChar(Len(aBad)))
    ConOut("baderr=" + TCSqlError())

    oConn:Close()
    ConOut("fim=true")
Return
