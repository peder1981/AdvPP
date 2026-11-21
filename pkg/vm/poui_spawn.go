package vm

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"sync"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// poSpawn é um processo filho de PoApp:Spawn. Executa SEM shell; stdout e
// stderr chegam ao laço linha a linha (eventos "line") e o código de saída
// por último (evento "exit").
type poSpawn struct {
	prog     string
	args     []string
	bLinha   advplrt.Value
	bFim     advplrt.Value
	cmd      *exec.Cmd
	iniciado bool
	parar    chan struct{} // fechado no fim do laço: a leitura para de postar
}

const poSpawnMaxLinha = 1 << 20 // 1 MB por linha

func (a *poAppState) spawn(prog string, args []string, bLinha, bFim advplrt.Value) int {
	a.nextSpawn++
	sp := &poSpawn{prog: prog, args: args, bLinha: bLinha, bFim: bFim}
	a.spawns[a.nextSpawn] = sp
	if a.active {
		a.startSpawn(a.nextSpawn, sp)
	}
	return a.nextSpawn
}

// startSpawn inicia o processo. Roda na goroutine da VM; as goroutines de
// leitura só postam eventos (com contrapressão: UIPostWait).
func (a *poAppState) startSpawn(id int, sp *poSpawn) {
	if sp.iniciado {
		return
	}
	sp.iniciado = true
	sp.parar = make(chan struct{})
	ui, parar := a.ui, sp.parar
	cmd := exec.Command(sp.prog, sp.args...)
	stdout, err1 := cmd.StdoutPipe()
	stderr, err2 := cmd.StderrPipe()
	if err1 != nil || err2 != nil || cmd.Start() != nil {
		go ui.UIPostWait(id, "exit", []byte("-1"), parar)
		return
	}
	sp.cmd = cmd
	go func() {
		var wg sync.WaitGroup
		ler := func(r io.Reader, erro bool) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 64*1024), poSpawnMaxLinha)
			postando := true
			for sc.Scan() {
				if !postando {
					continue // laço/sessão encerrados: só drena até o pipe fechar
				}
				data, _ := json.Marshal(map[string]any{"t": sc.Text(), "e": erro})
				postando = ui.UIPostWait(id, "line", data, parar)
			}
		}
		wg.Add(2)
		go ler(stdout, false)
		go ler(stderr, true)
		wg.Wait() // todas as linhas antes do fim
		code := 0
		if err := cmd.Wait(); err != nil {
			code = -1
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() >= 0 {
				code = ee.ExitCode()
			}
		}
		ui.UIPostWait(id, "exit", []byte(strconv.Itoa(code)), parar)
	}()
}

func (a *poAppState) killSpawn(id int) {
	if sp, ok := a.spawns[id]; ok && sp.cmd != nil {
		_ = sp.cmd.Process.Kill() // já terminado: os.ErrProcessDone, inofensivo
	}
}

// killSpawns encerra os processos ao fim do laço (Close ou sessão abandonada)
// e libera as goroutines de leitura, que deixam de postar.
func (a *poAppState) killSpawns() {
	for id, sp := range a.spawns {
		a.killSpawn(id)
		if sp.parar != nil {
			close(sp.parar)
		}
		delete(a.spawns, id)
	}
}

// poSpawnEvento converte "line"/"exit" nos argumentos dos blocos.
func (a *poAppState) poSpawnEvento(id int, ev string, val []byte) (advplrt.Value, []advplrt.Value) {
	sp, ok := a.spawns[id]
	if !ok {
		return nil, nil
	}
	if ev == "line" {
		var l struct {
			T string `json:"t"`
			E bool   `json:"e"`
		}
		_ = json.Unmarshal(val, &l)
		return sp.bLinha, []advplrt.Value{advplrt.NewString(l.T), advplrt.NewBool(l.E)}
	}
	code, _ := strconv.Atoi(string(val))
	delete(a.spawns, id)
	return sp.bFim, []advplrt.Value{advplrt.NewNumber(float64(code))}
}
