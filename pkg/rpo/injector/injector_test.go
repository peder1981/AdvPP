package injector

import (
	"bytes"
	"os"
	"testing"
)

// buildTestRPO constrói um RPO sintético válido para testes.
func buildTestRPO() []byte {
	selfOffset := uint32(70)
	header := make([]byte, 38)
	header[0] = byte(selfOffset & 0xFF)
	header[1] = byte((selfOffset >> 8) & 0xFF)
	header[2] = byte((selfOffset >> 16) & 0xFF)
	header[3] = byte((selfOffset >> 24) & 0xFF)
	copy(header[4:20], "test")
	
	admin := make([]byte, 32)
	body := make([]byte, 100)
	for i := range body {
		body[i] = byte(i & 0xFF)
	}
	
	footer := make([]byte, 34)
	copy(footer[0:10], "APNSRM0419")
	
	data := append(header, admin...)
	data = append(data, body...)
	data = append(data, footer...)
	
	return data
}

// TestNewInjector_ValidRPO testa criação com RPO válido.
func TestNewInjector_ValidRPO(t *testing.T) {
	data := buildTestRPO()
	
	inj, err := NewInjector(data)
	if err != nil {
		t.Fatalf("NewInjector falhou: %v", err)
	}
	if inj.RPOName() != "test" {
		t.Errorf("RPOName esperado 'test', got %s", inj.RPOName())
	}
	if inj.SelfOffset() != 70 {
		t.Errorf("SelfOffset esperado 70, got %d", inj.SelfOffset())
	}
	if len(inj.Body()) != 100 {
		t.Errorf("Body size esperado 100, got %d", len(inj.Body()))
	}
}

// TestNewInjector_TooSmall testa RPO muito pequeno.
func TestNewInjector_TooSmall(t *testing.T) {
	data := []byte{0x01, 0x00}
	_, err := NewInjector(data)
	if err == nil {
		t.Fatal("esperado erro para RPO pequeno")
	}
}

// TestNewInjector_InvalidSelfOffset testa selfOffset fora dos limites.
func TestNewInjector_InvalidSelfOffset(t *testing.T) {
	data := make([]byte, 100)
	data[0] = 0xC8
	data[1] = 0x00
	data[2] = 0x00
	data[3] = 0x00

	_, err := NewInjector(data)
	if err == nil {
		t.Fatal("esperado erro para selfOffset inválido")
	}
}

// TestExtractAPORecords_ExtraiRegistroVálido testa extração de APO.
func TestExtractAPORecords_ExtraiRegistroVálido(t *testing.T) {
	data := []byte{
		0x04, 0x00, 0x00, 0x00,
		0x54, 0x45, 0x53, 0x54, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x00, 0x00,
		0xDE, 0xAD, 0xBE, 0xEF,
	}

	records := ExtractAPORecords(data)
	if len(records) != 1 {
		t.Fatalf("esperado 1 registro, got %d", len(records))
	}

	rec := records[0]
	if rec.Name != "TEST" {
		t.Errorf("Nome esperado 'TEST', got %s", rec.Name)
	}
	if rec.BuildType != 1 {
		t.Errorf("BuildType esperado 1, got %d", rec.BuildType)
	}
	if rec.BinaryType != 2 {
		t.Errorf("BinaryType esperado 2, got %d", rec.BinaryType)
	}
	if len(rec.Code) != 4 {
		t.Errorf("Code size esperado 4, got %d", len(rec.Code))
	}
	if !bytes.Equal(rec.Code, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("Code content inválido")
	}
}

// TestExtractAPORecords_MúltiplosRegistros testa múltiplos APOs.
func TestExtractAPORecords_MúltiplosRegistros(t *testing.T) {
	// Nomes com pelo menos 2 caracteres
	data := []byte{
		// Registro 1: name="FUNC", codeSize=4
		0x04, 0x00, 0x00, 0x00, // codeSize = 4
		0x46, 0x55, 0x4E, 0x43, 0x00, // "FUNC\0"
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // timestamp
		0x01, 0x00, 0x00, 0x00, // build
		0x02, 0x00, 0x00, 0x00, // binary
		0x01, 0x02, 0x03, 0x04, // code
		
		// Registro 2: name="CLASS", codeSize=8
		0x08, 0x00, 0x00, 0x00, // codeSize = 8
		0x43, 0x4C, 0x41, 0x53, 0x53, 0x00, // "CLASS\0"
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // timestamp
		0x03, 0x00, 0x00, 0x00, // build
		0x04, 0x00, 0x00, 0x00, // binary
		0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, // code
	}

	records := ExtractAPORecords(data)
	if len(records) != 2 {
		t.Fatalf("esperado 2 registros, got %d", len(records))
	}

	if records[0].Name != "FUNC" {
		t.Errorf("Registro 0: nome esperado 'FUNC', got %s", records[0].Name)
	}
	if records[0].Code[0] != 0x01 || records[0].Code[3] != 0x04 {
		t.Errorf("Registro 0: código inválido")
	}
	
	if records[1].Name != "CLASS" {
		t.Errorf("Registro 1: nome esperado 'CLASS', got %s", records[1].Name)
	}
	if records[1].Code[0] != 0x05 || records[1].Code[7] != 0x0C {
		t.Errorf("Registro 1: código inválido")
	}
}

// TestSave_ReconstróiRPO testa salvamento e reconstrução.
func TestSave_ReconstróiRPO(t *testing.T) {
	data, err := os.ReadFile("../testdata/live_capture.rpo")
	if err != nil {
		t.Skip("RPO de teste não encontrado")
	}

	inj, err := NewInjector(data)
	if err != nil {
		t.Fatalf("NewInjector falhou: %v", err)
	}

	tmpFile := t.TempDir() + "/test_save_rpo.rpo"
	defer os.Remove(tmpFile)

	err = inj.Save(tmpFile)
	if err != nil {
		t.Fatalf("Save falhou: %v", err)
	}

	saved, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Erro ao ler arquivo salvo: %v", err)
	}

	if len(saved) != len(data) {
		t.Errorf("Tamanho diferente: saved=%d, original=%d", len(saved), len(data))
	}
}

// TestReplaceAPO_MesmoTamanho testa substituição de mesmo tamanho.
func TestReplaceAPO_MesmoTamanho(t *testing.T) {
	data, err := os.ReadFile("../testdata/live_capture.rpo")
	if err != nil {
		t.Skip("RPO de teste não encontrado")
	}

	inj, err := NewInjector(data)
	if err != nil {
		t.Fatalf("NewInjector falhou: %v", err)
	}

	err = inj.LoadCapture("../testdata/live_capture.json")
	if err != nil {
		t.Fatalf("LoadCapture falhou: %v", err)
	}

	inj.DecodeBody()

	newCode := make([]byte, 14)
	for i := range newCode {
		newCode[i] = 0xFF
	}

	err = inj.ReplaceAPO(0, newCode)
	if err != nil {
		t.Fatalf("ReplaceAPO falhou: %v", err)
	}
}

// TestSortRecords_OrdenaPorOffset testa ordenação de registros.
func TestSortRecords_OrdenaPorOffset(t *testing.T) {
	records := []*APORecord{
		{Offset: 100, Name: "C"},
		{Offset: 50, Name: "B"},
		{Offset: 0, Name: "A"},
	}

	SortRecords(records)

	if records[0].Name != "A" {
		t.Errorf("Primeiro esperado 'A', got %s", records[0].Name)
	}
	if records[1].Name != "B" {
		t.Errorf("Segundo esperado 'B', got %s", records[1].Name)
	}
	if records[2].Name != "C" {
		t.Errorf("Terceiro esperado 'C', got %s", records[2].Name)
	}
}
