package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/runtime-sh/runtime/packages/datasource"
)

func sampleDataset() *Dataset {
	return &Dataset{
		Columns: []string{"id", "name", "active"},
		Rows: [][]interface{}{
			{1, "Ada", true},
			{2, "Linus, B.", false},
			{3, "O'Brien <x>", true},
		},
	}
}

func TestCSVExporterRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := (CSVExporter{}).Export(&buf, sampleDataset()); err != nil {
		t.Fatalf("export csv: %v", err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("expected 4 records (header + 3 rows), got %d", len(records))
	}
	if records[0][0] != "id" || records[0][2] != "active" {
		t.Fatalf("unexpected header: %v", records[0])
	}
	if records[2][1] != "Linus, B." {
		t.Fatalf("comma value not preserved: %q", records[2][1])
	}
	if records[3][1] != "O'Brien <x>" {
		t.Fatalf("special chars not preserved: %q", records[3][1])
	}
}

func TestJSONExporterStructure(t *testing.T) {
	var buf bytes.Buffer
	if err := (JSONExporter{}).Export(&buf, sampleDataset()); err != nil {
		t.Fatalf("export json: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(got))
	}
	if got[0]["name"] != "Ada" {
		t.Fatalf("expected name Ada, got %v", got[0]["name"])
	}
	if got[1]["active"] != false {
		t.Fatalf("expected active false, got %v (%T)", got[1]["active"], got[1]["active"])
	}
}

func TestXMLExporterContainsRows(t *testing.T) {
	var buf bytes.Buffer
	if err := (XMLExporter{Indent: true}).Export(&buf, sampleDataset()); err != nil {
		t.Fatalf("export xml: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<dataset>") || !strings.Contains(out, "</dataset>") {
		t.Fatalf("missing dataset element: %s", out)
	}
	if !strings.Contains(out, `column="name"`) {
		t.Fatalf("missing column attribute: %s", out)
	}
	// Special characters must be escaped.
	if !strings.Contains(out, "O&#39;Brien &lt;x&gt;") && !strings.Contains(out, "O'Brien &lt;x&gt;") {
		t.Fatalf("special chars not escaped: %s", out)
	}
}

func TestXLSXExporterProducesValidZip(t *testing.T) {
	var buf bytes.Buffer
	if err := (XLSXExporter{}).Export(&buf, sampleDataset()); err != nil {
		t.Fatalf("export xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open xlsx zip: %v", err)
	}
	required := map[string]bool{
		"[Content_Types].xml":        false,
		"_rels/.rels":                false,
		"xl/workbook.xml":            false,
		"xl/_rels/workbook.xml.rels": false,
		"xl/worksheets/sheet1.xml":   false,
	}
	var sheet string
	for _, f := range zr.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open sheet: %v", err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			sheet = string(data)
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("missing required xlsx part: %s", name)
		}
	}
	if !strings.Contains(sheet, "Ada") || !strings.Contains(sheet, "<row") {
		t.Fatalf("sheet missing data: %s", sheet)
	}
}

func TestMetadataMethods(t *testing.T) {
	cases := []struct {
		e   Exporter
		ct  string
		ext string
	}{
		{CSVExporter{}, "text/csv", "csv"},
		{JSONExporter{}, "application/json", "json"},
		{XMLExporter{}, "application/xml", "xml"},
		{XLSXExporter{}, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"},
	}
	for _, c := range cases {
		if got := c.e.ContentType(); got != c.ct {
			t.Errorf("%T ContentType = %q, want %q", c.e, got, c.ct)
		}
		if got := c.e.Extension(); got != c.ext {
			t.Errorf("%T Extension = %q, want %q", c.e, got, c.ext)
		}
	}
}

func TestExportUnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	if err := (CSVExporter{}).Export(&buf, "not a dataset"); err == nil {
		t.Fatal("expected error for unsupported data type, got nil")
	}
}

func TestFromIteratorComposesWithDataSource(t *testing.T) {
	cols := []datasource.Column{
		{Name: "id", Type: "int"},
		{Name: "name", Type: "text"},
	}
	rows := [][]interface{}{
		{1, "Ada"},
		{2, "Grace"},
	}
	src := datasource.NewMemorySource(cols, rows)
	defer src.Close()

	it, err := src.Query("select *")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer it.Close()

	ds, err := FromIterator(it, cols)
	if err != nil {
		t.Fatalf("from iterator: %v", err)
	}
	if len(ds.Columns) != 2 || ds.Columns[1] != "name" {
		t.Fatalf("unexpected columns: %v", ds.Columns)
	}
	if len(ds.Rows) != 2 || ds.Rows[1][1] != "Grace" {
		t.Fatalf("unexpected rows: %v", ds.Rows)
	}
}

func TestExportIteratorEndToEnd(t *testing.T) {
	cols := []datasource.Column{{Name: "id", Type: "int"}, {Name: "name", Type: "text"}}
	src := datasource.NewMemorySource(cols, [][]interface{}{{1, "Ada"}})
	defer src.Close()

	it, err := src.Query("")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer it.Close()

	var buf bytes.Buffer
	if err := ExportIterator(CSVExporter{}, &buf, it, cols); err != nil {
		t.Fatalf("export iterator: %v", err)
	}
	if !strings.Contains(buf.String(), "Ada") {
		t.Fatalf("expected exported data to contain Ada: %q", buf.String())
	}
}
