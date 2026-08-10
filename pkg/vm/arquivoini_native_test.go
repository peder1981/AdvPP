package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Helper to create a temp INI file with content
func createTempINI(t *testing.T, dir string, name string, content string) string {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp INI: %v", err)
	}
	return path
}

// Helper to read INI file content
func readINI(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read INI: %v", err)
	}
	return string(data)
}

func TestDeleteKeyINI(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Test case 1: Delete existing key
	iniPath := createTempINI(t, dir, "test.ini", "[SECTION1]\nKEY1=VALUE1\nKEY2=VALUE2\n[SECTION2]\nKEY3=VALUE3\n")
	result, err := v.natives["DELETEKEYINI"].Fn([]advplrt.Value{
		advplrt.NewString("SECTION1"),
		advplrt.NewString("KEY1"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteKeyINI failed: %v", err)
	}
	if !result.(*advplrt.BoolValue).Val {
		t.Error("expected true, got false")
	}
	// Verify KEY1 is deleted but KEY2 remains
	content := readINI(t, iniPath)
	// Expected: section headers with trailing newline, and may have final blank line
	if !strings.Contains(content, "[SECTION1]") || !strings.Contains(content, "KEY2=VALUE2") {
		t.Errorf("missing expected content after delete: %q", content)
	}
	if strings.Contains(content, "KEY1=VALUE1") {
		t.Error("KEY1 should have been deleted")
	}
	if !strings.Contains(content, "[SECTION2]") || !strings.Contains(content, "KEY3=VALUE3") {
		t.Errorf("unexpected missing content after delete: %q", content)
	}

	// Test case 2: Try to delete non-existent key
	result, err = v.natives["DELETEKEYINI"].Fn([]advplrt.Value{
		advplrt.NewString("SECTION1"),
		advplrt.NewString("NONEXISTENT"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteKeyINI failed: %v", err)
	}
	if result.(*advplrt.BoolValue).Val {
		t.Error("expected false for non-existent key, got true")
	}
}

func TestDeleteSectionINI(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Test case 1: Delete existing section
	iniPath := createTempINI(t, dir, "test.ini", "[SECTION1]\nKEY1=VALUE1\n[SECTION2]\nKEY2=VALUE2\n")
	result, err := v.natives["DELETESECTIONINI"].Fn([]advplrt.Value{
		advplrt.NewString("SECTION1"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteSectionINI failed: %v", err)
	}
	if !result.(*advplrt.BoolValue).Val {
		t.Error("expected true, got false")
	}
	content := readINI(t, iniPath)
	// Verify SECTION2 remains and SECTION1 is deleted
	if !strings.Contains(content, "[SECTION2]") || !strings.Contains(content, "KEY2=VALUE2") {
		t.Errorf("unexpected content after delete: %q", content)
	}
	if strings.Contains(content, "[SECTION1]") {
		t.Error("SECTION1 should have been deleted")
	}

	// Test case 2: Try to delete non-existent section
	result, err = v.natives["DELETESECTIONINI"].Fn([]advplrt.Value{
		advplrt.NewString("NONEXISTENT"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteSectionINI failed: %v", err)
	}
	if result.(*advplrt.BoolValue).Val {
		t.Error("expected false for non-existent section, got true")
	}
}

func TestGetINISessions(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Test case 1: Get sections from valid INI
	iniPath := createTempINI(t, dir, "test.ini", "[SECTION1]\nKEY1=VALUE1\n[SECTION2]\nKEY2=VALUE2\n[SECTION3]\nKEY3=VALUE3\n")
	result, err := v.natives["GETINISESSIONS"].Fn([]advplrt.Value{
		advplrt.NewString(iniPath),
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetINISessions failed: %v", err)
	}
	arr := result.(*advplrt.ArrayValue)
	if len(arr.Elements) != 3 {
		t.Errorf("expected 3 sections, got %d", len(arr.Elements))
	}
	expected := []string{"SECTION1", "SECTION2", "SECTION3"}
	for i, exp := range expected {
		if i < len(arr.Elements) {
			val := arr.Elements[i].(*advplrt.StringValue).Val
			if val != exp {
				t.Errorf("section %d: expected %q, got %q", i, exp, val)
			}
		}
	}

	// Test case 2: Non-existent file returns empty array
	result, err = v.natives["GETINISESSIONS"].Fn([]advplrt.Value{
		advplrt.NewString(filepath.Join(dir, "nonexistent.ini")),
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetINISessions failed: %v", err)
	}
	arr = result.(*advplrt.ArrayValue)
	if len(arr.Elements) != 0 {
		t.Errorf("expected empty array for non-existent file, got %d elements", len(arr.Elements))
	}
}

func TestGetPvProfileInt(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Test case 1: Read existing numeric value
	iniPath := createTempINI(t, dir, "test.ini", "[TCP]\nPORT=8080\n[DATABASE]\nCONNECTIONS=100\n")
	result, err := v.natives["GETPVPROFILEINT"].Fn([]advplrt.Value{
		advplrt.NewString("TCP"),
		advplrt.NewString("PORT"),
		advplrt.NewNumber(0),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfileInt failed: %v", err)
	}
	val := result.(*advplrt.NumberValue).Val
	if val != 8080 {
		t.Errorf("expected 8080, got %f", val)
	}

	// Test case 2: Return default for non-existent key
	result, err = v.natives["GETPVPROFILEINT"].Fn([]advplrt.Value{
		advplrt.NewString("TCP"),
		advplrt.NewString("NONEXISTENT"),
		advplrt.NewNumber(999),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfileInt failed: %v", err)
	}
	val = result.(*advplrt.NumberValue).Val
	if val != 999 {
		t.Errorf("expected 999 (default), got %f", val)
	}

	// Test case 3: Non-numeric value returns default
	result, err = v.natives["GETPVPROFILEINT"].Fn([]advplrt.Value{
		advplrt.NewString("TCP"),
		advplrt.NewString("HOSTNAME"),
		advplrt.NewNumber(42),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err == nil {
		// Add non-numeric value to INI first
		createTempINI(t, dir, "test2.ini", "[TCP]\nHOSTNAME=localhost\n")
		result, _ = v.natives["GETPVPROFILEINT"].Fn([]advplrt.Value{
			advplrt.NewString("TCP"),
			advplrt.NewString("HOSTNAME"),
			advplrt.NewNumber(42),
			advplrt.NewString(filepath.Join(dir, "test2.ini")),
			advplrt.Nil,
			advplrt.Nil,
		})
		val = result.(*advplrt.NumberValue).Val
		if val != 42 {
			t.Errorf("expected 42 (default for non-numeric), got %f", val)
		}
	}
}

func TestGetPvProfString(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Test case 1: Read existing string value
	iniPath := createTempINI(t, dir, "test.ini", "[DRIVERS]\nACTIVE=CTREE\nDATABASE=ORACLE\n")
	result, err := v.natives["GETPVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("DRIVERS"),
		advplrt.NewString("ACTIVE"),
		advplrt.NewString("UNDEFINED"),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfString failed: %v", err)
	}
	val := result.(*advplrt.StringValue).Val
	if val != "CTREE" {
		t.Errorf("expected 'CTREE', got %q", val)
	}

	// Test case 2: Return default for non-existent key
	result, err = v.natives["GETPVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("DRIVERS"),
		advplrt.NewString("NONEXISTENT"),
		advplrt.NewString("DEFAULT_VALUE"),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfString failed: %v", err)
	}
	val = result.(*advplrt.StringValue).Val
	if val != "DEFAULT_VALUE" {
		t.Errorf("expected 'DEFAULT_VALUE', got %q", val)
	}

	// Test case 3: Non-existent section returns default
	result, err = v.natives["GETPVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("NONEXISTENT"),
		advplrt.NewString("KEY"),
		advplrt.NewString("FALLBACK"),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfString failed: %v", err)
	}
	val = result.(*advplrt.StringValue).Val
	if val != "FALLBACK" {
		t.Errorf("expected 'FALLBACK', got %q", val)
	}
}

func TestGetSrvProfString(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Set environment variable to point to our test INI
	oldEnv := os.Getenv("ADVPP_APPSERVER_INI")
	defer func() {
		if oldEnv == "" {
			os.Unsetenv("ADVPP_APPSERVER_INI")
		} else {
			os.Setenv("ADVPP_APPSERVER_INI", oldEnv)
		}
	}()

	// Create a server INI with an environment section
	srvIniPath := filepath.Join(dir, "appserver.ini")
	content := "[ENVIRONMENT]\nStartPath=C:\\totvs\nRootPath=C:\\totvs\\data\nCustomKey=CustomValue\n"
	if err := os.WriteFile(srvIniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create server INI: %v", err)
	}
	os.Setenv("ADVPP_APPSERVER_INI", srvIniPath)

	// Test case 1: Read existing value from server INI
	result, err := v.natives["GETSRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("StartPath"),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("GetSrvProfString failed: %v", err)
	}
	val := result.(*advplrt.StringValue).Val
	if val != "C:\\totvs" {
		t.Errorf("expected 'C:\\\\totvs', got %q", val)
	}

	// Test case 2: Return default for non-existent key
	result, err = v.natives["GETSRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("NonexistentKey"),
		advplrt.NewString("/default/path"),
	})
	if err != nil {
		t.Fatalf("GetSrvProfString failed: %v", err)
	}
	val = result.(*advplrt.StringValue).Val
	if val != "/default/path" {
		t.Errorf("expected '/default/path', got %q", val)
	}
}

func TestWriteSrvProfString(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Set environment variable to point to our test INI
	oldEnv := os.Getenv("ADVPP_APPSERVER_INI")
	defer func() {
		if oldEnv == "" {
			os.Unsetenv("ADVPP_APPSERVER_INI")
		} else {
			os.Setenv("ADVPP_APPSERVER_INI", oldEnv)
		}
	}()

	// Create a server INI
	srvIniPath := filepath.Join(dir, "appserver.ini")
	content := "[ENVIRONMENT]\nStartPath=C:\\totvs\n"
	if err := os.WriteFile(srvIniPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create server INI: %v", err)
	}
	os.Setenv("ADVPP_APPSERVER_INI", srvIniPath)

	// Test case 1: Write new key-value pair
	result, err := v.natives["WRITESRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("NewKey"),
		advplrt.NewString("NewValue"),
	})
	if err != nil {
		t.Fatalf("WriteSrvProfString failed: %v", err)
	}
	if !result.(*advplrt.BoolValue).Val {
		t.Error("expected true, got false")
	}

	// Verify the value was written
	result, err = v.natives["GETSRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("NewKey"),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("GetSrvProfString failed: %v", err)
	}
	val := result.(*advplrt.StringValue).Val
	if val != "NewValue" {
		t.Errorf("expected 'NewValue', got %q", val)
	}

	// Test case 2: Overwrite existing key
	result, err = v.natives["WRITESRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("StartPath"),
		advplrt.NewString("D:\\newpath"),
	})
	if err != nil {
		t.Fatalf("WriteSrvProfString failed: %v", err)
	}
	if !result.(*advplrt.BoolValue).Val {
		t.Error("expected true, got false")
	}

	// Verify the value was updated
	result, err = v.natives["GETSRVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("StartPath"),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("GetSrvProfString failed: %v", err)
	}
	val = result.(*advplrt.StringValue).Val
	if val != "D:\\newpath" {
		t.Errorf("expected 'D:\\\\newpath', got %q", val)
	}
}

func TestCommentsPreserved(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Create INI with comments and blank lines
	iniPath := createTempINI(t, dir, "test.ini",
		"; Configuration file\n"+
			"[SETTINGS]\n"+
			"; This is a comment\n"+
			"KEY1=VALUE1\n"+
			"# Another comment style\n"+
			"KEY2=VALUE2\n"+
			"\n"+
			"; Comment at end\n")

	// First verify we can read from the file
	result, err := v.natives["GETPVPROFSTRING"].Fn([]advplrt.Value{
		advplrt.NewString("SETTINGS"),
		advplrt.NewString("KEY1"),
		advplrt.NewString(""),
		advplrt.NewString(iniPath),
		advplrt.Nil,
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetPvProfString failed: %v", err)
	}
	val := result.(*advplrt.StringValue).Val
	if val != "VALUE1" {
		t.Errorf("expected 'VALUE1', got %q", val)
	}

	// Delete a key from the file
	_, err = v.natives["DELETEKEYINI"].Fn([]advplrt.Value{
		advplrt.NewString("SETTINGS"),
		advplrt.NewString("KEY1"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteKeyINI failed: %v", err)
	}

	// Verify comments are preserved in the file after deletion
	content := readINI(t, iniPath)
	if !strings.Contains(content, "; Configuration file") {
		t.Error("top comment was lost")
	}
	if !strings.Contains(content, "; This is a comment") {
		t.Error("inline comment was lost")
	}
	if !strings.Contains(content, "# Another comment style") {
		t.Error("hash-style comment was lost")
	}
	if !strings.Contains(content, "KEY2=VALUE2") {
		t.Error("unrelated key was lost")
	}
	// KEY1 should be deleted
	if strings.Contains(content, "KEY1=VALUE1") {
		t.Error("KEY1 should have been deleted")
	}
}

func TestGetINISessionsNoPreambleLeakage(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	// Create INI with leading comment (preamble) - common real-world pattern
	iniPath := createTempINI(t, dir, "test.ini",
		"; Configuration file\n"+
			"; Last modified: 2026-08-10\n"+
			"[SECTION1]\n"+
			"KEY1=VALUE1\n"+
			"[SECTION2]\n"+
			"KEY2=VALUE2\n")

	// Call GetINISessions on this file with leading comment
	result, err := v.natives["GETINISESSIONS"].Fn([]advplrt.Value{
		advplrt.NewString(iniPath),
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("GetINISessions failed: %v", err)
	}

	arr := result.(*advplrt.ArrayValue)

	// Should return exactly 2 sections: SECTION1 and SECTION2
	// Must NOT include the internal "\x00PREAMBLE" marker
	if len(arr.Elements) != 2 {
		t.Errorf("expected 2 sections, got %d", len(arr.Elements))
	}

	// Verify section names
	sections := make([]string, len(arr.Elements))
	for i, elem := range arr.Elements {
		sections[i] = elem.(*advplrt.StringValue).Val
	}

	expectedSections := []string{"SECTION1", "SECTION2"}
	for i, expected := range expectedSections {
		if i < len(sections) && sections[i] != expected {
			t.Errorf("section %d: expected %q, got %q", i, expected, sections[i])
		}
	}

	// Critical: verify preamble marker does NOT appear in results
	for _, sec := range sections {
		if sec == "\x00PREAMBLE" {
			t.Error("REGRESSION: internal preamble marker leaked into GetINISessions result")
		}
		if strings.Contains(sec, "\x00") {
			t.Errorf("REGRESSION: section name contains null byte: %q", sec)
		}
	}
}

func TestKeyOrderDeterministic(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	dir := t.TempDir()

	iniPath := filepath.Join(dir, "test.ini")

	// Create initial INI with keys in specific order
	initialContent := "[SECTION1]\nZkey=z_value\nAkey=a_value\nMkey=m_value\n"
	if err := os.WriteFile(iniPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create INI: %v", err)
	}

	// Delete Akey via DELETEKEYINI
	result, err := v.natives["DELETEKEYINI"].Fn([]advplrt.Value{
		advplrt.NewString("SECTION1"),
		advplrt.NewString("AKEY"),
		advplrt.NewString(iniPath),
	})
	if err != nil {
		t.Fatalf("DeleteKeyINI failed: %v", err)
	}
	if !result.(*advplrt.BoolValue).Val {
		t.Fatal("DeleteKeyINI returned false")
	}

	// Re-read file content
	content := readINI(t, iniPath)

	// Verify file structure - should still have section header and remaining keys
	// Note: keys are normalized to uppercase during parsing/serialization
	if !strings.Contains(content, "[SECTION1]") {
		t.Error("section header was lost")
	}
	if !strings.Contains(content, "ZKEY=z_value") {
		t.Errorf("ZKEY=z_value was lost (content: %q)", content)
	}
	if !strings.Contains(content, "MKEY=m_value") {
		t.Errorf("MKEY=m_value was lost (content: %q)", content)
	}
	if strings.Contains(content, "AKEY=a_value") {
		t.Error("AKEY should have been deleted")
	}

	// Verify order is deterministic by re-reading multiple times
	for i := 0; i < 3; i++ {
		content2 := readINI(t, iniPath)
		// Extract key lines in order
		lines := strings.Split(content2, "\n")
		var keyLines []string
		inSection := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "[SECTION1]" {
				inSection = true
				continue
			}
			if inSection {
				if strings.HasPrefix(trimmed, "[") {
					break
				}
				if strings.Contains(trimmed, "=") {
					keyLines = append(keyLines, trimmed)
				}
			}
		}

		// Verify keys appear in consistent order
		if len(keyLines) != 2 {
			t.Errorf("iteration %d: expected 2 keys, got %d: %v", i, len(keyLines), keyLines)
		}
		// First key should be Zkey, second should be Mkey (insertion order preserved)
		if !strings.HasPrefix(keyLines[0], "ZKEY=") {
			t.Errorf("iteration %d: expected first key to be ZKEY, got %s", i, keyLines[0])
		}
		if !strings.HasPrefix(keyLines[1], "MKEY=") {
			t.Errorf("iteration %d: expected second key to be MKEY, got %s", i, keyLines[1])
		}
	}
}
