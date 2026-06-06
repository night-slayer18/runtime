// Cassandra adapter: a real datasource.DataSource backed by
// github.com/gocql/gocql. Cassandra speaks CQL over its own binary protocol and
// is not database/sql compatible, so this adapter uses gocql's native cluster
// and session API while presenting the standard Schema/Query/Execute/Close
// surface.
//
// # Connecting from a DSN
//
// The DSN is the comma-separated host list parsed from a "cassandra://" URL,
// optionally carrying a keyspace and credentials, e.g.
//
//	cassandra://host1,host2:9042/mykeyspace
//	cassandra://user:pass@host:9042/mykeyspace
//
// parseCassandraDSN turns that into a *gocql.ClusterConfig. A bounded connect
// timeout ensures Connect fails fast with a clear connection error (never
// ErrDriverUnavailable) when no node is reachable.
//
// # Schema and queries
//
//   - Schema() reads column metadata for the configured table from gocql's
//     keyspace metadata, mapping each column to a datasource.Column.
//   - Query() executes the supplied CQL and streams result rows back through a
//     RowIterator, mapping each gocql value to a scalar cell via gocqlToCell.
//   - Execute() runs a non-SELECT CQL statement; Cassandra does not report
//     affected-row counts, so Result is zero-valued on success.
package datasource

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// cassandraConnectTimeout bounds how long a connection attempt waits before
// reporting the cluster unreachable. It is a package variable so tests can
// shorten it without a live server.
var cassandraConnectTimeout = 5 * time.Second

// cassandraSource adapts a connected gocql session to datasource.DataSource.
type cassandraSource struct {
	session  *gocql.Session
	keyspace string
	// schemaTable is the table Schema() introspects, set via SetSchemaQuery.
	schemaTable string
}

// cassandraFactory is the ConnectionFactory registered for the Cassandra
// backend.
func cassandraFactory(dsn string) (ds.DataSource, error) {
	return openCassandra(dsn)
}

// openCassandra builds a cluster config from the DSN and establishes a session.
// A failure to reach any node is returned as a connection error.
func openCassandra(dsn string) (ds.DataSource, error) {
	cluster, keyspace, err := parseCassandraDSN(dsn)
	if err != nil {
		return nil, err
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("datasource: connect cassandra: %w", err)
	}
	return &cassandraSource{session: session, keyspace: keyspace}, nil
}

// parseCassandraDSN converts a bare Cassandra DSN (the form ParseConnectionString
// produces for "cassandra://...") into a gocql cluster config and keyspace.
//
// Accepted shapes (scheme already stripped):
//
//	host
//	host1,host2:9042
//	user:pass@host:9042/keyspace
func parseCassandraDSN(dsn string) (*gocql.ClusterConfig, string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, "", fmt.Errorf("datasource: empty cassandra connection string")
	}

	var user, pass string
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		cred := dsn[:at]
		dsn = dsn[at+1:]
		if c := strings.IndexByte(cred, ':'); c >= 0 {
			user, pass = cred[:c], cred[c+1:]
		} else {
			user = cred
		}
	}

	var keyspace string
	if slash := strings.IndexByte(dsn, '/'); slash >= 0 {
		keyspace = strings.TrimSpace(dsn[slash+1:])
		dsn = dsn[:slash]
	}

	hostPart := dsn
	port := 0
	// A trailing ":port" applies to the whole host list. Detect it on the last
	// host entry to avoid mis-parsing IPv6 (not supported in this simple form).
	hosts := strings.Split(hostPart, ",")
	if len(hosts) > 0 {
		last := hosts[len(hosts)-1]
		if c := strings.LastIndex(last, ":"); c >= 0 {
			p, err := parsePort(last[c+1:])
			if err != nil {
				return nil, "", fmt.Errorf("datasource: cassandra port: %w", err)
			}
			port = p
			hosts[len(hosts)-1] = last[:c]
		}
	}
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	cleaned := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h != "" {
			cleaned = append(cleaned, h)
		}
	}
	if len(cleaned) == 0 {
		return nil, "", fmt.Errorf("datasource: cassandra dsn %q has no hosts", dsn)
	}

	cluster := gocql.NewCluster(cleaned...)
	if port != 0 {
		cluster.Port = port
	}
	cluster.ConnectTimeout = cassandraConnectTimeout
	cluster.Timeout = cassandraConnectTimeout
	if keyspace != "" {
		cluster.Keyspace = keyspace
	}
	if user != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: user, Password: pass}
	}
	return cluster, keyspace, nil
}

// parsePort validates and parses a port number.
func parsePort(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty port")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid port %q", s)
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 || n > 65535 {
		return 0, fmt.Errorf("port %q out of range", s)
	}
	return n, nil
}

// SetSchemaQuery implements SchemaQueryConfigurable: for Cassandra the "schema
// query" is the name of the table whose columns Schema() reports. A
// "keyspace.table" form overrides the session keyspace for introspection.
func (c *cassandraSource) SetSchemaQuery(query string) {
	c.schemaTable = strings.TrimSpace(query)
}

// Schema reads column metadata for the configured table from gocql's keyspace
// metadata and maps it onto datasource columns.
func (c *cassandraSource) Schema() ([]ds.Column, error) {
	if c.session == nil {
		return nil, ds.ErrClosed
	}
	if c.schemaTable == "" {
		return nil, fmt.Errorf("datasource: no table configured; set the table via SetSchemaQuery first")
	}
	keyspace, table := c.resolveTable(c.schemaTable)
	if keyspace == "" {
		return nil, fmt.Errorf("datasource: cassandra schema: no keyspace (use \"keyspace.table\" or a DSN keyspace)")
	}

	meta, err := c.session.KeyspaceMetadata(keyspace)
	if err != nil {
		return nil, fmt.Errorf("datasource: cassandra metadata: %w", err)
	}
	tbl, ok := meta.Tables[table]
	if !ok {
		return nil, fmt.Errorf("datasource: cassandra table %q not found in keyspace %q", table, keyspace)
	}
	return columnsFromCassandraTable(tbl), nil
}

// columnsFromCassandraTable maps gocql table metadata onto datasource columns,
// ordering partition keys first, then clustering keys, then the remaining
// columns by name for a stable layout.
func columnsFromCassandraTable(tbl *gocql.TableMetadata) []ds.Column {
	primary := map[string]bool{}
	var cols []ds.Column
	for _, col := range tbl.PartitionKey {
		primary[col.Name] = true
		cols = append(cols, ds.Column{Name: col.Name, Type: col.Type.Type().String(), IsPrimary: true})
	}
	for _, col := range tbl.ClusteringColumns {
		primary[col.Name] = true
		cols = append(cols, ds.Column{Name: col.Name, Type: col.Type.Type().String(), IsPrimary: true})
	}
	rest := make([]string, 0, len(tbl.Columns))
	for name := range tbl.Columns {
		if !primary[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		col := tbl.Columns[name]
		cols = append(cols, ds.Column{Name: name, Type: col.Type.Type().String(), Nullable: true})
	}
	return cols
}

// resolveTable splits a "keyspace.table" reference, falling back to the session
// keyspace when only a table name is given.
func (c *cassandraSource) resolveTable(ref string) (keyspace, table string) {
	if dot := strings.IndexByte(ref, '.'); dot >= 0 {
		return ref[:dot], ref[dot+1:]
	}
	return c.keyspace, ref
}

// Query executes a CQL SELECT (or any row-returning statement) and streams the
// results through a RowIterator. Column names and types come from the iterator's
// result metadata, so the row shape always matches the query.
func (c *cassandraSource) Query(cql string) (ds.RowIterator, error) {
	if c.session == nil {
		return nil, ds.ErrClosed
	}
	q := strings.TrimSpace(cql)
	if q == "" {
		return nil, fmt.Errorf("datasource: empty cassandra query")
	}
	iter := c.session.Query(q).Iter()
	return newCassandraIterator(iter), nil
}

// Execute runs a non-row-returning CQL statement. Cassandra does not report an
// affected-row count, so a successful Execute returns a zero Result.
func (c *cassandraSource) Execute(cql string) (ds.Result, error) {
	if c.session == nil {
		return ds.Result{}, ds.ErrClosed
	}
	q := strings.TrimSpace(cql)
	if q == "" {
		return ds.Result{}, fmt.Errorf("datasource: empty cassandra statement")
	}
	if err := c.session.Query(q).Exec(); err != nil {
		return ds.Result{}, fmt.Errorf("datasource: cassandra execute: %w", err)
	}
	return ds.Result{}, nil
}

// Close closes the session. It is idempotent.
func (c *cassandraSource) Close() error {
	if c.session == nil {
		return nil
	}
	c.session.Close()
	c.session = nil
	return nil
}

// gocqlToCell normalises a value produced by gocql into a scalar cell value for
// the shared table/export layers. gocql already decodes CQL types into native
// Go types; this collapses the handful that need readable rendering.
func gocqlToCell(v interface{}) interface{} {
	switch val := v.(type) {
	case nil:
		return nil
	case string, bool, int, int8, int16, int32, int64, float32, float64:
		return val
	case []byte:
		return string(val)
	case time.Time:
		return val.UTC().Format(time.RFC3339Nano)
	case gocql.UUID:
		return val.String()
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprint(val)
	}
}

// newCassandraIterator buffers a gocql iterator's rows into a RowIterator. It
// reads all rows eagerly so the iterator can report a single terminal error via
// Err and so Scan destinations stay simple (*interface{} / *string), matching
// the other adapters and the model layer.
func newCassandraIterator(iter *gocql.Iter) *cassandraIterator {
	cols := iter.Columns()
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}

	var rows [][]interface{}
	for {
		rowMap := make(map[string]interface{}, len(cols))
		if !iter.MapScan(rowMap) {
			break
		}
		cells := make([]interface{}, len(cols))
		for i, name := range names {
			cells[i] = gocqlToCell(rowMap[name])
		}
		rows = append(rows, cells)
	}

	return &cassandraIterator{
		columns: names,
		rows:    rows,
		pos:     -1,
		err:     iter.Close(),
	}
}

// cassandraIterator streams buffered, projected CQL rows.
type cassandraIterator struct {
	columns []string
	rows    [][]interface{}
	pos     int
	err     error
	closed  bool
}

// Next advances to the next row.
func (it *cassandraIterator) Next() bool {
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

// Scan copies the current row's cells into dest. Destinations must be
// *interface{} or *string, matching the projection used by the model layer.
func (it *cassandraIterator) Scan(dest ...interface{}) error {
	if it.err != nil {
		return it.err
	}
	if it.closed {
		return ds.ErrClosed
	}
	if it.pos < 0 || it.pos >= len(it.rows) {
		return fmt.Errorf("datasource: Scan called without a current row")
	}
	row := it.rows[it.pos]
	if len(dest) != len(row) {
		return fmt.Errorf("%w: have %d, want %d", ds.ErrColumnCount, len(dest), len(row))
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *interface{}:
			*target = row[i]
		case *string:
			if row[i] == nil {
				*target = ""
			} else {
				*target = fmt.Sprint(row[i])
			}
		default:
			return fmt.Errorf("datasource: unsupported scan destination %T at column %d", d, i)
		}
	}
	return nil
}

// Err returns the error reported when the underlying gocql iterator closed.
func (it *cassandraIterator) Err() error { return it.err }

// Close releases the iterator. It is idempotent.
func (it *cassandraIterator) Close() error {
	it.closed = true
	return nil
}
