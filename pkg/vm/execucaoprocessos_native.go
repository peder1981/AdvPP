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
			// Loop de 1 a 15 (máximo de 15 parâmetros opcionais, conforme TDN)
			for i := 1; i <= 15 && i < len(args); i++ {
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
			// Nota sobre overflow: channel com buffer de 100 evita bloqueios de IPCGo
			// durante a criação do semaforo em IPCWaitEx. Se mais de 100 sinais chegarem
			// antes de ser consumidos, IPCGo retornará false. Isso é aceitável pois:
			// - Protheus real é single-threaded per work process; múltiplas threads em
			//   paralelo é simulação (não comportamento real)
			// - Implementações de IPC em servidores reais também têm limites (queue size)
			// - A alternativa (sem buffer) causaria deadlock se IPCGo e IPCWaitEx rodarem
			//   em sequência
			state = &ipcSemaphoreState{
				waiters: 0,
				ch:      make(chan []advplrt.Value, 100),
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
			// Recebeu dados de IPCGo — retorna .T.
			//
			// LIMITAÇÃO: AdvPP não implementa suporte a argumentos por referência
			// para funções nativas. A TDN especifica que IPCWaitEx deve permitir
			// até 15 argumentos por referência (via @variável) que seriam mutados
			// com os dados recebidos de IPCGo. Isso é uma limitação arquitetural
			// do VM que afeta potencialmente várias outras funções.
			//
			// Comportamento atual: a função retorna .T. corretamente (sinal recebido),
			// mas os dados passados por IPCGo não são refletidos nas variáveis do
			// caller. Para usar IPC corretamente nesta implementação, seria necessário
			// repensar o mecanismo de chamada de nativas ou usar workarounds (ex.:
			// variáveis globais, returns estruturados).
			//
			// Ver: https://github.com/advpl/compiler/issues/XXX (design de byref nativas)
			_ = data // dados recebidos, mas não podem ser passados ao caller

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
