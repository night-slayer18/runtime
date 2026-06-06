package schema

import (
	"errors"
	"strings"
	"testing"
)

// errMissing is a sentinel used to verify custom Validator invocation.
var errMissing = errors.New("custom validator rejected value")

func TestValidate_RequiredFieldMissing(t *testing.T) {
	s := New(
		Field{Name: "name", Type: TypeString, Required: true},
		Field{Name: "age", Type: TypeInt, Required: false},
	)

	// Required field absent -> error.
	err := s.Validate(map[string]interface{}{"age": 30})
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	if !strings.Contains(err.Error(), `field "name"`) ||
		!strings.Contains(err.Error(), "required field is missing") {
		t.Fatalf("unexpected error for missing required field: %v", err)
	}

	// Required field present -> no error.
	if err := s.Validate(map[string]interface{}{"name": "ada"}); err != nil {
		t.Fatalf("expected no error when required field present, got %v", err)
	}
}

func TestValidate_OptionalFieldMissingIsOK(t *testing.T) {
	s := New(Field{Name: "nickname", Type: TypeString, Required: false})
	if err := s.Validate(map[string]interface{}{}); err != nil {
		t.Fatalf("expected no error for missing optional field, got %v", err)
	}
}

func TestValidate_NilDataTreatedAsEmpty(t *testing.T) {
	// Optional fields only: nil data must validate cleanly.
	s := New(Field{Name: "x", Type: TypeInt, Required: false})
	if err := s.Validate(nil); err != nil {
		t.Fatalf("expected nil data to validate as empty, got %v", err)
	}

	// Required field with nil data: must report missing.
	s2 := New(Field{Name: "x", Type: TypeInt, Required: true})
	if err := s2.Validate(nil); err == nil {
		t.Fatal("expected error for required field with nil data, got nil")
	}
}

func TestValidate_TypeMismatches(t *testing.T) {
	cases := []struct {
		name     string
		ftype    FieldType
		good     interface{}
		bad      interface{}
		expected string
	}{
		{"string", TypeString, "hello", 123, "string"},
		{"bool", TypeBool, true, "nope", "bool"},
		{"int", TypeInt, 42, "not-int", "int"},
		{"float", TypeFloat, 3.14, 7, "float"},
		{"map", TypeMap, map[string]interface{}{"a": 1}, []interface{}{1}, "map"},
		{"slice", TypeSlice, []interface{}{1, 2}, "not-slice", "slice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(Field{Name: "f", Type: tc.ftype, Required: true})

			// Correct type should pass.
			if err := s.Validate(map[string]interface{}{"f": tc.good}); err != nil {
				t.Fatalf("expected good value %v (%T) to pass, got %v", tc.good, tc.good, err)
			}

			// Wrong type should fail with a descriptive message.
			err := s.Validate(map[string]interface{}{"f": tc.bad})
			if err == nil {
				t.Fatalf("expected type error for %v (%T), got nil", tc.bad, tc.bad)
			}
			if !strings.Contains(err.Error(), "expected type "+tc.expected) {
				t.Fatalf("error did not mention expected type %q: %v", tc.expected, err)
			}
		})
	}
}

func TestValidate_TypeAnyAcceptsEverything(t *testing.T) {
	s := New(Field{Name: "f", Type: TypeAny, Required: true})
	for _, v := range []interface{}{"s", 1, true, 1.5, map[string]interface{}{}, []interface{}{}} {
		if err := s.Validate(map[string]interface{}{"f": v}); err != nil {
			t.Fatalf("TypeAny should accept %v (%T), got %v", v, v, err)
		}
	}
}

func TestValidate_IntAcceptsSizedVariants(t *testing.T) {
	s := New(Field{Name: "f", Type: TypeInt, Required: true})
	for _, v := range []interface{}{int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1)} {
		if err := s.Validate(map[string]interface{}{"f": v}); err != nil {
			t.Fatalf("TypeInt should accept %T, got %v", v, err)
		}
	}
}

func TestValidate_FloatAcceptsBothWidths(t *testing.T) {
	s := New(Field{Name: "f", Type: TypeFloat, Required: true})
	for _, v := range []interface{}{float32(1.5), float64(1.5)} {
		if err := s.Validate(map[string]interface{}{"f": v}); err != nil {
			t.Fatalf("TypeFloat should accept %T, got %v", v, err)
		}
	}
}

func TestValidate_CustomValidatorInvoked(t *testing.T) {
	called := false
	s := New(Field{
		Name:     "email",
		Type:     TypeString,
		Required: true,
		Validator: func(v interface{}) error {
			called = true
			if s, _ := v.(string); !strings.Contains(s, "@") {
				return errMissing
			}
			return nil
		},
	})

	// Validator passes.
	if err := s.Validate(map[string]interface{}{"email": "a@b.com"}); err != nil {
		t.Fatalf("expected valid email to pass, got %v", err)
	}
	if !called {
		t.Fatal("expected custom validator to be invoked")
	}

	// Validator rejects.
	err := s.Validate(map[string]interface{}{"email": "invalid"})
	if err == nil {
		t.Fatal("expected custom validator to reject value, got nil")
	}
	if !errors.Is(err, errMissing) {
		t.Fatalf("expected wrapped errMissing, got %v", err)
	}
}

func TestValidate_CustomValidatorSkippedOnTypeMismatch(t *testing.T) {
	called := false
	s := New(Field{
		Name:     "n",
		Type:     TypeInt,
		Required: true,
		Validator: func(interface{}) error {
			called = true
			return nil
		},
	})

	// Type is wrong, so the validator must not run.
	err := s.Validate(map[string]interface{}{"n": "not-an-int"})
	if err == nil {
		t.Fatal("expected type error, got nil")
	}
	if called {
		t.Fatal("custom validator should be skipped when the type check fails")
	}
}

func TestValidate_UnknownFieldIgnored(t *testing.T) {
	s := New(Field{Name: "known", Type: TypeString, Required: true})
	// Non-strict Validate ignores unknown keys.
	err := s.Validate(map[string]interface{}{
		"known":   "value",
		"unknown": "extra",
	})
	if err != nil {
		t.Fatalf("Validate should ignore unknown fields, got %v", err)
	}
}

func TestStrictValidate_UnknownFieldRejected(t *testing.T) {
	s := New(Field{Name: "known", Type: TypeString, Required: true})
	err := s.StrictValidate(map[string]interface{}{
		"known":   "value",
		"unknown": "extra",
	})
	if err == nil {
		t.Fatal("StrictValidate should reject unknown fields, got nil")
	}
	if !strings.Contains(err.Error(), `field "unknown"`) ||
		!strings.Contains(err.Error(), "unknown field not permitted") {
		t.Fatalf("unexpected strict-mode error: %v", err)
	}
}

func TestStrictValidate_AllKnownFieldsPass(t *testing.T) {
	s := New(
		Field{Name: "a", Type: TypeString, Required: true},
		Field{Name: "b", Type: TypeInt, Required: false},
	)
	err := s.StrictValidate(map[string]interface{}{"a": "x", "b": 1})
	if err != nil {
		t.Fatalf("StrictValidate should pass when all keys are known, got %v", err)
	}
}

func TestValidate_AggregatesMultipleErrors(t *testing.T) {
	s := New(
		Field{Name: "name", Type: TypeString, Required: true},
		Field{Name: "age", Type: TypeInt, Required: true},
	)
	// name missing + age wrong type => two aggregated errors.
	err := s.StrictValidate(map[string]interface{}{
		"age":   "old",
		"extra": true,
	})
	if err == nil {
		t.Fatal("expected aggregated errors, got nil")
	}
	msg := err.Error()
	for _, want := range []string{`field "name"`, `field "age"`, `field "extra"`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("aggregated error missing %q: %v", want, msg)
		}
	}
}

func TestValidate_NilValueAcceptedForTypedField(t *testing.T) {
	// A present-but-nil value passes the type check (presence is enforced
	// only via Required, which is satisfied because the key exists).
	s := New(Field{Name: "f", Type: TypeString, Required: true})
	if err := s.Validate(map[string]interface{}{"f": nil}); err != nil {
		t.Fatalf("expected nil value to pass type check, got %v", err)
	}
}

func TestFieldType_String(t *testing.T) {
	cases := map[FieldType]string{
		TypeAny:    "any",
		TypeString: "string",
		TypeBool:   "bool",
		TypeInt:    "int",
		TypeFloat:  "float",
		TypeMap:    "map",
		TypeSlice:  "slice",
	}
	for ft, want := range cases {
		if got := ft.String(); got != want {
			t.Fatalf("FieldType(%d).String() = %q, want %q", int(ft), got, want)
		}
	}
}

func TestAdd_AppendsField(t *testing.T) {
	s := New().
		Add(Field{Name: "a", Type: TypeString, Required: true}).
		Add(Field{Name: "b", Type: TypeInt})
	if len(s.Fields()) != 2 {
		t.Fatalf("expected 2 fields after Add chain, got %d", len(s.Fields()))
	}
	if err := s.Validate(map[string]interface{}{"a": "x"}); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
}
