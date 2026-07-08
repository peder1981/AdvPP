package mvc

import (
	"fmt"
	"strings"
)

// FWFormModel represents the Model in MVC pattern
type FWFormModel struct {
	Name        string
	Fields      []*FieldDef
	Validations map[string][]ValidationRule
	PrimaryKey  []string
}

type FieldDef struct {
	Name     string
	Type     string
	Size     int
	Decimal  int
	Required bool
	Editable bool
	Default  interface{}
	Help     string
	When     string
	Valid    string
	Init     string
	Picture  string
}

type ValidationRule struct {
	Name    string
	Message string
	Handler func(interface{}) bool
}

// NewFWFormModel creates a new FormModel
func NewFWFormModel(name string) *FWFormModel {
	return &FWFormModel{
		Name:        name,
		Fields:      make([]*FieldDef, 0),
		Validations: make(map[string][]ValidationRule),
		PrimaryKey:  make([]string, 0),
	}
}

// AddField adds a field definition to the model
func (m *FWFormModel) AddField(field *FieldDef) {
	m.Fields = append(m.Fields, field)
}

// AddValidation adds a validation rule for a field
func (m *FWFormModel) AddValidation(fieldName string, rule ValidationRule) {
	if m.Validations[fieldName] == nil {
		m.Validations[fieldName] = make([]ValidationRule, 0)
	}
	m.Validations[fieldName] = append(m.Validations[fieldName], rule)
}

// Validate validates all fields in the model
func (m *FWFormModel) Validate(data map[string]interface{}) error {
	for _, field := range m.Fields {
		if field.Required {
			if _, exists := data[field.Name]; !exists {
				return fmt.Errorf("field %s is required", field.Name)
			}
		}

		if rules, ok := m.Validations[field.Name]; ok {
			for _, rule := range rules {
				if value, exists := data[field.Name]; exists {
					if !rule.Handler(value) {
						return fmt.Errorf("validation failed for %s: %s", field.Name, rule.Message)
					}
				}
			}
		}
	}
	return nil
}

// GetField returns a field definition by name
func (m *FWFormModel) GetField(name string) *FieldDef {
	for _, field := range m.Fields {
		if strings.EqualFold(field.Name, name) {
			return field
		}
	}
	return nil
}

// SetPrimaryKey sets the primary key fields
func (m *FWFormModel) SetPrimaryKey(keys ...string) {
	m.PrimaryKey = keys
}

// AddRequiredValidation adds a required field validation
func (m *FWFormModel) AddRequiredValidation(fieldName string) {
	m.AddValidation(fieldName, ValidationRule{
		Name:    "required",
		Message: fmt.Sprintf("%s is required", fieldName),
		Handler: func(val interface{}) bool {
			return val != nil
		},
	})
}

// AddLengthValidation adds a length validation for string fields
func (m *FWFormModel) AddLengthValidation(fieldName string, min, max int) {
	m.AddValidation(fieldName, ValidationRule{
		Name:    "length",
		Message: fmt.Sprintf("%s must be between %d and %d characters", fieldName, min, max),
		Handler: func(val interface{}) bool {
			if s, ok := val.(string); ok {
				return len(s) >= min && len(s) <= max
			}
			return false
		},
	})
}

// AddRangeValidation adds a range validation for numeric fields
func (m *FWFormModel) AddRangeValidation(fieldName string, min, max float64) {
	m.AddValidation(fieldName, ValidationRule{
		Name:    "range",
		Message: fmt.Sprintf("%s must be between %g and %g", fieldName, min, max),
		Handler: func(val interface{}) bool {
			if n, ok := val.(float64); ok {
				return n >= min && n <= max
			}
			return false
		},
	})
}

// AddCustomValidation adds a custom validation rule
func (m *FWFormModel) AddCustomValidation(fieldName, message string, handler func(interface{}) bool) {
	m.AddValidation(fieldName, ValidationRule{
		Name:    "custom",
		Message: message,
		Handler: handler,
	})
}
