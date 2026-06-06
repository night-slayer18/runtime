// Package datasource provides a unified abstraction over data sources for
// Runtime applications. A DataSource exposes its schema, supports read queries
// that stream rows back through a RowIterator, and supports write/DDL
// statements that report their effect through a Result.
//
// The package also ships an in-memory DataSource implementation so component
// packages (table, export, ...) and applications have a concrete, dependency
// free consumer to test and prototype against.
package datasource

import (
	"errors"
	"fmt"
	"sync"
)

// Column describes a single column in a DataSource's schema.
type Column struct {
	Name      string      // column name
	Type      string      // declared type, e.g. "int", "text"
	Nullable  bool        // whether the column accepts NULL values
	Default   interface{} // default value, or nil when none
	IsPrimary bool        // whether the column is part of the primary key
}

// Result reports the outcome of an Execute call (an INSERT/UPDATE/DELETE or
// other non-row-returning statement).
type Result struct {
	RowsAffected int64 // number of rows changed by the statement
	LastInsertID int64 // identifier generated for the last inserted row, if any
}

// RowIterator streams the rows produced by a Query. Iteration follows the
// standard Go cursor contract: call Next to advance, Scan to read the current
// row, and Err after iteration completes to surface any error that stopped it.
//
//	it, err := ds.Query("...")
//	if err != nil { ... }
//	defer it.Close()
//	for it.Next() {
//	    if err := it.Scan(&a, &b); err != nil { ... }
//	}
//	if err := it.Err(); err != nil { ... }
type RowIterator interface {
	// Next advances to the next row, returning false when no rows remain or
	// when an error has occurred. After Next returns false, callers should
	// inspect Err.
	Next() bool
	// Scan copies the columns of the current row into the values pointed at by
	// dest. The number of destinations must match the number of columns.
	Scan(dest ...interface{}) error
	// Err returns the first non-nil error encountered during iteration, if any.
	Err() error
	// Close releases resources held by the iterator. It is safe to call Close
	// multiple times.
	Close() error
}

// DataSource is the unified interface implemented by every Runtime data source.
type DataSource interface {
	// Schema returns the columns describing the rows this source produces.
	Schema() ([]Column, error)
	// Query runs a read query and returns an iterator over the resulting rows.
	Query(query string) (RowIterator, error)
	// Execute runs a statement that does not return rows and reports its effect.
	Execute(query string) (Result, error)
	// Close releases resources held by the data source. It is safe to call
	// Close multiple times.
	Close() error
}

// Common errors returned by DataSource implementations.
var (
	// ErrClosed is returned when an operation is attempted on a closed source.
	ErrClosed = errors.New("datasource: closed")
	// ErrColumnCount is returned by Scan when the number of destinations does
	// not match the number of columns in a row.
	ErrColumnCount = errors.New("datasource: destination count does not match column count")
)

// QueryFunc lets the in-memory source answer arbitrary query strings. It
// receives the raw query and the source's current rows, and returns the rows
// that should be streamed back. When nil, the in-memory source returns all of
// its rows for any query.
type QueryFunc func(query string, rows [][]interface{}) ([][]interface{}, error)

// ExecuteFunc lets the in-memory source react to a statement and mutate its
// rows. It receives the raw statement and a pointer to the source's rows so it
// can append, update, or delete entries, and returns the Result to report. When
// nil, Execute is a no-op that returns a zero Result.
type ExecuteFunc func(query string, rows *[][]interface{}) (Result, error)

// MemorySource is an in-memory DataSource implementation. It holds a fixed
// schema and a slice of rows, and is safe for concurrent use.
type MemorySource struct {
	mu      sync.Mutex
	columns []Column
	rows    [][]interface{}
	onQuery QueryFunc
	onExec  ExecuteFunc
	closed  bool
}

// NewMemorySource creates an in-memory DataSource with the given schema and
// rows. The columns and rows are copied so later mutations to the supplied
// slices do not affect the source.
func NewMemorySource(columns []Column, rows [][]interface{}) *MemorySource {
	return &MemorySource{
		columns: cloneColumns(columns),
		rows:    cloneRows(rows),
	}
}

// WithQueryFunc sets a custom query handler and returns the source for
// chaining. Passing nil restores the default behaviour (return all rows).
func (m *MemorySource) WithQueryFunc(fn QueryFunc) *MemorySource {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onQuery = fn
	return m
}

// WithExecuteFunc sets a custom execute handler and returns the source for
// chaining. Passing nil restores the default behaviour (no-op, zero Result).
func (m *MemorySource) WithExecuteFunc(fn ExecuteFunc) *MemorySource {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExec = fn
	return m
}

// Schema returns a copy of the source's columns.
func (m *MemorySource) Schema() ([]Column, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	return cloneColumns(m.columns), nil
}

// Query returns an iterator over the rows selected for the given query. With no
// custom QueryFunc, every row is returned.
func (m *MemorySource) Query(query string) (RowIterator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}

	var selected [][]interface{}
	if m.onQuery != nil {
		out, err := m.onQuery(query, cloneRows(m.rows))
		if err != nil {
			return nil, fmt.Errorf("datasource: query: %w", err)
		}
		selected = cloneRows(out)
	} else {
		selected = cloneRows(m.rows)
	}

	return &memoryIterator{
		columnCount: len(m.columns),
		rows:        selected,
		pos:         -1,
	}, nil
}

// Execute runs a statement against the source. With no custom ExecuteFunc it is
// a no-op returning a zero Result.
func (m *MemorySource) Execute(query string) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Result{}, ErrClosed
	}
	if m.onExec == nil {
		return Result{}, nil
	}
	res, err := m.onExec(query, &m.rows)
	if err != nil {
		return Result{}, fmt.Errorf("datasource: execute: %w", err)
	}
	return res, nil
}

// Close marks the source closed. Subsequent operations return ErrClosed. Close
// is idempotent.
func (m *MemorySource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// memoryIterator iterates over a snapshot of rows produced by a MemorySource
// query. It is not safe for concurrent use by multiple goroutines.
type memoryIterator struct {
	columnCount int
	rows        [][]interface{}
	pos         int
	err         error
	closed      bool
}

// Next advances the cursor to the next row.
func (it *memoryIterator) Next() bool {
	if it.closed || it.err != nil {
		return false
	}
	if it.pos+1 >= len(it.rows) {
		it.pos = len(it.rows)
		return false
	}
	it.pos++
	return true
}

// Scan copies the current row's columns into dest.
func (it *memoryIterator) Scan(dest ...interface{}) error {
	if it.err != nil {
		return it.err
	}
	if it.closed {
		return ErrClosed
	}
	if it.pos < 0 || it.pos >= len(it.rows) {
		return errors.New("datasource: Scan called without a current row")
	}
	row := it.rows[it.pos]
	if len(dest) != len(row) {
		it.err = fmt.Errorf("%w: have %d, want %d", ErrColumnCount, len(dest), len(row))
		return it.err
	}
	for i, d := range dest {
		if err := assign(d, row[i]); err != nil {
			it.err = fmt.Errorf("datasource: scan column %d: %w", i, err)
			return it.err
		}
	}
	return nil
}

// Err returns the first error that stopped iteration, if any.
func (it *memoryIterator) Err() error {
	return it.err
}

// Close releases the iterator. It is idempotent.
func (it *memoryIterator) Close() error {
	it.closed = true
	return nil
}

// assign copies src into the value pointed at by dest. dest must be a non-nil
// pointer. When dest is *interface{} the source value is stored as-is;
// otherwise the concrete types must be assignable.
func assign(dest interface{}, src interface{}) error {
	switch d := dest.(type) {
	case nil:
		return errors.New("destination is nil")
	case *interface{}:
		*d = src
		return nil
	case *string:
		v, ok := src.(string)
		if !ok {
			return typeMismatch(src, "string")
		}
		*d = v
		return nil
	case *int:
		v, ok := src.(int)
		if !ok {
			return typeMismatch(src, "int")
		}
		*d = v
		return nil
	case *int64:
		v, ok := src.(int64)
		if !ok {
			return typeMismatch(src, "int64")
		}
		*d = v
		return nil
	case *float64:
		v, ok := src.(float64)
		if !ok {
			return typeMismatch(src, "float64")
		}
		*d = v
		return nil
	case *bool:
		v, ok := src.(bool)
		if !ok {
			return typeMismatch(src, "bool")
		}
		*d = v
		return nil
	default:
		return fmt.Errorf("unsupported destination type %T", dest)
	}
}

func typeMismatch(src interface{}, want string) error {
	return fmt.Errorf("cannot assign %T to *%s", src, want)
}

func cloneColumns(in []Column) []Column {
	if in == nil {
		return nil
	}
	out := make([]Column, len(in))
	copy(out, in)
	return out
}

func cloneRows(in [][]interface{}) [][]interface{} {
	if in == nil {
		return nil
	}
	out := make([][]interface{}, len(in))
	for i, row := range in {
		r := make([]interface{}, len(row))
		copy(r, row)
		out[i] = r
	}
	return out
}
