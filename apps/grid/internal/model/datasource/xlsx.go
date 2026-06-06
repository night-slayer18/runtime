package datasource

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// registerXLSX registers the XLSX format. An .xlsx file is a ZIP archive of XML
// parts (Office Open XML), so it can be read with archive/zip + encoding/xml
// from the standard library — no third-party dependency, and therefore always
// available offline. This reader handles the common cases produced by the
// export package and by spreadsheet applications: shared strings, inline
// strings, and raw numeric/boolean values.
func registerXLSX() {
	register(Format{
		Name:       "XLSX",
		Extensions: []string{"xlsx"},
		Available:  true,
		Read:       readXLSX,
	})
}

// xlsxSharedStrings models xl/sharedStrings.xml: a table of strings referenced
// by index from cells with t="s".
type xlsxSharedStrings struct {
	Items []xlsxSI `xml:"si"`
}

// xlsxSI is one shared-string item. Text may be a single <t> or split across
// multiple rich-text runs (<r><t>...); we concatenate all <t> content.
type xlsxSI struct {
	T    string   `xml:"t"`
	Runs []string `xml:"r>t"`
}

// value returns the full text of a shared-string item, joining rich-text runs.
func (s xlsxSI) value() string {
	if len(s.Runs) > 0 {
		return strings.Join(s.Runs, "")
	}
	return s.T
}

// xlsxWorksheet models a worksheet part's row/cell structure.
type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

// xlsxCell is a single cell. R is the A1 reference, T is the type
// ("s" shared string, "inlineStr" inline string, "str" formula string, "b"
// boolean, or empty for number). V holds the value (or shared-string index);
// inline strings live under <is><t>.
type xlsxCell struct {
	R      string `xml:"r,attr"`
	T      string `xml:"t,attr"`
	V      string `xml:"v"`
	Inline string `xml:"is>t"`
}

// readXLSX opens an .xlsx archive, resolves the first worksheet and the shared
// string table, and converts the sheet into a DataSource. The first populated
// row becomes the header.
func readXLSX(path string) (ds.DataSource, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx %s: %w", path, err)
	}
	defer zr.Close()

	var sheetFile *zip.File
	var sharedFile *zip.File
	sheetName := "" // pick the lexicographically-first worksheet for determinism
	for _, f := range zr.File {
		name := f.Name
		switch {
		case strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml"):
			if sheetName == "" || name < sheetName {
				sheetName = name
				sheetFile = f
			}
		case name == "xl/sharedStrings.xml":
			sharedFile = f
		}
	}
	if sheetFile == nil {
		return nil, fmt.Errorf("xlsx %s: no worksheet found", path)
	}

	shared, err := readSharedStrings(sharedFile)
	if err != nil {
		return nil, err
	}

	ws, err := readWorksheet(sheetFile)
	if err != nil {
		return nil, err
	}

	header, rows := worksheetToRows(ws, shared)
	return buildSource(header, rows), nil
}

// readSharedStrings parses the shared-strings part, returning the resolved
// string values indexed by position. A nil file yields an empty table.
func readSharedStrings(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open sharedStrings: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read sharedStrings: %w", err)
	}
	var sst xlsxSharedStrings
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil, fmt.Errorf("parse sharedStrings: %w", err)
	}
	out := make([]string, len(sst.Items))
	for i, si := range sst.Items {
		out[i] = si.value()
	}
	return out, nil
}

// readWorksheet parses a worksheet part into its row/cell structure.
func readWorksheet(f *zip.File) (xlsxWorksheet, error) {
	rc, err := f.Open()
	if err != nil {
		return xlsxWorksheet{}, fmt.Errorf("open worksheet: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return xlsxWorksheet{}, fmt.Errorf("read worksheet: %w", err)
	}
	var ws xlsxWorksheet
	if err := xml.Unmarshal(data, &ws); err != nil {
		return xlsxWorksheet{}, fmt.Errorf("parse worksheet: %w", err)
	}
	return ws, nil
}

// worksheetToRows flattens a worksheet into a header plus data rows. Cells are
// placed at the column index derived from their A1 reference so sparse rows are
// aligned correctly; the header is the first row in document order. The width of
// every row is normalised to the widest row so the schema is rectangular.
func worksheetToRows(ws xlsxWorksheet, shared []string) (header []string, rows [][]string) {
	if len(ws.Rows) == 0 {
		return nil, nil
	}

	type sparseRow struct {
		cells map[int]string
		width int
	}

	parsed := make([]sparseRow, 0, len(ws.Rows))
	maxWidth := 0
	for _, r := range ws.Rows {
		sr := sparseRow{cells: map[int]string{}}
		for _, c := range r.Cells {
			col := columnIndex(c.R)
			if col < 0 {
				// No reference: fall back to next sequential column.
				col = sr.width
			}
			sr.cells[col] = cellValue(c, shared)
			if col+1 > sr.width {
				sr.width = col + 1
			}
		}
		if sr.width > maxWidth {
			maxWidth = sr.width
		}
		parsed = append(parsed, sr)
	}

	dense := make([][]string, len(parsed))
	for i, sr := range parsed {
		row := make([]string, maxWidth)
		for col, val := range sr.cells {
			if col < maxWidth {
				row[col] = val
			}
		}
		dense[i] = row
	}

	header = dense[0]
	rows = dense[1:]
	return header, rows
}

// cellValue resolves a cell's textual value, following its type: shared-string
// index, inline string, or literal value.
func cellValue(c xlsxCell, shared []string) string {
	switch c.T {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.V))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	case "inlineStr":
		return c.Inline
	case "b":
		if strings.TrimSpace(c.V) == "1" {
			return "TRUE"
		}
		return "FALSE"
	default:
		return c.V
	}
}

// columnIndex converts an A1-style cell reference into a zero-based column
// index ("A1" -> 0, "B2" -> 1, "AA3" -> 26). It returns -1 when ref has no
// leading letters.
func columnIndex(ref string) int {
	letters := ref
	for i, r := range ref {
		if r >= '0' && r <= '9' {
			letters = ref[:i]
			break
		}
	}
	letters = strings.ToUpper(letters)
	if letters == "" {
		return -1
	}
	col := 0
	for _, r := range letters {
		if r < 'A' || r > 'Z' {
			return -1
		}
		col = col*26 + int(r-'A'+1)
	}
	return col - 1
}
