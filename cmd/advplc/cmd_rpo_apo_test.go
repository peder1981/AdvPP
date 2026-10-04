package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdRpoApo_DirGeraJSON(t *testing.T) {
	// copia as fixtures reais para um dir temporário
	src := filepath.Join("..", "..", "pkg", "rpo", "testdata", "apo")
	dir := t.TempDir()
	for _, name := range []string{"EXTXDEF.PRW", "ABSLOGGER.PRW"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("lendo fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	err := cmdRpoApo([]string{
		"--catalog", filepath.Join("..", "..", "pkg", "rpo", "testdata", "catalog_tiny.txt"),
		"--out", out,
		dir,
	})
	if err != nil {
		t.Fatalf("cmdRpoApo: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "apo_report.json"))
	if err != nil {
		t.Fatalf("lendo relatório: %v", err)
	}
	var report []struct {
		FileName       string `json:"file"`
		Kind           string `json:"kind"`
		Identifiers    []string `json:"identifiers"`
		CallCandidates []struct {
			Name       string `json:"name"`
			Confidence string `json:"confidence"`
		} `json:"call_candidates"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(report) != 2 {
		t.Fatalf("report tem %d entradas, esperado 2", len(report))
	}
	byFile := map[string]int{}
	for i, e := range report {
		byFile[e.FileName] = i
		if e.Kind == "" {
			t.Errorf("entry %d sem kind", i)
		}
	}
	extIdx, ok := byFile["EXTXDEF.PRW"]
	if !ok {
		t.Fatalf("EXTXDEF.PRW ausente: %+v", report)
	}
	if report[extIdx].Kind != "AdvPL" {
		t.Errorf("EXTXDEF kind = %q", report[extIdx].Kind)
	}
	absIdx := byFile["ABSLOGGER.PRW"]
	for _, c := range report[absIdx].CallCandidates {
		if c.Name == "ABSLOGGER" && c.Confidence != "INFERIDO" {
			t.Errorf("confidence = %q, esperado INFERIDO", c.Confidence)
		}
	}
}

func TestCmdRpoApo_FormatoMD(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "pkg", "rpo", "testdata", "apo", "EXTXDEF.PRW"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "EXTXDEF.PRW"), data, 0644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := cmdRpoApo([]string{"--format", "md", "--out", out, dir}); err != nil {
		t.Fatalf("cmdRpoApo: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "apo_report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "| EXTXDEF.PRW | AdvPL | 39 |") {
		t.Errorf("tabela MD não contém linha esperada:\n%s", raw)
	}
}
