package vm

import (
	"sort"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodeclasseNatives registra funções de manipulação de classe:
// AttIsMemberOf, ClassDataArr, ClassMethArr, FindClass, GetClassName, MethIsMemberOf.
func (v *VM) registerManipulacaodeclasseNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// GetClassName(oObj) -> cClassName
	natives["GETCLASSNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := getArg(args, 0)
		if obj == nil || obj == advplrt.Nil {
			return advplrt.NewString(""), nil
		}
		o, ok := obj.(*advplrt.ObjectValue)
		if !ok {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(o.ClassName), nil
	}

	// FindClass(cClassName) -> lFound
	natives["FINDCLASS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cClassName := getArg(args, 0)
		if cClassName == nil || cClassName == advplrt.Nil {
			return advplrt.False, nil
		}
		s, ok := cClassName.(*advplrt.StringValue)
		if !ok {
			return advplrt.False, nil
		}
		// Lookup in v.classes (case-insensitive key)
		_, found := v.classes[strings.ToUpper(s.Val)]
		return advplrt.NewBool(found), nil
	}

	// AttIsMemberOf(oObj, cAttName, [lRecursive]) -> lFound
	natives["ATTISMEMBEROF"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := getArg(args, 0)
		cAttName := getArg(args, 1)
		lRecursive := false
		if len(args) > 2 && args[2] != nil && args[2] != advplrt.Nil {
			if b, ok := args[2].(*advplrt.BoolValue); ok {
				lRecursive = b.Val
			}
		}

		if obj == nil || obj == advplrt.Nil {
			return advplrt.False, nil
		}

		o, ok := obj.(*advplrt.ObjectValue)
		if !ok {
			return advplrt.False, nil
		}

		if cAttName == nil || cAttName == advplrt.Nil {
			return advplrt.False, nil
		}

		attName, ok := cAttName.(*advplrt.StringValue)
		if !ok {
			return advplrt.False, nil
		}

		attNameUpper := strings.ToUpper(attName.Val)

		// Check current class
		if o.Class != nil {
			// Check with case-insensitive lookup
			for propName := range o.Class.Properties {
				if strings.ToUpper(propName) == attNameUpper {
					return advplrt.True, nil
				}
			}

			// If recursive, check parent class chain
			if lRecursive {
				currentClass := o.Class
				for currentClass != nil && currentClass.Parent != "" {
					parentClass, found := v.classes[strings.ToUpper(currentClass.Parent)]
					if !found {
						break
					}
					for propName := range parentClass.Properties {
						if strings.ToUpper(propName) == attNameUpper {
							return advplrt.True, nil
						}
					}
					currentClass = parentClass
				}
			}
		}

		return advplrt.False, nil
	}

	// MethIsMemberOf(oObj, cMethName, [lRecursive]) -> lFound
	natives["METHISMEMBEROF"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := getArg(args, 0)
		cMethName := getArg(args, 1)
		lRecursive := false
		if len(args) > 2 && args[2] != nil && args[2] != advplrt.Nil {
			if b, ok := args[2].(*advplrt.BoolValue); ok {
				lRecursive = b.Val
			}
		}

		if obj == nil || obj == advplrt.Nil {
			return advplrt.False, nil
		}

		o, ok := obj.(*advplrt.ObjectValue)
		if !ok {
			return advplrt.False, nil
		}

		if cMethName == nil || cMethName == advplrt.Nil {
			return advplrt.False, nil
		}

		methName, ok := cMethName.(*advplrt.StringValue)
		if !ok {
			return advplrt.False, nil
		}

		methNameUpper := strings.ToUpper(methName.Val)

		// Check current class
		if o.Class != nil {
			if _, found := o.Class.Methods[methNameUpper]; found {
				return advplrt.True, nil
			}

			// If recursive, check parent class chain
			if lRecursive {
				currentClass := o.Class
				for currentClass != nil && currentClass.Parent != "" {
					parentClass, found := v.classes[strings.ToUpper(currentClass.Parent)]
					if !found {
						break
					}
					if _, found := parentClass.Methods[methNameUpper]; found {
						return advplrt.True, nil
					}
					currentClass = parentClass
				}
			}
		}

		return advplrt.False, nil
	}

	// ClassDataArr(oObj, [lParent]) -> aData
	// Returns array of [name, value, id] (+ classname if lParent=.T.)
	natives["CLASSDATAARR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := getArg(args, 0)
		lParent := false
		if len(args) > 1 && args[1] != nil && args[1] != advplrt.Nil {
			if b, ok := args[1].(*advplrt.BoolValue); ok {
				lParent = b.Val
			}
		}

		if obj == nil || obj == advplrt.Nil {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		o, ok := obj.(*advplrt.ObjectValue)
		if !ok {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		var result []advplrt.Value

		// Collect properties from current class
		if o.Class != nil {
			// Get sorted property names for determinism
			propNames := make([]string, 0, len(o.Class.Properties))
			for name := range o.Class.Properties {
				propNames = append(propNames, name)
			}
			sort.Strings(propNames)

			for _, propName := range propNames {
				row := make([]advplrt.Value, 0)
				// Column 1: property name
				row = append(row, advplrt.NewString(propName))
				// Column 2: property value (from Props if exists, otherwise Nil)
				var val advplrt.Value = advplrt.Nil
				if propVal, found := o.Props[propName]; found {
					val = propVal
				}
				row = append(row, val)
				// Column 3: id (dummy value, just use 1)
				row = append(row, advplrt.NewNumber(1))
				// Column 4: classname (only if lParent=.T.)
				if lParent {
					row = append(row, advplrt.NewString(o.Class.Name))
				}
				result = append(result, advplrt.NewArray(row))
			}

			// If lParent, also include parent class properties
			if lParent {
				currentClass := o.Class
				for currentClass != nil && currentClass.Parent != "" {
					parentClass, found := v.classes[strings.ToUpper(currentClass.Parent)]
					if !found {
						break
					}

					// Get sorted parent property names
					parentPropNames := make([]string, 0, len(parentClass.Properties))
					for name := range parentClass.Properties {
						parentPropNames = append(parentPropNames, name)
					}
					sort.Strings(parentPropNames)

					for _, propName := range parentPropNames {
						row := make([]advplrt.Value, 0)
						row = append(row, advplrt.NewString(propName))
						var val advplrt.Value = advplrt.Nil
						if propVal, found := o.Props[propName]; found {
							val = propVal
						}
						row = append(row, val)
						row = append(row, advplrt.NewNumber(1))
						row = append(row, advplrt.NewString(parentClass.Name))
						result = append(result, advplrt.NewArray(row))
					}

					currentClass = parentClass
				}
			}
		}

		return advplrt.NewArray(result), nil
	}

	// ClassMethArr(oObj, [lParent]) -> aData
	// Returns array of [name, paramsArray] (+ classname if lParent=.T.)
	natives["CLASSMETHARR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		obj := getArg(args, 0)
		lParent := false
		if len(args) > 1 && args[1] != nil && args[1] != advplrt.Nil {
			if b, ok := args[1].(*advplrt.BoolValue); ok {
				lParent = b.Val
			}
		}

		if obj == nil || obj == advplrt.Nil {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		o, ok := obj.(*advplrt.ObjectValue)
		if !ok {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		var result []advplrt.Value

		// Collect methods from current class
		if o.Class != nil {
			// Get sorted method names for determinism
			methNames := make([]string, 0, len(o.Class.Methods))
			for name := range o.Class.Methods {
				methNames = append(methNames, name)
			}
			sort.Strings(methNames)

			for _, methName := range methNames {
				methDef := o.Class.Methods[methName]
				row := make([]advplrt.Value, 0)
				// Column 1: method name
				row = append(row, advplrt.NewString(methDef.Name))
				// Column 2: parameters array
				paramNames := make([]advplrt.Value, 0)
				if methDef.Params != nil {
					for _, param := range methDef.Params {
						paramNames = append(paramNames, advplrt.NewString(param.Name))
					}
				}
				row = append(row, advplrt.NewArray(paramNames))
				// Column 3: classname (only if lParent=.T.)
				if lParent {
					row = append(row, advplrt.NewString(o.Class.Name))
				}
				result = append(result, advplrt.NewArray(row))
			}

			// If lParent, also include parent class methods
			if lParent {
				currentClass := o.Class
				for currentClass != nil && currentClass.Parent != "" {
					parentClass, found := v.classes[strings.ToUpper(currentClass.Parent)]
					if !found {
						break
					}

					// Get sorted parent method names
					parentMethNames := make([]string, 0, len(parentClass.Methods))
					for name := range parentClass.Methods {
						parentMethNames = append(parentMethNames, name)
					}
					sort.Strings(parentMethNames)

					for _, methName := range parentMethNames {
						methDef := parentClass.Methods[methName]
						row := make([]advplrt.Value, 0)
						row = append(row, advplrt.NewString(methDef.Name))
						paramNames := make([]advplrt.Value, 0)
						if methDef.Params != nil {
							for _, param := range methDef.Params {
								paramNames = append(paramNames, advplrt.NewString(param.Name))
							}
						}
						row = append(row, advplrt.NewArray(paramNames))
						row = append(row, advplrt.NewString(parentClass.Name))
						result = append(result, advplrt.NewArray(row))
					}

					currentClass = parentClass
				}
			}
		}

		return advplrt.NewArray(result), nil
	}
}
