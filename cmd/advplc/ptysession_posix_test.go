//go:build !windows

package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/creack/pty"
)

// ptySession is the small cross-platform surface TestBuildStandaloneInteractive
// needs: write simulated keystrokes, read back the rendered screen, and wait
// for the process to exit. POSIX gets it from a real pty (creack/pty);
// Windows gets it from ConPty (ptysession_windows_test.go) — same test, same
// coverage, different OS API underneath.
type ptySession interface {
	Write(p []byte) (int, error)
	Read(p []byte) (int, error)
	Wait() error
	Close() error
}

type posixPTYSession struct {
	f   *os.File
	cmd *exec.Cmd
}

func startPTYSession(t *testing.T, outPath string, env []string, cols, rows int) ptySession {
	t.Helper()
	cmd := exec.Command(outPath)
	cmd.Env = env
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		t.Fatalf("pty.StartWithSize: %v", err)
	}
	return &posixPTYSession{f: f, cmd: cmd}
}

func (s *posixPTYSession) Write(p []byte) (int, error) { return s.f.Write(p) }
func (s *posixPTYSession) Read(p []byte) (int, error)  { return s.f.Read(p) }
func (s *posixPTYSession) Wait() error                 { return s.cmd.Wait() }
func (s *posixPTYSession) Close() error                { return s.f.Close() }
