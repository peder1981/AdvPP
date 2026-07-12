// Package dap implementa a camada de transporte do Debug Adapter Protocol
// (DAP) sobre stdio: frames "Content-Length: N\r\n\r\n<N bytes de JSON>",
// igual ao LSP. Só a camada de framing/leitura/escrita — o dispatch dos
// comandos DAP específicos do AdvPP fica em server.go. Stdlib puro, mesmo
// padrão do pkg/mcp.
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Envelope é o formato mínimo comum às três formas de mensagem DAP
// (request/response/event), usado só para dispatch inicial.
type Envelope struct {
	Seq        int             `json:"seq"`
	Type       string          `json:"type"`
	Command    string          `json:"command,omitempty"`
	Event      string          `json:"event,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
	RequestSeq int             `json:"request_seq,omitempty"`
	Success    bool            `json:"success,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// Conn lê/escreve mensagens DAP com framing Content-Length sobre um
// io.Reader/io.Writer (tipicamente os.Stdin/os.Stdout).
type Conn struct {
	r      *bufio.Reader
	w      io.Writer
	wMu    sync.Mutex
	seqCtr int32
}

func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{r: bufio.NewReader(r), w: w}
}

// ReadMessage bloqueia até a próxima mensagem completa chegar, ou retorna
// io.EOF quando o stdin fecha (cliente desconectou).
func (c *Conn) ReadMessage() (*Envelope, error) {
	var contentLength int
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // linha em branco separa headers do corpo
		}
		if strings.HasPrefix(line, "Content-Length:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("dap: Content-Length inválido: %q", v)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("dap: mensagem sem Content-Length")
	}
	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	var env Envelope
	if err := json.Unmarshal(buf, &env); err != nil {
		return nil, fmt.Errorf("dap: JSON inválido: %w", err)
	}
	return &env, nil
}

func (c *Conn) nextSeq() int { return int(atomic.AddInt32(&c.seqCtr, 1)) }

func (c *Conn) write(msg map[string]any) error {
	msg["seq"] = c.nextSeq()
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.wMu.Lock()
	defer c.wMu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// SendResponse envia a resposta a um request de seq requestSeq. body pode
// ser nil (resposta sem corpo, ex.: launch/continue).
func (c *Conn) SendResponse(requestSeq int, command string, success bool, message string, body any) error {
	msg := map[string]any{
		"type":        "response",
		"request_seq": requestSeq,
		"command":     command,
		"success":     success,
	}
	if message != "" {
		msg["message"] = message
	}
	if body != nil {
		msg["body"] = body
	}
	return c.write(msg)
}

// SendEvent envia um evento assíncrono (initialized, stopped, output,
// terminated, exited, thread).
func (c *Conn) SendEvent(event string, body any) error {
	msg := map[string]any{
		"type":  "event",
		"event": event,
	}
	if body != nil {
		msg["body"] = body
	}
	return c.write(msg)
}
