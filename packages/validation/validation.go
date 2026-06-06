// Package validation provides composable input validators for Runtime
// applications.
//
// A Validator is a function that inspects a single value and returns an error
// when the value is invalid, or nil when it is acceptable. The signature is
// deliberately identical to schema.Field.Validator
// (func(interface{}) error), so any Validator produced here can be assigned
// directly to a schema field without an adapter:
//
//	field := schema.Field{
//	    Name:      "username",
//	    Type:      schema.TypeString,
//	    Required:  true,
//	    Validator: validation.All(validation.MinLength(3), validation.MaxLength(32)),
//	}
//
// Constructors return validators that are safe to reuse and share across
// goroutines; they hold no mutable state.
package validation

import (
	"fmt"
	"regexp"
)

// Validator inspects a value and reports whether it is valid. It returns nil
// when the value is acceptable and a descriptive error otherwise.
//
// Validator is intentionally identical to the function type expected by
// schema.Field.Validator, so the two packages compose without any adapter.
type Validator func(interface{}) error

// Required returns a Validator that rejects nil values and empty strings.
//
// A value is considered missing when it is nil or, for strings, when it has
// zero length. All other values (including the boolean false and the integer
// zero) are treated as present.
func Required() Validator {
	return func(v interface{}) error {
		if v == nil {
			return fmt.Errorf("value is required")
		}
		if s, ok := v.(string); ok && len(s) == 0 {
			return fmt.Errorf("value is required")
		}
		return nil
	}
}

// MinLength returns a Validator that requires a string value to have at least
// n characters (counted as runes). Non-string values fail with a type error.
// A nil value passes so that MinLength can be combined with Required to control
// presence independently.
func MinLength(n int) Validator {
	return func(v interface{}) error {
		if v == nil {
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
		if l := len([]rune(s)); l < n {
			return fmt.Errorf("length %d is below minimum %d", l, n)
		}
		return nil
	}
}

// MaxLength returns a Validator that requires a string value to have at most
// n characters (counted as runes). Non-string values fail with a type error.
// A nil value passes so that MaxLength can be combined with Required to control
// presence independently.
func MaxLength(n int) Validator {
	return func(v interface{}) error {
		if v == nil {
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
		if l := len([]rune(s)); l > n {
			return fmt.Errorf("length %d exceeds maximum %d", l, n)
		}
		return nil
	}
}

// Pattern returns a Validator that requires a string value to match the
// supplied regular expression. A nil regex rejects every value with an error so
// that misconfiguration is surfaced rather than silently accepted. Non-string
// values fail with a type error; a nil value passes.
func Pattern(regex *regexp.Regexp) Validator {
	return func(v interface{}) error {
		if regex == nil {
			return fmt.Errorf("pattern validator has no regular expression")
		}
		if v == nil {
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
		if !regex.MatchString(s) {
			return fmt.Errorf("value %q does not match pattern %s", s, regex.String())
		}
		return nil
	}
}

// InRange returns a Validator that requires an integer value to fall within the
// inclusive range [min, max]. Integer kinds (int and its sized variants, plus
// the unsigned variants) are accepted and compared as int64. Non-integer values
// fail with a type error; a nil value passes.
func InRange(min, max int) Validator {
	return func(v interface{}) error {
		if v == nil {
			return nil
		}
		n, ok := toInt64(v)
		if !ok {
			return fmt.Errorf("expected integer, got %T", v)
		}
		if n < int64(min) || n > int64(max) {
			return fmt.Errorf("value %d is outside range [%d, %d]", n, min, max)
		}
		return nil
	}
}

// All returns a Validator that runs every supplied validator in order and
// returns the first error encountered. With no validators it always passes.
// The composed validator passes if and only if every individual validator
// passes.
func All(validators ...Validator) Validator {
	return func(v interface{}) error {
		for _, validate := range validators {
			if validate == nil {
				continue
			}
			if err := validate(v); err != nil {
				return err
			}
		}
		return nil
	}
}

// Compose is an alias for All, provided for readability at call sites that read
// better as "compose these validators".
func Compose(validators ...Validator) Validator {
	return All(validators...)
}

// toInt64 converts any Go integer kind to an int64, reporting whether the value
// was an integer. Unsigned values are converted as-is; callers compare against
// signed bounds.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	default:
		return 0, false
	}
}
