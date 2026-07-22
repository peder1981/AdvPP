/*/{Protheus.doc} AlgosAdvpl
    Biblioteca de algoritmos classicos de logica / leetcode / script, 100% em AdvPL
    (AdvPP). Cada rotina e uma Static Function pura e testavel; o User Function
    AlgosAdvpl() roda asserts sobre todas. Serve tambem de corpus de treino para o
    modelo de codigo dev_nn.prw (viés algoritmico).
    @type  user function
    @author AdvPP
    @since 2026-07-22
/*/
User Function AlgosAdvpl()
    Local nFail := 0

    nFail += Verifica("Fatorial",     Fatorial(5) == 120)
    nFail += Verifica("Fibonacci",    Fibonacci(10) == 55)
    nFail += Verifica("MDC",          MDC(48, 36) == 12)
    nFail += Verifica("MMC",          MMC(4, 6) == 12)
    nFail += Verifica("EhPrimo",      EhPrimo(97) .And. !EhPrimo(91))
    nFail += Verifica("Potencia",     Potencia(2, 10) == 1024)
    nFail += Verifica("SomaDigitos",  SomaDigitos(12345) == 15)
    nFail += Verifica("Reverso",      Reverso("advpl") == "lpvda")
    nFail += Verifica("Palindromo",   Palindromo("arara") .And. !Palindromo("casa"))
    nFail += Verifica("ContaVogais",  ContaVogais("programacao") == 5)
    nFail += Verifica("Anagrama",     Anagrama("amor", "roma") .And. !Anagrama("abc", "abd"))
    nFail += Verifica("BuscaLinear",  BuscaLinear({3,7,1,9,4}, 9) == 4)
    nFail += Verifica("BuscaBinaria", BuscaBinaria({1,3,5,7,9,11}, 7) == 4)
    nFail += Verifica("BolhaSort",    ArrIgual(BolhaSort({5,2,9,1,7}), {1,2,5,7,9}))
    nFail += Verifica("QuickSort",    ArrIgual(QuickSort({8,3,5,1,9,2}), {1,2,3,5,8,9}))
    nFail += Verifica("InsercaoSort", ArrIgual(InsercaoSort({4,1,3,2}), {1,2,3,4}))
    nFail += Verifica("Maximo",       Maximo({3,8,1,9,4}) == 9)
    nFail += Verifica("SomaArray",    SomaArray({1,2,3,4,5}) == 15)
    nFail += Verifica("Kadane",       Kadane({-2,1,-3,4,-1,2,1,-5,4}) == 6)
    nFail += Verifica("DoisSoma",     ArrIgual(DoisSoma({2,7,11,15}, 9), {1,2}))
    nFail += Verifica("FizzBuzz",     FizzBuzz(15) == "FizzBuzz" .And. FizzBuzz(9) == "Fizz")
    nFail += Verifica("ParentesesOk", ParentesesOk("(()())") .And. !ParentesesOk("(()"))
    nFail += Verifica("TrocaMoedas",  TrocaMoedas({1,3,4}, 6) == 2)
    nFail += Verifica("LCS",          LCS("abcde", "ace") == 3)
    nFail += Verifica("ContaPalavras", ContaPalavras("o rato roeu a roupa") == 5)

    If nFail == 0
        ConOut("OK: todos os algoritmos passaram.")
    Else
        ConOut("FALHA: " + Str(nFail,2) + " algoritmo(s) com erro.")
    EndIf
Return

Static Function Verifica(cNome, lOk)
    If !lOk
        ConOut("  FALHA: " + cNome)
        Return 1
    EndIf
Return 0

Static Function ArrIgual(a, b)
    Local i := 0
    If Len(a) != Len(b)
        Return .F.
    EndIf
    For i := 1 To Len(a)
        If a[i] != b[i]
            Return .F.
        EndIf
    Next i
Return .T.

// ---------------------------------------------------------------- Recursao / matematica

Static Function Fatorial(n)
    If n <= 1
        Return 1
    EndIf
Return n * Fatorial(n - 1)

Static Function Fibonacci(n)
    Local a := 0
    Local b := 1
    Local t := 0
    Local i := 0
    For i := 1 To n
        t := a + b
        a := b
        b := t
    Next i
Return a

Static Function MDC(a, b)
    Local t := 0
    Do While b != 0
        t := b
        b := a % b
        a := t
    EndDo
Return a

Static Function MMC(a, b)
Return (a * b) / MDC(a, b)

Static Function EhPrimo(n)
    Local i := 0
    If n < 2
        Return .F.
    EndIf
    For i := 2 To Int(Sqrt(n))
        If n % i == 0
            Return .F.
        EndIf
    Next i
Return .T.

Static Function Potencia(nBase, nExp)
    Local r := 1
    Local i := 0
    For i := 1 To nExp
        r := r * nBase
    Next i
Return r

Static Function SomaDigitos(n)
    Local s := 0
    n := Abs(n)
    Do While n > 0
        s := s + (n % 10)
        n := Int(n / 10)
    EndDo
Return s

// ---------------------------------------------------------------- Strings

Static Function Reverso(c)
    Local r := ""
    Local i := 0
    For i := Len(c) To 1 Step -1
        r := r + SubStr(c, i, 1)
    Next i
Return r

Static Function Palindromo(c)
Return c == Reverso(c)

Static Function ContaVogais(c)
    Local n := 0
    Local i := 0
    Local ch := ""
    For i := 1 To Len(c)
        ch := Lower(SubStr(c, i, 1))
        If ch $ "aeiou"
            n++
        EndIf
    Next i
Return n

Static Function Anagrama(a, b)
    Local aA := StrParaCodigos(Lower(a))
    Local aB := StrParaCodigos(Lower(b))
    If Len(aA) != Len(aB)
        Return .F.
    EndIf
    aSort(aA)
    aSort(aB)
Return ArrIgual(aA, aB)

// Array de codigos (numericos) dos chars — aSort ordena numerico de forma confiavel.
Static Function StrParaCodigos(c)
    Local a := {}
    Local i := 0
    For i := 1 To Len(c)
        aAdd(a, Asc(SubStr(c, i, 1)))
    Next i
Return a

Static Function ContaPalavras(c)
    Local n := 0
    Local i := 0
    Local lDentro := .F.
    Local ch := ""
    For i := 1 To Len(c)
        ch := SubStr(c, i, 1)
        If ch == " "
            lDentro := .F.
        ElseIf !lDentro
            lDentro := .T.
            n++
        EndIf
    Next i
Return n

// ---------------------------------------------------------------- Busca / ordenacao

Static Function BuscaLinear(a, x)
    Local i := 0
    For i := 1 To Len(a)
        If a[i] == x
            Return i
        EndIf
    Next i
Return 0

Static Function BuscaBinaria(a, x)
    Local lo := 1
    Local hi := Len(a)
    Local mid := 0
    Do While lo <= hi
        mid := Int((lo + hi) / 2)
        If a[mid] == x
            Return mid
        ElseIf a[mid] < x
            lo := mid + 1
        Else
            hi := mid - 1
        EndIf
    EndDo
Return 0

Static Function BolhaSort(aOrig)
    Local a := aClone(aOrig)
    Local i := 0
    Local j := 0
    Local t := 0
    For i := 1 To Len(a) - 1
        For j := 1 To Len(a) - i
            If a[j] > a[j + 1]
                t := a[j]
                a[j] := a[j + 1]
                a[j + 1] := t
            EndIf
        Next j
    Next i
Return a

Static Function InsercaoSort(aOrig)
    Local a := aClone(aOrig)
    Local i := 0
    Local j := 0
    Local chave := 0
    For i := 2 To Len(a)
        chave := a[i]
        j := i - 1
        Do While j >= 1 .And. a[j] > chave
            a[j + 1] := a[j]
            j--
        EndDo
        a[j + 1] := chave
    Next i
Return a

Static Function QuickSort(aOrig)
    Local a := aClone(aOrig)
    QSort(a, 1, Len(a))
Return a

Static Function QSort(a, lo, hi)
    Local p := 0
    If lo < hi
        p := Particiona(a, lo, hi)
        QSort(a, lo, p - 1)
        QSort(a, p + 1, hi)
    EndIf
Return

Static Function Particiona(a, lo, hi)
    Local piv := a[hi]
    Local i := lo - 1
    Local j := 0
    Local t := 0
    For j := lo To hi - 1
        If a[j] <= piv
            i++
            t := a[i]
            a[i] := a[j]
            a[j] := t
        EndIf
    Next j
    t := a[i + 1]
    a[i + 1] := a[hi]
    a[hi] := t
Return i + 1

// ---------------------------------------------------------------- Agregacao / DP / leetcode

Static Function Maximo(a)
    Local m := a[1]
    Local i := 0
    For i := 2 To Len(a)
        If a[i] > m
            m := a[i]
        EndIf
    Next i
Return m

Static Function SomaArray(a)
    Local s := 0
    Local i := 0
    For i := 1 To Len(a)
        s := s + a[i]
    Next i
Return s

// Kadane: maior soma de subarray contiguo.
Static Function Kadane(a)
    Local best := a[1]
    Local cur := a[1]
    Local i := 0
    For i := 2 To Len(a)
        If cur + a[i] > a[i]
            cur := cur + a[i]
        Else
            cur := a[i]
        EndIf
        If cur > best
            best := cur
        EndIf
    Next i
Return best

// Two Sum: indices (1-based) de dois numeros que somam alvo.
Static Function DoisSoma(a, alvo)
    Local i := 0
    Local j := 0
    For i := 1 To Len(a) - 1
        For j := i + 1 To Len(a)
            If a[i] + a[j] == alvo
                Return {i, j}
            EndIf
        Next j
    Next i
Return {0, 0}

Static Function FizzBuzz(n)
    If n % 15 == 0
        Return "FizzBuzz"
    ElseIf n % 3 == 0
        Return "Fizz"
    ElseIf n % 5 == 0
        Return "Buzz"
    EndIf
Return Str(n, 10)

// Valida parenteses balanceados.
Static Function ParentesesOk(c)
    Local nAbre := 0
    Local i := 0
    Local ch := ""
    For i := 1 To Len(c)
        ch := SubStr(c, i, 1)
        If ch == "("
            nAbre++
        ElseIf ch == ")"
            nAbre--
            If nAbre < 0
                Return .F.
            EndIf
        EndIf
    Next i
Return nAbre == 0

// Troca de moedas: minimo de moedas para formar o valor (DP).
Static Function TrocaMoedas(aMoedas, nAlvo)
    Local aDp := Array(nAlvo + 1)
    Local i := 0
    Local j := 0
    Local nInf := nAlvo + 1
    aDp[1] := 0
    For i := 1 To nAlvo
        aDp[i + 1] := nInf
    Next i
    For i := 1 To nAlvo
        For j := 1 To Len(aMoedas)
            If aMoedas[j] <= i
                If aDp[i - aMoedas[j] + 1] + 1 < aDp[i + 1]
                    aDp[i + 1] := aDp[i - aMoedas[j] + 1] + 1
                EndIf
            EndIf
        Next j
    Next i
    If aDp[nAlvo + 1] == nInf
        Return -1
    EndIf
Return aDp[nAlvo + 1]

// Maior subsequencia comum (comprimento) via DP.
Static Function LCS(a, b)
    Local nA := Len(a)
    Local nB := Len(b)
    Local aDp := {}
    Local i := 0
    Local j := 0
    Local aLin := {}
    For i := 1 To nA + 1
        aLin := {}
        For j := 1 To nB + 1
            aAdd(aLin, 0)
        Next j
        aAdd(aDp, aLin)
    Next i
    For i := 1 To nA
        For j := 1 To nB
            If SubStr(a, i, 1) == SubStr(b, j, 1)
                aDp[i + 1][j + 1] := aDp[i][j] + 1
            ElseIf aDp[i][j + 1] > aDp[i + 1][j]
                aDp[i + 1][j + 1] := aDp[i][j + 1]
            Else
                aDp[i + 1][j + 1] := aDp[i + 1][j]
            EndIf
        Next j
    Next i
Return aDp[nA + 1][nB + 1]
