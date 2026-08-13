// Raiz real do problema (confirmado via `nm` rodado no próprio runner
// macOS da CI, não hipótese): o construtor `tArith() {}` definido INLINE
// dentro do corpo da classe é implicitamente `inline` pela regra do C++
// (mergeable entre unidades de tradução / candidato a COMDAT) — mesmo com
// visibility("default") na classe e __attribute__((used)) no método, o
// `nm` mostrou o símbolo mangled presente no binário só como local
// ("t __ZN6tArithC1Ev"/"t __ZN6tArithC2Ev", minúsculo — não externamente
// visível a Dlsym), enquanto Add/factory (mesmas anotações) ficaram
// globais ("T", maiúsculo). Definição FORA da classe (como já é o padrão
// em dllcpp2.cpp, que nunca teve esse problema) tem linkage externa
// comum, sem ambiguidade de merge entre TUs — resolve na raiz, sem
// precisar de EXPORT/KEEP/-fvisibility.
class tArith {
public:
    tArith();
    double Add(double x, double y);
    static tArith *factory();
};

tArith::tArith() {}

double tArith::Add(double x, double y) { return x + y; }

tArith *tArith::factory() { return new tArith(); }
