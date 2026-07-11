package ui

import "strings"

// ConsoleWriter adapts an OutputConsole to io.Writer, splitting arbitrary
// writes (e.g. VM console output, or a subprocess's combined stdout/stderr)
// into line-based Append calls; a partial trailing line is buffered until
// the next Write completes it.
type ConsoleWriter struct {
	console *OutputConsole
	partial string
}

func NewConsoleWriter(console *OutputConsole) *ConsoleWriter {
	return &ConsoleWriter{console: console}
}

func (w *ConsoleWriter) Write(p []byte) (int, error) {
	w.partial += string(p)
	lines := strings.Split(w.partial, "\n")
	for _, line := range lines[:len(lines)-1] {
		w.console.Append(line)
	}
	w.partial = lines[len(lines)-1]
	return len(p), nil
}
