// EXPORT espelha o "#define EXPORT __declspec(dllexport)" dos exemplos
// reais da TDN (DynCall - CallMethod / NewObj): controla se um símbolo
// JÁ EMITIDO fica visível na tabela de símbolos exportados do .dylib/.so
// (achado real via CI macOS: sem visibility("default") na classe, a
// Itanium C++ ABI da Apple pode deixar C1/C2 de fora mesmo emitidos).
//
// KEEP (__attribute__((used))) resolve um problema DIFERENTE: força o
// compilador a EMITIR CÓDIGO para um método sem uso aparente na própria
// TU (o construtor só é chamado indiretamente, dentro de factory(), que
// o compilador pode inlinar) — sem KEEP, nem chega a existir símbolo pra
// visibility("default") tornar exportado. As duas anotações resolvem
// problemas de estágios diferentes (emissão vs. exportação) e são
// necessárias juntas — confirmado: EXPORT sozinho quebrou até no Linux
// (constructor deixou de ser emitido), KEEP sozinho quebrou só no macOS
// (emitido mas não exportado).
#if defined(_WIN32)
#define EXPORT __declspec(dllexport)
#define KEEP
#else
#define EXPORT __attribute__((visibility("default")))
#define KEEP __attribute__((used))
#endif

class EXPORT tArith {
public:
    KEEP tArith() {}
    KEEP double Add(double x, double y) { return x + y; }
    KEEP static tArith *factory() { return new tArith(); }
};
