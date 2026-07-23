#include "totvs.ch"

// Teste da classe nativa WSRestServer (pkg/rest): expõe funções AdvPL/TLPP
// anotadas com @Get/@Post como rotas de um servidor REST real, rodando
// sobre HTTP. Executar com: advplc run tests/rest_server_test.prw
// (o processo bloqueia servindo HTTP até receber SIGKILL/SIGTERM).
//
// Rotas expostas (porta fixa 18321, usada pelo teste de integração Go em
// cmd/advplc/rest_integration_test.go):
//   GET  /clientes           -> ListaClientes (auto-registrada por @Get)
//   GET  /clientes/{id}      -> GetCliente    (auto-registrada por @Get, path param)
//   POST /clientes           -> NovoCliente   (auto-registrada por @Post)
//   GET  /manual             -> RotaManual    (registrada via AddRoute, sem anotação)

User Function RestServerTst()
    Local oRest := WSRestServer():New("advpp-demo-rest", "1.0.0")
    oRest:AddRoute("GET", "/manual", "RotaManual")
    oRest:Serve(18321)
Return

@Get("/clientes")
User Function ListaClientes(oParam)
Return { { "id": 1, "nome": "Ana" }, { "id": 2, "nome": "Bruno" } }

@Get("/clientes/{id}")
User Function GetCliente(oParam)
    Local jRet := JsonObject():New()
    jRet["id"] := oParam:ID
    jRet["nome"] := "Cliente " + oParam:ID
Return jRet

@Post("/clientes")
User Function NovoCliente(oParam)
    Local jRet := JsonObject():New()
    jRet["criado"] := .T.
    jRet["nome"] := oParam:NOME
Return jRet

User Function RotaManual(oParam)
Return "rota manual OK"
