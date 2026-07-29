// Fixture usada por TestBuildStandaloneInteractive (cmd/advplc): exercita,
// num executável standalone (`advplc build`) real rodando sob um PTY real,
// o caminho que standalone_console_test.prw deliberadamente não cobre —
// FWGetText/FWMenuSelect lendo teclado e FWMBrowse fazendo CRUD. Sem isso
// nenhum teste automatizado teria pego a regressão real: um `advplc build`
// que compilava e "rodava" mas nunca lia stdin (login silencioso, saída
// instantânea) e um FWMBrowse que só existia para o modo web.
#include "totvs.ch"

User Function Main()
    Local cNome := ""
    Local nOp := 0
    Local oBrowse := Nil

    TCSqlExec("CREATE TABLE IF NOT EXISTS ITST (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, ITST_NOME CHAR(40), D_E_L_E_T_ CHAR(1) DEFAULT ' ')")

    cNome := FWGetText("Seu nome", "")
    ConOut("NOME_LIDO=" + cNome)

    nOp := FWMenuSelect({"Ir para o browse", "Sair"}, "Menu de teste")
    ConOut("MENU_ESCOLHIDO=" + Str(nOp, 1))
    If nOp != 1
        Return .T.
    EndIf

    oBrowse := FWMBrowse():New()
    oBrowse:SetAlias("ITST")
    oBrowse:SetDescription("Registros de teste")
    oBrowse:Activate()

    ConOut("FIM_DO_TESTE")
Return .T.
