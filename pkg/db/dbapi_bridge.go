package db

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// dbapiBridge é o cliente Go do advpp-dbapi-bridge (tools/dbapi-bridge):
// processo C++ que dlopen o dbapi.so da TOTVS e fala com um dbaccess64
// real. Protocolo binário documentado em tools/dbapi-bridge/bridge.cpp.
//
// Endereço do bridge: env ADVPP_DBAPI_BRIDGE (default 127.0.0.1:17890).
//
// mapeamento ConnConfig para driver "DBACCESS" (DbConnection:New/TCLINK):
//   Host     -> host do dbaccess64        (default 127.0.0.1)
//   Port     -> porta do dbaccess64       (default 7890)
//   Service  -> environment "DRIVER/env"  (ex.: "POSTGRES/p1212510")
//   User     -> usuário dbapi             (default "")
//   Password -> não usado
//   env ADVPP_DBAPI_LOCKTYPE -> locktype  (default 0)

const (
	dbapiOpConnect    = 1
	dbapiOpQuery      = 2
	dbapiOpFetch      = 3
	dbapiOpExec       = 4
	dbapiOpCloseWA    = 5
	dbapiOpDisconnect = 6
	dbapiOpPing       = 7

	dbapiTagRow  = 0x82
	dbapiTagCols = 0x83
	dbapiTagDone = 0x84

	dbapiNullLen = 0xFFFFFFFF
)

type dbapiField struct {
	data   []byte
	isNull bool
}

type dbapiColMeta struct {
	name  string
	typ   string
	size  int
	dec   int
	flags int
}

type dbapiRow struct {
	recno int64
	vals  []dbapiField
}

// dbapiBridgeClient é uma conexão TCP com o bridge. Uma goroutine por
// request: todas as chamadas são serializadas por mu (o bridge também
// serializa o dbapi.so internamente).
type dbapiBridgeClient struct {
	conn net.Conn
	mu   sync.Mutex
}

func dialDbapiBridge() (*dbapiBridgeClient, error) {
	addr := os.Getenv("ADVPP_DBAPI_BRIDGE")
	if addr == "" {
		addr = "127.0.0.1:17890"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dbapi bridge %s inacessivel: %v (suba tools/dbapi-bridge)", addr, err)
	}
	return &dbapiBridgeClient{conn: conn}, nil
}

func (c *dbapiBridgeClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ---- codec ----

func dbapiAppendField(out []byte, f dbapiField) []byte {
	if f.isNull {
		return append(out, 0xff, 0xff, 0xff, 0xff)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(f.data)))
	out = append(out, hdr[:]...)
	return append(out, f.data...)
}

func dbapiAppendU32(out []byte, v uint32) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], v)
	return append(out, hdr[:]...)
}

func dbapiAppendU64(out []byte, v uint64) []byte {
	var hdr [8]byte
	binary.BigEndian.PutUint64(hdr[:], v)
	return append(out, hdr[:]...)
}

func dbapiStr(s string) dbapiField { return dbapiField{data: []byte(s)} }

func (c *dbapiBridgeClient) sendRequest(op byte, fields []dbapiField) error {
	payload := []byte{op}
	payload = dbapiAppendU32(payload, uint32(len(fields)))
	for _, f := range fields {
		payload = dbapiAppendField(payload, f)
	}
	frame := dbapiAppendU32(nil, uint32(len(payload)))
	frame = append(frame, payload...)
	if _, err := c.conn.Write(frame); err != nil {
		return err
	}
	return nil
}

// readFrame lê um frame [u32 len][payload].
func (c *dbapiBridgeClient) readFrame() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c.conn, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > 256*1024*1024 {
		return nil, fmt.Errorf("frame dbapi gigante: %d bytes", n)
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(c.conn, buf)
	return buf, err
}

type dbapiParser struct {
	buf []byte
	off int
}

func (p *dbapiParser) u8() (byte, error) {
	if p.off+1 > len(p.buf) {
		return 0, io.ErrUnexpectedEOF
	}
	b := p.buf[p.off]
	p.off++
	return b, nil
}
func (p *dbapiParser) u32() (uint32, error) {
	if p.off+4 > len(p.buf) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint32(p.buf[p.off:])
	p.off += 4
	return v, nil
}
func (p *dbapiParser) u64() (uint64, error) {
	if p.off+8 > len(p.buf) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint64(p.buf[p.off:])
	p.off += 8
	return v, nil
}
func (p *dbapiParser) field() (dbapiField, error) {
	n, err := p.u32()
	if err != nil {
		return dbapiField{}, err
	}
	if n == dbapiNullLen {
		return dbapiField{isNull: true}, nil
	}
	if p.off+int(n) > len(p.buf) {
		return dbapiField{}, io.ErrUnexpectedEOF
	}
	f := dbapiField{data: p.buf[p.off : p.off+int(n)]}
	p.off += int(n)
	return f, nil
}
func (p *dbapiParser) fields(count uint32) ([]dbapiField, error) {
	out := make([]dbapiField, 0, count)
	for i := uint32(0); i < count; i++ {
		f, err := p.field()
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// readUntilDone lê frames até o frame terminal (status OK/ERR),
// entregando push frames (COLS/ROW/DONE) ao callback onPush.
func (c *dbapiBridgeClient) readUntilDone(onPush func(tag byte, payload []byte) error) ([]dbapiField, bool, error) {
	for {
		frame, err := c.readFrame()
		if err != nil {
			return nil, false, err
		}
		if len(frame) == 0 {
			return nil, false, fmt.Errorf("frame vazio do bridge")
		}
		tag := frame[0]
		switch tag {
		case dbapiTagRow, dbapiTagCols, dbapiTagDone:
			if onPush != nil {
				if err := onPush(tag, frame[1:]); err != nil {
					return nil, false, err
				}
			}
		case 0, 1: // status OK / ERR
			p := &dbapiParser{buf: frame[1:]}
			nf, err := p.u32()
			if err != nil {
				return nil, false, err
			}
			fs, err := p.fields(nf)
			if err != nil {
				return nil, false, err
			}
			if tag == 1 {
				msg := "erro dbapi"
				if len(fs) > 0 && !fs[0].isNull {
					msg = string(fs[0].data)
				}
				return fs, false, fmt.Errorf("%s", msg)
			}
			return fs, true, nil
		default:
			return nil, false, fmt.Errorf("tag desconhecida do bridge: 0x%02x", tag)
		}
	}
}

func dbapiFieldStr(f dbapiField) string {
	if f.isNull {
		return ""
	}
	return string(f.data)
}

// ---- API de alto nível ----

func (c *dbapiBridgeClient) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendRequest(dbapiOpPing, nil); err != nil {
		return err
	}
	_, ok, err := c.readUntilDone(nil)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("ping falhou")
	}
	return nil
}

// Connect abre uma conexão lógica no dbaccess64 via bridge.
// Retorna o connID do lado do bridge.
func (c *dbapiBridgeClient) Connect(host, env, user string, port, locktype int) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{
		dbapiStr(host), dbapiStr(env), dbapiStr(user),
		dbapiStr(strconv.Itoa(port)), dbapiStr(strconv.Itoa(locktype)),
	}
	if err := c.sendRequest(dbapiOpConnect, fields); err != nil {
		return 0, err
	}
	fs, ok, err := c.readUntilDone(nil)
	if err != nil {
		return 0, err
	}
	if !ok || len(fs) == 0 {
		return 0, fmt.Errorf("connect sem connID")
	}
	id, err := strconv.Atoi(dbapiFieldStr(fs[0]))
	if err != nil {
		return 0, fmt.Errorf("connID invalido: %q", dbapiFieldStr(fs[0]))
	}
	return id, nil
}

func (c *dbapiBridgeClient) Disconnect(connID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendRequest(dbapiOpDisconnect, []dbapiField{dbapiStr(strconv.Itoa(connID))}); err != nil {
		return err
	}
	_, _, err := c.readUntilDone(nil)
	return err
}

// Query abre uma workarea com o resultado do SQL. Devolve wa + metadados
// das colunas. As linhas são lidas via Fetch.
func (c *dbapiBridgeClient) Query(connID int, sql string) (int, []dbapiColMeta, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{dbapiStr(strconv.Itoa(connID)), dbapiStr(sql)}
	if err := c.sendRequest(dbapiOpQuery, fields); err != nil {
		return 0, nil, err
	}

	var cols []dbapiColMeta
	var waStr string
	onPush := func(tag byte, payload []byte) error {
		switch tag {
		case dbapiTagCols:
			p := &dbapiParser{buf: payload}
			nf, err := p.u32()
			if err != nil {
				return err
			}
			fs, err := p.fields(nf)
			if err != nil {
				return err
			}
			if len(fs) < 2 {
				return fmt.Errorf("COLS malformado")
			}
			waStr = dbapiFieldStr(fs[0])
			ncols, _ := strconv.Atoi(dbapiFieldStr(fs[1]))
			cols = make([]dbapiColMeta, 0, ncols)
			for i := 0; i < ncols; i++ {
				base := 2 + i*5
				if base+4 >= len(fs) {
					break
				}
				sz, _ := strconv.Atoi(dbapiFieldStr(fs[base+2]))
				dc, _ := strconv.Atoi(dbapiFieldStr(fs[base+3]))
				fl, _ := strconv.Atoi(dbapiFieldStr(fs[base+4]))
				cols = append(cols, dbapiColMeta{
					name:  strings.ToUpper(dbapiFieldStr(fs[base])),
					typ:   strings.ToUpper(dbapiFieldStr(fs[base+1])),
					size:  sz, dec: dc, flags: fl,
				})
			}
		}
		return nil
	}
	fs, ok, err := c.readUntilDone(onPush)
	if err != nil {
		return 0, nil, err
	}
	if !ok {
		return 0, nil, fmt.Errorf("query sem OK")
	}
	if waStr == "" && len(fs) > 0 {
		waStr = dbapiFieldStr(fs[0])
	}
	wa, err := strconv.Atoi(waStr)
	if err != nil {
		return 0, nil, fmt.Errorf("wa invalido: %q", waStr)
	}
	return wa, cols, nil
}

// Fetch lê até maxRows linhas a partir da posição atual da workarea.
// Devolve as linhas e se chegou ao fim (eof).
func (c *dbapiBridgeClient) Fetch(connID, wa, maxRows int) ([]dbapiRow, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{
		dbapiStr(strconv.Itoa(connID)),
		dbapiStr(strconv.Itoa(wa)),
		dbapiStr(strconv.Itoa(maxRows)),
	}
	if err := c.sendRequest(dbapiOpFetch, fields); err != nil {
		return nil, false, err
	}

	var rows []dbapiRow
	eof := false
	onPush := func(tag byte, payload []byte) error {
		switch tag {
		case dbapiTagRow:
			p := &dbapiParser{buf: payload}
			nrows, err := p.u32()
			if err != nil {
				return err
			}
			for i := uint32(0); i < nrows; i++ {
				rec, err := p.u64()
				if err != nil {
					return err
				}
				nf, err := p.u32()
				if err != nil {
					return err
				}
				vals, err := p.fields(nf)
				if err != nil {
					return err
				}
				rows = append(rows, dbapiRow{recno: int64(rec), vals: vals})
			}
		case dbapiTagDone:
			p := &dbapiParser{buf: payload}
			nf, err := p.u32()
			if err != nil {
				return err
			}
			fs, err := p.fields(nf)
			if err != nil {
				return err
			}
			if len(fs) > 0 {
				eof = dbapiFieldStr(fs[0]) == "1"
			}
		}
		return nil
	}
	_, ok, err := c.readUntilDone(onPush)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("fetch sem OK")
	}
	return rows, eof, nil
}

// CloseWA fecha uma workarea aberta por Query.
func (c *dbapiBridgeClient) CloseWA(connID, wa int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{dbapiStr(strconv.Itoa(connID)), dbapiStr(strconv.Itoa(wa))}
	if err := c.sendRequest(dbapiOpCloseWA, fields); err != nil {
		return err
	}
	_, _, err := c.readUntilDone(nil)
	return err
}

// Exec executa SQL sem retorno de linhas (INSERT/UPDATE/DDL/SP).
func (c *dbapiBridgeClient) Exec(connID int, sql string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{dbapiStr(strconv.Itoa(connID)), dbapiStr(sql)}
	if err := c.sendRequest(dbapiOpExec, fields); err != nil {
		return err
	}
	_, _, err := c.readUntilDone(nil)
	return err
}

// ---- engine ----

// DbapiSQLEngine implementa DBEngine + SQLEngine (duck typing) sobre o
// advpp-dbapi-bridge → dbapi.so → dbaccess64 → SGBD do Protheus.
// Mesmo modelo do RemoteSQLEngine: SelectArea carrega as linhas em
// memória e a navegação opera sobre o slice — semântica idêntica,
// fonte de dados diferente.
type DbapiSQLEngine struct {
	client   *dbapiBridgeClient
	connID   int
	cfg      ConnConfig
	alias    string
	columns  []columnInfo
	records  []map[string]advplrt.Value
	current  int
	isLocked bool

	recordsMutex sync.RWMutex
	driver       string
	closeOnce    sync.Once
}

// dbapiTypeToSQLType traduz o tipo de campo do dbAccess (C/N/F/D/L/M/B...)
// para um nome de tipo que isRemoteNumericSQLType/isRemoteDateSQLType
// reconhecem — reuso das conversões já validadas do RemoteSQLEngine.
func dbapiTypeToSQLType(t string) string {
	switch t {
	case "N", "F", "I":
		return "NUMERIC"
	case "D", "T":
		return "DATE"
	case "L":
		return "BOOLEAN"
	default:
		return "CHAR"
	}
}

// NewDbapiEngineFrom abre a conexão com o bridge e com o dbaccess64
// configurado em cfg (ver mapeamento no topo do arquivo).
func NewDbapiEngineFrom(cfg ConnConfig) (*DbapiSQLEngine, error) {
	if err := acquireRemoteSlot(); err != nil {
		return nil, err
	}
	client, err := dialDbapiBridge()
	if err != nil {
		releaseRemoteSlot()
		return nil, err
	}

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 7890
	}
	env := cfg.Service
	if env == "" {
		return nil, fmt.Errorf("DBACCESS: Service deve ser o environment dbAccess (ex.: POSTGRES/p1212510)")
	}
	locktype := envInt("ADVPP_DBAPI_LOCKTYPE", 0)

	connID, err := client.Connect(host, env, cfg.User, port, locktype)
	if err != nil {
		client.Close()
		releaseRemoteSlot()
		return nil, fmt.Errorf("DBACCESS connect: %v", err)
	}
	return &DbapiSQLEngine{
		client:  client,
		connID:  connID,
		cfg:     cfg,
		driver:  "DBACCESS",
		current: -1,
	}, nil
}

// Clone abre uma NOVA conexão bridge+dbapi com a mesma configuração
// (mesmo padrão do RemoteSQLEngine.Clone para VMs-filhas).
func (e *DbapiSQLEngine) Clone() (*DbapiSQLEngine, error) {
	return NewDbapiEngineFrom(e.cfg)
}

func (e *DbapiSQLEngine) SelectArea(alias string) error {
	e.alias = strings.ToUpper(alias)
	if !identRe.MatchString(e.alias) {
		return fmt.Errorf("invalid table name: %q", e.alias)
	}

	wa, cols, err := e.client.Query(e.connID, "SELECT * FROM "+e.alias)
	if err != nil {
		return fmt.Errorf("table %s not found: %v", e.alias, err)
	}
	defer e.client.CloseWA(e.connID, wa)

	e.columns = make([]columnInfo, len(cols))
	for i, cm := range cols {
		e.columns[i] = columnInfo{name: cm.name, sqlType: dbapiTypeToSQLType(cm.typ)}
	}

	// drena todas as páginas da workarea
	var all []dbapiRow
	for {
		rows, eof, err := e.client.Fetch(e.connID, wa, 5000)
		if err != nil {
			return err
		}
		all = append(all, rows...)
		if eof {
			break
		}
	}

	e.records = make([]map[string]advplrt.Value, 0, len(all))
	for _, r := range all {
		record := make(map[string]advplrt.Value)
		for i, cm := range cols {
			if i >= len(r.vals) {
				break
			}
			f := r.vals[i]
			if f.isNull {
				record[cm.name] = advplrt.Nil
				continue
			}
			s := string(f.data)
			// campos char vêm com padding de tamanho fixo — apara só o
			// excedente, preserva espaços internos significativos
			if cm.typ == "C" && cm.size > 0 && len(s) > cm.size {
				s = s[:cm.size]
			}
			record[cm.name] = remoteValue(s, e.columns[i].sqlType)
		}
		e.records = append(e.records, record)
	}
	e.current = 0
	return nil
}

// QueryRows implementa SQLEngine: executa o SQL via bridge e devolve
// todas as linhas como mapas coluna->string (mesmo formato do
// RemoteSQLEngine.QueryRows — chaves em MAIÚSCULAS, NULL -> "").
func (e *DbapiSQLEngine) QueryRows(query string, args ...any) ([]map[string]string, error) {
	if len(args) > 0 {
		// placeholders "?" não são expandidos pelo dbapi — rejeita honesto
		// em vez de interpolar (evita SQL injection).
		return nil, fmt.Errorf("DBACCESS QueryRows: args não suportados (use literais)")
	}
	wa, cols, err := e.client.Query(e.connID, query)
	if err != nil {
		return nil, err
	}
	defer e.client.CloseWA(e.connID, wa)

	var out []map[string]string
	for {
		rows, eof, err := e.client.Fetch(e.connID, wa, 5000)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			row := make(map[string]string, len(cols))
			for i, cm := range cols {
				if i >= len(r.vals) || r.vals[i].isNull {
					row[cm.name] = ""
					continue
				}
				row[cm.name] = string(r.vals[i].data)
			}
			out = append(out, row)
		}
		if eof {
			break
		}
	}
	return out, nil
}

// Exec implementa SQLEngine: SQLExec no dbaccess64.
func (e *DbapiSQLEngine) Exec(query string, args ...any) error {
	if len(args) > 0 {
		return fmt.Errorf("DBACCESS Exec: args não suportados (use literais)")
	}
	return e.client.Exec(e.connID, query)
}

// ---- navegação DBEngine (idêntica ao RemoteSQLEngine) ----

func (e *DbapiSQLEngine) Seek(key string) (bool, error) {
	for i, record := range e.records {
		for _, val := range record {
			if fmt.Sprintf("%v", val) == key {
				e.current = i
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *DbapiSQLEngine) Skip(count int) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) == 0 {
		return nil
	}
	e.current += count
	if e.current < 0 {
		e.current = 0
	}
	if e.current > len(e.records) {
		e.current = len(e.records)
	}
	return nil
}

func (e *DbapiSQLEngine) GoTop() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.current = 0
	return nil
}

func (e *DbapiSQLEngine) GoBottom() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

func (e *DbapiSQLEngine) EOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current >= len(e.records)
}

func (e *DbapiSQLEngine) BOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current < 0
}

func (e *DbapiSQLEngine) FieldGet(field string) (advplrt.Value, error) {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	if e.current < 0 || e.current >= len(e.records) {
		return advplrt.Nil, nil
	}
	if val, ok := e.records[e.current][strings.ToUpper(field)]; ok {
		return val, nil
	}
	return advplrt.Nil, nil
}

func (e *DbapiSQLEngine) FieldPut(field string, val advplrt.Value) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if e.current < 0 || e.current >= len(e.records) {
		return fmt.Errorf("no current record")
	}
	e.records[e.current][strings.ToUpper(field)] = val
	return nil
}

// RecLock usa o mesmo mutex por tabela do pacote (getTableLock) — lock
// em processo, não lock real no dbAccess (mesma limitação honesta do
// RemoteSQLEngine/SQLiteEngine).
func (e *DbapiSQLEngine) RecLock() error {
	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		return fmt.Errorf("RecLock: no current record")
	}
	e.recordsMutex.RUnlock()
	if e.isLocked {
		return fmt.Errorf("RecLock: record already locked")
	}
	getTableLock(e.alias).Lock()
	e.isLocked = true
	return nil
}

// MsUnlock libera o lock em processo. Escrita de volta ao banco ainda
// não é suportada nesta fatia (read path) — FieldPut altera só a cópia
// em memória, como no SQLiteEngine antes do MsUnlock.
func (e *DbapiSQLEngine) MsUnlock() error {
	if !e.isLocked {
		return nil
	}
	e.isLocked = false
	getTableLock(e.alias).Unlock()
	return nil
}

// Append ainda não é suportado nesta fatia (read path): retorna erro
// honesto em vez de fingir persistência.
func (e *DbapiSQLEngine) Append() error {
	return fmt.Errorf("DBACCESS: DbAppend ainda não suportado (fatia read path)")
}

func (e *DbapiSQLEngine) FieldPos(field string) int {
	field = strings.ToUpper(field)
	for i, c := range e.columns {
		if c.name == field {
			return i + 1
		}
	}
	return 0
}

func (e *DbapiSQLEngine) RecCount() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return len(e.records)
}

func (e *DbapiSQLEngine) RecNo() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	if e.current < 0 || e.current >= len(e.records) {
		return 0
	}
	if n, ok := e.records[e.current]["R_E_C_N_O_"].(*advplrt.NumberValue); ok {
		return int(n.Val)
	}
	return e.current + 1
}

// Close desconecta a sessão dbapi e fecha o socket do bridge.
// Idempotente; devolve a vaga do limite de conexões remotas.
func (e *DbapiSQLEngine) Close() error {
	var err error
	e.closeOnce.Do(func() {
		if e.client != nil {
			if e.connID > 0 {
				e.client.Disconnect(e.connID)
			}
			err = e.client.Close()
		}
		releaseRemoteSlot()
	})
	return err
}
