// Package keymap defines vault-specific key bindings.
package keymap

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/runtime-sh/runtime/packages/tui"
)

type KeyMap struct{ tui.GlobalKeyMap }

func Default() KeyMap { return KeyMap{GlobalKeyMap: tui.DefaultGlobalKeyMap()} }

func (k KeyMap) ShortHelp() []key.Binding { return []key.Binding{k.Search, k.Help, k.Quit} }
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.PageUp, k.PageDown, k.Top, k.Bottom},
		{k.Search, k.Help, k.Quit},
	}
}
