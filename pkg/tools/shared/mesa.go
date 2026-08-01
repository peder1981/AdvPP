// NOTA: nome sem sufixo _windows de proposito. O sufixo e restricao de build
// do Go e excluiria este arquivo em Linux e macOS, onde a funcao ainda
// precisa existir (como no-op) para os chamadores compilarem.

package shared

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// varReexec marca o processo já relançado, para não relançar em laço.
const varReexec = "ADVPP_MESA_LLVMPIPE"

// ForcaMesaPorSoftware relança o próprio processo com o Mesa3D fixado no
// driver llvmpipe, quando há um Mesa instalado ao lado do executável.
//
// Por que isto existe: no Windows o Mesa tenta o driver d3d12 antes do
// llvmpipe. Em máquina virtual — QEMU/QXL comprovadamente, e provavelmente
// outras — o d3d12 inicializa pela metade e o processo morre com
// 0x80070057 DEPOIS de já ter criado e desenhado a janela. O sintoma é
// cruel: a janela aparece por um instante e some, e nada é registrado.
//
// Por que relançar em vez de só definir a variável: o Mesa lê GALLIUM_DRIVER
// com getenv() do runtime C, que trabalha sobre uma cópia do ambiente feita
// na inicialização do processo. os.Setenv chama SetEnvironmentVariable, que
// não atualiza essa cópia — definir a variável dentro do próprio processo não
// tem efeito nenhum. Ela precisa estar no bloco de ambiente na hora do
// CreateProcess, e a única forma de conseguir isso sem um executável lançador
// separado é o programa criar o próprio filho.
//
// Só age quando o Mesa está de fato ao lado do executável. Máquina com OpenGL
// próprio não instala o Mesa, não entra aqui, e não paga processo nenhum.
//
// Não retorna quando relança: encerra o processo pai com o código do filho.
func ForcaMesaPorSoftware() {
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv(varReexec) != "" {
		return // já é o processo relançado
	}
	// Respeita quem definiu a variável à mão, para diagnóstico ou para testar
	// outro driver.
	if os.Getenv("GALLIUM_DRIVER") != "" {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	// libgallium_wgl.dll é o driver de verdade do Mesa; opengl32.dll sozinho
	// é só o carregador e existe também no System32.
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "libgallium_wgl.dll")); err != nil {
		return // sem Mesa ao lado: nada a forçar
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(),
		"GALLIUM_DRIVER=llvmpipe",
		"LIBGL_ALWAYS_SOFTWARE=1",
		varReexec+"=1")
	// Herda os descritores: o filho é o programa de verdade, e quem redirecionou
	// a saída (um .bat de diagnóstico, por exemplo) tem que continuar vendo tudo.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if saida, ok := err.(*exec.ExitError); ok {
			os.Exit(saida.ExitCode())
		}
		// Não conseguiu nem criar o filho: segue neste processo mesmo. Pior
		// tentar e falhar do que não abrir por causa da tentativa.
		return
	}
	os.Exit(0)
}
