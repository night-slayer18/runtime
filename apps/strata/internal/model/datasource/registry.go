// Package datasource wires Runtime Strata to concrete databases behind the
// shared datasource.DataSource abstraction.
//
// # Driver-registry architecture (Requirement 7.4)
//
// Strata must support many database backends and "should support future
// database drivers" without changes to its core. To satisfy that, this package
// is built around a small registry: every backend registers a ConnectionFactory
// under a stable name, and the application connects purely by name + DSN. The
// concrete drivers are therefore pluggable — adding a new backend is a single
// Register call in an init function, and removing one never touches the UI or
// model code.
//
// # Driver availability and offline builds
//
// Database drivers are third-party packages. The SQL backends (PostgreSQL,
// MySQL, SQLite) are implemented as thin adapters over the standard library's
// database/sql, so the adapter itself has no compile-time dependency on any
// driver: it calls sql.Open(driverName, dsn). The concrete drivers are
// blank-imported in drivers.go, which is the single, isolated place that pulls
// them in. When a driver is present (registered with database/sql) the backend
// works end to end; when it is absent, Connect surfaces a clear
// "driver unavailable: install X" error instead of failing the build.
//
// MongoDB and Cassandra are not database/sql compatible — they need bespoke
// clients (go.mongodb.org/mongo-driver, github.com/gocql/gocql) with their own
// query models. They are implemented as dedicated datasource.DataSource
// adapters (mongodb.go, cassandra.go) and registered with real factories, so
// every backend is a genuine driver-backed connector. The Unavailable helper
// remains available for wiring future backends whose drivers are not yet
// compiled in.
package datasource

import (
	"fmt"
	"sort"
	"sync"

	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// Backend names recognised by the registry. They double as the keys callers
// pass to Connect and as the database/sql driver names for the SQL backends.
const (
	BackendPostgres  = "postgres"
	BackendMySQL     = "mysql"
	BackendSQLite    = "sqlite"
	BackendMongoDB   = "mongodb"
	BackendCassandra = "cassandra"
)

// ConnectionFactory opens a connection described by dsn and returns a
// datasource.DataSource backed by it. Factories must not panic; an unreachable
// server or unavailable driver is reported as an error.
type ConnectionFactory func(dsn string) (ds.DataSource, error)

// ErrDriverUnavailable is returned (wrapped) when a backend is registered but
// its underlying driver is not compiled into the binary. Callers can test for
// it with errors.Is to distinguish "missing driver" from "bad DSN" or
// "server unreachable".
var ErrDriverUnavailable = fmt.Errorf("datasource: driver unavailable")

// registry maps a backend name to its factory. It is guarded by mu so backends
// can register from init functions across files concurrently and callers can
// Connect from any goroutine.
var (
	mu       sync.RWMutex
	registry = map[string]ConnectionFactory{}
)

// Register installs (or replaces) the factory for the named backend. It is
// intended to be called from init functions; the last registration for a given
// name wins, which lets a real driver override a placeholder.
func Register(name string, factory ConnectionFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Backends returns the registered backend names in sorted order.
func Backends() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Registered reports whether a backend with the given name has a factory.
func Registered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := registry[name]
	return ok
}

// Connect resolves the named backend's factory and opens a connection to dsn.
// It returns a typed error when the backend is unknown and propagates the
// factory's error (which may wrap ErrDriverUnavailable) otherwise.
func Connect(backend, dsn string) (ds.DataSource, error) {
	mu.RLock()
	factory, ok := registry[backend]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("datasource: unknown backend %q (registered: %v)", backend, Backends())
	}
	return factory(dsn)
}

// Unavailable returns a ConnectionFactory that always fails with a clear,
// actionable message naming the module to install. All five shipped backends
// (PostgreSQL, MySQL, SQLite, MongoDB, Cassandra) are real driver-backed
// connectors, so this helper is currently unused at registration time; it is
// retained for wiring a future backend whose driver is intentionally left out
// of a given build, so the backend can still appear in Backends() as a
// documented extension point.
func Unavailable(backend, installModule string) ConnectionFactory {
	return func(string) (ds.DataSource, error) {
		return nil, fmt.Errorf("%w: %s requires %q; build Strata with that driver to enable it",
			ErrDriverUnavailable, backend, installModule)
	}
}
