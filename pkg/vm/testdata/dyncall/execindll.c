// Fixture para o wrapper de compatibilidade TDN (examples/dyncall/
// execindll_compat.prw): implementa a assinatura clássica documentada em
// ExecInDLLOpen/ExecInDLLRun/ExeDLLRun2 (TDN DynCall):
//   void ExecInClientDLL(int idCommand, char* buffParam, char* buffOutput, int buffLen)
// Devolve em buffOutput o eco "ECHO:<idCommand>:<buffParam>", truncado a
// buffLen-1 bytes + terminador nulo — o suficiente para provar que o
// wrapper abre a DLL, chama a função e lê o buffer de volta corretamente.
#include <string.h>
#include <stdio.h>

#if defined(_WIN32)
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

EXPORT void ExecInClientDLL(int idCommand, char *buffParam, char *buffOutput, int buffLen) {
    char tmp[512];
    snprintf(tmp, sizeof(tmp), "ECHO:%d:%s", idCommand, buffParam);
    strncpy(buffOutput, tmp, (size_t)buffLen - 1);
    buffOutput[buffLen - 1] = '\0';
}
