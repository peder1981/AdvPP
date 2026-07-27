User Function ChatMain()
    Local cPeerId := "peer-default"
    Local cListen := "127.0.0.1:9000"

    ConOut("=== AdvPP Freenet Distributed Chat ===")
    ConOut("Peer: " + cPeerId)
    ConOut("Listening: " + cListen)

    // TODO: ChatInit(cPeerId, cListen)
    // TODO: ChatLoop()

    ConOut("Chat closed.")
return
endFunc

Static Function ChatInit(cPeerId, cListen)
    ConOut("TODO: Initialize P2P peer")
return

Static Function ChatLoop()
    Local nOpt := 0

    do while .T.
        ConOut("")
        ConOut("Chat Menu:")
        ConOut("  1. Read messages")
        ConOut("  2. Send message")
        ConOut("  3. Peer info")
        ConOut("  4. Exit")
        accept nOpt

        do case
            case nOpt == 1
                ChatRead()
            case nOpt == 2
                ChatSend()
            case nOpt == 3
                ChatInfo()
            case nOpt == 4
                exit
        endcase
    enddo
return

Static Function ChatRead()
    ConOut("TODO: Fetch and display messages")
return

Static Function ChatSend()
    Local cText := ""
    ConOut("Message (empty to cancel): ")
    accept cText
    if !empty(cText)
        ConOut("TODO: Send message: " + cText)
    endif
return

Static Function ChatInfo()
    ConOut("TODO: Show peer info")
return
