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

// FWWizardControl represents a wizard/step-by-step control
type FWWizardControl struct {
	Name         string
	Title        string
	Steps        []*WizardStep
	CurrentStep  int
	Width        int
	Height       int
	ShowCancel   bool
	ShowHelp     bool
	FinishAction string
	CancelAction string
	HelpAction   string
}

// WizardStep represents a single step in the wizard
type WizardStep struct {
	Name        string
	Title       string
	Description string
	View        *FWFormView
	Validation  string
	NextAction  string
	PrevAction  string
}

// NewFWWizardControl creates a new WizardControl
func NewFWWizardControl(name string, title string) *FWWizardControl {
	return &FWWizardControl{
		Name:        name,
		Title:       title,
		Steps:       make([]*WizardStep, 0),
		CurrentStep: 0,
		Width:       600,
		Height:      450,
		ShowCancel:  true,
		ShowHelp:    false,
	}
}

// AddStep adds a step to the wizard
func (w *FWWizardControl) AddStep(step *WizardStep) {
	w.Steps = append(w.Steps, step)
}

// GetCurrentStep returns the current wizard step
func (w *FWWizardControl) GetCurrentStep() *WizardStep {
	if w.CurrentStep >= 0 && w.CurrentStep < len(w.Steps) {
		return w.Steps[w.CurrentStep]
	}
	return nil
}

// NextStep moves to the next step
func (w *FWWizardControl) NextStep() bool {
	if w.CurrentStep < len(w.Steps)-1 {
		w.CurrentStep++
		return true
	}
	return false
}

// PreviousStep moves to the previous step
func (w *FWWizardControl) PreviousStep() bool {
	if w.CurrentStep > 0 {
		w.CurrentStep--
		return true
	}
	return false
}

// CanMoveNext checks if can move to next step
func (w *FWWizardControl) CanMoveNext() bool {
	return w.CurrentStep < len(w.Steps)-1
}

// CanMovePrevious checks if can move to previous step
func (w *FWWizardControl) CanMovePrevious() bool {
	return w.CurrentStep > 0
}

// IsFirstStep checks if current step is the first
func (w *FWWizardControl) IsFirstStep() bool {
	return w.CurrentStep == 0
}

// IsLastStep checks if current step is the last
func (w *FWWizardControl) IsLastStep() bool {
	return w.CurrentStep == len(w.Steps)-1
}

// SetStep sets the current step by index
func (w *FWWizardControl) SetStep(stepIndex int) error {
	if stepIndex < 0 || stepIndex >= len(w.Steps) {
		return fmt.Errorf("invalid step index: %d", stepIndex)
	}
	w.CurrentStep = stepIndex
	return nil
}

// GetStepCount returns the total number of steps
func (w *FWWizardControl) GetStepCount() int {
	return len(w.Steps)
}

// ValidateCurrentStep validates the current step
func (w *FWWizardControl) ValidateCurrentStep() error {
	currentStep := w.GetCurrentStep()
	if currentStep == nil {
		return fmt.Errorf("no current step")
	}
	if currentStep.View != nil {
		return currentStep.View.Validate()
	}
	return nil
}

// FWDynDialog represents a dynamic dialog
type FWDynDialog struct {
	Name       string
	Title      string
	Width      int
	Height     int
	Components []*Component
	Buttons    []*Component
	Modal      bool
	Resizable  bool
	Position   string // "CENTER", "TOP", "BOTTOM", etc.
}

// NewFWDynDialog creates a new dynamic dialog
func NewFWDynDialog(name string, title string) *FWDynDialog {
	return &FWDynDialog{
		Name:       name,
		Title:      title,
		Width:      400,
		Height:     300,
		Components: make([]*Component, 0),
		Buttons:    make([]*Component, 0),
		Modal:      true,
		Resizable:  false,
		Position:   "CENTER",
	}
}

// AddComponent adds a component to the dialog
func (d *FWDynDialog) AddComponent(comp *Component) {
	d.Components = append(d.Components, comp)
}

// AddButton adds a button to the dialog
func (d *FWDynDialog) AddButton(button *Component) {
	d.Buttons = append(d.Buttons, button)
}

// FWPanel represents a panel container
type FWPanel struct {
	Name        string
	Components  []*Component
	Width       int
	Height      int
	BackColor   string
	BorderStyle string
}

// NewFWPanel creates a new panel
func NewFWPanel(name string) *FWPanel {
	return &FWPanel{
		Name:       name,
		Components: make([]*Component, 0),
		Width:      200,
		Height:     150,
	}
}

// AddComponent adds a component to the panel
func (p *FWPanel) AddComponent(comp *Component) {
	p.Components = append(p.Components, comp)
}

// FWGroupBox represents a group box with title
type FWGroupBox struct {
	Name       string
	Title      string
	Components []*Component
	Width      int
	Height     int
	BackColor  string
}

// NewFWGroupBox creates a new group box
func NewFWGroupBox(name string, title string) *FWGroupBox {
	return &FWGroupBox{
		Name:       name,
		Title:      title,
		Components: make([]*Component, 0),
		Width:      200,
		Height:     150,
	}
}

// AddComponent adds a component to the group box
func (g *FWGroupBox) AddComponent(comp *Component) {
	g.Components = append(g.Components, comp)
}

// FWTabs represents a tab control
type FWTabs struct {
	Name      string
	TabPages  []*TabPage
	ActiveTab int
	Width     int
	Height    int
	Position  string // "TOP", "BOTTOM", "LEFT", "RIGHT"
}

// TabPage represents a single tab page
type TabPage struct {
	Name       string
	Title      string
	Components []*Component
	View       *FWFormView
}

// NewFWTabs creates a new tab control
func NewFWTabs(name string) *FWTabs {
	return &FWTabs{
		Name:      name,
		TabPages:  make([]*TabPage, 0),
		ActiveTab: 0,
		Width:     400,
		Height:    300,
		Position:  "TOP",
	}
}

// AddTabPage adds a tab page to the tabs control
func (t *FWTabs) AddTabPage(tab *TabPage) {
	t.TabPages = append(t.TabPages, tab)
}

// GetActiveTab returns the active tab page
func (t *FWTabs) GetActiveTab() *TabPage {
	if t.ActiveTab >= 0 && t.ActiveTab < len(t.TabPages) {
		return t.TabPages[t.ActiveTab]
	}
	return nil
}

// SetActiveTab sets the active tab by index
func (t *FWTabs) SetActiveTab(tabIndex int) error {
	if tabIndex < 0 || tabIndex >= len(t.TabPages) {
		return fmt.Errorf("invalid tab index: %d", tabIndex)
	}
	t.ActiveTab = tabIndex
	return nil
}

// FWSplitter represents a splitter control
type FWSplitter struct {
	Name        string
	Orientation string // "HORIZONTAL", "VERTICAL"
	Panel1      *FWPanel
	Panel2      *FWPanel
	SplitterPos int
	Width       int
	Height      int
	Resizable   bool
}

// NewFWSplitter creates a new splitter
func NewFWSplitter(name string, orientation string) *FWSplitter {
	return &FWSplitter{
		Name:        name,
		Orientation: orientation,
		SplitterPos: 50,
		Width:       400,
		Height:      300,
		Resizable:   true,
	}
}

// FWTreeView represents a tree view control
type FWTreeView struct {
	Name        string
	Nodes       []*TreeNode
	Width       int
	Height      int
	ShowLines   bool
	ShowButtons bool
}

// TreeNode represents a tree node
type TreeNode struct {
	Name     string
	Text     string
	Children []*TreeNode
	Expanded bool
	Data     interface{}
}

// NewFWTreeView creates a new tree view
func NewFWTreeView(name string) *FWTreeView {
	return &FWTreeView{
		Name:        name,
		Nodes:       make([]*TreeNode, 0),
		Width:       200,
		Height:      300,
		ShowLines:   true,
		ShowButtons: true,
	}
}

// AddNode adds a node to the tree view
func (t *FWTreeView) AddNode(node *TreeNode) {
	t.Nodes = append(t.Nodes, node)
}

// FWListView represents a list view control
type FWListView struct {
	Name      string
	Columns   []*ListViewColumn
	Items     []*ListViewItem
	Width     int
	Height    int
	ViewStyle string // "REPORT", "ICON", "SMALLICON", "LIST"
}

// ListViewColumn represents a list view column
type ListViewColumn struct {
	Name  string
	Title string
	Width int
	Align string
}

// ListViewItem represents a list view item
type ListViewItem struct {
	Name   string
	Values []string
	Data   interface{}
}

// NewFWListView creates a new list view
func NewFWListView(name string) *FWListView {
	return &FWListView{
		Name:      name,
		Columns:   make([]*ListViewColumn, 0),
		Items:     make([]*ListViewItem, 0),
		Width:     400,
		Height:    300,
		ViewStyle: "REPORT",
	}
}

// AddColumn adds a column to the list view
func (l *FWListView) AddColumn(column *ListViewColumn) {
	l.Columns = append(l.Columns, column)
}

// AddItem adds an item to the list view
func (l *FWListView) AddItem(item *ListViewItem) {
	l.Items = append(l.Items, item)
}
