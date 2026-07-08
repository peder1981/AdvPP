package mvc

import (
	"fmt"
)

// FWFormBrowse represents a grid/browse component in MVC pattern
type FWFormBrowse struct {
	Name        string
	Model       *FWFormModel
	Title       string
	Width       int
	Height      int
	Columns     []*BrowseColumn
	Fields      []*BrowseField
	Alias       string
	Order       string
	Filter      string
	ReadOnly    bool
	AllowAdd    bool
	AllowEdit   bool
	AllowDelete bool
	LineNumbers bool
	MarkColumn  bool
	Events      map[string]EventHandler
}

type BrowseColumn struct {
	Name     string
	Title    string
	Width    int
	Align    string // "LEFT", "CENTER", "RIGHT"
	Picture  string
	Order    bool
	Visible  bool
	Editable bool
}

type BrowseField struct {
	Name   string
	Column string
	Order  int
}

// NewFWFormBrowse creates a new FormBrowse
func NewFWFormBrowse(name string, model *FWFormModel) *FWFormBrowse {
	return &FWFormBrowse{
		Name:        name,
		Model:       model,
		Width:       800,
		Height:      400,
		Columns:     make([]*BrowseColumn, 0),
		Fields:      make([]*BrowseField, 0),
		ReadOnly:    false,
		AllowAdd:    true,
		AllowEdit:   true,
		AllowDelete: true,
		LineNumbers: true,
		MarkColumn:  true,
		Events:      make(map[string]EventHandler),
	}
}

// AddColumn adds a column to the browse
func (b *FWFormBrowse) AddColumn(col *BrowseColumn) {
	b.Columns = append(b.Columns, col)
}

// AddField adds a field to the browse
func (b *FWFormBrowse) AddField(field *BrowseField) {
	b.Fields = append(b.Fields, field)
}

// SetAlias sets the table alias
func (b *FWFormBrowse) SetAlias(alias string) {
	b.Alias = alias
}

// SetOrder sets the order clause
func (b *FWFormBrowse) SetOrder(order string) {
	b.Order = order
}

// SetFilter sets the filter clause
func (b *FWFormBrowse) SetFilter(filter string) {
	b.Filter = filter
}

// SetTitle sets the browse title
func (b *FWFormBrowse) SetTitle(title string) {
	b.Title = title
}

// SetSize sets the browse dimensions
func (b *FWFormBrowse) SetSize(width, height int) {
	b.Width = width
	b.Height = height
}

// SetReadOnly sets the browse as read-only
func (b *FWFormBrowse) SetReadOnly(readOnly bool) {
	b.ReadOnly = readOnly
}

// SetPermissions sets CRUD permissions
func (b *FWFormBrowse) SetPermissions(add, edit, delete bool) {
	b.AllowAdd = add
	b.AllowEdit = edit
	b.AllowDelete = delete
}

// AddEvent adds an event handler
func (b *FWFormBrowse) AddEvent(eventName string, handler EventHandler) {
	b.Events[eventName] = handler
}

// GetColumn returns a column by name
func (b *FWFormBrowse) GetColumn(name string) *BrowseColumn {
	for _, col := range b.Columns {
		if col.Name == name {
			return col
		}
	}
	return nil
}

// Validate validates the browse configuration
func (b *FWFormBrowse) Validate() error {
	if len(b.Columns) == 0 {
		return fmt.Errorf("browse must have at least one column")
	}
	if b.Alias == "" {
		return fmt.Errorf("browse must have an alias")
	}
	return nil
}

// TriggerEvent triggers an event handler
func (b *FWFormBrowse) TriggerEvent(eventName string, context map[string]interface{}) error {
	if handler, ok := b.Events[eventName]; ok {
		return handler.Handler(b, context)
	}
	return nil
}

// AddOnLineChange adds an onLineChange event handler
func (b *FWFormBrowse) AddOnLineChange(handler EventHandler) {
	b.Events["onLineChange"] = handler
}

// AddOnDbClick adds an onDbClick event handler
func (b *FWFormBrowse) AddOnDbClick(handler EventHandler) {
	b.Events["onDbClick"] = handler
}

// AddOnHeaderClick adds an onHeaderClick event handler
func (b *FWFormBrowse) AddOnHeaderClick(columnName string, handler EventHandler) {
	b.Events[fmt.Sprintf("onHeaderClick_%s", columnName)] = handler
}
