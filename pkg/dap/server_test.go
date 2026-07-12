package dap_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/dap"
	"github.com/advpl/compiler/pkg/lexer"
	"github.com/advpl/compiler/pkg/parser"
	"github.com/advpl/compiler/pkg/preprocessor"
	"github.com/advpl/compiler/pkg/vm"
)

const fixture = `User Function TestDebug()
	Local nX := 1
	Local nY := 2
	nX := nX + nY
	ConOut("done")
Return nX
`

func compileFixture(sourceFile string) (*compiler.Bytecode, error) {
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, err
	}
	pp := preprocessor.NewPreprocessor([]string{filepath.Dir(sourceFile)})
	processed, err := pp.Process(string(source), sourceFile)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.Tokenize(processed, sourceFile)
	if err != nil {
		return nil, err
	}
	p := parser.NewParser(tokens, sourceFile, pp.GetDefines())
	prog, err := p.Parse()
	if err != nil {
		return nil, err
	}
	return compiler.Compile(prog)
}

// writeRequest frameia e escreve um request DAP cru, do ponto de vista do
// "cliente" (editor) — testa o framing real, não só o dispatch em memória.
func writeRequest(w io.Writer, seq int, command string, args any) error {
	msg := map[string]any{"seq": seq, "type": "request", "command": command}
	if args != nil {
		msg["arguments"] = args
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func TestDebugSessionBreakpointStepVariables(t *testing.T) {
	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "test.prw")
	if err := os.WriteFile(sourceFile, []byte(fixture), 0644); err != nil {
		t.Fatal(err)
	}

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	serverConn := dap.NewConn(serverR, serverW)
	srv := dap.NewServer(serverConn, compileFixture, func(*vm.VM) {})

	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Run() }()

	clientConn := dap.NewConn(clientR, io.Discard)
	events := make(chan *dap.Envelope, 32)
	go func() {
		for {
			env, err := clientConn.ReadMessage()
			if err != nil {
				close(events)
				return
			}
			events <- env
		}
	}()

	nextEvent := func(t *testing.T, want string) *dap.Envelope {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case env, ok := <-events:
				if !ok {
					t.Fatalf("event stream closed waiting for %q", want)
				}
				name := env.Command
				if env.Type == "event" {
					name = env.Event
				}
				if name == want {
					return env
				}
			case <-deadline:
				t.Fatalf("timed out waiting for %q", want)
			}
		}
	}

	seq := 0
	send := func(command string, args any) {
		seq++
		if err := writeRequest(clientW, seq, command, args); err != nil {
			t.Fatal(err)
		}
	}

	send("initialize", map[string]any{})
	nextEvent(t, "initialize")  // response
	nextEvent(t, "initialized") // event

	send("launch", map[string]any{"program": sourceFile, "stopOnEntry": false})
	nextEvent(t, "launch")

	// Breakpoint na linha "nX := nX + nY" (linha 4 do fixture).
	send("setBreakpoints", map[string]any{
		"breakpoints": []map[string]any{{"line": 4}},
	})
	bpResp := nextEvent(t, "setBreakpoints")
	var bpBody struct {
		Breakpoints []struct {
			Verified bool `json:"verified"`
			Line     int  `json:"line"`
		} `json:"breakpoints"`
	}
	json.Unmarshal(bpResp.Body, &bpBody)
	if len(bpBody.Breakpoints) != 1 || !bpBody.Breakpoints[0].Verified || bpBody.Breakpoints[0].Line != 4 {
		t.Fatalf("unexpected setBreakpoints response: %+v", bpBody)
	}

	send("configurationDone", nil)
	nextEvent(t, "configurationDone")

	stopped := nextEvent(t, "stopped")
	var stoppedBody struct {
		Reason string `json:"reason"`
		Line   int    `json:"line"`
	}
	json.Unmarshal(stopped.Body, &stoppedBody)
	if stoppedBody.Reason != "breakpoint" || stoppedBody.Line != 4 {
		t.Fatalf("expected stop at breakpoint line 4, got %+v", stoppedBody)
	}

	send("stackTrace", map[string]any{"threadId": 1})
	stResp := nextEvent(t, "stackTrace")
	var stBody struct {
		StackFrames []struct {
			Name string `json:"name"`
			Line int    `json:"line"`
		} `json:"stackFrames"`
	}
	json.Unmarshal(stResp.Body, &stBody)
	if len(stBody.StackFrames) == 0 || stBody.StackFrames[0].Line != 4 {
		t.Fatalf("unexpected stackTrace: %+v", stBody)
	}

	send("scopes", map[string]any{"frameId": 0})
	scResp := nextEvent(t, "scopes")
	var scBody struct {
		Scopes []struct {
			VariablesReference int `json:"variablesReference"`
		} `json:"scopes"`
	}
	json.Unmarshal(scResp.Body, &scBody)
	if len(scBody.Scopes) == 0 {
		t.Fatal("no scopes returned")
	}

	send("variables", map[string]any{"variablesReference": scBody.Scopes[0].VariablesReference})
	varResp := nextEvent(t, "variables")
	var varBody struct {
		Variables []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	json.Unmarshal(varResp.Body, &varBody)
	found := map[string]string{}
	for _, v := range varBody.Variables {
		found[v.Name] = v.Value
	}
	// Antes de executar a linha 4, nX ainda deve ser 1 (não 3) — prova que a
	// pausa acontece ANTES da instrução, não depois.
	if found["nX"] != "1" || found["nY"] != "2" {
		t.Fatalf("unexpected locals at breakpoint: %+v", found)
	}

	send("continue", map[string]any{"threadId": 1})
	nextEvent(t, "continue")

	nextEvent(t, "terminated")
	nextEvent(t, "exited")

	send("disconnect", nil)
	select {
	case err := <-serverDone:
		if err != nil && err != io.ErrClosedPipe {
			t.Fatalf("server exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down after disconnect")
	}

	clientW.Close()
}
