package shared

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// FieldType representa os tipos de campos AdvPL
type FieldType string

const (
	FieldTypeChar   FieldType = "C"
	FieldTypeNum    FieldType = "N"
	FieldTypeDate   FieldType = "D"
	FieldTypeLog    FieldType = "L"
	FieldTypeMemo   FieldType = "M"
	FieldTypeDouble FieldType = "B"
)

// Field representa um campo de tabela
type Field struct {
	Name     string
	Type     FieldType
	Size     int
	Decimal  int
	Picture  string
	Help     string
	Required bool
}

// Record representa um registro de dados
type Record struct {
	Recno   int
	Fields  map[string]interface{}
	Deleted bool
}

// Index representa um índice
type Index struct {
	Name       string
	Expression string
	Filter     string
	Unique     bool
	Descending bool
}

// DatabaseDriver interface para abstração de banco de dados
type DatabaseDriver interface {
	Open(file string, readOnly, shared bool) error
	Close() error
	GetStructure() ([]Field, error)
	GetData(offset, limit int) ([]Record, error)
	GetRecord(recno int) (*Record, error)
	AddRecord(record Record) (int, error)
	UpdateRecord(recno int, record Record) error
	DeleteRecord(recno int) error
	RecallRecord(recno int) error
	Pack() error
	Zap() error
	GetIndexes() ([]Index, error)
	CreateIndex(name string, expression string) error
	DropIndex(name string) error
	SetOrder(indexName string) error
	GoTop() error
	GoBottom() error
	Skip(n int) error
	Seek(key interface{}) (bool, error)
	Locate(filter string) (bool, error)
	Count() (int, error)
	Sum(fieldName string) (float64, error)
	GetFileName() string
	GetAlias() string
	IsReadOnly() bool
	IsShared() bool
}

// TableInfo representa informações de uma tabela aberta
type TableInfo struct {
	File      string
	Alias     string
	Driver    string
	DriverObj DatabaseDriver
	ReadOnly  bool
	Shared    bool
	Structure []Field
	Indexes   []Index
}

// TableManager gerencia múltiplas tabelas abertas
type TableManager struct {
	tables  map[string]*TableInfo
	current string
}

// NewTableManager cria um novo gerenciador de tabelas
func NewTableManager() *TableManager {
	return &TableManager{
		tables: make(map[string]*TableInfo),
	}
}

// OpenTable abre uma tabela
func (tm *TableManager) OpenTable(file, driver string, readOnly, shared bool) (*TableInfo, error) {
	// Verifica se já está aberta
	for alias, info := range tm.tables {
		if info.File == file {
			tm.current = alias
			return info, fmt.Errorf("tabela já está aberta com alias: %s", alias)
		}
	}

	// Cria alias único
	alias := filepath.Base(file)
	ext := filepath.Ext(alias)
	if ext != "" {
		alias = alias[:len(alias)-len(ext)]
	}

	// Garante alias único
	baseAlias := alias
	counter := 1
	for _, exists := tm.tables[alias]; exists; {
		alias = fmt.Sprintf("%s%d", baseAlias, counter)
		counter++
	}

	// Cria driver apropriado
	var dbDriver DatabaseDriver
	switch strings.ToUpper(driver) {
	case "DBF", "DBFCDXADS", "DBFCDXAX":
		dbDriver = NewDBFDriver()
	case "TOPCONN", "TOPCONNECT":
		dbDriver = NewTopConnectDriver()
	case "CTREECDX":
		dbDriver = NewCtreeDriver()
	case "BTVCDX", "BTREIVE":
		dbDriver = NewBTrieveDriver()
	case "SQLITE", "DB":
		dbDriver = NewSQLiteDriver()
	default:
		// Detecta automaticamente se for arquivo .db ou .sqlite
		if strings.HasSuffix(strings.ToLower(file), ".db") || strings.HasSuffix(strings.ToLower(file), ".sqlite") || strings.HasSuffix(strings.ToLower(file), ".sqlite3") {
			dbDriver = NewSQLiteDriver()
		} else {
			return nil, fmt.Errorf("driver não suportado: %s", driver)
		}
	}

	// Abre tabela
	if err := dbDriver.Open(file, readOnly, shared); err != nil {
		return nil, err
	}

	// Obtém estrutura
	structure, err := dbDriver.GetStructure()
	if err != nil {
		dbDriver.Close()
		return nil, err
	}

	// Obtém índices
	indexes, err := dbDriver.GetIndexes()
	if err != nil {
		dbDriver.Close()
		return nil, err
	}

	// Cria info da tabela
	info := &TableInfo{
		File:      file,
		Alias:     alias,
		Driver:    driver,
		DriverObj: dbDriver,
		ReadOnly:  readOnly,
		Shared:    shared,
		Structure: structure,
		Indexes:   indexes,
	}

	tm.tables[alias] = info
	tm.current = alias

	return info, nil
}

// CloseTable fecha uma tabela
func (tm *TableManager) CloseTable(alias string) error {
	_, exists := tm.tables[alias]
	if !exists {
		return fmt.Errorf("tabela não encontrada: %s", alias)
	}

	// Fecha driver
	// TODO: Implementar fechamento do driver

	delete(tm.tables, alias)

	if tm.current == alias {
		tm.current = ""
	}

	return nil
}

// GetCurrentTable retorna a tabela atual
func (tm *TableManager) GetCurrentTable() (*TableInfo, error) {
	if tm.current == "" {
		return nil, fmt.Errorf("nenhuma tabela selecionada")
	}

	info, exists := tm.tables[tm.current]
	if !exists {
		return nil, fmt.Errorf("tabela não encontrada: %s", tm.current)
	}

	return info, nil
}

// SetCurrentTable define a tabela atual
func (tm *TableManager) SetCurrentTable(alias string) error {
	_, exists := tm.tables[alias]
	if !exists {
		return fmt.Errorf("tabela não encontrada: %s", alias)
	}

	tm.current = alias
	return nil
}

// GetTables retorna todas as tabelas abertas
func (tm *TableManager) GetTables() []*TableInfo {
	tables := make([]*TableInfo, 0, len(tm.tables))
	for _, info := range tm.tables {
		tables = append(tables, info)
	}
	return tables
}

// DBFDriver implementa DatabaseDriver para arquivos DBF
type DBFDriver struct {
	file      string
	alias     string
	readOnly  bool
	shared    bool
	structure []Field
	indexes   []Index
	records   []Record
	current   int
}

// NewDBFDriver cria um novo driver DBF
func NewDBFDriver() *DBFDriver {
	return &DBFDriver{
		records: make([]Record, 0),
		current: -1,
	}
}

// Open abre um arquivo DBF
func (d *DBFDriver) Open(file string, readOnly, shared bool) error {
	d.file = file
	d.alias = filepath.Base(file)
	d.readOnly = readOnly
	d.shared = shared

	// Simula estrutura
	d.structure = []Field{
		{Name: "FIELD1", Type: FieldTypeChar, Size: 10, Decimal: 0},
		{Name: "FIELD2", Type: FieldTypeNum, Size: 10, Decimal: 2},
		{Name: "FIELD3", Type: FieldTypeDate, Size: 8, Decimal: 0},
	}

	return nil
}

// Close fecha o arquivo DBF
func (d *DBFDriver) Close() error {
	d.file = ""
	d.alias = ""
	d.records = nil
	d.current = -1
	return nil
}

// GetStructure retorna a estrutura da tabela
func (d *DBFDriver) GetStructure() ([]Field, error) {
	return d.structure, nil
}

// GetData retorna dados da tabela
func (d *DBFDriver) GetData(offset, limit int) ([]Record, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || offset+limit > len(d.records) {
		limit = len(d.records) - offset
	}

	return d.records[offset : offset+limit], nil
}

// GetRecord retorna um registro específico
func (d *DBFDriver) GetRecord(recno int) (*Record, error) {
	if recno < 1 || recno > len(d.records) {
		return nil, fmt.Errorf("recno inválido: %d", recno)
	}
	return &d.records[recno-1], nil
}

// AddRecord adiciona um registro
func (d *DBFDriver) AddRecord(record Record) (int, error) {
	if d.readOnly {
		return 0, fmt.Errorf("tabela é somente leitura")
	}

	record.Recno = len(d.records) + 1
	if record.Fields == nil {
		record.Fields = make(map[string]interface{})
	}

	d.records = append(d.records, record)
	return record.Recno, nil
}

// UpdateRecord atualiza um registro
func (d *DBFDriver) UpdateRecord(recno int, record Record) error {
	if d.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if recno < 1 || recno > len(d.records) {
		return fmt.Errorf("recno inválido: %d", recno)
	}

	record.Recno = recno
	d.records[recno-1] = record
	return nil
}

// DeleteRecord deleta um registro
func (d *DBFDriver) DeleteRecord(recno int) error {
	if d.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if recno < 1 || recno > len(d.records) {
		return fmt.Errorf("recno inválido: %d", recno)
	}

	d.records[recno-1].Deleted = true
	return nil
}

// RecallRecord recupera um registro deletado
func (d *DBFDriver) RecallRecord(recno int) error {
	if d.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if recno < 1 || recno > len(d.records) {
		return fmt.Errorf("recno inválido: %d", recno)
	}

	d.records[recno-1].Deleted = false
	return nil
}

// Pack compacta a tabela
func (d *DBFDriver) Pack() error {
	if d.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	activeRecords := make([]Record, 0)
	for _, rec := range d.records {
		if !rec.Deleted {
			activeRecords = append(activeRecords, rec)
		}
	}

	d.records = activeRecords
	for i := range d.records {
		d.records[i].Recno = i + 1
	}

	return nil
}

// Zap limpa a tabela
func (d *DBFDriver) Zap() error {
	if d.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	d.records = make([]Record, 0)
	d.current = -1
	return nil
}

// GetIndexes retorna os índices
func (d *DBFDriver) GetIndexes() ([]Index, error) {
	return d.indexes, nil
}

// CreateIndex cria um índice
func (d *DBFDriver) CreateIndex(name string, expression string) error {
	d.indexes = append(d.indexes, Index{
		Name:       name,
		Expression: expression,
		Unique:     false,
		Descending: false,
	})
	return nil
}

// DropIndex remove um índice
func (d *DBFDriver) DropIndex(name string) error {
	for i, idx := range d.indexes {
		if idx.Name == name {
			d.indexes = append(d.indexes[:i], d.indexes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("índice não encontrado: %s", name)
}

// SetOrder define o índice ativo
func (d *DBFDriver) SetOrder(indexName string) error {
	return nil
}

// GoTop vai para o topo
func (d *DBFDriver) GoTop() error {
	if len(d.records) > 0 {
		d.current = 0
	}
	return nil
}

// GoBottom vai para o final
func (d *DBFDriver) GoBottom() error {
	if len(d.records) > 0 {
		d.current = len(d.records) - 1
	}
	return nil
}

// Skip pula n registros
func (d *DBFDriver) Skip(n int) error {
	d.current += n
	if d.current < 0 {
		d.current = 0
	}
	if d.current >= len(d.records) {
		d.current = len(d.records)
	}
	return nil
}

// Seek busca por chave
func (d *DBFDriver) Seek(key interface{}) (bool, error) {
	return false, nil
}

// Locate localiza por filtro
func (d *DBFDriver) Locate(filter string) (bool, error) {
	return false, nil
}

// Count conta registros
func (d *DBFDriver) Count() (int, error) {
	count := 0
	for _, rec := range d.records {
		if !rec.Deleted {
			count++
		}
	}
	return count, nil
}

// Sum soma campo
func (d *DBFDriver) Sum(fieldName string) (float64, error) {
	sum := 0.0
	for _, rec := range d.records {
		if !rec.Deleted {
			if val, ok := rec.Fields[fieldName]; ok {
				if num, ok := val.(float64); ok {
					sum += num
				}
			}
		}
	}
	return sum, nil
}

// GetFileName retorna o nome do arquivo
func (d *DBFDriver) GetFileName() string {
	return d.file
}

// GetAlias retorna o alias
func (d *DBFDriver) GetAlias() string {
	return d.alias
}

// IsReadOnly retorna se é somente leitura
func (d *DBFDriver) IsReadOnly() bool {
	return d.readOnly
}

// IsShared retorna se é compartilhado
func (d *DBFDriver) IsShared() bool {
	return d.shared
}

// TopConnectDriver implementa DatabaseDriver para TopConnect
type TopConnectDriver struct {
	file     string
	alias    string
	readOnly bool
	shared   bool
}

// NewTopConnectDriver cria um novo driver TopConnect
func NewTopConnectDriver() *TopConnectDriver {
	return &TopConnectDriver{}
}

// Open abre uma conexão TopConnect
func (t *TopConnectDriver) Open(file string, readOnly, shared bool) error {
	t.file = file
	t.alias = file
	t.readOnly = readOnly
	t.shared = shared
	return nil
}

// Close fecha a conexão
func (t *TopConnectDriver) Close() error {
	return nil
}

// GetStructure retorna a estrutura
func (t *TopConnectDriver) GetStructure() ([]Field, error) {
	return []Field{}, nil
}

// GetData retorna dados
func (t *TopConnectDriver) GetData(offset, limit int) ([]Record, error) {
	return []Record{}, nil
}

// GetRecord retorna um registro
func (t *TopConnectDriver) GetRecord(recno int) (*Record, error) {
	return nil, nil
}

// AddRecord adiciona um registro
func (t *TopConnectDriver) AddRecord(record Record) (int, error) {
	return 0, nil
}

// UpdateRecord atualiza um registro
func (t *TopConnectDriver) UpdateRecord(recno int, record Record) error {
	return nil
}

// DeleteRecord deleta um registro
func (t *TopConnectDriver) DeleteRecord(recno int) error {
	return nil
}

// RecallRecord recupera um registro
func (t *TopConnectDriver) RecallRecord(recno int) error {
	return nil
}

// Pack compacta
func (t *TopConnectDriver) Pack() error {
	return nil
}

// Zap limpa
func (t *TopConnectDriver) Zap() error {
	return nil
}

// GetIndexes retorna índices
func (t *TopConnectDriver) GetIndexes() ([]Index, error) {
	return []Index{}, nil
}

// CreateIndex cria índice
func (t *TopConnectDriver) CreateIndex(name string, expression string) error {
	return nil
}

// DropIndex remove índice
func (t *TopConnectDriver) DropIndex(name string) error {
	return nil
}

// SetOrder define ordem
func (t *TopConnectDriver) SetOrder(indexName string) error {
	return nil
}

// GoTop vai para topo
func (t *TopConnectDriver) GoTop() error {
	return nil
}

// GoBottom vai para final
func (t *TopConnectDriver) GoBottom() error {
	return nil
}

// Skip pula registros
func (t *TopConnectDriver) Skip(n int) error {
	return nil
}

// Seek busca
func (t *TopConnectDriver) Seek(key interface{}) (bool, error) {
	return false, nil
}

// Locate localiza
func (t *TopConnectDriver) Locate(filter string) (bool, error) {
	return false, nil
}

// Count conta
func (t *TopConnectDriver) Count() (int, error) {
	return 0, nil
}

// Sum soma
func (t *TopConnectDriver) Sum(fieldName string) (float64, error) {
	return 0, nil
}

// GetFileName retorna nome do arquivo
func (t *TopConnectDriver) GetFileName() string {
	return t.file
}

// GetAlias retorna alias
func (t *TopConnectDriver) GetAlias() string {
	return t.alias
}

// IsReadOnly retorna se é readonly
func (t *TopConnectDriver) IsReadOnly() bool {
	return t.readOnly
}

// IsShared retorna se é shared
func (t *TopConnectDriver) IsShared() bool {
	return t.shared
}

// CtreeDriver implementa DatabaseDriver para Ctree
type CtreeDriver struct {
	DBFDriver
}

// NewCtreeDriver cria um novo driver Ctree
func NewCtreeDriver() *CtreeDriver {
	return &CtreeDriver{}
}

// BTrieveDriver implementa DatabaseDriver para BTrieve
type BTrieveDriver struct {
	DBFDriver
}

// NewBTrieveDriver cria um novo driver BTrieve
func NewBTrieveDriver() *BTrieveDriver {
	return &BTrieveDriver{}
}

// SQLiteDriver implementa DatabaseDriver para SQLite
type SQLiteDriver struct {
	db        *sql.DB
	file      string
	alias     string
	table     string
	readOnly  bool
	shared    bool
	structure []Field
	indexes   []Index
}

// NewSQLiteDriver cria um novo driver SQLite
func NewSQLiteDriver() *SQLiteDriver {
	return &SQLiteDriver{}
}

// Open abre um arquivo SQLite
func (s *SQLiteDriver) Open(file string, readOnly, shared bool) error {
	s.file = file
	s.alias = filepath.Base(file)
	s.readOnly = readOnly
	s.shared = shared

	// Abre o banco de dados SQLite
	var err error
	s.db, err = sql.Open("sqlite3", file)
	if err != nil {
		return fmt.Errorf("erro ao abrir banco SQLite: %w", err)
	}

	// Verifica conexão
	if err := s.db.Ping(); err != nil {
		return fmt.Errorf("erro ao conectar ao banco SQLite: %w", err)
	}

	// Se o arquivo contiver "/", assume que é tabela específica
	// Ex: database.db/table_name
	if strings.Contains(file, "/") {
		parts := strings.Split(file, "/")
		if len(parts) > 1 {
			s.table = parts[len(parts)-1]
		}
	}

	// Se não especificou tabela, obtém a primeira tabela
	if s.table == "" {
		rows, err := s.db.Query("SELECT name FROM sqlite_master WHERE type='table' LIMIT 1")
		if err != nil {
			return fmt.Errorf("erro ao listar tabelas: %w", err)
		}
		defer rows.Close()

		if rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				return fmt.Errorf("erro ao ler nome da tabela: %w", err)
			}
			s.table = tableName
		} else {
			return fmt.Errorf("nenhuma tabela encontrada no banco de dados")
		}
	}

	// Obtém estrutura da tabela
	if err := s.loadStructure(); err != nil {
		return fmt.Errorf("erro ao carregar estrutura: %w", err)
	}

	// Obtém índices
	if err := s.loadIndexes(); err != nil {
		return fmt.Errorf("erro ao carregar índices: %w", err)
	}

	return nil
}

// Close fecha o arquivo SQLite
func (s *SQLiteDriver) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// loadStructure carrega a estrutura da tabela
func (s *SQLiteDriver) loadStructure() error {
	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	// Usa aspas ao redor do nome da tabela para evitar erros de sintaxe
	query := fmt.Sprintf("PRAGMA table_info(\"%s\")", s.table)
	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("erro ao executar PRAGMA table_info: %w", err)
	}
	defer rows.Close()

	s.structure = []Field{}
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}

		// Converte tipo SQLite para tipo AdvPL
		fieldType := FieldTypeChar
		size := 0
		decimal := 0

		dataType = strings.ToUpper(dataType)
		if strings.Contains(dataType, "INT") {
			fieldType = FieldTypeNum
			size = 10
		} else if strings.Contains(dataType, "REAL") || strings.Contains(dataType, "FLOAT") || strings.Contains(dataType, "DOUBLE") {
			fieldType = FieldTypeNum
			size = 14
			decimal = 4
		} else if strings.Contains(dataType, "TEXT") || strings.Contains(dataType, "VARCHAR") || strings.Contains(dataType, "CHAR") {
			fieldType = FieldTypeChar
			size = 50
		} else if strings.Contains(dataType, "BLOB") {
			fieldType = FieldTypeMemo
		}

		s.structure = append(s.structure, Field{
			Name:     name,
			Type:     fieldType,
			Size:     size,
			Decimal:  decimal,
			Required: notNull == 1,
		})
	}

	return nil
}

// loadIndexes carrega os índices da tabela
func (s *SQLiteDriver) loadIndexes() error {
	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	// Usa aspas ao redor do nome da tabela para evitar erros de sintaxe
	query := fmt.Sprintf("PRAGMA index_list(\"%s\")", s.table)
	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("erro ao executar PRAGMA index_list: %w", err)
	}
	defer rows.Close()

	s.indexes = []Index{}
	for rows.Next() {
		var seq int
		var name, origin string
		var partial int
		var unique int

		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}

		// Obtém colunas do índice
		indexQuery := fmt.Sprintf("PRAGMA index_info(\"%s\")", name)
		indexRows, err := s.db.Query(indexQuery)
		if err != nil {
			continue
		}

		var expression string
		for indexRows.Next() {
			var seqNo, cid int
			var name sql.NullString
			if err := indexRows.Scan(&seqNo, &cid, &name); err != nil {
				continue
			}
			if cid < len(s.structure) {
				if expression != "" {
					expression += "+"
				}
				expression += s.structure[cid].Name
			}
		}
		indexRows.Close()

		s.indexes = append(s.indexes, Index{
			Name:       name,
			Expression: expression,
			Unique:     unique == 1,
			Descending: false,
		})
	}

	return nil
}

// GetStructure retorna a estrutura da tabela
func (s *SQLiteDriver) GetStructure() ([]Field, error) {
	return s.structure, nil
}

// GetData retorna dados da tabela
func (s *SQLiteDriver) GetData(offset, limit int) ([]Record, error) {
	if s.table == "" {
		return nil, fmt.Errorf("tabela não especificada")
	}

	query := "SELECT * FROM " + s.table
	if limit > 0 {
		query += " LIMIT " + fmt.Sprintf("%d", limit)
		if offset > 0 {
			query += " OFFSET " + fmt.Sprintf("%d", offset)
		}
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var records []Record
	recno := offset + 1

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		fields := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				fields[col] = string(b)
			} else {
				fields[col] = val
			}
		}

		records = append(records, Record{
			Recno:   recno,
			Fields:  fields,
			Deleted: false,
		})
		recno++
	}

	return records, nil
}

// GetRecord retorna um registro específico
func (s *SQLiteDriver) GetRecord(recno int) (*Record, error) {
	records, err := s.GetData(recno-1, 1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("recno inválido: %d", recno)
	}
	return &records[0], nil
}

// AddRecord adiciona um registro
func (s *SQLiteDriver) AddRecord(record Record) (int, error) {
	if s.readOnly {
		return 0, fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return 0, fmt.Errorf("tabela não especificada")
	}

	// Constrói query INSERT
	columns := make([]string, 0, len(record.Fields))
	placeholders := make([]string, 0, len(record.Fields))
	values := make([]interface{}, 0, len(record.Fields))

	for col, val := range record.Fields {
		columns = append(columns, col)
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	query := "INSERT INTO " + s.table + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"

	result, err := s.db.Exec(query, values...)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// UpdateRecord atualiza um registro
func (s *SQLiteDriver) UpdateRecord(recno int, record Record) error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	// Constrói query UPDATE
	updates := make([]string, 0, len(record.Fields))
	values := make([]interface{}, 0, len(record.Fields))

	for col, val := range record.Fields {
		updates = append(updates, col+" = ?")
		values = append(values, val)
	}

	query := "UPDATE " + s.table + " SET " + strings.Join(updates, ", ") + " WHERE rowid = ?"
	values = append(values, recno)

	_, err := s.db.Exec(query, values...)
	return err
}

// DeleteRecord deleta um registro
func (s *SQLiteDriver) DeleteRecord(recno int) error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	_, err := s.db.Exec("DELETE FROM "+s.table+" WHERE rowid = ?", recno)
	return err
}

// RecallRecord recupera um registro deletado
func (s *SQLiteDriver) RecallRecord(recno int) error {
	return fmt.Errorf("recall não suportado em SQLite")
}

// Pack compacta a tabela
func (s *SQLiteDriver) Pack() error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	_, err := s.db.Exec("VACUUM")
	return err
}

// Zap limpa a tabela
func (s *SQLiteDriver) Zap() error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	_, err := s.db.Exec("DELETE FROM " + s.table)
	return err
}

// GetIndexes retorna os índices
func (s *SQLiteDriver) GetIndexes() ([]Index, error) {
	return s.indexes, nil
}

// CreateIndex cria um índice
func (s *SQLiteDriver) CreateIndex(name string, expression string) error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	if s.table == "" {
		return fmt.Errorf("tabela não especificada")
	}

	query := "CREATE INDEX IF NOT EXISTS " + name + " ON " + s.table + " (" + expression + ")"
	_, err := s.db.Exec(query)
	return err
}

// DropIndex remove um índice
func (s *SQLiteDriver) DropIndex(name string) error {
	if s.readOnly {
		return fmt.Errorf("tabela é somente leitura")
	}

	_, err := s.db.Exec("DROP INDEX IF EXISTS " + name)
	return err
}

// SetOrder define o índice ativo
func (s *SQLiteDriver) SetOrder(indexName string) error {
	return nil
}

// GoTop vai para o topo
func (s *SQLiteDriver) GoTop() error {
	return nil
}

// GoBottom vai para o final
func (s *SQLiteDriver) GoBottom() error {
	return nil
}

// Skip pula n registros
func (s *SQLiteDriver) Skip(n int) error {
	return nil
}

// Seek busca por chave
func (s *SQLiteDriver) Seek(key interface{}) (bool, error) {
	return false, nil
}

// Locate localiza por filtro
func (s *SQLiteDriver) Locate(filter string) (bool, error) {
	return false, nil
}

// Count conta registros
func (s *SQLiteDriver) Count() (int, error) {
	if s.table == "" {
		return 0, fmt.Errorf("tabela não especificada")
	}

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + s.table).Scan(&count)
	return count, err
}

// Sum soma campo
func (s *SQLiteDriver) Sum(fieldName string) (float64, error) {
	if s.table == "" {
		return 0, fmt.Errorf("tabela não especificada")
	}

	var sum float64
	err := s.db.QueryRow("SELECT SUM(" + fieldName + ") FROM " + s.table).Scan(&sum)
	return sum, err
}

// GetFileName retorna o nome do arquivo
func (s *SQLiteDriver) GetFileName() string {
	return s.file
}

// GetAlias retorna o alias
func (s *SQLiteDriver) GetAlias() string {
	return s.alias
}

// IsReadOnly retorna se é somente leitura
func (s *SQLiteDriver) IsReadOnly() bool {
	return s.readOnly
}

// IsShared retorna se é compartilhado
func (s *SQLiteDriver) IsShared() bool {
	return s.shared
}

// ListTables lista todas as tabelas do banco de dados
func (s *SQLiteDriver) ListTables() ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("banco de dados não está aberto")
	}

	rows, err := s.db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		// Ignora tabelas do sistema SQLite
		if !strings.HasPrefix(tableName, "sqlite_") {
			tables = append(tables, tableName)
		}
	}

	return tables, nil
}

// CopyFile copia um arquivo
func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// FileExists verifica se arquivo existe
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
