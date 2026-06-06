// Package config provides shared configuration loading for Runtime applications.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// DefaultWatchInterval is the polling cadence used by Watch when no explicit
// interval is supplied. Polling keeps the watcher cross-platform and free of
// external dependencies.
const DefaultWatchInterval = time.Second

// ErrNotFound is returned when no config file exists yet (not fatal).
var ErrNotFound = errors.New("config file not found")

// PluginConfig contains plugin-specific settings shared across Runtime apps.
type PluginConfig struct {
	Paths   []string        `json:"paths"`   // directories to search for plugins
	Enabled map[string]bool `json:"enabled"` // per-plugin enable/disable state, keyed by plugin name
}

// BaseConfig holds fields common to every Runtime application.
type BaseConfig struct {
	Theme    string       `json:"theme"`     // "default", "light", or a custom theme name
	Mouse    bool         `json:"mouse"`     // enable mouse support
	LogLevel string       `json:"log_level"` // "debug" | "info" | "warn" | "error"
	Plugin   PluginConfig `json:"plugin"`    // plugin discovery and enablement settings
}

// DefaultBase returns sensible defaults.
func DefaultBase() BaseConfig {
	return BaseConfig{
		Theme:    "default",
		Mouse:    false,
		LogLevel: "warn",
		Plugin: PluginConfig{
			Paths:   []string{},
			Enabled: map[string]bool{},
		},
	}
}

// Dir returns the platform-appropriate config directory for a Runtime app.
//
//	Linux/macOS: $XDG_CONFIG_HOME/runtime/<app>  (fallback: ~/.config/runtime/<app>)
//	Windows:     %APPDATA%\runtime\<app>
func Dir(app string) (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("%%APPDATA%% not set")
		}
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "runtime", app), nil
}

// Load reads a JSON config file for the given app into dst.
// dst must be a pointer. Returns ErrNotFound if no file exists yet.
func Load(app string, dst any) error {
	dir, err := Dir(app)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

// Save writes dst as JSON to the config file for the given app.
func Save(app string, src any) error {
	dir, err := Dir(app)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, data, 0o600)
}

// Watch monitors the given app's config.json and invokes onChange with the
// freshly loaded BaseConfig whenever the file changes (created, modified, or
// removed and recreated). It applies saved changes without requiring a restart.
//
// Watch uses a polling strategy so it stays cross-platform and free of external
// dependencies. It returns a stop function that cancels the watcher and waits
// for the background goroutine to exit. The returned error is non-nil only when
// the config directory cannot be resolved.
//
// onChange is never called with a config that failed to parse; malformed files
// are skipped until they become valid again. Watch does not invoke onChange for
// the file's initial state, only for subsequent changes, so callers should Load
// the current config before starting the watch.
func Watch(app string, onChange func(BaseConfig)) (stop func(), err error) {
	return WatchInterval(app, DefaultWatchInterval, onChange)
}

// WatchInterval behaves like Watch but lets the caller control the polling
// cadence. A non-positive interval falls back to DefaultWatchInterval.
func WatchInterval(app string, interval time.Duration, onChange func(BaseConfig)) (stop func(), err error) {
	if onChange == nil {
		return nil, fmt.Errorf("config: onChange callback must not be nil")
	}
	dir, err := Dir(app)
	if err != nil {
		return nil, err
	}
	if interval <= 0 {
		interval = DefaultWatchInterval
	}
	path := filepath.Join(dir, "config.json")

	done := make(chan struct{})
	finished := make(chan struct{})
	last := fileState(path)

	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cur := fileState(path)
				if cur == last {
					continue
				}
				last = cur
				var cfg BaseConfig
				if loadErr := Load(app, &cfg); loadErr != nil {
					// Skip missing or malformed files; report once they are valid.
					continue
				}
				onChange(cfg)
			}
		}
	}()

	var once sync.Once
	stop = func() {
		once.Do(func() { close(done) })
		<-finished
	}
	return stop, nil
}

// fileState captures a lightweight fingerprint of the config file used to
// detect changes between polls. A zero value means the file is absent.
func fileState(path string) watchState {
	info, err := os.Stat(path)
	if err != nil {
		return watchState{}
	}
	return watchState{size: info.Size(), modTime: info.ModTime().UnixNano()}
}

// watchState is a comparable fingerprint of a config file's size and mtime.
type watchState struct {
	size    int64
	modTime int64
}
