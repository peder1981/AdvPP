package ui

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// browseColumn/browseSpec/browseAction mirror the JSON wire format
// vm.BrowseUI speaks (pkg/vm/browse.go) — a FWMBrowse over a SQLite table,
// columns/rows already resolved server-side from the SX3 dictionary (or
// physical columns as a fallback).
type browseColumn struct {
	Property string `json:"property"`
	Label    string `json:"label"`
	Type     string `json:"type"` // C | N | D | L | M (tipos SX3)
	Size     int    `json:"size"`
	Decimal  int    `json:"decimal"`
}

type browseSpec struct {
	Title   string           `json:"title"`
	Alias   string           `json:"alias"`
	Columns []browseColumn   `json:"columns"`
	Items   []map[string]any `json:"items"`
}

type browseAction struct {
	Action string            `json:"action"` // save | delete | close
	Recno  int64             `json:"recno"`  // rowid; 0 = new record
	Data   map[string]string `json:"data"`
}

// Browse implements vm.BrowseUI: renders a FWMBrowse as a real Fyne grid
// (widget.Table, row 0 = header) with Novo/Editar/Excluir/Fechar actions.
// Blocks until one of those resolves an action back to the VM, which then
// re-queries the table and calls Browse again with fresh data — so a
// save/delete visibly rebuilds the grid rather than updating it in place;
// simpler and safer than trying to keep one grid instance alive and in
// sync across calls, at the cost of a brief flicker on each edit.
func (p *FyneUIProvider) Browse(specJSON []byte) []byte {
	var spec browseSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return nil
	}

	result := make(chan browseAction, 1)
	sent := false
	var dlg *dialog.CustomDialog
	send := func(act browseAction) {
		if sent {
			return
		}
		sent = true
		result <- act
		dlg.Hide()
	}

	selected := -1 // index into spec.Items, -1 = none

	table := widget.NewTable(
		func() (int, int) { return len(spec.Items) + 1, len(spec.Columns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(spec.Columns[id.Col].Label)
				return
			}
			label.TextStyle = fyne.TextStyle{}
			item := spec.Items[id.Row-1]
			label.SetText(fmt.Sprintf("%v", item[spec.Columns[id.Col].Property]))
		},
	)
	for i, col := range spec.Columns {
		width := float32(120)
		if col.Size > 0 && col.Size < 20 {
			width = float32(col.Size)*8 + 20
		}
		table.SetColumnWidth(i, width)
	}
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			table.Unselect(id)
			return
		}
		selected = id.Row - 1
	}

	recnoOf := func(idx int) int64 {
		if idx < 0 || idx >= len(spec.Items) {
			return 0
		}
		// json.Unmarshal into map[string]any always produces float64 for
		// JSON numbers, never int64.
		n, _ := spec.Items[idx]["recno"].(float64)
		return int64(n)
	}

	editForm := func(current map[string]any, recno int64) {
		entries := make(map[string]*widget.Entry, len(spec.Columns))
		items := make([]*widget.FormItem, 0, len(spec.Columns))
		for _, c := range spec.Columns {
			e := widget.NewEntry()
			if current != nil {
				if v, ok := current[c.Property]; ok && v != nil {
					e.SetText(fmt.Sprintf("%v", v))
				}
			}
			entries[c.Property] = e
			items = append(items, widget.NewFormItem(c.Label, e))
		}
		dialog.ShowForm(spec.Title, "Salvar", "Cancelar", items, func(confirmed bool) {
			if !confirmed {
				return
			}
			data := make(map[string]string, len(entries))
			for name, e := range entries {
				data[name] = e.Text
			}
			send(browseAction{Action: "save", Recno: recno, Data: data})
		}, p.window)
	}

	novoBtn := widget.NewButton("Novo", func() {
		editForm(nil, 0)
	})
	editarBtn := widget.NewButton("Editar", func() {
		if selected < 0 {
			return
		}
		editForm(spec.Items[selected], recnoOf(selected))
	})
	excluirBtn := widget.NewButton("Excluir", func() {
		if selected < 0 {
			return
		}
		recno := recnoOf(selected)
		dialog.ShowConfirm("Confirma", "Excluir o registro selecionado?", func(ok bool) {
			if ok {
				send(browseAction{Action: "delete", Recno: recno})
			}
		}, p.window)
	})
	fecharBtn := widget.NewButton("Fechar", func() {
		send(browseAction{Action: "close"})
	})

	buttons := container.NewHBox(novoBtn, editarBtn, excluirBtn, fecharBtn)
	content := container.NewBorder(nil, buttons, nil, nil, table)

	dlg = dialog.NewCustomWithoutButtons(spec.Title, content, p.window)
	dlg.Resize(fyne.NewSize(640, 420))
	dlg.SetOnClosed(func() {
		send(browseAction{Action: "close"})
	})
	dlg.Show()

	act := <-result
	data, _ := json.Marshal(act)
	return data
}
