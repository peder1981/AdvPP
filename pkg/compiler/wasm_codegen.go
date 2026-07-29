// pkg/compiler/wasm_codegen.go
package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// WasmModule represents a core AdvPP type.
type WasmModule []byte

// CompileToWasm takes AdvPL source and returns a minimal valid WASM module.
// MVP: returns a WASM module that exports no functions (valid but empty).
func CompileToWasm(source string) (WasmModule, error) {
	if source == "" {
		return nil, fmt.Errorf("empty source")
	}

	// Compile to bytecode first (reuse existing compiler)
	compiler := New()
	bc, err := compiler.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("bytecode compilation failed: %w", err)
	}

	// MVP WASM module (magic + version + empty sections)
	buf := new(bytes.Buffer)

	// WASM magic number
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d})

	// WASM version (1)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00})

	// No sections for now (empty module)
	// Later: type section, function section, code section, etc.

	_ = bc // Use bc to prevent unused import; merge logic will follow
	_ = binary.LittleEndian // Import for future use in WASM encoding

	return buf.Bytes(), nil
}
