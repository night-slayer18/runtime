package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// Feature: runtime-ecosystem, Property 2 & 3: theme and config consistency across apps
//
// Config half (Property 3): All five Runtime applications (grid, prism, pulse,
// strata, vault) load and save configuration through this single shared package
// with only their app name differing. So for any BaseConfig saved under one app
// name, loading it under each of the five app names must either succeed with
// identical values or fail with a clear, typed error. This models "all five
// apps accept/reject the same config consistently".
//
// Validates: Requirements 4.3

// ecosystemApps is the fixed set of Runtime applications. Each loads config via
// this package using only its own name, so the schema is shared by construction.
var ecosystemApps = []string{"grid", "prism", "pulse", "strata", "vault"}

// TestEcosystem_ConfigConsistencyAcrossFiveApps asserts that a config authored
// for one Runtime app is handled identically by all five apps: every app either
// loads the exact same values or rejects the file with a clear, typed error.
//
// Feature: runtime-ecosystem, Property 2 & 3: theme and config consistency across apps
// Validates: Requirements 4.3
func TestEcosystem_ConfigConsistencyAcrossFiveApps(t *testing.T) {
	const iterations = 200 // exceeds the design's 100-iteration minimum

	r := rand.New(rand.NewSource(0xEC05C0)) // fixed seed -> reproducible failures

	for i := 0; i < iterations; i++ {
		i := i
		t.Run(fmt.Sprintf("iter_%03d", i), func(t *testing.T) {
			root := t.TempDir()
			setConfigHome(t, root)

			want := genBaseConfig(r)

			// The author app whose config every other app will read.
			author := ecosystemApps[i%len(ecosystemApps)]
			if err := Save(author, want); err != nil {
				t.Fatalf("Save under %q: %v", author, err)
			}

			authorDir, err := Dir(author)
			if err != nil {
				t.Fatalf("Dir(%q): %v", author, err)
			}
			shared, err := os.ReadFile(filepath.Join(authorDir, "config.json"))
			if err != nil {
				t.Fatalf("read author config: %v", err)
			}

			// Place the identical config file under every app's directory,
			// simulating a user sharing one config across the whole ecosystem.
			for _, app := range ecosystemApps {
				dir, err := Dir(app)
				if err != nil {
					t.Fatalf("Dir(%q): %v", app, err)
				}
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatalf("mkdir %q: %v", dir, err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.json"), shared, 0o600); err != nil {
					t.Fatalf("write %q config: %v", app, err)
				}
			}

			// Every app loads the shared config. Record how each app resolved
			// it so we can also assert the five apps agree with one another.
			type outcome struct {
				cfg     BaseConfig
				loadErr error
			}
			outcomes := make(map[string]outcome, len(ecosystemApps))

			for _, app := range ecosystemApps {
				var got BaseConfig
				loadErr := Load(app, &got)
				outcomes[app] = outcome{cfg: got, loadErr: loadErr}

				switch {
				case loadErr == nil:
					// Success path: values must match the authored config.
					if !jsonEqual(t, want, got) {
						wb, _ := json.Marshal(want)
						gb, _ := json.Marshal(got)
						t.Fatalf("app %q loaded mismatched config:\n want=%s\n  got=%s", app, wb, gb)
					}
				case isClearTypedError(loadErr):
					// Acceptable: rejected with a clear, typed error.
				default:
					t.Fatalf("app %q returned an unclear error: %v", app, loadErr)
				}
			}

			// Cross-app agreement: all five apps must reach the same verdict
			// (all succeed, or all reject) for one identical config file.
			ref := ecosystemApps[0]
			refOK := outcomes[ref].loadErr == nil
			for _, app := range ecosystemApps[1:] {
				if (outcomes[app].loadErr == nil) != refOK {
					t.Fatalf("apps disagree on identical config: %q ok=%v but %q ok=%v",
						ref, refOK, app, outcomes[app].loadErr == nil)
				}
			}
		})
	}
}
