#include "totvs.ch"

User Function MultiDbSmoke()
    Local oConn := DbConnection():New("POSTGRES", GetEnv("ADVPP_PG_HOST"), 5432, "meubanco", GetEnv("ADVPP_PG_USER"), GetEnv("ADVPP_PG_PASSWORD"))
    Local aRows

    If !oConn:Connect()
        ConOut("Falha ao conectar: " + oConn:GetError())
        Return
    EndIf

    aRows := TCGenQry("SELECT * FROM CLIENTES")
    ConOut("Linhas: " + cValToChar(Len(aRows)))

    oConn:Close()
Return
