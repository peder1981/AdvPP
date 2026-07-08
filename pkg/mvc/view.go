package mvc

import (
	"fmt"
)

// FWFormView represents the View in MVC pattern
type FWFormView struct {
	Name       string
	Model      *FWFormModel
	Title      string
	Width      int
	Height     int
	Components []*Component
	Dialogs    []*Dialog
	MenuBar    *MenuBar
	ToolBar    *ToolBar
	StatusBar  *StatusBar
	Events     map[string]EventHandler
}

type Component struct {
	Name       string
	Type       string // "TButton", "TGet", "TComboBox", "TCheckBox", etc.
	X          int
	Y          int
	Width      int
	Height     int
	FieldName  string
	Label      string
	Value      interface{}
	Enabled    bool
	Visible    bool
	ReadOnly   bool
	Picture    string
	When       string
	Valid      string
	Help       string
	Options    []string // For TComboBox, TListBox
	Style      string
	FontName   string
	FontSize   int
	FontBold   bool
	FontItalic bool
	BackColor  string
	ForeColor  string
}

type Dialog struct {
	Name       string
	Title      string
	Width      int
	Height     int
	Components []*Component
	Buttons    []*Component
}

type MenuBar struct {
	Items []*MenuItem
}

type MenuItem struct {
	Name      string
	Label     string
	Action    string
	Shortcut  string
	Items     []*MenuItem // For submenus
	Separator bool
}

type ToolBar struct {
	Buttons []*ToolButton
}

type ToolButton struct {
	Name    string
	Action  string
	Icon    string
	Tooltip string
}

type StatusBar struct {
	Panels []*StatusPanel
}

type StatusPanel struct {
	Name  string
	Text  string
	Width int
}

// NewFWFormView creates a new FormView
func NewFWFormView(name string, model *FWFormModel) *FWFormView {
	return &FWFormView{
		Name:       name,
		Model:      model,
		Width:      800,
		Height:     600,
		Components: make([]*Component, 0),
		Dialogs:    make([]*Dialog, 0),
		Events:     make(map[string]EventHandler),
	}
}

// AddComponent adds a component to the view
func (v *FWFormView) AddComponent(comp *Component) {
	v.Components = append(v.Components, comp)
}

// AddDialog adds a dialog to the view
func (v *FWFormView) AddDialog(dialog *Dialog) {
	v.Dialogs = append(v.Dialogs, dialog)
}

// SetTitle sets the view title
func (v *FWFormView) SetTitle(title string) {
	v.Title = title
}

// SetSize sets the view dimensions
func (v *FWFormView) SetSize(width, height int) {
	v.Width = width
	v.Height = height
}

// AddEvent adds an event handler
func (v *FWFormView) AddEvent(eventName string, handler EventHandler) {
	v.Events[eventName] = handler
}

// GetComponent returns a component by name
func (v *FWFormView) GetComponent(name string) *Component {
	for _, comp := range v.Components {
		if comp.Name == name {
			return comp
		}
	}
	return nil
}

// SetValue sets a component value
func (v *FWFormView) SetValue(compName string, value interface{}) error {
	comp := v.GetComponent(compName)
	if comp == nil {
		return fmt.Errorf("component %s not found", compName)
	}
	comp.Value = value
	return nil
}

// GetValue gets a component value
func (v *FWFormView) GetValue(compName string) (interface{}, error) {
	comp := v.GetComponent(compName)
	if comp == nil {
		return nil, fmt.Errorf("component %s not found", compName)
	}
	return comp.Value, nil
}

// Validate validates all components in the view
func (v *FWFormView) Validate() error {
	data := make(map[string]interface{})
	for _, comp := range v.Components {
		if comp.FieldName != "" {
			data[comp.FieldName] = comp.Value
		}
	}
	return v.Model.Validate(data)
}

// TriggerEvent triggers an event handler
func (v *FWFormView) TriggerEvent(eventName string, context map[string]interface{}) error {
	if handler, ok := v.Events[eventName]; ok {
		return handler.Handler(v, context)
	}
	return nil
}

// AddOnChange adds an onChange event handler
func (v *FWFormView) AddOnChange(compName string, handler EventHandler) {
	v.Events[fmt.Sprintf("onChange_%s", compName)] = handler
}

// AddOnClick adds an onClick event handler
func (v *FWFormView) AddOnClick(compName string, handler EventHandler) {
	v.Events[fmt.Sprintf("onClick_%s", compName)] = handler
}

// AddOnFocus adds an onGotFocus event handler
func (v *FWFormView) AddOnFocus(compName string, handler EventHandler) {
	v.Events[fmt.Sprintf("onGotFocus_%s", compName)] = handler
}

// AddOnBlur adds an onLostFocus event handler
func (v *FWFormView) AddOnBlur(compName string, handler EventHandler) {
	v.Events[fmt.Sprintf("onLostFocus_%s", compName)] = handler
}
