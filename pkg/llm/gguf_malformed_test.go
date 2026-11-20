package llm

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Header GGUF hostil não pode derrubar o processo (OOM/pânico): contagens
// absurdas viram erro.
func TestOpenMalformedGGUF(t *testing.T) {
	le := binary.LittleEndian
	cases := map[string][]byte{
		"nTensors gigante": le.AppendUint64(le.AppendUint64(le.AppendUint32(le.AppendUint32(nil, ggufMagic), 3), 1<<62), 0),
		"string gigante":   le.AppendUint64(le.AppendUint64(le.AppendUint64(le.AppendUint32(le.AppendUint32(nil, ggufMagic), 3), 0), 1), 1<<62),
	}
	for name, data := range cases {
		path := filepath.Join(t.TempDir(), "x.gguf")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if g, err := Open(path); err == nil {
			g.Close()
			t.Errorf("%s: esperava erro", name)
		}
	}
}
