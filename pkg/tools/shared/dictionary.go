package shared

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	
	_ "github.com/mattn/go-sqlite3"
)

// Dictionary representa o dicionário de dados
type Dictionary struct {
	db        *sql.DB
	dbPath    string
	loaded    bool
}

// NewDictionary cria um novo dicionário
func NewDictionary(dbPath string) (*Dictionary, error) {
	// Garante que o diretório existe
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório: %w", err)
	}
	
	// Abre ou cria o banco de dados
	db, err := sql.Open("sqlite3", dbPath)
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
	// Tabela SX2 - Tabelas
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX2 (
			X2_CHAVE TEXT PRIMARY KEY,
			X2_ALIAS TEXT NOT NULL,
			X2_NOME TEXT NOT NULL,
			X2_NOMEUSR TEXT,
			X2_MODULO TEXT,
			X2_TIPO TEXT,
			X2_DESCRIC TEXT,
			X2_PATH TEXT,
			X2_ARQUIVO TEXT,
			X2_MODO TEXT,
			X2_TAMANHO INTEGER,
			X2_FILIAL TEXT,
			X2_ROTINA TEXT,
			X2_UNICO TEXT,
			X2_EXCLU TEXT,
			X2_BLOQUE TEXT,
			X2_IDXBLOQ TEXT,
			X2_MODOIMP TEXT,
			X2_IDXBLO TEXT,
			X2_RECURS TEXT,
			X2_REGEMP TEXT,
			X2_REGFIL TEXT,
			X2_LOGDBF TEXT,
			X2_LOGSQL TEXT,
			X2_LOGTAB TEXT,
			X2_LOGFIL TEXT,
			X2_LOGREC TEXT,
			X2_LOGCPY TEXT,
			X2_LOGCRT TEXT,
			X2_LOGALT TEXT,
			X2_LOGDEL TEXT,
			X2_LOGDIF TEXT,
			X2_LOGUPD TEXT,
			X2_LOGCNV TEXT,
			X2_LOGIMP TEXT,
			X2_LOGEXP TEXT,
			X2_LOGAUD TEXT,
			X2_LOGVAL TEXT,
			X2_LOGCHK TEXT,
			X2_LOGTRG TEXT,
			X2_LOGIDX TEXT,
			X2_LOGREF TEXT,
			X2_LOGERR TEXT,
			X2_LOGWAR TEXT,
			X2_LOGINF TEXT,
			X2_LOGDEB TEXT,
			X2_LOGTRC TEXT,
			X2_LOGPRF TEXT,
			X2_LOGTST TEXT,
			X2_LOGDEV TEXT,
			X2_LOGHLP TEXT,
			X2_LOGDOC TEXT,
			X2_LOGFAQ TEXT,
			X2_LOGTUT TEXT,
			X2_LOGEXM TEXT,
			X2_LOGSMP TEXT,
			X2_LOGTMP TEXT,
			X2_LOGBKP TEXT,
			X2_LOGRST TEXT,
			X2_LOGARC TEXT,
			X2_LOGZIP TEXT,
			X2_LOGFTP TEXT,
			X2_LOGHTTP TEXT,
			X2_LOGSMTP TEXT,
			X2_LOGPOP3 TEXT,
			X2_LOGIMAP TEXT,
			X2_LOGLDAP TEXT,
			X2_LOGAD TEXT,
			X2_LOGDNS TEXT,
			X2_LOGDHCP TEXT,
			X2_LOGNTP TEXT,
			X2_LOGSNMP TEXT,
			X2_LOGSYS TEXT,
			X2_LOGNET TEXT,
			X2_LOGSEC TEXT,
			X2_LOGFIL TEXT,
			X2_LOGDIR TEXT,
			X2_LOGUSR TEXT,
			X2_LOGGRP TEXT,
			X2_LOGPRM TEXT,
			X2_LOGENV TEXT,
			X2_LOGCFG TEXT,
			X2_LOGINI TEXT,
			X2_LOGREG TEXT,
			X2_LOGKEY TEXT,
			X2_LOGVAL TEXT,
			X2_LOGDAT TEXT,
			X2_LOGTIM TEXT,
			X2_LOGDTM TEXT,
			X2_LOGNUM TEXT,
			X2_LOGSTR TEXT,
			X2_LOGBLN TEXT,
			X2_LOGARR TEXT,
			X2_LOGOBJ TEXT,
			X2_LOGFUN TEXT,
			X2_LOGPRC TEXT,
			X2_LOGCLS TEXT,
			X2_LOGINT TEXT,
			X2_LOGEXT TEXT,
			X2_LOGLIB TEXT,
			X2_LOGMOD TEXT,
			X2_LOGPKG TEXT,
			X2_LOGAPI TEXT,
			X2_LOGCLI TEXT,
			X2_LOGSVR TEXT,
			X2_LOGDBS TEXT,
			X2_LOGSQL TEXT,
			X2_LOGNOS TEXT,
			X2_LOGCCH TEXT,
			X2_LOGMEM TEXT,
			X2_LOGCPU TEXT,
			X2_LOGDSK TEXT,
			X2_LOGNET TEXT,
			X2_LOGIO TEXT,
			X2_LOGDEV TEXT,
			X2_LOGDRV TEXT,
			X2_LOGHWD TEXT,
			X2_LOGFWW TEXT,
			X2_LOGOS TEXT,
			X2_LOGKRN TEXT,
			X2_LOGSHL TEXT,
			X2_LOGCMD TEXT,
			X2_LOGSCR TEXT,
			X2_LOGBAT TEXT,
			X2_LOGSH TEXT,
			X2_LOGPS TEXT,
			X2_LOGPY TEXT,
			X2_LOGRB TEXT,
			X2LOGPL TEXT,
			X2LOGJS TEXT,
			X2LOGTS TEXT,
			X2LOGGO TEXT,
			X2LOGRS TEXT,
			X2LOGPH TEXT,
			X2LOGJA TEXT,
			X2LOGCS TEXT,
			X2LOGCP TEXT,
			X2LOGVB TEXT,
			X2LOGDL TEXT,
			X2LOGAS TEXT,
			X2LOGFS TEXT,
			X2LOGHS TEXT,
			X2LOGCO TEXT,
			X2LOGKOT TEXT,
			X2LOGSC TEXT,
			X2LOGSW TEXT,
			X2LOGPL TEXT,
			X2LOGSQL TEXT,
			X2LOGOR TEXT,
			X2LOGPG TEXT,
			X2LOGMS TEXT,
			X2LOGMY TEXT,
			X2LOGSQ TEXT,
			X2LOGDB TEXT,
			X2LOGFX TEXT,
			X2LOGMM TEXT,
			X2LOGAD TEXT,
			X2LOGID TEXT,
			X2LOGND TEXT,
			X2LOGRD TEXT,
			X2LOGWD TEXT,
			X2LOGFD TEXT,
			X2LOGKV TEXT,
			X2LOGGD TEXT,
			X2LOGTS TEXT,
			X2LOGIN TEXT,
			X2LOGOU TEXT,
			X2LOGAP TEXT,
			X2LOGAZ TEXT,
			X2LOGAW TEXT,
			X2LOGS3 TEXT,
			X2LOGEC TEXT,
			X2LOGCF TEXT,
			X2LOGFN TEXT,
			X2LOGBF TEXT,
			X2LOGAF TEXT,
			X2LOGSF TEXT,
			X2LOGNF TEXT,
			X2LOGDF TEXT,
			X2LOGMF TEXT,
			X2LOGTF TEXT,
			X2LOGVF TEXT,
			X2LOGPF TEXT,
			X2LOGEF TEXT,
			X2LOGIF TEXT,
			X2LOGOF TEXT,
			X2LOGUF TEXT,
			X2LOGLF TEXT,
			X2LOGRF TEXT,
			X2LOGTF TEXT,
			X2LOGNF TEXT,
			X2LOGDF TEXT,
			X2LOGMF TEXT,
			X2LOGTF TEXT,
			X2LOGVF TEXT,
			X2LOGPF TEXT,
			X2LOGEF TEXT,
			X2LOGIF TEXT,
			X2LOGOF TEXT,
			X2LOGUF TEXT,
			X2LOGLF TEXT,
			X2LOGRF TEXT
		)
	`); err != nil {
		return err
	}
	
	// Tabela SX3 - Campos
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX3 (
			X3_ARQUIVO TEXT NOT NULL,
			X3_ORDEM INTEGER NOT NULL,
			X3_CAMPO TEXT NOT NULL,
			X3_TIPO TEXT NOT NULL,
			X3_TAMANHO INTEGER,
			X3_DECIMAL INTEGER,
			X3_TITULO TEXT,
			X3_DESCRIC TEXT,
			X3_PICTURE TEXT,
			X3_VALID TEXT,
			X3_USUARIO TEXT,
			X3_CONTEXT TEXT,
			X3_CBOX TEXT,
			X3_PAR01 TEXT,
			X3_PAR02 TEXT,
			X3_PAR03 TEXT,
			X3_PAR04 TEXT,
			X3_PAR05 TEXT,
			X3_F3 TEXT,
			X3_RESERV TEXT,
			X3_CHECK TEXT,
			X3_TRIGGER TEXT,
			X3_PROPRI TEXT,
			X3_BROWSE TEXT,
			X3_VISUAL TEXT,
			X3_RELACAO TEXT,
			X3_FOLDER TEXT,
			X3_IDX TEXT,
			X3_WHEN TEXT,
			X3_OBRIGAT TEXT,
			X3_VLDUSER TEXT,
			X3_VLDEXEC TEXT,
			X3_NIVEL INTEGER,
			X3_NOMVIS TEXT,
			X3_PICTURE TEXT,
			X3_MODAL TEXT,
			X3_DOMINIO TEXT,
			X3_DOMINI1 TEXT,
			X3_DOMINI2 TEXT,
			X3_DOMINI3 TEXT,
			X3_DOMINI4 TEXT,
			X3_DOMINI5 TEXT,
			X3_DOMINI6 TEXT,
			X3_DOMINI7 TEXT,
			X3_DOMINI8 TEXT,
			X3_DOMINI9 TEXT,
			X3_DOMIN10 TEXT,
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
		return err
	}
	
	// Tabela SIX - Índices
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SIX (
			IX_ARQUIVO TEXT NOT NULL,
			IX_INDICE INTEGER NOT NULL,
			IX_ORDEM INTEGER NOT NULL,
			IX_CHAVE TEXT NOT NULL,
			IX_DESCRIC TEXT,
			IX_APROPRI TEXT,
			IX_NICKNAME TEXT,
			IX_FILIAL TEXT,
			IX_ORIGEM TEXT,
			IX_PROPRI TEXT,
			IX_SELO TEXT,
			IX_ACEPES TEXT,
			IX_TIPO TEXT,
			IX_ORDEN TEXT,
			IX_AUTO TEXT,
			IX_EXPR TEXT,
			IX_FILTRO TEXT,
			IX_BLOQUE TEXT,
			IX_FORNE TEXT,
			IX_SEPAR TEXT,
			IX_PERM TEXT,
			IX_NIVEL INTEGER,
			IX_TABELA TEXT,
			IX_CAMPO TEXT,
			IX_RECURS TEXT,
			IX_REGEMP TEXT,
			IX_REGFIL TEXT,
			IX_IDXBLOQ TEXT,
			IX_IDXBLO TEXT,
			IX_RECURS TEXT,
			IX_REGEMP TEXT,
			IX_REGFIL TEXT,
			PRIMARY KEY (IX_ARQUIVO, IX_INDICE, IX_ORDEM)
		)
	`); err != nil {
		return err
	}
	
	// Tabela SX7 - Triggers
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX7 (
			X7_ARQUIVO TEXT NOT NULL,
			X7_CAMPO TEXT NOT NULL,
			X7_SEQUENC INTEGER NOT NULL,
			X7_EVENTO TEXT NOT NULL,
			X7_ROTINA TEXT,
			X7_CONDIC TEXT,
			X7_CARGA TEXT,
			X7_UNICO TEXT,
			X7_MODAL TEXT,
			X7_RECURS TEXT,
			X7_REGEMP TEXT,
			X7_REGFIL TEXT,
			X7_IDXBLOQ TEXT,
			X7_IDXBLO TEXT,
			X7_RECURS TEXT,
			X7_REGEMP TEXT,
			X7_REGFIL TEXT,
			PRIMARY KEY (X7_ARQUIVO, X7_CAMPO, X7_SEQUENC)
		)
	`); err != nil {
		return err
	}
	
	// Tabela SX5 - Genéricas
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX5 (
			X5_TABELA TEXT NOT NULL,
			X5_CHAVE TEXT NOT NULL,
			X5_DESCRIC TEXT NOT NULL,
			X5_DESCSPA TEXT,
			X5_DESCENG TEXT,
			X5_TIPO TEXT,
			X5_TAMANHO INTEGER,
			X5_DECIMAL INTEGER,
			X5_PRESEL TEXT,
			X5_PICTURE TEXT,
			X5_VALID TEXT,
			X5_USUARIO TEXT,
			X5_CBOX TEXT,
			X5_CAMPO1 TEXT,
			X5_CAMPO2 TEXT,
			X5_CAMPO3 TEXT,
			X5_CAMPO4 TEXT,
			X5_CAMPO5 TEXT,
			X5_F3 TEXT,
			X5_RESERV TEXT,
			X5_CHECK TEXT,
			X5_TRIGGER TEXT,
			X5_PROPRI TEXT,
			X5_BROWSE TEXT,
			X5_VISUAL TEXT,
			X5_RELACAO TEXT,
			X5_FOLDER TEXT,
			X5_IDX TEXT,
			X5_WHEN TEXT,
			X5_OBRIGAT TEXT,
			X5_VLDUSER TEXT,
			X5_VLDEXEC TEXT,
			X5_NIVEL INTEGER,
			X5_NOMVIS TEXT,
			X5_PICTURE TEXT,
			X5_MODAL TEXT,
			X5_DOMINIO TEXT,
			X5_DOMINI1 TEXT,
			X5_DOMINI2 TEXT,
			X5_DOMINI3 TEXT,
			X5_DOMINI4 TEXT,
			X5_DOMINI5 TEXT,
			X5_DOMINI6 TEXT,
			X5_DOMINI7 TEXT,
			X5_DOMINI8 TEXT,
			X5_DOMINI9 TEXT,
			X5_DOMINI10 TEXT,
			X5_DOMINI11 TEXT,
			X5_DOMINI12 TEXT,
			X5_DOMINI13 TEXT,
			X5_DOMINI14 TEXT,
			X5_DOMINI15 TEXT,
			X5_DOMINI16 TEXT,
			X5_DOMINI17 TEXT,
			X5_DOMINI18 TEXT,
			X5_DOMINI19 TEXT,
			X5_DOMINI20 TEXT,
			X5_DOMINI21 TEXT,
			X5_DOMINI22 TEXT,
			X5_DOMINI23 TEXT,
			X5_DOMINI24 TEXT,
			X5_DOMINI25 TEXT,
			X5_DOMINI26 TEXT,
			X5_DOMINI27 TEXT,
			X5_DOMINI28 TEXT,
			X5_DOMINI29 TEXT,
			X5_DOMINI30 TEXT,
			X5_DOMINI31 TEXT,
			X5_DOMINI32 TEXT,
			X5_DOMINI33 TEXT,
			X5_DOMINI34 TEXT,
			X5_DOMINI35 TEXT,
			X5_DOMINI36 TEXT,
			X5_DOMINI37 TEXT,
			X5_DOMINI38 TEXT,
			X5_DOMINI39 TEXT,
			X5_DOMINI40 TEXT,
			X5_DOMINI41 TEXT,
			X5_DOMINI42 TEXT,
			X5_DOMINI43 TEXT,
			X5_DOMINI44 TEXT,
			X5_DOMINI45 TEXT,
			X5_DOMINI46 TEXT,
			X5_DOMINI47 TEXT,
			X5_DOMINI48 TEXT,
			X5_DOMINI49 TEXT,
			X5_DOMINI50 TEXT,
			PRIMARY KEY (X5_TABELA, X5_CHAVE)
		)
	`); err != nil {
		return err
	}
	
	// Tabela SX6 - Parâmetros
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SX6 (
			X6_VAR TEXT NOT NULL,
			X6_TIPO TEXT NOT NULL,
			X6_DESCRIC TEXT NOT NULL,
			X6_DESCSPA TEXT,
			X6_DESCENG TEXT,
			X6_TAMANHO INTEGER,
			X6_DECIMAL INTEGER,
			X6_PRESEL TEXT,
			X6_PICTURE TEXT,
			X6_VALID TEXT,
			X6_USUARIO TEXT,
			X6_CBOX TEXT,
			X6_CAMPO1 TEXT,
			X6_CAMPO2 TEXT,
			X6_CAMPO3 TEXT,
			X6_CAMPO4 TEXT,
			X6_CAMPO5 TEXT,
			X6_F3 TEXT,
			X6_RESERV TEXT,
			X6_CHECK TEXT,
			X6_TRIGGER TEXT,
			X6_PROPRI TEXT,
			X6_BROWSE TEXT,
			X6_VISUAL TEXT,
			X6_RELACAO TEXT,
			X6_FOLDER TEXT,
			X6_IDX TEXT,
			X6_WHEN TEXT,
			X6_OBRIGAT TEXT,
			X6_VLDUSER TEXT,
			X6_VLDEXEC TEXT,
			X6_NIVEL INTEGER,
			X6_NOMVIS TEXT,
			X6_PICTURE TEXT,
			X6_MODAL TEXT,
			X6_DOMINIO TEXT,
			X6_DOMINI1 TEXT,
			X6_DOMINI2 TEXT,
			X6_DOMINI3 TEXT,
			X6_DOMINI4 TEXT,
			X6_DOMINI5 TEXT,
			X6_DOMINI6 TEXT,
			X6_DOMINI7 TEXT,
			X6_DOMINI8 TEXT,
			X6_DOMINI9 TEXT,
			X6_DOMINI10 TEXT,
			X6_DOMINI11 TEXT,
			X6_DOMINI12 TEXT,
			X6_DOMINI13 TEXT,
			X6_DOMINI14 TEXT,
			X6_DOMINI15 TEXT,
			X6_DOMINI16 TEXT,
			X6_DOMINI17 TEXT,
			X6_DOMINI18 TEXT,
			X6_DOMINI19 TEXT,
			X6_DOMINI20 TEXT,
			X6_DOMINI21 TEXT,
			X6_DOMINI22 TEXT,
			X6_DOMINI23 TEXT,
			X6_DOMINI24 TEXT,
			X6_DOMINI25 TEXT,
			X6_DOMINI26 TEXT,
			X6_DOMINI27 TEXT,
			X6_DOMINI28 TEXT,
			X6_DOMINI29 TEXT,
			X6_DOMINI30 TEXT,
			X6_DOMINI31 TEXT,
			X6_DOMINI32 TEXT,
			X6_DOMINI33 TEXT,
			X6_DOMINI34 TEXT,
			X6_DOMINI35 TEXT,
			X6_DOMINI36 TEXT,
			X6_DOMINI37 TEXT,
			X6_DOMINI38 TEXT,
			X6_DOMINI39 TEXT,
			X6_DOMINI40 TEXT,
			X6_DOMINI41 TEXT,
			X6_DOMINI42 TEXT,
			X6_DOMINI43 TEXT,
			X6_DOMINI44 TEXT,
			X6_DOMINI45 TEXT,
			X6_DOMINI46 TEXT,
			X6_DOMINI47 TEXT,
			X6_DOMINI48 TEXT,
			X6_DOMINI49 TEXT,
			X6_DOMINI50 TEXT,
			PRIMARY KEY (X6_VAR)
		)
	`); err != nil {
		return err
	}
	
	// Tabela SXB - Perguntas
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS SXB (
			XB_ALIAS TEXT NOT NULL,
			XB_TIPO TEXT NOT NULL,
			XB_DESCRIC TEXT NOT NULL,
			XB_DESCSPA TEXT,
			XB_DESCENG TEXT,
			XB_TAMANHO INTEGER,
			XB_DECIMAL INTEGER,
			XB_PRESEL TEXT,
			XB_PICTURE TEXT,
			XB_VALID TEXT,
			XB_USUARIO TEXT,
			XB_CBOX TEXT,
			XB_CAMPO1 TEXT,
			XB_CAMPO2 TEXT,
			XB_CAMPO3 TEXT,
			XB_CAMPO4 TEXT,
			XB_CAMPO5 TEXT,
			XB_F3 TEXT,
			XB_RESERV TEXT,
			XB_CHECK TEXT,
			XB_TRIGGER TEXT,
			XB_PROPRI TEXT,
			XB_BROWSE TEXT,
			XB_VISUAL TEXT,
			XB_RELACAO TEXT,
			XB_FOLDER TEXT,
			XB_IDX TEXT,
			XB_WHEN TEXT,
			XB_OBRIGAT TEXT,
			XB_VLDUSER TEXT,
			XB_VLDEXEC TEXT,
			XB_NIVEL INTEGER,
			XB_NOMVIS TEXT,
			XB_PICTURE TEXT,
			XB_MODAL TEXT,
			XB_DOMINIO TEXT,
			XB_DOMINI1 TEXT,
			XB_DOMINI2 TEXT,
			XB_DOMINI3 TEXT,
			XB_DOMINI4 TEXT,
			XB_DOMINI5 TEXT,
			XB_DOMINI6 TEXT,
			XB_DOMINI7 TEXT,
			XB_DOMINI8 TEXT,
			XB_DOMINI9 TEXT,
			XB_DOMINI10 TEXT,
			XB_DOMINI11 TEXT,
			XB_DOMINI12 TEXT,
			XB_DOMINI13 TEXT,
			XB_DOMINI14 TEXT,
			XB_DOMINI15 TEXT,
			XB_DOMINI16 TEXT,
			XB_DOMINI17 TEXT,
			XB_DOMINI18 TEXT,
			XB_DOMINI19 TEXT,
			XB_DOMINI20 TEXT,
			XB_DOMINI21 TEXT,
			XB_DOMINI22 TEXT,
			XB_DOMINI23 TEXT,
			XB_DOMINI24 TEXT,
			XB_DOMINI25 TEXT,
			XB_DOMINI26 TEXT,
			XB_DOMINI27 TEXT,
			XB_DOMINI28 TEXT,
			XB_DOMINI29 TEXT,
			XB_DOMINI30 TEXT,
			XB_DOMINI31 TEXT,
			XB_DOMINI32 TEXT,
			XB_DOMINI33 TEXT,
			XB_DOMINI34 TEXT,
			XB_DOMINI35 TEXT,
			XB_DOMINI36 TEXT,
			XB_DOMINI37 TEXT,
			XB_DOMINI38 TEXT,
			XB_DOMINI39 TEXT,
			XB_DOMINI40 TEXT,
			XB_DOMINI41 TEXT,
			XB_DOMINI42 TEXT,
			XB_DOMINI43 TEXT,
			XB_DOMINI44 TEXT,
			XB_DOMINI45 TEXT,
			XB_DOMINI46 TEXT,
			XB_DOMINI47 TEXT,
			XB_DOMINI48 TEXT,
			XB_DOMINI49 TEXT,
			XB_DOMINI50 TEXT,
			PRIMARY KEY (XB_ALIAS)
		)
	`); err != nil {
		return err
	}
	
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
		tamanho, decimal int
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
