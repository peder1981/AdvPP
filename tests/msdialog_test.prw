#include "totvs.ch"

// Teste da fase 4 do renderer web: MSDIALOG legado (@ x,y SAY/GET/BUTTON)
// renderizado como modal PO-UI por heurística de grade.
// Executar com: advplc serve tests/msdialog_test.prw
User Function DlgTst()
    Local oDlg
    Local cNome  := "MARIA DA SILVA"
    Local cCid   := "SAO PAULO"
    Local nIdade := 30

    ConOut("Abrindo dialogo legado...")

    DEFINE MSDIALOG oDlg TITLE "Cadastro rapido" FROM 0,0 TO 200,400 PIXEL

    @ 10, 10 SAY "Nome:"   PIXEL
    @ 10, 60 GET cNome     PIXEL
    @ 30, 10 SAY "Cidade:" PIXEL
    @ 30, 60 GET cCid      PIXEL
    @ 50, 10 SAY "Idade:"  PIXEL
    @ 50, 60 GET nIdade    PIXEL
    @ 80, 10 BUTTON "Confirmar" SIZE 40, 12 PIXEL

    ACTIVATE MSDIALOG oDlg CENTERED

    ConOut("Nome digitado..: " + cNome)
    ConOut("Cidade digitada: " + cCid)
    ConOut("Idade digitada.: " + cValToChar(nIdade))
Return
