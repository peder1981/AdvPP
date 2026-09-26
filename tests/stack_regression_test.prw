// Regressão do vazamento de pilha em comando-expressão (FULL-REVIEW C1).
//
// Antes da correção, todo comando cujo valor é descartado — chamada de
// função isolada, x++, x-- — deixava um valor na pilha da VM a cada
// iteração. Passando de MaxStackSize (10.000) o push falhava em silêncio
// e o programa entrava em loop infinito. Cada laço abaixo passa desse
// limite; se algum travar, a regressão voltou.
#include "totvs.ch"

User Function Main()
   Local nI    := 0
   Local nC    := 0
   Local aA    := {}

   // 1) x++ como comando (o gatilho original)
   For nI := 1 To 20000
      nC++
   Next
   ConOut("incr nC=" + cValToChar(nC))

   // 2) chamada de função como comando
   nC := 0
   For nI := 1 To 20000
      nC := Incrementa(nC)
   Next
   ConOut("call nC=" + cValToChar(nC))

   // 3) chamada nativa como comando (aAdd)
   For nI := 1 To 20000
      aAdd(aA, nI)
   Next
   ConOut("aadd len=" + cValToChar(Len(aA)))

   // 4) x-- como comando
   For nI := 1 To 20000
      nC--
   Next
   ConOut("decr nC=" + cValToChar(nC))

   ConOut("stack-regression OK")
Return

Static Function Incrementa(n)
Return n + 1
