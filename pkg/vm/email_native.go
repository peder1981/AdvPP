package vm

import (
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerTratamentodeemailNatives registra funções de tratamento de email:
// GetMailObj, SetMailObj.
func (v *VM) registerTratamentodeemailNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// GetMailObj(cID) -> oMail
	natives["GETMAILOBJ"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cID := advplrt.ToString(getArg(args, 0))

		v.mailObjectsMu.Lock()
		defer v.mailObjectsMu.Unlock()

		if obj, exists := v.mailObjects[cID]; exists {
			return obj, nil
		}
		return advplrt.Nil, nil
	}

	// SetMailObj(cID, oMailObj) -> Nil
	natives["SETMAILOBJ"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cID := advplrt.ToString(getArg(args, 0))
		oMailObj := getArg(args, 1)

		v.mailObjectsMu.Lock()
		defer v.mailObjectsMu.Unlock()

		if oMailObj == advplrt.Nil {
			// Remove the object if Nil is passed
			delete(v.mailObjects, cID)
		} else {
			// Store the object
			v.mailObjects[cID] = oMailObj
		}
		return advplrt.Nil, nil
	}
}
