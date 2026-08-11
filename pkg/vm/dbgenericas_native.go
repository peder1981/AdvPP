package vm

// Natives DB "Funcoes-genericas" — TDN: Functions/Banco-de-Dados/Funcoes-genericas.
//
// Implementa 22 funções DB* do Protheus sobre o SQLiteEngine/DBEngine:
// DBNickIndexKey, DBOrderInfo, DBOrderNickname, DBRecall, DBRecordInfo,
// DBReindex, DBRLock, DBRLockList, DBRUnlock, DBSetActFld, DBSetDriver,
// DBSetIndex, DBSetNickname, DBSqlExec, DBSqlPlan, DBStruct, DBTblCopy,
// DBUnlock, DBUnlockAll, DBUseArea, Deleted, FCount.
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

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

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
		if v.dbEngine != nil {
			if err := v.dbEngine.SelectArea(cFile); err != nil {
				return advplrt.Nil, nil
			}
		}
		s := v.dbGenStateFor()
		s.mu.Lock()
		s.physTable[cAlias] = cFile
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
