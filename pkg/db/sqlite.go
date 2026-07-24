package db

import (
	"database/sql"
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/advpl/compiler/pkg/tools/shared"
)

type columnInfo struct {
	name    string
	sqlType string
}

type SQLiteEngine struct {
	db      *sql.DB
	alias   string
	columns []columnInfo
	records []map[string]advplrt.Value
	current int
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

func (e *SQLiteEngine) Skip(count int) error {
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

func (e *SQLiteEngine) GoTop() error {
	e.current = 0
	return nil
}

func (e *SQLiteEngine) GoBottom() error {
	if len(e.records) > 0 {
		e.current = len(e.records) - 1
	}
	return nil
}

func (e *SQLiteEngine) EOF() bool {
	return e.current >= len(e.records)
}

func (e *SQLiteEngine) BOF() bool {
	return e.current < 0
}

func (e *SQLiteEngine) FieldGet(field string) (advplrt.Value, error) {
	if e.current < 0 || e.current >= len(e.records) {
		return advplrt.Nil, nil
	}

	field = strings.ToUpper(field)
	if val, ok := e.records[e.current][field]; ok {
		return val, nil
	}

	return advplrt.Nil, nil
}

func (e *SQLiteEngine) FieldPut(field string, val advplrt.Value) error {
	if e.current < 0 || e.current >= len(e.records) {
		return fmt.Errorf("no current record")
	}

	field = strings.ToUpper(field)
	e.records[e.current][field] = val
	return nil
}

func (e *SQLiteEngine) RecLock() error {
	// In a real implementation, this would lock the record
	return nil
}

// MsUnlock grava o registro corrente no banco via UPDATE — fecha o ciclo
// DbAppend/RecLock -> FieldPut (via alias->campo) -> MsUnlock. Antes desta
// correção, era um no-op: toda mutação via FieldPut ficava só em memória e
// se perdia ao fechar o processo.
func (e *SQLiteEngine) MsUnlock() error {
	if e.current < 0 || e.current >= len(e.records) {
		return nil
	}
	record := e.records[e.current]
	recno, ok := record["R_E_C_N_O_"]
	if !ok {
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

// Append insere um registro em branco de verdade (valores tipo-apropriados
// pela coluna: numérico 0, texto "") e posiciona nele — DbAppend() no
// AdvPL. Antes desta correção era um no-op: RecCount() não mudava.
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

func (e *SQLiteEngine) RecCount() int {
	return len(e.records)
}

func (e *SQLiteEngine) RecNo() int {
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
