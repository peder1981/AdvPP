package rpo

import (
	"os"
	"strings"
	"encoding/binary"
	"testing"
)

func TestParseCustomRPO(t *testing.T) {
	data, err := os.ReadFile("/home/peder/Conciliador BLU/build/custom.rpo")
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if f.Name != "custom" {
		t.Errorf("Name = %q, want %q", f.Name, "custom")
	}
	if f.Sentinel != 0xFFFFFFFF {
		t.Errorf("Sentinel = 0x%08X, want 0xFFFFFFFF", f.Sentinel)
	}
	if f.FooterMagic != "APNSRM0419" {
		t.Errorf("FooterMagic = %q, want %q", f.FooterMagic, "APNSRM0419")
	}
	if len(f.AdminSection) == 0 {
		t.Error("AdminSection is empty")
	}
	if len(f.Body) == 0 {
		t.Error("Body is empty")
	}
}

func TestParseTTTM120(t *testing.T) {
	data, err := os.ReadFile("/home/peder/my-advpl-project/tttm120.rpo")
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if f.Sentinel != 0xFFFFFF00 {
		t.Errorf("Sentinel = 0x%08X, want 0xFFFFFF00", f.Sentinel)
	}
	if f.FooterMagic != "APNSRM0421" {
		t.Errorf("FooterMagic = %q, want %q", f.FooterMagic, "APNSRM0421")
	}
}

func TestParseTLPP(t *testing.T) {
	data, err := os.ReadFile("/home/peder/GhidraProjects/appserver/tlpp.rpo")
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if f.Sentinel != 0x0000FFFF {
		t.Errorf("Sentinel = 0x%08X, want 0x0000FFFF", f.Sentinel)
	}
	if f.FooterMagic != "APNSRM0420" {
		t.Errorf("FooterMagic = %q, want %q", f.FooterMagic, "APNSRM0420")
	}
}

func TestRoundTripCustom(t *testing.T) {
	data, err := os.ReadFile("/home/peder/Conciliador BLU/build/custom.rpo")
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	out := f.Bytes()
	if len(out) != len(data) {
		t.Errorf("Bytes() len = %d, want %d", len(out), len(data))
	}
	for i := range data {
		if out[i] != data[i] {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, out[i], data[i])
			break
		}
	}
}

func TestRoundTripTTTM120(t *testing.T) {
	data, err := os.ReadFile("/home/peder/my-advpl-project/tttm120.rpo")
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	out := f.Bytes()
	if len(out) != len(data) {
		t.Fatalf("Bytes() len = %d, want %d", len(out), len(data))
	}
	diff := 0
	for i := range data {
		if out[i] != data[i] {
			diff++
		}
	}
	if diff > 0 {
		t.Errorf("Bytes() differs in %d bytes", diff)
	}
}

func TestParseDifferentSentinels(t *testing.T) {
	// Verify all three known sentinels are accepted
	for _, sentinel := range KnownSentinels {
		// Build header with correct sentinel value
		header := make([]byte, HeaderSize)
		binary.LittleEndian.PutUint32(header[0:4], HeaderSize) // selfOffset
		copy(header[4:20], []byte("test"))
		// Write sentinel at the correct offset (byte 40 from start = offset 4+16+4+16)
		binary.LittleEndian.PutUint32(header[4+nameFieldSize+4:4+nameFieldSize+4+sentinelSize], sentinel)

		f := &File{
			Header:       header,
			Sentinel:     sentinel,
			AdminSection: []byte{0x01},
			Body:         []byte{0x02},
			FooterMagic:  "APNSRM0419",
			FooterTrail:  make([]byte, 24),
		}
		out := f.Bytes()
		parsed, err := Parse(out)
		if err != nil {
			t.Errorf("sentinel 0x%08X: Parse() error = %v", sentinel, err)
			continue
		}
		if parsed.Sentinel != sentinel {
			t.Errorf("sentinel 0x%08X: got 0x%08X", sentinel, parsed.Sentinel)
		}
	}
}

func TestParseUnknownSentinel(t *testing.T) {
	f := &File{
		Header:       make([]byte, HeaderSize),
		Sentinel:     0xDEADBEEF,
		AdminSection: []byte{0x01},
		Body:         []byte{0x02},
		FooterMagic:  "APNSRM0419",
		FooterTrail:  make([]byte, 24),
	}
	out := f.Bytes()
	_, err := Parse(out)
	if err == nil {
		t.Error("Parse() expected error for unknown sentinel, got nil")
	}
}

func TestRoundTripSynthetic(t *testing.T) {
	// Build a synthetic RPO with empty AdminSection
	header := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], HeaderSize) // selfOffset
	copy(header[4:20], []byte("test"))
	// Write sentinel at correct offset
	binary.LittleEndian.PutUint32(header[sentinelOffset:sentinelOffset+sentinelSize], 0xFFFFFFFF)

	f := &File{
		Header:       header,
		Name:         "test",
		SelfOffset:   HeaderSize,
		Sentinel:     0xFFFFFFFF,
		AdminSection: []byte{}, // empty
		Body:         []byte{0xDE, 0xAD, 0xBE, 0xEF},
		FooterMagic:  "APNSRM0419",
		FooterTrail:  make([]byte, 24),
	}

	out := f.Bytes()
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Sentinel != 0xFFFFFFFF {
		t.Errorf("Sentinel = 0x%08X, want 0xFFFFFFFF", parsed.Sentinel)
	}
	if string(parsed.AdminSection) != "" {
		t.Errorf("AdminSection = %q, want empty", parsed.AdminSection)
	}
	if string(parsed.Body) != "\xde\xad\xbe\xef" {
		t.Errorf("Body = %q, want deadbeef", parsed.Body)
	}
}
func TestNormalizeFuncName(t *testing.T) {
	cases := map[string]string{
		"U_U_RPOEXTRACT":           "U_RPOEXTRACT",
		"U_RPOEXTRACT":             "U_RPOEXTRACT",
		"CUSTOM.BLU.API.U_U_BLUX": "CUSTOM.BLU.API.U_BLUX",
		"U_BVBATCH":                "U_BVBATCH",
		"#NONE#ORTD041.TLPP#NONE#": "#NONE#ORTD041.TLPP#NONE#",
	}
	for in, want := range cases {
		if got := normalizeFuncName(in); got != want {
			t.Errorf("normalizeFuncName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractResult(t *testing.T) {
	result := &ExtractResult{
		Count:     3,
		Functions: []string{"u_test1", "u_test2", "u_test3"},
		Apos: []ApoInfo{
			{Index: 0, Name: "u_test1", FuncName: "test1"},
			{Index: 1, Name: "u_test2", FuncName: "test2"},
			{Index: 2, Name: "u_test3", FuncName: "test3"},
		},
	}
	if result.FunctionCount() != 3 {
		t.Errorf("FunctionCount() = %d, want 3", result.FunctionCount())
	}
	if name, ok := result.GetFunction(1); !ok || name != "u_test2" {
		t.Errorf("GetFunction(1) = %q, want u_test2", name)
	}
	if result.HasFunction("u_test2") {
		// expected
	} else {
		t.Error("HasFunction(u_test2) = false, want true")
	}
	if result.HasFunction("nonexistent") {
		t.Error("HasFunction(nonexistent) = true, want false")
	}
	if _, ok := result.GetFunction(99); ok {
		t.Error("GetFunction(99) returned ok=true, want false")
	}
}

func TestParseExtractJSON(t *testing.T) {
	j := `{ "count": 2, "functions": ["f1", "f2"], "all_apos": [{"index":0,"name":"f1"}] }`
	tmp, err := os.CreateTemp("", "rpo_extract_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(j); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	result, err := ParseExtractJSON(tmp.Name())
	if err != nil {
		t.Fatalf("ParseExtractJSON() error = %v", err)
	}
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.Functions) != 2 {
		t.Errorf("Functions len = %d, want 2", len(result.Functions))
	}
	if len(result.Apos) != 1 {
		t.Errorf("Apos len = %d, want 1", len(result.Apos))
	}
}

func TestIdentifyRealRPOs(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantType     RPOType
		wantMagic    string
		wantSentinel uint32
	}{
		{"custom BLU", "/home/peder/Conciliador BLU/build/custom.rpo", RPOCustom, "APNSRM0419", 0xFFFFFFFF},
		{"custom alt", "/home/peder/my-advpl-project/custom.rpo", RPOCustom, "APNSRM0419", 0xFFFFFFFF},
		{"tttm120", "/home/peder/my-advpl-project/tttm120.rpo", RPOTttm120, "APNSRM0421", 0xFFFFFF00},
		{"tlpp", "/home/peder/GhidraProjects/appserver/tlpp.rpo", RPOTlpp, "APNSRM0420", 0x0000FFFF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.path)
			if err != nil {
				t.Skipf("skip: %v", err)
			}
			profile, err := Identify(data)
			if err != nil {
				t.Fatalf("Identify() error = %v", err)
			}
			if profile.Type != tt.wantType {
				t.Errorf("Type = %v (%s), want %v", profile.Type, profile.Type, tt.wantType)
			}
			if profile.Magic != tt.wantMagic {
				t.Errorf("Magic = %q, want %q", profile.Magic, tt.wantMagic)
			}
			if profile.Sentinel != tt.wantSentinel {
				t.Errorf("Sentinel = 0x%08X, want 0x%08X", profile.Sentinel, tt.wantSentinel)
			}
		})
	}
}

func TestSuggestExtractCommand(t *testing.T) {
	tests := []struct {
		name    string
		ptype   RPOType
		rpo     string
		wantSub string
	}{
		{"custom", RPOCustom, "/tmp/custom.rpo", "extract_rpo.py"},
		{"tttm120", RPOTttm120, "/tmp/tttm120.rpo", "timeout 120"},
		{"tlpp", RPOTlpp, "/tmp/tlpp.rpo", "extract_rpo.py"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &RPOProfile{Type: tt.ptype}
			got := p.SuggestExtractCommand(tt.rpo)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("SuggestExtractCommand() = %q, want to contain %q", got, tt.wantSub)
			}
		})
	}
}
