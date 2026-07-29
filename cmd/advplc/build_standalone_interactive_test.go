package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// termResponder answers the terminal-capability queries bubbletea/termenv
// send on startup (OSC 11 "what's your background color?", DSR "where's
// the cursor?") — every real terminal emulator answers these within
// microseconds, which is why huh renders instantly for a human. A test
// harness that never answers doesn't just render late: leftover synthetic
// keystrokes (Write below) sent into that unanswered window land while the
// terminal is still in cooked mode and the query-response parser is still
// reading, corrupting it and hanging the whole test far past termenv's own
// 5s OSCTimeout — this is what made the very first version of this test
// flaky/hanging in a way that took a while to pin down as a harness problem
// rather than a product one. Responding immediately, like a real terminal
// would, removes the race entirely instead of just out-waiting it.
type termResponder struct {
	answeredBG     bool
	answeredCursor bool
}

func (r *termResponder) respond(sess ptySession, full *bytes.Buffer) {
	s := full.String()
	if !r.answeredBG && strings.Contains(s, "\x1b]11;?") {
		r.answeredBG = true
		_, _ = sess.Write([]byte("\x1b]11;rgb:1e1e/1e1e/1e1e\x1b\\"))
	}
	if !r.answeredCursor && strings.Contains(s, "\x1b[6n") {
		r.answeredCursor = true
		_, _ = sess.Write([]byte("\x1b[24;1R"))
	}
}

// readPump feeds every byte read from sess into a channel until sess is
// closed or the read errors out. A background goroutine + channel (instead
// of *os.File's SetReadDeadline, which ConPty's Read doesn't support) is
// what lets ptyExpect poll with a timeout uniformly on both the POSIX pty
// and the Windows ConPty backends.
func readPump(sess ptySession) <-chan []byte {
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				ch <- chunk
			}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

// ptyExpect drains reads until needle appears, or the deadline passes. Every
// byte read is appended to full (an append-only transcript, for the final
// assertions and failure dumps) and also to work — but on a match, work is
// truncated to just past the match. Several needles in this test occur more
// than once ("Selecione o registro" for both Editar and Excluir, "(nenhum
// registro)" before the first insert and again after the delete); without
// truncating work, the second wait would match instantly against the
// *first* occurrence still sitting in the buffer and send its input before
// the prompt it's meant to answer even exists yet.
func ptyExpect(sess ptySession, reads <-chan []byte, work, full *bytes.Buffer, term *termResponder, needle string, timeout time.Duration) bool {
	consume := func() bool {
		s := work.String()
		if idx := strings.Index(s, needle); idx >= 0 {
			work.Reset()
			work.WriteString(s[idx+len(needle):])
			return true
		}
		return false
	}
	if consume() {
		return true
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case chunk, ok := <-reads:
			if !ok {
				return false
			}
			work.Write(chunk)
			full.Write(chunk)
			term.respond(sess, full)
			if consume() {
				return true
			}
		case <-deadline.C:
			return false
		}
	}
}

// TestBuildStandaloneInteractive is the regression test the manual
// (human-in-the-loop) debugging session that found these bugs didn't have:
// it builds a real standalone executable via `advplc build`, runs it under
// a REAL pty (not a pipe — FWGetText's password masking and the TTY
// detection that decides console-vs-Fyne both key off term.IsTerminal,
// which pipes fail), and drives FWGetText, FWMenuSelect, and a full
// FWMBrowse CRUD cycle (Novo/Editar/Excluir) exactly like a person typing
// at a keyboard would. Before the fix this landed on, none of that path
// had any automated coverage — TestBuildStandaloneSmoke deliberately uses
// a 100%-console, zero-input fixture, so a build that silently never read
// stdin (FWGetText returning its default with no UIProvider attached) or
// an FWMBrowse that only worked in `advplc serve` both compiled and
// "passed" every existing test while being unusable as an app.
//
// Runs on every OS the project ships for (see ptysession_posix_test.go /
// ptysession_windows_test.go): POSIX gets a real pty via creack/pty,
// Windows gets a real ConPty via UserExistsError/conpty (pure Go, no cgo,
// straight over golang.org/x/sys/windows syscalls) — same test, same
// assertions, same coverage, just a different OS API underneath.
func TestBuildStandaloneInteractive(t *testing.T) {
	if testing.Short() {
		t.Skip("builda um executável standalone e roda sob PTY; pulado com -short")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("repoRoot não parece um checkout do AdvPP: %v", err)
	}

	binName := "advplc"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, binName)
	build := exec.Command("go", "build", "-o", binPath, "./cmd/advplc")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build advplc: %v\n%s", err, out)
	}

	outName := "standalone_interactive"
	if runtime.GOOS == "windows" {
		outName += ".exe"
	}
	outPath := filepath.Join(tmpDir, outName)
	buildCmd := exec.Command(binPath, "build", "tests/standalone_interactive_test.prw", "-o", outPath)
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "ADVPP_SRC="+repoRoot)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("advplc build: %v\n%s", err, out)
	}

	dbPath := filepath.Join(tmpDir, "interactive_test.db")
	env := append(os.Environ(), "ADVPP_DB="+dbPath, "TERM=xterm-256color")

	// A real terminal always reports a size the moment it spawns a child;
	// an unsized pty/ConPty leaves it at 0x0. huh/bubbletea don't render
	// real content against a zero-size viewport — the form just sits
	// there printing nothing but its "enter submit" help line forever,
	// which looks exactly like a hang and burned a lot of time to tell
	// apart from an actual one before this got added.
	sess := startPTYSession(t, outPath, env, 120, 40)
	defer sess.Close()
	reads := readPump(sess)

	var work, full bytes.Buffer
	term := &termResponder{}
	fail := func(step string) {
		t.Fatalf("%s: não recebi o esperado a tempo.\n--- transcrição completa ---\n%s", step, full.String())
	}
	expect := func(needle string, timeout time.Duration) bool {
		return ptyExpect(sess, reads, &work, &full, term, needle, timeout)
	}
	// send waits a beat before writing: each huh.Form.Run() tears down raw
	// mode when it returns and the *next* one re-enables it on its own
	// goroutine, so there's a real (if brief) window right after a needle
	// like "(nenhum registro)" — printed by plain fmt.Println, before the
	// *next* form has even started — where the terminal is back in cooked
	// mode. A keystroke landing in that window gets swallowed by the
	// canonical-mode line discipline (echoed as bare text, never delivered
	// to the new form) instead of being read by anything, so the form
	// blocks forever waiting for input that already came and went. No
	// human types fast enough to hit this in real use; a script firing the
	// next key the instant a needle matches can.
	send := func(s, label string) {
		time.Sleep(150 * time.Millisecond)
		if _, err := sess.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", label, err)
		}
	}

	if !expect("Seu nome", 10*time.Second) {
		fail("prompt de nome")
	}
	send("Fulano de Tal\r", "nome")

	if !expect("Menu de teste", 10*time.Second) {
		fail("menu")
	}
	send("1\r", "menu")

	if !expect("(nenhum registro)", 10*time.Second) {
		fail("browse vazio (Novo/Editar/Excluir/Voltar)")
	}
	send("1\r", "novo") // Novo

	if !expect("ITST_NOME", 10*time.Second) {
		fail("prompt de campo (Novo) — só ITST_NOME deve ser pedido, nunca R_E_C_N_O_")
	}
	send("Registro A\r", "valor do campo")

	if !expect("Registro A", 10*time.Second) {
		fail("lista mostrando o registro recém-criado")
	}
	send("\x1b[B\r", "editar (down + enter)") // Editar = 2º item

	if !expect("Selecione o registro", 10*time.Second) {
		fail("seleção de registro para editar")
	}
	send("1\r", "seleção") // registro #1

	if !expect("ITST_NOME", 10*time.Second) {
		fail("formulário de edição — deve reabrir com ITST_NOME pré-preenchido")
	}
	if !expect("Registro A", 10*time.Second) {
		fail("valor atual (Registro A) pré-preenchido no campo de edição")
	}
	// o cursor começa no fim do texto pré-preenchido ("Registro A") — sem
	// apagar primeiro, "Registro B" apareceria concatenado, não substituindo.
	send(strings.Repeat("\x7f", len("Registro A"))+"Registro B\r", "novo valor")

	if !expect("Registro B", 10*time.Second) {
		fail("lista mostrando o registro editado")
	}
	send("\x1b[B\x1b[B\r", "excluir (2x down + enter)") // Excluir = 3º item

	if !expect("Selecione o registro", 10*time.Second) {
		fail("seleção de registro para excluir")
	}
	send("1\r", "seleção") // registro #1

	if !expect("Confirma excluir", 10*time.Second) {
		fail("confirmação de exclusão")
	}
	send("s\r", "confirmação")

	if !expect("(nenhum registro)", 10*time.Second) {
		fail("lista voltando a ficar vazia após excluir")
	}
	send("\x1b[B\x1b[B\x1b[B\r", "voltar (3x down + enter)") // Voltar = 4º item

	if !expect("FIM_DO_TESTE", 10*time.Second) {
		fail("fim do programa")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	waitErr := make(chan error, 1)
	go func() { waitErr <- sess.Wait() }()
	select {
	case err := <-waitErr:
		if err != nil {
			t.Fatalf("processo terminou com erro: %v\n--- transcrição completa ---\n%s", err, full.String())
		}
	case <-ctx.Done():
		t.Fatalf("processo não terminou sozinho após ver FIM_DO_TESTE\n--- transcrição completa ---\n%s", full.String())
	}

	got := full.String()
	if !strings.Contains(got, "NOME_LIDO=Fulano de Tal") {
		t.Errorf("FWGetText não leu o nome digitado (login/prompt silencioso é exatamente a regressão original); saída:\n%s", got)
	}
	if !strings.Contains(got, "MENU_ESCOLHIDO=1") {
		t.Errorf("FWMenuSelect não registrou a escolha do menu; saída:\n%s", got)
	}

	db, err := os.ReadFile(dbPath)
	if err != nil || len(db) == 0 {
		t.Fatalf("banco de teste não foi criado em %s: %v", dbPath, err)
	}
}
