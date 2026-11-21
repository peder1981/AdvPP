//go:build windows

package vm

import (
	"errors"
	"io"
)

// ptyDisponivel: sem pseudo-terminal no Windows nesta versão (ConPTY fica
// para depois); AdvTerminal:New devolve erro claro.
const ptyDisponivel = false

func abrirPty(string, []string) (poPtyProc, io.Reader, func() int, error) {
	return nil, nil, nil, errors.New("AdvTerminal: terminal indisponível no Windows nesta versão")
}
