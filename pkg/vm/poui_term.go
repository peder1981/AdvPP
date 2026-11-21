package vm

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// poTerm é o processo de um AdvTerminal num pseudo-terminal (pty). A
// goroutine de leitura envia a saída direto ao browser (op "term") e posta
// "exit" no fim; a entrada e o redimensionamento chegam pelo laço da VM.
type poTerm struct {
	prog     string
	args     []string
	iniciado bool
	parar    chan struct{}
	proc     poPtyProc
}

// poPtyProc é a parte dependente de plataforma (poui_term_pty.go).
type poPtyProc interface {
	io.Writer
	Resize(cols, rows uint16) error
	Kill()
}

// startTerm abre o pty e começa a ler. Roda na goroutine da VM.
func (a *poAppState) startTerm(c *poComp) {
	t := c.term
	if t == nil || t.iniciado {
		return
	}
	t.iniciado = true
	t.parar = make(chan struct{})
	ui, parar, id := a.ui, t.parar, c.id
	proc, saida, espera, err := abrirPty(t.prog, t.args)
	if err != nil {
		a.setProp(c, "running", false)
		go ui.UIPostWait(id, "termexit", []byte("-1"), parar)
		return
	}
	t.proc = proc
	a.setProp(c, "running", true)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := saida.Read(buf)
			if n > 0 {
				data, _ := json.Marshal(map[string]any{"op": "term", "id": id, "data": base64.StdEncoding.EncodeToString(buf[:n])})
				select {
				case <-parar: // laço encerrado: só drena
				default:
					ui.UISend(data)
				}
			}
			if err != nil {
				break
			}
		}
		code := espera()
		ui.UIPostWait(id, "termexit", []byte(strconv.Itoa(code)), parar)
	}()
}

// killTerms encerra os terminais ao fim do laço.
func (a *poAppState) killTerms() {
	for _, c := range a.byID {
		if t := c.term; t != nil && t.parar != nil {
			if t.proc != nil {
				t.proc.Kill()
			}
			close(t.parar)
			t.parar = nil
		}
	}
}

// poTermEvento trata input/resize/termexit de um terminal ("termexit" para
// não colidir com o "exit" dos processos de Spawn).
func (a *poAppState) poTermEvento(c *poComp, ev string, val []byte) (advplrt.Value, []advplrt.Value) {
	t := c.term
	switch ev {
	case "input":
		if t.proc != nil {
			_, _ = t.proc.Write([]byte(poJSONTexto(val)))
		}
	case "resize":
		var tam struct{ Cols, Rows uint16 }
		if json.Unmarshal(val, &tam) == nil && t.proc != nil && tam.Cols > 0 && tam.Rows > 0 {
			_ = t.proc.Resize(tam.Cols, tam.Rows)
		}
	case "termexit":
		code, _ := strconv.Atoi(string(val))
		t.proc = nil
		a.setProp(c, "running", false)
		return c.handlers["exit"], []advplrt.Value{c.obj, advplrt.NewNumber(float64(code))}
	}
	return nil, nil
}
