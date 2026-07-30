User Function TestSpace()
	ConOut("Before Space")
	Local cStr := Space(100000)
	ConOut("After Space: " + cValToChar(Len(cStr)))
Return .T.
