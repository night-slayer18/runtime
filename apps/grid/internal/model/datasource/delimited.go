package datasource

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// registerDelimited registers the CSV and TSV formats. Both are decoded with
// encoding/csv from the standard library (TSV is CSV with a tab delimiter), so
// they are always available regardless of network access.
func registerDelimited() {
	register(Format{
		Name:       "CSV",
		Extensions: []string{"csv"},
		Available:  true,
		Read:       func(path string) (ds.DataSource, error) { return readDelimited(path, ',') },
	})
	register(Format{
		Name:       "TSV",
		Extensions: []string{"tsv", "tab"},
		Available:  true,
		Read:       func(path string) (ds.DataSource, error) { return readDelimited(path, '\t') },
	})
}

// readDelimited parses a delimiter-separated file into a DataSource. The first
// record is the header (column names); the remaining records are data rows.
// Records with a varying number of fields are tolerated (FieldsPerRecord = -1)
// and normalised to the header width by buildSource.
func readDelimited(path string, delim rune) (ds.DataSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = delim
	r.FieldsPerRecord = -1 // allow ragged rows; buildSource normalises them
	r.LazyQuotes = true

	header, err := r.Read()
	if err == io.EOF {
		// An empty file yields an empty schema and no rows.
		return buildSource(nil, nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read record: %w", err)
		}
		rows = append(rows, rec)
	}
	return buildSource(header, rows), nil
}
