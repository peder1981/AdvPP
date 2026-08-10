package vm

import (
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerExecucaoentreprocessosNatives registra funções de execução entre processos:
// IPCCount, IPCGo, IPCWaitEx.
func (v *VM) registerExecucaoentreprocessosNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// IPCCount(cSemaforo) -> nRet — retorna o número de threads livres em espera
	natives["IPCCOUNT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		semaphore := getArgString(args, 0, "")
		// Normaliza o nome do semaforo para uppercase (conforme TDN)
		semaphore = strings.ToUpper(semaphore)

		v.ipcSemaphoresMu.Lock()
		defer v.ipcSemaphoresMu.Unlock()

		state, exists := v.ipcSemaphores[semaphore]
		if !exists {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(float64(state.waiters)), nil
	}

	// IPCGo(cSemaforo, [param1, param2, ...]) -> lRet — envia sinal para primeira thread em espera
	// Retorna .T. se conseguiu enviar para um waiter, .F. caso contrário
	natives["IPCGO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		semaphore := getArgString(args, 0, "")
		// Normaliza o nome do semaforo para uppercase (conforme TDN)
		semaphore = strings.ToUpper(semaphore)

		v.ipcSemaphoresMu.Lock()
		state, exists := v.ipcSemaphores[semaphore]
		if !exists || state.waiters == 0 {
			v.ipcSemaphoresMu.Unlock()
			return advplrt.NewBool(false), nil
		}

		// Extrai os argumentos opcionais (até 15) a passar para o waiter
		var data []advplrt.Value
		if len(args) > 1 {
			for i := 1; i < len(args) && i <= 16; i++ {
				data = append(data, getArg(args, i))
			}
		}

		// Decrementa o contador de waiters e tenta enviar os dados
		state.waiters--
		v.ipcSemaphoresMu.Unlock()

		// Tenta enviar os dados sem bloquear (não-bloqueante)
		select {
		case state.ch <- data:
			return advplrt.NewBool(true), nil
		default:
			// Se não conseguir enviar (channel cheio), retorna false
			return advplrt.NewBool(false), nil
		}
	}

	// IPCWaitEx(cSemaforo, nTimeOut, [param1, param2, ...]) -> lRet
	// Coloca a thread em espera até que IPCGo sinalize ou timeout expire
	// Retorna .T. se receber sinal de IPCGo, .F. se timeout
	natives["IPCWAITEX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		semaphore := getArgString(args, 0, "")
		// Normaliza o nome do semaforo para uppercase (conforme TDN)
		semaphore = strings.ToUpper(semaphore)

		timeoutMs := int64(advplrt.ToFloat(getArg(args, 1)))

		v.ipcSemaphoresMu.Lock()
		// Cria o semaforo se não existir
		state, exists := v.ipcSemaphores[semaphore]
		if !exists {
			state = &ipcSemaphoreState{
				waiters: 0,
				ch:      make(chan []advplrt.Value, 100), // buffer para evitar bloqueios
			}
			v.ipcSemaphores[semaphore] = state
		}
		// Incrementa o contador de waiters
		state.waiters++
		v.ipcSemaphoresMu.Unlock()

		// Aguarda dados com timeout
		timeout := time.Duration(timeoutMs) * time.Millisecond
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case data := <-state.ch:
			// Recebeu dados de IPCGo
			// Agora precisa atualizar os argumentos por referência
			// Para simplificar a implementação inicial, apenas retornamos true
			// A passagem de dados por referência é complexa em AdvPL e requer
			// análise de como os argumentos são passados

			// Se houver argumentos por referência (a partir do índice 2),
			// atualiza-os com os dados recebidos
			for i, val := range data {
				argIdx := i + 2 // começa no índice 2 (após semaforo e timeout)
				if argIdx < len(args) {
					// Para argumentos por referência, seria necessário um mecanismo especial
					// Por enquanto, apenas registramos que os dados foram recebidos
					_ = val
				}
			}

			return advplrt.NewBool(true), nil
		case <-timer.C:
			// Timeout expirou
			v.ipcSemaphoresMu.Lock()
			// Decrementa o contador de waiters
			if state.waiters > 0 {
				state.waiters--
			}
			// Se não houver mais waiters, limpa o semaforo
			if state.waiters == 0 {
				delete(v.ipcSemaphores, semaphore)
			}
			v.ipcSemaphoresMu.Unlock()
			return advplrt.NewBool(false), nil
		}
	}
}
