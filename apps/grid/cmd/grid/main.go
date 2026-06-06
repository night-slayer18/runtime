// Command grid is the Runtime Grid application.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	gridds "github.com/runtime-sh/runtime/apps/grid/internal/model/datasource"
	"github.com/runtime-sh/runtime/apps/grid/internal/ui"
)

func main() {
	// Top-level panic handler: recover, write the stack trace to stderr using
	// the standard runtime-{app} error format, and exit with code 1.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "runtime-grid: panic: %v\n\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime-grid: %v\n", err)
		os.Exit(1)
	}
}

// run wires up the program and enforces the fail-closed launch policy.
//
// Per Requirement 7.1, Grid must support CSV, TSV, XLSX, Parquet, and Arrow,
// and if any required format fails to initialize the application SHALL NOT
// launch until all formats are available. CheckAvailability returns a non-nil
// error enumerating any unavailable format; we refuse to launch in that case
// rather than starting a UI that can only import some of its formats.
func run() error {
	if err := gridds.CheckAvailability(); err != nil {
		return err
	}

	// The first non-flag argument, if present, is the file to import on startup.
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	p := tea.NewProgram(ui.New(path), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
