package vm

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"golang.org/x/term"
)

// inputHistoryLimit é o tamanho máximo do histórico de linhas do ConIn
// (setas up/down), no mesmo espírito do histórico de shell do bash/zsh e
// de harnesses como opencode/claude CLI.
const inputHistoryLimit = 20

// pushHistory adiciona uma linha não vazia ao histórico do ConIn, ignorando
// repetição consecutiva (mesmo comportamento do HISTCONTROL=ignoredups do
// bash) e mantendo só as últimas inputHistoryLimit entradas.
func (v *VM) pushHistory(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if n := len(v.inputHistory); n > 0 && v.inputHistory[n-1] == line {
		return
	}
	v.inputHistory = append(v.inputHistory, line)
	if len(v.inputHistory) > inputHistoryLimit {
		v.inputHistory = v.inputHistory[len(v.inputHistory)-inputHistoryLimit:]
	}
}

// readLine implementa o ConIn nativo. Em stdin não-interativo (pipe/redirect,
// como os testes `printf ... | ./shortcoder`), mantém o comportamento antigo
// de ReadString('\n') inalterado. Em terminal interativo, entra em raw mode
// e roda um mini line-editor com atalhos comuns de shell/readline: setas
// up/down para navegar o histórico, left/right/Home/End/Ctrl+A/Ctrl+E para
// mover o cursor, Backspace/Delete, Ctrl+U/Ctrl+K para apagar até o
// início/fim da linha, Ctrl+L para limpar a tela e Ctrl+C para cancelar a
// linha atual (equivalente a apertar Enter em branco).
func (v *VM) readLine(prompt string) (advplrt.Value, error) {
	if prompt != "" {
		fmt.Fprint(stdoutW, prompt)
	}

	fd := int(os.Stdin.Fd())
	if v.stdinReader == nil {
		v.stdinReader = bufio.NewReader(os.Stdin)
	}
	reader := v.stdinReader

	if !term.IsTerminal(fd) {
		return v.readLinePlain(reader)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Sem raw mode disponível (ex: fd redirecionado de forma incomum) —
		// cai pro caminho antigo em vez de travar o programa.
		return v.readLinePlain(reader)
	}
	defer term.Restore(fd, oldState)

	buf := []rune{}
	cursor := 0
	histIdx := len(v.inputHistory) // == len(history) significa "linha atual, fora do histórico"
	saved := ""

	redraw := func() {
		fmt.Fprint(stdoutW, "\r\x1b[K", prompt, string(buf))
		if back := len(buf) - cursor; back > 0 {
			fmt.Fprintf(stdoutW, "\x1b[%dD", back)
		}
	}

	insertRune := func(r rune) {
		buf = append(buf, 0)
		copy(buf[cursor+1:], buf[cursor:])
		buf[cursor] = r
		cursor++
	}

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			term.Restore(fd, oldState)
			fmt.Fprintln(stdoutW)
			if len(buf) == 0 {
				return advplrt.Nil, nil
			}
			line := string(buf)
			v.pushHistory(line)
			return advplrt.NewString(line), nil
		}

		switch r {
		case '\r', '\n':
			fmt.Fprintln(stdoutW)
			line := string(buf)
			v.pushHistory(line)
			return advplrt.NewString(line), nil

		case 3: // Ctrl+C: cancela a linha, não sai do programa
			fmt.Fprintln(stdoutW, "^C")
			return advplrt.NewString(""), nil

		case 4: // Ctrl+D: EOF só se a linha estiver vazia (igual bash)
			if len(buf) == 0 {
				fmt.Fprintln(stdoutW)
				return advplrt.Nil, nil
			}

		case 127, 8: // Backspace
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
				redraw()
			}

		case 1: // Ctrl+A -> Home
			cursor = 0
			redraw()

		case 5: // Ctrl+E -> End
			cursor = len(buf)
			redraw()

		case 21: // Ctrl+U -> apaga do início da linha até o cursor
			if cursor > 0 {
				buf = buf[cursor:]
				cursor = 0
				redraw()
			}

		case 11: // Ctrl+K -> apaga do cursor até o fim da linha
			if cursor < len(buf) {
				buf = buf[:cursor]
				redraw()
			}

		case 12: // Ctrl+L -> limpa a tela, mantém a linha em edição
			fmt.Fprint(stdoutW, "\x1b[2J\x1b[H")
			redraw()

		case 27: // ESC — possível sequência de seta/Home/End
			b1, _, err1 := reader.ReadRune()
			if err1 != nil {
				continue
			}
			if b1 != '[' && b1 != 'O' {
				continue
			}
			b2, _, err2 := reader.ReadRune()
			if err2 != nil {
				continue
			}
			switch b2 {
			case 'A': // Up
				if histIdx > 0 {
					if histIdx == len(v.inputHistory) {
						saved = string(buf)
					}
					histIdx--
					buf = []rune(v.inputHistory[histIdx])
					cursor = len(buf)
					redraw()
				}
			case 'B': // Down
				if histIdx < len(v.inputHistory) {
					histIdx++
					if histIdx == len(v.inputHistory) {
						buf = []rune(saved)
					} else {
						buf = []rune(v.inputHistory[histIdx])
					}
					cursor = len(buf)
					redraw()
				}
			case 'C': // Right
				if cursor < len(buf) {
					cursor++
					redraw()
				}
			case 'D': // Left
				if cursor > 0 {
					cursor--
					redraw()
				}
			case 'H': // Home (xterm)
				cursor = 0
				redraw()
			case 'F': // End (xterm)
				cursor = len(buf)
				redraw()
			case '1', '3', '4', '7', '8': // Home/End/Delete (formato "ESC [ n ~")
				b3, _, err3 := reader.ReadRune()
				if err3 != nil || b3 != '~' {
					continue
				}
				switch b2 {
				case '3': // Delete
					if cursor < len(buf) {
						buf = append(buf[:cursor], buf[cursor+1:]...)
						redraw()
					}
				case '1', '7': // Home
					cursor = 0
					redraw()
				case '4', '8': // End
					cursor = len(buf)
					redraw()
				}
			}

		default:
			if r >= 32 || r == '\t' {
				insertRune(r)
				redraw()
			}
		}
	}
}

// readLinePlain é o comportamento histórico do ConIn (stdin não-interativo):
// lê uma linha crua até '\n', sem edição nem histórico.
func (v *VM) readLinePlain(reader *bufio.Reader) (advplrt.Value, error) {
	line, err := reader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && line == "" {
		return advplrt.Nil, nil
	}
	v.pushHistory(line)
	return advplrt.NewString(line), nil
}
