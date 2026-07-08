package vm

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// gridProc é o estado Go da classe FWGridProcess (TDN: interface padrão de
// processamento em grid — partes unitárias sem dependência entre si,
// despachadas para um pool de threads).
//
// Neste runtime headless não há a interface gráfica de configuração: a
// classe implementa a semântica de processamento (pool limitado por
// SetThreadGrid, StopExecute, IsFinished, meters e log em memória).
type gridProc struct {
	funName     string
	title       string
	description string
	process     advplrt.Value // bProcess: bloco {|lEnd| ...}
	perg        string
	gridFunc    string // função executada nas threads do grid
	saveLog     bool

	threads    int
	maxThreads int
	sem        chan struct{} // limita threads simultâneas do grid
	wg         sync.WaitGroup
	pending    atomic.Int64
	stopped    atomic.Bool
	activated  bool
	abort      bool
	afterExec  advplrt.Value

	mu      sync.Mutex
	lastLog string
	meters  map[int]*gridMeter
}

type gridMeter struct {
	max, cur int
	msg      string
}

func newGridObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("FWGridProcess", nil)
	obj.Native = &gridProc{
		threads: 1,
		meters:  map[int]*gridMeter{},
	}
	return obj
}

// newGridWorkerVM cria o VM isolado de uma thread do grid (semântica de
// work process: memória própria, conexão de banco própria).
func (v *VM) newGridWorkerVM() *VM {
	job := NewVM(v.bc, false)
	job.dbFactory = v.dbFactory
	if v.dbFactory != nil {
		job.dbEngine = v.dbFactory()
	}
	return job
}

// evalBlock avalia um codeblock em um VM filho (os codeblocks deste runtime
// não capturam variáveis externas; recebem apenas seus parâmetros).
func (v *VM) evalBlock(cb advplrt.Value, args ...advplrt.Value) (advplrt.Value, error) {
	block, ok := cb.(*advplrt.CodeBlockValue)
	if !ok {
		return advplrt.Nil, fmt.Errorf("FWGridProcess: bProcess não é um bloco de código")
	}
	job := v.newGridWorkerVM()
	// convenção do OP_EVAL_CODEBLOCK: locals[0] = o próprio bloco
	return job.RunFunction(block.FuncName, append([]advplrt.Value{cb}, args...))
}

func (v *VM) callGridProcessMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	g, ok := obj.Native.(*gridProc)
	if !ok {
		return fmt.Errorf("FWGridProcess: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		// New(cFunName, cTitle, cDescription, bProcess, cPerg, cGrid, lSaveLog)
		g.funName = advplrt.ToString(getArg(args, 0))
		g.title = advplrt.ToString(getArg(args, 1))
		g.description = advplrt.ToString(getArg(args, 2))
		g.process = getArg(args, 3)
		g.perg = advplrt.ToString(getArg(args, 4))
		g.gridFunc = advplrt.ToString(getArg(args, 5))
		g.saveLog = advplrt.ToBool(getArg(args, 6))
		v.push(obj)

	case "SETTHREADGRID":
		n := int(advplrt.ToFloat(getArg(args, 0)))
		if n < 1 {
			n = 1
		}
		if g.maxThreads > 0 && n > g.maxThreads {
			n = g.maxThreads
		}
		g.threads = n
		v.push(advplrt.NewNumber(float64(n)))

	case "SETMAXTHREADGRID":
		g.maxThreads = int(advplrt.ToFloat(getArg(args, 0)))
		if g.maxThreads > 0 && g.threads > g.maxThreads {
			g.threads = g.maxThreads
		}
		v.push(advplrt.Nil)

	case "ACTIVATE", "EXECUTE":
		// Sem UI de configuração: ativa e executa bProcess sincronamente,
		// depois espera as threads do grid pendentes e roda o AfterExecute.
		g.activated = true
		g.stopped.Store(false)
		if g.sem == nil {
			g.sem = make(chan struct{}, g.threads)
		}
		var err error
		if g.process != nil {
			if _, ok := g.process.(*advplrt.CodeBlockValue); ok {
				_, err = v.evalBlock(g.process, advplrt.False)
			}
		}
		g.wg.Wait()
		if g.afterExec != nil {
			if _, ok := g.afterExec.(*advplrt.CodeBlockValue); ok {
				_, _ = v.evalBlock(g.afterExec)
			}
		}
		if err != nil {
			return err
		}
		v.push(advplrt.Nil)

	case "CALLEXECUTE":
		// Despacha a função de grid para o pool de threads. Retorna .F. se
		// o processamento foi interrompido (StopExecute), .T. caso contrário.
		if g.gridFunc == "" {
			return fmt.Errorf("FWGridProcess: cGrid não configurado no New()")
		}
		if g.stopped.Load() {
			v.push(advplrt.False)
			return nil
		}
		if g.sem == nil {
			g.sem = make(chan struct{}, g.threads)
		}
		params := make([]advplrt.Value, len(args))
		copy(params, args)
		worker := v.newGridWorkerVM()
		// captura o canal atual: se g.sem for trocado depois, o release
		// precisa devolver a vaga ao MESMO canal de onde a adquiriu
		sem := g.sem
		g.wg.Add(1)
		g.pending.Add(1)
		sem <- struct{}{} // backpressure: bloqueia quando o pool está cheio
		go func() {
			defer g.wg.Done()
			defer g.pending.Add(-1)
			defer func() { <-sem }()
			if g.stopped.Load() {
				return
			}
			if _, err := worker.RunFunction(g.gridFunc, params); err != nil {
				fmt.Printf("FWGridProcess(%s) thread error: %v\n", g.gridFunc, err)
			}
		}()
		v.push(advplrt.NewBool(!g.stopped.Load()))

	case "STOPEXECUTE":
		// ponytail: SetAbort(.F.) não bloqueia parada programática aqui —
		// no Protheus ele só esconde o botão de cancelar da interface
		g.stopped.Store(true)
		v.push(advplrt.Nil)

	case "ISFINISHED":
		v.push(advplrt.NewBool(g.pending.Load() == 0 && !g.stopped.Load()))

	case "SETABORT":
		g.abort = advplrt.ToBool(getArg(args, 0))
		v.push(advplrt.NewBool(g.abort))

	case "SETAFTEREXECUTE":
		g.afterExec = getArg(args, 0)
		v.push(advplrt.Nil)

	case "SETMETERS":
		n := int(advplrt.ToFloat(getArg(args, 0)))
		g.mu.Lock()
		for i := 1; i <= n; i++ {
			if g.meters[i] == nil {
				g.meters[i] = &gridMeter{}
			}
		}
		g.mu.Unlock()
		v.push(advplrt.NewNumber(float64(n)))

	case "SETMAXMETER":
		nMax := int(advplrt.ToFloat(getArg(args, 0)))
		nMeter := int(advplrt.ToFloat(getArg(args, 1)))
		if nMeter < 1 {
			nMeter = 1
		}
		g.mu.Lock()
		if g.meters[nMeter] == nil {
			g.meters[nMeter] = &gridMeter{}
		}
		g.meters[nMeter].max = nMax
		g.meters[nMeter].cur = 0
		g.meters[nMeter].msg = advplrt.ToString(getArg(args, 2))
		g.mu.Unlock()
		v.push(advplrt.Nil)

	case "SETINCMETER":
		nMeter := int(advplrt.ToFloat(getArg(args, 0)))
		if nMeter < 1 {
			nMeter = 1
		}
		g.mu.Lock()
		if g.meters[nMeter] == nil {
			g.meters[nMeter] = &gridMeter{}
		}
		g.meters[nMeter].cur++
		if msg := advplrt.ToString(getArg(args, 1)); msg != "" {
			g.meters[nMeter].msg = msg
		}
		g.mu.Unlock()
		v.push(advplrt.Nil)

	case "SAVELOG":
		msg := advplrt.ToString(getArg(args, 0))
		g.mu.Lock()
		g.lastLog = msg
		g.mu.Unlock()
		if g.saveLog {
			fmt.Printf("[FWGridProcess:%s] %s\n", g.funName, msg)
		}
		v.push(advplrt.Nil)

	case "GETLASTLOG":
		g.mu.Lock()
		last := g.lastLog
		g.mu.Unlock()
		v.push(advplrt.NewString(last))

	case "SETNOPARAM", "DEACTIVATE":
		if method == "DEACTIVATE" {
			g.activated = false
		}
		v.push(advplrt.Nil)

	default:
		return fmt.Errorf("unknown method %s on FWGridProcess", strings.ToLower(method))
	}
	return nil
}
