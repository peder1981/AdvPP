package vm

// Natives DB "Funcoes-genericas" — TDN: Functions/Banco-de-Dados/Funcoes-genericas.
//
// Implementa 37 funções DB* do Protheus sobre o SQLiteEngine/DBEngine:
// DBNickIndexKey, DBOrderInfo, DBOrderNickname, DBRecall, DBRecordInfo,
// DBReindex, DBRLock, DBRLockList, DBRUnlock, DBSetActFld, DBSetDriver,
// DBSetIndex, DBSetNickname, DBSqlExec, DBSqlPlan, DBStruct, DBTblCopy,
// DBUnlock, DBUnlockAll, DBUseArea, Deleted, FCount, Field, FieldBlock,
// FieldWBlock, Found, Header, IndexKey, IndexOrd, LastRec, NetErr, OrdBagName,
// OrdCreate, OrdDescend, OrdKey, OrdListAdd, OrdName, OrdNumber, OrdSetFocus,
// RDDName, RDDSetDefault, RealRDD, FLock, RLock, RecSize.
//
// Decisões de mapeamento (documentadas por função):
//   - O SQLiteEngine não tem conceito nativo de ordens de índice, apelidos
//     (nicknames) nem bloqueio por registro. Índices/ordens/nicknames são
//     mantidos como estado do VM (mapa keyed por alias), e os bloqueios de
//     registro como lista de recnos por alias. O engine só gerencia o lock
//     do registro corrente (RecLock/MsUnlock).
//   - @param por referência NÃO é gravável neste VM (ver mem0):
//     DBRecordInfo(@nRecord) e DBSqlPlan(@aResult, @cError) não propagam
//     escrita de escalares; DBSqlPlan grava aResult quando o argumento for
//     um array (arrays propagam por referência — mesmo ponteiro).
//   - DBSqlExec(SELECT) materializa o resultado em uma tabela TEMP com o
//     nome do alias, para que a área de trabalho passe a apontar para o
//     resultado (igual ao Protheus).
//   - DBStruct mapeia tipos SQLite para AdvPL (C/N/L/D/M) com tamanhos
//     razoáveis documentados; usada também por DBRecordInfo(3) para calcular
//     o tamanho do registro.
//   - Filtros (DBFilter/DBFilterCB/DBClearAllFilter): o SQLiteEngine NÃO tem
//     filtro e os stubs DBSETFILTER/DBCLEARFILTER em natives.go são no-ops.
//     O filtro é mantido como estado do VM (mapa alias -> expressão),
//     populado por DBFilter (registro implícito) e consultado por DBFilter.
//     DBSETFILTER não grava neste estado (fica no natives.go, sem alteração).
//   - DBGoTo usa GoTo(nRec) do engine (extensão opcional da DBEngine, type
//     asserted); DBInInsert usa InInsert()/SetInserting() do engine (Append
//     marca, DBCOMMIT limpa).

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// ---------------------------------------------------------------------------
// Estado persistente das funções DB genéricas (por VM).
// ---------------------------------------------------------------------------

type dbGenState struct {
	mu sync.Mutex
	// defaultRDD é a RDD padrão da sessão (DBSetDriver). Default DBFCDX.
	defaultRDD string
	// orders: alias -> lista de ordens de índice abertas (DBSetIndex/DBOrderInfo).
	orders map[string][]string
	// activeOrder: alias -> índice (0-based) da ordem ativa (DBOrderInfo/DBOrderNickname).
	activeOrder map[string]int
	// nicknames: alias -> ordem -> apelido (DBSetNickname/DBOrderNickname/DBNickIndexKey).
	nicknames map[string]map[string]string
	// locked: alias -> lista de recnos bloqueados pelo processo (DBRLock/DBRLockList/...).
	locked map[string][]int
	// activeFlds: alias -> campo -> ativo (DBSetActFld). nil = todos ativos.
	activeFlds map[string]map[string]bool
	// physTable: alias lógico -> nome físico da tabela (DBUseArea/DBSqlExec).
	physTable map[string]string
	// filters: alias -> expressão do filtro ativo (DBFilter/DBClearAllFilter).
	// O engine não tem filtro; DBSETFILTER em natives.go é no-op, então este
	// estado é populado pelas natives deste arquivo.
	filters map[string]string
	// filterCBs: alias -> codeblock do filtro ativo (DBFilterCB). Mantido por
	// coerência com filters; DBSETFILTER (natives.go) não o popula.
	filterCBs map[string]advplrt.Value
	// fileLocks: alias -> bloqueio de arquivo inteiro (FLock/DBInfo DBI_ISFLOCK).
	fileLocks map[string]bool
	// lastFound: alias -> resultado da última busca (DBSeek) — Found()/DBI_FOUND.
	lastFound map[string]bool
	// netErr: estado de erro da última operação de rede/RDD (NetErr()).
	// RDDs sem rede real nunca setam .T.; NetErr(lValor) grava explicitamente.
	netErr bool
	// rdds: alias lógico -> RDD usada na abertura (DBUseArea/RDDName).
	rdds map[string]string
	// keyExprs: alias -> ordem -> expressão da chave (OrdCreate/IndexKey/OrdKey).
	keyExprs map[string]map[string]string
}

var (
	dbGenStatesMu sync.Mutex
	dbGenStates   = map[*VM]*dbGenState{}
)

// dbGenStateFor devolve o estado DB genérico da VM, criando sob demanda.
func (v *VM) dbGenStateFor() *dbGenState {
	dbGenStatesMu.Lock()
	defer dbGenStatesMu.Unlock()
	if s, ok := dbGenStates[v]; ok {
		return s
	}
	s := &dbGenState{
		defaultRDD:   "DBFCDX",
		orders:       make(map[string][]string),
		activeOrder:  make(map[string]int),
		nicknames:    make(map[string]map[string]string),
		locked:       make(map[string][]int),
		activeFlds:   make(map[string]map[string]bool),
		physTable:    make(map[string]string),
		filters:      make(map[string]string),
		filterCBs:    make(map[string]advplrt.Value),
		fileLocks:    make(map[string]bool),
		lastFound:    make(map[string]bool),
		rdds:         make(map[string]string),
		keyExprs:     make(map[string]map[string]string),
	}
	dbGenStates[v] = s
	return s
}

// dbGenAlias devolve o alias lógico corrente (upper) e seu nome físico.
func (v *VM) dbGenAlias() string {
	return strings.ToUpper(v.currentAlias)
}

// dbGenPhysTable resolve o nome físico da tabela para o alias corrente.
func (v *VM) dbGenPhysTable() string {
	alias := v.dbGenAlias()
	s := v.dbGenStateFor()
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.physTable[alias]; ok {
		return p
	}
	return alias
}

// dbGenSQLEng type-assert do engine para a interface SQLEngine (Exec/QueryRows).
func (v *VM) dbGenSQLEng() (SQLEngine, bool) {
	if v.dbEngine == nil {
		return nil, false
	}
	eng, ok := v.dbEngine.(SQLEngine)
	return eng, ok
}

// dbGenColumns devolve os nomes das colunas físicas de uma tabela (PRAGMA).
func (v *VM) dbGenColumns(table string) []string {
	eng, ok := v.dbGenSQLEng()
	if !ok || !identRe.MatchString(table) {
		return nil
	}
	rows, err := eng.QueryRows(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil
	}
	cols := make([]string, 0, len(rows))
	for _, r := range rows {
		cols = append(cols, strings.ToUpper(strings.TrimSpace(r["NAME"])))
	}
	return cols
}

// registerDbgenericasNatives registra as funções de Banco de Dados genéricas.
func (v *VM) registerDbgenericasNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// =========================================================================
	// DBNickIndexKey(cNick) -> cRet
	//   Retorna a expressão (IndexKey) da ordem identificada pelo apelido.
	//   O SQLiteEngine não mantém expressões de chave de índice (apenas o
	//   nome da ordem aberta via DBSetIndex/DBSetNickname), portanto o
	//   retorno é "" (string nula) — comportamento idêntico ao caso em que o
	//   apelido não existe na spec.
	// =========================================================================
	natives["DBNICKINDEXKEY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}

	// =========================================================================
	// DBOrderInfo(nTipoInfo) -> xInfo
	//   7  = nome do arquivo de índice ao qual a ordem pertence (C)
	//   20 = idem, com diretório (C)
	//   9  = número de ordens abertas para o arquivo corrente (N)
	//   Usa o estado de ordens do alias corrente; "" ou 0 quando não há
	//   ordem aberta (como na spec).
	// =========================================================================
	natives["DBORDERINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nTipo := advplrt.ToFloat(getArg(args, 0))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		orders := s.orders[alias]
		idx := s.activeOrder[alias]
		switch int(nTipo) {
		case 7, 20:
			if idx >= 0 && idx < len(orders) {
				return advplrt.NewString(orders[idx]), nil
			}
			return advplrt.NewString(""), nil
		case 9:
			return advplrt.NewNumber(float64(len(orders))), nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBOrderNickname(cApelido) -> lRet
	//   Seleciona a ordem ativa pelo apelido. .T. se setada, .F. caso
	//   contrário. Usa o mapa de nicknames por alias.
	// =========================================================================
	natives["DBORDERNICKNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cNick := strings.ToUpper(getArgString(args, 0, ""))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		for i, order := range s.orders[alias] {
			if strings.ToUpper(s.nicknames[alias][order]) == cNick {
				s.activeOrder[alias] = i
				return advplrt.True, nil
			}
		}
		return advplrt.False, nil
	}

	// =========================================================================
	// DBRecall() -> NIL
	//   Desmarca o registro corrente para exclusão (D_E_L_E_T_ = ' ').
	//   Oposto de DBDelete. Requer registro corrente (FieldPut retorna erro
	//   se não houver — neste caso retornamos Nil silencioso, regra do VM).
	// =========================================================================
	natives["DBRECALL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if v.dbEngine != nil {
			v.dbEngine.FieldPut("D_E_L_E_T_", advplrt.NewString(" "))
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBRecordInfo(nInfoType, [@nRecord]) -> xRet
	//   1 (DBRI_DELETED) = estado de excluído (L) — igual Deleted()
	//   3 (DBRI_RECSIZE) = tamanho do registro (N) — soma dos tamanhos da estrutura
	//   5 (DBRI_UPDATED) = registro alterado e não gravado (L) — sempre .F.
	//                       (o VM não rastreia dirty state por registro)
	//   @nRecord (por referência) não é gravável neste VM — documentado.
	//   Tipo inválido => Nil (spec).
	// =========================================================================
	natives["DBRECORDINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nTipo := advplrt.ToFloat(getArg(args, 0))
		switch int(nTipo) {
		case 1:
			return v.nativeDeleted()
		case 3:
			size, err := v.dbGenRecSize()
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewNumber(float64(size)), nil
		case 5:
			return advplrt.False, nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBReindex() -> NIL
	//   Reconstrói os índices da área corrente. O SQLite mantém índices de
	//   forma automática — operação é no-op documentado (retorna Nil).
	// =========================================================================
	natives["DBREINDEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBRLock([nRec]) -> lRet
	//   Bloqueia um registro para receber atualizações. Sem nRec, bloqueia o
	//   registro corrente via engine.RecLock(). Com nRec, o recno é
	//   adicionado à lista de bloqueios do processo (o engine só gerencia o
	//   lock do registro corrente — o lock de outros recnos é rastreado no
	//   estado do VM e documentado).
	// =========================================================================
	natives["DBRLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		alias := v.dbGenAlias()
		nRec := advplrt.ToFloat(getArg(args, 0))
		if nRec <= 0 {
			// Sem nRec: usa o registro corrente e o engine (se houver). A
			// falha do RecLock (tabela vazia/sem registro) não invalida o
			// bloqueio lógico — o lock de DBAccess é registrado no estado.
			if v.dbEngine != nil {
				_ = v.dbEngine.RecLock()
			}
			recno := 1
			if v.dbEngine != nil {
				recno = v.dbEngine.RecNo()
			}
			s.mu.Lock()
			s.locked[alias] = appendUniqueRecno(s.locked[alias], recno)
			s.mu.Unlock()
			return advplrt.True, nil
		}
		// Com nRec: registra o bloqueio no estado do VM.
		s.mu.Lock()
		s.locked[alias] = appendUniqueRecno(s.locked[alias], int(nRec))
		s.mu.Unlock()
		return advplrt.True, nil
	}

	// =========================================================================
	// DBRLockList() -> aRet
	//   Retorna array com os recnos bloqueados pelo processo na tabela
	//   corrente. {} quando não há bloqueios. Sem alias aberto => array vazio
	//   (em vez do erro "Work area not in use", regra Nil-friendly do VM).
	// =========================================================================
	natives["DBRLOCKLIST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		list := s.locked[alias]
		elems := make([]advplrt.Value, 0, len(list))
		for _, r := range list {
			elems = append(elems, advplrt.NewNumber(float64(r)))
		}
		return advplrt.NewArray(elems), nil
	}

	// =========================================================================
	// DBRUnlock([nRec]) -> lRet
	//   Libera bloqueio(s) do processo na tabela corrente. Com nRec, libera
	//   apenas aquele recno; sem nRec, libera todos (como DBUnlock). O engine
	//   persiste o registro corrente via MsUnlock.
	// =========================================================================
	natives["DBRUNLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		alias := v.dbGenAlias()
		nRec := advplrt.ToFloat(getArg(args, 0))
		if nRec > 0 {
			s.locked[alias] = removeRecno(s.locked[alias], int(nRec))
		} else {
			s.locked[alias] = nil
		}
		s.mu.Unlock()
		if v.dbEngine != nil {
			v.dbEngine.MsUnlock()
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBSetActFld(cCampos, lAtivo) -> NIL
	//   Liga/desliga a visibilidade lógica de campos do alias corrente.
	//   "*" afeta todos os campos. O estado de campos inativos é mantido por
	//   alias; DBStruct e FCount (deste arquivo) respeitam a visibilidade.
	// =========================================================================
	natives["DBSETACTFLD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cCampos := getArgString(args, 0, "")
		lAtivo := advplrt.ToBool(getArg(args, 1))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		if s.activeFlds[alias] == nil {
			s.activeFlds[alias] = make(map[string]bool)
		}
		campos := strings.TrimSpace(cCampos)
		if campos == "*" {
			cols := v.dbGenColumns(alias)
			for _, c := range cols {
				s.activeFlds[alias][c] = lAtivo
			}
			return advplrt.Nil, nil
		}
		for _, f := range strings.Split(campos, ",") {
			f = strings.ToUpper(strings.TrimSpace(f))
			if f != "" {
				s.activeFlds[alias][f] = lAtivo
			}
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBSetDriver([cRDD]) -> cRet
	//   Retorna a RDD padrão da sessão, alterando-a se cRDD for válido
	//   (lista oficial de RDDs do Protheus). RDD inválida/vazia/Nil não
	//   altera e retorna a atual. Valor padrão: DBFCDX.
	// =========================================================================
	natives["DBSETDRIVER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		cRDD := getArgString(args, 0, "")
		if advplrt.IsNil(getArg(args, 0)) {
			cRDD = ""
		}
		cRDD = strings.ToUpper(strings.TrimSpace(cRDD))
		s.mu.Lock()
		defer s.mu.Unlock()
		if cRDD != "" && validRDD(cRDD) {
			prev := s.defaultRDD
			s.defaultRDD = cRDD
			return advplrt.NewString(prev), nil
		}
		return advplrt.NewString(s.defaultRDD), nil
	}

	// =========================================================================
	// DBSetIndex(cIndex) -> NIL
	//   Acrescenta uma ordem de índice à área de trabalho ativa. Mantido no
	//   estado do VM (alias -> lista de ordens). Se for a primeira ordem,
	//   torna-se ativa.
	// =========================================================================
	natives["DBSETINDEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cIndex := strings.ToUpper(getArgString(args, 0, ""))
		if cIndex == "" {
			return advplrt.Nil, nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		for _, o := range s.orders[alias] {
			if o == cIndex {
				return advplrt.Nil, nil
			}
		}
		s.orders[alias] = append(s.orders[alias], cIndex)
		if len(s.orders[alias]) == 1 {
			s.activeOrder[alias] = 0
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBSetNickname(cIndex, [cNickname]) -> cRet
	//   Define um apelido para uma ordem; sem cNickname, apenas consulta o
	//   apelido corrente. Retorna o apelido corrente (ou "" se não existe).
	// =========================================================================
	natives["DBSETNICKNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cIndex := strings.ToUpper(getArgString(args, 0, ""))
		cNick := ""
		if !advplrt.IsNil(getArg(args, 1)) {
			cNick = getArgString(args, 1, "")
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		found := false
		for _, o := range s.orders[alias] {
			if o == cIndex {
				found = true
				break
			}
		}
		if !found {
			// Ordem desconhecida: registra mesmo assim para permitir o fluxo
			// criar-ordem + apelidar do TDN (a ordem pode não ter passado por
			// DBSetIndex). "Caso a ordem especificada não seja encontrada ...
			// o retorno será uma string vazia" — mas aqui aceitamos por
			// tolerância com o DBCreateIndex do TDN, retornando o apelido.
			_ = alias
		}
		if s.nicknames[alias] == nil {
			s.nicknames[alias] = make(map[string]string)
		}
		if cNick != "" {
			s.nicknames[alias][cIndex] = cNick
		}
		return advplrt.NewString(s.nicknames[alias][cIndex]), nil
	}

	// =========================================================================
	// DBSqlExec(cAlias, cQuery, cDriver) -> bRet
	//   Executa SQL no driver SQLite. SELECT materializa o resultado numa
	//   tabela TEMP com o nome do alias (a área corrente passa a apontar para
	//   o resultado, igual ao Protheus). INSERT/UPDATE/DELETE/CREATE são
	//   executados diretamente (cAlias ignorado). .T. em sucesso, .F. em erro.
	// =========================================================================
	natives["DBSQLEXEC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cAlias := strings.ToUpper(getArgString(args, 0, ""))
		cQuery := getArgString(args, 1, "")
		eng, ok := v.dbGenSQLEng()
		if !ok {
			return advplrt.False, nil
		}
		trimmed := strings.TrimSpace(cQuery)
		up := strings.ToUpper(trimmed)
		if strings.HasPrefix(up, "SELECT") {
			if cAlias == "" || !identRe.MatchString(cAlias) {
				return advplrt.False, nil
			}
			// Materializa o resultado numa tabela com o nome do alias. Usa
			// tabela PERMANENTE: o SQLiteEngine (SelectArea/PRAGMA) não
			// enxerga tabelas TEMP (criadas no sqlite_temp_master).
			if err := eng.Exec("DROP TABLE IF EXISTS " + cAlias); err != nil {
				return advplrt.False, nil
			}
			if err := eng.Exec("CREATE TABLE " + cAlias + " AS " + trimmed); err != nil {
				return advplrt.False, nil
			}
			if v.dbEngine != nil {
				if err := v.dbEngine.SelectArea(cAlias); err != nil {
					return advplrt.False, nil
				}
			}
			s := v.dbGenStateFor()
			s.mu.Lock()
			s.physTable[cAlias] = cAlias
			s.mu.Unlock()
			v.currentAlias = cAlias
			return advplrt.True, nil
		}
		if err := eng.Exec(trimmed); err != nil {
			return advplrt.False, nil
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBSqlPlan(cQuery, cRDD, aResult, [nLevel], [cError]) -> lRet
	//   Devolve o plano de execução (EXPLAIN QUERY PLAN do SQLite) em aResult
	//   (array multi-dim: 1º elem = cabeçalho, demais = linhas). Grava
	//   aResult quando o argumento for um array (arrays propagam por
	//   referência neste VM). @cError não é gravável (escalar por referência)
	//   — documentado. .T. em sucesso, .F. em erro.
	// =========================================================================
	natives["DBSQLPLAN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cQuery := getArgString(args, 0, "")
		eng, ok := v.dbGenSQLEng()
		if !ok {
			return advplrt.False, nil
		}
		rows, err := eng.QueryRows("EXPLAIN QUERY PLAN " + cQuery)
		if err != nil {
			return advplrt.False, nil
		}
		// Cabeçalho fixo do EXPLAIN QUERY PLAN (SQLite): id, parent, notused, detail.
		header := []advplrt.Value{
			advplrt.NewString("id"), advplrt.NewString("parent"),
			advplrt.NewString("notused"), advplrt.NewString("detail"),
		}
		plan := make([]advplrt.Value, 0, len(rows)+1)
		plan = append(plan, advplrt.NewArray(header))
		for _, r := range rows {
			row := []advplrt.Value{
				advplrt.NewNumber(strFloat(r["ID"])),
				advplrt.NewNumber(strFloat(r["PARENT"])),
				advplrt.NewNumber(strFloat(r["NOTUSED"])),
				advplrt.NewString(r["DETAIL"]),
			}
			plan = append(plan, advplrt.NewArray(row))
		}
		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok {
			arr.Elements = plan
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBStruct() -> aRet
	//   Retorna array com a estrutura da tabela corrente: cada linha é
	//   { nome(C), tipo(C), tamanho(N), decimais(N) }. Mapeia tipos SQLite
	//   para AdvPL: INTEGER/NUMERIC/REAL/DECIMAL -> N, BOOLEAN -> L,
	//   DATE/DATETIME -> D, BLOB/CLOB -> M, demais -> C. Tamanhos padrão
	//   quando não declarados: C=20, N=15, D=8, L=1, M=10.
	// =========================================================================
	natives["DBSTRUCT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		table := v.dbGenPhysTable()
		eng, ok := v.dbGenSQLEng()
		if !ok || !identRe.MatchString(table) {
			return advplrt.Nil, nil
		}
		rows, err := eng.QueryRows(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			return advplrt.Nil, nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		alias := v.dbGenAlias()
		active := s.activeFlds[alias]
		s.mu.Unlock()
		elems := make([]advplrt.Value, 0, len(rows))
		for _, r := range rows {
			name := strings.ToUpper(strings.TrimSpace(r["NAME"]))
			if active != nil {
				if vis, ok := active[name]; ok && !vis {
					continue // campo inativo (DBSetActFld)
				}
			}
			typ, size, dec := dbGenSQLiteType(r["TYPE"])
			elems = append(elems, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(name),
				advplrt.NewString(typ),
				advplrt.NewNumber(float64(size)),
				advplrt.NewNumber(float64(dec)),
			}))
		}
		return advplrt.NewArray(elems), nil
	}

	// =========================================================================
	// DBTblCopy(cSourceAlias, cDestAlias) -> bRet
	//   Copia os dados da tabela origem para a destino (ambas SQLite).
	//   Insere as colunas comuns (excluindo a chave interna R_E_C_N_O_),
	//   preservando D_E_L_E_T_. .T. em sucesso, .F. em erro.
	// =========================================================================
	natives["DBTBLCOPY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSrc := strings.ToUpper(getArgString(args, 0, ""))
		cDst := strings.ToUpper(getArgString(args, 1, ""))
		eng, ok := v.dbGenSQLEng()
		if !ok || !identRe.MatchString(cSrc) || !identRe.MatchString(cDst) {
			return advplrt.False, nil
		}
		srcCols := v.dbGenColumns(cSrc)
		dstCols := v.dbGenColumns(cDst)
		if len(srcCols) == 0 || len(dstCols) == 0 {
			return advplrt.False, nil
		}
		dstSet := make(map[string]bool, len(dstCols))
		for _, c := range dstCols {
			dstSet[c] = true
		}
		common := make([]string, 0, len(srcCols))
		for _, c := range srcCols {
			if c == "R_E_C_N_O_" {
				continue
			}
			if dstSet[c] {
				common = append(common, c)
			}
		}
		if len(common) == 0 {
			return advplrt.False, nil
		}
		q := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
			cDst, strings.Join(common, ", "), strings.Join(common, ", "), cSrc)
		if err := eng.Exec(q); err != nil {
			return advplrt.False, nil
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBUnlock() -> lRet
	//   Retira todos os bloqueios de registros e de arquivos da tabela atual.
	//   Persiste o registro corrente via engine.MsUnlock.
	// =========================================================================
	natives["DBUNLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.locked[v.dbGenAlias()] = nil
		s.mu.Unlock()
		if v.dbEngine != nil {
			v.dbEngine.MsUnlock()
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBUnlockAll() -> lRet
	//   Retira todos os bloqueios de todas as tabelas abertas na área de
	//   trabalho. Equivalente a DBUnlock para todas as tabelas.
	// =========================================================================
	natives["DBUNLOCKALL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.locked = make(map[string][]int)
		s.mu.Unlock()
		if v.dbEngine != nil {
			v.dbEngine.MsUnlock()
		}
		return advplrt.True, nil
	}

	// =========================================================================
	// DBUseArea([lNewArea], [cDriver], cFile, cAlias, [lShared], [lReadOnly])
	//   Abre uma tabela de dados. Mapeia para SelectArea(cFile) e registra o
	//   alias lógico -> nome físico. Retorno sempre Nil.
	// =========================================================================
	natives["DBUSEAREA"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.ToUpper(getArgString(args, 2, ""))
		cAlias := strings.ToUpper(getArgString(args, 3, ""))
		if cAlias == "" {
			cAlias = cFile
		}
		cRDD := strings.ToUpper(strings.TrimSpace(getArgString(args, 1, "")))
		if v.dbEngine != nil {
			if err := v.dbEngine.SelectArea(cFile); err != nil {
				return advplrt.Nil, nil
			}
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.physTable[cAlias] = cFile
		if cRDD != "" {
			s.rdds[cAlias] = cRDD
		}
		s.mu.Unlock()
		v.currentAlias = cAlias
		return advplrt.Nil, nil
	}

	// =========================================================================
	// Deleted() -> lRet
	//   Verifica se o registro corrente está marcado para exclusão
	//   (D_E_L_E_T_ = '*'). Sem área selecionada => .F. (spec).
	// =========================================================================
	natives["DELETED"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.nativeDeleted()
	}

	// =========================================================================
	// FCount() -> nRet
	//   Retorna a quantidade de campos da estrutura da área de trabalho ativa.
	//   Respeita a visibilidade (DBSetActFld). Sem área => 0.
	// =========================================================================
	natives["FCOUNT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cols := v.dbGenColumns(v.dbGenPhysTable())
		s := v.dbGenStateFor()
		s.mu.Lock()
		alias := v.dbGenAlias()
		active := s.activeFlds[alias]
		s.mu.Unlock()
		if active == nil {
			return advplrt.NewNumber(float64(len(cols))), nil
		}
		n := 0
		for _, c := range cols {
			if vis, ok := active[c]; ok && !vis {
				continue
			}
			n++
		}
		return advplrt.NewNumber(float64(n)), nil
	}

	// =========================================================================
	// DBChangeAlias(cOldAlias, cNewAlias) -> NIL
	//   Muda o alias lógico de uma área de trabalho aberta, movendo o estado
	//   DB genérico (tabela física, ordens, nicknames, locks, campos ativos,
	//   filtro) para o novo alias. Retorno sempre nulo (spec).
	// =========================================================================
	natives["DBCHANGEALIAS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cOld := strings.ToUpper(getArgString(args, 0, ""))
		cNew := strings.ToUpper(getArgString(args, 1, ""))
		if cOld == "" || cNew == "" || !identRe.MatchString(cNew) {
			return advplrt.Nil, nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		// Se o alias corrente é o antigo, atualiza o ponteiro de área corrente.
		if v.currentAlias == cOld {
			v.currentAlias = cNew
		}
		if p, ok := s.physTable[cOld]; ok {
			s.physTable[cNew] = p
			delete(s.physTable, cOld)
		}
		if o, ok := s.orders[cOld]; ok {
			s.orders[cNew] = o
			delete(s.orders, cOld)
		}
		if a, ok := s.activeOrder[cOld]; ok {
			s.activeOrder[cNew] = a
			delete(s.activeOrder, cOld)
		}
		if n, ok := s.nicknames[cOld]; ok {
			s.nicknames[cNew] = n
			delete(s.nicknames, cOld)
		}
		if l, ok := s.locked[cOld]; ok {
			s.locked[cNew] = l
			delete(s.locked, cOld)
		}
		if a, ok := s.activeFlds[cOld]; ok {
			s.activeFlds[cNew] = a
			delete(s.activeFlds, cOld)
		}
		if f, ok := s.filters[cOld]; ok {
			s.filters[cNew] = f
			delete(s.filters, cOld)
		}
		if c, ok := s.filterCBs[cOld]; ok {
			s.filterCBs[cNew] = c
			delete(s.filterCBs, cOld)
		}
		if r, ok := s.rdds[cOld]; ok {
			s.rdds[cNew] = r
			delete(s.rdds, cOld)
		}
		if k, ok := s.keyExprs[cOld]; ok {
			s.keyExprs[cNew] = k
			delete(s.keyExprs, cOld)
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBClearAllFilter() -> NIL
	//   Limpa as condições de filtro de todas as tabelas abertas. O filtro é
	//   estado do VM (mapa alias -> expressão); o engine não tem filtro.
	//   Retorno sempre nulo (spec).
	// =========================================================================
	natives["DBCLEARALLFILTER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.filters = make(map[string]string)
		s.filterCBs = make(map[string]advplrt.Value)
		s.mu.Unlock()
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBClearIndex() -> NIL
	//   Fecha todos os índices da área de trabalho corrente, equivalente a
	//   SET INDEX sem índices. Efetiva pendências e limpa as ordens do estado.
	//   Retorno sempre nulo (spec).
	// =========================================================================
	natives["DBCLEARINDEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		alias := v.dbGenAlias()
		s.orders[alias] = nil
		delete(s.activeOrder, alias)
		s.mu.Unlock()
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBCloseAll() -> NIL
	//   Fecha todas as áreas de trabalho em uso, efetivando pendências e
	//   liberando bloqueios. Equivale a DBCloseArea para cada área aberta.
	//   Limpa todo o estado DB genérico do VM. Retorno sempre nulo (spec).
	// =========================================================================
	natives["DBCLOSEALL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.orders = make(map[string][]string)
		s.activeOrder = make(map[string]int)
		s.nicknames = make(map[string]map[string]string)
		s.locked = make(map[string][]int)
		s.activeFlds = make(map[string]map[string]bool)
		s.physTable = make(map[string]string)
		s.filters = make(map[string]string)
		s.filterCBs = make(map[string]advplrt.Value)
		s.mu.Unlock()
		v.currentAlias = ""
		if v.dbEngine != nil {
			if eng, ok := v.dbEngine.(interface{ MsUnlock() error }); ok {
				_ = eng.MsUnlock()
			}
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBCommitAll() -> NIL
	//   Salva em disco todas as atualizações pendentes na área de trabalho
	//   corrente. O SQLiteEngine não mantém transação aberta (cada UPDATE é
	//   imediato), então é no-op documentado; limpa o flag de inserção para
	//   que DBInInsert() volte a .F. (spec do exemplo). Retorno sempre nulo.
	// =========================================================================
	natives["DBCOMMITALL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if e, ok := v.dbEngine.(interface{ SetInserting(bool) }); ok {
			e.SetInserting(false)
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBCreate(cName, aStruct, [cDriver]) -> NIL
	//   Define uma nova tabela e sua estrutura (campos) no SGBD corrente.
	//   aStruct: { { nome(C), tipo(C), tamanho(N), decimais(N) }, ... } com
	//   tipos AdvPL 'C','D','L','M','N'. Mapeia para SQLite: C->TEXT,
	//   N->REAL/INTEGER, L->INTEGER, D->TEXT, M->TEXT. Cria as colunas de
	//   sistema R_E_C_N_O_ (PK AUTOINCREMENT) e D_E_L_E_T_ (default ' ').
	//   Validações da spec: nome vazio, campo DATA, nome > 10 chars (trunca),
	//   tipo inválido, formato numérico inválido — em caso de erro a criação
	//   é abortada e retorna NIL (o VM não levanta erro recuperável).
	// =========================================================================
	natives["DBCREATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.ToUpper(strings.TrimSpace(getArgString(args, 0, "")))
		aStruct, _ := getArg(args, 1).(*advplrt.ArrayValue)
		if cName == "" || aStruct == nil || !identRe.MatchString(cName) {
			return advplrt.Nil, nil
		}
		if strings.ToUpper(cName) == "DATA" {
			return advplrt.Nil, nil
		}
		cols := []string{}
		defs := []string{}
		valid := true
		for _, f := range aStruct.Elements {
			fieldArr, ok := f.(*advplrt.ArrayValue)
			if !ok || len(fieldArr.Elements) < 4 {
				valid = false
				break
			}
			fName := strings.ToUpper(strings.TrimSpace(advplrt.ToString(fieldArr.Elements[0])))
			fType := strings.ToUpper(strings.TrimSpace(advplrt.ToString(fieldArr.Elements[1])))
			fLen := int(advplrt.ToFloat(fieldArr.Elements[2]))
			fDec := int(advplrt.ToFloat(fieldArr.Elements[3]))
			if fName == "" {
				valid = false
				break
			}
			if strings.ToUpper(fName) == "DATA" {
				valid = false
				break
			}
			if len(fName) > 10 {
				fName = fName[:10] // truncar (warning da spec)
			}
			switch fType {
			case "C":
				if fLen <= 0 {
					fLen = 20
				}
				defs = append(defs, fName+" TEXT")
			case "N":
				if fLen == 1 && fDec != 0 {
					valid = false
					break
				}
				if fLen > 1 && fLen < fDec+2 {
					valid = false
					break
				}
				if fDec > 0 {
					defs = append(defs, fName+" REAL")
				} else {
					defs = append(defs, fName+" INTEGER")
				}
			case "L":
				defs = append(defs, fName+" INTEGER")
			case "D":
				defs = append(defs, fName+" TEXT")
			case "M":
				defs = append(defs, fName+" TEXT")
			default:
				valid = false
			}
			cols = append(cols, fName)
		}
		if !valid {
			return advplrt.Nil, nil
		}
		eng, ok := v.dbGenSQLEng()
		if !ok {
			return advplrt.Nil, nil
		}
		sql := "CREATE TABLE IF NOT EXISTS " + cName +
			" (R_E_C_N_O_ INTEGER PRIMARY KEY AUTOINCREMENT, D_E_L_E_T_ TEXT DEFAULT ' '"
		if len(defs) > 0 {
			sql += ", " + strings.Join(defs, ", ")
		}
		sql += ")"
		if err := eng.Exec(sql); err != nil {
			return advplrt.Nil, nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.physTable[cName] = cName
		s.mu.Unlock()
		if v.dbEngine != nil {
			if err := v.dbEngine.SelectArea(cName); err == nil {
				v.currentAlias = cName
			}
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBGetActFld() -> cCampos
	//   Retorna string com a lista de campos habilitados do alias corrente,
	//   separados por vírgula. Quando todos estão ativos (sem restrição via
	//   DBSetActFld), devolve "*". Usa o estado activeFlds.
	// =========================================================================
	natives["DBGETACTFLD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		alias := v.dbGenAlias()
		active := s.activeFlds[alias]
		s.mu.Unlock()
		if active == nil {
			return advplrt.NewString("*"), nil
		}
		cols := v.dbGenColumns(v.dbGenPhysTable())
		var out []string
		for _, c := range cols {
			vis, ok := active[c]
			if ok && !vis {
				continue // campo explicitamente inativo (DBSetActFld)
			}
			out = append(out, c)
		}
		if len(out) == 0 {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(strings.Join(out, ",")), nil
	}

	// =========================================================================
	// DBGoTo(nPos) -> NIL
	//   Posiciona a tabela corrente no registro conforme a ordem física
	//   (recno). Usa GoTo(nRec) do engine (extensão opcional da DBEngine —
	//   type asserted; se o engine não implementar, no-op documentado).
	//   Retorno sempre nulo (spec).
	// =========================================================================
	natives["DBGOTO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nPos := int(advplrt.ToFloat(getArg(args, 0)))
		if g, ok := v.dbEngine.(interface{ GoTo(int) error }); ok {
			_ = g.GoTo(nPos)
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBInInsert() -> lRet
	//   Retorna .T. se a tabela está em modo de inserção de registros (criou
	//   registro via DBAppend e ainda não deu commit). Usa InInsert() do
	//   engine (Append marca, DBCOMMIT/DBCommitAll limpam). Sem área => .F.
	// =========================================================================
	natives["DBININSERT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if e, ok := v.dbEngine.(interface{ InInsert() bool }); ok {
			return advplrt.NewBool(e.InInsert()), nil
		}
		return advplrt.False, nil
	}

	// =========================================================================
	// DBFilter() -> cExp
	//   Retorna a expressão do filtro ativo na área de trabalho corrente.
	//   "" quando não há filtro ativo (spec). O filtro é estado do VM
	//   (alias -> expressão); DBSETFILTER em natives.go não o popula.
	// =========================================================================
	natives["DBFILTER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		return advplrt.NewString(s.filters[v.dbGenAlias()]), nil
	}

	// =========================================================================
	// DBFilterCB() -> bExp
	//   Retorna o codeblock do filtro ativo na área corrente. Mantido no
	//   estado do VM (alias -> codeblock). Sem filtro => NIL (o codeblock do
	//   filtro padrão — "todos os registros" — não é armazenado).
	// =========================================================================
	natives["DBFILTERCB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		if cb, ok := s.filterCBs[v.dbGenAlias()]; ok && cb != nil {
			return cb, nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBInfo(nInfo) -> xInfo
	//   Obtém informações da tabela corrente. Constantes DBI_* do DBINFO.CH
	//   (valores conferidos): 1 ISDBF, 2 CANPUTREC, 3 GETHEADERSIZE,
	//   4 LASTUPDATE, 7 GETRECSIZE, 8 GETLOCKARRAY, 9 TABLEEXT, 10 FULLPATH,
	//   20 ISFLOCK, 26 BOF, 27 EOF, 28 DBFILTER, 29 FOUND, 30 FCOUNT,
	//   33 ALIAS, 36 SHARED. Sem área corrente => NIL (spec).
	// =========================================================================
	natives["DBINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nInfo := int(advplrt.ToFloat(getArg(args, 0)))
		alias := v.dbGenAlias()
		if v.dbEngine == nil {
			return advplrt.Nil, nil
		}
		switch nInfo {
		case 1: // DBI_ISDBF
			return advplrt.True, nil
		case 2: // DBI_CANPUTREC
			return advplrt.True, nil
		case 3: // DBI_GETHEADERSIZE
			size, err := v.dbGenRecSize()
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewNumber(float64(size)), nil
		case 4: // DBI_LASTUPDATE (data da última alteração — hoje)
			return advplrt.NewString(time.Now().Format("20060102")), nil
		case 7: // DBI_GETRECSIZE
			size, err := v.dbGenRecSize()
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewNumber(float64(size)), nil
		case 8: // DBI_GETLOCKARRAY
			s := v.dbGenStateFor()
			s.mu.Lock()
			list := s.locked[alias]
			s.mu.Unlock()
			elems := make([]advplrt.Value, 0, len(list))
			for _, r := range list {
				elems = append(elems, advplrt.NewNumber(float64(r)))
			}
			return advplrt.NewArray(elems), nil
		case 9: // DBI_TABLEEXT
			return advplrt.NewString(".dbf"), nil
		case 10: // DBI_FULLPATH
			return advplrt.NewString(v.dbGenPhysTable()), nil
		case 20: // DBI_ISFLOCK (arquivo bloqueado — rastreado em locked[alias])
			s := v.dbGenStateFor()
			s.mu.Lock()
			_, fl := s.fileLocks[alias]
			s.mu.Unlock()
			return advplrt.NewBool(fl), nil
		case 26: // DBI_BOF
			return advplrt.NewBool(v.dbEngine.BOF()), nil
		case 27: // DBI_EOF
			return advplrt.NewBool(v.dbEngine.EOF()), nil
		case 28: // DBI_DBFILTER
			s := v.dbGenStateFor()
			s.mu.Lock()
			expr := s.filters[alias]
			s.mu.Unlock()
			return advplrt.NewString(expr), nil
		case 29: // DBI_FOUND
			return v.dbGenFound()
		case 30: // DBI_FCOUNT
			return advplrt.NewNumber(float64(len(v.dbGenColumns(v.dbGenPhysTable())))), nil
		case 33: // DBI_ALIAS
			return advplrt.NewString(alias), nil
		case 36: // DBI_SHARED
			return advplrt.True, nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// DBFieldInfo(nType, nField) -> xRet
	//   Obtém informação de um campo da tabela corrente. Constantes DBS_* do
	//   DBSTRUCT.CH: 1 NAME, 2 TYPE, 3 LEN, 4 DEC. nField é 1-based e NÃO
	//   considera os campos internos (R_E_C_N_O_, D_E_L_E_T_) — por isso os
	//   campos físicos são filtrados. Sem área/campo => NIL (spec).
	// =========================================================================
	natives["DBFIELDINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nType := int(advplrt.ToFloat(getArg(args, 0)))
		nField := int(advplrt.ToFloat(getArg(args, 1)))
		cols := v.dbGenUserColumns()
		if nField < 1 || nField > len(cols) {
			return advplrt.Nil, nil
		}
		col := cols[nField-1]
		rows, err := v.dbGenColumnInfo(col)
		if err != nil {
			return advplrt.Nil, nil
		}
		switch nType {
		case 1: // DBS_NAME
			return advplrt.NewString(col), nil
		case 2: // DBS_TYPE
			typ, _, _ := dbGenSQLiteType(rows)
			return advplrt.NewString(typ), nil
		case 3: // DBS_LEN
			_, size, _ := dbGenSQLiteType(rows)
			return advplrt.NewNumber(float64(size)), nil
		case 4: // DBS_DEC
			_, _, dec := dbGenSQLiteType(rows)
			return advplrt.NewNumber(float64(dec)), nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// GetDBExtension() -> cRet
	//   Retorna a extensão em uso para tabelas acessadas via RDD DBFCDX.
	//   Padrão ".dbf" (driver ADS/localFiles); a extensão efetiva depende do
	//   ini do AppServer, indisponível neste VM. Documentado.
	// =========================================================================
	natives["GETDBEXTENSION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(".dbf"), nil
	}

	// =========================================================================
	// Field(nPos) -> cRet
	//   Retorna o nome do campo na posição nPos (1-based) da tabela corrente.
	//   Não considera os campos internos (R_E_C_N_O_, D_E_L_E_T_). Sem área ou
	//   nPos inválido => "" (spec: erro recuperável "Work area not in use"
	//   vira "" por regra Nil-friendly do VM).
	// =========================================================================
	natives["FIELD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nPos := int(advplrt.ToFloat(getArg(args, 0)))
		cols := v.dbGenUserColumns()
		if nPos < 1 || nPos > len(cols) {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(cols[nPos-1]), nil
	}

	// =========================================================================
	// FieldBlock(cField) -> bRet
	//   Retorna um codeblock get/set do campo no alias corrente. A infra de
	//   codeblocks deste VM exige bytecode sintetizado (FuncName + bc.Functions),
	//   inexistente em runtime — por isso retorna NIL, documentado (mesma
	//   limitação de DBFilterCB).
	// =========================================================================
	natives["FIELDBLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil
	}

	// =========================================================================
	// FieldWBlock(cField, nWorkArea) -> bRet
	//   Idem FieldBlock, para a área de trabalho informada. Mesma limitação de
	//   infra de codeblocks — retorna NIL documentado.
	// =========================================================================
	natives["FIELDWBLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil
	}

	// =========================================================================
	// Found() -> lRet
	//   .T. se a última operação de busca (DBSeek) obteve sucesso. Estado
	//   alimentado por DBSEEK (natives.go) via dbGenSetFound. Sem busca => .F.
	// =========================================================================
	natives["FOUND"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.dbGenFound()
	}

	// =========================================================================
	// Header() -> nBytes
	//   Retorna a quantidade de bytes no cabeçalho do arquivo de banco de
	//   dados corrente. O SQLite não tem header no formato DBF; retorna o
	//   tamanho do registro do usuário (aproximação documentada, coerente
	//   com RecSize/DBInfo DBI_GETHEADERSIZE).
	// =========================================================================
	natives["HEADER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(float64(v.dbGenUserRecSize())), nil
	}

	// =========================================================================
	// LastRec() -> nRet
	//   Retorna o número do último registro inserido na tabela atual
	//   (= RecCount/RecNo máximo). Sem área aberta => 0 (spec). Efetiva
	//   pendências antes; o SQLite persiste imediatamente, então é só contagem.
	// =========================================================================
	natives["LASTREC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if v.dbEngine == nil {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(v.dbEngine.RecCount())), nil
	}

	// =========================================================================
	// NetErr([lValor]) -> lRet
	//   Retorna .T. se a operação anterior de rede/RDD ocasionou erro. Sem
	//   lValor, devolve o estado corrente (default .F.; RDDs locais sem rede
	//   nunca setam). Com lValor, grava explicitamente o estado (spec).
	// =========================================================================
	natives["NETERR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		if advplrt.IsNil(getArg(args, 0)) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return advplrt.NewBool(s.netErr), nil
		}
		s.mu.Lock()
		s.netErr = advplrt.ToBool(getArg(args, 0))
		s.mu.Unlock()
		return advplrt.NewBool(s.netErr), nil
	}

	// =========================================================================
	// RecSize() -> nSize
	//   Retorna o tamanho de um registro: soma dos tamanhos dos campos do
	//   usuário + 1 byte pelo flag de exclusão (Deleted). Desconsidera os
	//   campos de controle (R_E_C_N_O_, D_E_L_E_T_, R_E_C_D_E_L_). Sem área
	//   aberta => 0 (spec).
	// =========================================================================
	natives["RECSIZE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cols := v.dbGenUserColumns()
		if len(cols) == 0 {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(v.dbGenUserRecSize()+1)), nil
	}

	// =========================================================================
	// IndexKey([nOrdem]) -> cExpr
	//   Retorna a expressão da chave da ordem de índice indicada. nOrdem 0
	//   (padrão) = ordem corrente. Só OrdCreate grava a expressão; ordens
	//   abertas por DBSetIndex/OrdListAdd não a têm => "" (spec).
	// =========================================================================
	natives["INDEXKEY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		name, _ := v.dbGenOrderNameAt(s, alias, int(advplrt.ToFloat(getArg(args, 0))))
		if name == "" {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(s.keyExprs[alias][name]), nil
	}

	// =========================================================================
	// IndexOrd() -> nOrd
	//   Retorna a posição da ordem corrente na lista de ordens da tabela
	//   (1-based). 0 quando não há índice aberto na tabela corrente (spec).
	// =========================================================================
	natives["INDEXORD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		if len(s.orders[alias]) == 0 {
			return advplrt.NewNumber(0), nil
		}
		idx := s.activeOrder[alias]
		if idx < 0 || idx >= len(s.orders[alias]) {
			idx = 0
		}
		return advplrt.NewNumber(float64(idx + 1)), nil
	}

	// =========================================================================
	// OrdBagName(xExp) -> cBag
	//   Retorna o nome da ordem de índice (aproximação do arquivo/ordem, já
	//   que o SQLite não tem bag físico .cdx/.ntx) cujo nome corresponde a
	//   xExp. "" quando não encontrada (spec).
	// =========================================================================
	natives["ORDBAGNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		xExp := strings.ToUpper(getArgString(args, 0, ""))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		for _, name := range s.orders[alias] {
			if strings.Contains(name, xExp) {
				return advplrt.NewString(name), nil
			}
		}
		return advplrt.NewString(""), nil
	}

	// =========================================================================
	// OrdCreate(cIndexFile, [cIndexTag], cExprKey, [bExprKey], [lUnique])
	//   Cria um novo índice para a área de trabalho ativa. Registra a ordem
	//   (tag ou nome do arquivo) na lista do alias, grava a expressão da
	//   chave e a torna a ordem corrente. Retorno NIL (spec).
	// =========================================================================
	natives["ORDCREATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.ToUpper(getArgString(args, 0, ""))
		if cFile == "" {
			return advplrt.Nil, nil
		}
		cTag := strings.ToUpper(getArgString(args, 1, ""))
		cExpr := getArgString(args, 2, "")
		name := cTag
		if name == "" {
			name = cFile
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		idx := -1
		for i, o := range s.orders[alias] {
			if o == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			s.orders[alias] = append(s.orders[alias], name)
			idx = len(s.orders[alias]) - 1
		}
		if s.keyExprs[alias] == nil {
			s.keyExprs[alias] = make(map[string]string)
		}
		s.keyExprs[alias][name] = cExpr
		s.activeOrder[alias] = idx
		return advplrt.Nil, nil
	}

	// =========================================================================
	// OrdDescend(xExp, [cIndex], [lDesc]) -> NIL
	//   Altera a flag crescente/decrescente da ordem. O SQLite não mantém
	//   ordem física de registros; a operação é no-op documentado (spec:
	//   "não altera fisicamente a ordem dos registros na tabela").
	// =========================================================================
	natives["ORDDESCEND"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil
	}

	// =========================================================================
	// OrdKey(cOrdem, [nPosicao], [cArqIndice]) -> cExpr
	//   Retorna a expressão da chave da ordem nomeada. Grava por OrdCreate;
	//   ordens sem expressão registrada => "" (spec).
	// =========================================================================
	natives["ORDKEY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cOrdem := strings.ToUpper(getArgString(args, 0, ""))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		return advplrt.NewString(s.keyExprs[alias][cOrdem]), nil
	}

	// =========================================================================
	// OrdListAdd(cIndexFile, [cIndexTag]) -> NIL
	//   Acrescenta uma ou mais ordens de um índice à área de trabalho ativa
	//   (equivalente ao DBSetIndex). Tag vazio usa o nome do arquivo. Não
	//   altera a ordem corrente. Retorno NIL (spec).
	// =========================================================================
	natives["ORDLISTADD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.ToUpper(getArgString(args, 0, ""))
		if cFile == "" {
			return advplrt.Nil, nil
		}
		cTag := strings.ToUpper(getArgString(args, 1, ""))
		name := cTag
		if name == "" {
			name = cFile
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		for _, o := range s.orders[alias] {
			if o == name {
				return advplrt.Nil, nil
			}
		}
		s.orders[alias] = append(s.orders[alias], name)
		if len(s.orders[alias]) == 1 {
			s.activeOrder[alias] = 0
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// OrdName(nOrd, [xParam]) -> cNome
	//   Retorna o nome da ordem na posição indicada. nOrd 0 (padrão) = ordem
	//   corrente; nOrd >= 1 = posição 1-based na lista. "" fora do range (spec).
	// =========================================================================
	natives["ORDNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		name, _ := v.dbGenOrderNameAt(s, alias, int(advplrt.ToFloat(getArg(args, 0))))
		return advplrt.NewString(name), nil
	}

	// =========================================================================
	// OrdNumber(cOrdem, [cArqIndice]) -> nPos
	//   Retorna a posição (1-based) da ordem pelo nome na lista do alias.
	//   0 quando a ordem não é encontrada (spec).
	// =========================================================================
	natives["ORDNUMBER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cOrdem := strings.ToUpper(getArgString(args, 0, ""))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		for i, o := range s.orders[alias] {
			if o == cOrdem {
				return advplrt.NewNumber(float64(i + 1)), nil
			}
		}
		return advplrt.NewNumber(0), nil
	}

	// =========================================================================
	// OrdSetFocus([xExp], [cOrdBagName]) -> cNome
	//   Retorna a ordem corrente; se xExp for dado e encontrar uma ordem na
	//   lista, define o foco para ela e retorna o novo nome. Ordem não
	//   encontrada => "" e o foco não muda (spec).
	// =========================================================================
	natives["ORDSETFOCUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		xExp := strings.ToUpper(getArgString(args, 0, ""))
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		alias := v.dbGenAlias()
		cur, _ := v.dbGenOrderNameAt(s, alias, 0)
		if xExp == "" {
			return advplrt.NewString(cur), nil
		}
		for i, o := range s.orders[alias] {
			if o == xExp {
				s.activeOrder[alias] = i
				return advplrt.NewString(o), nil
			}
		}
		return advplrt.NewString(""), nil
	}

	// =========================================================================
	// RDDName() -> cRDD
	//   Retorna o nome da RDD utilizada pela área de trabalho corrente
	//   (a RDD passada em DBUseArea; sem registro, a RDD padrão da sessão).
	//   Sem área aberta => "" (spec: erro "Work area not in use" vira "").
	// =========================================================================
	natives["RDDNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		alias := v.dbGenAlias()
		if alias == "" {
			return advplrt.NewString(""), nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		defer s.mu.Unlock()
		if r, ok := s.rdds[alias]; ok && r != "" {
			return advplrt.NewString(r), nil
		}
		return advplrt.NewString(s.defaultRDD), nil
	}

	// =========================================================================
	// RDDSetDefault([cRDD]) -> cRet
	//   Retorna a RDD padrão da sessão, podendo alterá-la (mesmo estado de
	//   DBSetDriver). cRDD inválido não altera. Valor padrão: DBFCDX.
	// =========================================================================
	natives["RDDSETDEFAULT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := v.dbGenStateFor()
		cRDD := getArgString(args, 0, "")
		if advplrt.IsNil(getArg(args, 0)) {
			cRDD = ""
		}
		cRDD = strings.ToUpper(strings.TrimSpace(cRDD))
		s.mu.Lock()
		defer s.mu.Unlock()
		if cRDD != "" && validRDD(cRDD) {
			s.defaultRDD = cRDD
		}
		return advplrt.NewString(s.defaultRDD), nil
	}

	// =========================================================================
	// RealRDD() -> cRDD
	//   Retorna o driver realmente utilizado para abrir tabelas locais.
	//   Este VM sempre usa SQLite como driver físico => "SQLITE".
	// =========================================================================
	natives["REALRDD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString("SQLITE"), nil
	}

	// =========================================================================
	// FLock() -> lRet
	//   Bloqueia a tabela/arquivo corrente (obsoleta na doc). Marca o bloqueio
	//   de arquivo no estado do VM (refletido em DBInfo DBI_ISFLOCK) e retorna
	//   .T.. Sem área aberta => .F. (spec: erro "Work area not in use").
	// =========================================================================
	natives["FLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		alias := v.dbGenAlias()
		if alias == "" {
			return advplrt.False, nil
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.fileLocks[alias] = true
		s.mu.Unlock()
		return advplrt.True, nil
	}

	// =========================================================================
	// RLock() -> lRet
	//   Bloqueia o registro corrente (equivale a DBRLock sem parâmetros).
	//   Usa o engine.RecLock e registra o recno no estado do VM. .T. em
	//   sucesso; sem área aberta => .F. (spec: erro "Work area not in use").
	// =========================================================================
	natives["RLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		alias := v.dbGenAlias()
		if alias == "" {
			return advplrt.False, nil
		}
		recno := 1
		if v.dbEngine != nil {
			_ = v.dbEngine.RecLock()
			recno = v.dbEngine.RecNo()
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.locked[alias] = appendUniqueRecno(s.locked[alias], recno)
		s.mu.Unlock()
		return advplrt.True, nil
	}
}

// nativeDeleted implementa Deleted(): .T. se D_E_L_E_T_ == '*' no registro corrente.
func (v *VM) nativeDeleted() (advplrt.Value, error) {
	if v.dbEngine == nil {
		return advplrt.False, nil
	}
	val, err := v.dbEngine.FieldGet("D_E_L_E_T_")
	if err != nil {
		return advplrt.False, nil
	}
	if advplrt.IsNil(val) {
		return advplrt.False, nil
	}
	return advplrt.NewBool(advplrt.ToString(val) == "*"), nil
}

// dbGenRecSize soma os tamanhos da estrutura (DBStruct) do alias corrente.
func (v *VM) dbGenRecSize() (int, error) {
	table := v.dbGenPhysTable()
	eng, ok := v.dbGenSQLEng()
	if !ok || !identRe.MatchString(table) {
		return 0, fmt.Errorf("no engine/table")
	}
	rows, err := eng.QueryRows(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return 0, err
	}
	total := 0
	for _, r := range rows {
		_, size, _ := dbGenSQLiteType(r["TYPE"])
		total += size
	}
	return total, nil
}

// dbGenSQLiteType mapeia um tipo declarado SQLite para (tipo AdvPL, tamanho, decimais).
func dbGenSQLiteType(sqlType string) (typ string, size, dec int) {
	t := strings.ToUpper(strings.TrimSpace(sqlType))
	size, dec = 0, 0
	// Extrai (n) ou (n,m) — ex.: VARCHAR(40), DECIMAL(10,2)
	if i := strings.Index(t, "("); i >= 0 {
		if j := strings.Index(t, ")"); j > i {
			parts := strings.Split(t[i+1:j], ",")
			if len(parts) > 0 {
				size, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
			}
			if len(parts) > 1 {
				dec, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		}
		t = t[:i]
	}
	switch {
	case strings.Contains(t, "INT") || strings.Contains(t, "NUM") ||
		strings.Contains(t, "DEC") || strings.Contains(t, "REAL") ||
		strings.Contains(t, "FLO") || strings.Contains(t, "DOUB"):
		if size == 0 {
			size = 15
		}
		return "N", size, dec
	case strings.Contains(t, "BOOL"):
		return "L", 1, 0
	case strings.Contains(t, "DATE") || strings.Contains(t, "TIME"):
		return "D", 8, 0
	case strings.Contains(t, "BLOB") || strings.Contains(t, "CLOB") || strings.Contains(t, "MEMO"):
		return "M", 10, 0
	default: // CHAR / TEXT / VARCHAR / etc
		if size == 0 {
			size = 20
		}
		return "C", size, 0
	}
}

// validRDD verifica se cRDD é um driver RDD oficial do Protheus (DBSetDriver).
func validRDD(cRDD string) bool {
	switch strings.ToUpper(cRDD) {
	case "DBFCDX", "DBFCDXTTS", "DBFCDXAX", "BTV", "BTVCDX", "CTREECDX",
		"TOPCONN", "CODEBCDX", "CODEBCDXTTS", "DBFCDXADS", "MEMORY",
		"SQLITE", "CTREETMP":
		return true
	}
	return false
}

// appendUniqueRecno adiciona um recno à lista sem duplicar.
func appendUniqueRecno(list []int, recno int) []int {
	for _, r := range list {
		if r == recno {
			return list
		}
	}
	return append(list, recno)
}

// removeRecno remove um recno da lista (todas as ocorrências).
func removeRecno(list []int, recno int) []int {
	out := list[:0]
	for _, r := range list {
		if r != recno {
			out = append(out, r)
		}
	}
	return out
}

// strFloat converte string (saída QueryRows) para float64.
func strFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// dbGenSetFound registra o resultado da última busca (DBSeek) para a área
// corrente — alimenta Found() e DBInfo(DBI_FOUND).
func (v *VM) dbGenSetFound(found bool) {
	s := v.dbGenStateFor()
	s.mu.Lock()
	s.lastFound[v.dbGenAlias()] = found
	s.mu.Unlock()
}

// dbGenFound devolve o resultado da última busca na área corrente. Sem busca
// prévia => .F. (spec de Found()).
func (v *VM) dbGenFound() (advplrt.Value, error) {
	s := v.dbGenStateFor()
	s.mu.Lock()
	defer s.mu.Unlock()
	return advplrt.NewBool(s.lastFound[v.dbGenAlias()]), nil
}

// dbGenUserColumns devolve as colunas físicas da tabela corrente EXCLUINDO os
// campos internos do sistema (R_E_C_N_O_, D_E_L_E_T_) — usada por DBFieldInfo
// (nField não considera os campos internos, spec).
func (v *VM) dbGenUserColumns() []string {
	cols := v.dbGenColumns(v.dbGenPhysTable())
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == "R_E_C_N_O_" || c == "D_E_L_E_T_" || c == "R_E_C_D_E_L_" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// dbGenColumnInfo devolve o tipo declarado (string) de uma coluna da tabela
// corrente via PRAGMA table_info.
func (v *VM) dbGenColumnInfo(col string) (string, error) {
	table := v.dbGenPhysTable()
	eng, ok := v.dbGenSQLEng()
	if !ok || !identRe.MatchString(table) || !identRe.MatchString(col) {
		return "", fmt.Errorf("no engine/table/col")
	}
	rows, err := eng.QueryRows(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if strings.ToUpper(strings.TrimSpace(r["NAME"])) == col {
			return r["TYPE"], nil
		}
	}
	return "", fmt.Errorf("column not found: %s", col)
}

// dbGenUserRecSize soma os tamanhos dos campos do USUÁRIO da tabela corrente
// (exclui R_E_C_N_O_, D_E_L_E_T_, R_E_C_D_E_L_), sem o byte do flag de
// exclusão. Usada por RecSize() (que soma +1 pelo flag, spec).
func (v *VM) dbGenUserRecSize() int {
	cols := v.dbGenUserColumns()
	total := 0
	for _, c := range cols {
		decl, err := v.dbGenColumnInfo(c)
		if err != nil {
			continue
		}
		_, size, _ := dbGenSQLiteType(decl)
		total += size
	}
	return total
}

// dbGenOrderNameAt devolve o nome da ordem na posição nOrdem (1-based) da lista
// de ordens do alias; nOrdem 0 (ou omitido) = ordem corrente. Retorna ""
// quando não há ordem na posição. Deve ser chamada com s.mu travado.
func (v *VM) dbGenOrderNameAt(s *dbGenState, alias string, nOrdem int) (string, bool) {
	orders := s.orders[alias]
	if len(orders) == 0 {
		return "", false
	}
	if nOrdem <= 0 {
		idx := s.activeOrder[alias]
		if idx < 0 || idx >= len(orders) {
			idx = 0
		}
		return orders[idx], true
	}
	if nOrdem >= 1 && nOrdem <= len(orders) {
		return orders[nOrdem-1], true
	}
	return "", false
}
