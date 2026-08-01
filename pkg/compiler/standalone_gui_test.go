package compiler

import (
	"strings"
	"testing"
)

// Sem -H=windowsgui o .exe do Windows fica no subsistema console: um
// duplo-clique aloca um console, o stub ve stdin como TTY e escolhe a UI de
// terminal, que nao tem MSDIALOG. Cross-compilar de Linux para Windows
// esbarra no mingw, entao a montagem do argv e o que da pra checar aqui.
func TestGoBuildArgsWindowsGUI(t *testing.T) {
	casos := []struct {
		nome   string
		gui    bool
		goos   string
		quer   bool
	}{
		{"windows gui", true, "windows", true},
		{"windows console", false, "windows", false},
		{"linux gui", true, "linux", false},
		{"darwin gui", true, "darwin", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			args := goBuildArgs("app", c.gui, c.goos)
			tem := strings.Contains(strings.Join(args, " "), "-H=windowsgui")
			if tem != c.quer {
				t.Fatalf("goBuildArgs(gui=%v, goos=%q) = %v; -H=windowsgui presente=%v, queria %v",
					c.gui, c.goos, args, tem, c.quer)
			}
			if args[len(args)-1] != "." {
				t.Fatalf("pacote alvo tem que ser o ultimo argumento: %v", args)
			}
		})
	}
}
