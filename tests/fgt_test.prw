// Fixture de regressão para FWGetText com modo password (3º argumento opcional).
// Testa caminhos de texto normal e password em execução headless (retorna def).
# include "totvs.ch"

User Function FgtTest()
    Local cTexto := FWGetText("Digite um texto", "default_text")
    ConOut("texto=[" + cTexto + "]")

    Local cSenha := FWGetText("Digite uma senha", "s3nh4", .T.)
    ConOut("senha=[" + cSenha + "]")

    // Chamada com .F. explícito — deve retornar o mesmo que sem o argumento
    Local cNormal := FWGetText("Teste normal", "", .F.)
    ConOut("normal=[" + cNormal + "]")
Return
