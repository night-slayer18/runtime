package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetBuiltinThemes(t *testing.T) {
	for _, name := range []string{"default", "light"} {
		if _, ok := Get(name); !ok {
			t.Fatalf("expected built-in theme %q to be registered", name)
		}
	}
}

func TestGetUnknownTheme(t *testing.T) {
	if _, ok := Get("does-not-exist"); ok {
		t.Fatal("expected unknown theme lookup to report not found")
	}
}

func TestResolveFallsBackToDefault(t *testing.T) {
	if got := Resolve("does-not-exist"); got != Default {
		t.Fatal("expected Resolve to fall back to Default for unknown theme")
	}
	if got := Resolve(""); got != Default {
		t.Fatal("expected Resolve to fall back to Default for empty name")
	}
}

func TestResolveKnownTheme(t *testing.T) {
	if got := Resolve("light"); got != Light {
		t.Fatal("expected Resolve(\"light\") to return the Light palette")
	}
}

func TestRegisterCustomTheme(t *testing.T) {
	custom := Default
	custom.Primary = Light.Accent
	Register("custom-test", custom)
	defer delete(registry, "custom-test")

	got, ok := Get("custom-test")
	if !ok {
		t.Fatal("expected registered custom theme to be found")
	}
	if got.Primary != custom.Primary {
		t.Fatal("expected registered custom theme to retain its palette values")
	}
}

func TestNamesIncludesBuiltins(t *testing.T) {
	names := Names()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["default"] || !found["light"] {
		t.Fatalf("expected Names to include built-in themes, got %v", names)
	}
}

func TestNewStylesDerivesFromPalette(t *testing.T) {
	styles := NewStyles(Light)
	if styles.Title.GetForeground() != Light.Primary {
		t.Fatal("expected Title style foreground to derive from palette Primary")
	}
}

func TestApplyBuiltinTheme(t *testing.T) {
	styles, err := Apply("light")
	if err != nil {
		t.Fatalf("expected Apply(\"light\") to succeed, got %v", err)
	}
	if styles.Title.GetForeground() != Light.Primary {
		t.Fatal("expected applied Styles to derive from the Light palette")
	}
}

func TestApplyUnknownTheme(t *testing.T) {
	if _, err := Apply("does-not-exist"); err == nil {
		t.Fatal("expected Apply to return an error for an unknown theme")
	}
	if _, err := Apply(""); err == nil {
		t.Fatal("expected Apply to return an error for an empty name")
	}
}

func TestApplyMatchesNewStyles(t *testing.T) {
	got, err := Apply("default")
	if err != nil {
		t.Fatalf("expected Apply(\"default\") to succeed, got %v", err)
	}
	want := NewStyles(Default)
	if got.Title.GetForeground() != want.Title.GetForeground() ||
		got.Selected.GetBackground() != want.Selected.GetBackground() {
		t.Fatal("expected Apply to produce the same Styles as NewStyles(Default)")
	}
}

func TestLoadFileParsesPalette(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ocean.json")
	contents := `{
		"primary": { "light": "#112233", "dark": "#445566" },
		"background": { "light": "#FFFFFF", "dark": "#000000" }
	}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}

	p, err := LoadFile(path)
	if err != nil {
		t.Fatalf("expected LoadFile to succeed, got %v", err)
	}
	if p.Primary.Light != "#112233" || p.Primary.Dark != "#445566" {
		t.Fatalf("unexpected Primary color: %+v", p.Primary)
	}
	if p.Background.Light != "#FFFFFF" || p.Background.Dark != "#000000" {
		t.Fatalf("unexpected Background color: %+v", p.Background)
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := LoadFile(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected LoadFile to return an error for a missing file")
	}
}

func TestLoadFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected LoadFile to return an error for invalid JSON")
	}
}

func TestLoadFileThenApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	contents := `{ "primary": { "light": "#AABBCC", "dark": "#AABBCC" } }`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}

	p, err := LoadFile(path)
	if err != nil {
		t.Fatalf("expected LoadFile to succeed, got %v", err)
	}
	Register("custom-loadfile", p)
	defer delete(registry, "custom-loadfile")

	styles, err := Apply("custom-loadfile")
	if err != nil {
		t.Fatalf("expected Apply on the registered custom theme to succeed, got %v", err)
	}
	if styles.Title.GetForeground() != p.Primary {
		t.Fatal("expected applied Styles to use the loaded custom palette")
	}
}
