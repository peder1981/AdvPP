package compiler

import (
	"encoding/json"
	"os"
)

// BytecodeFile represents the serialized bytecode file format
type BytecodeFile struct {
	Version    string      `json:"version"`
	Constants  []Constant  `json:"constants"`
	Functions  FunctionMap `json:"functions"`
	Classes    ClassMap    `json:"classes"`
	Code       []Instruction `json:"code"`
	MainOffset int         `json:"mainOffset"`
	NumGlobals int         `json:"numGlobals"`
}

type FunctionMap map[string]*FunctionInfo
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

	return &Bytecode{
		Constants:  bf.Constants,
		Functions:  bf.Functions,
		Classes:    bf.Classes,
		Code:       bf.Code,
		MainOffset: bf.MainOffset,
		NumGlobals: bf.NumGlobals,
	}, nil
}
