// Connection-string parsing: maps a single CLI argument onto a registered
// backend name and the DSN its driver expects.
//
// Strata is launched with one connection string, e.g.
//
//	runtime-strata postgres://user:pass@localhost:5432/app
//	runtime-strata mysql://user:pass@tcp(127.0.0.1:3306)/app
//	runtime-strata sqlite:file:data.db
//	runtime-strata sqlite::memory:
//
// The scheme selects the backend; the remainder is handed to the backend's
// driver. Keeping this mapping in the datasource package (rather than main)
// means the CLI and any future callers share one definition of how a
// connection string is interpreted.
package datasource

import (
	"fmt"
	"strings"
)

// schemeBackends maps a connection-string scheme to a registered backend name.
// Multiple schemes can alias the same backend (postgres/postgresql).
var schemeBackends = map[string]string{
	"postgres":   BackendPostgres,
	"postgresql": BackendPostgres,
	"mysql":      BackendMySQL,
	"sqlite":     BackendSQLite,
	"sqlite3":    BackendSQLite,
	"mongodb":    BackendMongoDB,
	"cassandra":  BackendCassandra,
}

// ParseConnectionString splits a connection string into a backend name and the
// DSN its driver expects. The scheme (text before the first ":") selects the
// backend; the remainder is the driver DSN.
//
// Backend-specific handling of the remainder:
//
//   - postgres: the full URL (including the "postgres://" scheme) is passed
//     through, because lib/pq accepts the URL form directly.
//   - mongodb: the full URL (including the "mongodb://" scheme) is passed
//     through, because the mongo driver's options.ApplyURI expects a complete
//     connection URI with its scheme intact.
//   - mysql / sqlite / cassandra / others: the scheme and any "//" separator
//     are stripped, because those drivers expect a bare DSN (mysql), file/URI
//     (sqlite), or host list (cassandra).
//
// It returns an error for an empty string or an unrecognised scheme, listing
// the schemes it understands.
func ParseConnectionString(conn string) (backend, dsn string, err error) {
	conn = strings.TrimSpace(conn)
	if conn == "" {
		return "", "", fmt.Errorf("datasource: empty connection string")
	}

	idx := strings.Index(conn, ":")
	if idx <= 0 {
		return "", "", fmt.Errorf("datasource: connection string %q missing scheme (want e.g. \"sqlite:...\" or \"postgres://...\")", conn)
	}
	scheme := strings.ToLower(conn[:idx])

	backend, ok := schemeBackends[scheme]
	if !ok {
		return "", "", fmt.Errorf("datasource: unknown scheme %q in %q (supported: %s)", scheme, conn, supportedSchemes())
	}

	switch backend {
	case BackendPostgres:
		// lib/pq parses the full URL itself.
		dsn = conn
	case BackendMongoDB:
		// The mongo driver's options.ApplyURI expects the full URI including
		// its "mongodb://" (or "mongodb+srv://") scheme, so pass it through
		// unchanged.
		dsn = conn
	default:
		// Strip "scheme:" and an optional "//" authority separator so the
		// driver receives the bare DSN/URI it expects.
		rest := conn[idx+1:]
		rest = strings.TrimPrefix(rest, "//")
		dsn = rest
	}
	return backend, dsn, nil
}

// supportedSchemes returns the recognised schemes as a sorted, comma-separated
// list for use in error messages.
func supportedSchemes() string {
	seen := map[string]bool{}
	var schemes []string
	for s := range schemeBackends {
		if !seen[s] {
			seen[s] = true
			schemes = append(schemes, s)
		}
	}
	// Stable order without importing sort twice; small slice.
	for i := 0; i < len(schemes); i++ {
		for j := i + 1; j < len(schemes); j++ {
			if schemes[j] < schemes[i] {
				schemes[i], schemes[j] = schemes[j], schemes[i]
			}
		}
	}
	return strings.Join(schemes, ", ")
}
