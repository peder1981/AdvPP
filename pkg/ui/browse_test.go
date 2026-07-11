package ui

import (
	"encoding/json"
	"testing"
)

// TestBrowseSpecUnmarshal exercises the exact JSON shape pkg/vm/browse.go's
// runBrowse produces — the two packages don't share Go types, so a tag
// drift on either side would otherwise fail silently at runtime.
func TestBrowseSpecUnmarshal(t *testing.T) {
	raw := `{
		"title": "Cadastro de Clientes",
		"alias": "SA1",
		"columns": [
			{"property":"A1_COD","label":"Codigo","type":"C","size":6},
			{"property":"A1_NOME","label":"Nome","type":"C","size":40}
		],
		"items": [
			{"recno":1,"A1_COD":"000001","A1_NOME":"MARIA"},
			{"recno":2,"A1_COD":"000002","A1_NOME":"JOAO"}
		]
	}`

	var spec browseSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if spec.Title != "Cadastro de Clientes" || spec.Alias != "SA1" {
		t.Errorf("Title/Alias = %q/%q", spec.Title, spec.Alias)
	}
	if len(spec.Columns) != 2 || spec.Columns[0].Property != "A1_COD" {
		t.Fatalf("Columns = %+v", spec.Columns)
	}
	if len(spec.Items) != 2 {
		t.Fatalf("Items = %+v", spec.Items)
	}
	if recno, _ := spec.Items[0]["recno"].(float64); recno != 1 {
		t.Errorf("Items[0][recno] = %v", spec.Items[0]["recno"])
	}
	if spec.Items[1]["A1_NOME"] != "JOAO" {
		t.Errorf("Items[1][A1_NOME] = %v", spec.Items[1]["A1_NOME"])
	}
}

// TestBrowseActionMarshal checks the response shape pkg/vm/browse.go's
// runBrowse expects back: Action, Recno (rowid; 0 for a new record), and a
// flat Data map keyed by column name.
func TestBrowseActionMarshal(t *testing.T) {
	act := browseAction{
		Action: "save",
		Recno:  0,
		Data:   map[string]string{"A1_COD": "000003", "A1_NOME": "PEDRO"},
	}
	data, err := json.Marshal(act)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if got["action"] != "save" {
		t.Errorf("action = %v", got["action"])
	}
	if recno, _ := got["recno"].(float64); recno != 0 {
		t.Errorf("recno = %v", got["recno"])
	}
	values, ok := got["data"].(map[string]any)
	if !ok || values["A1_NOME"] != "PEDRO" {
		t.Errorf("data = %v", got["data"])
	}
}
