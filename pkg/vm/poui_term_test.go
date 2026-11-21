//go:build !windows

package vm

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// saidaTerm junta os bytes das ops "term" enviadas até agora.
func saidaTerm(f *fakeUI) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var b strings.Builder
	for _, s := range f.sent {
		var op struct {
			Op   string `json:"op"`
			Data string `json:"data"`
		}
		if json.Unmarshal(s, &op) == nil && op.Op == "term" {
			d, _ := base64.StdEncoding.DecodeString(op.Data)
			b.Write(d)
		}
	}
	return b.String()
}

func esperaSaida(t *testing.T, f *fakeUI, trecho string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if strings.Contains(saidaTerm(f), trecho) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("saída do terminal sem %q: %q", trecho, saidaTerm(f))
}

const termPrg = `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oT   := AdvTerminal():New(oPg, %s)
	oT:OnExit({|o, n| ConOut("exit=" + cValToChar(n)), oApp:Close()})
	oApp:Activate()
Return
`

func prgTerm(args string) string { return strings.Replace(termPrg, "%s", args, 1) }

func TestTerminalEco(t *testing.T) {
	// ids: página 1, terminal 2
	f := newFakeUI(fakeEv{2, "input", []byte(`"oi-advpp\n"`)})
	_, _, done := rodaComUI(t, prgTerm(`"cat"`), f)
	esperaSaida(t, f, "oi-advpp")
	close(f.quit)
	<-done
}

func TestTerminalExit(t *testing.T) {
	f := newFakeUI()
	out, errc, done := rodaComUI(t, prgTerm(`"sh", {"-c", "exit 3"}`), f)
	esperaFim(t, done, errc)
	if strings.Count(out.String(), "exit=3") != 1 {
		t.Errorf("saída = %q", out.String())
	}
	ok := false
	for _, s := range f.sent {
		ok = ok || strings.Contains(string(s), `"running":false`)
	}
	if !ok {
		t.Errorf("sem patch running:false: %q", f.sent)
	}
}

func TestTerminalBytesCrus(t *testing.T) {
	f := newFakeUI()
	_, errc, done := rodaComUI(t, prgTerm(`"sh", {"-c", "printf '\303'; sleep 0.2; printf '\247'"}`), f)
	esperaFim(t, done, errc)
	if !strings.Contains(saidaTerm(f), "\xc3\xa7") {
		t.Errorf("bytes: %q", saidaTerm(f))
	}
}

func TestTerminalResize(t *testing.T) {
	f := newFakeUI(fakeEv{2, "resize", []byte(`{"cols":100,"rows":30}`)}, fakeEv{2, "input", []byte(`"stty size; exit 0\n"`)})
	_, errc, done := rodaComUI(t, prgTerm(`"sh"`), f)
	esperaFim(t, done, errc)
	if !strings.Contains(saidaTerm(f), "30 100") {
		t.Errorf("saída: %q", saidaTerm(f))
	}
}

func TestTerminalMortoAoFecharSessao(t *testing.T) {
	f := newFakeUI()
	_, _, done := rodaComUI(t, prgTerm(`"sh", {"-c", "echo PID:$$; exec sleep 30"}`), f)
	esperaSaida(t, f, "PID:")
	s := saidaTerm(f)
	pid, _ := strconv.Atoi(strings.TrimSpace(strings.SplitN(s[strings.Index(s, "PID:")+4:], "\n", 2)[0]))
	if pid == 0 {
		t.Fatalf("PID: %q", s)
	}
	close(f.quit)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("programa não saiu")
	}
	for i := 0; i < 200; i++ {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("processo %d do terminal continuou vivo", pid)
}
