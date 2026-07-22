// Fixture: contrato de ordem de GetNames/DelName em JsonObject.
// Cobre dois defeitos corrigidos na revisao final:
//   (a) oJson:GetNames() (metodo OO, callNativeMethod "GETNAMES" em vm.go)
//       deve retornar as chaves na ordem de INSERCAO (obj.Keys), igual ao
//       native GetNames(oJson) — antes iterava obj.Props (map Go, ordem
//       aleatoria).
//   (b) oJson:DelName("k") deve remover "k" tanto de Props quanto de Keys
//       (via ObjectValue.DelProp), preservando a ordem relativa das chaves
//       restantes — antes so removia de Props, deixando GetNames listar
//       chaves-fantasma ja deletadas. Alem disso a chave deve ser
//       case-sensitive (mesma semantica de j["x"] := v / SetProp), sem o
//       ToUpper que havia antes.
// Auto-verificavel: imprime "OK: N/N verificacoes passaram." (padrao
// pt_nn.prw / dynleak_test.prw).
User Function GetNamesEdgeTst()
    Local nFail := 0

    nFail += TestMethodOrder()
    nFail += TestDelNameKeepsOrder()
    nFail += TestDelNameCaseSensitive()

    If nFail == 0
        ConOut("OK: 3/3 verificacoes passaram.")
    Else
        ConOut("FALHA: " + Str(nFail,1) + " verificacao(oes) falharam.")
    EndIf
Return

// (a) oJson:GetNames() (metodo OO) preserva ordem de insercao.
Static Function TestMethodOrder()
    Local nErr := 0
    Local oJson := JsonObject():New()
    Local aK := {}
    Local cGot := ""

    oJson["zeta"]  := 1
    oJson["alpha"] := 2
    oJson["meio"]  := 3

    aK := oJson:GetNames()
    cGot := Ordena2Str(aK)

    If cGot != "zeta,alpha,meio"
        ConOut("FALHA (metodo OO ordem): esperado zeta,alpha,meio obtido " + cGot)
        nErr++
    EndIf
Return nErr

// (b) apos DelName, a chave some de GetNames e a ordem das demais persiste.
Static Function TestDelNameKeepsOrder()
    Local nErr := 0
    Local oJson := JsonObject():New()
    Local aK := {}
    Local cGot := ""

    oJson["um"]   := 1
    oJson["dois"] := 2
    oJson["tres"] := 3
    oJson["quatro"] := 4

    oJson:DelName("dois")

    aK := oJson:GetNames()
    cGot := Ordena2Str(aK)

    If cGot != "um,tres,quatro"
        ConOut("FALHA (delname ordem): esperado um,tres,quatro obtido " + cGot)
        nErr++
    EndIf

    If ContemChave(oJson:GetNames(), "dois")
        ConOut("FALHA (delname presenca): 'dois' ainda presente apos DelName")
        nErr++
    EndIf
Return nErr

// DelName deve ser case-sensitive, igual ao bracket/SetProp. Verifica via
// GetNames (reflete Props+Keys diretamente) para nao depender de
// HasProperty, que faz lookup case-insensitive e nao serve para essa
// verificacao.
Static Function TestDelNameCaseSensitive()
    Local nErr := 0
    Local oJson := JsonObject():New()
    Local lDel := .F.

    oJson["Chave"] := 10

    // Chave com casing diferente NAO deve remover (case-sensitive).
    lDel := oJson:DelName("chave")
    If lDel
        ConOut("FALHA (delname case): DelName('chave') removeu 'Chave' (deveria ser case-sensitive)")
        nErr++
    EndIf
    If !ContemChave(oJson:GetNames(), "Chave")
        ConOut("FALHA (delname case): 'Chave' foi removida por engano")
        nErr++
    EndIf

    // Chave exata remove normalmente.
    lDel := oJson:DelName("Chave")
    If !lDel .Or. ContemChave(oJson:GetNames(), "Chave")
        ConOut("FALHA (delname case): DelName('Chave') nao removeu a chave exata")
        nErr++
    EndIf
Return nErr

Static Function ContemChave(aK, cChave)
    Local i := 0
    For i := 1 To Len(aK)
        If aK[i] == cChave
            Return .T.
        EndIf
    Next i
Return .F.

Static Function Ordena2Str(aK)
    Local cRet := ""
    Local i := 0
    For i := 1 To Len(aK)
        If i > 1
            cRet += ","
        EndIf
        cRet += AllTrim(aK[i])
    Next i
Return cRet
