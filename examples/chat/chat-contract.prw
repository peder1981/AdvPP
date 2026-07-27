class ChatContract
    private data aMessages as Array

    method new() as object Constructor
    method GetMessages() as Array
    method AddMessage(cFrom as Character, cText as Character) as Logical
    method AddMessageWithTimestamp(cFrom as Character, cText as Character, cTs as Character) as Logical
endclass

method new() as object class ChatContract
    ::aMessages := {}
return Self

method GetMessages() as Array class ChatContract
    Local aResult := {}
    Local i
    for i := 1 to len(::aMessages)
        aAdd(aResult, ::aMessages[i])
    next
return aResult

method AddMessage(cFrom as Character, cText as Character) as Logical class ChatContract
    Local dNow as Date
    Local cTs as Character

    // Timestamp (simplified: system time as string)
    dNow := Date()
    cTs := Str(dNow) + Str(Seconds())

    // Use AddMessageWithTimestamp with generated timestamp
return ::AddMessageWithTimestamp(cFrom, cText, cTs)

method AddMessageWithTimestamp(cFrom as Character, cText as Character, cTs as Character) as Logical class ChatContract
    Local oMsg as Object
    Local i

    // Check if message already exists (idempotence)
    for i := 1 to len(::aMessages)
        oMsg := ::aMessages[i]
        if oMsg:from == cFrom .and. oMsg:ts == cTs
            return .T. // Already exists, return success
        endif
    next

    // New message
    oMsg := JsonObject():New()
    oMsg:from := cFrom
    oMsg:text := cText
    oMsg:ts := cTs

    aAdd(::aMessages, oMsg)
return .T.
