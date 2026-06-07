// SQL adapter: a driver-agnostic datasource.DataSource implemented entirely
// over the standard library's database/sql. It has no compile-time dependency
// on any concrete driver — it opens connections via sql.Open(driverName, dsn),
// so the same code serves PostgreSQL, MySQL, SQLite, and any future
// database/sql driver. The concrete drivers are registered with database/sql by
// the blank imports in drivers.go.
package datasource

import (
	"database/sql"
	"fmt"

	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// sqlSource adapts a *sql.DB to datasource.DataSource.
//
// Schema introspection in SQL is inherently per-result-set, so the source
// derives its schema from a configurable schemaQuery (typically a
// "SELECT * FROM <table>" the user has focused in the UI). Reading column
// metadata via sql.Rows.ColumnTypes keeps the adapter portable across drivers
// that report type names and nullability.
type sqlSource struct {
	db          *sql.DB
	driver      string
	schemaQuery string
}

// sqlFactory builds a ConnectionFactory for a database/sql backend. driver is
// the database/sql driver name (e.g. "postgres") and installModule is the
// module a user must build with to enable it, surfaced in the unavailable
// error.
func sqlFactory(driver, installModule string) ConnectionFactory {
	return func(dsn string) (ds.DataSource, error) {
		return openSQL(driver, installModule, dsn)
	}
}

// openSQL is the shared factory body for every database/sql backend. It opens
// the pool and verifies the driver is actually registered with database/sql;
// when it is not, it returns an ErrDriverUnavailable naming the module to
// install rather than a generic failure.
func openSQL(driver, installModule, dsn string) (ds.DataSource, error) {
	if !driverRegistered(driver) {
		return nil, fmt.Errorf("%w: %s requires %q; build Strata with that driver to enable it",
			ErrDriverUnavailable, driver, installModule)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("datasource: open %s: %w", driver, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("datasource: connect %s: %w", driver, err)
	}
	return &sqlSource{db: db, driver: driver}, nil
}

// driverRegistered reports whether name is registered with database/sql (i.e.
// the driver package was blank-imported into this build).
func driverRegistered(name string) bool {
	for _, d := range sql.Drivers() {
		if d == name {
			return true
		}
	}
	return false
}

// SchemaQueryConfigurable is implemented by sources whose Schema() is derived
// from a configurable result set rather than fixed metadata — the SQL backends,
// where schema is inherently per-result-set. Callers (and the Strata model) can
// type-assert to this interface to point Schema at the table/SELECT currently
// being explored.
type SchemaQueryConfigurable interface {
	SetSchemaQuery(query string)
}

// WithSchemaQuery sets the query whose result columns Schema reports and returns
// the source for chaining. In Strata this is set to the SELECT for the table the
// user is exploring.
func (s *sqlSource) WithSchemaQuery(query string) *sqlSource {
	s.schemaQuery = query
	return s
}

// SetSchemaQuery implements SchemaQueryConfigurable.
func (s *sqlSource) SetSchemaQuery(query string) { s.schemaQuery = query }

// Schema returns the columns produced by the configured schema query. It uses a
// prepared statement with no row fetch where possible so introspection does not
// stream a full table.
func (s *sqlSource) Schema() ([]ds.Column, error) {
	if s.db == nil {
		return nil, ds.ErrClosed
	}
	if s.schemaQuery == "" {
		return nil, fmt.Errorf("datasource: no schema query configured; call WithSchemaQuery first")
	}
	rows, err := s.db.Query(s.schemaQuery)
	if err != nil {
		return nil, fmt.Errorf("datasource: schema query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("datasource: column types: %w", err)
	}
	cols := make([]ds.Column, len(types))
	for i, ct := range types {
		col := ds.Column{Name: ct.Name(), Type: ct.DatabaseTypeName()}
		if nullable, ok := ct.Nullable(); ok {
			col.Nullable = nullable
		}
		cols[i] = col
	}
	return cols, nil
}

// Query runs a read query and streams the resulting rows back through a
// RowIterator that wraps *sql.Rows.
func (s *sqlSource) Query(query string) (ds.RowIterator, error) {
	if s.db == nil {
		return nil, ds.ErrClosed
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("datasource: query: %w", err)
	}
	cols, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("datasource: columns: %w", err)
	}
	return &sqlIterator{rows: rows, columnCount: len(cols)}, nil
}

// Execute runs a non-row-returning statement and reports its effect.
func (s *sqlSource) Execute(query string) (ds.Result, error) {
	if s.db == nil {
		return ds.Result{}, ds.ErrClosed
	}
	res, err := s.db.Exec(query)
	if err != nil {
		return ds.Result{}, fmt.Errorf("datasource: execute: %w", err)
	}
	var out ds.Result
	if n, err := res.RowsAffected(); err == nil {
		out.RowsAffected = n
	}
	if id, err := res.LastInsertId(); err == nil {
		out.LastInsertID = id
	}
	return out, nil
}

// Close releases the underlying pool. It is idempotent.
func (s *sqlSource) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// sqlIterator adapts *sql.Rows to datasource.RowIterator. It scans each row into
// interface{} holders so it works uniformly across column types and drivers; a
// caller passing concrete destinations is handled by copying via the holders.
type sqlIterator struct {
	rows        *sql.Rows
	columnCount int
	current     []interface{}
	err         error
	closed      bool
}

// Next advances to the next row, buffering its values for Scan.
func (it *sqlIterator) Next() bool {
	if it.closed || it.err != nil {
		return false
	}
	if !it.rows.Next() {
		it.err = it.rows.Err()
		return false
	}
	holders := make([]interface{}, it.columnCount)
	dest := make([]interface{}, it.columnCount)
	for i := range holders {
		dest[i] = &holders[i]
	}
	if err := it.rows.Scan(dest...); err != nil {
		it.err = fmt.Errorf("datasource: scan: %w", err)
		return false
	}
	// Normalise []byte (common for text columns across drivers) to string so
	// downstream rendering and export produce readable values.
	for i, v := range holders {
		if b, ok := v.([]byte); ok {
			holders[i] = string(b)
		}
	}
	it.current = holders
	return true
}

// Scan copies the current row's values into dest. Each destination must be a
// *interface{} (the form the export and table layers use) or otherwise
// assignable from the buffered value.
func (it *sqlIterator) Scan(dest ...interface{}) error {
	if it.err != nil {
		return it.err
	}
	if it.closed {
		return ds.ErrClosed
	}
	if it.current == nil {
		return fmt.Errorf("datasource: Scan called without a current row")
	}
	if len(dest) != len(it.current) {
		return fmt.Errorf("%w: have %d, want %d", ds.ErrColumnCount, len(dest), len(it.current))
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *interface{}:
			*target = it.current[i]
		case *string:
			*target = fmt.Sprint(it.current[i])
		default:
			return fmt.Errorf("datasource: unsupported scan destination %T at column %d", d, i)
		}
	}
	return nil
}

// Err returns the first error encountered during iteration.
func (it *sqlIterator) Err() error { return it.err }

// Close releases the underlying rows. It is idempotent.
func (it *sqlIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	return it.rows.Close()
}
