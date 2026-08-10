package vm

import (
	"fmt"
	"sort"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerControledeprocessamentoNatives registra funções de controle de
// processamento: ExUserException, Findfunction (pré-existente), GetPrograms,
// JobInfo (stub), KillApp (stub), KillUser (stub), ManualJob, PCount,
// ProcLine (stub), ProcName (stub), setFinishAppHandler (stub), Sleep
// (pré-existente), SmartJob, StartJob (pré-existente), SysRefresh (stub),
// UserException (pré-existente), WaitRun (pré-existente), iif (pré-existente).
//
// Stubs confirmados em docs/tdn-gap-stubs.md (sem spec implementável num
// runtime embutido sem AppServer): KillApp, setFinishAppHandler, KillUser,
// SysRefresh, JobInfo, ProcLine, UserException, ProcName — NÃO são
// registrados aqui (os pré-existentes equivalentes seguem intocados).
func (v *VM) registerControledeprocessamentoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ExUserException(cTexto) -> aborta a aplicação com o texto como erro
	// do usuário. Não há janela de Error log num runtime CLI; a semântica
	// observável é a interrupção da execução com a mensagem.
	natives["EXUSEREXCEPTION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		msg := advplrt.ToString(getArg(args, 0))
		return nil, advplrt.NewError(msg)
	}

	// GetPrograms() -> aRet — array com o nome das User Functions (programas)
	// AdvPL carregadas em memória, em ordem alfabética.
	natives["GETPROGRAMS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		names := make([]string, 0, len(v.bc.Functions))
		for name := range v.bc.Functions {
			names = append(names, name)
		}
		sort.Strings(names)
		elems := make([]advplrt.Value, 0, len(names))
		for _, n := range names {
			elems = append(elems, advplrt.NewString(n))
		}
		return advplrt.NewArray(elems), nil
	}

	// ManualJob(cJobName, cEnv, cJobType, cOnStart, cOnConnect, cOnExit,
	// cSSKey, nInactive, nMin, nMax, nMinFree, nIncr, nWaitTime) -> sem retorno
	// Executa o Job descrito no próprio runtime (sem AppServer não há pool de
	// threads servidor; a semântica honesta é executar a função-alvo num job
	// isolado): cJobType ""/inválido/WEB* executa cOnConnect, "MDI" executa
	// cOnStart com cSSKey. Parâmetros de pool (nMin/nMax/nIncr/nMinFree/
	// nInactive/nWaitTime) são aceitos e ignorados — sem pool multi-thread.
	natives["MANUALJOB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cJobName := strings.Trim(advplrt.ToString(getArg(args, 0)), " ")
		cJobType := strings.ToUpper(strings.Trim(advplrt.ToString(getArg(args, 2)), " "))
		cOnStart := strings.Trim(advplrt.ToString(getArg(args, 3)), " ")
		cOnConnect := strings.Trim(advplrt.ToString(getArg(args, 4)), " ")
		cSSKey := advplrt.ToString(getArg(args, 6))

		if cJobName == "" {
			return advplrt.Nil, fmt.Errorf("ManualJob: missing job name")
		}
		if strings.Contains(cJobName, ",") {
			return advplrt.Nil, fmt.Errorf("ManualJob: job name cannot contain comma")
		}

		var target string
		var params []advplrt.Value
		switch cJobType {
		case "MDI":
			// MDI: executa cOnStart uma vez, usando cSSKey
			target = cOnStart
			if cSSKey != "" {
				params = []advplrt.Value{advplrt.NewString(cSSKey)}
			}
		default:
			// "" ou inválido: trata como Job de Start executando cOnConnect
			target = cOnConnect
		}
		if target == "" {
			return advplrt.Nil, fmt.Errorf("ManualJob(%s): no target function for job type %q", cJobName, cJobType)
		}

		// Execução isolada (mesma semântica de job do StartJob wait=false)
		if err := v.StartJob(target, false, params); err != nil {
			return advplrt.Nil, err
		}
		return advplrt.Nil, nil
	}

	// PCount() -> nRet — número de parâmetros passados na chamada atual
	// (função, método ou codeblock). O VM rastreia o ArgCount do frame ativo.
	natives["PCOUNT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if v.current == nil {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(v.current.ArgCount)), nil
	}

	// SmartJob(cName, cEnv, lWait, parm1...parm25) -> lRet
	// Executa cName numa thread isolada sem interface. Sem AppServer a fila
	// FIFO/limites de recursos (seção SMARTJOB ini) não existem; a semântica
	// honesta é disparar o job de forma não-bloqueante (lWait é sempre .F.
	// internamente, conforme TDN). Retorna .T. se o job entrou na fila.
	natives["SMARTJOB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cName := strings.Trim(advplrt.ToString(getArg(args, 0)), " ")
		if cName == "" {
			return advplrt.False, nil
		}
		// Valida a existência da função ANTES de enfileirar (TDN: .F. se
		// houver problema de parâmetros ou de entrada na fila). Um job
		// asíncrono não propagaria o erro de "função não existe" de volta.
		if !v.functionExists(cName) {
			return advplrt.False, nil
		}
		var params []advplrt.Value
		if len(args) > 3 {
			params = args[3:]
		}
		if err := v.StartJob(cName, false, params); err != nil {
			return advplrt.False, nil
		}
		return advplrt.True, nil
	}
}
