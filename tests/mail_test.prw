// tests/mail_test.prw — TMailMessage: caminho de validação (Send() sem
// configurar servidor/remetente/destinatário deve falhar com erro claro,
// não travar nem enviar nada). Envio real de rede não é testável aqui sem
// um servidor SMTP disponível.
User Function MailTest()
    Local oMail := TMailMessage():New()
    Local lErro := .F.

    Begin Sequence
        oMail:Send()
    Recover
        lErro := .T.
    End Sequence

    ConOut("erro_sem_config=" + cValToChar(lErro))

    oMail:SetServer("smtp.exemplo.invalido", "587")
    oMail:SetFrom("sindico@exemplo.invalido")
    oMail:AddTo("condomino@exemplo.invalido")
    oMail:SetSubject("Teste")
    oMail:SetBody("Corpo do teste")

    // Servidor não existe de verdade — Send() deve falhar (erro de rede),
    // não travar o processo nem devolver sucesso falso.
    Local lErro2 := .F.
    Begin Sequence
        oMail:Send()
    Recover
        lErro2 := .T.
    End Sequence
    ConOut("erro_servidor_invalido=" + cValToChar(lErro2))
Return
