package vm

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newAmbienteTestVM cria uma VM e registra as natives de Ambiente num mapa
// avulso (o natives.go não é alterado — o registro manual é o caminho de
// teste documentado).
func newAmbienteTestVM(t *testing.T) map[string]func(args []advplrt.Value) (advplrt.Value, error) {
	t.Helper()
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerAmbienteNatives(natives)
	return natives
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestGetTempPath(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETTEMPPATH"](nil)
	if err != nil {
		t.Fatalf("GETTEMPPATH erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Errorf("GETTEMPPATH retornou vazio")
	}
	if advplrt.ToString(got) != os.TempDir() {
		t.Errorf("GETTEMPPATH = %q, quer os.TempDir()=%q", advplrt.ToString(got), os.TempDir())
	}
}

func TestGetComputerName(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETCOMPUTERNAME"](nil)
	if err != nil {
		t.Fatalf("GETCOMPUTERNAME erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Errorf("GETCOMPUTERNAME retornou vazio")
	}
}

func TestGetCodePage(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETCODEPAGE"](nil)
	if err != nil {
		t.Fatalf("GETCODEPAGE erro: %v", err)
	}
	if advplrt.ToString(got) != "CP1252" {
		t.Errorf("GETCODEPAGE = %q, quer CP1252", advplrt.ToString(got))
	}
}

func TestUUIDRandom(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["UUIDRANDOM"](nil)
	if err != nil {
		t.Fatalf("UUIDRANDOM erro: %v", err)
	}
	u1 := advplrt.ToString(got)
	if !uuidRe.MatchString(u1) {
		t.Errorf("UUIDRANDOM formato inválido: %q", u1)
	}
	got2, _ := natives["UUIDRANDOM"](nil)
	u2 := advplrt.ToString(got2)
	if u1 == u2 {
		t.Errorf("UUIDRANDOM retornou o mesmo UUID duas vezes: %q", u1)
	}
}

func TestUUIDRandomSeq(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["UUIDRANDOMSEQ"](nil)
	if err != nil {
		t.Fatalf("UUIDRANDOMSEQ erro: %v", err)
	}
	u1 := advplrt.ToString(got)
	if !uuidRe.MatchString(u1) {
		t.Errorf("UUIDRANDOMSEQ formato inválido: %q", u1)
	}
	got2, _ := natives["UUIDRANDOMSEQ"](nil)
	u2 := advplrt.ToString(got2)
	if u1 == u2 {
		t.Errorf("UUIDRANDOMSEQ retornou o mesmo UUID duas vezes: %q", u1)
	}
	if u1 >= u2 {
		t.Errorf("UUIDRANDOMSEQ não é monotônico: %q depois de %q", u2, u1)
	}
}

func TestGetServerIP(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETSERVERIP"](nil)
	if err != nil {
		t.Fatalf("GETSERVERIP erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Errorf("GETSERVERIP retornou vazio (sem interfaces?)")
	}
	// lGetAllAddress = .T. deve retornar array
	all, err := natives["GETSERVERIP"]([]advplrt.Value{advplrt.True})
	if err != nil {
		t.Fatalf("GETSERVERIP(.T.) erro: %v", err)
	}
	if _, ok := all.(*advplrt.ArrayValue); !ok {
		t.Errorf("GETSERVERIP(.T.) retornou %v, quer array", all)
	}
}

func TestGetClientIP(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETCLIENTIP"](nil)
	if err != nil {
		t.Fatalf("GETCLIENTIP erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Errorf("GETCLIENTIP retornou vazio (sem interfaces?)")
	}
}

func TestSerialNumber(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["SERIALNUMBER"](nil)
	if err != nil {
		t.Fatalf("SERIALNUMBER erro: %v", err)
	}
	s1 := advplrt.ToString(got)
	if !regexp.MustCompile(`^[0-9A-F]{4}-[0-9A-F]{4}$`).MatchString(s1) {
		t.Errorf("SERIALNUMBER formato inválido: %q", s1)
	}
	got2, _ := natives["SERIALNUMBER"](nil)
	if advplrt.ToString(got2) != s1 {
		t.Errorf("SERIALNUMBER não é estável: %q != %q", advplrt.ToString(got2), s1)
	}
}

func TestGetHardwareId(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETHARDWAREID"](nil)
	if err != nil {
		t.Fatalf("GETHARDWAREID erro: %v", err)
	}
	if !regexp.MustCompile(`^[0-9A-F]{4}-[0-9A-F]{4}$`).MatchString(advplrt.ToString(got)) {
		t.Errorf("GETHARDWAREID formato inválido: %q", advplrt.ToString(got))
	}
}

func TestIsSrvUnix(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["ISSRVUNIX"](nil)
	if err != nil {
		t.Fatalf("ISSRVUNIX erro: %v", err)
	}
	if _, ok := got.(*advplrt.BoolValue); !ok {
		t.Errorf("ISSRVUNIX retornou %v, quer booleano", got)
	}
}

func TestGetSrvArch(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETSRVARCH"](nil)
	if err != nil {
		t.Fatalf("GETSRVARCH erro: %v", err)
	}
	switch advplrt.ToString(got) {
	case "x86_64", "i686", "aarch32", "aarch64", "unknown":
	default:
		t.Errorf("GETSRVARCH retornou valor inesperado: %q", advplrt.ToString(got))
	}
}

func TestGetSrvInfo(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETSRVINFO"](nil)
	if err != nil {
		t.Fatalf("GETSRVINFO erro: %v", err)
	}
	a, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("GETSRVINFO retornou %v, quer array", got)
	}
	if len(a.Elements) < 13 {
		t.Errorf("GETSRVINFO tem %d elementos, quer >= 13", len(a.Elements))
	}
	if advplrt.ToString(a.Elements[0]) == "" {
		t.Errorf("GETSRVINFO[1] (hostname) vazio")
	}
}

func TestGetRmtDate(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETRMTDATE"](nil)
	if err != nil {
		t.Fatalf("GETRMTDATE erro: %v", err)
	}
	if _, ok := got.(*advplrt.DateValue); !ok {
		t.Errorf("GETRMTDATE retornou %v, quer data", got)
	}
}

func TestGetRmtTime(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETRMTTIME"](nil)
	if err != nil {
		t.Fatalf("GETRMTTIME erro: %v", err)
	}
	if !regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`).MatchString(advplrt.ToString(got)) {
		t.Errorf("GETRMTTIME formato inválido: %q", advplrt.ToString(got))
	}
}

func TestGetUserInfoArray(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETUSERINFOARRAY"](nil)
	if err != nil {
		t.Fatalf("GETUSERINFOARRAY erro: %v", err)
	}
	outer, ok := got.(*advplrt.ArrayValue)
	if !ok || len(outer.Elements) == 0 {
		t.Fatalf("GETUSERINFOARRAY retornou %v, quer array não vazio", got)
	}
	row, ok := outer.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(row.Elements) < 16 {
		t.Errorf("GETUSERINFOARRAY linha com %d colunas, quer >= 16", len(row.Elements))
	}
}

func TestMetricsName(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["METRICSNAME"](nil)
	if err != nil {
		t.Fatalf("METRICSNAME erro: %v", err)
	}
	out := advplrt.ToString(got)
	if !strings.Contains(out, `"names"`) || !strings.Contains(out, "startdate") {
		t.Errorf("METRICSNAME JSON inesperado: %q", out)
	}
	withVer, err := natives["METRICSNAME"]([]advplrt.Value{advplrt.True})
	if err != nil {
		t.Fatalf("METRICSNAME(.T.) erro: %v", err)
	}
	if !strings.Contains(advplrt.ToString(withVer), `"version"`) {
		t.Errorf("METRICSNAME(.T.) sem versão: %q", advplrt.ToString(withVer))
	}
}

func TestMetricsRead(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["METRICSREAD"](nil)
	if err != nil {
		t.Fatalf("METRICSREAD erro: %v", err)
	}
	out := advplrt.ToString(got)
	for _, want := range []string{`"version"`, "startdate", "memory_ram_total"} {
		if !strings.Contains(out, want) {
			t.Errorf("METRICSREAD JSON sem %q: %s", want, out)
		}
	}
	// filtro com nome inválido
	filter := advplrt.NewArray([]advplrt.Value{advplrt.NewString("InvalidMetric")})
	gotF, err := natives["METRICSREAD"]([]advplrt.Value{filter})
	if err != nil {
		t.Fatalf("METRICSREAD(filtro) erro: %v", err)
	}
	if !strings.Contains(advplrt.ToString(gotF), `"error":"invalid metric"`) {
		t.Errorf("METRICSREAD filtro inválido sem error: %q", advplrt.ToString(gotF))
	}
}

func TestSetKSysLogDelKSysLog(t *testing.T) {
	natives := newAmbienteTestVM(t)
	// limpa estado prévio
	ksyslogMu.Lock()
	ksyslogTags = map[string]string{}
	ksyslogMu.Unlock()
	os.Remove(ksyslogFilePath())

	if _, err := natives["SETKSYSLOG"]([]advplrt.Value{
		advplrt.NewString("exemplo_key"),
		advplrt.NewString("msg=\"valor\""),
	}); err != nil {
		t.Fatalf("SETKSYSLOG erro: %v", err)
	}
	ksyslogMu.Lock()
	_, exists := ksyslogTags["exemplo_key"]
	ksyslogMu.Unlock()
	if !exists {
		t.Errorf("SETKSYSLOG não registrou a tag")
	}
	if _, err := os.Stat(ksyslogFilePath()); err != nil {
		t.Errorf("SETKSYSLOG não criou o arquivo de log: %v", err)
	}

	if _, err := natives["DELKSYSLOG"]([]advplrt.Value{advplrt.NewString("exemplo_key")}); err != nil {
		t.Fatalf("DELKSYSLOG erro: %v", err)
	}
	ksyslogMu.Lock()
	_, exists = ksyslogTags["exemplo_key"]
	ksyslogMu.Unlock()
	if exists {
		t.Errorf("DELKSYSLOG não removeu a tag")
	}
	if _, err := os.Stat(ksyslogFilePath()); err == nil {
		t.Errorf("DELKSYSLOG não removeu o arquivo de log")
	}
}

func TestShowInfMem(t *testing.T) {
	natives := newAmbienteTestVM(t)
	aInfo := advplrt.NewArray([]advplrt.Value{
		advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(0), advplrt.NewNumber(0)}),
		advplrt.NewArray([]advplrt.Value{advplrt.NewNumber(0), advplrt.NewNumber(0)}),
	})
	got, err := natives["SHOWINFMEM"]([]advplrt.Value{
		advplrt.NewString("Início"),
		aInfo,
	})
	if err != nil {
		t.Fatalf("SHOWINFMEM erro: %v", err)
	}
	if !advplrt.ToBool(got) {
		t.Errorf("SHOWINFMEM retornou %v, quer .T.", got)
	}
	row, ok := aInfo.Elements[0].(*advplrt.ArrayValue)
	if !ok || len(row.Elements) != 2 {
		t.Errorf("SHOWINFMEM não preencheu aInfo[1] com [kb, count]")
	}
}

func TestGetTempPathWithArgs(t *testing.T) {
	natives := newAmbienteTestVM(t)
	got, err := natives["GETTEMPPATH"]([]advplrt.Value{advplrt.True, advplrt.True})
	if err != nil {
		t.Fatalf("GETTEMPPATH(.T.,.T.) erro: %v", err)
	}
	if advplrt.ToString(got) == "" {
		t.Errorf("GETTEMPPATH(.T.,.T.) retornou vazio")
	}
}
