User Function TestSpace1MB()
	ConOut("Before Space 1MB")
	Local cStr := Space(1048576)
	ConOut("After Space 1MB: " + cValToChar(Len(cStr)))
Return .T.
