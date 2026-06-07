package model_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runtime-sh/runtime/apps/grid/internal/model"
	gridds "github.com/runtime-sh/runtime/apps/grid/internal/model/datasource"
	"github.com/runtime-sh/runtime/packages/config"
	"github.com/runtime-sh/runtime/packages/theme"
)

// sampleCSV is a small dataset used by the import tests.
const sampleCSV = "id,name,role\n1,Ada,engineer\n2,Linus,maintainer\n3,Grace,admiral\n"

// writeSampleCSV writes sampleCSV to a temp file and returns its path.
func writeSampleCSV(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "people.csv")
	if err := os.WriteFile(path, []byte(sampleCSV), 0o600); err != nil {
		t.Fatalf("write sample csv: %v", err)
	}
	return path
}

// Test_GridStartup_LoadsConfigThemeAndImportsCSV exercises the happy path:
// resolving config + theme, constructing the model, and importing a CSV through
// the datasource. It validates Requirements 7.1 (import with fidelity), 10.1
// (config load), and 1.3 (data available for navigation/feedback).
func Test_GridStartup_LoadsConfigThemeAndImportsCSV(t *testing.T) {
	// Config + theme load: defaults resolve to a valid style set.
	cfg := config.DefaultBase()
	if err := config.Load("grid", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("load config: %v", err)
	}
	styles, err := theme.Apply(cfg.Theme)
	if err != nil {
		t.Fatalf("apply theme %q: %v", cfg.Theme, err)
	}

	m := model.New(styles)
	m.SetSize(80, 24)

	path := writeSampleCSV(t)
	if err := m.LoadFile(path); err != nil {
		t.Fatalf("load csv: %v", err)
	}
	defer func() { _ = m.Close() }()

	if !m.Loaded() {
		t.Fatal("expected model to report loaded after import")
	}
	if got := m.Table().RowCount(); got != 3 {
		t.Fatalf("row count = %d, want 3", got)
	}
	cols := m.Columns()
	if len(cols) != 3 {
		t.Fatalf("column count = %d, want 3", len(cols))
	}
	if cols[0].Name != "id" || cols[1].Name != "name" || cols[2].Name != "role" {
		t.Fatalf("unexpected columns: %+v", cols)
	}

	// Data fidelity: the first imported row is selectable and intact.
	row, ok := m.Table().SelectedRow()
	if !ok {
		t.Fatal("expected a selected row after import")
	}
	if row.Cells[1] != "Ada" {
		t.Fatalf("first row name = %q, want Ada", row.Cells[1])
	}

	// Search filters the table (case-insensitive).
	m.Search("grace")
	if got := m.Table().RowCount(); got != 1 {
		t.Fatalf("filtered row count = %d, want 1", got)
	}
	m.Search("")
	if got := m.Table().RowCount(); got != 3 {
		t.Fatalf("row count after clearing filter = %d, want 3", got)
	}
}

// Test_FailClosed_RefusesLaunchWhenFormatUnavailable validates the fail-closed
// launch policy of Requirement 7.1: when any required format fails to
// initialize, the launch check returns an error and the app must not launch.
func Test_FailClosed_RefusesLaunchWhenFormatUnavailable(t *testing.T) {
	// All five required formats (CSV, TSV, XLSX, Parquet, Arrow) are now backed
	// by real readers, so the default fail-closed check must allow launch.
	if err := gridds.CheckAvailability(); err != nil {
		t.Fatalf("expected launch to be allowed when all formats are available, got: %v", err)
	}

	// Simulate a build where a single required format fails to initialize:
	// launch must be refused and the offending format named.
	restoreP := gridds.SetAvailability("Parquet", false)
	defer restoreP()
	err := gridds.CheckAvailability()
	if err == nil {
		t.Fatal("expected CheckAvailability to fail when Parquet is unavailable")
	}
	if !strings.Contains(err.Error(), "Parquet") {
		t.Fatalf("error should name the unavailable Parquet format, got: %v", err)
	}

	// A second format failing must also be enumerated.
	restoreA := gridds.SetAvailability("Arrow", false)
	defer restoreA()
	err = gridds.CheckAvailability()
	if err == nil {
		t.Fatal("expected CheckAvailability to fail when Parquet and Arrow are unavailable")
	}
	if !strings.Contains(err.Error(), "Parquet") || !strings.Contains(err.Error(), "Arrow") {
		t.Fatalf("error should enumerate all unavailable formats, got: %v", err)
	}
}
