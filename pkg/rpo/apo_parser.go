package rpo

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// ApoRecordType represents the type of an APO record
type ApoRecordType uint32

const (
	ApoFuncHeader    ApoRecordType = 0x00000001 // Function header
	ApoFuncBody      ApoRecordType = 0x00000002 // Function body/code
	ApoDebugInfo     ApoRecordType = 0x00000003 // Debug information
	ApoMetadata      ApoRecordType = 0x00000004 // Metadata
	ApoParameter     ApoRecordType = 0x00000005 // Function parameters
	ApoLocalVar      ApoRecordType = 0x00000006 // Local variables
	ApoReturnValue   ApoRecordType = 0x00000007 // Return value info
	ApoLineNumber    ApoRecordType = 0x00000008 // Line number mapping
	ApoClassDef      ApoRecordType = 0x00000009 // Class definition
	ApoMethodDef     ApoRecordType = 0x0000000A // Method definition
	ApoPropertyDef   ApoRecordType = 0x0000000B // Property definition
	ApoEventDef      ApoRecordType = 0x0000000C // Event definition
	ApoInterfaceDef  ApoRecordType = 0x0000000D // Interface definition
	ApoConstantDef   ApoRecordType = 0x0000000E // Constant definition
	ApoImportDef     ApoRecordType = 0x0000000F // Import definition
	ApoNamespaceDef  ApoRecordType = 0x00000010 // Namespace definition
	ApoStringTable   ApoRecordType = 0xFFFFFFFE // String table
	ApoEndMarker     ApoRecordType = 0x7FFFFFFF // End marker
)

// String returns the name of the record type
func (t ApoRecordType) String() string {
	switch t {
	case ApoFuncHeader:
		return "FUNC_HEADER"
	case ApoFuncBody:
		return "FUNC_BODY"
	case ApoDebugInfo:
		return "DEBUG_INFO"
	case ApoMetadata:
		return "METADATA"
	case ApoParameter:
		return "PARAMETER"
	case ApoLocalVar:
		return "LOCAL_VAR"
	case ApoReturnValue:
		return "RETURN_VALUE"
	case ApoLineNumber:
		return "LINE_NUMBER"
	case ApoClassDef:
		return "CLASS_DEF"
	case ApoMethodDef:
		return "METHOD_DEF"
	case ApoPropertyDef:
		return "PROPERTY_DEF"
	case ApoEventDef:
		return "EVENT_DEF"
	case ApoInterfaceDef:
		return "INTERFACE_DEF"
	case ApoConstantDef:
		return "CONSTANT_DEF"
	case ApoImportDef:
		return "IMPORT_DEF"
	case ApoNamespaceDef:
		return "NAMESPACE_DEF"
	case ApoStringTable:
		return "STRING_TABLE"
	case ApoEndMarker:
		return "END_MARKER"
	default:
		return fmt.Sprintf("UNKNOWN_0x%08X", uint32(t))
	}
}

// ApoRecord represents a parsed APO record
type ApoRecord struct {
	Type     ApoRecordType `json:"type"`
	Length   uint32        `json:"length"`
	Data     []byte        `json:"-"`
	Offset   uint32        `json:"offset"`
	Parsed   interface{}   `json:"parsed,omitempty"`
	Error    error         `json:"error,omitempty"`
}

// ApoParser parses APO records from decrypted RPO body
type ApoParser struct {
	Data       []byte
	Offset     uint32
	Records    []*ApoRecord
	Strings    map[uint32]string
	Functions  map[string]*ApoRecord
	Classes    map[string]*ApoRecord
}

// NewApoParser creates a new APO parser
func NewApoParser(data []byte) *ApoParser {
	return &ApoParser{
		Data:      data,
		Offset:    0,
		Records:   make([]*ApoRecord, 0),
		Strings:   make(map[uint32]string),
		Functions: make(map[string]*ApoRecord),
		Classes:   make(map[string]*ApoRecord),
	}
}

// ParseAll parses all APO records from the data
func (p *ApoParser) ParseAll() ([]*ApoRecord, error) {
	p.Records = make([]*ApoRecord, 0)
	
	for p.Offset < uint32(len(p.Data)) {
		record, err := p.ParseNext()
		if err != nil {
			return p.Records, err
		}
		if record == nil {
			break
		}
		p.Records = append(p.Records, record)
	}
	
	return p.Records, nil
}

// ParseNext parses the next APO record
func (p *ApoParser) ParseNext() (*ApoRecord, error) {
	// Check if we have enough data for header
	if p.Offset+8 > uint32(len(p.Data)) {
		return nil, nil // End of data
	}
	
	// Read header
	recordType := binary.LittleEndian.Uint32(p.Data[p.Offset : p.Offset+4])
	recordLen := binary.LittleEndian.Uint32(p.Data[p.Offset+4 : p.Offset+8])
	
	p.Offset += 8
	
	// Validate length
	if recordLen > uint32(len(p.Data)-int(p.Offset)) {
		return nil, fmt.Errorf("record length %d exceeds remaining data", recordLen)
	}
	
	// Read data
	data := make([]byte, recordLen)
	copy(data, p.Data[p.Offset:p.Offset+recordLen])
	p.Offset += recordLen
	
	// Create record
	record := &ApoRecord{
		Type:   ApoRecordType(recordType),
		Length: recordLen,
		Data:   data,
		Offset: p.Offset - recordLen - 8,
	}
	
	// Parse based on type
	parsed, err := p.parseRecord(record)
	if err != nil {
		record.Error = err
	} else {
		record.Parsed = parsed
	}
	
	return record, nil
}

// parseRecord parses record-specific data
func (p *ApoParser) parseRecord(record *ApoRecord) (interface{}, error) {
	switch record.Type {
	case ApoFuncHeader:
		return p.parseFuncHeader(record)
	case ApoFuncBody:
		return p.parseFuncBody(record)
	case ApoStringTable:
		return p.parseStringTable(record)
	case ApoClassDef:
		return p.parseClassDef(record)
	case ApoMetadata:
		return p.parseMetadata(record)
	default:
		return p.parseGeneric(record)
	}
}

// parseFuncHeader parses function header record
func (p *ApoParser) parseFuncHeader(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data := record.Data
	
	if len(data) < 4 {
		return result, fmt.Errorf("func header too short")
	}
	
	// Parse function name (null-terminated string)
	nameEnd := bytesIndex(data, 0)
	if nameEnd < 0 {
		nameEnd = len(data)
	}
	result["name"] = string(data[:nameEnd])
	
	// Store in functions map
	p.Functions[string(data[:nameEnd])] = record
	
	// Parse remaining fields
	if len(data) >= 8 {
		result["flags"] = binary.LittleEndian.Uint32(data[4:8])
	}
	
	return result, nil
}

// parseFuncBody parses function body record
func (p *ApoParser) parseFuncBody(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data := record.Data
	
	// First 4 bytes might be function reference
	if len(data) >= 4 {
		result["funcRef"] = binary.LittleEndian.Uint32(data[0:4])
	}
	
	// Rest is machine code or bytecode
	result["code"] = data
	
	return result, nil
}

// parseStringTable parses string table record
func (p *ApoParser) parseStringTable(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data := record.Data
	
	// Parse null-terminated strings
	strings := make([]string, 0)
	offset := 0
	for offset < len(data) {
		// Find null terminator
		end := -1
		for i := offset; i < len(data); i++ {
			if data[i] == 0 {
				end = i - offset
				break
			}
		}
		if end < 0 {
			// No null terminator, take rest
			strings = append(strings, string(data[offset:]))
			break
		}
		strings = append(strings, string(data[offset:offset+end]))
		offset += end + 1
	}
	
	result["strings"] = strings
	return result, nil
}

// parseClassDef parses class definition record
func (p *ApoParser) parseClassDef(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data := record.Data
	
	if len(data) < 4 {
		return result, fmt.Errorf("class def too short")
	}
	
	// Parse class name
	nameEnd := bytesIndex(data, 0)
	if nameEnd < 0 {
		nameEnd = len(data)
	}
	result["name"] = string(data[:nameEnd])
	
	// Store in classes map
	p.Classes[string(data[:nameEnd])] = record
	
	return result, nil
}

// parseMetadata parses metadata record
func (p *ApoParser) parseMetadata(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	data := record.Data
	
	// Parse key-value pairs (simplified)
	if len(data) >= 4 {
		result["version"] = binary.LittleEndian.Uint32(data[0:4])
	}
	
	return result, nil
}

// parseGeneric parses unknown record types
func (p *ApoParser) parseGeneric(record *ApoRecord) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	result["raw_hex"] = fmt.Sprintf("%x", record.Data[:min(64, len(record.Data))])
	return result, nil
}

// bytesIndex finds the index of a byte in a slice
func bytesIndex(data []byte, b byte) int {
	for i, c := range data {
		if c == b {
			return i
		}
	}
	return -1
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetFunction returns a function record by name
func (p *ApoParser) GetFunction(name string) *ApoRecord {
	return p.Functions[name]
}

// GetClass returns a class record by name
func (p *ApoParser) GetClass(name string) *ApoRecord {
	return p.Classes[name]
}

// GetFunctions returns all function records
func (p *ApoParser) GetFunctions() map[string]*ApoRecord {
	return p.Functions
}

// GetClasses returns all class records
func (p *ApoParser) GetClasses() map[string]*ApoRecord {
	return p.Classes
}

// WriteTo writes parsed records to a writer
func (p *ApoParser) WriteTo(w io.Writer) (int64, error) {
	var total int64
	
	for _, record := range p.Records {
		// Write header
		header := make([]byte, 8)
		binary.LittleEndian.PutUint32(header[0:4], uint32(record.Type))
		binary.LittleEndian.PutUint32(header[4:8], record.Length)
		
		n, err := w.Write(header)
		total += int64(n)
		if err != nil {
			return total, err
		}
		
		// Write data
		n, err = w.Write(record.Data)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	
	return total, nil
}

// Summary returns a summary of parsed records
func (p *ApoParser) Summary() string {
	typeCounts := make(map[ApoRecordType]int)
	for _, record := range p.Records {
		typeCounts[record.Type]++
	}
	
	lines := make([]string, 0)
	lines = append(lines, "APO Parser Summary:")
	lines = append(lines, fmt.Sprintf("  Total records: %d", len(p.Records)))
	lines = append(lines, fmt.Sprintf("  Functions: %d", len(p.Functions)))
	lines = append(lines, fmt.Sprintf("  Classes: %d", len(p.Classes)))
	lines = append(lines, "")
	lines = append(lines, "Record Types:")
	
	for typ, count := range typeCounts {
		lines = append(lines, fmt.Sprintf("  %-20s: %d", typ.String(), count))
	}
	
	return strings.Join(lines, "\n")
}
