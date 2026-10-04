// Package dap porta para Go o stack de depuração TDS usado nas ondas 20–43
// do laboratório: token via advpls (LS, JSON-RPC sobre stdio) e sessão via
// debugAdapter-linux (DAP sobre stdio). Referência: dap_lib.py.
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rpcMode int

const (
	rpcLSP rpcMode = iota
	rpcDAP
)

type rpcMsg = map[string]any

// RPC multiplexa requisições/respostas sobre stdio com framing
// "Content-Length: N\r\n\r\n<json>" (idêntico ao StdioRPC do dap_lib.py).
type RPC struct {
	mu      sync.Mutex
	w       io.WriteCloser
	mode    rpcMode
	nextID  int64
	pending map[int64]chan rpcMsg
	notifs  []rpcMsg
	lastErr error
	done    chan struct{}
	once    sync.Once
	cmd     *exec.Cmd
}

func newRPC(r io.Reader, w io.WriteCloser, mode rpcMode, cmd *exec.Cmd) *RPC {
	rpc := &RPC{
		w: w, mode: mode, cmd: cmd,
		pending: map[int64]chan rpcMsg{},
		done:    make(chan struct{}),
	}
	go rpc.readLoop(r)
	return rpc
}

// newRPCProcess spawn cmd e conecta os stdio. O stderr deve estar
// configurado no próprio cmd (arquivo de log do LS/DA).
func newRPCProcess(cmd *exec.Cmd, mode rpcMode) (*RPC, error) {
	cmd.Env = append(cmd.Environ(), "TMPDIR=/tmp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("iniciando %s: %w", cmd.Path, err)
	}
	return newRPC(stdout, stdin, mode, cmd), nil
}

func (r *RPC) readLoop(rd io.Reader) {
	br := bufio.NewReader(rd)
	for {
		headers := map[string]string{}
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				r.fail(err)
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if k, v, ok := strings.Cut(line, ":"); ok {
				headers[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		n, convErr := strconv.Atoi(headers["content-length"])
		if convErr != nil || n <= 0 {
			r.fail(fmt.Errorf("content-length inválido: %q", headers["content-length"]))
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(br, body); err != nil {
			r.fail(err)
			return
		}
		var m rpcMsg
		if err := json.Unmarshal(body, &m); err != nil {
			continue // frame não-JSON: ignora (como o Python)
		}
		r.dispatch(m)
	}
}

// dispatch roda sempre com r.mu segurado.
func (r *RPC) dispatch(m rpcMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mode == rpcDAP {
		if m["type"] == "response" {
			if id, ok := m["request_seq"].(float64); ok {
				if ch, ok := r.pending[int64(id)]; ok {
					ch <- m
				}
			}
			return
		}
		r.notifs = append(r.notifs, m)
		return
	}
	// LSP
	if id, hasID := m["id"]; hasID {
		if _, hasResult := m["result"]; hasResult {
			if ch, ok := r.pending[int64(asFloat(id))]; ok {
				ch <- m
			}
			return
		}
		if _, hasErr := m["error"]; hasErr {
			if ch, ok := r.pending[int64(asFloat(id))]; ok {
				ch <- m
			}
			return
		}
		if _, hasMethod := m["method"]; hasMethod {
			// requisição servidor→cliente: responde null (dap_lib.py)
			_ = r.writeLocked(rpcMsg{"jsonrpc": "2.0", "id": id, "result": nil})
			return
		}
	}
	r.notifs = append(r.notifs, m)
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// writeLocked escreve um frame JSON; r.mu deve estar segurado.
func (r *RPC) writeLocked(m rpcMsg) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(r.w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = r.w.Write(data)
	return err
}

// Request envia uma requisição e aguarda a resposta (timeout ou processo
// encerrado).
func (r *RPC) Request(method string, params rpcMsg, timeout time.Duration) (rpcMsg, error) {
	r.mu.Lock()
	select {
	case <-r.done:
		err := r.rerrLocked()
		r.mu.Unlock()
		return nil, fmt.Errorf("processo encerrado (%s): %w", method, err)
	default:
	}
	r.nextID++
	id := r.nextID
	ch := make(chan rpcMsg, 1)
	r.pending[id] = ch
	payload := rpcMsg{}
	if r.mode == rpcDAP {
		payload = rpcMsg{"seq": id, "type": "request", "command": method, "arguments": params}
	} else {
		payload = rpcMsg{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	}
	err := r.writeLocked(payload)
	r.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("enviando %s: %w", method, err)
	}
	select {
	case m := <-ch:
		return m, nil
	case <-time.After(timeout):
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
		return nil, fmt.Errorf("timeout aguardando %s", method)
	case <-r.done:
		return nil, fmt.Errorf("processo encerrado (aguardando %s)", method)
	}
}

// Notify envia sem aguardar resposta (somente LSP).
func (r *RPC) Notify(method string, params rpcMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if params == nil {
		params = rpcMsg{}
	}
	_ = r.writeLocked(rpcMsg{"jsonrpc": "2.0", "method": method, "params": params})
}

// Drain aguarda secs e retorna até limit notificações acumuladas
// (mesma semântica do drain do dap_lib.py).
func (r *RPC) Drain(secs time.Duration, limit int) []rpcMsg {
	time.Sleep(secs)
	r.mu.Lock()
	defer r.mu.Unlock()
	n := limit
	if n > len(r.notifs) {
		n = len(r.notifs)
	}
	out := make([]rpcMsg, n)
	copy(out, r.notifs[:n])
	r.notifs = r.notifs[n:]
	return out
}

// fail marca o fim do stream (EOF do processo).
func (r *RPC) fail(err error) {
	r.mu.Lock()
	r.lastErr = err
	r.mu.Unlock()
	r.once.Do(func() { close(r.done) })
}

func (r *RPC) rerrLocked() error {
	if r.lastErr != nil {
		return r.lastErr
	}
	return fmt.Errorf("stream fechado")
}

// Close encerra o processo (se houver) e sinaliza os aguardadores.
func (r *RPC) Close() error {
	r.mu.Lock()
	r.lastErr = fmt.Errorf("fechado pelo chamador")
	r.mu.Unlock()
	r.once.Do(func() { close(r.done) })
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Kill()
	}
	return nil
}
