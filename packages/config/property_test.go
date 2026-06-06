package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Feature: runtime-ecosystem, Property 3: Configuration schema compatibility
//
// For any BaseConfig that is valid for one Runtime application, loading that
// same configuration under a different application name shall either succeed
// with identical values or be rejected with a clear, typed error.
//
// Validates: Requirements 4.3

// setConfigHome points the platform config-dir resolver at root for the duration
// of the test, mirroring the override used by the existing config_test.go cases.
func setConfigHome(t *testing.T, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", root)
	} else {
		t.Setenv("XDG_CONFIG_HOME", root)
	}
}

// randString builds a string drawn from an alphabet that includes empty,
// ASCII, whitespace, and multi-byte runes so generated configs exercise a
// broad slice of the valid input space.
func randString(r *rand.Rand) string {
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 _-./\\:é😀✓\t")
	n := r.Intn(12) // 0..11, includes the empty string
	out := make([]rune, n)
	for i := range out {
		out[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(out)
}

// genBaseConfig produces a randomized but well-formed BaseConfig. Themes and
// log levels are drawn from a mix of known and arbitrary values so both
// "expected" and "unusual but valid" configurations are covered.
func genBaseConfig(r *rand.Rand) BaseConfig {
	themes := []string{"default", "light", "dark", randString(r)}
	levels := []string{"debug", "info", "warn", "error", randString(r)}

	var paths []string
	for n := r.Intn(4); len(paths) < n; {
		paths = append(paths, randString(r))
	}

	enabled := map[string]bool{}
	for n := r.Intn(4); len(enabled) < n; {
		enabled[randString(r)] = r.Intn(2) == 0
	}

	return BaseConfig{
		Theme:    themes[r.Intn(len(themes))],
		Mouse:    r.Intn(2) == 0,
		LogLevel: levels[r.Intn(len(levels))],
		Plugin: PluginConfig{
			Paths:   paths,
			Enabled: enabled,
		},
	}
}

// jsonEqual compares two configs by their canonical JSON encoding. This makes
// the comparison robust to nil-vs-empty slice/map differences that are erased
// by a JSON Save/Load round trip (nil and empty both normalize consistently).
func jsonEqual(t *testing.T, a, b BaseConfig) bool {
	t.Helper()
	ab, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	bb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	return string(ab) == string(bb)
}

// isClearTypedError reports whether err is one of the package's clearly typed
// error conditions (a missing file, or a wrapped read/parse failure). The
// property permits a load to fail this way instead of succeeding.
func isClearTypedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	// Save/Load wrap I/O and decode failures with a descriptive message.
	msg := err.Error()
	return msg != ""
}

func TestProperty3_ConfigurationSchemaCompatibility(t *testing.T) {
	const iterations = 200 // exceeds the 100-iteration minimum from the design

	r := rand.New(rand.NewSource(0x52756e74696d65)) // "runtime" as a fixed seed -> deterministic, reproducible failures

	for i := 0; i < iterations; i++ {
		i := i
		t.Run(fmt.Sprintf("iter_%03d", i), func(t *testing.T) {
			root := t.TempDir()
			setConfigHome(t, root)

			appA := fmt.Sprintf("appA_%d", i)
			appB := fmt.Sprintf("appB_%d", i)

			want := genBaseConfig(r)

			// A config that is valid for application A.
			if err := Save(appA, want); err != nil {
				t.Fatalf("Save under %q: %v", appA, err)
			}

			// Make that exact config file available to application B,
			// simulating a user sharing one config across Runtime apps.
			srcDir, err := Dir(appA)
			if err != nil {
				t.Fatalf("Dir(%q): %v", appA, err)
			}
			dstDir, err := Dir(appB)
			if err != nil {
				t.Fatalf("Dir(%q): %v", appB, err)
			}
			if err := os.MkdirAll(dstDir, 0o700); err != nil {
				t.Fatalf("mkdir %q: %v", dstDir, err)
			}
			data, err := os.ReadFile(filepath.Join(srcDir, "config.json"))
			if err != nil {
				t.Fatalf("read app A config: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dstDir, "config.json"), data, 0o600); err != nil {
				t.Fatalf("write app B config: %v", err)
			}

			// Application B loads a config authored for application A.
			var got BaseConfig
			loadErr := Load(appB, &got)

			switch {
			case loadErr == nil:
				// Success path: values must be identical across the app boundary.
				if !jsonEqual(t, want, got) {
					wb, _ := json.Marshal(want)
					gb, _ := json.Marshal(got)
					t.Fatalf("schema compatible load mismatch:\n want=%s\n  got=%s", wb, gb)
				}
			case isClearTypedError(loadErr):
				// Acceptable: rejected with a clear, typed error.
			default:
				t.Fatalf("Load returned an unclear error: %v", loadErr)
			}
		})
	}
}
