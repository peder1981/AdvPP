package ui

import (
	"encoding/json"
	"testing"
)

// TestDialogSpecUnmarshal exercises the exact JSON shape pkg/vm/dialog.go's
// buildDialogSpec produces — the two packages don't share Go types (the
// wire format is the only contract between them), so a tag drift on either
// side would otherwise fail silently at runtime instead of at compile time.
func TestDialogSpecUnmarshal(t *testing.T) {
	raw := `{
		"title": "Cadastro rapido",
		"rows": [
			[{"kind":"say","text":"Nome:","x":10,"y":10,"index":0},
			 {"kind":"get","name":"cNome","value":"MARIA","picture":"","x":60,"y":10,"index":0}]
		],
		"buttons": [
			{"kind":"button","text":"Confirmar","index":0,"x":10,"y":80}
		]
	}`

	var spec dialogSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if spec.Title != "Cadastro rapido" {
		t.Errorf("Title = %q", spec.Title)
	}
	if len(spec.Rows) != 1 || len(spec.Rows[0]) != 2 {
		t.Fatalf("unexpected Rows shape: %+v", spec.Rows)
	}
	if spec.Rows[0][0].Kind != "say" || spec.Rows[0][0].Text != "Nome:" {
		t.Errorf("say control = %+v", spec.Rows[0][0])
	}
	if spec.Rows[0][1].Kind != "get" || spec.Rows[0][1].Name != "cNome" || spec.Rows[0][1].Value != "MARIA" {
		t.Errorf("get control = %+v", spec.Rows[0][1])
	}
	if len(spec.Buttons) != 1 || spec.Buttons[0].Text != "Confirmar" {
		t.Errorf("Buttons = %+v", spec.Buttons)
	}
}

// TestDialogActionMarshal checks the response shape pkg/vm/dialog.go's
// runDialog expects back: Action, Index, and a flat Data map keyed by GET
// variable name (used for writeback into the AdvPL locals).
func TestDialogActionMarshal(t *testing.T) {
	act := dialogAction{
		Action: "button",
		Index:  0,
		Data:   map[string]string{"cNome": "JOAO PEREIRA", "cCid": "SAO PAULO"},
	}
	data, err := json.Marshal(act)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if got["action"] != "button" {
		t.Errorf("action = %v", got["action"])
	}
	values, ok := got["data"].(map[string]any)
	if !ok || values["cNome"] != "JOAO PEREIRA" {
		t.Errorf("data = %v", got["data"])
	}
}

func TestMinFloat(t *testing.T) {
	if got := minFloat(3, 5); got != 3 {
		t.Errorf("minFloat(3,5) = %v, want 3", got)
	}
	if got := minFloat(5, 3); got != 3 {
		t.Errorf("minFloat(5,3) = %v, want 3", got)
	}
}
