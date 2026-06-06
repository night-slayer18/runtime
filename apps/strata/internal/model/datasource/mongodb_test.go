package datasource

import (
	"errors"
	"testing"
	"time"

	ds "github.com/runtime-sh/runtime/packages/datasource"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestMongoFactory_RegisteredAsRealConnector confirms the MongoDB backend is
// registered with the real factory (not the Unavailable stub).
func TestMongoFactory_RegisteredAsRealConnector(t *testing.T) {
	if !Registered(BackendMongoDB) {
		t.Fatal("mongodb backend not registered")
	}
}

// TestOpenMongo_EmptyDSN verifies an empty connection string fails clearly
// without attempting a network connection.
func TestOpenMongo_EmptyDSN(t *testing.T) {
	if _, err := openMongo("   "); err == nil {
		t.Fatal("expected error for empty mongodb dsn")
	}
}

// TestConnectMongo_UnreachableReturnsConnectionError verifies that connecting
// to an unreachable server returns a real connection error, NOT
// ErrDriverUnavailable. It uses a short server-selection timeout so it fails
// fast.
func TestConnectMongo_UnreachableReturnsConnectionError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent connect test in -short mode")
	}
	// 127.0.0.1:1 is a closed port; the driver fails server selection quickly.
	_, err := Connect(BackendMongoDB, "mongodb://127.0.0.1:1/db?serverSelectionTimeoutMS=500&connectTimeoutMS=500")
	if err == nil {
		t.Fatal("expected a connection error, got nil")
	}
	if errors.Is(err, ErrDriverUnavailable) {
		t.Fatalf("got ErrDriverUnavailable, want a real connection error: %v", err)
	}
}

// TestParseMongoQuery covers the accepted query forms.
func TestParseMongoQuery(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		defaultColl    string
		wantCollection string
		wantFilterLen  int
		wantErr        bool
	}{
		{name: "empty uses default with empty filter", query: "", defaultColl: "users", wantCollection: "users", wantFilterLen: 0},
		{name: "bare collection name", query: "orders", defaultColl: "users", wantCollection: "orders", wantFilterLen: 0},
		{name: "collection plus filter", query: `orders {"active": true}`, defaultColl: "users", wantCollection: "orders", wantFilterLen: 1},
		{name: "bare filter uses default", query: `{"active": true}`, defaultColl: "users", wantCollection: "users", wantFilterLen: 1},
		{name: "invalid filter errors", query: `users {not json}`, defaultColl: "users", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coll, filter, err := parseMongoQuery(tc.query, tc.defaultColl)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if coll != tc.wantCollection {
				t.Errorf("collection = %q, want %q", coll, tc.wantCollection)
			}
			if len(filter) != tc.wantFilterLen {
				t.Errorf("filter len = %d, want %d", len(filter), tc.wantFilterLen)
			}
		})
	}
}

// TestDatabaseFromMongoURI covers default-database extraction.
func TestDatabaseFromMongoURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"mongodb://localhost:27017/app", "app"},
		{"mongodb://localhost:27017", ""},
		{"mongodb://localhost:27017/", ""},
		{"mongodb://user:pass@h1,h2:27017/mydb?replicaSet=rs0", "mydb"},
		{"mongodb+srv://cluster.example.com/prod?retryWrites=true", "prod"},
	}
	for _, tc := range tests {
		if got := databaseFromMongoURI(tc.uri); got != tc.want {
			t.Errorf("databaseFromMongoURI(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

// TestColumnsFromDoc verifies schema inference preserves field order and flags
// the _id primary key.
func TestColumnsFromDoc(t *testing.T) {
	doc := bson.D{
		{Key: "_id", Value: bson.NewObjectID()},
		{Key: "name", Value: "Ada"},
		{Key: "age", Value: int32(36)},
		{Key: "active", Value: true},
	}
	cols := columnsFromDoc(doc)
	if len(cols) != 4 {
		t.Fatalf("column count = %d, want 4", len(cols))
	}
	wantNames := []string{"_id", "name", "age", "active"}
	for i, c := range cols {
		if c.Name != wantNames[i] {
			t.Errorf("col %d name = %q, want %q", i, c.Name, wantNames[i])
		}
	}
	if !cols[0].IsPrimary {
		t.Error("_id column should be marked primary")
	}
	if cols[1].Type != "string" || cols[2].Type != "int32" || cols[3].Type != "bool" {
		t.Errorf("unexpected types: %v", cols)
	}
}

// TestBsonToCell verifies BSON-value to scalar-cell mapping.
func TestBsonToCell(t *testing.T) {
	oid := bson.NewObjectID()
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	dt := bson.NewDateTimeFromTime(when)

	tests := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"nil", nil, nil},
		{"string", "hello", "hello"},
		{"int32", int32(7), int32(7)},
		{"int64", int64(9), int64(9)},
		{"int promotes to int64", 5, int64(5)},
		{"double", 3.5, 3.5},
		{"bool", true, true},
		{"objectId hex", oid, oid.Hex()},
		{"datetime rfc3339", dt, when.Format(time.RFC3339Nano)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := bsonToCell(tc.in); got != tc.want {
				t.Errorf("bsonToCell(%v) = %v (%T), want %v (%T)", tc.in, got, got, tc.want, tc.want)
			}
		})
	}
}

// TestBsonToCell_NestedDocument renders nested documents as JSON strings.
func TestBsonToCell_NestedDocument(t *testing.T) {
	nested := bson.D{{Key: "city", Value: "Paris"}}
	got := bsonToCell(nested)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("nested doc cell = %T, want string", got)
	}
	if s == "" {
		t.Fatal("nested doc rendered as empty string")
	}
}

// TestMongoIterator_StreamsProjectedRows verifies the iterator projects
// documents onto a fixed field order, filling absent fields with nil.
func TestMongoIterator_StreamsProjectedRows(t *testing.T) {
	fields := []string{"_id", "name", "age"}
	docs := []bson.D{
		{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "Ada"}, {Key: "age", Value: int32(36)}},
		{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "Linus"}}, // age missing
	}
	it := newMongoIterator(fields, docs)
	defer it.Close()

	var rows [][]interface{}
	for it.Next() {
		dest := make([]interface{}, len(fields))
		holders := make([]interface{}, len(fields))
		for i := range holders {
			dest[i] = &holders[i]
		}
		if err := it.Scan(dest...); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows = append(rows, holders)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[0][1] != "Ada" || rows[0][2] != int32(36) {
		t.Errorf("row0 = %v, want [1 Ada 36]", rows[0])
	}
	if rows[1][2] != nil {
		t.Errorf("row1 age = %v, want nil (missing field)", rows[1][2])
	}
}

// TestMongoIterator_ColumnCountMismatch verifies Scan rejects a wrong
// destination count.
func TestMongoIterator_ColumnCountMismatch(t *testing.T) {
	it := newMongoIterator([]string{"a", "b"}, []bson.D{{{Key: "a", Value: 1}, {Key: "b", Value: 2}}})
	defer it.Close()
	if !it.Next() {
		t.Fatal("expected a row")
	}
	var one interface{}
	if err := it.Scan(&one); !errors.Is(err, ds.ErrColumnCount) {
		t.Fatalf("error = %v, want ErrColumnCount", err)
	}
}
