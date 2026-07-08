// Debug test for For loop
User Function TestFor()
    Local nI := 0
    Local nSum := 0

    ConOut("Before for loop")

    For nI := 1 To 5
        ConOut("Inside loop: nI=" + Str(nI))
        nSum := nSum + nI
    Next

    ConOut("After for loop")
    ConOut("Sum = " + Str(nSum))
    ConOut("nI = " + Str(nI))
Return
