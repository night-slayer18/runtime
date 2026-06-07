// Package model defines the core data model for runtime-grid.
//
// GridModel binds the shared component packages (table, search, datasource,
// export) into the application's state. It owns the loaded DataSource, the
// virtualized table that renders it, the active search, and the selection/edit
// state, and it exposes the operations the UI layer drives: loading a file,
// navigating, searching, and exporting.
package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gridds "github.com/runtime-sh/runtime/apps/grid/internal/model/datasource"
	ds "github.com/runtime-sh/runtime/packages/datasource"
	"github.com/runtime-sh/runtime/packages/export"
	"github.com/runtime-sh/runtime/packages/search"
	"github.com/runtime-sh/runtime/packages/table"
	"github.com/runtime-sh/runtime/packages/theme"
)

// Selection tracks the currently selected cell within the table: the row index
// within the active view and the column index.
type Selection struct {
	Row int
	Col int
}

// GridModel holds all application state for runtime-grid.
type GridModel struct {
	// source is the currently loaded data source, or nil when no file is open.
	source ds.DataSource
	// columns is the schema of the loaded source.
	columns []ds.Column
	// table is the virtualized table view bound to the loaded rows.
	table *table.Table
	// search performs case-insensitive filtering over the table.
	search *search.Searcher
	// selection is the active cell.
	selection Selection
	// editMode reports whether the model is in cell-edit mode.
	editMode bool
	// path is the file path of the loaded source, if any.
	path string
	// styles is the active theme style set used by the table.
	styles theme.Styles
}

// New returns a GridModel with the given theme styles and an empty table ready
// to receive data.
func New(styles theme.Styles) *GridModel {
	return &GridModel{
		table:  table.New(styles),
		search: search.New(),
		styles: styles,
	}
}

// Table exposes the underlying table for the UI to render.
func (m *GridModel) Table() *table.Table { return m.table }

// Path returns the path of the loaded file, or "" when nothing is loaded.
func (m *GridModel) Path() string { return m.path }

// Loaded reports whether a data source is currently loaded.
func (m *GridModel) Loaded() bool { return m.source != nil }

// Columns returns the schema of the loaded source.
func (m *GridModel) Columns() []ds.Column { return m.columns }

// EditMode reports whether the model is in cell-edit mode.
func (m *GridModel) EditMode() bool { return m.editMode }

// ToggleEdit flips edit mode and returns the new state.
func (m *GridModel) ToggleEdit() bool {
	m.editMode = !m.editMode
	return m.editMode
}

// SetStyles updates the theme styles applied to the table. This makes a live
// theme change a single bounded operation.
func (m *GridModel) SetStyles(styles theme.Styles) {
	m.styles = styles
	m.table.SetStyles(styles)
}

// SetSize forwards the viewport dimensions to the table.
func (m *GridModel) SetSize(width, height int) {
	m.table.SetSize(width, height)
}

// LoadFile imports the file at path through the Grid datasource registry and
// binds the resulting rows to the table. It replaces any previously loaded
// source (closing it first) and resets navigation and search state.
func (m *GridModel) LoadFile(path string) error {
	source, err := gridds.Open(path)
	if err != nil {
		return fmt.Errorf("import %s: %w", filepath.Base(path), err)
	}
	if err := m.bind(source); err != nil {
		_ = source.Close()
		return err
	}
	m.path = path
	return nil
}

// bind loads the schema and all rows from source into the table. Grid uses the
// in-memory datasource, so reading every row up front is expected and bounded
// by the imported file.
func (m *GridModel) bind(source ds.DataSource) error {
	columns, err := source.Schema()
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	rows, err := readAllRows(source, len(columns))
	if err != nil {
		return err
	}

	if m.source != nil {
		_ = m.source.Close()
	}
	m.source = source
	m.columns = columns
	m.search = search.New()
	m.selection = Selection{}
	m.editMode = false

	m.table.SetData(rows, toTableColumns(columns))
	return nil
}

// readAllRows drains every row from the source's default query into table rows.
func readAllRows(source ds.DataSource, colCount int) ([]table.Row, error) {
	it, err := source.Query("")
	if err != nil {
		return nil, fmt.Errorf("query rows: %w", err)
	}
	defer func() { _ = it.Close() }()

	var rows []table.Row
	for it.Next() {
		dest := make([]interface{}, colCount)
		ptrs := make([]interface{}, colCount)
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := it.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		cells := make([]string, colCount)
		for i, v := range dest {
			cells[i] = valueString(v)
		}
		rows = append(rows, table.Row{Cells: cells})
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return rows, nil
}

// toTableColumns converts datasource columns into table columns, sizing each to
// its title and marking it sortable.
func toTableColumns(cols []ds.Column) []table.Column {
	out := make([]table.Column, len(cols))
	for i, c := range cols {
		width := len(c.Name)
		if width < 8 {
			width = 8
		}
		if width > 32 {
			width = 32
		}
		out[i] = table.Column{Title: c.Name, Width: width, Sortable: true}
	}
	return out
}

// valueString renders a scanned cell value as a display string.
func valueString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	default:
		return fmt.Sprint(val)
	}
}

// Navigate forwards a navigation key to the table and syncs the selection.
func (m *GridModel) Navigate(key string) {
	m.table.Navigate(key)
	if c := m.table.Cursor(); c >= 0 {
		m.selection.Row = c
	}
}

// Search filters the table to rows matching query (case-insensitive). An empty
// query clears the filter.
func (m *GridModel) Search(query string) {
	m.table.Filter(query)
	if c := m.table.Cursor(); c >= 0 {
		m.selection.Row = c
	}
}

// Selection returns the active cell.
func (m *GridModel) Selection() Selection { return m.selection }

// Export writes the loaded data to path. The exporter is chosen from the path's
// extension (csv, json, xml, xlsx); unknown extensions default to CSV. It
// returns an error when nothing is loaded.
func (m *GridModel) Export(path string) error {
	if m.source == nil {
		return fmt.Errorf("export: no data loaded")
	}
	exporter := exporterFor(path)

	it, err := m.source.Query("")
	if err != nil {
		return fmt.Errorf("export: query rows: %w", err)
	}
	defer func() { _ = it.Close() }()

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("export: create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := export.ExportIterator(exporter, f, it, m.columns); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}

// exporterFor selects an Exporter based on the file extension of path,
// defaulting to CSV.
func exporterFor(path string) export.Exporter {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "json":
		return export.JSONExporter{Indent: true}
	case "xml":
		return export.XMLExporter{Indent: true}
	case "xlsx":
		return export.XLSXExporter{}
	default:
		return export.CSVExporter{}
	}
}

// Close releases the loaded data source, if any.
func (m *GridModel) Close() error {
	if m.source != nil {
		err := m.source.Close()
		m.source = nil
		return err
	}
	return nil
}
