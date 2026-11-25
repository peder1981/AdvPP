package db

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

// bridgeAddr devolve o endereço configurado (host:port).
func bridgeAddr() string {
	if a := os.Getenv("ADVPP_DBAPI_BRIDGE"); a != "" {
		return a
	}
	return "127.0.0.1:17890"
}

// ensureDbapiBridge tenta subir o advpp-dbapi-bridge quando o dial
// falha. Localiza o binário (env ADVPP_DBAPI_BRIDGE_BIN → mesmo dir do
// executável → PATH) e o dbapi.so (env ADVPP_DBAPI_SO → mesmo dir do
// bridge → /usr/lib → /usr/local/lib → /opt/dbapi). ponytail: só faz
// spawn em endereço loopback; corridas entre processos são benignas
// (o perdedor falha no bind e morre, o dial seguinte acerta o vencedor).
func ensureDbapiBridge() {
	host, port, err := net.SplitHostPort(bridgeAddr())
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
		return // endereço remoto: não spawn local
	}
	bin := os.Getenv("ADVPP_DBAPI_BRIDGE_BIN")
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			cand := filepath.Join(filepath.Dir(exe), "advpp-dbapi-bridge")
			if _, err := os.Stat(cand); err == nil {
				bin = cand
			}
		}
	}
	if bin == "" {
		if p, err := exec.LookPath("advpp-dbapi-bridge"); err == nil {
			bin = p
		}
	}
	if bin == "" {
		return
	}
	so := os.Getenv("ADVPP_DBAPI_SO")
	if so == "" {
		dirs := []string{filepath.Dir(bin), "/usr/lib", "/usr/local/lib", "/opt/dbapi"}
		if exe, err := os.Executable(); err == nil {
			dirs = append([]string{filepath.Dir(exe)}, dirs...)
		}
		for _, d := range dirs {
			cand := filepath.Join(d, "dbapi.so")
			if _, err := os.Stat(cand); err == nil {
				so = cand
				break
			}
		}
	}
	args := []string{}
	if so != "" {
		args = append(args, so)
	}
	args = append(args, port)
	cmd := exec.Command(bin, args...)
	cmd.Stderr = nil // logs do bridge não poluem o advplc (ponytail)
	if err := cmd.Start(); err != nil {
		return
	}
	go cmd.Wait() // zombie avoidance; processo vive até o SO matar
}

func dialDbapiBridge() (*dbapiBridgeClient, error) {
	addr := bridgeAddr()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		ensureDbapiBridge()
		// retry: o bridge leva ~1s para dlopen dbapi.so + bind
		for i := 0; i < 20; i++ {
			time.Sleep(250 * time.Millisecond)
			conn, err = net.DialTimeout("tcp", addr, time.Second)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("dbapi bridge %s inacessivel: %v "+
			"(suba tools/dbapi-bridge ou aponte ADVPP_DBAPI_BRIDGE_BIN/ADVPP_DBAPI_SO)", addr, err)
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

// WaCmd executa um comando de workarea remota (opcode 8). args viram
// p1..p3. Devolve os campos da resposta OK.
func (c *dbapiBridgeClient) WaCmd(connID int, cmd string, args ...string) ([]dbapiField, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := []dbapiField{dbapiStr(strconv.Itoa(connID)), dbapiStr(cmd)}
	for _, a := range args {
		fields = append(fields, dbapiStr(a))
	}
	// WACMD espera [connID, cmd, p1, p2] — completa com vazios
	for len(fields) < 4 {
		fields = append(fields, dbapiStr(""))
	}
	if err := c.sendRequest(8, fields); err != nil {
		return nil, err
	}
	fs, _, err := c.readUntilDone(nil)
	return fs, err
}

// waStr extrai o primeiro campo string de uma resposta WaCmd.
func waStr(fs []dbapiField, i int) string {
	if i < len(fs) && !fs[i].isNull {
		return string(fs[i].data)
	}
	return ""
}

// dbapiFieldSlice corta o valor no tamanho declarado do campo.
//
// Semântica dbapi (verificada empiricamente): GetFieldN/GetField
// devolvem char* para dentro do record buffer e a string CORRE até o
// fim do registro — campos seguintes vêm concatenados. O fielddesc
// informa a largura real (size); o valor correto é s[:size].
// ponytail: fatia em bytes — em bases multi-byte (UTF-8) pode partir
// um caractere; o client dbAccess fala WIN1252, então é byte-safe na
// prática. Se um dia houver encoding UTF-8, trocar por rune-aware.
func dbapiFieldSlice(s string, size int) string {
	if size > 0 && len(s) > size {
		return s[:size]
	}
	return s
}

// dbapiFieldSliceT fatia só tipos texto — o bridge já decodifica 'F'
// (double) para string numérica e tipos binários exóticos chegam como
// hex; fatiar por size corromperia esses valores.
func dbapiFieldSliceT(s string, cm dbapiColMeta) string {
	if cm.typ == "C" || cm.typ == "M" || cm.typ == "" {
		return dbapiFieldSlice(s, cm.size)
	}
	return s
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

	// modo workarea remota: waRemote > 0 quando SelectArea abriu a
	// tabela via OpenFile do dbapi (navegação sob demanda, sem carregar
	// tudo em memória). waCols/waIdx descrevem os campos para FieldGet.
	waRemote  int
	waSeek    int // workarea extra p/ Seek2 — SetKey muda a ordem de
	//            navegação, então seek usa wa separada e devolve o
	//            recno para a wa principal (GoTo), sem poluir a ordem
	waCols    []dbapiColMeta
	waIdx     map[string]int
	waRecCount int // -1 = ainda não consultado (ver RecCount)
	waAppending bool // Append() posicionou além do fim; MsUnlock faz Write

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
		driver:     "DBACCESS",
		current:    -1,
		waRecCount: -1,
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
	e.closeRemoteWA()

	// Tenta workarea remota real primeiro (OpenFile do dbapi): navegação
	// sob demanda sem carregar a tabela em memória. Se o alias não for
	// uma tabela ISAM do ambiente dbAccess, cai no SELECT * em memória.
	if wa, err := e.openRemoteWA(e.alias); err == nil {
		e.waRemote = wa
		e.waNav("gotop") // paridade com o modo memória (current=0)
		return nil
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
			// GetFieldN devolve campo+resto do registro; fatia na largura
			// declarada (dbapiFieldSlice). O trim extra de campos 'C' foi
			// removido: size já é a largura exata do campo.
			s = dbapiFieldSliceT(s, cm)
			record[cm.name] = remoteValue(s, e.columns[i].sqlType)
		}
		e.records = append(e.records, record)
	}
	e.current = 0
	return nil
}

// openRemoteWA abre a tabela como workarea ISAM remota via WACMD open
// e monta waCols/waIdx a partir de colcount+fielddesc. Retorna o handle
// da workarea no bridge ou erro.
func (e *DbapiSQLEngine) openRemoteWA(file string) (int, error) {
	fs, err := e.client.WaCmd(e.connID, "open", file)
	if err != nil || len(fs) == 0 {
		return 0, fmt.Errorf("openRemoteWA: %v", err)
	}
	wa, err := strconv.Atoi(waStr(fs, 0))
	if err != nil || wa <= 0 {
		return 0, fmt.Errorf("openRemoteWA: wa invalido %q", waStr(fs, 0))
	}

	// colunas via colcount + fielddesc(0-based)
	nfs, err := e.client.WaCmd(e.connID, "colcount", strconv.Itoa(wa))
	if err != nil {
		e.client.WaCmd(e.connID, "close", strconv.Itoa(wa))
		return 0, err
	}
	ncols, _ := strconv.Atoi(waStr(nfs, 0))
	if ncols <= 0 || ncols > 1024 {
		e.client.WaCmd(e.connID, "close", strconv.Itoa(wa))
		return 0, fmt.Errorf("openRemoteWA: colcount invalido %d", ncols)
	}

	cols := make([]dbapiColMeta, 0, ncols)
	idx := make(map[string]int, ncols)
	for i := 0; i < ncols; i++ {
		dfs, err := e.client.WaCmd(e.connID, "fielddesc", strconv.Itoa(wa), strconv.Itoa(i))
		if err != nil || len(dfs) < 5 {
			break
		}
		sz, _ := strconv.Atoi(waStr(dfs, 2))
		dc, _ := strconv.Atoi(waStr(dfs, 3))
		fl, _ := strconv.Atoi(waStr(dfs, 4))
		name := waStr(dfs, 0)
		cols = append(cols, dbapiColMeta{
			name: strings.ToUpper(name), typ: strings.ToUpper(waStr(dfs, 1)),
			size: sz, dec: dc, flags: fl,
		})
		idx[strings.ToUpper(name)] = i + 1 // GetFieldN é 1-based
	}
	if len(cols) == 0 {
		e.client.WaCmd(e.connID, "close", strconv.Itoa(wa))
		return 0, fmt.Errorf("openRemoteWA: sem colunas")
	}

	e.waCols = cols
	e.waIdx = idx
	e.waRecCount = -1
	e.waSeek = 0
	e.columns = make([]columnInfo, len(cols))
	for i, cm := range cols {
		e.columns[i] = columnInfo{name: cm.name, sqlType: dbapiTypeToSQLType(cm.typ)}
	}
	e.records = nil
	e.current = -1
	return wa, nil
}

func (e *DbapiSQLEngine) closeRemoteWA() {
	if e.waRemote > 0 && e.client != nil {
		e.client.WaCmd(e.connID, "close", strconv.Itoa(e.waRemote))
	}
	e.waRemote = 0
	if e.waSeek > 0 && e.client != nil {
		e.client.WaCmd(e.connID, "close", strconv.Itoa(e.waSeek))
	}
	e.waSeek = 0
	e.waCols = nil
	e.waIdx = nil
	e.waRecCount = -1
	e.waAppending = false
}

// waNav executa comando de navegação remota e devolve o valor string.
func (e *DbapiSQLEngine) waNav(cmd string, extra ...string) (string, error) {
	args := append([]string{strconv.Itoa(e.waRemote)}, extra...)
	fs, err := e.client.WaCmd(e.connID, cmd, args...)
	if err != nil {
		return "", err
	}
	return waStr(fs, 0), nil
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
				// GetFieldN devolve o campo + resto do registro concatenado;
				// fatia na largura declarada (ver dbapiFieldSlice).
				row[cm.name] = dbapiFieldSliceT(string(r.vals[i].data), cm)
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
	if e.waRemote > 0 {
		// tenta seek indexado numa workarea auxiliar: SetKey muda a
		// ordem de navegação da wa, então seek2 roda numa wa separada
		// (mesmo arquivo, índice 1) e o recno encontrado volta para a
		// wa principal via GoTo. Seek2 é soft seek — verifica o
		// registro corrente; match -> O(1); senão cai no scan.
		if rec, ok := e.waSeekIndex(key); ok {
			if _, err := e.waNav("goto", rec); err == nil {
				if match, err := e.waSeekMatch(key); err == nil && match {
					return true, nil
				}
			}
		}
		// fallback: scan server-side (WACMD find) — compara key com
		// todos os campos dentro do bridge, sem roundtrip por campo.
		// Devolve recno>=0 se achou (wa já posicionada), -1 se não.
		fs, err := e.client.WaCmd(e.connID, "find",
			strconv.Itoa(e.waRemote), key)
		if err != nil {
			return false, err
		}
		rec, _ := strconv.ParseInt(waStr(fs, 0), 10, 64)
		return rec >= 0, nil
	}
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

// waSeekIndex abre (uma vez) uma workarea auxiliar com o índice 1
// ativo, executa Seek2 e devolve o recno posicionado. Abre o arquivo
// pelo alias — OpenFile resolve o mesmo nome usado em openRemoteWA.
func (e *DbapiSQLEngine) waSeekIndex(key string) (string, bool) {
	if e.waSeek <= 0 {
		fs, err := e.client.WaCmd(e.connID, "open", e.alias)
		if err != nil || len(fs) == 0 {
			return "", false
		}
		wa, err := strconv.Atoi(waStr(fs, 0))
		if err != nil || wa <= 0 {
			return "", false
		}
		e.client.WaCmd(e.connID, "setkey", strconv.Itoa(wa), "1")
		e.waSeek = wa
	}
	fs, err := e.client.WaCmd(e.connID, "seek2",
		strconv.Itoa(e.waSeek), key)
	if err != nil || len(fs) < 2 {
		return "", false
	}
	if rc, _ := strconv.Atoi(waStr(fs, 0)); rc != 0 {
		return "", false
	}
	return waStr(fs, 1), true
}

// waSeekMatch compara key com todos os campos do registro corrente da
// workarea remota (mesmo critério "qualquer campo" do Seek em memória).
func (e *DbapiSQLEngine) waSeekMatch(key string) (bool, error) {
	for i, cm := range e.waCols {
		fs, err := e.client.WaCmd(e.connID, "fieldn",
			strconv.Itoa(e.waRemote), strconv.Itoa(i+1))
		if err != nil {
			return false, err
		}
		if len(fs) == 0 || fs[0].isNull {
			continue
		}
		v := dbapiFieldSliceT(string(fs[0].data), cm)
		if v == key {
			return true, nil
		}
	}
	return false, nil
}

func (e *DbapiSQLEngine) Skip(count int) error {
	if e.waRemote > 0 {
		_, err := e.waNav("skip", strconv.Itoa(count))
		return err
	}
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
	if e.waRemote > 0 {
		_, err := e.waNav("gotop")
		e.current = 0
		return err
	}
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.current = 0
	return nil
}

func (e *DbapiSQLEngine) GoBottom() error {
	if e.waRemote > 0 {
		_, err := e.waNav("gobottom")
		e.current = e.RecCount() - 1
		return err
	}
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

func (e *DbapiSQLEngine) EOF() bool {
	if e.waRemote > 0 {
		v, err := e.waNav("eof")
		return err == nil && v == "1"
	}
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current >= len(e.records)
}

func (e *DbapiSQLEngine) BOF() bool {
	if e.waRemote > 0 {
		v, err := e.waNav("bof")
		return err == nil && v == "1"
	}
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current < 0
}

func (e *DbapiSQLEngine) FieldGet(field string) (advplrt.Value, error) {
	if e.waRemote > 0 {
		return e.waFieldGet(field)
	}
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

// waFieldGet lê um campo da workarea remota: resolve o índice 1-based
// pelo mapa de colunas, chama fieldn e fatia na largura declarada.
func (e *DbapiSQLEngine) waFieldGet(field string) (advplrt.Value, error) {
	name := strings.ToUpper(field)
	idx, ok := e.waIdx[name]
	if !ok {
		return advplrt.Nil, nil
	}
	fs, err := e.client.WaCmd(e.connID, "fieldn",
		strconv.Itoa(e.waRemote), strconv.Itoa(idx))
	if err != nil {
		return advplrt.Nil, err
	}
	if len(fs) == 0 || fs[0].isNull {
		return advplrt.Nil, nil
	}
	cm := e.waCols[idx-1]
	s := dbapiFieldSliceT(string(fs[0].data), cm)
	return remoteValue(s, dbapiTypeToSQLType(cm.typ)), nil
}

// waFieldStr converte um advplrt.Value para o texto que SetFieldN
// espera. Nil -> "" (limpa o campo). ponytail: DateValue sai como
// "dd/mm/aaaa" e BoolValue como ".T."/".F." — se o dbapi exigir outro
// formato para D/L, ajustar aqui quando houver base com esses tipos.
func waFieldStr(val advplrt.Value) string {
	if val == nil {
		return ""
	}
	if _, ok := val.(*advplrt.NilValue); ok {
		return ""
	}
	return val.String()
}

func (e *DbapiSQLEngine) FieldPut(field string, val advplrt.Value) error {
	if e.waRemote > 0 {
		// workarea remota: SetFieldN grava no buffer do record corrente;
		// a persistência acontece no MsUnlock (Update ou Write se Append).
		name := strings.ToUpper(field)
		idx, ok := e.waIdx[name]
		if !ok {
			return fmt.Errorf("FieldPut: campo %s inexistente em %s", name, e.alias)
		}
		_, err := e.client.WaCmd(e.connID, "setfieldn",
			strconv.Itoa(e.waRemote), strconv.Itoa(idx), waFieldStr(val))
		return err
	}
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
	if e.waRemote > 0 {
		// lock real no dbAccess (tTOPClient::Lock no recno corrente)
		if e.isLocked {
			return fmt.Errorf("RecLock: record already locked")
		}
		rec, err := e.waNav("recno")
		if err != nil {
			return err
		}
		if _, err := e.client.WaCmd(e.connID, "lock",
			strconv.Itoa(e.waRemote), rec); err != nil {
			return err
		}
		e.isLocked = true
		return nil
	}
	if e.waRemote == 0 {
		e.recordsMutex.RLock()
		if e.current < 0 || e.current >= len(e.records) {
			e.recordsMutex.RUnlock()
			return fmt.Errorf("RecLock: no current record")
		}
		e.recordsMutex.RUnlock()
	}
	if e.isLocked {
		return fmt.Errorf("RecLock: record already locked")
	}
	getTableLock(e.alias).Lock()
	e.isLocked = true
	return nil
}

// MsUnlock: em workarea remota grava o buffer (Update no recno, ou
// Write se veio de Append) e libera o lock real. No modo memória
// mantém o comportamento anterior (só libera o lock em processo).
func (e *DbapiSQLEngine) MsUnlock() error {
	if e.waRemote > 0 {
		if !e.isLocked && !e.waAppending {
			return nil
		}
		cmd := "update"
		if e.waAppending {
			cmd = "write"
			e.waAppending = false
			e.waRecCount = -1 // cache de COUNT(*) ficou stale
		}
		if _, err := e.client.WaCmd(e.connID, cmd, strconv.Itoa(e.waRemote)); err != nil {
			e.isLocked = false
			return err
		}
		rec, _ := e.waNav("recno")
		e.client.WaCmd(e.connID, "unlock", strconv.Itoa(e.waRemote), rec)
		e.isLocked = false
		return nil
	}
	if !e.isLocked {
		return nil
	}
	e.isLocked = false
	getTableLock(e.alias).Unlock()
	return nil
}

// Append posiciona a workarea remota além do último registro; a
// gravação acontece no MsUnlock via Write (flag waAppending).
func (e *DbapiSQLEngine) Append() error {
	if e.waRemote > 0 {
		if _, err := e.client.WaCmd(e.connID, "append", strconv.Itoa(e.waRemote)); err != nil {
			return err
		}
		e.waAppending = true
		return nil
	}
	return fmt.Errorf("DBACCESS: DbAppend em modo memória ainda não suportado")
}

func (e *DbapiSQLEngine) FieldPos(field string) int {
	field = strings.ToUpper(field)
	if e.waRemote > 0 {
		if idx, ok := e.waIdx[field]; ok {
			return idx
		}
		return 0
	}
	for i, c := range e.columns {
		if c.name == field {
			return i + 1
		}
	}
	return 0
}

func (e *DbapiSQLEngine) RecCount() int {
	if e.waRemote > 0 {
		// ponytail: RecCount(wa) do dbapi devolve sentinela (~1e9) em
		// workarea ISAM — medido em SX2990. COUNT(*) via SQL é a fonte
		// confiável; cacheado por SelectArea (stale se a tabela mudar
		// durante a sessão — aceitável nesta fatia).
		if e.waRecCount < 0 {
			rows, err := e.QueryRows("SELECT COUNT(*) AS C FROM " + e.alias)
			if err == nil && len(rows) > 0 {
				n, _ := strconv.Atoi(strings.TrimSpace(rows[0]["C"]))
				e.waRecCount = n
			} else {
				e.waRecCount = 0
			}
		}
		return e.waRecCount
	}
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return len(e.records)
}

func (e *DbapiSQLEngine) RecNo() int {
	if e.waRemote > 0 {
		v, err := e.waNav("recno")
		if err != nil {
			return 0
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return int(n)
	}
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
		e.closeRemoteWA()
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
