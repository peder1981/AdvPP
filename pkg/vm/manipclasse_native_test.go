package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestGetClassName(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a class definition for testing
	classDef := &advplrt.ClassDef{
		Name:       "TestClass",
		Properties: make(map[string]string),
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["TESTCLASS"] = classDef

	// Create an object instance of the class
	obj := advplrt.NewObject("TestClass", classDef)

	// Test GetClassName
	got, err := v.natives["GETCLASSNAME"].Fn([]advplrt.Value{obj})
	if err != nil {
		t.Fatalf("GetClassName retornou erro: %v", err)
	}

	str, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Errorf("GetClassName() retornou tipo %T, quer *StringValue", got)
	}
	if str.Val != "TestClass" {
		t.Errorf("GetClassName() = %q, quer %q", str.Val, "TestClass")
	}
}

func TestFindClass(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Register a class
	classDef := &advplrt.ClassDef{
		Name:       "MyClass",
		Properties: make(map[string]string),
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["MYCLASS"] = classDef

	cases := []struct {
		name        string
		className   string
		shouldExist bool
	}{
		{"classe existente", "MyClass", true},
		{"classe inexistente", "NonExistent", false},
	}

	for _, c := range cases {
		got, err := v.natives["FINDCLASS"].Fn([]advplrt.Value{advplrt.NewString(c.className)})
		if err != nil {
			t.Fatalf("FindClass(%s) retornou erro: %v", c.className, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok {
			t.Errorf("FindClass(%s) retornou tipo %T, quer *BoolValue", c.className, got)
		}
		if b.Val != c.shouldExist {
			t.Errorf("FindClass(%s) = %v, quer %v", c.className, b.Val, c.shouldExist)
		}
	}
}

func TestAttIsMemberOf(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a class with properties
	classDef := &advplrt.ClassDef{
		Name:       "TestClass",
		Properties: map[string]string{"fcProp": "C", "fnNumber": "N", "flFlag": "L"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["TESTCLASS"] = classDef

	// Create an object instance
	obj := advplrt.NewObject("TestClass", classDef)
	obj.SetProp("fcProp", advplrt.NewString("test"))
	obj.SetProp("fnNumber", advplrt.NewNumber(42))

	cases := []struct {
		name      string
		propName  string
		recursive bool
		expected  bool
	}{
		{"propriedade existente", "fcProp", false, true},
		{"propriedade inexistente", "nonexistent", false, false},
		{"case insensitive", "FCPROP", false, true},
		{"numeric property", "fnNumber", false, true},
		{"logical property", "flFlag", false, true},
	}

	for _, c := range cases {
		args := []advplrt.Value{
			obj,
			advplrt.NewString(c.propName),
		}
		if c.recursive {
			args = append(args, advplrt.NewBool(true))
		}

		got, err := v.natives["ATTISMEMBEROF"].Fn(args)
		if err != nil {
			t.Fatalf("AttIsMemberOf(%s, %q) retornou erro: %v", c.name, c.propName, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok {
			t.Errorf("AttIsMemberOf(%s) retornou tipo %T, quer *BoolValue", c.name, got)
		}
		if b.Val != c.expected {
			t.Errorf("AttIsMemberOf(%s, %q) = %v, quer %v", c.name, c.propName, b.Val, c.expected)
		}
	}
}

func TestMethIsMemberOf(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a class with methods
	classDef := &advplrt.ClassDef{
		Name:       "TestClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {Name: "new", ClassName: "TestClass"},
			"SHOW": {Name: "show", ClassName: "TestClass"},
			"VALIDATE": {Name: "validate", ClassName: "TestClass"},
		},
	}
	v.classes["TESTCLASS"] = classDef

	// Create an object instance
	obj := advplrt.NewObject("TestClass", classDef)

	cases := []struct {
		name      string
		methName  string
		recursive bool
		expected  bool
	}{
		{"método existente", "new", false, true},
		{"método inexistente", "nonexistent", false, false},
		{"case insensitive", "NEW", false, true},
		{"show method", "show", false, true},
		{"validate method", "validate", false, true},
	}

	for _, c := range cases {
		args := []advplrt.Value{
			obj,
			advplrt.NewString(c.methName),
		}
		if c.recursive {
			args = append(args, advplrt.NewBool(true))
		}

		got, err := v.natives["METHISMEMBEROF"].Fn(args)
		if err != nil {
			t.Fatalf("MethIsMemberOf(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok {
			t.Errorf("MethIsMemberOf(%s) retornou tipo %T, quer *BoolValue", c.name, got)
		}
		if b.Val != c.expected {
			t.Errorf("MethIsMemberOf(%s, %q) = %v, quer %v", c.name, c.methName, b.Val, c.expected)
		}
	}
}

func TestClassDataArr(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a class with properties
	classDef := &advplrt.ClassDef{
		Name:       "TestClass",
		Properties: map[string]string{"fcProp": "C", "fnNumber": "N"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["TESTCLASS"] = classDef

	// Create an object instance with some property values
	obj := advplrt.NewObject("TestClass", classDef)
	obj.SetProp("fcProp", advplrt.NewString("hello"))
	obj.SetProp("fnNumber", advplrt.NewNumber(123))

	// Test without parent
	got, err := v.natives["CLASSDATAARR"].Fn([]advplrt.Value{obj})
	if err != nil {
		t.Fatalf("ClassDataArr retornou erro: %v", err)
	}

	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Errorf("ClassDataArr() retornou tipo %T, quer *ArrayValue", got)
		return
	}

	// Should have 2 entries (one per property)
	if len(arr.Elements) != 2 {
		t.Errorf("ClassDataArr() retornou %d elementos, quer 2", len(arr.Elements))
	}

	// Each entry should be an array [name, value, id]
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}
		// Minimum 2 columns: name, value
		if len(row.Elements) < 2 {
			t.Errorf("ClassDataArr()[%d] tem %d colunas, quer pelo menos 2", i, len(row.Elements))
		}
	}
}

func TestClassMethArr(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a class with methods
	classDef := &advplrt.ClassDef{
		Name:       "TestClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"NEW": {
				Name:      "new",
				ClassName: "TestClass",
				Params: []*advplrt.ParamDef{
					{Name: "cParam1", Type: "C"},
					{Name: "nParam2", Type: "N"},
				},
			},
			"SHOW": {
				Name:      "show",
				ClassName: "TestClass",
				Params:    []*advplrt.ParamDef{},
			},
		},
	}
	v.classes["TESTCLASS"] = classDef

	// Create an object instance
	obj := advplrt.NewObject("TestClass", classDef)

	// Test without parent
	got, err := v.natives["CLASSMETHARR"].Fn([]advplrt.Value{obj})
	if err != nil {
		t.Fatalf("ClassMethArr retornou erro: %v", err)
	}

	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Errorf("ClassMethArr() retornou tipo %T, quer *ArrayValue", got)
		return
	}

	// Should have 2 entries (one per method)
	if len(arr.Elements) != 2 {
		t.Errorf("ClassMethArr() retornou %d elementos, quer 2", len(arr.Elements))
	}

	// Each entry should be an array [name, paramsArray]
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}
		// Minimum 2 columns: name, params array
		if len(row.Elements) < 2 {
			t.Errorf("ClassMethArr()[%d] tem %d colunas, quer pelo menos 2", i, len(row.Elements))
		}
		// Second column should be an array (params)
		_, ok = row.Elements[1].(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d][1] (params) é tipo %T, quer *ArrayValue", i, row.Elements[1])
		}
	}
}
