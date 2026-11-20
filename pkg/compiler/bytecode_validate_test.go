package compiler

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// .bc adulterado não pode derrubar a VM (OOM/pânico): LoadBytecode recusa.
func TestLoadBytecodeRejectsHostile(t *testing.T) {
	cases := map[string]string{
		"numGlobals negativo": `{"code":[],"numGlobals":-5}`,
		"numGlobals gigante":  `{"code":[],"numGlobals":1099511627776}`,
		"mainOffset fora":     `{"code":[],"mainOffset":99}`,
		"locais negativos":    `{"code":[],"functions":{"T":{"NumLocals":-1}}}`,
		"operando gigante":    `{"code":[{"Op":1,"Arg":1099511627776}]}`,
	}
	for name, js := range cases {
		path := filepath.Join(t.TempDir(), "x.bc")
		os.WriteFile(path, []byte(js), 0o600)
		if _, err := LoadBytecode(path); err == nil {
			t.Errorf("%s: esperava erro", name)
		}
	}
	path := filepath.Join(t.TempDir(), "ok.bc")
	os.WriteFile(path, []byte(`{"code":[{"Op":1,"Arg":-1,"Arg2":32772}],"numGlobals":3}`), 0o600)
	if _, err := LoadBytecode(path); err != nil {
		t.Errorf("bytecode válido rejeitado: %v", err)
	}
}

// Constante ±Inf/NaN (literal 1.8e308) precisa sobreviver a Save/Load —
// antes `advplc compile` falhava com "json: unsupported value: +Inf".
func TestBytecodeNonFiniteConstants(t *testing.T) {
	bc := &Bytecode{Constants: []Constant{
		{Type: "number", Num: math.Inf(1)}, {Type: "number", Num: math.Inf(-1)},
		{Type: "number", Num: math.NaN()}, {Type: "number", Num: 1.5}, {Type: "string", Str: "x"},
	}}
	path := filepath.Join(t.TempDir(), "inf.bc")
	if err := SaveBytecode(bc, path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBytecode(path)
	if err != nil {
		t.Fatal(err)
	}
	c := got.Constants
	if !math.IsInf(c[0].Num, 1) || !math.IsInf(c[1].Num, -1) || !math.IsNaN(c[2].Num) || c[3].Num != 1.5 || c[4].Str != "x" {
		t.Fatalf("constantes não sobreviveram: %+v", c)
	}
}
