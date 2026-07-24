// entrypoint_lib_test.prw — lib auxiliar. Se este arquivo virar o ponto de
// entrada por engano (chamado sem argumentos), cValor fica sem valor: antes
// da correção, comparar esse slot a Nil (`cValor == Nil`, padrão comum pra
// parâmetro opcional) derrubava a VM com SIGSEGV em vez de avaliar .T./.F.
User Function EpLibHelper(cValor)
   If cValor == Nil
      ConOut("biblioteca chamada - ERRADO")
   EndIf
Return
