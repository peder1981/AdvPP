package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// `advplc build` embute o bytecode num stub Go e chama `go build`. Isso exige
// o toolchain Go e — porque o stub importa o Fyne — um compilador C, já que
// CGO é obrigatório: sem ele o build morre em
// "go-gl/gl: build constraints exclude all Go files".
//
// Exigir os dois no PATH é razoável na máquina de quem desenvolve o AdvPP, e
// não é em quem só instalou o produto. O instalador do Windows pode baixar
// ambos para uma pasta `toolchain/` ao lado do advplc; as funções deste
// arquivo é que fazem essa pasta ser encontrada. Sem ela, tudo continua
// exatamente como antes: `go` e `gcc` vêm do PATH.
//
// `advplc run`, `check` e `serve` não passam por aqui — só o `build`.

// nomeExecutavel acrescenta a extensão da plataforma quando há uma.
func nomeExecutavel(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// pastaToolchain devolve a pasta de ferramentas instalada junto do advplc, ou
// "" quando não existe.
func pastaToolchain() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// EvalSymlinks porque em Linux o advplc costuma ser alcançado por um link
	// em /usr/local/bin, e a pasta de ferramentas fica junto do arquivo real.
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dir := filepath.Join(filepath.Dir(exe), "toolchain")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

// arquivoExecutavel diz se caminho existe e não é diretório.
func arquivoExecutavel(caminho string) bool {
	st, err := os.Stat(caminho)
	return err == nil && !st.IsDir()
}

// goBinario decide qual `go` chamar: ADVPP_GO, depois o toolchain instalado
// ao lado do advplc, depois o PATH.
//
// Devolve erro em vez de deixar o exec falhar sozinho porque "exec: go:
// executable file not found in $PATH" não diz a quem lê o que fazer a
// respeito.
func goBinario() (string, error) {
	if env := strings.TrimSpace(os.Getenv("ADVPP_GO")); env != "" {
		if arquivoExecutavel(env) {
			return env, nil
		}
		return "", fmt.Errorf("ADVPP_GO aponta para %q, que não é um executável", env)
	}
	if tc := pastaToolchain(); tc != "" {
		if cand := filepath.Join(tc, "go", "bin", nomeExecutavel("go")); arquivoExecutavel(cand) {
			return cand, nil
		}
	}
	return "go", nil
}

// gccToolchain devolve o compilador C instalado junto do advplc, ou "".
//
// Procura em vez de assumir um caminho fixo: o nome da pasta raiz depende de
// como o distribuidor empacotou o zip (mingw64, mingw32, o nome completo da
// build) e mudar de fornecedor não deveria exigir mexer aqui. A busca é rasa
// — dois níveis, só pastas `bin` — então não sai varrendo disco.
func gccToolchain() string {
	tc := pastaToolchain()
	if tc == "" {
		return ""
	}
	return buscaGccEm(tc)
}

// buscaGccEm procura o gcc dentro de uma pasta de toolchain. Separada de
// gccToolchain para poder ser testada sem depender de os.Executable().
func buscaGccEm(tc string) string {
	gcc := nomeExecutavel("gcc")

	if cand := filepath.Join(tc, "bin", gcc); arquivoExecutavel(cand) {
		return cand
	}
	entradas, err := os.ReadDir(tc)
	if err != nil {
		return ""
	}
	for _, e := range entradas {
		// A pasta do Go e pulada: um gcc ali seria coincidencia, nao o
		// compilador C que se procura.
		if !e.IsDir() || e.Name() == "go" {
			continue
		}
		if cand := filepath.Join(tc, e.Name(), "bin", gcc); arquivoExecutavel(cand) {
			return cand
		}
	}
	return ""
}

// defineEnv troca (ou acrescenta) uma variável. Acrescentar sem trocar
// deixaria a mesma chave duas vezes no ambiente, e qual delas vale fica por
// conta da plataforma — comportamento que não se quer descobrir em produção.
func defineEnv(env []string, chave, valor string) []string {
	prefixo := chave + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefixo) {
			env[i] = prefixo + valor
			return env
		}
	}
	return append(env, prefixo+valor)
}

// ambienteDeBuild monta o ambiente do `go build`.
//
// GOFLAGS=-mod=mod: o go.mod temporário declara só o módulo do stub, sem as
// dependências transitivas nem go.sum. -mod=mod deixa o próprio `go build`
// completar o grafo em vez de exigir um `go mod tidy` antes.
//
// GOROOT não é definido de propósito: o Go deduz o dele a partir da
// localização do binário chamado, e um GOROOT errado é falha clássica e
// difícil de ler.
func ambienteDeBuild() []string {
	env := defineEnv(os.Environ(), "GOFLAGS", "-mod=mod")

	gcc := gccToolchain()
	if gcc == "" {
		return env
	}
	// CC explícito em vez de confiar na ordem do PATH, e o bin do mingw na
	// frente do PATH porque o gcc chama as próprias ferramentas auxiliares
	// (as, ld, collect2) por nome.
	env = defineEnv(env, "CC", gcc)
	env = defineEnv(env, "CGO_ENABLED", "1")
	bin := filepath.Dir(gcc)
	env = defineEnv(env, "PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return env
}
