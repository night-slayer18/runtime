// Command strata is the Runtime Strata application.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runtime-sh/runtime/apps/strata/internal/ui"
)

func main() {
	// Top-level panic handler: recover, write the stack trace to stderr using
	// the standard runtime-{app} error format, and exit with code 1.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "runtime-strata: panic: %v\n\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime-strata: %v\n", err)
		os.Exit(1)
	}
}

// run wires up the program. The first non-flag argument, if present, is the
// connection string Strata connects to on startup (for example
// "sqlite:file:data.db", "postgres://user:pass@host/db", or
// "mysql://user:pass@tcp(host:3306)/db"). The scheme selects the database
// backend via the datasource registry; configuration and theme are loaded by
// the UI on startup.
func run() error {
	var conn string
	if len(os.Args) > 1 {
		conn = os.Args[1]
	}

	p := tea.NewProgram(ui.New(conn), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
