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

func TestAttIsMemberOfWithInheritance(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create parent class with properties
	parentDef := &advplrt.ClassDef{
		Name:       "ParentClass",
		Properties: map[string]string{"fcParentProp": "C", "fnParentNum": "N"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["PARENTCLASS"] = parentDef

	// Create child class with parent reference
	childDef := &advplrt.ClassDef{
		Name:       "ChildClass",
		Parent:     "ParentClass",
		Properties: map[string]string{"fcChildProp": "C"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["CHILDCLASS"] = childDef

	// Create object of child class
	obj := advplrt.NewObject("ChildClass", childDef)

	cases := []struct {
		name      string
		propName  string
		recursive bool
		expected  bool
	}{
		// Own properties always found
		{"propriedade propria SEM recursive", "fcChildProp", false, true},
		{"propriedade propria COM recursive", "fcChildProp", true, true},

		// Parent properties only found with recursive=.T.
		{"propriedade pai SEM recursive", "fcParentProp", false, false},
		{"propriedade pai COM recursive", "fcParentProp", true, true},
		{"propriedade pai numero COM recursive", "fnParentNum", true, true},

		// Non-existent properties never found
		{"propriedade inexistente SEM recursive", "nonexistent", false, false},
		{"propriedade inexistente COM recursive", "nonexistent", true, false},
	}

	for _, c := range cases {
		args := []advplrt.Value{
			obj,
			advplrt.NewString(c.propName),
			advplrt.NewBool(c.recursive),
		}

		got, err := v.natives["ATTISMEMBEROF"].Fn(args)
		if err != nil {
			t.Fatalf("AttIsMemberOf(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok {
			t.Errorf("AttIsMemberOf(%s) retornou tipo %T, quer *BoolValue", c.name, got)
		}
		if b.Val != c.expected {
			t.Errorf("AttIsMemberOf(%s, recursive=%v) = %v, quer %v", c.name, c.recursive, b.Val, c.expected)
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

func TestMethIsMemberOfWithInheritance(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create parent class with methods
	parentDef := &advplrt.ClassDef{
		Name:       "ParentClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"PARENTMETH": {Name: "parentMeth", ClassName: "ParentClass"},
			"SHARED": {Name: "shared", ClassName: "ParentClass"},
		},
	}
	v.classes["PARENTCLASS"] = parentDef

	// Create child class with parent reference
	childDef := &advplrt.ClassDef{
		Name:       "ChildClass",
		Parent:     "ParentClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"CHILDMETH": {Name: "childMeth", ClassName: "ChildClass"},
			"SHARED": {Name: "shared", ClassName: "ChildClass"},
		},
	}
	v.classes["CHILDCLASS"] = childDef

	// Create object of child class
	obj := advplrt.NewObject("ChildClass", childDef)

	cases := []struct {
		name      string
		methName  string
		recursive bool
		expected  bool
	}{
		// Own methods always found
		{"método próprio SEM recursive", "childMeth", false, true},
		{"método próprio COM recursive", "childMeth", true, true},

		// Parent methods only found with recursive=.T.
		{"método pai SEM recursive", "parentMeth", false, false},
		{"método pai COM recursive", "parentMeth", true, true},

		// Non-existent methods never found
		{"método inexistente SEM recursive", "nonexistent", false, false},
		{"método inexistente COM recursive", "nonexistent", true, false},
	}

	for _, c := range cases {
		args := []advplrt.Value{
			obj,
			advplrt.NewString(c.methName),
			advplrt.NewBool(c.recursive),
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
			t.Errorf("MethIsMemberOf(%s, recursive=%v) = %v, quer %v", c.name, c.recursive, b.Val, c.expected)
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

	// Validate structure and content of each row
	propsSeen := make(map[string]bool)
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}

		// Must have at least 3 columns: name, value, id (classname only if lParent=.T.)
		if len(row.Elements) < 3 {
			t.Errorf("ClassDataArr()[%d] tem %d colunas, quer pelo menos 3", i, len(row.Elements))
			continue
		}

		// Verify column 0: property name is a string
		nameVal, ok := row.Elements[0].(*advplrt.StringValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d][0] (name) é tipo %T, quer *StringValue", i, row.Elements[0])
			continue
		}

		// Verify column 1: property value is correct
		propName := nameVal.Val
		expectedVal := obj.Props[propName]
		if expectedVal == nil {
			t.Errorf("ClassDataArr()[%d][1] (value) era nil, mas esperava valor para %q", i, propName)
			continue
		}
		if !row.Elements[1].Equals(expectedVal) {
			t.Errorf("ClassDataArr()[%d][1] (value) = %v, quer %v", i, row.Elements[1], expectedVal)
		}

		// Verify column 2: id is a number
		_, ok = row.Elements[2].(*advplrt.NumberValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d][2] (id) é tipo %T, quer *NumberValue", i, row.Elements[2])
		}

		// Without lParent, should not have column 3 (classname)
		if len(row.Elements) > 3 {
			t.Errorf("ClassDataArr()[%d] tem %d colunas, quer exatamente 3 sem lParent", i, len(row.Elements))
		}

		propsSeen[propName] = true
	}

	// Verify all expected properties were returned
	if !propsSeen["fcProp"] {
		t.Errorf("ClassDataArr() não incluiu fcProp")
	}
	if !propsSeen["fnNumber"] {
		t.Errorf("ClassDataArr() não incluiu fnNumber")
	}
}

func TestClassDataArrWithInheritance(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create parent class
	parentDef := &advplrt.ClassDef{
		Name:       "ParentClass",
		Properties: map[string]string{"fcParentProp": "C"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["PARENTCLASS"] = parentDef

	// Create child class
	childDef := &advplrt.ClassDef{
		Name:       "ChildClass",
		Parent:     "ParentClass",
		Properties: map[string]string{"fcChildProp": "C"},
		Methods:    make(map[string]*advplrt.MethodDef),
	}
	v.classes["CHILDCLASS"] = childDef

	// Create object with property values
	obj := advplrt.NewObject("ChildClass", childDef)
	obj.SetProp("fcChildProp", advplrt.NewString("child"))
	obj.SetProp("fcParentProp", advplrt.NewString("parent"))

	// Test with lParent=.T.
	got, err := v.natives["CLASSDATAARR"].Fn([]advplrt.Value{obj, advplrt.NewBool(true)})
	if err != nil {
		t.Fatalf("ClassDataArr(lParent=.T.) retornou erro: %v", err)
	}

	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Errorf("ClassDataArr() retornou tipo %T, quer *ArrayValue", got)
		return
	}

	// Should have 2 entries (1 child + 1 parent property)
	if len(arr.Elements) != 2 {
		t.Errorf("ClassDataArr(lParent=.T.) retornou %d elementos, quer 2", len(arr.Elements))
	}

	// Verify rows include classname in column 3
	classesSeen := make(map[string]bool)
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}

		// Must have 4 columns with lParent=.T.: name, value, id, classname
		if len(row.Elements) != 4 {
			t.Errorf("ClassDataArr()[%d] tem %d colunas, quer exatamente 4 com lParent=.T.", i, len(row.Elements))
			continue
		}

		// Column 3: classname should be a string
		classnameVal, ok := row.Elements[3].(*advplrt.StringValue)
		if !ok {
			t.Errorf("ClassDataArr()[%d][3] (classname) é tipo %T, quer *StringValue", i, row.Elements[3])
			continue
		}

		classesSeen[classnameVal.Val] = true
	}

	// Verify we see both parent and child class names
	if !classesSeen["ChildClass"] {
		t.Errorf("ClassDataArr(lParent=.T.) não incluiu propriedades da ChildClass")
	}
	if !classesSeen["ParentClass"] {
		t.Errorf("ClassDataArr(lParent=.T.) não incluiu propriedades da ParentClass")
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

	// Validate structure and content of each row
	methsSeen := make(map[string]bool)
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}

		// Must have at least 2 columns: name, params (classname only if lParent=.T.)
		if len(row.Elements) < 2 {
			t.Errorf("ClassMethArr()[%d] tem %d colunas, quer pelo menos 2", i, len(row.Elements))
			continue
		}

		// Verify column 0: method name is a string
		nameVal, ok := row.Elements[0].(*advplrt.StringValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d][0] (name) é tipo %T, quer *StringValue", i, row.Elements[0])
			continue
		}

		// Verify column 1: params is an array
		paramsArr, ok := row.Elements[1].(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d][1] (params) é tipo %T, quer *ArrayValue", i, row.Elements[1])
			continue
		}

		// Validate param contents
		methName := nameVal.Val
		if methName == "new" && len(paramsArr.Elements) != 2 {
			t.Errorf("ClassMethArr()[%d] method 'new' tem %d parâmetros, quer 2", i, len(paramsArr.Elements))
		}
		if methName == "show" && len(paramsArr.Elements) != 0 {
			t.Errorf("ClassMethArr()[%d] method 'show' tem %d parâmetros, quer 0", i, len(paramsArr.Elements))
		}

		// Without lParent, should not have column 2 (classname)
		if len(row.Elements) > 2 {
			t.Errorf("ClassMethArr()[%d] tem %d colunas, quer exatamente 2 sem lParent", i, len(row.Elements))
		}

		methsSeen[methName] = true
	}

	// Verify all expected methods were returned
	if !methsSeen["new"] {
		t.Errorf("ClassMethArr() não incluiu método 'new'")
	}
	if !methsSeen["show"] {
		t.Errorf("ClassMethArr() não incluiu método 'show'")
	}
}

func TestClassMethArrWithInheritance(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create parent class with methods
	parentDef := &advplrt.ClassDef{
		Name:       "ParentClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"PARENTMETH": {
				Name:      "parentMeth",
				ClassName: "ParentClass",
				Params:    []*advplrt.ParamDef{{Name: "cArg", Type: "C"}},
			},
		},
	}
	v.classes["PARENTCLASS"] = parentDef

	// Create child class with methods
	childDef := &advplrt.ClassDef{
		Name:       "ChildClass",
		Parent:     "ParentClass",
		Properties: make(map[string]string),
		Methods: map[string]*advplrt.MethodDef{
			"CHILDMETH": {
				Name:      "childMeth",
				ClassName: "ChildClass",
				Params:    []*advplrt.ParamDef{},
			},
		},
	}
	v.classes["CHILDCLASS"] = childDef

	// Create object
	obj := advplrt.NewObject("ChildClass", childDef)

	// Test with lParent=.T.
	got, err := v.natives["CLASSMETHARR"].Fn([]advplrt.Value{obj, advplrt.NewBool(true)})
	if err != nil {
		t.Fatalf("ClassMethArr(lParent=.T.) retornou erro: %v", err)
	}

	arr, ok := got.(*advplrt.ArrayValue)
	if !ok {
		t.Errorf("ClassMethArr() retornou tipo %T, quer *ArrayValue", got)
		return
	}

	// Should have 2 entries (1 child + 1 parent method)
	if len(arr.Elements) != 2 {
		t.Errorf("ClassMethArr(lParent=.T.) retornou %d elementos, quer 2", len(arr.Elements))
	}

	// Verify rows include classname in column 2
	classesSeen := make(map[string]bool)
	methsSeen := make(map[string]bool)
	for i, elem := range arr.Elements {
		row, ok := elem.(*advplrt.ArrayValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d] é tipo %T, quer *ArrayValue", i, elem)
			continue
		}

		// Must have 3 columns with lParent=.T.: name, params, classname
		if len(row.Elements) != 3 {
			t.Errorf("ClassMethArr()[%d] tem %d colunas, quer exatamente 3 com lParent=.T.", i, len(row.Elements))
			continue
		}

		// Column 0: method name
		nameVal, ok := row.Elements[0].(*advplrt.StringValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d][0] (name) é tipo %T, quer *StringValue", i, row.Elements[0])
			continue
		}

		// Column 2: classname should be a string
		classnameVal, ok := row.Elements[2].(*advplrt.StringValue)
		if !ok {
			t.Errorf("ClassMethArr()[%d][2] (classname) é tipo %T, quer *StringValue", i, row.Elements[2])
			continue
		}

		classesSeen[classnameVal.Val] = true
		methsSeen[nameVal.Val] = true
	}

	// Verify we see both parent and child methods
	if !methsSeen["childMeth"] {
		t.Errorf("ClassMethArr(lParent=.T.) não incluiu métodos da ChildClass")
	}
	if !methsSeen["parentMeth"] {
		t.Errorf("ClassMethArr(lParent=.T.) não incluiu métodos da ParentClass")
	}

	// Verify we see both class names
	if !classesSeen["ChildClass"] {
		t.Errorf("ClassMethArr(lParent=.T.) não marcou método como da ChildClass")
	}
	if !classesSeen["ParentClass"] {
		t.Errorf("ClassMethArr(lParent=.T.) não marcou método como da ParentClass")
	}
}
