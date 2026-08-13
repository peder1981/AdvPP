// EXPORT: no Unix, __attribute__((visibility("default"))) na classe
// inteira garante que símbolos JÁ EMITIDOS (C1/C2 de construtor incluso)
// fiquem na tabela de símbolos exportados do .so/.dylib — achado real via
// CI macOS, onde a Itanium C++ ABI da Apple deixava o construtor de fora
// mesmo emitido. No Windows/MinGW, EXPORT fica vazio de propósito: uma
// vez que QUALQUER símbolo no módulo usa __declspec(dllexport), o linker
// do MinGW muda de "exporta tudo por padrão" para "exporta só o marcado
// explicitamente" — adicionar dllexport aqui quebrou até factory()
// (achado real via CI Windows, regressão introduzida numa tentativa
// anterior deste mesmo fix). Sem nenhuma anotação, o MinGW já exporta
// tudo, que é exatamente o comportamento que já funcionava lá.
//
// KEEP (__attribute__((used))) resolve um problema diferente do EXPORT:
// força EMISSÃO de código para um método sem uso aparente na própria TU
// (o construtor só é chamado indiretamente, dentro de factory(), que o
// compilador pode inlinar) — sem isso, não existe symbol nenhum para
// visibility("default") tornar exportado. Suportado por GCC/Clang em
// Linux, macOS e MinGW por igual, sem necessidade de ifdef.
#if defined(_WIN32)
#define EXPORT
#else
#define EXPORT __attribute__((visibility("default")))
#endif
#define KEEP __attribute__((used))

class EXPORT tArith {
public:
    KEEP tArith() {}
    KEEP double Add(double x, double y) { return x + y; }
    KEEP static tArith *factory() { return new tArith(); }
};
