package datasource

import (
	"errors"
	"testing"
	"time"

	"github.com/gocql/gocql"
	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// TestCassandraFactory_RegisteredAsRealConnector confirms the Cassandra backend
// is registered with the real factory (not the Unavailable stub).
func TestCassandraFactory_RegisteredAsRealConnector(t *testing.T) {
	if !Registered(BackendCassandra) {
		t.Fatal("cassandra backend not registered")
	}
}

// TestParseCassandraDSN covers host/port/keyspace/credential parsing.
func TestParseCassandraDSN(t *testing.T) {
	tests := []struct {
		name         string
		dsn          string
		wantHosts    []string
		wantPort     int
		wantKeyspace string
		wantUser     string
		wantErr      bool
	}{
		{name: "single host", dsn: "localhost", wantHosts: []string{"localhost"}, wantPort: 0},
		{name: "host and port", dsn: "localhost:9042", wantHosts: []string{"localhost"}, wantPort: 9042},
		{name: "multi host with port", dsn: "h1,h2:9042", wantHosts: []string{"h1", "h2"}, wantPort: 9042},
		{name: "keyspace", dsn: "localhost:9042/mykeyspace", wantHosts: []string{"localhost"}, wantPort: 9042, wantKeyspace: "mykeyspace"},
		{name: "credentials", dsn: "user:pass@localhost:9042/ks", wantHosts: []string{"localhost"}, wantPort: 9042, wantKeyspace: "ks", wantUser: "user"},
		{name: "empty errors", dsn: "", wantErr: true},
		{name: "bad port errors", dsn: "localhost:notaport", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster, keyspace, err := parseCassandraDSN(tc.dsn)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.dsn)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cluster.Hosts) != len(tc.wantHosts) {
				t.Fatalf("hosts = %v, want %v", cluster.Hosts, tc.wantHosts)
			}
			for i, h := range tc.wantHosts {
				if cluster.Hosts[i] != h {
					t.Errorf("host %d = %q, want %q", i, cluster.Hosts[i], h)
				}
			}
			if tc.wantPort != 0 && cluster.Port != tc.wantPort {
				t.Errorf("port = %d, want %d", cluster.Port, tc.wantPort)
			}
			if keyspace != tc.wantKeyspace {
				t.Errorf("keyspace = %q, want %q", keyspace, tc.wantKeyspace)
			}
			if tc.wantUser != "" {
				auth, ok := cluster.Authenticator.(gocql.PasswordAuthenticator)
				if !ok {
					t.Fatalf("authenticator type = %T, want PasswordAuthenticator", cluster.Authenticator)
				}
				if auth.Username != tc.wantUser {
					t.Errorf("user = %q, want %q", auth.Username, tc.wantUser)
				}
			}
		})
	}
}

// TestParseCassandraDSN_AppliesConnectTimeout confirms the bounded timeout is
// applied so connect attempts fail fast.
func TestParseCassandraDSN_AppliesConnectTimeout(t *testing.T) {
	cluster, _, err := parseCassandraDSN("localhost:9042")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.ConnectTimeout != cassandraConnectTimeout {
		t.Errorf("connect timeout = %v, want %v", cluster.ConnectTimeout, cassandraConnectTimeout)
	}
}

// TestResolveTable covers keyspace.table resolution against the session
// keyspace fallback.
func TestResolveTable(t *testing.T) {
	c := &cassandraSource{keyspace: "sessionks"}
	if ks, tbl := c.resolveTable("users"); ks != "sessionks" || tbl != "users" {
		t.Errorf("resolveTable(users) = %q.%q, want sessionks.users", ks, tbl)
	}
	if ks, tbl := c.resolveTable("otherks.events"); ks != "otherks" || tbl != "events" {
		t.Errorf("resolveTable(otherks.events) = %q.%q, want otherks.events", ks, tbl)
	}
}

// TestConnectCassandra_UnreachableReturnsConnectionError verifies connecting to
// an unreachable cluster returns a real connection error, NOT
// ErrDriverUnavailable.
func TestConnectCassandra_UnreachableReturnsConnectionError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent connect test in -short mode")
	}
	old := cassandraConnectTimeout
	cassandraConnectTimeout = 500 * time.Millisecond
	defer func() { cassandraConnectTimeout = old }()

	_, err := Connect(BackendCassandra, "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected a connection error, got nil")
	}
	if errors.Is(err, ErrDriverUnavailable) {
		t.Fatalf("got ErrDriverUnavailable, want a real connection error: %v", err)
	}
}

// TestGocqlToCell verifies gocql-value to scalar-cell mapping.
func TestGocqlToCell(t *testing.T) {
	uuid, _ := gocql.ParseUUID("11111111-2222-3333-4444-555555555555")
	when := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)

	tests := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"nil", nil, nil},
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"int64", int64(99), int64(99)},
		{"float64", 1.5, 1.5},
		{"bool", true, true},
		{"bytes to string", []byte("blob"), "blob"},
		{"time rfc3339", when, when.Format(time.RFC3339Nano)},
		{"uuid string", uuid, uuid.String()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := gocqlToCell(tc.in); got != tc.want {
				t.Errorf("gocqlToCell(%v) = %v (%T), want %v (%T)", tc.in, got, got, tc.want, tc.want)
			}
		})
	}
}

// TestCassandraIterator_StreamsRows exercises the iterator against a
// pre-populated buffer (no live server), verifying Scan and the cursor
// contract.
func TestCassandraIterator_StreamsRows(t *testing.T) {
	it := &cassandraIterator{
		columns: []string{"id", "name"},
		rows: [][]interface{}{
			{int64(1), "Ada"},
			{int64(2), "Linus"},
		},
		pos: -1,
	}
	defer func() { _ = it.Close() }()

	var names []string
	for it.Next() {
		var id, name interface{}
		if err := it.Scan(&id, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name.(string))
	}
	if err := it.Err(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(names) != 2 || names[0] != "Ada" || names[1] != "Linus" {
		t.Fatalf("names = %v, want [Ada Linus]", names)
	}
}

// TestCassandraIterator_ColumnCountMismatch verifies Scan rejects a wrong
// destination count.
func TestCassandraIterator_ColumnCountMismatch(t *testing.T) {
	it := &cassandraIterator{
		columns: []string{"a", "b"},
		rows:    [][]interface{}{{1, 2}},
		pos:     -1,
	}
	defer func() { _ = it.Close() }()
	if !it.Next() {
		t.Fatal("expected a row")
	}
	var one interface{}
	if err := it.Scan(&one); !errors.Is(err, ds.ErrColumnCount) {
		t.Fatalf("error = %v, want ErrColumnCount", err)
	}
}

// TestColumnsFromCassandraTable verifies metadata mapping orders primary keys
// first and marks them primary.
func TestColumnsFromCassandraTable(t *testing.T) {
	tbl := &gocql.TableMetadata{
		PartitionKey:      []*gocql.ColumnMetadata{{Name: "id", Type: gocql.NewNativeType(0, gocql.TypeInt, "")}},
		ClusteringColumns: []*gocql.ColumnMetadata{{Name: "created", Type: gocql.NewNativeType(0, gocql.TypeTimestamp, "")}},
		Columns: map[string]*gocql.ColumnMetadata{
			"id":      {Name: "id", Type: gocql.NewNativeType(0, gocql.TypeInt, "")},
			"created": {Name: "created", Type: gocql.NewNativeType(0, gocql.TypeTimestamp, "")},
			"name":    {Name: "name", Type: gocql.NewNativeType(0, gocql.TypeText, "")},
			"email":   {Name: "email", Type: gocql.NewNativeType(0, gocql.TypeText, "")},
		},
	}
	cols := columnsFromCassandraTable(tbl)
	if len(cols) != 4 {
		t.Fatalf("column count = %d, want 4", len(cols))
	}
	// Primary keys first: id (partition), created (clustering).
	if cols[0].Name != "id" || !cols[0].IsPrimary {
		t.Errorf("col0 = %+v, want primary id", cols[0])
	}
	if cols[1].Name != "created" || !cols[1].IsPrimary {
		t.Errorf("col1 = %+v, want primary created", cols[1])
	}
	// Remaining columns sorted by name: email, name.
	if cols[2].Name != "email" || cols[3].Name != "name" {
		t.Errorf("non-key order = %q,%q, want email,name", cols[2].Name, cols[3].Name)
	}
}
