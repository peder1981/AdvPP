// entrypoint_main_test.prw — comentário de cabeçalho ANTES do #include:
// convenção real (GesCon) que expôs dois bugs juntos — o comentário
// confundia a detecção de "onde começa o conteúdo do arquivo raiz", o que
// fazia a função da lib virar o ponto de entrada e crashar ao comparar um
// parâmetro nunca passado contra Nil.
#include "entrypoint_lib_test.prw"

User Function EpMain()
   ConOut("arquivo raiz chamado - CERTO")
Return
