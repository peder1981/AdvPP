//go:build !windows

package vm

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSpawnLinhasEFim(t *testing.T) {
	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:Spawn("sh", {"-c", "echo a; echo b >&2; exit 3"}, ;
		{|c, l| ConOut("L:" + c + cValToChar(l))}, ;
		{|n| ConOut("F:" + cValToChar(n)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	s := out.String()
	for _, frag := range []string{"L:a.F.", "L:b.T.", "F:3"} {
		if !strings.Contains(s, frag) {
			t.Errorf("saída sem %s: %q", frag, s)
		}
	}
	if strings.Index(s, "F:3") < strings.Index(s, "L:a") || strings.Index(s, "F:3") < strings.Index(s, "L:b") {
		t.Errorf("fim chegou antes das linhas: %q", s)
	}
}

func TestSpawnNaoPerdeLinhas(t *testing.T) {
	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp  := PoApp():New("T")
	Local oPg   := PoPage():New(oApp, "P")
	Local nCont := 0
	Local nSoma := 0
	oApp:Spawn("seq", {"1", "5000"}, ;
		{|c| nCont++, nSoma += Val(c)}, ;
		{|n| ConOut("R:" + cValToChar(nCont) + "/" + cValToChar(nSoma)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "R:5000/12502500") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestSpawnProgramaInexistente(t *testing.T) {
	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:Spawn("programa-que-nao-existe-xyz", {}, {|| .T.}, {|n| ConOut("F:" + cValToChar(n)), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "F:-1") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestSpawnSemShell(t *testing.T) {
	marcador := filepath.Join(t.TempDir(), "injetado")
	f := newFakeUI()
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:Spawn("echo", {"a;", "touch", "`+marcador+`"}, {|c| ConOut("L:" + c)}, {|n| oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if _, err := os.Stat(marcador); err == nil {
		t.Fatal("';' executou um segundo comando")
	}
	if !strings.Contains(out.String(), "L:a; touch") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestSpawnMortoAoFecharSessao(t *testing.T) {
	f := newFakeUI()
	out, _, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:Spawn("sh", {"-c", "echo $$; exec sleep 30"}, {|c| ConOut("PID:" + c)}, {|n| .T.})
	oApp:Activate()
Return
`, f)
	pid := 0
	for i := 0; i < 200 && pid == 0; i++ {
		if j := strings.Index(out.String(), "PID:"); j >= 0 {
			pid, _ = strconv.Atoi(strings.TrimSpace(strings.SplitN(out.String()[j+4:], "\n", 2)[0]))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatalf("processo não informou o PID: %q", out.String())
	}
	close(f.quit)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("programa não saiu com a sessão encerrada")
	}
	for i := 0; i < 200; i++ {
		if syscall.Kill(pid, 0) != nil {
			return // processo não existe mais
		}
		time.Sleep(10 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("processo %d continuou vivo depois que a sessão fechou", pid)
}
