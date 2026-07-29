package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tools/shared"
)

// identRe validates identifier names (table names, column names, etc)
// to prevent SQL injection. Only allows alphanumeric chars and underscore.
var identRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type columnInfo struct {
	name    string
	sqlType string
}

// Global table locks (per-table semaphores) to implement RecLock/MsUnlock
// CWE-362: Race Condition prevention
var (
	tableLocksMu sync.Mutex
	tableLocks   = make(map[string]*sync.Mutex)
)

// getTableLock returns or creates a per-table mutex for concurrent record access
func getTableLock(table string) *sync.Mutex {
	tableLocksMu.Lock()
	defer tableLocksMu.Unlock()

	if _, exists := tableLocks[table]; !exists {
		tableLocks[table] = &sync.Mutex{}
	}
	return tableLocks[table]
}

type SQLiteEngine struct {
	db           *sql.DB
	alias        string
	columns      []columnInfo
	records      []map[string]advplrt.Value
	current      int
	isLocked     bool        // whether RecLock was called (lock held)
	recordsMutex sync.RWMutex // protect concurrent access to records slice
}

func NewSQLiteEngine(dbPath string) (*SQLiteEngine, error) {
	db, err := shared.OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}

	return &SQLiteEngine{
		db:      db,
		records: make([]map[string]advplrt.Value, 0),
		current: -1,
	}, nil
}

// loadColumns lê a estrutura física da tabela (nome + tipo declarado de
// cada coluna, na ordem real) — base pra Append (valores em branco
// tipo-apropriados), MsUnlock (UPDATE coluna a coluna) e FieldPos.
func (e *SQLiteEngine) loadColumns() error {
	rows, err := e.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", e.alias))
	if err != nil {
		return err
	}
	defer rows.Close()

	e.columns = nil
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		e.columns = append(e.columns, columnInfo{name: strings.ToUpper(name), sqlType: strings.ToUpper(ctype)})
	}
	return rows.Err()
}

func (e *SQLiteEngine) SelectArea(alias string) error {
	e.alias = strings.ToUpper(alias)

	// Validate alias to prevent SQL injection (CWE-89, OWASP A03:2021)
	if !identRe.MatchString(e.alias) {
		return fmt.Errorf("invalid table name: %q", e.alias)
	}

	if err := e.loadColumns(); err != nil {
		return err
	}

	// Query the table structure
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 0", e.alias)
	rows, err := e.db.Query(query)
	if err != nil {
		return fmt.Errorf("table %s not found: %v", e.alias, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Load all records
	query = fmt.Sprintf("SELECT * FROM %s", e.alias)
	rows, err = e.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	e.records = make([]map[string]advplrt.Value, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		record := make(map[string]advplrt.Value)
		for i, col := range columns {
			record[strings.ToUpper(col)] = convertDBValue(values[i])
		}
		e.records = append(e.records, record)
	}

	e.current = 0
	return nil
}

func (e *SQLiteEngine) Seek(key string) (bool, error) {
	if len(e.records) == 0 {
		return false, nil
	}

	// Simple linear search - in real Protheus this would use indexes
	for i, record := range e.records {
		// Check first field as key
		for _, val := range record {
			if fmt.Sprintf("%v", val) == key {
				e.current = i
				return true, nil
			}
		}
	}

	return false, nil
}

// Skip moves the record pointer by count records
// Thread-safe via read-write lock (CWE-362)
func (e *SQLiteEngine) Skip(count int) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()

	if len(e.records) == 0 {
		return nil
	}

	e.current += count
	if e.current < 0 {
		e.current = 0
	}
	if e.current >= len(e.records) {
		e.current = len(e.records) - 1
	}

	return nil
}

// GoTop moves the record pointer to the first record
// Thread-safe via read-write lock (CWE-362)
func (e *SQLiteEngine) GoTop() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()

	e.current = 0
	return nil
}

// GoBottom moves the record pointer to the last record
// Thread-safe via read-write lock (CWE-362)
func (e *SQLiteEngine) GoBottom() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()

	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

// EOF returns true if record pointer is past end of records
// Thread-safe via read-only lock (CWE-362)
func (e *SQLiteEngine) EOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()

	return e.current >= len(e.records)
}

// BOF returns true if record pointer is before beginning of records
// Thread-safe via read-only lock (CWE-362)
func (e *SQLiteEngine) BOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()

	return e.current < 0
}

// FieldGet safely retrieves a field value from the current record
// Uses read-write lock for thread-safe concurrent access (CWE-362)
func (e *SQLiteEngine) FieldGet(field string) (advplrt.Value, error) {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()

	if e.current < 0 || e.current >= len(e.records) {
		return advplrt.Nil, nil
	}

	field = strings.ToUpper(field)
	if val, ok := e.records[e.current][field]; ok {
		return val, nil
	}

	return advplrt.Nil, nil
}

// FieldPut safely sets a field value in the current record
// Must be preceded by RecLock() to prevent concurrent modifications (CWE-362)
// Uses read-write lock for thread-safe concurrent access
func (e *SQLiteEngine) FieldPut(field string, val advplrt.Value) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()

	if e.current < 0 || e.current >= len(e.records) {
		return fmt.Errorf("no current record")
	}

	field = strings.ToUpper(field)
	e.records[e.current][field] = val
	return nil
}

// RecLock locks the current record in the current work area.
// Implements per-table semaphore to prevent concurrent modifications (CWE-362: Race Condition).
// Returns error if no record is current or already locked.
func (e *SQLiteEngine) RecLock() error {
	// Validate that we have a current record
	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		return fmt.Errorf("RecLock: no current record")
	}
	e.recordsMutex.RUnlock()

	if e.isLocked {
		return fmt.Errorf("RecLock: record already locked")
	}

	// Acquire table-level lock to serialize access to this table
	tableLock := getTableLock(e.alias)
	tableLock.Lock()
	e.isLocked = true
	return nil
}

// MsUnlock releases the record lock and persists changes to the database via UPDATE.
// Completes the DbAppend/RecLock -> FieldPut (via alias->campo) -> MsUnlock cycle.
// Before this fix, it was a no-op: all mutations via FieldPut stayed in memory and were lost.
// Now: atomic UPDATE writes changes to disk; lock is released for other work areas.
// CWE-362: Race Condition prevention via per-table semaphore
func (e *SQLiteEngine) MsUnlock() error {
	if !e.isLocked {
		// Not locked, nothing to unlock
		return nil
	}

	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		e.isLocked = false

		// Release table-level lock
		tableLock := getTableLock(e.alias)
		tableLock.Unlock()

		return nil
	}
	record := e.records[e.current]
	e.recordsMutex.RUnlock()

	recno, ok := record["R_E_C_N_O_"]
	if !ok {
		e.isLocked = false

		// Release table-level lock
		tableLock := getTableLock(e.alias)
		tableLock.Unlock()

		return fmt.Errorf("MsUnlock: registro sem R_E_C_N_O_")
	}

	var setClauses []string
	var vals []any
	for _, c := range e.columns {
		if c.name == "R_E_C_N_O_" {
			continue
		}
		setClauses = append(setClauses, c.name+" = ?")
		vals = append(vals, valueToSQL(record[c.name]))
	}
	vals = append(vals, valueToSQL(recno))

	query := fmt.Sprintf("UPDATE %s SET %s WHERE R_E_C_N_O_ = ?", e.alias, strings.Join(setClauses, ", "))
	_, err := e.db.Exec(query, vals...)

	// Always release lock, even if UPDATE failed
	e.isLocked = false
	tableLock := getTableLock(e.alias)
	tableLock.Unlock()

	return err
}

// valueToSQL converte um advplrt.Value de volta pro tipo Go que o driver
// SQL espera — inverso de convertDBValue.
func valueToSQL(v advplrt.Value) any {
	if v == nil {
		return nil
	}
	switch v.Type() {
	case "N":
		return v.(*advplrt.NumberValue).Val
	case "C", "M":
		return v.(*advplrt.StringValue).Val
	case "L":
		if v.(*advplrt.BoolValue).Val {
			return 1
		}
		return 0
	default:
		return v.String()
	}
}

// Append inserts a blank record (with type-appropriate default values)
// and positions to it — DbAppend() in AdvPL.
// Thread-safe via write lock (CWE-362)
// Before this fix, it was a no-op: RecCount() didn't change.
func (e *SQLiteEngine) Append() error {
	if e.alias == "" || len(e.columns) == 0 {
		return fmt.Errorf("DbAppend: nenhuma área selecionada")
	}

	var cols []string
	var placeholders []string
	var vals []any
	blank := make(map[string]advplrt.Value)
	for _, c := range e.columns {
		if c.name == "R_E_C_N_O_" {
			continue
		}
		var v any
		switch {
		case c.name == "D_E_L_E_T_":
			v = " "
			blank[c.name] = advplrt.NewString(" ")
		case c.name == "R_E_C_D_E_L_":
			v = 0
			blank[c.name] = advplrt.NewNumber(0)
		case strings.Contains(c.sqlType, "INT") || strings.Contains(c.sqlType, "REAL") || strings.Contains(c.sqlType, "NUM"):
			v = 0
			blank[c.name] = advplrt.NewNumber(0)
		default:
			v = ""
			blank[c.name] = advplrt.NewString("")
		}
		cols = append(cols, c.name)
		placeholders = append(placeholders, "?")
		vals = append(vals, v)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", e.alias, strings.Join(cols, ","), strings.Join(placeholders, ","))
	res, err := e.db.Exec(query, vals...)
	if err != nil {
		return err
	}
	newRecno, err := res.LastInsertId()
	if err != nil {
		return err
	}
	blank["R_E_C_N_O_"] = advplrt.NewNumber(float64(newRecno))

	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()

	e.records = append(e.records, blank)
	e.current = len(e.records) - 1
	return nil
}

// FieldPos devolve a posição 1-based da coluna física — 0 se não existir.
// Antes desta correção era um stub, sempre devolvia 0.
func (e *SQLiteEngine) FieldPos(field string) int {
	field = strings.ToUpper(field)
	for i, c := range e.columns {
		if c.name == field {
			return i + 1
		}
	}
	return 0
}

// RecCount returns the total number of records in the current work area
// Thread-safe via read-only lock (CWE-362)
func (e *SQLiteEngine) RecCount() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()

	return len(e.records)
}

// RecNo returns the current record number (1-based)
// Thread-safe via read-only lock (CWE-362)
func (e *SQLiteEngine) RecNo() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()

	return e.current + 1
}

// QueryRows executa SQL direto e devolve as linhas como mapas coluna→string
// (chaves em maiúsculas) — extensão vm.SQLEngine usada pelo FWMBrowse.
func (e *SQLiteEngine) QueryRows(query string, args ...any) ([]map[string]string, error) {
	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []map[string]string{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range columns {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		record := map[string]string{}
		for i, col := range columns {
			if values[i] == nil {
				record[strings.ToUpper(col)] = ""
			} else {
				record[strings.ToUpper(col)] = fmt.Sprintf("%v", values[i])
			}
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

// Exec executa um comando SQL (INSERT/UPDATE/DELETE) — extensão vm.SQLEngine.
func (e *SQLiteEngine) Exec(query string, args ...any) error {
	_, err := e.db.Exec(query, args...)
	return err
}

func (e *SQLiteEngine) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func convertDBValue(value interface{}) advplrt.Value {
	if value == nil {
		return advplrt.Nil
	}

	switch v := value.(type) {
	case int:
		return advplrt.NewNumber(float64(v))
	case int64:
		return advplrt.NewNumber(float64(v))
	case float64:
		return advplrt.NewNumber(v)
	case string:
		return advplrt.NewString(v)
	case bool:
		return advplrt.NewBool(v)
	default:
		return advplrt.NewString(fmt.Sprintf("%v", v))
	}
}
