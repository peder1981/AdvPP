package vm

import (
	"fmt"
	"os"
	"strings"
)

// cmdLine é uma linha de comando do WaitRun/WinExec já dividida em argv,
// mais os redirecionamentos de E/S pedidos. Nada aqui passa por shell: não
// há ';', '&&', '|', '$VAR', '*' nem substituição de comando — esses
// caracteres viram texto comum dos argumentos (CWE-78).
type cmdLine struct {
	argv      []string
	stdin     string // < arq
	stdout    string // > arq | >> arq
	stderr    string // 2> arq | 2>> arq
	appendOut bool
	appendErr bool
	errToOut  bool // 2>&1: stderr vai para onde o stdout for
}

type cmdToken struct {
	text   string
	quoted bool // teve aspas em alguma parte: nunca é operador
}

// splitCmdTokens divide respeitando aspas simples e duplas (que agrupam e
// são removidas). Barra invertida é literal (caminhos Windows).
func splitCmdTokens(s string) ([]cmdToken, error) {
	var toks []cmdToken
	var cur strings.Builder
	inTok, quoted := false, false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inTok, quoted = r, true, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inTok {
				toks = append(toks, cmdToken{cur.String(), quoted})
				cur.Reset()
				inTok, quoted = false, false
			}
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("aspas não fechadas na linha de comando")
	}
	if inTok {
		toks = append(toks, cmdToken{cur.String(), quoted})
	}
	return toks, nil
}

// parseCommandLine monta argv e redirecionamentos. Operadores reconhecidos
// (sem aspas, separados ou colados no alvo): > >> 1> 1>> 2> 2>> < 2>&1.
func parseCommandLine(s string) (cmdLine, error) {
	var cl cmdLine
	toks, err := splitCmdTokens(s)
	if err != nil {
		return cl, err
	}
	// Mais longos primeiro: ">>" antes de ">".
	ops := []string{"2>&1", "1>>", "2>>", ">>", "1>", "2>", ">", "<"}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		op := ""
		if !t.quoted {
			for _, o := range ops {
				if strings.HasPrefix(t.text, o) {
					op = o
					break
				}
			}
		}
		if op == "" {
			cl.argv = append(cl.argv, t.text)
			continue
		}
		if op == "2>&1" {
			if t.text != op {
				return cl, fmt.Errorf("redirecionamento inválido: %q", t.text)
			}
			cl.errToOut = true
			continue
		}
		target := t.text[len(op):]
		if target == "" {
			if i+1 >= len(toks) {
				return cl, fmt.Errorf("redirecionamento %q sem arquivo", op)
			}
			i++
			target = toks[i].text
		}
		switch op {
		case ">", "1>":
			cl.stdout, cl.appendOut = target, false
		case ">>", "1>>":
			cl.stdout, cl.appendOut = target, true
		case "2>":
			cl.stderr, cl.appendErr = target, false
		case "2>>":
			cl.stderr, cl.appendErr = target, true
		case "<":
			cl.stdin = target
		}
	}
	return cl, nil
}

// openRedirect abre o arquivo de saída de um redirecionamento.
func openRedirect(path string, appendMode bool) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	return os.OpenFile(path, flags, 0o644)
}
