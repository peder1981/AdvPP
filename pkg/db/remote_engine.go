package db

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// RemoteSQLEngine implementa DBEngine + SQLEngine (por duck typing, igual
// SQLiteEngine) sobre um *sql.DB real (Postgres/Oracle/MSSQL). Mesmo
// modelo do SQLiteEngine: SelectArea carrega TODAS as linhas em memória e
// a navegação (Skip/GoTop/...) opera sobre esse slice, não sobre um cursor
// de banco — RecLock/MsUnlock usam o mesmo mutex por tabela do pacote
// (getTableLock), sem lock real do banco (mesma limitação honesta do
// SQLiteEngine).
type RemoteSQLEngine struct {
	db           *sql.DB
	dialect      Dialect
	alias        string
	columns      []columnInfo
	records      []map[string]advplrt.Value
	current      int
	isLocked     bool
	recordsMutex sync.RWMutex
}

func NewRemoteSQLEngine(sqlDB *sql.DB, dialect Dialect) *RemoteSQLEngine {
	return &RemoteSQLEngine{db: sqlDB, dialect: dialect, current: -1}
}

func (e *RemoteSQLEngine) SelectArea(alias string) error {
	e.alias = strings.ToUpper(alias)
	if !identRe.MatchString(e.alias) {
		return fmt.Errorf("invalid table name: %q", e.alias)
	}

	rows, err := e.db.Query(fmt.Sprintf("SELECT * FROM %s WHERE 1=0", e.alias))
	if err != nil {
		return fmt.Errorf("table %s not found: %v", e.alias, err)
	}
	cols, err := rows.Columns()
	rows.Close()
	if err != nil {
		return err
	}

	e.columns = nil
	for _, c := range cols {
		e.columns = append(e.columns, columnInfo{name: strings.ToUpper(c)})
	}

	rows, err = e.db.Query(fmt.Sprintf("SELECT * FROM %s", e.alias))
	if err != nil {
		return err
	}
	defer rows.Close()

	e.records = make([]map[string]advplrt.Value, 0)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		record := make(map[string]advplrt.Value)
		for i, c := range cols {
			record[strings.ToUpper(c)] = convertDBValue(values[i])
		}
		e.records = append(e.records, record)
	}
	e.current = 0
	return rows.Err()
}

func (e *RemoteSQLEngine) Seek(key string) (bool, error) {
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

func (e *RemoteSQLEngine) Skip(count int) error {
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

func (e *RemoteSQLEngine) GoTop() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.current = 0
	return nil
}

func (e *RemoteSQLEngine) GoBottom() error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

func (e *RemoteSQLEngine) EOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current >= len(e.records)
}

func (e *RemoteSQLEngine) BOF() bool {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return e.current < 0
}

func (e *RemoteSQLEngine) FieldGet(field string) (advplrt.Value, error) {
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

func (e *RemoteSQLEngine) FieldPut(field string, val advplrt.Value) error {
	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	if e.current < 0 || e.current >= len(e.records) {
		return fmt.Errorf("no current record")
	}
	e.records[e.current][strings.ToUpper(field)] = val
	return nil
}

func (e *RemoteSQLEngine) RecLock() error {
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

func (e *RemoteSQLEngine) MsUnlock() error {
	if !e.isLocked {
		return nil
	}
	defer func() {
		e.isLocked = false
		getTableLock(e.alias).Unlock()
	}()

	e.recordsMutex.RLock()
	if e.current < 0 || e.current >= len(e.records) {
		e.recordsMutex.RUnlock()
		return nil
	}
	record := e.records[e.current]
	e.recordsMutex.RUnlock()

	recno, ok := record["R_E_C_N_O_"]
	if !ok {
		return fmt.Errorf("MsUnlock: registro sem R_E_C_N_O_")
	}

	var setClauses []string
	var vals []any
	pos := 1
	for _, c := range e.columns {
		if c.name == "R_E_C_N_O_" {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", c.name, e.dialect.Placeholder(pos)))
		vals = append(vals, valueToSQL(record[c.name]))
		pos++
	}
	vals = append(vals, valueToSQL(recno))
	query := fmt.Sprintf("UPDATE %s SET %s WHERE R_E_C_N_O_ = %s",
		e.alias, strings.Join(setClauses, ", "), e.dialect.Placeholder(pos))
	_, err := e.db.Exec(query, vals...)
	return err
}

// Append calcula R_E_C_N_O_ no cliente (max atual + 1) em vez de depender
// de LastInsertId()/RETURNING — ver nota de design da Task 6 do plano.
//
// Sem lock de banco real (mesma limitação honesta do resto do arquivo), o
// cálculo client-side do recno é vulnerável a TOCTOU entre duas chamadas
// concorrentes de Append() na mesma tabela: ambas podem ler o mesmo
// max(R_E_C_N_O_) antes que a primeira termine o INSERT, gerando recno
// duplicado no banco remoto. getTableLock(e.alias) — o mesmo mutex por
// tabela usado por RecLock/MsUnlock — serializa TODA a seção crítica
// (scan → INSERT → append em e.records), não só o acesso ao slice em
// memória, para que só uma chamada de Append() por tabela esteja em
// voo por vez.
func (e *RemoteSQLEngine) Append() error {
	if e.alias == "" || len(e.columns) == 0 {
		return fmt.Errorf("DbAppend: nenhuma área selecionada")
	}

	tableLock := getTableLock(e.alias)
	tableLock.Lock()
	defer tableLock.Unlock()

	e.recordsMutex.RLock()
	var maxRecno float64
	for _, r := range e.records {
		if n, ok := r["R_E_C_N_O_"].(*advplrt.NumberValue); ok && n.Val > maxRecno {
			maxRecno = n.Val
		}
	}
	newRecno := maxRecno + 1
	e.recordsMutex.RUnlock()

	var cols []string
	var placeholders []string
	var vals []any
	blank := make(map[string]advplrt.Value)
	pos := 1
	for _, c := range e.columns {
		switch c.name {
		case "R_E_C_N_O_":
			cols = append(cols, c.name)
			placeholders = append(placeholders, e.dialect.Placeholder(pos))
			vals = append(vals, newRecno)
			blank[c.name] = advplrt.NewNumber(newRecno)
			pos++
			continue
		case "D_E_L_E_T_":
			blank[c.name] = advplrt.NewString(" ")
			vals = append(vals, " ")
		default:
			blank[c.name] = advplrt.NewString("")
			vals = append(vals, "")
		}
		cols = append(cols, c.name)
		placeholders = append(placeholders, e.dialect.Placeholder(pos))
		pos++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", e.alias, strings.Join(cols, ","), strings.Join(placeholders, ","))
	if _, err := e.db.Exec(query, vals...); err != nil {
		return err
	}

	e.recordsMutex.Lock()
	defer e.recordsMutex.Unlock()
	e.records = append(e.records, blank)
	e.current = len(e.records) - 1
	return nil
}

func (e *RemoteSQLEngine) FieldPos(field string) int {
	field = strings.ToUpper(field)
	for i, c := range e.columns {
		if c.name == field {
			return i + 1
		}
	}
	return 0
}

func (e *RemoteSQLEngine) RecCount() int {
	e.recordsMutex.RLock()
	defer e.recordsMutex.RUnlock()
	return len(e.records)
}

func (e *RemoteSQLEngine) RecNo() int {
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

func (e *RemoteSQLEngine) QueryRows(query string, args ...any) ([]map[string]string, error) {
	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]string
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]string)
		for i, c := range cols {
			row[strings.ToUpper(c)] = convertDBValue(values[i]).String()
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *RemoteSQLEngine) Exec(query string, args ...any) error {
	_, err := e.db.Exec(query, args...)
	return err
}
