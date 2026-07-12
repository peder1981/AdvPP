package dap

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/vm"
)

// CompileFunc compila um fonte AdvPL/TLPP em bytecode — injetada pelo
// chamador (cmd/advplc) para evitar que este pacote dependa de
// lexer/parser/preprocessor diretamente.
type CompileFunc func(sourceFile string) (*compiler.Bytecode, error)

// AttachRuntime conecta recursos externos ao VM antes do Run() (banco de
// dados compartilhado, etc.) — mesma função usada por run/serve.
type AttachRuntime func(*vm.VM)

// Server implementa uma sessão de debug DAP, em dois modos:
//
//   - "launch" (advplc debug, sobre stdio): compila e executa um único
//     fonte, dono do ciclo de vida completo da VM.
//   - "attach" (advplc serve --debug-port, sobre TCP): não compila nada —
//     cada sessão de browser já cria sua própria VM (webui.RunFunc); este
//     servidor só se registra como "pronto para depurar a próxima sessão"
//     via OfferSession. Só uma sessão por vez é depurada; uma segunda aba
//     de browser conectando durante uma depuração roda normalmente, sem
//     debugger anexado — trade-off deliberado (ver README).
type Server struct {
	conn    *Conn
	compile CompileFunc
	attach  AttachRuntime

	isAttachMode bool

	mu          sync.Mutex
	sourcePath  string
	dbg         *vm.Debugger
	vmRef       *vm.VM
	attachReady bool
}

func NewServer(conn *Conn, compile CompileFunc, attach AttachRuntime) *Server {
	return &Server{conn: conn, compile: compile, attach: attach}
}

// NewAttachServer cria um Server em modo "attach" — usado por
// `advplc serve --debug-port`, uma instância por conexão DAP recebida no
// listener TCP. Não compila nem executa nada diretamente; OfferSession é
// quem entrega VMs de sessões de browser reais para serem depuradas.
func NewAttachServer(conn *Conn, sourcePath string) *Server {
	return &Server{conn: conn, isAttachMode: true, sourcePath: sourcePath}
}

// OfferSession oferece v para depuração se um cliente DAP anexado estiver
// pronto (attach + configurationDone já ocorreram) e nenhuma outra sessão
// estiver sendo depurada agora. Se aceito (claimed=true), o chamador DEVE
// chamar v.Run() ele mesmo (já com o Debugger anexado) e, ao final, chamar
// release(err) para notificar o cliente DAP e liberar a próxima sessão.
func (s *Server) OfferSession(v *vm.VM) (claimed bool, release func(error)) {
	s.mu.Lock()
	if !s.isAttachMode || !s.attachReady || s.vmRef != nil {
		s.mu.Unlock()
		return false, nil
	}
	s.vmRef = v
	s.mu.Unlock()

	v.AttachDebugger(s.dbg)
	return true, func(runErr error) {
		if runErr != nil {
			s.conn.SendEvent("output", map[string]any{"category": "stderr", "output": runErr.Error() + "\n"})
		}
		s.conn.SendEvent("terminated", nil)
		s.conn.SendEvent("exited", map[string]any{"exitCode": 0})
		s.mu.Lock()
		s.vmRef = nil
		s.mu.Unlock()
	}
}

// Run processa mensagens até o cliente desconectar ou o stdin fechar.
func (s *Server) Run() error {
	for {
		env, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		if env.Type != "request" {
			continue
		}
		if s.handle(env) {
			return nil // disconnect/terminate pediu encerramento
		}
	}
}

func (s *Server) handle(env *Envelope) (stop bool) {
	switch env.Command {
	case "initialize":
		s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{
			"supportsConfigurationDoneRequest": true,
		})
		s.conn.SendEvent("initialized", nil)

	case "launch":
		s.handleLaunch(env)

	case "attach":
		s.handleAttach(env)

	case "setBreakpoints":
		s.handleSetBreakpoints(env)

	case "configurationDone":
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)
		if s.isAttachMode {
			s.mu.Lock()
			s.attachReady = true
			s.mu.Unlock()
		} else {
			s.startExecution()
		}

	case "threads":
		s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{
			"threads": []map[string]any{{"id": 1, "name": "main"}},
		})

	case "stackTrace":
		s.handleStackTrace(env)

	case "scopes":
		s.handleScopes(env)

	case "variables":
		s.handleVariables(env)

	case "continue":
		s.dbg.Continue()
		s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{"allThreadsContinued": true})

	case "next":
		s.dbg.Next(s.vmRef)
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)

	case "stepIn":
		s.dbg.StepIn(s.vmRef)
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)

	case "stepOut":
		s.dbg.StepOut(s.vmRef)
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)

	case "pause":
		s.dbg.RequestPause()
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)

	case "disconnect", "terminate":
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)
		return true

	default:
		// Comando não implementado: responde sucesso vazio em vez de travar
		// o cliente — DAP tolera respostas vazias para requests opcionais.
		s.conn.SendResponse(env.Seq, env.Command, true, "", nil)
	}
	return false
}

type launchArgs struct {
	Program     string `json:"program"`
	StopOnEntry bool   `json:"stopOnEntry"`
}

func (s *Server) handleLaunch(env *Envelope) {
	var args launchArgs
	json.Unmarshal(env.Arguments, &args)
	s.sourcePath = args.Program

	bc, err := s.compile(args.Program)
	if err != nil {
		s.conn.SendResponse(env.Seq, env.Command, false, fmt.Sprintf("compile error: %v", err), nil)
		s.conn.SendEvent("terminated", nil)
		return
	}

	v := vm.NewVM(bc, false)
	if s.attach != nil {
		s.attach(v)
	}
	v.SetOutputWriter(&outputWriter{conn: s.conn})

	dbg := vm.NewDebugger()
	dbg.SetStopOnEntry(args.StopOnEntry)
	dbg.OnStop = func(reason string, line int) {
		s.conn.SendEvent("stopped", map[string]any{
			"reason":            reason,
			"threadId":          1,
			"allThreadsStopped": true,
			"line":              line,
		})
	}
	v.AttachDebugger(dbg)

	s.vmRef = v
	s.dbg = dbg

	s.conn.SendResponse(env.Seq, env.Command, true, "", nil)
}

// handleAttach cria o Debugger que OfferSession vai anexar à próxima sessão
// de browser oferecida pelo webui — não compila nem cria VM nenhuma aqui,
// isso já existe e é dono do webui.RunFunc.
func (s *Server) handleAttach(env *Envelope) {
	dbg := vm.NewDebugger()
	dbg.OnStop = func(reason string, line int) {
		s.conn.SendEvent("stopped", map[string]any{
			"reason":            reason,
			"threadId":          1,
			"allThreadsStopped": true,
			"line":              line,
		})
	}
	s.dbg = dbg
	s.conn.SendResponse(env.Seq, env.Command, true, "", nil)
}

func (s *Server) handleSetBreakpoints(env *Envelope) {
	var args struct {
		Breakpoints []struct {
			Line int `json:"line"`
		} `json:"breakpoints"`
	}
	json.Unmarshal(env.Arguments, &args)

	lines := make([]int, 0, len(args.Breakpoints))
	verified := make([]map[string]any, 0, len(args.Breakpoints))
	for _, bp := range args.Breakpoints {
		lines = append(lines, bp.Line)
		verified = append(verified, map[string]any{"verified": true, "line": bp.Line})
	}
	if s.dbg != nil {
		s.dbg.SetBreakpoints(lines)
	}
	s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{"breakpoints": verified})
}

// startExecution roda o VM em background depois de configurationDone —
// nesse ponto os breakpoints já foram registrados pelo cliente.
func (s *Server) startExecution() {
	if s.vmRef == nil {
		return
	}
	go func() {
		_, err := s.vmRef.Run()
		if err != nil {
			s.conn.SendEvent("output", map[string]any{"category": "stderr", "output": err.Error() + "\n"})
		}
		s.conn.SendEvent("terminated", nil)
		s.conn.SendEvent("exited", map[string]any{"exitCode": 0})
	}()
}

func (s *Server) handleStackTrace(env *Envelope) {
	frames := s.vmRef.DebugStackFrames()
	out := make([]map[string]any, 0, len(frames))
	for i, f := range frames {
		out = append(out, map[string]any{
			"id":   i,
			"name": f.Name,
			"line": f.Line,
			"column": 1,
			"source": map[string]any{
				"name": filepath.Base(s.sourcePath),
				"path": s.sourcePath,
			},
		})
	}
	s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{
		"stackFrames": out,
		"totalFrames": len(out),
	})
}

func (s *Server) handleScopes(env *Envelope) {
	var args struct {
		FrameId int `json:"frameId"`
	}
	json.Unmarshal(env.Arguments, &args)
	s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{
		"scopes": []map[string]any{
			{"name": "Locals", "variablesReference": 1000 + args.FrameId, "expensive": false},
		},
	})
}

func (s *Server) handleVariables(env *Envelope) {
	var args struct {
		VariablesReference int `json:"variablesReference"`
	}
	json.Unmarshal(env.Arguments, &args)
	frameIndex := args.VariablesReference - 1000

	vars := s.vmRef.DebugLocals(frameIndex)
	out := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		out = append(out, map[string]any{
			"name":               v.Name,
			"value":              v.Value,
			"type":               v.Type,
			"variablesReference": 0,
		})
	}
	s.conn.SendResponse(env.Seq, env.Command, true, "", map[string]any{"variables": out})
}

// OutputWriter expõe um io.Writer que espelha texto como eventos DAP
// "output" — o webui usa isso (via io.MultiWriter, junto do OutWriter do
// browser) pra refletir ConOut também no Debug Console durante attach.
func (s *Server) OutputWriter() io.Writer {
	return &outputWriter{conn: s.conn}
}

// outputWriter espelha ConOut/print do programa debugado como eventos DAP
// "output", pro console de debug do editor.
type outputWriter struct{ conn *Conn }

func (w *outputWriter) Write(p []byte) (int, error) {
	w.conn.SendEvent("output", map[string]any{"category": "stdout", "output": string(p)})
	return len(p), nil
}
