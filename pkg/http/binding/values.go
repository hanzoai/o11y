package binding

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
)

var (
	ErrCodeUnsupportedBindingKind = errors.MustNewCode("unsupported_binding_kind")
	ErrCodeInvalidBindingTarget   = errors.MustNewCode("invalid_binding_target")
)

// Unmarshaler is implemented by types that decode themselves from a single raw
// parameter. It is the one extension point of bindValues: every enum in the
// codebase reaches it through valuer, so a named type never needs a case here.
type Unmarshaler interface {
	// UnmarshalParam decodes and assigns a value from a form or query param.
	UnmarshalParam(param string) error
}

var unmarshalerType = reflect.TypeFor[Unmarshaler]()

// bindValues maps values onto the fields of ptr, a non-nil pointer to a struct,
// selecting each field by its `tag` struct tag. A field is left untouched when
// its key is absent and it declares no `,default=` option, which is what makes
// a nil pointer field distinguishable from a supplied zero.
func bindValues(ptr any, values map[string][]string, tag string) error {
	value := reflect.ValueOf(ptr)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errors.Newf(errors.TypeInvalidInput, ErrCodeInvalidBindingTarget, "binding target must be a non-nil pointer, got %T", ptr)
	}

	_, err := bindValue(value.Elem(), reflect.StructField{}, values, tag)
	return err
}

// bindValue binds one value, reporting whether anything was set. Pointers are
// allocated only if their element ends up set; anonymous structs are flattened
// into the parent namespace; every other struct is offered to bindField first
// so that an Unmarshaler wins over its own fields.
func bindValue(value reflect.Value, field reflect.StructField, values map[string][]string, tag string) (bool, error) {
	if field.Tag.Get(tag) == "-" {
		return false, nil
	}

	if value.Kind() == reflect.Pointer {
		pointer, isNew := value, value.IsNil()
		if isNew {
			pointer = reflect.New(value.Type().Elem())
		}

		set, err := bindValue(pointer.Elem(), field, values, tag)
		if err != nil || !set {
			return false, err
		}

		if isNew {
			value.Set(pointer)
		}

		return true, nil
	}

	if value.Kind() != reflect.Struct || !field.Anonymous {
		set, err := bindField(value, field, values, tag)
		if err != nil {
			return false, err
		}
		if set {
			return true, nil
		}
	}

	if value.Kind() == reflect.Struct {
		typ, set := value.Type(), false
		for i := range typ.NumField() {
			structField := typ.Field(i)
			if structField.PkgPath != "" && !structField.Anonymous {
				continue
			}

			ok, err := bindValue(value.Field(i), structField, values, tag)
			if err != nil {
				return false, err
			}

			set = set || ok
		}

		return set, nil
	}

	return false, nil
}

// bindField resolves the key and default for a single struct field and sets it.
func bindField(value reflect.Value, field reflect.StructField, values map[string][]string, tag string) (bool, error) {
	key, opts, _ := strings.Cut(field.Tag.Get(tag), ",")
	if key == "" {
		key = field.Name
	}
	if key == "" {
		return false, nil
	}

	defaultValue, hasDefault := defaultOption(opts)

	vs, ok := values[key]
	if !ok && !hasDefault {
		return false, nil
	}

	if value.Kind() == reflect.Slice && !implementsUnmarshaler(value) {
		if len(vs) == 0 {
			if !hasDefault {
				return false, nil
			}
			vs = []string{defaultValue}
		}

		slice := reflect.MakeSlice(value.Type(), len(vs), len(vs))
		for i, v := range vs {
			if err := setValue(v, slice.Index(i)); err != nil {
				return false, err
			}
		}
		value.Set(slice)

		return true, nil
	}

	raw := defaultValue
	if len(vs) > 0 && vs[0] != "" {
		raw = vs[0]
	}

	return true, setValue(raw, value)
}

// defaultOption reports the `default=` option of a tag, if any.
func defaultOption(opts string) (string, bool) {
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")

		if key, value, _ := strings.Cut(opt, "="); key == "default" {
			return value, true
		}
	}

	return "", false
}

// setValue parses raw into value. An empty raw is the zero of the kind rather
// than a parse error, so `?limit=` behaves like an omitted limit.
func setValue(raw string, value reflect.Value) error {
	if implementsUnmarshaler(value) {
		return value.Addr().Interface().(Unmarshaler).UnmarshalParam(raw)
	}

	if value.Kind() != reflect.String {
		raw = strings.TrimSpace(raw)
	}

	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		if raw == "" {
			raw = "false"
		}
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		value.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if raw == "" {
			raw = "0"
		}
		parsed, err := strconv.ParseInt(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if raw == "" {
			raw = "0"
		}
		parsed, err := strconv.ParseUint(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		if raw == "" {
			raw = "0"
		}
		parsed, err := strconv.ParseFloat(raw, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetFloat(parsed)
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return setValue(raw, value.Elem())
	default:
		return errors.Newf(errors.TypeInvalidInput, ErrCodeUnsupportedBindingKind, "cannot bind %q into %s", raw, value.Type())
	}

	return nil
}

func implementsUnmarshaler(value reflect.Value) bool {
	return value.CanAddr() && value.Addr().Type().Implements(unmarshalerType)
}
