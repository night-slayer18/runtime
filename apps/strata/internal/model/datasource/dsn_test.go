package datasource

import "testing"

func TestParseConnectionString(t *testing.T) {
	tests := []struct {
		name        string
		conn        string
		wantBackend string
		wantDSN     string
		wantErr     bool
	}{
		{
			name:        "postgres url passes through whole",
			conn:        "postgres://user:pass@localhost:5432/app",
			wantBackend: BackendPostgres,
			wantDSN:     "postgres://user:pass@localhost:5432/app",
		},
		{
			name:        "postgresql alias maps to postgres backend",
			conn:        "postgresql://localhost/app",
			wantBackend: BackendPostgres,
			wantDSN:     "postgresql://localhost/app",
		},
		{
			name:        "mysql strips scheme and authority separator",
			conn:        "mysql://user:pass@tcp(127.0.0.1:3306)/app",
			wantBackend: BackendMySQL,
			wantDSN:     "user:pass@tcp(127.0.0.1:3306)/app",
		},
		{
			name:        "sqlite file uri keeps file: prefix",
			conn:        "sqlite:file:data.db",
			wantBackend: BackendSQLite,
			wantDSN:     "file:data.db",
		},
		{
			name:        "sqlite memory dsn",
			conn:        "sqlite::memory:",
			wantBackend: BackendSQLite,
			wantDSN:     ":memory:",
		},
		{
			name:        "sqlite3 alias",
			conn:        "sqlite3:file:x.db",
			wantBackend: BackendSQLite,
			wantDSN:     "file:x.db",
		},
		{
			name:        "mongodb keeps full uri with scheme",
			conn:        "mongodb://localhost:27017/db",
			wantBackend: BackendMongoDB,
			wantDSN:     "mongodb://localhost:27017/db",
		},
		{
			name:        "cassandra recognised scheme",
			conn:        "cassandra://localhost:9042",
			wantBackend: BackendCassandra,
			wantDSN:     "localhost:9042",
		},
		{name: "empty string errors", conn: "", wantErr: true},
		{name: "missing scheme errors", conn: "justapath", wantErr: true},
		{name: "leading colon errors", conn: ":nope", wantErr: true},
		{name: "unknown scheme errors", conn: "oracle://host/db", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend, dsn, err := ParseConnectionString(tc.conn)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got backend=%q dsn=%q", tc.conn, backend, dsn)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if backend != tc.wantBackend {
				t.Errorf("backend = %q, want %q", backend, tc.wantBackend)
			}
			if dsn != tc.wantDSN {
				t.Errorf("dsn = %q, want %q", dsn, tc.wantDSN)
			}
		})
	}
}

// TestParseConnectionString_ResolvesToRegisteredBackend confirms every scheme
// maps to a backend that is actually registered, so a parsed connection string
// can always be handed to Connect.
func TestParseConnectionString_ResolvesToRegisteredBackend(t *testing.T) {
	for _, conn := range []string{
		"postgres://h/d", "mysql://h/d", "sqlite::memory:",
		"mongodb://h", "cassandra://h",
	} {
		backend, _, err := ParseConnectionString(conn)
		if err != nil {
			t.Fatalf("parse %q: %v", conn, err)
		}
		if !Registered(backend) {
			t.Errorf("backend %q (from %q) is not registered", backend, conn)
		}
	}
}
