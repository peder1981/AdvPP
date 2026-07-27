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

    dNow := Date()
    cTs := Str(dNow) + Str(Seconds())

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

User Function Main()
    Local oChat := ChatContract()
    Local aMessages := {}
    Local oMsg
    Local cCapturedTs

    // Test 1: Empty chat
    aMessages := oChat:GetMessages()
    if len(aMessages) != 0
        ConOut("FAIL: Expected empty chat")
        return .F.
    endif

    // Test 2: Add message
    if !oChat:AddMessage("alice", "hello")
        ConOut("FAIL: AddMessage failed")
        return .F.
    endif

    aMessages := oChat:GetMessages()
    if len(aMessages) != 1
        ConOut("FAIL: Expected 1 message")
        return .F.
    endif

    // Test 3: Idempotence (timestamp-based deduplication)
    // Extract timestamp from first message
    oMsg := aMessages[1]
    cCapturedTs := oMsg:ts

    // Try to add same message again with SAME timestamp (simulating re-received network message)
    if !oChat:AddMessageWithTimestamp("alice", "hello", cCapturedTs)
        ConOut("FAIL: AddMessageWithTimestamp duplicate failed")
        return .F.
    endif

    aMessages := oChat:GetMessages()
    if len(aMessages) != 1
        ConOut("FAIL: Expected 1 message (duplicate ignored)")
        return .F.
    endif

    // Test 4: Different timestamp creates new message
    if !oChat:AddMessageWithTimestamp("alice", "hello", "differenttimestamp")
        ConOut("FAIL: AddMessageWithTimestamp different ts failed")
        return .F.
    endif

    aMessages := oChat:GetMessages()
    if len(aMessages) != 2
        ConOut("FAIL: Expected 2 messages (different timestamp)")
        return .F.
    endif

    ConOut("PASS: All tests passed")
return .T.
