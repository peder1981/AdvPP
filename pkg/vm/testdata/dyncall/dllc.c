#include <stdlib.h>
#include <string.h>

double Add(double x, double y) {
    return x + y;
}
int add(int a, int b) {
    return a + b;
}
int SumInt(int a, int b) {
    return a + b;
}
void PrintData(short a, unsigned short b, float c, unsigned long long d, const char *e) {
    (void)a; (void)b; (void)c; (void)d; (void)e;
}
double gGlobal = 3.5;

// Espelha o exemplo "lado da biblioteca" de TDN DynCall - NewPointer/
// StrLen/StrCpy/MemCpy: retorna um buffer alocado no heap C, terminado
// em '\0', que o lado TLPP consulta via um ponteiro opaco.
void *getPtr(void) {
    char *p = (char *)malloc(64);
    strcpy(p, "Dyncall Test");
    return p;
}
