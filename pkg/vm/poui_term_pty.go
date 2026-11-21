//go:build !windows

package vm

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// ptyDisponivel: há pseudo-terminal nesta plataforma.
const ptyDisponivel = true

type ptyProc struct {
	f   *os.File
	cmd *exec.Cmd
}

func (p *ptyProc) Write(b []byte) (int, error) { return p.f.Write(b) }

func (p *ptyProc) Resize(cols, rows uint16) error {
	return pty.Setsize(p.f, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *ptyProc) Kill() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

// abrirPty roda prog SEM shell num pty 80x24. Devolve o processo, a saída e
// a espera pelo fim (código de saída; -1 se morto/sem código).
func abrirPty(prog string, args []string) (poPtyProc, io.Reader, func() int, error) {
	cmd := exec.Command(prog, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return nil, nil, nil, err
	}
	espera := func() int {
		err := cmd.Wait()
		f.Close()
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() >= 0 {
			return ee.ExitCode()
		}
		return -1
	}
	return &ptyProc{f: f, cmd: cmd}, f, espera, nil
}
