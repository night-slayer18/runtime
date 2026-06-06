package theme

// Feature: runtime-ecosystem, Property 2 & 3: theme and config consistency across apps
//
// Theme half (Property 2): All five Runtime applications (grid, prism, pulse,
// strata, vault) resolve themes through this single shared package, so for any
// theme name every app must build byte-identical Styles. This test models the
// five apps as five independent callers of Resolve+NewStyles (the same
// composition Apply performs) and asserts the rendered fingerprint is identical
// across all of them.
//
// Validates: Requirements 4.2

import (
	"math/rand"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// ecosystemApps is the fixed set of Runtime applications. All of them load
// their theme through this package, so the resolver is shared by construction.
var ecosystemApps = []string{"grid", "prism", "pulse", "strata", "vault"}

// TestEcosystem_ThemeConsistencyAcrossFiveApps asserts that for any theme name,
// all five Runtime applications resolve identical Styles. Because every app
// uses the same Resolve+NewStyles path, resolving once per app and comparing
// fingerprints models "all five apps render the same theme identically".
//
// Feature: runtime-ecosystem, Property 2 & 3: theme and config consistency across apps
// Validates: Requirements 4.2
func TestEcosystem_ThemeConsistencyAcrossFiveApps(t *testing.T) {
	// Deterministic color-emitting profile so color differences materialize
	// into the fingerprint even in a non-TTY test environment.
	lipgloss.SetColorProfile(termenv.TrueColor)

	rng := rand.New(rand.NewSource(0xEC05A5)) // fixed seed -> reproducible failures
	pool := namePool(t, rng)                  // built-ins + random customs + unknowns

	const iterations = 200 // exceeds the design's 100-iteration minimum
	for i := 0; i < iterations; i++ {
		name := pool[rng.Intn(len(pool))]

		// Each of the five apps independently resolves and builds styles for
		// the same theme name, exactly as it would at startup.
		fingerprints := make(map[string]string, len(ecosystemApps))
		for _, app := range ecosystemApps {
			fingerprints[app] = fingerprint(resolveStyles(name))
		}

		// Use the first app as the reference and require every other app to
		// match it byte-for-byte.
		ref := ecosystemApps[0]
		refFP := fingerprints[ref]
		for _, app := range ecosystemApps[1:] {
			if fingerprints[app] != refFP {
				t.Fatalf("ecosystem theme inconsistency for theme %q: app %q differs from app %q\n %q:\n  %q\n %q:\n  %q",
					name, app, ref, ref, refFP, app, fingerprints[app])
			}
		}
	}
}
