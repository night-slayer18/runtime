// Package schema provides data schema validation for Runtime applications.
//
// A Schema is an ordered set of Fields. Each Field declares a name, an
// expected FieldType, whether it is required, and an optional custom
// Validator. Validate checks that supplied data conforms to the declared
// fields; StrictValidate additionally rejects any keys that are not declared
// by the schema.
package schema

import (
	"errors"
	"fmt"
	"sort"
)

// FieldType enumerates the value kinds a schema field can declare.
type FieldType int

const (
	// TypeAny accepts any value (no type checking is performed).
	TypeAny FieldType = iota
	// TypeString accepts Go string values.
	TypeString
	// TypeBool accepts Go bool values.
	TypeBool
	// TypeInt accepts integer values (int and its sized variants).
	TypeInt
	// TypeFloat accepts floating-point values (float32, float64).
	TypeFloat
	// TypeMap accepts map[string]interface{} values (nested objects).
	TypeMap
	// TypeSlice accepts []interface{} values (arrays).
	TypeSlice
)

// String returns a human-readable name for the field type.
func (t FieldType) String() string {
	switch t {
	case TypeAny:
		return "any"
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeMap:
		return "map"
	case TypeSlice:
		return "slice"
	default:
		return fmt.Sprintf("FieldType(%d)", int(t))
	}
}

// Field declares a single expected key in a data map.
//
// Validator has the signature func(interface{}) error, which is identical to
// the validation.Validator type in the validation package. A validation
// Validator (e.g. validation.MinLength(3) or validation.All(...)) is therefore
// directly assignable to this field without any adapter, letting the two
// packages compose. The schema package does not import validation, so the
// dependency only flows one way (from a consumer that uses both).
type Field struct {
	Name      string                  // key name in the data map
	Type      FieldType               // expected value type
	Required  bool                    // whether the key must be present
	Validator func(interface{}) error // optional custom validation (run after the type check)
}

// Schema is an ordered collection of Fields describing the expected shape of a
// data map. The zero value is a valid, empty schema. Use New to build one
// fluently.
type Schema struct {
	fields []Field
}

// New constructs a Schema from the provided fields.
func New(fields ...Field) *Schema {
	return &Schema{fields: append([]Field(nil), fields...)}
}

// Add appends a field to the schema and returns the schema for chaining.
func (s *Schema) Add(f Field) *Schema {
	s.fields = append(s.fields, f)
	return s
}

// Fields returns a copy of the schema's declared fields.
func (s *Schema) Fields() []Field {
	return append([]Field(nil), s.fields...)
}

// Validate checks that data satisfies every declared field. It verifies that
// required fields are present, that present values match their declared type,
// and that any custom Validator passes. Unknown keys (keys present in data but
// not declared in the schema) are ignored. A nil data map is treated as an
// empty map. All discovered problems are aggregated into a single error.
func (s *Schema) Validate(data map[string]interface{}) error {
	return s.validate(data, false)
}

// StrictValidate behaves like Validate but additionally rejects any key in
// data that is not declared by the schema.
func (s *Schema) StrictValidate(data map[string]interface{}) error {
	return s.validate(data, true)
}

func (s *Schema) validate(data map[string]interface{}, strict bool) error {
	var errs []error

	for _, f := range s.fields {
		val, ok := data[f.Name]
		if !ok {
			if f.Required {
				errs = append(errs, fmt.Errorf("field %q: required field is missing", f.Name))
			}
			continue
		}

		if err := checkType(f.Type, val); err != nil {
			errs = append(errs, fmt.Errorf("field %q: %w", f.Name, err))
			// Skip the custom validator when the type is already wrong.
			continue
		}

		if f.Validator != nil {
			if err := f.Validator(val); err != nil {
				errs = append(errs, fmt.Errorf("field %q: %w", f.Name, err))
			}
		}
	}

	if strict {
		known := make(map[string]struct{}, len(s.fields))
		for _, f := range s.fields {
			known[f.Name] = struct{}{}
		}
		var unknown []string
		for k := range data {
			if _, ok := known[k]; !ok {
				unknown = append(unknown, k)
			}
		}
		// Sort for deterministic error output.
		sort.Strings(unknown)
		for _, k := range unknown {
			errs = append(errs, fmt.Errorf("field %q: unknown field not permitted in strict mode", k))
		}
	}

	return errors.Join(errs...)
}

// checkType reports whether val matches the expected FieldType. A nil value is
// considered acceptable for any type (presence is enforced separately via the
// Required flag).
func checkType(t FieldType, val interface{}) error {
	if t == TypeAny || val == nil {
		return nil
	}

	switch t {
	case TypeString:
		if _, ok := val.(string); !ok {
			return typeError(t, val)
		}
	case TypeBool:
		if _, ok := val.(bool); !ok {
			return typeError(t, val)
		}
	case TypeInt:
		switch val.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			return nil
		default:
			return typeError(t, val)
		}
	case TypeFloat:
		switch val.(type) {
		case float32, float64:
			return nil
		default:
			return typeError(t, val)
		}
	case TypeMap:
		if _, ok := val.(map[string]interface{}); !ok {
			return typeError(t, val)
		}
	case TypeSlice:
		if _, ok := val.([]interface{}); !ok {
			return typeError(t, val)
		}
	}
	return nil
}

func typeError(t FieldType, val interface{}) error {
	return fmt.Errorf("expected type %s, got %T", t, val)
}
