package vm

import (
	"os"
	"sort"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerControleimpressaoNatives registra as funções de Controle de
// impressão. Das 19 funções da categoria, 18 são stubs genuínos do TDN
// (páginas com corpo vazio — ver docs/tdn-gap-stubs.md): __Eject, _PCol,
// _PRow, DevOut, DevOutPict, DevPos, FechaRel, GetConnStatus, GetImpInf,
// InitPrint, PreparePrint, PrintOut, PrnFlush, QOut, QQOut, RmvToken,
// SetPrc, SndToPrnWin. A única com spec real é GetPortActive.
func (v *VM) registerControleimpressaoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// GetPortActive( < lDirect > ) -> aPort — array com os nomes das portas
	// disponíveis. Sem infraestrutura de impressora no runtime embutido, o
	// retorno honesto é a enumeração real das portas seriais/paralelas
	// presentes no SO (lDirect distingue AppServer do Smart Client no
	// Protheus; aqui não há distinção — ambas retornam a mesma enumeração).
	// Nenhuma porta encontrada => {} (comportamento das builds >7.00.111010P).
	natives["GETPORTACTIVE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		ports := enumerateSerialPorts()
		elems := make([]advplrt.Value, 0, len(ports))
		for _, p := range ports {
			elems = append(elems, advplrt.NewString(p))
		}
		return advplrt.NewArray(elems), nil
	}
}

// enumerateSerialPorts devolve os nomes das portas seriais/paralelas
// presentes no sistema via /dev (Linux): ttyS*, ttyUSB*, ttyACM*, ttyAMA*,
// lp*. Ordenado para saída determinística.
func enumerateSerialPorts() []string {
	var ports []string
	for _, dev := range []string{"/dev"} {
		entries, err := os.ReadDir(dev)
		if err != nil {
			return ports
		}
		for _, e := range entries {
			name := e.Name()
			for _, prefix := range []string{"ttyS", "ttyUSB", "ttyACM", "ttyAMA", "ttyXRUSB", "lp"} {
				if strings.HasPrefix(name, prefix) {
					ports = append(ports, name)
					break
				}
			}
		}
	}
	sort.Strings(ports)
	return ports
}
