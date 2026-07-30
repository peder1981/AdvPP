User Function TestSimple()
	ConOut("Test started")
	Local aArr := {}
	Local i := 1
	For i := 1 To 100
		aAdd(aArr, i)
	Next
	ConOut("Array size: " + cValToChar(Len(aArr)))
Return .T.
