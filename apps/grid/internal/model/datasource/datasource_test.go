package datasource_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/parquet-go/parquet-go"
	gridds "github.com/runtime-sh/runtime/apps/grid/internal/model/datasource"
	ds "github.com/runtime-sh/runtime/packages/datasource"
	"github.com/runtime-sh/runtime/packages/export"
)

// scanAll drains a DataSource into header names and string rows.
func scanAll(t *testing.T, source ds.DataSource) ([]string, [][]string) {
	t.Helper()
	cols, err := source.Schema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	it, err := source.Query("")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = it.Close() }()
	var rows [][]string
	for it.Next() {
		dest := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := it.Scan(ptrs...); err != nil {
			t.Fatalf("scan: %v", err)
		}
		row := make([]string, len(cols))
		for i, v := range dest {
			if v != nil {
				row[i], _ = v.(string)
			}
		}
		rows = append(rows, row)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return names, rows
}

func TestOpenCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.csv")
	if err := os.WriteFile(path, []byte("a,b\n1,2\n3,4\n"), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	source, err := gridds.Open(path)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer func() { _ = source.Close() }()
	names, rows := scanAll(t, source)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("header = %v", names)
	}
	if len(rows) != 2 || rows[0][0] != "1" || rows[1][1] != "4" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestOpenTSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.tsv")
	if err := os.WriteFile(path, []byte("x\ty\n10\t20\n"), 0o600); err != nil {
		t.Fatalf("write tsv: %v", err)
	}

	source, err := gridds.Open(path)
	if err != nil {
		t.Fatalf("open tsv: %v", err)
	}
	defer func() { _ = source.Close() }()
	names, rows := scanAll(t, source)
	if names[0] != "x" || names[1] != "y" {
		t.Fatalf("header = %v", names)
	}
	if rows[0][0] != "10" || rows[0][1] != "20" {
		t.Fatalf("rows = %v", rows)
	}
}

// TestOpenXLSX_RoundTrip writes an .xlsx with the export package's stdlib writer
// then reads it back with the stdlib reader, confirming the XLSX path is real.
func TestOpenXLSX_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	dataset := &export.Dataset{
		Columns: []string{"id", "name"},
		Rows: [][]interface{}{
			{"1", "Ada"},
			{"2", "Linus"},
		},
	}
	if err := (export.XLSXExporter{}).Export(f, dataset); err != nil {
		_ = f.Close()
		t.Fatalf("export xlsx: %v", err)
	}
	_ = f.Close()

	source, err := gridds.Open(path)
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer func() { _ = source.Close() }()
	names, rows := scanAll(t, source)
	if len(names) != 2 || names[0] != "id" || names[1] != "name" {
		t.Fatalf("header = %v", names)
	}
	if len(rows) != 2 || rows[0][1] != "Ada" || rows[1][1] != "Linus" {
		t.Fatalf("rows = %v", rows)
	}
}

// TestOpenParquetGarbageFails confirms that a non-Parquet file with a .parquet
// extension fails to decode rather than silently succeeding.
func TestOpenParquetGarbageFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.parquet")
	if err := os.WriteFile(path, []byte("not really parquet"), 0o600); err != nil {
		t.Fatalf("write parquet: %v", err)
	}

	if _, err := gridds.Open(path); err == nil {
		t.Fatal("expected garbage Parquet open to fail")
	}
}

// parquetPerson is the row schema written by the Parquet round-trip test. The
// parquet struct tags name the leaf columns, which the reader maps to grid
// column names.
type parquetPerson struct {
	ID   int64  `parquet:"id"`
	Name string `parquet:"name"`
	Role string `parquet:"role"`
}

// TestOpenParquet_RoundTrip writes a small Parquet file with parquet-go's
// generic writer, reads it back through the real reader, and asserts schema and
// row fidelity.
func TestOpenParquet_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.parquet")

	want := []parquetPerson{
		{ID: 1, Name: "Ada", Role: "engineer"},
		{ID: 2, Name: "Linus", Role: "maintainer"},
		{ID: 3, Name: "Grace", Role: "admiral"},
	}
	if err := parquet.WriteFile(path, want); err != nil {
		t.Fatalf("write parquet: %v", err)
	}

	source, err := gridds.Open(path)
	if err != nil {
		t.Fatalf("open parquet: %v", err)
	}
	defer func() { _ = source.Close() }()

	names, rows := scanAll(t, source)
	wantNames := []string{"id", "name", "role"}
	if len(names) != len(wantNames) {
		t.Fatalf("header = %v, want %v", names, wantNames)
	}
	for i, n := range wantNames {
		if names[i] != n {
			t.Fatalf("header[%d] = %q, want %q", i, names[i], n)
		}
	}
	if len(rows) != len(want) {
		t.Fatalf("row count = %d, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i][0] != strconv.FormatInt(w.ID, 10) {
			t.Errorf("row %d id = %q, want %d", i, rows[i][0], w.ID)
		}
		if rows[i][1] != w.Name {
			t.Errorf("row %d name = %q, want %q", i, rows[i][1], w.Name)
		}
		if rows[i][2] != w.Role {
			t.Errorf("row %d role = %q, want %q", i, rows[i][2], w.Role)
		}
	}
}

// TestOpenArrow_RoundTrip writes a small Arrow IPC file with arrow/ipc's file
// writer, reads it back through the real reader, and asserts schema and row
// fidelity (including a null cell mapping to the empty string).
func TestOpenArrow_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.arrow")

	pool := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "name", Type: arrow.BinaryTypes.String},
	}, nil)

	b := array.NewRecordBuilder(pool, schema)
	defer b.Release()
	b.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2, 3}, nil)
	nameB := b.Field(1).(*array.StringBuilder)
	nameB.Append("Ada")
	nameB.AppendNull()
	nameB.Append("Grace")
	rec := b.NewRecordBatch()
	defer rec.Release()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create arrow file: %v", err)
	}
	w, err := ipc.NewFileWriter(f, ipc.WithSchema(schema), ipc.WithAllocator(pool))
	if err != nil {
		_ = f.Close()
		t.Fatalf("new arrow writer: %v", err)
	}
	if err := w.Write(rec); err != nil {
		_ = f.Close()
		t.Fatalf("write arrow record: %v", err)
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		t.Fatalf("close arrow writer: %v", err)
	}
	_ = f.Close()

	source, err := gridds.Open(path)
	if err != nil {
		t.Fatalf("open arrow: %v", err)
	}
	defer func() { _ = source.Close() }()

	names, rows := scanAll(t, source)
	if len(names) != 2 || names[0] != "id" || names[1] != "name" {
		t.Fatalf("header = %v", names)
	}
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	if rows[0][0] != "1" || rows[0][1] != "Ada" {
		t.Errorf("row 0 = %v, want [1 Ada]", rows[0])
	}
	// The null name cell must render as the empty string.
	if rows[1][0] != "2" || rows[1][1] != "" {
		t.Errorf("row 1 = %v, want [2 <empty>]", rows[1])
	}
	if rows[2][0] != "3" || rows[2][1] != "Grace" {
		t.Errorf("row 2 = %v, want [3 Grace]", rows[2])
	}
}
