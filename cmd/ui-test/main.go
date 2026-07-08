package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/advpl/compiler/pkg/mvc"
	"github.com/advpl/compiler/pkg/ui"
)

func main() {
	a := app.New()
	w := a.NewWindow("AdvPP UI Test")

	// Create MVC model
	model := mvc.NewFWFormModel("TestModel")

	// Create MVC view
	view := mvc.NewFWFormView("TestView", model)
	view.SetTitle("AdvPP UI Components Test")
	view.SetSize(600, 500)

	// Add components to view
	view.AddComponent(&mvc.Component{
		Name:  "lblTitle",
		Type:  "TLabel",
		Label: "UI Components Test",
	})

	view.AddComponent(&mvc.Component{
		Name:  "txtName",
		Type:  "TGet",
		Label: "Name:",
		Value: "Test User",
	})

	view.AddComponent(&mvc.Component{
		Name:    "cmbRole",
		Type:    "TComboBox",
		Label:   "Role:",
		Options: []string{"Admin", "User", "Guest"},
		Value:   "User",
	})

	view.AddComponent(&mvc.Component{
		Name:    "chkActive",
		Type:    "TCheckBox",
		Label:   "Active",
		Value:   true,
	})

	view.AddComponent(&mvc.Component{
		Name:  "btnSave",
		Type:  "TButton",
		Label: "Save",
	})

	view.AddComponent(&mvc.Component{
		Name:  "btnCancel",
		Type:  "TButton",
		Label: "Cancel",
	})

	// Create renderer
	renderer := ui.NewRenderer(w)

	// Render the view
	content := renderer.RenderFormView(view)

	// Add status bar
	statusBar := &mvc.StatusBar{
		Panels: []*mvc.StatusPanel{
			{Name: "status", Text: "Ready", Width: 200},
			{Name: "info", Text: "UI Test", Width: 100},
		},
	}
	statusBarWidget := renderer.RenderStatusBar(statusBar)

	// Add toolbar
	toolBar := &mvc.ToolBar{
		Buttons: []*mvc.ToolButton{
			{Name: "new", Icon: "New", Action: "New"},
			{Name: "open", Icon: "Open", Action: "Open"},
			{Name: "save", Icon: "Save", Action: "Save"},
		},
	}
	toolBarWidget := renderer.RenderToolBar(toolBar)

	// Layout: toolbar on top, content in middle, status bar at bottom
	mainContainer := container.NewBorder(
		toolBarWidget,
		statusBarWidget,
		nil,
		nil,
		content,
	)

	w.SetContent(mainContainer)
	w.Resize(fyne.NewSize(600, 500))
	w.ShowAndRun()
}
