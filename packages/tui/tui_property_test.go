package tui

import (
	"testing"
	"testing/quick"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Feature: runtime-ecosystem, Property 1: Keyboard navigation is universally functional
//
// For any valid key binding in GlobalKeyMap, dispatching its keys produces the
// matching Action and renders feedback synchronously (no blocking).
//
// Validates: Requirements 1.1, 1.2, 1.3

// specialKeys maps the non-rune key strings used by DefaultGlobalKeyMap to the
// tea.KeyType that produces an equivalent tea.KeyMsg.String(). Anything not in
// this map is treated as a sequence of runes (e.g. "q", "k", "/").
var specialKeys = map[string]tea.KeyType{
	"ctrl+c": tea.KeyCtrlC,
	"up":     tea.KeyUp,
	"down":   tea.KeyDown,
	"left":   tea.KeyLeft,
	"right":  tea.KeyRight,
	"pgup":   tea.KeyPgUp,
	"pgdown": tea.KeyPgDown,
	"ctrl+u": tea.KeyCtrlU,
	"ctrl+d": tea.KeyCtrlD,
}

// keyMsgFromString reconstructs a tea.KeyMsg whose String() equals the supplied
// key string, mirroring how a terminal would deliver the key to the program.
func keyMsgFromString(s string) tea.KeyMsg {
	if t, ok := specialKeys[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// expectation pairs a single bound key with the Action it must dispatch to.
type expectation struct {
	key    string
	action Action
}

// buildExpectations enumerates every key bound in the supplied key map together
// with the Action that key must resolve to. This is the full valid input space
// for the property.
func buildExpectations(km GlobalKeyMap) []expectation {
	bindings := []struct {
		binding interface{ Keys() []string }
		action  Action
	}{
		{km.Quit, ActionQuit},
		{km.Help, ActionHelp},
		{km.Up, ActionUp},
		{km.Down, ActionDown},
		{km.Left, ActionLeft},
		{km.Right, ActionRight},
		{km.PageUp, ActionPageUp},
		{km.PageDown, ActionPageDown},
		{km.Top, ActionTop},
		{km.Bottom, ActionBottom},
		{km.Search, ActionSearch},
	}

	var out []expectation
	for _, b := range bindings {
		for _, k := range b.binding.Keys() {
			out = append(out, expectation{key: k, action: b.action})
		}
	}
	return out
}

// TestProperty1_KeyboardNavigationUniversallyFunctional verifies that for any
// valid key binding in GlobalKeyMap, dispatching its keys produces the matching
// Action, that the mapping is deterministic, and that Dispatch returns
// synchronously without blocking.
func TestProperty1_KeyboardNavigationUniversallyFunctional(t *testing.T) {
	km := DefaultGlobalKeyMap()
	expectations := buildExpectations(km)
	if len(expectations) == 0 {
		t.Fatal("expected at least one bound key in DefaultGlobalKeyMap")
	}

	// Property: a randomly chosen valid key always dispatches to its bound
	// Action, deterministically and synchronously.
	prop := func(pick uint16) bool {
		exp := expectations[int(pick)%len(expectations)]
		msg := keyMsgFromString(exp.key)

		// Synchronous, non-blocking dispatch: the call must complete promptly.
		done := make(chan Action, 1)
		go func() { done <- km.Dispatch(msg) }()

		var got Action
		select {
		case got = <-done:
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Dispatch blocked for key %q (action %v)", exp.key, exp.action)
			return false
		}

		// Correctness: the matched Action is the bound one.
		if got != exp.action {
			t.Errorf("Dispatch(%q) = %v, want %v", exp.key, got, exp.action)
			return false
		}

		// Determinism: dispatching the same key again yields the same Action.
		if again := km.Dispatch(msg); again != got {
			t.Errorf("Dispatch(%q) non-deterministic: %v then %v", exp.key, got, again)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Error(err)
	}
}
