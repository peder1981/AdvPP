//go:build windows

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/UserExistsError/conpty"
)

// ptySession — see ptysession_posix_test.go for the shared doc comment.
type ptySession interface {
	Write(p []byte) (int, error)
	Read(p []byte) (int, error)
	Wait() error
	Close() error
}

type windowsPTYSession struct {
	cpty *conpty.ConPty
}

// startPTYSession attaches outPath to a real Windows ConPty (the same
// pseudo-console API a real terminal emulator uses) via UserExistsError/conpty
// — pure Go over golang.org/x/sys/windows syscalls, no cgo. commandLine is a
// single string (Win32 CreateProcess convention, not argv); outPath is quoted
// defensively since Go's t.TempDir() can contain spaces on some CI runners.
func startPTYSession(t *testing.T, outPath string, env []string, cols, rows int) ptySession {
	t.Helper()
	cpty, err := conpty.Start(
		fmt.Sprintf("%q", outPath),
		conpty.ConPtyDimensions(cols, rows),
		conpty.ConPtyEnv(env),
	)
	if err != nil {
		t.Fatalf("conpty.Start: %v", err)
	}
	return &windowsPTYSession{cpty: cpty}
}

func (s *windowsPTYSession) Write(p []byte) (int, error) { return s.cpty.Write(p) }
func (s *windowsPTYSession) Read(p []byte) (int, error)  { return s.cpty.Read(p) }

func (s *windowsPTYSession) Wait() error {
	code, err := s.cpty.Wait(context.Background())
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("processo saiu com código %d", code)
	}
	return nil
}

func (s *windowsPTYSession) Close() error { return s.cpty.Close() }
