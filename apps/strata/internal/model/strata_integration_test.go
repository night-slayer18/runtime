package model_test

import (
	"errors"
	"testing"

	appds "github.com/runtime-sh/runtime/apps/strata/internal/model"
	regds "github.com/runtime-sh/runtime/apps/strata/internal/model/datasource"
	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// TestStrata_SQLiteIntegration exercises the full Strata path against an
// in-process SQLite database: connect through the driver registry, read the
// schema of a table, run a query through the datasource adapter, and render the
// results into the shared table component.
//
// The modernc.org/sqlite driver is pure-Go and wired into the build, so this
// runs in-process with no external server. SQLite is opened with an in-memory
// DSN. (If a future build dropped the SQLite driver, regds.Connect would return
// ErrDriverUnavailable; the MemorySource test below covers the same
// schema-read + query + table-render path regardless.)
func TestStrata_SQLiteIntegration(t *testing.T) {
	if !regds.Registered(regds.BackendSQLite) {
		t.Skip("sqlite backend not registered")
	}

	st := appds.New()
	// Shared in-memory database for the lifetime of the connection.
	if err := st.Connect(regds.BackendSQLite, "file:strata_test?mode=memory&cache=shared"); err != nil {
		if errors.Is(err, regds.ErrDriverUnavailable) {
			t.Skipf("sqlite driver unavailable: %v", err)
		}
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Create a table and seed rows.
	if _, err := st.Source.Execute(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, active INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := st.Source.Execute(`INSERT INTO users (id, name, active) VALUES (1, 'Ada', 1), (2, 'Linus', 0), (3, 'Grace', 1)`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Point schema introspection at the users table, then read schema.
	st.SetSchemaQuery(`SELECT id, name, active FROM users`)
	cols, err := st.ReadSchema()
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("schema column count = %d, want 3", len(cols))
	}
	wantNames := []string{"id", "name", "active"}
	for i, c := range cols {
		if c.Name != wantNames[i] {
			t.Errorf("column %d name = %q, want %q", i, c.Name, wantNames[i])
		}
	}

	// Run a query through the adapter and render into the table.
	n, err := st.RunQuery(`SELECT id, name, active FROM users ORDER BY id`)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}
	if n != 3 {
		t.Fatalf("rows loaded = %d, want 3", n)
	}
	if got := st.Table.RowCount(); got != 3 {
		t.Fatalf("table row count = %d, want 3", got)
	}

	// Verify the first rendered row carries the expected values.
	row, ok := st.Table.SelectedRow()
	if !ok {
		t.Fatal("expected a selected row")
	}
	if len(row.Cells) != 3 || row.Cells[0] != "1" || row.Cells[1] != "Ada" {
		t.Fatalf("first row = %v, want [1 Ada 1]", row.Cells)
	}

	// The rendered view must include data and not be empty.
	if view := st.Table.View(); view == "" {
		t.Fatal("table view is empty")
	}
}

// TestStrata_MemorySourceIntegration exercises the identical schema-read +
// query + table-render path against the in-memory DataSource. It does not
// depend on any driver, so it always runs and documents the fallback path
// described for offline builds.
func TestStrata_MemorySourceIntegration(t *testing.T) {
	cols := []ds.Column{
		{Name: "id", Type: "int", IsPrimary: true},
		{Name: "name", Type: "text"},
	}
	rows := [][]interface{}{
		{1, "Ada"},
		{2, "Linus"},
	}
	src := ds.NewMemorySource(cols, rows)

	st := appds.New()
	st.UseSource("memory", src)

	got, err := st.ReadSchema()
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("schema column count = %d, want 2", len(got))
	}

	n, err := st.RunQuery("SELECT * FROM t")
	if err != nil {
		t.Fatalf("run query: %v", err)
	}
	if n != 2 {
		t.Fatalf("rows loaded = %d, want 2", n)
	}
	if st.Table.RowCount() != 2 {
		t.Fatalf("table row count = %d, want 2", st.Table.RowCount())
	}
}

// TestStrata_NativeBackendsConnectError confirms the native (non-database/sql)
// backends MongoDB and Cassandra are registered as real, driver-backed
// connectors: connecting to an unreachable server returns a genuine connection
// error, NOT ErrDriverUnavailable (which is reserved for "driver not compiled
// in"). The connection attempts use unroutable/closed addresses with short
// driver timeouts so the test fails fast without a live server.
func TestStrata_NativeBackendsConnectError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent connect test in -short mode")
	}
	cases := []struct {
		backend string
		dsn     string
	}{
		{regds.BackendMongoDB, "mongodb://127.0.0.1:1/db?serverSelectionTimeoutMS=500&connectTimeoutMS=500"},
		{regds.BackendCassandra, "127.0.0.1:1"},
	}
	for _, tc := range cases {
		if !regds.Registered(tc.backend) {
			t.Errorf("backend %q should be registered as a real connector", tc.backend)
			continue
		}
		_, err := regds.Connect(tc.backend, tc.dsn)
		if err == nil {
			t.Errorf("backend %q: expected a connection error, got nil", tc.backend)
			continue
		}
		if errors.Is(err, regds.ErrDriverUnavailable) {
			t.Errorf("backend %q: got ErrDriverUnavailable, want a real connection error: %v", tc.backend, err)
		}
	}
}
