package vm

// Funções DB DBAccess (TC*) — TDN: Functions/Banco-de-Dados/Funcoes-DBAccess.
//
// As funções TC* representam o acesso SQL do Protheus através do driver
// DBAccess (TOPCONN). Nesta VM elas são mapeadas sobre o SQLiteEngine
// (pkg/db) aberto por conexão: TCLink cria um SQLiteEngine apontando para um
// arquivo .db em temp dir e guarda o handle em um estado GLOBAL do pacote
// (`dbstate`), seguindo o modelo "conexão ativa" do DBAccess real
// (TCGetConn/TCSetConn/TCIsConnected/TCUnlink/TCQuit).
//
// O arquivo registra as natives via `registerDbaccessNatives` — NÃO toca em
// natives.go; a registração é feita manualmente (e nos testes, com o mesmo
// mapa de natives passado explicitamente).
//
// Limitações honestas documentadas:
//   - SQLite não tem Stored Procedures: TCSPExist devolve .F. e TCSPExec
//     devolve NIL, registrando o motivo em TCSQLError().
//   - SQLite não tem pool ODBC nem threads de DBAccess: TCGetIO/TCPoolInfo/
//     TCDrivers/TCGetInfo devolvem valores neutros coerentes.
//   - Parâmetros de string por referência (@cMsg, @cError, @cOut, @cTable,
//     @cStruct, @cType, @nErro) não podem ser escritos de volta pela VM
//     (natives recebem cópias de valores); arrays passados por referência
//     (aResult) são mutáveis e são preenchidos.
//   - MSParse/MSParseFull implementam um parser simples e honesto que valida
//     o dialeto de destino e normaliza o texto SQL (sem conversão real de
//     dialeto — SQLite aqui é o SGBD de facto).
//   - TCGenQry/TCGenQry2 devolvem o resultado da query como ARRAY AdvPL
//     (linha = array de valores) — desvio da spec (retorno ""), seguindo a
//     instrução de mapeamento do agente (a abertura via DBUseArea não é
//     exercitada nesta VM).

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Versões simuladas do DBAccess/DBAPI — no AdvPP não existe servidor
// DBAccess; devolvemos builds estáveis documentadas para os getters.
const (
	dbaccessServerBuild = "24.3.1.0"
	dbaccessServerDate  = "20260101"
	dbaccessAPIVersion  = "18.2.1.3"
	dbaccessAPIBuild    = "20260101-20260101"
)

// dbaccessSessionID evita que duas execuções reusem o mesmo arquivo .db de
// conexão em temp dir (poluição de estado entre testes/processos).
var dbaccessSessionID = fmt.Sprintf("%x", time.Now().UnixNano())

// dbViewMeta guarda a definição das views criadas pelas funções TCView*
// (tabela master + estrutura), permitindo TCHasView/TCViewStruct sem reler
// o catálogo do SGBD.
type dbViewMeta struct {
	name      string
	master    string
	structStr string
	one2one   bool
}

// dbstateConn é uma conexão DBAccess simulada: um SQLiteEngine aberto em
// temp dir + os metadados da conexão (driver, sid, pool, views).
type dbstateConn struct {
	id       int
	connStr  string
	server   string
	port     int
	driver   string
	dbsid    string
	sid      int
	engine   DBEngine
	sqlEng   SQLEngine
	dbPath   string
	views    map[string]*dbViewMeta
	inPool   bool
	poolName string
	poolTime time.Time
	closed   bool
}

// dbaccessFieldType registra o tratamento de tipo pedido via TCSetField
// para uma coluna de uma query (análogo ao DBAccess real).
type dbaccessFieldType struct {
	cType     string
	size      int
	precision int
}

// dbaccessColDef é uma coluna de estrutura AdvPL {nome, tipo, tamanho, dec}.
type dbaccessColDef struct {
	name  string
	ctype string
	size  int
	dec   int
}

// dbAccessState é o estado GLOBAL das conexões DBAccess do pacote.
type dbAccessState struct {
	mu         sync.Mutex
	conns      map[int]*dbstateConn
	active     int                        // conexão ativa (TCGetConn/TCSetConn)
	nextID     int
	pools      map[string][]int           // nome do pool -> ids de conexão
	maxMap     int                        // TCMaxMap (mínimo de colunas p/ TCSrvMap)
	sqlErr     string                     // TCSQLError
	mspErr     string                     // MSParseError
	config     map[string]string          // TCConfig (chave -> valor)
	params     map[string]string          // TOP_PARAM equivalente (TCSetParam)
	fieldTypes map[string]map[string]dbaccessFieldType // TCSetField por alias
	srvMap     map[string]string          // TCSrvMap: alias -> lista de campos
	txDepth    int                        // TCCommit: transações aninhadas
	replayOn   bool                       // TCSQLReplay: coleta ativa
}

// dbstate é o singleton de conexões DBAccess do pacote.
var dbstate = newDbaccessState()

func newDbaccessState() *dbAccessState {
	return &dbAccessState{
		conns:      make(map[int]*dbstateConn),
		active:     -1,
		nextID:     1,
		pools:      make(map[string][]int),
		maxMap:     25,
		config:     dbaccessDefaultConfig(),
		params:     make(map[string]string),
		fieldTypes: make(map[string]map[string]dbaccessFieldType),
		srvMap:     make(map[string]string),
	}
}

func dbaccessDefaultConfig() map[string]string {
	return map[string]string{
		"SETUSEROWSTAMP": "OFF",
		"SETAUTOSTAMP":   "OFF",
		"SETMEMOINQUERY": "OFF",
		"SETUSEROWINSDT": "OFF",
		"SETAUTOINSDT":   "OFF",
		"TCSOFTREFRESH":  "OFF",
		"SETAUTORECNO":   "OFF",
		"SETVIEWENABLED": "ON",
	}
}

// resetDbaccessState reinicializa o estado global — usado pelos testes.
func resetDbaccessState() {
	dbstate.mu.Lock()
	defer dbstate.mu.Unlock()
	for _, c := range dbstate.conns {
		dbaccessCloseConnLocked(c)
	}
	dbstate.conns = make(map[int]*dbstateConn)
	dbstate.pools = make(map[string][]int)
	dbstate.active = -1
	dbstate.nextID = 1
	dbstate.maxMap = 25
	dbstate.sqlErr = ""
	dbstate.mspErr = ""
	dbstate.txDepth = 0
	dbstate.replayOn = false
	dbstate.params = make(map[string]string)
	dbstate.fieldTypes = make(map[string]map[string]dbaccessFieldType)
	dbstate.srvMap = make(map[string]string)
	dbstate.config = dbaccessDefaultConfig()
}

// dbaccessCloseConnLocked fecha o engine de uma conexão (mu já travada).
func dbaccessCloseConnLocked(c *dbstateConn) {
	if c == nil || c.closed {
		return
	}
	if cl, ok := c.engine.(interface{ Close() error }); ok && cl != nil {
		_ = cl.Close()
	}
	c.closed = true
}

// dbaccessActiveConn devolve a conexão ativa aberta, ou false.
func dbaccessActiveConn() (*dbstateConn, bool) {
	dbstate.mu.Lock()
	defer dbstate.mu.Unlock()
	c, ok := dbstate.conns[dbstate.active]
	if !ok || c == nil || c.closed {
		return nil, false
	}
	return c, true
}

// dbaccessSetSQLErr registra a última ocorrência de erro de statement
// (consumida por TCSQLError).
func dbaccessSetSQLErr(msg string) {
	dbstate.mu.Lock()
	dbstate.sqlErr = msg
	dbstate.mu.Unlock()
}

// dbaccessOpenEngine abre um SQLiteEngine dedicado para uma conexão em um
// arquivo .db único em temp dir. O *SQLiteEngine implementa DBEngine e
// SQLEngine simultaneamente.
func dbaccessOpenEngine(id int) (DBEngine, SQLEngine, string, error) {
	dbPath := filepath.Join(os.TempDir(), fmt.Sprintf("advpp_dba_%s_%d.db", dbaccessSessionID, id))
	eng, err := db.NewSQLiteEngine(dbPath)
	if err != nil {
		return nil, nil, "", err
	}
	return eng, eng, dbPath, nil
}

// dbaccessObjectType devolve "table", "view", "index" ou "" conforme o
// objeto existir no sqlite_master da conexão.
func dbaccessObjectType(c *dbstateConn, name string) string {
	rows, err := c.sqlEng.QueryRows("SELECT type FROM sqlite_master WHERE name = ?", name)
	if err != nil || len(rows) == 0 {
		return ""
	}
	return strings.ToLower(rows[0]["TYPE"])
}

func dbaccessObjectExists(c *dbstateConn, name string) bool {
	return dbaccessObjectType(c, name) != ""
}

// dbaccessQueryRowsOrdered executa a query preservando a ordem das colunas,
// com fallback determinístico (ordem alfabética) quando o engine não
// expõe QueryRowsOrdered.
func dbaccessQueryRowsOrdered(c *dbstateConn, query string, args ...any) ([]string, []map[string]string, error) {
	if oq, ok := c.sqlEng.(interface {
		QueryRowsOrdered(query string, args ...any) ([]string, []map[string]string, error)
	}); ok {
		return oq.QueryRowsOrdered(query, args...)
	}
	rows, err := c.sqlEng.QueryRows(query, args...)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	cols := []string{}
	for _, r := range rows {
		for k := range r {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)
	return cols, rows, nil
}

// dbaccessBuildArr converte linhas (mapa coluna→string) em um array AdvPL
// de linhas, cada linha um array de valores na ordem de cols.
func dbaccessBuildArr(cols []string, rows []map[string]string) *advplrt.ArrayValue {
	elems := make([]advplrt.Value, 0, len(rows))
	for _, r := range rows {
		rowElems := make([]advplrt.Value, len(cols))
		for i, col := range cols {
			rowElems[i] = advplrt.NewString(r[col])
		}
		elems = append(elems, advplrt.NewArray(rowElems))
	}
	return advplrt.NewArray(elems)
}

// dbaccessValueToAny converte um Value AdvPL para um argumento SQL nativo.
func dbaccessValueToAny(v advplrt.Value) any {
	if v == nil || advplrt.IsNil(v) {
		return nil
	}
	switch t := v.(type) {
	case *advplrt.NumberValue:
		return t.Val
	case *advplrt.BoolValue:
		if t.Val {
			return 1
		}
		return 0
	case *advplrt.StringValue:
		return t.Val
	default:
		return advplrt.ToString(v)
	}
}

// dbaccessSQLType converte {tipo AdvPL, tamanho, decimais} em um tipo SQLite.
func dbaccessSQLType(def dbaccessColDef) string {
	switch strings.ToUpper(def.ctype) {
	case "N", "F":
		if def.dec > 0 {
			return fmt.Sprintf("NUMERIC(%d,%d)", def.size, def.dec)
		}
		return fmt.Sprintf("NUMERIC(%d)", def.size)
	case "L":
		return "INTEGER"
	case "D":
		return "TEXT"
	default: // C, M e demais
		return "TEXT"
	}
}

// dbaccessStructToCols converte um array AdvPL de estruturas
// {{nome, tipo, tamanho, dec}, ...} em um mapa nome->definição.
func dbaccessStructToCols(a advplrt.Value) map[string]dbaccessColDef {
	out := map[string]dbaccessColDef{}
	arr, ok := a.(*advplrt.ArrayValue)
	if !ok || arr == nil {
		return out
	}
	for _, el := range arr.Elements {
		row, ok := el.(*advplrt.ArrayValue)
		if !ok || row == nil || len(row.Elements) < 2 {
			continue
		}
		name := strings.ToUpper(advplrt.ToString(row.Elements[0]))
		if name == "" {
			continue
		}
		def := dbaccessColDef{name: name, ctype: strings.ToUpper(advplrt.ToString(row.Elements[1]))}
		if len(row.Elements) >= 3 {
			def.size = int(advplrt.ToFloat(row.Elements[2]))
		}
		if len(row.Elements) >= 4 {
			def.dec = int(advplrt.ToFloat(row.Elements[3]))
		}
		out[name] = def
	}
	return out
}

// dbaccessStructRow monta uma linha de estrutura de query (equivalente a
// DBStruct): {nome, tipo, tamanho, decimais}.
func dbaccessStructRow(name, ctype string, size, dec int) *advplrt.ArrayValue {
	return advplrt.NewArray([]advplrt.Value{
		advplrt.NewString(name),
		advplrt.NewString(ctype),
		advplrt.NewNumber(float64(size)),
		advplrt.NewNumber(float64(dec)),
	})
}

// supportedDBs são os dialetos aceitos por MSParse/MSParseFull (TDN).
var supportedDBs = map[string]bool{
	"INFORMIX": true,
	"DB2":      true,
	"ORACLE":   true,
	"SYBASE":   true,
	"MSSQL":    true,
	"MYSQL":    true,
	"POSTGRES": true,
	"POSTGRESQL": true,
}

// dbaccessParseSQL é o parser simples e honesto do MSParse: valida o dialeto
// de destino, remove linhas "GO" (delimitador MSSQL) e normaliza o texto.
// Não converte dialeto de fato — o SGBD de facto desta VM é o SQLite.
func dbaccessParseSQL(cSQL, cBD string) (converted, errMsg string) {
	sql := strings.TrimSpace(cSQL)
	bd := strings.ToUpper(strings.TrimSpace(cBD))
	if sql == "" {
		return "", "MSParse: SQL vazio"
	}
	if bd == "" || !supportedDBs[bd] {
		return "", "MSParse: banco de destino invalido: " + cBD
	}
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.EqualFold(strings.TrimSpace(ln), "go") {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n"), ""
}

// registerDbaccessNatives registra as 54 funções DB DBAccess (TC* + MSParse*
// + DBCreateIndex) sobre o SQLiteEngine. Assinatura idêntica às demais
// register*Natives do pacote — o mapa é preenchido e NÃO é registrado em
// natives.go (a chamada fica a cargo do chamador/teste).
func (v *VM) registerDbaccessNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {

	// TCLink([cConn],[cServerAddr],[nPort]) -> nHwnd
	// Cria uma nova conexão (SQLiteEngine em temp dir), que passa a ser a
	// ativa. Devolve id >= 0; negativo em falha. cConn = "DRIVER/nome".
	natives["TCLINK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cConn := getArgString(args, 0, "")
		cServer := getArgString(args, 1, "")
		nPort := 7890
		if len(args) >= 3 && !advplrt.IsNil(args[2]) {
			nPort = int(advplrt.ToFloat(args[2]))
		}
		driver, connName := "MSSQL", "ADVPP"
		if cConn != "" {
			parts := strings.SplitN(cConn, "/", 2)
			driver = strings.ToUpper(strings.TrimSpace(parts[0]))
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				connName = strings.TrimSpace(parts[1])
			}
		}
		if driver == "" {
			driver = "MSSQL"
		}
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		id := dbstate.nextID
		dbstate.nextID++
		eng, sqlEng, dbPath, err := dbaccessOpenEngine(id)
		if err != nil {
			dbstate.sqlErr = err.Error()
			return advplrt.NewNumber(-1), nil
		}
		conn := &dbstateConn{
			id:      id,
			connStr: cConn,
			server:  cServer,
			port:    nPort,
			driver:  driver,
			dbsid:   fmt.Sprintf("ADVPP-DBSID-%d", id),
			sid:     id,
			engine:  eng,
			sqlEng:  sqlEng,
			dbPath:  dbPath,
			views:   make(map[string]*dbViewMeta),
		}
		_ = connName
		dbstate.conns[id] = conn
		dbstate.active = id
		dbstate.sqlErr = ""
		return advplrt.NewNumber(float64(id)), nil
	}

	// TCUnlink([nHandle],[lVerbose]) -> lRet
	// Encerra a conexão informada (ou a ativa). .T. em sucesso.
	natives["TCUNLINK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		target := dbstate.active
		if len(args) >= 1 && !advplrt.IsNil(args[0]) {
			target = int(advplrt.ToFloat(args[0]))
		}
		c, ok := dbstate.conns[target]
		if !ok || c == nil || c.closed {
			return advplrt.False, nil
		}
		dbaccessCloseConnLocked(c)
		delete(dbstate.conns, target)
		for name, list := range dbstate.pools {
			out := list[:0]
			for _, id := range list {
				if id != target {
					out = append(out, id)
				}
			}
			dbstate.pools[name] = out
		}
		if dbstate.active == target {
			dbstate.active = -1
			for id, cc := range dbstate.conns {
				if cc != nil && !cc.closed && id > dbstate.active {
					dbstate.active = id
				}
			}
		}
		return advplrt.True, nil
	}

	// TCQuit([nOption]) -> lRet
	// Finaliza todas as conexões ativas da thread atual.
	natives["TCQUIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		for _, c := range dbstate.conns {
			if c != nil && !c.closed {
				dbaccessCloseConnLocked(c)
			}
		}
		dbstate.conns = make(map[int]*dbstateConn)
		dbstate.pools = make(map[string][]int)
		dbstate.active = -1
		return advplrt.True, nil
	}

	// TCGetConn() -> nHnd — conexão ativa, ou -1.
	natives["TCGETCONN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		if c, ok := dbstate.conns[dbstate.active]; ok && c != nil && !c.closed {
			return advplrt.NewNumber(float64(dbstate.active)), nil
		}
		return advplrt.NewNumber(-1), nil
	}

	// TCSetConn(nHandle) -> lRet — alterna a conexão corrente.
	natives["TCSETCONN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		h := int(advplrt.ToFloat(getArg(args, 0)))
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		c, ok := dbstate.conns[h]
		if !ok || c == nil || c.closed {
			return advplrt.False, nil
		}
		dbstate.active = h
		return advplrt.True, nil
	}

	// TCIsConnected([nHwnd]) -> lRet — verifica se está conectado.
	natives["TCISCONNECTED"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		target := dbstate.active
		if len(args) >= 1 && !advplrt.IsNil(args[0]) {
			target = int(advplrt.ToFloat(args[0]))
		}
		c, ok := dbstate.conns[target]
		if !ok || c == nil || c.closed {
			return advplrt.False, nil
		}
		return advplrt.True, nil
	}

	// TCGetDB() -> cRet — identificador do SGBD da conexão ativa.
	natives["TCGETDB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if c, ok := dbaccessActiveConn(); ok {
			return advplrt.NewString(c.driver), nil
		}
		return advplrt.NewString(""), nil
	}

	// TCGetSID() -> nRet — número da thread/processo no DBAccess (-1 sem conexão).
	natives["TCGETSID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if c, ok := dbaccessActiveConn(); ok {
			return advplrt.NewNumber(float64(c.sid)), nil
		}
		return advplrt.NewNumber(-1), nil
	}

	// TCGetDBSID() -> cRet — identificador da conexão no SGBD.
	natives["TCGETDBSID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if c, ok := dbaccessActiveConn(); ok {
			return advplrt.NewString(c.dbsid), nil
		}
		return advplrt.NewString(""), nil
	}

	// TCGetBuild([lDate]) -> cRet — build do DBAccess conectado.
	natives["TCGETBUILD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewString("V40f-"), nil
		}
		lDate := len(args) >= 1 && advplrt.ToBool(args[0])
		if lDate {
			return advplrt.NewString(dbaccessServerBuild + "-" + dbaccessServerDate), nil
		}
		return advplrt.NewString(dbaccessServerBuild), nil
	}

	// TCVersion() -> cRet — versão do build do DBAccess conectado.
	natives["TCVERSION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(dbaccessServerBuild), nil
	}

	// TCAPIVersion() -> cRet — versão da build da DBAPI.
	natives["TCAPIVERSION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(dbaccessAPIVersion), nil
	}

	// TCAPIBuild() -> cRet — build e data de geração da DBAPI.
	natives["TCAPIBUILD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewString("00000000"), nil
		}
		return advplrt.NewString(dbaccessAPIBuild), nil
	}

	// TCGetInfo(nSlot, [cParam]) -> cInfoStr — informação do DBAccess por slot.
	natives["TCGETINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nSlot := int(advplrt.ToFloat(getArg(args, 0)))
		_ = getArgString(args, 1, "")
		arch := "64Bits"
		if strconv.IntSize == 32 {
			arch = "32Bits"
		}
		mode := "Release"
		if v.debugger != nil {
			mode = "Debug"
		}
		dbstate.mu.Lock()
		nConns := 0
		for _, c := range dbstate.conns {
			if c != nil && !c.closed {
				nConns++
			}
		}
		var connInfo string
		if c, ok := dbstate.conns[dbstate.active]; ok && c != nil && !c.closed {
			connInfo = fmt.Sprintf("CONNECTION\t%d\t%s\t%s\t%d", c.id, c.driver, c.connStr, c.port)
		}
		dbstate.mu.Unlock()
		switch nSlot {
		case 1:
			return advplrt.NewString(dbaccessServerBuild), nil
		case 2:
			return advplrt.NewString("0"), nil
		case 3:
			return advplrt.NewString("0"), nil
		case 4:
			return advplrt.NewString(runtime.GOOS), nil
		case 5:
			return advplrt.NewString(""), nil
		case 6:
			return advplrt.NewString(strconv.Itoa(nConns)), nil
		case 7:
			return advplrt.NewString("0"), nil
		case 8:
			return advplrt.NewString(""), nil
		case 9:
			return advplrt.NewString("0"), nil
		case 10:
			return advplrt.NewString(""), nil
		case 11:
			return advplrt.NewString(""), nil
		case 12:
			return advplrt.NewString(""), nil
		case 13:
			return advplrt.NewString(connInfo), nil
		case 14:
			return advplrt.NewString(""), nil
		case 15:
			return advplrt.NewString("STANDALONE"), nil
		case 16:
			return advplrt.NewString(""), nil
		case 17:
			return advplrt.NewString(""), nil
		case 18:
			return advplrt.NewString(arch), nil
		case 19:
			return advplrt.NewString(runtime.GOOS), nil
		case 20:
			return advplrt.NewString(mode), nil
		case 21:
			return advplrt.NewString("0"), nil
		case 22:
			return advplrt.NewString("2048"), nil
		case 23:
			return advplrt.NewString(""), nil
		case 24:
			return advplrt.NewString(""), nil
		case 25:
			return advplrt.NewString("0"), nil
		case 26:
			return advplrt.NewString("0"), nil
		case 27, 28, 29, 30, 31:
			return advplrt.NewString(""), nil
		case 32:
			return advplrt.NewString("0"), nil
		default:
			return advplrt.NewString(""), nil
		}
	}

	// TCGetIO(nThreshold) -> aIOs — array {thread, IOs/s} — vazio (sem threads).
	natives["TCGETIO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		return advplrt.NewArray([]advplrt.Value{}), nil
	}

	// TCDrivers() -> aRet — drivers ODBC "instalados" (conjunto suportado).
	natives["TCDRIVERS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		names := []string{"AdvPP SQLite3 Driver", "SQL Server", "Oracle", "PostgreSQL", "MySQL", "DB2", "Informix"}
		elems := make([]advplrt.Value, 0, len(names))
		for _, n := range names {
			attrs := advplrt.NewArray([]advplrt.Value{
				advplrt.NewString("DriverODBCVer=03.52"),
				advplrt.NewString("SQLLevel=1"),
				advplrt.NewString("UsageCount=1"),
			})
			elems = append(elems, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(n), attrs,
			}))
		}
		return advplrt.NewArray(elems), nil
	}

	// TCPoolInfo() -> aRet — array {thread, pool, tempo(seg)} das conexões no pool.
	natives["TCPOOLINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		var elems []advplrt.Value
		for name, list := range dbstate.pools {
			for _, id := range list {
				c := dbstate.conns[id]
				if c == nil || c.closed {
					continue
				}
				secs := int(time.Since(c.poolTime).Seconds())
				elems = append(elems, advplrt.NewArray([]advplrt.Value{
					advplrt.NewNumber(float64(c.sid)),
					advplrt.NewString(name),
					advplrt.NewNumber(float64(secs)),
				}))
			}
		}
		sort.Slice(elems, func(i, j int) bool {
			return advplrt.ToString(elems[i].(*advplrt.ArrayValue).Elements[1]) <
				advplrt.ToString(elems[j].(*advplrt.ArrayValue).Elements[1])
		})
		return advplrt.NewArray(elems), nil
	}

	// TCSetPool(cPool, [lEcho]) -> lRet — adiciona a conexão ativa ao pool.
	natives["TCSETPOOL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cPool := getArgString(args, 0, "")
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		c, ok := dbstate.conns[dbstate.active]
		if !ok || c == nil || c.closed || c.inPool {
			return advplrt.False, nil
		}
		c.inPool = true
		c.poolName = cPool
		c.poolTime = time.Now()
		dbstate.pools[cPool] = append(dbstate.pools[cPool], c.id)
		return advplrt.True, nil
	}

	// TCGetPool(cPool) -> nRet — retira uma conexão do pool (ativa). -35 se vazio.
	natives["TCGETPOOL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cPool := getArgString(args, 0, "")
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		list := dbstate.pools[cPool]
		if len(list) == 0 {
			return advplrt.NewNumber(-35), nil
		}
		id := list[0]
		dbstate.pools[cPool] = list[1:]
		if c, ok := dbstate.conns[id]; ok && c != nil {
			c.inPool = false
			c.poolName = ""
			dbstate.active = id
		}
		return advplrt.NewNumber(float64(id)), nil
	}

	// TCConfig(cParms) -> cRet — altera/consulta configurações do DBAccess.
	natives["TCCONFIG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(dbaccessConfig(getArgString(args, 0, ""))), nil
	}

	// TCSetParam(cParam, cValue) -> nRet — TOP_PARAM equivalente (0 sucesso).
	natives["TCSETPARAM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cParam := getArgString(args, 0, "")
		cValue := getArgString(args, 1, "")
		if _, ok := dbaccessActiveConn(); !ok {
			dbaccessSetSQLErr("TCSetParam - statement ignored - No connection.")
			return advplrt.NewNumber(-2), nil
		}
		if cParam == "" || cValue == "" {
			dbaccessSetSQLErr("TCSetParam: nome/valor vazio")
			return advplrt.NewNumber(-1), nil
		}
		dbstate.mu.Lock()
		dbstate.params[cParam] = cValue
		dbstate.mu.Unlock()
		dbaccessSetSQLErr("")
		return advplrt.NewNumber(0), nil
	}

	// TCSetField(cAlias, cField, cType, [nSize], [nPrecision]) -> uRet
	// Define o tratamento de tipo de uma coluna de query (estado global).
	natives["TCSETFIELD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cAlias := getArgString(args, 0, "")
		cField := strings.ToUpper(getArgString(args, 1, ""))
		cType := strings.ToUpper(getArgString(args, 2, ""))
		nSize := int(advplrt.ToFloat(getArg(args, 3)))
		nPrecision := int(advplrt.ToFloat(getArg(args, 4)))
		if cType != "D" && cType != "L" && cType != "N" {
			return advplrt.Nil, nil
		}
		if cType == "N" {
			if nSize < 1 || nSize > 18 || nPrecision < 0 || nPrecision > nSize-1 {
				return advplrt.Nil, nil
			}
		}
		dbstate.mu.Lock()
		if dbstate.fieldTypes[cAlias] == nil {
			dbstate.fieldTypes[cAlias] = make(map[string]dbaccessFieldType)
		}
		dbstate.fieldTypes[cAlias][cField] = dbaccessFieldType{cType: cType, size: nSize, precision: nPrecision}
		dbstate.mu.Unlock()
		return advplrt.Nil, nil
	}

	// TCAlter(cTable, aEstruturaAtual, aEstruturaNova, [@nErro]) -> lRet
	// Altera a estrutura de uma tabela via ALTER TABLE do SQLite.
	natives["TCALTER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTable := strings.ToUpper(getArgString(args, 0, ""))
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCAlter - no connection")
			return advplrt.False, nil
		}
		if !identRe.MatchString(cTable) {
			dbaccessSetSQLErr("TCAlter: invalid table name: " + cTable)
			return advplrt.False, nil
		}
		curCols := dbaccessStructToCols(getArg(args, 1))
		newCols := dbaccessStructToCols(getArg(args, 2))
		for name := range newCols {
			if !identRe.MatchString(name) {
				dbaccessSetSQLErr("TCAlter: invalid column name: " + name)
				return advplrt.False, nil
			}
		}
		// Colunas inseridas -> ADD COLUMN
		for name, def := range newCols {
			if _, ok := curCols[name]; ok {
				continue
			}
			ddl := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", cTable, name, dbaccessSQLType(def))
			if err := c.sqlEng.Exec(ddl); err != nil {
				dbaccessSetSQLErr(err.Error())
				return advplrt.False, nil
			}
		}
		// Colunas eliminadas -> DROP COLUMN
		for name := range curCols {
			if _, ok := newCols[name]; ok {
				continue
			}
			ddl := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", cTable, name)
			if err := c.sqlEng.Exec(ddl); err != nil {
				dbaccessSetSQLErr(err.Error())
				return advplrt.False, nil
			}
		}
		// Colunas alteradas: SQLite não suporta ALTER COLUMN; a conversão
		// suportada pelo DBAccess real (N->C preservando dados) é no-op aqui.
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCCanOpen(cTable, [cIndex]) -> lRet — verifica existência de tabela/índice.
	natives["TCCANOPEN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTable := strings.ToUpper(getArgString(args, 0, ""))
		cIndex := strings.ToUpper(getArgString(args, 1, ""))
		if cTable == "" {
			return advplrt.False, nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.False, nil
		}
		if cIndex != "" {
			rows, err := c.sqlEng.QueryRows("SELECT name FROM sqlite_master WHERE type='index' AND name = ?", cIndex)
			if err != nil {
				dbaccessSetSQLErr(err.Error())
				return advplrt.False, nil
			}
			return advplrt.NewBool(len(rows) > 0), nil
		}
		rows, err := c.sqlEng.QueryRows("SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?", cTable)
		if err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		return advplrt.NewBool(len(rows) > 0), nil
	}

	// TCCheckUp([@cMsg]) -> lRet — health check da conexão corrente.
	natives["TCCHECKUP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.False, nil
		}
		if _, err := c.sqlEng.QueryRows("SELECT 1 AS OK"); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		return advplrt.True, nil
	}

	// TCCommit(nOption, [xParam]) -> uRet — controle de transação (no-op).
	natives["TCCOMMIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nOption := int(advplrt.ToFloat(getArg(args, 0)))
		_ = getArg(args, 1)
		dbstate.mu.Lock()
		switch nOption {
		case 1:
			dbstate.txDepth++
		default:
			dbstate.txDepth = 0
		}
		dbstate.mu.Unlock()
		return advplrt.Nil, nil
	}

	// TCDBInsert(cTable, cCols, aData, [nOption]) -> nRet
	// Inclusão em bloco de registros (INSERT com placeholders).
	natives["TCDBINSERT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTable := strings.ToUpper(getArgString(args, 0, ""))
		cCols := getArgString(args, 1, "")
		if cTable == "" || cCols == "" {
			dbaccessSetSQLErr("TCDBInsert - Invalid Empty Field List")
			return advplrt.NewNumber(-1), nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCDBInsert - statement ignored - No connection.")
			return advplrt.NewNumber(-1), nil
		}
		cols := strings.Split(cCols, ",")
		for i := range cols {
			cols[i] = strings.ToUpper(strings.TrimSpace(cols[i]))
			if !identRe.MatchString(cols[i]) {
				dbaccessSetSQLErr("TCDBInsert - Invalid Field List")
				return advplrt.NewNumber(-1), nil
			}
		}
		arr, ok := getArg(args, 2).(*advplrt.ArrayValue)
		if !ok || arr == nil || len(arr.Elements) == 0 {
			dbaccessSetSQLErr("TCDBInsert - Invalid Empty Data Array")
			return advplrt.NewNumber(-1), nil
		}
		ph := make([]string, len(cols))
		for i := range ph {
			ph[i] = "?"
		}
		ddl := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", cTable, strings.Join(cols, ","), strings.Join(ph, ","))
		for ri, rowEl := range arr.Elements {
			row, ok := rowEl.(*advplrt.ArrayValue)
			if !ok || row == nil {
				dbaccessSetSQLErr(fmt.Sprintf("TCDBInsert - Unexpected Element Type ( Row %d )", ri+1))
				return advplrt.NewNumber(-1), nil
			}
			if len(row.Elements) != len(cols) {
				dbaccessSetSQLErr(fmt.Sprintf("TCDBInsert - Number of columns does not match field list ( Row %d )", ri+1))
				return advplrt.NewNumber(-1), nil
			}
			vals := make([]any, len(cols))
			for ci, cel := range row.Elements {
				vals[ci] = dbaccessValueToAny(cel)
			}
			if err := c.sqlEng.Exec(ddl, vals...); err != nil {
				dbaccessSetSQLErr(err.Error())
				return advplrt.NewNumber(-1), nil
			}
		}
		dbaccessSetSQLErr("")
		return advplrt.NewNumber(0), nil
	}

	// TCDelFile(cName) -> lRet — exclui tabela ou view (DROP).
	natives["TCDELFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.ToUpper(getArgString(args, 0, ""))
		if cName == "" || !identRe.MatchString(cName) {
			dbaccessSetSQLErr("TCDelFile: invalid name")
			return advplrt.False, nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCDelFile - no connection")
			return advplrt.False, nil
		}
		typ := dbaccessObjectType(c, cName)
		if typ == "" {
			dbaccessSetSQLErr("TCDelFile: object not found: " + cName)
			return advplrt.False, nil
		}
		drop := "DROP TABLE"
		if typ == "view" {
			drop = "DROP VIEW"
		}
		if err := c.sqlEng.Exec(fmt.Sprintf("%s %s", drop, cName)); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		delete(c.views, cName)
		dbstate.mu.Unlock()
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCGenQry(xPar1, xPar2, cQuery) -> aRet
	// Executa a query na conexão ativa e devolve o resultado como array de
	// linhas (linha = array de valores). Desvio documentado da spec (""),
	// conforme instrução de mapeamento do agente.
	natives["TCGENQRY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		_ = getArg(args, 1)
		cQuery := getArgString(args, 2, "")
		return dbaccessQueryToArr(nil, cQuery, nil)
	}

	// TCGenQry2(xPar1, xPar2, cQuery, aValues) -> aRet
	// Igual à TCGenQry, com BIND dos marcadores "?" da query pelos valores
	// do array aValues.
	natives["TCGENQRY2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		_ = getArg(args, 1)
		cQuery := getArgString(args, 2, "")
		return dbaccessQueryToArr(nil, cQuery, getArg(args, 3))
	}

	// TCSqlToArr(cQuery, aResult, [aBinds], [aSetFields], [aQryStru]) -> nRet
	// Executa a query e preenche aResult (por referência) com as linhas.
	// 0 em sucesso, negativo em falha (detalhe em TCSQLError).
	natives["TCSQLTOARR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cQuery := getArgString(args, 0, "")
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCSqlToArr - no connection")
			return advplrt.NewNumber(-1), nil
		}
		var binds []any
		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok && arr != nil {
			for _, el := range arr.Elements {
				binds = append(binds, dbaccessValueToAny(el))
			}
		}
		cols, rows, err := dbaccessQueryRowsOrdered(c, cQuery, binds...)
		if err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.NewNumber(-1), nil
		}
		dbaccessSetSQLErr("")
		arrOut := dbaccessBuildArr(cols, rows)
		if aRes, ok := getArg(args, 1).(*advplrt.ArrayValue); ok && aRes != nil {
			aRes.Elements = arrOut.Elements
		}
		if aQS, ok := getArg(args, 4).(*advplrt.ArrayValue); ok && aQS != nil {
			structElems := make([]advplrt.Value, 0, len(cols))
			for _, col := range cols {
				structElems = append(structElems, dbaccessStructRow(col, "C", 0, 0))
			}
			aQS.Elements = structElems
		}
		return advplrt.NewNumber(0), nil
	}

	// TCSqlError() -> cRet — última ocorrência de erro de statement.
	natives["TCSQLERROR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		return advplrt.NewString(dbstate.sqlErr), nil
	}

	// TCSPExec(cStoredProcedure, [xParam]) -> aResult
	// SQLite não tem stored procedures: devolve NIL e registra o motivo.
	natives["TCSPEXEC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		_ = getArg(args, 1)
		dbaccessSetSQLErr("TCSPExec - stored procedures not supported on SQLite")
		return advplrt.Nil, nil
	}

	// TCSPExist(cStoredProc) -> lRet — sempre .F. (SQLite sem SPs).
	natives["TCSPEXIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		return advplrt.False, nil
	}

	// TCSqlPlan(cQuery, aResult, [nLevel]) -> nRet
	// Plano de execução via EXPLAIN QUERY PLAN do SQLite; aResult =
	// {descrição das colunas, linhas do plano}. 0 em sucesso.
	natives["TCSQLPLAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cQuery := getArgString(args, 0, "")
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCSqlPlan - no connection")
			return advplrt.NewNumber(-1), nil
		}
		cols, rows, err := dbaccessQueryRowsOrdered(c, "EXPLAIN QUERY PLAN "+cQuery)
		if err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.NewNumber(-1), nil
		}
		dbaccessSetSQLErr("")
		colDefs := make([]advplrt.Value, 0, len(cols))
		for _, col := range cols {
			colDefs = append(colDefs, dbaccessStructRow(col, "C", 0, 0))
		}
		if aRes, ok := getArg(args, 1).(*advplrt.ArrayValue); ok && aRes != nil {
			aRes.Elements = []advplrt.Value{
				advplrt.NewArray(colDefs),
				dbaccessBuildArr(cols, rows),
			}
		}
		return advplrt.NewNumber(0), nil
	}

	// TCSqlReplay(nOption, @cMessage) -> lRet — coleta de trace (estado local).
	natives["TCSQLREPLAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nOption := int(advplrt.ToFloat(getArg(args, 0)))
		_ = getArg(args, 1)
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		switch nOption {
		case 1: // implementação existe
			return advplrt.True, nil
		case 2: // inicia
			dbstate.replayOn = true
			return advplrt.True, nil
		case 3: // finaliza
			dbstate.replayOn = false
			return advplrt.True, nil
		case 4: // ativo?
			return advplrt.NewBool(dbstate.replayOn), nil
		case 5, 6, 7:
			return advplrt.True, nil
		default:
			return advplrt.False, nil
		}
	}

	// TCSrvMap(cAlias, [cMap], [bRefresh]) -> lRet
	// Mapeia os campos de seleção da tabela (estado por alias).
	natives["TCSRVMAP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cAlias := strings.ToUpper(getArgString(args, 0, ""))
		cMap := getArgString(args, 1, "")
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		if cMap == "" {
			delete(dbstate.srvMap, cAlias)
		} else {
			dbstate.srvMap[cAlias] = cMap
		}
		dbstate.mu.Unlock()
		return advplrt.True, nil
	}

	// TCSrvType() -> cRet — plataforma da conexão (RDD SQL TOPCONN = "TOP4").
	natives["TCSRVTYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := dbaccessActiveConn(); !ok {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString("TOP4"), nil
	}

	// TCStruct(cName) -> aRet — estrutura da tabela/view no SGBD
	// {nome, tipo SQL, nulo, tamanho, decimais, tipo AdvPL}.
	natives["TCSTRUCT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.ToUpper(getArgString(args, 0, ""))
		if !identRe.MatchString(cName) {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		rows, err := c.sqlEng.QueryRows("PRAGMA table_info(" + cName + ")")
		if err != nil || len(rows) == 0 {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		elems := make([]advplrt.Value, 0, len(rows))
		for _, r := range rows {
			name := r["NAME"]
			sqlType := r["TYPE"]
			nullable := r["NOTNULL"] != "1"
			advType, size, dec := dbGenSQLiteType(sqlType)
			elems = append(elems, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(name),
				advplrt.NewString(sqlType),
				advplrt.NewBool(nullable),
				advplrt.NewNumber(float64(size)),
				advplrt.NewNumber(float64(dec)),
				advplrt.NewString(advType),
			}))
		}
		return advplrt.NewArray(elems), nil
	}

	// TCUnique(cTabela, [cColumns]) -> nRet
	// Cria/apaga o índice único "cTabela_UNQ" (colunas separadas por "+").
	natives["TCUNIQUE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTable := strings.ToUpper(getArgString(args, 0, ""))
		cColumns := getArgString(args, 1, "")
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCUnique - no connection")
			return advplrt.NewNumber(-1), nil
		}
		if !identRe.MatchString(cTable) {
			dbaccessSetSQLErr("TCUnique: invalid table name")
			return advplrt.NewNumber(-1), nil
		}
		if cColumns == "" {
			if err := c.sqlEng.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s_UNQ", cTable)); err != nil {
				dbaccessSetSQLErr(err.Error())
				return advplrt.NewNumber(-1), nil
			}
			dbaccessSetSQLErr("")
			return advplrt.NewNumber(0), nil
		}
		cols := strings.Split(strings.ToUpper(strings.ReplaceAll(cColumns, "+", ",")), ",")
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
			if !identRe.MatchString(cols[i]) {
				dbaccessSetSQLErr("TCUnique: invalid column: " + cols[i])
				return advplrt.NewNumber(-1), nil
			}
		}
		ddl := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s_UNQ ON %s (%s)", cTable, cTable, strings.Join(cols, ","))
		if err := c.sqlEng.Exec(ddl); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.NewNumber(-1), nil
		}
		dbaccessSetSQLErr("")
		return advplrt.NewNumber(0), nil
	}

	// TCViewOne(cView, cTable) -> lRet — view 1:1 com todos os campos da tabela.
	natives["TCVIEWONE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cView := strings.ToUpper(getArgString(args, 0, ""))
		cTable := strings.ToUpper(getArgString(args, 1, ""))
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCViewOne - no connection")
			return advplrt.False, nil
		}
		if !identRe.MatchString(cView) || !identRe.MatchString(cTable) {
			dbaccessSetSQLErr("TCViewOne: invalid name")
			return advplrt.False, nil
		}
		if dbaccessObjectExists(c, cView) {
			dbaccessSetSQLErr("TCViewOne: object already exists: " + cView)
			return advplrt.False, nil
		}
		if dbaccessObjectType(c, cTable) != "table" {
			dbaccessSetSQLErr("TCViewOne: table not found: " + cTable)
			return advplrt.False, nil
		}
		if err := c.sqlEng.Exec(fmt.Sprintf("CREATE VIEW %s AS SELECT * FROM %s", cView, cTable)); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		c.views[cView] = &dbViewMeta{name: cView, master: cTable, one2one: true}
		dbstate.mu.Unlock()
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCViewMulti(cView, cTable, cStruct) -> lRet
	// View sobre múltiplas tabelas (cStruct = "tab,campo,tab,campo,...").
	natives["TCVIEWMULTI"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cView := strings.ToUpper(getArgString(args, 0, ""))
		cTable := strings.ToUpper(getArgString(args, 1, ""))
		cStruct := getArgString(args, 2, "")
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCViewMulti - no connection")
			return advplrt.False, nil
		}
		if !identRe.MatchString(cView) || !identRe.MatchString(cTable) {
			dbaccessSetSQLErr("TCViewMulti: invalid name")
			return advplrt.False, nil
		}
		if dbaccessObjectExists(c, cView) {
			dbaccessSetSQLErr("TCViewMulti: object already exists: " + cView)
			return advplrt.False, nil
		}
		if dbaccessObjectType(c, cTable) != "table" {
			dbaccessSetSQLErr("TCViewMulti: table not found: " + cTable)
			return advplrt.False, nil
		}
		parts := strings.Split(cStruct, ",")
		if len(parts) == 0 || len(parts)%2 != 0 {
			dbaccessSetSQLErr("TCViewMulti: invalid cStruct")
			return advplrt.False, nil
		}
		var sel []string
		var froms []string
		seenFields := map[string]bool{}
		usedTables := map[string]bool{}
		for i := 0; i < len(parts); i += 2 {
			t := strings.ToUpper(strings.TrimSpace(parts[i]))
			f := strings.ToUpper(strings.TrimSpace(parts[i+1]))
			if !identRe.MatchString(t) || !identRe.MatchString(f) {
				dbaccessSetSQLErr("TCViewMulti: invalid table/field name")
				return advplrt.False, nil
			}
			if dbaccessObjectType(c, t) != "table" {
				dbaccessSetSQLErr("TCViewMulti: table not found: " + t)
				return advplrt.False, nil
			}
			if seenFields[f] {
				dbaccessSetSQLErr("TCViewMulti: duplicated field: " + f)
				return advplrt.False, nil
			}
			seenFields[f] = true
			if !usedTables[t] {
				usedTables[t] = true
				froms = append(froms, t)
			}
			sel = append(sel, fmt.Sprintf("%s.%s AS %s", t, f, f))
		}
		sort.Strings(froms)
		qry := fmt.Sprintf("CREATE VIEW %s AS SELECT %s FROM %s",
			cView, strings.Join(sel, ", "), strings.Join(froms, ", "))
		if err := c.sqlEng.Exec(qry); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		c.views[cView] = &dbViewMeta{name: cView, master: cTable, structStr: cStruct}
		dbstate.mu.Unlock()
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCView2DB(cView, cTable) -> lRet — materializa uma view em tabela física.
	natives["TCVIEW2DB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cView := strings.ToUpper(getArgString(args, 0, ""))
		cTable := strings.ToUpper(getArgString(args, 1, ""))
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCView2DB - no connection")
			return advplrt.False, nil
		}
		if !identRe.MatchString(cView) || !identRe.MatchString(cTable) {
			dbaccessSetSQLErr("TCView2DB: invalid name")
			return advplrt.False, nil
		}
		if dbaccessObjectType(c, cView) != "view" {
			dbaccessSetSQLErr("TCView2DB: view not found: " + cView)
			return advplrt.False, nil
		}
		if dbaccessObjectExists(c, cTable) {
			dbaccessSetSQLErr("TCView2DB: table already exists: " + cTable)
			return advplrt.False, nil
		}
		if err := c.sqlEng.Exec(fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s", cTable, cView)); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCViewRen(cViewName, cViewNewName) -> lRet — renomeia uma view.
	natives["TCVIEWREN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cView := strings.ToUpper(getArgString(args, 0, ""))
		cNew := strings.ToUpper(getArgString(args, 1, ""))
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCViewRen - no connection")
			return advplrt.False, nil
		}
		if !identRe.MatchString(cView) || !identRe.MatchString(cNew) {
			dbaccessSetSQLErr("TCViewRen: invalid name")
			return advplrt.False, nil
		}
		if dbaccessObjectType(c, cView) != "view" {
			dbaccessSetSQLErr("TCViewRen: view not found: " + cView)
			return advplrt.False, nil
		}
		if err := c.sqlEng.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", cView, cNew)); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		if m, ok := c.views[cView]; ok {
			delete(c.views, cView)
			m.name = cNew
			c.views[cNew] = m
		}
		dbstate.mu.Unlock()
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCViewStruct(cView, @cTable, @cStruct) -> lRet
	// Devolve .T. se a view existe. cTable/cStruct (strings por referência)
	// não podem ser escritos de volta nesta VM — documentado.
	natives["TCVIEWSTRUCT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cView := strings.ToUpper(getArgString(args, 0, ""))
		_ = getArg(args, 1)
		_ = getArg(args, 2)
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCViewStruct - no connection")
			return advplrt.False, nil
		}
		dbstate.mu.Lock()
		_, found := c.views[cView]
		dbstate.mu.Unlock()
		if !found {
			dbaccessSetSQLErr("TCViewStruct: view not found: " + cView)
			return advplrt.False, nil
		}
		dbaccessSetSQLErr("")
		return advplrt.True, nil
	}

	// TCHasView(cTable) -> aViews — views associadas à tabela.
	natives["TCHASVIEW"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTable := strings.ToUpper(getArgString(args, 0, ""))
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		dbstate.mu.Lock()
		var names []string
		for _, m := range c.views {
			if m.master == cTable {
				names = append(names, m.name)
			}
		}
		dbstate.mu.Unlock()
		sort.Strings(names)
		elems := make([]advplrt.Value, len(names))
		for i, n := range names {
			elems[i] = advplrt.NewString(n)
		}
		return advplrt.NewArray(elems), nil
	}

	// TCIsView(cName) -> lRet — o objeto informado é uma view?
	natives["TCISVIEW"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.ToUpper(getArgString(args, 0, ""))
		if cName == "" {
			return advplrt.False, nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.False, nil
		}
		return advplrt.NewBool(dbaccessObjectType(c, cName) == "view"), nil
	}

	// TCObject(cObject, [@cType]) -> lRet — o objeto existe no SGBD?
	natives["TCOBJECT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cObject := strings.ToUpper(getArgString(args, 0, ""))
		_ = getArg(args, 1)
		if cObject == "" {
			return advplrt.False, nil
		}
		c, ok := dbaccessActiveConn()
		if !ok {
			return advplrt.False, nil
		}
		return advplrt.NewBool(dbaccessObjectExists(c, cObject)), nil
	}

	// TCRefresh(cTable) -> uRet — recria cache de definições (no-op aqui).
	natives["TCREFRESH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		_ = getArg(args, 0)
		return advplrt.Nil, nil
	}

	// TCMaxMap(cNum) -> uRet — mínimo de colunas para o TCSrvMap.
	natives["TCMAXMAP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		n := int(advplrt.ToFloat(getArg(args, 0)))
		if n < 1 {
			n = 1
		}
		dbstate.mu.Lock()
		dbstate.maxMap = n
		dbstate.mu.Unlock()
		return advplrt.Nil, nil
	}

	// DBCreateIndex(cName, cExprKey, [bExprKey], [lUnique]) -> uRet
	// Cria índice sobre a tabela da área de trabalho corrente
	// (v.currentAlias). Sempre retorna NIL (spec); erro em TCSQLError.
	natives["DBCREATEINDEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.ToUpper(getArgString(args, 0, ""))
		cExprKey := strings.ToUpper(getArgString(args, 1, ""))
		lUnique := len(args) >= 4 && advplrt.ToBool(args[3])
		c, ok := dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("DBCreateIndex - no connection")
			return advplrt.Nil, nil
		}
		table := strings.ToUpper(strings.TrimSpace(v.currentAlias))
		if table == "" {
			dbaccessSetSQLErr("DBCreateIndex: no workarea")
			return advplrt.Nil, nil
		}
		if !identRe.MatchString(cName) || !identRe.MatchString(table) {
			dbaccessSetSQLErr("DBCreateIndex: invalid name")
			return advplrt.Nil, nil
		}
		cols := strings.ReplaceAll(cExprKey, "+", ",")
		uniq := ""
		if lUnique {
			uniq = "UNIQUE "
		}
		ddl := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", uniq, cName, table, cols)
		if err := c.sqlEng.Exec(ddl); err != nil {
			dbaccessSetSQLErr(err.Error())
			return advplrt.Nil, nil
		}
		dbaccessSetSQLErr("")
		return advplrt.Nil, nil
	}

	// MSParse(cSQL, cBD, [lComp]) -> cResult
	// Converte (de forma simples/honesta) uma SP ANSI para o dialeto alvo;
	// devolve "" com o motivo em MSParseError em caso de falha.
	natives["MSPARSE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSQL := getArgString(args, 0, "")
		cBD := getArgString(args, 1, "")
		conv, errMsg := dbaccessParseSQL(cSQL, cBD)
		dbstate.mu.Lock()
		dbstate.mspErr = errMsg
		dbstate.mu.Unlock()
		if errMsg != "" {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(conv), nil
	}

	// MSParseFull(cSQL, cBD, cError, cOut) -> iRet
	// 1 em sucesso, 0 em falha. cError/cOut (strings por referência) não
	// podem ser escritos de volta nesta VM — o motivo fica em MSParseError.
	natives["MSPARSEFULL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSQL := getArgString(args, 0, "")
		cBD := getArgString(args, 1, "")
		_, errMsg := dbaccessParseSQL(cSQL, cBD)
		dbstate.mu.Lock()
		dbstate.mspErr = errMsg
		dbstate.mu.Unlock()
		if errMsg != "" {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(1), nil
	}

	// MSParseError() -> cMensagem — último erro do MSParse.
	natives["MSPARSEERROR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dbstate.mu.Lock()
		defer dbstate.mu.Unlock()
		return advplrt.NewString(dbstate.mspErr), nil
	}
}

// dbaccessQueryToArr executa uma query na conexão ativa e devolve o
// resultado como array AdvPL de linhas. Se aBinds (array) for informado,
// os marcadores "?" da query são substituídos pelos valores em ordem.
func dbaccessQueryToArr(c *dbstateConn, query string, aBinds advplrt.Value) (*advplrt.ArrayValue, error) {
	if strings.TrimSpace(query) == "" {
		return advplrt.NewArray([]advplrt.Value{}), nil
	}
	if c == nil {
		var ok bool
		c, ok = dbaccessActiveConn()
		if !ok {
			dbaccessSetSQLErr("TCGenQry - no connection")
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
	}
	var args []any
	if arr, ok := aBinds.(*advplrt.ArrayValue); ok && arr != nil {
		for _, el := range arr.Elements {
			args = append(args, dbaccessValueToAny(el))
		}
	}
	cols, rows, err := dbaccessQueryRowsOrdered(c, query, args...)
	if err != nil {
		dbaccessSetSQLErr(err.Error())
		return advplrt.NewArray([]advplrt.Value{}), nil
	}
	dbaccessSetSQLErr("")
	return dbaccessBuildArr(cols, rows), nil
}

// dbaccessConfigOptions lista as configurações retornadas por
// TCConfig('ALL_CONFIG_OPTIONS').
var dbaccessConfigOptions = strings.Join([]string{
	"SETUSEROWSTAMP", "GETUSEROWSTAMP", "SETAUTOSTAMP", "GETAUTOSTAMP",
	"SETMEMOINQUERY", "GETMEMOINQUERY", "SETUSEROWINSDT", "GETUSEROWINSDT",
	"SETAUTOINSDT", "GETAUTOINSDT", "TCSOFTREFRESH", "GETTCSOFTREFRESH",
	"SETTEMPKEEPALIVE", "SETUUIDFIELDS", "SETUUIDFIELDSNC", "LISTUUIDFIELDS",
	"GETAUTORECNO", "SETAUTORECNO", "SETVIEWENABLED", "GETVIEWENABLED",
	"ALL_CONFIG_OPTIONS",
}, ";")

// dbaccessConfig implementa TCConfig: SET*/GET* das opções suportadas.
func dbaccessConfig(cParms string) string {
	trimmed := strings.TrimSpace(cParms)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	if upper == "ALL_CONFIG_OPTIONS" {
		return dbaccessConfigOptions
	}
	dbstate.mu.Lock()
	defer dbstate.mu.Unlock()
	if i := strings.Index(upper, "="); i > 0 {
		key := strings.TrimSpace(upper[:i])
		val := strings.TrimSpace(trimmed[i+1:])
		valUpper := strings.ToUpper(val)
		switch key {
		case "SETUSEROWSTAMP", "SETAUTOSTAMP", "SETMEMOINQUERY",
			"SETUSEROWINSDT", "SETAUTOINSDT", "TCSOFTREFRESH",
			"SETAUTORECNO", "SETVIEWENABLED":
			if valUpper != "ON" && valUpper != "OFF" {
				return "INVALID_OPTION"
			}
			dbstate.config[key] = valUpper
			return "OK"
		case "SETTEMPKEEPALIVE":
			dbstate.config[key] = val
			return "OK"
		case "SETUUIDFIELDS", "SETUUIDFIELDSNC":
			dbstate.config[key] = val
			return "OK"
		case "LISTUUIDFIELDS":
			return ""
		}
		return ""
	}
	switch upper {
	case "GETUSEROWSTAMP":
		return dbstate.config["SETUSEROWSTAMP"]
	case "GETAUTOSTAMP":
		return dbstate.config["SETAUTOSTAMP"]
	case "GETMEMOINQUERY":
		return dbstate.config["SETMEMOINQUERY"]
	case "GETUSEROWINSDT":
		return dbstate.config["SETUSEROWINSDT"]
	case "GETAUTOINSDT":
		return dbstate.config["SETAUTOINSDT"]
	case "GETTCSOFTREFRESH":
		return dbstate.config["TCSOFTREFRESH"]
	case "GETAUTORECNO":
		return dbstate.config["SETAUTORECNO"]
	case "GETVIEWENABLED":
		return dbstate.config["SETVIEWENABLED"]
	}
	return ""
}
