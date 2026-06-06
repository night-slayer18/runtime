package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg builds a tea.KeyMsg from a binding-style key string. Single-rune keys
// (e.g. "k", "q", "/") are sent as runes; named keys (e.g. "up", "pgup",
// "ctrl+d") are matched by their tea.KeyType so key.Matches resolves them the
// same way Bubble Tea would at runtime.
func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		// Treat as a literal rune sequence (handles "k", "j", "g", "G", "q",
		// "?", "/", etc).
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

// TestDefaultGlobalKeyMapExposesCommonActions asserts the canonical key map
// binds every common action the Runtime ecosystem relies on: navigation
// (up/down/left/right, page up/down, top/bottom), search, help, and quit.
//
// _Requirements: 4.1_
func TestDefaultGlobalKeyMapExposesCommonActions(t *testing.T) {
	km := DefaultGlobalKeyMap()

	cases := []struct {
		name    string
		binding key.Binding
		want    []string
	}{
		{"Up", km.Up, []string{"up", "k"}},
		{"Down", km.Down, []string{"down", "j"}},
		{"Left", km.Left, []string{"left", "h"}},
		{"Right", km.Right, []string{"right", "l"}},
		{"PageUp", km.PageUp, []string{"pgup", "ctrl+u"}},
		{"PageDown", km.PageDown, []string{"pgdown", "ctrl+d"}},
		{"Top", km.Top, []string{"g"}},
		{"Bottom", km.Bottom, []string{"G"}},
		{"Search", km.Search, []string{"/"}},
		{"Help", km.Help, []string{"?"}},
		{"Quit", km.Quit, []string{"q", "ctrl+c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys := tc.binding.Keys()
			if len(keys) == 0 {
				t.Fatalf("%s binding exposes no keys", tc.name)
			}
			got := make(map[string]bool, len(keys))
			for _, k := range keys {
				got[k] = true
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("%s binding missing expected key %q; have %v", tc.name, w, keys)
				}
			}
		})
	}
}

// TestDefaultGlobalKeyMapCoversNavigationSearchHelpQuit asserts the map binds
// at least one key for each common-action category, guarding against a category
// being dropped entirely.
//
// _Requirements: 4.1_
func TestDefaultGlobalKeyMapCoversNavigationSearchHelpQuit(t *testing.T) {
	km := DefaultGlobalKeyMap()

	categories := map[string]key.Binding{
		"navigation:up":       km.Up,
		"navigation:down":     km.Down,
		"navigation:left":     km.Left,
		"navigation:right":    km.Right,
		"navigation:pageUp":   km.PageUp,
		"navigation:pageDown": km.PageDown,
		"navigation:top":      km.Top,
		"navigation:bottom":   km.Bottom,
		"search":              km.Search,
		"help":                km.Help,
		"quit":                km.Quit,
	}

	for name, b := range categories {
		if len(b.Keys()) == 0 {
			t.Errorf("common action %q has no bound keys", name)
		}
	}
}

// TestTwoKeymapsResolveIdentically asserts that two independently constructed
// key maps produce identical Dispatch results for the same inputs, which is the
// guarantee that lets every Runtime application share consistent navigation.
//
// _Requirements: 4.1_
func TestTwoKeymapsResolveIdentically(t *testing.T) {
	a := DefaultGlobalKeyMap()
	b := DefaultGlobalKeyMap()

	// Every key referenced by the canonical bindings, plus a couple of inputs
	// that should resolve to ActionNone.
	inputs := []string{
		"up", "k", "down", "j", "left", "h", "right", "l",
		"pgup", "ctrl+u", "pgdown", "ctrl+d",
		"g", "G", "/", "?", "q", "ctrl+c",
		"x", "z", // unmatched
	}

	for _, in := range inputs {
		msg := keyMsg(in)
		ga := a.Dispatch(msg)
		gb := b.Dispatch(msg)
		if ga != gb {
			t.Errorf("dispatch mismatch for input %q: a=%v b=%v", in, ga, gb)
		}
	}
}

// TestDispatchMapsCommonActions verifies each common-action key resolves to the
// expected Action via Dispatch, confirming the keymap is wired end to end.
//
// _Requirements: 4.1_
func TestDispatchMapsCommonActions(t *testing.T) {
	km := DefaultGlobalKeyMap()

	cases := []struct {
		input string
		want  Action
	}{
		{"up", ActionUp},
		{"k", ActionUp},
		{"down", ActionDown},
		{"j", ActionDown},
		{"left", ActionLeft},
		{"h", ActionLeft},
		{"right", ActionRight},
		{"l", ActionRight},
		{"pgup", ActionPageUp},
		{"ctrl+u", ActionPageUp},
		{"pgdown", ActionPageDown},
		{"ctrl+d", ActionPageDown},
		{"g", ActionTop},
		{"G", ActionBottom},
		{"/", ActionSearch},
		{"?", ActionHelp},
		{"q", ActionQuit},
		{"ctrl+c", ActionQuit},
		{"x", ActionNone},
	}

	for _, tc := range cases {
		got := km.Dispatch(keyMsg(tc.input))
		if got != tc.want {
			t.Errorf("Dispatch(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestPackageDispatchMatchesKeyMapDispatch asserts the package-level Dispatch
// convenience wrapper agrees with an independently constructed key map, so
// callers that hold their own map and callers that don't resolve identically.
//
// _Requirements: 4.1_
func TestPackageDispatchMatchesKeyMapDispatch(t *testing.T) {
	km := DefaultGlobalKeyMap()

	inputs := []string{
		"up", "k", "down", "j", "left", "h", "right", "l",
		"pgup", "ctrl+u", "pgdown", "ctrl+d",
		"g", "G", "/", "?", "q", "ctrl+c", "x",
	}

	for _, in := range inputs {
		msg := keyMsg(in)
		if got, want := Dispatch(msg), km.Dispatch(msg); got != want {
			t.Errorf("package Dispatch(%q) = %v, key map Dispatch = %v", in, got, want)
		}
	}
}
