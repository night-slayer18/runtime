package table

import "testing"

// collectColumn renders the active view in order and returns the value of the
// given column for every row, walking the cursor from top to bottom. It lets
// these tests assert on the full ordered contents of the view rather than just
// the selected row.
func collectColumn(tbl *Table, col int) []string {
	n := tbl.RowCount()
	out := make([]string, 0, n)
	tbl.Navigate("g") // jump to top
	for i := 0; i < n; i++ {
		r, ok := tbl.SelectedRow()
		if !ok {
			break
		}
		out = append(out, cellAt(r, col))
		tbl.Navigate("down")
	}
	return out
}

// TestFilterAllRowsMatch covers the edge case where a non-empty query matches
// every row in the dataset (distinct from the empty-query case, which clears
// the filter entirely).
func TestFilterAllRowsMatch(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "alpha", "shared"),
		NewRow("2", "beta", "shared"),
		NewRow("3", "gamma", "shared"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Filter("shared") // present in the Note cell of every row
	if tbl.RowCount() != 3 {
		t.Fatalf("expected all 3 rows to match 'shared', got %d", tbl.RowCount())
	}
	got := collectColumn(tbl, 1)
	want := []string{"alpha", "beta", "gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected row %d name %q, got %q", i, want[i], got[i])
		}
	}
}

// TestFilterPartialMatchSubset covers a query that matches some but not all
// rows, verifying only the matching subset survives.
func TestFilterPartialMatchSubset(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "apple", "fruit"),
		NewRow("2", "banana", "fruit"),
		NewRow("3", "carrot", "veg"),
		NewRow("4", "apricot", "fruit"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Filter("ap") // matches apple, apricot
	if tbl.RowCount() != 2 {
		t.Fatalf("expected 2 rows matching 'ap', got %d", tbl.RowCount())
	}
	got := collectColumn(tbl, 1)
	want := []string{"apple", "apricot"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected row %d name %q, got %q", i, want[i], got[i])
		}
	}
}

// TestSortIsStable verifies that SortBy preserves the original relative order of
// rows that compare equal on the sort column (a stable sort).
func TestSortIsStable(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "first", "x"),
		NewRow("1", "second", "x"),
		NewRow("1", "third", "x"),
		NewRow("1", "fourth", "x"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.SortBy(0) // all IDs equal; stable sort must preserve input order
	got := collectColumn(tbl, 1)
	want := []string{"first", "second", "third", "fourth"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stable sort: expected row %d name %q, got %q", i, want[i], got[i])
		}
	}
}

// TestFilterThenSort verifies that filtering then sorting composes: the sort
// orders only the rows surviving the filter.
func TestFilterThenSort(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("3", "apple", "fruit"),
		NewRow("1", "apricot", "fruit"),
		NewRow("9", "carrot", "veg"),
		NewRow("2", "avocado", "fruit"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Filter("ap") // apple, apricot
	tbl.SortBy(0)    // ascending by ID over the filtered subset
	if tbl.RowCount() != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", tbl.RowCount())
	}
	got := collectColumn(tbl, 0)
	want := []string{"1", "3"} // apricot=1, apple=3 sorted ascending
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filter+sort: expected row %d id %q, got %q", i, want[i], got[i])
		}
	}
}

// TestSortThenFilter verifies the reverse composition: sorting then filtering
// keeps the sort order applied to the surviving rows.
func TestSortThenFilter(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("3", "crab", "x"),
		NewRow("1", "cranberry", "y"),
		NewRow("2", "date", "z"),
		NewRow("4", "fig", "w"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.SortBy(0)    // ascending by ID: 1,2,3,4
	tbl.Filter("cr") // cranberry(1), crab(3) -> both contain 'cr'
	if tbl.RowCount() != 2 {
		t.Fatalf("expected 2 rows after sort+filter, got %d", tbl.RowCount())
	}
	got := collectColumn(tbl, 0)
	want := []string{"1", "3"} // sort order preserved within the filtered subset
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort+filter: expected row %d id %q, got %q", i, want[i], got[i])
		}
	}
}

// TestFilterClearAfterPartialRestoresAll verifies that clearing a filter that
// previously excluded rows restores the full dataset (complements the existing
// empty-query test where the prior filter matched everything).
func TestFilterClearAfterPartialRestoresAll(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "apple", "fruit"),
		NewRow("2", "banana", "fruit"),
		NewRow("3", "carrot", "veg"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Filter("apple")
	if tbl.RowCount() != 1 {
		t.Fatalf("expected 1 row while filtered, got %d", tbl.RowCount())
	}
	tbl.Filter("") // clear
	if tbl.RowCount() != 3 {
		t.Fatalf("expected all 3 rows after clearing filter, got %d", tbl.RowCount())
	}
}

// TestSortByDescendingThenToggleBackToAscending verifies the direction toggles
// correctly across repeated SortBy calls on the same column, including stable
// ordering on the way back.
func TestSortByToggleDirection(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("2", "b", "x"),
		NewRow("3", "c", "y"),
		NewRow("1", "a", "z"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.SortBy(0) // asc
	if got := collectColumn(tbl, 0); got[0] != "1" || got[2] != "3" {
		t.Fatalf("expected ascending 1..3, got %v", got)
	}
	tbl.SortBy(0) // desc
	if got := collectColumn(tbl, 0); got[0] != "3" || got[2] != "1" {
		t.Fatalf("expected descending 3..1, got %v", got)
	}
	tbl.SortBy(0) // back to asc
	if got := collectColumn(tbl, 0); got[0] != "1" || got[2] != "3" {
		t.Fatalf("expected ascending again 1..3, got %v", got)
	}
}
