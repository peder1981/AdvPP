package compiler

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

// SerializeBytecode converte bytecode JSON para formato binário
func SerializeBytecode(jsonPath string) ([]byte, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("lendo bytecode: %w", err)
	}

	var bf BytecodeFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parseando bytecode: %w", err)
	}

	var buf []byte
	
	// Version
	version := []byte("1.0\x00")
	buf = append(buf, version...)
	
	// Constants
	buf = append(buf, encodeConstants(bf.Constants)...)
	
	// Functions
	buf = append(buf, encodeFunctions(bf.Functions)...)
	
	// Classes
	buf = append(buf, encodeClasses(bf.Classes)...)
	
	// Code
	buf = append(buf, encodeCode(bf.Code)...)
	
	return buf, nil
}

func encodeConstants(constants []Constant) []byte {
	var buf []byte
	
	buf = append(buf, encodeUint32(uint32(len(constants)))...)
	
	for _, c := range constants {
		var typeByte byte
		if c.Type == "string" {
			typeByte = 0
		} else if c.Type == "number" {
			typeByte = 1
		} else {
			typeByte = 2
		}
		buf = append(buf, typeByte)
		buf = append(buf, encodeUint32(uint32(c.Num))...)
		if c.Type == "string" {
			buf = append(buf, []byte(c.Str)...)
			buf = append(buf, 0)
		}
	}
	
	return buf
}

func encodeFunctions(funcs FunctionMap) []byte {
	var buf []byte
	
	buf = append(buf, encodeUint32(uint32(len(funcs)))...)
	
	for name, info := range funcs {
		buf = append(buf, []byte(name)...)
		buf = append(buf, 0)
		buf = append(buf, encodeUint32(uint32(info.Offset))...)
	}
	
	return buf
}

func encodeClasses(classes ClassMap) []byte {
	var buf []byte
	
	buf = append(buf, encodeUint32(uint32(len(classes)))...)
	
	for name := range classes {
		buf = append(buf, []byte(name)...)
		buf = append(buf, 0)
		buf = append(buf, []byte{0, 0, 0, 0}...) // offset placeholder
	}
	
	return buf
}

func encodeCode(instructions []Instruction) []byte {
	var buf []byte
	
	buf = append(buf, encodeUint32(uint32(len(instructions)))...)
	
	for _, instr := range instructions {
		buf = append(buf, byte(instr.Op))
		buf = append(buf, encodeUint32(uint32(instr.Arg))...)
		buf = append(buf, encodeUint32(uint32(instr.Arg2))...)
		if instr.Str != "" {
			buf = append(buf, []byte(instr.Str)...)
			buf = append(buf, 0)
		}
	}
	
	return buf
}

func encodeUint32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}
