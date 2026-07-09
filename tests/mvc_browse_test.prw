#include "totvs.ch"

// Teste da fase 2 do renderer web: FWMBrowse sobre a SA1 renderizado
// como po-table no browser (colunas do dicionário SX3, CRUD via
// po-dynamic-form). Executar com: advplc serve tests/mvc_browse_test.prw
User Function BrwTst()
    Local oBrowse

    ConOut("Abrindo cadastro de clientes (SA1)...")

    oBrowse := FWMBrowse():New()
    oBrowse:SetAlias("SA1")
    oBrowse:SetDescription("Cadastro de Clientes")
    oBrowse:Activate()

    ConOut("Browse encerrado.")
Return
