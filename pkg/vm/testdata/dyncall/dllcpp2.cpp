// Espelha exatamente o exemplo "lado da biblioteca" de TDN DynCall -
// CallMethod: declaração na classe, definição fora — por isso emitido
// sempre pelo compilador, sem precisar de __attribute__((used)).
class tArith {
public:
    tArith();
    tArith *factory();
    int add(int a, int b);
};

tArith::tArith() {}

tArith *tArith::factory() {
    tArith *p;
    p = new tArith();
    return p;
}

int tArith::add(int a, int b) {
    return a + b;
}
