#include "totvs.ch"

// Teste de integração ponta-a-ponta pra RpcSetEnv/FWxFilial:
// verifica que as natives de estado de sessão multi-filial funcionam
// e que FWxFilial retorna a filial truncada conforme o nível de
// compartilhamento da tabela (default: nível 6, exclusiva).
User Function MultiFilialTest()
    Local nFail := 0

    ConOut("========== Teste de Multi-Filial ==========")

    // Teste 1: RpcSetEnv e FWxFilial
    ConOut("")
    ConOut("Teste 1: RpcSetEnv + FWxFilial")
    ConOut("---")

    RpcSetEnv("010101")
    ConOut("filial 010101: " + FWxFilial("SA1"))

    If FWxFilial("SA1") != "010101"
        ConOut("FALHA: FWxFilial retornou " + FWxFilial("SA1") + " esperado 010101")
        nFail++
    EndIf

    RpcSetEnv("010102")
    ConOut("filial 010102: " + FWxFilial("SA1"))

    // Sem X2_FILIAL_COMPART pra SA1: default nivel 6, devolve a propria
    // filial ativa inteira.
    If FWxFilial("SA1") == "010102"
        ConOut("PASS: FWxFilial sem config usa nivel 6")
    Else
        ConOut("FALHA: FWxFilial sem config = " + FWxFilial("SA1"))
        nFail++
    EndIf

    // Teste 2: Criação de tabela com FILIAL e validação
    ConOut("")
    ConOut("Teste 2: Tabela com coluna FILIAL")
    ConOut("---")

    TCSqlExec("DROP TABLE IF EXISTS MFT_TEST")
    TCSqlExec("CREATE TABLE MFT_TEST (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, D_E_L_E_T_ TEXT DEFAULT ' ', R_E_C_D_E_L_ INTEGER DEFAULT 0, MFT_CODIGO TEXT, FILIAL TEXT)")

    RpcSetEnv("010101")
    ConOut("Inserindo registro em filial 010101")
    TCSqlExec("INSERT INTO MFT_TEST (MFT_CODIGO, FILIAL) VALUES ('REC1', '" + FWxFilial("MFT_TEST") + "')")

    RpcSetEnv("010102")
    ConOut("Inserindo registro em filial 010102")
    TCSqlExec("INSERT INTO MFT_TEST (MFT_CODIGO, FILIAL) VALUES ('REC2', '" + FWxFilial("MFT_TEST") + "')")

    Local aRows := TCSqlQuery("SELECT MFT_CODIGO, FILIAL FROM MFT_TEST ORDER BY MFT_CODIGO")

    If Len(aRows) == 2
        ConOut("PASS: Dois registros inseridos")
        ConOut("  REC1 FILIAL=" + aRows[1]:FILIAL)
        ConOut("  REC2 FILIAL=" + aRows[2]:FILIAL)
    Else
        ConOut("FALHA: Esperado 2 registros, encontrado " + Str(Len(aRows)))
        nFail++
    EndIf

    ConOut("")
    If nFail == 0
        ConOut("========== OK: todos os testes passaram ==========")
    Else
        ConOut("========== FALHA: " + Str(nFail,1) + " verificacoes falharam ==========")
    EndIf

Return
