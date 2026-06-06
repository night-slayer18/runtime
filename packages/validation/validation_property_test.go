package validation

import (
	"regexp"
	"testing"
	"testing/quick"
)

// Feature: runtime-ecosystem, Property: validator composition
//
// For any input and any set of validators, composing them with All passes if
// and only if every individual validator passes. Equivalently:
//
//	All(vs...)(input) == nil  <=>  for every v in vs, v(input) == nil
//
// Validates: Requirements 7.5

// validatorPool is the fixed catalogue of validators the property samples from.
// It spans every constructor in the package and several configurations of each,
// so the randomly assembled sets exercise a broad mix of pass/fail behaviour
// across the candidate inputs below.
var validatorPool = []Validator{
	Required(),
	MinLength(0),
	MinLength(3),
	MinLength(8),
	MaxLength(2),
	MaxLength(5),
	MaxLength(64),
	Pattern(regexp.MustCompile(`^[a-z]+$`)),
	Pattern(regexp.MustCompile(`^\d+$`)),
	Pattern(nil), // always errors; ensures failing validators are represented
	InRange(0, 10),
	InRange(-5, 5),
	InRange(100, 200),
	nil, // All documents that nil validators are skipped
}

// inputPool is the set of candidate values fed to the validators. It mixes
// strings of varying length and character classes, integer kinds, nil, and a
// few other types so that type-mismatch paths are exercised too.
var inputPool = []interface{}{
	nil,
	"",
	"a",
	"abc",
	"abcdef",
	"héllo",
	"ABC",
	"abc123",
	"123",
	"this-is-a-fairly-long-string-value",
	0,
	3,
	7,
	-3,
	150,
	int64(5),
	uint(8),
	true,
	3.14,
}

// allIndividuallyPass reports whether every validator in vs accepts input. This
// is the independent reference computation the composed validator is checked
// against.
func allIndividuallyPass(vs []Validator, input interface{}) bool {
	for _, v := range vs {
		if v == nil {
			continue
		}
		if v(input) != nil {
			return false
		}
	}
	return true
}

// TestPropertyValidatorComposition verifies that for any input and any set of
// validators, the composed validator (All) passes exactly when every individual
// validator passes.
//
// Feature: runtime-ecosystem, Property: validator composition
//
// Validates: Requirements 7.5
func TestPropertyValidatorComposition(t *testing.T) {
	// prop draws a random subset of the validator pool and a random input,
	// then compares the composed result against the independent reference.
	prop := func(validatorPicks []uint8, inputPick uint16) bool {
		// Build the validator set from the random picks. An empty pick list
		// exercises the documented "empty composition passes" case.
		vs := make([]Validator, 0, len(validatorPicks))
		for _, p := range validatorPicks {
			vs = append(vs, validatorPool[int(p)%len(validatorPool)])
		}

		input := inputPool[int(inputPick)%len(inputPool)]

		composedPasses := All(vs...)(input) == nil
		everyPasses := allIndividuallyPass(vs, input)

		if composedPasses != everyPasses {
			t.Errorf("composition mismatch: All(...)(%#v) passed=%v but every-individual-passed=%v (set size %d)",
				input, composedPasses, everyPasses, len(vs))
			return false
		}

		// Compose is documented as an alias for All; it must agree.
		if (Compose(vs...)(input) == nil) != composedPasses {
			t.Errorf("Compose disagreed with All for input %#v", input)
			return false
		}

		return true
	}

	// MaxCount exceeds the design's 100-iteration minimum for property tests.
	cfg := &quick.Config{MaxCount: 500}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}
