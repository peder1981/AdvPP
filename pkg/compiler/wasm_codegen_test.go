// pkg/compiler/wasm_codegen_test.go
package compiler

import (
	"testing"
)

func TestCompileSimpleToWasm(t *testing.T) {
	source := `
User Function HelloWorld()
    Local nResult := 42
Return nResult
`
	wasm, err := CompileToWasm(source)
	if err != nil {
		t.Fatalf("CompileToWasm failed: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("WASM module is empty")
	}
	// WASM magic number: 0x00 0x61 0x73 0x6d
	if len(wasm) < 4 || wasm[0] != 0x00 || wasm[1] != 0x61 || wasm[2] != 0x73 || wasm[3] != 0x6d {
		t.Errorf("Invalid WASM magic number: %v", wasm[:4])
	}
}

func TestCompileWithMergeFunction(t *testing.T) {
	source := `
User Function MergeCounter(nOld as Numeric, nNew as Numeric) as Numeric
Return Max(nOld, nNew)
`
	wasm, err := CompileToWasm(source)
	if err != nil {
		t.Fatalf("CompileToWasm failed: %v", err)
	}
	if len(wasm) == 0 {
		t.Fatal("WASM module is empty")
	}
}
