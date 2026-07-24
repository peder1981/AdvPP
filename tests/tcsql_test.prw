// Fixture de regressão pras natives TCSqlExec/TCSqlQuery (SQL direto pra
// User Function, motor de persistência real do FWMBrowse exposto ao AdvPL —
// ver CHANGELOG). Cria uma tabela de teste, insere, consulta e confere.
User Function TCSqlTest()
    Local aRows

    TCSqlExec("CREATE TABLE IF NOT EXISTS TCSQL_TEST (T1_CODIGO TEXT, T1_VALOR REAL)")
    TCSqlExec("DELETE FROM TCSQL_TEST")
    TCSqlExec("INSERT INTO TCSQL_TEST (T1_CODIGO, T1_VALOR) VALUES ('A1', 10.5)")
    TCSqlExec("INSERT INTO TCSQL_TEST (T1_CODIGO, T1_VALOR) VALUES ('A2', 20.5)")

    aRows := TCSqlQuery("SELECT T1_CODIGO, T1_VALOR FROM TCSQL_TEST ORDER BY T1_CODIGO")

    ConOut("linhas=" + Str(Len(aRows)))
    ConOut("linha1_codigo=" + aRows[1]:T1_CODIGO)
    ConOut("linha1_valor=" + aRows[1]:T1_VALOR)
    ConOut("linha2_codigo=" + aRows[2]:T1_CODIGO)
Return
