package theme

// Feature: runtime-ecosystem, Property 2: Theme consistency across applications
//
// Property 2: For any valid theme config, resolving + building Styles for a
// theme name from two independent callers (modeling two different Runtime
// applications) yields byte-identical Styles: the same colors, spacing, and
// borders. Apply(name) wraps NewStyles(Resolve(name)); until Apply lands we
// exercise the same composition directly so the property holds for Apply too.
//
// Validates: Requirements 4.2

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// propertyIterations is the minimum number of randomized inputs exercised by
// the consistency property, per the design's testing strategy (>= 100).
const propertyIterations = 200

// resolveStyles models what Apply(name) does: resolve a palette by name and
// build the derived Styles. This is the unit whose determinism we assert.
func resolveStyles(name string) Styles {
	return NewStyles(Resolve(name))
}

// styleFingerprint renders a single lipgloss.Style over a fixed sample so that
// every visually meaningful attribute is materialized into bytes: foreground
// and background colors become ANSI escape sequences, padding/margins become
// spacing, and border definitions become border glyphs. Two styles producing
// the same fingerprint are byte-identical in appearance.
func styleFingerprint(s lipgloss.Style) string {
	const sample = "Xy 09"
	// A fixed width/height forces padding and borders to render even when the
	// sample text is short, so spacing and border differences are observable.
	return s.Width(12).Height(2).Render(sample)
}

// fingerprint renders every field of a Styles into one deterministic string.
// Equality of two fingerprints means the two Styles are byte-identical across
// colors, spacing, and borders.
func fingerprint(s Styles) string {
	var b strings.Builder
	fields := []lipgloss.Style{
		s.App, s.Header, s.Footer, s.Pane,
		s.Title, s.Subtitle, s.Body, s.Muted,
		s.Selected, s.Focused, s.Unfocused,
		s.Success, s.Warning, s.Error, s.Info,
		s.KeyHint, s.Badge,
	}
	for i, f := range fields {
		b.WriteString(styleFingerprint(f))
		if i < len(fields)-1 {
			b.WriteByte('\x1e') // record separator so fields can't bleed together
		}
	}
	return b.String()
}

// namePool builds the set of theme names the property samples over: the
// built-in themes, several randomly generated custom themes registered into
// the registry, plus some unknown names that exercise the documented Default
// fallback. This covers "built-in + registered customs" per the task.
func namePool(t *testing.T, rng *rand.Rand) []string {
	t.Helper()

	pool := Names() // built-ins (default, light) and anything already registered

	// Register a handful of random custom themes and clean them up afterward.
	const customCount = 6
	for i := 0; i < customCount; i++ {
		name := randomThemeName(rng)
		Register(name, randomPalette(rng))
		pool = append(pool, name)
		nm := name
		t.Cleanup(func() { delete(registry, nm) })
	}

	// Include some unknown names; Resolve must degrade to Default for these,
	// and that fallback must itself be deterministic across callers.
	for i := 0; i < 3; i++ {
		pool = append(pool, "unknown-"+randomThemeName(rng))
	}

	return pool
}

func randomThemeName(rng *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-"
	n := 4 + rng.Intn(8)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[rng.Intn(len(alphabet))])
	}
	return "custom-" + b.String()
}

func randomColor(rng *rand.Rand) lipgloss.AdaptiveColor {
	hex := func() string {
		const digits = "0123456789ABCDEF"
		var b strings.Builder
		b.WriteByte('#')
		for i := 0; i < 6; i++ {
			b.WriteByte(digits[rng.Intn(len(digits))])
		}
		return b.String()
	}
	return lipgloss.AdaptiveColor{Light: hex(), Dark: hex()}
}

func randomPalette(rng *rand.Rand) Palette {
	c := func() lipgloss.AdaptiveColor { return randomColor(rng) }
	return Palette{
		Background: c(), Surface: c(), Overlay: c(),
		Text: c(), Subtle: c(), Muted: c(),
		Primary: c(), Secondary: c(), Accent: c(),
		Success: c(), Warning: c(), Error: c(), Info: c(),
		Border: c(), BorderFocus: c(),
	}
}

// TestProperty2_ThemeConsistencyAcrossApplications asserts that for any valid
// theme name, two independent resolutions produce byte-identical Styles.
//
// Feature: runtime-ecosystem, Property 2: Theme consistency across applications
// Validates: Requirements 4.2
func TestProperty2_ThemeConsistencyAcrossApplications(t *testing.T) {
	// Force a deterministic, color-emitting profile so that color differences
	// are actually rendered into the fingerprint instead of being stripped in
	// a non-TTY test environment.
	lipgloss.SetColorProfile(termenv.TrueColor)

	rng := rand.New(rand.NewSource(0x7E5A1)) // fixed seed: reproducible failures
	pool := namePool(t, rng)

	for i := 0; i < propertyIterations; i++ {
		name := pool[rng.Intn(len(pool))]

		// Two independent callers (e.g. two different Runtime applications)
		// each resolve and build styles for the same theme name.
		callerA := resolveStyles(name)
		callerB := resolveStyles(name)

		fpA := fingerprint(callerA)
		fpB := fingerprint(callerB)

		if fpA != fpB {
			t.Fatalf("Property 2 violated: theme %q produced non-identical Styles across two independent callers\n caller A: %q\n caller B: %q",
				name, fpA, fpB)
		}
	}
}
