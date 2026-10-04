package db

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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

// maxRemoteConns limita quantas conexões remotas (uma por sessão/requisição,
// ver pinPool) o processo mantém abertas. ADVPP_MAX_REMOTE_CONNS sobrescreve.
var maxRemoteConns = envInt("ADVPP_MAX_REMOTE_CONNS", 50)

var (
	remoteSlotsMu sync.Mutex
	remoteSlots   int
)

func acquireRemoteSlot() error {
	remoteSlotsMu.Lock()
	defer remoteSlotsMu.Unlock()
	if remoteSlots >= maxRemoteConns {
		return fmt.Errorf("limite de conexoes remotas atingido (%d)", maxRemoteConns)
	}
	remoteSlots++
	return nil
}

func releaseRemoteSlot() {
	remoteSlotsMu.Lock()
	if remoteSlots > 0 {
		remoteSlots--
	}
	remoteSlotsMu.Unlock()
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

// pinPool prende o *sql.DB a UMA conexão física: SET search_path (e qualquer
// estado de sessão do SGBD) passa a valer para todo comando seguinte deste
// *sql.DB. Sem isto o pool do database/sql espalhava os comandos entre
// conexões diferentes e o search_path ficava só na conexão onde caiu.
// Se o driver repõe uma conexão caída, a nova nasce no search_path padrão
// (public): o erro é "tabela não existe", nunca dado de outra empresa.
func pinPool(sqlDB *sql.DB) {
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)
}

// OpenRemote abre uma conexão real de banco externo pelo nome do driver
// declarado em DbConnection:New()/TCLINK ("POSTGRES", "ORACLE", "MSSQL"),
// presa a uma conexão física (pinPool) e contada no limite do processo —
// quem abre devolve a vaga com releaseRemoteSlot ao fechar (RemoteSQLEngine.Close).
func OpenRemote(driver string, cfg ConnConfig) (*sql.DB, Dialect, error) {
	if err := acquireRemoteSlot(); err != nil {
		return nil, nil, err
	}
	var (
		sqlDB   *sql.DB
		dialect Dialect
		err     error
	)
	switch strings.ToUpper(strings.TrimSpace(driver)) {
	case "POSTGRES", "POSTGRESQL":
		sqlDB, dialect, err = openPostgres(cfg)
	case "ORACLE":
		sqlDB, dialect, err = openOracle(cfg)
	case "MSSQL", "SQLSERVER":
		sqlDB, dialect, err = openMSSQL(cfg)
	default:
		err = fmt.Errorf("OpenRemote: driver desconhecido %q (use POSTGRES, ORACLE ou MSSQL)", driver)
	}
	if err != nil {
		releaseRemoteSlot()
		return nil, nil, err
	}
	pinPool(sqlDB)
	return sqlDB, dialect, nil
}
