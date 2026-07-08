package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/advpl/compiler/pkg/mvc"
)

// Renderer converts MVC components to Fyne widgets
type Renderer struct {
	window fyne.Window
}

// NewRenderer creates a new UI renderer
func NewRenderer(window fyne.Window) *Renderer {
	return &Renderer{window: window}
}

// RenderFormView renders an FWFormView as a Fyne container
func (r *Renderer) RenderFormView(view *mvc.FWFormView) fyne.CanvasObject {
	components := container.NewVBox()

	for _, comp := range view.Components {
		widget := r.RenderComponent(comp)
		if widget != nil {
			components.Add(widget)
		}
	}

	scroll := container.NewScroll(components)
	scroll.SetMinSize(fyne.NewSize(float32(view.Width), float32(view.Height)))

	return container.NewBorder(nil, nil, nil, nil, scroll)
}

// RenderComponent renders a single MVC component as a Fyne widget
func (r *Renderer) RenderComponent(comp *mvc.Component) fyne.CanvasObject {
	switch comp.Type {
	case "TButton":
		return r.RenderButton(comp)
	case "TGet":
		return r.RenderGet(comp)
	case "TComboBox":
		return r.RenderComboBox(comp)
	case "TCheckBox":
		return r.RenderCheckBox(comp)
	case "TLabel":
		return r.RenderLabel(comp)
	default:
		return nil
	}
}

// RenderButton renders a TButton as a Fyne button
func (r *Renderer) RenderButton(comp *mvc.Component) fyne.CanvasObject {
	label := comp.Label
	if label == "" {
		label = comp.Name
	}

	button := widget.NewButton(label, func() {
		// Trigger onClick event if exists
	})

	return button
}

// RenderGet renders a TGet as a Fyne entry
func (r *Renderer) RenderGet(comp *mvc.Component) fyne.CanvasObject {
	label := comp.Label
	if label == "" {
		label = comp.Name
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder(label)
	if comp.Value != nil {
		entry.SetText(comp.Value.(string))
	}
	entry.Disable()

	return container.NewBorder(
		widget.NewLabel(label),
		nil, nil, nil,
		entry,
	)
}

// RenderComboBox renders a TComboBox as a Fyne select
func (r *Renderer) RenderComboBox(comp *mvc.Component) fyne.CanvasObject {
	label := comp.Label
	if label == "" {
		label = comp.Name
	}

	selectWidget := widget.NewSelect(comp.Options, func(selected string) {
		// Handle selection
	})

	if comp.Value != nil {
		selectWidget.SetSelected(comp.Value.(string))
	}

	return container.NewBorder(
		widget.NewLabel(label),
		nil, nil, nil,
		selectWidget,
	)
}

// RenderCheckBox renders a TCheckBox as a Fyne check
func (r *Renderer) RenderCheckBox(comp *mvc.Component) fyne.CanvasObject {
	label := comp.Label
	if label == "" {
		label = comp.Name
	}

	checked := false
	if comp.Value != nil {
		if b, ok := comp.Value.(bool); ok {
			checked = b
		}
	}

	check := widget.NewCheck(label, func(checked bool) {
		// Handle check change
	})
	check.SetChecked(checked)

	return check
}

// RenderLabel renders a TLabel as a Fyne label
func (r *Renderer) RenderLabel(comp *mvc.Component) fyne.CanvasObject {
	text := comp.Label
	if text == "" {
		text = comp.Name
	}

	return widget.NewLabel(text)
}

// ShowFormView displays a form view in a new window
func (r *Renderer) ShowFormView(view *mvc.FWFormView) {
	content := r.RenderFormView(view)

	if view.Title != "" {
		r.window.SetTitle(view.Title)
	}

	r.window.SetContent(content)
	r.window.Resize(fyne.NewSize(float32(view.Width), float32(view.Height)))
	r.window.Show()
}

// RenderMenuBar renders a MenuBar as Fyne menu
func (r *Renderer) RenderMenuBar(menuBar *mvc.MenuBar) *fyne.MainMenu {
	if menuBar == nil {
		return nil
	}

	var menus []*fyne.Menu

	for _, item := range menuBar.Items {
		if !item.Separator {
			menu := r.RenderMenu(item)
			if menu != nil {
				menus = append(menus, menu)
			}
		}
	}

	return fyne.NewMainMenu(menus...)
}

// RenderMenu renders a MenuItem as a Fyne Menu
func (r *Renderer) RenderMenu(item *mvc.MenuItem) *fyne.Menu {
	var menuItems []*fyne.MenuItem

	if item.Items != nil && len(item.Items) > 0 {
		// Submenu items
		for _, subItem := range item.Items {
			if subItem.Separator {
				menuItems = append(menuItems, fyne.NewMenuItemSeparator())
			} else {
				menuItem := fyne.NewMenuItem(subItem.Label, func() {
					// Handle menu action
				})
				menuItems = append(menuItems, menuItem)
			}
		}
	} else {
		// Single menu item
		menuItem := fyne.NewMenuItem(item.Label, func() {
			// Handle menu action
		})
		menuItems = append(menuItems, menuItem)
	}

	return fyne.NewMenu(item.Label, menuItems...)
}

// RenderToolBar renders a ToolBar as Fyne toolbar
func (r *Renderer) RenderToolBar(toolBar *mvc.ToolBar) fyne.CanvasObject {
	if toolBar == nil {
		return nil
	}

	toolbar := container.NewHBox()

	for _, button := range toolBar.Buttons {
		btn := widget.NewButton(button.Icon, func() {
			// Handle tool button action
		})
		toolbar.Add(btn)
	}

	return toolbar
}

// RenderStatusBar renders a StatusBar as Fyne status bar
func (r *Renderer) RenderStatusBar(statusBar *mvc.StatusBar) fyne.CanvasObject {
	if statusBar == nil {
		return nil
	}

	panels := container.NewHBox()

	for _, panel := range statusBar.Panels {
		label := widget.NewLabel(panel.Text)
		panels.Add(label)
	}

	return panels
}
