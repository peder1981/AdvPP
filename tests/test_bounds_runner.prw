#include "totvs.ch"

Function TestArrayGetNegativeIndex()
	Local aArr := {1, 2, 3}
	Local uResult := Nil

	// Negative index should return nil or error, not crash
	uResult := aArr[-1]

	// Should not crash; result should be nil or error
	Return (uResult == Nil)
EndFunction

Function TestArrayGetOutOfBounds()
	Local aArr := {1, 2, 3}
	Local uResult := Nil

	// Out of bounds access should return nil, not crash
	uResult := aArr[999]

	Return uResult == Nil
EndFunction

Function TestDivisionByZero()
	Local nResult := 0

	// Division by zero should return error or inf, not crash
	nResult := 1 / 0

	// If we reach here, it was safe (no crash)
	Return .T.
EndFunction

Function TestSubStrNegativeStart()
	Local cStr := "hello"
	Local cResult := SubStr(cStr, -1, 3)

	// Negative start should be clamped to 1 or return empty
	Return (cResult == "") .Or. (cResult == "hel")
EndFunction

Function Main()
	Local nPassed := 0
	Local nFailed := 0

	ConOut("Running bounds checking tests...")
	ConOut("")

	If TestArrayGetNegativeIndex()
		nPassed++
		ConOut("✓ TestArrayGetNegativeIndex")
	Else
		nFailed++
		ConOut("✗ TestArrayGetNegativeIndex")
	EndIf

	If TestArrayGetOutOfBounds()
		nPassed++
		ConOut("✓ TestArrayGetOutOfBounds")
	Else
		nFailed++
		ConOut("✗ TestArrayGetOutOfBounds")
	EndIf

	If TestDivisionByZero()
		nPassed++
		ConOut("✓ TestDivisionByZero")
	Else
		nFailed++
		ConOut("✗ TestDivisionByZero")
	EndIf

	If TestSubStrNegativeStart()
		nPassed++
		ConOut("✓ TestSubStrNegativeStart")
	Else
		nFailed++
		ConOut("✗ TestSubStrNegativeStart")
	EndIf

	ConOut("")
	ConOut("Results:")
	ConOut("  Passed: " + cValToChar(nPassed))
	ConOut("  Failed: " + cValToChar(nFailed))
	ConOut("")

	Return (nFailed == 0)
EndFunction
