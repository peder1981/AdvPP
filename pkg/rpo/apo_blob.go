package rpo

import (
	"fmt"
)

// ApoHeaderPrefix é o prefixo observado em TODOS os 3.465 blobs APO
// extraídos (onda 39): 75 00 00 46 46. Verificado por hexdump em
// EXTXDEF.PRW, TECA740A.PRW, ABSLOGGER.PRW e amostra TLPP (2026-10-04).
var ApoHeaderPrefix = []byte{0x75, 0x00, 0x00, 0x46, 0x46}

// ApoKind é o byte de tipo na posição 5 do blob.
type ApoKind byte

const (
	ApoKindAdvPL ApoKind = 'F' // PRW/PRX/PRG/APH/APW
	ApoKindTLPP  ApoKind = 'T' // TLPP
)

// String retorna a forma legível do kind.
func (k ApoKind) String() string {
	switch k {
	case ApoKindAdvPL:
		return "AdvPL"
	case ApoKindTLPP:
		return "TLPP"
	}
	return fmt.Sprintf("desconhecido(%q)", byte(k))
}

// ApoBlob é a desmontagem de um blob APO extraído via GetApoRes.
type ApoBlob struct {
	Raw  []byte
	Kind ApoKind
	Size int
}

// ParseApoBlob valida o framing mínimo (magic 5 bytes + kind) do blob APO.
// O restante do layout (tabelas de records) ainda não é100% decifrado —
// ver docs/RPO-EXTRACTION-METHODOLOGY.md; as tabelas derivadas (strings,
// literais) são preenchidas por varredura nas tasks seguintes.
func ParseApoBlob(data []byte) (*ApoBlob, error) {
	if len(data) < len(ApoHeaderPrefix)+1 {
		return nil, fmt.Errorf("apo: blob muito pequeno (%d bytes)", len(data))
	}
	for i, b := range ApoHeaderPrefix {
		if data[i] != b {
			return nil, fmt.Errorf("apo: magic inválida no offset %d (0x%02X != 0x%02X)", i, data[i], b)
		}
	}
	kind := ApoKind(data[5])
	if kind != ApoKindAdvPL && kind != ApoKindTLPP {
		return nil, fmt.Errorf("apo: kind desconhecido 0x%02X (esperado 'F' ou 'T')", data[5])
	}
	return &ApoBlob{Raw: data, Kind: kind, Size: len(data)}, nil
}
