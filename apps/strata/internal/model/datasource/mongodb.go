// MongoDB adapter: a real datasource.DataSource backed by the official
// go.mongodb.org/mongo-driver client. MongoDB is a document store and is not
// database/sql compatible, so this adapter speaks the driver's native API
// directly while presenting the same Schema/Query/Execute/Close surface every
// other Strata backend exposes.
//
// # Mapping the document model onto rows and columns
//
// Strata renders tabular result sets, so this adapter projects documents onto a
// fixed column set:
//
//   - Schema() samples one document from the configured collection and infers a
//     column per top-level field, preserving the document's field order. An
//     empty collection yields an empty (but valid) schema.
//   - Query() treats its argument as a collection name, an optional collection
//     name followed by an extended-JSON filter, or a bare extended-JSON filter
//     applied to the schema collection. Matching documents are projected onto
//     the schema's field order (or, when no schema has been read, onto the union
//     of fields discovered across the result) and streamed back row by row.
//   - BSON values are normalised to scalar cells via bsonToCell so the shared
//     table and export layers render readable values.
//
// Connection failures (unreachable server, bad URI) surface as plain errors —
// never ErrDriverUnavailable, which is reserved for "driver not compiled in".
package datasource

import (
	"context"
	"fmt"
	"strings"
	"time"

	ds "github.com/runtime-sh/runtime/packages/datasource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoServerSelectionTimeout bounds how long Connect waits to select a server
// before reporting the host unreachable. It is only applied when the connection
// URI does not specify serverSelectionTimeoutMS itself, and is a package
// variable so tests can shorten it without depending on a live server.
var mongoServerSelectionTimeout = 5 * time.Second

// mongoOperationTimeout bounds individual Schema/Query operations.
var mongoOperationTimeout = 10 * time.Second

// mongoSource adapts a connected *mongo.Client to datasource.DataSource.
type mongoSource struct {
	client *mongo.Client
	db     *mongo.Database
	// schemaCollection is the collection Schema() samples and the default
	// collection Query() targets when only a filter is supplied. It is set via
	// SetSchemaQuery (the SchemaQueryConfigurable contract).
	schemaCollection string
	// fields records the field order discovered by the most recent Schema call
	// so Query projects documents onto a stable column layout.
	fields []string
}

// mongoFactory is the ConnectionFactory registered for the MongoDB backend.
func mongoFactory(dsn string) (ds.DataSource, error) {
	return openMongo(dsn)
}

// openMongo connects to MongoDB using the supplied connection URI. It applies a
// bounded server-selection timeout and pings the deployment so an unreachable
// server is reported immediately as a connection error.
func openMongo(uri string) (ds.DataSource, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, fmt.Errorf("datasource: empty mongodb connection string")
	}

	opts := options.Client().ApplyURI(uri)
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("datasource: mongodb uri: %w", err)
	}
	if opts.ServerSelectionTimeout == nil {
		opts.SetServerSelectionTimeout(mongoServerSelectionTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectDeadline(opts))
	defer cancel()

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("datasource: connect mongodb: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("datasource: connect mongodb: %w", err)
	}

	dbName := databaseFromMongoURI(uri)
	if dbName == "" {
		dbName = "test"
	}
	return &mongoSource{client: client, db: client.Database(dbName)}, nil
}

// connectDeadline derives an overall connect deadline from the configured
// server-selection timeout, leaving headroom for the ping round trip.
func connectDeadline(opts *options.ClientOptions) time.Duration {
	d := mongoServerSelectionTimeout
	if opts.ServerSelectionTimeout != nil {
		d = *opts.ServerSelectionTimeout
	}
	d += 2 * time.Second
	return d
}

// SetSchemaQuery implements SchemaQueryConfigurable: for MongoDB the "schema
// query" is the name of the collection to sample and explore.
func (m *mongoSource) SetSchemaQuery(query string) {
	m.schemaCollection = strings.TrimSpace(query)
}

// Schema samples a single document from the configured collection and derives a
// column per top-level field, preserving field order. An empty collection
// returns an empty schema rather than an error.
func (m *mongoSource) Schema() ([]ds.Column, error) {
	if m.client == nil {
		return nil, ds.ErrClosed
	}
	if m.schemaCollection == "" {
		return nil, fmt.Errorf("datasource: no collection configured; set the collection via SetSchemaQuery first")
	}
	ctx, cancel := m.opContext()
	defer cancel()

	var doc bson.D
	err := m.db.Collection(m.schemaCollection).FindOne(ctx, bson.D{}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		m.fields = nil
		return []ds.Column{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("datasource: mongodb schema: %w", err)
	}
	cols := columnsFromDoc(doc)
	m.fields = columnNames(cols)
	return cols, nil
}

// Query runs a find against the resolved collection and streams matching
// documents back as rows. See parseMongoQuery for the accepted forms.
func (m *mongoSource) Query(query string) (ds.RowIterator, error) {
	if m.client == nil {
		return nil, ds.ErrClosed
	}
	collection, filter, err := parseMongoQuery(query, m.schemaCollection)
	if err != nil {
		return nil, err
	}
	if collection == "" {
		return nil, fmt.Errorf("datasource: mongodb query: no collection specified")
	}

	ctx, cancel := m.opContext()
	defer cancel()

	cursor, err := m.db.Collection(collection).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("datasource: mongodb query: %w", err)
	}
	var docs []bson.D
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("datasource: mongodb query: %w", err)
	}

	fields := m.fields
	if len(fields) == 0 {
		fields = unionFields(docs)
	}
	return newMongoIterator(fields, docs), nil
}

// Execute is not supported for MongoDB: document writes do not map onto the
// SQL-style Result contract. It returns a clear error so callers fail loudly
// rather than silently succeeding.
func (m *mongoSource) Execute(string) (ds.Result, error) {
	if m.client == nil {
		return ds.Result{}, ds.ErrClosed
	}
	return ds.Result{}, fmt.Errorf("datasource: mongodb does not support Execute; use Query with a collection and filter")
}

// Close disconnects the client. It is idempotent.
func (m *mongoSource) Close() error {
	if m.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), mongoOperationTimeout)
	defer cancel()
	err := m.client.Disconnect(ctx)
	m.client = nil
	m.db = nil
	return err
}

// opContext returns a bounded context for a single operation.
func (m *mongoSource) opContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), mongoOperationTimeout)
}

// parseMongoQuery interprets a Strata query string for MongoDB. Accepted forms:
//
//	"users"                      -> collection "users", empty filter
//	"users {\"active\": true}"   -> collection "users", the given filter
//	"{\"active\": true}"         -> default collection, the given filter
//	""                            -> default collection, empty filter
//
// Filters are parsed as MongoDB extended JSON so operators like {"$gt": 1} work.
func parseMongoQuery(query, defaultCollection string) (collection string, filter bson.D, err error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return defaultCollection, bson.D{}, nil
	}
	if strings.HasPrefix(q, "{") {
		f, perr := parseMongoFilter(q)
		if perr != nil {
			return "", nil, perr
		}
		return defaultCollection, f, nil
	}
	if i := strings.IndexByte(q, '{'); i >= 0 {
		coll := strings.TrimSpace(q[:i])
		f, perr := parseMongoFilter(q[i:])
		if perr != nil {
			return "", nil, perr
		}
		return coll, f, nil
	}
	// Bare collection name (use the first whitespace-delimited token).
	return strings.Fields(q)[0], bson.D{}, nil
}

// parseMongoFilter decodes a MongoDB extended-JSON filter.
func parseMongoFilter(s string) (bson.D, error) {
	var f bson.D
	if err := bson.UnmarshalExtJSON([]byte(s), true, &f); err != nil {
		return nil, fmt.Errorf("datasource: mongodb filter %q: %w", s, err)
	}
	return f, nil
}

// databaseFromMongoURI extracts the default database name from a MongoDB
// connection URI, returning "" when none is present. It tolerates multi-host
// URIs and srv records by inspecting only the path segment after the authority.
func databaseFromMongoURI(uri string) string {
	s := uri
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(s[i+1:])
}

// columnsFromDoc derives a column per top-level field in a BSON document,
// preserving the document's field order.
func columnsFromDoc(doc bson.D) []ds.Column {
	cols := make([]ds.Column, 0, len(doc))
	for _, e := range doc {
		cols = append(cols, ds.Column{
			Name:      e.Key,
			Type:      bsonTypeName(e.Value),
			Nullable:  true,
			IsPrimary: e.Key == "_id",
		})
	}
	return cols
}

// columnNames returns the names of cols in order.
func columnNames(cols []ds.Column) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names
}

// unionFields computes a stable, first-seen-ordered union of the top-level
// field names across docs. It is used when Query runs without a prior Schema
// call so the result still has a deterministic column layout.
func unionFields(docs []bson.D) []string {
	seen := map[string]bool{}
	var fields []string
	for _, doc := range docs {
		for _, e := range doc {
			if !seen[e.Key] {
				seen[e.Key] = true
				fields = append(fields, e.Key)
			}
		}
	}
	return fields
}

// bsonTypeName returns a short, human-readable type name for a BSON value, used
// to populate Column.Type.
func bsonTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case float64:
		return "double"
	case bool:
		return "bool"
	case bson.ObjectID:
		return "objectId"
	case bson.DateTime:
		return "date"
	case bson.Decimal128:
		return "decimal"
	case bson.Binary:
		return "binary"
	case bson.A:
		return "array"
	case bson.D, bson.M:
		return "object"
	default:
		return "mixed"
	}
}

// bsonToCell normalises a BSON value into a scalar cell value suitable for the
// shared table/export layers. Scalars pass through; ObjectIDs, dates and
// decimals become readable strings; nested documents and arrays are rendered as
// extended JSON.
func bsonToCell(v interface{}) interface{} {
	switch val := v.(type) {
	case nil:
		return nil
	case string, bool, int32, int64, float64:
		return val
	case int:
		return int64(val)
	case bson.ObjectID:
		return val.Hex()
	case bson.DateTime:
		return val.Time().UTC().Format(time.RFC3339Nano)
	case bson.Decimal128:
		return val.String()
	case bson.Binary:
		return fmt.Sprintf("Binary(%d bytes)", len(val.Data))
	case bson.A, bson.D, bson.M:
		if b, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: val}}, false, false); err == nil {
			// Strip the {"v":...} wrapper to expose just the value's JSON.
			s := string(b)
			s = strings.TrimPrefix(s, `{"v":`)
			s = strings.TrimSuffix(s, "}")
			return strings.TrimSpace(s)
		}
		return fmt.Sprint(val)
	default:
		return fmt.Sprint(val)
	}
}

// docCells projects a document onto the given field order, returning a cell per
// field (nil when a field is absent from the document).
func docCells(doc bson.D, fields []string) []interface{} {
	index := make(map[string]interface{}, len(doc))
	for _, e := range doc {
		index[e.Key] = e.Value
	}
	cells := make([]interface{}, len(fields))
	for i, f := range fields {
		if raw, ok := index[f]; ok {
			cells[i] = bsonToCell(raw)
		} else {
			cells[i] = nil
		}
	}
	return cells
}

// newMongoIterator builds a RowIterator over a buffered document set projected
// onto fields.
func newMongoIterator(fields []string, docs []bson.D) *mongoIterator {
	rows := make([][]interface{}, len(docs))
	for i, doc := range docs {
		rows[i] = docCells(doc, fields)
	}
	return &mongoIterator{fields: fields, rows: rows, pos: -1}
}

// mongoIterator streams buffered, projected documents. It mirrors the cursor
// contract used by the SQL and in-memory iterators.
type mongoIterator struct {
	fields []string
	rows   [][]interface{}
	pos    int
	closed bool
}

// Next advances to the next row.
func (it *mongoIterator) Next() bool {
	if it.closed {
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
func (it *mongoIterator) Scan(dest ...interface{}) error {
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

// Err always returns nil: the iterator operates on a fully buffered result, so
// any read error is surfaced by Query before the iterator is returned.
func (it *mongoIterator) Err() error { return nil }

// Close releases the iterator. It is idempotent.
func (it *mongoIterator) Close() error {
	it.closed = true
	return nil
}
