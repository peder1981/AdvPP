package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Em Linux e macOS a função tem que ser inofensiva: só o Windows tem o
// problema do d3d12, e relançar processo em outra plataforma seria dano puro.
func TestForcaMesaNaoFazNadaForaDoWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("este teste cobre o caminho não-Windows")
	}
	// Se relançasse, o teste entraria em recursão e nunca retornaria.
	ForcaMesaPorSoftware()
}

// Marca de relançamento presente: não pode relançar de novo, sob pena de
// laço infinito de processos.
func TestForcaMesaNaoRelancaDuasVezes(t *testing.T) {
	t.Setenv(varReexec, "1")
	ForcaMesaPorSoftware()
}

// Sem o Mesa ao lado do executável não há o que forçar, e o programa não
// deve pagar um processo extra.
func TestForcaMesaExigeMesaAoLado(t *testing.T) {
	t.Setenv(varReexec, "")
	t.Setenv("GALLIUM_DRIVER", "")
	exe, err := os.Executable()
	if err != nil {
		t.Skip("sem os.Executable")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "libgallium_wgl.dll")); err == nil {
		t.Skip("há um Mesa ao lado do binário de teste")
	}
	ForcaMesaPorSoftware() // retorna sem relançar
}
