package table

import (
	"strings"
	"testing"

	"github.com/runtime-sh/runtime/packages/theme"
)

func sampleColumns() []Column {
	return []Column{
		{Title: "ID", Width: 5, Sortable: true},
		{Title: "Name", Width: 10, Sortable: true},
		{Title: "Note", Width: 12, Sortable: false},
	}
}

func makeRows(n int) []Row {
	rows := make([]Row, n)
	for i := 0; i < n; i++ {
		rows[i] = NewRow(
			pad(i),
			"name",
			"note",
		)
	}
	return rows
}

func pad(i int) string {
	// stable, sortable zero-padded id
	s := ""
	for _, d := range itoa(i) {
		s += string(d)
	}
	return s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func newTestTable() *Table {
	t := New(theme.DefaultStyles)
	t.SetSize(80, 11) // header + 10 data rows
	return t
}

func TestSetDataResetsState(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(100), sampleColumns())
	if tbl.RowCount() != 100 {
		t.Fatalf("expected 100 rows, got %d", tbl.RowCount())
	}
	if tbl.Cursor() != 0 {
		t.Fatalf("expected cursor at 0, got %d", tbl.Cursor())
	}
}

func TestNavigateDownAndUp(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(100), sampleColumns())

	tbl.Navigate("down")
	tbl.Navigate("down")
	if tbl.Cursor() != 2 {
		t.Fatalf("expected cursor at 2, got %d", tbl.Cursor())
	}
	tbl.Navigate("up")
	if tbl.Cursor() != 1 {
		t.Fatalf("expected cursor at 1, got %d", tbl.Cursor())
	}
}

func TestNavigateClampsAtBounds(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(5), sampleColumns())

	tbl.Navigate("up") // already at top
	if tbl.Cursor() != 0 {
		t.Fatalf("cursor should clamp at 0, got %d", tbl.Cursor())
	}
	for i := 0; i < 20; i++ {
		tbl.Navigate("down")
	}
	if tbl.Cursor() != 4 {
		t.Fatalf("cursor should clamp at last row 4, got %d", tbl.Cursor())
	}
}

func TestNavigateJumpTopBottom(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(1000), sampleColumns())

	tbl.Navigate("G")
	if tbl.Cursor() != 999 {
		t.Fatalf("expected cursor at 999, got %d", tbl.Cursor())
	}
	tbl.Navigate("g")
	if tbl.Cursor() != 0 {
		t.Fatalf("expected cursor at 0, got %d", tbl.Cursor())
	}
}

func TestNavigatePaging(t *testing.T) {
	tbl := newTestTable() // 10 visible data rows
	tbl.SetData(makeRows(100), sampleColumns())

	tbl.Navigate("pgdn")
	if tbl.Cursor() != 10 {
		t.Fatalf("expected cursor at 10 after page down, got %d", tbl.Cursor())
	}
	tbl.Navigate("pgup")
	if tbl.Cursor() != 0 {
		t.Fatalf("expected cursor at 0 after page up, got %d", tbl.Cursor())
	}
}

func TestFilterSelectsMatchingRows(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "alice", "x"),
		NewRow("2", "bob", "y"),
		NewRow("3", "carol", "z"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Filter("bob")
	if tbl.RowCount() != 1 {
		t.Fatalf("expected 1 match, got %d", tbl.RowCount())
	}
	r, ok := tbl.SelectedRow()
	if !ok || r.Cells[1] != "bob" {
		t.Fatalf("expected selected row bob, got %+v ok=%v", r, ok)
	}
}

func TestFilterCaseInsensitive(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{NewRow("1", "Alice", "x")}
	tbl.SetData(rows, sampleColumns())
	tbl.Filter("alice")
	if tbl.RowCount() != 1 {
		t.Fatalf("expected case-insensitive match, got %d", tbl.RowCount())
	}
}

func TestFilterEmptyQueryRestoresAll(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(50), sampleColumns())
	tbl.Filter("name")
	if tbl.RowCount() != 50 {
		t.Fatalf("expected all 50 rows to match 'name', got %d", tbl.RowCount())
	}
	tbl.Filter("")
	if tbl.RowCount() != 50 {
		t.Fatalf("expected 50 rows after clearing filter, got %d", tbl.RowCount())
	}
}

func TestFilterNoMatches(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(10), sampleColumns())
	tbl.Filter("zzz-nonexistent")
	if tbl.RowCount() != 0 {
		t.Fatalf("expected 0 matches, got %d", tbl.RowCount())
	}
	if tbl.Cursor() != -1 {
		t.Fatalf("expected cursor -1 on empty view, got %d", tbl.Cursor())
	}
	if _, ok := tbl.SelectedRow(); ok {
		t.Fatalf("expected no selected row on empty view")
	}
}

func TestSortByAscendingDescending(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("3", "c", "z"),
		NewRow("1", "a", "x"),
		NewRow("2", "b", "y"),
	}
	tbl.SetData(rows, sampleColumns())

	tbl.SortBy(0) // ascending by ID
	if r, _ := tbl.SelectedRow(); r.Cells[0] != "1" {
		t.Fatalf("expected first row id 1 after asc sort, got %s", r.Cells[0])
	}
	tbl.SortBy(0) // toggle to descending
	if r, _ := tbl.SelectedRow(); r.Cells[0] != "3" {
		t.Fatalf("expected first row id 3 after desc sort, got %s", r.Cells[0])
	}
}

func TestSortByIgnoresNonSortableColumn(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("2", "b", "z"),
		NewRow("1", "a", "x"),
	}
	tbl.SetData(rows, sampleColumns())
	tbl.SortBy(2) // Note column is not sortable
	if r, _ := tbl.SelectedRow(); r.Cells[0] != "2" {
		t.Fatalf("expected order unchanged for non-sortable column, got %s", r.Cells[0])
	}
}

func TestSortByIgnoresOutOfRange(t *testing.T) {
	tbl := newTestTable()
	tbl.SetData(makeRows(3), sampleColumns())
	tbl.SortBy(99) // should be a no-op, not panic
	tbl.SortBy(-1)
	if tbl.RowCount() != 3 {
		t.Fatalf("expected 3 rows, got %d", tbl.RowCount())
	}
}

// TestViewVirtualization is the core guarantee: View only materializes the
// visible window of rows regardless of dataset size.
func TestViewVirtualizationOnlyRendersVisibleWindow(t *testing.T) {
	tbl := newTestTable() // 10 visible data rows
	rows := make([]Row, 1000)
	for i := 0; i < 1000; i++ {
		rows[i] = NewRow(itoa(i), "marker"+itoa(i), "note")
	}
	tbl.SetData(rows, sampleColumns())

	out := tbl.View()

	// A row far outside the visible window must never appear in the output.
	if strings.Contains(out, "marker999") {
		t.Fatalf("offscreen row marker999 should not be rendered")
	}
	// The output line count is bounded by the viewport, not the dataset.
	lines := strings.Count(out, "\n") + 1
	if lines > 11 { // header + 10 data rows
		t.Fatalf("expected at most 11 lines, got %d", lines)
	}
}

func TestViewScrollFollowsCursor(t *testing.T) {
	tbl := newTestTable()
	rows := make([]Row, 100)
	for i := 0; i < 100; i++ {
		rows[i] = NewRow(itoa(i), "row"+itoa(i), "note")
	}
	tbl.SetData(rows, sampleColumns())

	tbl.Navigate("G") // jump to bottom
	sb := func() Scrollbar { tbl.View(); return tbl.Scrollbar() }()
	if sb.Offset == 0 {
		t.Fatalf("expected viewport to scroll when cursor jumps to bottom")
	}
	if tbl.Cursor() < sb.Offset || tbl.Cursor() >= sb.Offset+sb.Visible {
		t.Fatalf("cursor %d should be within visible window [%d,%d)", tbl.Cursor(), sb.Offset, sb.Offset+sb.Visible)
	}
}

func TestFitPadsAndTruncates(t *testing.T) {
	if got := fit("ab", 5); got != "ab   " {
		t.Fatalf("expected padded 'ab   ', got %q", got)
	}
	if got := fit("abcdef", 4); got != "abc…" {
		t.Fatalf("expected truncated 'abc…', got %q", got)
	}
	if got := fit("abc", 3); got != "abc" {
		t.Fatalf("expected exact 'abc', got %q", got)
	}
	if got := fit("anything", 0); got != "" {
		t.Fatalf("expected empty for width 0, got %q", got)
	}
}

func TestScrollbarRenderHeight(t *testing.T) {
	sb := Scrollbar{Total: 100, Visible: 10, Offset: 0}
	lines := sb.Render(10, theme.DefaultStyles)
	if len(lines) != 10 {
		t.Fatalf("expected 10 scrollbar lines, got %d", len(lines))
	}
}

// countingProvider is a RowProvider that records which indices were fetched and
// generates rows lazily. It lets tests assert that the table pulls only the
// rows it needs rather than materializing the whole dataset.
type countingProvider struct {
	n       int
	fetched map[int]int // index -> number of RowAt calls
}

func newCountingProvider(n int) *countingProvider {
	return &countingProvider{n: n, fetched: make(map[int]int)}
}

func (p *countingProvider) Len() int { return p.n }

func (p *countingProvider) RowAt(i int) (Row, bool) {
	if i < 0 || i >= p.n {
		return Row{}, false
	}
	p.fetched[i]++
	return NewRow(itoa(i), "row"+itoa(i), "note"), true
}

func (p *countingProvider) distinctFetched() int { return len(p.fetched) }

func TestSetProviderReportsProviderLength(t *testing.T) {
	tbl := newTestTable()
	tbl.SetProvider(newCountingProvider(1000), sampleColumns())
	if tbl.RowCount() != 1000 {
		t.Fatalf("expected 1000 rows from provider, got %d", tbl.RowCount())
	}
	if tbl.Cursor() != 0 {
		t.Fatalf("expected cursor at 0, got %d", tbl.Cursor())
	}
}

func TestSetProviderNilUsesEmptyProvider(t *testing.T) {
	tbl := newTestTable()
	tbl.SetProvider(nil, sampleColumns())
	if tbl.RowCount() != 0 {
		t.Fatalf("expected 0 rows for nil provider, got %d", tbl.RowCount())
	}
	if _, ok := tbl.SelectedRow(); ok {
		t.Fatalf("expected no selected row for empty provider")
	}
}

// TestProviderRendersOnlyVisibleWindow is the streaming guarantee: with no
// filter or sort active, rendering a huge provider-backed table must only pull
// the rows inside the viewport, never the full dataset.
func TestProviderRendersOnlyVisibleWindow(t *testing.T) {
	tbl := newTestTable() // 10 visible data rows
	p := newCountingProvider(1_000_000)
	tbl.SetProvider(p, sampleColumns())

	_ = tbl.View()

	if got := p.distinctFetched(); got > 11 {
		t.Fatalf("expected at most ~11 rows fetched for a viewport render, got %d", got)
	}
	// A far-offscreen row must never have been requested.
	if _, ok := p.fetched[999_999]; ok {
		t.Fatalf("offscreen row 999999 should not have been fetched")
	}
}

// TestProviderNavigationIsBounded verifies that navigating through a
// provider-backed table touches only a bounded number of rows per render,
// independent of dataset size.
func TestProviderNavigationIsBounded(t *testing.T) {
	tbl := newTestTable()
	p := newCountingProvider(1_000_000)
	tbl.SetProvider(p, sampleColumns())

	tbl.Navigate("pgdn")
	tbl.Navigate("pgdn")
	_ = tbl.View()

	// Even after navigation, only windows worth of rows should be touched.
	if got := p.distinctFetched(); got > 40 {
		t.Fatalf("expected a bounded number of fetched rows, got %d", got)
	}
}

func TestProviderSelectedRowPullsOnDemand(t *testing.T) {
	tbl := newTestTable()
	p := newCountingProvider(1000)
	tbl.SetProvider(p, sampleColumns())

	tbl.Navigate("down")
	tbl.Navigate("down")
	r, ok := tbl.SelectedRow()
	if !ok {
		t.Fatalf("expected a selected row")
	}
	if r.Cells[0] != "2" {
		t.Fatalf("expected selected row id 2, got %s", r.Cells[0])
	}
}

func TestProviderFilterPullsRowsAndSelects(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("1", "alice", "x"),
		NewRow("2", "bob", "y"),
		NewRow("3", "carol", "z"),
	}
	tbl.SetProvider(sliceProvider{rows: rows}, sampleColumns())

	tbl.Filter("bob")
	if tbl.RowCount() != 1 {
		t.Fatalf("expected 1 match via provider filter, got %d", tbl.RowCount())
	}
	r, ok := tbl.SelectedRow()
	if !ok || r.Cells[1] != "bob" {
		t.Fatalf("expected selected row bob, got %+v ok=%v", r, ok)
	}
}

func TestProviderSortOrdersView(t *testing.T) {
	tbl := newTestTable()
	rows := []Row{
		NewRow("3", "c", "z"),
		NewRow("1", "a", "x"),
		NewRow("2", "b", "y"),
	}
	tbl.SetProvider(sliceProvider{rows: rows}, sampleColumns())

	tbl.SortBy(0)
	if r, _ := tbl.SelectedRow(); r.Cells[0] != "1" {
		t.Fatalf("expected first row id 1 after asc sort, got %s", r.Cells[0])
	}
	tbl.SortBy(0)
	if r, _ := tbl.SelectedRow(); r.Cells[0] != "3" {
		t.Fatalf("expected first row id 3 after desc sort, got %s", r.Cells[0])
	}
}

func TestSetDataUsesProviderPath(t *testing.T) {
	// SetData must keep working for the in-memory case and route through the
	// same provider-backed rendering/navigation logic.
	tbl := newTestTable()
	tbl.SetData(makeRows(100), sampleColumns())
	if tbl.RowCount() != 100 {
		t.Fatalf("expected 100 rows via SetData, got %d", tbl.RowCount())
	}
	tbl.Navigate("G")
	if tbl.Cursor() != 99 {
		t.Fatalf("expected cursor 99 after jump to bottom, got %d", tbl.Cursor())
	}
}
