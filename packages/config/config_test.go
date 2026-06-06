package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDefaultBase(t *testing.T) {
	d := DefaultBase()
	if d.Theme != "default" {
		t.Errorf("Theme = %q, want %q", d.Theme, "default")
	}
	if d.Mouse {
		t.Errorf("Mouse = true, want false")
	}
	if d.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", d.LogLevel, "warn")
	}
	if d.Plugin.Paths == nil {
		t.Error("Plugin.Paths is nil, want non-nil slice")
	}
	if d.Plugin.Enabled == nil {
		t.Error("Plugin.Enabled is nil, want non-nil map")
	}
}

func TestDir(t *testing.T) {
	app := "testapp"
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join("C:", "Users", "test", "AppData", "Roaming"))
	} else {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	}

	dir, err := Dir(app)
	if err != nil {
		t.Fatalf("Dir returned error: %v", err)
	}

	if filepath.Base(dir) != app {
		t.Errorf("Dir base = %q, want %q", filepath.Base(dir), app)
	}
	if filepath.Base(filepath.Dir(dir)) != "runtime" {
		t.Errorf("Dir parent = %q, want %q", filepath.Base(filepath.Dir(dir)), "runtime")
	}
}

func TestDirExplicitXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG_CONFIG_HOME resolution is for unix-like systems")
	}
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	dir, err := Dir("grid")
	if err != nil {
		t.Fatalf("Dir returned error: %v", err)
	}
	want := filepath.Join("/tmp/xdg", "runtime", "grid")
	if dir != want {
		t.Errorf("Dir = %q, want %q", dir, want)
	}
}

func TestDirWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("APPDATA resolution is windows-specific")
	}
	appData := filepath.Join("C:", "Users", "test", "AppData", "Roaming")
	t.Setenv("APPDATA", appData)
	dir, err := Dir("vault")
	if err != nil {
		t.Fatalf("Dir returned error: %v", err)
	}
	want := filepath.Join(appData, "runtime", "vault")
	if dir != want {
		t.Errorf("Dir = %q, want %q", dir, want)
	}
}

func TestDirWindowsMissingAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("APPDATA error path is windows-specific")
	}
	t.Setenv("APPDATA", "")
	if _, err := Dir("vault"); err == nil {
		t.Error("Dir returned nil error with APPDATA unset, want error")
	}
}

func TestDirXDGFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fallback test is for unix-like systems")
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	dir, err := Dir("grid")
	if err != nil {
		t.Fatalf("Dir returned error: %v", err)
	}
	want := filepath.Join(home, ".config", "runtime", "grid")
	if dir != want {
		t.Errorf("Dir = %q, want %q", dir, want)
	}
}

func TestLoadNotFound(t *testing.T) {
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}
	var c BaseConfig
	err := Load("missingapp", &c)
	if err != ErrNotFound {
		t.Errorf("Load error = %v, want ErrNotFound", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}

	app := "roundtrip"
	want := DefaultBase()
	want.Mouse = true
	want.Theme = "light"
	want.Plugin.Paths = []string{"/opt/plugins", "~/.runtime/plugins"}
	want.Plugin.Enabled = map[string]bool{"foo": true, "bar": false}

	if err := Save(app, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var got BaseConfig
	if err := Load(app, &got); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Theme != want.Theme || got.Mouse != want.Mouse || got.LogLevel != want.LogLevel {
		t.Errorf("base fields mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Plugin.Paths) != len(want.Plugin.Paths) {
		t.Fatalf("Plugin.Paths len = %d, want %d", len(got.Plugin.Paths), len(want.Plugin.Paths))
	}
	for i, p := range want.Plugin.Paths {
		if got.Plugin.Paths[i] != p {
			t.Errorf("Plugin.Paths[%d] = %q, want %q", i, got.Plugin.Paths[i], p)
		}
	}
	for k, v := range want.Plugin.Enabled {
		if got.Plugin.Enabled[k] != v {
			t.Errorf("Plugin.Enabled[%q] = %v, want %v", k, got.Plugin.Enabled[k], v)
		}
	}
}

func TestSavePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not meaningful on windows")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	app := "permapp"
	if err := Save(app, DefaultBase()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	dir, _ := Dir(app)
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perm = %o, want 600", perm)
	}
}

func TestSaveDirPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not meaningful on windows")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	app := "permdirapp"
	if err := Save(app, DefaultBase()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	dir, _ := Dir(app)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir perm = %o, want 700", perm)
	}
}

func TestPluginConfigJSONTags(t *testing.T) {
	c := DefaultBase()
	c.Plugin.Paths = []string{"/a"}
	c.Plugin.Enabled = map[string]bool{"x": true}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	plugin, ok := raw["plugin"].(map[string]any)
	if !ok {
		t.Fatalf("missing 'plugin' object in JSON: %s", data)
	}
	if _, ok := plugin["paths"]; !ok {
		t.Errorf("missing 'paths' key in plugin JSON: %s", data)
	}
	if _, ok := plugin["enabled"]; !ok {
		t.Errorf("missing 'enabled' key in plugin JSON: %s", data)
	}
}

func TestWatchNilCallback(t *testing.T) {
	if _, err := Watch("watchapp", nil); err == nil {
		t.Error("Watch with nil callback should return an error")
	}
}

func TestWatchDetectsSavedChanges(t *testing.T) {
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}

	app := "watchapp"

	// Seed an initial config so the watcher starts from a known state.
	initial := DefaultBase()
	if err := Save(app, initial); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	updates := make(chan BaseConfig, 1)
	stop, err := WatchInterval(app, 10*time.Millisecond, func(c BaseConfig) {
		select {
		case updates <- c:
		default:
		}
	})
	if err != nil {
		t.Fatalf("WatchInterval: %v", err)
	}
	defer stop()

	// Save a changed config; the watcher should observe it without a restart.
	changed := DefaultBase()
	changed.Theme = "light"
	changed.Mouse = true
	// Sleep a moment so the modtime/size fingerprint reliably differs.
	time.Sleep(20 * time.Millisecond)
	if err := Save(app, changed); err != nil {
		t.Fatalf("Save changed: %v", err)
	}

	select {
	case got := <-updates:
		if got.Theme != "light" || !got.Mouse {
			t.Errorf("onChange got %+v, want Theme=light Mouse=true", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for config change notification")
	}
}

func TestWatchStopIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}

	stop, err := WatchInterval("watchstop", 10*time.Millisecond, func(BaseConfig) {})
	if err != nil {
		t.Fatalf("WatchInterval: %v", err)
	}
	stop()
	stop() // second call must not panic or block
}

func TestWatchIgnoresMalformedFile(t *testing.T) {
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", tmp)
	} else {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}

	app := "watchbad"
	dir, err := Dir(app)
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "config.json")

	called := make(chan BaseConfig, 1)
	stop, err := WatchInterval(app, 10*time.Millisecond, func(c BaseConfig) {
		called <- c
	})
	if err != nil {
		t.Fatalf("WatchInterval: %v", err)
	}
	defer stop()

	// Write a malformed config; onChange must not fire for it.
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case c := <-called:
		t.Fatalf("onChange fired for malformed file: %+v", c)
	case <-time.After(200 * time.Millisecond):
		// expected: no callback for malformed content
	}

	// Now write valid config; the watcher should recover and fire.
	valid := DefaultBase()
	valid.Theme = "light"
	if err := Save(app, valid); err != nil {
		t.Fatalf("Save valid: %v", err)
	}

	select {
	case c := <-called:
		if c.Theme != "light" {
			t.Errorf("onChange Theme = %q, want light", c.Theme)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for valid config notification")
	}
}
