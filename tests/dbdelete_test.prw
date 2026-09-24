// tests/dbdelete_test.prw — DbDelete() marca D_E_L_E_T_ = '*', Deleted()
// reflete o estado, e DbRecall() desfaz. Antes, DbDelete era um stub no-op
// (FULL-REVIEW A1a): a exclusão sumia silenciosamente e Deleted() nunca
// virava .T. após um DbDelete.
User Function DbDeleteTest()
    TCSqlExec("CREATE TABLE IF NOT EXISTS DD_TEST (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, D_E_L_E_T_ TEXT DEFAULT ' ', R_E_C_D_E_L_ INTEGER DEFAULT 0, DD_CODIGO TEXT)")
    TCSqlExec("DELETE FROM DD_TEST")

    DbSelectArea("DD_TEST")
    DbAppend()
    RecLock()
    DD_TEST->DD_CODIGO := "A"
    MsUnlock()

    DbGoTop()
    ConOut("antes=" + cValToChar(Deleted()))

    RecLock()
    DbDelete()
    MsUnlock()
    ConOut("apos_delete=" + cValToChar(Deleted()))

    RecLock()
    DbRecall()
    MsUnlock()
    ConOut("apos_recall=" + cValToChar(Deleted()))

    ConOut("dbdelete-test OK")
Return
