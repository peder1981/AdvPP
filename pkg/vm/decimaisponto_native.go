package vm

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// decState é o estado Go de um "decimal de ponto fixo" (TDN: "Decimais de
// Ponto Fixo"), um tipo distinto do Numeric comum de AdvPL (que neste VM é
// float64 puro, ver pkg/runtime/values.go NumberValue). Ao contrário do
// Numeric, o decimal de ponto fixo carrega precisão (quantidade total de
// dígitos) e escala (quantidade de casas decimais) explícitas e é
// representado aqui com aritmética racional exata (math/big.Rat), não
// float64 — usar float64 reintroduziria exatamente o erro de arredondamento
// de ponto flutuante que este tipo existe para evitar (ver observação do
// DEC_CREATE sobre constantes numéricas sofrerem desvio na 15ª casa decimal
// quando não passadas como string).
//
// Representado como *advplrt.ObjectValue{ClassName: "DECIMAL", Native:
// *decState}, seguindo o mesmo padrão do tHashMap (Task 19,
// matrizhashmap_native.go): funções despachadas como natives comuns
// (DEC_ADD(dec1, dec2), não dec1:DEC_ADD(dec2)), confirmado pela sintaxe de
// todos os exemplos TDN desta categoria.
type decState struct {
	val   *big.Rat // valor exato, sempre quantizado a `scale` casas decimais
	prec  int       // precisão: quantidade total de dígitos (1..63)
	scale int       // escala: quantidade de casas decimais (0..prec-1)
}

// decClampPrecScale aplica limites defensivos de segurança (evita pânico em
// construção de string / expoente negativo de big.Int) quando precisão ou
// escala vierem fora da faixa documentada. A TDN documenta faixas válidas
// (0 < precisão < 64; 0 <= escala < precisão) mas nem toda função declara
// exceção para violação (ex.: DEC_CREATE explicitamente tolera estouro de
// precisão do VALOR, apenas emitindo aviso em builds novas) — por isso
// aqui apenas fixamos os limites estruturais mínimos para não quebrar o VM,
// sem replicar o aviso de console (fora do escopo observável em testes).
func decClampPrecScale(prec, scale int) (int, int) {
	if prec < 1 {
		prec = 1
	}
	if prec > 63 {
		prec = 63
	}
	if scale < 0 {
		scale = 0
	}
	if scale > prec-1 {
		scale = prec - 1
	}
	return prec, scale
}

// decRoundRat arredonda r para `scale` casas decimais segundo o modo
// documentado por DEC_RESCALE/DEC_RESIZE (nRound): 0 = arredonda 5 para
// cima (metade se afasta de zero), 1 = arredonda 5 para baixo (metade se
// aproxima de zero), 2 = trunca (sempre aproxima de zero, ignorando resto).
func decRoundRat(r *big.Rat, scale int, mode int) *big.Rat {
	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(pow))
	num := new(big.Int).Set(scaled.Num())
	den := new(big.Int).Set(scaled.Denom())
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int)) // trunca em direção a zero (T-division)

	if rem.Sign() != 0 && mode != 2 {
		remAbs := new(big.Int).Abs(rem)
		denAbs := new(big.Int).Abs(den)
		twiceRem := new(big.Int).Mul(remAbs, big.NewInt(2))
		cmp := twiceRem.Cmp(denAbs) // compara 2*|resto| com |denominador|

		roundAway := false
		if mode == 1 { // metade para baixo (para zero): só afasta se > , empate fica
			roundAway = cmp > 0
		} else { // mode 0 (padrão): metade para cima (afasta de zero, empate inclusive)
			roundAway = cmp >= 0
		}
		if roundAway {
			if q.Sign() < 0 || (q.Sign() == 0 && r.Sign() < 0) {
				q.Sub(q, big.NewInt(1))
			} else {
				q.Add(q, big.NewInt(1))
			}
		}
	}
	return new(big.Rat).SetFrac(q, pow)
}

// newDecState cria um decState já quantizado à escala informada (arredonda
// pelo modo padrão "metade para cima"). Precisão/escala já devem estar
// dentro dos limites estruturais (ver decClampPrecScale).
func newDecState(val *big.Rat, prec, scale int) *decState {
	prec, scale = decClampPrecScale(prec, scale)
	return &decState{val: decRoundRat(val, scale, 0), prec: prec, scale: scale}
}

// decParseValue interpreta xValue (caractere ou numérico, conforme
// DEC_CREATE) como big.Rat. Se for caractere inválido como decimal, o valor
// inicial é 0 (documentado). Retorna erro se o tipo não for caractere nem
// numérico.
func decParseValue(v advplrt.Value) (*big.Rat, error) {
	switch t := v.(type) {
	case *advplrt.StringValue:
		s := strings.Trim(t.Val, " ")
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			return big.NewRat(0, 1), nil
		}
		return r, nil
	case *advplrt.NumberValue:
		r := new(big.Rat)
		// Preserva o valor float64 exato (bit-a-bit), incluindo o desvio de
		// ponto flutuante documentado pela TDN para constantes numéricas
		// não citadas entre aspas — não é bug, é o comportamento nativo.
		if r.SetFloat64(t.Val) == nil {
			return big.NewRat(0, 1), nil
		}
		return r, nil
	default:
		return nil, fmt.Errorf("DEC_CREATE: xValue deve ser caractere ou numérico")
	}
}

// decGetState extrai o *decState de um valor decimal de ponto fixo (objeto
// ObjectValue com ClassName "DECIMAL"). ok=false se v não for um decimal.
func decGetState(v advplrt.Value) (*decState, bool) {
	obj, ok := v.(*advplrt.ObjectValue)
	if !ok {
		return nil, false
	}
	state, ok := obj.Native.(*decState)
	if !ok {
		return nil, false
	}
	return state, true
}

// decNewObject encapsula um decState num ObjectValue "DECIMAL".
func decNewObject(state *decState) *advplrt.ObjectValue {
	obj := advplrt.NewObject("DECIMAL", nil)
	obj.Native = state
	return obj
}

// decFormat produz a representação decimal em string com exatamente
// `scale` casas decimais (padded com zeros à direita quando necessário),
// usada pelos testes para validar o resultado das operações.
func decFormat(d *decState) string {
	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d.scale)), nil)
	scaled := decRoundRat(d.val, d.scale, 0) // já deveria estar quantizado; reforça segurança
	num := new(big.Rat).Mul(scaled, new(big.Rat).SetInt(pow))
	intPart := new(big.Int).Set(num.Num()) // num já é inteiro (Denom()==1) pois scaled tem escala `scale`
	neg := intPart.Sign() < 0
	intPart.Abs(intPart)
	s := intPart.String()
	if d.scale > 0 {
		for len(s) <= d.scale {
			s = "0" + s
		}
		s = s[:len(s)-d.scale] + "." + s[len(s)-d.scale:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// decIntDigits estima a quantidade de dígitos da parte inteira de um
// decimal com precisão p e escala s (usado no cálculo de precisão/escala
// resultante das operações aritméticas).
func decIntDigits(p, s int) int {
	d := p - s
	if d < 1 {
		d = 1
	}
	return d
}

// decAddSubPrecScale calcula a precisão/escala do resultado de DEC_ADD,
// DEC_SUB e DEC_MOD.
//
// NOTA IMPORTANTE: a subpágina "Cálculo de Precisão e Escala em Operações"
// linkada por todas as funções desta categoria é, verificadamente, um stub
// genuíno do TDN (H1 presente, corpo vazio — só boilerplate/metadados,
// confirmado por leitura direta do mirror). Não existe especificação
// oficial disponível para consultar. As regras abaixo são uma convenção
// própria, deliberadamente modelada nas regras padrão de aritmética DECIMAL
// do SQL Server/T-SQL (fonte amplamente documentada e testada para o mesmo
// problema — soma de precisão/escala com "headroom" para carry, produto de
// precisões para multiplicação, fórmula clássica de escala estendida para
// divisão) por ser o precedente mais próximo e defensável de um sistema
// real de "fixed-point decimal" com semântica equivalente. Documentado
// explicitamente aqui e no relatório da task — não é comportamento
// confirmado pela TDN.
func decAddSubPrecScale(p1, s1, p2, s2 int) (int, int) {
	intDigits := decIntDigits(p1, s1)
	if d2 := decIntDigits(p2, s2); d2 > intDigits {
		intDigits = d2
	}
	scale := s1
	if s2 > scale {
		scale = s2
	}
	prec := intDigits + scale + 1 // +1 de headroom para eventual carry (ex.: 9+1=10)
	return decClampPrecScale(prec, scale)
}

// decMulPrecScale calcula a precisão/escala do resultado de DEC_MUL
// (convenção própria — ver nota em decAddSubPrecScale).
func decMulPrecScale(p1, s1, p2, s2 int) (int, int) {
	scale := s1 + s2
	prec := p1 + p2 + 1
	return decClampPrecScale(prec, scale)
}

// decDivPrecScale calcula a precisão/escala do resultado de DEC_DIV
// (convenção própria, modelada na fórmula clássica de divisão DECIMAL do
// T-SQL — ver nota em decAddSubPrecScale).
func decDivPrecScale(p1, s1, p2, s2 int) (int, int) {
	scale := s1 + p2 + 1
	if scale < 6 {
		scale = 6
	}
	prec := decIntDigits(p1, s1) + s2 + scale
	return decClampPrecScale(prec, scale)
}

// decModPrecScale calcula a precisão/escala do resultado de DEC_MOD: o
// resto de uma divisão é sempre menor (em módulo) que o divisor, então a
// parte inteira do resultado é limitada pela parte inteira de dRight
// (convenção própria — ver nota em decAddSubPrecScale). p1 (precisão do
// dividendo) é intencionalmente ignorado: a magnitude do resto depende só
// do divisor, não da precisão de dLeft.
func decModPrecScale(p1, s1, p2, s2 int) (int, int) {
	_ = p1
	scale := s1
	if s2 > scale {
		scale = s2
	}
	prec := decIntDigits(p2, s2) + scale
	return decClampPrecScale(prec, scale)
}

// decRatIsInt reporta se r representa um número inteiro exato.
func decRatIsInt(r *big.Rat) bool {
	return r.IsInt()
}

// decPow calcula dLeft ^ dRight. Quando o expoente é um inteiro exato
// (caso comum e único documentado pelo exemplo TDN, DEC_POW(3,2)=9),
// calcula por multiplicação racional repetida, preservando exatidão total
// (sem passar por float64). Quando o expoente não é inteiro — caso que a
// TDN não documenta com exemplo — cai para math.Pow em float64 como
// aproximação best-effort (limitação documentada: perde a garantia de
// exatidão de ponto fixo nesse caso específico, mas não há especificação
// TDN de como um decimal de ponto fixo deveria representar um expoente
// fracionário exatamente).
func decPow(base *big.Rat, exp *big.Rat) *big.Rat {
	if decRatIsInt(exp) {
		n := new(big.Int).Quo(exp.Num(), exp.Denom())
		neg := n.Sign() < 0
		if neg {
			n.Neg(n)
		}
		result := big.NewRat(1, 1)
		bi := new(big.Rat).Set(base)
		e := new(big.Int).Set(n)
		one := big.NewInt(1)
		for e.Sign() > 0 {
			result.Mul(result, bi)
			e.Sub(e, one)
		}
		if neg {
			if result.Sign() == 0 {
				return big.NewRat(0, 1)
			}
			result = new(big.Rat).Inv(result)
		}
		return result
	}
	bf, _ := base.Float64()
	ef, _ := exp.Float64()
	rf := math.Pow(bf, ef)
	out := new(big.Rat)
	if out.SetFloat64(rf) == nil {
		return big.NewRat(0, 1)
	}
	return out
}

// registerDecimaisdePontoFixoNatives registra as funções da categoria TDN
// "Decimais de Ponto Fixo": DEC_ADD, DEC_CREATE, DEC_DIV, DEC_MOD, DEC_MUL,
// DEC_POW, DEC_RESCALE, DEC_RESIZE, DEC_ROUND, DEC_SUB.
//
// DEC_TO_DBL NÃO é implementada: confirmada em docs/tdn-gap-stubs.md como
// stub sem spec real na fonte TDN espelhada — pulada por instrução
// explícita da task, não por omissão.
func (v *VM) registerDecimaisdePontoFixoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// DEC_CREATE( < xValue >, < iPrecision >, < iScale > ) -> dRet
	natives["DEC_CREATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		val, err := decParseValue(getArg(args, 0))
		if err != nil {
			return advplrt.Nil, err
		}
		prec := int(math.Trunc(advplrt.ToFloat(getArg(args, 1))))
		scale := int(math.Trunc(advplrt.ToFloat(getArg(args, 2))))
		state := newDecState(val, prec, scale)
		return decNewObject(state), nil
	}

	// DEC_ADD( < dLeft >, < dRight > ) -> dRet
	natives["DEC_ADD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_ADD: dLeft e dRight devem ser decimais de ponto fixo")
		}
		sum := new(big.Rat).Add(l.val, r.val)
		prec, scale := decAddSubPrecScale(l.prec, l.scale, r.prec, r.scale)
		return decNewObject(newDecState(sum, prec, scale)), nil
	}

	// DEC_SUB( < dLeft >, < dRight > ) -> dRet
	natives["DEC_SUB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_SUB: dLeft e dRight devem ser decimais de ponto fixo")
		}
		diff := new(big.Rat).Sub(l.val, r.val)
		prec, scale := decAddSubPrecScale(l.prec, l.scale, r.prec, r.scale)
		return decNewObject(newDecState(diff, prec, scale)), nil
	}

	// DEC_MUL( < dLeft >, < dRight > ) -> dRet
	natives["DEC_MUL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_MUL: dLeft e dRight devem ser decimais de ponto fixo")
		}
		prod := new(big.Rat).Mul(l.val, r.val)
		prec, scale := decMulPrecScale(l.prec, l.scale, r.prec, r.scale)
		return decNewObject(newDecState(prod, prec, scale)), nil
	}

	// DEC_DIV( < dLeft >, < dRight > ) -> dRet
	natives["DEC_DIV"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_DIV: dLeft e dRight devem ser decimais de ponto fixo")
		}
		if r.val.Sign() == 0 {
			return advplrt.Nil, fmt.Errorf("DEC_DIV: divisão por zero")
		}
		quo := new(big.Rat).Quo(l.val, r.val)
		prec, scale := decDivPrecScale(l.prec, l.scale, r.prec, r.scale)
		return decNewObject(newDecState(quo, prec, scale)), nil
	}

	// DEC_MOD( < dLeft >, < dRight > ) -> dRet
	natives["DEC_MOD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_MOD: dLeft e dRight devem ser decimais de ponto fixo")
		}
		if r.val.Sign() == 0 {
			return advplrt.Nil, fmt.Errorf("DEC_MOD: divisão por zero")
		}
		// Resto de divisão truncada (como fmod): dLeft - trunc(dLeft/dRight)*dRight
		q := new(big.Rat).Quo(l.val, r.val)
		qInt := new(big.Int).Quo(q.Num(), q.Denom()) // trunca em direção a zero
		qRat := new(big.Rat).SetInt(qInt)
		mod := new(big.Rat).Sub(l.val, new(big.Rat).Mul(qRat, r.val))
		prec, scale := decModPrecScale(l.prec, l.scale, r.prec, r.scale)
		return decNewObject(newDecState(mod, prec, scale)), nil
	}

	// DEC_POW( < dLeft >, < dRight > ) -> dRet
	natives["DEC_POW"] = func(args []advplrt.Value) (advplrt.Value, error) {
		l, ok1 := decGetState(getArg(args, 0))
		r, ok2 := decGetState(getArg(args, 1))
		if !ok1 || !ok2 {
			return advplrt.Nil, fmt.Errorf("DEC_POW: dLeft e dRight devem ser decimais de ponto fixo")
		}
		pow := decPow(l.val, r.val)
		// Convenção própria (sem regra TDN documentada p/ potenciação):
		// mantém a precisão/escala do dLeft (base), replicando o exemplo
		// da TDN (base e expoente com mesma p/s, resultado dentro da
		// mesma escala do exemplo).
		return decNewObject(newDecState(pow, l.prec, l.scale)), nil
	}

	// DEC_RESCALE( < dNum >, < nScale >, [ nRound ] ) -> dRet
	natives["DEC_RESCALE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		d, ok := decGetState(getArg(args, 0))
		if !ok {
			return advplrt.Nil, fmt.Errorf("DEC_RESCALE: dNum deve ser um decimal de ponto fixo")
		}
		newScale := int(math.Trunc(advplrt.ToFloat(getArg(args, 1))))
		mode := 0
		if len(args) > 2 && args[2] != nil {
			mode = int(math.Trunc(advplrt.ToFloat(args[2])))
		}
		// A precisão total (quantidade de dígitos) não muda em RESCALE,
		// apenas a escala e o valor arredondado a ela — só a escala está
		// documentada como parâmetro alterável.
		prec := d.prec
		// Documentado literalmente em DEC_RESCALE.md: "Caso <dNum> não
		// seja do tipo decimal, ou <nScale> seja menor que 0 ou maior o
		// igual à precisão do número, ou <nRound> seja menor que 0 ou
		// maior que 2, uma exceção será lançada" — mesmo contrato de
		// DEC_RESIZE, não apenas clamping defensivo (correção pós-review:
		// a primeira versão desta native clampava nScale silenciosamente
		// em vez de lançar erro, por leitura incorreta do documento).
		if newScale < 0 || newScale >= prec {
			return advplrt.Nil, fmt.Errorf("DEC_RESCALE: nScale deve ser maior ou igual a zero e menor que a precisão de dNum")
		}
		if mode < 0 || mode > 2 {
			return advplrt.Nil, fmt.Errorf("DEC_RESCALE: nRound deve ser 0, 1 ou 2")
		}
		rounded := decRoundRat(d.val, newScale, mode)
		return decNewObject(&decState{val: rounded, prec: prec, scale: newScale}), nil
	}

	// DEC_RESIZE( < dNum >, < nPrecision >, < nScale >, [ nRound ] ) -> dRet
	natives["DEC_RESIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		d, ok := decGetState(getArg(args, 0))
		if !ok {
			return advplrt.Nil, fmt.Errorf("DEC_RESIZE: dNum deve ser um decimal de ponto fixo")
		}
		newPrec := int(math.Trunc(advplrt.ToFloat(getArg(args, 1))))
		newScale := int(math.Trunc(advplrt.ToFloat(getArg(args, 2))))
		mode := 2 // padrão documentado: truncate
		if len(args) > 3 && args[3] != nil {
			mode = int(math.Trunc(advplrt.ToFloat(args[3])))
		}
		if newPrec <= 0 || newPrec >= 64 {
			return advplrt.Nil, fmt.Errorf("DEC_RESIZE: nPrecision deve ser maior que zero e menor que 64")
		}
		if newScale < 0 || newScale >= newPrec {
			return advplrt.Nil, fmt.Errorf("DEC_RESIZE: nScale deve ser maior ou igual a zero e menor que nPrecision")
		}
		if mode < 0 || mode > 2 {
			return advplrt.Nil, fmt.Errorf("DEC_RESIZE: nRound deve ser 0, 1 ou 2")
		}
		rounded := decRoundRat(d.val, newScale, mode)
		return decNewObject(&decState{val: rounded, prec: newPrec, scale: newScale}), nil
	}

	// DEC_ROUND( < dNum >, < nRound > ) -> dRet
	natives["DEC_ROUND"] = func(args []advplrt.Value) (advplrt.Value, error) {
		d, ok := decGetState(getArg(args, 0))
		if !ok {
			return advplrt.Nil, fmt.Errorf("DEC_ROUND: dNum deve ser um decimal de ponto fixo")
		}
		nRound := int(math.Trunc(advplrt.ToFloat(getArg(args, 1))))
		if nRound < 0 || nRound >= d.scale {
			return advplrt.Nil, fmt.Errorf("DEC_ROUND: nRound deve ser >= 0 e menor que a escala de dNum")
		}
		// Arredonda para nRound casas (sempre "metade para cima", modo 0)
		// mas preserva a precisão/escala ORIGINAIS de dNum — apenas o
		// valor interno é ajustado (zeros à direita a partir de nRound),
		// exatamente como no exemplo TDN: dec1 tem escala 3 (5.759),
		// DEC_ROUND(dec1,1) = 5.800 (ainda escala 3, agora com zeros).
		rounded := decRoundRat(d.val, nRound, 0)
		return decNewObject(&decState{val: rounded, prec: d.prec, scale: d.scale}), nil
	}
}
