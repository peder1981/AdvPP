package advplrt

import "fmt"

type Environment struct {
	variables  map[string]Value
	parent     *Environment
	scope      string
	isFunction bool
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		variables:  make(map[string]Value),
		parent:     parent,
		scope:      "local",
		isFunction: false,
	}
}

func NewFunctionEnv() *Environment {
	return &Environment{
		variables:  make(map[string]Value),
		parent:     nil,
		scope:      "local",
		isFunction: true,
	}
}

func (e *Environment) Define(name string, value Value) {
	e.variables[normalizeName(name)] = value
}

func (e *Environment) Get(name string) (Value, error) {
	key := normalizeName(name)
	if val, ok := e.variables[key]; ok {
		return val, nil
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, fmt.Errorf("variable %s does not exist", name)
}

func (e *Environment) Set(name string, value Value) error {
	key := normalizeName(name)
	if _, ok := e.variables[key]; ok {
		e.variables[key] = value
		return nil
	}
	if e.parent != nil {
		return e.parent.Set(name, value)
	}
	e.variables[key] = value
	return nil
}

func (e *Environment) Has(name string) bool {
	key := normalizeName(name)
	if _, ok := e.variables[key]; ok {
		return true
	}
	if e.parent != nil {
		return e.parent.Has(name)
	}
	return false
}

func (e *Environment) Parent() *Environment { return e.parent }

func normalizeName(name string) string { return upper(name) }

func upper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		result[i] = c
	}
	return string(result)
}
