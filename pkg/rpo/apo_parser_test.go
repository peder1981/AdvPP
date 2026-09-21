package rpo

import (
	"os"
	"testing"
)

func TestAPOParserBasic(t *testing.T) {
	// Test with custom.rpo if available
	data, err := os.ReadFile("testdata/custom-updated.rpo")
	if err != nil {
		t.Skip("Test data not found")
	}
	
	parser := NewAPOParser(data[12:len(data)-36]) // Content only
	
	// Test candidate scanning
	candidates := parser.ScanForCandidates()
	t.Logf("Found %d candidates", len(candidates))
	
	// Test top candidates
	top := parser.GetTopCandidates(10)
	for i, cand := range top {
		t.Logf("[%d] Offset: %d, Size: %d, Type: 0x%02X, Confidence: %.2f",
			i, cand.Offset, cand.Size, cand.Type, cand.Confidence)
	}
}

func TestGetTypeName(t *testing.T) {
	tests := []struct {
		typeId uint8
		want   string
	}{
		{APO_TYPE_FUNCTION, "Function"},
		{APO_TYPE_METHOD, "Method"},
		{APO_TYPE_CLASS, "Class"},
		{0xFF, "Unknown_0xFF"},
	}
	
	for _, tt := range tests {
		got := GetTypeName(tt.typeId)
		if got != tt.want {
			t.Errorf("GetTypeName(%d) = %v, want %v", tt.typeId, got, tt.want)
		}
	}
}

func TestIsValidAdvplName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"GetSx3Cache", true},
		{"U_MyFunction", true},
		{"Invalid Name", false},
		{"123Start", false},
		{"", false},
		{"A", true},
		{"ThisIsAVeryLongFunctionNameThatExceedsLimit", false},
	}
	
	for _, tt := range tests {
		got := isValidAdvplName(tt.name)
		if got != tt.valid {
			t.Errorf("isValidAdvplName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}
