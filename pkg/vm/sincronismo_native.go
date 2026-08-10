package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerSincronismoNatives registra funções de sincronismo de dados:
// GlbNmLock, GlbNmUnlock.
func (v *VM) registerSincronismoNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// GlbNmLock(cText) -> lLck — realiza o bloqueio de um identificador nomeado
	// Retorna .T. se o bloqueio foi obtido com sucesso, .F. se já está bloqueado.
	natives["GLBNMLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cText := getArg(args, 0)
		lockName := ""
		if str, ok := cText.(*advplrt.StringValue); ok {
			lockName = str.Val
		} else {
			// Convert to string if needed
			lockName = advplrt.ToString(cText)
		}

		v.namedLocksMu.Lock()
		defer v.namedLocksMu.Unlock()

		if v.namedLocks[lockName] {
			// Already locked - return false
			return advplrt.NewBool(false), nil
		}

		// Acquire lock
		v.namedLocks[lockName] = true
		return advplrt.NewBool(true), nil
	}

	// GlbNmUnlock(cText) -> lLck — libera um bloqueio de um identificador nomeado
	// Retorna .T. se o bloqueio foi liberado com sucesso, .F. se não existe ou não pertence ao processo.
	natives["GLBNMUNLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cText := getArg(args, 0)
		lockName := ""
		if str, ok := cText.(*advplrt.StringValue); ok {
			lockName = str.Val
		} else {
			// Convert to string if needed
			lockName = advplrt.ToString(cText)
		}

		v.namedLocksMu.Lock()
		defer v.namedLocksMu.Unlock()

		if !v.namedLocks[lockName] {
			// Lock doesn't exist - can't unlock
			return advplrt.NewBool(false), nil
		}

		// Release the lock
		delete(v.namedLocks, lockName)
		return advplrt.NewBool(true), nil
	}
}
