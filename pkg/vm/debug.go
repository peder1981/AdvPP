package vm

import (
	"sync"

	"github.com/advpl/compiler/pkg/compiler"
)

// StepMode indica o que o runLoop deve fazer até a próxima parada.
type StepMode int

const (
	StepNone StepMode = iota
	StepOver
	StepIn
	StepOut
)

// Debugger é um hook opcional plugado num VM (AttachDebugger) que pausa a
// runLoop em breakpoints e passos de linha, expondo o estado do frame atual
// para inspeção enquanto pausado. Sem Debugger anexado (caso comum: run,
// compile, serve), o custo é um único `if v.debugger != nil` por instrução.
//
// Só cobre a runLoop principal — o mini-loop de tryOperatorOverload (chamada
// de método dentro de overload de operador) não passa pelo hook, então
// stepping não entra nesses métodos. Granularidade é por linha-fonte, um
// único arquivo ativo por sessão (sem stepping entre múltiplos fontes).
type Debugger struct {
	mu             sync.Mutex
	breakpoints    map[int]bool
	stepMode       StepMode
	stepDepth      int
	stepFromLine   int
	lastLine       int
	pauseRequested bool
	stopOnEntry    bool
	entrySeen      bool
	resumeCh       chan struct{}

	// OnStop é chamado (na goroutine da VM) sempre que a execução pausa.
	// O handler deve inspecionar o estado via VM.DebugStackFrames /
	// VM.DebugLocals e eventualmente chamar Continue/Next/StepIn/StepOut.
	OnStop func(reason string, line int)
}

func NewDebugger() *Debugger {
	return &Debugger{
		breakpoints: make(map[int]bool),
		resumeCh:    make(chan struct{}),
		lastLine:    -1,
	}
}

// AttachDebugger liga o Debugger ao VM. Deve ser chamado antes de Run().
func (v *VM) AttachDebugger(d *Debugger) { v.debugger = d }

func (d *Debugger) SetBreakpoints(lines []int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.breakpoints = make(map[int]bool, len(lines))
	for _, l := range lines {
		d.breakpoints[l] = true
	}
}

func (d *Debugger) SetStopOnEntry(v bool) { d.stopOnEntry = v }

// checkBreak roda na goroutine da VM, a cada instrução, quando um Debugger
// está anexado. Bloqueia em resumeCh quando decide pausar.
func (d *Debugger) checkBreak(v *VM, instr compiler.Instruction) {
	line := instr.Line
	if line == 0 {
		return
	}

	d.mu.Lock()
	pauseReq := d.pauseRequested
	d.pauseRequested = false
	d.mu.Unlock()
	if pauseReq {
		d.lastLine = line
		d.pause("pause", line)
		return
	}

	if line == d.lastLine {
		return
	}
	d.lastLine = line

	if !d.entrySeen {
		d.entrySeen = true
		if d.stopOnEntry {
			d.pause("entry", line)
			return
		}
	}

	d.mu.Lock()
	isBp := d.breakpoints[line]
	mode := d.stepMode
	depth := d.stepDepth
	fromLine := d.stepFromLine
	d.mu.Unlock()

	curDepth := len(v.frames)
	shouldStop, reason := false, "breakpoint"
	if isBp {
		shouldStop = true
	} else {
		switch mode {
		case StepOver:
			if curDepth <= depth && line != fromLine {
				shouldStop, reason = true, "step"
			}
		case StepIn:
			if line != fromLine {
				shouldStop, reason = true, "step"
			}
		case StepOut:
			if curDepth < depth && line != fromLine {
				shouldStop, reason = true, "step"
			}
		}
	}

	if shouldStop {
		d.mu.Lock()
		d.stepMode = StepNone
		d.mu.Unlock()
		d.pause(reason, line)
	}
}

func (d *Debugger) pause(reason string, line int) {
	if d.OnStop != nil {
		d.OnStop(reason, line)
	}
	<-d.resumeCh
}

func (d *Debugger) resume(mode StepMode, depth, fromLine int) {
	d.mu.Lock()
	d.stepMode = mode
	d.stepDepth = depth
	d.stepFromLine = fromLine
	d.mu.Unlock()
	d.resumeCh <- struct{}{}
}

func (d *Debugger) Continue()        { d.resume(StepNone, 0, -1) }
func (d *Debugger) Next(v *VM)       { d.resume(StepOver, len(v.frames), d.lastLine) }
func (d *Debugger) StepIn(v *VM)     { d.resume(StepIn, len(v.frames), d.lastLine) }
func (d *Debugger) StepOut(v *VM)    { d.resume(StepOut, len(v.frames)-1, d.lastLine) }
func (d *Debugger) RequestPause()    { d.mu.Lock(); d.pauseRequested = true; d.mu.Unlock() }

// StackFrameInfo é uma entrada de call stack exposta ao adaptador DAP.
// Index 0 é sempre o frame mais interno (topo), como o protocolo espera.
type StackFrameInfo struct {
	Name string
	Line int
}

func (v *VM) DebugStackFrames() []StackFrameInfo {
	out := make([]StackFrameInfo, 0, len(v.frames))
	for i := len(v.frames) - 1; i >= 0; i-- {
		f := v.frames[i]
		line := 0
		if f.IP > 0 && f.IP-1 < len(f.Code) {
			line = f.Code[f.IP-1].Line
		}
		out = append(out, StackFrameInfo{Name: f.FuncName, Line: line})
	}
	return out
}

// VarInfo é uma variável local exposta ao adaptador DAP.
type VarInfo struct {
	Name  string
	Value string
	Type  string
}

// DebugLocals lista as variáveis do frame de índice frameIndex (0 = topo,
// mesma convenção de DebugStackFrames). Nomes vêm de FunctionInfo.ParamNames
// / LocalNames; sem essa metadata (ex.: frame sintético de entrada), cai
// para "localN".
func (v *VM) DebugLocals(frameIndex int) []VarInfo {
	idx := len(v.frames) - 1 - frameIndex
	if idx < 0 || idx >= len(v.frames) {
		return nil
	}
	f := v.frames[idx]

	named := make(map[int]string)
	if info, ok := v.bc.Functions[f.FuncName]; ok {
		for i, p := range info.ParamNames {
			named[i] = p
		}
		for name, i := range info.LocalNames {
			if _, taken := named[i]; !taken {
				named[i] = name
			}
		}
	}

	out := make([]VarInfo, 0, len(f.Locals))
	for i, val := range f.Locals {
		if val == nil {
			continue
		}
		name := named[i]
		if name == "" {
			name = "local"
		}
		out = append(out, VarInfo{Name: name, Value: val.String(), Type: val.Type()})
	}
	return out
}
