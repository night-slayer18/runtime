package datasource

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/parquet-go/parquet-go"
	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// registerParquet registers the Parquet format using parquet-go
// (github.com/parquet-go/parquet-go), a pure-Go Parquet implementation. The
// reader decodes the file footer/schema and streams row groups, so the format
// is genuinely available.
func registerParquet() {
	register(Format{
		Name:       "Parquet",
		Extensions: []string{"parquet"},
		Available:  true,
		Read:       readParquet,
	})
}

// readParquet opens a Parquet file, reads its schema into the grid's column
// model, and streams every row into the in-memory DataSource. Each Parquet leaf
// column becomes one grid column; values are rendered to the grid's string-cell
// representation faithfully, with nulls mapped to the empty string.
func readParquet(path string) (ds.DataSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open parquet %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat parquet %s: %w", path, err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("parse parquet %s: %w", path, err)
	}

	schema := pf.Schema()
	// Schema.Columns returns the dotted paths of every leaf column in column
	// order; the last path element is the column's name. This matches the
	// per-column ordering of the values returned by Reader.ReadRows.
	paths := schema.Columns()
	header := make([]string, len(paths))
	for i, p := range paths {
		if len(p) == 0 {
			header[i] = fmt.Sprintf("col%d", i)
			continue
		}
		header[i] = p[len(p)-1]
	}

	reader := parquet.NewReader(pf)
	defer reader.Close()

	colCount := len(header)
	var rows [][]string
	buf := make([]parquet.Row, 256)
	for {
		n, err := reader.ReadRows(buf)
		for i := 0; i < n; i++ {
			rows = append(rows, parquetRowToStrings(buf[i], colCount))
		}
		if err != nil {
			// io.EOF (and any error) is returned alongside the final batch; a
			// non-EOF error is a genuine decode failure.
			if isParquetEOF(err) {
				break
			}
			return nil, fmt.Errorf("read parquet rows: %w", err)
		}
		if n == 0 {
			break
		}
	}

	return buildSource(header, rows), nil
}

// parquetRowToStrings renders a Parquet row into colCount string cells. Each
// value is placed at its declared column index so the layout matches the
// schema; null values become the empty string.
func parquetRowToStrings(row parquet.Row, colCount int) []string {
	cells := make([]string, colCount)
	for _, v := range row {
		col := v.Column()
		if col < 0 || col >= colCount {
			continue
		}
		if v.IsNull() {
			cells[col] = ""
			continue
		}
		cells[col] = v.String()
	}
	return cells
}

// isParquetEOF reports whether err signals the end of the row stream. ReadRows
// returns io.EOF when it has read the last rows.
func isParquetEOF(err error) bool {
	return errors.Is(err, io.EOF)
}
