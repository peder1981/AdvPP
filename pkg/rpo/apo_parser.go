package rpo

import (
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
)

// APO Record Types - Based on analysis of Protheus RPO format
const (
	APO_TYPE_FUNCTION   = 0x01 // User Function
	APO_TYPE_METHOD     = 0x02 // Class Method
	APO_TYPE_CLASS      = 0x03 // Class definition
	APO_TYPE_PROPERTY   = 0x04 // Class property
	APO_TYPE_VARIABLE   = 0x05 // Local/Global variable
	APO_TYPE_CONSTANT   = 0x06 // Constant definition
	APO_TYPE_INCLUDE    = 0x07 // Include directive
	APO_TYPE_LIBRARY    = 0x08 // Library reference
	APO_TYPE_NAMESPACE  = 0x09 // Namespace definition
	APO_TYPE_EVENT      = 0x0A // Event handler
	APO_TYPE_TRIGGER    = 0x0B // Table trigger
	APO_TYPE_INDEX      = 0x0C // Database index
	APO_TYPE_RELATION   = 0x0D // Table relation
	APO_TYPE_VALIDATION = 0x0E // Field validation
	APO_TYPE_UI         = 0x0F // UI element
	APO_TYPE_REPORT     = 0x10 // Report definition
	APO_TYPE_MENU       = 0x11 // Menu definition
	APO_TYPE_PROCESS    = 0x12 // Business process
	APO_TYPE_SERVICE    = 0x13 // Web service
	APO_TYPE_JOB        = 0x14 // Background job
)

// APORecord represents a single APO record in the RPO
type APORecord struct {
	Offset     int               `json:"offset"`
	Size       int               `json:"size"`
	Type       uint8             `json:"type"`
	TypeName   string            `json:"type_name"`
	Name       string            `json:"name"`
	Data       []byte            `json:"-"`
	Properties map[string]string `json:"properties,omitempty"`
}

// APOParser extracts APO records from RPO content
type APOParser struct {
	data       []byte
	records    []*APORecord
	candidates []APOCandidate
}

// APOCandidate represents a potential APO record header
type APOCandidate struct {
	Offset     int
	Size       uint32
	Type       uint8
	Confidence float64
}

// NewAPOParser creates a new APO parser for the given data
func NewAPOParser(data []byte) *APOParser {
	return &APOParser{
		data:     data,
		records:  make([]*APORecord, 0),
		candidates: make([]APOCandidate, 0),
	}
}

// GetTypeName returns the human-readable name for an APO type
func GetTypeName(typeId uint8) string {
	switch typeId {
	case APO_TYPE_FUNCTION:
		return "Function"
	case APO_TYPE_METHOD:
		return "Method"
	case APO_TYPE_CLASS:
		return "Class"
	case APO_TYPE_PROPERTY:
		return "Property"
	case APO_TYPE_VARIABLE:
		return "Variable"
	case APO_TYPE_CONSTANT:
		return "Constant"
	case APO_TYPE_INCLUDE:
		return "Include"
	case APO_TYPE_LIBRARY:
		return "Library"
	case APO_TYPE_NAMESPACE:
		return "Namespace"
	case APO_TYPE_EVENT:
		return "Event"
	case APO_TYPE_TRIGGER:
		return "Trigger"
	case APO_TYPE_INDEX:
		return "Index"
	case APO_TYPE_RELATION:
		return "Relation"
	case APO_TYPE_VALIDATION:
		return "Validation"
	case APO_TYPE_UI:
		return "UI"
	case APO_TYPE_REPORT:
		return "Report"
	case APO_TYPE_MENU:
		return "Menu"
	case APO_TYPE_PROCESS:
		return "Process"
	case APO_TYPE_SERVICE:
		return "Service"
	case APO_TYPE_JOB:
		return "Job"
	default:
		return fmt.Sprintf("Unknown_0x%02X", typeId)
	}
}

// ScanForCandidates scans the content for potential APO record headers
func (p *APOParser) ScanForCandidates() []APOCandidate {
	p.candidates = make([]APOCandidate, 0)
	
	// Scan in 4-byte alignment
	for i := 0; i < len(p.data)-8; i += 4 {
		// Read size (4 bytes, little-endian)
		size := binary.LittleEndian.Uint32(p.data[i : i+4])
		
		// Read type (1 byte)
		if i+4 >= len(p.data) {
			continue
		}
		typeId := p.data[i+4]
		
		// Validate size (reasonable range for APO records)
		if size < 32 || size > 10*1024*1024 {
			continue
		}
		
		// Validate type (known range)
		if typeId > 0x20 {
			continue
		}
		
		// Calculate confidence based on patterns
		confidence := p.calculateConfidence(i, size, typeId)
		
		p.candidates = append(p.candidates, APOCandidate{
			Offset: i,
			Size:   size,
			Type:   typeId,
			Confidence: confidence,
		})
	}
	
	return p.candidates
}

// calculateConfidence calculates how likely a candidate is to be a real APO record
func (p *APOParser) calculateConfidence(offset int, size uint32, typeId uint8) float64 {
	confidence := 0.5 // Base confidence
	
	// Check if followed by printable text (function names)
	if offset+8 < len(p.data) {
		end := offset + 8 + 15
		if end > len(p.data) {
			end = len(p.data)
		}
		nextBytes := p.data[offset+5:end]
		printable := 0
		for _, b := range nextBytes {
			if (b >= 0x20 && b <= 0x7E) || b == 0x00 {
				printable++
			}
		}
		if len(nextBytes) > 0 && printable > len(nextBytes)*7/10 {
			confidence += 0.2
		}
	}
	
	// Check alignment
	if offset%16 == 0 {
		confidence += 0.1
	}
	
	// Type-specific adjustments
	switch typeId {
	case APO_TYPE_FUNCTION, APO_TYPE_METHOD:
		confidence += 0.1 // Common types
	case APO_TYPE_CLASS:
		confidence += 0.15 // Important type
	}
	
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return confidence
}

// ParseRecords extracts APO records from candidates
func (p *APOParser) ParseRecords(minConfidence float64) []*APORecord {
	candidates := p.ScanForCandidates()
	
	p.records = make([]*APORecord, 0)
	for _, cand := range candidates {
		if cand.Confidence < minConfidence {
			continue
		}
		
		// Extract record
		record := p.extractRecord(cand)
		if record != nil {
			p.records = append(p.records, record)
		}
	}
	
	return p.records
}

// extractRecord extracts an APO record from a candidate
func (p *APOParser) extractRecord(cand APOCandidate) *APORecord {
	end := cand.Offset + int(cand.Size)
	if end > len(p.data) {
		return nil
	}
	
	data := p.data[cand.Offset:end]
	
	record := &APORecord{
		Offset: cand.Offset,
		Size: int(cand.Size),
		Type: cand.Type,
		TypeName: GetTypeName(cand.Type),
		Data: data,
		Properties: make(map[string]string),
	}
	
	// Extract name (usually first printable string after header)
	name := p.extractName(data)
	record.Name = name
	if name != "" {
		record.Properties["name"] = name
	}
	
	// Extract additional properties based on type
	p.extractProperties(record, data)
	
	return record
}

// extractName extracts the function/method name from APO data
func (p *APOParser) extractName(data []byte) string {
	// Skip the 5-byte header (4 size + 1 type)
	offset := 5
	if offset >= len(data) {
		return ""
	}
	
	// Find first null-terminated string
	nameEnd := -1
	for i := offset; i < len(data)-1; i++ {
		if data[i] == 0x00 && i > offset {
			nameEnd = i
			break
		}
	}
	
	if nameEnd > 0 {
		nameBytes := data[offset:nameEnd]
		// Validate it looks like a valid identifier
		if isValidAdvplName(string(nameBytes)) {
			return string(nameBytes)
		}
	}
	
	return ""
}

// isValidAdvplName checks if a string is a valid AdvPL identifier
func isValidAdvplName(name string) bool {
	if len(name) == 0 || len(name) > 30 {
		return false
	}
	
	// Must start with letter or underscore
	first := name[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || first == '_') {
		return false
	}
	
	// Allow alphanumeric and underscore
	nameRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	return nameRegex.MatchString(name)
}

// extractProperties extracts type-specific properties from APO data
func (p *APOParser) extractProperties(record *APORecord, data []byte) {
	// Common properties extraction
	switch record.Type {
	case APO_TYPE_FUNCTION, APO_TYPE_METHOD:
		p.extractFunctionProperties(record, data)
	case APO_TYPE_CLASS:
		p.extractClassProperties(record, data)
	case APO_TYPE_VARIABLE:
		p.extractVariableProperties(record, data)
	}
}

// extractFunctionProperties extracts properties from function APO records
func (p *APOParser) extractFunctionProperties(record *APORecord, data []byte) {
	// Look for parameter count
	if len(data) > 10 {
		paramCount := int(data[5])
		if paramCount >= 0 && paramCount < 50 {
			record.Properties["parameters"] = fmt.Sprintf("%d", paramCount)
		}
	}
	
	// Look for return type indicator
	if len(data) > 12 {
		record.Properties["has_return"] = fmt.Sprintf("%v", data[11] != 0)
	}
}

// extractClassProperties extracts properties from class APO records
func (p *APOParser) extractClassProperties(record *APORecord, data []byte) {
	// Look for parent class reference
	if len(data) > 8 {
		record.Properties["has_parent"] = fmt.Sprintf("%v", data[5] != 0)
	}
}

// extractVariableProperties extracts properties from variable APO records
func (p *APOParser) extractVariableProperties(record *APORecord, data []byte) {
	// Look for variable type
	if len(data) > 8 {
		varType := data[5]
		typeNames := map[uint8]string{
			0x00: "auto",
			0x01: "local",
			0x02: "static",
			0x03: "global",
			0x04: "parameter",
		}
		if name, ok := typeNames[varType]; ok {
			record.Properties["var_type"] = name
		}
	}
}

// ExtractFunctions extracts all function/method names from RPO content
func (p *APOParser) ExtractFunctions() []string {
	records := p.ParseRecords(0.6)
	
	functions := make([]string, 0)
	for _, record := range records {
		if record.Type == APO_TYPE_FUNCTION || record.Type == APO_TYPE_METHOD {
			if record.Name != "" {
				functions = append(functions, record.Name)
			}
		}
	}
	
	return functions
}

// ExtractAll extracts all APO information from RPO content
func (p *APOParser) ExtractAll() map[string]interface{} {
	records := p.ParseRecords(0.5)
	
	result := map[string]interface{}{
		"total_records": len(records),
		"by_type":      make(map[string]int),
		"records":      make([]map[string]interface{}, 0),
	}
	
	// Count by type
	for _, record := range records {
		result["by_type"].(map[string]int)[record.TypeName]++
	}
	
	// Add records
	for _, record := range records {
		recordMap := map[string]interface{}{
			"offset": record.Offset,
			"size": record.Size,
			"type": record.TypeName,
			"name": record.Name,
		}
		if len(record.Properties) > 0 {
			recordMap["properties"] = record.Properties
		}
		result["records"] = append(result["records"].([]map[string]interface{}), recordMap)
	}
	
	return result
}

// DumpToWriter dumps APO information to a writer
func (p *APOParser) DumpToWriter(w io.Writer) {
	records := p.ParseRecords(0.5)
	
	fmt.Fprintf(w, "=== APO Records Analysis ===\n")
	fmt.Fprintf(w, "Total records found: %d\n\n", len(records))
	
	// Summary by type
	typeCounts := make(map[string]int)
	for _, r := range records {
		typeCounts[r.TypeName]++
	}
	
	fmt.Fprintf(w, "By type:\n")
	for typeName, count := range typeCounts {
		fmt.Fprintf(w, "  %-20s: %d\n", typeName, count)
	}
	fmt.Fprintf(w, "\n")
	
	// List records
	fmt.Fprintf(w, "Records:\n")
	for i, record := range records {
		fmt.Fprintf(w, "  [%d] Offset: %d, Size: %d, Type: %s", i+1, record.Offset, record.Size, record.TypeName)
		if record.Name != "" {
			fmt.Fprintf(w, ", Name: %s", record.Name)
		}
		fmt.Fprintf(w, "\n")
		
		if len(record.Properties) > 0 {
			for k, v := range record.Properties {
				fmt.Fprintf(w, "      %s: %s\n", k, v)
			}
		}
	}
}

// FindCandidatesAboveThreshold finds candidates with confidence above threshold
func (p *APOParser) FindCandidatesAboveThreshold(threshold float64) []APOCandidate {
	candidates := p.ScanForCandidates()
	
	filtered := make([]APOCandidate, 0)
	for _, cand := range candidates {
		if cand.Confidence >= threshold {
			filtered = append(filtered, cand)
		}
	}
	
	return filtered
}

// GetTopCandidates returns the top N candidates by confidence
func (p *APOParser) GetTopCandidates(n int) []APOCandidate {
	candidates := p.ScanForCandidates()
	
	// Sort by confidence (bubble sort for simplicity)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Confidence > candidates[i].Confidence {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	
	if n > len(candidates) {
		n = len(candidates)
	}
	
	result := make([]APOCandidate, n)
	copy(result, candidates[:n])
	
	return result
}
