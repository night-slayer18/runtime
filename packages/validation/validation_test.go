package validation

import (
	"regexp"
	"testing"
)

func TestRequired(t *testing.T) {
	v := Required()
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"nil fails", nil, true},
		{"empty string fails", "", true},
		{"non-empty string passes", "x", false},
		{"zero int passes", 0, false},
		{"false bool passes", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("Required()(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestMinLength(t *testing.T) {
	v := MinLength(3)
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"shorter fails", "ab", true},
		{"exact passes", "abc", false},
		{"longer passes", "abcd", false},
		{"counts runes not bytes", "héé", false}, // 3 runes
		{"non-string fails", 5, true},
		{"nil passes", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("MinLength(3)(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestMaxLength(t *testing.T) {
	v := MaxLength(3)
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"shorter passes", "ab", false},
		{"exact passes", "abc", false},
		{"longer fails", "abcd", true},
		{"non-string fails", 5, true},
		{"nil passes", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("MaxLength(3)(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestPattern(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	v := Pattern(re)
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"match passes", "abc", false},
		{"no match fails", "abc1", true},
		{"non-string fails", 5, true},
		{"nil passes", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("Pattern(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}

	t.Run("nil regex fails", func(t *testing.T) {
		if err := Pattern(nil)("anything"); err == nil {
			t.Error("Pattern(nil) should return an error")
		}
	})
}

func TestInRange(t *testing.T) {
	v := InRange(1, 10)
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"below fails", 0, true},
		{"lower bound passes", 1, false},
		{"middle passes", 5, false},
		{"upper bound passes", 10, false},
		{"above fails", 11, true},
		{"int64 supported", int64(5), false},
		{"uint supported", uint(5), false},
		{"non-int fails", "5", true},
		{"nil passes", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v(tt.input); (err != nil) != tt.wantErr {
				t.Errorf("InRange(1,10)(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestAll(t *testing.T) {
	v := All(Required(), MinLength(2), MaxLength(4))

	t.Run("all pass", func(t *testing.T) {
		if err := v("abc"); err != nil {
			t.Errorf("expected pass, got %v", err)
		}
	})
	t.Run("first failure short-circuits", func(t *testing.T) {
		if err := v(""); err == nil {
			t.Error("expected failure for empty string")
		}
	})
	t.Run("later failure caught", func(t *testing.T) {
		if err := v("abcde"); err == nil {
			t.Error("expected failure for too-long string")
		}
	})
	t.Run("empty composition passes", func(t *testing.T) {
		if err := All()("anything"); err != nil {
			t.Errorf("expected empty composition to pass, got %v", err)
		}
	})
	t.Run("nil validators skipped", func(t *testing.T) {
		if err := All(nil, Required(), nil)("x"); err != nil {
			t.Errorf("expected pass with nil validators skipped, got %v", err)
		}
	})
}

func TestCompose_AliasesAll(t *testing.T) {
	v := Compose(Required(), MinLength(1))
	if err := v("a"); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
	if err := v(""); err == nil {
		t.Error("expected failure for empty string")
	}
}

// TestSchemaFieldCompatibility documents that a Validator is directly
// assignable to the func(interface{}) error signature used by
// schema.Field.Validator, with no adapter required.
func TestSchemaFieldCompatibility(t *testing.T) {
	var fieldValidator func(interface{}) error = MinLength(3)
	if err := fieldValidator("abcd"); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
	if err := fieldValidator("ab"); err == nil {
		t.Error("expected failure for short string")
	}
}
