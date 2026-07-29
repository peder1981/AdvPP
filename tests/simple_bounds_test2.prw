#include "totvs.ch"

ConOut("Test 1: Array access at valid index")
Local aArr := {10, 20, 30}
ConOut("  aArr[1] = " + cValToChar(aArr[1]))
ConOut("  Expected: 10")
ConOut("")

ConOut("Test 2: Array access at out-of-bounds index")
ConOut("  aArr[999] = " + cValToChar(aArr[999]))
ConOut("  Expected: Nil")
ConOut("")

ConOut("Test 3: Array access at negative index")
ConOut("  aArr[-1] = " + cValToChar(aArr[-1]))
ConOut("  Expected: Nil")
ConOut("")

ConOut("Test 4: SubStr with valid params")
Local cStr := "hello"
ConOut("  SubStr('hello', 1, 3) = '" + SubStr(cStr, 1, 3) + "'")
ConOut("  Expected: 'hel'")
ConOut("")

ConOut("Test 5: SubStr with negative start")
ConOut("  SubStr('hello', -1, 3) = '" + SubStr(cStr, -1, 3) + "'")
ConOut("  Expected: 'hel' (clamped to 1)")
ConOut("")

ConOut("Test 6: SubStr with out-of-bounds start")
ConOut("  SubStr('hello', 100, 3) = '" + SubStr(cStr, 100, 3) + "'")
ConOut("  Expected: empty")
ConOut("")

ConOut("Test 7: At() with valid substring")
ConOut("  At('ll', 'hello') = " + cValToChar(At("ll", "hello")))
ConOut("  Expected: 3")
ConOut("")

ConOut("Test 8: At() with empty search string")
ConOut("  At('', 'hello') = " + cValToChar(At("", "hello")))
ConOut("  Expected: 0")
ConOut("")

ConOut("Test 9: Division by zero")
Local nResult := 1 / 0
ConOut("  Type of result: " + Type(nResult))
ConOut("  Expected: O (ErrorValue) or N (Number)")
ConOut("")

ConOut("Test 10: Modulo by zero")
Local nMod := 5 % 0
ConOut("  Type of result: " + Type(nMod))
ConOut("  Expected: O (ErrorValue) or N (Number)")
ConOut("")

ConOut("All tests completed without crashing!")
Return .T.
