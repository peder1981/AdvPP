// tests/tcsql_erro_test.prw -- erro de SQL em TCSqlExec/TCSqlQuery não
// encerra o programa: TCSqlExec devolve .F., TCSqlQuery devolve {} e a
// mensagem fica em TCSqlError() (como no Protheus, onde o erro também volta
// para o programa). Um "BEGIN; ...; COMMIT;" que falha no meio é desfeito.
User Function TCSqlErro()
    Local lOk := .T.
    Local aRows := {}

    TCSqlExec("DROP TABLE IF EXISTS TCSQL_ERRO")
    TCSqlExec("CREATE TABLE TCSQL_ERRO (A INTEGER NOT NULL)")

    lOk := TCSqlExec("INSERT INTO TCSQL_ERRO VALUES (NULL)")
    ConOut("exec_com_erro=" + cValToChar(lOk))
    ConOut("erro_tem_notnull=" + cValToChar("NOT NULL" $ Upper(TCSqlError())))

    lOk := TCSqlExec("INSERT INTO TCSQL_ERRO VALUES (7)")
    ConOut("exec_ok=" + cValToChar(lOk))
    ConOut("erro_limpo=" + cValToChar(Empty(TCSqlError())))

    aRows := TCSqlQuery("SELECT * FROM TABELA_QUE_NAO_EXISTE")
    ConOut("query_com_erro_linhas=" + cValToChar(Len(aRows)))
    ConOut("query_erro_msg=" + cValToChar(!Empty(TCSqlError())))

    lOk := TCSqlExec("BEGIN;" + Chr(10) + "INSERT INTO TCSQL_ERRO VALUES (8);" + Chr(10) + ;
        "INSERT INTO TCSQL_ERRO VALUES (NULL);" + Chr(10) + "COMMIT;")
    ConOut("bloco_com_erro=" + cValToChar(lOk))
    aRows := TCSqlQuery("SELECT COUNT(*) AS Q FROM TCSQL_ERRO")
    ConOut("linhas_depois_do_bloco=" + aRows[1]:Q)

    TCSqlExec("DROP TABLE TCSQL_ERRO")
    ConOut("chegou_ao_fim=.T.")
Return
