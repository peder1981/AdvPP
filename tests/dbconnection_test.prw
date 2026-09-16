// Fixture de regressão pra classe DbConnection ser alcançável a partir de
// fonte AdvPL real, compilada pelo compilador (não só chamada direta aos
// métodos Go de teste). Ver achado #1 da revisão final da branch multidb:
// "DBCONNECTION" faltava no mapa builtinClasses do compilador, então
// DbConnection():New(...) compilava como chamada de função desconhecida em
// vez de OP_NEW_INSTANCE — a classe nunca era alcançável de AdvPL real.
//
// Connect() contra uma porta fechada em localhost é suficiente pra provar
// que a classe é construível e o método Connect() é despachado de verdade
// (sem precisar de um servidor de banco real disponível no CI).
User Function DbConnTest()
    Local oConn := DbConnection():New("POSTGRES", "127.0.0.1", 1, "nodb", "nouser", "nopass")
    Local lOk := oConn:Connect()

    If lOk
        ConOut("connect=true")
    Else
        ConOut("connect=false")
    EndIf

    If Len(oConn:GetError()) > 0
        ConOut("temerro=true")
    Else
        ConOut("temerro=false")
    EndIf

    oConn:Close()
Return
