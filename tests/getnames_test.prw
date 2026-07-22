User Function GetNamesTst()
    Local j := JsonObject():New()
    Local aK := {}
    Local i := 0
    Local cCat := ""
    j["um"]   := 1
    j["dois"] := 2
    j["tres"] := 3
    aK := GetNames(j)
    ConOut("count (esperado 3): " + Str(Len(aK)))
    For i := 1 To Len(aK)
        cCat += AllTrim(aK[i]) + ","
    Next i
    ConOut("ordem (esperado um,dois,tres,): " + cCat)
Return
