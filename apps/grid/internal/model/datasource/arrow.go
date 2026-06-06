package datasource

import (
	"fmt"
	"os"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// registerArrow registers the Arrow format, reading the Arrow IPC file format
// (Feather v2 / .arrow / .ipc) via github.com/apache/arrow-go/v18/arrow/ipc.
// The reader decodes the file footer/schema and iterates record batches, so the
// format is genuinely available.
func registerArrow() {
	register(Format{
		Name:       "Arrow",
		Extensions: []string{"arrow", "ipc", "feather"},
		Available:  true,
		Read:       readArrow,
	})
}

// readArrow opens an Arrow IPC file, reads its schema into the grid's column
// model, and iterates every record batch, mapping each column/row into the
// grid's string-cell representation. Null values become the empty string.
func readArrow(path string) (ds.DataSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open arrow %s: %w", path, err)
	}
	defer f.Close()

	r, err := ipc.NewFileReader(f, ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return nil, fmt.Errorf("parse arrow %s: %w", path, err)
	}
	defer r.Close()

	schema := r.Schema()
	fields := schema.Fields()
	header := make([]string, len(fields))
	for i, fld := range fields {
		header[i] = fld.Name
	}
	colCount := len(header)

	var rows [][]string
	for i := 0; i < r.NumRecords(); i++ {
		rec, err := r.RecordBatch(i)
		if err != nil {
			return nil, fmt.Errorf("read arrow record %d: %w", i, err)
		}
		rows = append(rows, arrowRecordToStrings(rec, colCount)...)
	}

	return buildSource(header, rows), nil
}

// arrowRecordToStrings flattens an Arrow record batch into string rows in
// row-major order. Each column array's value at row index r is rendered with
// ValueStr; nulls map to the empty string. Missing columns are left as the
// empty string so every row matches the schema width.
func arrowRecordToStrings(rec arrow.RecordBatch, colCount int) [][]string {
	nRows := int(rec.NumRows())
	nCols := int(rec.NumCols())
	out := make([][]string, nRows)
	for ri := 0; ri < nRows; ri++ {
		cells := make([]string, colCount)
		for ci := 0; ci < nCols && ci < colCount; ci++ {
			cells[ci] = arrowCellValue(rec.Column(ci), ri)
		}
		out[ri] = cells
	}
	return out
}

// arrowCellValue renders the value at row index ri of an Arrow column array as
// a string, mapping nulls to the empty string.
func arrowCellValue(col arrow.Array, ri int) string {
	if col.IsNull(ri) {
		return ""
	}
	return col.ValueStr(ri)
}
