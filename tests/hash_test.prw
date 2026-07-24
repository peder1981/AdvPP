// tests/hash_test.prw — FWHash(cTexto) deve ser determinístico (mesma
// entrada, mesmo hash) e sensível a mudança de entrada.
User Function HashTest()
    Local cHash1 := FWHash("senha123")
    Local cHash2 := FWHash("senha123")
    Local cHash3 := FWHash("outrasenha")
    ConOut("igual=" + cValToChar(cHash1 == cHash2))
    ConOut("diferente=" + cValToChar(cHash1 != cHash3))
    ConOut("tamanho=" + Str(Len(cHash1)))
Return
