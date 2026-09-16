package vm

import (
	"strings"

	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// dbConnState é o estado Go da classe DbConnection: credenciais de uma
// conexão externa real (Postgres/Oracle/MSSQL), setadas em New() e usadas
// em Connect(). A senha nunca é persistida em lugar hardcoded pelo AdvPP —
// quem chama New() decide de onde ela vem (GetEnv, cofre, etc).
type dbConnState struct {
	driver    string
	host      string
	port      int
	service   string
	user      string
	password  string
	connID    int
	lastError string
}

func newDbConnectionObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("DbConnection", nil)
	obj.Native = &dbConnState{}
	return obj
}

// callDbConnectionMethod implementa a classe nativa DbConnection: conexão
// real a bancos externos (Postgres/Oracle/MSSQL) via pkg/db.OpenRemote
// (Tasks 5-6), registrada no mesmo estado global `dbstate` já usado por
// TCLINK — assim TCSQLEXEC/TCGENQRY/etc. funcionam sem alteração contra
// uma conexão aberta por DbConnection:Connect().
func (v *VM) callDbConnectionMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*dbConnState)
	if !ok {
		return advplrt.NewError("DbConnection: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		st.driver = strings.ToUpper(advplrt.ToString(getArg(args, 0)))
		st.host = advplrt.ToString(getArg(args, 1))
		st.port = int(advplrt.ToFloat(getArg(args, 2)))
		st.service = advplrt.ToString(getArg(args, 3))
		st.user = advplrt.ToString(getArg(args, 4))
		st.password = advplrt.ToString(getArg(args, 5))
		v.push(obj)
	case "CONNECT":
		cfg := db.ConnConfig{Host: st.host, Port: st.port, Service: st.service, User: st.user, Password: st.password}
		sqlDB, dialect, err := db.OpenRemote(st.driver, cfg)
		if err != nil {
			st.lastError = err.Error()
			v.push(advplrt.False)
			return nil
		}
		engine := db.NewRemoteSQLEngine(sqlDB, dialect)

		dbstate.mu.Lock()
		id := dbstate.nextID
		dbstate.nextID++
		dbstate.conns[id] = &dbstateConn{
			id:     id,
			driver: st.driver,
			server: st.host,
			port:   st.port,
			engine: engine,
			sqlEng: engine,
			remote: true,
		}
		dbstate.active = id
		dbstate.mu.Unlock()

		st.connID = id
		st.lastError = ""
		v.push(advplrt.True)
	case "CLOSE":
		dbstate.mu.Lock()
		if c, ok := dbstate.conns[st.connID]; ok {
			dbaccessCloseConnLocked(c)
			delete(dbstate.conns, st.connID)
			if dbstate.active == st.connID {
				dbstate.active = -1
			}
		}
		dbstate.mu.Unlock()
		v.push(advplrt.Nil)
	case "GETERROR":
		v.push(advplrt.NewString(st.lastError))
	default:
		return advplrt.NewError("DbConnection: método desconhecido " + method)
	}
	return nil
}
