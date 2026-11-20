package compiler

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
)

// BytecodeFile represents the serialized bytecode file format
type BytecodeFile struct {
	Version    string        `json:"version"`
	Constants  []Constant    `json:"constants"`
	Functions  FunctionMap   `json:"functions"`
	Classes    ClassMap      `json:"classes"`
	Code       []Instruction `json:"code"`
	MainOffset int           `json:"mainOffset"`
	NumGlobals int           `json:"numGlobals"`
}

// FunctionMap represents a core AdvPP type.
type FunctionMap map[string]*FunctionInfo

// ClassMap represents a core AdvPP type.
type ClassMap map[string]*ClassInfo

// SaveBytecode saves bytecode to a file
func SaveBytecode(bc *Bytecode, filename string) error {
	bf := BytecodeFile{
		Version:    "1.0",
		Constants:  bc.Constants,
		Functions:  bc.Functions,
		Classes:    bc.Classes,
		Code:       bc.Code,
		MainOffset: bc.MainOffset,
		NumGlobals: bc.NumGlobals,
	}

	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadBytecode loads bytecode from a file
func LoadBytecode(filename string) (*Bytecode, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var bf BytecodeFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, err
	}

	if err := bf.validate(); err != nil {
		return nil, fmt.Errorf("bytecode inválido em %s: %w", filename, err)
	}

	return &Bytecode{
		Constants:  bf.Constants,
		Functions:  bf.Functions,
		Classes:    bf.Classes,
		Code:       bf.Code,
		MainOffset: bf.MainOffset,
		NumGlobals: bf.NumGlobals,
	}, nil
}

// maxSlots: teto de globais/locais/params; maxArg: teto de |Arg|/|Arg2|
// (contagens de array/argumentos viram make() direto na VM).
const (
	maxSlots = 1 << 16
	maxArg   = 1 << 24
)

// validate barra os campos de um .bc (arquivo não confiável — `advplc exec`)
// que a VM usa como tamanho/índice sem checar: valores negativos ou enormes
// derrubavam o processo (makeslice/index out of range) ou esgotavam a
// memória (NumGlobals = 2^40 → OOM fatal, não recuperável).
// ponytail: checagem estrutural, não semântica por opcode; um Arg fora do
// alcance do opcode ainda vira pânico Go (seguro, sem OOM).
func (bf *BytecodeFile) validate() error {
	if bf.NumGlobals < 0 || bf.NumGlobals > maxSlots {
		return fmt.Errorf("numGlobals fora do limite: %d", bf.NumGlobals)
	}
	if bf.MainOffset < 0 || bf.MainOffset > len(bf.Code) {
		return fmt.Errorf("mainOffset fora do código: %d", bf.MainOffset)
	}
	for name, fn := range bf.Functions {
		if fn == nil {
			return fmt.Errorf("função %q vazia", name)
		}
		if fn.NumLocals < 0 || fn.NumLocals > maxSlots || fn.NumParams < 0 || fn.NumParams > maxSlots {
			return fmt.Errorf("função %q: locais/params fora do limite", name)
		}
		if !fn.IsNative && (fn.Offset < 0 || fn.Offset > len(bf.Code)) {
			return fmt.Errorf("função %q: offset fora do código: %d", name, fn.Offset)
		}
	}
	for i, ins := range bf.Code {
		if ins.Arg < -maxArg || ins.Arg > maxArg || ins.Arg2 < -maxArg || ins.Arg2 > maxArg {
			return fmt.Errorf("instrução %d: operando fora do limite", i)
		}
	}
	return nil
}

// MarshalJSON: JSON não representa ±Inf/NaN (ex.: literal 1.8e308 estoura
// float64) — esses viram string ("+Inf", "-Inf", "NaN"); números finitos
// continuam número, então o formato dos .bc existentes não muda.
func (c Constant) MarshalJSON() ([]byte, error) {
	type plain Constant
	if !math.IsInf(c.Num, 0) && !math.IsNaN(c.Num) {
		return json.Marshal(plain(c))
	}
	return json.Marshal(struct{ Type, Num, Str string }{c.Type, strconv.FormatFloat(c.Num, 'g', -1, 64), c.Str})
}

// UnmarshalJSON aceita Num como número ou como a string gerada acima.
func (c *Constant) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string
		Num  json.RawMessage
		Str  string
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Type, c.Str, c.Num = raw.Type, raw.Str, 0
	if len(raw.Num) == 0 || string(raw.Num) == "null" {
		return nil
	}
	s := string(raw.Num)
	if s[0] == '"' {
		if err := json.Unmarshal(raw.Num, &s); err != nil {
			return err
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("constante numérica inválida: %s", raw.Num)
	}
	c.Num = n
	return nil
}
