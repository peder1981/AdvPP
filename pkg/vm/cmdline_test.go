package vm

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestParseCommandLine(t *testing.T) {
	casos := []struct {
		in   string
		want cmdLine
	}{
		{"echo a  b", cmdLine{argv: []string{"echo", "a", "b"}}},
		{`ls "meu arq.txt" 'x y'`, cmdLine{argv: []string{"ls", "meu arq.txt", "x y"}}},
		{`cmd a"b c"d`, cmdLine{argv: []string{"cmd", "ab cd"}}},
		{`cmd ""`, cmdLine{argv: []string{"cmd", ""}}},
		{`C:\bin\x.exe C:\tmp`, cmdLine{argv: []string{`C:\bin\x.exe`, `C:\tmp`}}},
		{"seq 1 3 > out.txt", cmdLine{argv: []string{"seq", "1", "3"}, stdout: "out.txt"}},
		{"seq 1 3 >out.txt", cmdLine{argv: []string{"seq", "1", "3"}, stdout: "out.txt"}},
		{"seq 1 3 >> out.txt 2>&1", cmdLine{argv: []string{"seq", "1", "3"}, stdout: "out.txt", appendOut: true, errToOut: true}},
		{"x 2> err.txt < in.txt", cmdLine{argv: []string{"x"}, stderr: "err.txt", stdin: "in.txt"}},
		{"x 2>>err.txt 1> o", cmdLine{argv: []string{"x"}, stderr: "err.txt", appendErr: true, stdout: "o"}},
		{`x > "saida com espaco.txt"`, cmdLine{argv: []string{"x"}, stdout: "saida com espaco.txt"}},
		// Operadores entre aspas são texto
		{`echo ">" "2>&1"`, cmdLine{argv: []string{"echo", ">", "2>&1"}}},
		// Sem shell: separadores/expansões viram argumentos literais (CWE-78)
		{"echo test; rm -rf /tmp/x && id | cat $HOME `id`", cmdLine{argv: []string{"echo", "test;", "rm", "-rf", "/tmp/x", "&&", "id", "|", "cat", "$HOME", "`id`"}}},
	}
	for _, c := range casos {
		got, err := parseCommandLine(c.in)
		if err != nil {
			t.Errorf("%q: erro %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q:\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
	for _, ruim := range []string{`echo "aberto`, "x >", "x 2>&1x"} {
		if _, err := parseCommandLine(ruim); err == nil {
			t.Errorf("%q: esperava erro", ruim)
		}
	}
}

// TestWaitRunRedirecionaSemShell: o uso documentado (capturar a saída com
// "> arquivo") funciona, e um ';' não executa um segundo comando.
func TestWaitRunRedirecionaSemShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa echo/sh de sistemas POSIX")
	}
	dir := t.TempDir()
	waitRun := NewVM(&compiler.Bytecode{}, false).natives["WAITRUN"].Fn
	run := func(cmd string) float64 {
		t.Helper()
		v, err := waitRun([]advplrt.Value{advplrt.NewString(cmd)})
		if err != nil {
			t.Fatal(err)
		}
		return advplrt.ToFloat(v)
	}
	ler := func(nome string) string {
		b, _ := os.ReadFile(filepath.Join(dir, nome))
		return string(b)
	}

	saida := filepath.Join(dir, "saida.txt")
	if rc := run("echo ola > " + saida); rc != 0 {
		t.Fatalf("rc = %v", rc)
	}
	run("echo mundo >> " + saida)
	if got := ler("saida.txt"); got != "ola\nmundo\n" {
		t.Errorf("saida.txt = %q", got)
	}

	marcador := filepath.Join(dir, "injetado")
	run("echo a; touch " + marcador + " > " + filepath.Join(dir, "inj.txt"))
	if _, err := os.Stat(marcador); err == nil {
		t.Fatal("';' executou um segundo comando: injeção de comando")
	}
	if got := ler("inj.txt"); !strings.Contains(got, "a; touch") {
		t.Errorf("inj.txt = %q (esperava o texto literal)", got)
	}

	if rc := run(`sh -c "echo erro >&2; exit 3" 2> ` + filepath.Join(dir, "err.txt")); rc != 3 {
		t.Errorf("rc = %v, esperado 3", rc)
	}
	if got := ler("err.txt"); got != "erro\n" {
		t.Errorf("err.txt = %q", got)
	}
	if rc := run(`echo "aspas abertas`); rc != -1 {
		t.Errorf("linha inválida: rc = %v, esperado -1", rc)
	}
}

// TestGetEnvInexistente: variável não definida devolve "" (não "Nil"), ou o
// default informado.
func TestGetEnvInexistente(t *testing.T) {
	getEnv := NewVM(&compiler.Bytecode{}, false).natives["GETENV"].Fn
	const nome = "ADVPP_TESTE_VAR_QUE_NAO_EXISTE"
	os.Unsetenv(nome)
	casos := []struct {
		args []advplrt.Value
		want string
	}{
		{[]advplrt.Value{advplrt.NewString(nome)}, ""},
		{[]advplrt.Value{advplrt.NewString(nome), advplrt.Nil}, ""},
		{[]advplrt.Value{advplrt.NewString(nome), advplrt.NewString("padrao")}, "padrao"},
	}
	for _, c := range casos {
		v, err := getEnv(c.args)
		if err != nil {
			t.Fatal(err)
		}
		if got := advplrt.ToString(v); got != c.want {
			t.Errorf("GetEnv(%d args) = %q, want %q", len(c.args), got, c.want)
		}
	}
	t.Setenv(nome, "valor")
	if v, _ := getEnv([]advplrt.Value{advplrt.NewString(nome), advplrt.NewString("padrao")}); advplrt.ToString(v) != "valor" {
		t.Errorf("variável definida: %q", advplrt.ToString(v))
	}
}
