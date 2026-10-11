package injector

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// AdminIndex representa o índice APO na seção admin do RPO.
// Formato confirmado via Ghidra (tApoFileBase::WriteIndex):
//
//	[uint32: count]
//	para cada entrada:
//	  [uint32: name_len]
//	  [name_len bytes: nome]
//	  [\0]
//	  [double: timestamp] OU [0xFFFFFFFF] (4 bytes placeholder)
//	  [int32: field_0x4c] OU [0xFFFFFFFF]
//	  [int32: field_0x50] OU [0xFFFFFFFF]
//	  [int32: field_0x44] OU [0xFFFFFFFF]
//	  [int32: field_0x48] OU [0xFFFFFFFF]
//	  [int32: field_0x20] OU [0xFFFFFFFF]
//	  [int32: field_0x24] OU [0xFFFFFFFF]
//	  [int32: field_0x40] OU [0xFFFFFFFF]
//	  [int32: field_0x60] OU [0xFFFFFFFF]
//	  [uint8: field_0x81]
//
// Tamanho mínimo por entrada: 53 + len(name) bytes
// Tamanho máximo por entrada: 57 + len(name) bytes
type AdminIndex struct {
	Entries []*AdminIndexEntry
}

// AdminIndexEntry é uma entrada no índice admin.
type AdminIndexEntry struct {
	Name      string
	Timestamp float64
	Field4C   int32
	Field50   int32
	Field44   int32
	Field48   int32
	Field20   int32
	Field24   int32
	Field40   int32
	Field60   int32
	Field81   byte
	HasFull   bool // true se tiver timestamp completo (8B), false se placeholder (4B)
}

// ParseAdminIndex decodifica o índice APO a partir dos bytes da admin section.
// Retorna os entries e o número de bytes consumidos.
func ParseAdminIndex(data []byte) (*AdminIndex, int, error) {
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("admin section muito pequeno (%d bytes)", len(data))
	}

	idx := &AdminIndex{}
	offset := 0

	// Ler count
	if offset+4 > len(data) {
		return nil, 0, fmt.Errorf("count truncado")
	}
	count := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Se count for muito grande, pode ser que o formato seja diferente
	// Tentar parsear mesmo assim com limite razoável
	if count > 100000 {
		// Talvez o count não esteja no início — tentar encontrar um padrão válido
		count = 0
	}

	for i := uint32(0); i < count && offset < len(data); i++ {
		// Ler name_len
		if offset+4 > len(data) {
			break
		}
		nameLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		if nameLen == 0 || nameLen > 256 {
			break
		}

		// Ler nome + null
		if offset+int(nameLen)+1 > len(data) {
			return nil, 0, fmt.Errorf("nome truncado em offset %d", offset)
		}
		nameBytes := data[offset : offset+int(nameLen)]
		nullIdx := bytes.IndexByte(nameBytes, 0)
		if nullIdx >= 0 {
			nameBytes = nameBytes[:nullIdx]
		}
		name := string(nameBytes)
		offset += int(nameLen)

		if len(name) < 1 {
			break
		}

		// Tentar ler timestamp (8B double) ou placeholder (4B 0xFFFFFFFF)
		hasFull := false
		var timestamp float64
		var placeholder4 uint32

		if offset+8 <= len(data) {
			tsBits := binary.LittleEndian.Uint64(data[offset : offset+8])
			ts := uint64ToFloat(tsBits)
			if ts > 10000 && ts < 100000 {
				// Timestamp válido
				timestamp = ts
				hasFull = true
				offset += 8
			} else {
				// Provavelmente é 0xFFFFFFFF como primeiro 4 bytes de um placeholder
				placeholder4 = binary.LittleEndian.Uint32(data[offset : offset+4])
				if placeholder4 == 0xFFFFFFFF {
					offset += 4
				} else {
					// Não é placeholder — tentar como timestamp de novo
					timestamp = ts
					hasFull = true
					offset += 8
				}
			}
		}

		if !hasFull && placeholder4 != 0xFFFFFFFF {
			// Se não era placeholder, retornar erro
			hasFull = true
			timestamp = 0
		}

		// Ler 9 campos int32 (ou placeholders)
		fields := make([]int32, 9)
		
		for j := 0; j < 9; j++ {
			if offset+4 > len(data) {
				break
			}
			val := binary.LittleEndian.Uint32(data[offset : offset+4])
			offset += 4
			if val == 0xFFFFFFFF {
				fields[j] = -1
			} else {
				fields[j] = int32(val)
			}
		}

		// O último campo (0x81) é um byte, não int32
		// Mas no WriteIndex, ele é lido como parte do fluxo
		// Vamos ajustar: o campo 0x81 é apenas 1 byte
		// Na verdade, do assembly, parece que todos são escritos como 4B via WriteEPKvji
		// exceto o último que é WriteMSChar (1B)

		entry := &AdminIndexEntry{
			Name:      name,
			Timestamp: timestamp,
			Field4C:   fields[0],
			Field50:   fields[1],
			Field44:   fields[2],
			Field48:   fields[3],
			Field20:   fields[4],
			Field24:   fields[5],
			Field40:   fields[6],
			Field60:   fields[7],
			HasFull:   hasFull,
		}

		// O campo 0x81 é lido separadamente
		if j := 8; j < len(fields) {
			entry.Field81 = byte(fields[j])
		}

		idx.Entries = append(idx.Entries, entry)
	}

	return idx, offset, nil
}

// SerializeAdminIndex serializa o índice de volta para bytes.
func SerializeAdminIndex(idx *AdminIndex) []byte {
	var buf bytes.Buffer

	// Escrever count
	binary.Write(&buf, binary.LittleEndian, uint32(len(idx.Entries)))

	for _, e := range idx.Entries {
		nameBytes := []byte(e.Name)

		// Escrever name_len + name + null
		binary.Write(&buf, binary.LittleEndian, uint32(len(nameBytes)+1))
		buf.Write(nameBytes)
		buf.WriteByte(0)

		// Escrever timestamp ou placeholder
		if e.HasFull {
			binary.Write(&buf, binary.LittleEndian, floatToUint64(e.Timestamp))
		} else {
			binary.Write(&buf, binary.LittleEndian, uint32(0xFFFFFFFF))
		}

		// Escrever 9 campos int32
		binary.Write(&buf, binary.LittleEndian, e.Field4C)
		binary.Write(&buf, binary.LittleEndian, e.Field50)
		binary.Write(&buf, binary.LittleEndian, e.Field44)
		binary.Write(&buf, binary.LittleEndian, e.Field48)
		binary.Write(&buf, binary.LittleEndian, e.Field20)
		binary.Write(&buf, binary.LittleEndian, e.Field24)
		binary.Write(&buf, binary.LittleEndian, e.Field40)
		binary.Write(&buf, binary.LittleEndian, e.Field60)
		binary.Write(&buf, binary.LittleEndian, e.Field81)
	}

	return buf.Bytes()
}

// FindEntry encontra um entry pelo nome.
func (idx *AdminIndex) FindEntry(name string) *AdminIndexEntry {
	for _, e := range idx.Entries {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// AddEntry adiciona um novo entry ao índice.
func (idx *AdminIndex) AddEntry(name string, timestamp float64, codeSize int) {
	idx.Entries = append(idx.Entries, &AdminIndexEntry{
		Name:      name,
		Timestamp: timestamp,
		Field4C:   int32(codeSize),
		Field50:   0,
		Field44:   0,
		Field48:   0,
		Field20:   0,
		Field24:   0,
		Field40:   0,
		Field60:   0,
		Field81:   0,
		HasFull:   true,
	})
}

// RemoveEntry remove um entry pelo nome.
func (idx *AdminIndex) RemoveEntry(name string) bool {
	for i, e := range idx.Entries {
		if e.Name == name {
			idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
			return true
		}
	}
	return false
}
