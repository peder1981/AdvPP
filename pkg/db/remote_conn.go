package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ConnConfig são os dados de conexão de um banco externo real — nunca
// contém um valor hardcoded: quem chama Open()/DbConnection:New() decide
// de onde vem a senha (GetEnv, cofre, etc).
type ConnConfig struct {
	Host     string
	Port     int
	Service  string // nome do banco (Postgres/MSSQL) ou SID/service name (Oracle)
	User     string
	Password string
}

// Dialect isola a única diferença de SQL entre os 3 drivers remotos: a
// sintaxe de placeholder de parâmetro. Tudo o mais (paginação, locking)
// não existe — RemoteSQLEngine carrega os registros em memória e navega
// em Go, igual ao SQLiteEngine.
type Dialect interface {
	Name() string
	Placeholder(pos int) string
}

type postgresDialect struct{}

func (postgresDialect) Name() string               { return "POSTGRES" }
func (postgresDialect) Placeholder(pos int) string { return fmt.Sprintf("$%d", pos) }

type oracleDialect struct{}

func (oracleDialect) Name() string               { return "ORACLE" }
func (oracleDialect) Placeholder(pos int) string { return fmt.Sprintf(":%d", pos) }

type mssqlDialect struct{}

func (mssqlDialect) Name() string               { return "MSSQL" }
func (mssqlDialect) Placeholder(pos int) string { return fmt.Sprintf("@p%d", pos) }

func dialectFor(driver string) (Dialect, bool) {
	switch strings.ToUpper(strings.TrimSpace(driver)) {
	case "POSTGRES", "POSTGRESQL":
		return postgresDialect{}, true
	case "ORACLE":
		return oracleDialect{}, true
	case "MSSQL", "SQLSERVER":
		return mssqlDialect{}, true
	}
	return nil, false
}

// OpenRemote abre uma conexão real de banco externo pelo nome do driver
// declarado em DbConnection:New()/TCLINK ("POSTGRES", "ORACLE", "MSSQL").
func OpenRemote(driver string, cfg ConnConfig) (*sql.DB, Dialect, error) {
	switch strings.ToUpper(strings.TrimSpace(driver)) {
	case "POSTGRES", "POSTGRESQL":
		return openPostgres(cfg)
	case "ORACLE":
		return openOracle(cfg)
	case "MSSQL", "SQLSERVER":
		return openMSSQL(cfg)
	}
	return nil, nil, fmt.Errorf("OpenRemote: driver desconhecido %q (use POSTGRES, ORACLE ou MSSQL)", driver)
}
