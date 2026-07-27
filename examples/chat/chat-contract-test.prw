#include "chat-contract.prw"

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
