package vm

import (
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TestJsonMapToAdvplObjectKeepsKeys garante que o objeto de parâmetros
// entregue aos handlers WSRestServer/MCPServer popula Props E Keys.
// Regressão da issue #3 ("jsonMapToAdvplObject missing Keys"): a escrita
// direta em obj.Props deixava Keys vazio, então GetNames() retornava [].
func TestJsonMapToAdvplObjectKeepsKeys(t *testing.T) {
	obj := jsonMapToAdvplObject(map[string]any{
		"subject":  "Duplicate charge",
		"criteria": []any{"billing", "technical"},
		"nested":   map[string]any{"a": 1.0},
	})

	// Props em maiúsculas (convenção de acesso obj:CAMPO)
	if _, ok := obj.Props["SUBJECT"]; !ok {
		t.Errorf("Props sem SUBJECT; chaves=%v", keysOf(obj))
	}
	if _, ok := obj.Props["CRITERIA"]; !ok {
		t.Errorf("Props sem CRITERIA; chaves=%v", keysOf(obj))
	}
	if _, ok := obj.Props["NESTED"]; !ok {
		t.Errorf("Props sem NESTED; chaves=%v", keysOf(obj))
	}

	// Keys na ordem de inserção (base do GetNames)
	if len(obj.Keys) != 3 {
		t.Fatalf("len(Keys) = %d, esperado 3 (Keys=%v)", len(obj.Keys), obj.Keys)
	}
	want := map[string]bool{"SUBJECT": true, "CRITERIA": true, "NESTED": true}
	for _, k := range obj.Keys {
		if !want[k] {
			t.Errorf("Keys contém chave inesperada %q (Keys=%v)", k, obj.Keys)
		}
		delete(want, k)
	}
	for k := range want {
		t.Errorf("Keys sem %q (Keys=%v)", k, obj.Keys)
	}

	// Aninhado também mantém Keys
	nested, ok := obj.Props["NESTED"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("NESTED não é *ObjectValue: %T", obj.Props["NESTED"])
	}
	if len(nested.Keys) != 1 || nested.Keys[0] != "A" {
		t.Errorf("NESTED.Keys = %v, esperado [A]", nested.Keys)
	}

	// Array preservado
	arr, ok := obj.Props["CRITERIA"].(*advplrt.ArrayValue)
	if !ok {
		t.Fatalf("CRITERIA não é *ArrayValue: %T", obj.Props["CRITERIA"])
	}
	if len(arr.Elements) != 2 {
		t.Errorf("len(CRITERIA) = %d, esperado 2", len(arr.Elements))
	}
}

func keysOf(o *advplrt.ObjectValue) []string {
	out := make([]string, 0, len(o.Props))
	for k := range o.Props {
		out = append(out, k)
	}
	return out
}
