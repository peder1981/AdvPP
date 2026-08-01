package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefineEnvTrocaEmVezDeDuplicar(t *testing.T) {
	// Duplicar a chave deixa a escolha para a plataforma. Este teste existe
	// porque a versão anterior fazia append cego em os.Environ().
	env := []string{"A=1", "GOFLAGS=-x", "B=2"}
	env = defineEnv(env, "GOFLAGS", "-mod=mod")

	n := 0
	for _, e := range env {
		if strings.HasPrefix(e, "GOFLAGS=") {
			n++
			if e != "GOFLAGS=-mod=mod" {
				t.Fatalf("valor errado: %q", e)
			}
		}
	}
	if n != 1 {
		t.Fatalf("esperava 1 GOFLAGS, achei %d em %v", n, env)
	}
	if len(env) != 3 {
		t.Fatalf("nao devia ter crescido: %v", env)
	}
}

func TestDefineEnvAcrescentaQuandoNaoExiste(t *testing.T) {
	env := defineEnv([]string{"A=1"}, "CC", "/usr/bin/gcc")
	if len(env) != 2 || env[1] != "CC=/usr/bin/gcc" {
		t.Fatalf("esperava a chave acrescentada, veio %v", env)
	}
}

func TestGoBinarioUsaADVPPGO(t *testing.T) {
	falso := filepath.Join(t.TempDir(), nomeExecutavel("go"))
	if err := os.WriteFile(falso, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADVPP_GO", falso)

	got, err := goBinario()
	if err != nil {
		t.Fatal(err)
	}
	if got != falso {
		t.Fatalf("ADVPP_GO tem precedencia: got %q, quer %q", got, falso)
	}
}

func TestGoBinarioRecusaADVPPGOInvalida(t *testing.T) {
	// Silenciosamente cair no PATH esconderia o erro de digitacao do usuario
	// ate o build falhar por outro motivo.
	t.Setenv("ADVPP_GO", filepath.Join(t.TempDir(), "nao-existe"))
	if _, err := goBinario(); err == nil {
		t.Fatal("esperava erro para ADVPP_GO apontando para nada")
	}
}

func TestGoBinarioCaiNoPATH(t *testing.T) {
	t.Setenv("ADVPP_GO", "")
	got, err := goBinario()
	if err != nil {
		t.Fatal(err)
	}
	// Sem toolchain instalado ao lado do advplc (caso de quem roda os testes),
	// tem que continuar sendo o "go" do PATH — o comportamento de sempre.
	if pastaToolchain() == "" && got != "go" {
		t.Fatalf("sem toolchain deveria usar o PATH, veio %q", got)
	}
}

func TestAmbienteDeBuildSempreTemGoflags(t *testing.T) {
	t.Setenv("ADVPP_GO", "")
	env := ambienteDeBuild()
	achou := false
	for _, e := range env {
		if e == "GOFLAGS=-mod=mod" {
			achou = true
		}
	}
	if !achou {
		t.Fatal("GOFLAGS=-mod=mod é o que evita exigir um go mod tidy antes do build")
	}
}

func TestAmbienteDeBuildNaoDefineGOROOT(t *testing.T) {
	// GOROOT errado é falha clássica e ilegível; o Go deduz o dele sozinho a
	// partir do binário chamado.
	t.Setenv("ADVPP_GO", "")
	antes := os.Getenv("GOROOT")
	for _, e := range ambienteDeBuild() {
		if strings.HasPrefix(e, "GOROOT=") && e != "GOROOT="+antes {
			t.Fatalf("nao deveria mexer em GOROOT: %q", e)
		}
	}
}

// montaToolchain cria uma pasta toolchain/ ao lado de um "advplc" falso e
// devolve a pasta. Não dá para usar pastaToolchain() nos testes porque ela
// olha para os.Executable(), que no teste é o binário do próprio go test —
// então os testes abaixo exercem gccToolchain indiretamente, pela busca.
func montaGcc(t *testing.T, subpasta string) string {
	t.Helper()
	tc := t.TempDir()
	bin := filepath.Join(tc, subpasta, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	gcc := filepath.Join(bin, nomeExecutavel("gcc"))
	if err := os.WriteFile(gcc, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return gcc
}

// O nome da pasta raiz vem do zip do fornecedor: winlibs usa mingw64, outro
// empacotador usaria outra coisa. A busca existe para isso não virar edição
// de código a cada troca.
func TestGccEncontradoSejaQualForONomeDaPasta(t *testing.T) {
	for _, nome := range []string{"mingw", "mingw64", "winlibs-x86_64-13.2.0"} {
		gcc := montaGcc(t, nome)
		tc := filepath.Dir(filepath.Dir(filepath.Dir(gcc)))
		if got := buscaGccEm(tc); got != gcc {
			t.Errorf("pasta %q: got %q, quer %q", nome, got, gcc)
		}
	}
}

func TestGccIgnoraAPastaDoGo(t *testing.T) {
	// go/bin/gcc não existe numa distribuição real do Go; se existisse, seria
	// coincidência e não o compilador C que se procura.
	gcc := montaGcc(t, "go")
	tc := filepath.Dir(filepath.Dir(filepath.Dir(gcc)))
	if got := buscaGccEm(tc); got != "" {
		t.Fatalf("nao devia aceitar go/bin/gcc: %q", got)
	}
}

func TestGccVazioQuandoNaoHaToolchain(t *testing.T) {
	if got := buscaGccEm(t.TempDir()); got != "" {
		t.Fatalf("esperava vazio, veio %q", got)
	}
}
