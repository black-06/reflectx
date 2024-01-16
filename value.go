package reflectx

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

const root = "__ROOT__"

var (
	defaultValueOption = ValueOptions{IgnorePtr: true}
)

func GetValue(value any, path string) (any, error) {
	return defaultValueOption.GetValue(value, path)
}

func SetValue(value any, path string, v any) error {
	return defaultValueOption.SetValue(value, path, v)
}

func ValueEntryByPath(value any, path string) (*ValueEntry, error) {
	return defaultValueOption.ValueEntryByPath(value, path)
}

type ValueEntry struct {
	reflect.Value
	Parent reflect.Value

	// Key is current Value's key in Parent map.
	Key reflect.Value
	// Idx is current Value's idx in Parent struct fields or Parent array/slice
	Idx int
}

// StructField returns current reflect.StructField in Parent struct.
func (e ValueEntry) StructField() (reflect.StructField, bool) {
	if e.Parent.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	return e.Parent.Type().Field(e.Idx), true
}

type ValueOptions struct {
	IgnorePtr bool
}

// GetValue get value by ValueEntryByPath.
// Note: you can't get un-exported field.
func (o ValueOptions) GetValue(value any, path string) (any, error) {
	entry, err := o.ValueEntryByPath(value, path)
	if err != nil {
		return nil, err
	}
	entry.Value = o.DeRef(entry.Value)
	if !entry.Value.IsValid() || !entry.Value.CanInterface() {
		return nil, errors.New("invalid value")
	}
	return entry.Value.Interface(), nil
}

// SetValue set value by ValueEntryByPath.
// Note: you can't set un-exported field and that the field and value types must match.
func (o ValueOptions) SetValue(value any, path string, v any) error {
	entry, err := o.ValueEntryByPath(value, path)
	if err != nil {
		return err
	}
	switch entry.Parent.Kind() {
	case reflect.Map:
		entry.Parent.SetMapIndex(entry.Key, reflect.ValueOf(v))
	case reflect.String:
		switch v := v.(type) {
		case byte:
			bytes := []byte(entry.Parent.String())
			bytes[entry.Idx] = v
			entry.Parent.SetString(string(bytes))
		case rune:
			runes := []rune(entry.Parent.String())
			runes[entry.Idx] = v
			entry.Parent.SetString(string(runes))
		default:
			return errors.New("setting value should be a byte or rune")
		}
	default:
		entry.Value = o.DeRef(entry.Value)
		if entry.Value.CanAddr() {
			entry.Value.Set(reflect.ValueOf(v))
		} else {
			return errors.New("value is unaddressable")
		}
	}
	return nil
}

// ValueEntryByPath access a field of value by a path.
// The path is represented by Go's expression which can be parsed by go/parser.ParseExpr.
// But the extra feature is, you can use "$" to represent the value root path.
// You can use selectors and indexes in a path.
// Slice and arrays index allow only expressions of int.
// Maps key allow expressions of int, float, complex, char and string.
func (o ValueOptions) ValueEntryByPath(value any, path string) (*ValueEntry, error) {
	if strings.HasPrefix(path, "$") {
		path = root + path[1:]
	} else {
		path = root + "." + path
	}

	expr, err := parser.ParseExpr(path)
	if err != nil {
		return nil, fmt.Errorf("invliad path: %v", err)
	}

	entry := ValueEntry{Value: o.DeRef(reflect.ValueOf(value))}
	if err = o.walkEntry(&entry, expr); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (o ValueOptions) walkEntry(entry *ValueEntry, expr ast.Expr) error {
	switch expr := expr.(type) {
	case *ast.SelectorExpr:
		if err := o.walkEntry(entry, expr.X); err != nil {
			return err
		}
		if err := o.walkEntry(entry, expr.Sel); err != nil {
			return err
		}
		return nil
	case *ast.IndexExpr:
		if err := o.walkEntry(entry, expr.X); err != nil {
			return err
		}
		if err := o.walkEntry(entry, expr.Index); err != nil {
			return err
		}
		return nil
	case *ast.Ident:
		if expr.Name == root {
			return nil
		}
		if !ast.IsExported(expr.Name) {
			return errors.New("cannot access unexported field")
		}
		entry.Parent = o.DeRef(entry.Value)
		if !entry.Parent.IsValid() {
			return errors.New("invalid value")
		}
		if entry.Parent.Kind() != reflect.Struct {
			return fmt.Errorf("value does not match the path, type %q is not a struct", entry.Parent.Type().Name())
		}
		field, ok := entry.Parent.Type().FieldByName(expr.Name)
		if !ok {
			return fmt.Errorf("value does not match the path, type %q has no field named %q", entry.Parent.Type().Name(), expr.Name)
		}
		entry.Idx = field.Index[0]
		entry.Value = entry.Parent.FieldByIndex(field.Index)
		return nil
	case *ast.BasicLit:
		entry.Parent = o.DeRef(entry.Value)
		switch entry.Parent.Kind() {
		case reflect.Array, reflect.Slice, reflect.String:
			if expr.Kind != token.INT {
				return errors.New("value does not match the path")
			}
			idx, err := strconv.ParseInt(expr.Value, 10, 64)
			if err != nil {
				return err
			}
			entry.Idx = int(idx)
			if entry.Parent.Len() <= entry.Idx {
				return fmt.Errorf("index %d out of range %d", entry.Idx, entry.Parent.Len())
			}
			entry.Value = entry.Parent.Index(entry.Idx)
			return nil
		case reflect.Map:
			var key any
			var err error
			switch expr.Kind {
			case token.INT:
				if key, err = strconv.ParseInt(expr.Value, 10, 64); err != nil {
					return err
				}
			case token.FLOAT:
				if key, err = strconv.ParseFloat(expr.Value, 64); err != nil {
					return err
				}
			case token.IMAG:
				if key, err = strconv.ParseComplex(expr.Value, 128); err != nil {
					return err
				}
			case token.CHAR:
				if key, _, _, err = strconv.UnquoteChar(expr.Value[1:len(expr.Value)-1], '\''); err != nil {
					return err
				}
			case token.STRING:
				if key, err = strconv.Unquote(expr.Value); err != nil {
					return err
				}
			default:
				return errors.New("unknown basic type")
			}
			entry.Key = reflect.ValueOf(key)
			if kt := entry.Parent.Type().Key(); entry.Key.CanConvert(kt) {
				entry.Key = entry.Key.Convert(kt)
			} else {
				return fmt.Errorf("invalid map key %s", expr.Value)
			}
			entry.Value = entry.Parent.MapIndex(entry.Key)
			if !entry.Value.IsValid() {
				entry.Value = reflect.Zero(entry.Parent.Type().Elem())
			}
			return nil
		default:
			return fmt.Errorf(`value is not slice or map near "[%s]"`, expr.Value)
		}
	default:
		return errors.New("unknown expr")
	}
}

func (o ValueOptions) DeRef(rv reflect.Value) reflect.Value {
	for {
		kind := rv.Kind()
		switch {
		case o.IgnorePtr && kind == reflect.Ptr:
			if rv.IsNil() {
				if rv.CanAddr() {
					rv.Set(reflect.New(rv.Type().Elem()))
				} else {
					rv = reflect.New(rv.Type().Elem())
				}
			}
			rv = rv.Elem()
		case kind == reflect.Interface:
			rv = rv.Elem()
		default:
			return rv
		}
	}
}
