// tests/http_native_test.prw
// Testa as natives HTTP (FWHttpGet, FWHttpPost, FWHttpPut, FWHttpPatch,
// FWHttpDelete, FWHttpBody, FWHttpStatus, FWHttpError) com servidor de
// teste local (iniciado pelo Go test http_native_test.go).
// O Go test passa a URL do servidor via variável de ambiente ADVPP_TEST_URL.

User Function HttpNativeTest()
    Local cTestUrl := GetEnv("ADVPP_TEST_URL")
    If cTestUrl == ""
        ConOut("status=skip_url_nao_configurada")
        Return
    EndIf

    // --- GET ---
    Local nStatus := FWHttpGet(cTestUrl + "/echo-get?nome=teste", "", "")
    ConOut("get_status=" + Str(nStatus))
    If nStatus > 0
        ConOut("get_body_ok=" + IIf(FWHttpBody() != "", "1", "0"))
        ConOut("get_last_status=" + Str(FWHttpStatus()))
    EndIf

    // --- POST ---
    nStatus := FWHttpPost(cTestUrl + "/echo-post", '{"teste":123}', "application/json", "", "")
    ConOut("post_status=" + Str(nStatus))
    If nStatus > 0
        ConOut("post_body_ok=" + IIf(FWHttpBody() != "", "1", "0"))
    EndIf

    // --- PUT ---
    nStatus := FWHttpPut(cTestUrl + "/echo-put", '{"id":1}', "application/json", "", "")
    ConOut("put_status=" + Str(nStatus))

    // --- PATCH ---
    nStatus := FWHttpPatch(cTestUrl + "/echo-patch", '{"campo":"valor"}', "application/json", "", "")
    ConOut("patch_status=" + Str(nStatus))

    // --- DELETE ---
    nStatus := FWHttpDelete(cTestUrl + "/echo-delete", "", "")
    ConOut("delete_status=" + Str(nStatus))

    // --- URL inválida (deve retornar 0 sem crash) ---
    nStatus := FWHttpGet("http://192.0.2.1:1/teste", "", "")
    ConOut("erro_status=" + Str(nStatus))
    ConOut("erro_msg_vazia=" + IIf(FWHttpError() == "", "1", "0"))
    IIf(nStatus == 0, ConOut("erro_tratado=1"), ConOut("erro_tratado=0"))

    ConOut("--- HttpNativeTest FIM ---")
Return
