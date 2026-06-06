// Package table provides a high-performance, virtualized table component shared
// by all Runtime applications.
//
// The component is designed for very large datasets: only the rows and columns
// that fall inside the current viewport are ever materialized into the rendered
// view string, so the cost of a render (and of a navigation keystroke) is
// bounded by the size of the visible window rather than the size of the
// dataset. This is what lets the table stay responsive with 1,000,000+ rows.
//
// Styling is delegated entirely to theme.Styles so the table looks identical
// across every Runtime application.
package table

import (
	"sort"
	"strings"

	"github.com/runtime-sh/runtime/packages/theme"
)

// defaultHeight is the viewport height (including the header line) used when a
// caller never calls SetSize. It keeps the table usable in tests and headless
// contexts without forcing every consumer to configure dimensions.
const defaultHeight = 20

// defaultWidth is the viewport width used when a caller never calls SetSize.
const defaultWidth = 80

// Column describes a single table column.
type Column struct {
	// Title is the header label rendered for the column.
	Title string
	// Width is the rendered cell width in terminal cells. Values are padded or
	// truncated to this width.
	Width int
	// Sortable reports whether SortBy may order rows by this column.
	Sortable bool
}

// Row is a single table record. Cells are positional and align with the
// table's columns by index.
type Row struct {
	// Cells holds the string value for each column, indexed positionally.
	Cells []string
}

// NewRow is a convenience constructor for a Row from individual cell values.
func NewRow(cells ...string) Row {
	return Row{Cells: cells}
}

// RowProvider supplies rows to the table on demand. It is the abstraction that
// lets the table back its display with a streaming or lazily-loaded data set
// instead of an in-memory slice: the table only ever calls RowAt for the rows
// inside the current viewport, so a provider may fetch, decode, or page rows in
// from disk, a network source, or a database without the table materializing
// the full dataset.
//
// Implementations must be safe to call repeatedly with the same index and
// should be cheap for indices that have already been fetched. RowAt reports
// false for indices outside [0, Len()).
type RowProvider interface {
	// Len reports the total number of rows the provider can supply.
	Len() int
	// RowAt returns the row at index i and true, or a zero Row and false when
	// i is out of range.
	RowAt(i int) (Row, bool)
}

// sliceProvider is the in-memory RowProvider used by SetData. It adapts a plain
// []Row to the RowProvider interface so the in-memory and streaming cases share
// a single rendering and navigation path.
type sliceProvider struct {
	rows []Row
}

// Len implements RowProvider.
func (s sliceProvider) Len() int { return len(s.rows) }

// RowAt implements RowProvider.
func (s sliceProvider) RowAt(i int) (Row, bool) {
	if i < 0 || i >= len(s.rows) {
		return Row{}, false
	}
	return s.rows[i], true
}

// Scrollbar describes the vertical scroll state of the table. It is recomputed
// on every render from the current viewport and the size of the visible row
// set, and exposes enough information to draw a proportional scroll indicator.
type Scrollbar struct {
	// Total is the number of rows in the current (filtered) view.
	Total int
	// Visible is the number of data rows that fit in the viewport.
	Visible int
	// Offset is the index of the first visible row within the view.
	Offset int
}

// Render draws the scrollbar as a vertical column of runes height lines tall.
// The "thumb" is sized proportionally to the fraction of rows visible and
// positioned according to Offset. styles supplies the glyph styling.
func (s Scrollbar) Render(height int, styles theme.Styles) []string {
	lines := make([]string, height)
	if height <= 0 {
		return lines
	}
	// When everything fits there is nothing to scroll: draw an empty track.
	if s.Total <= s.Visible || s.Total == 0 {
		for i := range lines {
			lines[i] = styles.Muted.Render("│")
		}
		return lines
	}

	// Size the thumb proportionally to the visible fraction (at least 1 line).
	thumb := height * s.Visible / s.Total
	if thumb < 1 {
		thumb = 1
	}
	if thumb > height {
		thumb = height
	}

	// Position the thumb proportionally to the scroll offset.
	maxOffset := s.Total - s.Visible
	pos := 0
	if maxOffset > 0 {
		pos = (height - thumb) * s.Offset / maxOffset
	}
	if pos > height-thumb {
		pos = height - thumb
	}

	for i := 0; i < height; i++ {
		if i >= pos && i < pos+thumb {
			lines[i] = styles.Selected.Render(" ")
		} else {
			lines[i] = styles.Muted.Render("│")
		}
	}
	return lines
}

// Table is a virtualized table widget. The zero value is not ready for use;
// construct one with New so it has a sane viewport size and styles.
type Table struct {
	columns []Column

	// provider supplies rows on demand. SetData wraps an in-memory slice in a
	// sliceProvider; SetProvider installs a streaming/lazy provider directly.
	// The table only ever asks the provider for rows inside the current
	// viewport, so the full dataset is never materialized for rendering or
	// navigation.
	provider RowProvider

	// view holds row indices into the provider in current display order after
	// the active sort and filter have been applied. It is only populated when a
	// filter or sort is active; when neither is active it is nil and view
	// indices map directly onto provider indices (the identity view). This is
	// what keeps the unfiltered/unsorted streaming path from scanning or
	// materializing every row.
	view []int

	// query is the active case-insensitive filter substring; empty means no
	// filter.
	query string
	// sortCol is the column index the view is sorted by, or -1 for none.
	sortCol int
	// sortAsc reports the active sort direction.
	sortAsc bool

	// cursor is the selected position within view.
	cursor int
	// rowOff is the index within view of the first visible row.
	rowOff int
	// colOff is the index of the leftmost visible column.
	colOff int

	width  int
	height int

	styles    theme.Styles
	scrollbar Scrollbar
}

// New returns a Table ready for use with the supplied styles and default
// viewport dimensions. Call SetSize to match the available terminal area and
// SetData to load content.
func New(styles theme.Styles) *Table {
	return &Table{
		sortCol:  -1,
		sortAsc:  true,
		width:    defaultWidth,
		height:   defaultHeight,
		styles:   styles,
		provider: sliceProvider{},
	}
}

// SetStyles replaces the styles used for rendering. This makes a theme change a
// single, bounded operation.
func (t *Table) SetStyles(styles theme.Styles) {
	t.styles = styles
}

// SetSize sets the viewport dimensions in terminal cells. The height includes
// the header line, so the number of visible data rows is height-1.
func (t *Table) SetSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	t.width = width
	t.height = height
	t.clampViewport()
}

// SetData replaces the table's columns and rows and resets navigation, sort,
// and filter state. It is the in-memory entry point: rows are wrapped in a
// RowProvider so the in-memory and streaming paths share identical rendering
// and navigation logic.
func (t *Table) SetData(rows []Row, columns []Column) {
	t.SetProvider(sliceProvider{rows: rows}, columns)
}

// SetProvider backs the table with a RowProvider so rows are pulled on demand
// rather than held in a slice. Use it for streaming or lazily-loaded data sets
// that should not be fully materialized in memory. Like SetData it resets
// navigation, sort, and filter state.
//
// When no filter or sort is active the table reads rows straight from the
// provider for the visible window only; it never iterates the whole dataset.
// Applying a filter or sort necessarily walks the provider once to build the
// display order, so callers backing the table with an out-of-core provider
// should expect those operations to pull every row.
func (t *Table) SetProvider(provider RowProvider, columns []Column) {
	if provider == nil {
		provider = sliceProvider{}
	}
	t.provider = provider
	t.columns = columns
	t.query = ""
	t.sortCol = -1
	t.sortAsc = true
	t.cursor = 0
	t.rowOff = 0
	t.colOff = 0
	t.rebuildView()
}

// rebuildView recomputes the display order from the current provider, applying
// the active filter then the active sort. When neither a filter nor a sort is
// active it clears t.view (the identity view) so the table reads rows directly
// from the provider without materializing an index slice — this is the bounded,
// non-scanning path used for streaming data. A filter or sort requires a single
// pass over the provider to compute the order.
func (t *Table) rebuildView() {
	n := t.provider.Len()
	q := strings.ToLower(t.query)

	// Fast path: no filter and no sort means the display order is exactly the
	// provider order, so we avoid building (and storing) an index slice.
	if q == "" && t.sortCol < 0 {
		t.view = nil
		t.clampViewport()
		return
	}

	t.view = t.view[:0]
	if cap(t.view) < n {
		t.view = make([]int, 0, n)
	}

	for i := 0; i < n; i++ {
		if q == "" {
			t.view = append(t.view, i)
			continue
		}
		if r, ok := t.provider.RowAt(i); ok && rowMatches(r, q) {
			t.view = append(t.view, i)
		}
	}

	if t.sortCol >= 0 {
		col := t.sortCol
		asc := t.sortAsc
		sort.SliceStable(t.view, func(a, b int) bool {
			av := cellAt(t.rowAt(t.view[a]), col)
			bv := cellAt(t.rowAt(t.view[b]), col)
			if asc {
				return av < bv
			}
			return av > bv
		})
	}

	t.clampViewport()
}

// viewLen returns the number of rows in the active display view. With an active
// filter or sort this is len(t.view); otherwise it is the provider length
// (identity view).
func (t *Table) viewLen() int {
	if t.view != nil {
		return len(t.view)
	}
	return t.provider.Len()
}

// providerIndex maps a position within the active view to the underlying
// provider index. With the identity view (no filter/sort) the position is the
// provider index.
func (t *Table) providerIndex(viewPos int) int {
	if t.view != nil {
		return t.view[viewPos]
	}
	return viewPos
}

// rowAt fetches a row from the provider by provider index, returning a zero Row
// when the index is out of range.
func (t *Table) rowAt(i int) Row {
	r, _ := t.provider.RowAt(i)
	return r
}

// rowMatches reports whether any cell of r contains the (already lower-cased)
// query substring.
func rowMatches(r Row, lowerQuery string) bool {
	for _, c := range r.Cells {
		if strings.Contains(strings.ToLower(c), lowerQuery) {
			return true
		}
	}
	return false
}

// cellAt safely returns the cell value at index col, or "" when out of range.
func cellAt(r Row, col int) string {
	if col < 0 || col >= len(r.Cells) {
		return ""
	}
	return r.Cells[col]
}

// SortBy orders the view by the given column index. Calling it repeatedly with
// the same column toggles between ascending and descending order. Columns that
// are out of range or not marked Sortable are ignored.
func (t *Table) SortBy(column int) {
	if column < 0 || column >= len(t.columns) {
		return
	}
	if !t.columns[column].Sortable {
		return
	}
	if t.sortCol == column {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = column
		t.sortAsc = true
	}
	t.rebuildView()
}

// Filter restricts the view to rows where any cell contains query
// (case-insensitive). An empty query clears the filter. The selection is reset
// to the top of the filtered view.
func (t *Table) Filter(query string) {
	t.query = query
	t.cursor = 0
	t.rowOff = 0
	t.rebuildView()
}

// Navigate updates the selection in response to a key. Recognized keys cover
// the universal Runtime navigation bindings:
//
//	up/k, down/j        move one row
//	left/h, right/l     move one column horizontally
//	pgup/ctrl+u         page up
//	pgdn/ctrl+d         page down
//	home/g              jump to top
//	end/G               jump to bottom
//
// Navigation only adjusts cursor and viewport offsets; it never scans or
// materializes rows outside the visible window, so its cost is independent of
// dataset size.
func (t *Table) Navigate(key string) {
	page := t.visibleRows()
	if page < 1 {
		page = 1
	}
	switch key {
	case "up", "k":
		t.moveCursor(-1)
	case "down", "j":
		t.moveCursor(1)
	case "pgup", "ctrl+u":
		t.moveCursor(-page)
	case "pgdn", "ctrl+d":
		t.moveCursor(page)
	case "home", "g":
		t.cursor = 0
	case "end", "G":
		t.cursor = t.viewLen() - 1
	case "left", "h":
		if t.colOff > 0 {
			t.colOff--
		}
	case "right", "l":
		if t.colOff < len(t.columns)-1 {
			t.colOff++
		}
	}
	t.clampViewport()
}

// moveCursor shifts the selection by delta rows, clamped to the view bounds.
func (t *Table) moveCursor(delta int) {
	t.cursor += delta
}

// clampViewport keeps cursor within the view and scrolls rowOff/colOff so the
// cursor stays visible. It is the only place scroll offsets are normalized.
func (t *Table) clampViewport() {
	n := t.viewLen()
	if n == 0 {
		t.cursor = 0
		t.rowOff = 0
		return
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor >= n {
		t.cursor = n - 1
	}

	visible := t.visibleRows()
	if visible < 1 {
		visible = 1
	}

	// Scroll up to reveal the cursor.
	if t.cursor < t.rowOff {
		t.rowOff = t.cursor
	}
	// Scroll down to reveal the cursor.
	if t.cursor >= t.rowOff+visible {
		t.rowOff = t.cursor - visible + 1
	}
	// Keep the offset within bounds.
	maxOff := n - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if t.rowOff > maxOff {
		t.rowOff = maxOff
	}
	if t.rowOff < 0 {
		t.rowOff = 0
	}

	if t.colOff < 0 {
		t.colOff = 0
	}
	if len(t.columns) > 0 && t.colOff > len(t.columns)-1 {
		t.colOff = len(t.columns) - 1
	}
}

// visibleRows is the number of data rows that fit in the viewport, reserving
// one line for the header.
func (t *Table) visibleRows() int {
	v := t.height - 1
	if v < 0 {
		v = 0
	}
	return v
}

// visibleColumns returns the inclusive-exclusive column index range that fits
// in the viewport width starting from colOff.
func (t *Table) visibleColumns() (start, end int) {
	start = t.colOff
	if start > len(t.columns) {
		start = len(t.columns)
	}
	used := 0
	end = start
	for end < len(t.columns) {
		w := t.columns[end].Width + 1 // +1 for the inter-column gap
		if used+w > t.width && end > start {
			break
		}
		used += w
		end++
	}
	return start, end
}

// Cursor returns the index of the currently selected row within the active
// (filtered/sorted) view, or -1 when the view is empty.
func (t *Table) Cursor() int {
	if t.viewLen() == 0 {
		return -1
	}
	return t.cursor
}

// SelectedRow returns the currently selected Row and true, or a zero Row and
// false when the view is empty.
func (t *Table) SelectedRow() (Row, bool) {
	if t.viewLen() == 0 {
		return Row{}, false
	}
	return t.rowAt(t.providerIndex(t.cursor)), true
}

// RowCount returns the number of rows in the active view.
func (t *Table) RowCount() int {
	return t.viewLen()
}

// Scrollbar returns the scroll state computed by the most recent render.
func (t *Table) Scrollbar() Scrollbar {
	return t.scrollbar
}

// View renders the visible window of the table into a string. Only the rows
// between the current offset and offset+visibleRows, and the columns that fit
// the viewport width, are materialized — this is the core virtualization
// guarantee that keeps rendering cost independent of dataset size.
func (t *Table) View() string {
	visible := t.visibleRows()
	colStart, colEnd := t.visibleColumns()

	// Update scroll state for the (possibly external) scrollbar.
	t.scrollbar = Scrollbar{
		Total:   t.viewLen(),
		Visible: visible,
		Offset:  t.rowOff,
	}

	var b strings.Builder

	// Header row.
	b.WriteString(t.renderHeader(colStart, colEnd))

	// Data rows: only the visible window is iterated.
	bar := t.scrollbar.Render(visible, t.styles)
	for i := 0; i < visible; i++ {
		b.WriteByte('\n')
		viewIdx := t.rowOff + i
		var line string
		if viewIdx < t.viewLen() {
			selected := viewIdx == t.cursor
			line = t.renderRow(t.rowAt(t.providerIndex(viewIdx)), colStart, colEnd, selected)
		} else {
			line = "" // pad short datasets to a stable viewport height
		}
		if i < len(bar) {
			b.WriteString(bar[i])
			b.WriteByte(' ')
		}
		b.WriteString(line)
	}

	return b.String()
}

// renderHeader builds the styled header line for the visible columns.
func (t *Table) renderHeader(colStart, colEnd int) string {
	cells := make([]string, 0, colEnd-colStart)
	for c := colStart; c < colEnd; c++ {
		col := t.columns[c]
		title := col.Title
		if t.sortCol == c {
			if t.sortAsc {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}
		cells = append(cells, fit(title, col.Width))
	}
	return t.styles.Header.Render(strings.Join(cells, " "))
}

// renderRow builds a single styled data line for the visible columns.
func (t *Table) renderRow(r Row, colStart, colEnd int, selected bool) string {
	cells := make([]string, 0, colEnd-colStart)
	for c := colStart; c < colEnd; c++ {
		cells = append(cells, fit(cellAt(r, c), t.columns[c].Width))
	}
	line := strings.Join(cells, " ")
	if selected {
		return t.styles.Selected.Render(line)
	}
	return t.styles.Body.Render(line)
}

// fit pads s with spaces or truncates it (with an ellipsis) to exactly w
// terminal cells, measured in runes.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) == w {
		return s
	}
	if len(r) < w {
		return s + strings.Repeat(" ", w-len(r))
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}
