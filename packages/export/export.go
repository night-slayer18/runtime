// Package export provides data export utilities for Runtime applications.
//
// The package defines a small Exporter interface and four concrete
// implementations that share a common, dependency-free data model:
//
//   - CSVExporter  – comma-separated values via encoding/csv
//   - JSONExporter – an array of column→value objects via encoding/json
//   - XMLExporter  – a <dataset>/<row>/<cell> document via encoding/xml
//   - XLSXExporter – a minimal, valid Office Open XML spreadsheet built from
//     archive/zip + encoding/xml (no third-party dependency required)
//
// Exporters operate on a Dataset (a set of named columns and rows). To compose
// with the datasource package, FromIterator drains a datasource.RowIterator
// into a Dataset, so any DataSource query result can be exported directly.
package export

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/runtime-sh/runtime/packages/datasource"
)

// Dataset is the common in-memory representation exporters operate on. Columns
// holds the ordered column names and Rows holds the row values; every row is
// expected to have len(Columns) cells, though exporters tolerate short or long
// rows by padding/truncating to the column count.
type Dataset struct {
	Columns []string
	Rows    [][]interface{}
}

// Exporter renders a dataset to a writer in a specific format.
type Exporter interface {
	// Export writes data to w. data must be a *Dataset or Dataset; any other
	// type results in an error.
	Export(w io.Writer, data interface{}) error
	// ContentType returns the MIME type produced by the exporter.
	ContentType() string
	// Extension returns the file extension (without a leading dot).
	Extension() string
}

// coerce normalises the data argument accepted by every Exporter into a
// *Dataset. It accepts *Dataset and Dataset values.
func coerce(data interface{}) (*Dataset, error) {
	switch d := data.(type) {
	case *Dataset:
		if d == nil {
			return nil, fmt.Errorf("export: nil *Dataset")
		}
		return d, nil
	case Dataset:
		return &d, nil
	default:
		return nil, fmt.Errorf("export: unsupported data type %T, want *export.Dataset or export.Dataset", data)
	}
}

// cell returns the i-th cell of row, or nil when the row is shorter than i+1.
func cell(row []interface{}, i int) interface{} {
	if i < 0 || i >= len(row) {
		return nil
	}
	return row[i]
}

// cellString renders a cell value as a string. nil becomes the empty string and
// every other value uses a stable textual form.
func cellString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	case bool:
		return strconv.FormatBool(val)
	case time.Time:
		return val.Format(time.RFC3339)
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprint(val)
	}
}

// FromIterator drains it into a Dataset using the supplied columns to name and
// size each row. It scans every row into interface{} destinations, so it works
// with any RowIterator whose Scan supports *interface{} targets (such as the
// datasource in-memory source). The iterator is not closed; callers retain
// ownership and should Close it.
func FromIterator(it datasource.RowIterator, columns []datasource.Column) (*Dataset, error) {
	if it == nil {
		return nil, fmt.Errorf("export: nil RowIterator")
	}
	ds := &Dataset{
		Columns: make([]string, len(columns)),
		Rows:    [][]interface{}{},
	}
	for i, c := range columns {
		ds.Columns[i] = c.Name
	}

	n := len(columns)
	for it.Next() {
		row := make([]interface{}, n)
		dest := make([]interface{}, n)
		for i := range row {
			dest[i] = &row[i]
		}
		if err := it.Scan(dest...); err != nil {
			return nil, fmt.Errorf("export: scan row: %w", err)
		}
		ds.Rows = append(ds.Rows, row)
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("export: iterate rows: %w", err)
	}
	return ds, nil
}

// ExportIterator is a convenience helper that drains a datasource.RowIterator
// using the given schema and writes it through the supplied Exporter.
func ExportIterator(e Exporter, w io.Writer, it datasource.RowIterator, columns []datasource.Column) error {
	ds, err := FromIterator(it, columns)
	if err != nil {
		return err
	}
	return e.Export(w, ds)
}

// CSVExporter writes a dataset as comma-separated values with a header row.
//
// Output is standard RFC 4180 CSV (via encoding/csv) so it interoperates with
// any conforming reader. Note one inherent CSV format limitation that affects
// round-tripping: a record consisting of a single empty field serializes as an
// empty line, and RFC 4180 makes that indistinguishable from a blank line.
// Go's encoding/csv (like many readers) deliberately skips blank lines on read,
// so a single-column row whose only cell renders to the empty string cannot be
// recovered — the blank line is dropped. This is a property of the CSV format
// itself, not of this exporter; formats that carry structure explicitly (such
// as JSON via JSONExporter) round-trip such rows without loss. Producing
// non-standard output to disambiguate the case would break interoperability
// with other CSV readers, so the exporter stays spec-compliant instead.
type CSVExporter struct{}

// Export writes the dataset to w as CSV.
func (CSVExporter) Export(w io.Writer, data interface{}) error {
	ds, err := coerce(data)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(ds.Columns); err != nil {
		return fmt.Errorf("export: write csv header: %w", err)
	}
	record := make([]string, len(ds.Columns))
	for _, row := range ds.Rows {
		for i := range ds.Columns {
			record[i] = cellString(cell(row, i))
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("export: write csv row: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("export: flush csv: %w", err)
	}
	return nil
}

// ContentType returns the CSV MIME type.
func (CSVExporter) ContentType() string { return "text/csv" }

// Extension returns the CSV file extension.
func (CSVExporter) Extension() string { return "csv" }

// JSONExporter writes a dataset as a JSON array of objects, one per row, keyed
// by column name. Cell values are emitted with their native JSON types.
type JSONExporter struct {
	// Indent, when true, pretty-prints the output with two-space indentation.
	Indent bool
}

// Export writes the dataset to w as JSON.
func (e JSONExporter) Export(w io.Writer, data interface{}) error {
	ds, err := coerce(data)
	if err != nil {
		return err
	}
	records := make([]map[string]interface{}, 0, len(ds.Rows))
	for _, row := range ds.Rows {
		obj := make(map[string]interface{}, len(ds.Columns))
		for i, name := range ds.Columns {
			obj[name] = cell(row, i)
		}
		records = append(records, obj)
	}
	enc := json.NewEncoder(w)
	if e.Indent {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(records); err != nil {
		return fmt.Errorf("export: encode json: %w", err)
	}
	return nil
}

// ContentType returns the JSON MIME type.
func (JSONExporter) ContentType() string { return "application/json" }

// Extension returns the JSON file extension.
func (JSONExporter) Extension() string { return "json" }

// XMLExporter writes a dataset as an XML document of the form:
//
//	<dataset>
//	  <row>
//	    <cell column="id">1</cell>
//	    <cell column="name">Ada</cell>
//	  </row>
//	</dataset>
//
// Using a column attribute keeps the document valid even when column names are
// not legal XML element names.
type XMLExporter struct {
	// Indent, when true, pretty-prints the output with two-space indentation.
	Indent bool
}

type xmlDataset struct {
	XMLName xml.Name `xml:"dataset"`
	Rows    []xmlRow `xml:"row"`
}

type xmlRow struct {
	Cells []xmlCell `xml:"cell"`
}

type xmlCell struct {
	Column string `xml:"column,attr"`
	Value  string `xml:",chardata"`
}

// Export writes the dataset to w as XML.
func (e XMLExporter) Export(w io.Writer, data interface{}) error {
	ds, err := coerce(data)
	if err != nil {
		return err
	}
	doc := xmlDataset{Rows: make([]xmlRow, 0, len(ds.Rows))}
	for _, row := range ds.Rows {
		cells := make([]xmlCell, len(ds.Columns))
		for i, name := range ds.Columns {
			cells[i] = xmlCell{Column: name, Value: cellString(cell(row, i))}
		}
		doc.Rows = append(doc.Rows, xmlRow{Cells: cells})
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("export: write xml header: %w", err)
	}
	enc := xml.NewEncoder(w)
	if e.Indent {
		enc.Indent("", "  ")
	}
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("export: encode xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("export: flush xml: %w", err)
	}
	return nil
}

// ContentType returns the XML MIME type.
func (XMLExporter) ContentType() string { return "application/xml" }

// Extension returns the XML file extension.
func (XMLExporter) Extension() string { return "xml" }

// XLSXExporter writes a dataset as a minimal but valid Office Open XML
// spreadsheet (.xlsx). An .xlsx file is a ZIP archive of XML parts; this
// exporter assembles the smallest set of parts a conforming reader requires:
//
//	[Content_Types].xml          – declares the content types of the parts
//	_rels/.rels                  – package relationships (points at the workbook)
//	xl/workbook.xml              – declares a single worksheet
//	xl/_rels/workbook.xml.rels   – relates the workbook to the worksheet
//	xl/worksheets/sheet1.xml     – the cell data (header + rows)
//
// Cells are written as inline strings (t="inlineStr"), which avoids the need
// for a shared-strings table while remaining fully spec-compliant. This keeps
// the writer pure-stdlib (archive/zip + encoding/xml) with no third-party
// dependency or module-cache requirement.
type XLSXExporter struct {
	// SheetName names the single worksheet. Defaults to "Sheet1" when empty.
	SheetName string
}

// Export writes the dataset to w as an .xlsx archive.
func (e XLSXExporter) Export(w io.Writer, data interface{}) error {
	ds, err := coerce(data)
	if err != nil {
		return err
	}
	sheetName := e.SheetName
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	zw := zip.NewWriter(w)

	parts := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", rootRelsXML},
		{"xl/workbook.xml", workbookXML(sheetName)},
		{"xl/_rels/workbook.xml.rels", workbookRelsXML},
		{"xl/worksheets/sheet1.xml", sheetXML(ds)},
	}
	for _, p := range parts {
		fw, err := zw.Create(p.name)
		if err != nil {
			return fmt.Errorf("export: create xlsx part %s: %w", p.name, err)
		}
		if _, err := io.WriteString(fw, p.content); err != nil {
			return fmt.Errorf("export: write xlsx part %s: %w", p.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("export: finalize xlsx: %w", err)
	}
	return nil
}

// ContentType returns the XLSX MIME type.
func (XLSXExporter) ContentType() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// Extension returns the XLSX file extension.
func (XLSXExporter) Extension() string { return "xlsx" }

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
	`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
	`</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
	`</Relationships>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
	`</Relationships>`

func workbookXML(sheetName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="` + xmlEscape(sheetName) + `" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`
}

// sheetXML renders the worksheet part: a header row of column names followed by
// one row per dataset row, all using inline strings.
func sheetXML(ds *Dataset) string {
	var b []byte
	b = append(b, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`...)
	b = append(b, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`...)

	rowNum := 1
	// Header row.
	b = appendSheetRow(b, rowNum, ds.Columns, func(i int) string { return ds.Columns[i] }, len(ds.Columns))
	// Data rows.
	for _, row := range ds.Rows {
		rowNum++
		b = appendSheetRow(b, rowNum, ds.Columns, func(i int) string { return cellString(cell(row, i)) }, len(ds.Columns))
	}

	b = append(b, `</sheetData></worksheet>`...)
	return string(b)
}

// appendSheetRow appends a single <row> with n inline-string cells. valueAt
// resolves the textual value for column index i.
func appendSheetRow(b []byte, rowNum int, _ []string, valueAt func(i int) string, n int) []byte {
	b = append(b, `<row r="`...)
	b = strconv.AppendInt(b, int64(rowNum), 10)
	b = append(b, `">`...)
	for i := 0; i < n; i++ {
		ref := cellRef(i, rowNum)
		b = append(b, `<c r="`...)
		b = append(b, ref...)
		b = append(b, `" t="inlineStr"><is><t xml:space="preserve">`...)
		b = append(b, xmlEscape(valueAt(i))...)
		b = append(b, `</t></is></c>`...)
	}
	b = append(b, `</row>`...)
	return b
}

// cellRef returns the A1-style reference for a zero-based column index and a
// one-based row number, e.g. (0, 1) -> "A1", (26, 2) -> "AA2".
func cellRef(col, row int) string {
	return columnName(col) + strconv.Itoa(row)
}

// columnName converts a zero-based column index into spreadsheet letters
// (0 -> "A", 25 -> "Z", 26 -> "AA").
func columnName(col int) string {
	name := ""
	col++ // shift to 1-based for bijective base-26
	for col > 0 {
		col--
		name = string(rune('A'+(col%26))) + name
		col /= 26
	}
	return name
}

// xmlEscape escapes a string for safe inclusion in XML character data and
// attribute values.
func xmlEscape(s string) string {
	var buf []byte
	if err := xml.EscapeText(escapeWriter{&buf}, []byte(s)); err != nil {
		// xml.EscapeText only fails if the writer fails; our writer never does.
		return s
	}
	return string(buf)
}

// escapeWriter adapts a *[]byte to io.Writer for xml.EscapeText.
type escapeWriter struct{ b *[]byte }

func (w escapeWriter) Write(p []byte) (int, error) {
	*w.b = append(*w.b, p...)
	return len(p), nil
}
