package rpo

import "testing"

// Arquivos RPO/APO vêm de terceiros: nenhum byte pode derrubar o parser.
func FuzzParse(f *testing.F) {
	f.Add(make([]byte, 128))
	f.Fuzz(func(t *testing.T, data []byte) {
		Parse(data)
		ParseApoBlob(data)
		NewAPOParser(data).ExtractAll()
	})
}
