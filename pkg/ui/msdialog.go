package ui

import (
	"encoding/json"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// dlgControl/dialogSpec/dialogAction mirror the JSON wire format vm.DialogUI
// speaks (pkg/vm/dialog.go) — a control declared via `@ x,y SAY/GET/BUTTON`,
// already grouped into rows by the VM's y/x grid heuristic before it
// reaches here. Only the JSON-tagged fields exist on the wire; the VM keeps
// type/codeblock/frame info server-side and only needs the raw text back.
type dlgControl struct {
	Kind    string  `json:"kind"` // say | get | button
	Name    string  `json:"name,omitempty"`
	Text    string  `json:"text,omitempty"`
	Value   string  `json:"value,omitempty"`
	Picture string  `json:"picture,omitempty"`
	Index   int     `json:"index"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

type dialogSpec struct {
	Title   string          `json:"title"`
	Rows    [][]*dlgControl `json:"rows"`
	Buttons []*dlgControl   `json:"buttons"`
}

type dialogAction struct {
	Action string            `json:"action"` // button | close
	Index  int               `json:"index"`
	Data   map[string]string `json:"data"`
}

// Dialog implements vm.DialogUI: renders a legacy MSDIALOG (`DEFINE
// MSDIALOG` / `@ x,y SAY|GET|BUTTON` / `ACTIVATE MSDIALOG`) as a real Fyne
// dialog and blocks until the user clicks a button or dismisses it.
//
// This method is called from the VM's own goroutine, which must NOT be the
// Fyne main/event goroutine — the VM run loop needs to be started via
// `go v.Run()` (see cmd/advpp-ide, cmd/adveditor) rather than called
// directly from a menu handler, exactly so that blocking here waiting on
// the result channel doesn't freeze the event loop that has to process the
// button clicks resolving that same channel.
func (p *FyneUIProvider) Dialog(specJSON []byte) []byte {
	var spec dialogSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return nil
	}

	entries := make(map[string]*widget.Entry)
	rows := container.NewVBox()
	for _, row := range spec.Rows {
		line := container.NewHBox()
		for _, ctl := range row {
			switch ctl.Kind {
			case "say":
				line.Add(widget.NewLabel(ctl.Text))
			case "get":
				e := widget.NewEntry()
				e.SetText(ctl.Value)
				entries[ctl.Name] = e
				line.Add(e)
			}
		}
		rows.Add(line)
	}
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(420, minFloat(float32(len(spec.Rows))*44, 360)))

	result := make(chan dialogAction, 1)
	var dlg *dialog.CustomDialog
	sent := false
	send := func(act dialogAction) {
		if sent {
			return
		}
		sent = true
		result <- act
		dlg.Hide()
	}

	buttons := container.NewHBox()
	for i, btn := range spec.Buttons {
		index := i
		buttons.Add(widget.NewButton(btn.Text, func() {
			data := make(map[string]string, len(entries))
			for name, e := range entries {
				data[name] = e.Text
			}
			send(dialogAction{Action: "button", Index: index, Data: data})
		}))
	}

	content := container.NewBorder(nil, buttons, nil, nil, scroll)
	dlg = dialog.NewCustomWithoutButtons(spec.Title, content, p.window)
	dlg.SetOnClosed(func() {
		// Escape / programmatic dismiss without a button click.
		send(dialogAction{Action: "close"})
	})
	dlg.Show()

	act := <-result
	data, _ := json.Marshal(act)
	return data
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
