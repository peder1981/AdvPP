#include "totvs.ch"

// Test 1: Array out-of-bounds access
Local aArr := {1, 2, 3}
ConOut("Test 1: Array out-of-bounds access")
ConOut("  aArr[999] = " + cValToChar(aArr[999]))
ConOut("  Expected: nil")
ConOut("")

// Test 2: SubStr with negative start
Local cStr := "hello"
ConOut("Test 2: SubStr with negative start")
Local cResult := SubStr(cStr, -1, 3)
ConOut("  SubStr('hello', -1, 3) = '" + cResult + "'")
ConOut("  Expected: 'hel' or empty")
ConOut("")

// Test 3: Division by zero
ConOut("Test 3: Division by zero")
Local nDiv := 0
Try
	Local nResult := 1 / nDiv
	ConOut("  1 / 0 = " + cValToChar(nResult))
Catch
	ConOut("  1 / 0 caught an error")
EndTry
ConOut("  Expected: error or inf (no crash)")
ConOut("")

ConOut("All tests completed without crashing!")
Return .T.
