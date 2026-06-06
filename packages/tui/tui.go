// Package tui provides shared Bubble Tea primitives for Runtime applications.
package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ----------------------------------------------------------------------------
// Common messages
// ----------------------------------------------------------------------------

// QuitMsg signals all models to cleanly shut down.
type QuitMsg struct{}

// ErrMsg wraps an error for propagation through the Bubble Tea update loop.
type ErrMsg struct{ Err error }

func (e ErrMsg) Error() string { return e.Err.Error() }

// ResizeMsg is sent when the terminal is resized.
type ResizeMsg struct {
	Width  int
	Height int
}

// ----------------------------------------------------------------------------
// Shared key bindings
// ----------------------------------------------------------------------------

// GlobalKeyMap defines key bindings that every Runtime application honours.
type GlobalKeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Search   key.Binding
}

// DefaultGlobalKeyMap returns the canonical Runtime key bindings.
func DefaultGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	}
}

// ----------------------------------------------------------------------------
// Dispatch
// ----------------------------------------------------------------------------

// Action identifies a universal navigation/command action matched from a key
// press. It lets every Runtime application share identical navigation handling
// by switching on a single value instead of re-implementing key.Matches checks.
type Action int

const (
	// ActionNone indicates the key did not match any global binding.
	ActionNone Action = iota
	ActionQuit
	ActionHelp
	ActionUp
	ActionDown
	ActionLeft
	ActionRight
	ActionPageUp
	ActionPageDown
	ActionTop
	ActionBottom
	ActionSearch
)

// String returns a human-readable name for the action.
func (a Action) String() string {
	switch a {
	case ActionQuit:
		return "quit"
	case ActionHelp:
		return "help"
	case ActionUp:
		return "up"
	case ActionDown:
		return "down"
	case ActionLeft:
		return "left"
	case ActionRight:
		return "right"
	case ActionPageUp:
		return "page-up"
	case ActionPageDown:
		return "page-down"
	case ActionTop:
		return "top"
	case ActionBottom:
		return "bottom"
	case ActionSearch:
		return "search"
	default:
		return "none"
	}
}

// Dispatch maps a tea.KeyMsg to the matching global Action using the supplied
// key map. It returns ActionNone when no binding matches. Applications can call
// Dispatch to obtain identical navigation handling without duplicating the
// underlying key.Matches checks. Bindings are evaluated in a fixed order so the
// result is deterministic.
func (k GlobalKeyMap) Dispatch(msg tea.KeyMsg) Action {
	switch {
	case key.Matches(msg, k.Quit):
		return ActionQuit
	case key.Matches(msg, k.Help):
		return ActionHelp
	case key.Matches(msg, k.Up):
		return ActionUp
	case key.Matches(msg, k.Down):
		return ActionDown
	case key.Matches(msg, k.Left):
		return ActionLeft
	case key.Matches(msg, k.Right):
		return ActionRight
	case key.Matches(msg, k.PageUp):
		return ActionPageUp
	case key.Matches(msg, k.PageDown):
		return ActionPageDown
	case key.Matches(msg, k.Top):
		return ActionTop
	case key.Matches(msg, k.Bottom):
		return ActionBottom
	case key.Matches(msg, k.Search):
		return ActionSearch
	default:
		return ActionNone
	}
}

// Dispatch maps a tea.KeyMsg to a matching Action using the canonical
// DefaultGlobalKeyMap. It is a convenience wrapper for callers that do not hold
// their own key map instance.
func Dispatch(msg tea.KeyMsg) Action {
	return DefaultGlobalKeyMap().Dispatch(msg)
}

// ----------------------------------------------------------------------------
// Size helper
// ----------------------------------------------------------------------------

// Size represents terminal dimensions passed through models.
type Size struct {
	Width  int
	Height int
}

// WindowSizeMsg converts a tea.WindowSizeMsg into a tui.Size.
func WindowSizeMsg(msg tea.WindowSizeMsg) Size {
	return Size{Width: msg.Width, Height: msg.Height}
}

// ----------------------------------------------------------------------------
// StatusBar
// ----------------------------------------------------------------------------

// StatusItem is a key/value pair rendered in the status bar.
type StatusItem struct {
	Key   string
	Value string
}
