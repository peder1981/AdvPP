package shared

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Dictionary representa o dicionário de dados
type Dictionary struct {
	db     *sql.DB
	dbPath string
	loaded bool
}

// NewDictionary cria um novo dicionário
func NewDictionary(dbPath string) (*Dictionary, error) {
	// Garante que o diretório existe
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório: %w", err)
	}

	// Abre ou cria o banco de dados (opener compartilhado, com pragmas)
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir banco de dados: %w", err)
	}

	dict := &Dictionary{
		db:     db,
		dbPath: dbPath,
		loaded: false,
	}

	// Cria tabelas se não existirem
	if err := dict.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao criar tabelas: %w", err)
	}

	// Popula dados iniciais se necessário
	if err := dict.populateInitialData(); err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao popular dados iniciais: %w", err)
	}

	dict.loaded = true

	// Salva o caminho do banco como padrão na configuração
	if err := SetDefaultDatabase(dbPath); err != nil {
		// Não falha se não conseguir salvar a configuração
		fmt.Printf("Aviso: não foi possível salvar configuração: %v\n", err)
	}

	return dict, nil
}

// Close fecha o dicionário
func (d *Dictionary) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// createTables cria as tabelas do dicionário
func (d *Dictionary) createTables() error {
	fmt.Println("Criando tabelas do dicionário...")
	
	// Tabela SX2 - Tabelas
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX2 (
			X2_CHAVE TEXT PRIMARY KEY,
			X2_ALIAS TEXT NOT NULL,
			X2_NOME TEXT NOT NULL,
			X2_NOMEUSR TEXT,
			X2_MODULO TEXT,
			X2_TIPO TEXT,
			X2_DESCRIC TEXT
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SX2: %v\n", err)
		return err
	}
	fmt.Println("Tabela SX2 criada com sucesso")
	
	// Tabela SX3 - Campos
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX3 (
			X3_ARQUIVO TEXT NOT NULL,
			X3_ORDEM INTEGER NOT NULL,
			X3_CAMPO TEXT NOT NULL,
			X3_TIPO TEXT,
			X3_TAMANHO INTEGER,
			X3_DECIMAL INTEGER,
			X3_TITULO TEXT,
			X3_DESCRIC TEXT,
			X3_PICTURE TEXT,
			X3_VALID TEXT,
			X3_USADO TEXT,
			X3_RELACAO TEXT,
			X3_INICIO TEXT,
			X3_FIM TEXT,
			X3_CONTEXT TEXT,
			X3_CBOX TEXT,
			X3_CBOXSPA TEXT,
			X3_CBOXENG TEXT,
			X3_OBRIGAT TEXT,
			X3_VISUAL TEXT,
			X3_PROPRI TEXT,
			X3_BLOQUE TEXT,
			X3_FOLDER TEXT,
			X3_ORIGEM TEXT,
			X3_VIRTUAL TEXT,
			X3_IDXBLO TEXT,
			X3_RECURS TEXT,
			X3_REGEMP TEXT,
			X3_REGFIL TEXT,
			X3_DOMINI TEXT,
			X3_DOMINI1 TEXT,
			X3_DOMINI2 TEXT,
			X3_DOMINI3 TEXT,
			X3_DOMINI4 TEXT,
			X3_DOMINI5 TEXT,
			X3_DOMINI6 TEXT,
			X3_DOMINI7 TEXT,
			X3_DOMINI8 TEXT,
			X3_DOMINI9 TEXT,
			X3_DOMINI10 TEXT,
			X3_DOMINI11 TEXT,
			X3_DOMINI12 TEXT,
			X3_DOMINI13 TEXT,
			X3_DOMINI14 TEXT,
			X3_DOMINI15 TEXT,
			X3_DOMINI16 TEXT,
			X3_DOMINI17 TEXT,
			X3_DOMINI18 TEXT,
			X3_DOMINI19 TEXT,
			X3_DOMINI20 TEXT,
			X3_DOMINI21 TEXT,
			X3_DOMINI22 TEXT,
			X3_DOMINI23 TEXT,
			X3_DOMINI24 TEXT,
			X3_DOMINI25 TEXT,
			X3_DOMINI26 TEXT,
			X3_DOMINI27 TEXT,
			X3_DOMINI28 TEXT,
			X3_DOMINI29 TEXT,
			X3_DOMINI30 TEXT,
			X3_DOMINI31 TEXT,
			X3_DOMINI32 TEXT,
			X3_DOMINI33 TEXT,
			X3_DOMINI34 TEXT,
			X3_DOMINI35 TEXT,
			X3_DOMINI36 TEXT,
			X3_DOMINI37 TEXT,
			X3_DOMINI38 TEXT,
			X3_DOMINI39 TEXT,
			X3_DOMINI40 TEXT,
			X3_DOMINI41 TEXT,
			X3_DOMINI42 TEXT,
			X3_DOMINI43 TEXT,
			X3_DOMINI44 TEXT,
			X3_DOMINI45 TEXT,
			X3_DOMINI46 TEXT,
			X3_DOMINI47 TEXT,
			X3_DOMINI48 TEXT,
			X3_DOMINI49 TEXT,
			X3_DOMINI50 TEXT,
			PRIMARY KEY (X3_ARQUIVO, X3_ORDEM)
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SX3: %v\n", err)
		return err
	}
	fmt.Println("Tabela SX3 criada com sucesso")
	
	// Tabela SIX - Índices
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SIX (
			IX_ARQUIVO TEXT NOT NULL,
			IX_INDICE INTEGER NOT NULL,
			IX_ORDEM INTEGER NOT NULL,
			IX_CHAVE TEXT NOT NULL,
			IX_DESCRIC TEXT,
			PRIMARY KEY (IX_ARQUIVO, IX_INDICE, IX_ORDEM)
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SIX: %v\n", err)
		return err
	}
	fmt.Println("Tabela SIX criada com sucesso")
	
	// Tabela SX7 - Triggers
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX7 (
			X7_ARQUIVO TEXT NOT NULL,
			X7_CAMPO TEXT NOT NULL,
			X7_SEQUENCIA INTEGER NOT NULL,
			X7_REGRA TEXT,
			X7_CONDICAO TEXT,
			X7_ACAO TEXT,
			PRIMARY KEY (X7_ARQUIVO, X7_CAMPO, X7_SEQUENCIA)
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SX7: %v\n", err)
		return err
	}
	fmt.Println("Tabela SX7 criada com sucesso")
	
	// Tabela SX5 - Genéricas
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX5 (
			X5_TABELA TEXT NOT NULL,
			X5_CHAVE TEXT NOT NULL,
			X5_DESCRIC TEXT,
			X5_DESCSPA TEXT,
			X5_DESCENG TEXT,
			X5_TIPO TEXT,
			X5_TAMANHO INTEGER,
			X5_DECIMAL INTEGER,
			PRIMARY KEY (X5_TABELA, X5_CHAVE)
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SX5: %v\n", err)
		return err
	}
	fmt.Println("Tabela SX5 criada com sucesso")
	
	// Tabela SX6 - Parâmetros
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX6 (
			X6_VAR TEXT NOT NULL PRIMARY KEY,
			X6_TIPO TEXT,
			X6_DESCRIC TEXT,
			X6_CONTEUD TEXT,
			X6_DSCSPA TEXT,
			X6_DSCENG TEXT
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SX6: %v\n", err)
		return err
	}
	fmt.Println("Tabela SX6 criada com sucesso")
	
	// Tabela SXB - Lookups
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SXB (
			XB_ALIAS TEXT NOT NULL,
			XB_CAMPO TEXT NOT NULL,
			XB_TABELA TEXT,
			XB_VALOR TEXT,
			XB_DESCRIC TEXT,
			XB_TIPO TEXT,
			XB_FILTRO TEXT,
			PRIMARY KEY (XB_ALIAS, XB_CAMPO)
		)
	`); err != nil {
		fmt.Printf("Erro ao criar tabela SXB: %v\n", err)
		return err
	}
	fmt.Println("Tabela SXB criada com sucesso")
	
	fmt.Println("Todas as tabelas criadas com sucesso")
	return nil
}
// populateInitialData popula dados iniciais do dicionário
func (d *Dictionary) populateInitialData() error {
	// Verifica se já tem dados
	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM SX2").Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		return nil // Já tem dados
	}

	// Adiciona tabelas comuns do Protheus
	tables := []struct {
		chave, alias, nome, nomeusr, modulo, tipo, descricao string
	}{
		{"SA1", "SA1", "SA1", "Clientes", "SIGAFAT", "C", "Cadastro de Clientes"},
		{"SA2", "SA2", "SA2", "Fornecedores", "SIGACOM", "C", "Cadastro de Fornecedores"},
		{"SE1", "SE1", "SE1", "Contas a Receber", "SIGAFIN", "C", "Contas a Receber"},
		{"SE2", "SE2", "SE2", "Contas a Pagar", "SIGAFIN", "C", "Contas a Pagar"},
		{"SB1", "SB1", "SB1", "Produtos", "SIGAEST", "C", "Cadastro de Produtos"},
		{"SD1", "SD1", "SD1", "Vendas", "SIGAFAT", "C", "Documento de Vendas"},
		{"SF1", "SF1", "SF1", "Notas Fiscais", "SIGAFAT", "C", "Notas Fiscais"},
		{"SF2", "SF2", "SF2", "Itens da Nota Fiscal", "SIGAFAT", "C", "Itens da Nota Fiscal"},
		{"ZZ1", "ZZ1", "ZZ1", "Usuários", "SIGACFG", "C", "Cadastro de Usuários"},
		{"ZZ2", "ZZ2", "ZZ2", "Grupos de Acesso", "SIGACFG", "C", "Grupos de Acesso"},
	}

	for _, table := range tables {
		if _, err := d.db.Exec(`
			INSERT INTO SX2 (X2_CHAVE, X2_ALIAS, X2_NOME, X2_NOMEUSR, X2_MODULO, X2_TIPO, X2_DESCRIC)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, table.chave, table.alias, table.nome, table.nomeusr, table.modulo, table.tipo, table.descricao); err != nil {
			return err
		}
	}

	// Adiciona campos para SA1 (Clientes)
	sa1Fields := []struct {
		arquivo string
		ordem   int
		campo   string
		tipo    string
		tamanho int
		decimal int
		titulo  string
		descric string
	}{
		{"SA1", 1, "A1_COD", "C", 6, 0, "Código", "Código do Cliente"},
		{"SA1", 2, "A1_LOJA", "C", 2, 0, "Loja", "Loja do Cliente"},
		{"SA1", 3, "A1_NOME", "C", 40, 0, "Nome", "Nome do Cliente"},
		{"SA1", 4, "A1_NREDUZ", "C", 15, 0, "Nome Reduzido", "Nome Reduzido do Cliente"},
		{"SA1", 5, "A1_TIPO", "C", 1, 0, "Tipo", "Tipo de Cliente (F-Física, J-Jurídica)"},
		{"SA1", 6, "A1_CGC", "C", 14, 0, "CGC/CPF", "CGC ou CPF do Cliente"},
		{"SA1", 7, "A1_END", "C", 40, 0, "Endereço", "Endereço do Cliente"},
		{"SA1", 8, "A1_BAIRRO", "C", 30, 0, "Bairro", "Bairro do Cliente"},
		{"SA1", 9, "A1_MUN", "C", 40, 0, "Município", "Município do Cliente"},
		{"SA1", 10, "A1_EST", "C", 2, 0, "Estado", "Estado do Cliente"},
		{"SA1", 11, "A1_CEP", "C", 8, 0, "CEP", "CEP do Cliente"},
		{"SA1", 12, "A1_TEL", "C", 15, 0, "Telefone", "Telefone do Cliente"},
		{"SA1", 13, "A1_EMAIL", "C", 60, 0, "E-mail", "E-mail do Cliente"},
		{"SA1", 14, "A1_MSBLQ", "L", 1, 0, "Bloqueado", "Cliente Bloqueado"},
		{"SA1", 15, "A1_VEND", "C", 6, 0, "Vendedor", "Vendedor do Cliente"},
	}

	for _, field := range sa1Fields {
		if _, err := d.db.Exec(`
			INSERT INTO SX3 (X3_ARQUIVO, X3_ORDEM, X3_CAMPO, X3_TIPO, X3_TAMANHO, X3_DECIMAL, X3_TITULO, X3_DESCRIC)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, field.arquivo, field.ordem, field.campo, field.tipo, field.tamanho, field.decimal, field.titulo, field.descric); err != nil {
			return err
		}
	}

	// Adiciona índices para SA1
	sa1Indexes := []struct {
		arquivo string
		indice  int
		ordem   int
		chave   string
		descric string
	}{
		{"SA1", 1, 1, "A1_FILIAL+A1_COD+A1_LOJA", "Chave Primária"},
		{"SA1", 2, 1, "A1_NOME", "Nome do Cliente"},
		{"SA1", 3, 1, "A1_CGC", "CGC/CPF do Cliente"},
		{"SA1", 4, 1, "A1_VEND", "Vendedor do Cliente"},
	}

	for _, idx := range sa1Indexes {
		if _, err := d.db.Exec(`
			INSERT INTO SIX (IX_ARQUIVO, IX_INDICE, IX_ORDEM, IX_CHAVE, IX_DESCRIC)
			VALUES (?, ?, ?, ?, ?)
		`, idx.arquivo, idx.indice, idx.ordem, idx.chave, idx.descric); err != nil {
			return err
		}
	}

	// Adiciona genéricas (SX5)
	genericas := []struct {
		tabela, chave, descr, tipo string
		tamanho, decimal           int
	}{
		{"X3_TIPO", "C", "Caracter", "C", 0, 0},
		{"X3_TIPO", "N", "Numérico", "N", 0, 0},
		{"X3_TIPO", "D", "Data", "D", 0, 0},
		{"X3_TIPO", "L", "Lógico", "L", 0, 0},
		{"X3_TIPO", "M", "Memo", "M", 0, 0},
		{"A1_TIPO", "F", "Física", "C", 0, 0},
		{"A1_TIPO", "J", "Jurídica", "C", 0, 0},
	}

	for _, gen := range genericas {
		if _, err := d.db.Exec(`
			INSERT INTO SX5 (X5_TABELA, X5_CHAVE, X5_DESCRIC, X5_TIPO, X5_TAMANHO, X5_DECIMAL)
			VALUES (?, ?, ?, ?, ?, ?)
		`, gen.tabela, gen.chave, gen.descr, gen.tipo, gen.tamanho, gen.decimal); err != nil {
			return err
		}
	}

	return nil
}

// GetTables retorna todas as tabelas do dicionário
func (d *Dictionary) GetTables() ([]map[string]interface{}, error) {
	rows, err := d.db.Query("SELECT * FROM SX2")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []map[string]interface{}
	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		tables = append(tables, row)
	}

	return tables, nil
}

// GetFields retorna campos de uma tabela
func (d *Dictionary) GetFields(table string) ([]map[string]interface{}, error) {
	rows, err := d.db.Query("SELECT * FROM SX3 WHERE X3_ARQUIVO = ? ORDER BY X3_ORDEM", table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []map[string]interface{}
	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		fields = append(fields, row)
	}

	return fields, nil
}

// GetIndexes retorna índices de uma tabela
func (d *Dictionary) GetIndexes(table string) ([]map[string]interface{}, error) {
	rows, err := d.db.Query("SELECT * FROM SIX WHERE IX_ARQUIVO = ? ORDER BY IX_INDICE, IX_ORDEM", table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []map[string]interface{}
	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		indexes = append(indexes, row)
	}

	return indexes, nil
}

// GetGenericas retorna genéricas de uma tabela
func (d *Dictionary) GetGenericas(table string) ([]map[string]interface{}, error) {
	rows, err := d.db.Query("SELECT * FROM SX5 WHERE X5_TABELA = ?", table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genericas []map[string]interface{}
	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		genericas = append(genericas, row)
	}

	return genericas, nil
}

// AddTable adiciona uma nova tabela ao dicionário
func (d *Dictionary) AddTable(chave, alias, nome, nomeusr, modulo, tipo, descricao string) error {
	_, err := d.db.Exec(`
		INSERT INTO SX2 (X2_CHAVE, X2_ALIAS, X2_NOME, X2_NOMEUSR, X2_MODULO, X2_TIPO, X2_DESCRIC)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, chave, alias, nome, nomeusr, modulo, tipo, descricao)
	return err
}

// AddField adiciona um campo a uma tabela
func (d *Dictionary) AddField(arquivo string, ordem int, campo, tipo string, tamanho, decimal int, titulo, descric string) error {
	_, err := d.db.Exec(`
		INSERT INTO SX3 (X3_ARQUIVO, X3_ORDEM, X3_CAMPO, X3_TIPO, X3_TAMANHO, X3_DECIMAL, X3_TITULO, X3_DESCRIC)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, arquivo, ordem, campo, tipo, tamanho, decimal, titulo, descric)
	return err
}

// AddIndex adiciona um índice a uma tabela
func (d *Dictionary) AddIndex(arquivo string, indice, ordem int, chave, descric string) error {
	_, err := d.db.Exec(`
		INSERT INTO SIX (IX_ARQUIVO, IX_INDICE, IX_ORDEM, IX_CHAVE, IX_DESCRIC)
		VALUES (?, ?, ?, ?, ?)
	`, arquivo, indice, ordem, chave, descric)
	return err
}

// IsLoaded retorna se o dicionário está carregado
func (d *Dictionary) IsLoaded() bool {
	return d.loaded
}

// GetDBPath retorna o caminho do banco de dados
func (d *Dictionary) GetDBPath() string {
	return d.dbPath
}
