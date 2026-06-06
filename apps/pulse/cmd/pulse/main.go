// Command pulse is the Runtime Pulse application.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runtime-sh/runtime/apps/pulse/internal/ui"
)

func main() {
	// Top-level panic handler: recover, write the stack trace to stderr using
	// the standard runtime-{app} error format, and exit with code 1.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "runtime-pulse: panic: %v\n\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	p := tea.NewProgram(ui.New(path), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime-pulse: %v\n", err)
		os.Exit(1)
	}
}
