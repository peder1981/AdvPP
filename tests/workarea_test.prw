// tests/workarea_test.prw — DbAppend/RecLock/FieldPut(via alias->campo)/
// MsUnlock agora persistem de verdade em SQLite (antes eram stubs sem
// efeito — achado confirmado durante o planejamento do GesCon, ver
// CHANGELOG). Usa TCSqlExec só pra preparar/conferir a tabela de teste,
// não pra exercitar o que este fixture testa.
User Function WorkareaTest()
    TCSqlExec("CREATE TABLE IF NOT EXISTS WA_TEST (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, D_E_L_E_T_ TEXT DEFAULT ' ', R_E_C_D_E_L_ INTEGER DEFAULT 0, WA_CODIGO TEXT, WA_VALOR REAL)")
    TCSqlExec("DELETE FROM WA_TEST")

    DbSelectArea("WA_TEST")
    DbAppend()
    RecLock()
    WA_TEST->WA_CODIGO := "X1"
    WA_TEST->WA_VALOR := 42
    MsUnlock()

    Local aConfere := TCSqlQuery("SELECT WA_CODIGO, WA_VALOR FROM WA_TEST")
    ConOut("qtd=" + Str(Len(aConfere)))
    ConOut("codigo=" + aConfere[1]:WA_CODIGO)
    ConOut("valor=" + aConfere[1]:WA_VALOR)

    Local nPos := FieldPos("WA_CODIGO")
    ConOut("fieldpos=" + Str(nPos))
Return
