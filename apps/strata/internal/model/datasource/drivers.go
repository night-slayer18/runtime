// Driver wiring: the single, isolated place that pulls concrete drivers into
// the build and registers every backend with the registry.
//
// # What is wired and why
//
//   - SQLite (modernc.org/sqlite): WIRED. A pure-Go SQLite driver with no cgo
//     requirement, so it builds and runs in-process anywhere — ideal for the
//     in-process integration test and for local exploration.
//
//   - PostgreSQL (github.com/lib/pq): WIRED. Pure-Go, registers the "postgres"
//     database/sql driver.
//
//   - MySQL (github.com/go-sql-driver/mysql): WIRED. Pure-Go, registers the
//     "mysql" database/sql driver.
//
//   - MongoDB (go.mongodb.org/mongo-driver): WIRED. MongoDB is a document store
//     and is not database/sql compatible, so it cannot use the shared SQL
//     adapter; instead mongodb.go implements datasource.DataSource directly over
//     the official mongo client. It is registered with a real factory that
//     connects via the driver and reports a clear connection error when no
//     server is reachable.
//
//   - Cassandra (github.com/gocql/gocql): WIRED. Cassandra (CQL) is likewise not
//     database/sql compatible; cassandra.go implements datasource.DataSource
//     directly over gocql's native session API and is registered with a real
//     factory.
//
// The SQL adapter (sql.go) talks only to database/sql; the Mongo and Cassandra
// adapters talk to their native clients. Every backend is now a real,
// driver-backed connector, so Connect returns a genuine connection error (not
// ErrDriverUnavailable) when a server is unreachable.
package datasource

import (
	// Blank-imported drivers register themselves with database/sql via their
	// init functions. They are pure-Go and resolve through the module cache, so
	// they do not force cgo or system libraries.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func init() {
	// --- database/sql backends ---
	// Each factory is driver-agnostic: it asks database/sql to open the named
	// driver, so it works whenever the corresponding blank import above is
	// present and degrades to ErrDriverUnavailable when it is not.
	Register(BackendPostgres, sqlFactory("postgres", "github.com/lib/pq"))
	Register(BackendMySQL, sqlFactory("mysql", "github.com/go-sql-driver/mysql"))
	// modernc.org/sqlite registers itself under the driver name "sqlite".
	Register(BackendSQLite, sqlFactory("sqlite", "modernc.org/sqlite"))

	// --- native (non-database/sql) backends ---
	// Real driver-backed adapters; connection failures surface as connection
	// errors, never ErrDriverUnavailable.
	Register(BackendMongoDB, mongoFactory)
	Register(BackendCassandra, cassandraFactory)
}
