package vm

import (
	"fmt"
	"sync/atomic"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// asyncJobResult guarda o resultado de um job disparado via FWJOBSTART,
// consumido (uma única vez) por FWJOBPOLL.
type asyncJobResult struct {
	done  bool
	value advplrt.Value
}

// registerAsyncJobNatives registers FWJOBSTART/FWJOBPOLL: um mecanismo de
// job assíncrono COM retorno de valor, complementar ao StartJob/STARTJOB
// existente (que é fire-and-forget e não é alterado por este arquivo).
func (v *VM) registerAsyncJobNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// FWJOBSTART(cFuncName, params...) -> cJobId
	// Dispara cFuncName numa goroutine com VM isolado (mesmo mecanismo e
	// limite de concorrência de StartJob com wait=false). Retorna
	// imediatamente um id opaco para coleta posterior via FWJOBPOLL.
	natives["FWJOBSTART"] = func(args []advplrt.Value) (advplrt.Value, error) {
		funcName := advplrt.ToString(getArg(args, 0))
		if funcName == "" {
			return advplrt.Nil, fmt.Errorf("FWJOBSTART: missing function name")
		}
		var params []advplrt.Value
		if len(args) > 1 {
			params = args[1:]
		}

		jobID := fmt.Sprintf("job-%d", atomic.AddInt64(&v.jobIDSeq, 1))

		currentCount := atomic.LoadInt32(&activeJobsCount)
		if currentCount >= int32(MaxConcurrentJobs) {
			return advplrt.Nil, fmt.Errorf("max concurrent jobs exceeded (%d)", MaxConcurrentJobs)
		}
		newCount := atomic.AddInt32(&activeJobsCount, 1)
		if newCount > int32(MaxConcurrentJobs) {
			atomic.AddInt32(&activeJobsCount, -1)
			return advplrt.Nil, fmt.Errorf("max concurrent jobs exceeded (%d)", MaxConcurrentJobs)
		}

		job := NewVM(v.bc, false)
		job.dbFactory = v.dbFactory
		if v.dbFactory != nil {
			job.dbEngine = v.dbFactory()
		}

		// Entrada criada ANTES de disparar a goroutine: assim FWJOBDONE/
		// FWJOBPOLL sempre encontram o id (done:false) enquanto o job roda,
		// em vez de tratarem "ainda rodando" e "id desconhecido" da mesma
		// forma. A goroutine abaixo ATUALIZA esta mesma entrada (não cria
		// uma nova) quando termina.
		v.jobResults.Store(jobID, &asyncJobResult{done: false})

		v.jobs.Add(1)
		go func() {
			defer v.jobs.Done()
			defer atomic.AddInt32(&activeJobsCount, -1)
			result, err := job.RunFunction(funcName, params)
			if err != nil {
				fmt.Printf("FWJOBSTART(%s) error: %v\n", funcName, err)
				result = advplrt.NewString("")
			}
			v.jobResults.Store(jobID, &asyncJobResult{done: true, value: result})
		}()

		return advplrt.NewString(jobID), nil
	}

	// FWJOBDONE(cJobId) -> lDone
	// .T. quando o job já terminou (valor disponível via FWJOBPOLL) ou
	// quando cJobId é desconhecido/já consumido (fail-open: evita loop
	// infinito de polling em cima de um id inválido). .F. enquanto ainda
	// está rodando. Não é destrutivo (não faz Delete) — só FWJOBPOLL remove
	// a entrada. Existe para separar "terminou?" de "qual é o valor?",
	// porque o valor de retorno legítimo de uma função-alvo pode ser Nil
	// (mesmo ValType "U" usado por FWJOBPOLL para "ainda pendente").
	natives["FWJOBDONE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		jobID := advplrt.ToString(getArg(args, 0))
		raw, ok := v.jobResults.Load(jobID)
		if !ok {
			return advplrt.NewBool(true), nil
		}
		r := raw.(*asyncJobResult)
		return advplrt.NewBool(r.done), nil
	}

	// FWJOBPOLL(cJobId) -> uResultado
	// Nil enquanto pendente/desconhecido. Valor pronto (uma única vez —
	// remove a entrada ao ler) assim que o job termina. O valor pronto pode
	// legitimamente ser Nil (função-alvo com "Return Nil"/"Return"); use
	// FWJOBDONE para saber se já pode chamar FWJOBPOLL para consumir.
	natives["FWJOBPOLL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		jobID := advplrt.ToString(getArg(args, 0))
		raw, ok := v.jobResults.Load(jobID)
		if !ok {
			return advplrt.Nil, nil
		}
		r := raw.(*asyncJobResult)
		if !r.done {
			return advplrt.Nil, nil
		}
		v.jobResults.Delete(jobID)
		return r.value, nil
	}
}
