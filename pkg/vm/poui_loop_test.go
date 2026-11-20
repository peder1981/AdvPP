package vm

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeEv struct {
	id  int
	ev  string
	val []byte
}

// fakeUI faz o papel do browser: guarda o que a VM envia e entrega os
// eventos enfileirados em in. Fechar quit simula a sessão abandonada.
type fakeUI struct {
	mu       sync.Mutex
	sent     [][]byte
	in       chan fakeEv
	quit     chan struct{}
	postToIn bool // UIPost (timers) entra na fila de eventos
	posts    atomic.Int32
	onNext   func() // chamado a cada UINext (inspeção da VM no teste)
}

func newFakeUI(evs ...fakeEv) *fakeUI {
	f := &fakeUI{in: make(chan fakeEv, 64), quit: make(chan struct{})}
	for _, e := range evs {
		f.in <- e
	}
	return f
}

func (f *fakeUI) MsgInfo(string, string)               {}
func (f *fakeUI) MsgStop(string, string)               {}
func (f *fakeUI) MsgAlert(string, string)              {}
func (f *fakeUI) MsgYesNo(string, string) bool         { return false }
func (f *fakeUI) Menu([]string, string) int            { return 0 }
func (f *fakeUI) InputText(_, d string, _ bool) string { return d }

func (f *fakeUI) UISend(op []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, append([]byte(nil), op...))
}

func (f *fakeUI) UIPost(id int, ev string, val []byte) {
	select {
	case <-f.quit:
		return
	default:
	}
	f.posts.Add(1)
	if f.postToIn {
		select {
		case f.in <- fakeEv{id, ev, val}:
		default:
		}
	}
}

func (f *fakeUI) UIPostWait(id int, ev string, val []byte) bool {
	select {
	case f.in <- fakeEv{id, ev, val}:
		return true
	case <-f.quit:
		return false
	}
}

func (f *fakeUI) UINext() (int, string, []byte) {
	if f.onNext != nil {
		f.onNext()
	}
	select {
	case e := <-f.in:
		return e.id, e.ev, e.val
	case <-f.quit:
		runtime.Goexit()
		return 0, "", nil
	}
}

func (f *fakeUI) ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, s := range f.sent {
		var m struct{ Op string }
		json.Unmarshal(s, &m)
		out = append(out, m.Op)
	}
	return out
}

// rodaComUI executa o programa com a fakeUI numa goroutine própria (o laço
// pode terminar por Goexit). done fecha quando a goroutine sai.
func rodaComUI(t *testing.T, src string, f *fakeUI) (out *syncBuf, errc chan error, done chan struct{}) {
	t.Helper()
	v := NewVM(compilarPrg(t, src), true)
	v.SetUIProvider(f)
	out = &syncBuf{}
	v.SetOutputWriter(out)
	errc = make(chan error, 1)
	done = make(chan struct{})
	go func() {
		defer close(done)
		_, err := v.Run()
		errc <- err
	}()
	return out, errc, done
}

func esperaFim(t *testing.T, done chan struct{}, errc chan error) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("o laço de eventos não terminou")
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestActivateMontaETrataClique(t *testing.T) {
	// ids: página 1, tag 2, botão 3
	f := newFakeUI(fakeEv{3, "click", []byte("null")})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oTag := PoTag():New(oPg, "inicial")
	Local oBtn := PoButton():New(oPg, "Ok", {|| oTag:SetValue("clicado"), oApp:Close()})
	oApp:Activate()
	ConOut("fim")
Return
`, f)
	esperaFim(t, done, errc)
	if got := strings.Join(f.ops(), ","); got != "mount,batch,close" {
		t.Fatalf("ops = %s", got)
	}
	if string(f.sent[1]) != `{"op":"batch","ops":[{"id":2,"op":"patch","props":{"value":"clicado"}}]}` {
		t.Errorf("batch = %s", f.sent[1])
	}
	if !strings.Contains(out.String(), "fim") {
		t.Errorf("o programa não seguiu depois do Activate: %q", out.String())
	}
}

func TestChangeAtualizaValorSemPatch(t *testing.T) {
	f := newFakeUI(fakeEv{2, "change", []byte(`"abc"`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oInp := PoInput():New(oPg, "Nome")
	oInp:OnChange({|o, c| ConOut("v=" + o:GetValue() + "/" + c), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "v=abc/abc") {
		t.Errorf("saída = %q", out.String())
	}
	if got := strings.Join(f.ops(), ","); got != "mount,close" {
		t.Errorf("ops = %s (change vindo do browser não gera patch)", got)
	}
}

func TestChangeValorNumerico(t *testing.T) {
	f := newFakeUI(fakeEv{2, "change", []byte(`42`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oInp := PoInput():New(oPg, "N")
	oInp:OnChange({|o, c| ConOut("t=" + ValType(c) + c), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "t=C42") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestSelectEntregaLinha(t *testing.T) {
	// ids: página 1, tabela 2, botão 3
	f := newFakeUI(fakeEv{2, "select", []byte(`1`)}, fakeEv{2, "select", []byte(`9`)}, fakeEv{3, "click", nil})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oTab := PoTable():New(oPg)
	Local oBtn := PoButton():New(oPg, "Sair", {|| oApp:Close()})
	oTab:SetColumns({{"a", "A"}, {"n", "N", "number"}})
	oTab:SetItems({{"x", 1}, {"y", 2}})
	oTab:OnSelect({|o, a| ConOut("sel=" + a[1] + cValToChar(a[2]))})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if strings.Count(out.String(), "sel=") != 1 || !strings.Contains(out.String(), "sel=y2") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestPageAction(t *testing.T) {
	f := newFakeUI(fakeEv{1, "action", []byte(`0`)})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oPg:AddAction("Ok", {|| ConOut("acao"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "acao") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestHandlerErroNaoDerrubaLaco(t *testing.T) {
	// ids: página 1, botão com erro 2, botão sair 3
	f := newFakeUI(fakeEv{2, "click", nil}, fakeEv{3, "click", nil})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB1  := PoButton():New(oPg, "Erro", {|| FuncaoQueNaoExiste()})
	Local oB2  := PoButton():New(oPg, "Sair", {|| ConOut("saiu"), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "erro no evento click do componente 2") {
		t.Errorf("erro não reportado: %q", out.String())
	}
	if !strings.Contains(out.String(), "saiu") {
		t.Errorf("o laço não continuou depois do erro: %q", out.String())
	}
}

func TestEventoDesconhecidoIgnorado(t *testing.T) {
	// ids: página 1, input 2, botão 3
	f := newFakeUI(fakeEv{999, "click", nil}, fakeEv{2, "click", nil}, fakeEv{3, "click", nil})
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oInp := PoInput():New(oPg, "N")
	Local oBtn := PoButton():New(oPg, "Sair", {|| oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if got := strings.Join(f.ops(), ","); got != "mount,close" {
		t.Errorf("ops = %s", got)
	}
	if strings.Contains(out.String(), "erro") {
		t.Errorf("saída inesperada: %q", out.String())
	}
}

func TestTimerDisparaEPara(t *testing.T) {
	f := newFakeUI()
	f.postToIn = true
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp   := PoApp():New("T")
	Local oPg    := PoPage():New(oApp, "P")
	Local nVezes := 0
	oApp:SetTimer(100, {|| nVezes++, ConOut("tick"), Tick(oApp, nVezes)})
	oApp:Activate()
Return

Static Function Tick(oApp, nVezes)
	If nVezes >= 3
		oApp:Close()
	EndIf
Return
`, f)
	esperaFim(t, done, errc)
	if n := strings.Count(out.String(), "tick"); n != 3 {
		t.Errorf("ticks = %d (%q)", n, out.String())
	}
}

func TestTimerParaAoFecharSessao(t *testing.T) {
	antes := runtime.NumGoroutine()
	f := newFakeUI()
	_, _, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	oApp:SetTimer(100, {|| .T.})
	oApp:Activate()
Return
`, f)
	for i := 0; i < 100 && len(f.ops()) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(250 * time.Millisecond) // deixa o timer postar algumas vezes
	close(f.quit)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a goroutine do programa não saiu com a sessão encerrada")
	}
	time.Sleep(300 * time.Millisecond)
	if depois := runtime.NumGoroutine(); depois > antes+1 {
		t.Errorf("goroutines: antes %d, depois %d (timer vazou)", antes, depois)
	}
}

func TestSetTimerIntervaloMinimo(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	oApp:SetTimer(50, {|| .T.})
Return
`)
	if err == nil || !strings.Contains(err.Error(), "mínimo 100 ms") {
		t.Errorf("erro = %v", err)
	}
}

// Handler com erro repetido (ex.: timer quebrado num app que fica dias
// aberto) não pode deixar lixo na pilha de operandos da VM a cada evento.
func TestHandlerErroNaoAcumulaPilha(t *testing.T) {
	var evs []fakeEv
	for i := 0; i < 50; i++ {
		evs = append(evs, fakeEv{2, "click", nil})
	}
	evs = append(evs, fakeEv{3, "click", nil})
	f := newFakeUI(evs...)
	v := NewVM(compilarPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oB1  := PoButton():New(oPg, "Erro", {|| 1 + FuncaoQueNaoExiste(2, 3)})
	Local oB2  := PoButton():New(oPg, "Sair", {|| oApp:Close()})
	oApp:Activate()
Return
`), true)
	v.SetUIProvider(f)
	v.SetOutputWriter(&syncBuf{})
	var maxPilha int
	f.onNext = func() {
		if n := len(v.stack); n > maxPilha {
			maxPilha = n
		}
	}
	if _, err := v.Run(); err != nil {
		t.Fatal(err)
	}
	if maxPilha > 20 {
		t.Errorf("pilha cresceu para %d itens após 50 erros em handler", maxPilha)
	}
}
