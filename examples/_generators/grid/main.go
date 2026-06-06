// Command grid-gen produces the binary Grid sample files (Parquet, Arrow, and
// XLSX) from the same "people" dataset used by examples/grid/people.csv.
//
// It deliberately reuses the SAME libraries the Grid app reads with:
//   - Parquet:  github.com/parquet-go/parquet-go
//   - Arrow:    github.com/apache/arrow-go/v18
//   - XLSX:     github.com/runtime-sh/runtime/packages/export (the app's writer)
//
// Run from the repo root:
//
//	go run ./examples/_generators/grid
//
// Output is written to examples/grid/{people.parquet,people.arrow,people.xlsx}.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/parquet-go/parquet-go"
	"github.com/runtime-sh/runtime/packages/export"
)

// Person mirrors the columns of examples/grid/people.csv. The parquet struct
// tags name the columns so the generated Parquet schema matches the CSV header.
type Person struct {
	ID         int64  `parquet:"id"`
	Name       string `parquet:"name"`
	Email      string `parquet:"email"`
	Department string `parquet:"department"`
	Salary     int64  `parquet:"salary"`
	StartDate  string `parquet:"start_date"`
	Active     bool   `parquet:"active"`
}

func people() []Person {
	return []Person{
		{1, "Ada Lovelace", "ada.lovelace@example.com", "Engineering", 142000, "2019-03-11", true},
		{2, "Alan Turing", "alan.turing@example.com", "Research", 155000, "2017-06-23", true},
		{3, "Grace Hopper", "grace.hopper@example.com", "Engineering", 138500, "2018-01-15", true},
		{4, "Katherine Johnson", "katherine.johnson@example.com", "Research", 131000, "2020-09-01", true},
		{5, "Margaret Hamilton", "margaret.hamilton@example.com", "Engineering", 147250, "2016-11-30", true},
		{6, "Dennis Ritchie", "dennis.ritchie@example.com", "Platform", 151000, "2015-04-19", false},
		{7, "Barbara Liskov", "barbara.liskov@example.com", "Research", 149900, "2019-08-05", true},
		{8, "Linus Torvalds", "linus.torvalds@example.com", "Platform", 160000, "2014-02-28", true},
		{9, "Radia Perlman", "radia.perlman@example.com", "Networking", 144750, "2018-07-12", true},
		{10, "Tim Berners-Lee", "tim.bernerslee@example.com", "Platform", 158300, "2013-10-01", false},
		{11, "Donald Knuth", "donald.knuth@example.com", "Research", 162500, "2012-05-17", true},
		{12, "Edsger Dijkstra", "edsger.dijkstra@example.com", "Research", 150100, "2016-03-22", false},
	}
}

func main() {
	outDir := "examples/grid"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}

	rows := people()

	if err := writeParquet(filepath.Join(outDir, "people.parquet"), rows); err != nil {
		fail(fmt.Errorf("parquet: %w", err))
	}
	if err := writeArrow(filepath.Join(outDir, "people.arrow"), rows); err != nil {
		fail(fmt.Errorf("arrow: %w", err))
	}
	if err := writeXLSX(filepath.Join(outDir, "people.xlsx"), rows); err != nil {
		fail(fmt.Errorf("xlsx: %w", err))
	}

	fmt.Printf("wrote %d people rows to %s/{people.parquet,people.arrow,people.xlsx}\n", len(rows), outDir)
}

// writeParquet writes the rows as a Parquet file via parquet-go's generic
// writer, the same library Grid reads Parquet with.
func writeParquet(path string, rows []Person) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := parquet.NewGenericWriter[Person](f)
	if _, err := w.Write(rows); err != nil {
		return err
	}
	return w.Close()
}

// writeArrow writes the rows as an Arrow IPC file (Feather v2 / .arrow) via
// apache/arrow-go, the same library Grid reads Arrow with.
func writeArrow(path string, rows []Person) error {
	pool := memory.DefaultAllocator
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "email", Type: arrow.BinaryTypes.String},
		{Name: "department", Type: arrow.BinaryTypes.String},
		{Name: "salary", Type: arrow.PrimitiveTypes.Int64},
		{Name: "start_date", Type: arrow.BinaryTypes.String},
		{Name: "active", Type: arrow.FixedWidthTypes.Boolean},
	}, nil)

	b := array.NewRecordBuilder(pool, schema)
	defer b.Release()

	for _, p := range rows {
		b.Field(0).(*array.Int64Builder).Append(p.ID)
		b.Field(1).(*array.StringBuilder).Append(p.Name)
		b.Field(2).(*array.StringBuilder).Append(p.Email)
		b.Field(3).(*array.StringBuilder).Append(p.Department)
		b.Field(4).(*array.Int64Builder).Append(p.Salary)
		b.Field(5).(*array.StringBuilder).Append(p.StartDate)
		b.Field(6).(*array.BooleanBuilder).Append(p.Active)
	}

	rec := b.NewRecord()
	defer rec.Release()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := ipc.NewFileWriter(f, ipc.WithSchema(schema), ipc.WithAllocator(pool))
	if err != nil {
		return err
	}
	if err := w.Write(rec); err != nil {
		return err
	}
	return w.Close()
}

// writeXLSX writes the rows as an .xlsx workbook using the app's own export
// package writer, so the generated file matches what Grid's XLSX reader expects.
func writeXLSX(path string, rows []Person) error {
	ds := &export.Dataset{
		Columns: []string{"id", "name", "email", "department", "salary", "start_date", "active"},
	}
	for _, p := range rows {
		ds.Rows = append(ds.Rows, []interface{}{
			p.ID, p.Name, p.Email, p.Department, p.Salary, p.StartDate, p.Active,
		})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return export.XLSXExporter{SheetName: "people"}.Export(f, ds)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "grid-gen:", err)
	os.Exit(1)
}
