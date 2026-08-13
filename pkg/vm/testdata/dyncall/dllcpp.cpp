#define KEEP __attribute__((used))

class tArith {
public:
    // KEEP no construtor: sem uso aparente fora de factory() (que o
    // compilador pode inlinar), o linker do macOS (dead-strip mais
    // agressivo que o ld do Linux) pode omitir o símbolo mangled
    // _ZN6tArithC1Ev do .dylib final — achado real via CI macOS, onde
    // TestTRunDllNewObjComTamanhoAlocaEChamaConstrutor (que chama o
    // construtor via CallMethod explicitamente) falhava por Dlsym não
    // encontrar o símbolo, mesmo com o mesmo fonte funcionando no Linux.
    KEEP tArith() {}
    KEEP double Add(double x, double y) { return x + y; }
    KEEP static tArith *factory() { return new tArith(); }
};
