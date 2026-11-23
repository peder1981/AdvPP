package parser

import "testing"

// Regressões do corpus Protheus 12.1.2510: formas reais de WSRESTFUL e de
// sintaxe legada que o parser precisa aceitar sem alterar o fonte.

func TestWSRestfulImplClausulasOrdemArbitraria(t *testing.T) {
	src := `WSRESTFUL productionorders DESCRIPTION "x" FORMAT "application/json"
	WSMETHOD GET PEGGING WSRECEIVE branchId, productionOrder QUERYPARAM Page, PageSize WSSERVICE productionorders
	WSMETHOD POST RASTREIO PATHPARAM recordNumber, recStatus WSSERVICE productionorders
END WSRESTFUL

WSMETHOD GET PEGGING WSRECEIVE branchId, productionOrder QUERYPARAM Page, PageSize WSSERVICE productionorders
Return .T.

WSMETHOD POST RASTREIO PATHPARAM recordNumber, recStatus HEADERPARAM authorization WSSERVICE productionorders
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	if len(prog.Methods) != 2 {
		t.Fatalf("esperava 2 métodos, got %d", len(prog.Methods))
	}
	for _, m := range prog.Methods {
		if m.ClassName != "productionorders" {
			t.Errorf("método %s: ClassName=%q", m.Name, m.ClassName)
		}
	}
}

func TestWSRestfulImplNomeNumerico(t *testing.T) {
	src := `WSRESTFUL svc DESCRIPTION "x"
	WSMETHOD GET 0001 WSSERVICE svc
END WSRESTFUL

WSMETHOD GET 0001 WSSERVICE svc
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	if len(prog.Methods) != 1 || prog.Methods[0].Name != "0001" {
		t.Fatalf("nome numérico não preservado: %+v", prog.Methods)
	}
}

// `QUERYPARAM ..., format WSSERVICE x` — `format` é nome de parâmetro
// (vem depois de vírgula), não a cláusula FORMAT. Sem essa regra a
// palavra-cláusula vazava para o top-level e um parseWSClient espúrio
// sobrescrevia wsClassKind, quebrando todos os WSMETHOD seguintes.
func TestWSRestfulQueryParamColideComClausula(t *testing.T) {
	src := `WSRESTFUL mrpresults DESCRIPTION "x"
	WSMETHOD GET PARAM PATHPARAM branchId, ticket QUERYPARAM Fields, Order, Page, PageSize, format WSSERVICE mrpresults
	WSMETHOD GET SUMPROD PATHPARAM branchId QUERYPARAM Fields WSSERVICE mrpresults
END WSRESTFUL

WSMETHOD GET PARAM PATHPARAM branchId, ticket QUERYPARAM Fields, Order, Page, PageSize, format WSSERVICE mrpresults
Return .T.

WSMETHOD GET SUMPROD PATHPARAM branchId QUERYPARAM Fields WSSERVICE mrpresults
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	if len(prog.Methods) != 2 {
		t.Fatalf("esperava 2 métodos, got %d", len(prog.Methods))
	}
	for _, m := range prog.Methods {
		if m.ClassName != "mrpresults" {
			t.Errorf("método %s: ClassName=%q (vazou p/ top-level?)", m.Name, m.ClassName)
		}
	}
}

// Nome do serviço colidindo com keyword (`WSRESTFUL Data`, RHNP02).
func TestWSRestfulNomeKeyword(t *testing.T) {
	src := `WSRESTFUL Data DESCRIPTION "x" FORMAT "application/json"
	WSMETHOD GET list WSRECEIVE id WSSERVICE Data
END WSRESTFUL

WSMETHOD GET list WSRECEIVE id WSSERVICE Data
Return .T.
`
	prog, _ := parseWSRestfulSrc(t, src)
	if len(prog.Classes) != 1 || prog.Classes[0].Name != "Data" {
		t.Fatalf("classe WSRESTFUL não registrada: %+v", prog.Classes)
	}
	if len(prog.Methods) != 1 || prog.Methods[0].ClassName != "Data" {
		t.Fatalf("método não ligado ao serviço Data: %+v", prog.Methods)
	}
}

func TestArgumentosPontoVirgulaEspurio(t *testing.T) {
	src := `Function T(cAlias, cCampo)
	Local n := If(AT("_", cAlias) ; ,cCampo)
	Local m := f(1 ;)
	Local k := g(1 ; , 2)
Return
`
	parseWSRestfulSrc(t, src)
}

// Flags de GUI coladas como argumentos sem vírgula e penduradas após ')'
// (`bSETGET(cFiliais MEMO HSCROLL)`, `TSay():New(...)HSCROLL` — idioma
// das classes TSay/TGet/TGroup do corpus 12.1.2510).
func TestArgumentosFlagsGUISemVirgula(t *testing.T) {
	src := `Function T(cFiliais)
	bSETGET(cFiliais MEMO HSCROLL)
	TSay():New(1, "t", , , , 20 MEMO HSCROLL)
	TSay():New(1)HSCROLL
Return
`
	parseWSRestfulSrc(t, src)
}

// Alvos de Default além de variável simples: propriedade de self
// (`oWS:Page`, `Self:Page`, `::Page`) e elemento indexado.
func TestDefaultAlvosCompostos(t *testing.T) {
	src := `Function T()
	Default oWS:Page := 1
	Default Self:Page := 2
	Default ::Page := 3
	Default aArr[1] := 4
Return
`
	parseWSRestfulSrc(t, src)
}

// Nome de WSMETHOD iniciando com dígito (`WSMETHOD GET 1customers ...`)
// — o lexer parte em NUMBER+IDENT colados; CNTM300.prw do corpus
// 12.1.2510 tem uma família inteira desses métodos.
func TestWSMethodNomeIniciandoComDigito(t *testing.T) {
	src := `WSRESTFUL WSCNTA300 DESCRIPTION "x"
	WSDATA page AS INTEGER OPTIONAL
	WSMETHOD GET 1customers DESCRIPTION "d" PATH 'customers' TTALK 'V1' PRODUCES APPLICATION_JSON
	WSMETHOD GET 0001 DESCRIPTION "d" PATH 'x' TTALK 'V1'
END WSRESTFUL

WSMETHOD GET customers WSRECEIVE searchKey, page, unit WSREST WSCNTA300
Local lRet := .T.
Return( lRet )
`
	parseWSRestfulSrc(t, src)
}

// `json` como identificador AdvPL (colide com a keyword TLPP de tipo):
// `Local json as OBJECT`, `Default json := ...` e `json[campo]` —
// GPEA240API/GPEAADIAPI/TRMA010API do corpus 12.1.2510.
func TestJsonComoIdentificadorAdvPL(t *testing.T) {
	src := `Function T()
	Local json as OBJECT
	Default json := JsonObject():New()
	Local cParEmp
	cParEmp := json[aCampos[nItem]]
	cRecebe := json:FromJSON(cBody)
Return
`
	parseWSRestfulSrc(t, src)
}
