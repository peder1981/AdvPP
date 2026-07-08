// Hello World - basic AdvPL test
User Function Hello()
    Local cMsg := "Hello World from AdvPL Compiler!"
    Local nNum := 42
    Local lFlag := .T.

    ConOut(cMsg)
    ConOut("Number: " + Str(nNum))
    ConOut("Flag: " + IIF(lFlag, "True", "False"))

    // Test arithmetic
    Local nA := 10
    Local nB := 3
    ConOut("Add: " + Str(nA + nB))
    ConOut("Sub: " + Str(nA - nB))
    ConOut("Mul: " + Str(nA * nB))
    ConOut("Div: " + Str(nA / nB))

    // Test string functions
    ConOut("Upper: " + Upper("hello"))
    ConOut("Lower: " + Lower("WORLD"))
    ConOut("AllTrim: [" + AllTrim("  spaces  ") + "]")
    ConOut("SubStr: " + SubStr("Hello World", 1, 5))
    ConOut("Len: " + Str(Len("Hello")))

    // Test if/else
    If nA > nB
        ConOut("nA is greater than nB")
    Else
        ConOut("nA is not greater than nB")
    EndIf

    // Test for loop
    Local nSum := 0
    Local nI := 0
    For nI := 1 To 10
        nSum := nSum + nI
    Next
    ConOut("Sum 1-10: " + Str(nSum))

    // Test while loop
    Local nCount := 0
    While nCount < 5
        ConOut("Count: " + Str(nCount))
        nCount := nCount + 1
    EndDo

    // Test array
    Local aItems := {1, 2, 3, 4, 5}
    ConOut("Array len: " + Str(Len(aItems)))
    ConOut("Array[1]: " + Str(aItems[1]))
    ConOut("Array[3]: " + Str(aItems[3]))

    // Test do case
    Local nDay := 3
    Do Case
        Case nDay == 1
            ConOut("Monday")
        Case nDay == 2
            ConOut("Tuesday")
        Case nDay == 3
            ConOut("Wednesday")
        Otherwise
            ConOut("Other day")
    EndCase

    ConOut("Done!")
Return
