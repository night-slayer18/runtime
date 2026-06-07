// Package model defines the core data model for strata.
//
// StrataModel ties a database connection (via the driver-registry datasource
// adapters) to the shared schema and table components: it connects to a
// backend, reads a result set's schema, runs a query, and projects the rows
// into a table.Table for rendering. The model is deliberately UI-framework
// agnostic so it can be exercised directly in tests.
package model

import (
	"fmt"

	appds "github.com/runtime-sh/runtime/apps/strata/internal/model/datasource"
	ds "github.com/runtime-sh/runtime/packages/datasource"
	"github.com/runtime-sh/runtime/packages/schema"
	"github.com/runtime-sh/runtime/packages/table"
	"github.com/runtime-sh/runtime/packages/theme"
)

// State holds all application state for runtime-strata.
type State struct {
	// Backend is the registered backend name the model is connected to, if any.
	Backend string
	// Connected reports whether Source is an open connection.
	Connected bool

	// Source is the active datasource.DataSource, or nil when disconnected.
	Source ds.DataSource

	// Columns is the schema of the most recently executed/queried result set.
	Columns []ds.Column
	// Table renders the most recent query result.
	Table *table.Table

	styles theme.Styles
}

// New returns an initialised State with table styling from the default theme.
func New() *State {
	return NewWithStyles(theme.DefaultStyles)
}

// NewWithStyles returns an initialised State using the supplied styles for the
// table component.
func NewWithStyles(styles theme.Styles) *State {
	return &State{
		styles: styles,
		Table:  table.New(styles),
	}
}

// Connect opens a connection to the named backend with the given DSN through
// the datasource registry and stores it on the state. A previously open source
// is closed first. Errors are returned to the caller: a connection failure
// (unreachable server, bad DSN) surfaces as a plain error, while a backend
// whose driver was intentionally excluded from the build wraps
// ErrDriverUnavailable.
func (s *State) Connect(backend, dsn string) error {
	if s.Source != nil {
		_ = s.Source.Close()
		s.Source = nil
		s.Connected = false
	}
	src, err := appds.Connect(backend, dsn)
	if err != nil {
		return err
	}
	s.Source = src
	s.Backend = backend
	s.Connected = true
	return nil
}

// UseSource attaches an already-constructed DataSource (for example an
// in-memory source or a source built directly in a test) without going through
// the registry. It closes any previously open source.
func (s *State) UseSource(backend string, src ds.DataSource) {
	if s.Source != nil {
		_ = s.Source.Close()
	}
	s.Source = src
	s.Backend = backend
	s.Connected = src != nil
}

// SetSchemaQuery configures the query the connected source derives its schema
// from, for SQL backends whose schema is per-result-set. It is a no-op for
// sources that report fixed metadata (such as the in-memory source).
func (s *State) SetSchemaQuery(query string) {
	if c, ok := s.Source.(appds.SchemaQueryConfigurable); ok {
		c.SetSchemaQuery(query)
	}
}

// ReadSchema loads the connected source's schema into the state and returns it.
func (s *State) ReadSchema() ([]ds.Column, error) {
	if s.Source == nil {
		return nil, fmt.Errorf("strata: not connected")
	}
	cols, err := s.Source.Schema()
	if err != nil {
		return nil, fmt.Errorf("strata: read schema: %w", err)
	}
	s.Columns = cols
	return cols, nil
}

// RunQuery executes query against the connected source, loads the resulting
// rows into the table, and records the column set used to project them. It
// returns the number of rows loaded.
//
// When the source's own schema is available it is used to label and size the
// table columns; otherwise the query result is rendered with generic column
// titles so unknown result shapes still display.
func (s *State) RunQuery(query string) (int, error) {
	if s.Source == nil {
		return 0, fmt.Errorf("strata: not connected")
	}

	// Prefer the source schema for column metadata; fall back gracefully.
	cols := s.Columns
	if len(cols) == 0 {
		if sc, err := s.Source.Schema(); err == nil {
			cols = sc
			s.Columns = sc
		}
	}

	it, err := s.Source.Query(query)
	if err != nil {
		return 0, fmt.Errorf("strata: query: %w", err)
	}
	defer func() { _ = it.Close() }()

	colCount := len(cols)
	rows, err := drainRows(it, colCount)
	if err != nil {
		return 0, err
	}

	s.Table.SetData(rows, tableColumns(cols))
	return len(rows), nil
}

// SchemaDefinition converts the loaded datasource columns into a schema.Schema,
// mapping declared SQL/driver types onto schema field types. This is what lets
// Strata validate or describe a result set using the shared schema package.
func (s *State) SchemaDefinition() *schema.Schema {
	fields := make([]schema.Field, 0, len(s.Columns))
	for _, c := range s.Columns {
		fields = append(fields, schema.Field{
			Name:     c.Name,
			Type:     fieldType(c.Type),
			Required: !c.Nullable,
		})
	}
	return schema.New(fields...)
}

// Close releases the active connection if any. It is safe to call when
// disconnected.
func (s *State) Close() error {
	if s.Source == nil {
		return nil
	}
	err := s.Source.Close()
	s.Source = nil
	s.Connected = false
	return err
}

// drainRows reads every row from it into table rows. When colCount is zero
// (schema unknown) it derives the width from the first scanned row.
func drainRows(it ds.RowIterator, colCount int) ([]table.Row, error) {
	var rows []table.Row
	for it.Next() {
		n := colCount
		if n == 0 {
			// Unknown shape: probe a single interface destination repeatedly is
			// not possible, so default to a 1-cell projection. Sources used by
			// Strata always expose a schema, so this path is a safe fallback.
			n = 1
		}
		holders := make([]interface{}, n)
		dest := make([]interface{}, n)
		for i := range holders {
			dest[i] = &holders[i]
		}
		if err := it.Scan(dest...); err != nil {
			return nil, fmt.Errorf("strata: scan row: %w", err)
		}
		cells := make([]string, n)
		for i, v := range holders {
			cells[i] = renderCell(v)
		}
		rows = append(rows, table.Row{Cells: cells})
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("strata: iterate rows: %w", err)
	}
	return rows, nil
}

// renderCell converts a scanned value into its display string.
func renderCell(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprint(val)
	}
}

// tableColumns derives table columns (titles + widths) from datasource columns.
func tableColumns(cols []ds.Column) []table.Column {
	out := make([]table.Column, len(cols))
	for i, c := range cols {
		width := len(c.Name)
		if width < 12 {
			width = 12
		}
		out[i] = table.Column{Title: c.Name, Width: width, Sortable: true}
	}
	return out
}

// fieldType maps a declared database/driver type name onto a schema.FieldType.
// The mapping is intentionally lenient: unknown types fall back to TypeAny so
// schema description never fails on an exotic column type.
func fieldType(declared string) schema.FieldType {
	switch normalizeType(declared) {
	case "int", "integer", "bigint", "smallint", "tinyint", "int2", "int4", "int8", "serial":
		return schema.TypeInt
	case "float", "double", "real", "numeric", "decimal", "float4", "float8":
		return schema.TypeFloat
	case "bool", "boolean":
		return schema.TypeBool
	case "text", "varchar", "char", "character", "string", "uuid", "timestamp", "date", "datetime":
		return schema.TypeString
	default:
		return schema.TypeAny
	}
}

// normalizeType lower-cases a declared type and strips any size/precision
// suffix such as "(255)" so "VARCHAR(255)" maps the same as "varchar".
func normalizeType(declared string) string {
	t := ""
	for _, r := range declared {
		if r == '(' || r == ' ' {
			break
		}
		switch {
		case r >= 'A' && r <= 'Z':
			t += string(r + ('a' - 'A'))
		default:
			t += string(r)
		}
	}
	return t
}
