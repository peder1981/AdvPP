package db

import (
	"database/sql"
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteEngine struct {
	db      *sql.DB
	alias   string
	records []map[string]advplrt.Value
	current int
}

func NewSQLiteEngine(dbPath string) (*SQLiteEngine, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	return &SQLiteEngine{
		db:      db,
		records: make([]map[string]advplrt.Value, 0),
		current: -1,
	}, nil
}

func (e *SQLiteEngine) SelectArea(alias string) error {
	e.alias = strings.ToUpper(alias)
	
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

func (e *SQLiteEngine) MsUnlock() error {
	// In a real implementation, this would unlock the record
	return nil
}

func (e *SQLiteEngine) RecCount() int {
	return len(e.records)
}

func (e *SQLiteEngine) RecNo() int {
	return e.current + 1
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
