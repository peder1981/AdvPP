package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/advpl/compiler/pkg/rpo"
)

// TestRpoDecomposeBuildRoundTrip cobre a regressão de 2026-09-19: rpoBuild
// esquecia de repassar o Header bruto lido de header.bin para o rpo.File
// reconstruído, fazendo Bytes() gerar um cabeçalho vazio (arquivo 38 bytes
// menor e completamente diferente do original) em vez de reaproveitar os
// bytes originais preservados por rpoDecompose.
func TestRpoDecomposeBuildRoundTrip(t *testing.T) {
	header := make([]byte, rpo.HeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], rpo.HeaderSize)
	copy(header[4:20], []byte("test"))
	binary.LittleEndian.PutUint32(header[24:28], 0xFFFFFFFF) // sentinel offset

	original := &rpo.File{
		Header:       header,
		Name:         "test",
		SelfOffset:   rpo.HeaderSize,
		Sentinel:     0xFFFFFFFF,
		AdminSection: []byte{0x01, 0x02, 0x03},
		Body:         []byte{0xDE, 0xAD, 0xBE, 0xEF},
		FooterMagic:  "APNSRM0419",
		FooterTrail:  make([]byte, 24),
	}

	dir := t.TempDir()
	rpoPath := filepath.Join(dir, "custom.rpo")
	if err := os.WriteFile(rpoPath, original.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	decomposeDir := filepath.Join(dir, "decomposed")
	if err := rpoDecompose(rpoPath, decomposeDir); err != nil {
		t.Fatalf("rpoDecompose() error = %v", err)
	}

	rebuiltPath := filepath.Join(dir, "rebuilt.rpo")
	if err := rpoBuild(decomposeDir, rebuiltPath); err != nil {
		t.Fatalf("rpoBuild() error = %v", err)
	}

	want := original.Bytes()
	got, err := os.ReadFile(rebuiltPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("round-trip não é byte-idêntico: len(want)=%d len(got)=%d", len(want), len(got))
	}
}
