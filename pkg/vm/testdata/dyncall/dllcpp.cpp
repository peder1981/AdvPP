#define KEEP __attribute__((used))

class tArith {
public:
    tArith() {}
    KEEP double Add(double x, double y) { return x + y; }
    KEEP static tArith *factory() { return new tArith(); }
};
