package contract

import (
	"fmt"
	"sync"
)

// ContractRuntime manages WASM contract loading and execution.
type ContractRuntime struct {
	mu        sync.RWMutex
	contracts map[string]*Contract // Key → Contract
}

// NewContractRuntime creates a new contract runtime.
func NewContractRuntime() *ContractRuntime {
	return &ContractRuntime{
		contracts: make(map[string]*Contract),
	}
}

// LoadContract loads a contract into the runtime.
func (cr *ContractRuntime) LoadContract(c *Contract) error {
	if c == nil || len(c.Code) == 0 {
		return fmt.Errorf("invalid contract: nil or empty code")
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.contracts[c.Key] = c
	return nil
}

// GetContract retrieves a contract by key.
func (cr *ContractRuntime) GetContract(key string) *Contract {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return cr.contracts[key]
}

// ExecuteContract calls a function in a contract (stub for now).
func (cr *ContractRuntime) ExecuteContract(key, fn string, args []interface{}) (interface{}, error) {
	cr.mu.RLock()
	contract := cr.contracts[key]
	cr.mu.RUnlock()

	if contract == nil {
		return nil, fmt.Errorf("contract not found: %s", key)
	}

	// TODO: instantiate WASM and call function via wasmtime
	return nil, fmt.Errorf("execution not yet implemented")
}
