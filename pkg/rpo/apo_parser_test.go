package rpo

import (
	"bytes"
	"testing"
)

func TestApoParser_ParseFuncHeader(t *testing.T) {
	data := []byte{
		0x01, 0x00, 0x00, 0x00,
		0x07, 0x00, 0x00, 0x00,
		'M', 'y', 'F', 'u', 'n', 'c', 0x00,
	}

	parser := NewApoParser(data)
	records, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("ParseAll failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Type != ApoFuncHeader {
		t.Errorf("Expected FUNC_HEADER, got %v", records[0].Type)
	}

	parsed := records[0].Parsed.(map[string]interface{})
	if parsed["name"] != "MyFunc" {
		t.Errorf("Expected 'MyFunc', got %v", parsed["name"])
	}
}

func TestApoParser_MultipleRecords(t *testing.T) {
	var data []byte
	
	// Record 1: FUNC_HEADER
	data = append(data, []byte{
		0x01, 0x00, 0x00, 0x00,
		0x05, 0x00, 0x00, 0x00,
		'T', 'e', 's', 't', 0x00,
	}...)
	
	// Record 2: STRING_TABLE (0xFEFFFFFF)
	data = append(data, []byte{
		0xFE, 0xFF, 0xFF, 0xFF,
		0x07, 0x00, 0x00, 0x00,
		'h', 'i', 0x00, 't', 'h', 'e', 'r', 0x00,
	}...)

	parser := NewApoParser(data)
	records, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("ParseAll failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}

	if records[0].Type != ApoFuncHeader {
		t.Errorf("Expected first record to be FUNC_HEADER")
	}

	if records[1].Type != ApoStringTable {
		t.Errorf("Expected second record to be STRING_TABLE, got %v", records[1].Type)
	}
}

func TestApoParser_FunctionLookup(t *testing.T) {
	data := []byte{
		0x01, 0x00, 0x00, 0x00,
		0x08, 0x00, 0x00, 0x00,
		'A', 'd', 'd', 'R', 'o', 'u', 't', 'e', 0x00,
		0x01, 0x00, 0x00, 0x00,
	}

	_, err := NewApoParser(data).ParseAll()
	if err != nil {
		t.Fatalf("ParseAll failed: %v", err)
	}

	parser := NewApoParser(data)
	parser.ParseAll()

	funcRecord := parser.GetFunction("AddRoute")
	if funcRecord == nil {
		t.Fatal("Expected to find function 'AddRoute'")
	}
}

func TestApoParser_StringTable(t *testing.T) {
	data := []byte{
		0xFE, 0xFF, 0xFF, 0xFF,
		0x0B, 0x00, 0x00, 0x00,
		'h', 'e', 'l', 'l', 'o', 0x00, 'w', 'o', 'r', 'l', 'd', 0x00,
	}

	parser := NewApoParser(data)
	records, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("ParseAll failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	parsed := records[0].Parsed.(map[string]interface{})
	strings, ok := parsed["strings"].([]string)
	if !ok {
		t.Fatalf("Expected strings array, got %T", parsed["strings"])
	}

	if len(strings) != 2 {
		t.Errorf("Expected 2 strings, got %d", len(strings))
	}

	if strings[0] != "hello" || strings[1] != "world" {
		t.Errorf("Expected ['hello', 'world'], got %v", strings)
	}
}

func TestApoParser_WriteTo(t *testing.T) {
	original := []byte{
		0x01, 0x00, 0x00, 0x00,
		0x05, 0x00, 0x00, 0x00,
		'T', 'e', 's', 't', 0x00,
	}

	parser := NewApoParser(original)
	parser.ParseAll()

	var buf bytes.Buffer
	n, err := parser.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if int(n) != len(original) {
		t.Errorf("Expected written %d bytes, got %d", len(original), n)
	}

	if !bytes.Equal(buf.Bytes(), original) {
		t.Error("Written data does not match original")
	}
}

func TestApoParser_Summary(t *testing.T) {
	var data []byte
	
	for i := 0; i < 3; i++ {
		name := []byte{byte('F' + i), 'u', 'n', 'c', 0x00}
		data = append(data, []byte{
			0x01, 0x00, 0x00, 0x00,
			byte(len(name)), 0x00, 0x00, 0x00,
		}...)
		data = append(data, name...)
	}

	parser := NewApoParser(data)
	parser.ParseAll()

	summary := parser.Summary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}

func TestApoRecordType_String(t *testing.T) {
	tests := []struct {
		typ      ApoRecordType
		expected string
	}{
		{ApoFuncHeader, "FUNC_HEADER"},
		{ApoFuncBody, "FUNC_BODY"},
		{ApoStringTable, "STRING_TABLE"},
		{ApoEndMarker, "END_MARKER"},
		{ApoRecordType(0xFFFFFFFF), "UNKNOWN_0xFFFFFFFF"},
	}

	for _, tt := range tests {
		if tt.typ.String() != tt.expected {
			t.Errorf("ApoRecordType(%d).String() = %s, want %s", tt.typ, tt.typ.String(), tt.expected)
		}
	}
}

func TestApoParser_ParseFuncBody(t *testing.T) {
	data := []byte{
		0x02, 0x00, 0x00, 0x00,
		0x08, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE,
	}

	parser := NewApoParser(data)
	records, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("ParseAll failed: %v", err)
	}

	parsed := records[0].Parsed.(map[string]interface{})
	if parsed["funcRef"].(uint32) != 1 {
		t.Errorf("Expected funcRef 1, got %v", parsed["funcRef"])
	}
}

func BenchmarkApoParser_Parse(b *testing.B) {
	var data []byte
	for i := 0; i < 100; i++ {
		name := []byte{byte('F' + i%26), 'u', 'n', 'c', '_', byte(i), 0x00}
		data = append(data, []byte{
			0x01, 0x00, 0x00, 0x00,
			byte(len(name)), 0x00, 0x00, 0x00,
		}...)
		data = append(data, name...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewApoParser(data)
		parser.ParseAll()
	}
}
