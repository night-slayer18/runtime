// Command vault is the Runtime Vault application.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runtime-sh/runtime/apps/vault/internal/ui"
)

func main() {
	// Top-level panic handler: recover, write the stack trace to stderr using
	// the standard runtime-{app} error format, and exit with code 1.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "runtime-vault: panic: %v\n\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	p := tea.NewProgram(ui.New(path), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime-vault: %v\n", err)
		os.Exit(1)
	}
}
